//go:build windows
// +build windows

package backend

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
)

func findBrowserUserDataProcessesOS(userDataDir string) ([]browserUserDataProcess, error) {
	userDataDir = strings.TrimSpace(userDataDir)
	if userDataDir == "" {
		return nil, nil
	}
	fullUserDataDir, err := filepath.Abs(userDataDir)
	if err != nil {
		fullUserDataDir = userDataDir
	}

	psScript := `param([string]$UserDataDir)
$ErrorActionPreference = 'SilentlyContinue'
$target = [System.IO.Path]::GetFullPath($UserDataDir).TrimEnd('\')
$items = @()
Get-CimInstance Win32_Process | Where-Object { $_.CommandLine -and $_.CommandLine -match '--user-data-dir=' } | ForEach-Object {
  $cmd = [string]$_.CommandLine
  $match = [regex]::Match($cmd, '--user-data-dir=(?:"([^"]+)"|([^\s]+))')
  if (-not $match.Success) { return }
  $dir = $match.Groups[1].Value
  if ([string]::IsNullOrWhiteSpace($dir)) { $dir = $match.Groups[2].Value }
  if ([string]::IsNullOrWhiteSpace($dir)) { return }
  try { $dir = [System.IO.Path]::GetFullPath($dir).TrimEnd('\') } catch {}
  if (-not $dir.Equals($target, [System.StringComparison]::OrdinalIgnoreCase)) { return }
  $port = 0
  $portMatch = [regex]::Match($cmd, '--remote-debugging-port=(\d+)')
  if ($portMatch.Success) { [void][int]::TryParse($portMatch.Groups[1].Value, [ref]$port) }
  $items += [pscustomobject]@{ pid = [int]$_.ProcessId; debugPort = [int]$port; commandLine = $cmd }
}
$items | ConvertTo-Json -Compress
`

	output, err := runPowerShellJSON(psScript, "-UserDataDir", fullUserDataDir)
	if err != nil {
		return nil, err
	}
	output = bytes.TrimSpace(output)
	if len(output) == 0 || bytes.Equal(output, []byte("null")) {
		return nil, nil
	}

	var list []browserUserDataProcess
	if bytes.HasPrefix(output, []byte("[")) {
		if err := json.Unmarshal(output, &list); err != nil {
			return nil, err
		}
	} else {
		var item browserUserDataProcess
		if err := json.Unmarshal(output, &item); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, nil
}

func terminateBrowserUserDataProcessOS(pid int, timeout time.Duration) error {
	if pid <= 0 {
		return nil
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	if tryCloseBrowserViaCDPPID(pid, timeout) {
		return nil
	}

	softKillCmd := exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T")
	hideWindow(softKillCmd)
	_ = softKillCmd.Run()
	if waitProcessExitWindows(pid, timeout) {
		return nil
	}

	forceKillCmd := exec.Command("taskkill", "/F", "/PID", strconv.Itoa(pid), "/T")
	hideWindow(forceKillCmd)
	if err := forceKillCmd.Run(); err != nil {
		return err
	}
	if !waitProcessExitWindows(pid, 2*time.Second) {
		return fmt.Errorf("process still running")
	}
	return nil
}

func tryCloseBrowserViaCDPPID(pid int, timeout time.Duration) bool {
	processes, err := findBrowserProcessByPID(pid)
	if err != nil || len(processes) == 0 {
		return false
	}
	debugPort := processes[0].DebugPort
	if debugPort <= 0 {
		debugPort = parseRemoteDebuggingPort(processes[0].CommandLine)
	}
	return tryCloseBrowserViaCDP(debugPort, timeout)
}

func findBrowserProcessByPID(pid int) ([]browserUserDataProcess, error) {
	if pid <= 0 {
		return nil, nil
	}
	psScript := `param([int]$PidValue)
$ErrorActionPreference = 'SilentlyContinue'
$p = Get-CimInstance Win32_Process | Where-Object { $_.ProcessId -eq $PidValue } | Select-Object -First 1
if ($null -eq $p) { exit 0 }
$cmd = [string]$p.CommandLine
$port = 0
$portMatch = [regex]::Match($cmd, '--remote-debugging-port=(\d+)')
if ($portMatch.Success) { [void][int]::TryParse($portMatch.Groups[1].Value, [ref]$port) }
[pscustomobject]@{ pid = [int]$p.ProcessId; debugPort = [int]$port; commandLine = $cmd } | ConvertTo-Json -Compress
`
	output, err := runPowerShellJSON(psScript, "-PidValue", strconv.Itoa(pid))
	if err != nil {
		return nil, err
	}
	output = bytes.TrimSpace(output)
	if len(output) == 0 || bytes.Equal(output, []byte("null")) {
		return nil, nil
	}
	var item browserUserDataProcess
	if err := json.Unmarshal(output, &item); err != nil {
		return nil, err
	}
	return []browserUserDataProcess{item}, nil
}

func runPowerShellJSON(script string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	powershellPath := `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`
	if _, err := exec.LookPath(powershellPath); err != nil {
		if fallbackPath, lookErr := exec.LookPath("powershell.exe"); lookErr == nil {
			powershellPath = fallbackPath
		}
	}

	commandText := "& { " + strings.TrimSpace(script) + " }"
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") && !strings.ContainsAny(arg, " \t\r\n") {
			commandText += " " + arg
			continue
		}
		commandText += " '" + strings.ReplaceAll(arg, "'", "''") + "'"
	}
	commandArgs := []string{
		"-NoProfile",
		"-ExecutionPolicy",
		"Bypass",
		"-EncodedCommand",
		encodePowerShellCommand(commandText),
	}
	cmd := exec.CommandContext(ctx, powershellPath, commandArgs...)
	hideWindow(cmd)
	output, err := cmd.Output()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("powershell query timed out")
	}
	if err != nil {
		return nil, err
	}
	return output, nil
}

func encodePowerShellCommand(command string) string {
	codeUnits := utf16.Encode([]rune(command))
	data := make([]byte, len(codeUnits)*2)
	for index, codeUnit := range codeUnits {
		binary.LittleEndian.PutUint16(data[index*2:], codeUnit)
	}
	return base64.StdEncoding.EncodeToString(data)
}
