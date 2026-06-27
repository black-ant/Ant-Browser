# CDP 服务架构设计

## 📐 架构概览

### **现状问题**
```
前端 ──WebSocket──> CDP (Browser)
  ❌ 前端直接管理WebSocket连接
  ❌ 刷新页面数据丢失
  ❌ 多窗口抓包冲突
  ❌ 断线重连逻辑复杂
  ❌ 无法持久化缓存
```

### **目标架构**
```
前端 ──HTTP/SSE──> 后端CDP服务 ──WebSocket──> CDP (Browser/Page)
  ✅ 后端统一管理连接
  ✅ 数据持久化缓存
  ✅ 会话隔离
  ✅ 自动重连
  ✅ HAR/JSON导出
```

---

## 🏗️ 后端设计

### **1. CDP会话管理器**

#### **核心结构**
```go
// CDPSession CDP调试会话
type CDPSession struct {
    SessionID    string                 // 会话ID
    ProfileID    string                 // 窗口ID
    DebugPort    int                    // CDP端口
    TargetID     string                 // 目标ID (browser/page)
    
    // WebSocket连接
    ws           *websocket.Conn
    wsURL        string
    connected    bool
    
    // 数据缓存
    networkRequests  []NetworkRequest   // 网络请求
    consoleLogs      []ConsoleLog       // Console日志
    storageData      map[string]string  // Storage数据
    
    // 请求映射
    requestMap       map[string]*NetworkRequest  // CDP requestId -> Request
    
    // 订阅者（前端SSE连接）
    subscribers      map[string]chan CDPEvent
    
    // 锁
    mu               sync.RWMutex
    
    // 生命周期
    ctx              context.Context
    cancel           context.CancelFunc
    lastActivity     time.Time
    
    // 自动重连
    reconnectAttempts int
    maxReconnects     int
}

// CDPEvent CDP事件
type CDPEvent struct {
    Type      string      `json:"type"`      // network/console/storage/screenshot/error
    Data      interface{} `json:"data"`
    Timestamp time.Time   `json:"timestamp"`
}

// NetworkRequest 网络请求
type NetworkRequest struct {
    RequestID      string            `json:"requestId"`
    URL            string            `json:"url"`
    Method         string            `json:"method"`
    StatusCode     int               `json:"statusCode"`
    StatusText     string            `json:"statusText"`
    Type           string            `json:"type"`
    Timestamp      int64             `json:"timestamp"`
    RequestHeaders map[string]string `json:"requestHeaders"`
    ResponseHeaders map[string]string `json:"responseHeaders"`
    RequestBody    string            `json:"requestBody"`
    ResponseBody   string            `json:"responseBody"`
    Duration       int64             `json:"duration"`
    Size           int64             `json:"size"`
    MimeType       string            `json:"mimeType"`
}

// ConsoleLog Console日志
type ConsoleLog struct {
    ID         string `json:"id"`
    Type       string `json:"type"`  // log/warn/error/info
    Message    string `json:"message"`
    Timestamp  int64  `json:"timestamp"`
    StackTrace string `json:"stackTrace,omitempty"`
}
```

#### **会话管理器**
```go
// CDPManager CDP会话管理器
type CDPManager struct {
    sessions map[string]*CDPSession  // sessionID -> session
    mu       sync.RWMutex
}

func NewCDPManager() *CDPManager {
    return &CDPManager{
        sessions: make(map[string]*CDPSession),
    }
}

// CreateSession 创建CDP会话
func (m *CDPManager) CreateSession(profileID string, debugPort int, targetType string) (*CDPSession, error)

// GetSession 获取会话
func (m *CDPManager) GetSession(sessionID string) (*CDPSession, error)

// CloseSession 关闭会话
func (m *CDPManager) CloseSession(sessionID string) error

// ListSessions 列出所有会话
func (m *CDPManager) ListSessions() []*CDPSession
```

---

### **2. CDP协议处理**

#### **连接初始化**
```go
func (s *CDPSession) Connect() error {
    // 1. 获取CDP端点列表
    targets, err := s.fetchCDPTargets()
    
    // 2. 选择目标（browser或page）
    targetURL := s.selectTarget(targets)
    
    // 3. 建立WebSocket连接
    s.ws, err = websocket.Dial(targetURL, "", origin)
    
    // 4. 启用所需的Domain
    s.enableDomains()
    
    // 5. 开始监听事件
    go s.listenCDPEvents()
    
    return nil
}

func (s *CDPSession) fetchCDPTargets() ([]Target, error) {
    resp, err := http.Get(fmt.Sprintf("http://localhost:%d/json/list", s.DebugPort))
    // 解析返回的targets
    var targets []Target
    json.NewDecoder(resp.Body).Decode(&targets)
    return targets, nil
}

func (s *CDPSession) selectTarget(targets []Target) string {
    // 根据targetType选择
    for _, t := range targets {
        if s.TargetID == "browser" && t.Type == "browser" {
            return t.WebSocketDebuggerURL
        }
        if s.TargetID == "page" && t.Type == "page" {
            return t.WebSocketDebuggerURL
        }
    }
    return targets[0].WebSocketDebuggerURL
}

func (s *CDPSession) enableDomains() error {
    domains := []string{
        "Network",
        "Runtime",
        "Storage",
        "Page",
        "Performance",
    }
    
    for _, domain := range domains {
        s.sendCommand(fmt.Sprintf("%s.enable", domain), nil)
    }
    return nil
}
```

#### **事件监听**
```go
func (s *CDPSession) listenCDPEvents() {
    for {
        select {
        case <-s.ctx.Done():
            return
        default:
            var msg CDPMessage
            if err := websocket.JSON.Receive(s.ws, &msg); err != nil {
                s.handleDisconnect(err)
                return
            }
            s.handleCDPMessage(&msg)
        }
    }
}

func (s *CDPSession) handleCDPMessage(msg *CDPMessage) {
    switch msg.Method {
    case "Network.requestWillBeSent":
        s.handleNetworkRequest(msg.Params)
    case "Network.responseReceived":
        s.handleNetworkResponse(msg.Params)
    case "Network.loadingFinished":
        s.handleNetworkFinished(msg.Params)
    case "Runtime.consoleAPICalled":
        s.handleConsoleLog(msg.Params)
    case "Runtime.exceptionThrown":
        s.handleException(msg.Params)
    case "Storage.domStorageItemAdded":
        s.handleStorageChanged(msg.Params)
    }
}
```

#### **网络请求处理**
```go
func (s *CDPSession) handleNetworkRequest(params map[string]interface{}) {
    requestID := params["requestId"].(string)
    request := params["request"].(map[string]interface{})
    
    req := &NetworkRequest{
        RequestID:      requestID,
        URL:            request["url"].(string),
        Method:         request["method"].(string),
        Type:           params["type"].(string),
        Timestamp:      time.Now().UnixMilli(),
        RequestHeaders: convertHeaders(request["headers"]),
    }
    
    // 如果有POST数据
    if postData, ok := request["postData"]; ok {
        req.RequestBody = postData.(string)
    }
    
    s.mu.Lock()
    s.requestMap[requestID] = req
    s.networkRequests = append(s.networkRequests, *req)
    
    // 限制数量
    if len(s.networkRequests) > 500 {
        s.networkRequests = s.networkRequests[100:]
    }
    s.mu.Unlock()
    
    // 推送事件给订阅者
    s.broadcastEvent(CDPEvent{
        Type: "network.request",
        Data: req,
        Timestamp: time.Now(),
    })
}

func (s *CDPSession) handleNetworkResponse(params map[string]interface{}) {
    requestID := params["requestId"].(string)
    response := params["response"].(map[string]interface{})
    
    s.mu.Lock()
    req, exists := s.requestMap[requestID]
    if exists {
        req.StatusCode = int(response["status"].(float64))
        req.StatusText = response["statusText"].(string)
        req.ResponseHeaders = convertHeaders(response["headers"])
        req.MimeType = response["mimeType"].(string)
        
        // 异步获取响应体
        go s.fetchResponseBody(requestID)
    }
    s.mu.Unlock()
    
    s.broadcastEvent(CDPEvent{
        Type: "network.response",
        Data: req,
        Timestamp: time.Now(),
    })
}

func (s *CDPSession) fetchResponseBody(requestID string) {
    result, err := s.sendCommand("Network.getResponseBody", map[string]interface{}{
        "requestId": requestID,
    })
    
    if err == nil && result != nil {
        body := result["body"].(string)
        base64Encoded := result["base64Encoded"].(bool)
        
        if base64Encoded {
            decoded, _ := base64.StdEncoding.DecodeString(body)
            body = string(decoded)
        }
        
        s.mu.Lock()
        if req, exists := s.requestMap[requestID]; exists {
            req.ResponseBody = body
            req.Size = int64(len(body))
        }
        s.mu.Unlock()
    }
}
```

---

### **3. 数据缓存与查询**

```go
// GetNetworkRequests 获取网络请求
func (s *CDPSession) GetNetworkRequests(filter *NetworkFilter) []NetworkRequest {
    s.mu.RLock()
    defer s.mu.RUnlock()
    
    var filtered []NetworkRequest
    for _, req := range s.networkRequests {
        if filter.Match(&req) {
            filtered = append(filtered, req)
        }
    }
    return filtered
}

// GetConsoleLogs 获取Console日志
func (s *CDPSession) GetConsoleLogs(filter *ConsoleFilter) []ConsoleLog {
    s.mu.RLock()
    defer s.mu.RUnlock()
    
    var filtered []ConsoleLog
    for _, log := range s.consoleLogs {
        if filter.Match(&log) {
            filtered = append(filtered, log)
        }
    }
    return filtered
}

// ClearCache 清空缓存
func (s *CDPSession) ClearCache() {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    s.networkRequests = []NetworkRequest{}
    s.consoleLogs = []ConsoleLog{}
    s.requestMap = make(map[string]*NetworkRequest)
}
```

---

### **4. HAR导出**

```go
// ExportHAR 导出HAR格式
func (s *CDPSession) ExportHAR() ([]byte, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    
    har := HAR{
        Log: HARLog{
            Version: "1.2",
            Creator: HARCreator{
                Name:    "Ant Browser",
                Version: "1.0",
            },
            Pages:   []HARPage{},
            Entries: []HAREntry{},
        },
    }
    
    // 转换请求为HAR格式
    for _, req := range s.networkRequests {
        entry := s.convertToHAREntry(&req)
        har.Log.Entries = append(har.Log.Entries, entry)
    }
    
    return json.MarshalIndent(har, "", "  ")
}

type HAR struct {
    Log HARLog `json:"log"`
}

type HARLog struct {
    Version string      `json:"version"`
    Creator HARCreator  `json:"creator"`
    Pages   []HARPage   `json:"pages"`
    Entries []HAREntry  `json:"entries"`
}

type HAREntry struct {
    StartedDateTime string     `json:"startedDateTime"`
    Time            float64    `json:"time"`
    Request         HARRequest `json:"request"`
    Response        HARResponse `json:"response"`
    Cache           HARCache    `json:"cache"`
    Timings         HARTimings  `json:"timings"`
}
```

---

### **5. 后端API接口**

```go
// CDPSessionCreate 创建CDP会话
func (a *App) CDPSessionCreate(profileID string, targetType string) (string, error)

// CDPSessionClose 关闭CDP会话
func (a *App) CDPSessionClose(sessionID string) error

// CDPGetNetworkRequests 获取网络请求
func (a *App) CDPGetNetworkRequests(sessionID string, filter NetworkFilter) ([]NetworkRequest, error)

// CDPGetConsoleLogs 获取Console日志
func (a *App) CDPGetConsoleLogs(sessionID string, filter ConsoleFilter) ([]ConsoleLog, error)

// CDPExecuteJavaScript 执行JavaScript
func (a *App) CDPExecuteJavaScript(sessionID string, code string) (string, error)

// CDPCaptureScreenshot 截图
func (a *App) CDPCaptureScreenshot(sessionID string) (string, error)

// CDPExportHAR 导出HAR
func (a *App) CDPExportHAR(sessionID string) (string, error)

// CDPClearCache 清空缓存
func (a *App) CDPClearCache(sessionID string) error

// CDPGetStorageData 获取Storage数据
func (a *App) CDPGetStorageData(sessionID string, storageType string) (map[string]string, error)
```

---

## 🌐 前端设计

### **1. 移除WebSocket直连**

#### **旧代码（删除）**
```typescript
const [ws, setWs] = useState<WebSocket | null>(null)

const connectWebSocket = () => {
  const wsUrl = `ws://localhost:${profile.debugPort}/devtools/browser`
  const websocket = new WebSocket(wsUrl)
  // ...
}
```

#### **新代码（使用后端API）**
```typescript
const [sessionId, setSessionId] = useState<string | null>(null)

const startCapture = async () => {
  try {
    // 创建CDP会话
    const sessionId = await CDPSessionCreate(selectedProfileId, 'page')
    setSessionId(sessionId)
    
    // 开始轮询或SSE订阅事件
    subscribeToEvents(sessionId)
  } catch (error) {
    toast.error('启动抓包失败')
  }
}

const subscribeToEvents = (sessionId: string) => {
  // 使用Server-Sent Events订阅实时事件
  const eventSource = new EventSource(`/api/cdp/events/${sessionId}`)
  
  eventSource.onmessage = (event) => {
    const cdpEvent = JSON.parse(event.data)
    handleCDPEvent(cdpEvent)
  }
  
  eventSource.onerror = () => {
    // 自动重连由后端处理
    toast.warn('连接中断，正在重连...')
  }
}

const handleCDPEvent = (event: CDPEvent) => {
  switch (event.type) {
    case 'network.request':
      addNetworkRequest(event.data)
      break
    case 'console.log':
      addConsoleLog(event.data)
      break
  }
}
```

---

### **2. API调用**

```typescript
// frontend/src/modules/browser/api.ts

export async function CDPSessionCreate(profileId: string, targetType: string): Promise<string> {
  const bindings = await getBindings()
  return await bindings.CDPSessionCreate(profileId, targetType)
}

export async function CDPGetNetworkRequests(sessionId: string, filter?: any): Promise<NetworkRequest[]> {
  const bindings = await getBindings()
  return await bindings.CDPGetNetworkRequests(sessionId, filter || {})
}

export async function CDPExportHAR(sessionId: string): Promise<string> {
  const bindings = await getBindings()
  return await bindings.CDPExportHAR(sessionId)
}
```

---

## 🎯 验收标准实现

### **1. 刷新页面数据不丢**
- ✅ 后端缓存所有数据
- ✅ 前端刷新后通过sessionId重新获取

### **2. 多窗口互不干扰**
- ✅ 每个窗口独立的sessionId
- ✅ 会话隔离的数据缓存

### **3. 断线自动重连**
- ✅ 后端自动重连CDP
- ✅ 前端无感知切换

### **4. HAR/JSON导出**
- ✅ 后端统一生成
- ✅ 支持过滤和格式化

---

## 📊 实施步骤

### **Phase 1: 后端CDP服务** (2-3小时)
1. ✅ 创建CDP会话管理器
2. ✅ 实现WebSocket连接和事件监听
3. ✅ 实现网络请求/Console日志缓存
4. ✅ 实现HAR导出

### **Phase 2: 后端API** (1小时)
1. ✅ 创建/关闭会话API
2. ✅ 数据查询API
3. ✅ 命令执行API (JS/截图)
4. ✅ 导出API

### **Phase 3: 前端重构** (2小时)
1. ✅ 移除WebSocket直连代码
2. ✅ 使用后端API
3. ✅ 实现SSE事件订阅（可选）
4. ✅ 更新UI逻辑

### **Phase 4: 测试验证** (1小时)
1. ✅ 多窗口测试
2. ✅ 断线重连测试
3. ✅ 数据持久化测试
4. ✅ HAR导出验证

---

**总计：6-7小时工作量**

准备开始实施！🚀
