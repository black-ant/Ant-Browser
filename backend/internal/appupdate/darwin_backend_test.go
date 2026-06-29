package appupdate

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestDarwinBackendValidateInstallModeAcceptsUserWritableAppBundle(t *testing.T) {
	home := t.TempDir()
	applications := filepath.Join(home, "Applications")
	appRoot := writeFakeDarwinBundle(t, applications)
	layout := NewLayout(appRoot, filepath.Join(home, "Library", "Application Support", "ant-browser"))

	if err := (DarwinBackend{}).ValidateInstallMode(layout); err != nil {
		t.Fatalf("ValidateInstallMode returned error: %v", err)
	}
}

func TestCopyDirCopiesSafeSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires symlink support")
	}
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "dst")
	if err := os.MkdirAll(filepath.Join(src, "Framework.framework", "Versions", "A", "Resources"), 0o755); err != nil {
		t.Fatalf("create resources: %v", err)
	}
	if err := os.WriteFile(filepath.Join(src, "Framework.framework", "Versions", "A", "Resources", "file.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatalf("write resource: %v", err)
	}
	if err := os.Symlink("Versions/A/Resources", filepath.Join(src, "Framework.framework", "Resources")); err != nil {
		t.Skipf("symlink creation unsupported: %v", err)
	}

	if err := copyDir(src, dst); err != nil {
		t.Fatalf("copyDir returned error: %v", err)
	}
	target, err := os.Readlink(filepath.Join(dst, "Framework.framework", "Resources"))
	if err != nil {
		t.Fatalf("read copied symlink: %v", err)
	}
	if target != "Versions/A/Resources" {
		t.Fatalf("unexpected symlink target: %q", target)
	}
}

func TestCopyDirRejectsEscapingSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires symlink support")
	}
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "dst")
	if err := os.Symlink("../escape", filepath.Join(src, "escape-link")); err != nil {
		t.Skipf("symlink creation unsupported: %v", err)
	}

	if err := copyDir(src, dst); err == nil {
		t.Fatal("expected escaping symlink rejection")
	}
}

func TestConfigureDetachedDarwinCommandPreparesLaunchContext(t *testing.T) {
	cmd := exec.Command(filepath.Join(t.TempDir(), "Ant Browser.app", "Contents", "MacOS", "ant-chrome"), "--post-update-check", "plan.json")

	devNull, err := configureDetachedDarwinCommand(cmd)
	if err != nil {
		t.Fatalf("configureDetachedDarwinCommand returned error: %v", err)
	}
	t.Cleanup(func() {
		_ = devNull.Close()
	})

	wantDir := filepath.Dir(cmd.Path)
	if cmd.Dir != wantDir {
		t.Fatalf("cmd.Dir = %q, want %q", cmd.Dir, wantDir)
	}
	if cmd.Stdin == nil || cmd.Stdout == nil || cmd.Stderr == nil {
		t.Fatal("expected detached command stdio to be connected")
	}
	if runtime.GOOS == "darwin" && cmd.SysProcAttr == nil {
		t.Fatal("expected detached darwin command to set SysProcAttr")
	}
}

func TestDarwinBackendValidateInstallModeRejectsApplicationsInstall(t *testing.T) {
	layout := NewLayout("/Applications/Ant Browser.app", filepath.Join(t.TempDir(), "state"))
	if err := (DarwinBackend{}).ValidateInstallMode(layout); err == nil {
		t.Fatal("expected /Applications install to be rejected")
	}
}

func TestDarwinBackendValidateInstallModeRejectsSystemApplicationsInstall(t *testing.T) {
	layout := NewLayout("/System/Applications/Ant Browser.app", filepath.Join(t.TempDir(), "state"))
	if err := (DarwinBackend{}).ValidateInstallMode(layout); err == nil {
		t.Fatal("expected /System/Applications install to be rejected")
	}
}

func TestDarwinBackendValidateInstallModeRejectsSymlinkInstallRoot(t *testing.T) {
	root := t.TempDir()
	target := writeFakeDarwinBundle(t, filepath.Join(root, "Applications"))
	link := filepath.Join(root, "Linked Ant Browser.app")
	if err := os.Symlink(target, link); err != nil {
		if errors.Is(err, os.ErrPermission) {
			t.Skipf("symlink creation unsupported: %v", err)
		}
		t.Fatalf("create install root symlink: %v", err)
	}
	layout := NewLayout(link, filepath.Join(root, "state"))

	if err := (DarwinBackend{}).ValidateInstallMode(layout); err == nil {
		t.Fatal("expected symlink install root to be rejected")
	}
}

func TestDarwinBackendValidateInstallModeRejectsNonAppRoot(t *testing.T) {
	layout := NewLayout(filepath.Join(t.TempDir(), "Maka Browser"), filepath.Join(t.TempDir(), "state"))
	if err := (DarwinBackend{}).ValidateInstallMode(layout); err == nil {
		t.Fatal("expected non-.app install root to be rejected")
	}
}

func TestDarwinBackendValidateInstallModeRejectsStateRootInsideAppBundle(t *testing.T) {
	root := t.TempDir()
	appRoot := writeFakeDarwinBundle(t, root)
	layout := NewLayout(appRoot, filepath.Join(appRoot, "Contents", "MacOS", "data"))
	if err := (DarwinBackend{}).ValidateInstallMode(layout); err == nil {
		t.Fatal("expected state root inside app bundle to be rejected")
	}
}

func TestDarwinBackendValidateInstallModeRejectsStateRootCaseVariantInsideAppBundle(t *testing.T) {
	root := t.TempDir()
	appRoot := writeFakeDarwinBundle(t, root)
	caseVariantAppRoot := filepath.Join(filepath.Dir(appRoot), strings.ToUpper(filepath.Base(appRoot)))
	layout := NewLayout(appRoot, filepath.Join(caseVariantAppRoot, "Contents", "MacOS", "data"))
	if err := (DarwinBackend{}).ValidateInstallMode(layout); err == nil {
		t.Fatal("expected case-variant state root inside app bundle to be rejected")
	}
}

func TestDarwinBackendValidateInstallModeRejectsStateRootSymlinkInsideAppBundle(t *testing.T) {
	root := t.TempDir()
	appRoot := writeFakeDarwinBundle(t, root)
	stateLink := filepath.Join(root, "state-link")
	if err := os.Symlink(filepath.Join(appRoot, "Contents", "MacOS"), stateLink); err != nil {
		if errors.Is(err, os.ErrPermission) {
			t.Skipf("symlink creation unsupported: %v", err)
		}
		t.Fatalf("create state symlink: %v", err)
	}
	layout := NewLayout(appRoot, stateLink)
	if err := (DarwinBackend{}).ValidateInstallMode(layout); err == nil {
		t.Fatal("expected symlinked state root inside app bundle to be rejected")
	}
}

func TestDarwinBackendValidateInstallModeRejectsStateRootWithSymlinkParentInsideAppBundle(t *testing.T) {
	root := t.TempDir()
	appRoot := writeFakeDarwinBundle(t, root)
	stateParentLink := filepath.Join(root, "state-parent-link")
	if err := os.Symlink(filepath.Join(appRoot, "Contents", "MacOS"), stateParentLink); err != nil {
		if errors.Is(err, os.ErrPermission) {
			t.Skipf("symlink creation unsupported: %v", err)
		}
		t.Fatalf("create state parent symlink: %v", err)
	}
	layout := NewLayout(appRoot, filepath.Join(stateParentLink, "state-that-does-not-exist"))

	if err := (DarwinBackend{}).ValidateInstallMode(layout); err == nil {
		t.Fatal("expected state root with symlinked parent inside app bundle to be rejected")
	}
}

func TestDarwinBackendPrepareApplyCopiesRunnerOutsideAppBundle(t *testing.T) {
	skipOnWindowsForExecutableBits(t)

	root := t.TempDir()
	appRoot := writeFakeDarwinBundle(t, filepath.Join(root, "Applications"))
	stateRoot := filepath.Join(root, "state")
	stagedRoot := filepath.Join(root, "staged")
	writeFakeDarwinBundle(t, stagedRoot)
	currentExe := filepath.Join(appRoot, "Contents", "MacOS", "ant-chrome")
	runnerPath := filepath.Join(stateRoot, "app-update", "runner", "darwin-test", "ant-chrome-update-runner")
	plan := ApplyPlan{
		InstallRoot:    appRoot,
		StateRoot:      stateRoot,
		Target:         "darwin-arm64",
		StagedPath:     stagedRoot,
		CurrentExePath: currentExe,
		RunnerPath:     runnerPath,
	}

	if err := (DarwinBackend{}).PrepareApply(plan); err != nil {
		t.Fatalf("PrepareApply returned error: %v", err)
	}
	data, err := os.ReadFile(runnerPath)
	if err != nil {
		t.Fatalf("read runner: %v", err)
	}
	if string(data) != "#!/bin/sh\n" {
		t.Fatalf("unexpected runner content: %q", string(data))
	}
	info, err := os.Stat(runnerPath)
	if err != nil {
		t.Fatalf("stat runner: %v", err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("unexpected runner mode: got %v want %v", info.Mode().Perm(), os.FileMode(0o700))
	}
	rel, err := filepath.Rel(appRoot, runnerPath)
	if err != nil {
		t.Fatalf("runner relative path: %v", err)
	}
	if rel == "." || (!filepath.IsAbs(rel) && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
		t.Fatalf("runner must not live inside app bundle: %s", runnerPath)
	}
}

func TestDarwinBackendPrepareApplyMakesExistingRunnerExecutable(t *testing.T) {
	skipOnWindowsForExecutableBits(t)

	root := t.TempDir()
	appRoot := writeFakeDarwinBundle(t, filepath.Join(root, "Applications"))
	stateRoot := filepath.Join(root, "state")
	stagedRoot := filepath.Join(root, "staged")
	writeFakeDarwinBundle(t, stagedRoot)
	currentExe := filepath.Join(appRoot, "Contents", "MacOS", "ant-chrome")
	runnerPath := filepath.Join(stateRoot, "app-update", "runner", "darwin-test", "ant-chrome-update-runner")
	if err := os.MkdirAll(filepath.Dir(runnerPath), 0o700); err != nil {
		t.Fatalf("mkdir runner dir: %v", err)
	}
	if err := os.WriteFile(runnerPath, []byte("old runner"), 0o600); err != nil {
		t.Fatalf("write existing runner: %v", err)
	}
	plan := ApplyPlan{
		InstallRoot:    appRoot,
		StateRoot:      stateRoot,
		Target:         "darwin-arm64",
		StagedPath:     stagedRoot,
		CurrentExePath: currentExe,
		RunnerPath:     runnerPath,
	}

	if err := (DarwinBackend{}).PrepareApply(plan); err != nil {
		t.Fatalf("PrepareApply returned error: %v", err)
	}
	info, err := os.Stat(runnerPath)
	if err != nil {
		t.Fatalf("stat runner: %v", err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("unexpected runner mode: got %v want %v", info.Mode().Perm(), os.FileMode(0o700))
	}
}

func TestDarwinBackendPrepareApplyRejectsRunnerInsideAppBundle(t *testing.T) {
	skipOnWindowsForExecutableBits(t)

	root := t.TempDir()
	appRoot := writeFakeDarwinBundle(t, filepath.Join(root, "Applications"))
	stateRoot := filepath.Join(root, "state")
	stagedRoot := filepath.Join(root, "staged")
	writeFakeDarwinBundle(t, stagedRoot)
	plan := ApplyPlan{
		InstallRoot:    appRoot,
		StateRoot:      stateRoot,
		Target:         "darwin-arm64",
		StagedPath:     stagedRoot,
		CurrentExePath: filepath.Join(appRoot, "Contents", "MacOS", "ant-chrome"),
		RunnerPath:     filepath.Join(appRoot, "Contents", "MacOS", "ant-chrome-update-runner"),
	}

	if err := (DarwinBackend{}).PrepareApply(plan); err == nil {
		t.Fatal("expected runner inside app bundle to be rejected")
	}
}

func TestDarwinBackendPrepareApplyRejectsRunnerSymlinkInsideAppBundle(t *testing.T) {
	skipOnWindowsForExecutableBits(t)

	root := t.TempDir()
	appRoot := writeFakeDarwinBundle(t, filepath.Join(root, "Applications"))
	stateRoot := filepath.Join(root, "state")
	stagedRoot := filepath.Join(root, "staged")
	writeFakeDarwinBundle(t, stagedRoot)
	insideRunner := filepath.Join(appRoot, "Contents", "MacOS", "ant-chrome-update-runner")
	if err := os.WriteFile(insideRunner, []byte("old runner"), 0o700); err != nil {
		t.Fatalf("write inside runner: %v", err)
	}
	runnerPath := filepath.Join(stateRoot, "app-update", "runner", "darwin-test", "runner-link")
	if err := os.MkdirAll(filepath.Dir(runnerPath), 0o700); err != nil {
		t.Fatalf("mkdir runner dir: %v", err)
	}
	if err := os.Symlink(insideRunner, runnerPath); err != nil {
		if errors.Is(err, os.ErrPermission) {
			t.Skipf("symlink creation unsupported: %v", err)
		}
		t.Fatalf("create runner symlink: %v", err)
	}
	plan := ApplyPlan{
		InstallRoot:    appRoot,
		StateRoot:      stateRoot,
		Target:         "darwin-arm64",
		StagedPath:     stagedRoot,
		CurrentExePath: filepath.Join(appRoot, "Contents", "MacOS", "ant-chrome"),
		RunnerPath:     runnerPath,
	}

	if err := (DarwinBackend{}).PrepareApply(plan); err == nil {
		t.Fatal("expected symlinked runner inside app bundle to be rejected")
	}
}

func TestDarwinBackendPrepareApplyRejectsRunnerWithSymlinkParentInsideAppBundle(t *testing.T) {
	skipOnWindowsForExecutableBits(t)

	root := t.TempDir()
	appRoot := writeFakeDarwinBundle(t, filepath.Join(root, "Applications"))
	stateRoot := filepath.Join(root, "state")
	stagedRoot := filepath.Join(root, "staged")
	writeFakeDarwinBundle(t, stagedRoot)
	runnerParentLink := filepath.Join(stateRoot, "runner-parent-link")
	if err := os.MkdirAll(stateRoot, 0o700); err != nil {
		t.Fatalf("mkdir state root: %v", err)
	}
	if err := os.Symlink(filepath.Join(appRoot, "Contents", "MacOS"), runnerParentLink); err != nil {
		if errors.Is(err, os.ErrPermission) {
			t.Skipf("symlink creation unsupported: %v", err)
		}
		t.Fatalf("create runner parent symlink: %v", err)
	}
	plan := ApplyPlan{
		InstallRoot:    appRoot,
		StateRoot:      stateRoot,
		Target:         "darwin-arm64",
		StagedPath:     stagedRoot,
		CurrentExePath: filepath.Join(appRoot, "Contents", "MacOS", "ant-chrome"),
		RunnerPath:     filepath.Join(runnerParentLink, "runner-that-does-not-exist"),
	}

	if err := (DarwinBackend{}).PrepareApply(plan); err == nil {
		t.Fatal("expected runner with symlinked parent inside app bundle to be rejected")
	}
}

func TestDarwinBackendSpawnApplyRunnerRejectsMissingPreparedRunner(t *testing.T) {
	root := t.TempDir()
	appRoot := writeFakeDarwinBundle(t, filepath.Join(root, "Applications"))
	stateRoot := filepath.Join(root, "state")
	layout := NewLayout(appRoot, stateRoot)
	plan := ApplyPlan{
		InstallRoot:    appRoot,
		StateRoot:      stateRoot,
		Target:         "darwin-arm64",
		RunnerPath:     filepath.Join(stateRoot, "app-update", "runner", "missing-runner"),
		CurrentExePath: filepath.Join(appRoot, "Contents", "MacOS", "ant-chrome"),
	}
	planPath, err := WritePlan(layout, plan)
	if err != nil {
		t.Fatalf("write plan: %v", err)
	}

	if err := (DarwinBackend{}).SpawnApplyRunner(planPath); err == nil {
		t.Fatal("expected missing prepared runner to be rejected")
	}
}

func TestDarwinBackendSpawnApplyRunnerUsesPreparedRunner(t *testing.T) {
	skipOnWindowsForExecutableBits(t)

	root := t.TempDir()
	appRoot := writeFakeDarwinBundle(t, filepath.Join(root, "Applications"))
	stateRoot := filepath.Join(root, "state")
	runnerPath := filepath.Join(stateRoot, "app-update", "runner", "darwin-test", "ant-chrome-update-runner")
	argsPath := filepath.Join(root, "args.txt")
	fallbackMarkerPath := filepath.Join(root, "fallback-used.txt")
	currentExe := filepath.Join(appRoot, "Contents", "MacOS", "ant-chrome")
	if err := os.MkdirAll(filepath.Dir(runnerPath), 0o700); err != nil {
		t.Fatalf("mkdir runner dir: %v", err)
	}
	if err := os.WriteFile(runnerPath, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \""+argsPath+"\"\n"), 0o700); err != nil {
		t.Fatalf("write runner: %v", err)
	}
	if err := os.WriteFile(currentExe, []byte("#!/bin/sh\nprintf fallback > \""+fallbackMarkerPath+"\"\n"), 0o700); err != nil {
		t.Fatalf("write fallback executable: %v", err)
	}

	layout := NewLayout(appRoot, stateRoot)
	plan := ApplyPlan{
		InstallRoot:    appRoot,
		StateRoot:      stateRoot,
		Target:         "darwin-arm64",
		RunnerPath:     runnerPath,
		CurrentExePath: currentExe,
	}
	planPath, err := WritePlan(layout, plan)
	if err != nil {
		t.Fatalf("write plan: %v", err)
	}

	backend := DarwinBackend{CurrentExePath: currentExe}
	if err := backend.SpawnApplyRunner(planPath); err != nil {
		t.Fatalf("SpawnApplyRunner returned error: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(argsPath); err == nil {
			break
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("stat args file: %v", err)
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for runner args file: %s", argsPath)
		}
		time.Sleep(10 * time.Millisecond)
	}
	data, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("read args file: %v", err)
	}
	want := "--apply-update\n" + planPath + "\n"
	if string(data) != want {
		t.Fatalf("runner args = %q, want %q", string(data), want)
	}
	if _, err := os.Stat(fallbackMarkerPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("fallback executable was used, stat err = %v", err)
	}
}

func TestDarwinBackendSpawnApplyRunnerRejectsExistingRunnerInsideAppBundle(t *testing.T) {
	root := t.TempDir()
	appRoot := writeFakeDarwinBundle(t, filepath.Join(root, "Applications"))
	stateRoot := filepath.Join(root, "state")
	runnerPath := filepath.Join(appRoot, "Contents", "MacOS", "ant-chrome-update-runner")
	if err := os.WriteFile(runnerPath, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatalf("write runner: %v", err)
	}
	layout := NewLayout(appRoot, stateRoot)
	plan := ApplyPlan{
		InstallRoot:    appRoot,
		StateRoot:      stateRoot,
		Target:         "darwin-arm64",
		RunnerPath:     runnerPath,
		CurrentExePath: filepath.Join(appRoot, "Contents", "MacOS", "ant-chrome"),
	}
	planPath, err := WritePlan(layout, plan)
	if err != nil {
		t.Fatalf("write plan: %v", err)
	}

	if err := (DarwinBackend{}).SpawnApplyRunner(planPath); err == nil {
		t.Fatal("expected existing runner inside app bundle to be rejected")
	}
}

func TestDarwinBackendSpawnApplyRunnerRejectsExistingRunnerSymlinkInsideAppBundle(t *testing.T) {
	root := t.TempDir()
	appRoot := writeFakeDarwinBundle(t, filepath.Join(root, "Applications"))
	stateRoot := filepath.Join(root, "state")
	insideRunner := filepath.Join(appRoot, "Contents", "MacOS", "ant-chrome-update-runner")
	if err := os.WriteFile(insideRunner, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatalf("write inside runner: %v", err)
	}
	runnerPath := filepath.Join(stateRoot, "runner-link")
	if err := os.MkdirAll(filepath.Dir(runnerPath), 0o700); err != nil {
		t.Fatalf("mkdir runner dir: %v", err)
	}
	if err := os.Symlink(insideRunner, runnerPath); err != nil {
		if errors.Is(err, os.ErrPermission) {
			t.Skipf("symlink creation unsupported: %v", err)
		}
		t.Fatalf("create runner symlink: %v", err)
	}
	layout := NewLayout(appRoot, stateRoot)
	plan := ApplyPlan{
		InstallRoot:    appRoot,
		StateRoot:      stateRoot,
		Target:         "darwin-arm64",
		RunnerPath:     runnerPath,
		CurrentExePath: filepath.Join(appRoot, "Contents", "MacOS", "ant-chrome"),
	}
	planPath, err := WritePlan(layout, plan)
	if err != nil {
		t.Fatalf("write plan: %v", err)
	}

	if err := (DarwinBackend{}).SpawnApplyRunner(planPath); err == nil {
		t.Fatal("expected existing symlinked runner inside app bundle to be rejected")
	}
}

func TestDarwinBackendBackupReplaceAndRollback(t *testing.T) {
	root := t.TempDir()
	installRoot := writeFakeDarwinBundle(t, filepath.Join(root, "Applications"))
	stateRoot := filepath.Join(root, "state")
	if err := os.WriteFile(filepath.Join(installRoot, "Contents", "MacOS", "old-marker.txt"), []byte("old"), 0o600); err != nil {
		t.Fatalf("write old marker: %v", err)
	}

	stagedRoot := filepath.Join(root, "staged")
	stagedApp := writeFakeDarwinBundle(t, stagedRoot)
	if err := os.WriteFile(filepath.Join(stagedApp, "Contents", "MacOS", "new-marker.txt"), []byte("new"), 0o600); err != nil {
		t.Fatalf("write new marker: %v", err)
	}

	plan := ApplyPlan{
		InstallRoot:   installRoot,
		StateRoot:     stateRoot,
		Target:        "darwin-arm64",
		OldAppVersion: "1.1.0",
		NewAppVersion: "1.2.0",
		StagedPath:    stagedRoot,
		BackupPath:    filepath.Join(NewLayout(installRoot, stateRoot).BackupsRoot(), "1.1.0-test"),
	}
	backend := DarwinBackend{SuppressRelaunch: true}

	if err := backend.backupInstall(plan); err != nil {
		t.Fatalf("backupInstall returned error: %v", err)
	}
	if err := backend.replaceInstall(plan); err != nil {
		t.Fatalf("replaceInstall returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(installRoot, "Contents", "MacOS", "new-marker.txt")); err != nil {
		t.Fatalf("expected new marker after replace: %v", err)
	}
	if _, err := os.Stat(filepath.Join(installRoot, "Contents", "MacOS", "old-marker.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected old marker removed after replace, err=%v", err)
	}
	if err := backend.rollbackInstall(plan); err != nil {
		t.Fatalf("rollbackInstall returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(installRoot, "Contents", "MacOS", "old-marker.txt")); err != nil {
		t.Fatalf("expected old marker after rollback: %v", err)
	}
}

func TestDarwinBackendPostUpdateCheckWritesSucceededState(t *testing.T) {
	root := t.TempDir()
	installRoot := writeFakeDarwinBundle(t, filepath.Join(root, "Applications"))
	stateRoot := filepath.Join(root, "state")
	layout := NewLayout(installRoot, stateRoot)
	plan := ApplyPlan{
		InstallRoot:   installRoot,
		StateRoot:     stateRoot,
		Target:        "darwin-arm64",
		OldAppVersion: "1.1.0",
		NewAppVersion: "1.2.0",
	}
	planPath, err := WritePlan(layout, plan)
	if err != nil {
		t.Fatalf("WritePlan returned error: %v", err)
	}

	if err := (DarwinBackend{CurrentAppVersion: "1.2.0", SuppressRelaunch: true}).PostUpdateCheck(planPath); err != nil {
		t.Fatalf("PostUpdateCheck returned error: %v", err)
	}
	state, err := ReadState(layout)
	if err != nil {
		t.Fatalf("ReadState returned error: %v", err)
	}
	if state.Status != PersistentStatusSucceeded || state.LocalAppVersion != "1.2.0" || state.RemoteAppVersion != "1.2.0" {
		t.Fatalf("unexpected state: %+v", state)
	}
}

func TestDarwinBackendPostUpdateCheckAcceptsRenamedAppBundle(t *testing.T) {
	skipOnWindowsForExecutableBits(t)

	root := t.TempDir()
	applications := filepath.Join(root, "Applications")
	installRoot := writeFakeDarwinBundle(t, applications)
	renamedRoot := filepath.Join(applications, "Maka Browser Beta.app")
	if err := os.Rename(installRoot, renamedRoot); err != nil {
		t.Fatalf("rename app bundle: %v", err)
	}
	stateRoot := filepath.Join(root, "state")
	layout := NewLayout(renamedRoot, stateRoot)
	plan := ApplyPlan{
		InstallRoot:   renamedRoot,
		StateRoot:     stateRoot,
		Target:        "darwin-arm64",
		OldAppVersion: "1.1.0",
		NewAppVersion: "1.2.0",
	}
	planPath, err := WritePlan(layout, plan)
	if err != nil {
		t.Fatalf("WritePlan returned error: %v", err)
	}

	if err := (DarwinBackend{CurrentAppVersion: "1.2.0", SuppressRelaunch: true}).PostUpdateCheck(planPath); err != nil {
		t.Fatalf("PostUpdateCheck returned error: %v", err)
	}
}

func TestDarwinBackendPostUpdateCheckLaunchesApplication(t *testing.T) {
	root := t.TempDir()
	installRoot := writeFakeDarwinBundle(t, filepath.Join(root, "Applications"))
	stateRoot := filepath.Join(root, "state")
	layout := NewLayout(installRoot, stateRoot)
	plan := ApplyPlan{
		InstallRoot:   installRoot,
		StateRoot:     stateRoot,
		Target:        "darwin-arm64",
		OldAppVersion: "1.1.0",
		NewAppVersion: "1.2.0",
	}
	planPath, err := WritePlan(layout, plan)
	if err != nil {
		t.Fatalf("WritePlan returned error: %v", err)
	}

	var gotPath string
	var gotArgs []string
	oldStartDetached := startDetachedDarwinCommand
	startDetachedDarwinCommand = func(cmd *exec.Cmd) error {
		gotPath = cmd.Path
		gotArgs = append([]string(nil), cmd.Args...)
		return nil
	}
	t.Cleanup(func() {
		startDetachedDarwinCommand = oldStartDetached
	})

	if err := (DarwinBackend{CurrentAppVersion: "1.2.0"}).PostUpdateCheck(planPath); err != nil {
		t.Fatalf("PostUpdateCheck returned error: %v", err)
	}
	wantPath := filepath.Join(installRoot, "Contents", "MacOS", "ant-chrome")
	if gotPath != wantPath {
		t.Fatalf("launch path = %q, want %q", gotPath, wantPath)
	}
	if len(gotArgs) != 1 || gotArgs[0] != wantPath {
		t.Fatalf("launch args = %q, want [%q]", gotArgs, wantPath)
	}
}

func TestDarwinBackendPostUpdateCheckRejectsVersionMismatch(t *testing.T) {
	root := t.TempDir()
	installRoot := writeFakeDarwinBundle(t, filepath.Join(root, "Applications"))
	stateRoot := filepath.Join(root, "state")
	layout := NewLayout(installRoot, stateRoot)
	plan := ApplyPlan{
		InstallRoot:   installRoot,
		StateRoot:     stateRoot,
		Target:        "darwin-arm64",
		OldAppVersion: "1.1.0",
		NewAppVersion: "1.2.0",
	}
	planPath, err := WritePlan(layout, plan)
	if err != nil {
		t.Fatalf("WritePlan returned error: %v", err)
	}

	err = (DarwinBackend{CurrentAppVersion: "1.1.0", SuppressRelaunch: true}).PostUpdateCheck(planPath)
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

func TestDarwinBackendPostUpdateCheckRejectsInvalidBundle(t *testing.T) {
	root := t.TempDir()
	installRoot := writeFakeDarwinBundle(t, filepath.Join(root, "Applications"))
	stateRoot := filepath.Join(root, "state")
	if err := os.Remove(filepath.Join(installRoot, "Contents", "Info.plist")); err != nil {
		t.Fatalf("remove Info.plist: %v", err)
	}
	layout := NewLayout(installRoot, stateRoot)
	plan := ApplyPlan{
		InstallRoot:   installRoot,
		StateRoot:     stateRoot,
		Target:        "darwin-arm64",
		OldAppVersion: "1.1.0",
		NewAppVersion: "1.2.0",
	}
	planPath, err := WritePlan(layout, plan)
	if err != nil {
		t.Fatalf("WritePlan returned error: %v", err)
	}

	err = (DarwinBackend{CurrentAppVersion: "1.2.0", SuppressRelaunch: true}).PostUpdateCheck(planPath)
	if err == nil {
		t.Fatal("expected invalid bundle error")
	}
	state, readErr := ReadState(layout)
	if readErr != nil {
		t.Fatalf("ReadState returned error: %v", readErr)
	}
	if state.Status != PersistentStatusFailedManualRepair || state.LastError.Code != "APP-UPDATE-POST-CHECK-BUNDLE-INVALID" {
		t.Fatalf("unexpected invalid bundle state: %+v", state)
	}
}

func TestDarwinBackendPostUpdateCheckRejectsInstalledBundleMutableData(t *testing.T) {
	root := t.TempDir()
	applications := filepath.Join(root, "Applications")
	installRoot := writeFakeDarwinBundle(t, applications)
	renamedRoot := filepath.Join(applications, "Maka Browser Beta.app")
	if err := os.Rename(installRoot, renamedRoot); err != nil {
		t.Fatalf("rename app bundle: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(renamedRoot, "Contents", "MacOS", "data"), 0o700); err != nil {
		t.Fatalf("mkdir mutable data: %v", err)
	}
	stateRoot := filepath.Join(root, "state")
	layout := NewLayout(renamedRoot, stateRoot)
	plan := ApplyPlan{
		InstallRoot:   renamedRoot,
		StateRoot:     stateRoot,
		Target:        "darwin-arm64",
		OldAppVersion: "1.1.0",
		NewAppVersion: "1.2.0",
	}
	planPath, err := WritePlan(layout, plan)
	if err != nil {
		t.Fatalf("WritePlan returned error: %v", err)
	}

	err = (DarwinBackend{CurrentAppVersion: "1.2.0", SuppressRelaunch: true}).PostUpdateCheck(planPath)
	if err == nil {
		t.Fatal("expected installed mutable data error")
	}
	state, readErr := ReadState(layout)
	if readErr != nil {
		t.Fatalf("ReadState returned error: %v", readErr)
	}
	if state.Status != PersistentStatusFailedManualRepair || state.LastError.Code != "APP-UPDATE-POST-CHECK-BUNDLE-INVALID" {
		t.Fatalf("unexpected mutable data state: %+v", state)
	}
}

func TestDarwinBackendRunApplyWritesVerifyingAndLaunchesPostCheck(t *testing.T) {
	skipOnWindowsForExecutableBits(t)

	root := t.TempDir()
	installRoot := writeFakeDarwinBundle(t, filepath.Join(root, "Applications"))
	stateRoot := filepath.Join(root, "state")
	if err := os.WriteFile(filepath.Join(installRoot, "Contents", "MacOS", "old-marker.txt"), []byte("old"), 0o600); err != nil {
		t.Fatalf("write old marker: %v", err)
	}

	stagedRoot := filepath.Join(root, "staged")
	stagedApp := writeFakeDarwinBundle(t, stagedRoot)
	postCheckArgsPath := filepath.Join(root, "post-check-args.txt")
	postCheckScript := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"" + postCheckArgsPath + "\"\n"
	if err := os.WriteFile(filepath.Join(stagedApp, "Contents", "MacOS", "ant-chrome"), []byte(postCheckScript), 0o700); err != nil {
		t.Fatalf("write staged ant-chrome: %v", err)
	}
	if err := os.WriteFile(filepath.Join(stagedApp, "Contents", "MacOS", "new-marker.txt"), []byte("new"), 0o600); err != nil {
		t.Fatalf("write new marker: %v", err)
	}

	layout := NewLayout(installRoot, stateRoot)
	plan := ApplyPlan{
		InstallRoot:    installRoot,
		StateRoot:      stateRoot,
		Target:         "darwin-arm64",
		OldAppVersion:  "1.1.0",
		NewAppVersion:  "1.2.0",
		StagedPath:     stagedRoot,
		BackupPath:     filepath.Join(layout.BackupsRoot(), "1.1.0-test"),
		ManifestSource: "test-source",
		ManifestURL:    "https://example.test/manifest.json",
		PayloadURL:     "https://example.test/payload.zip",
	}
	planPath, err := WritePlan(layout, plan)
	if err != nil {
		t.Fatalf("WritePlan returned error: %v", err)
	}

	if err := (DarwinBackend{}).RunApply(planPath); err != nil {
		t.Fatalf("RunApply returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(installRoot, "Contents", "MacOS", "new-marker.txt")); err != nil {
		t.Fatalf("expected new marker after RunApply: %v", err)
	}
	if _, err := os.Stat(filepath.Join(installRoot, "Contents", "MacOS", "old-marker.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected old marker removed after RunApply, err=%v", err)
	}
	state, err := ReadState(layout)
	if err != nil {
		t.Fatalf("ReadState returned error: %v", err)
	}
	if state.Status != PersistentStatusVerifying || state.PlanPath != planPath || state.Target != "darwin-arm64" {
		t.Fatalf("unexpected verifying state: %+v", state)
	}
	if state.ManifestSource != plan.ManifestSource || state.ManifestURL != plan.ManifestURL || state.PayloadURL != plan.PayloadURL {
		t.Fatalf("state lost manifest context: %+v", state)
	}
	waitForFile(t, postCheckArgsPath)
	data, err := os.ReadFile(postCheckArgsPath)
	if err != nil {
		t.Fatalf("read post-check args: %v", err)
	}
	want := "--post-update-check\n" + planPath + "\n"
	if string(data) != want {
		t.Fatalf("post-check args = %q, want %q", string(data), want)
	}
}

func TestDarwinBackendRunApplyRollsBackWhenCopyFails(t *testing.T) {
	skipOnWindowsForExecutableBits(t)

	root := t.TempDir()
	installRoot := writeFakeDarwinBundle(t, filepath.Join(root, "Applications"))
	stateRoot := filepath.Join(root, "state")
	if err := os.WriteFile(filepath.Join(installRoot, "Contents", "MacOS", "old-marker.txt"), []byte("old"), 0o600); err != nil {
		t.Fatalf("write old marker: %v", err)
	}
	stagedRoot := filepath.Join(root, "staged")
	stagedApp := writeFakeDarwinBundle(t, stagedRoot)
	if err := os.Symlink(filepath.Join(stagedApp, "Contents", "Info.plist"), filepath.Join(stagedApp, "Contents", "MacOS", "copy-fails-link")); err != nil {
		if errors.Is(err, os.ErrPermission) {
			t.Skipf("symlink creation unsupported: %v", err)
		}
		t.Fatalf("create staged symlink: %v", err)
	}
	layout := NewLayout(installRoot, stateRoot)
	plan := ApplyPlan{
		InstallRoot:    installRoot,
		StateRoot:      stateRoot,
		Target:         "darwin-arm64",
		OldAppVersion:  "1.1.0",
		NewAppVersion:  "1.2.0",
		StagedPath:     stagedRoot,
		BackupPath:     filepath.Join(layout.BackupsRoot(), "1.1.0-test"),
		ManifestSource: "test-source",
		ManifestURL:    "https://example.test/manifest.json",
		PayloadURL:     "https://example.test/payload.zip",
	}
	planPath, err := WritePlan(layout, plan)
	if err != nil {
		t.Fatalf("WritePlan returned error: %v", err)
	}

	err = (DarwinBackend{}).RunApply(planPath)
	if err == nil {
		t.Fatal("expected RunApply copy failure")
	}
	if _, statErr := os.Stat(filepath.Join(installRoot, "Contents", "MacOS", "old-marker.txt")); statErr != nil {
		t.Fatalf("expected rollback to restore old marker: %v", statErr)
	}
	state, readErr := ReadState(layout)
	if readErr != nil {
		t.Fatalf("ReadState returned error: %v", readErr)
	}
	if state.Status != PersistentStatusRolledBack || state.LastError.Code != "APP-UPDATE-APPLY-FAILED-ROLLED-BACK" {
		t.Fatalf("unexpected rollback state: %+v", state)
	}
	if state.PlanPath != planPath || state.Target != plan.Target || state.ManifestURL != plan.ManifestURL || state.PayloadURL != plan.PayloadURL {
		t.Fatalf("rollback state lost plan context: %+v", state)
	}
}

func TestDarwinBackendRunApplyRejectsTamperedStagedPayloadBeforeRemovingInstall(t *testing.T) {
	skipOnWindowsForExecutableBits(t)

	root := t.TempDir()
	installRoot := writeFakeDarwinBundle(t, filepath.Join(root, "Applications"))
	stateRoot := filepath.Join(root, "state")
	oldMarker := filepath.Join(installRoot, "Contents", "MacOS", "old-marker.txt")
	if err := os.WriteFile(oldMarker, []byte("old"), 0o600); err != nil {
		t.Fatalf("write old marker: %v", err)
	}
	stagedRoot := filepath.Join(root, "staged")
	stagedApp := writeFakeDarwinBundle(t, stagedRoot)
	if err := os.Remove(filepath.Join(stagedApp, "Contents", "Info.plist")); err != nil {
		t.Fatalf("remove staged Info.plist: %v", err)
	}
	layout := NewLayout(installRoot, stateRoot)
	plan := ApplyPlan{
		InstallRoot:   installRoot,
		StateRoot:     stateRoot,
		Target:        "darwin-arm64",
		OldAppVersion: "1.1.0",
		NewAppVersion: "1.2.0",
		StagedPath:    stagedRoot,
		BackupPath:    filepath.Join(layout.BackupsRoot(), "1.1.0-test"),
	}
	planPath, err := WritePlan(layout, plan)
	if err != nil {
		t.Fatalf("WritePlan returned error: %v", err)
	}

	err = (DarwinBackend{}).RunApply(planPath)
	if err == nil {
		t.Fatal("expected tampered staged payload error")
	}
	if _, statErr := os.Stat(oldMarker); statErr != nil {
		t.Fatalf("install should remain in place before failed replace: %v", statErr)
	}
}

func TestDarwinBackendRunApplyRejectsBackupPathInsideAppBeforeRemovingInstall(t *testing.T) {
	skipOnWindowsForExecutableBits(t)

	root := t.TempDir()
	installRoot := writeFakeDarwinBundle(t, filepath.Join(root, "Applications"))
	stateRoot := filepath.Join(root, "state")
	oldMarker := filepath.Join(installRoot, "Contents", "MacOS", "old-marker.txt")
	if err := os.WriteFile(oldMarker, []byte("old"), 0o600); err != nil {
		t.Fatalf("write old marker: %v", err)
	}
	stagedRoot := filepath.Join(root, "staged")
	writeFakeDarwinBundle(t, stagedRoot)
	layout := NewLayout(installRoot, stateRoot)
	plan := ApplyPlan{
		InstallRoot:   installRoot,
		StateRoot:     stateRoot,
		Target:        "darwin-arm64",
		OldAppVersion: "1.1.0",
		NewAppVersion: "1.2.0",
		StagedPath:    stagedRoot,
		BackupPath:    filepath.Join(installRoot, "Contents", "MacOS", "backup"),
	}
	planPath, err := WritePlan(layout, plan)
	if err != nil {
		t.Fatalf("WritePlan returned error: %v", err)
	}

	err = (DarwinBackend{}).RunApply(planPath)
	if err == nil {
		t.Fatal("expected backup path inside app error")
	}
	if _, statErr := os.Stat(oldMarker); statErr != nil {
		t.Fatalf("install should remain in place before invalid backup path: %v", statErr)
	}
}

func TestDarwinBackendRunApplyRejectsRunningProcessBeforeReplace(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Darwin process waiting only runs on macOS")
	}
	skipOnWindowsForExecutableBits(t)

	root := t.TempDir()
	installRoot := writeFakeDarwinBundle(t, filepath.Join(root, "Applications"))
	stateRoot := filepath.Join(root, "state")
	oldMarker := filepath.Join(installRoot, "Contents", "MacOS", "old-marker.txt")
	if err := os.WriteFile(oldMarker, []byte("old"), 0o600); err != nil {
		t.Fatalf("write old marker: %v", err)
	}
	stagedRoot := filepath.Join(root, "staged")
	writeFakeDarwinBundle(t, stagedRoot)

	sleepCmd := exec.Command("sleep", "2")
	if err := sleepCmd.Start(); err != nil {
		t.Fatalf("start sleep: %v", err)
	}
	defer func() {
		_ = sleepCmd.Process.Kill()
		_ = sleepCmd.Wait()
	}()

	oldTimeout := darwinProcessExitTimeout
	oldInterval := darwinProcessPollInterval
	darwinProcessExitTimeout = 30 * time.Millisecond
	darwinProcessPollInterval = 5 * time.Millisecond
	defer func() {
		darwinProcessExitTimeout = oldTimeout
		darwinProcessPollInterval = oldInterval
	}()

	layout := NewLayout(installRoot, stateRoot)
	plan := ApplyPlan{
		InstallRoot:      installRoot,
		StateRoot:        stateRoot,
		Target:           "darwin-arm64",
		OldAppVersion:    "1.1.0",
		NewAppVersion:    "1.2.0",
		StagedPath:       stagedRoot,
		BackupPath:       filepath.Join(layout.BackupsRoot(), "1.1.0-test"),
		WaitForProcessID: sleepCmd.Process.Pid,
	}
	planPath, err := WritePlan(layout, plan)
	if err != nil {
		t.Fatalf("WritePlan returned error: %v", err)
	}

	err = (DarwinBackend{}).RunApply(planPath)
	if err == nil {
		t.Fatal("expected running process timeout")
	}
	if _, statErr := os.Stat(oldMarker); statErr != nil {
		t.Fatalf("install should not be replaced while process is running: %v", statErr)
	}
	state, readErr := ReadState(layout)
	if readErr != nil {
		t.Fatalf("ReadState returned error: %v", readErr)
	}
	if state.Status != PersistentStatusFailedManualRepair || state.LastError.Code != "APP-UPDATE-PROCESS-STILL-RUNNING" {
		t.Fatalf("unexpected running process state: %+v", state)
	}
}

func TestDarwinRunnerPathDefaultUnderRunnerRoot(t *testing.T) {
	root := t.TempDir()
	appRoot := filepath.Join(root, "Ant Browser.app")
	stateRoot := filepath.Join(root, "state")
	layout := NewLayout(appRoot, stateRoot)

	got := darwinRunnerPath(ApplyPlan{InstallRoot: appRoot, StateRoot: stateRoot})
	want := filepath.Join(layout.RunnerRoot(), "ant-chrome-update-runner")
	if got != want {
		t.Fatalf("darwinRunnerPath default = %q, want %q", got, want)
	}
}

func waitForFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(path); err == nil {
			return
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("stat file %s: %v", path, err)
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for file: %s", path)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestDarwinProtectedApplicationInstallRoot(t *testing.T) {
	cases := []struct {
		name string
		path string
		want bool
	}{
		{name: "applications app", path: "/Applications/Ant Browser.app", want: true},
		{name: "system applications app", path: "/System/Applications/Ant Browser.app", want: true},
		{name: "case variant", path: "/applications/Ant Browser.app", want: true},
		{name: "protected target bypass", path: "/Applications/Ant Browser.app", want: true},
		{name: "user applications", path: filepath.Join(t.TempDir(), "Applications", "Ant Browser.app"), want: false},
		{name: "prefix sibling", path: "/ApplicationsBackup/Ant Browser.app", want: false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := darwinProtectedApplicationInstallRoot(tt.path); got != tt.want {
				t.Fatalf("darwinProtectedApplicationInstallRoot(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestDarwinProtectedApplicationInstallRootRejectsResolvedProtectedTarget(t *testing.T) {
	raw := filepath.Join(t.TempDir(), "Applications", "Ant Browser.app")
	resolved := "/Applications/Ant Browser.app"

	if darwinProtectedApplicationInstallRoot(raw) {
		t.Fatalf("raw user install path should not be protected: %s", raw)
	}
	if !darwinProtectedApplicationInstallRoot(resolved) {
		t.Fatalf("resolved protected install path should be protected: %s", resolved)
	}
	if !(darwinProtectedApplicationInstallRoot(raw) || darwinProtectedApplicationInstallRoot(resolved)) {
		t.Fatal("expected raw/resolved protected check to reject resolved protected target")
	}
}

func TestPathInsideRootDarwin(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Ant Browser.app")
	cases := []struct {
		name string
		path string
		root string
		want bool
	}{
		{name: "empty path", path: "", root: root, want: false},
		{name: "empty root", path: filepath.Join(root, "Contents"), root: "", want: false},
		{name: "same root", path: root, root: root, want: true},
		{name: "child", path: filepath.Join(root, "Contents", "MacOS"), root: root, want: true},
		{name: "sibling prefix", path: root + "-backup", root: root, want: false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := pathInsideRootDarwin(tt.path, tt.root); got != tt.want {
				t.Fatalf("pathInsideRootDarwin(%q, %q) = %v, want %v", tt.path, tt.root, got, tt.want)
			}
		})
	}
}
