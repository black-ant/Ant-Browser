//go:build !windows

package backend

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func (a *App) launchUpdateWaiter(pending UpdatePendingInfo) error {
	dir := a.updateDataPath("updates")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	scriptPath := filepath.Join(dir, fmt.Sprintf("wait-install-%d.sh", os.Getpid()))
	opener := "xdg-open"
	if runtime.GOOS == "darwin" {
		opener = "open"
	}
	script := fmt.Sprintf(`#!/bin/sh
while kill -0 %d 2>/dev/null; do sleep 0.5; done
%s %s
rm -f "$0"
`, pending.PID, opener, shellQuote(pending.InstallerPath))
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		return err
	}

	// nohup + & 让等待脚本脱离应用进程组，应用退出后继续等待并打开安装包。
	command := fmt.Sprintf("nohup sh %s >/dev/null 2>&1 &", shellQuote(scriptPath))
	if err := exec.Command("sh", "-c", command).Start(); err != nil {
		_ = os.Remove(scriptPath)
		return fmt.Errorf("启动更新等待脚本失败: %w", err)
	}
	return nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
