package config

import (
	"os"
	"strings"
)

// DefaultWebviewGPUEnabled 是 app.webview_gpu 的默认取值。
//
// 默认为 false，也就是保持 1.9.0 及之前的行为：管理面板走软件渲染。
// 这样本开关是纯粹的 opt-in，不会因为升级改变任何现有用户的启动路径——
// 当初关闭 GPU 是为了规避 WebView2 进程在退出阶段挂起、以及安全软件拦截渲染器
// 导致的白屏，这条兼容性保证不能靠一次升级就推翻。
//
// 需要更流畅的磨砂动效和列表重绘时，显式配置 app.webview_gpu: true 即可。
// 软件渲染下的掉帧问题另由前端降级处理（GPU 关闭时不再使用逐帧 backdrop-filter
// 动画），所以默认路径同样不会再出现逐帧模糊带来的卡顿。
const DefaultWebviewGPUEnabled = false

// WebviewGPUEnvVar 是该开关的应急覆盖入口。
//
// 安装版把 config.yaml 落在安装目录时（例如 Program Files），普通用户不方便改文件，
// 可以用 ANT_BROWSER_WEBVIEW_GPU=0/1 临时覆盖，优先级高于配置文件。
const WebviewGPUEnvVar = "ANT_BROWSER_WEBVIEW_GPU"

// WebviewGPUSource 记录最终取值来自哪里，便于日志排查。
type WebviewGPUSource string

const (
	// WebviewGPUSourceEnv 表示被环境变量覆盖。
	WebviewGPUSourceEnv WebviewGPUSource = "env:" + WebviewGPUEnvVar
	// WebviewGPUSourceConfig 表示来自 config.yaml 的显式配置。
	WebviewGPUSourceConfig WebviewGPUSource = "config"
	// WebviewGPUSourceDefault 表示配置文件里没有该项，使用内置默认值。
	WebviewGPUSourceDefault WebviewGPUSource = "default"
)

// ResolveWebviewGPU 解析 GPU 加速开关的最终取值。
//
// 优先级：环境变量 > config.yaml > 内置默认值。
func ResolveWebviewGPU(app AppConfig) (bool, WebviewGPUSource) {
	return resolveWebviewGPU(app, os.Getenv)
}

func resolveWebviewGPU(app AppConfig, getenv func(string) string) (bool, WebviewGPUSource) {
	if getenv != nil {
		if enabled, ok := parseWebviewGPUBool(getenv(WebviewGPUEnvVar)); ok {
			return enabled, WebviewGPUSourceEnv
		}
	}
	if app.WebviewGPU != nil {
		return *app.WebviewGPU, WebviewGPUSourceConfig
	}
	return DefaultWebviewGPUEnabled, WebviewGPUSourceDefault
}

// parseWebviewGPUBool 接受常见的开关写法；无法识别时返回 ok=false，
// 让配置继续落到下一优先级，而不是把拼写错误当成“关闭”。
func parseWebviewGPUBool(raw string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return false, false
	case "1", "true", "yes", "on", "enable", "enabled":
		return true, true
	case "0", "false", "no", "off", "disable", "disabled":
		return false, true
	default:
		return false, false
	}
}
