package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestBackupStopRuntimeForMaintenanceClearsProfileRuntimeState(t *testing.T) {
	oldTryClose := backupTryCloseBrowserViaCDP
	oldTerminate := backupTerminateBrowserProcessesByUserDataDir
	backupTryCloseBrowserViaCDP = func(int, time.Duration) bool { return false }
	backupTerminateBrowserProcessesByUserDataDir = func(string, time.Duration) (bool, error) { return false, nil }
	t.Cleanup(func() {
		backupTryCloseBrowserViaCDP = oldTryClose
		backupTerminateBrowserProcessesByUserDataDir = oldTerminate
	})

	root := t.TempDir()
	manager := browser.NewManager(config.DefaultConfig(), root)
	profile := &browser.Profile{
		ProfileId:        "profile-1",
		ProfileName:      "实例 1",
		Running:          true,
		Pid:              1234,
		DebugPort:        9222,
		DebugReady:       true,
		WindowMarkerCode: "7",
	}
	manager.Profiles[profile.ProfileId] = profile
	app := NewApp(root)
	app.browserMgr = manager
	app.browserProcessMonitors[profile.ProfileId] = &browserProcessMonitor{}

	app.backupStopRuntimeForMaintenance()

	if profile.Running || profile.Pid != 0 || profile.DebugPort != 0 || profile.DebugReady || profile.WindowMarkerCode != "" {
		t.Fatalf("profile runtime state was not cleared: %+v", profile)
	}
	if len(manager.BrowserProcesses) != 0 {
		t.Fatalf("tracked browser processes = %d, want 0", len(manager.BrowserProcesses))
	}
	if len(app.browserProcessMonitors) != 0 {
		t.Fatalf("browser process monitors = %d, want 0", len(app.browserProcessMonitors))
	}
}

func TestBackupStopRuntimeForMaintenanceStopsSpeedScheduler(t *testing.T) {
	oldTryClose := backupTryCloseBrowserViaCDP
	oldTerminate := backupTerminateBrowserProcessesByUserDataDir
	backupTryCloseBrowserViaCDP = func(int, time.Duration) bool { return false }
	backupTerminateBrowserProcessesByUserDataDir = func(string, time.Duration) (bool, error) { return false, nil }
	t.Cleanup(func() {
		backupTryCloseBrowserViaCDP = oldTryClose
		backupTerminateBrowserProcessesByUserDataDir = oldTerminate
	})

	root := t.TempDir()
	app := NewApp(root)
	scheduler := browser.NewProxySpeedScheduler(nil, nil, 0, 0)
	scheduler.Start()
	app.speedScheduler = scheduler
	t.Cleanup(scheduler.Stop)

	app.backupStopRuntimeForMaintenance()

	if app.speedScheduler != nil {
		t.Fatal("维护流程应停止并清空测速调度器")
	}
}

func TestBackupStopRuntimeForMaintenanceCancelsBackgroundTasks(t *testing.T) {
	app := NewApp(t.TempDir())
	app.setRuntimeContext(context.Background())
	taskStopped := make(chan struct{})
	if err := app.startBackgroundTask(func(ctx context.Context) {
		<-ctx.Done()
		close(taskStopped)
	}); err != nil {
		t.Fatalf("startBackgroundTask returned error: %v", err)
	}

	if err := app.backupStopRuntimeForMaintenance(); err != nil {
		t.Fatalf("backupStopRuntimeForMaintenance returned error: %v", err)
	}
	select {
	case <-taskStopped:
	case <-time.After(time.Second):
		t.Fatal("background task was not cancelled")
	}
	if err := app.startBackgroundTask(func(context.Context) {}); err == nil {
		t.Fatal("maintenance should block new background tasks")
	}
	app.resumeBackgroundTasks()
	if err := app.startBackgroundTask(func(context.Context) {}); err != nil {
		t.Fatalf("background tasks did not resume after maintenance: %v", err)
	}
}

func TestBackupStopRuntimeForMaintenanceKeepsStateWhenTerminationFails(t *testing.T) {
	oldTryClose := backupTryCloseBrowserViaCDP
	oldTerminate := backupTerminateBrowserProcessesByUserDataDir
	backupTryCloseBrowserViaCDP = func(int, time.Duration) bool { return false }
	backupTerminateBrowserProcessesByUserDataDir = func(string, time.Duration) (bool, error) {
		return false, errors.New("termination blocked")
	}
	t.Cleanup(func() {
		backupTryCloseBrowserViaCDP = oldTryClose
		backupTerminateBrowserProcessesByUserDataDir = oldTerminate
	})

	root := t.TempDir()
	manager := browser.NewManager(config.DefaultConfig(), root)
	profile := &browser.Profile{
		ProfileId:   "profile-1",
		UserDataDir: "profile-1",
		Running:     true,
		Pid:         1234,
		DebugPort:   9222,
	}
	manager.Profiles[profile.ProfileId] = profile
	app := NewApp(root)
	app.browserMgr = manager

	if err := app.backupStopRuntimeForMaintenance(); err == nil {
		t.Fatal("runtime stop returned nil despite process termination failure")
	}
	if !profile.Running || profile.Pid != 1234 || profile.DebugPort != 9222 {
		t.Fatalf("runtime state was cleared after stop failure: %+v", profile)
	}
}

func TestStaleBrowserMonitorCannotStopReusedProfileID(t *testing.T) {
	root := t.TempDir()
	manager := browser.NewManager(config.DefaultConfig(), root)
	oldProfile := &browser.Profile{ProfileId: "profile-1", Running: true}
	manager.Profiles[oldProfile.ProfileId] = oldProfile
	app := NewApp(root)
	app.browserMgr = manager

	monitor := &browserProcessMonitor{waitDone: make(chan struct{})}
	close(monitor.waitDone)
	app.browserProcessMonitors[oldProfile.ProfileId] = monitor
	manager.Mutex.Lock()
	app.markProfileStoppedLocked(oldProfile.ProfileId, oldProfile)
	manager.Mutex.Unlock()

	newProfile := &browser.Profile{ProfileId: oldProfile.ProfileId, Running: true}
	manager.Profiles[newProfile.ProfileId] = newProfile
	app.waitBrowserProcess(newProfile.ProfileId, monitor)

	if !newProfile.Running {
		t.Fatal("stale browser monitor changed the reused profile state")
	}
}

func TestBackupScopePreviewUsesUnifiedStateRoot(t *testing.T) {
	root := filepath.Clean(t.TempDir())
	app := NewApp(root)
	app.config = config.DefaultConfig()

	preview, err := app.BackupGetScopeDefinition()
	if err != nil {
		t.Fatalf("BackupGetScopeDefinition returned error: %v", err)
	}
	exportScope, err := app.backupBuildScope()
	if err != nil {
		t.Fatalf("backupBuildScope returned error: %v", err)
	}
	if preview.AppRoot != app.appStateRootAbs() || exportScope.AppRoot != preview.AppRoot {
		t.Fatalf("scope roots differ: preview=%q export=%q state=%q", preview.AppRoot, exportScope.AppRoot, app.appStateRootAbs())
	}
}
