package cdp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Target CDP目标
type Target struct {
	ID                      string `json:"id"`
	Type                    string `json:"type"`
	Title                   string `json:"title"`
	URL                     string `json:"url"`
	WebSocketDebuggerURL    string `json:"webSocketDebuggerUrl"`
}

// CDPMessage CDP协议消息
type CDPMessage struct {
	ID     int                    `json:"id,omitempty"`
	Method string                 `json:"method,omitempty"`
	Params map[string]interface{} `json:"params,omitempty"`
	Result map[string]interface{} `json:"result,omitempty"`
	Error  *CDPError              `json:"error,omitempty"`
}

// CDPError CDP错误
type CDPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// NetworkRequest 网络请求
type NetworkRequest struct {
	RequestID       string            `json:"requestId"`
	URL             string            `json:"url"`
	Method          string            `json:"method"`
	StatusCode      int               `json:"statusCode"`
	StatusText      string            `json:"statusText"`
	Type            string            `json:"type"`
	Timestamp       int64             `json:"timestamp"`
	RequestHeaders  map[string]string `json:"requestHeaders"`
	ResponseHeaders map[string]string `json:"responseHeaders"`
	RequestBody     string            `json:"requestBody"`
	ResponseBody    string            `json:"responseBody"`
	Duration        int64             `json:"duration"`
	Size            int64             `json:"size"`
	MimeType        string            `json:"mimeType"`
	StartTime       int64             `json:"startTime"`
	EndTime         int64             `json:"endTime"`
}

// ConsoleLog Console日志
type ConsoleLog struct {
	ID         string `json:"id"`
	Type       string `json:"type"` // log/warn/error/info
	Message    string `json:"message"`
	Timestamp  int64  `json:"timestamp"`
	StackTrace string `json:"stackTrace,omitempty"`
}

// CDPEvent CDP事件
type CDPEvent struct {
	Type      string      `json:"type"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
}

// CDPSession CDP调试会话
type CDPSession struct {
	SessionID   string
	ProfileID   string
	DebugPort   int
	TargetType  string // "browser" or "page"

	// WebSocket连接
	ws        *websocket.Conn
	wsURL     string
	connected bool

	// 数据缓存
	networkRequests []NetworkRequest
	consoleLogs     []ConsoleLog
	storageData     map[string]string

	// 请求映射
	requestMap map[string]*NetworkRequest

	// CDP命令
	commandID      int
	pendingCommands map[int]chan *CDPMessage

	// 订阅者
	subscribers map[string]chan CDPEvent

	// 锁
	mu           sync.RWMutex
	commandMu    sync.Mutex
	writeMu      sync.Mutex // 串行化 WebSocket 写：gorilla 不允许并发写

	// 生命周期
	ctx          context.Context
	cancel       context.CancelFunc
	lastActivity time.Time

	// 自动重连
	reconnectAttempts int
	maxReconnects     int
}

// NewCDPSession 创建新的CDP会话
func NewCDPSession(profileID string, debugPort int, targetType string) *CDPSession {
	ctx, cancel := context.WithCancel(context.Background())

	return &CDPSession{
		SessionID:       fmt.Sprintf("cdp-%s-%d", profileID, time.Now().UnixNano()),
		ProfileID:       profileID,
		DebugPort:       debugPort,
		TargetType:      targetType,
		networkRequests: []NetworkRequest{},
		consoleLogs:     []ConsoleLog{},
		storageData:     make(map[string]string),
		requestMap:      make(map[string]*NetworkRequest),
		pendingCommands: make(map[int]chan *CDPMessage),
		subscribers:     make(map[string]chan CDPEvent),
		ctx:             ctx,
		cancel:          cancel,
		lastActivity:    time.Now(),
		maxReconnects:   5,
	}
}

// Connect 连接到CDP
func (s *CDPSession) Connect() error {
	// 1. 获取可用的targets
	targets, err := s.fetchTargets()
	if err != nil {
		return fmt.Errorf("获取CDP targets失败: %w", err)
	}

	// 2. 选择目标
	wsURL := s.selectTarget(targets)
	if wsURL == "" {
		return fmt.Errorf("未找到合适的CDP target")
	}

	s.wsURL = wsURL

	// 3. 建立WebSocket连接
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	ws, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("连接CDP WebSocket失败: %w", err)
	}

	s.ws = ws
	s.connected = true

	// 先启动事件读循环，再启用 domains：
	// enableDomains 通过 SendCommand 等待命令响应，而响应由 listenEvents 读取分发；
	// 若读循环未先启动，enableDomains 会一直等到超时，导致"连接失败"。
	go s.listenEvents()
	go s.maintainConnection()

	// 启用CDP domains
	if err := s.enableDomains(); err != nil {
		s.ws.Close()
		return fmt.Errorf("启用CDP domains失败: %w", err)
	}

	return nil
}

// fetchTargets 获取CDP targets
func (s *CDPSession) fetchTargets() ([]Target, error) {
	url := fmt.Sprintf("http://localhost:%d/json/list", s.DebugPort)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var targets []Target
	if err := json.NewDecoder(resp.Body).Decode(&targets); err != nil {
		return nil, err
	}

	return targets, nil
}

// selectTarget 选择目标
func (s *CDPSession) selectTarget(targets []Target) string {
	for _, t := range targets {
		if s.TargetType == "browser" && t.Type == "browser" {
			return t.WebSocketDebuggerURL
		}
		if s.TargetType == "page" && t.Type == "page" {
			return t.WebSocketDebuggerURL
		}
	}

	// 默认返回第一个
	if len(targets) > 0 {
		return targets[0].WebSocketDebuggerURL
	}

	return ""
}

// enableDomains 启用CDP domains
func (s *CDPSession) enableDomains() error {
	domains := []string{
		"Network",
		"Runtime",
		"Log",
		"Page",
	}

	for _, domain := range domains {
		_, err := s.SendCommand(fmt.Sprintf("%s.enable", domain), nil)
		if err != nil {
			return fmt.Errorf("启用 %s domain失败: %w", domain, err)
		}
	}

	return nil
}

// SendCommand 发送CDP命令
func (s *CDPSession) SendCommand(method string, params map[string]interface{}) (map[string]interface{}, error) {
	s.commandMu.Lock()
	s.commandID++
	id := s.commandID
	resultChan := make(chan *CDPMessage, 1)
	s.pendingCommands[id] = resultChan
	s.commandMu.Unlock()

	msg := CDPMessage{
		ID:     id,
		Method: method,
		Params: params,
	}

	s.writeMu.Lock()
	werr := s.ws.WriteJSON(msg)
	s.writeMu.Unlock()
	if werr != nil {
		s.commandMu.Lock()
		delete(s.pendingCommands, id)
		s.commandMu.Unlock()
		return nil, werr
	}

	// 等待响应（超时10秒）
	select {
	case result := <-resultChan:
		if result.Error != nil {
			return nil, fmt.Errorf("CDP错误: %s", result.Error.Message)
		}
		return result.Result, nil
	case <-time.After(10 * time.Second):
		s.commandMu.Lock()
		delete(s.pendingCommands, id)
		s.commandMu.Unlock()
		return nil, fmt.Errorf("命令超时")
	}
}

// Close 关闭会话
func (s *CDPSession) Close() error {
	s.cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	// 关闭所有订阅者
	for _, ch := range s.subscribers {
		close(ch)
	}
	s.subscribers = make(map[string]chan CDPEvent)

	if s.ws != nil {
		return s.ws.Close()
	}

	return nil
}

// Subscribe 订阅事件
func (s *CDPSession) Subscribe(subscriberID string) <-chan CDPEvent {
	s.mu.Lock()
	defer s.mu.Unlock()

	ch := make(chan CDPEvent, 100)
	s.subscribers[subscriberID] = ch
	return ch
}

// Unsubscribe 取消订阅
func (s *CDPSession) Unsubscribe(subscriberID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ch, exists := s.subscribers[subscriberID]; exists {
		close(ch)
		delete(s.subscribers, subscriberID)
	}
}

// broadcastEvent 广播事件给所有订阅者
func (s *CDPSession) broadcastEvent(event CDPEvent) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, ch := range s.subscribers {
		select {
		case ch <- event:
		default:
			// 通道满了，跳过
		}
	}
}
