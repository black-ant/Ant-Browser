package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"

	"ant-chrome/backend"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCP stdio 桥接。
//
// 部分 MCP 客户端只支持 stdio 传输（把服务端当子进程拉起），无法直连 HTTP
// 端点。这个子命令就是那层适配：从 stdin 收 JSON-RPC 消息，转发到主程序的
// /mcp 端点，再把响应写回 stdout。
//
// 它不自己实现任何工具——工具只有一份，在主程序进程里，因为它们需要访问
// 浏览器实例、数据库和代理内核。桥接进程只负责搬运消息。

const (
	// mcpStdioFlag 是触发桥接模式的命令行参数。
	mcpStdioFlag = "--mcp-stdio"

	// connectTimeout 是连接主程序 MCP 端点的超时。
	// 主程序没启动时应尽快失败并给出可读提示，而不是让客户端一直干等。
	connectTimeout = 10 * time.Second
)

// isMCPStdioInvocation 判断本次启动是否为 stdio 桥接模式。
//
// 必须在获取单实例锁之前调用：桥接是短命的辅助进程，不应参与 GUI 单实例
// 竞争，否则会被判定为「已有实例」而直接退出。
func isMCPStdioInvocation(args []string) bool {
	for _, arg := range args {
		if strings.TrimSpace(arg) == mcpStdioFlag {
			return true
		}
	}
	return false
}

// runMCPStdioBridge 阻塞运行桥接，直到 stdin 关闭或收到中断信号。
// 返回进程退出码。
func runMCPStdioBridge(appRoot string) int {
	cfg, err := backend.LoadConfig(backend.ResolveRuntimePath(appRoot, "config.yaml"))
	if err != nil {
		cfg = backend.DefaultConfig()
	}

	endpoint := backend.ResolveMCPClientEndpoint(cfg)
	if !endpoint.Enabled {
		fmt.Fprintln(os.Stderr, "MCP 服务未启用。请在 Ant Browser 的「设置 > MCP 服务」中开启后重试。")
		return 1
	}

	// 信号与 stdin 关闭都应终止桥接：前者是用户中断，后者是客户端断开。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := bridgeStdioToHTTP(ctx, endpoint); err != nil {
		// 对端正常断开不是错误：客户端退出时会关闭 stdin。
		if errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) {
			return 0
		}
		fmt.Fprintf(os.Stderr, "MCP stdio 桥接结束：%v\n", err)
		return 1
	}
	return 0
}

// bridgeStdioToHTTP 在 stdio 与 HTTP 两个连接之间双向搬运 JSON-RPC 消息。
func bridgeStdioToHTTP(ctx context.Context, endpoint backend.MCPClientEndpoint) error {
	return bridgeToHTTP(ctx, endpoint, &mcp.StdioTransport{})
}

// bridgeToHTTP 是桥接的核心实现，下游传输可替换以便测试。
//
// 这里刻意工作在 jsonrpc 消息层而不是 MCP 语义层：桥接不需要理解消息内容，
// 透传能天然兼容后续的协议演进，也避免中间多一次语义转换。
func bridgeToHTTP(ctx context.Context, endpoint backend.MCPClientEndpoint, downstreamTransport mcp.Transport) error {
	// 先做可达性预检。StreamableClientTransport.Connect 不产生网络 I/O，
	// 真正的失败要等到客户端发出第一条消息才暴露；没有预检时，
	// 主程序未运行的场景会一直静默等待，而不是立刻给出可读提示。
	if err := probeEndpoint(ctx, endpoint.URL); err != nil {
		return err
	}

	httpTransport := &mcp.StreamableClientTransport{
		Endpoint:   endpoint.URL,
		HTTPClient: newBridgeHTTPClient(endpoint.AuthHeader, endpoint.AuthValue),
	}

	connectCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()

	upstream, err := httpTransport.Connect(connectCtx)
	if err != nil {
		return fmt.Errorf("无法连接 Ant Browser MCP 服务（%s）：%w；请确认 Ant Browser 正在运行", endpoint.URL, err)
	}
	defer upstream.Close()

	downstream, err := downstreamTransport.Connect(ctx)
	if err != nil {
		return fmt.Errorf("初始化 stdio 传输失败：%w", err)
	}
	defer downstream.Close()

	return pumpBothDirections(ctx, downstream, upstream)
}

// probeEndpoint 检查目标端口是否有服务在监听。
//
// 只做 TCP 连通性检查，不发 HTTP 请求：这里要区分的是「主程序没运行」
// 和「主程序在运行但 MCP 有问题」，后者应当由正常的协议错误来表达。
func probeEndpoint(ctx context.Context, rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("MCP 端点地址无效（%s）：%w", rawURL, err)
	}

	host := parsed.Host
	if parsed.Port() == "" {
		host = net.JoinHostPort(parsed.Hostname(), "80")
	}

	dialCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()

	conn, err := (&net.Dialer{}).DialContext(dialCtx, "tcp", host)
	if err != nil {
		return fmt.Errorf("无法连接 Ant Browser MCP 服务（%s）：%w；请确认 Ant Browser 正在运行，且已在「设置 > MCP 服务」中开启", rawURL, err)
	}
	return conn.Close()
}

// pumpBothDirections 并发搬运两个方向的消息，任一方向结束即返回。
func pumpBothDirections(ctx context.Context, downstream, upstream mcp.Connection) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// 缓冲为 2，保证后结束的那个 goroutine 也能写入而不泄漏。
	errs := make(chan error, 2)
	go func() { errs <- pump(ctx, downstream, upstream) }()
	go func() { errs <- pump(ctx, upstream, downstream) }()

	select {
	case err := <-errs:
		// 一端关闭后另一端继续搬运没有意义，defer 的 cancel 会让它退出。
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func pump(ctx context.Context, from, to mcp.Connection) error {
	for {
		msg, err := from.Read(ctx)
		if err != nil {
			return err
		}
		if err := to.Write(ctx, msg); err != nil {
			return err
		}
	}
}

// newBridgeHTTPClient 构造带鉴权头的 HTTP 客户端。
//
// 不设 Timeout：MCP 的 SSE 是长连接，整体超时会把正常的流式响应掐断。
// 连接阶段的超时由调用方的 context 控制。
func newBridgeHTTPClient(header, value string) *http.Client {
	if header == "" || value == "" {
		return http.DefaultClient
	}
	return &http.Client{
		Transport: &authRoundTripper{header: header, value: value, base: http.DefaultTransport},
	}
}

type authRoundTripper struct {
	header string
	value  string
	base   http.RoundTripper
}

func (rt *authRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// RoundTripper 约定不得修改入参请求，因此克隆后再加头。
	cloned := req.Clone(req.Context())
	cloned.Header.Set(rt.header, rt.value)
	return rt.base.RoundTrip(cloned)
}
