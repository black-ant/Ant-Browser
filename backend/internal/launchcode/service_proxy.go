package launchcode

import (
	"net/http"
	"strings"

	"ant-chrome/backend/internal/config"
)

// 代理池与内核能力。
//
// 与实例/自动化不同，这两个域此前没有 HTTP 层，proxy manager 也没有注入
// LaunchServer。这里沿用本包既有的「可选接口 + 类型断言」惯例（见 server.go），
// 宿主未实现时统一返回 Unavailable，而不是 panic 或静默返回空值。

// ProxyLister 可选接口：提供代理池列表。
type ProxyLister interface {
	ListProxyNodes() []config.BrowserProxy
}

// ProxySpeedTester 可选接口：对单个代理测速并持久化结果。
type ProxySpeedTester interface {
	TestProxyNodeSpeed(proxyID string) ProxySpeedResult
}

// ProxyHealthChecker 可选接口：检测代理出口 IP 健康度。
type ProxyHealthChecker interface {
	CheckProxyNodeHealth(proxyID string) ProxyHealthResult
}

// CoreLister 可选接口：提供浏览器内核列表。
type CoreLister interface {
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
	lister, ok := s.starter.(ProxyLister)
	if !ok {
		return nil, newServiceError(http.StatusServiceUnavailable, proxyAPIUnavailable)
	}
	return lister.ListProxyNodes(), nil
}

// TestProxySpeed 对指定代理测速。
// 注意这里会按当前连接栈启动桥接进程，属于耗时操作。
func (s *LaunchServer) TestProxySpeed(proxyID string) (*ProxySpeedResult, error) {
	proxyID = strings.TrimSpace(proxyID)
	if proxyID == "" {
		return nil, newServiceError(http.StatusBadRequest, "proxyId is required")
	}

	tester, ok := s.starter.(ProxySpeedTester)
	if !ok {
		return nil, newServiceError(http.StatusServiceUnavailable, proxyAPIUnavailable)
	}

	result := tester.TestProxyNodeSpeed(proxyID)
	return &result, nil
}

// CheckProxyHealth 检测代理出口 IP 的归属与风险信息。
func (s *LaunchServer) CheckProxyHealth(proxyID string) (*ProxyHealthResult, error) {
	proxyID = strings.TrimSpace(proxyID)
	if proxyID == "" {
		return nil, newServiceError(http.StatusBadRequest, "proxyId is required")
	}

	checker, ok := s.starter.(ProxyHealthChecker)
	if !ok {
		return nil, newServiceError(http.StatusServiceUnavailable, proxyAPIUnavailable)
	}

	result := checker.CheckProxyNodeHealth(proxyID)
	return &result, nil
}

// ListCores 返回已登记的浏览器内核。
func (s *LaunchServer) ListCores() ([]config.BrowserCore, error) {
	lister, ok := s.starter.(CoreLister)
	if !ok {
		return nil, newServiceError(http.StatusServiceUnavailable, "browser core api is unavailable")
	}
	return lister.ListBrowserCores(), nil
}
