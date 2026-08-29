package backend

import (
	"fmt"
	"strings"

	"ant-chrome/backend/internal/config"
)

// MCP 客户端连接信息。
//
// 这些解析逻辑放在 backend 包而不是 main 包：config 是 internal 包，
// 根包无法直接引用；同时也让「端点地址怎么算」在服务端和桥接端只有一份实现。

// MCPClientEndpoint 描述连接本机 MCP 服务所需的信息。
type MCPClientEndpoint struct {
	// Enabled 为 false 时其余字段无意义。
	Enabled bool
	// URL 是 MCP 端点的完整地址。
	URL string
	// AuthHeader / AuthValue 为空表示无需鉴权。
	AuthHeader string
	AuthValue  string
}

// ResolveMCPClientEndpoint 从配置推导 MCP 端点信息。
//
// 注意这里用的是配置里的首选端口，而不是 LaunchServer 实际绑定的端口。
// 两者在正常情况下一致；不一致说明首选端口被占用导致主程序启动失败，
// 此时连接失败并给出提示，比连到一个非预期端口更安全。
func ResolveMCPClientEndpoint(cfg *Config) MCPClientEndpoint {
	if cfg == nil || !cfg.MCP.Enabled {
		return MCPClientEndpoint{}
	}

	port := cfg.LaunchServer.Port
	if port <= 0 {
		port = config.DefaultLaunchServerPort
	}

	path := strings.TrimSpace(cfg.MCP.Path)
	if path == "" {
		path = config.DefaultMCPPath
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	path = strings.TrimRight(path, "/")
	if path == "" {
		path = config.DefaultMCPPath
	}

	endpoint := MCPClientEndpoint{
		Enabled: true,
		URL:     fmt.Sprintf("http://127.0.0.1:%d%s", port, path),
	}

	if cfg.LaunchServer.Auth.Enabled {
		if key := strings.TrimSpace(cfg.LaunchServer.Auth.APIKey); key != "" {
			header := strings.TrimSpace(cfg.LaunchServer.Auth.Header)
			if header == "" {
				header = config.DefaultLaunchServerAPIKeyHeader
			}
			endpoint.AuthHeader = header
			endpoint.AuthValue = key
		}
	}

	return endpoint
}
