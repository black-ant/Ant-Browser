//go:build windows

package backend

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
)

func (a *App) launchUpdateWaiter(pending UpdatePendingInfo) error {
	dir := a.updateDataPath("updates")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	scriptPath := filepath.Join(dir, fmt.Sprintf("wait-install-%d.ps1", os.Getpid()))
	script := `param([int]$TargetPid, [string]$Installer)
while (Get-Process -Id $TargetPid -ErrorAction SilentlyContinue) { Start-Sleep -Milliseconds 500 }
Start-Process -FilePath $Installer
Remove-Item -LiteralPath $PSCommandPath -Force -ErrorAction SilentlyContinue
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
		return err
	}

	cmd := exec.Command(
		"powershell.exe",
		"-NoProfile",
		"-WindowStyle", "Hidden",
		"-ExecutionPolicy", "Bypass",
		"-File", scriptPath,
		strconv.Itoa(pending.PID),
		pending.InstallerPath,
	)
	// DETACHED_PROCESS | CREATE_NEW_PROCESS_GROUP：主程序退出时等待脚本必须继续存活。
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000008 | syscall.CREATE_NEW_PROCESS_GROUP}
	if err := cmd.Start(); err != nil {
		_ = os.Remove(scriptPath)
		return fmt.Errorf("启动更新等待脚本失败: %w", err)
	}
	return nil
}
