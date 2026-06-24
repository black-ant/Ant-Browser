package backend

import (
	"fmt"

	"ant-chrome/backend/internal/cdp"
	"ant-chrome/backend/internal/logger"
)

// CDPSessionCreate 创建CDP会话
func (a *App) CDPSessionCreate(profileID string, targetType string) (string, error) {
	// 获取实例的调试端口
	a.browserMgr.InitData()
	a.browserMgr.Mutex.Lock()
	profile, exists := a.browserMgr.Profiles[profileID]
	a.browserMgr.Mutex.Unlock()

	if !exists {
		return "", fmt.Errorf("实例不存在: %s", profileID)
	}

	if !profile.Running {
		return "", fmt.Errorf("实例未运行")
	}

	if profile.DebugPort == 0 {
		return "", fmt.Errorf("实例调试端口未开启")
	}

	// 创建CDP会话
	session, err := a.cdpManager.CreateSession(profileID, profile.DebugPort, targetType)
	if err != nil {
		logger.New("CDP").Error("[CDP] 创建会话失败", logger.F("profile_id", profileID), logger.F("debug_port", profile.DebugPort), logger.F("error", err.Error()))
		return "", fmt.Errorf("创建CDP会话失败: %w", err)
	}

	logger.New("CDP").Info("[CDP] 会话已创建", logger.F("session_id", session.SessionID), logger.F("profile_id", profileID))

	return session.SessionID, nil
}

// CDPSessionClose 关闭CDP会话
func (a *App) CDPSessionClose(sessionID string) error {
	// 验证会话存在
	if _, err := a.cdpManager.GetSession(sessionID); err != nil {
		return fmt.Errorf("关闭CDP会话失败: %w", err)
	}

	if err := a.cdpManager.CloseSession(sessionID); err != nil {
		return fmt.Errorf("关闭CDP会话失败: %w", err)
	}

	logger.New("CDP").Info("[CDP] 会话已关闭", logger.F("session_id", sessionID))
	return nil
}

// CDPGetNetworkRequests 获取网络请求
func (a *App) CDPGetNetworkRequests(sessionID string) ([]cdp.NetworkRequest, error) {
	session, err := a.cdpManager.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	return session.GetNetworkRequests(), nil
}

// CDPGetConsoleLogs 获取Console日志
func (a *App) CDPGetConsoleLogs(sessionID string) ([]cdp.ConsoleLog, error) {
	session, err := a.cdpManager.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	return session.GetConsoleLogs(), nil
}

// CDPEnableConsoleCapture 惰性启用 Runtime 域以捕获页面 console 日志。
// 仅在前端实际切换到「控制台」时调用，避免 Runtime.enable 这一最易被检测站点
// 探测的 CDP 痕迹长期暴露。重复调用安全。
func (a *App) CDPEnableConsoleCapture(sessionID string) error {
	session, err := a.cdpManager.GetSession(sessionID)
	if err != nil {
		return err
	}
	if err := session.EnableConsoleCapture(); err != nil {
		return fmt.Errorf("启用控制台捕获失败: %w", err)
	}
	logger.New("CDP").Info("[CDP] 控制台捕获已启用", logger.F("session_id", sessionID))
	return nil
}

// CDPClearNetworkRequests 清空网络请求
func (a *App) CDPClearNetworkRequests(sessionID string) error {
	session, err := a.cdpManager.GetSession(sessionID)
	if err != nil {
		return err
	}

	session.ClearNetworkRequests()
	logger.New("CDP").Info("[CDP] 网络请求已清空", logger.F("session_id", sessionID))
	return nil
}

// CDPClearConsoleLogs 清空Console日志
func (a *App) CDPClearConsoleLogs(sessionID string) error {
	session, err := a.cdpManager.GetSession(sessionID)
	if err != nil {
		return err
	}

	session.ClearConsoleLogs()
	logger.New("CDP").Info("[CDP] Console日志已清空", logger.F("session_id", sessionID))
	return nil
}

// CDPGetWebSocketMessages 获取WebSocket消息
func (a *App) CDPGetWebSocketMessages(sessionID string) ([]cdp.WebSocketMessage, error) {
	session, err := a.cdpManager.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	return session.GetWebSocketMessages(), nil
}

// CDPClearWebSocketMessages 清空WebSocket消息
func (a *App) CDPClearWebSocketMessages(sessionID string) error {
	session, err := a.cdpManager.GetSession(sessionID)
	if err != nil {
		return err
	}

	session.ClearWebSocketMessages()
	logger.New("CDP").Info("[CDP] WebSocket消息已清空", logger.F("session_id", sessionID))
	return nil
}

// CDPExecuteJavaScript 执行JavaScript
func (a *App) CDPExecuteJavaScript(sessionID string, code string) (string, error) {
	session, err := a.cdpManager.GetSession(sessionID)
	if err != nil {
		return "", err
	}

	result, err := session.ExecuteJavaScript(code)
	if err != nil {
		return "", fmt.Errorf("执行JavaScript失败: %w", err)
	}

	logger.New("CDP").Info("[CDP] JavaScript已执行", logger.F("session_id", sessionID))
	return result, nil
}

// CDPCaptureScreenshot 截图
func (a *App) CDPCaptureScreenshot(sessionID string) (string, error) {
	session, err := a.cdpManager.GetSession(sessionID)
	if err != nil {
		return "", err
	}

	screenshot, err := session.CaptureScreenshot()
	if err != nil {
		return "", fmt.Errorf("截图失败: %w", err)
	}

	logger.New("CDP").Info("[CDP] 截图已捕获", logger.F("session_id", sessionID))
	return screenshot, nil
}

// CDPExportHAR 导出HAR
func (a *App) CDPExportHAR(sessionID string) (string, error) {
	session, err := a.cdpManager.GetSession(sessionID)
	if err != nil {
		return "", err
	}

	harData, err := session.ExportHAR()
	if err != nil {
		return "", fmt.Errorf("导出HAR失败: %w", err)
	}

	logger.New("CDP").Info("[CDP] HAR已导出", logger.F("session_id", sessionID))
	return string(harData), nil
}

// CDPGetStatistics 获取统计信息
func (a *App) CDPGetStatistics(sessionID string) (map[string]interface{}, error) {
	session, err := a.cdpManager.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	return session.GetStatistics(), nil
}

// CDPGetStorage 获取 localStorage / sessionStorage 数据
func (a *App) CDPGetStorage(sessionID string, storageType string) (map[string]string, error) {
	session, err := a.cdpManager.GetSession(sessionID)
	if err != nil {
		return nil, err
	}
	data, err := session.GetStorageData(storageType)
	if err != nil {
		return nil, fmt.Errorf("获取存储数据失败: %w", err)
	}
	return data, nil
}

// CDPReloadPage 通过 CDP 重新加载当前页面（用于完整抓取一次页面加载的网络请求，
// 避免在实例里手动 F5）。
func (a *App) CDPReloadPage(sessionID string) error {
	session, err := a.cdpManager.GetSession(sessionID)
	if err != nil {
		return err
	}
	if _, err := session.SendCommand("Page.reload", map[string]interface{}{"ignoreCache": false}); err != nil {
		return fmt.Errorf("重新加载页面失败: %w", err)
	}
	logger.New("CDP").Info("[CDP] 页面已重新加载", logger.F("session_id", sessionID))
	return nil
}

// CDPListSessions 列出所有会话
func (a *App) CDPListSessions() ([]string, error) {
	return a.cdpManager.ListSessions(), nil
}

// CDPGetCookies 获取所有 Cookie
func (a *App) CDPGetCookies(sessionID string) ([]cdp.Cookie, error) {
	session, err := a.cdpManager.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	cookies, err := session.GetCookies()
	if err != nil {
		return nil, fmt.Errorf("获取 Cookie 失败: %w", err)
	}

	return cookies, nil
}

// CDPSetCookie 设置 Cookie
func (a *App) CDPSetCookie(sessionID string, cookie cdp.Cookie) error {
	session, err := a.cdpManager.GetSession(sessionID)
	if err != nil {
		return err
	}

	if err := session.SetCookie(cookie); err != nil {
		return fmt.Errorf("设置 Cookie 失败: %w", err)
	}

	logger.New("CDP").Info("[CDP] Cookie 已设置", logger.F("session_id", sessionID), logger.F("name", cookie.Name))
	return nil
}

// CDPDeleteCookie 删除 Cookie
func (a *App) CDPDeleteCookie(sessionID string, name string, domain string, path string) error {
	session, err := a.cdpManager.GetSession(sessionID)
	if err != nil {
		return err
	}

	if err := session.DeleteCookie(name, domain, path); err != nil {
		return fmt.Errorf("删除 Cookie 失败: %w", err)
	}

	logger.New("CDP").Info("[CDP] Cookie 已删除", logger.F("session_id", sessionID), logger.F("name", name))
	return nil
}

// CDPClearAllCookies 清空所有 Cookie
func (a *App) CDPClearAllCookies(sessionID string) error {
	session, err := a.cdpManager.GetSession(sessionID)
	if err != nil {
		return err
	}

	if err := session.ClearAllCookies(); err != nil {
		return fmt.Errorf("清空 Cookie 失败: %w", err)
	}

	logger.New("CDP").Info("[CDP] 所有 Cookie 已清空", logger.F("session_id", sessionID))
	return nil
}

// CDPEnableIntercept 启用请求拦截
func (a *App) CDPEnableIntercept(sessionID string) error {
	session, err := a.cdpManager.GetSession(sessionID)
	if err != nil {
		return err
	}

	if err := session.EnableIntercept(); err != nil {
		return fmt.Errorf("启用拦截失败: %w", err)
	}

	logger.New("CDP").Info("[CDP] 请求拦截已启用", logger.F("session_id", sessionID))
	return nil
}

// CDPDisableIntercept 禁用请求拦截
func (a *App) CDPDisableIntercept(sessionID string) error {
	session, err := a.cdpManager.GetSession(sessionID)
	if err != nil {
		return err
	}

	if err := session.DisableIntercept(); err != nil {
		return fmt.Errorf("禁用拦截失败: %w", err)
	}

	logger.New("CDP").Info("[CDP] 请求拦截已禁用", logger.F("session_id", sessionID))
	return nil
}

// CDPAddInterceptRule 添加拦截规则
func (a *App) CDPAddInterceptRule(sessionID string, rule cdp.InterceptRule) error {
	session, err := a.cdpManager.GetSession(sessionID)
	if err != nil {
		return err
	}

	if err := session.AddInterceptRule(rule); err != nil {
		return fmt.Errorf("添加拦截规则失败: %w", err)
	}

	logger.New("CDP").Info("[CDP] 拦截规则已添加", logger.F("session_id", sessionID), logger.F("rule_id", rule.ID))
	return nil
}

// CDPRemoveInterceptRule 删除拦截规则
func (a *App) CDPRemoveInterceptRule(sessionID string, ruleID string) error {
	session, err := a.cdpManager.GetSession(sessionID)
	if err != nil {
		return err
	}

	if err := session.RemoveInterceptRule(ruleID); err != nil {
		return fmt.Errorf("删除拦截规则失败: %w", err)
	}

	logger.New("CDP").Info("[CDP] 拦截规则已删除", logger.F("session_id", sessionID), logger.F("rule_id", ruleID))
	return nil
}

// CDPGetInterceptRules 获取拦截规则列表
func (a *App) CDPGetInterceptRules(sessionID string) ([]cdp.InterceptRule, error) {
	session, err := a.cdpManager.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	return session.GetInterceptRules(), nil
}

// CDPUpdateInterceptRule 更新拦截规则
func (a *App) CDPUpdateInterceptRule(sessionID string, rule cdp.InterceptRule) error {
	session, err := a.cdpManager.GetSession(sessionID)
	if err != nil {
		return err
	}

	if err := session.UpdateInterceptRule(rule); err != nil {
		return fmt.Errorf("更新拦截规则失败: %w", err)
	}

	logger.New("CDP").Info("[CDP] 拦截规则已更新", logger.F("session_id", sessionID), logger.F("rule_id", rule.ID))
	return nil
}
