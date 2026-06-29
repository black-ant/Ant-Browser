package appupdate

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type WindowsBackend struct {
	CurrentExePath    string
	CurrentAppVersion string
	ProcessID         int
	SuppressRelaunch  bool
}

func (b WindowsBackend) Target() string {
	return "windows-amd64"
}

func (b WindowsBackend) ValidateInstallMode(layout Layout) error {
	install := filepath.Clean(layout.InstallRoot)
	lower := strings.ToLower(filepath.ToSlash(strings.ReplaceAll(install, `\`, `/`)))
	if strings.Contains(lower, "/program files/") ||
		strings.HasSuffix(lower, "/program files") ||
		strings.Contains(lower, "/program files (x86)/") ||
		strings.HasSuffix(lower, "/program files (x86)") {
		return ErrUnsupportedInstall
	}
	if err := os.MkdirAll(install, 0o755); err != nil {
		return err
	}
	probe, err := os.CreateTemp(install, ".app-update-write-*")
	if err != nil {
		return fmt.Errorf("install root is not writable: %w", err)
	}
	name := probe.Name()
	_ = probe.Close()
	_ = os.Remove(name)
	return nil
}

func (b WindowsBackend) PrepareApply(plan ApplyPlan) error {
	if err := ValidateStagedPayload(plan.Target, plan.StagedPath); err != nil {
		return err
	}
	if _, err := os.Stat(plan.InstallRoot); err != nil {
		return err
	}
	return b.prepareRunner(plan)
}

func (b WindowsBackend) SpawnApplyRunner(planPath string) error {
	plan, err := ReadPlan(planPath)
	if err != nil {
		return err
	}
	exe := runnerExePath(plan)
	if _, err := os.Stat(exe); err != nil {
		exe = strings.TrimSpace(b.CurrentExePath)
		if exe == "" {
			exe, err = os.Executable()
			if err != nil {
				return err
			}
		}
	}
	cmd := exec.Command(exe, "--apply-update", planPath)
	hideWindow(cmd)
	return cmd.Start()
}

func (b WindowsBackend) RunApply(planPath string) error {
	plan, err := ReadPlan(planPath)
	if err != nil {
		return err
	}
	layout := NewLayout(plan.InstallRoot, plan.StateRoot)
	if plan.WaitForProcessID > 0 && !waitForProcessExit(plan.WaitForProcessID, 20*time.Second) {
		err := fmt.Errorf("app process did not exit before apply timeout: pid %d", plan.WaitForProcessID)
		_ = WriteState(layout, PersistentState{
			Status:           PersistentStatusFailedManualRepair,
			LocalAppVersion:  plan.OldAppVersion,
			RemoteAppVersion: plan.NewAppVersion,
			PlanPath:         planPath,
			BackupPath:       plan.BackupPath,
			LastError: ErrorInfo{
				Code:    "APP-UPDATE-PROCESS-STILL-RUNNING",
				Message: err.Error(),
			},
		})
		return err
	}

	if err := WriteState(layout, PersistentState{
		Status:           PersistentStatusApplying,
		LocalAppVersion:  plan.OldAppVersion,
		RemoteAppVersion: plan.NewAppVersion,
		PlanPath:         planPath,
		BackupPath:       plan.BackupPath,
	}); err != nil {
		return err
	}
	if err := b.closeInstalledProcesses(plan); err != nil {
		_ = WriteState(layout, PersistentState{
			Status:           PersistentStatusFailedManualRepair,
			LocalAppVersion:  plan.OldAppVersion,
			RemoteAppVersion: plan.NewAppVersion,
			PlanPath:         planPath,
			BackupPath:       plan.BackupPath,
			LastError: ErrorInfo{
				Code:    "APP-UPDATE-CLOSE-PROCESSES-FAILED",
				Message: err.Error(),
			},
		})
		return err
	}
	if err := b.backupInstall(plan); err != nil {
		_ = WriteState(layout, PersistentState{
			Status: PersistentStatusFailedManualRepair,
			LastError: ErrorInfo{
				Code:    "APP-UPDATE-BACKUP-FAILED-MANUAL-REPAIR",
				Message: err.Error(),
			},
		})
		return err
	}
	if err := b.replaceInstall(plan); err != nil {
		if rollbackErr := b.rollbackInstall(plan); rollbackErr != nil {
			_ = WriteState(layout, PersistentState{
				Status:     PersistentStatusFailedManualRepair,
				BackupPath: plan.BackupPath,
				LastError: ErrorInfo{
					Code:    "APP-UPDATE-ROLLBACK-FAILED-MANUAL-REPAIR",
					Message: rollbackErr.Error(),
				},
			})
			return err
		}
		_ = WriteState(layout, PersistentState{
			Status:     PersistentStatusRolledBack,
			BackupPath: plan.BackupPath,
			LastError: ErrorInfo{
				Code:    "APP-UPDATE-APPLY-FAILED-ROLLED-BACK",
				Message: err.Error(),
			},
		})
		return err
	}

	if err := WriteState(layout, PersistentState{
		Status:           PersistentStatusVerifying,
		LocalAppVersion:  plan.OldAppVersion,
		RemoteAppVersion: plan.NewAppVersion,
		PlanPath:         planPath,
		BackupPath:       plan.BackupPath,
	}); err != nil {
		return err
	}
	return b.launchPostUpdateCheck(plan, planPath)
}

func (b WindowsBackend) closeInstalledProcesses(plan ApplyPlan) error {
	if runtime.GOOS != "windows" {
		return nil
	}
	installRoot := strings.TrimSpace(plan.InstallRoot)
	if installRoot == "" {
		return nil
	}
	script, err := os.CreateTemp("", "ant-browser-close-installed-*.ps1")
	if err != nil {
		return err
	}
	scriptPath := script.Name()
	defer os.Remove(scriptPath)

	if _, err := script.WriteString(windowsCloseInstalledProcessesScript()); err != nil {
		_ = script.Close()
		return err
	}
	if err := script.Close(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "powershell.exe",
		"-NoProfile",
		"-ExecutionPolicy", "Bypass",
		"-File", scriptPath,
		"-InstallDir", filepath.Clean(installRoot),
		"-ExcludePath", runnerExePath(plan),
	)
	hideWindow(cmd)
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("close installed processes timed out")
	}
	if err != nil {
		return fmt.Errorf("close installed processes failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	b.taskkillKnownRuntimeProcesses()
	time.Sleep(700 * time.Millisecond)
	return nil
}

func (b WindowsBackend) taskkillKnownRuntimeProcesses() {
	for _, image := range []string{"ant-chrome.exe", "xray.exe", "sing-box.exe"} {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		cmd := exec.CommandContext(ctx, "taskkill.exe", "/F", "/T", "/IM", image)
		hideWindow(cmd)
		_ = cmd.Run()
		cancel()
	}
}

func windowsCloseInstalledProcessesScript() string {
	return `param([string]$InstallDir, [string]$ExcludePath)
$ErrorActionPreference = 'SilentlyContinue'
if ([string]::IsNullOrWhiteSpace($InstallDir) -or -not (Test-Path -LiteralPath $InstallDir)) { exit 0 }
$root = [System.IO.Path]::GetFullPath($InstallDir).TrimEnd('\') + '\'
$rootText = $root.ToLowerInvariant()
$exclude = ''
if (-not [string]::IsNullOrWhiteSpace($ExcludePath)) { $exclude = [System.IO.Path]::GetFullPath($ExcludePath) }
function Get-MakaBrowserProcesses {
  @(Get-CimInstance Win32_Process | Where-Object {
    $exe = if ($_.ExecutablePath) { $_.ExecutablePath } else { '' }
    $cmd = if ($_.CommandLine) { $_.CommandLine.ToLowerInvariant() } else { '' }
    $name = if ($_.Name) { $_.Name } else { '' }
    $isExcluded = ($exclude -ne '' -and $exe -ne '' -and $exe.Equals($exclude, [System.StringComparison]::OrdinalIgnoreCase))
    (
      ($exe -ne '' -and $exe.StartsWith($root, [System.StringComparison]::OrdinalIgnoreCase)) -or
      ($cmd -ne '' -and $cmd.Contains($rootText)) -or
      ($name.Equals('chrome.exe', [System.StringComparison]::OrdinalIgnoreCase) -and $cmd.Contains('ant browser'))
    ) -and
    -not $isExcluded
  })
}
$deadline = (Get-Date).AddSeconds(10)
do {
  $procs = Get-MakaBrowserProcesses
  if (-not $procs -or $procs.Count -eq 0) { exit 0 }
  foreach ($p in $procs) {
    try { Stop-Process -Id $p.ProcessId -Force -ErrorAction Stop } catch {}
  }
  Start-Sleep -Milliseconds 400
} while ((Get-Date) -lt $deadline)
$left = Get-MakaBrowserProcesses
if ($left -and $left.Count -gt 0) {
  $names = ($left | ForEach-Object { $_.Name + '#' + $_.ProcessId }) -join ', '
  Write-Host ('still running: ' + $names)
  exit 1
}
exit 0
`
}

func (b WindowsBackend) PostUpdateCheck(planPath string) error {
	plan, err := ReadPlan(planPath)
	if err != nil {
		return err
	}
	layout := NewLayout(plan.InstallRoot, plan.StateRoot)
	if currentVersion := strings.TrimSpace(b.CurrentAppVersion); currentVersion != "" && currentVersion != strings.TrimSpace(plan.NewAppVersion) {
		err := fmt.Errorf("post-update version mismatch: expected %s, got %s", plan.NewAppVersion, currentVersion)
		_ = WriteState(layout, PersistentState{
			Status:           PersistentStatusFailedManualRepair,
			LocalAppVersion:  currentVersion,
			RemoteAppVersion: plan.NewAppVersion,
			PlanPath:         planPath,
			BackupPath:       plan.BackupPath,
			LastError: ErrorInfo{
				Code:    "APP-UPDATE-POST-CHECK-VERSION-MISMATCH",
				Message: err.Error(),
			},
		})
		return err
	}
	if err := WriteState(layout, PersistentState{
		Status:           PersistentStatusSucceeded,
		LocalAppVersion:  plan.NewAppVersion,
		RemoteAppVersion: plan.NewAppVersion,
		PlanPath:         planPath,
		BackupPath:       plan.BackupPath,
	}); err != nil {
		return err
	}
	if b.SuppressRelaunch {
		return nil
	}
	return b.launchApplication(plan)
}

func (b WindowsBackend) backupInstall(plan ApplyPlan) error {
	if err := os.RemoveAll(plan.BackupPath); err != nil {
		return err
	}
	return copyDir(plan.InstallRoot, plan.BackupPath)
}

func (b WindowsBackend) replaceInstall(plan ApplyPlan) error {
	entries, err := os.ReadDir(plan.InstallRoot)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if preserveInstallEntry(entry.Name()) {
			continue
		}
		if err := b.removeInstallEntryWithRetry(plan, filepath.Join(plan.InstallRoot, entry.Name())); err != nil {
			return err
		}
	}
	return b.copyInstallPayloadWithRetry(plan)
}

func (b WindowsBackend) rollbackInstall(plan ApplyPlan) error {
	entries, err := os.ReadDir(plan.InstallRoot)
	if err == nil {
		for _, entry := range entries {
			if preserveInstallEntry(entry.Name()) {
				continue
			}
			if removeErr := b.removeInstallEntryWithRetry(plan, filepath.Join(plan.InstallRoot, entry.Name())); removeErr != nil {
				return removeErr
			}
		}
	}
	return b.copyBackupWithRetry(plan)
}

func (b WindowsBackend) removeInstallEntryWithRetry(plan ApplyPlan, path string) error {
	return retryWindowsInstallMutation(func() error {
		err := os.RemoveAll(path)
		if err != nil {
			_ = b.closeInstalledProcesses(plan)
		}
		return err
	})
}

func (b WindowsBackend) copyInstallPayloadWithRetry(plan ApplyPlan) error {
	return retryWindowsInstallMutation(func() error {
		err := copyInstallPayload(plan.StagedPath, plan.InstallRoot)
		if err != nil {
			_ = b.closeInstalledProcesses(plan)
		}
		return err
	})
}

func (b WindowsBackend) copyBackupWithRetry(plan ApplyPlan) error {
	return retryWindowsInstallMutation(func() error {
		err := copyDir(plan.BackupPath, plan.InstallRoot)
		if err != nil {
			_ = b.closeInstalledProcesses(plan)
		}
		return err
	})
}

func retryWindowsInstallMutation(fn func() error) error {
	var lastErr error
	for attempt := 0; attempt < 6; attempt++ {
		if err := fn(); err != nil {
			lastErr = err
			time.Sleep(time.Duration(250*(attempt+1)) * time.Millisecond)
			continue
		}
		return nil
	}
	return lastErr
}

func (b WindowsBackend) prepareRunner(plan ApplyPlan) error {
	exe := strings.TrimSpace(plan.CurrentExePath)
	if exe == "" {
		exe = strings.TrimSpace(b.CurrentExePath)
	}
	if exe == "" {
		var err error
		exe, err = os.Executable()
		if err != nil {
			return err
		}
	}
	return copyFileMode(exe, runnerExePath(plan), 0o700)
}

func (b WindowsBackend) launchPostUpdateCheck(plan ApplyPlan, planPath string) error {
	exe := filepath.Join(plan.InstallRoot, "ant-chrome.exe")
	if strings.TrimSpace(plan.CurrentExePath) != "" {
		exe = filepath.Join(plan.InstallRoot, filepath.Base(plan.CurrentExePath))
	}
	cmd := exec.Command(exe, "--post-update-check", planPath)
	hideWindow(cmd)
	return cmd.Start()
}

func (b WindowsBackend) launchApplication(plan ApplyPlan) error {
	exe := filepath.Join(plan.InstallRoot, "ant-chrome.exe")
	if strings.TrimSpace(plan.CurrentExePath) != "" {
		exe = filepath.Join(plan.InstallRoot, filepath.Base(plan.CurrentExePath))
	}
	cmd := exec.Command(exe)
	hideWindow(cmd)
	return cmd.Start()
}

func runnerExePath(plan ApplyPlan) string {
	if path := strings.TrimSpace(plan.RunnerPath); path != "" {
		return filepath.Clean(path)
	}
	return filepath.Join(NewLayout(plan.InstallRoot, plan.StateRoot).RunnerRoot(), "ant-chrome-update-runner.exe")
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, dirMode(info.Mode()))
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return copySafeSymlink(src, path, target)
		}
		return copyFileMode(path, target, info.Mode().Perm())
	})
}

func copyInstallPayload(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dst, dirMode(info.Mode()))
		}
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if len(parts) > 0 && preserveInstallEntry(parts[0]) {
			if _, err := os.Stat(filepath.Join(dst, parts[0])); err == nil {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			} else if !os.IsNotExist(err) {
				return err
			}
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, dirMode(info.Mode()))
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink copy is not supported: %s", path)
		}
		return copyFileMode(path, target, info.Mode().Perm())
	})
}

func preserveInstallEntry(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "data", "runtime", "diagnostics", "config.yaml", "proxies.yaml", ".ant-license.json":
		return true
	default:
		return false
	}
}

func copySafeSymlink(srcRoot, srcPath, dstPath string) error {
	linkTarget, err := os.Readlink(srcPath)
	if err != nil {
		return err
	}
	if strings.TrimSpace(linkTarget) == "" {
		return fmt.Errorf("symlink target is empty: %s", srcPath)
	}
	if filepath.IsAbs(linkTarget) {
		return fmt.Errorf("symlink target uses absolute path: %s -> %s", srcPath, linkTarget)
	}

	srcRootAbs, err := filepath.Abs(srcRoot)
	if err != nil {
		return err
	}
	resolved := filepath.Clean(filepath.Join(filepath.Dir(srcPath), filepath.FromSlash(linkTarget)))
	rel, err := filepath.Rel(srcRootAbs, resolved)
	if err != nil {
		return err
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("symlink target escapes source root: %s -> %s", srcPath, linkTarget)
	}

	if err := os.MkdirAll(filepath.Dir(dstPath), 0o700); err != nil {
		return err
	}
	if err := os.RemoveAll(dstPath); err != nil {
		return err
	}
	return os.Symlink(linkTarget, dstPath)
}

func copyFileMode(src, dst string, mode os.FileMode) error {
	if mode == 0 {
		mode = 0o600
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	syncErr := out.Sync()
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}

func waitForProcessExit(pid int, timeout time.Duration) bool {
	if runtime.GOOS != "windows" || pid <= 0 {
		return true
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !isWindowsProcessRunning(pid) {
			return true
		}
		time.Sleep(250 * time.Millisecond)
	}
	return false
}

func isWindowsProcessRunning(pid int) bool {
	cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH")
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	text := strings.ToLower(string(out))
	return strings.Contains(text, fmt.Sprintf("%d", pid)) && !strings.Contains(text, "no tasks")
}
