// Package mcpserver 把 Ant Browser 的本地能力暴露为 MCP（Model Context Protocol）工具。
//
// 设计原则：MCP 不是第二套 API，而是现有能力的第二种表达方式。
// 所有工具都通过 launchcode.LaunchServer 的服务门面调用既有业务逻辑，
// 不新增执行链路，避免两套语义漂移。
//
// 传输形态有两种，共用同一个 *mcp.Server：
//   - Streamable HTTP：挂在 LaunchServer 的 /mcp 路径上，复用其端口、
//     localhost 限制和 API Key 鉴权
//   - stdio：由 `ant-chrome --mcp-stdio` 子命令桥接到上面的 HTTP 端点，
//     用于只支持 stdio 的客户端
package mcpserver

import (
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	// ServerName 是 MCP initialize 握手时上报的实现名。
	ServerName = "ant-browser"

	// DefaultPath 是 MCP 端点默认挂载路径。
	DefaultPath = "/mcp"

	// sessionTimeout 控制空闲会话的回收时间。
	//
	// LaunchServer 的 http.Server 没有设置任何超时（见 launchcode/server.go），
	// 这对 SSE 长连接是好事，但意味着空闲会话不会被自动清理；而 Stop() 只有
	// 5 秒优雅关闭窗口。这里主动设一个上限，避免残留会话拖住关闭流程。
	sessionTimeout = 30 * time.Minute
)

// Provider 是 MCP 工具依赖的能力集合。
//
// 用接口而不是直接依赖 *launchcode.LaunchServer，是为了让测试能注入
// 假实现——真实实现需要 SQLite、浏览器进程和代理内核，无法在单测里构造。
type Provider interface {
	InstanceProvider
	AutomationProvider
	ProxyProvider
}

// Server 持有 MCP 协议层与能力提供方。
type Server struct {
	provider Provider
	version  string
}

// New 创建 MCP 服务。version 会在 initialize 握手里上报给客户端。
func New(provider Provider, version string) *Server {
	if version == "" {
		version = "unknown"
	}
	return &Server{provider: provider, version: version}
}

// buildMCPServer 构造一个注册好全部工具的 *mcp.Server。
//
// 每个 HTTP 会话都会调用一次：SDK 的 session 状态挂在 *mcp.Server 上，
// 复用同一个实例会让并发会话互相干扰。工具注册本身是纯内存操作，开销可忽略。
func (s *Server) buildMCPServer() *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{
		Name:    ServerName,
		Version: s.version,
	}, nil)

	registerInstanceTools(srv, s.provider)
	registerAutomationTools(srv, s.provider)
	registerProxyTools(srv, s.provider)

	return srv
}

// Options 控制 MCP 传输层行为。
type Options struct {
	// Stateless 为 true 时不维护会话状态，每个请求独立处理。
	// 此时 GET / DELETE 返回 405，也无法接收服务端主动推送的消息。
	Stateless bool
}

// Handler 返回 Streamable HTTP 形态的 http.Handler，用于挂载到 LaunchServer。
func (s *Server) Handler(opts Options) http.Handler {
	return mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return s.buildMCPServer() },
		&mcp.StreamableHTTPOptions{
			Stateless:      opts.Stateless,
			SessionTimeout: sessionTimeout,
		},
	)
}

// MCPServer 暴露底层 *mcp.Server，供 stdio 传输和测试直接使用。
func (s *Server) MCPServer() *mcp.Server {
	return s.buildMCPServer()
}

// ToolCount 返回对外暴露的工具数量，供设置页展示。
// SDK 未提供枚举已注册工具的公开 API，这里维护一个常量；
// TestToolsAreRegistered 会断言实际数量与之一致，防止漏改。
func ToolCount() int {
	return toolCount
}

// toolCount 是当前注册的工具总数。新增或删除工具时必须同步更新。
const toolCount = 18
