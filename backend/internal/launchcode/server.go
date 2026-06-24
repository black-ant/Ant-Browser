package launchcode

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/logger"
)

// BrowserStarter 浏览器启动接口（由 App 层实现并注入）
type BrowserStarter interface {
	StartInstance(profileId string) (*browser.Profile, error)
}

// BrowserStopper 浏览器停止接口（由 App 层实现并注入）
type BrowserStopper interface {
	StopInstance(profileId string) error
}

// LaunchRequestParams 支持外部自动化透传的一次性启动参数
type LaunchRequestParams struct {
	LaunchArgs           []string `json:"launchArgs"`
	StartURLs            []string `json:"startUrls"`
	SkipDefaultStartURLs bool     `json:"skipDefaultStartUrls"`
}

// LaunchRequest POST /api/launch 的请求体
type LaunchRequest struct {
	Code        string          `json:"code"`
	Key         string          `json:"key"`
	ProfileID   string          `json:"profileId"`
	ProfileName string          `json:"profileName"`
	Keyword     string          `json:"keyword"`
	Keywords    []string        `json:"keywords"`
	Tag         string          `json:"tag"`
	Tags        []string        `json:"tags"`
	GroupID     string          `json:"groupId"`
	MatchMode   string          `json:"matchMode"`
	Selector    *LaunchSelector `json:"selector"`
	LaunchRequestParams
}

// BrowserStarterWithParams 可选接口：支持带参数启动实例
type BrowserStarterWithParams interface {
	StartInstanceWithParams(profileId string, params LaunchRequestParams) (*browser.Profile, error)
}

// LaunchCallRecord 接口调用记录
type LaunchCallRecord struct {
	Timestamp   string              `json:"timestamp"`
	Method      string              `json:"method"`
	Path        string              `json:"path"`
	ClientIP    string              `json:"clientIp"`
	Code        string              `json:"code"`
	Selector    LaunchSelector      `json:"selector,omitempty"`
	ProfileID   string              `json:"profileId"`
	ProfileName string              `json:"profileName"`
	Params      LaunchRequestParams `json:"params"`
	OK          bool                `json:"ok"`
	Status      int                 `json:"status"`
	Error       string              `json:"error"`
	DurationMs  int64               `json:"durationMs"`
}

// LaunchServer 本地 HTTP 唤起服务
type LaunchServer struct {
	service       *LaunchCodeService
	starter       BrowserStarter
	stopper       BrowserStopper
	browserMgr    *browser.Manager
	port          int
	server        *http.Server
	mu            sync.Mutex
	authMu        sync.RWMutex
	logMu         sync.Mutex
	callLogs      []LaunchCallRecord
	activeMu      sync.RWMutex
	activePort    int
	activeID      string
	activeName    string
	apiAuth       APIAuthConfig
	cdpRateLimit  *rateLimiter // CDP 代理速率限制器
}

// NewLaunchServer 创建 LaunchServer
func NewLaunchServer(service *LaunchCodeService, starter BrowserStarter, stopper BrowserStopper, mgr *browser.Manager, port int) *LaunchServer {
	srv := &LaunchServer{
		service:      service,
		starter:      starter,
		stopper:      stopper,
		browserMgr:   mgr,
		port:         port,
		cdpRateLimit: newRateLimiter(100, 200), // 每秒 100 请求,突发 200
	}
	srv.SetAPIAuthConfig(APIAuthConfig{})
	return srv
}

// Start 非阻塞启动 HTTP 服务。
// 规则：
//   - port <= 0：自动分配随机可用端口（仅内部测试/显式传 0 时）
//   - port > 0：绑定指定固定端口；若被占用则直接返回错误
func (s *LaunchServer) Start() error {
	handler := s.buildHandler(true)

	preferredPort := s.port
	ln, port, err := bindLaunchListener(preferredPort)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.port = port
	s.server = &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	s.mu.Unlock()

	log := logger.New("LaunchServer")
	if preferredPort <= 0 {
		log.Info("LaunchServer 使用随机端口", logger.F("port", port))
	} else {
		log.Info("LaunchServer 使用固定端口", logger.F("port", port))
	}
	auth := s.apiAuthConfig()
	if auth.Active() {
		log.Info("LaunchServer API 认证已启用", logger.F("header", auth.Header))
	} else if auth.Requested() && !auth.Configured() {
		log.Warn("LaunchServer API 认证配置未生效", logger.F("reason", "api_key is empty"), logger.F("header", auth.Header))
	}
	log.Info("LaunchServer 已启动", logger.F("port", port))

	go func() {
		if serveErr := s.server.Serve(ln); serveErr != nil && serveErr != http.ErrServerClosed {
			log.Error("LaunchServer 异常退出", logger.F("error", serveErr.Error()))
		}
	}()

	return nil
}

func (s *LaunchServer) buildMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/profiles", s.handleProfiles)
	mux.HandleFunc("/api/profiles/", s.handleProfileByID)
	mux.HandleFunc("/api/launch", s.handleLaunchWithBody)
	mux.HandleFunc("/api/launch/logs", s.handleLaunchLogs)
	mux.HandleFunc("/api/launch/", s.handleLaunch)
	mux.HandleFunc("/api/stop", s.handleStop)
	mux.HandleFunc("/", s.handleCDPProxy)
	return mux
}

// requestBodyLimitMiddleware 限制所有 POST/PUT/PATCH 请求体大小
func (s *LaunchServer) requestBodyLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH" {
			// 限制 1MB,与 /api/launch 保持一致
			r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		}
		next.ServeHTTP(w, r)
	})
}

func (s *LaunchServer) buildHandler(includeLocalhost bool) http.Handler {
	var handler http.Handler = s.buildMux()
	handler = s.requestBodyLimitMiddleware(handler)
	handler = s.apiAuthMiddleware(handler)
	if includeLocalhost {
		handler = s.localhostMiddleware(handler)
	}
	return handler
}

func bindLaunchListener(preferredPort int) (net.Listener, int, error) {
	if preferredPort <= 0 {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return nil, 0, fmt.Errorf("自动分配端口失败: %w", err)
		}
		port, err := listenerPort(ln)
		if err != nil {
			_ = ln.Close()
			return nil, 0, err
		}
		return ln, port, nil
	}

	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(preferredPort))
	ln, err := net.Listen("tcp", addr)
	if err == nil {
		return ln, preferredPort, nil
	}
	return nil, 0, fmt.Errorf("端口 %d 不可用: %w", preferredPort, err)
}

func listenerPort(ln net.Listener) (int, error) {
	if ln == nil {
		return 0, fmt.Errorf("listener is nil")
	}
	if tcpAddr, ok := ln.Addr().(*net.TCPAddr); ok {
		return tcpAddr.Port, nil
	}

	_, rawPort, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		return 0, fmt.Errorf("解析监听地址失败: %w", err)
	}
	port, err := strconv.Atoi(rawPort)
	if err != nil {
		return 0, fmt.Errorf("解析端口失败: %w", err)
	}
	return port, nil
}

// Stop 优雅关闭（5 秒超时）
func (s *LaunchServer) Stop() error {
	s.mu.Lock()
	srv := s.server
	s.mu.Unlock()

	if srv == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}

// Port 返回实际绑定的端口
func (s *LaunchServer) Port() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.port
}

// CDPURL 返回对外暴露的固定 CDP 入口地址。
func (s *LaunchServer) CDPURL() string {
	port := s.Port()
	if port <= 0 {
		return ""
	}
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}

// ActiveDebugPort 返回当前活动实例的内部调试端口。
func (s *LaunchServer) ActiveDebugPort() int {
	s.activeMu.RLock()
	defer s.activeMu.RUnlock()
	return s.activePort
}

// SetActiveProfile 将统一入口切换到指定实例的调试端口。
func (s *LaunchServer) SetActiveProfile(profile *browser.Profile) {
	if profile == nil || profile.DebugPort <= 0 || !profile.DebugReady {
		return
	}

	s.activeMu.Lock()
	s.activePort = profile.DebugPort
	s.activeID = profile.ProfileId
	s.activeName = profile.ProfileName
	s.activeMu.Unlock()
}

// ClearActiveProfile 在当前活动实例停止后清空统一入口。
func (s *LaunchServer) ClearActiveProfile(profileID string) {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return
	}

	s.activeMu.Lock()
	if s.activeID == profileID {
		s.activePort = 0
		s.activeID = ""
		s.activeName = ""
	}
	s.activeMu.Unlock()
}

func (s *LaunchServer) activeTarget() (int, string, string) {
	s.activeMu.RLock()
	defer s.activeMu.RUnlock()
	return s.activePort, s.activeID, s.activeName
}

// isLoopbackHostname 判断主机名是否为本机回环地址（不含端口）。
func isLoopbackHostname(host string) bool {
	host = strings.TrimSpace(host)
	if host == "" {
		return false
	}
	// 去掉 IPv6 字面量的方括号，如 [::1]
	host = strings.Trim(host, "[]")
	switch strings.ToLower(host) {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

// hostnameOnly 提取 host[:port] 中的主机名部分。
func hostnameOnly(hostport string) string {
	hostport = strings.TrimSpace(hostport)
	if hostport == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(hostport); err == nil {
		return host
	}
	return hostport
}

// validateCDPProxyRequest 校验转发到 CDP 端口的请求来源，阻断浏览器发起的
// DNS-rebinding / localhost-CSRF 攻击。返回 (拒绝原因, 是否允许)。
//
// 规则（与 Chromium 调试端口一致）：
//   - Host 头必须指向回环地址（127.0.0.1 / localhost / ::1）。DNS-rebinding
//     攻击会携带攻击者控制的域名，从而被拒绝。
//   - 若存在 Origin 头（浏览器跨源请求必然携带），其主机名也必须为回环地址。
//     合法的命令行自动化工具不会发送 Origin，因此不受影响。
func validateCDPProxyRequest(r *http.Request) (string, bool) {
	if host := hostnameOnly(r.Host); host != "" && !isLoopbackHostname(host) {
		return "invalid Host header for CDP proxy", false
	}
	if origin := strings.TrimSpace(r.Header.Get("Origin")); origin != "" {
		u, err := url.Parse(origin)
		if err != nil || !isLoopbackHostname(hostnameOnly(u.Host)) {
			return "cross-origin requests are not allowed on the CDP proxy", false
		}
	}
	return "", true
}

// localhostMiddleware 只允许 127.0.0.1 访问
func (s *LaunchServer) localhostMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil || host != "127.0.0.1" {
			writeJSON(w, http.StatusForbidden, map[string]interface{}{
				"ok":    false,
				"error": "forbidden: only localhost is allowed",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// handleHealth GET /api/health
func (s *LaunchServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// handleCDPProxy 将统一端口上的非 /api 请求转发到当前活动实例的 CDP 端口。
//
// 安全说明：该端口转发的是 Chromium 调试协议（CDP），可执行任意 JS、读取
// cookie/存储、导航到 file:// 等，等同于完全控制浏览器。CDP 端点对 WebSocket
// 升级不做 CORS 限制，因此任何用户访问的网站都可能直接连到本机端口发起
// DNS-rebinding / localhost-CSRF 攻击。这里采用 Chromium 调试端口同款防护：
// 校验 Host 与 Origin —— 浏览器发起的跨源请求必然带有外部 Host 或 Origin，
// 而合法的命令行自动化工具（Selenium/Puppeteer 等）不会携带 Origin 且 Host
// 指向 127.0.0.1，因此可被放行。
func (s *LaunchServer) handleCDPProxy(w http.ResponseWriter, r *http.Request) {
	if reason, ok := validateCDPProxyRequest(r); !ok {
		writeJSON(w, http.StatusForbidden, map[string]interface{}{
			"ok":    false,
			"error": "forbidden: " + reason,
		})
		return
	}

	// 速率限制: 防止本地恶意进程 DoS
	clientIP := extractIP(r.RemoteAddr)
	if !s.cdpRateLimit.allow(clientIP) {
		writeJSON(w, http.StatusTooManyRequests, map[string]interface{}{
			"ok":    false,
			"error": "rate limit exceeded",
		})
		return
	}

	debugPort, profileID, profileName := s.activeTarget()
	if debugPort <= 0 {
		writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"ok":          false,
			"error":       "no active browser debug target",
			"profileId":   profileID,
			"profileName": profileName,
		})
		return
	}

	target, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", debugPort))
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid cdp target: %v", err), http.StatusInternalServerError)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, proxyErr error) {
		http.Error(w, fmt.Sprintf("cdp proxy error: %v", proxyErr), http.StatusBadGateway)
	}
	proxy.ServeHTTP(w, r)
}

// handleLaunch GET /api/launch/{code}
// ⚠️ GET 方法有副作用（启动浏览器实例），存在 CSRF 风险。
// 为了防止跨站请求伪造攻击，当 API 认证未启用时，要求自定义请求头 X-Launch-Request。
// 推荐使用 POST /api/launch 接口。
func (s *LaunchServer) handleLaunch(w http.ResponseWriter, r *http.Request) {
	startAt := time.Now()
	clientIP := remoteIP(r.RemoteAddr)
	selector := LaunchSelector{}
	if r.Method != http.MethodGet {
		msg := "method not allowed"
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"ok":    false,
			"error": msg,
		})
		s.appendLaunchLog(r.Method, r.URL.Path, clientIP, "", selector, LaunchRequestParams{}, false, http.StatusMethodNotAllowed, msg, "", "", startAt)
		return
	}

	// CSRF 防护：当 API 认证未启用时，要求自定义请求头
	auth := s.apiAuthConfig()
	if !auth.Active() {
		if r.Header.Get("X-Launch-Request") == "" {
			msg := "Missing required header X-Launch-Request. For security, GET requests with side-effects require this header to prevent CSRF attacks. Consider using POST /api/launch instead."
			writeJSON(w, http.StatusForbidden, map[string]interface{}{
				"ok":    false,
				"error": msg,
			})
			s.appendLaunchLog(r.Method, r.URL.Path, clientIP, "", selector, LaunchRequestParams{}, false, http.StatusForbidden, msg, "", "", startAt)
			return
		}
	}

	code := strings.TrimPrefix(r.URL.Path, "/api/launch/")
	if strings.TrimSpace(code) == "" {
		msg := "launch code not found"
		writeJSON(w, http.StatusNotFound, map[string]interface{}{
			"ok":    false,
			"error": msg,
		})
		s.appendLaunchLog(r.Method, r.URL.Path, clientIP, "", selector, LaunchRequestParams{}, false, http.StatusNotFound, msg, "", "", startAt)
		return
	}

	selector = normalizeLaunchSelector(LaunchSelector{Code: code})
	profile, launchCode, status, errMsg := s.launchByCode(code, LaunchRequestParams{})
	if errMsg != "" {
		writeJSON(w, status, map[string]interface{}{
			"ok":    false,
			"error": errMsg,
		})
		s.appendLaunchLog(r.Method, r.URL.Path, clientIP, selector.Code, selector, LaunchRequestParams{}, false, status, errMsg, "", "", startAt)
		return
	}

	s.SetActiveProfile(profile)
	writeJSON(w, http.StatusOK, s.launchSuccessPayload(profile, launchCode))
	s.appendLaunchLog(r.Method, r.URL.Path, clientIP, launchCode, selector, LaunchRequestParams{}, true, http.StatusOK, "", profile.ProfileId, profile.ProfileName, startAt)
}

func (s *LaunchServer) launchSuccessPayload(profile *browser.Profile, launchCode string) map[string]interface{} {
	cdpURL := s.CDPURL()
	cdpPort := s.Port()
	if cdpURL == "" && profile != nil && profile.DebugReady && profile.DebugPort > 0 {
		cdpPort = profile.DebugPort
		cdpURL = fmt.Sprintf("http://127.0.0.1:%d", profile.DebugPort)
	}

	return map[string]interface{}{
		"ok":             true,
		"profileId":      profile.ProfileId,
		"profileName":    profile.ProfileName,
		"launchCode":     launchCode,
		"pid":            profile.Pid,
		"debugPort":      profile.DebugPort,
		"debugReady":     profile.DebugReady,
		"runtimeWarning": profile.RuntimeWarning,
		"cdpPort":        cdpPort,
		"cdpUrl":         cdpURL,
	}
}

// handleLaunchWithBody POST /api/launch
func (s *LaunchServer) handleLaunchWithBody(w http.ResponseWriter, r *http.Request) {
	startAt := time.Now()
	clientIP := remoteIP(r.RemoteAddr)
	selector := LaunchSelector{}
	if r.Method != http.MethodPost {
		msg := "method not allowed"
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"ok":    false,
			"error": msg,
		})
		s.appendLaunchLog(r.Method, r.URL.Path, clientIP, "", selector, LaunchRequestParams{}, false, http.StatusMethodNotAllowed, msg, "", "", startAt)
		return
	}

	var req LaunchRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		msg := "invalid request body"
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok":    false,
			"error": msg,
		})
		s.appendLaunchLog(r.Method, r.URL.Path, clientIP, "", selector, LaunchRequestParams{}, false, http.StatusBadRequest, msg, "", "", startAt)
		return
	}

	selector = mergeLaunchSelector(req)
	if selector.IsEmpty() {
		msg := "selector is required"
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok":    false,
			"error": msg,
		})
		s.appendLaunchLog(r.Method, r.URL.Path, clientIP, "", selector, req.LaunchRequestParams, false, http.StatusBadRequest, msg, "", "", startAt)
		return
	}

	req.LaunchArgs = normalizeStringSlice(req.LaunchArgs)
	req.StartURLs = normalizeStringSlice(req.StartURLs)
	if selector.MatchMode == launchMatchModeAll {
		profiles, status, errMsg := s.launchAllBySelector(selector, req.LaunchRequestParams)
		if errMsg != "" {
			writeJSON(w, status, map[string]interface{}{
				"ok":    false,
				"error": errMsg,
			})
			s.appendLaunchLog(r.Method, r.URL.Path, clientIP, selector.Code, selector, req.LaunchRequestParams, false, status, errMsg, "", "", startAt)
			return
		}

		activeProfile, profileIDs, profileNames := summarizeLaunchedProfiles(profiles)
		if activeProfile != nil {
			s.SetActiveProfile(activeProfile)
		}
		writeJSON(w, http.StatusOK, s.launchBatchSuccessPayload(profiles))
		s.appendLaunchLog(r.Method, r.URL.Path, clientIP, selector.Code, selector, req.LaunchRequestParams, true, http.StatusOK, "", profileIDs, profileNames, startAt)
		return
	}

	profile, launchCode, status, errMsg := s.launchBySelector(selector, req.LaunchRequestParams)
	if errMsg != "" {
		writeJSON(w, status, map[string]interface{}{
			"ok":    false,
			"error": errMsg,
		})
		s.appendLaunchLog(r.Method, r.URL.Path, clientIP, launchCode, selector, req.LaunchRequestParams, false, status, errMsg, "", "", startAt)
		return
	}

	s.SetActiveProfile(profile)
	writeJSON(w, http.StatusOK, s.launchSuccessPayload(profile, launchCode))
	s.appendLaunchLog(r.Method, r.URL.Path, clientIP, launchCode, selector, req.LaunchRequestParams, true, http.StatusOK, "", profile.ProfileId, profile.ProfileName, startAt)
}

// handleLaunchLogs GET /api/launch/logs?limit=50
func (s *LaunchServer) handleLaunchLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"ok":    false,
			"error": "method not allowed",
		})
		return
	}

	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			if n < 1 {
				n = 1
			}
			if n > 200 {
				n = 200
			}
			limit = n
		}
	}

	items := s.listLaunchLogs(limit)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":    true,
		"items": items,
	})
}

// handleStop POST /api/stop 停止实例
func (s *LaunchServer) handleStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"ok":    false,
			"error": "method not allowed",
		})
		return
	}

	// 检查是否注入了 stopper
	if s.stopper == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]interface{}{
			"ok":    false,
			"error": "stop functionality not available",
		})
		return
	}

	// 解析请求体
	type StopRequest struct {
		Code        string   `json:"code"`
		ProfileID   string   `json:"profileId"`
		ProfileName string   `json:"profileName"`
		Keyword     string   `json:"keyword"`
		Keywords    []string `json:"keywords"`
		Tag         string   `json:"tag"`
		Tags        []string `json:"tags"`
		GroupID     string   `json:"groupId"`
	}

	var req StopRequest
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	if err := dec.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok":    false,
			"error": "invalid request body",
		})
		return
	}

	// 构建选择器
	selector := LaunchSelector{
		Code:        req.Code,
		ProfileID:   req.ProfileID,
		ProfileName: req.ProfileName,
		Keyword:     req.Keyword,
		Keywords:    req.Keywords,
		Tag:         req.Tag,
		Tags:        req.Tags,
		GroupID:     req.GroupID,
	}

	// 归一化并验证选择器
	selector = normalizeLaunchSelector(selector)
	if selector.IsEmpty() {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok":    false,
			"error": "selector is required",
		})
		return
	}
	if err := selector.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok":    false,
			"error": err.Error(),
		})
		return
	}

	// 解析 profileID
	var profileID string
	var err error

	if selector.OnlyCode() {
		// 通过 LaunchCode 解析
		profileID, err = s.service.Resolve(selector.Code)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]interface{}{
				"ok":    false,
				"error": "launch code not found",
			})
			return
		}
	} else {
		// 通过其他选择器查找
		profileSnapshot, status, errMsg := s.findProfileBySelector(selector)
		if errMsg != "" {
			writeJSON(w, status, map[string]interface{}{
				"ok":    false,
				"error": errMsg,
			})
			return
		}
		profileID = profileSnapshot.ProfileId
	}

	// 停止实例
	if err := s.stopper.StopInstance(profileID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok":    false,
			"error": fmt.Sprintf("stop instance failed: %v", err),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":        true,
		"profileId": profileID,
		"message":   "instance stopped successfully",
	})
}

func (s *LaunchServer) launchByCode(code string, params LaunchRequestParams) (*browser.Profile, string, int, string) {
	return s.launchBySelectorInternal(normalizeLaunchSelector(LaunchSelector{Code: code}), params, false)
}

func (s *LaunchServer) launchBySelector(selector LaunchSelector, params LaunchRequestParams) (*browser.Profile, string, int, string) {
	return s.launchBySelectorInternal(selector, params, true)
}

func (s *LaunchServer) launchProfile(profileID string, params LaunchRequestParams) (*browser.Profile, error) {
	if starterWithParams, ok := s.starter.(BrowserStarterWithParams); ok {
		profile, err := starterWithParams.StartInstanceWithParams(profileID, params)
		return normalizeLaunchedProfileRuntime(profile), err
	}
	profile, err := s.starter.StartInstance(profileID)
	return normalizeLaunchedProfileRuntime(profile), err
}

func normalizeLaunchedProfileRuntime(profile *browser.Profile) *browser.Profile {
	if profile == nil {
		return nil
	}

	// Backward compatibility: older starter implementations only filled pid/debugPort.
	if !profile.Running && (profile.Pid > 0 || profile.DebugPort > 0) {
		profile.Running = true
	}
	if !profile.DebugReady &&
		profile.DebugPort > 0 &&
		strings.TrimSpace(profile.RuntimeWarning) == "" &&
		(profile.Running || profile.Pid > 0) {
		profile.DebugReady = true
	}

	return profile
}

func (s *LaunchServer) launchBySelectorInternal(selector LaunchSelector, params LaunchRequestParams, allowCodeKeywordFallback bool) (*browser.Profile, string, int, string) {
	var (
		profileID  string
		launchCode string
		err        error
	)

	selector = normalizeLaunchSelector(selector)
	if selector.IsEmpty() {
		return nil, "", http.StatusBadRequest, "selector is required"
	}
	if err = selector.Validate(); err != nil {
		return nil, "", http.StatusBadRequest, err.Error()
	}
	selector = s.withCodeKeywordFallback(selector, allowCodeKeywordFallback)

	if selector.OnlyCode() {
		profileID, err = s.service.Resolve(selector.Code)
		if err != nil {
			// 记录认证失败
			log := logger.New("LaunchServer")
			log.Warn("Launch Code认证失败",
				logger.F("code", selector.Code),
				logger.F("error", err.Error()))
			return nil, "", http.StatusNotFound, "launch code not found"
		}
		launchCode = selector.Code
	} else {
		profileSnapshot, status, errMsg := s.findProfileBySelector(selector)
		if errMsg != "" {
			if selector.Code != "" {
				launchCode = selector.Code
			}
			return nil, launchCode, status, errMsg
		}
		profileID = profileSnapshot.ProfileId
		launchCode = profileSnapshot.LaunchCode
	}

	profile, err := s.launchProfile(profileID, params)
	if err != nil {
		return nil, launchCode, http.StatusInternalServerError, err.Error()
	}

	if launchCode == "" && s.service != nil && profile != nil {
		if code, codeErr := s.service.EnsureCode(profile.ProfileId); codeErr == nil {
			launchCode = code
		}
	}
	if profile != nil && launchCode != "" {
		profile.LaunchCode = launchCode
	}

	return profile, launchCode, http.StatusOK, ""
}

func (s *LaunchServer) launchAllBySelector(selector LaunchSelector, params LaunchRequestParams) ([]*browser.Profile, int, string) {
	selector = normalizeLaunchSelector(selector)
	if selector.IsEmpty() {
		return nil, http.StatusBadRequest, "selector is required"
	}
	if err := selector.Validate(); err != nil {
		return nil, http.StatusBadRequest, err.Error()
	}
	selector = s.withCodeKeywordFallback(selector, true)

	snapshots, status, errMsg := s.findProfilesBySelector(selector)
	if errMsg != "" {
		return nil, status, errMsg
	}

	profiles := make([]*browser.Profile, 0, len(snapshots))
	for _, snapshot := range snapshots {
		profile, err := s.launchProfile(snapshot.ProfileId, params)
		if err != nil {
			label := strings.TrimSpace(snapshot.ProfileName)
			if label == "" {
				label = snapshot.ProfileId
			}
			return profiles, http.StatusInternalServerError, fmt.Sprintf("failed to start profile %s after launching %d profile(s): %v", label, len(profiles), err)
		}

		launchCode := snapshot.LaunchCode
		if launchCode == "" && s.service != nil && profile != nil {
			if code, codeErr := s.service.EnsureCode(profile.ProfileId); codeErr == nil {
				launchCode = code
			}
		}
		if profile != nil && launchCode != "" {
			profile.LaunchCode = launchCode
		}

		profiles = append(profiles, profile)
	}

	return profiles, http.StatusOK, ""
}

func (s *LaunchServer) withCodeKeywordFallback(selector LaunchSelector, allow bool) LaunchSelector {
	if !allow || strings.TrimSpace(selector.Code) == "" {
		return selector
	}
	if s.service != nil {
		if _, err := s.service.Resolve(selector.Code); err == nil {
			return selector
		}
	}

	fallback := selector
	if strings.TrimSpace(fallback.Key) == "" {
		fallback.Key = selector.Code
	}
	fallback.Code = ""
	return fallback
}

func (s *LaunchServer) launchBatchSuccessPayload(profiles []*browser.Profile) map[string]interface{} {
	items := make([]map[string]interface{}, 0, len(profiles))
	for i, profile := range profiles {
		if profile == nil {
			continue
		}
		item := map[string]interface{}{
			"profileId":      profile.ProfileId,
			"profileName":    profile.ProfileName,
			"launchCode":     profile.LaunchCode,
			"pid":            profile.Pid,
			"debugPort":      profile.DebugPort,
			"debugReady":     profile.DebugReady,
			"runtimeWarning": profile.RuntimeWarning,
			"isActive":       i == len(profiles)-1,
		}
		items = append(items, item)
	}

	activeProfile, _, _ := summarizeLaunchedProfiles(profiles)
	cdpURL := s.CDPURL()
	cdpPort := s.Port()
	if cdpURL == "" && activeProfile != nil && activeProfile.DebugReady && activeProfile.DebugPort > 0 {
		cdpPort = activeProfile.DebugPort
		cdpURL = fmt.Sprintf("http://127.0.0.1:%d", activeProfile.DebugPort)
	}

	payload := map[string]interface{}{
		"ok":        true,
		"matchMode": launchMatchModeAll,
		"count":     len(items),
		"items":     items,
		"cdpPort":   cdpPort,
		"cdpUrl":    cdpURL,
	}
	if activeProfile != nil {
		payload["activeProfileId"] = activeProfile.ProfileId
		payload["activeProfileName"] = activeProfile.ProfileName
	}
	return payload
}

func summarizeLaunchedProfiles(profiles []*browser.Profile) (*browser.Profile, string, string) {
	if len(profiles) == 0 {
		return nil, "", ""
	}

	ids := make([]string, 0, len(profiles))
	names := make([]string, 0, len(profiles))
	var active *browser.Profile
	for _, profile := range profiles {
		if profile == nil {
			continue
		}
		active = profile
		ids = append(ids, profile.ProfileId)
		if trimmed := strings.TrimSpace(profile.ProfileName); trimmed != "" {
			names = append(names, trimmed)
		}
	}
	return active, strings.Join(ids, ","), strings.Join(names, ",")
}

// writeJSON 写入 JSON 响应
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Content-Security-Policy", "default-src 'none'")
	w.Header().Set("X-XSS-Protection", "1; mode=block")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// NewTestHandler 返回不含 localhost 限制的 handler，仅供测试使用
func NewTestHandler(s *LaunchServer) http.Handler {
	return s.buildHandler(false)
}

func normalizeStringSlice(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		v := strings.TrimSpace(item)
		if v != "" {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (s *LaunchServer) appendLaunchLog(method, path, clientIP, code string, selector LaunchSelector, params LaunchRequestParams, ok bool, status int, errMsg, profileID, profileName string, startAt time.Time) {
	entry := LaunchCallRecord{
		Timestamp:   time.Now().Format(time.RFC3339),
		Method:      method,
		Path:        path,
		ClientIP:    clientIP,
		Code:        strings.TrimSpace(code),
		Selector:    selector,
		ProfileID:   profileID,
		ProfileName: profileName,
		Params:      params,
		OK:          ok,
		Status:      status,
		Error:       errMsg,
		DurationMs:  time.Since(startAt).Milliseconds(),
	}

	s.logMu.Lock()
	s.callLogs = append(s.callLogs, entry)
	if len(s.callLogs) > 500 {
		s.callLogs = append([]LaunchCallRecord(nil), s.callLogs[len(s.callLogs)-500:]...)
	}
	s.logMu.Unlock()

	log := logger.New("LaunchServer")
	if ok {
		log.Info("Launch API 调用", logger.F("method", method), logger.F("path", path), logger.F("code", entry.Code), logger.F("profile_id", profileID), logger.F("status", status), logger.F("duration_ms", entry.DurationMs))
		return
	}
	log.Warn("Launch API 调用失败", logger.F("method", method), logger.F("path", path), logger.F("code", entry.Code), logger.F("status", status), logger.F("error", errMsg), logger.F("duration_ms", entry.DurationMs))
}

func (s *LaunchServer) listLaunchLogs(limit int) []LaunchCallRecord {
	s.logMu.Lock()
	defer s.logMu.Unlock()

	if limit <= 0 {
		limit = 50
	}
	if limit > len(s.callLogs) {
		limit = len(s.callLogs)
	}
	if limit == 0 {
		return []LaunchCallRecord{}
	}

	out := make([]LaunchCallRecord, 0, limit)
	for i := len(s.callLogs) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, s.callLogs[i])
	}
	return out
}

func remoteIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}
