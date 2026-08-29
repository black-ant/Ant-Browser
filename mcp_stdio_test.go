package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ant-chrome/backend"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// stdio 桥接的端到端测试。
//
// 桥接把客户端的 stdio 连接转发到主程序的 HTTP 端点，本身不理解 MCP 语义，
// 只在 jsonrpc 消息层搬运。因此这里用一个最小的 MCP 服务做对端即可验证链路；
// 真实工具集的行为由 internal/mcpserver 的测试覆盖。
//
// 客户端一侧用内存传输代替真实的 stdin/stdout（两者都实现 mcp.Transport），
// 从而不必真的起子进程。

func TestIsMCPStdioInvocation(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{"no args", nil, false},
		{"flag present", []string{"--mcp-stdio"}, true},
		{"flag among others", []string{"-foo", "--mcp-stdio"}, true},
		{"flag with spaces", []string{"  --mcp-stdio  "}, true},
		{"unrelated flag", []string{"--mcp"}, false},
		{"similar prefix", []string{"--mcp-stdio-extra"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isMCPStdioInvocation(tt.args); got != tt.want {
				t.Errorf("isMCPStdioInvocation(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

// newUpstreamServer 起一个最小 MCP 服务作为桥接的上游。
func newUpstreamServer(t *testing.T) string {
	t.Helper()

	type echoInput struct {
		Text string `json:"text"`
	}
	type echoOutput struct {
		Echo string `json:"echo"`
	}

	build := func() *mcp.Server {
		srv := mcp.NewServer(&mcp.Implementation{Name: "upstream", Version: "v1"}, nil)
		mcp.AddTool(srv, &mcp.Tool{Name: "echo", Description: "echo back the input"},
			func(_ context.Context, _ *mcp.CallToolRequest, in echoInput) (*mcp.CallToolResult, echoOutput, error) {
				return nil, echoOutput{Echo: in.Text}, nil
			})
		return srv
	}

	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return build() }, nil)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server.URL
}

// TestBridgeForwardsMessagesBothWays 验证请求能到上游、响应能回到客户端。
func TestBridgeForwardsMessagesBothWays(t *testing.T) {
	upstreamURL := newUpstreamServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	clientTransport, bridgeTransport := mcp.NewInMemoryTransports()
	go func() {
		_ = bridgeToHTTP(ctx, backend.MCPClientEndpoint{Enabled: true, URL: upstreamURL}, bridgeTransport)
	}()

	client := mcp.NewClient(&mcp.Implementation{Name: "bridge-test", Version: "v1"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("connect through bridge: %v", err)
	}
	defer session.Close()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("list tools through bridge: %v", err)
	}
	if len(tools.Tools) != 1 || tools.Tools[0].Name != "echo" {
		t.Fatalf("tools through bridge = %+v, want a single echo tool", tools.Tools)
	}

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "echo",
		Arguments: map[string]any{"text": "hello"},
	})
	if err != nil {
		t.Fatalf("call tool through bridge: %v", err)
	}
	if result.IsError {
		t.Fatalf("tool call through bridge failed: %v", result.Content)
	}

	structured, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("structured content type = %T, want map", result.StructuredContent)
	}
	if structured["echo"] != "hello" {
		t.Errorf("echo = %v, want hello; the response did not survive the round trip", structured["echo"])
	}
}

// TestBridgeReportsUnreachableUpstream 确认主程序未运行时快速失败，
// 而不是让客户端一直等待。
func TestBridgeReportsUnreachableUpstream(t *testing.T) {
	// 起一个立刻关闭的服务，借此拿到一个确定无人监听的地址。
	server := httptest.NewServer(nil)
	deadURL := server.URL
	server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, bridgeTransport := mcp.NewInMemoryTransports()
	err := bridgeToHTTP(ctx, backend.MCPClientEndpoint{Enabled: true, URL: deadURL}, bridgeTransport)
	if err == nil {
		t.Fatal("err = nil, want a connection failure")
	}
	if ctx.Err() != nil {
		t.Error("bridge blocked until the test context expired instead of failing fast")
	}
}
