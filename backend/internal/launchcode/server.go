package launchcode

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"ant-chrome/backend/internal/automation"
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/gateway"
	"ant-chrome/backend/internal/logger"
)

// BrowserStarter 浏览器启动接口（由 App 层实现并注入）
type BrowserStarter interface {
	StartInstance(profileId string) (*browser.Profile, error)
}

// BrowserStatusProvider 可选接口：提供实例运行态查询。
type BrowserStatusProvider interface {
	StatusInstance(profileId string) (*browser.Profile, error)
}

// LaunchRequestParams 支持外部自动化透传的一次性启动参数
type LaunchRequestParams struct {
	LaunchArgs           []string `json:"launchArgs"`
	StartURLs            []string `json:"startUrls"`
	SkipDefaultStartURLs bool     `json:"skipDefaultStartUrls"`
	ProxyId              string   `json:"proxyId"`
	ProxyConfig          string   `json:"proxyConfig"`
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

// BrowserStopper 可选接口：支持停止运行中的实例。
type BrowserStopper interface {
	StopInstance(profileId string) (*browser.Profile, error)
}

// BrowserDebugWaiter 可选接口：等待实例调试端口进入可接管状态。
type BrowserDebugWaiter interface {
	WaitInstanceDebugReady(profileId string, debugPort int, timeout time.Duration) (*browser.Profile, bool, error)
}

// AutomationScriptLister 可选接口：提供自动化脚本列表。
type AutomationScriptLister interface {
	AutomationScriptList() ([]automation.ScriptRecord, error)
}

// AutomationScriptGetter 可选接口：提供单个自动化脚本详情。
type AutomationScriptGetter interface {
	AutomationScriptGet(scriptID string) (*automation.ScriptRecord, error)
}

// AutomationScriptRunner 可选接口：执行自动化脚本。
type AutomationScriptRunner interface {
	AutomationScriptRunWithOptions(input automation.ScriptRunRequest) (*automation.ScriptRunRecord, error)
}

// AutomationScriptRunLister 可选接口：提供自动化脚本运行记录。
type AutomationScriptRunLister interface {
	AutomationScriptRunList(limit int) ([]automation.ScriptRunRecord, error)
}

// ProxyGatewayController 是可选的代理网关控制器。
// LaunchServer 只依赖这组窄接口，因此不会把 App 层实现泄漏到 launchcode 包。
type ProxyGatewayController interface {
	SwitchProxy(profileID, proxyID, proxyConfig string, force bool) (ProxyGatewaySwitchResult, error)
	GetRouting(profileID string) (gateway.RoutingConfig, error)
	SaveRouting(profileID string, routing gateway.RoutingConfig, force bool) (gateway.Status, error)
	Status(profileID string) (gateway.Status, error)
}

type ProxyGatewaySwitchResult struct {
	Gateway     gateway.Status
	AppliedLive bool
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
	service      *LaunchCodeService
	starter      BrowserStarter
	browserMgr   *browser.Manager
	port         int
	server       *http.Server
	mu           sync.Mutex
	authMu       sync.RWMutex
	logMu        sync.Mutex
	callLogs     []LaunchCallRecord
	activeMu     sync.RWMutex
	activePort   int
	activeID     string
	activeName   string
	apiAuth      APIAuthConfig
	proxyGateway ProxyGatewayController
}

// NewLaunchServer 创建 LaunchServer
func NewLaunchServer(service *LaunchCodeService, starter BrowserStarter, mgr *browser.Manager, port int) *LaunchServer {
	srv := &LaunchServer{
		service:    service,
		starter:    starter,
		browserMgr: mgr,
		port:       port,
	}
	srv.SetAPIAuthConfig(APIAuthConfig{})
	return srv
}

// SetProxyGatewayController 注入可选的实例代理网关控制器。
func (s *LaunchServer) SetProxyGatewayController(controller ProxyGatewayController) {
	s.mu.Lock()
	s.proxyGateway = controller
	s.mu.Unlock()
}

func (s *LaunchServer) proxyGatewayController() ProxyGatewayController {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.proxyGateway
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
	s.server = &http.Server{Handler: handler}
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

// ActiveProfile 返回当前统一 CDP 入口对应的实例信息。
func (s *LaunchServer) ActiveProfile() (string, string, int) {
	port, profileID, profileName := s.activeTarget()
	return profileID, profileName, port
}
