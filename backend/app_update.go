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
	latestReleaseAPIURL = "https://api.github.com/repos/black-ant/Ant-Browser/releases/latest"
	updateCheckTimeout  = 5 * time.Second
)

type updateCheckResult struct {
	OK              bool
	CurrentVersion  string
	LatestVersion   string
	UpdateAvailable bool
	ReleaseURL      string
	ReleaseName     string
	PublishedAt     string
	Error           string
}

type githubLatestRelease struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	HTMLURL     string `json:"html_url"`
	PublishedAt string `json:"published_at"`
}

// CheckForUpdates 检查 GitHub Releases 中的最新稳定版本。
func (a *App) CheckForUpdates() map[string]interface{} {
	result := checkForUpdates(context.Background(), a.appVersion(), latestReleaseAPIURL, &http.Client{Timeout: updateCheckTimeout})
	return map[string]interface{}{
		"ok":              result.OK,
		"currentVersion":  result.CurrentVersion,
		"latestVersion":   result.LatestVersion,
		"updateAvailable": result.UpdateAvailable,
		"releaseUrl":      result.ReleaseURL,
		"releaseName":     result.ReleaseName,
		"publishedAt":     result.PublishedAt,
		"error":           result.Error,
	}
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
	result.ReleaseName = strings.TrimSpace(release.Name)
	result.PublishedAt = strings.TrimSpace(release.PublishedAt)
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
