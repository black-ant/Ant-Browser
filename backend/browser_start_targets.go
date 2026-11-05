package backend

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/logger"
)

func browserLightStartEnabled(cfg *config.Config) bool {
	if cfg == nil || cfg.Browser.LightStartEnabled == nil {
		return true
	}
	return *cfg.Browser.LightStartEnabled
}

func resolveConfiguredStartTargets(startURLs []string, defaultStartURLs []string, skipDefaultStartURLs bool) []string {
	normalizedStartURLs := normalizeNonEmptyStrings(startURLs)
	if len(normalizedStartURLs) > 0 {
		return normalizedStartURLs
	}

	if skipDefaultStartURLs {
		return nil
	}

	return normalizeNonEmptyStrings(defaultStartURLs)
}

func buildBrowserLaunchTargets(startURLs []string, defaultStartURLs []string, skipDefaultStartURLs bool, restoreLastSession bool, lightStartEnabled bool) ([]string, []string, bool) {
	configuredTargets := resolveConfiguredStartTargets(startURLs, defaultStartURLs, skipDefaultStartURLs)
	if restoreLastSession && len(configuredTargets) > 0 {
		return nil, configuredTargets, true
	}
	if lightStartEnabled && len(configuredTargets) > 0 {
		return []string{"about:blank"}, configuredTargets, false
	}
	if len(configuredTargets) > 0 {
		return configuredTargets, nil, false
	}
	if !restoreLastSession {
		return []string{"about:blank"}, nil, false
	}
	return nil, nil, false
}

type deferredStartTargetsPlan struct {
	targets []string
	newTabs bool
}

func deferredStartTargetsWarning(targets []string, err error) string {
	normalized := normalizeNonEmptyStrings(targets)
	if len(normalized) <= 1 {
		return fmt.Sprintf("浏览器已启动，但启动页未能自动打开：%v。可稍后手动打开。", err)
	}
	return fmt.Sprintf("浏览器已启动，但 %d 个启动页未能自动打开：%v。可稍后手动打开。", len(normalized), err)
}

func (a *App) storeDeferredStartTargets(profileId string, targets []string, newTabs bool) {
	if a == nil {
		return
	}

	normalized := normalizeNonEmptyStrings(targets)
	a.deferredStartTargetsMu.Lock()
	defer a.deferredStartTargetsMu.Unlock()

	if len(normalized) == 0 {
		delete(a.deferredStartTargets, profileId)
		return
	}
	a.deferredStartTargets[profileId] = deferredStartTargetsPlan{targets: append([]string{}, normalized...), newTabs: newTabs}
}

func (a *App) consumeDeferredStartTargets(profileId string) deferredStartTargetsPlan {
	if a == nil {
		return deferredStartTargetsPlan{}
	}

	a.deferredStartTargetsMu.Lock()
	defer a.deferredStartTargetsMu.Unlock()

	plan := a.deferredStartTargets[profileId]
	plan.targets = append([]string{}, plan.targets...)
	delete(a.deferredStartTargets, profileId)
	return plan
}

func (a *App) clearDeferredStartTargets(profileId string) {
	if a == nil {
		return
	}

	a.deferredStartTargetsMu.Lock()
	delete(a.deferredStartTargets, profileId)
	a.deferredStartTargetsMu.Unlock()
}

func cdpBrowserCallResult(debugPort int, method string, params map[string]any) (map[string]any, error) {
	body, err := cdpGetEndpointBody(debugPort, "/json/version")
	if err != nil {
		return nil, fmt.Errorf("CDP /json/version 请求失败: %w", err)
	}
	var version cdpBrowserVersion
	if err := json.Unmarshal(body, &version); err != nil {
		return nil, fmt.Errorf("CDP browser target 解析失败: %w", err)
	}
	wsURL := strings.TrimSpace(version.WebSocketDebuggerUrl)
	if wsURL == "" {
		return nil, fmt.Errorf("未找到浏览器级 WebSocket 调试地址")
	}

	conn, err := cdpDialWebSocket(wsURL)
	if err != nil {
		return nil, fmt.Errorf("浏览器级 WebSocket 连接失败: %w", err)
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(cdpWebSocketReadTimeout))

	msg := cdpMessage{Id: 1, Method: method, Params: params}
	if err := conn.WriteJSON(msg); err != nil {
		return nil, fmt.Errorf("浏览器级 CDP 命令发送失败: %w", err)
	}

	var cdpResp cdpResponse
	if err := conn.ReadJSON(&cdpResp); err != nil {
		return nil, fmt.Errorf("浏览器级 CDP 响应读取失败: %w", err)
	}
	if cdpResp.Error != nil {
		return nil, fmt.Errorf("浏览器级 CDP 错误: %s", cdpResp.Error.Message)
	}
	return cdpResp.Result, nil
}

func createBrowserStartTarget(debugPort int, url string) error {
	_, err := cdpBrowserCallResult(debugPort, "Target.createTarget", map[string]any{
		"url": url,
	})
	return err
}

func openBrowserStartTargets(debugPort int, targets []string) error {
	normalized := normalizeNonEmptyStrings(targets)
	if len(normalized) == 0 {
		return nil
	}

	_, navigateErr := cdpCall(debugPort, "Page.navigate", map[string]any{
		"url": normalized[0],
	})
	if navigateErr == nil {
		for _, url := range normalized[1:] {
			if err := createBrowserStartTarget(debugPort, url); err != nil {
				return err
			}
		}
		return nil
	}

	for _, url := range normalized {
		if err := createBrowserStartTarget(debugPort, url); err != nil {
			return fmt.Errorf("首个页面改为新标签回退失败：navigate=%v, createTarget=%w", navigateErr, err)
		}
	}
	return nil
}

func openBrowserStartTargetsInNewTabs(debugPort int, targets []string) error {
	normalized := normalizeNonEmptyStrings(targets)
	for _, url := range normalized {
		if err := createBrowserStartTarget(debugPort, url); err != nil {
			return err
		}
	}
	return nil
}

func openDeferredStartTargets(debugPort int, plan deferredStartTargetsPlan) error {
	if plan.newTabs {
		return openBrowserStartTargetsInNewTabs(debugPort, plan.targets)
	}
	return openBrowserStartTargets(debugPort, plan.targets)
}

func (a *App) setProfileRuntimeWarning(profileId string, debugPort int, warning string) (*BrowserProfile, bool) {
	return a.setProfileRuntimeWarningForMonitor(profileId, debugPort, warning, nil)
}

func (a *App) setProfileRuntimeWarningForMonitor(profileId string, debugPort int, warning string, monitor *browserProcessMonitor) (*BrowserProfile, bool) {
	if a == nil || a.browserMgr == nil {
		return nil, false
	}

	a.browserMgr.Mutex.Lock()
	defer a.browserMgr.Mutex.Unlock()
	if !a.browserProcessMonitorIsCurrentLocked(profileId, monitor) {
		return nil, false
	}

	profile, exists := a.browserMgr.Profiles[profileId]
	if !exists || profile == nil || !profile.Running || profile.DebugPort != debugPort {
		return nil, false
	}

	if profile.RuntimeWarning == warning && profile.LastError == "" {
		return copyBrowserProfileSnapshot(profile), false
	}
	profile.RuntimeWarning = warning
	profile.LastError = ""
	return copyBrowserProfileSnapshot(profile), true
}

func (a *App) finalizeDeferredStartTargets(profileId string, debugPort int) (*BrowserProfile, bool) {
	return a.finalizeDeferredStartTargetsForMonitor(profileId, debugPort, nil)
}

func (a *App) finalizeDeferredStartTargetsForMonitor(profileId string, debugPort int, monitor *browserProcessMonitor) (*BrowserProfile, bool) {
	if !a.browserProcessMonitorIsCurrent(profileId, monitor) {
		return nil, false
	}
	plan := a.consumeDeferredStartTargets(profileId)
	if len(plan.targets) == 0 {
		return nil, false
	}
	if !a.browserProcessMonitorIsCurrent(profileId, monitor) {
		return nil, false
	}

	if err := openDeferredStartTargets(debugPort, plan); err != nil {
		warning := deferredStartTargetsWarning(plan.targets, err)
		snapshot, changed := a.setProfileRuntimeWarningForMonitor(profileId, debugPort, warning, monitor)
		logger.New("Browser").Warn("浏览器已就绪，但启动页延后打开失败",
			logger.F("profile_id", profileId),
			logger.F("debug_port", debugPort),
			logger.F("target_count", len(plan.targets)),
			logger.F("error", err.Error()),
			logger.F("warning", warning),
		)
		return snapshot, changed
	}

	return nil, false
}
