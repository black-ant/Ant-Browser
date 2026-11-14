package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"testing"
)

func TestRecoverBrowserRuntimeStatesRestoresActiveProfile(t *testing.T) {
	app := NewApp(t.TempDir())
	app.config = config.DefaultConfig()
	app.config.Browser.MaxConcurrentInstances = 1
	app.browserMgr = browser.NewManager(app.config, app.appRoot)
	profile := &BrowserProfile{ProfileId: "preserved-profile", UserDataDir: "preserved-profile"}
	app.browserMgr.Profiles[profile.ProfileId] = profile

	originalDetector := detectBrowserRuntimeForStartup
	detectBrowserRuntimeForStartup = func(string) (browserRuntimeDetection, bool) {
		return browserRuntimeDetection{DebugPort: 9222, DebugReady: true}, true
	}
	t.Cleanup(func() { detectBrowserRuntimeForStartup = originalDetector })

	app.recoverBrowserRuntimeStates()

	if !profile.Running || !profile.DebugReady || profile.DebugPort != 9222 {
		t.Fatalf("recovered profile runtime = %+v", profile)
	}
	gate := app.launchGate()
	gate.mu.Lock()
	occupied := gate.occupied()
	gate.mu.Unlock()
	if occupied != 1 {
		t.Fatalf("launch gate occupied = %d, want 1", occupied)
	}

	app.browserMgr.Mutex.Lock()
	app.markProfileStoppedLocked(profile.ProfileId, profile)
	app.browserMgr.Mutex.Unlock()
}
