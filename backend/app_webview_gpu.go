package backend

import (
	goruntime "runtime"
	"strings"

	appconfig "ant-chrome/backend/internal/config"

	"github.com/wailsapp/wails/v2/pkg/options/linux"
)

// WebviewGPUStatus 是管理面板 WebView 硬件加速的当前状态。
//
// 这个开关只在创建窗口前生效，修改 config.yaml 后必须重启应用才会生效，
// 所以状态里固定带 RestartRequired，避免前端把它当成可热切换项。
type WebviewGPUStatus struct {
	// Enabled 是配置解析出来的期望值。
	// 当 Applicable 为 false（macOS）时它不产生任何效果，实际渲染路径见 Policy。
	Enabled bool `json:"enabled"`
	// Applicable 表示当前平台是否支持该开关：Windows / Linux 为 true，macOS 为 false。
	Applicable bool `json:"applicable"`
	// Source 记录取值来源：env:ANT_BROWSER_WEBVIEW_GPU / config / default。
	Source string `json:"source"`
	// Policy 是落到具体平台后的实际策略名，便于和 lifecycle 日志对照。
	Policy string `json:"policy"`
	// Platform 是当前系统；macOS 的 GPU 策略由 WKWebView 托管，该开关不生效。
	Platform string `json:"platform"`
	// RestartRequired 表示该项需要重启应用才会生效。
	RestartRequired bool `json:"restartRequired"`
}

// ResolveWebviewGPU 解析 app.webview_gpu 的最终取值（环境变量 > config.yaml > 默认）。
//
// 平台差异只影响 Applicable / Policy：配置本身在三个平台上都能解析出来，
// 但只有 Windows（WebView2）和 Linux（WebKitGTK）会真正作用到渲染路径。
func ResolveWebviewGPU(app appconfig.AppConfig) WebviewGPUStatus {
	enabled, source := appconfig.ResolveWebviewGPU(app)
	return buildWebviewGPUStatus(enabled, source, goruntime.GOOS)
}

// GetWebviewGPUStatus 返回当前 GPU 加速开关状态，供设置页展示和排障使用。
func (a *App) GetWebviewGPUStatus() WebviewGPUStatus {
	if a == nil || a.config == nil {
		enabled, source := appconfig.ResolveWebviewGPU(appconfig.AppConfig{})
		return buildWebviewGPUStatus(enabled, source, goruntime.GOOS)
	}
	return ResolveWebviewGPU(a.config.App)
}

// WebviewGPUPolicy 把开关映射为 WebKitGTK 的硬件加速策略（仅 Linux 使用）。
//
// Wails 在 Linux 上有已知问题（wails#2977）：不显式设置 Options 时
// WebviewGpuPolicy 默认就是 Never，也就是永远软件渲染；这里显式给出两种取值，
// 避免“配置写了 true 但实际仍是软件渲染”。
func WebviewGPUPolicy(enabled bool) linux.WebviewGpuPolicy {
	if enabled {
		return linux.WebviewGpuPolicyOnDemand
	}
	return linux.WebviewGpuPolicyNever
}

func buildWebviewGPUStatus(enabled bool, source appconfig.WebviewGPUSource, goos string) WebviewGPUStatus {
	return WebviewGPUStatus{
		Enabled:         enabled,
		Applicable:      webviewGPUApplicable(goos),
		Source:          string(source),
		Policy:          webviewGPUPolicyName(goos, enabled),
		Platform:        goos,
		RestartRequired: true,
	}
}

// webviewGPUApplicable 说明该平台是否支持进程级 GPU 开关。
// macOS 的 WKWebView 不提供这一层控制，故返回 false。
func webviewGPUApplicable(goos string) bool {
	switch strings.ToLower(strings.TrimSpace(goos)) {
	case "windows", "linux":
		return true
	default:
		return false
	}
}

func webviewGPUPolicyName(goos string, enabled bool) string {
	switch strings.ToLower(strings.TrimSpace(goos)) {
	case "windows":
		if enabled {
			return "webview2-gpu"
		}
		return "webview2-software"
	case "linux":
		if enabled {
			return "webkitgtk-on-demand"
		}
		return "webkitgtk-never"
	case "darwin":
		// WKWebView 不接受进程级 GPU 开关，由系统按硬件与省电策略决定。
		return "system-managed"
	default:
		return "unknown"
	}
}
