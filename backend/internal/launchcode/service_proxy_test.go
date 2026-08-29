package launchcode

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"ant-chrome/backend/internal/config"
)

// 代理池与内核能力的注入测试。
//
// 这组能力与实例/自动化不同：走的是显式注入的 ProxyProvider，
// 而不是对 starter 做类型断言（见 service_proxy.go 顶部说明）。

type fakeProxyProvider struct {
	proxies []config.BrowserProxy
	cores   []config.BrowserCore
}

func (f fakeProxyProvider) ListProxyNodes() []config.BrowserProxy { return f.proxies }

func (f fakeProxyProvider) TestProxyNodeSpeed(id string) ProxySpeedResult {
	return ProxySpeedResult{ProxyID: id, Ok: true, LatencyMs: 42}
}

func (f fakeProxyProvider) CheckProxyNodeHealth(id string) ProxyHealthResult {
	return ProxyHealthResult{ProxyID: id, Ok: true, IP: "1.2.3.4"}
}

func (f fakeProxyProvider) ListBrowserCores() []config.BrowserCore { return f.cores }

// TestProxyToolsUnavailableWithoutProvider 确认未注入时返回可识别的
// Unavailable 错误，而不是 panic 或静默返回空列表。
func TestProxyToolsUnavailableWithoutProvider(t *testing.T) {
	server := NewLaunchServer(nil, nil, nil, 0)

	if _, err := server.ListProxies(); err == nil {
		t.Error("ListProxies err = nil, want unavailable")
	} else if svcErr, ok := err.(*ServiceError); !ok || !svcErr.Unavailable() {
		t.Errorf("ListProxies err = %v, want a ServiceError reporting unavailable", err)
	}

	if _, err := server.ListCores(); err == nil {
		t.Error("ListCores err = nil, want unavailable")
	}
	if _, err := server.TestProxySpeed("p1"); err == nil {
		t.Error("TestProxySpeed err = nil, want unavailable")
	}
	if _, err := server.CheckProxyHealth("p1"); err == nil {
		t.Error("CheckProxyHealth err = nil, want unavailable")
	}
}

func TestProxyToolsUseInjectedProvider(t *testing.T) {
	server := NewLaunchServer(nil, nil, nil, 0)
	server.SetProxyProvider(fakeProxyProvider{
		proxies: []config.BrowserProxy{{ProxyId: "p1", ProxyName: "Node A"}},
		cores:   []config.BrowserCore{{CoreId: "c1", CoreName: "Chrome 142"}},
	})

	proxies, err := server.ListProxies()
	if err != nil {
		t.Fatalf("ListProxies: %v", err)
	}
	if len(proxies) != 1 || proxies[0].ProxyId != "p1" {
		t.Errorf("proxies = %+v, want the injected node", proxies)
	}

	cores, err := server.ListCores()
	if err != nil {
		t.Fatalf("ListCores: %v", err)
	}
	if len(cores) != 1 || cores[0].CoreId != "c1" {
		t.Errorf("cores = %+v, want the injected core", cores)
	}

	speed, err := server.TestProxySpeed("p1")
	if err != nil {
		t.Fatalf("TestProxySpeed: %v", err)
	}
	if !speed.Ok || speed.LatencyMs != 42 {
		t.Errorf("speed = %+v, want the provider result", speed)
	}
}

// TestProxyIDIsRequired 确认空 proxyId 走参数校验而不是打到 provider。
func TestProxyIDIsRequired(t *testing.T) {
	server := NewLaunchServer(nil, nil, nil, 0)
	server.SetProxyProvider(fakeProxyProvider{})

	if _, err := server.TestProxySpeed("  "); err == nil {
		t.Error("TestProxySpeed err = nil, want a bad-request error")
	} else if svcErr, ok := err.(*ServiceError); !ok || svcErr.Status != http.StatusBadRequest {
		t.Errorf("err = %v, want 400", err)
	}
}

// TestMCPHandlerSurvivesServerLifecycle 覆盖装配顺序：
// SetMCPHandler 必须在 Start 之前调用，handler 链在 Start 时一次性构建。
// 这条约束靠注释无法保证，用一次真实的 Start/Stop 来锁住。
func TestMCPHandlerSurvivesServerLifecycle(t *testing.T) {
	server := NewLaunchServer(nil, nil, nil, 0) // port<=0 -> 随机端口
	server.SetMCPHandler("/mcp", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	if err := server.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { _ = server.Stop() })

	url := server.MCPURL()
	if url == "" {
		t.Fatal("MCPURL() is empty after Start")
	}

	request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("request the mounted MCP endpoint: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusTeapot {
		t.Errorf("status = %d, want 418; the handler was not reached through the real listener", response.StatusCode)
	}
}

// TestHandlerMountedAfterStartIsIgnored 固化上面那条顺序约束的反面：
// Start 之后再挂载不会生效，避免有人误以为可以热插拔。
func TestHandlerMountedAfterStartIsIgnored(t *testing.T) {
	server := NewLaunchServer(nil, nil, nil, 0)
	if err := server.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { _ = server.Stop() })

	server.SetMCPHandler("/mcp", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	response, err := http.Post(server.CDPURL()+"/mcp", "application/json", nil)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer response.Body.Close()

	// 未生效时该路径归兜底的 CDP 代理处理，没有活动实例所以是 503。
	if response.StatusCode == http.StatusTeapot {
		t.Error("handler mounted after Start took effect; the documented order constraint no longer holds")
	}
}

// TestNewTestHandlerSkipsLocalhostGuard 记录测试辅助函数的既有语义。
func TestNewTestHandlerSkipsLocalhostGuard(t *testing.T) {
	server := NewLaunchServer(nil, nil, nil, 0)
	handler := NewTestHandler(server)

	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	request.RemoteAddr = "10.0.0.5:1234"

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("status = %d, want 200; NewTestHandler should bypass the localhost guard", recorder.Code)
	}
}
