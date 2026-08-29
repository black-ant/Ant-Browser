package backend

import (
	"fmt"
	"os"
	"path/filepath"

	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/launchcode"
	"ant-chrome/backend/internal/mcpserver"
)

// MCP 服务的装配与设置。
//
// MCP 端点挂在 LaunchServer 上，因此没有独立的启停生命周期：
// 开关变化通过重建 handler 链生效，而重建 handler 链需要重启 LaunchServer。

// applyMCPConfig 按当前配置把 MCP 端点挂到 LaunchServer 上。
// 必须在 LaunchServer.Start() 之前调用。
func (a *App) applyMCPConfig() {
	if a.launchServer == nil {
		return
	}
	applyMCPConfigTo(a.launchServer, a.config, a.appVersion())
}

// applyMCPConfigTo 是不依赖 App 状态的装配函数，
// 便于 restartLaunchServer 在替换 server 实例时复用。
func applyMCPConfigTo(server *launchcode.LaunchServer, cfg *config.Config, version string) {
	if server == nil {
		return
	}
	if cfg == nil || !cfg.MCP.Enabled {
		server.SetMCPHandler("", nil)
		return
	}

	path := cfg.MCP.Path
	if path == "" {
		path = config.DefaultMCPPath
	}
	server.SetMCPHandler(path, mcpserver.New(server, version).Handler(mcpserver.Options{
		Stateless: cfg.MCP.Stateless,
	}))
}

// currentExecutablePath 返回当前进程的可执行文件路径。
// 取不到时返回空串，由前端展示占位提示。
func currentExecutablePath() string {
	path, err := os.Executable()
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}

// GetMCPServerInfo 返回 MCP 服务的当前状态（Wails 绑定）。
func (a *App) GetMCPServerInfo() map[string]interface{} {
	enabled := false
	path := config.DefaultMCPPath
	stateless := false
	if a.config != nil {
		enabled = a.config.MCP.Enabled
		stateless = a.config.MCP.Stateless
		if a.config.MCP.Path != "" {
			path = a.config.MCP.Path
		}
	}

	info := map[string]interface{}{
		"enabled":   enabled,
		"path":      path,
		"stateless": stateless,
		"ready":     false,
		"url":       "",
		"toolCount": mcpserver.ToolCount(),
		// 设置页要据此生成 stdio 客户端配置；由后端提供避免前端拼路径出错。
		"executablePath": currentExecutablePath(),
	}

	if a.launchServer != nil {
		if url := a.launchServer.MCPURL(); url != "" {
			info["ready"] = true
			info["url"] = url
		}
		// MCP 与 /api/* 共用同一套鉴权，前端据此提示用户要不要带 header。
		info["authEnabled"] = a.launchServer.APIAuthEnabled()
		info["authHeader"] = a.launchServer.APIAuthHeader()
	} else {
		info["authEnabled"] = false
		info["authHeader"] = launchcode.DefaultAPIKeyHeader
	}

	return info
}

// SaveMCPSettings 保存 MCP 开关并立即生效（Wails 绑定）。
//
// 开关变化需要重建 LaunchServer 的 handler 链，因此这里会重启 LaunchServer。
// 端口不变，重启对现有 /api/* 调用方是短暂不可用，不是地址变更。
func (a *App) SaveMCPSettings(enabled bool) (map[string]interface{}, error) {
	if a.config == nil {
		return nil, fmt.Errorf("mcp config is not initialized")
	}
	if a.config.MCP.Enabled == enabled {
		return a.GetMCPServerInfo(), nil
	}

	previous := a.config.MCP.Enabled
	a.config.MCP.Enabled = enabled
	if err := a.config.Save(a.resolveAppPath("config.yaml")); err != nil {
		a.config.MCP.Enabled = previous
		return nil, err
	}

	port := a.config.LaunchServer.Port
	if a.launchServer != nil {
		port = a.launchServer.Port()
	}
	if err := a.restartLaunchServer(port); err != nil {
		a.config.MCP.Enabled = previous
		_ = a.config.Save(a.resolveAppPath("config.yaml"))
		return nil, err
	}

	return a.GetMCPServerInfo(), nil
}
