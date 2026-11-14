package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"net"
	"testing"
	"time"
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

func TestRecoveredRuntimeConsumesInstanceLaunchSlot(t *testing.T) {
	root := t.TempDir()
	manager := browser.NewManager(config.DefaultConfig(), root)
	profile := &browser.Profile{ProfileId: "recovered-profile"}
	manager.Profiles[profile.ProfileId] = profile
	app := NewApp(root)
	app.config = config.DefaultConfig()
	app.config.Browser.MaxConcurrentInstances = 1
	app.browserMgr = manager

	manager.Mutex.Lock()
	app.markProfileRunningLocked(profile.ProfileId, profile, nil, 0, 0, false, "")
	manager.Mutex.Unlock()

	type acquireResult struct {
		release func()
		queued  bool
	}
	granted := make(chan acquireResult, 1)
	go func() {
		release, queued := app.acquireInstanceLaunchSlot("next-profile")
		granted <- acquireResult{release: release, queued: queued}
	}()

	select {
	case <-granted:
		t.Fatal("recovered running profile did not consume the only launch slot")
	case <-time.After(100 * time.Millisecond):
	}

	manager.Mutex.Lock()
	app.markProfileStoppedLocked(profile.ProfileId, profile)
	manager.Mutex.Unlock()

	select {
	case result := <-granted:
		if !result.queued {
			t.Fatal("next profile should have waited for the recovered runtime to stop")
		}
		result.release()
	case <-time.After(time.Second):
		t.Fatal("stopping recovered runtime did not release the launch slot")
	}
}
