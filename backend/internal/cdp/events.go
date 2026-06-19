package cdp

import (
	"encoding/base64"
	"fmt"
	"time"
)

// listenEvents 监听CDP事件
func (s *CDPSession) listenEvents() {
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			var msg CDPMessage
			if err := s.ws.ReadJSON(&msg); err != nil {
				s.handleDisconnect(err)
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
	case "Runtime.consoleAPICalled":
		s.handleConsoleAPICalled(msg.Params)
	case "Runtime.exceptionThrown":
		s.handleExceptionThrown(msg.Params)
	case "Log.entryAdded":
		s.handleLogEntryAdded(msg.Params)
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
	s.networkRequests = append(s.networkRequests, *req)

	// 限制缓存大小
	if len(s.networkRequests) > 500 {
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

		req.StatusCode = int(statusCode)
		req.StatusText = statusText
		req.ResponseHeaders = convertHeaders(headers)
		req.MimeType = mimeType
	}
	s.mu.Unlock()

	if exists {
		// 异步获取响应体
		go s.fetchResponseBody(requestID)

		// 广播事件
		s.broadcastEvent(CDPEvent{
			Type:      "network.response",
			Data:      req,
			Timestamp: time.Now(),
		})
	}
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

	s.mu.Lock()
	if req, exists := s.requestMap[requestID]; exists {
		req.ResponseBody = body
		req.Size = int64(len(body))
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

	// 限制缓存大小
	if len(s.consoleLogs) > 1000 {
		s.consoleLogs = s.consoleLogs[200:]
	}
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
	s.mu.Unlock()

	// 广播事件
	s.broadcastEvent(CDPEvent{
		Type:      "console.log",
		Data:      log,
		Timestamp: time.Now(),
	})
}

// convertHeaders 转换headers
func convertHeaders(headers map[string]interface{}) map[string]string {
	result := make(map[string]string)
	for k, v := range headers {
		result[k] = fmt.Sprintf("%v", v)
	}
	return result
}

// handleDisconnect 处理断线
func (s *CDPSession) handleDisconnect(err error) {
	s.connected = false

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
			// 发送ping保持连接
			if s.connected {
				_, err := s.SendCommand("Browser.getVersion", nil)
				if err != nil {
					s.handleDisconnect(err)
				}
			}
		}
	}
}
