//go:build windows
// +build windows

package backend

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func killResidualRuntimeProcesses(appRoot string) error {
	appRoot = strings.TrimSpace(appRoot)
	if appRoot == "" {
		return nil
	}
	if _, err := os.Stat(appRoot); err != nil {
		return nil
	}

	exePath, _ := os.Executable()
	psScript := `param([string]$Root, [string]$ExcludePath)
$ErrorActionPreference = 'SilentlyContinue'
if ([string]::IsNullOrWhiteSpace($Root) -or -not (Test-Path -LiteralPath $Root)) { exit 0 }
$root = [System.IO.Path]::GetFullPath($Root).TrimEnd('\') + '\'
$exclude = ''
if (-not [string]::IsNullOrWhiteSpace($ExcludePath)) {
  $exclude = [System.IO.Path]::GetFullPath($ExcludePath)
}
function Get-AntChromeResidualProcesses {
  @(
    Get-CimInstance Win32_Process | Where-Object {
      $_.ExecutablePath -and (
        $_.ExecutablePath.StartsWith(($root + 'bin\'), [System.StringComparison]::OrdinalIgnoreCase) -or
        $_.ExecutablePath.StartsWith(($root + 'chrome\'), [System.StringComparison]::OrdinalIgnoreCase)
      ) -and (
        $exclude -eq '' -or
        -not $_.ExecutablePath.Equals($exclude, [System.StringComparison]::OrdinalIgnoreCase)
      )
    }
  )
}
$browserProcesses = @(
  Get-CimInstance Win32_Process | Where-Object {
    $_.CommandLine -and
    (
      ($_.ExecutablePath -and $_.ExecutablePath.StartsWith(($root + 'chrome\'), [System.StringComparison]::OrdinalIgnoreCase)) -or
      ($_.CommandLine -match '(?i)--user-data-dir(?:=|\s)' -and $_.CommandLine -match [regex]::Escape($root))
    )
  }
)
$targets = @(Get-AntChromeResidualProcesses | Where-Object {
  $_.ExecutablePath.StartsWith(($root + 'bin\'), [System.StringComparison]::OrdinalIgnoreCase)
} | Sort-Object ProcessId -Descending)
if ($browserProcesses.Count -gt 0 -or $targets.Count -eq 0) { exit 0 }
foreach ($p in $targets) {
  try { Stop-Process -Id $p.ProcessId -Force -ErrorAction Stop } catch {}
}
Start-Sleep -Milliseconds 400
$left = @(Get-AntChromeResidualProcesses | Where-Object {
  $_.ExecutablePath.StartsWith(($root + 'bin\'), [System.StringComparison]::OrdinalIgnoreCase)
})
if ($left.Count -gt 0) {
  $names = ($left | ForEach-Object { $_.Name + '#' + $_.ProcessId }) -join ', '
  Write-Host ('still running: ' + $names)
  exit 1
}
exit 0
`

	tempFile, err := os.CreateTemp("", "ant-chrome-cleanup-*.ps1")
	if err != nil {
		return fmt.Errorf("创建清理脚本失败: %w", err)
	}
	scriptPath := tempFile.Name()
	if _, err := tempFile.WriteString(psScript); err != nil {
		tempFile.Close()
		_ = os.Remove(scriptPath)
		return fmt.Errorf("写入清理脚本失败: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(scriptPath)
		return fmt.Errorf("关闭清理脚本失败: %w", err)
	}
	defer os.Remove(scriptPath)

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	powershellPath := `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`
	if _, err := os.Stat(powershellPath); err != nil {
		if fallbackPath, lookErr := exec.LookPath("powershell.exe"); lookErr == nil {
			powershellPath = fallbackPath
		} else {
			return fmt.Errorf("未找到 powershell.exe")
		}
	}

	cmd := exec.CommandContext(ctx, powershellPath, "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", filepath.Clean(scriptPath), "-Root", appRoot, "-ExcludePath", exePath)
	hideWindow(cmd)

	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("清理残留进程超时")
	}
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("清理残留进程失败: %s", message)
	}
	return nil
}
