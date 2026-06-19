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
		return "", fmt.Errorf("创建CDP会话失败: %w", err)
	}

	logger.New("CDP").Info("[CDP] 会话已创建", logger.F("session_id", session.SessionID), logger.F("profile_id", profileID))

	return session.SessionID, nil
}

// CDPSessionClose 关闭CDP会话
func (a *App) CDPSessionClose(sessionID string) error {
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

// CDPListSessions 列出所有会话
func (a *App) CDPListSessions() ([]string, error) {
	return a.cdpManager.ListSessions(), nil
}
