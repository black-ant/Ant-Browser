package backend

import (
	"time"

	"ant-chrome/backend/internal/cdp"
)

// app_cdp_transport.go 统一管理 CDP 传输方式（调试端口 vs --remote-debugging-pipe）。
//
// 默认 "port"：全平台沿用调试端口（仅绑 127.0.0.1，网页经 PNA/Host 校验无法直连），稳定可靠。
// "pipe"：经 fd 3/4 与浏览器通信、不开本地端口；仅 Linux/macOS 支持，实验性。
//   ⚠️ 当前 pipe 模式未完整支持 DevTools 页面功能（网络抓包/控制台/HAR/截图等），暂时禁用。
//   待完整支持后再开放配置选项。
// Windows 始终回退端口（os/exec 无法把句柄映射成子进程 fd 3/4）。

// cdpUsePipe 返回当前是否应使用 --remote-debugging-pipe。
func (a *App) cdpUsePipe() bool {
	// 暂时强制禁用 pipe 模式，直到完整支持 DevTools 页面的所有功能
	// （包括 CDPSessionCreate/CDP Manager/LaunchServer CDP 代理等）
	return false

	// TODO: 待 pipe 模式完整支持后，取消下面注释并移除上面的 return false
	// if a.config == nil {
	// 	return false
	// }
	// switch strings.ToLower(strings.TrimSpace(a.config.Browser.CDPTransport)) {
	// case "pipe":
	// 	return goruntime.GOOS != "windows"
	// default: // "", "port", "auto" 及未知值一律走端口（安全默认）
	// 	return false
	// }
}

// registerPipeConn 记录某实例的共享 pipe 连接。
func (a *App) registerPipeConn(profileID string, conn *cdp.PipeConn) {
	if conn == nil {
		return
	}
	a.pipeConnsMu.Lock()
	if a.pipeConns == nil {
		a.pipeConns = make(map[string]*cdp.PipeConn)
	}
	// 若已有旧连接，先关闭，避免句柄泄露。
	if old := a.pipeConns[profileID]; old != nil && old != conn {
		old.Close()
	}
	a.pipeConns[profileID] = conn
	a.pipeConnsMu.Unlock()
}

// cdpPipeConnFor 返回实例的共享 pipe 连接；端口模式或无连接时返回 nil。
func (a *App) cdpPipeConnFor(profileID string) *cdp.PipeConn {
	a.pipeConnsMu.Lock()
	defer a.pipeConnsMu.Unlock()
	if a.pipeConns == nil {
		return nil
	}
	return a.pipeConns[profileID]
}

// closePipeConn 关闭并移除某实例的 pipe 连接（实例停止时调用）。
func (a *App) closePipeConn(profileID string) {
	a.pipeConnsMu.Lock()
	conn := a.pipeConns[profileID]
	if conn != nil {
		delete(a.pipeConns, profileID)
	}
	a.pipeConnsMu.Unlock()
	if conn != nil {
		conn.Close()
	}
}

// profileCDP 表示与某实例浏览器通信的 CDP 通道，自动适配 pipe 或调试端口，
// 供 Cookie / 用户名扫描等"单命令"型消费者统一调用。
type profileCDP struct {
	pipe      *cdp.PipeConn // 非 nil = pipe 模式
	debugPort int           // pipe 为 nil 时使用
}

// profileCDP 解析某实例可用的 CDP 通道。优先 pipe，否则回退调试端口。
func (a *App) profileCDP(profileID string) (*profileCDP, error) {
	if conn := a.cdpPipeConnFor(profileID); conn != nil {
		return &profileCDP{pipe: conn}, nil
	}
	port, err := a.getDebugPort(profileID)
	if err != nil {
		return nil, err
	}
	return &profileCDP{debugPort: port}, nil
}

// callPage 向页面目标发送一条 CDP 命令并返回 result。
func (pc *profileCDP) callPage(method string, params map[string]any) (map[string]any, error) {
	if pc.pipe != nil {
		return pc.pipe.CallPage(method, params)
	}
	return cdpCall(pc.debugPort, method, params)
}

// callPageTimeout 同 callPage，但可指定超时（用于长轮询的页面脚本 evaluate）。
func (pc *profileCDP) callPageTimeout(method string, params map[string]any, timeout time.Duration) (map[string]any, error) {
	if pc.pipe != nil {
		return pc.pipe.CallPageTimeout(method, params, timeout)
	}
	return cdpCallTimeout(pc.debugPort, method, params, timeout)
}

// callBrowser 发送浏览器级 CDP 命令。
func (pc *profileCDP) callBrowser(method string, params map[string]any) error {
	if pc.pipe != nil {
		_, err := pc.pipe.CallBrowser(method, params)
		return err
	}
	return cdpBrowserCall(pc.debugPort, method, params)
}
