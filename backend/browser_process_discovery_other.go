//go:build !windows
// +build !windows

package backend

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func findBrowserUserDataProcessesOS(userDataDir string) ([]browserUserDataProcess, error) {
	targetDir := normalizeProcessUserDataDir(userDataDir)
	if targetDir == "" {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "ps", "-axo", "pid=,command=")
	output, err := command.Output()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("查询浏览器进程超时")
	}
	if err != nil {
		return nil, fmt.Errorf("查询浏览器进程失败: %w", err)
	}
	return parseBrowserProcessListOutput(string(output), targetDir), nil
}

func terminateBrowserUserDataProcessOS(pid int, timeout time.Duration) error {
	if pid <= 0 {
		return nil
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if err := process.Kill(); err != nil {
		if isProcessAlreadyFinished(err) {
			return nil
		}
		return err
	}

	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := process.Signal(syscall.Signal(0)); err != nil {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err := process.Signal(syscall.Signal(0)); err != nil {
		return nil
	}
	return fmt.Errorf("进程 %d 在 %s 内仍未退出", pid, timeout)
}

func parseBrowserProcessListOutput(output string, targetDir string) []browserUserDataProcess {
	targetDir = normalizeProcessUserDataDir(targetDir)
	if targetDir == "" {
		return nil
	}

	result := make([]browserUserDataProcess, 0)
	for _, line := range strings.Split(output, "\n") {
		pid, commandLine, ok := parseProcessListLine(line)
		if !ok || commandLine == "" {
			continue
		}
		args := splitProcessCommandLine(commandLine)
		userDataDir := processCommandLineFlagValue(args, "--user-data-dir")
		if normalizeProcessUserDataDir(userDataDir) != targetDir {
			continue
		}
		result = append(result, browserUserDataProcess{
			PID:         pid,
			DebugPort:   parseRemoteDebuggingPort(commandLine),
			CommandLine: commandLine,
		})
	}
	return result
}

func parseProcessListLine(line string) (int, string, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return 0, "", false
	}
	index := 0
	for index < len(line) && line[index] >= '0' && line[index] <= '9' {
		index++
	}
	if index == 0 || index == len(line) {
		return 0, "", false
	}
	pid, err := strconv.Atoi(line[:index])
	if err != nil || pid <= 0 {
		return 0, "", false
	}
	commandLine := strings.TrimSpace(line[index:])
	if commandLine == "" {
		return 0, "", false
	}
	return pid, commandLine, true
}

func processCommandLineFlagValue(args []string, name string) string {
	for index, arg := range args {
		if strings.HasPrefix(arg, name+"=") {
			return strings.TrimSpace(strings.TrimPrefix(arg, name+"="))
		}
		if arg == name && index+1 < len(args) {
			return strings.TrimSpace(args[index+1])
		}
	}
	return ""
}

func normalizeProcessUserDataDir(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if absolute, err := filepath.Abs(value); err == nil {
		value = absolute
	}
	return filepath.Clean(value)
}
