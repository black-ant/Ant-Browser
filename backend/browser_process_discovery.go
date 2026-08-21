package backend

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type browserUserDataProcess struct {
	PID         int    `json:"pid"`
	DebugPort   int    `json:"debugPort"`
	CommandLine string `json:"commandLine"`
}

type browserRuntimeDetection struct {
	PID        int
	DebugPort  int
	DebugReady bool
}

var errBrowserStartHandledByRecoveredRuntime = errors.New("browser start handled by recovered runtime")

var findBrowserUserDataProcesses = findBrowserUserDataProcessesOS
var terminateBrowserUserDataProcess = terminateBrowserUserDataProcessOS

var remoteDebuggingPortPattern = regexp.MustCompile(`(?i)--remote-debugging-port=(\d+)`)

func parseRemoteDebuggingPort(commandLine string) int {
	matches := remoteDebuggingPortPattern.FindStringSubmatch(commandLine)
	if len(matches) < 2 {
		return 0
	}
	port, err := strconv.Atoi(matches[1])
	if err != nil || port <= 0 {
		return 0
	}
	return port
}

func detectBrowserRuntimeByUserDataDir(userDataDir string) (browserRuntimeDetection, bool) {
	userDataDir = strings.TrimSpace(userDataDir)
	if userDataDir == "" {
		return browserRuntimeDetection{}, false
	}

	if detection, ok := detectBrowserRuntimeByActivePort(userDataDir); ok {
		return detection, true
	}

	processes, err := findBrowserUserDataProcesses(userDataDir)
	if err != nil || len(processes) == 0 {
		return browserRuntimeDetection{}, false
	}

	for _, process := range processes {
		debugPort := process.DebugPort
		if debugPort <= 0 {
			debugPort = parseRemoteDebuggingPort(process.CommandLine)
		}
		if debugPort > 0 {
			if err := probeBrowserDebugPort(debugPort, browserDebugProbeTimeout); err == nil {
				return browserRuntimeDetection{PID: process.PID, DebugPort: debugPort, DebugReady: true}, true
			}
		}
	}

	first := processes[0]
	return browserRuntimeDetection{PID: first.PID, DebugPort: first.DebugPort}, true
}

func detectBrowserRuntimeByActivePort(userDataDir string) (browserRuntimeDetection, bool) {
	userDataDir = strings.TrimSpace(userDataDir)
	if userDataDir == "" {
		return browserRuntimeDetection{}, false
	}

	if debugPort, err := readBrowserDebugPortFile(userDataDir); err == nil && debugPort > 0 {
		if err := probeBrowserDebugPort(debugPort, browserDebugProbeTimeout); err == nil {
			return browserRuntimeDetection{DebugPort: debugPort, DebugReady: true}, true
		}
	}

	return browserRuntimeDetection{}, false
}

// terminateBrowserUserDataOrphans 结束同一用户数据目录下无法被 CDP 接管的残留浏览器进程。
// 仅当主浏览器进程（无 --type= 参数）不携带调试端口时才清理，
// 避免误杀正在启动或受管实例的子进程；taskkill /T 会连同子进程一起结束。
func terminateBrowserUserDataOrphans(userDataDir string, timeout time.Duration) (bool, error) {
	processes, err := findBrowserUserDataProcesses(userDataDir)
	if err != nil {
		return false, err
	}
	mainPID := 0
	hasManaged := false
	for _, process := range processes {
		if process.PID <= 0 || isBrowserChildProcessCommandLine(process.CommandLine) {
			continue
		}
		debugPort := process.DebugPort
		if debugPort <= 0 {
			debugPort = parseRemoteDebuggingPort(process.CommandLine)
		}
		if debugPort > 0 {
			hasManaged = true
			continue
		}
		mainPID = process.PID
	}
	if hasManaged || mainPID <= 0 {
		return false, nil
	}
	if err := terminateBrowserUserDataProcess(mainPID, timeout); err != nil {
		return false, err
	}
	return true, nil
}

func isBrowserChildProcessCommandLine(commandLine string) bool {
	return strings.Contains(commandLine, "--type=")
}

func terminateBrowserProcessesByUserDataDir(userDataDir string, timeout time.Duration) (bool, error) {
	processes, err := findBrowserUserDataProcesses(userDataDir)
	if err != nil {
		return false, err
	}
	if len(processes) == 0 {
		return false, nil
	}

	var errs []error
	terminated := false
	for _, process := range processes {
		if process.PID <= 0 || process.PID == os.Getpid() {
			continue
		}
		terminated = true
		if err := terminateBrowserUserDataProcess(process.PID, timeout); err != nil {
			errs = append(errs, fmt.Errorf("pid %d: %w", process.PID, err))
		}
	}
	return terminated, errors.Join(errs...)
}
