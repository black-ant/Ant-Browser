package backend

import (
	"os"
	"path/filepath"
	"testing"

	"ant-chrome/backend/internal/appupdate"
)

func TestGetDesktopAppUpdateStateReturnsIdleWhenMissing(t *testing.T) {
	app := NewApp(t.TempDir(), "1.1.0")
	state, err := app.GetDesktopAppUpdateState()
	if err != nil {
		t.Fatalf("GetDesktopAppUpdateState returned error: %v", err)
	}
	if state.Kind != appupdate.UpdateKindNone {
		t.Fatalf("expected none, got %+v", state)
	}
	if state.LocalAppVersion != "1.1.0" {
		t.Fatalf("expected local version, got %+v", state)
	}
}

func TestClearDesktopAppUpdateFailureRemovesState(t *testing.T) {
	root := t.TempDir()
	app := NewApp(root, "1.1.0")
	layout := app.appUpdateLayout()
	if err := os.MkdirAll(filepath.Dir(layout.StatePath()), 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	if err := os.WriteFile(layout.StatePath(), []byte(`{"status":"failed_manual_repair"}`), 0o600); err != nil {
		t.Fatalf("write state: %v", err)
	}
	if err := app.ClearDesktopAppUpdateFailure(); err != nil {
		t.Fatalf("ClearDesktopAppUpdateFailure returned error: %v", err)
	}
	if _, err := os.Stat(layout.StatePath()); !os.IsNotExist(err) {
		t.Fatalf("expected state file removed, err=%v", err)
	}
}

func TestDownloadDesktopAppUpdateUsesManager(t *testing.T) {
	app := NewApp(t.TempDir(), "1.1.0")
	state, err := app.DownloadDesktopAppUpdate()
	if err != nil {
		t.Fatalf("DownloadDesktopAppUpdate returned error: %v", err)
	}
	if state.Kind != appupdate.UpdateKindNone || state.Status != appupdate.PersistentStatusIdle {
		t.Fatalf("expected missing manifest source to disable app update, got %+v", state)
	}
}

func TestAppUpdateManagerUsesSelectedPlatformBackend(t *testing.T) {
	app := NewApp(t.TempDir(), "1.1.0")
	manager := app.appUpdateManager()
	if manager.Platform == nil {
		t.Fatal("expected app update platform backend")
	}
	if manager.Platform.Target() == "" {
		t.Fatal("expected platform target")
	}
}

func TestRunAppUpdateCLIRejectsUnsupportedModeBeforePlatformWork(t *testing.T) {
	err := RunAppUpdateCLI("bogus", filepath.Join(t.TempDir(), "missing-plan.json"), "1.1.0")
	if err == nil {
		t.Fatal("expected unsupported cli mode error")
	}
}

func TestAppUpdateInstallRootForDarwinUsesAppBundleRoot(t *testing.T) {
	bundleRoot := filepath.Join(t.TempDir(), "Applications", "Maka Browser.app")
	exeRoot := filepath.Join(bundleRoot, "Contents", "MacOS")

	got := appUpdateInstallRootForOS("darwin", exeRoot)
	if got != bundleRoot {
		t.Fatalf("expected app bundle root: got=%s want=%s", got, bundleRoot)
	}
}

func TestAppUpdateInstallRootForDarwinKeepsNonBundleRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "dev-root")

	got := appUpdateInstallRootForOS("darwin", root)
	if got != root {
		t.Fatalf("expected non-bundle root unchanged: got=%s want=%s", got, root)
	}
}

func TestAppUpdateInstallRootForWindowsKeepsInstallRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Maka Browser", "bin")

	got := appUpdateInstallRootForOS("windows", root)
	if got != root {
		t.Fatalf("expected windows install root unchanged: got=%s want=%s", got, root)
	}
}

func TestAppUpdateStateRootForWindowsUsesLocalAppDataOutsideInstallRoot(t *testing.T) {
	localAppData := filepath.Join(t.TempDir(), "LocalAppData")
	t.Setenv("LOCALAPPDATA", localAppData)

	installRoot := filepath.Join(localAppData, "Programs", "Ant Browser")
	fallback := installRoot
	got := appUpdateStateRootForOS("windows", installRoot, fallback)
	want := filepath.Join(localAppData, "Ant Browser")

	if got != want {
		t.Fatalf("unexpected app update state root: got=%s want=%s", got, want)
	}
	if pathInsideRoot(got, installRoot) {
		t.Fatalf("app update state root must stay outside install root: state=%s install=%s", got, installRoot)
	}
}

func TestAppUpdateStateRootFallsBackWhenWindowsStateWouldBeInsideInstallRoot(t *testing.T) {
	installRoot := filepath.Join(t.TempDir(), "Maka Browser")
	t.Setenv("LOCALAPPDATA", installRoot)

	fallback := filepath.Join(t.TempDir(), "state")
	got := appUpdateStateRootForOS("windows", installRoot, fallback)

	if got != fallback {
		t.Fatalf("expected fallback state root when LOCALAPPDATA is inside install root: got=%s want=%s", got, fallback)
	}
}
