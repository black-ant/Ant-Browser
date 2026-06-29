package appupdate

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWindowsBackendRejectsProgramFiles(t *testing.T) {
	backend := WindowsBackend{}
	layout := NewLayout(`C:\Program Files\Maka Browser`, t.TempDir())
	if runtime.GOOS != "windows" {
		layout = NewLayout(`/Program Files/Maka Browser`, t.TempDir())
	}
	if err := backend.ValidateInstallMode(layout); err == nil {
		t.Fatal("expected Program Files install to be rejected")
	}
}

func TestWindowsBackendAllowsWritableUserInstall(t *testing.T) {
	backend := WindowsBackend{}
	layout := NewLayout(filepath.Join(t.TempDir(), "Maka Browser"), t.TempDir())
	if err := backend.ValidateInstallMode(layout); err != nil {
		t.Fatalf("ValidateInstallMode returned error: %v", err)
	}
}

func TestWindowsBackendBackupReplaceAndRollback(t *testing.T) {
	install := t.TempDir()
	state := t.TempDir()
	layout := NewLayout(install, state)
	if err := os.WriteFile(filepath.Join(install, "ant-chrome.exe"), []byte("old"), 0o600); err != nil {
		t.Fatalf("write old exe: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(install, "publish"), 0o755); err != nil {
		t.Fatalf("mkdir old publish: %v", err)
	}
	if err := os.WriteFile(filepath.Join(install, "publish", "runtime-manifest.json"), []byte(`{"old":true}`), 0o600); err != nil {
		t.Fatalf("write old manifest: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(install, "data"), 0o755); err != nil {
		t.Fatalf("mkdir data: %v", err)
	}
	if err := os.WriteFile(filepath.Join(install, "data", "app.db"), []byte("user-db"), 0o600); err != nil {
		t.Fatalf("write data: %v", err)
	}
	if err := os.WriteFile(filepath.Join(install, "config.yaml"), []byte("user-config"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	staged := filepath.Join(t.TempDir(), "staged")
	if err := os.MkdirAll(filepath.Join(staged, "publish"), 0o755); err != nil {
		t.Fatalf("mkdir staged: %v", err)
	}
	if err := os.WriteFile(filepath.Join(staged, "ant-chrome.exe"), []byte("new"), 0o600); err != nil {
		t.Fatalf("write new exe: %v", err)
	}
	if err := os.WriteFile(filepath.Join(staged, "publish", "runtime-manifest.json"), []byte(`{"new":true}`), 0o600); err != nil {
		t.Fatalf("write new manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(staged, "config.yaml"), []byte("default-config"), 0o600); err != nil {
		t.Fatalf("write staged config: %v", err)
	}

	plan := ApplyPlan{
		InstallRoot:   install,
		StateRoot:     state,
		Target:        "windows-amd64",
		OldAppVersion: "1.1.0",
		NewAppVersion: "1.2.0",
		StagedPath:    staged,
		BackupPath:    filepath.Join(layout.BackupsRoot(), "1.1.0-test"),
	}
	backend := WindowsBackend{}
	if err := backend.backupInstall(plan); err != nil {
		t.Fatalf("backupInstall returned error: %v", err)
	}
	if err := backend.replaceInstall(plan); err != nil {
		t.Fatalf("replaceInstall returned error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(install, "ant-chrome.exe"))
	if err != nil {
		t.Fatalf("read replaced exe: %v", err)
	}
	if string(data) != "new" {
		t.Fatalf("expected new exe, got %q", string(data))
	}
	data, err = os.ReadFile(filepath.Join(install, "data", "app.db"))
	if err != nil {
		t.Fatalf("read preserved data: %v", err)
	}
	if string(data) != "user-db" {
		t.Fatalf("expected user data to be preserved, got %q", string(data))
	}
	data, err = os.ReadFile(filepath.Join(install, "config.yaml"))
	if err != nil {
		t.Fatalf("read preserved config: %v", err)
	}
	if string(data) != "user-config" {
		t.Fatalf("expected user config to be preserved, got %q", string(data))
	}
	if err := backend.rollbackInstall(plan); err != nil {
		t.Fatalf("rollbackInstall returned error: %v", err)
	}
	data, err = os.ReadFile(filepath.Join(install, "ant-chrome.exe"))
	if err != nil {
		t.Fatalf("read rolled back exe: %v", err)
	}
	if string(data) != "old" {
		t.Fatalf("expected old exe after rollback, got %q", string(data))
	}
}

func TestWindowsBackendPrepareApplyCopiesRunnerOutsideInstallRoot(t *testing.T) {
	layout := NewLayout(filepath.Join(t.TempDir(), "install"), t.TempDir())
	if err := os.MkdirAll(layout.InstallRoot, 0o755); err != nil {
		t.Fatalf("mkdir install: %v", err)
	}
	currentExe := filepath.Join(layout.InstallRoot, "ant-chrome.exe")
	if err := os.WriteFile(currentExe, []byte("runner"), 0o700); err != nil {
		t.Fatalf("write current exe: %v", err)
	}
	staged := filepath.Join(t.TempDir(), "staged")
	if err := os.MkdirAll(filepath.Join(staged, "publish"), 0o755); err != nil {
		t.Fatalf("mkdir staged publish: %v", err)
	}
	if err := os.WriteFile(filepath.Join(staged, "ant-chrome.exe"), []byte("new"), 0o600); err != nil {
		t.Fatalf("write staged exe: %v", err)
	}
	if err := os.WriteFile(filepath.Join(staged, "publish", "runtime-manifest.json"), []byte(`{"schemaVersion":2}`), 0o600); err != nil {
		t.Fatalf("write staged manifest: %v", err)
	}
	plan := ApplyPlan{
		InstallRoot:    layout.InstallRoot,
		StateRoot:      layout.StateRoot,
		Target:         "windows-amd64",
		StagedPath:     staged,
		CurrentExePath: currentExe,
	}

	if err := (WindowsBackend{}).PrepareApply(plan); err != nil {
		t.Fatalf("PrepareApply returned error: %v", err)
	}
	data, err := os.ReadFile(runnerExePath(plan))
	if err != nil {
		t.Fatalf("read runner exe: %v", err)
	}
	if string(data) != "runner" {
		t.Fatalf("unexpected runner content: %q", string(data))
	}
	if filepath.Dir(runnerExePath(plan)) == layout.InstallRoot {
		t.Fatalf("runner should not be copied into install root")
	}
}

func TestWindowsBackendPostUpdateCheckWritesSucceededState(t *testing.T) {
	layout := NewLayout(filepath.Join(t.TempDir(), "install"), t.TempDir())
	plan := ApplyPlan{
		InstallRoot:   layout.InstallRoot,
		StateRoot:     layout.StateRoot,
		Target:        "windows-amd64",
		OldAppVersion: "1.1.0",
		NewAppVersion: "1.2.0",
	}
	planPath, err := WritePlan(layout, plan)
	if err != nil {
		t.Fatalf("WritePlan returned error: %v", err)
	}

	if err := (WindowsBackend{SuppressRelaunch: true}).PostUpdateCheck(planPath); err != nil {
		t.Fatalf("PostUpdateCheck returned error: %v", err)
	}
	state, err := ReadState(layout)
	if err != nil {
		t.Fatalf("ReadState returned error: %v", err)
	}
	if state.Status != PersistentStatusSucceeded || state.RemoteAppVersion != "1.2.0" {
		t.Fatalf("unexpected state: %+v", state)
	}
}

func TestWindowsBackendPostUpdateCheckRejectsVersionMismatch(t *testing.T) {
	layout := NewLayout(filepath.Join(t.TempDir(), "install"), t.TempDir())
	plan := ApplyPlan{
		InstallRoot:   layout.InstallRoot,
		StateRoot:     layout.StateRoot,
		Target:        "windows-amd64",
		OldAppVersion: "1.1.0",
		NewAppVersion: "1.2.0",
	}
	planPath, err := WritePlan(layout, plan)
	if err != nil {
		t.Fatalf("WritePlan returned error: %v", err)
	}

	err = (WindowsBackend{CurrentAppVersion: "1.1.0", SuppressRelaunch: true}).PostUpdateCheck(planPath)
	if err == nil {
		t.Fatal("expected version mismatch error")
	}
	state, readErr := ReadState(layout)
	if readErr != nil {
		t.Fatalf("ReadState returned error: %v", readErr)
	}
	if state.Status != PersistentStatusFailedManualRepair || state.LastError.Code != "APP-UPDATE-POST-CHECK-VERSION-MISMATCH" {
		t.Fatalf("unexpected mismatch state: %+v", state)
	}
}

func TestWindowsCloseInstalledProcessesScriptMatchesCommandLineReferences(t *testing.T) {
	script := windowsCloseInstalledProcessesScript()
	for _, want := range []string{
		"$_.CommandLine",
		"Contains($rootText",
		"chrome.exe",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("close installed processes script missing %q", want)
		}
	}
}

func TestWindowsBackendUsesRetryForInstallRootReplacement(t *testing.T) {
	source, err := os.ReadFile("windows_backend.go")
	if err != nil {
		t.Fatalf("read windows backend source: %v", err)
	}
	text := string(source)
	for _, want := range []string{
		"removeInstallEntryWithRetry",
		"copyInstallPayloadWithRetry",
		"rollbackInstall",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("windows backend source missing %q", want)
		}
	}
}
