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
	ID                   string `json:"id"`
	Type                 string `json:"type"`
	Title                string `json:"title"`
	URL                  string `json:"url"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

// CDPMessage CDP协议消息
type CDPMessage struct {
	ID     int                    `json:"id,omitempty"`
	Method string                 `json:"method,omitempty"`
	Params map[string]interface{} `json:"params,omitempty"`
	Result map[string]interface{} `json:"result,omitempty"`
	Error  *CDPError              `json:"error,omitempty"`
	// SessionID 用于 --remote-debugging-pipe 的 flatten 多路复用：单管道上按
	// sessionId 区分目标。端口/WebSocket 模式不使用该字段（omitempty 不影响旧路径）。
	SessionID string `json:"sessionId,omitempty"`
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
	Timing          *RequestTiming    `json:"timing,omitempty"`     // 详细的 timing 信息
	Truncated       bool              `json:"truncated"`            // 响应体是否被截断
	ParsedData      *ResponseData     `json:"parsedData,omitempty"` // 解析后的结构化数据
}

// RequestTiming 请求详细时间信息（基于 CDP ResourceTiming）
type RequestTiming struct {
	RequestTime       float64 `json:"requestTime"`       // 请求开始时间（秒，相对于导航开始）
	ProxyStart        float64 `json:"proxyStart"`        // 代理协商开始（毫秒）
	ProxyEnd          float64 `json:"proxyEnd"`          // 代理协商结束（毫秒）
	DNSStart          float64 `json:"dnsStart"`          // DNS 查询开始（毫秒）
	DNSEnd            float64 `json:"dnsEnd"`            // DNS 查询结束（毫秒）
	ConnectStart      float64 `json:"connectStart"`      // TCP 连接开始（毫秒）
	ConnectEnd        float64 `json:"connectEnd"`        // TCP 连接结束（毫秒）
	SSLStart          float64 `json:"sslStart"`          // SSL 握手开始（毫秒）
	SSLEnd            float64 `json:"sslEnd"`            // SSL 握手结束（毫秒）
	SendStart         float64 `json:"sendStart"`         // 发送请求开始（毫秒）
	SendEnd           float64 `json:"sendEnd"`           // 发送请求结束（毫秒）
	PushStart         float64 `json:"pushStart"`         // HTTP/2 Server Push 开始（毫秒）
	PushEnd           float64 `json:"pushEnd"`           // HTTP/2 Server Push 结束（毫秒）
	ReceiveHeadersEnd float64 `json:"receiveHeadersEnd"` // 接收响应头结束（毫秒）
}

// ConsoleLog Console日志
type ConsoleLog struct {
	ID         string `json:"id"`
	Type       string `json:"type"` // log/warn/error/info
	Message    string `json:"message"`
	Timestamp  int64  `json:"timestamp"`
	StackTrace string `json:"stackTrace,omitempty"`
}

// WebSocketMessage WebSocket消息
type WebSocketMessage struct {
	ID           string `json:"id"`
	RequestID    string `json:"requestId"`    // WebSocket 连接的请求 ID
	URL          string `json:"url"`          // WebSocket URL
	Direction    string `json:"direction"`    // "send" 或 "receive"
	Timestamp    int64  `json:"timestamp"`    // 时间戳（毫秒）
	Opcode       int    `json:"opcode"`       // 1=text, 2=binary
	Data         string `json:"data"`         // 消息内容
	PayloadSize  int    `json:"payloadSize"`  // 消息大小（字节）
	Masked       bool   `json:"masked"`       // 是否掩码
	ConnectionID string `json:"connectionId"` // 用于分组显示
}

// CDPEvent CDP事件
type CDPEvent struct {
	Type      string      `json:"type"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
}

// CDPSession CDP调试会话
type CDPSession struct {
	SessionID  string
	ProfileID  string
	DebugPort  int
	TargetType string // "browser" or "page"

	// WebSocket连接
	ws        *websocket.Conn
	wsURL     string
	connected bool

	// consoleCaptureEnabled 标记是否已惰性启用 Runtime 域以捕获页面 console.*。
	// Runtime.enable 是最易被检测站点经侧信道探测的 CDP 痕迹，默认不启用，
	// 仅当用户实际查看 DevTools「控制台」时才按需开启（见 EnableConsoleCapture）。
	consoleCaptureEnabled bool

	// 数据缓存
	networkRequests []*NetworkRequest // 改为指针切片，确保后续更新能反映到列表
	consoleLogs     []ConsoleLog
	wsMessages      []WebSocketMessage // WebSocket 消息列表
	storageData     map[string]string

	// 请求映射
	requestMap     map[string]*NetworkRequest
	fetchedBodyMap map[string]bool   // 跟踪已抓取响应体的请求（避免重复抓取）
	wsConnMap      map[string]string // requestID -> WebSocket URL 映射

	// 数据解析器
	parser *DataParser

	// CDP命令
	commandID       int
	pendingCommands map[int]chan *CDPMessage

	// 订阅者
	subscribers map[string]chan CDPEvent

	// 锁
	mu        sync.RWMutex
	commandMu sync.Mutex
	writeMu   sync.Mutex // 串行化 WebSocket 写：gorilla 不允许并发写
	consoleMu sync.Mutex // 串行化 EnableConsoleCapture，保证 Runtime.enable 仅发一次

	// 生命周期
	ctx          context.Context
	cancel       context.CancelFunc
	lastActivity time.Time

	// 自动重连
	reconnectAttempts int
	maxReconnects     int
	maintainOnce      sync.Once // 保证维护连接的 ping goroutine 整个会话仅启动一次（重连不重复启动）

	// 请求拦截
	interceptEnabled bool
	interceptRules   []InterceptRule
	interceptMu      sync.RWMutex
}

// InterceptRule 请求拦截规则
type InterceptRule struct {
	ID             string                `json:"id"`
	Name           string                `json:"name"`
	Enabled        bool                  `json:"enabled"`
	URLPattern     string                `json:"urlPattern"`               // URL 匹配模式（正则表达式）
	Method         string                `json:"method"`                   // HTTP 方法（空表示全部）
	Actions        InterceptActions      `json:"actions"`                  // 拦截动作
	ModifyRequest  *RequestModification  `json:"modifyRequest,omitempty"`  // 修改请求
	ModifyResponse *ResponseModification `json:"modifyResponse,omitempty"` // 修改响应
}

// InterceptActions 拦截动作
type InterceptActions struct {
	Block          bool `json:"block"`          // 阻止请求
	ModifyRequest  bool `json:"modifyRequest"`  // 修改请求
	ModifyResponse bool `json:"modifyResponse"` // 修改响应
	Delay          int  `json:"delay"`          // 延迟（毫秒）
}

// RequestModification 请求修改
type RequestModification struct {
	URL     string            `json:"url"`     // 修改 URL
	Method  string            `json:"method"`  // 修改方法
	Headers map[string]string `json:"headers"` // 修改请求头
	Body    string            `json:"body"`    // 修改请求体
}

// ResponseModification 响应修改
type ResponseModification struct {
	StatusCode int               `json:"statusCode"` // 修改状态码
	Headers    map[string]string `json:"headers"`    // 修改响应头
	Body       string            `json:"body"`       // 修改响应体
}

// NewCDPSession 创建新的CDP会话
func NewCDPSession(profileID string, debugPort int, targetType string) *CDPSession {
	ctx, cancel := context.WithCancel(context.Background())

	return &CDPSession{
		SessionID:       fmt.Sprintf("cdp-%s-%d", profileID, time.Now().UnixNano()),
		ProfileID:       profileID,
		DebugPort:       debugPort,
		TargetType:      targetType,
		networkRequests: []*NetworkRequest{},
		consoleLogs:     []ConsoleLog{},
		wsMessages:      []WebSocketMessage{},
		storageData:     make(map[string]string),
		requestMap:      make(map[string]*NetworkRequest),
		fetchedBodyMap:  make(map[string]bool),
		wsConnMap:       make(map[string]string),
		pendingCommands: make(map[int]chan *CDPMessage),
		subscribers:     make(map[string]chan CDPEvent),
		ctx:             ctx,
		cancel:          cancel,
		lastActivity:    time.Now(),
		maxReconnects:   5,
		parser:          NewDataParser(), // 初始化解析器
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

	// ws 指针与 connected 标志由 s.mu 保护：重连时 Connect 会重新赋值，
	// 而 SendCommand / maintainConnection / handleDisconnect 会并发读取，
	// 不加锁会触发数据竞争（go test -race）。writeMu 仅串行化 WriteJSON 本身，
	// 不保护指针重新赋值。
	s.mu.Lock()
	s.ws = ws
	s.connected = true
	s.mu.Unlock()

	// 先启动事件读循环，再启用 domains：
	// enableDomains 通过 SendCommand 等待命令响应，而响应由 listenEvents 读取分发；
	// 若读循环未先启动，enableDomains 会一直等到超时，导致"连接失败"。
	//
	// listenEvents 绑定到本次建立的具体连接 ws：重连后旧连接的读循环读到错误退出时，
	// 不会误触发对新连接的重连。maintainConnection 整个会话仅启动一次（maintainOnce），
	// 否则每次重连都会多泄漏一个常驻 ping goroutine。
	go s.listenEvents(ws)
	s.maintainOnce.Do(func() {
		go s.maintainConnection()
	})

	// 启用CDP domains
	if err := s.enableDomains(); err != nil {
		// enableDomains 失败时取消会话并关闭连接，避免残留 goroutine 和连接泄露
		s.cancel()
		s.mu.Lock()
		s.connected = false
		s.mu.Unlock()
		ws.Close()
		return fmt.Errorf("启用CDP domains失败: %w", err)
	}

	// 重连场景：若此前已开启控制台捕获，需重新启用 Runtime 域（enableDomains 默认不含 Runtime）
	s.consoleMu.Lock()
	needRuntime := s.consoleCaptureEnabled
	s.consoleMu.Unlock()
	if needRuntime {
		_, _ = s.SendCommand("Runtime.enable", nil)
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

// enableDomains 启用CDP domains。
// 注意：刻意不包含 Runtime —— Runtime.enable 会让页面可经侧信道探测出 CDP 在场
// （反检测最敏感的痕迹）。页面 console 捕获改由 EnableConsoleCapture 按需启用。
// Network/Log/Page 足以支撑被动的网络抓包与浏览器级日志监控。
func (s *CDPSession) enableDomains() error {
	domains := []string{
		"Network",
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

// EnableConsoleCapture 惰性启用 Runtime 域，以捕获页面 console.* 调用与未捕获异常。
// 仅当用户实际查看 DevTools「控制台」时调用，避免 Runtime.enable 这一最易被检测站点
// 探测的 CDP 痕迹长期暴露。重复调用安全（幂等）。
func (s *CDPSession) EnableConsoleCapture() error {
	s.consoleMu.Lock()
	defer s.consoleMu.Unlock()
	if s.consoleCaptureEnabled {
		return nil
	}
	if _, err := s.SendCommand("Runtime.enable", nil); err != nil {
		return err
	}
	s.consoleCaptureEnabled = true
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

	// 在 s.mu 下取当前连接快照，避免与重连时 Connect 的 s.ws 重新赋值竞争；
	// writeMu 再串行化对该连接的 WriteJSON（gorilla 不允许并发写）。
	s.mu.RLock()
	ws := s.ws
	s.mu.RUnlock()
	if ws == nil {
		s.commandMu.Lock()
		delete(s.pendingCommands, id)
		s.commandMu.Unlock()
		return nil, fmt.Errorf("CDP 连接不可用")
	}

	s.writeMu.Lock()
	werr := ws.WriteJSON(msg)
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
