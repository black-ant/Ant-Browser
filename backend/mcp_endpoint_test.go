package backend

import (
	"testing"

	"ant-chrome/backend/internal/config"
)

func TestResolveMCPClientEndpoint(t *testing.T) {
	tests := []struct {
		name       string
		cfg        *Config
		wantURL    string
		wantHeader string
		wantValue  string
		wantOff    bool
	}{
		{
			name:    "nil config",
			cfg:     nil,
			wantOff: true,
		},
		{
			name:    "disabled",
			cfg:     &Config{MCP: config.MCPConfig{Enabled: false}},
			wantOff: true,
		},
		{
			name: "defaults applied when unset",
			cfg: &Config{
				MCP: config.MCPConfig{Enabled: true},
			},
			wantURL: "http://127.0.0.1:19876/mcp",
		},
		{
			name: "custom port and path",
			cfg: &Config{
				LaunchServer: config.LaunchServerConfig{Port: 20000},
				MCP:          config.MCPConfig{Enabled: true, Path: "/custom"},
			},
			wantURL: "http://127.0.0.1:20000/custom",
		},
		{
			name: "path normalized",
			cfg: &Config{
				LaunchServer: config.LaunchServerConfig{Port: 19876},
				MCP:          config.MCPConfig{Enabled: true, Path: "mcp/"},
			},
			wantURL: "http://127.0.0.1:19876/mcp",
		},
		{
			name: "auth propagated",
			cfg: &Config{
				LaunchServer: config.LaunchServerConfig{
					Port: 19876,
					Auth: config.LaunchServerAuthConfig{Enabled: true, APIKey: "k", Header: "X-Custom"},
				},
				MCP: config.MCPConfig{Enabled: true},
			},
			wantURL:    "http://127.0.0.1:19876/mcp",
			wantHeader: "X-Custom",
			wantValue:  "k",
		},
		{
			name: "auth header defaults when blank",
			cfg: &Config{
				LaunchServer: config.LaunchServerConfig{
					Port: 19876,
					Auth: config.LaunchServerAuthConfig{Enabled: true, APIKey: "k"},
				},
				MCP: config.MCPConfig{Enabled: true},
			},
			wantURL:    "http://127.0.0.1:19876/mcp",
			wantHeader: config.DefaultLaunchServerAPIKeyHeader,
			wantValue:  "k",
		},
		{
			// 开启鉴权但没配 key 时服务端是 fail-open 的（见 launchcode/auth.go），
			// 桥接端必须保持一致，否则会带上空 header 造成困惑。
			name: "auth enabled without key is ignored",
			cfg: &Config{
				LaunchServer: config.LaunchServerConfig{
					Port: 19876,
					Auth: config.LaunchServerAuthConfig{Enabled: true, APIKey: ""},
				},
				MCP: config.MCPConfig{Enabled: true},
			},
			wantURL: "http://127.0.0.1:19876/mcp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveMCPClientEndpoint(tt.cfg)

			if tt.wantOff {
				if got.Enabled {
					t.Fatalf("Enabled = true, want false")
				}
				return
			}
			if !got.Enabled {
				t.Fatalf("Enabled = false, want true")
			}
			if got.URL != tt.wantURL {
				t.Errorf("URL = %q, want %q", got.URL, tt.wantURL)
			}
			if got.AuthHeader != tt.wantHeader {
				t.Errorf("AuthHeader = %q, want %q", got.AuthHeader, tt.wantHeader)
			}
			if got.AuthValue != tt.wantValue {
				t.Errorf("AuthValue = %q, want %q", got.AuthValue, tt.wantValue)
			}
		})
	}
}
