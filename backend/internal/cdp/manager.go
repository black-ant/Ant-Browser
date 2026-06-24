package cdp

import (
	"encoding/json"
	"fmt"
	"sync"
)

// Manager CDP会话管理器
type Manager struct {
	sessions      map[string]*CDPSession
	sessionOwners map[string]string // sessionID -> profileID，用于会话所有权验证
	mu            sync.RWMutex
}

// NewManager 创建CDP管理器
func NewManager() *Manager {
	return &Manager{
		sessions:      make(map[string]*CDPSession),
		sessionOwners: make(map[string]string),
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
	m.sessionOwners[session.SessionID] = profileID
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

// GetSessionWithOwnerCheck 获取会话并验证所有权
func (m *Manager) GetSessionWithOwnerCheck(sessionID string, profileID string) (*CDPSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("会话不存在: %s", sessionID)
	}

	owner, hasOwner := m.sessionOwners[sessionID]
	if !hasOwner || owner != profileID {
		return nil, fmt.Errorf("无权访问该会话")
	}

	return session, nil
}

// CloseSession 关闭会话
func (m *Manager) CloseSession(sessionID string) error {
	m.mu.Lock()
	session, exists := m.sessions[sessionID]
	if exists {
		delete(m.sessions, sessionID)
		delete(m.sessionOwners, sessionID)
	}
	m.mu.Unlock()

	if !exists {
		return fmt.Errorf("会话不存在: %s", sessionID)
	}

	return session.Close()
}

// CloseSessionsByProfile 关闭指定 profileID 的所有会话（实例停止时调用）
func (m *Manager) CloseSessionsByProfile(profileID string) {
	m.mu.Lock()
	toClose := make([]*CDPSession, 0)
	for id, session := range m.sessions {
		if session.ProfileID == profileID {
			toClose = append(toClose, session)
			delete(m.sessions, id)
			delete(m.sessionOwners, id)
		}
	}
	m.mu.Unlock()

	for _, session := range toClose {
		_ = session.Close()
	}
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

	// 返回值拷贝（解引用指针）
	requests := make([]NetworkRequest, len(s.networkRequests))
	for i, req := range s.networkRequests {
		if req != nil {
			requests[i] = *req
		}
	}
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

	s.networkRequests = []*NetworkRequest{}
	s.requestMap = make(map[string]*NetworkRequest)
	s.fetchedBodyMap = make(map[string]bool)
}

// ClearConsoleLogs 清空Console日志
func (s *CDPSession) ClearConsoleLogs() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.consoleLogs = []ConsoleLog{}
}

// GetWebSocketMessages 获取WebSocket消息
func (s *CDPSession) GetWebSocketMessages() []WebSocketMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 返回副本
	messages := make([]WebSocketMessage, len(s.wsMessages))
	copy(messages, s.wsMessages)
	return messages
}

// ClearWebSocketMessages 清空WebSocket消息
func (s *CDPSession) ClearWebSocketMessages() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.wsMessages = []WebSocketMessage{}
	s.wsConnMap = make(map[string]string)
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
		if req == nil {
			continue
		}
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

// Cookie CDP Cookie 结构
type Cookie struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	Expires  float64 `json:"expires"` // Unix timestamp
	Size     int     `json:"size"`
	HTTPOnly bool    `json:"httpOnly"`
	Secure   bool    `json:"secure"`
	Session  bool    `json:"session"`
	SameSite string  `json:"sameSite"` // Strict, Lax, None
}

// GetCookies 获取所有 Cookie
func (s *CDPSession) GetCookies() ([]Cookie, error) {
	result, err := s.SendCommand("Network.getAllCookies", nil)
	if err != nil {
		return nil, fmt.Errorf("获取 Cookie 失败: %w", err)
	}

	cookiesData, ok := result["cookies"].([]interface{})
	if !ok {
		return []Cookie{}, nil
	}

	cookies := make([]Cookie, 0, len(cookiesData))
	for _, c := range cookiesData {
		cookieMap, ok := c.(map[string]interface{})
		if !ok {
			continue
		}

		cookie := Cookie{
			Name:     getString(cookieMap, "name"),
			Value:    getString(cookieMap, "value"),
			Domain:   getString(cookieMap, "domain"),
			Path:     getString(cookieMap, "path"),
			Expires:  getFloat(cookieMap, "expires"),
			Size:     getInt(cookieMap, "size"),
			HTTPOnly: getBool(cookieMap, "httpOnly"),
			Secure:   getBool(cookieMap, "secure"),
			Session:  getBool(cookieMap, "session"),
			SameSite: getString(cookieMap, "sameSite"),
		}
		cookies = append(cookies, cookie)
	}

	return cookies, nil
}

// SetCookie 设置 Cookie
func (s *CDPSession) SetCookie(cookie Cookie) error {
	params := map[string]interface{}{
		"name":   cookie.Name,
		"value":  cookie.Value,
		"domain": cookie.Domain,
		"path":   cookie.Path,
	}

	if cookie.Expires > 0 {
		params["expires"] = cookie.Expires
	}
	if cookie.HTTPOnly {
		params["httpOnly"] = true
	}
	if cookie.Secure {
		params["secure"] = true
	}
	if cookie.SameSite != "" {
		params["sameSite"] = cookie.SameSite
	}

	_, err := s.SendCommand("Network.setCookie", params)
	if err != nil {
		return fmt.Errorf("设置 Cookie 失败: %w", err)
	}

	return nil
}

// DeleteCookie 删除 Cookie
func (s *CDPSession) DeleteCookie(name, domain, path string) error {
	params := map[string]interface{}{
		"name": name,
	}
	if domain != "" {
		params["domain"] = domain
	}
	if path != "" {
		params["path"] = path
	}

	_, err := s.SendCommand("Network.deleteCookies", params)
	if err != nil {
		return fmt.Errorf("删除 Cookie 失败: %w", err)
	}

	return nil
}

// ClearAllCookies 清空所有 Cookie
func (s *CDPSession) ClearAllCookies() error {
	_, err := s.SendCommand("Network.clearBrowserCookies", nil)
	if err != nil {
		return fmt.Errorf("清空 Cookie 失败: %w", err)
	}

	return nil
}

// 辅助函数
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getFloat(m map[string]interface{}, key string) float64 {
	if v, ok := m[key].(float64); ok {
		return v
	}
	return 0
}

func getInt(m map[string]interface{}, key string) int {
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	return 0
}

func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

// EnableIntercept 启用请求拦截
func (s *CDPSession) EnableIntercept() error {
	s.interceptMu.Lock()
	defer s.interceptMu.Unlock()

	if s.interceptEnabled {
		return nil // 已启用
	}

	// 启用 Fetch 域
	_, err := s.SendCommand("Fetch.enable", map[string]interface{}{
		"patterns": []map[string]interface{}{
			{"urlPattern": "*", "requestStage": "Request"},
		},
	})
	if err != nil {
		return fmt.Errorf("启用 Fetch 域失败: %w", err)
	}

	s.interceptEnabled = true
	return nil
}

// DisableIntercept 禁用请求拦截
func (s *CDPSession) DisableIntercept() error {
	s.interceptMu.Lock()
	defer s.interceptMu.Unlock()

	if !s.interceptEnabled {
		return nil
	}

	_, err := s.SendCommand("Fetch.disable", nil)
	if err != nil {
		return fmt.Errorf("禁用 Fetch 域失败: %w", err)
	}

	s.interceptEnabled = false
	return nil
}

// AddInterceptRule 添加拦截规则
func (s *CDPSession) AddInterceptRule(rule InterceptRule) error {
	s.interceptMu.Lock()
	defer s.interceptMu.Unlock()

	s.interceptRules = append(s.interceptRules, rule)
	return nil
}

// RemoveInterceptRule 删除拦截规则
func (s *CDPSession) RemoveInterceptRule(ruleID string) error {
	s.interceptMu.Lock()
	defer s.interceptMu.Unlock()

	for i, rule := range s.interceptRules {
		if rule.ID == ruleID {
			s.interceptRules = append(s.interceptRules[:i], s.interceptRules[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("规则不存在: %s", ruleID)
}

// GetInterceptRules 获取拦截规则列表
func (s *CDPSession) GetInterceptRules() []InterceptRule {
	s.interceptMu.RLock()
	defer s.interceptMu.RUnlock()

	rules := make([]InterceptRule, len(s.interceptRules))
	copy(rules, s.interceptRules)
	return rules
}

// UpdateInterceptRule 更新拦截规则
func (s *CDPSession) UpdateInterceptRule(rule InterceptRule) error {
	s.interceptMu.Lock()
	defer s.interceptMu.Unlock()

	for i, r := range s.interceptRules {
		if r.ID == rule.ID {
			s.interceptRules[i] = rule
			return nil
		}
	}

	return fmt.Errorf("规则不存在: %s", rule.ID)
}
