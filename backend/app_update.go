package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	defaultUpdateRepo   = "black-ant/Ant-Browser"
	latestReleaseAPIURL = "https://api.github.com/repos/" + defaultUpdateRepo + "/releases/latest"
	manifestAssetName   = "update-manifest.json"
	updateCheckTimeout  = 5 * time.Second
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
	Error           string                    `json:"error"`
}

type appUpdateReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type githubLatestRelease struct {
	TagName string                  `json:"tag_name"`
	HTMLURL string                  `json:"html_url"`
	Assets  []appUpdateReleaseAsset `json:"assets"`
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

func (a *App) updateReleaseAPIURL() string {
	repo := defaultUpdateRepo
	if a != nil && a.config != nil && strings.TrimSpace(a.config.Update.Repo) != "" {
		repo = strings.TrimSpace(a.config.Update.Repo)
	}
	return fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
}

func (a *App) updateHTTPClient() *http.Client {
	proxyConfig := ""
	if a != nil && a.config != nil {
		proxyConfig = strings.TrimSpace(a.config.Update.Proxy)
	}
	client, _, err := a.unifiedProxyCoreHTTPClient(updateCheckTimeout, proxyConfig)
	if err != nil || client == nil {
		return &http.Client{Timeout: updateCheckTimeout}
	}
	return client
}

// CheckForUpdates 保持旧接口，兼容已生成的前端绑定。
func (a *App) CheckForUpdates() map[string]interface{} {
	result := checkForUpdates(context.Background(), a.appVersion(), a.updateReleaseAPIURL(), a.updateHTTPClient())
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
func (a *App) CheckAppUpdate() AppUpdateInfo {
	if a == nil || a.config == nil || !a.config.Update.IsEnabled() {
		currentVersion := ""
		if a != nil {
			currentVersion = normalizeUpdateVersion(a.appVersion())
		}
		return AppUpdateInfo{CurrentVersion: currentVersion, Error: "更新检查已关闭"}
	}
	return fetchAppUpdateInfo(context.Background(), a.appVersion(), a.updateReleaseAPIURL(), a.updateHTTPClient())
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

func fetchAppUpdateInfo(ctx context.Context, currentVersion string, endpoint string, client *http.Client) AppUpdateInfo {
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

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		info.Error = fmt.Sprintf("创建更新请求失败: %v", err)
		return info
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	request.Header.Set("User-Agent", "Ant-Browser/"+info.CurrentVersion)

	response, err := client.Do(request)
	if err != nil {
		info.Error = fmt.Sprintf("更新检查请求失败: %v", err)
		return info
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		info.Error = fmt.Sprintf("更新服务返回 HTTP %d", response.StatusCode)
		return info
	}

	var release githubLatestRelease
	if err := json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&release); err != nil {
		info.Error = fmt.Sprintf("更新信息解析失败: %v", err)
		return info
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

	downloadURLs := make(map[string]string, len(release.Assets))
	for _, asset := range release.Assets {
		downloadURLs[asset.Name] = asset.BrowserDownloadURL
	}
	for _, asset := range release.Assets {
		if !strings.EqualFold(asset.Name, manifestAssetName) {
			continue
		}
		manifest, manifestErr := fetchUpdateManifest(ctx, asset.BrowserDownloadURL, client)
		if manifestErr != nil || manifest == nil {
			continue
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
