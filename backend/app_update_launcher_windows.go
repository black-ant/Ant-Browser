//go:build windows

package backend

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const updateWaiterLogName = "install-waiter.log"

func (a *App) launchUpdateWaiter(pending UpdatePendingInfo) error {
	dir := a.updateDataPath("updates")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	scriptPath := filepath.Join(dir, fmt.Sprintf("wait-install-%d.ps1", os.Getpid()))
	logPath := filepath.Join(dir, updateWaiterLogName)
	_ = os.Remove(logPath)

	script := `param([int]$TargetPid, [string]$Installer)
$ErrorActionPreference = 'Continue'
$logPath = Join-Path $PSScriptRoot 'install-waiter.log'
function Write-Log([string]$Message) {
    "[$(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')] $Message" | Out-File -LiteralPath $logPath -Append -Encoding utf8
}
Write-Log "waiter started: pid=$TargetPid installer=$Installer"
$deadline = (Get-Date).AddMinutes(30)
while ((Get-Process -Id $TargetPid -ErrorAction SilentlyContinue) -and ((Get-Date) -lt $deadline)) {
    Start-Sleep -Milliseconds 400
}
if (-not (Test-Path -LiteralPath $Installer)) {
    Write-Log "installer missing: $Installer"
    exit 1
}
for ($attempt = 1; $attempt -le 5; $attempt++) {
    try {
        Start-Process -FilePath $Installer -ErrorAction Stop
        Write-Log "installer launched (attempt $attempt)"
        Remove-Item -LiteralPath $PSCommandPath -Force -ErrorAction SilentlyContinue
        exit 0
    } catch {
        Write-Log "launch failed (attempt $attempt): $($_.Exception.Message)"
        Start-Sleep -Seconds 2
    }
}
Write-Log 'installer launch aborted'
exit 1
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
	// 不能用 DETACHED_PROCESS：该标志下 powershell.exe 会立即退出，等待脚本根本不执行。
	// CREATE_NO_WINDOW 保证无窗口，CREATE_NEW_PROCESS_GROUP 让脚本在应用退出后继续存活。
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow | syscall.CREATE_NEW_PROCESS_GROUP}
	if err := cmd.Start(); err != nil {
		_ = os.Remove(scriptPath)
		return fmt.Errorf("启动更新等待脚本失败: %w", err)
	}
	// 等待脚本写第一条日志再放行：否则界面已经退出、安装包却不会被拉起。
	if err := waitUpdateWaiterReady(logPath, cmd, 3*time.Second); err != nil {
		_ = os.Remove(scriptPath)
		return err
	}
	return nil
}

func waitUpdateWaiterReady(logPath string, cmd *exec.Cmd, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if data, err := os.ReadFile(logPath); err == nil && strings.Contains(string(data), "waiter started") {
			return nil
		}
		if time.Now().After(deadline) {
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			return fmt.Errorf("安装等待脚本没有启动，已取消退出安装；请重试或手动运行安装包")
		}
		time.Sleep(100 * time.Millisecond)
	}
}
