package launchcode

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// MCP 端点挂载与鉴权的回归测试。
//
// 重点保护两条容易被改坏的性质：
//  1. /mcp 必须显式注册，否则会被兜底路由当成 CDP 请求转发到 Chrome
//  2. /mcp 必须纳入 API Key 鉴权，否则等于绕过鉴权的后门

func newMCPTestServer(t *testing.T, mcpPath string) *LaunchServer {
	t.Helper()

	server := NewLaunchServer(nil, nil, nil, 0)
	if mcpPath != "" {
		server.SetMCPHandler(mcpPath, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("mcp-ok"))
		}))
	}
	return server
}

func TestMCPEndpointIsRoutedNotProxiedToCDP(t *testing.T) {
	server := newMCPTestServer(t, "/mcp")
	handler := NewTestHandler(server)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/mcp", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; the MCP handler was not reached", recorder.Code)
	}
	if body := recorder.Body.String(); body != "mcp-ok" {
		t.Errorf("body = %q, want mcp-ok", body)
	}
}

// TestUnmountedMCPPathFallsThroughToCDP 确认未启用 MCP 时行为不变：
// 该路径仍归兜底的 CDP 反向代理处理（无活动实例时返回 503）。
func TestUnmountedMCPPathFallsThroughToCDP(t *testing.T) {
	server := newMCPTestServer(t, "")
	handler := NewTestHandler(server)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/mcp", nil))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503 from the CDP fallback", recorder.Code)
	}
}

// TestMCPEndpointRequiresAPIKey 是本次改动的安全关键点。
// 鉴权中间件原本只覆盖 /api/ 前缀，若不扩展，/mcp 将完全不受保护。
func TestMCPEndpointRequiresAPIKey(t *testing.T) {
	server := newMCPTestServer(t, "/mcp")
	server.SetAPIAuthConfig(APIAuthConfig{Enabled: true, APIKey: "secret-key"})
	handler := NewTestHandler(server)

	tests := []struct {
		name       string
		key        string
		wantStatus int
	}{
		{"no key", "", http.StatusUnauthorized},
		{"wrong key", "wrong-key", http.StatusUnauthorized},
		{"correct key", "secret-key", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/mcp", nil)
			if tt.key != "" {
				request.Header.Set(DefaultAPIKeyHeader, tt.key)
			}

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			if recorder.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", recorder.Code, tt.wantStatus)
			}
		})
	}
}

// TestAPIAuthStillCoversAPIPrefix 确认扩展鉴权范围没有回退原有保护。
func TestAPIAuthStillCoversAPIPrefix(t *testing.T) {
	server := newMCPTestServer(t, "/mcp")
	server.SetAPIAuthConfig(APIAuthConfig{Enabled: true, APIKey: "secret-key"})
	handler := NewTestHandler(server)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if recorder.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 for /api/health without a key", recorder.Code)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	request.Header.Set(DefaultAPIKeyHeader, "secret-key")
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 for /api/health with a valid key", recorder.Code)
	}
}

// TestCDPProxyRemainsUnauthenticated 固化既有行为：
// 兜底的 CDP 反向代理不参与 API Key 鉴权，只受 localhost 限制保护。
// 这不是新引入的问题，但值得用测试写明，避免被误认为遗漏。
func TestCDPProxyRemainsUnauthenticated(t *testing.T) {
	server := newMCPTestServer(t, "/mcp")
	server.SetAPIAuthConfig(APIAuthConfig{Enabled: true, APIKey: "secret-key"})
	handler := NewTestHandler(server)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/json/version", nil))

	// 未鉴权即可抵达代理逻辑；没有活动实例所以是 503 而不是 401。
	if recorder.Code == http.StatusUnauthorized {
		t.Error("CDP proxy is now behind API auth; this changes existing behavior")
	}
}

func TestRequiresAPIAuthPathMatching(t *testing.T) {
	server := newMCPTestServer(t, "/mcp")

	tests := []struct {
		path string
		want bool
	}{
		{"/api/health", true},
		{"/api/launch", true},
		{"/mcp", true},
		{"/mcp/", true},
		{"/mcp/messages", true},
		{"/mcpsomething", false}, // 前缀相似但不是子路径，不应被误纳入
		{"/json/version", false},
		{"/", false},
	}

	for _, tt := range tests {
		if got := server.requiresAPIAuth(tt.path); got != tt.want {
			t.Errorf("requiresAPIAuth(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestNormalizeMountPath(t *testing.T) {
	tests := []struct{ input, want string }{
		{"/mcp", "/mcp"},
		{"mcp", "/mcp"},
		{"/mcp/", "/mcp"},
		{"  /mcp  ", "/mcp"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := normalizeMountPath(tt.input); got != tt.want {
			t.Errorf("normalizeMountPath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// TestMCPURLReflectsMountState 确认设置页拿到的地址与实际挂载状态一致。
func TestMCPURLReflectsMountState(t *testing.T) {
	server := newMCPTestServer(t, "")
	if url := server.MCPURL(); url != "" {
		t.Errorf("MCPURL() = %q, want empty when MCP is disabled", url)
	}

	server.SetMCPHandler("/mcp", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	if path := server.MCPPath(); path != "/mcp" {
		t.Errorf("MCPPath() = %q, want /mcp", path)
	}
	// 服务未 Start 时端口为 0，此时不应返回半成品地址。
	if url := server.MCPURL(); url != "" && !strings.HasPrefix(url, "http://127.0.0.1:") {
		t.Errorf("MCPURL() = %q, want empty or a loopback URL", url)
	}
}
