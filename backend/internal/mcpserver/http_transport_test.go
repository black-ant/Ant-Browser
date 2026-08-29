package mcpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"ant-chrome/backend/internal/browser"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Streamable HTTP 传输的端到端测试。
//
// 前面的测试走内存传输，验证的是工具逻辑；这里起真实的 HTTP 服务，
// 验证 Handler() 产出的端点确实符合 MCP 传输规范，能被标准客户端接上。

func newHTTPTestServer(t *testing.T, provider Provider, opts Options) string {
	t.Helper()

	handler := New(provider, "test").Handler(opts)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server.URL
}

func connectOverHTTP(t *testing.T, endpoint string) *mcp.ClientSession {
	t.Helper()

	client := mcp.NewClient(&mcp.Implementation{Name: "http-test-client", Version: "v1"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint: endpoint,
	}, nil)
	if err != nil {
		t.Fatalf("connect over streamable http: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

// TestStreamableHTTPEndToEnd 覆盖 stdio 桥接实际走的那条链路：
// StreamableClientTransport -> HTTP -> Handler -> 工具。
func TestStreamableHTTPEndToEnd(t *testing.T) {
	provider := &fakeProvider{profiles: []browser.Profile{{ProfileId: "p1", ProfileName: "test"}}}
	endpoint := newHTTPTestServer(t, provider, Options{})
	session := connectOverHTTP(t, endpoint)

	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("list tools over http: %v", err)
	}
	if len(tools.Tools) != ToolCount() {
		t.Errorf("tool count over http = %d, want %d", len(tools.Tools), ToolCount())
	}

	result := callTool(t, session, "ant_instance_list", map[string]any{})
	if result.IsError {
		t.Fatalf("unexpected error: %s", contentText(result))
	}

	var out listInstancesOutput
	decodeStructured(t, result, &out)
	if out.Count != 1 {
		t.Errorf("count = %d, want 1", out.Count)
	}
}

// TestStatelessModeWorks 确认无状态模式下工具调用依然可用。
// 该模式没有会话，GET / DELETE 会返回 405，但 POST 调用必须正常。
func TestStatelessModeWorks(t *testing.T) {
	provider := &fakeProvider{profiles: []browser.Profile{{ProfileId: "p1", ProfileName: "test"}}}
	endpoint := newHTTPTestServer(t, provider, Options{Stateless: true})
	session := connectOverHTTP(t, endpoint)

	result := callTool(t, session, "ant_instance_list", map[string]any{})
	if result.IsError {
		t.Fatalf("unexpected error in stateless mode: %s", contentText(result))
	}
}

// TestRejectsNonLocalhostHostHeader 确认 SDK 自带的 DNS rebinding 防护生效。
// 这是把端点绑在本机时的一道重要防线：恶意网页无法通过 DNS 重绑定驱动本地服务。
func TestRejectsNonLocalhostHostHeader(t *testing.T) {
	endpoint := newHTTPTestServer(t, &fakeProvider{}, Options{})

	request, err := http.NewRequest(http.MethodPost, endpoint, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	request.Host = "evil.example.com"
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want 403 for a non-localhost Host header", response.StatusCode)
	}
}
