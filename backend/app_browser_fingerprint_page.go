package backend

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const fingerprintCheckBookmarkURL = "ant://fingerprint-check"

// BrowserInstanceOpenFingerprintCheck 启动或复用实例，并在目标浏览器内打开本地指纹检测页。
func (a *App) BrowserInstanceOpenFingerprintCheck(profileId string) (*BrowserProfile, error) {
	profileId = strings.TrimSpace(profileId)
	if profileId == "" {
		return nil, fmt.Errorf("实例 ID 不能为空")
	}
	return a.browserInstanceStartInternal(profileId, nil, []string{fingerprintCheckBookmarkURL}, true, true, false, "", "")
}

func (a *App) ensureFingerprintCheckPageURL(profileId string) (string, error) {
	profile, err := a.fingerprintCheckProfileSnapshot(profileId)
	if err != nil {
		return "", err
	}
	expectedArgs := a.fingerprintCheckExpectedArgsFromProfile(profile)
	return a.ensureFingerprintCheckPageURLForExpectedArgsAndProfile(profileId, expectedArgs, profile, true)
}

func (a *App) ensureFingerprintCheckPageBookmarkURL(profileId string) (string, error) {
	profile, err := a.fingerprintCheckProfileSnapshot(profileId)
	if err != nil {
		return "", err
	}
	expectedArgs := a.fingerprintCheckExpectedArgsFromProfile(profile)
	return a.ensureFingerprintCheckPageURLForExpectedArgsAndProfile(profileId, expectedArgs, profile, false)
}

func (a *App) ensureFingerprintCheckPageURLForProfile(profileId string, coreId string, fingerprintArgs []string, withTimestamp bool) (string, error) {
	expectedArgs := a.buildFingerprintCheckExpectedArgs(profileId, coreId, fingerprintArgs, nil)
	profile, _ := a.fingerprintCheckProfileSnapshot(profileId)
	return a.ensureFingerprintCheckPageURLForExpectedArgsAndProfile(profileId, expectedArgs, profile, withTimestamp)
}

func (a *App) ensureFingerprintCheckPageURLForExpectedArgs(profileId string, expectedArgs []string, withTimestamp bool) (string, error) {
	return a.ensureFingerprintCheckPageURLForExpectedArgsAndProfile(profileId, expectedArgs, nil, withTimestamp)
}

func (a *App) ensureFingerprintCheckPageURLForExpectedArgsAndProfile(profileId string, expectedArgs []string, profile *BrowserProfile, withTimestamp bool) (string, error) {
	pagePath, err := a.writeFingerprintCheckPageForExpectedArgsAndProfile(profileId, expectedArgs, profile)
	if err != nil {
		return "", err
	}
	return fingerprintCheckPageFileURL(pagePath, profileId, withTimestamp), nil
}

func (a *App) writeFingerprintCheckPageForProfile(profileId string, coreId string, fingerprintArgs []string) (string, error) {
	expectedArgs := a.buildFingerprintCheckExpectedArgs(profileId, coreId, fingerprintArgs, nil)
	profile, _ := a.fingerprintCheckProfileSnapshot(profileId)
	return a.writeFingerprintCheckPageForExpectedArgsAndProfile(profileId, expectedArgs, profile)
}

func (a *App) writeFingerprintCheckPageForExpectedArgs(profileId string, expectedArgs []string) (string, error) {
	return a.writeFingerprintCheckPageForExpectedArgsAndProfile(profileId, expectedArgs, nil)
}

func (a *App) writeFingerprintCheckPageForExpectedArgsAndProfile(profileId string, expectedArgs []string, profile *BrowserProfile) (string, error) {
	pageDir := a.resolveAppPath(filepath.ToSlash(filepath.Join("data", "fingerprint-check", safeFingerprintCheckProfilePath(profileId))))
	if err := os.MkdirAll(pageDir, 0o755); err != nil {
		return "", fmt.Errorf("创建指纹检测页目录失败: %w", err)
	}
	contextData, err := a.buildFingerprintCheckPageContextForExpectedArgsAndProfile(profileId, expectedArgs, profile)
	if err != nil {
		return "", err
	}
	pagePath := filepath.Join(pageDir, "index.html")
	pageHTML := strings.Replace(fingerprintCheckHTML, "__FINGERPRINT_CHECK_CONTEXT__", string(contextData), 1)
	if err := os.WriteFile(pagePath, []byte(pageHTML), 0o644); err != nil {
		return "", fmt.Errorf("写入指纹检测页失败: %w", err)
	}
	return pagePath, nil
}

func fingerprintCheckPageFileURL(pagePath string, profileId string, withTimestamp bool) string {
	urlPath := filepath.ToSlash(pagePath)
	if len(urlPath) >= 2 && urlPath[1] == ':' {
		urlPath = "/" + urlPath
	}
	fileURL := url.URL{Scheme: "file", Path: urlPath}
	query := fileURL.Query()
	query.Set("profileId", profileId)
	if withTimestamp {
		query.Set("ts", fmt.Sprintf("%d", time.Now().UnixNano()))
	}
	fileURL.RawQuery = query.Encode()
	return fileURL.String()
}

func safeFingerprintCheckProfilePath(profileId string) string {
	profileId = strings.TrimSpace(profileId)
	if profileId == "" {
		return "unknown"
	}
	var builder strings.Builder
	for _, char := range profileId {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '-' || char == '_' {
			builder.WriteRune(char)
		} else {
			builder.WriteByte('_')
		}
	}
	if builder.Len() == 0 {
		return "unknown"
	}
	return builder.String()
}

func (a *App) resolveFingerprintCheckStartURL(profileId string, targetURL string) string {
	expectedArgs, err := a.fingerprintCheckProfileExpectedArgs(profileId)
	if err != nil {
		return targetURL
	}
	return a.resolveFingerprintCheckStartURLForExpectedArgs(profileId, expectedArgs, targetURL)
}

func (a *App) resolveFingerprintCheckStartURLForProfile(profileId string, coreId string, fingerprintArgs []string, targetURL string) string {
	expectedArgs := a.buildFingerprintCheckExpectedArgs(profileId, coreId, fingerprintArgs, nil)
	return a.resolveFingerprintCheckStartURLForExpectedArgs(profileId, expectedArgs, targetURL)
}

func (a *App) resolveFingerprintCheckStartURLForExpectedArgs(profileId string, expectedArgs []string, targetURL string) string {
	return a.resolveFingerprintCheckStartURLForExpectedArgsAndProfile(profileId, expectedArgs, nil, targetURL)
}

func (a *App) resolveFingerprintCheckStartURLForExpectedArgsAndProfile(profileId string, expectedArgs []string, profile *BrowserProfile, targetURL string) string {
	if !strings.EqualFold(strings.TrimSpace(targetURL), fingerprintCheckBookmarkURL) {
		return targetURL
	}
	pageURL, err := a.ensureFingerprintCheckPageURLForExpectedArgsAndProfile(profileId, expectedArgs, profile, true)
	if err != nil {
		return targetURL
	}
	return pageURL
}

func (a *App) resolveFingerprintCheckStartURLs(profileId string, urls []string) []string {
	expectedArgs, err := a.fingerprintCheckProfileExpectedArgs(profileId)
	if err != nil {
		return append([]string{}, urls...)
	}
	return a.resolveFingerprintCheckStartURLsForExpectedArgs(profileId, expectedArgs, urls)
}

func (a *App) resolveFingerprintCheckStartURLsForProfile(profileId string, coreId string, fingerprintArgs []string, urls []string) []string {
	expectedArgs := a.buildFingerprintCheckExpectedArgs(profileId, coreId, fingerprintArgs, nil)
	return a.resolveFingerprintCheckStartURLsForExpectedArgs(profileId, expectedArgs, urls)
}

func (a *App) resolveFingerprintCheckStartURLsForExpectedArgs(profileId string, expectedArgs []string, urls []string) []string {
	return a.resolveFingerprintCheckStartURLsForExpectedArgsAndProfile(profileId, expectedArgs, nil, urls)
}

func (a *App) resolveFingerprintCheckStartURLsForExpectedArgsAndProfile(profileId string, expectedArgs []string, profile *BrowserProfile, urls []string) []string {
	if len(urls) == 0 {
		return urls
	}
	out := append([]string{}, urls...)
	for index, item := range out {
		out[index] = a.resolveFingerprintCheckStartURLForExpectedArgsAndProfile(profileId, expectedArgs, profile, item)
	}
	return out
}

func (a *App) runtimeBookmarksForProfile(profileId string, bookmarks []BrowserBookmark) ([]BrowserBookmark, string, error) {
	profile, err := a.fingerprintCheckProfileSnapshot(profileId)
	if err != nil {
		return nil, "", err
	}
	expectedArgs := a.fingerprintCheckExpectedArgsFromProfile(profile)
	return a.runtimeBookmarksForProfileExpectedArgsAndProfile(profileId, expectedArgs, profile, bookmarks)
}

func (a *App) runtimeBookmarksForProfileData(profileId string, coreId string, fingerprintArgs []string, bookmarks []BrowserBookmark) ([]BrowserBookmark, string, error) {
	expectedArgs := a.buildFingerprintCheckExpectedArgs(profileId, coreId, fingerprintArgs, nil)
	profile, _ := a.fingerprintCheckProfileSnapshot(profileId)
	return a.runtimeBookmarksForProfileExpectedArgsAndProfile(profileId, expectedArgs, profile, bookmarks)
}

func (a *App) runtimeBookmarksForProfileExpectedArgs(profileId string, expectedArgs []string, bookmarks []BrowserBookmark) ([]BrowserBookmark, string, error) {
	return a.runtimeBookmarksForProfileExpectedArgsAndProfile(profileId, expectedArgs, nil, bookmarks)
}

func (a *App) runtimeBookmarksForProfileExpectedArgsAndProfile(profileId string, expectedArgs []string, profile *BrowserProfile, bookmarks []BrowserBookmark) ([]BrowserBookmark, string, error) {
	if len(bookmarks) == 0 {
		return bookmarks, "", nil
	}
	needsFingerprintURL := false
	for _, item := range bookmarks {
		if strings.EqualFold(strings.TrimSpace(item.URL), fingerprintCheckBookmarkURL) {
			needsFingerprintURL = true
			break
		}
	}
	if !needsFingerprintURL {
		return append([]BrowserBookmark{}, bookmarks...), "", nil
	}
	pageURL, err := a.ensureFingerprintCheckPageURLForExpectedArgsAndProfile(profileId, expectedArgs, profile, false)
	if err != nil {
		return nil, "", err
	}
	out := append([]BrowserBookmark{}, bookmarks...)
	for index := range out {
		if strings.EqualFold(strings.TrimSpace(out[index].URL), fingerprintCheckBookmarkURL) {
			out[index].URL = pageURL
		}
	}
	return out, pageURL, nil
}

type fingerprintCheckPageContext struct {
	ProfileId  string                         `json:"profileId"`
	Expected   BrowserFingerprintExpectedInfo `json:"expected"`
	Proxy      fingerprintCheckProxyContext   `json:"proxy"`
	UILanguage string                         `json:"uiLanguage"`
}

type fingerprintCheckProxyContext struct {
	Configured bool   `json:"configured"`
	Direct     bool   `json:"direct"`
	ProxyId    string `json:"proxyId,omitempty"`
	ProxyName  string `json:"proxyName,omitempty"`
	GroupName  string `json:"groupName,omitempty"`
	Type       string `json:"type,omitempty"`
	Host       string `json:"host,omitempty"`
	Port       string `json:"port,omitempty"`
	HasAuth    bool   `json:"hasAuth,omitempty"`
	Summary    string `json:"summary,omitempty"`
}

func (a *App) buildFingerprintCheckPageContext(profileId string) ([]byte, error) {
	profile, err := a.fingerprintCheckProfileSnapshot(profileId)
	if err != nil {
		return nil, err
	}
	expectedArgs := a.fingerprintCheckExpectedArgsFromProfile(profile)
	return a.buildFingerprintCheckPageContextForExpectedArgsAndProfile(profileId, expectedArgs, profile)
}

func (a *App) buildFingerprintCheckPageContextForProfile(profileId string, coreId string, fingerprintArgs []string) ([]byte, error) {
	expectedArgs := a.buildFingerprintCheckExpectedArgs(profileId, coreId, fingerprintArgs, nil)
	profile, _ := a.fingerprintCheckProfileSnapshot(profileId)
	return a.buildFingerprintCheckPageContextForExpectedArgsAndProfile(profileId, expectedArgs, profile)
}

func (a *App) buildFingerprintCheckPageContextForExpectedArgs(profileId string, expectedArgs []string) ([]byte, error) {
	return a.buildFingerprintCheckPageContextForExpectedArgsAndProfile(profileId, expectedArgs, nil)
}

func (a *App) buildFingerprintCheckPageContextForExpectedArgsAndProfile(profileId string, expectedArgs []string, profile *BrowserProfile) ([]byte, error) {
	expected := buildBrowserFingerprintExpectedForBackend(expectedArgs, a.profileCoreBackend(profile))
	data, err := json.MarshalIndent(fingerprintCheckPageContext{
		ProfileId:  profileId,
		Expected:   expected,
		Proxy:      a.buildFingerprintCheckProxyContext(profile),
		UILanguage: fingerprintCheckPreferredUILanguage(expected),
	}, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("生成指纹检测上下文失败: %w", err)
	}
	return append(data, '\n'), nil
}

func (a *App) buildFingerprintCheckProxyContext(profile *BrowserProfile) fingerprintCheckProxyContext {
	if profile == nil {
		return fingerprintCheckProxyContext{Summary: "Not Configured"}
	}

	ctx := fingerprintCheckProxyContext{
		ProxyId:   strings.TrimSpace(profile.ProxyId),
		ProxyName: strings.TrimSpace(profile.ProxyBindName),
	}
	proxyConfig := strings.TrimSpace(profile.ProxyConfig)
	if a != nil && a.browserMgr != nil && ctx.ProxyId != "" {
		if proxyItem, ok := a.browserMgr.GetProxyByID(ctx.ProxyId); ok {
			if ctx.ProxyName == "" {
				ctx.ProxyName = strings.TrimSpace(proxyItem.ProxyName)
			}
			ctx.GroupName = strings.TrimSpace(proxyItem.GroupName)
			if proxyConfig == "" {
				proxyConfig = strings.TrimSpace(proxyItem.ProxyConfig)
			}
		}
	}
	if ctx.ProxyName == "" {
		ctx.ProxyName = strings.TrimSpace(profile.ProxyBindName)
	}

	ctx.Type, ctx.Host, ctx.Port, ctx.Summary, ctx.HasAuth, ctx.Direct = fingerprintCheckProxyDescriptor(proxyConfig)
	ctx.Configured = strings.TrimSpace(proxyConfig) != "" || ctx.ProxyId != "" || ctx.ProxyName != ""
	if !ctx.Configured {
		ctx.Summary = "Not Configured"
	}
	return ctx
}

func fingerprintCheckProxyDescriptor(raw string) (proxyType string, host string, port string, summary string, hasAuth bool, direct bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", "", "", "Not Configured", false, false
	}
	lower := strings.ToLower(trimmed)
	if lower == "direct://" || lower == "__direct__" || lower == "direct" {
		return "direct", "", "", "direct://", false, true
	}
	if strings.HasPrefix(lower, "chain+") {
		return "chain", "", "", "Chain Proxy", false, false
	}
	if parsed, err := url.Parse(trimmed); err == nil && parsed.Scheme != "" {
		proxyType = strings.ToLower(parsed.Scheme)
		host = parsed.Hostname()
		port = parsed.Port()
		hasAuth = parsed.User != nil
		if host != "" {
			summary = proxyType + "://" + host
			if port != "" {
				summary += ":" + port
			}
			if hasAuth {
				summary += " (auth)"
			}
			return proxyType, host, port, summary, hasAuth, false
		}
		return proxyType, "", "", proxyType + "://***", hasAuth, false
	}
	if strings.HasPrefix(trimmed, "{") || strings.Contains(lower, "outbounds") || strings.Contains(lower, "proxies:") {
		return "config", "", "", "Structured Proxy Config", false, false
	}
	return "custom", "", "", "Custom Proxy Config", false, false
}

func fingerprintCheckPreferredUILanguage(expected BrowserFingerprintExpectedInfo) string {
	if fingerprintCheckPlatformUsesEnglish(expected.Platform) || fingerprintCheckPlatformUsesEnglish(expected.PlatformVersion) {
		return "en"
	}
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		return "en"
	}
	return "zh"
}

func fingerprintCheckPlatformUsesEnglish(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	return strings.Contains(normalized, "linux") || strings.Contains(normalized, "x11") || strings.Contains(normalized, "mac") || strings.Contains(normalized, "darwin")
}

func (a *App) fingerprintCheckProfileExpectedArgs(profileId string) ([]string, error) {
	profile, err := a.fingerprintCheckProfileSnapshot(profileId)
	if err != nil {
		return nil, err
	}
	return a.fingerprintCheckExpectedArgsFromProfile(profile), nil
}

func (a *App) buildFingerprintCheckExpectedArgs(profileId string, coreId string, fingerprintArgs []string, launchArgs []string) []string {
	fingerprintLaunchArgs := a.buildBrowserFingerprintCapabilityReport(profileId, coreId, fingerprintArgs).LaunchArgs
	sanitizedLaunchArgs, _ := sanitizeManagedLaunchArgs(launchArgs)
	return combineFingerprintExpectedArgs(fingerprintLaunchArgs, sanitizedLaunchArgs)
}

func combineFingerprintExpectedArgs(argGroups ...[]string) []string {
	result := make([]string, 0)
	for _, args := range argGroups {
		result = append(result, normalizeNonEmptyStrings(args)...)
	}
	return result
}

func (a *App) fingerprintCheckExpectedArgsFromProfile(profile *BrowserProfile) []string {
	if profile == nil {
		return nil
	}
	if browserProfileMayHaveRuntimeLaunchArgs(profile) {
		if args := normalizeNonEmptyStrings(profile.LastLaunchArgs); len(args) > 0 {
			return args
		}
		if args := a.recoverBrowserLaunchArgsForProfile(profile); len(args) > 0 {
			profile.LastLaunchArgs = append([]string{}, args...)
			return args
		}
	}
	return a.buildFingerprintCheckExpectedArgs(profile.ProfileId, profile.CoreId, profile.FingerprintArgs, profile.LaunchArgs)
}

func (a *App) fingerprintCheckExpectedArgsFromLockedProfile(profile *BrowserProfile) []string {
	if profile == nil {
		return nil
	}
	if browserProfileMayHaveRuntimeLaunchArgs(profile) {
		if args := normalizeNonEmptyStrings(profile.LastLaunchArgs); len(args) > 0 {
			return args
		}
		if args := a.recoverBrowserLaunchArgsForProfile(profile); len(args) > 0 {
			profile.LastLaunchArgs = append([]string{}, args...)
			return args
		}
	}
	return a.buildFingerprintCheckExpectedArgs(profile.ProfileId, profile.CoreId, profile.FingerprintArgs, profile.LaunchArgs)
}

func browserProfileMayHaveRuntimeLaunchArgs(profile *BrowserProfile) bool {
	return profile != nil && (profile.Running || profile.Pid > 0 || profile.DebugPort > 0)
}

func (a *App) recoverBrowserLaunchArgsForProfile(profile *BrowserProfile) []string {
	if a == nil || a.browserMgr == nil || profile == nil {
		return nil
	}
	if a.browserMgr.Config == nil || (!profile.Running && profile.Pid <= 0 && profile.DebugPort <= 0) {
		return nil
	}
	userDataDir := a.browserMgr.ResolveUserDataDir(profile)
	processes, err := findBrowserUserDataProcesses(userDataDir)
	if err != nil || len(processes) == 0 {
		return nil
	}
	if profile.Pid > 0 {
		for _, process := range processes {
			if process.PID == profile.Pid {
				return parseBrowserProcessCommandLineArgs(process.CommandLine)
			}
		}
	}
	if profile.DebugPort > 0 {
		for _, process := range processes {
			debugPort := process.DebugPort
			if debugPort <= 0 {
				debugPort = parseRemoteDebuggingPort(process.CommandLine)
			}
			if debugPort == profile.DebugPort {
				return parseBrowserProcessCommandLineArgs(process.CommandLine)
			}
		}
	}
	return parseBrowserProcessCommandLineArgs(processes[0].CommandLine)
}

func (a *App) fingerprintCheckProfileSnapshot(profileId string) (*BrowserProfile, error) {
	if a == nil || a.browserMgr == nil {
		return nil, fmt.Errorf("浏览器管理器未初始化")
	}
	a.browserMgr.Mutex.Lock()
	defer a.browserMgr.Mutex.Unlock()
	profile := a.browserMgr.Profiles[profileId]
	if profile == nil {
		return nil, fmt.Errorf("实例不存在: %s", profileId)
	}
	snapshot := *profile
	snapshot.FingerprintArgs = append([]string{}, profile.FingerprintArgs...)
	snapshot.LaunchArgs = append([]string{}, profile.LaunchArgs...)
	snapshot.LastLaunchArgs = append([]string{}, profile.LastLaunchArgs...)
	return &snapshot, nil
}

const fingerprintCheckHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>Ant 指纹检测</title>
  <style>
    :root { color-scheme: light; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", "Microsoft YaHei", "PingFang SC", "Hiragino Sans GB", "Noto Sans CJK SC", "Noto Sans SC", "WenQuanYi Micro Hei", sans-serif; }
    body { margin: 0; background: #f6f7f9; color: #111827; }
    main { max-width: 1480px; margin: 0 auto; padding: 24px; }
    header { display: flex; align-items: center; justify-content: space-between; gap: 16px; margin-bottom: 18px; }
    h1 { margin: 0; font-size: 22px; }
    .meta { color: #6b7280; font-size: 13px; margin-top: 4px; }
    .actions { display: flex; gap: 8px; flex-wrap: wrap; justify-content: flex-end; }
    button { border: 1px solid #111827; background: #111827; color: #fff; border-radius: 8px; height: 34px; padding: 0 12px; cursor: pointer; }
    button.secondary { background: #fff; color: #111827; }
    .grid { display: block; }
    section { background: #fff; border: 1px solid #e5e7eb; border-radius: 12px; overflow-x: auto; margin-bottom: 14px; }
    h2 { margin: 0; padding: 11px 13px; font-size: 14px; border-bottom: 1px solid #e5e7eb; background: #fafafa; }
    table { width: 100%; min-width: 1280px; border-collapse: collapse; }
    th, td { padding: 9px 12px; border-bottom: 1px solid #f1f5f9; vertical-align: top; font-size: 13px; text-align: left; }
    th { background: #f8fafc; color: #475569; font-weight: 600; white-space: nowrap; }
    tr:last-child td { border-bottom: 0; }
    td.item { width: 160px; color: #111827; font-weight: 600; white-space: nowrap; }
    td.source { width: 108px; color: #334155; font-weight: 600; white-space: nowrap; }
    td.hit { width: 92px; font-weight: 700; white-space: nowrap; }
    td.reason { width: 380px; color: #475569; }
    code { font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, "Microsoft YaHei", "PingFang SC", "Noto Sans CJK SC", "Noto Sans SC", "WenQuanYi Micro Hei", monospace; word-break: break-all; }
    .value-pair { display: grid; gap: 5px; }
    .value-line { display: grid; grid-template-columns: 42px minmax(0, 1fr); gap: 8px; align-items: start; }
    .value-label { color: #64748b; }
    .ok { color: #047857; }
    .warn { color: #b45309; }
    .bad { color: #b91c1c; }
    .muted { color: #64748b; }
    .summary { display: none; }
    .flow { margin-bottom: 14px; display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 8px; }
    .flow-step { background: #fff; border: 1px solid #e5e7eb; border-radius: 10px; padding: 10px 12px; font-size: 13px; color: #334155; }
    .flow-step strong { display: block; color: #111827; margin-bottom: 3px; }
    .diff-empty { background: #fff; border: 1px solid #e5e7eb; border-radius: 12px; padding: 12px; color: #475569; font-size: 13px; margin-bottom: 14px; }
    .proxy-panel { overflow: hidden; }
    .proxy-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0; }
    .proxy-card { padding: 12px 13px; border-right: 1px solid #e5e7eb; }
    .proxy-card:last-child { border-right: 0; }
    .proxy-title { font-size: 13px; font-weight: 700; color: #111827; margin-bottom: 8px; }
    .proxy-lines { display: grid; gap: 6px; }
    .proxy-line { display: grid; grid-template-columns: 70px minmax(0, 1fr); gap: 8px; font-size: 13px; }
    .proxy-label { color: #64748b; white-space: nowrap; }
    .proxy-value { color: #111827; min-width: 0; word-break: break-all; }
    .proxy-status { display: inline-flex; align-items: center; height: 22px; padding: 0 8px; border-radius: 999px; font-size: 12px; font-weight: 700; }
    .proxy-status.ok { color: #047857; background: #ecfdf5; }
    .proxy-status.warn { color: #b45309; background: #fffbeb; }
    .proxy-status.bad { color: #b91c1c; background: #fef2f2; }
    .proxy-source-list { line-height: 1.55; }
    pre { margin: 0; padding: 12px; white-space: pre-wrap; word-break: break-all; font-size: 12px; }
  </style>
</head>
<body>
<main>
  <header>
    <div>
      <h1 id="pageTitle">Ant 指纹检测</h1>
      <div class="meta" id="meta">正在检测当前浏览器真实指纹...</div>
    </div>
    <div class="actions">
      <button class="secondary" id="resetBaselineBtn">重建基线</button>
      <button class="secondary" id="saveBeforeBtn">保存修改前快照</button>
      <button class="secondary" id="clearBeforeBtn">清除修改前快照</button>
      <button class="secondary" id="refreshBtn">重新检测</button>
      <button id="copyBtn">复制 JSON</button>
    </div>
  </header>
  <div class="flow" id="flowSteps">
    <div class="flow-step"><strong>1 保存修改前</strong>旧配置启动后检测，点保存修改前快照。</div>
    <div class="flow-step"><strong>2 修改并重启</strong>改 Seed 或指纹配置后，关闭实例再启动。</div>
    <div class="flow-step"><strong>3 重新检测</strong>进入本页点重新检测，看修改前后变化。</div>
  </div>
  <div class="summary" id="summary"></div>
  <div id="changeApp"></div>
  <div id="proxyApp"></div>
  <div class="grid" id="app"></div>
</main>
<script>
var latestReport = null;
var latestContext = __FINGERPRINT_CHECK_CONTEXT__;
var fingerprintCheckRunning = false;
var latestBeforeSnapshot = null;
var FINGERPRINT_AUTO_REFRESH_MS = 60 * 60 * 1000;
var FINGERPRINT_AUTO_REFRESH_CHECK_MS = 60 * 1000;
var currentUILang = latestContext && latestContext.uiLanguage === 'en' ? 'en' : 'zh';
var UI_ZH = {
  'Ant Fingerprint Check': 'Ant 指纹检测',
  'Checking current browser fingerprint...': '正在检测当前浏览器真实指纹...',
  'Reset Baseline': '重建基线',
  'Save Before': '保存修改前快照',
  'Clear Before': '清除修改前快照',
  'Refresh': '重新检测',
  'Copy JSON': '复制 JSON',
  'Expected': '期望',
  'Actual': '实际',
  'Item': '指纹项',
  'Value': '值',
  'Source': '来源',
  'Expected Source': '期望来源',
  'Result': '结果',
  'Reason': '原因',
  'Time': '时间',
  'Fingerprint Check': '指纹对比',
  'Before/After Changes': '修改前后变化',
  'No before snapshot saved. To compare changes after Seed/config edits: run the old config, click Save Before, edit config and restart the instance, then click Refresh.': '未保存修改前快照。要看改 Seed 后哪些项变化：先在旧配置下点“保存修改前快照”，改配置并重启实例后再点“重新检测”。',
  'Effect Check': '效果观测',
  'Effect Baseline Set': '已建观测基线',
  'Effect Stable': '观测一致',
  'Effect Changed': '观测变化',
  'Runtime Baseline': '运行基线',
  'Baseline Match': '基线一致',
  'Baseline Changed': '基线变化',
  'Match': '命中',
  'Compatible': '口径匹配',
  'Mismatch': '未命中',
  'Risk': '风险',
  'Configured': '已配置',
  'Unsupported': '不可配置',
  'Baseline Set': '已建基线',
  'Not Configured': '未配置',
  'Not Collected': '未采集',
  'Config Expected': '配置期望',
  'Config Sent': '配置下发',
  'Built-in Expected': '内置期望',
  'Known Ineffective': '实测无效',
  'Before/After': '修改前后',
  'Changed': '已变化',
  'Unchanged': '未变化',
  'No Runtime Baseline': '未建立运行基线',
  'No Effect Baseline': '未建立观测基线',
  'JS unreadable': 'JS 不可读取',
  'Not used as expected': '不作为期望',
  'Fingerprint Seed': '指纹 Seed',
  'Native Exposure Disabled': '禁用原生暴露',
  'Language': '语言',
  'Languages': '语言列表',
  'Timezone': '时区',
  'CPU Cores': 'CPU 核心',
  'Device Memory': '设备内存',
  'Touch Points': '触控点',
  'Window Size': '窗口大小',
  'Color Depth': '颜色深度',
  'Brand': '品牌',
  'Brand Version': '品牌版本',
  'Platform': '平台',
  'Platform Version': '平台版本',
  'WebRTC Host': 'WebRTC Host',
  'Media Device Count': '媒体设备数量',
  'Canvas Noise': 'Canvas 噪声',
  'Audio Noise': 'Audio 噪声',
  'ClientRects Noise': 'ClientRects 噪声',
  'Screen Size': '屏幕尺寸',
  'Screen Height': '屏幕高度',
  'Config Seed': '配置 Seed',
  'Proxy Check': '代理检测',
  'Configured Proxy': '配置代理',
  'Browser Exit': '浏览器出口',
  'Name': '名称',
  'Type': '类型',
  'Endpoint': '地址',
  'Auth': '认证',
  'Group': '分组',
  'Current IP': '当前 IP',
  'Location': '归属地',
  'Reference Location': '参考归属',
  'Confidence': '置信度',
  'ASN/Network': 'ASN/网络',
  'Source Details': '来源明细',
  'ISP/Org': '运营商/组织',
  'Latency': '耗时',
  'Status': '状态',
  'Direct': '直连',
  'Yes': '有',
  'No': '无',
  'Detected': '已检测',
  'Majority Match': '多数一致',
  'Country Majority': '国家/地区多数一致',
  'Location Conflict': '归属地冲突',
  'Single Source': '单源',
  'Location conflict': '归属地存在冲突',
  'No location consensus': '无一致归属',
  'No source detail': '无来源明细',
  'No ASN': '无 ASN',
  'Datacenter': '机房',
  'Proxy/VPN': '代理/VPN',
  'IPXO/Rented Segment': 'IPXO/租赁段',
  'Detection Failed': '检测失败',
  'Not Checked': '未检测',
  'No proxy configured': '未配置代理',
  'No endpoint': '无地址',
  'No location': '无归属地',
  'Unknown': '未知',
  'Chain Proxy': '链式代理',
  'Structured Proxy Config': '结构化代理配置',
  'Custom Proxy Config': '自定义代理配置',
  'Public IP service unavailable': '公网 IP 服务不可用',
  'Checked: Local ': '检测时间：本地 ',
  'Before ': '修改前 ',
  ' / Current ': ' / 当前 ',
  'Seed is a launch parameter and cannot be read back from page JS': 'Seed 是启动参数，页面无法从 JS 反读',
  'This is a launch protection policy and cannot be read back from page JS': '该项是启动保护策略，页面无法从 JS 反读',
  'Compare navigator.language': '比对 navigator.language',
  'Compare navigator.languages prefix': '比对 navigator.languages 前缀',
  'Compare Intl.DateTimeFormat().resolvedOptions().timeZone': '比对 Intl.DateTimeFormat().resolvedOptions().timeZone',
  'Compare navigator.hardwareConcurrency': '比对 navigator.hardwareConcurrency',
  'Compare navigator.deviceMemory': '比对 navigator.deviceMemory',
  'Compare navigator.maxTouchPoints': '比对 navigator.maxTouchPoints',
  'Compare navigator.doNotTrack': '比对 navigator.doNotTrack',
  'Compare window.outerWidth/outerHeight': '比对 window.outerWidth/outerHeight',
  'Compare screen.colorDepth': '比对 screen.colorDepth',
  'Compare whether User-Agent contains expected brand': '比对 User-Agent 是否包含期望品牌',
  'Prefer full browser version comparison; if User-Agent exposes only major version, compare by major version': '优先比对完整浏览器版本；User-Agent 只暴露主版本时按主版本口径匹配',
  'Compare navigator.platform': '比对 navigator.platform',
  'Prefer full platform version comparison; if User-Agent / UA-CH exposes short version only, compare visible prefix': '优先比对完整系统版本；User-Agent / UA-CH 只暴露短版本时按可见版本前缀匹配',
  'Expected navigator.webdriver not to expose automation': '期望 navigator.webdriver 不暴露自动化',
  'Compare whether local host candidate is exposed': '比对是否暴露本机 host candidate',
  'No WebRTC expected value configured; showing actual collected value only': '未配置 WebRTC 期望，只展示实际采集值',
  'Standalone media device count parameter is ineffective in local Chrom-144 test and is not passed as runtime parameter': '媒体设备数量独立参数本地 Chrom-144 实测无效，未作为运行参数传递',
  'Canvas noise flag is passed as a launch parameter; page JS cannot read the flag directly. Check Canvas Hash stability for effect.': 'Canvas 噪声开关已作为启动参数下发；页面不能直接反读开关，效果看 Canvas Hash 是否稳定变化',
  'Standalone audio noise parameter is ineffective in local Chrom-144 test; observe audio changes through Seed and Audio Hash': 'Audio 独立噪声参数本地 Chrom-144 实测无效；音频变化通过 Seed 和 Audio Hash 观察',
  'ClientRects noise flag is passed as a launch parameter; page JS cannot read the flag directly. Check ClientRects Hash stability for effect.': 'ClientRects 噪声开关已作为启动参数下发；页面不能直接反读开关，效果看 ClientRects Hash 是否稳定变化',
  'Canvas Hash is an effect observation for Canvas noise, not expected config': 'Canvas Hash 是 Canvas 噪声效果观测值，不是配置期望',
  'Canvas Hash is detected output hash, not expected config': 'Canvas Hash 是检测输出哈希，不是配置期望',
  'Audio Hash is detected output hash, not expected config': 'Audio Hash 是检测输出哈希，不是配置期望',
  'ClientRects Hash is an effect observation for ClientRects noise, not expected config': 'ClientRects Hash 是 ClientRects 噪声效果观测值，不是配置期望',
  'ClientRects Hash is detected output hash, not expected config': 'ClientRects Hash 是检测输出哈希，不是配置期望',
  'Fonts Hash is detected output hash, not expected config': 'Fonts Hash 是检测输出哈希，不是配置期望',
  'Compare whether detected fonts include configured list': '比对检测到的字体是否包含配置列表',
  'Compare WebGL Vendor': '比对 WebGL Vendor',
  'Compare WebGL Renderer': '比对 WebGL Renderer',
  'WebGL Hash is detected output hash, not expected config': 'WebGL Hash 是检测输出哈希，不是配置期望',
  'Compare plugin list': '比对插件列表',
  'Compare MIME list': '比对 MIME 列表',
  'Compare screen size': '比对屏幕尺寸',
  'Compare DPR': '比对 DPR',
  'No explicit config and no actual value collected; runtime baseline cannot be created': '未显式配置，且本次没有可用实际值，无法建立运行基线',
  'Actual value matches expected value': '实际值与期望值一致',
  'Actual value does not match expected value': '实际值与期望值不一致',
  'Browser JS cannot read this launch config; only expected value is shown': '浏览器 JS 无法读取该启动配置，只展示期望值',
  'Local core test shows this standalone parameter is ineffective, so it is not used as expected config': '当前内核本地实测该独立参数无效，未作为可配置期望',
  'First collected value saved as runtime baseline': '首次采集并保存为运行基线',
  ' is an effect observation, not expected config': ' 是效果观测值，不是配置期望',
  '. Saved first actual value as effect baseline. After changing fingerprint config or Seed, reset baseline and refresh.': '；已保存首次实际值为观测基线。改过指纹配置或 Seed 后，先重建基线再刷新验证稳定性',
  '. Current value differs from effect baseline. This means output changed, not config failure. If config was not changed and it still changes after reset, then it is a stability issue.': '；当前实际值与观测基线不同，说明输出已变化，不等于配置失败。若未改配置且重建基线后仍变化，才是稳定性问题',
  '. Current value matches effect baseline.': '；当前实际值与观测基线一致',
  '. No actual value is available, so effect baseline cannot be created.': '；本次没有可用实际值，无法建立观测基线',
  ' has no explicit config value. The first actual value was saved as runtime baseline for later comparison.': ' 没有显式配置值；检测页已用首次实际采集值建立运行基线，后续按基线比对',
  ' has no explicit expected config. Current value differs from runtime baseline; this is an observed change, not config failure.': ' 没有显式配置期望；当前实际值与运行基线不同，这是观测值变化，不代表配置未生效',
  ' has no explicit expected config. Current value matches runtime baseline.': ' 没有显式配置期望；当前实际值与运行基线一致',
  ' has no explicit expected config and no actual value is available, so runtime baseline cannot be created.': ' 没有显式配置期望，且本次没有可用实际值，无法建立运行基线'
};
var UI_ZH_KEYS = Object.keys(UI_ZH).sort(function (left, right) { return right.length - left.length; });
function uiText(text) {
  var value = String(text || '');
  if (currentUILang !== 'zh') return value;
  if (Object.prototype.hasOwnProperty.call(UI_ZH, value)) return UI_ZH[value];
  var output = value;
  UI_ZH_KEYS.forEach(function (key) {
    if (key && output.indexOf(key) >= 0) output = output.split(key).join(UI_ZH[key]);
  });
  return output;
}
function uiValue(value) {
  var text = displayValue(value);
  if (currentUILang === 'zh' && Object.prototype.hasOwnProperty.call(UI_ZH, text)) return UI_ZH[text];
  return text;
}
function platformUsesEnglish(value) {
  var normalized = String(value || '').toLowerCase();
  return normalized.indexOf('linux') >= 0 || normalized.indexOf('x11') >= 0 || normalized.indexOf('mac') >= 0 || normalized.indexOf('darwin') >= 0;
}
function resolveUILanguage(context, report) {
  var expected = context && context.expected ? context.expected : {};
  var identity = report && report.identity ? report.identity : {};
  var uaData = identity.userAgentData || {};
  var candidates = [expected.platform, expected.platformVersion, identity.platform, identity.userAgent, uaData.platform, navigator.platform || '', navigator.userAgent || ''];
  for (var index = 0; index < candidates.length; index++) {
    if (platformUsesEnglish(candidates[index])) return 'en';
  }
  return context && context.uiLanguage === 'en' ? 'en' : 'zh';
}
function setNodeText(id, text) {
  var node = document.getElementById(id);
  if (node) node.textContent = uiText(text);
}
function applyStaticText() {
  document.documentElement.lang = currentUILang === 'en' ? 'en' : 'zh-CN';
  document.title = uiText('Ant Fingerprint Check');
  setNodeText('pageTitle', 'Ant Fingerprint Check');
  if (!latestReport) setNodeText('meta', 'Checking current browser fingerprint...');
  setNodeText('resetBaselineBtn', 'Reset Baseline');
  setNodeText('saveBeforeBtn', 'Save Before');
  setNodeText('clearBeforeBtn', 'Clear Before');
  setNodeText('refreshBtn', 'Refresh');
  setNodeText('copyBtn', 'Copy JSON');
  var flow = document.getElementById('flowSteps');
  if (flow) {
    if (currentUILang === 'en') {
      flow.innerHTML = '<div class="flow-step"><strong>1 Save Before</strong>Run the old config, then save a before snapshot.</div><div class="flow-step"><strong>2 Edit And Restart</strong>Change Seed or fingerprint config, then restart the instance.</div><div class="flow-step"><strong>3 Refresh</strong>Refresh this page and compare before/after changes.</div>';
    } else {
      flow.innerHTML = '<div class="flow-step"><strong>1 保存修改前</strong>旧配置启动后检测，点保存修改前快照。</div><div class="flow-step"><strong>2 修改并重启</strong>改 Seed 或指纹配置后，关闭实例再启动。</div><div class="flow-step"><strong>3 重新检测</strong>进入本页点重新检测，看修改前后变化。</div>';
    }
  }
}
function updateUILanguage(report) {
  currentUILang = resolveUILanguage(latestContext, report);
  applyStaticText();
}
function hashString(input) {
  var hash = 2166136261;
  var text = String(input || '');
  for (var i = 0; i < text.length; i++) {
    hash ^= text.charCodeAt(i);
    hash += (hash << 1) + (hash << 4) + (hash << 7) + (hash << 8) + (hash << 24);
  }
  return ('00000000' + (hash >>> 0).toString(16)).slice(-8);
}
function safe(fn, fallback) { try { return fn(); } catch (e) { return fallback; } }
function canvasHash() {
  return safe(function () {
    var canvas = document.createElement('canvas');
    canvas.width = 320; canvas.height = 96;
    var ctx = canvas.getContext('2d');
    ctx.textBaseline = 'top';
    ctx.font = '16px Arial';
    ctx.fillStyle = '#f60'; ctx.fillRect(4, 4, 150, 36);
    ctx.fillStyle = '#069'; ctx.fillText('Ant fingerprint check', 9, 12);
    ctx.strokeStyle = 'rgba(120,60,200,.85)'; ctx.beginPath(); ctx.arc(210, 44, 30, 0, Math.PI * 2); ctx.stroke();
    return hashString(canvas.toDataURL());
  }, '');
}
async function audioHash() {
  return await safe(async function () {
    var Ctor = window.OfflineAudioContext || window.webkitOfflineAudioContext;
    if (!Ctor) return '';
    var ctx = new Ctor(1, 44100, 44100);
    var osc = ctx.createOscillator();
    var comp = ctx.createDynamicsCompressor();
    osc.type = 'triangle'; osc.frequency.value = 10000;
    comp.threshold.value = -50; comp.knee.value = 40; comp.ratio.value = 12; comp.attack.value = 0; comp.release.value = .25;
    osc.connect(comp); comp.connect(ctx.destination); osc.start(0);
    var buffer = await ctx.startRendering();
    var data = buffer.getChannelData(0).slice(4500, 5000);
    return hashString(Array.prototype.map.call(data, function (v) { return v.toFixed(6); }).join(','));
  }, '');
}
function clientRectsHash() {
  return safe(function () {
    var node = document.createElement('div');
    node.style.cssText = 'position:absolute;left:-9999px;top:-9999px;width:180px;font:13px Arial;line-height:17px;';
    node.textContent = 'Ant fingerprint client rects check';
    document.body.appendChild(node);
    var rects = Array.prototype.map.call(node.getClientRects(), function (r) {
      return [r.x, r.y, r.width, r.height].map(function (v) { return Number(v).toFixed(3); }).join(':');
    }).join('|');
    document.body.removeChild(node);
    return hashString(rects);
  }, '');
}
function fontProbe() {
  return safe(function () {
    var baseFonts = ['monospace', 'sans-serif', 'serif'];
    var candidates = ['Arial', 'Calibri', 'Cambria', 'Consolas', 'Courier New', 'Georgia', 'Helvetica', 'Microsoft YaHei', 'PingFang SC', 'Roboto', 'Segoe UI', 'Times New Roman'];
    var text = 'mmmmmmmmmmlli';
    var size = '72px';
    var canvas = document.createElement('canvas');
    var ctx = canvas.getContext('2d');
    var base = {};
    baseFonts.forEach(function (font) { ctx.font = size + ' ' + font; base[font] = ctx.measureText(text).width; });
    var detected = candidates.filter(function (font) {
      return baseFonts.some(function (baseFont) {
        ctx.font = size + ' "' + font + '",' + baseFont;
        return ctx.measureText(text).width !== base[baseFont];
      });
    });
    return { detected: detected, hash: hashString(detected.join('|')) };
  }, { detected: [], hash: '' });
}
function webglInfo() {
  return safe(function () {
    var canvas = document.createElement('canvas');
    var gl = canvas.getContext('webgl') || canvas.getContext('experimental-webgl');
    if (!gl) return { vendor: '', renderer: '', hash: '' };
    var debug = gl.getExtension('WEBGL_debug_renderer_info');
    var vendor = debug ? gl.getParameter(debug.UNMASKED_VENDOR_WEBGL) : gl.getParameter(gl.VENDOR);
    var renderer = debug ? gl.getParameter(debug.UNMASKED_RENDERER_WEBGL) : gl.getParameter(gl.RENDERER);
    var params = [vendor, renderer, gl.getParameter(gl.VERSION), gl.getParameter(gl.SHADING_LANGUAGE_VERSION)].join('|');
    return { vendor: vendor || '', renderer: renderer || '', hash: hashString(params) };
  }, { vendor: '', renderer: '', hash: '' });
}
async function webrtcCandidates() {
  return await safe(function () {
    return new Promise(function (resolve) {
      if (!window.RTCPeerConnection) return resolve([]);
      var pc = new RTCPeerConnection({ iceServers: [] });
      var candidates = [];
      pc.createDataChannel('ant');
      pc.onicecandidate = function (event) {
        if (event && event.candidate && event.candidate.candidate) candidates.push(event.candidate.candidate);
      };
      pc.createOffer().then(function (offer) { return pc.setLocalDescription(offer); }).catch(function () {});
      setTimeout(function () { try { pc.close(); } catch (e) {} resolve(candidates); }, 1600);
    });
  }, []);
}
var COUNTRY_NAMES = {
  AD: 'Andorra', AE: 'United Arab Emirates', AF: 'Afghanistan', AG: 'Antigua and Barbuda', AI: 'Anguilla', AL: 'Albania', AM: 'Armenia', AO: 'Angola', AR: 'Argentina', AT: 'Austria', AU: 'Australia', AW: 'Aruba', AZ: 'Azerbaijan', BA: 'Bosnia and Herzegovina', BB: 'Barbados', BD: 'Bangladesh', BE: 'Belgium', BF: 'Burkina Faso', BG: 'Bulgaria', BH: 'Bahrain', BI: 'Burundi', BJ: 'Benin', BM: 'Bermuda', BN: 'Brunei', BO: 'Bolivia', BR: 'Brazil', BS: 'Bahamas', BT: 'Bhutan', BW: 'Botswana', BY: 'Belarus', BZ: 'Belize', CA: 'Canada', CD: 'Democratic Republic of the Congo', CF: 'Central African Republic', CG: 'Republic of the Congo', CH: 'Switzerland', CI: 'Ivory Coast', CL: 'Chile', CM: 'Cameroon', CN: 'China', CO: 'Colombia', CR: 'Costa Rica', CU: 'Cuba', CV: 'Cape Verde', CY: 'Cyprus', CZ: 'Czechia', DE: 'Germany', DJ: 'Djibouti', DK: 'Denmark', DM: 'Dominica', DO: 'Dominican Republic', DZ: 'Algeria', EC: 'Ecuador', EE: 'Estonia', EG: 'Egypt', ES: 'Spain', ET: 'Ethiopia', EU: 'EU', FI: 'Finland', FJ: 'Fiji', FR: 'France', GA: 'Gabon', GB: 'United Kingdom', GD: 'Grenada', GE: 'Georgia', GH: 'Ghana', GM: 'Gambia', GN: 'Guinea', GQ: 'Equatorial Guinea', GR: 'Greece', GT: 'Guatemala', HK: 'Hong Kong', HN: 'Honduras', HR: 'Croatia', HT: 'Haiti', HU: 'Hungary', ID: 'Indonesia', IE: 'Ireland', IL: 'Israel', IN: 'India', IQ: 'Iraq', IR: 'Iran', IS: 'Iceland', IT: 'Italy', JM: 'Jamaica', JO: 'Jordan', JP: 'Japan', KE: 'Kenya', KG: 'Kyrgyzstan', KH: 'Cambodia', KR: 'South Korea', KW: 'Kuwait', KZ: 'Kazakhstan', LA: 'Laos', LB: 'Lebanon', LK: 'Sri Lanka', LR: 'Liberia', LT: 'Lithuania', LU: 'Luxembourg', LV: 'Latvia', LY: 'Libya', MA: 'Morocco', MD: 'Moldova', ME: 'Montenegro', MG: 'Madagascar', MK: 'North Macedonia', ML: 'Mali', MM: 'Myanmar', MN: 'Mongolia', MO: 'Macau', MR: 'Mauritania', MT: 'Malta', MU: 'Mauritius', MV: 'Maldives', MW: 'Malawi', MX: 'Mexico', MY: 'Malaysia', MZ: 'Mozambique', NA: 'Namibia', NE: 'Niger', NG: 'Nigeria', NI: 'Nicaragua', NL: 'Netherlands', NO: 'Norway', NP: 'Nepal', NZ: 'New Zealand', OM: 'Oman', PA: 'Panama', PE: 'Peru', PH: 'Philippines', PK: 'Pakistan', PL: 'Poland', PR: 'Puerto Rico', PT: 'Portugal', PY: 'Paraguay', QA: 'Qatar', RO: 'Romania', RS: 'Serbia', RU: 'Russia', RW: 'Rwanda', SA: 'Saudi Arabia', SE: 'Sweden', SG: 'Singapore', SI: 'Slovenia', SK: 'Slovakia', SN: 'Senegal', SO: 'Somalia', SV: 'El Salvador', SY: 'Syria', TH: 'Thailand', TJ: 'Tajikistan', TM: 'Turkmenistan', TN: 'Tunisia', TR: 'Turkey', TT: 'Trinidad and Tobago', TW: 'Taiwan', TZ: 'Tanzania', UA: 'Ukraine', UG: 'Uganda', US: 'United States', UY: 'Uruguay', UZ: 'Uzbekistan', VE: 'Venezuela', VN: 'Vietnam', YE: 'Yemen', ZA: 'South Africa', ZM: 'Zambia', ZW: 'Zimbabwe'
};
var COUNTRY_NAME_TO_CODE = {
  'belgium': 'BE', 'china': 'CN', 'hong kong': 'HK', 'hong kong sar': 'HK', 'macau': 'MO', 'macao': 'MO', 'taiwan': 'TW', 'united states': 'US', 'united states of america': 'US', 'usa': 'US', 'us': 'US', 'eu': 'EU', 'european union': 'EU'
};
var SOURCE_LOCATION_PRIORITY = { 'ipinfo.io': 90, 'ipapi.is': 75, 'ipwho.is': 60 };
function cleanString(value) {
  return String(value || '').trim();
}
function normalizeCountryCode(country, code) {
  var rawCode = cleanString(code || '').toUpperCase();
  if (/^[A-Z]{2}$/.test(rawCode)) return rawCode;
  var text = cleanString(country || '').toLowerCase();
  return COUNTRY_NAME_TO_CODE[text] || '';
}
function countryNameFor(code, fallback) {
  var normalized = cleanString(code).toUpperCase();
  return COUNTRY_NAMES[normalized] || cleanString(fallback);
}
function normalizeRegionName(value) {
  var text = cleanString(value);
  if (/^HK-/.test(text.toUpperCase())) return 'Hong Kong';
  return text;
}
function normalizePublicIPPayload(source, data, targetIP) {
  data = data || {};
  if (source === 'ipwho.is' && data.success === false) throw new Error(data.message || 'ipwho.is failed');
  if (source === 'ipinfo.io' && data.bogon === true) throw new Error('bogon ip');
  var connection = data.connection || {};
  var location = data.location || {};
  var asnData = data.asn || {};
  var company = data.company || {};
  var datacenter = data.datacenter || {};
  var ip = cleanString(data.ip || data.query || targetIP || '');
  var country = '';
  var countryCode = '';
  var region = '';
  var city = '';
  var org = '';
  var asn = '';
  var network = '';
  var route = '';
  var netname = '';
  var kind = 'geo';
  var isDatacenter = false;
  var isProxy = false;
  var isVPN = false;
  if (source === 'ipinfo.io') {
    countryCode = normalizeCountryCode('', data.country);
    country = countryNameFor(countryCode, data.country);
    region = normalizeRegionName(data.region);
    city = cleanString(data.city);
    org = cleanString(data.org);
    asn = org.replace(/^AS(\d+).*$/i, '$1');
    if (asn === org) asn = '';
  } else if (source === 'ipapi.is') {
    country = cleanString(location.country);
    countryCode = normalizeCountryCode(country, location.country_code);
    country = countryNameFor(countryCode, country);
    region = normalizeRegionName(location.state || location.region);
    city = cleanString(location.city);
    org = cleanString(asnData.descr || company.name || datacenter.datacenter);
    asn = cleanString(asnData.asn);
    network = cleanString(company.network || datacenter.network);
    route = cleanString(asnData.route);
    netname = cleanString(company.netname);
    isDatacenter = data.is_datacenter === true || !!datacenter.datacenter;
    isProxy = data.is_proxy === true;
    isVPN = data.is_vpn === true;
  } else if (source === 'rdap.ripe') {
    kind = 'network';
    countryCode = normalizeCountryCode('', data.country);
    country = countryNameFor(countryCode, data.country);
    network = cleanString(data.handle || ([data.startAddress, data.endAddress].filter(Boolean).join(' - ')));
    route = data.cidr0_cidrs && data.cidr0_cidrs.length ? String(data.cidr0_cidrs[0].v4prefix || data.cidr0_cidrs[0].v6prefix || '') + '/' + String(data.cidr0_cidrs[0].length || '') : '';
    netname = cleanString(data.name);
    org = cleanString(data.name);
  } else if (source === 'ipify') {
    kind = 'ip';
  } else {
    country = cleanString(data.country || data.country_name);
    countryCode = normalizeCountryCode(country, data.country_code || data.countryCode);
    country = countryNameFor(countryCode, country);
    region = normalizeRegionName(data.region || data.regionName || data.region_name);
    city = cleanString(data.city);
    org = cleanString(connection.org || connection.isp || data.org || data.isp || data.asn_org);
    asn = cleanString(connection.asn || data.asn);
  }
  var rentedOrIpXO = /ipxo/i.test([netname, network, org, route].join(' '));
  return {
    ok: !!ip,
    ip: ip,
    country: country,
    countryCode: countryCode,
    region: region,
    city: city,
    org: org,
    asn: asn,
    network: network,
    route: route,
    netname: netname,
    datacenter: isDatacenter,
    proxy: isProxy,
    vpn: isVPN,
    rentedOrIpXO: rentedOrIpXO,
    kind: kind,
    source: source
  };
}
function fetchJSONWithTimeout(url, timeoutMs) {
  return new Promise(function (resolve, reject) {
    var controller = window.AbortController ? new AbortController() : null;
    var timer = setTimeout(function () {
      if (controller) controller.abort();
      reject(new Error('timeout'));
    }, timeoutMs);
    fetch(url, {
      cache: 'no-store',
      credentials: 'omit',
      signal: controller ? controller.signal : undefined,
      headers: { 'Accept': 'application/json' }
    }).then(function (response) {
      if (!response.ok) throw new Error('HTTP ' + response.status);
      return response.json();
    }).then(function (data) {
      clearTimeout(timer);
      resolve(data);
    }).catch(function (error) {
      clearTimeout(timer);
      reject(error);
    });
  });
}
async function collectPublicIPInfo() {
  var currentEndpoints = [
    { source: 'ipwho.is', url: 'https://ipwho.is/' },
    { source: 'ipinfo.io', url: 'https://ipinfo.io/json' },
    { source: 'ipapi.is', url: 'https://api.ipapi.is/' },
    { source: 'ipify', url: 'https://api.ipify.org?format=json' }
  ];
  var currentResults = await Promise.all(currentEndpoints.map(function (endpoint) {
    return fetchPublicIPSource(endpoint);
  }));
  var targetIP = pickConsensusIP(currentResults);
  if (!targetIP) return buildPublicIPFailure(currentResults);
  var detailEndpoints = [
    { source: 'ipwho.is', url: 'https://ipwho.is/' + encodeURIComponent(targetIP), targetIP: targetIP },
    { source: 'ipinfo.io', url: 'https://ipinfo.io/' + encodeURIComponent(targetIP) + '/json', targetIP: targetIP },
    { source: 'ipapi.is', url: 'https://api.ipapi.is/?q=' + encodeURIComponent(targetIP), targetIP: targetIP },
    { source: 'rdap.ripe', url: 'https://rdap.db.ripe.net/ip/' + encodeURIComponent(targetIP), targetIP: targetIP }
  ];
  var detailResults = await Promise.all(detailEndpoints.map(function (endpoint) {
    return fetchPublicIPSource(endpoint);
  }));
  return resolveIPAttribution(targetIP, mergeIPAttributionResults(detailResults, currentResults, targetIP));
}
async function fetchPublicIPSource(endpoint) {
  var startedAt = Date.now();
  try {
    var payload = await fetchJSONWithTimeout(endpoint.url, endpoint.timeoutMs || (endpoint.source === 'rdap.ripe' ? 4500 : 6500));
    var info = normalizePublicIPPayload(endpoint.source, payload, endpoint.targetIP || '');
    info.latencyMs = Date.now() - startedAt;
    if (!info.ok) throw new Error('empty ip');
    return info;
  } catch (error) {
    return { ok: false, source: endpoint.source, ip: endpoint.targetIP || '', kind: endpoint.source === 'rdap.ripe' ? 'network' : 'geo', latencyMs: Date.now() - startedAt, error: error && error.message ? error.message : String(error || 'failed') };
  }
}
function mergeIPAttributionResults(detailResults, currentResults, targetIP) {
  var bySource = {};
  var merged = (detailResults || []).slice();
  merged.forEach(function (item) { if (item && item.source) bySource[item.source] = item; });
  (currentResults || []).forEach(function (item) {
    if (!item || !item.source) return;
    if (item.source === 'ipify') {
      merged.push(item);
      return;
    }
    if (!item.ok || item.ip !== targetIP) return;
    if (!bySource[item.source] || bySource[item.source].ok !== true) {
      if (bySource[item.source]) merged = merged.filter(function (existing) { return existing.source !== item.source; });
      merged.push(item);
      bySource[item.source] = item;
    }
  });
  return merged;
}
function pickConsensusIP(results) {
  var counts = {};
  var first = '';
  (results || []).forEach(function (item) {
    if (!item || !item.ok || !item.ip) return;
    var ip = item.ip;
    if (!first) first = ip;
    counts[ip] = (counts[ip] || 0) + 1;
  });
  return Object.keys(counts).sort(function (left, right) { return counts[right] - counts[left]; })[0] || first;
}
function buildPublicIPFailure(results) {
  var errors = (results || []).map(function (item) {
    return item && item.source ? item.source + ': ' + (item.error || 'failed') : '';
  }).filter(Boolean).join('; ');
  return { ok: false, source: '', ip: '', country: '', countryCode: '', region: '', city: '', org: '', asn: '', network: '', route: '', locationStatus: 'failed', confidence: 'Detection Failed', latencyMs: 0, sources: results || [], error: errors || 'Public IP service unavailable' };
}
function sourceLocationKey(item, mode) {
  if (!item || !item.countryCode && !item.country) return '';
  var country = normalizeCountryCode(item.country, item.countryCode) || cleanString(item.country).toLowerCase();
  if (mode === 'country') return country;
  return [country, cleanString(item.region).toLowerCase(), cleanString(item.city).toLowerCase()].join('|');
}
function bestGroup(groups) {
  var best = null;
  Object.keys(groups).forEach(function (key) {
    var group = groups[key];
    if (!best || group.items.length > best.items.length || (group.items.length === best.items.length && group.priority > best.priority)) best = group;
  });
  return best;
}
function hasTopGroupTie(groups, topGroup) {
  if (!topGroup) return false;
  var topCount = topGroup.items.length;
  var tied = 0;
  Object.keys(groups).forEach(function (key) {
    if (groups[key].items.length === topCount) tied += 1;
  });
  return tied > 1;
}
function buildLocationGroups(sources, mode) {
  var groups = {};
  sources.forEach(function (item) {
    var key = sourceLocationKey(item, mode);
    if (!key) return;
    if (!groups[key]) groups[key] = { key: key, items: [], priority: 0, sample: item };
    groups[key].items.push(item);
    groups[key].priority = Math.max(groups[key].priority, SOURCE_LOCATION_PRIORITY[item.source] || 0);
    if ((SOURCE_LOCATION_PRIORITY[item.source] || 0) > (SOURCE_LOCATION_PRIORITY[groups[key].sample.source] || 0)) groups[key].sample = item;
  });
  return groups;
}
function chooseReferenceLocationSource(geoSources) {
  return geoSources.slice().sort(function (left, right) {
    return (SOURCE_LOCATION_PRIORITY[right.source] || 0) - (SOURCE_LOCATION_PRIORITY[left.source] || 0);
  })[0] || null;
}
function mergeNetworkAttribution(sources) {
  var sorted = sources.slice().sort(function (left, right) {
    var leftScore = left.source === 'ipapi.is' ? 100 : (left.source === 'rdap.ripe' ? 90 : 50);
    var rightScore = right.source === 'ipapi.is' ? 100 : (right.source === 'rdap.ripe' ? 90 : 50);
    return rightScore - leftScore;
  });
  var result = { org: '', asn: '', network: '', route: '', netname: '', datacenter: false, proxy: false, vpn: false, rentedOrIpXO: false };
  sorted.forEach(function (item) {
    if (!item || !item.ok) return;
    if (!result.org && item.org) result.org = item.org;
    if (!result.asn && item.asn) result.asn = item.asn;
    if (!result.network && item.network) result.network = item.network;
    if (!result.route && item.route) result.route = item.route;
    if (!result.netname && item.netname) result.netname = item.netname;
    result.datacenter = result.datacenter || item.datacenter === true;
    result.proxy = result.proxy || item.proxy === true;
    result.vpn = result.vpn || item.vpn === true;
    result.rentedOrIpXO = result.rentedOrIpXO || item.rentedOrIpXO === true;
  });
  return result;
}
function resolveIPAttribution(ip, results) {
  var successful = (results || []).filter(function (item) { return item && item.ok; });
  var geoSources = successful.filter(function (item) { return item.kind !== 'ip' && item.kind !== 'network' && (item.country || item.countryCode || item.region || item.city); });
  var exactGroups = buildLocationGroups(geoSources, 'exact');
  var countryGroups = buildLocationGroups(geoSources, 'country');
  var exactGroup = bestGroup(exactGroups);
  var countryGroup = bestGroup(countryGroups);
  var reference = chooseReferenceLocationSource(geoSources);
  var adopted = null;
  var status = 'failed';
  var confidence = 'Detection Failed';
  if (exactGroup && exactGroup.items.length >= 2 && !hasTopGroupTie(exactGroups, exactGroup)) {
    adopted = exactGroup.sample;
    status = 'detected';
    confidence = 'Majority Match';
  } else if (countryGroup && countryGroup.items.length >= 2 && !hasTopGroupTie(countryGroups, countryGroup)) {
    adopted = countryGroup.sample;
    status = 'detected';
    confidence = 'Country Majority';
  } else if (geoSources.length > 1) {
    adopted = reference;
    status = 'conflict';
    confidence = 'Location Conflict';
  } else if (geoSources.length === 1) {
    adopted = geoSources[0];
    status = 'single';
    confidence = 'Single Source';
  }
  var network = mergeNetworkAttribution(successful);
  var latency = successful.reduce(function (total, item) { return total + (item.latencyMs || 0); }, 0);
  if (!adopted) return buildPublicIPFailure(results);
  return {
    ok: true,
    ip: ip,
    country: adopted.country || '',
    countryCode: adopted.countryCode || '',
    region: adopted.region || '',
    city: adopted.city || '',
    org: network.org || adopted.org || '',
    asn: network.asn || adopted.asn || '',
    network: network.network || '',
    route: network.route || '',
    netname: network.netname || '',
    datacenter: network.datacenter,
    proxy: network.proxy,
    vpn: network.vpn,
    rentedOrIpXO: network.rentedOrIpXO,
    source: adopted.source || '',
    sources: results || [],
    locationStatus: status,
    confidence: confidence,
    referenceSource: reference ? reference.source : '',
    latencyMs: latency
  };
}
async function collect() {
  var uaData = navigator.userAgentData ? {
    brands: navigator.userAgentData.brands || [],
    mobile: navigator.userAgentData.mobile,
    platform: navigator.userAgentData.platform || ''
  } : null;
  var fonts = fontProbe();
  var gl = webglInfo();
  var networkResults = await Promise.all([webrtcCandidates(), collectPublicIPInfo()]);
  var candidates = networkResults[0];
  var proxyInfo = networkResults[1];
  return {
    generatedAt: new Date().toISOString(),
    urlProfileId: new URLSearchParams(location.search).get('profileId') || '',
    identity: {
      userAgent: navigator.userAgent || '',
      platform: navigator.platform || '',
      userAgentData: uaData,
      webdriver: navigator.webdriver === true
    },
    locale: {
      language: navigator.language || '',
      languages: Array.prototype.slice.call(navigator.languages || []),
      timezone: (Intl.DateTimeFormat().resolvedOptions() || {}).timeZone || '',
      timezoneOffset: new Date().getTimezoneOffset()
    },
    hardware: {
      hardwareConcurrency: navigator.hardwareConcurrency || 0,
      deviceMemory: navigator.deviceMemory || 0,
      maxTouchPoints: navigator.maxTouchPoints || 0,
      cookieEnabled: navigator.cookieEnabled === true,
      doNotTrack: navigator.doNotTrack || ''
    },
    screen: {
      width: screen.width || 0,
      height: screen.height || 0,
      availWidth: screen.availWidth || 0,
      availHeight: screen.availHeight || 0,
      colorDepth: screen.colorDepth || 0,
      pixelDepth: screen.pixelDepth || 0,
      devicePixelRatio: window.devicePixelRatio || 0,
      innerWidth: window.innerWidth || 0,
      innerHeight: window.innerHeight || 0,
      outerWidth: window.outerWidth || 0,
      outerHeight: window.outerHeight || 0
    },
    advanced: {
      canvasHash: canvasHash(),
      audioHash: await audioHash(),
      clientRectsHash: clientRectsHash(),
      fontHash: fonts.hash,
      detectedFonts: fonts.detected,
      webglVendor: gl.vendor,
      webglRenderer: gl.renderer,
      webglHash: gl.hash,
      plugins: Array.prototype.map.call(navigator.plugins || [], function (p) { return p.name || ''; }),
      mimeTypes: Array.prototype.map.call(navigator.mimeTypes || [], function (m) { return m.type || ''; })
    },
    network: {
      webrtcCandidates: candidates,
      localCandidateCount: candidates.filter(function (item) { return / typ host /.test(item); }).length,
      proxyInfo: proxyInfo
    }
  };
}
function csvItems(value) { return String(value || '').split(',').map(function (item) { return item.trim(); }).filter(Boolean); }
function matchExact(expected, actual) {
  if (expected === undefined || expected === null || expected === '') return 'unknown';
  return String(expected) === String(actual) ? 'match' : 'mismatch';
}
function matchArrayPrefix(expectedCsv, actualArray) {
  var expected = csvItems(expectedCsv);
  if (!expected.length) return 'unknown';
  var actual = (actualArray || []).map(function (item) { return String(item).trim(); });
  return expected.every(function (item, index) { return actual[index] === item; }) ? 'match' : 'mismatch';
}
function matchContains(expected, actual) {
  if (expected === undefined || expected === null || expected === '') return 'unknown';
  return String(actual || '').indexOf(String(expected)) >= 0 ? 'match' : 'mismatch';
}
function displayValue(value) {
  if (value === undefined || value === null || value === '') return '-';
  if (Array.isArray(value)) return value.length ? value.join(', ') : '-';
  if (typeof value === 'object') return JSON.stringify(value);
  return String(value);
}
function hasExpected(value) {
  return !(value === undefined || value === null || value === '');
}
function pairValue(expected, actual) {
  return '<div class="value-pair"><div class="value-line"><span class="value-label">' + uiText('Expected') + '</span><code>' + escapeHtml(uiValue(expected)) + '</code></div><div class="value-line"><span class="value-label">' + uiText('Actual') + '</span><code>' + escapeHtml(uiValue(actual)) + '</code></div></div>';
}
function normalizeDisplay(value) { return displayValue(value); }
function compareExactStatus(expected, actual) {
  if (!hasExpected(expected)) return 'unknown';
  return String(expected) === String(actual) ? 'match' : 'mismatch';
}
function compareArrayPrefixStatus(expectedCsv, actualArray) {
  var expected = csvItems(expectedCsv);
  if (!expected.length) return 'unknown';
  var actual = (actualArray || []).map(function (item) { return String(item).trim(); });
  return expected.every(function (item, index) { return actual[index] === item; }) ? 'match' : 'mismatch';
}
function compareArrayContainsAllStatus(expectedCsv, actualArray) {
  var expected = csvItems(expectedCsv);
  if (!expected.length) return 'unknown';
  var actual = (actualArray || []).map(function (item) { return String(item).trim().toLowerCase(); });
  return expected.every(function (item) { return actual.indexOf(String(item).trim().toLowerCase()) >= 0; }) ? 'match' : 'mismatch';
}
function compareContainsStatus(expected, actual) {
  if (!hasExpected(expected)) return 'unknown';
  return String(actual || '').indexOf(String(expected)) >= 0 ? 'match' : 'mismatch';
}
function versionParts(value) {
  var normalized = String(value || '').replace(/_/g, '.');
  var match = normalized.match(/\d+(?:\.\d+)*/);
  return match ? match[0].split('.').map(function (item) { return parseInt(item, 10); }).filter(function (item) { return !isNaN(item); }) : [];
}
function versionList(value, patterns) {
  var text = String(value || '').replace(/_/g, '.');
  var list = [];
  patterns.forEach(function (pattern) {
    var match;
    while ((match = pattern.exec(text)) !== null) {
      var parts = versionParts(match[1]);
      if (parts.length) list.push(parts);
    }
  });
  return list;
}
function sameVersionPrefix(left, right) {
  if (!left.length || !right.length) return false;
  var size = Math.min(left.length, right.length);
  for (var index = 0; index < size; index += 1) {
    if (left[index] !== right[index]) return false;
  }
  return true;
}
function compareBrowserVersionStatus(expected, actual) {
  if (!hasExpected(expected)) return 'unknown';
  if (compareContainsStatus(expected, actual) === 'match') return 'match';
  var expectedParts = versionParts(expected);
  if (!expectedParts.length) return 'mismatch';
  var browserVersions = versionList(actual, [/(?:Chrome|Chromium|Edg|OPR|Vivaldi)\/([0-9]+(?:[._][0-9]+)*)/g, /"version"\s*:\s*"([0-9]+(?:[._][0-9]+)*)"/g]);
  return browserVersions.some(function (actualParts) { return actualParts[0] === expectedParts[0]; }) ? 'compatible' : 'mismatch';
}
function comparePlatformVersionStatus(expected, actual) {
  if (!hasExpected(expected)) return 'unknown';
  if (compareContainsStatus(expected, actual) === 'match') return 'match';
  var expectedParts = versionParts(expected);
  if (!expectedParts.length) return 'mismatch';
  var platformVersions = versionList(actual, [/Windows NT\s+([0-9]+(?:[._][0-9]+)*)/g, /Mac OS X\s+([0-9]+(?:[._][0-9]+)*)/g, /Android\s+([0-9]+(?:[._][0-9]+)*)/g, /(?:CPU (?:iPhone )?OS|iPhone OS)\s+([0-9]+(?:[._][0-9]+)*)/g, /"platformVersion"\s*:\s*"([0-9]+(?:[._][0-9]+)*)"/g]);
  return platformVersions.some(function (actualParts) { return sameVersionPrefix(expectedParts, actualParts); }) ? 'compatible' : 'mismatch';
}
function normalizePlatformForCompare(value) {
  var normalized = String(value || '').trim().toLowerCase();
  if (!normalized) return '';
  if (['windows', 'win', 'win32', 'win64', 'wince'].indexOf(normalized) >= 0) return 'windows';
  if (normalized.indexOf('linux') === 0 || normalized === 'x11') return 'linux';
  if (normalized === 'mac' || normalized === 'macos' || normalized.indexOf('mac') >= 0) return 'macos';
  return normalized;
}
function comparePlatformStatus(expected, actual) {
  if (!hasExpected(expected)) return 'unknown';
  var expectedPlatform = normalizePlatformForCompare(expected);
  var actualPlatform = normalizePlatformForCompare(actual);
  return expectedPlatform && expectedPlatform === actualPlatform ? 'match' : 'mismatch';
}
function statusText(status, source) {
  if (source === 'Effect Check' && status === 'baseline') return 'Effect Baseline Set';
  if (source === 'Effect Check' && status === 'match') return 'Effect Stable';
  if (source === 'Effect Check' && status === 'mismatch') return 'Effect Changed';
  if (source === 'Runtime Baseline' && status === 'match') return 'Baseline Match';
  if (source === 'Runtime Baseline' && status === 'mismatch') return 'Baseline Changed';
  if (status === 'match') return 'Match';
  if (status === 'compatible') return 'Compatible';
  if (status === 'mismatch') return 'Mismatch';
  if (status === 'warning') return 'Risk';
  if (status === 'unreadable') return 'Configured';
  if (status === 'unsupported') return 'Unsupported';
  if (status === 'baseline') return 'Baseline Set';
  if (source === 'Not Configured') return 'Not Configured';
  return 'Not Collected';
}
function statusClass(status, source) {
  if (status === 'match') return 'ok';
  if (status === 'compatible') return 'ok';
  if (source === 'Effect Check' && status === 'mismatch') return 'warn';
  if (source === 'Runtime Baseline' && status === 'mismatch') return 'warn';
  if (status === 'mismatch') return 'bad';
  if (status === 'warning') return 'warn';
  return 'muted';
}
function reasonFor(status, reason) {
  if (reason) return reason;
  if (status === 'unknown') return 'No explicit config and no actual value collected; runtime baseline cannot be created';
  if (status === 'match') return 'Actual value matches expected value';
  if (status === 'mismatch') return 'Actual value does not match expected value';
  if (status === 'unreadable') return 'Browser JS cannot read this launch config; only expected value is shown';
  if (status === 'unsupported') return 'Local core test shows this standalone parameter is ineffective, so it is not used as expected config';
  if (status === 'baseline') return 'First collected value saved as runtime baseline';
  return '';
}
function sourceFor(status, expected, actual) {
  if (hasExpected(expected)) return 'Config Expected';
  if (status === 'baseline' || status === 'match' || status === 'mismatch') return 'Runtime Baseline';
  if (status === 'unreadable') return 'Config Sent';
  if (hasObservedValue(actual)) return 'Not Configured';
  return 'Not Collected';
}
function fingerprintRow(name, expected, actual, status, reason, source) {
  return {
    name: name,
    expected: expected,
    actual: actual,
    status: status,
    source: source || sourceFor(status, expected, actual),
    reason: reasonFor(status, reason)
  };
}
var latestBaseline = {};
var latestBaselineCreated = {};
function baselineStorageKey(context) {
  var expected = context && context.expected ? context.expected : {};
  var profileId = context && context.profileId ? context.profileId : 'unknown';
  var seed = expected.seed || 'no-seed';
  return 'ant:fingerprint-check:baseline:' + profileId + ':' + seed;
}
function loadFingerprintBaseline(context) {
  latestBaselineCreated = {};
  try {
    latestBaseline = JSON.parse(localStorage.getItem(baselineStorageKey(context)) || '{}') || {};
  } catch (e) {
    latestBaseline = {};
  }
}
function saveFingerprintBaseline(context) {
  try {
    localStorage.setItem(baselineStorageKey(context), JSON.stringify(latestBaseline));
  } catch (e) {}
}
function hasObservedValue(value) {
  if (value === undefined || value === null || value === '') return false;
  if (Array.isArray(value)) return value.length > 0;
  return true;
}
function cloneObservedValue(value) {
  if (Array.isArray(value)) return value.slice();
  if (value && typeof value === 'object') return JSON.parse(JSON.stringify(value));
  return value;
}
function sameObservedValue(left, right) {
  return JSON.stringify(left) === JSON.stringify(right);
}
function changeSnapshotStorageKey(context) {
  var profileId = context && context.profileId ? context.profileId : 'unknown';
  return 'ant:fingerprint-check:change-before:' + profileId;
}
function loadBeforeSnapshot(context) {
  try {
    latestBeforeSnapshot = JSON.parse(localStorage.getItem(changeSnapshotStorageKey(context)) || 'null');
  } catch (e) {
    latestBeforeSnapshot = null;
  }
}
function saveBeforeSnapshot(context, report) {
  if (!report) return;
  latestBeforeSnapshot = JSON.parse(JSON.stringify({
    context: context || {},
    report: report
  }));
  try {
    localStorage.setItem(changeSnapshotStorageKey(context), JSON.stringify(latestBeforeSnapshot));
  } catch (e) {}
}
function clearBeforeSnapshot(context) {
  latestBeforeSnapshot = null;
  try {
    localStorage.removeItem(changeSnapshotStorageKey(context));
  } catch (e) {}
}
function baselineValue(context, key, actual) {
  if (Object.prototype.hasOwnProperty.call(latestBaseline, key)) return latestBaseline[key];
  if (!hasObservedValue(actual)) return '';
  latestBaseline[key] = cloneObservedValue(actual);
  latestBaselineCreated[key] = true;
  saveFingerprintBaseline(context);
  return latestBaseline[key];
}
function observedExpected(context, key, explicitValue, actual, emptyExpectedText) {
  if (hasExpected(explicitValue)) return explicitValue;
  var value = baselineValue(context, key, actual);
  return hasObservedValue(value) ? value : (emptyExpectedText || 'No Runtime Baseline');
}
function observedStatus(context, key, explicitValue, actual, explicitCompare) {
  if (hasExpected(explicitValue)) return explicitCompare(explicitValue, actual);
  var value = baselineValue(context, key, actual);
  if (!hasObservedValue(value)) return 'unknown';
  if (latestBaselineCreated[key]) return 'baseline';
  return sameObservedValue(value, actual) ? 'match' : 'mismatch';
}
function observedReason(name, explicitValue, explicitReason, status, source) {
  if (hasExpected(explicitValue)) return explicitReason;
  if (source === 'Effect Check') {
    var prefix = explicitReason || (name + ' is an effect observation, not expected config');
    if (status === 'baseline') return prefix + '. Saved first actual value as effect baseline. After changing fingerprint config or Seed, reset baseline and refresh.';
    if (status === 'mismatch') return prefix + '. Current value differs from effect baseline. This means output changed, not config failure. If config was not changed and it still changes after reset, then it is a stability issue.';
    if (status === 'match') return prefix + '. Current value matches effect baseline.';
    return prefix + '. No actual value is available, so effect baseline cannot be created.';
  }
  if (status === 'baseline') return name + ' has no explicit config value. The first actual value was saved as runtime baseline for later comparison.';
  if (status === 'mismatch') return name + ' has no explicit expected config. Current value differs from runtime baseline; this is an observed change, not config failure.';
  if (status === 'match') return name + ' has no explicit expected config. Current value matches runtime baseline.';
  return name + ' has no explicit expected config and no actual value is available, so runtime baseline cannot be created.';
}
function observedSource(context, key, explicitValue, actual, observedSourceName) {
  if (hasExpected(explicitValue)) return 'Config Expected';
  var value = baselineValue(context, key, actual);
  return hasObservedValue(value) ? (observedSourceName || 'Runtime Baseline') : 'Not Collected';
}
function observedFingerprintRow(name, context, key, explicitValue, actual, explicitCompare, explicitReason, options) {
  options = options || {};
  var expectedValue = observedExpected(context, key, explicitValue, actual, options.emptyExpectedText);
  var status = observedStatus(context, key, explicitValue, actual, explicitCompare);
  var source = observedSource(context, key, explicitValue, actual, options.observedSourceName);
  var reason = observedReason(name, explicitValue, explicitReason, status, source);
  return fingerprintRow(name, expectedValue, actual, status, reason, source);
}
function effectObservedFingerprintRow(name, context, key, actual, reason) {
  return observedFingerprintRow(name, context, key, '', actual, compareExactStatus, reason, {
    observedSourceName: 'Effect Check',
    emptyExpectedText: 'No Effect Baseline'
  });
}
function resetFingerprintBaseline(context) {
  latestBaseline = {};
  latestBaselineCreated = {};
  try {
    localStorage.removeItem(baselineStorageKey(context));
  } catch (e) {}
}
function buildFingerprintRows(report, context) {
  loadFingerprintBaseline(context);
  var expected = context && context.expected ? context.expected : {};
  var uaVersion = report.identity.userAgent + (report.identity.userAgentData ? ' / ' + JSON.stringify(report.identity.userAgentData) : '');
  var rows = [];
  rows.push(fingerprintRow('Fingerprint Seed', expected.seed, 'JS unreadable', expected.seed ? 'unreadable' : 'unknown', 'Seed is a launch parameter and cannot be read back from page JS'));
  rows.push(fingerprintRow('Native Exposure Disabled', expected.disableSpoofing, 'JS unreadable', expected.disableSpoofing ? 'unreadable' : 'unknown', 'This is a launch protection policy and cannot be read back from page JS'));
  rows.push(observedFingerprintRow('Language', context, 'language', expected.language, report.locale.language, compareExactStatus, 'Compare navigator.language'));
  rows.push(observedFingerprintRow('Languages', context, 'languages', expected.acceptLanguage, report.locale.languages, compareArrayPrefixStatus, 'Compare navigator.languages prefix'));
  rows.push(observedFingerprintRow('Timezone', context, 'timezone', expected.timezone, report.locale.timezone, compareExactStatus, 'Compare Intl.DateTimeFormat().resolvedOptions().timeZone'));
  rows.push(observedFingerprintRow('CPU Cores', context, 'hardwareConcurrency', expected.hardwareConcurrency, report.hardware.hardwareConcurrency, compareExactStatus, 'Compare navigator.hardwareConcurrency'));
  rows.push(observedFingerprintRow('Device Memory', context, 'deviceMemory', expected.deviceMemory, report.hardware.deviceMemory, compareExactStatus, 'Compare navigator.deviceMemory'));
  rows.push(observedFingerprintRow('Touch Points', context, 'maxTouchPoints', expected.touchPoints, report.hardware.maxTouchPoints, compareExactStatus, 'Compare navigator.maxTouchPoints'));
  rows.push(observedFingerprintRow('Do Not Track', context, 'doNotTrack', expected.doNotTrack, report.hardware.doNotTrack, compareExactStatus, 'Compare navigator.doNotTrack'));
  rows.push(observedFingerprintRow('Window Size', context, 'windowSize', expected.windowSize, report.screen.outerWidth + ',' + report.screen.outerHeight, compareExactStatus, 'Compare window.outerWidth/outerHeight'));
  rows.push(observedFingerprintRow('Color Depth', context, 'colorDepth', expected.colorDepth, report.screen.colorDepth, compareExactStatus, 'Compare screen.colorDepth'));
  rows.push(fingerprintRow('Brand', expected.brand, report.identity.userAgent, compareContainsStatus(expected.brand, report.identity.userAgent), 'Compare whether User-Agent contains expected brand'));
  rows.push(observedFingerprintRow('Brand Version', context, 'brandVersion', expected.brandVersion, report.identity.userAgent, compareBrowserVersionStatus, 'Prefer full browser version comparison; if User-Agent exposes only major version, compare by major version'));
  rows.push(fingerprintRow('Platform', expected.platform, report.identity.platform, comparePlatformStatus(expected.platform, report.identity.platform), 'Compare navigator.platform'));
  rows.push(observedFingerprintRow('Platform Version', context, 'platformVersion', expected.platformVersion, uaVersion, comparePlatformVersionStatus, 'Prefer full platform version comparison; if User-Agent / UA-CH exposes short version only, compare visible prefix'));
  rows.push(fingerprintRow('Webdriver', 'false', report.identity.webdriver ? 'true' : 'false', report.identity.webdriver ? 'mismatch' : 'match', 'Expected navigator.webdriver not to expose automation', 'Built-in Expected'));
  var expectsWebRTCBlocked = hasExpected(expected.webrtcPolicy);
  rows.push(fingerprintRow('WebRTC Host', expectsWebRTCBlocked ? '0 host candidates' : '', report.network.localCandidateCount + ' host candidates', expectsWebRTCBlocked ? (report.network.localCandidateCount > 0 ? 'mismatch' : 'match') : 'unknown', expectsWebRTCBlocked ? 'Compare whether local host candidate is exposed' : 'No WebRTC expected value configured; showing actual collected value only'));
  rows.push(fingerprintRow('Media Device Count', 'Not used as expected', 'JS unreadable', 'unsupported', 'Standalone media device count parameter is ineffective in local Chrom-144 test and is not passed as runtime parameter', 'Known Ineffective'));
  rows.push(fingerprintRow('Canvas Noise', expected.canvasNoise, 'JS unreadable', expected.canvasNoise ? 'unreadable' : 'unknown', 'Canvas noise flag is passed as a launch parameter; page JS cannot read the flag directly. Check Canvas Hash stability for effect.'));
  rows.push(fingerprintRow('Audio Noise', 'Not used as expected', 'JS unreadable', 'unsupported', 'Standalone audio noise parameter is ineffective in local Chrom-144 test; observe audio changes through Seed and Audio Hash', 'Known Ineffective'));
  rows.push(fingerprintRow('ClientRects Noise', expected.clientRectsNoise, 'JS unreadable', expected.clientRectsNoise ? 'unreadable' : 'unknown', 'ClientRects noise flag is passed as a launch parameter; page JS cannot read the flag directly. Check ClientRects Hash stability for effect.'));
  rows.push(effectObservedFingerprintRow('Canvas Hash', context, 'canvasHash', report.advanced.canvasHash, expected.canvasNoise ? 'Canvas Hash is an effect observation for Canvas noise, not expected config' : 'Canvas Hash is detected output hash, not expected config'));
  rows.push(effectObservedFingerprintRow('Audio Hash', context, 'audioHash', report.advanced.audioHash, 'Audio Hash is detected output hash, not expected config'));
  rows.push(effectObservedFingerprintRow('ClientRects Hash', context, 'clientRectsHash', report.advanced.clientRectsHash, expected.clientRectsNoise ? 'ClientRects Hash is an effect observation for ClientRects noise, not expected config' : 'ClientRects Hash is detected output hash, not expected config'));
  rows.push(effectObservedFingerprintRow('Fonts Hash', context, 'fontHash', report.advanced.fontHash, 'Fonts Hash is detected output hash, not expected config'));
  rows.push(observedFingerprintRow('Detected Fonts', context, 'detectedFonts', expected.fontList, report.advanced.detectedFonts, compareArrayContainsAllStatus, 'Compare whether detected fonts include configured list'));
  rows.push(observedFingerprintRow('WebGL Vendor', context, 'webglVendor', expected.webGLVendor || expected.webglVendor, report.advanced.webglVendor, compareExactStatus, 'Compare WebGL Vendor'));
  rows.push(observedFingerprintRow('WebGL Renderer', context, 'webglRenderer', expected.webGLRenderer || expected.webglRenderer, report.advanced.webglRenderer, compareExactStatus, 'Compare WebGL Renderer'));
  rows.push(effectObservedFingerprintRow('WebGL Hash', context, 'webglHash', report.advanced.webglHash, 'WebGL Hash is detected output hash, not expected config'));
  rows.push(observedFingerprintRow('Plugins', context, 'plugins', '', report.advanced.plugins, compareExactStatus, 'Compare plugin list'));
  rows.push(observedFingerprintRow('MIME Types', context, 'mimeTypes', '', report.advanced.mimeTypes, compareExactStatus, 'Compare MIME list'));
  rows.push(observedFingerprintRow('Screen Size', context, 'screenSize', '', report.screen.width + 'x' + report.screen.height, compareExactStatus, 'Compare screen size'));
  rows.push(observedFingerprintRow('DPR', context, 'devicePixelRatio', '', report.screen.devicePixelRatio, compareExactStatus, 'Compare DPR'));
  return rows;
}
function shouldShowReason(status) {
  return status === 'mismatch' || status === 'warning' || status === 'unreadable' || status === 'unsupported' || status === 'baseline' || status === 'unknown';
}
function renderFingerprintTable(rows) {
  return '<section><h2>' + uiText('Fingerprint Check') + '</h2><table><thead><tr><th>' + uiText('Item') + '</th><th>' + uiText('Value') + '</th><th>' + uiText('Expected Source') + '</th><th>' + uiText('Result') + '</th><th>' + uiText('Reason') + '</th></tr></thead><tbody>' + rows.map(function (item) {
    var reason = shouldShowReason(item.status) ? item.reason : '';
    return '<tr><td class="item">' + escapeHtml(uiText(item.name)) + '</td><td>' + pairValue(item.expected, item.actual) + '</td><td class="source">' + escapeHtml(uiText(item.source)) + '</td><td class="hit ' + statusClass(item.status, item.source) + '">' + uiText(statusText(item.status, item.source)) + '</td><td class="reason">' + escapeHtml(uiText(reason)) + '</td></tr>';
  }).join('') + '</tbody></table></section>';
}
function proxyDisplayValue(value) {
  if (value === undefined || value === null || value === '') return '-';
  return String(value);
}
function proxyEndpoint(proxy) {
  if (!proxy || proxy.direct) return uiText('Direct');
  if (proxy.host) return proxy.host + (proxy.port ? ':' + proxy.port : '');
  return proxy.summary || uiText('No endpoint');
}
function proxyCountryName(info) {
  var code = normalizeCountryCode(info && info.country, info && info.countryCode);
  var country = countryNameFor(code, info && info.country);
  if (currentUILang !== 'zh') return country;
  var names = { BE: '比利时', CN: '中国', EU: '欧盟', HK: '中国香港', US: '美国' };
  return names[code] || country;
}
function proxyRegionName(value, countryCode) {
  var text = cleanString(value);
  if (currentUILang !== 'zh') return text;
  if (countryCode === 'HK' && /^hong kong$/i.test(text)) return '香港';
  if (countryCode === 'CN' && /^beijing$/i.test(text)) return '北京';
  return text;
}
function proxyLocation(info) {
  if (!info || !info.ok) return '-';
  var code = normalizeCountryCode(info.country, info.countryCode);
  return [proxyCountryName(info), proxyRegionName(info.region, code), proxyRegionName(info.city, code)].filter(Boolean).join(' / ') || uiText('No location');
}
function proxyLine(label, value) {
  return '<div class="proxy-line"><span class="proxy-label">' + uiText(label) + '</span><span class="proxy-value">' + escapeHtml(proxyDisplayValue(value)) + '</span></div>';
}
function proxyHTMLLine(label, html) {
  return '<div class="proxy-line"><span class="proxy-label">' + uiText(label) + '</span><span class="proxy-value">' + html + '</span></div>';
}
function proxyStatusLine(label, text, status) {
  return '<div class="proxy-line"><span class="proxy-label">' + uiText(label) + '</span><span class="proxy-value"><span class="proxy-status ' + escapeHtml(status) + '">' + uiText(text) + '</span></span></div>';
}
function proxyExitStatus(info) {
  if (!info || !info.ok) return { text: 'Detection Failed', status: 'bad' };
  if (info.locationStatus === 'conflict') return { text: 'Location Conflict', status: 'warn' };
  if (info.locationStatus === 'single') return { text: 'Single Source', status: 'warn' };
  return { text: 'Detected', status: 'ok' };
}
function proxyLocationText(info) {
  if (!info || !info.ok) return '-';
  var location = proxyLocation(info);
  if (info.locationStatus === 'conflict') return uiText('Location conflict') + '；' + uiText('Reference Location') + ': ' + location + (info.referenceSource ? ' (' + info.referenceSource + ')' : '');
  return location;
}
function proxyNetworkText(info) {
  if (!info || !info.ok) return '-';
  var parts = [];
  if (info.asn) parts.push('AS' + String(info.asn).replace(/^AS/i, ''));
  if (info.route) parts.push(info.route);
  else if (info.network) parts.push(info.network);
  if (info.org) parts.push(info.org);
  if (info.netname) parts.push(info.netname);
  if (info.datacenter) parts.push(uiText('Datacenter'));
  if (info.proxy || info.vpn) parts.push(uiText('Proxy/VPN'));
  if (info.rentedOrIpXO) parts.push(uiText('IPXO/Rented Segment'));
  return parts.length ? parts.join(' / ') : uiText('No ASN');
}
function proxySourceDetails(info) {
  var sources = info && Array.isArray(info.sources) ? info.sources : [];
  var lines = sources.filter(function (item) { return item && item.source && item.kind !== 'ip'; }).map(function (item) {
    if (!item.ok) return item.source + ': ' + (item.error || 'failed');
    var parts = [];
    if (item.kind === 'network') {
      parts.push(item.netname || item.org || 'network');
      parts.push(item.route || item.network || '');
      parts.push(item.country || item.countryCode || '');
    } else {
      parts.push(proxyLocation(item));
      var network = proxyNetworkText(item);
      if (network && network !== uiText('No ASN')) parts.push(network);
    }
    return item.source + ': ' + parts.filter(Boolean).join(' / ');
  });
  if (!lines.length && info && info.error) lines.push(info.error);
  if (!lines.length) return escapeHtml(uiText('No source detail'));
  return '<span class="proxy-source-list">' + lines.map(function (line) { return escapeHtml(line); }).join('<br>') + '</span>';
}
function renderProxyCheck(report) {
  var proxy = latestContext && latestContext.proxy ? latestContext.proxy : {};
  var network = report && report.network ? report.network : {};
  var info = network.proxyInfo || {};
  var configStatus = proxy.direct ? 'warn' : (proxy.configured ? 'ok' : 'warn');
  var configText = proxy.direct ? 'Direct' : (proxy.configured ? 'Configured' : 'No proxy configured');
  var exit = proxyExitStatus(info);
  var configLines = [
    proxyStatusLine('Status', configText, configStatus),
    proxyLine('Name', proxy.proxyName || proxy.proxyId || '-'),
    proxyLine('Type', proxy.type || '-'),
    proxyLine('Endpoint', proxyEndpoint(proxy)),
    proxyLine('Auth', proxy.hasAuth ? uiText('Yes') : uiText('No')),
    proxyLine('Group', proxy.groupName || '-')
  ].join('');
  var exitLines = [
    proxyStatusLine('Status', exit.text, exit.status),
    proxyLine('Current IP', info.ip || '-'),
    proxyLine('Location', proxyLocationText(info)),
    proxyLine('Confidence', info.ok ? uiText(info.confidence || 'Single Source') : '-'),
    proxyLine('ASN/Network', proxyNetworkText(info)),
    proxyLine('Latency', info.ok ? String(info.latencyMs || 0) + ' ms' : '-'),
    proxyHTMLLine('Source Details', proxySourceDetails(info))
  ].join('');
  return '<section class="proxy-panel"><h2>' + uiText('Proxy Check') + '</h2><div class="proxy-grid"><div class="proxy-card"><div class="proxy-title">' + uiText('Configured Proxy') + '</div><div class="proxy-lines">' + configLines + '</div></div><div class="proxy-card"><div class="proxy-title">' + uiText('Browser Exit') + '</div><div class="proxy-lines">' + exitLines + '</div></div></div></section>';
}
function valueAtPath(source, path) {
  return path.split('.').reduce(function (current, key) {
    return current && Object.prototype.hasOwnProperty.call(current, key) ? current[key] : undefined;
  }, source);
}
function beforeSnapshotReport(snapshot) {
  return snapshot && snapshot.report ? snapshot.report : snapshot;
}
function beforeSnapshotContext(snapshot) {
  return snapshot && snapshot.context ? snapshot.context : {};
}
function displayObservedValue(value) {
  if (value === undefined || value === null || value === '') return '-';
  if (Array.isArray(value)) return value.join(', ');
  if (typeof value === 'object') return JSON.stringify(value);
  return String(value);
}
var changeCompareFields = [
  ['Config Seed', 'context.expected.seed'],
  ['Canvas Hash', 'advanced.canvasHash'],
  ['Audio Hash', 'advanced.audioHash'],
  ['ClientRects Hash', 'advanced.clientRectsHash'],
  ['Fonts Hash', 'advanced.fontHash'],
  ['Detected Fonts', 'advanced.detectedFonts'],
  ['WebGL Vendor', 'advanced.webglVendor'],
  ['WebGL Renderer', 'advanced.webglRenderer'],
  ['WebGL Hash', 'advanced.webglHash'],
  ['Plugins', 'advanced.plugins'],
  ['MIME Types', 'advanced.mimeTypes'],
  ['Screen Size', 'screen.width'],
  ['Screen Height', 'screen.height'],
  ['DPR', 'screen.devicePixelRatio'],
  ['Language', 'locale.language'],
  ['Languages', 'locale.languages'],
  ['Timezone', 'locale.timezone'],
  ['CPU Cores', 'hardware.hardwareConcurrency'],
  ['Device Memory', 'hardware.deviceMemory'],
  ['Touch Points', 'hardware.maxTouchPoints'],
  ['User-Agent', 'identity.userAgent'],
  ['Platform', 'identity.platform']
];
function renderChangeComparison(report) {
  var target = document.getElementById('changeApp');
  if (!latestBeforeSnapshot) {
    target.innerHTML = '<div class="diff-empty">' + uiText('No before snapshot saved. To compare changes after Seed/config edits: run the old config, click Save Before, edit config and restart the instance, then click Refresh.') + '</div>';
    return;
  }
  var beforeReport = beforeSnapshotReport(latestBeforeSnapshot) || {};
  var beforeContext = beforeSnapshotContext(latestBeforeSnapshot);
  var beforeCompareSource = Object.assign({}, beforeReport, { context: beforeContext });
  var currentCompareSource = Object.assign({}, report || {}, { context: latestContext || {} });
  var beforeTime = beforeReport.generatedAt ? formatLocalDateTime(beforeReport.generatedAt) : '-';
  var currentTime = report && report.generatedAt ? formatLocalDateTime(report.generatedAt) : '-';
  var rows = changeCompareFields.map(function (field) {
    var beforeValue = valueAtPath(beforeCompareSource, field[1]);
    var currentValue = valueAtPath(currentCompareSource, field[1]);
    var changed = !sameObservedValue(beforeValue, currentValue);
    return '<tr><td class="item">' + escapeHtml(uiText(field[0])) + '</td><td>' + pairValue(displayObservedValue(beforeValue), displayObservedValue(currentValue)) + '</td><td class="source">' + uiText('Before/After') + '</td><td class="hit ' + (changed ? 'warn' : 'ok') + '">' + uiText(changed ? 'Changed' : 'Unchanged') + '</td><td class="reason">' + escapeHtml(uiText('Before ') + beforeTime + uiText(' / Current ') + currentTime) + '</td></tr>';
  }).join('');
  target.innerHTML = '<section><h2>' + uiText('Before/After Changes') + '</h2><table><thead><tr><th>' + uiText('Item') + '</th><th>' + uiText('Value') + '</th><th>' + uiText('Source') + '</th><th>' + uiText('Result') + '</th><th>' + uiText('Time') + '</th></tr></thead><tbody>' + rows + '</tbody></table></section>';
}
function padDatePart(value) { return String(value).padStart(2, '0'); }
function formatLocalDateTime(isoText) {
  var date = new Date(isoText);
  if (Number.isNaN(date.getTime())) return isoText || '-';
  return date.getFullYear() + '-' + padDatePart(date.getMonth() + 1) + '-' + padDatePart(date.getDate()) + ' ' + padDatePart(date.getHours()) + ':' + padDatePart(date.getMinutes()) + ':' + padDatePart(date.getSeconds());
}
function reportAgeMs(report) {
  var time = Date.parse(report && report.generatedAt ? report.generatedAt : '');
  if (Number.isNaN(time)) return 0;
  return Date.now() - time;
}
function renderMeta(report) {
  var parts = [uiText('Checked: Local ') + formatLocalDateTime(report.generatedAt), 'UTC ' + report.generatedAt];
  if (report.urlProfileId) parts.push('Profile: ' + report.urlProfileId);
  document.getElementById('meta').textContent = parts.join(' / ');
}
function escapeHtml(text) { return String(text).replace(/[&<>"']/g, function (m) { return ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'})[m]; }); }
function render(report) {
  updateUILanguage(report);
  var rows = buildFingerprintRows(report, latestContext);
  report.rows = rows;
  renderMeta(report);
  document.getElementById('summary').innerHTML = '';
  renderChangeComparison(report);
  document.getElementById('proxyApp').innerHTML = renderProxyCheck(report);
  document.getElementById('app').innerHTML = renderFingerprintTable(rows);
}
async function run() {
  if (fingerprintCheckRunning) return;
  fingerprintCheckRunning = true;
  try {
    latestReport = await collect();
    render(latestReport);
  } finally {
    fingerprintCheckRunning = false;
  }
}
function maybeAutoRefresh() {
  if (!latestReport || fingerprintCheckRunning) return;
  if (reportAgeMs(latestReport) >= FINGERPRINT_AUTO_REFRESH_MS) run();
}
document.getElementById('resetBaselineBtn').onclick = function () { resetFingerprintBaseline(latestContext); run(); };
document.getElementById('saveBeforeBtn').onclick = function () { saveBeforeSnapshot(latestContext, latestReport); renderChangeComparison(latestReport); };
document.getElementById('clearBeforeBtn').onclick = function () { clearBeforeSnapshot(latestContext); renderChangeComparison(latestReport); };
document.getElementById('refreshBtn').onclick = run;
document.getElementById('copyBtn').onclick = function () {
  if (!latestReport) return;
  navigator.clipboard.writeText(JSON.stringify(latestReport, null, 2)).catch(function () {});
};
setInterval(maybeAutoRefresh, FINGERPRINT_AUTO_REFRESH_CHECK_MS);
updateUILanguage(null);
loadBeforeSnapshot(latestContext);
run();
</script>
</body>
</html>`
