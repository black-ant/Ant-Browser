package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"net"
	"testing"
)

func TestMarkProfileRunningRegistersDetachedBrowserMonitor(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for debug port: %v", err)
	}
	defer listener.Close()

	root := t.TempDir()
	manager := browser.NewManager(config.DefaultConfig(), root)
	profile := &browser.Profile{ProfileId: "profile-1"}
	manager.Profiles[profile.ProfileId] = profile
	app := NewApp(root)
	app.browserMgr = manager
	app.profileWindowMarkers[profile.ProfileId] = &profileWindowMarker{
		code: "A",
		stop: make(chan struct{}),
	}

	debugPort := listener.Addr().(*net.TCPAddr).Port
	manager.Mutex.Lock()
	app.markProfileRunningLocked(profile.ProfileId, profile, nil, 1234, debugPort, true, "")
	monitor := app.browserProcessMonitors[profile.ProfileId]
	manager.Mutex.Unlock()

	if monitor == nil {
		t.Fatal("detached browser monitor was not registered")
	}

	manager.Mutex.Lock()
	app.markProfileStoppedLocked(profile.ProfileId, profile)
	manager.Mutex.Unlock()
}
