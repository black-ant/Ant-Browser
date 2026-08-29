package launchcode

import (
	"net/http"
	"strings"

	"ant-chrome/backend/internal/config"
)

// 代理池与内核能力。
//
// 与实例/自动化不同，这两个域此前没有 HTTP 层，proxy manager 也没有注入
// LaunchServer。
//
// 这里没有沿用本包其他能力那套「对 starter 做类型断言」的做法：starter 在
// 生产环境就是 *App，而 Wails 会把 *App 上所有导出方法都绑定给前端。若把这
// 几个能力做成 App 的方法，前端就会凭空多出一批语义重复的绑定。改为显式注入
// 一个小适配器，能力边界更清楚，也不污染前端 API。

// ProxyProvider 提供代理池与内核信息。由宿主显式注入。
type ProxyProvider interface {
	ListProxyNodes() []config.BrowserProxy
	TestProxyNodeSpeed(proxyID string) ProxySpeedResult
	CheckProxyNodeHealth(proxyID string) ProxyHealthResult
	ListBrowserCores() []config.BrowserCore
}

// ProxySpeedResult 是代理测速结果。
// 定义在本包是为了避免 launchcode 反向依赖 backend 包（会成环）。
type ProxySpeedResult struct {
	ProxyID   string `json:"proxyId"`
	Ok        bool   `json:"ok"`
	LatencyMs int64  `json:"latencyMs"`
	Engine    string `json:"engine"`
	Error     string `json:"error"`
}

// ProxyHealthResult 是代理出口 IP 健康检测结果。
type ProxyHealthResult struct {
	ProxyID        string `json:"proxyId"`
	Ok             bool   `json:"ok"`
	Source         string `json:"source"`
	Error          string `json:"error"`
	IP             string `json:"ip"`
	FraudScore     int64  `json:"fraudScore"`
	IsResidential  bool   `json:"isResidential"`
	Country        string `json:"country"`
	Region         string `json:"region"`
	City           string `json:"city"`
	AsOrganization string `json:"asOrganization"`
	UpdatedAt      string `json:"updatedAt"`
}

const proxyAPIUnavailable = "proxy api is unavailable"

// ListProxies 返回代理池中全部节点。
func (s *LaunchServer) ListProxies() ([]config.BrowserProxy, error) {
	provider := s.proxyProvider()
	if provider == nil {
		return nil, newServiceError(http.StatusServiceUnavailable, proxyAPIUnavailable)
	}
	return provider.ListProxyNodes(), nil
}

// TestProxySpeed 对指定代理测速。
// 注意这里会按当前连接栈启动桥接进程，属于耗时操作。
func (s *LaunchServer) TestProxySpeed(proxyID string) (*ProxySpeedResult, error) {
	proxyID = strings.TrimSpace(proxyID)
	if proxyID == "" {
		return nil, newServiceError(http.StatusBadRequest, "proxyId is required")
	}

	provider := s.proxyProvider()
	if provider == nil {
		return nil, newServiceError(http.StatusServiceUnavailable, proxyAPIUnavailable)
	}

	result := provider.TestProxyNodeSpeed(proxyID)
	return &result, nil
}

// CheckProxyHealth 检测代理出口 IP 的归属与风险信息。
func (s *LaunchServer) CheckProxyHealth(proxyID string) (*ProxyHealthResult, error) {
	proxyID = strings.TrimSpace(proxyID)
	if proxyID == "" {
		return nil, newServiceError(http.StatusBadRequest, "proxyId is required")
	}

	provider := s.proxyProvider()
	if provider == nil {
		return nil, newServiceError(http.StatusServiceUnavailable, proxyAPIUnavailable)
	}

	result := provider.CheckProxyNodeHealth(proxyID)
	return &result, nil
}

// ListCores 返回已登记的浏览器内核。
func (s *LaunchServer) ListCores() ([]config.BrowserCore, error) {
	provider := s.proxyProvider()
	if provider == nil {
		return nil, newServiceError(http.StatusServiceUnavailable, "browser core api is unavailable")
	}
	return provider.ListBrowserCores(), nil
}

// SetProxyProvider 注入代理池与内核能力。未注入时相关工具返回 Unavailable。
func (s *LaunchServer) SetProxyProvider(provider ProxyProvider) {
	s.proxyMu.Lock()
	s.proxy = provider
	s.proxyMu.Unlock()
}

func (s *LaunchServer) proxyProvider() ProxyProvider {
	s.proxyMu.RLock()
	defer s.proxyMu.RUnlock()
	return s.proxy
}
