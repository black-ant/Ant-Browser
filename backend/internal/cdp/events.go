package cdp

import (
	"encoding/base64"
	"fmt"
	"time"

	"ant-chrome/backend/internal/logger"

	"github.com/gorilla/websocket"
)

// listenEvents 监听CDP事件。
// conn 为本次连接建立时绑定的具体 WebSocket：重连后 Connect 会用新连接再启动一个
// listenEvents，旧连接读到错误后只让自己退出，不会误把新连接也判定为断线。
func (s *CDPSession) listenEvents(conn *websocket.Conn) {
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			var msg CDPMessage
			if err := conn.ReadJSON(&msg); err != nil {
				s.handleDisconnect(conn, err)
				return
			}

			s.lastActivity = time.Now()
			s.handleMessage(&msg)
		}
	}
}

// handleMessage 处理CDP消息
func (s *CDPSession) handleMessage(msg *CDPMessage) {
	// 如果是命令响应
	if msg.ID > 0 {
		s.commandMu.Lock()
		if ch, exists := s.pendingCommands[msg.ID]; exists {
			ch <- msg
			delete(s.pendingCommands, msg.ID)
		}
		s.commandMu.Unlock()
		return
	}

	// 如果是事件
	if msg.Method != "" {
		s.handleEvent(msg)
	}
}

// handleEvent 处理CDP事件
func (s *CDPSession) handleEvent(msg *CDPMessage) {
	switch msg.Method {
	case "Network.requestWillBeSent":
		s.handleNetworkRequestWillBeSent(msg.Params)
	case "Network.responseReceived":
		s.handleNetworkResponseReceived(msg.Params)
	case "Network.loadingFinished":
		s.handleNetworkLoadingFinished(msg.Params)
	case "Network.loadingFailed":
		s.handleNetworkLoadingFailed(msg.Params)
	case "Network.webSocketCreated":
		s.handleWebSocketCreated(msg.Params)
	case "Network.webSocketFrameSent":
		s.handleWebSocketFrameSent(msg.Params)
	case "Network.webSocketFrameReceived":
		s.handleWebSocketFrameReceived(msg.Params)
	case "Network.webSocketClosed":
		s.handleWebSocketClosed(msg.Params)
	case "Runtime.consoleAPICalled":
		s.handleConsoleAPICalled(msg.Params)
	case "Runtime.exceptionThrown":
		s.handleExceptionThrown(msg.Params)
	case "Log.entryAdded":
		s.handleLogEntryAdded(msg.Params)
	case "Fetch.requestPaused":
		// 异步处理，避免阻塞读循环（因为处理中会调用 SendCommand 等待响应）
		go s.handleFetchRequestPaused(msg.Params)
	}
}

// handleNetworkRequestWillBeSent 处理网络请求开始
func (s *CDPSession) handleNetworkRequestWillBeSent(params map[string]interface{}) {
	requestID, _ := params["requestId"].(string)
	request, _ := params["request"].(map[string]interface{})
	reqType, _ := params["type"].(string)
	timestamp, _ := params["timestamp"].(float64)

	url, _ := request["url"].(string)
	method, _ := request["method"].(string)
	headers, _ := request["headers"].(map[string]interface{})

	req := &NetworkRequest{
		RequestID:      requestID,
		URL:            url,
		Method:         method,
		Type:           reqType,
		Timestamp:      int64(timestamp * 1000),
		StartTime:      int64(timestamp * 1000),
		RequestHeaders: convertHeaders(headers),
	}

	// 如果有POST数据
	if postData, ok := request["postData"].(string); ok {
		req.RequestBody = postData
	}

	s.mu.Lock()
	s.requestMap[requestID] = req
	s.networkRequests = append(s.networkRequests, req)

	// 限制缓存大小：超过 500 时裁剪前 100 个，同时从 requestMap 和 fetchedBodyMap 中删除
	if len(s.networkRequests) > 500 {
		// 遍历被移除的请求，从 map 中删除
		for _, removedReq := range s.networkRequests[:100] {
			if removedReq != nil {
				delete(s.requestMap, removedReq.RequestID)
				delete(s.fetchedBodyMap, removedReq.RequestID)
			}
		}
		s.networkRequests = s.networkRequests[100:]
	}
	s.mu.Unlock()

	// 广播事件
	s.broadcastEvent(CDPEvent{
		Type:      "network.request",
		Data:      req,
		Timestamp: time.Now(),
	})
}

// handleNetworkResponseReceived 处理网络响应接收
func (s *CDPSession) handleNetworkResponseReceived(params map[string]interface{}) {
	requestID, _ := params["requestId"].(string)
	response, _ := params["response"].(map[string]interface{})

	s.mu.Lock()
	req, exists := s.requestMap[requestID]
	if exists {
		statusCode, _ := response["status"].(float64)
		statusText, _ := response["statusText"].(string)
		headers, _ := response["headers"].(map[string]interface{})
		mimeType, _ := response["mimeType"].(string)
		timing, _ := response["timing"].(map[string]interface{})

		req.StatusCode = int(statusCode)
		req.StatusText = statusText
		req.ResponseHeaders = convertHeaders(headers)
		req.MimeType = mimeType

		// 解析详细的 timing 信息
		if timing != nil {
			req.Timing = &RequestTiming{
				RequestTime:       getFloat64(timing, "requestTime"),
				ProxyStart:        getFloat64(timing, "proxyStart"),
				ProxyEnd:          getFloat64(timing, "proxyEnd"),
				DNSStart:          getFloat64(timing, "dnsStart"),
				DNSEnd:            getFloat64(timing, "dnsEnd"),
				ConnectStart:      getFloat64(timing, "connectStart"),
				ConnectEnd:        getFloat64(timing, "connectEnd"),
				SSLStart:          getFloat64(timing, "sslStart"),
				SSLEnd:            getFloat64(timing, "sslEnd"),
				SendStart:         getFloat64(timing, "sendStart"),
				SendEnd:           getFloat64(timing, "sendEnd"),
				PushStart:         getFloat64(timing, "pushStart"),
				PushEnd:           getFloat64(timing, "pushEnd"),
				ReceiveHeadersEnd: getFloat64(timing, "receiveHeadersEnd"),
			}
		}
	}
	s.mu.Unlock()

	if exists {
		// 广播事件
		s.broadcastEvent(CDPEvent{
			Type:      "network.response",
			Data:      req,
			Timestamp: time.Now(),
		})
	}
}

// getFloat64 辅助函数：从 map 中安全获取 float64
func getFloat64(m map[string]interface{}, key string) float64 {
	if v, ok := m[key].(float64); ok {
		return v
	}
	return -1 // -1 表示该阶段不存在
}

// handleNetworkLoadingFinished 处理网络加载完成
func (s *CDPSession) handleNetworkLoadingFinished(params map[string]interface{}) {
	requestID, _ := params["requestId"].(string)
	timestamp, _ := params["timestamp"].(float64)

	s.mu.Lock()
	req, exists := s.requestMap[requestID]
	if exists {
		req.EndTime = int64(timestamp * 1000)
		req.Duration = req.EndTime - req.StartTime
	}
	s.mu.Unlock()

	if exists {
		// 在 loadingFinished 时异步获取响应体（此时响应已完整）
		go s.fetchResponseBody(requestID)

		s.broadcastEvent(CDPEvent{
			Type:      "network.finished",
			Data:      req,
			Timestamp: time.Now(),
		})
	}
}

// handleNetworkLoadingFailed 处理网络加载失败
func (s *CDPSession) handleNetworkLoadingFailed(params map[string]interface{}) {
	requestID, _ := params["requestId"].(string)
	errorText, _ := params["errorText"].(string)

	s.mu.Lock()
	req, exists := s.requestMap[requestID]
	if exists {
		req.StatusCode = 0
		req.StatusText = errorText
	}
	s.mu.Unlock()

	if exists {
		s.broadcastEvent(CDPEvent{
			Type:      "network.failed",
			Data:      req,
			Timestamp: time.Now(),
		})
	}
}

// fetchResponseBody 获取响应体
func (s *CDPSession) fetchResponseBody(requestID string) {
	// 检查是否已抓取，避免重复
	s.mu.Lock()
	if s.fetchedBodyMap[requestID] {
		s.mu.Unlock()
		return
	}
	s.fetchedBodyMap[requestID] = true
	s.mu.Unlock()

	result, err := s.SendCommand("Network.getResponseBody", map[string]interface{}{
		"requestId": requestID,
	})

	if err != nil {
		return
	}

	body, _ := result["body"].(string)
	base64Encoded, _ := result["base64Encoded"].(bool)

	if base64Encoded {
		decoded, err := base64.StdEncoding.DecodeString(body)
		if err == nil {
			body = string(decoded)
		}
	}

	// 限制响应体大小：避免超大响应（如视频/大文件）导致内存溢出
	// 超过 10MB 则截断并添加提示
	const maxBodySize = 10 * 1024 * 1024 // 10MB
	originalSize := len(body)
	if originalSize > maxBodySize {
		body = body[:maxBodySize] + fmt.Sprintf("\n\n[响应体过大，已截断至 %d 字节，原始大小: %d 字节]", maxBodySize, originalSize)
	}

	s.mu.Lock()
	if req, exists := s.requestMap[requestID]; exists {
		req.ResponseBody = body
		req.Size = int64(originalSize) // 使用原始大小
		req.Truncated = originalSize > maxBodySize

		// 🆕 自动解析响应体
		if s.parser != nil && body != "" {
			contentType := req.MimeType
			if contentType == "" {
				// 从响应头获取 Content-Type
				if ct, ok := req.ResponseHeaders["content-type"]; ok {
					contentType = ct
				} else if ct, ok := req.ResponseHeaders["Content-Type"]; ok {
					contentType = ct
				}
			}
			req.ParsedData = s.parser.Parse(body, contentType, req.URL)
		}
	}
	s.mu.Unlock()
}

// handleConsoleAPICalled 处理Console API调用
func (s *CDPSession) handleConsoleAPICalled(params map[string]interface{}) {
	consoleType, _ := params["type"].(string)
	args, _ := params["args"].([]interface{})
	timestamp, _ := params["timestamp"].(float64)

	// 提取消息
	var message string
	for i, arg := range args {
		argMap, _ := arg.(map[string]interface{})
		value, _ := argMap["value"]
		if i > 0 {
			message += " "
		}
		message += fmt.Sprintf("%v", value)
	}

	log := ConsoleLog{
		ID:        fmt.Sprintf("console-%d", time.Now().UnixNano()),
		Type:      consoleType,
		Message:   message,
		Timestamp: int64(timestamp * 1000),
	}

	s.mu.Lock()
	s.consoleLogs = append(s.consoleLogs, log)
	s.trimConsoleLogsLocked()
	s.mu.Unlock()

	// 广播事件
	s.broadcastEvent(CDPEvent{
		Type:      "console.log",
		Data:      log,
		Timestamp: time.Now(),
	})
}

// handleExceptionThrown 处理异常抛出
func (s *CDPSession) handleExceptionThrown(params map[string]interface{}) {
	exceptionDetails, _ := params["exceptionDetails"].(map[string]interface{})
	exception, _ := exceptionDetails["exception"].(map[string]interface{})

	message, _ := exception["description"].(string)
	if message == "" {
		message, _ = exception["value"].(string)
	}

	// 提取堆栈信息
	var stackTrace string
	if stackTraceData, ok := exceptionDetails["stackTrace"].(map[string]interface{}); ok {
		if callFrames, ok := stackTraceData["callFrames"].([]interface{}); ok {
			for _, frame := range callFrames {
				frameMap, _ := frame.(map[string]interface{})
				functionName, _ := frameMap["functionName"].(string)
				url, _ := frameMap["url"].(string)
				lineNumber, _ := frameMap["lineNumber"].(float64)
				stackTrace += fmt.Sprintf("\n  at %s (%s:%d)", functionName, url, int(lineNumber))
			}
		}
	}

	timestamp, _ := exceptionDetails["timestamp"].(float64)

	log := ConsoleLog{
		ID:         fmt.Sprintf("error-%d", time.Now().UnixNano()),
		Type:       "error",
		Message:    message,
		Timestamp:  int64(timestamp * 1000),
		StackTrace: stackTrace,
	}

	s.mu.Lock()
	s.consoleLogs = append(s.consoleLogs, log)
	s.trimConsoleLogsLocked()
	s.mu.Unlock()

	// 广播事件
	s.broadcastEvent(CDPEvent{
		Type:      "console.error",
		Data:      log,
		Timestamp: time.Now(),
	})
}

// handleLogEntryAdded 处理日志条目添加
func (s *CDPSession) handleLogEntryAdded(params map[string]interface{}) {
	entry, _ := params["entry"].(map[string]interface{})

	level, _ := entry["level"].(string)
	text, _ := entry["text"].(string)
	timestamp, _ := entry["timestamp"].(float64)

	// 映射日志级别
	logType := "log"
	switch level {
	case "warning":
		logType = "warn"
	case "error":
		logType = "error"
	case "info":
		logType = "info"
	}

	log := ConsoleLog{
		ID:        fmt.Sprintf("log-%d", time.Now().UnixNano()),
		Type:      logType,
		Message:   text,
		Timestamp: int64(timestamp * 1000),
	}

	s.mu.Lock()
	s.consoleLogs = append(s.consoleLogs, log)
	s.trimConsoleLogsLocked()
	s.mu.Unlock()

	// 广播事件
	s.broadcastEvent(CDPEvent{
		Type:      "console.log",
		Data:      log,
		Timestamp: time.Now(),
	})
}

func (s *CDPSession) trimConsoleLogsLocked() {
	if len(s.consoleLogs) > 1000 {
		s.consoleLogs = s.consoleLogs[200:]
	}
}

// convertHeaders 转换headers
func convertHeaders(headers map[string]interface{}) map[string]string {
	result := make(map[string]string)
	for k, v := range headers {
		result[k] = fmt.Sprintf("%v", v)
	}
	return result
}

// handleDisconnect 处理断线（基于 reconnectAttempts/maxReconnects 自动重连）。
//
// conn 为触发本次断线的具体连接。重连后旧连接的读循环可能晚一步读到错误并调用本函数，
// 此时 s.ws 已指向新连接：必须忽略这种来自陈旧连接的断线，否则会把刚建好的新连接误判为
// 断线并触发又一次重连，形成重连风暴。conn 为 nil 时（如 maintainConnection 的 ping 失败）
// 不做陈旧性判断，按当前连接处理。
func (s *CDPSession) handleDisconnect(conn *websocket.Conn, err error) {
	// connected / ws 均由 s.mu 保护，避免与 Connect 的重新赋值竞争
	s.mu.Lock()
	if conn != nil && s.ws != conn {
		// 来自已被替换的旧连接，忽略
		s.mu.Unlock()
		return
	}
	wasConnected := s.connected
	s.connected = false
	s.mu.Unlock()

	if !wasConnected {
		return // 已处理过断线
	}

	// 广播断线事件
	s.broadcastEvent(CDPEvent{
		Type:      "connection.closed",
		Data:      map[string]interface{}{"error": err.Error()},
		Timestamp: time.Now(),
	})

	// 尝试重连
	if s.reconnectAttempts < s.maxReconnects {
		s.reconnectAttempts++
		time.Sleep(time.Duration(s.reconnectAttempts) * time.Second)

		if err := s.Connect(); err == nil {
			s.reconnectAttempts = 0
			s.broadcastEvent(CDPEvent{
				Type:      "connection.reconnected",
				Data:      nil,
				Timestamp: time.Now(),
			})
		}
	}
}

// maintainConnection 维护连接
func (s *CDPSession) maintainConnection() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			// 发送ping保持连接（使用锁保护 connected / ws 访问）
			s.mu.RLock()
			isConnected := s.connected
			conn := s.ws
			s.mu.RUnlock()

			if isConnected {
				_, err := s.SendCommand("Browser.getVersion", nil)
				if err != nil {
					// 传入 ping 时的连接：若期间已重连，handleDisconnect 会识别为
					// 陈旧连接并忽略，不会误判新连接断线。
					s.handleDisconnect(conn, err)
				}
			}
		}
	}
}

// handleWebSocketCreated 处理 WebSocket 创建
func (s *CDPSession) handleWebSocketCreated(params map[string]interface{}) {
	requestID, _ := params["requestId"].(string)
	url, _ := params["url"].(string)

	s.mu.Lock()
	s.wsConnMap[requestID] = url
	s.mu.Unlock()
}

// handleWebSocketFrameSent 处理 WebSocket 发送消息
func (s *CDPSession) handleWebSocketFrameSent(params map[string]interface{}) {
	requestID, _ := params["requestId"].(string)
	timestamp, _ := params["timestamp"].(float64)
	response, _ := params["response"].(map[string]interface{})

	opcode, _ := response["opcode"].(float64)
	payloadData, _ := response["payloadData"].(string)
	masked, _ := response["mask"].(bool)

	s.mu.Lock()
	url := s.wsConnMap[requestID]

	msg := WebSocketMessage{
		ID:           fmt.Sprintf("ws-send-%d", time.Now().UnixNano()),
		RequestID:    requestID,
		URL:          url,
		Direction:    "send",
		Timestamp:    int64(timestamp * 1000),
		Opcode:       int(opcode),
		Data:         payloadData,
		PayloadSize:  len(payloadData),
		Masked:       masked,
		ConnectionID: requestID,
	}

	s.wsMessages = append(s.wsMessages, msg)

	// 限制缓存大小
	if len(s.wsMessages) > 500 {
		s.wsMessages = s.wsMessages[100:]
	}
	s.mu.Unlock()

	// 广播事件
	s.broadcastEvent(CDPEvent{
		Type:      "websocket.message",
		Data:      msg,
		Timestamp: time.Now(),
	})
}

// handleWebSocketFrameReceived 处理 WebSocket 接收消息
func (s *CDPSession) handleWebSocketFrameReceived(params map[string]interface{}) {
	requestID, _ := params["requestId"].(string)
	timestamp, _ := params["timestamp"].(float64)
	response, _ := params["response"].(map[string]interface{})

	opcode, _ := response["opcode"].(float64)
	payloadData, _ := response["payloadData"].(string)
	masked, _ := response["mask"].(bool)

	s.mu.Lock()
	url := s.wsConnMap[requestID]

	msg := WebSocketMessage{
		ID:           fmt.Sprintf("ws-recv-%d", time.Now().UnixNano()),
		RequestID:    requestID,
		URL:          url,
		Direction:    "receive",
		Timestamp:    int64(timestamp * 1000),
		Opcode:       int(opcode),
		Data:         payloadData,
		PayloadSize:  len(payloadData),
		Masked:       masked,
		ConnectionID: requestID,
	}

	s.wsMessages = append(s.wsMessages, msg)

	// 限制缓存大小
	if len(s.wsMessages) > 500 {
		s.wsMessages = s.wsMessages[100:]
	}
	s.mu.Unlock()

	// 广播事件
	s.broadcastEvent(CDPEvent{
		Type:      "websocket.message",
		Data:      msg,
		Timestamp: time.Now(),
	})
}

// handleWebSocketClosed 处理 WebSocket 关闭
func (s *CDPSession) handleWebSocketClosed(params map[string]interface{}) {
	requestID, _ := params["requestId"].(string)

	s.mu.Lock()
	delete(s.wsConnMap, requestID)
	s.mu.Unlock()

	// 广播事件
	s.broadcastEvent(CDPEvent{
		Type: "websocket.closed",
		Data: map[string]interface{}{
			"requestId": requestID,
		},
		Timestamp: time.Now(),
	})
}

// handleFetchRequestPaused 处理 Fetch 请求暂停（拦截点）
func (s *CDPSession) handleFetchRequestPaused(params map[string]interface{}) {
	requestID, _ := params["requestId"].(string)
	request, _ := params["request"].(map[string]interface{})
	frameID, _ := params["frameId"].(string)
	resourceType, _ := params["resourceType"].(string)

	url, _ := request["url"].(string)
	method, _ := request["method"].(string)

	// 检查是否有匹配的拦截规则
	s.interceptMu.RLock()
	var matchedRule *InterceptRule
	for i := range s.interceptRules {
		rule := &s.interceptRules[i]
		if !rule.Enabled {
			continue
		}

		// 简单的 URL 匹配（支持通配符 *）
		if matchPattern(rule.URLPattern, url) {
			if rule.Method == "" || rule.Method == method {
				matchedRule = rule
				break
			}
		}
	}
	s.interceptMu.RUnlock()

	// 如果没有匹配的规则，继续请求
	if matchedRule == nil {
		_, err := s.SendCommand("Fetch.continueRequest", map[string]interface{}{
			"requestId": requestID,
		})
		if err != nil {
			logger.New("CDP").Error("[CDP] 继续请求失败", logger.F("request_id", requestID), logger.F("error", err))
		}
		return
	}

	// 执行拦截动作
	if matchedRule.Actions.Block {
		// 阻止请求
		_, err := s.SendCommand("Fetch.failRequest", map[string]interface{}{
			"requestId":   requestID,
			"errorReason": "BlockedByClient",
		})
		if err != nil {
			logger.New("CDP").Error("[CDP] 阻止请求失败", logger.F("request_id", requestID), logger.F("error", err))
		}
		return
	}

	if matchedRule.Actions.ModifyRequest && matchedRule.ModifyRequest != nil {
		// 修改请求
		modification := make(map[string]interface{})
		modification["requestId"] = requestID

		if matchedRule.ModifyRequest.URL != "" {
			modification["url"] = matchedRule.ModifyRequest.URL
		}
		if matchedRule.ModifyRequest.Method != "" {
			modification["method"] = matchedRule.ModifyRequest.Method
		}
		if len(matchedRule.ModifyRequest.Headers) > 0 {
			modification["headers"] = convertHeadersToArray(matchedRule.ModifyRequest.Headers)
		}
		if matchedRule.ModifyRequest.Body != "" {
			modification["postData"] = matchedRule.ModifyRequest.Body
		}

		_, err := s.SendCommand("Fetch.continueRequest", modification)
		if err != nil {
			logger.New("CDP").Error("[CDP] 修改请求失败", logger.F("request_id", requestID), logger.F("error", err))
		}
		return
	}

	// 默认继续请求
	_, err := s.SendCommand("Fetch.continueRequest", map[string]interface{}{
		"requestId": requestID,
	})
	if err != nil {
		logger.New("CDP").Error("[CDP] 默认继续请求失败", logger.F("request_id", requestID), logger.F("error", err))
	}

	// 避免未使用变量的编译错误
	_ = frameID
	_ = resourceType
}

// matchPattern 简单的通配符匹配
func matchPattern(pattern, str string) bool {
	if pattern == "*" {
		return true
	}

	// 简单实现：检查是否包含
	if len(pattern) > 0 && pattern[0] == '*' && pattern[len(pattern)-1] == '*' {
		// *keyword*
		keyword := pattern[1 : len(pattern)-1]
		return len(keyword) == 0 || contains(str, keyword)
	} else if len(pattern) > 0 && pattern[0] == '*' {
		// *suffix
		suffix := pattern[1:]
		return len(str) >= len(suffix) && str[len(str)-len(suffix):] == suffix
	} else if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		// prefix*
		prefix := pattern[:len(pattern)-1]
		return len(str) >= len(prefix) && str[:len(prefix)] == prefix
	}

	// 精确匹配
	return pattern == str
}

// contains 检查字符串是否包含子串
func contains(s, substr string) bool {
	return len(substr) == 0 || (len(s) >= len(substr) && indexOfSubstring(s, substr) >= 0)
}

// indexOfSubstring 查找子串位置
func indexOfSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// convertHeadersToArray 将 map 转换为数组格式
func convertHeadersToArray(headers map[string]string) []map[string]string {
	result := make([]map[string]string, 0, len(headers))
	for name, value := range headers {
		result = append(result, map[string]string{
			"name":  name,
			"value": value,
		})
	}
	return result
}
