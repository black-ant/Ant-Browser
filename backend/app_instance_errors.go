package backend

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const browserStartReadyTimeout = 10 * time.Second
const browserStartStableWindow = 1200 * time.Millisecond
const browserDebugProbeTimeout = 250 * time.Millisecond

var errBrowserDebugPortPending = errors.New("browser debug port pending")

type browserStartupExitError struct {
	exitErr    error
	stderrTail string
}

func (e *browserStartupExitError) Error() string {
	detail := e.Detail()
	if detail == "" && e.exitErr != nil {
		detail = strings.TrimSpace(e.exitErr.Error())
	}
	if detail == "" {
		return "browser process exited before ready"
	}
	return fmt.Sprintf("browser process exited before ready: %s", detail)
}

func (e *browserStartupExitError) Detail() string {
	lines := strings.Split(strings.TrimSpace(e.stderrTail), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			return line
		}
	}
	return ""
}

func newBrowserStartupExitError(result browserProcessExitResult) error {
	return &browserStartupExitError{
		exitErr:    result.Err,
		stderrTail: result.StderrTail,
	}
}

func waitBrowserDebugPortReady(initialDebugPort int, userDataDir string, timeout time.Duration, monitor *browserProcessMonitor) (int, error) {
	deadline := time.Now().Add(timeout)
	allowDetachedGrace := initialDebugPort > 0
	var lastErr error
	var exitResult browserProcessExitResult
	exitObserved := false

	for time.Now().Before(deadline) {
		debugPort, resolveErr := resolveBrowserDebugPort(initialDebugPort, userDataDir, monitor)
		if resolveErr == nil {
			if err := probeBrowserDebugPort(debugPort, browserDebugProbeTimeout); err == nil {
				return debugPort, nil
			} else {
				lastErr = err
			}
		} else if !errors.Is(resolveErr, errBrowserDebugPortPending) {
			lastErr = resolveErr
		}
		if monitor != nil && monitor.HasExited() {
			if !exitObserved {
				exitResult = monitor.Result()
				exitObserved = true
				if !allowDetachedGrace {
					return 0, newBrowserStartupExitError(exitResult)
				}
				exitDeadline := time.Now().Add(browserLauncherDetachGraceWindow)
				if exitDeadline.After(deadline) {
					deadline = exitDeadline
				}
			}
		}
		time.Sleep(150 * time.Millisecond)
	}
	if !exitObserved && monitor != nil && monitor.HasExited() {
		exitResult = monitor.Result()
		exitObserved = true
		if !allowDetachedGrace {
			return 0, newBrowserStartupExitError(exitResult)
		}
		postExitDeadline := time.Now().Add(browserLauncherDetachGraceWindow)
		for time.Now().Before(postExitDeadline) {
			if debugPort, resolveErr := resolveBrowserDebugPort(initialDebugPort, userDataDir, monitor); resolveErr == nil {
				if err := probeBrowserDebugPort(debugPort, browserDebugProbeTimeout); err == nil {
					return debugPort, nil
				}
			}
			time.Sleep(150 * time.Millisecond)
		}
	}
	if exitObserved {
		if debugPort, resolveErr := resolveBrowserDebugPort(initialDebugPort, userDataDir, monitor); resolveErr == nil {
			if err := probeBrowserDebugPort(debugPort, browserDebugProbeTimeout); err == nil {
				return debugPort, nil
			}
		}
		return 0, newBrowserStartupExitError(exitResult)
	}
	if lastErr != nil {
		if debugPort, resolveErr := resolveBrowserDebugPort(initialDebugPort, userDataDir, monitor); resolveErr == nil {
			return 0, fmt.Errorf("浏览器进程未在 %s 内完成启动，调试端口 %d 未就绪：%w", timeout.Round(time.Second), debugPort, lastErr)
		}
		return 0, fmt.Errorf("浏览器进程未在 %s 内完成启动，尚未获取调试端口：%w", timeout.Round(time.Second), lastErr)
	}

	if debugPort, resolveErr := resolveBrowserDebugPort(initialDebugPort, userDataDir, monitor); resolveErr == nil {
		return 0, fmt.Errorf("浏览器进程未在 %s 内完成启动，调试端口 %d 未就绪", timeout.Round(time.Second), debugPort)
	}

	return 0, fmt.Errorf("浏览器进程未在 %s 内完成启动，尚未获取调试端口", timeout.Round(time.Second))
}

func waitBrowserDebugPortStable(initialDebugPort int, userDataDir string, timeout time.Duration, stableFor time.Duration, monitor *browserProcessMonitor) (int, error) {
	debugPort, err := waitBrowserDebugPortReady(initialDebugPort, userDataDir, timeout, monitor)
	if err != nil {
		return 0, err
	}
	if stableFor <= 0 {
		return debugPort, nil
	}
	allowDetachedGrace := initialDebugPort > 0

	deadline := time.Now().Add(stableFor)
	for time.Now().Before(deadline) {
		if monitor != nil && monitor.HasExited() {
			if !allowDetachedGrace {
				return 0, newBrowserStartupExitError(monitor.Result())
			}
		}
		if err := probeBrowserDebugPort(debugPort, browserDebugProbeTimeout); err != nil {
			if monitor != nil && monitor.HasExited() {
				if !allowDetachedGrace {
					return 0, newBrowserStartupExitError(monitor.Result())
				}
			}
			return 0, fmt.Errorf("浏览器调试端口 %d 短暂就绪后又失效：%w", debugPort, err)
		}
		time.Sleep(150 * time.Millisecond)
	}
	return debugPort, nil
}

func resolveBrowserDebugPort(initialDebugPort int, userDataDir string, monitor *browserProcessMonitor) (int, error) {
	if initialDebugPort > 0 {
		return initialDebugPort, nil
	}
	if monitor != nil {
		if debugPort, ok := monitor.DebugPort(); ok {
			return debugPort, nil
		}
	}
	if debugPort, err := readBrowserDebugPortFile(userDataDir); err == nil {
		if monitor != nil {
			monitor.SetDebugPort(debugPort)
		}
		return debugPort, nil
	} else if !errors.Is(err, errBrowserDebugPortPending) {
		return 0, err
	}
	return 0, errBrowserDebugPortPending
}

func readBrowserDebugPortFile(userDataDir string) (int, error) {
	userDataDir = strings.TrimSpace(userDataDir)
	if userDataDir == "" {
		return 0, errBrowserDebugPortPending
	}

	data, err := os.ReadFile(filepath.Join(userDataDir, "DevToolsActivePort"))
	if err != nil {
		if os.IsNotExist(err) {
			return 0, errBrowserDebugPortPending
		}
		return 0, fmt.Errorf("读取 DevToolsActivePort 失败: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return 0, errBrowserDebugPortPending
	}

	port, err := strconv.Atoi(strings.TrimSpace(lines[0]))
	if err != nil || port <= 0 {
		return 0, fmt.Errorf("DevToolsActivePort 内容无效: %q", lines[0])
	}
	return port, nil
}

func probeBrowserDebugPort(debugPort int, requestTimeout time.Duration) error {
	if debugPort <= 0 {
		return fmt.Errorf("invalid debug port %d", debugPort)
	}

	client := &http.Client{Timeout: requestTimeout}
	versionErr := probeBrowserJSONVersion(client, debugPort)
	if versionErr == nil {
		return nil
	}

	listErr := probeBrowserJSONList(client, debugPort)
	if listErr == nil {
		return nil
	}

	return fmt.Errorf("%v; %v", versionErr, listErr)
}

func probeBrowserJSONVersion(client *http.Client, debugPort int) error {
	var payload struct {
		Browser              string `json:"Browser"`
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := fetchBrowserDebugJSON(client, debugPort, "/json/version", &payload); err != nil {
		return err
	}
	if strings.TrimSpace(payload.Browser) == "" && strings.TrimSpace(payload.WebSocketDebuggerURL) == "" {
		return fmt.Errorf("/json/version missing Browser and webSocketDebuggerUrl")
	}
	return nil
}

func probeBrowserJSONList(client *http.Client, debugPort int) error {
	var payload []map[string]interface{}
	return fetchBrowserDebugJSON(client, debugPort, "/json/list", &payload)
}

func fetchBrowserDebugJSON(client *http.Client, debugPort int, path string, dest interface{}) error {
	url := fmt.Sprintf("http://127.0.0.1:%d%s", debugPort, path)
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("%s request failed: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s returned HTTP %d", path, resp.StatusCode)
	}
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 256*1024))
	if err := decoder.Decode(dest); err != nil {
		return fmt.Errorf("%s returned invalid JSON: %w", path, err)
	}
	return nil
}

func canConnectDebugPort(debugPort int, dialTimeout time.Duration) bool {
	if debugPort <= 0 {
		return false
	}

	address := fmt.Sprintf("127.0.0.1:%d", debugPort)
	conn, err := net.DialTimeout("tcp", address, dialTimeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func describeChromeProcessStartError(chromeBinaryPath string, err error) string {
	raw := strings.TrimSpace(err.Error())
	lower := strings.ToLower(raw)

	switch {
	case strings.Contains(lower, "access is denied"),
		strings.Contains(lower, "permission denied"),
		strings.Contains(raw, "拒绝访问"):
		return fmt.Sprintf("窗口启动失败：系统拒绝启动浏览器进程。可执行文件：%s。请检查文件权限、杀毒软件拦截，或尝试以管理员身份运行。", chromeBinaryPath)
	case strings.Contains(lower, "not a valid win32 application"),
		strings.Contains(raw, "不是有效的 win32 应用程序"),
		strings.Contains(raw, "不是有效的 Win32 应用程序"),
		strings.Contains(raw, "bad exe format"),
		strings.Contains(lower, "exec format error"),
		strings.Contains(lower, "cannot execute binary file"):
		return fmt.Sprintf("窗口启动失败：当前浏览器内核与系统/架构不兼容。可执行文件：%s。请确认 Linux 环境使用的是对应架构的 Chrome 内核，而不是 Windows 可执行文件。", chromeBinaryPath)
	case strings.Contains(raw, "系统找不到指定的文件"),
		strings.Contains(lower, "file not found"),
		strings.Contains(lower, "no such file"),
		strings.Contains(lower, "cannot find the file"):
		return fmt.Sprintf("窗口启动失败：浏览器可执行文件不存在。可执行文件：%s。请检查内核路径是否正确，或重新下载内核。", chromeBinaryPath)
	case strings.Contains(raw, "目录名称无效"),
		strings.Contains(lower, "directory name is invalid"):
		return fmt.Sprintf("窗口启动失败：浏览器工作目录无效。当前目录：%s。请检查内核路径配置是否正确。", chromeBinaryPath)
	default:
		return fmt.Sprintf("窗口启动失败：浏览器进程拉起失败。可执行文件：%s。原因：%s。请检查内核文件是否完整、启动参数是否正确，或是否被安全软件拦截。", chromeBinaryPath, raw)
	}
}

func describeBrowserReadyTimeout(debugPort int, timeout time.Duration) string {
	if debugPort <= 0 {
		return fmt.Sprintf("窗口启动失败：浏览器进程已拉起，但在 %s 内未完成就绪，且未获取到调试端口。请检查内核文件是否完整、启动参数是否正确，或是否被安全软件拦截。", timeout.Round(time.Second))
	}
	return fmt.Sprintf("窗口启动失败：浏览器进程已拉起，但在 %s 内未完成就绪，调试端口 %d 未就绪。请检查内核文件是否完整、启动参数是否正确，或是否被安全软件拦截。", timeout.Round(time.Second), debugPort)
}

func describeBrowserReadyFailure(chromeBinaryPath string, debugPort int, timeout time.Duration, err error) string {
	var exitErr *browserStartupExitError
	if errors.As(err, &exitErr) {
		detail := exitErr.Detail()
		if detail == "" && exitErr.exitErr != nil {
			detail = strings.TrimSpace(exitErr.exitErr.Error())
		}
		if detail != "" {
			return fmt.Sprintf("窗口启动失败：浏览器进程在完成就绪前退出。可执行文件：%s。原因：%s。请检查内核文件是否完整、启动参数是否正确，或是否被安全软件拦截。", chromeBinaryPath, detail)
		}
		return fmt.Sprintf("窗口启动失败：浏览器进程在完成就绪前退出。可执行文件：%s。请检查内核文件是否完整、启动参数是否正确，或是否被安全软件拦截。", chromeBinaryPath)
	}
	return describeBrowserReadyTimeout(debugPort, timeout)
}
