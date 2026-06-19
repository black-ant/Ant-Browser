package cdp

import (
	"encoding/json"
	"fmt"
	"sync"
)

// Manager CDP会话管理器
type Manager struct {
	sessions map[string]*CDPSession
	mu       sync.RWMutex
}

// NewManager 创建CDP管理器
func NewManager() *Manager {
	return &Manager{
		sessions: make(map[string]*CDPSession),
	}
}

// CreateSession 创建CDP会话
func (m *Manager) CreateSession(profileID string, debugPort int, targetType string) (*CDPSession, error) {
	session := NewCDPSession(profileID, debugPort, targetType)

	if err := session.Connect(); err != nil {
		return nil, err
	}

	m.mu.Lock()
	m.sessions[session.SessionID] = session
	m.mu.Unlock()

	return session, nil
}

// GetSession 获取会话
func (m *Manager) GetSession(sessionID string) (*CDPSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("会话不存在: %s", sessionID)
	}

	return session, nil
}

// CloseSession 关闭会话
func (m *Manager) CloseSession(sessionID string) error {
	m.mu.Lock()
	session, exists := m.sessions[sessionID]
	if exists {
		delete(m.sessions, sessionID)
	}
	m.mu.Unlock()

	if !exists {
		return fmt.Errorf("会话不存在: %s", sessionID)
	}

	return session.Close()
}

// ListSessions 列出所有会话
func (m *Manager) ListSessions() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessionIDs := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		sessionIDs = append(sessionIDs, id)
	}

	return sessionIDs
}

// GetNetworkRequests 获取网络请求
func (s *CDPSession) GetNetworkRequests() []NetworkRequest {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 返回副本
	requests := make([]NetworkRequest, len(s.networkRequests))
	copy(requests, s.networkRequests)
	return requests
}

// GetConsoleLogs 获取Console日志
func (s *CDPSession) GetConsoleLogs() []ConsoleLog {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 返回副本
	logs := make([]ConsoleLog, len(s.consoleLogs))
	copy(logs, s.consoleLogs)
	return logs
}

// ClearNetworkRequests 清空网络请求
func (s *CDPSession) ClearNetworkRequests() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.networkRequests = []NetworkRequest{}
	s.requestMap = make(map[string]*NetworkRequest)
}

// ClearConsoleLogs 清空Console日志
func (s *CDPSession) ClearConsoleLogs() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.consoleLogs = []ConsoleLog{}
}

// ExecuteJavaScript 执行JavaScript
func (s *CDPSession) ExecuteJavaScript(code string) (string, error) {
	result, err := s.SendCommand("Runtime.evaluate", map[string]interface{}{
		"expression":    code,
		"returnByValue": true,
	})

	if err != nil {
		return "", err
	}

	// 提取结果
	if resultData, ok := result["result"].(map[string]interface{}); ok {
		if value, ok := resultData["value"]; ok {
			return fmt.Sprintf("%v", value), nil
		}
		if description, ok := resultData["description"].(string); ok {
			return description, nil
		}
	}

	return "", fmt.Errorf("无法提取执行结果")
}

// CaptureScreenshot 截图
func (s *CDPSession) CaptureScreenshot() (string, error) {
	result, err := s.SendCommand("Page.captureScreenshot", map[string]interface{}{
		"format": "png",
	})

	if err != nil {
		return "", err
	}

	// 提取base64图片数据
	if data, ok := result["data"].(string); ok {
		return data, nil
	}

	return "", fmt.Errorf("无法提取截图数据")
}

// GetStorageData 获取Storage数据
func (s *CDPSession) GetStorageData(storageType string) (map[string]string, error) {
	var domain string
	var key string

	switch storageType {
	case "localStorage":
		domain = "local"
		key = "localStorage"
	case "sessionStorage":
		domain = "session"
		key = "sessionStorage"
	default:
		return nil, fmt.Errorf("不支持的存储类型: %s", storageType)
	}

	// 执行JS获取storage
	code := fmt.Sprintf(`
		(() => {
			const data = {};
			for (let i = 0; i < %s.length; i++) {
				const key = %s.key(i);
				data[key] = %s.getItem(key);
			}
			return JSON.stringify(data);
		})()
	`, key, key, key)

	result, err := s.ExecuteJavaScript(code)
	if err != nil {
		return nil, err
	}

	// 解析JSON
	var data map[string]string
	if err := json.Unmarshal([]byte(result), &data); err != nil {
		return nil, fmt.Errorf("解析storage数据失败: %w", err)
	}

	_ = domain // 暂时不用
	return data, nil
}

// GetStatistics 获取统计信息
func (s *CDPSession) GetStatistics() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var totalSize int64
	var totalDuration int64
	var successCount int
	var failedCount int

	for _, req := range s.networkRequests {
		totalSize += req.Size
		totalDuration += req.Duration

		if req.StatusCode >= 200 && req.StatusCode < 400 {
			successCount++
		} else if req.StatusCode >= 400 {
			failedCount++
		}
	}

	avgDuration := int64(0)
	if len(s.networkRequests) > 0 {
		avgDuration = totalDuration / int64(len(s.networkRequests))
	}

	return map[string]interface{}{
		"total":       len(s.networkRequests),
		"success":     successCount,
		"failed":      failedCount,
		"totalSize":   totalSize,
		"avgDuration": avgDuration,
		"consoleLogs": len(s.consoleLogs),
	}
}
