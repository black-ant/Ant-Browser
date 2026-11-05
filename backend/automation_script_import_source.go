package backend

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"ant-chrome/backend/internal/automation"
)

const (
	automationRemoteImportTimeout = 20 * time.Second
	maxAutomationRemoteScriptSize = 16 << 20
)

func (a *App) loadAutomationBundleFromSource(source automation.ScriptSource) (automation.ImportedBundle, error) {
	sourceType := strings.TrimSpace(source.Type)
	switch sourceType {
	case "builtin":
		return automation.ImportBuiltinBundleFromSource(source)
	case "local-file":
		path := firstNonBlank(source.URI, source.Path)
		if path == "" {
			return automation.ImportedBundle{}, fmt.Errorf("本地脚本文件路径缺失")
		}
		return automation.ImportBundleFromFileWithOptions(path, buildAutomationImportSourceLabel(source), a.automationScriptImportOptions())
	case "local-dir":
		path := firstNonBlank(source.URI, source.Path)
		if path == "" {
			return automation.ImportedBundle{}, fmt.Errorf("本地脚本目录路径缺失")
		}
		return automation.ImportBundleFromDirectoryWithOptions(path, "", buildAutomationImportSourceLabel(source), a.automationScriptImportOptions())
	case "remote-url":
		return a.loadAutomationRemoteBundle(source.URI)
	case "git":
		return a.loadAutomationGitBundle(source.URI, source.Ref, source.Path)
	case "manual", "text", "":
		return automation.ImportedBundle{}, fmt.Errorf("当前脚本来源不支持重新导入")
	default:
		return automation.ImportedBundle{}, fmt.Errorf("当前脚本来源 %q 不支持重新导入", sourceType)
	}
}

func (a *App) loadAutomationRemoteBundle(rawURL string) (automation.ImportedBundle, error) {
	normalizedURL := strings.TrimSpace(rawURL)
	if normalizedURL == "" {
		return automation.ImportedBundle{}, fmt.Errorf("远程脚本地址不能为空")
	}

	parsedURL, err := url.Parse(normalizedURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return automation.ImportedBundle{}, fmt.Errorf("远程脚本地址不合法")
	}

	ctx, cancel := context.WithTimeout(context.Background(), automationRemoteImportTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, normalizedURL, nil)
	if err != nil {
		return automation.ImportedBundle{}, fmt.Errorf("创建远程脚本请求失败: %w", err)
	}

	resp, err := (&http.Client{Timeout: automationRemoteImportTimeout}).Do(req)
	if err != nil {
		return automation.ImportedBundle{}, fmt.Errorf("下载远程脚本失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return automation.ImportedBundle{}, fmt.Errorf("下载远程脚本失败: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxAutomationRemoteScriptSize+1))
	if err != nil {
		return automation.ImportedBundle{}, fmt.Errorf("读取远程脚本失败: %w", err)
	}
	if len(data) > maxAutomationRemoteScriptSize {
		return automation.ImportedBundle{}, fmt.Errorf("远程脚本文件过大")
	}

	nameHint := filepath.Base(parsedURL.Path)
	if nameHint == "" || nameHint == "." || nameHint == "/" {
		nameHint = "remote-script.cjs"
	}

	return automation.ImportBundleFromBytesWithOptions(nameHint, data, "远程地址 "+normalizedURL, a.automationScriptImportOptions())
}
