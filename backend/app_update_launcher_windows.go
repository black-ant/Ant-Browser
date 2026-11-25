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

	script := `param([int]$TargetPid, [string]$Installer, [string]$RelaunchPath)
$ErrorActionPreference = 'Continue'
$logPath = Join-Path $PSScriptRoot 'install-waiter.log'
function Write-Log([string]$Message) {
    "[$(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')] $Message" | Out-File -LiteralPath $logPath -Append -Encoding utf8
}
Write-Log "waiter started: pid=$TargetPid installer=$Installer relaunch=$RelaunchPath"
$deadline = (Get-Date).AddMinutes(30)
while ((Get-Process -Id $TargetPid -ErrorAction SilentlyContinue) -and ((Get-Date) -lt $deadline)) {
    Start-Sleep -Milliseconds 400
}
if (-not (Test-Path -LiteralPath $Installer)) {
    Write-Log "installer missing: $Installer"
    exit 1
}
$before = $null
if ($RelaunchPath -and (Test-Path -LiteralPath $RelaunchPath)) {
    $before = (Get-Item -LiteralPath $RelaunchPath).LastWriteTimeUtc
}

# 静默安装：NSIS 的 /S 不弹界面；安装完成后由脚本重新拉起新版本。
$silentOk = $false
for ($attempt = 1; $attempt -le 3; $attempt++) {
    try {
        $proc = Start-Process -FilePath $Installer -ArgumentList '/S' -Wait -PassThru -ErrorAction Stop
        Write-Log "silent install finished (attempt $attempt) exit=$($proc.ExitCode)"
        if ($proc.ExitCode -eq 0) { $silentOk = $true; break }
    } catch {
        Write-Log "silent install failed (attempt $attempt): $($_.Exception.Message)"
        Start-Sleep -Seconds 2
    }
}

$after = $null
if ($RelaunchPath -and (Test-Path -LiteralPath $RelaunchPath)) {
    $after = (Get-Item -LiteralPath $RelaunchPath).LastWriteTimeUtc
}
if ($silentOk -and $before -and $after -and ($before -eq $after)) {
    Write-Log 'silent install left executable unchanged, falling back to interactive installer'
    $silentOk = $false
}

if (-not $silentOk) {
    # 静默失败时退回可见安装向导，避免用户什么都没看到。
    try {
        Start-Process -FilePath $Installer -ErrorAction Stop
        Write-Log 'interactive installer launched'
    } catch {
        Write-Log "interactive installer failed: $($_.Exception.Message)"
    }
    Remove-Item -LiteralPath $PSCommandPath -Force -ErrorAction SilentlyContinue
    exit 1
}

if ($RelaunchPath -and (Test-Path -LiteralPath $RelaunchPath)) {
    try {
        Start-Process -FilePath $RelaunchPath -ErrorAction Stop
        Write-Log "application relaunched: $RelaunchPath"
    } catch {
        Write-Log "relaunch failed: $($_.Exception.Message)"
    }
}
Remove-Item -LiteralPath $PSCommandPath -Force -ErrorAction SilentlyContinue
exit 0
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
		return err
	}

	// 安装完成后由等待脚本重新拉起新版本，路径取当前进程的可执行文件。
	relaunchPath, exeErr := os.Executable()
	if exeErr != nil {
		relaunchPath = ""
	}
	cmd := exec.Command(
		"powershell.exe",
		"-NoProfile",
		"-WindowStyle", "Hidden",
		"-ExecutionPolicy", "Bypass",
		"-File", scriptPath,
		strconv.Itoa(pending.PID),
		pending.InstallerPath,
		relaunchPath,
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
