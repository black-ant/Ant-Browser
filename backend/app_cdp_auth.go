package backend

import (
	"fmt"

	"ant-chrome/backend/internal/cdp"
	"ant-chrome/backend/internal/logger"
)

// CDPGetNetworkRequestsAuth 获取网络请求（带所有权验证）
func (a *App) CDPGetNetworkRequestsAuth(sessionID string, profileID string) ([]cdp.NetworkRequest, error) {
	session, err := a.cdpManager.GetSessionWithOwnerCheck(sessionID, profileID)
	if err != nil {
		// 记录认证失败
		logger.New("CDP").Warn("CDP会话所有权验证失败",
			logger.F("session_id", sessionID),
			logger.F("profile_id", profileID),
			logger.F("operation", "GetNetworkRequests"),
			logger.F("error", err.Error()))
		return nil, err
	}

	return session.GetNetworkRequests(), nil
}

// CDPGetConsoleLogsAuth 获取Console日志（带所有权验证）
func (a *App) CDPGetConsoleLogsAuth(sessionID string, profileID string) ([]cdp.ConsoleLog, error) {
	session, err := a.cdpManager.GetSessionWithOwnerCheck(sessionID, profileID)
	if err != nil {
		// 记录认证失败
		logger.New("CDP").Warn("CDP会话所有权验证失败",
			logger.F("session_id", sessionID),
			logger.F("profile_id", profileID),
			logger.F("operation", "GetConsoleLogs"),
			logger.F("error", err.Error()))
		return nil, err
	}

	return session.GetConsoleLogs(), nil
}

// CDPExecuteJavaScriptAuth 执行JavaScript（带所有权验证）
func (a *App) CDPExecuteJavaScriptAuth(sessionID string, profileID string, code string) (string, error) {
	session, err := a.cdpManager.GetSessionWithOwnerCheck(sessionID, profileID)
	if err != nil {
		// 记录认证失败
		logger.New("CDP").Warn("CDP会话所有权验证失败",
			logger.F("session_id", sessionID),
			logger.F("profile_id", profileID),
			logger.F("operation", "ExecuteJavaScript"),
			logger.F("error", err.Error()))
		return "", err
	}

	result, err := session.ExecuteJavaScript(code)
	if err != nil {
		return "", fmt.Errorf("执行JavaScript失败: %w", err)
	}

	return result, nil
}

// CDPGetCookiesAuth 获取所有 Cookie（带所有权验证）
func (a *App) CDPGetCookiesAuth(sessionID string, profileID string) ([]cdp.Cookie, error) {
	session, err := a.cdpManager.GetSessionWithOwnerCheck(sessionID, profileID)
	if err != nil {
		// 记录认证失败
		logger.New("CDP").Warn("CDP会话所有权验证失败",
			logger.F("session_id", sessionID),
			logger.F("profile_id", profileID),
			logger.F("operation", "GetCookies"),
			logger.F("error", err.Error()))
		return nil, err
	}

	cookies, err := session.GetCookies()
	if err != nil {
		return nil, fmt.Errorf("获取 Cookie 失败: %w", err)
	}

	return cookies, nil
}

// CDPSetCookieAuth 设置 Cookie（带所有权验证）
func (a *App) CDPSetCookieAuth(sessionID string, profileID string, cookie cdp.Cookie) error {
	session, err := a.cdpManager.GetSessionWithOwnerCheck(sessionID, profileID)
	if err != nil {
		// 记录认证失败
		logger.New("CDP").Warn("CDP会话所有权验证失败",
			logger.F("session_id", sessionID),
			logger.F("profile_id", profileID),
			logger.F("operation", "SetCookie"),
			logger.F("error", err.Error()))
		return err
	}

	if err := session.SetCookie(cookie); err != nil {
		return fmt.Errorf("设置 Cookie 失败: %w", err)
	}

	return nil
}

// CDPClearAllCookiesAuth 清空所有 Cookie（带所有权验证）
func (a *App) CDPClearAllCookiesAuth(sessionID string, profileID string) error {
	session, err := a.cdpManager.GetSessionWithOwnerCheck(sessionID, profileID)
	if err != nil {
		// 记录认证失败
		logger.New("CDP").Warn("CDP会话所有权验证失败",
			logger.F("session_id", sessionID),
			logger.F("profile_id", profileID),
			logger.F("operation", "ClearAllCookies"),
			logger.F("error", err.Error()))
		return err
	}

	if err := session.ClearAllCookies(); err != nil {
		return fmt.Errorf("清空 Cookie 失败: %w", err)
	}

	return nil
}
