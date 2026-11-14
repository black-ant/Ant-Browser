package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"errors"
	"os"
	"testing"
	"time"
)

func TestAppendProfileRuntimeWarningLockedDoesNotRelockManager(t *testing.T) {
	app := NewApp("")
	app.browserMgr = browser.NewManager(nil, "")
	profile := &BrowserProfile{ProfileId: "profile-1"}
	app.browserMgr.Profiles[profile.ProfileId] = profile

	app.browserMgr.Mutex.Lock()
	done := make(chan struct{})
	go func() {
		app.appendProfileRuntimeWarningLocked(profile.ProfileId, profile, "extension load failed")
		close(done)
	}()

	completed := false
	select {
	case <-done:
		completed = true
	case <-time.After(time.Second):
	}
	app.browserMgr.Mutex.Unlock()

	if !completed {
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("runtime warning update remained blocked after manager mutex release")
		}
		t.Fatal("runtime warning update attempted to re-lock the manager mutex")
	}
	if profile.RuntimeWarning != "extension load failed" {
		t.Fatalf("runtime warning = %q", profile.RuntimeWarning)
	}
}

func TestBrowserStartAbortHelper(t *testing.T) {
	if os.Getenv("ANT_CHROME_BROWSER_START_ABORT_HELPER") != "1" {
		return
	}
	<-time.After(10 * time.Second)
}

func TestStartBrowserProfileAbortsPromptlyDuringFullShutdown(t *testing.T) {
	t.Setenv("ANT_CHROME_BROWSER_START_ABORT_HELPER", "1")
	app := NewApp(t.TempDir())
	app.config = config.DefaultConfig()
	app.browserMgr = browser.NewManager(app.config, t.TempDir())
	profile := &BrowserProfile{ProfileId: "profile-shutdown", ProfileName: "shutdown-profile"}
	app.browserMgr.Profiles[profile.ProfileId] = profile

	plan := &browserStartPlan{
		profile:           profile,
		chromeBinaryPath:  os.Args[0],
		userDataDir:       t.TempDir(),
		args:              []string{"-test.run=^TestBrowserStartAbortHelper$", "--"},
		assignedDebugPort: 0,
		startReadyTimeout: 5 * time.Second,
		maxStartAttempts:  1,
		totalReadyTimeout: 5 * time.Second,
	}

	result := make(chan error, 1)
	startedAt := time.Now()
	go func() {
		_, err := app.startBrowserProfileWithPlan(browserStartInput{ProfileID: profile.ProfileId}, plan)
		result <- err
	}()

	time.Sleep(100 * time.Millisecond)
	app.stopRuntimeEvents()

	select {
	case err := <-result:
		if !errors.Is(err, errBrowserStartRuntimeStopped) {
			t.Fatalf("start error = %v, want %v", err, errBrowserStartRuntimeStopped)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("full shutdown did not promptly abort browser startup")
	}
	if elapsed := time.Since(startedAt); elapsed > 4*time.Second {
		t.Fatalf("startup abort took %s, want under 4s", elapsed)
	}
	if profile.Running {
		t.Fatal("aborted browser startup marked the profile as running")
	}
}

func TestBrowserInstanceStopRecoversDetectedRuntime(t *testing.T) {
	app := NewApp(t.TempDir())
	app.config = config.DefaultConfig()
	app.browserMgr = browser.NewManager(app.config, t.TempDir())
	profile := &BrowserProfile{ProfileId: "profile-recovered-stop", ProfileName: "recovered-stop", UserDataDir: "recovered-stop"}
	app.browserMgr.Profiles[profile.ProfileId] = profile

	originalDetect := detectBrowserRuntimeForStop
	originalClose := closeBrowserViaCDPForStop
	originalReachable := debugPortReachableForStop
	detectBrowserRuntimeForStop = func(string) (browserRuntimeDetection, bool) {
		return browserRuntimeDetection{PID: 4321, DebugPort: 9333, DebugReady: true}, true
	}
	closedPort := 0
	closeBrowserViaCDPForStop = func(debugPort int, _ time.Duration) bool {
		closedPort = debugPort
		return true
	}
	debugPortReachableForStop = func(int, time.Duration) bool { return false }
	t.Cleanup(func() {
		detectBrowserRuntimeForStop = originalDetect
		closeBrowserViaCDPForStop = originalClose
		debugPortReachableForStop = originalReachable
	})

	stopped, err := app.BrowserInstanceStop(profile.ProfileId)
	if err != nil {
		t.Fatalf("BrowserInstanceStop() error = %v", err)
	}
	if closedPort != 9333 {
		t.Fatalf("CDP close port = %d, want 9333", closedPort)
	}
	if stopped.Running || stopped.DebugPort != 0 || stopped.Pid != 0 {
		t.Fatalf("stopped profile runtime = %+v, want cleared", stopped)
	}
	if stopped.LastStopAt == "" {
		t.Fatal("stop did not record LastStopAt")
	}
}
