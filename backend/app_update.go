package backend

import (
	"ant-chrome/backend/internal/logger"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	defaultUpdateRepo     = "black-ant/Ant-Browser"
	latestReleaseAPIURL   = "https://api.github.com/repos/" + defaultUpdateRepo + "/releases/latest"
	manifestAssetName     = "update-manifest.json"
	updateCheckTimeout = 5 * time.Second
	// updateResolveTimeout 用于下载前解析更新包地址：此时请求可能要走用户临时选择的代理节点，
	// 内核桥接首次启动和跨境握手都明显慢于直连，5 秒检查超时会误判为失败。
	updateResolveTimeout  = 20 * time.Second
	updateDownloadTimeout = 30 * time.Minute
)

type updateCheckResult struct {
	OK              bool
	CurrentVersion  string
	LatestVersion   string
	UpdateAvailable bool
	ReleaseURL      string
	Error           string
}

// AppUpdateAsset 是某个平台可下载的安装包信息。
type AppUpdateAsset struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

// AppUpdateInfo 是给前端使用的结构化更新信息。
type AppUpdateInfo struct {
	OK              bool                      `json:"ok"`
	CurrentVersion  string                    `json:"currentVersion"`
	LatestVersion   string                    `json:"latestVersion"`
	UpdateAvailable bool                      `json:"updateAvailable"`
	ReleaseURL      string                    `json:"releaseUrl"`
	NotesURL        string                    `json:"notesUrl"`
	Force           bool                      `json:"force"`
	Assets          map[string]AppUpdateAsset `json:"assets"`
	// Versions 是当前通道下高于当前版本的可选版本（按版本降序，最多 updateVersionOptionLimit 个）。
	Versions []string `json:"versions"`
	Error    string   `json:"error"`
}

// 版本下拉只允许少量选项：多版本并存容易出现清单/资产与版本对不上的问题。
const updateVersionOptionLimit = 2

type appUpdateReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type githubLatestRelease struct {
	TagName    string                  `json:"tag_name"`
	HTMLURL    string                  `json:"html_url"`
	Prerelease bool                    `json:"prerelease"`
	Assets     []appUpdateReleaseAsset `json:"assets"`
}

// 更新通道：stable 只取非预发布版本，preview 只取预发布版本。
const (
	UpdateChannelStable  = "stable"
	UpdateChannelPreview = "preview"
)

func normalizeUpdateChannel(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), UpdateChannelPreview) {
		return UpdateChannelPreview
	}
	return UpdateChannelStable
}

type appUpdateManifest struct {
	Version  string `json:"version"`
	NotesURL string `json:"notesUrl"`
	Force    bool   `json:"force"`
	Assets   map[string]struct {
		Name   string `json:"name"`
		SHA256 string `json:"sha256"`
	} `json:"assets"`
}

func (a *App) updateRepo() string {
	repo := defaultUpdateRepo
	if a != nil && a.config != nil && strings.TrimSpace(a.config.Update.Repo) != "" {
		repo = strings.TrimSpace(a.config.Update.Repo)
	}
	return repo
}

func (a *App) updateReleaseAPIURL() string {
	return fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", a.updateRepo())
}

// updateReleasesListURL 用于预览通道：GitHub 的 /releases/latest 会跳过预发布版本，
// 只能拉列表后按 prerelease 标记挑选。
func (a *App) updateReleasesListURL() string {
	return fmt.Sprintf("https://api.github.com/repos/%s/releases?per_page=30", a.updateRepo())
}

func (a *App) updateProxyConfig() string {
	if a == nil || a.config == nil {
		return ""
	}
	return strings.TrimSpace(a.config.Update.Proxy)
}

func (a *App) updateHTTPClient() (*http.Client, error) {
	return a.updateHTTPClientFor(a.updateProxyConfig())
}

func (a *App) updateHTTPClientFor(proxyConfig string) (*http.Client, error) {
	return a.updateHTTPClientForSelection(updateCheckTimeout, proxyConfig, "")
}

func (a *App) updateDownloadHTTPClientFor(proxyConfig string, proxyID string) (*http.Client, error) {
	return a.updateHTTPClientForSelection(updateDownloadTimeout, proxyConfig, proxyID)
}

func (a *App) updateHTTPClientForSelection(timeout time.Duration, proxyConfig string, proxyID string) (*http.Client, error) {
	client, _, err := a.unifiedProxyCoreHTTPClientForProxyID(timeout, strings.TrimSpace(proxyConfig), strings.TrimSpace(proxyID))
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, fmt.Errorf("更新代理不可用")
	}
	return client, nil
}

// CheckForUpdates 保持旧接口，兼容已生成的前端绑定。
func (a *App) CheckForUpdates() map[string]interface{} {
	currentVersion := normalizeUpdateVersion(a.appVersion())
	client, err := a.updateHTTPClient()
	if err != nil {
		return map[string]interface{}{
			"ok":              false,
			"currentVersion":  currentVersion,
			"latestVersion":   "",
			"updateAvailable": false,
			"releaseUrl":      "",
			"error":           "更新代理配置错误: " + err.Error(),
		}
	}
	result := checkForUpdates(context.Background(), a.appVersion(), a.updateReleaseAPIURL(), client)
	return map[string]interface{}{
		"ok":              result.OK,
		"currentVersion":  result.CurrentVersion,
		"latestVersion":   result.LatestVersion,
		"updateAvailable": result.UpdateAvailable,
		"releaseUrl":      result.ReleaseURL,
		"error":           result.Error,
	}
}

// CheckAppUpdate 返回结构化的更新信息，包含各平台安装包与校验值。
// 使用配置里的更新代理（默认直连）；需要在界面上临时指定节点时走 CheckAppUpdateWithProxySelection。
func (a *App) CheckAppUpdate() AppUpdateInfo {
	return a.CheckAppUpdateWithProxySelection(a.updateProxyConfig(), "")
}

// CheckAppUpdateWithProxySelection 允许前端先选代理再检查更新，不写入配置。
// 更新清单放在 github.com 上，直连被阻断时只能靠代理节点读取，所以检查阶段也必须支持选节点。
func (a *App) CheckAppUpdateWithProxySelection(proxyConfig string, proxyID string) AppUpdateInfo {
	return a.CheckAppUpdateWithChannel(proxyConfig, proxyID, UpdateChannelStable)
}

// CheckAppUpdateWithChannel 在指定通道上检查更新：stable 只认非预发布版本，preview 只认预发布版本。
func (a *App) CheckAppUpdateWithChannel(proxyConfig string, proxyID string, channel string) AppUpdateInfo {
	return a.CheckAppUpdateVersion(proxyConfig, proxyID, channel, "")
}

// CheckAppUpdateVersion 允许查看指定版本的更新信息；version 为空时取通道内最新版本。
func (a *App) CheckAppUpdateVersion(proxyConfig string, proxyID string, channel string, version string) AppUpdateInfo {
	if a == nil || a.config == nil || !a.config.Update.IsEnabled() {
		currentVersion := ""
		if a != nil {
			currentVersion = normalizeUpdateVersion(a.appVersion())
		}
		return AppUpdateInfo{CurrentVersion: currentVersion, Assets: map[string]AppUpdateAsset{}, Error: "更新检查已关闭"}
	}
	client, err := a.updateHTTPClientForSelection(updateResolveTimeout, proxyConfig, proxyID)
	if err != nil {
		logger.New("AppUpdate").Warn("检查更新时构建代理客户端失败",
			logger.F("proxy_id", strings.TrimSpace(proxyID)),
			logger.F("error", err.Error()),
		)
		return AppUpdateInfo{
			CurrentVersion: normalizeUpdateVersion(a.appVersion()),
			Assets:         map[string]AppUpdateAsset{},
			Error:          "更新代理配置错误: " + err.Error(),
		}
	}
	return fetchAppUpdateInfoForChannel(
		context.Background(),
		a.appVersion(),
		normalizeUpdateChannel(channel),
		version,
		a.updateReleasesListURL(),
		client,
	)
}

func checkForUpdates(ctx context.Context, currentVersion string, endpoint string, client *http.Client) updateCheckResult {
	result := updateCheckResult{CurrentVersion: normalizeUpdateVersion(currentVersion)}
	if result.CurrentVersion == "" || strings.EqualFold(result.CurrentVersion, "unknown") {
		result.Error = "当前应用版本未知"
		return result
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		result.Error = fmt.Sprintf("创建更新请求失败: %v", err)
		return result
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	request.Header.Set("User-Agent", "Ant-Browser/"+result.CurrentVersion)

	if client == nil {
		client = &http.Client{Timeout: updateCheckTimeout}
	}
	response, err := client.Do(request)
	if err != nil {
		result.Error = fmt.Sprintf("更新检查请求失败: %v", err)
		return result
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		result.Error = fmt.Sprintf("更新服务返回 HTTP %d", response.StatusCode)
		return result
	}

	var release githubLatestRelease
	if err := json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&release); err != nil {
		result.Error = fmt.Sprintf("更新信息解析失败: %v", err)
		return result
	}

	result.LatestVersion = normalizeUpdateVersion(release.TagName)
	result.ReleaseURL = strings.TrimSpace(release.HTMLURL)
	if result.LatestVersion == "" {
		result.Error = "最新 Release 缺少有效版本号"
		return result
	}

	comparison, err := compareUpdateVersions(result.CurrentVersion, result.LatestVersion)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.OK = true
	result.UpdateAvailable = comparison < 0
	return result
}

func newUpdateRequest(ctx context.Context, endpoint string, currentVersion string) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("创建更新请求失败: %v", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	request.Header.Set("User-Agent", "Ant-Browser/"+currentVersion)
	return request, nil
}

// fetchUpdateReleases 拉取 Release 列表：正式/预览通道共用，便于列出版本供选择。
func fetchUpdateReleases(ctx context.Context, listURL string, currentVersion string, client *http.Client) ([]githubLatestRelease, error) {
	if strings.TrimSpace(listURL) == "" {
		return nil, fmt.Errorf("缺少更新列表地址")
	}
	request, err := newUpdateRequest(ctx, listURL, currentVersion)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("更新检查请求失败: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("更新服务返回 HTTP %d", response.StatusCode)
	}
	var releases []githubLatestRelease
	if err := json.NewDecoder(io.LimitReader(response.Body, 8<<20)).Decode(&releases); err != nil {
		return nil, fmt.Errorf("更新信息解析失败: %v", err)
	}
	return releases, nil
}

// filterReleasesByChannel 过滤出当前通道的版本并按版本号降序排列。
func filterReleasesByChannel(releases []githubLatestRelease, channel string) []githubLatestRelease {
	filtered := make([]githubLatestRelease, 0, len(releases))
	for _, item := range releases {
		if normalizeUpdateVersion(item.TagName) == "" {
			continue
		}
		if channel == UpdateChannelPreview {
			if !item.Prerelease {
				continue
			}
		} else if item.Prerelease {
			continue
		}
		filtered = append(filtered, item)
	}
	sort.SliceStable(filtered, func(left int, right int) bool {
		comparison, err := compareUpdateVersions(filtered[left].TagName, filtered[right].TagName)
		return err == nil && comparison > 0
	})
	return filtered
}

// fetchAppUpdateInfo 保持旧签名（正式通道最新版本），供既有测试与调用方使用。
func fetchAppUpdateInfo(ctx context.Context, currentVersion string, endpoint string, client *http.Client) AppUpdateInfo {
	return fetchAppUpdateInfoForChannel(ctx, currentVersion, UpdateChannelStable, "", endpoint, client)
}

// fetchAppUpdateInfoForChannel 按通道取更新信息；targetVersion 非空时取指定版本。
// 不论正式还是预览通道，先拉列表再按通道过滤，这样同一个响应就能支撑版本下拉。
func fetchAppUpdateInfoForChannel(ctx context.Context, currentVersion string, channel string, targetVersion string, listURL string, client *http.Client) AppUpdateInfo {
	info := AppUpdateInfo{
		CurrentVersion: normalizeUpdateVersion(currentVersion),
		Assets:         map[string]AppUpdateAsset{},
	}
	if info.CurrentVersion == "" || strings.EqualFold(info.CurrentVersion, "unknown") {
		info.Error = "当前应用版本未知"
		return info
	}
	if client == nil {
		client = &http.Client{Timeout: updateCheckTimeout}
	}

	releases, err := fetchUpdateReleases(ctx, listURL, info.CurrentVersion, client)
	if err != nil {
		info.Error = err.Error()
		return info
	}
	normalizedChannel := normalizeUpdateChannel(channel)
	candidates := filterReleasesByChannel(releases, normalizedChannel)
	if len(candidates) == 0 {
		if normalizedChannel == UpdateChannelPreview {
			info.Error = "预览通道当前没有可用的预览版本"
		} else {
			info.Error = "没有可用的正式版本"
		}
		return info
	}

	release := candidates[0]
	if wanted := normalizeUpdateVersion(targetVersion); wanted != "" {
		matched := false
		for _, item := range candidates {
			if normalizeUpdateVersion(item.TagName) == wanted {
				release = item
				matched = true
				break
			}
		}
		if !matched {
			info.Error = "没有找到版本 v" + wanted
			return info
		}
	}
	info.LatestVersion = normalizeUpdateVersion(release.TagName)
	info.ReleaseURL = strings.TrimSpace(release.HTMLURL)
	info.NotesURL = info.ReleaseURL
	if info.LatestVersion == "" {
		info.Error = "最新 Release 缺少有效版本号"
		return info
	}
	comparison, err := compareUpdateVersions(info.CurrentVersion, info.LatestVersion)
	if err != nil {
		info.Error = err.Error()
		return info
	}
	info.OK = true
	info.UpdateAvailable = comparison < 0
	// 只列高于当前版本的版本，最多 updateVersionOptionLimit 个，避免归档版本带来的不一致。
	for _, item := range candidates {
		candidateVersion := normalizeUpdateVersion(item.TagName)
		candidateComparison, compareErr := compareUpdateVersions(info.CurrentVersion, candidateVersion)
		if compareErr != nil || candidateComparison >= 0 {
			continue
		}
		info.Versions = append(info.Versions, candidateVersion)
		if len(info.Versions) >= updateVersionOptionLimit {
			break
		}
	}

	downloadURLs := make(map[string]string, len(release.Assets))
	for _, asset := range release.Assets {
		downloadURLs[asset.Name] = asset.BrowserDownloadURL
	}
	manifestFound := false
	for _, asset := range release.Assets {
		if !strings.EqualFold(asset.Name, manifestAssetName) {
			continue
		}
		manifestFound = true
		manifest, manifestErr := fetchUpdateManifest(ctx, asset.BrowserDownloadURL, client)
		if manifestErr != nil || manifest == nil {
			detail := "未知错误"
			if manifestErr != nil {
				detail = manifestErr.Error()
			}
			info.OK = false
			info.UpdateAvailable = false
			info.Error = "读取更新清单失败: " + detail
			return info
		}
		if normalizeUpdateVersion(manifest.Version) != info.LatestVersion {
			info.OK = false
			info.UpdateAvailable = false
			info.Error = "更新清单版本与 Release 不一致"
			return info
		}
		info.Force = manifest.Force
		if strings.TrimSpace(manifest.NotesURL) != "" {
			info.NotesURL = strings.TrimSpace(manifest.NotesURL)
		}
		for key, item := range manifest.Assets {
			downloadURL := downloadURLs[item.Name]
			if downloadURL == "" {
				continue
			}
			info.Assets[key] = AppUpdateAsset{
				Name:   item.Name,
				URL:    downloadURL,
				SHA256: strings.TrimSpace(item.SHA256),
			}
		}
		break
	}
	if info.UpdateAvailable {
		if !manifestFound {
			info.OK = false
			info.UpdateAvailable = false
			info.Error = "Release 缺少 update-manifest.json"
			return info
		}
		if len(info.Assets) == 0 {
			info.OK = false
			info.UpdateAvailable = false
			info.Error = "当前平台没有可用的更新包"
			return info
		}
		if _, ok := info.Assets[updateAssetKey()]; !ok {
			info.OK = false
			info.UpdateAvailable = false
			info.Error = "当前平台没有可用的更新包: " + updateAssetKey()
			return info
		}
	}
	return info
}

func fetchUpdateManifest(ctx context.Context, manifestURL string, client *http.Client) (*appUpdateManifest, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", response.StatusCode)
	}
	var manifest appUpdateManifest
	if err := json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}

func normalizeUpdateVersion(value string) string {
	version := strings.TrimSpace(value)
	version = strings.TrimPrefix(version, "v")
	version = strings.TrimPrefix(version, "V")
	if metadataIndex := strings.Index(version, "+"); metadataIndex >= 0 {
		version = version[:metadataIndex]
	}
	return strings.TrimSpace(version)
}

func compareUpdateVersions(left string, right string) (int, error) {
	leftVersion, err := parseUpdateVersion(left)
	if err != nil {
		return 0, err
	}
	rightVersion, err := parseUpdateVersion(right)
	if err != nil {
		return 0, err
	}

	coreLength := len(leftVersion.core)
	if len(rightVersion.core) > coreLength {
		coreLength = len(rightVersion.core)
	}
	for index := 0; index < coreLength; index++ {
		leftPart := 0
		if index < len(leftVersion.core) {
			leftPart = leftVersion.core[index]
		}
		rightPart := 0
		if index < len(rightVersion.core) {
			rightPart = rightVersion.core[index]
		}
		if leftPart < rightPart {
			return -1, nil
		}
		if leftPart > rightPart {
			return 1, nil
		}
	}

	if len(leftVersion.prerelease) == 0 && len(rightVersion.prerelease) > 0 {
		return 1, nil
	}
	if len(leftVersion.prerelease) > 0 && len(rightVersion.prerelease) == 0 {
		return -1, nil
	}
	for index := 0; index < len(leftVersion.prerelease) && index < len(rightVersion.prerelease); index++ {
		comparison := compareUpdatePrereleasePart(leftVersion.prerelease[index], rightVersion.prerelease[index])
		if comparison != 0 {
			return comparison, nil
		}
	}
	if len(leftVersion.prerelease) < len(rightVersion.prerelease) {
		return -1, nil
	}
	if len(leftVersion.prerelease) > len(rightVersion.prerelease) {
		return 1, nil
	}
	return 0, nil
}

type parsedUpdateVersion struct {
	core       []int
	prerelease []string
}

func parseUpdateVersion(value string) (parsedUpdateVersion, error) {
	normalized := normalizeUpdateVersion(value)
	if normalized == "" {
		return parsedUpdateVersion{}, fmt.Errorf("版本号为空")
	}
	parts := strings.SplitN(normalized, "-", 2)
	coreParts := strings.Split(parts[0], ".")
	parsed := parsedUpdateVersion{core: make([]int, 0, len(coreParts))}
	for _, part := range coreParts {
		if part == "" {
			return parsedUpdateVersion{}, fmt.Errorf("无效版本号: %s", value)
		}
		for _, character := range part {
			if character < '0' || character > '9' {
				return parsedUpdateVersion{}, fmt.Errorf("无效版本号: %s", value)
			}
		}
		parsedNumber, err := strconv.Atoi(part)
		if err != nil {
			return parsedUpdateVersion{}, fmt.Errorf("无效版本号: %s", value)
		}
		parsed.core = append(parsed.core, parsedNumber)
	}
	if len(parts) == 2 {
		for _, identifier := range strings.Split(parts[1], ".") {
			if identifier == "" {
				return parsedUpdateVersion{}, fmt.Errorf("无效版本号: %s", value)
			}
			parsed.prerelease = append(parsed.prerelease, identifier)
		}
	}
	return parsed, nil
}

func compareUpdatePrereleasePart(left string, right string) int {
	leftNumber, leftIsNumber := parseUpdatePrereleaseNumber(left)
	rightNumber, rightIsNumber := parseUpdatePrereleaseNumber(right)
	if leftIsNumber && rightIsNumber {
		if leftNumber < rightNumber {
			return -1
		}
		if leftNumber > rightNumber {
			return 1
		}
		return 0
	}
	if leftIsNumber != rightIsNumber {
		if leftIsNumber {
			return -1
		}
		return 1
	}
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	return 0
}

func parseUpdatePrereleaseNumber(value string) (int, bool) {
	if value == "" {
		return 0, false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return 0, false
		}
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}
	return parsed, true
}
