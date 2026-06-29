package managedinstance_test

import (
	"ant-chrome/backend/internal/apppath"
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/database"
	"ant-chrome/backend/internal/managedinstance"
	"ant-chrome/backend/internal/workspace"
	"encoding/json"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func requireProfile(t *testing.T, mgr *browser.Manager, profileID string) *browser.Profile {
	t.Helper()

	profile, ok := mgr.Profiles[profileID]
	if !ok || profile == nil {
		t.Fatalf("expected profile %s to exist", profileID)
	}
	return profile
}

func testBrowserConfig() *config.Config {
	return config.DefaultConfig()
}

func newManagerWithCore(t *testing.T, coreID, corePath string) *browser.Manager {
	t.Helper()

	appRoot := t.TempDir()
	exePath := filepath.Join(appRoot, filepath.FromSlash(corePath), filepath.FromSlash(browser.CoreExecutableCandidates()[0]))
	if err := os.MkdirAll(filepath.Dir(exePath), 0o755); err != nil {
		t.Fatalf("create core directory: %v", err)
	}
	if err := os.WriteFile(exePath, []byte("stub"), 0o755); err != nil {
		t.Fatalf("write core executable: %v", err)
	}

	cfg := testBrowserConfig()
	cfg.Browser.Cores = []config.BrowserCore{
		{
			CoreId:    coreID,
			CoreName:  "Fingerprint Chromium",
			CorePath:  corePath,
			IsDefault: true,
		},
	}

	return browser.NewManager(cfg, appRoot)
}

func TestOpenManagedShopRequestCarriesWorkspaceBusinessContext(t *testing.T) {
	sessionBundle := workspace.SessionBundle{
		PlatformCode:    "1688",
		LastObservedURL: "https://work.1688.com/dashboard",
		UserAgent:       "test-agent",
	}
	launchContext := workspace.ShopLaunchContext{
		TargetURL:     "https://work.1688.com/?shopId=b2b-222082061706256a1a",
		SessionBundle: sessionBundle,
	}
	req := managedinstance.OpenRequest{
		ShopID:        "b2b-222082061706256a1a",
		ProfileID:     "1688:b2b-222082061706256a1a",
		TargetURL:     "https://work.1688.com/?shopId=b2b-222082061706256a1a",
		LaunchContext: launchContext,
		SessionBundle: sessionBundle,
		ManagedMode:   true,
		SessionReady:  true,
		PreferVisible: true,
	}

	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("unmarshal request payload: %v", err)
	}
	if got["shopId"] != req.ShopID {
		t.Fatalf("expected shopId %q, got %#v", req.ShopID, got["shopId"])
	}
	if got["profileId"] != req.ProfileID {
		t.Fatalf("expected profileId %q, got %#v", req.ProfileID, got["profileId"])
	}
	if got["targetUrl"] != req.TargetURL {
		t.Fatalf("expected targetUrl %q, got %#v", req.TargetURL, got["targetUrl"])
	}
	launchContextJSON, ok := got["launchContext"].(map[string]any)
	if !ok {
		t.Fatalf("expected launchContext object, got %#v", got["launchContext"])
	}
	if launchContextJSON["targetUrl"] != launchContext.TargetURL {
		t.Fatalf("expected launchContext.targetUrl %q, got %#v", launchContext.TargetURL, launchContextJSON["targetUrl"])
	}
	launchContextSessionBundleJSON, ok := launchContextJSON["sessionBundle"].(map[string]any)
	if !ok {
		t.Fatalf("expected launchContext.sessionBundle object, got %#v", launchContextJSON["sessionBundle"])
	}
	if launchContextSessionBundleJSON["lastObservedUrl"] != sessionBundle.LastObservedURL {
		t.Fatalf("expected launchContext.sessionBundle.lastObservedUrl %q, got %#v", sessionBundle.LastObservedURL, launchContextSessionBundleJSON["lastObservedUrl"])
	}
	sessionBundleJSON, ok := got["sessionBundle"].(map[string]any)
	if !ok {
		t.Fatalf("expected sessionBundle object, got %#v", got["sessionBundle"])
	}
	if sessionBundleJSON["platformCode"] != sessionBundle.PlatformCode {
		t.Fatalf("expected sessionBundle.platformCode %q, got %#v", sessionBundle.PlatformCode, sessionBundleJSON["platformCode"])
	}
	if got["managedMode"] != req.ManagedMode {
		t.Fatalf("expected managedMode %v, got %#v", req.ManagedMode, got["managedMode"])
	}
	if got["sessionReady"] != req.SessionReady {
		t.Fatalf("expected sessionReady %v, got %#v", req.SessionReady, got["sessionReady"])
	}
	if got["preferVisible"] != req.PreferVisible {
		t.Fatalf("expected preferVisible %v, got %#v", req.PreferVisible, got["preferVisible"])
	}
}

func TestOpenManagedShopAllowsDetachedStateRootFingerprintCoreOnDarwin(t *testing.T) {
	if goruntime.GOOS != "darwin" {
		t.Skip("detached macOS app bundle state-root core policy is darwin-specific")
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	appRoot := filepath.Join(home, "Applications", "Maka Browser.app", "Contents", "MacOS")
	if err := os.MkdirAll(appRoot, 0o755); err != nil {
		t.Fatalf("create app root: %v", err)
	}

	corePath := "chrome/fingerprint-macos"
	coreExe := filepath.Join(apppath.StateRoot(appRoot), filepath.FromSlash(corePath), filepath.FromSlash(browser.CoreExecutableCandidates()[0]))
	if err := os.MkdirAll(filepath.Dir(coreExe), 0o755); err != nil {
		t.Fatalf("create state-root core dir: %v", err)
	}
	if err := os.WriteFile(coreExe, []byte("stub"), 0o755); err != nil {
		t.Fatalf("write state-root core executable: %v", err)
	}

	cfg := testBrowserConfig()
	cfg.Browser.Cores = []config.BrowserCore{{
		CoreId:    "fingerprint-macos",
		CoreName:  "Fingerprint Chromium",
		CorePath:  corePath,
		IsDefault: true,
	}}
	mgr := browser.NewManager(cfg, appRoot)
	mgr.Profiles["alibaba:b2b-2220886712247ef48d"] = &browser.Profile{
		ProfileId: "alibaba:b2b-2220886712247ef48d",
		CoreId:    "fingerprint-macos",
	}

	service, err := managedinstance.NewService(managedinstance.Dependencies{BrowserMgr: mgr})
	if err != nil {
		t.Fatalf("new managed instance service: %v", err)
	}

	var started bool
	service.SetOpenRuntime(managedinstance.NativeOpenRuntime{
		StartManagedProfile: func(profileID string, targetURL string, preferVisible bool) (*browser.Profile, error) {
			started = true
			return &browser.Profile{
				ProfileId:  profileID,
				DebugReady: true,
				DebugPort:  9222,
				Pid:        1234,
			}, nil
		},
		ImportCookies: func(profileID string, bundle workspace.SessionBundle) error { return nil },
		ListTargets: func(profileID string) ([]workspace.OpenRuntimeTarget, error) {
			return []workspace.OpenRuntimeTarget{{
				TargetID:   "target-1",
				CurrentURL: "https://work.1688.com/",
				PageTitle:  "1688 商家工作台",
			}}, nil
		},
		ActivateTarget:     func(profileID string, targetID string) error { return nil },
		NavigateTarget:     func(profileID string, targetID string, targetURL string) error { return nil },
		CreateTarget:       func(profileID string, targetURL string) (string, error) { return "target-1", nil },
		WaitForTargetReady: func(profileID string, targetID string, timeout time.Duration) error { return nil },
		CloseTarget:        func(profileID string, targetID string) error { return nil },
	})

	result, err := service.OpenManagedShop(managedinstance.OpenRequest{
		ShopID:       "b2b-2220886712247ef48d",
		ProfileID:    "alibaba:b2b-2220886712247ef48d",
		TargetURL:    "https://work.1688.com/",
		ManagedMode:  true,
		SessionReady: true,
	})
	if err != nil {
		t.Fatalf("open managed shop: %v", err)
	}
	if !started {
		t.Fatal("expected state-root fingerprint core to pass policy and start managed profile")
	}
	if result == nil || !result.Success {
		t.Fatalf("expected successful open result, got %#v", result)
	}
}

func TestOpenManagedShopResultCarriesRuntimeOutcome(t *testing.T) {
	result := managedinstance.OpenResult{
		ProfileID:  "1688:b2b-222082061706256a1a",
		PID:        12345,
		DebugPort:  9222,
		CurrentURL: "https://work.1688.com/dashboard",
		PageTitle:  "1688 Workbench",
		Success:    true,
		Code:       "ok",
		Message:    "opened",
	}

	payload, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("unmarshal result payload: %v", err)
	}
	if got["profileId"] != result.ProfileID {
		t.Fatalf("expected profileId %q, got %#v", result.ProfileID, got["profileId"])
	}
	if got["pid"] != float64(result.PID) {
		t.Fatalf("expected pid %d, got %#v", result.PID, got["pid"])
	}
	if got["debugPort"] != float64(result.DebugPort) {
		t.Fatalf("expected debugPort %d, got %#v", result.DebugPort, got["debugPort"])
	}
	if got["currentUrl"] != result.CurrentURL {
		t.Fatalf("expected currentUrl %q, got %#v", result.CurrentURL, got["currentUrl"])
	}
	if got["pageTitle"] != result.PageTitle {
		t.Fatalf("expected pageTitle %q, got %#v", result.PageTitle, got["pageTitle"])
	}
	if got["success"] != result.Success {
		t.Fatalf("expected success %v, got %#v", result.Success, got["success"])
	}
	if got["code"] != result.Code {
		t.Fatalf("expected code %q, got %#v", result.Code, got["code"])
	}
	if got["message"] != result.Message {
		t.Fatalf("expected message %q, got %#v", result.Message, got["message"])
	}
}

func TestNewManagedInstanceServiceRequiresBrowserManager(t *testing.T) {
	_, err := managedinstance.NewService(managedinstance.Dependencies{})
	if err == nil {
		t.Fatal("expected missing browser manager error")
	}
}

func TestNewManagedInstanceServiceAcceptsBrowserManager(t *testing.T) {
	service, err := managedinstance.NewService(managedinstance.Dependencies{
		BrowserMgr: &browser.Manager{},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if service == nil {
		t.Fatal("expected non-nil service")
	}
}

func TestResolveManagedCoreFailsWithoutConfiguredFingerprintCore(t *testing.T) {
	mgr := browser.NewManager(testBrowserConfig(), t.TempDir())
	service, err := managedinstance.NewService(managedinstance.Dependencies{BrowserMgr: mgr})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, err = service.ResolveManagedCore(&browser.Profile{
		ProfileId: "1688:b2b-222082061706256a1a",
		CoreId:    "",
	})
	if err == nil {
		t.Fatal("expected managed core resolution to fail")
	}
	if !strings.Contains(err.Error(), "ANT_FINGERPRINT_CORE_REQUIRED") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveManagedCoreUsesProfileCoreWhenConfigured(t *testing.T) {
	mgr := newManagerWithCore(t, "core-1688", "fingerprint-core")
	service, err := managedinstance.NewService(managedinstance.Dependencies{BrowserMgr: mgr})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	corePath, err := service.ResolveManagedCore(&browser.Profile{
		ProfileId: "1688:b2b-222082061706256a1a",
		CoreId:    "core-1688",
	})
	if err != nil {
		t.Fatalf("resolve core: %v", err)
	}

	expectedPath := filepath.Join(mgr.AppRoot, "fingerprint-core", filepath.FromSlash(browser.CoreExecutableCandidates()[0]))
	if corePath != expectedPath {
		t.Fatalf("unexpected core path: got=%s want=%s", corePath, expectedPath)
	}
}

func TestOpenManagedShopMigratesSystemChromeProfileCoreToFingerprintCore(t *testing.T) {
	appRoot := t.TempDir()
	systemRoot := t.TempDir()
	systemExe := filepath.Join(systemRoot, filepath.FromSlash(browser.CoreExecutableCandidates()[0]))
	if err := os.MkdirAll(filepath.Dir(systemExe), 0o755); err != nil {
		t.Fatalf("create system core directory: %v", err)
	}
	if err := os.WriteFile(systemExe, []byte("stub"), 0o755); err != nil {
		t.Fatalf("write system core executable: %v", err)
	}

	fingerprintCorePath := "chrome/fingerprint-core"
	fingerprintExe := filepath.Join(appRoot, fingerprintCorePath, filepath.FromSlash(browser.CoreExecutableCandidates()[0]))
	if err := os.MkdirAll(filepath.Dir(fingerprintExe), 0o755); err != nil {
		t.Fatalf("create fingerprint core directory: %v", err)
	}
	if err := os.WriteFile(fingerprintExe, []byte("stub"), 0o755); err != nil {
		t.Fatalf("write fingerprint core executable: %v", err)
	}

	cfg := testBrowserConfig()
	cfg.Browser.Cores = []config.BrowserCore{
		{
			CoreId:    "system-chrome",
			CoreName:  "System Chrome",
			CorePath:  systemRoot,
			IsDefault: true,
		},
		{
			CoreId:   "core-1688",
			CoreName: "Fingerprint Chromium",
			CorePath: fingerprintCorePath,
		},
	}
	mgr := browser.NewManager(cfg, appRoot)
	profileID := "1688:b2b-2220216067652a3709"
	profile := &browser.Profile{
		ProfileId:  profileID,
		CoreId:     "system-chrome",
		Running:    true,
		DebugReady: true,
	}
	mgr.Profiles[profileID] = profile

	service, err := managedinstance.NewService(managedinstance.Dependencies{BrowserMgr: mgr})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	service.SetOpenRuntime(managedinstance.NativeOpenRuntime{
		EnsureManagedProfile: func(req managedinstance.OpenRequest) (*browser.Profile, error) {
			return profile, nil
		},
		FindRunningProfile: func(profileID string) (*browser.Profile, bool) {
			return &browser.Profile{ProfileId: profileID, CoreId: profile.CoreId, Running: true, DebugReady: true}, true
		},
		StartManagedProfile: func(profileID string, targetURL string, preferVisible bool) (*browser.Profile, error) {
			return &browser.Profile{ProfileId: profileID, CoreId: profile.CoreId, Running: true, DebugReady: true}, nil
		},
		ImportCookies: func(profileID string, bundle workspace.SessionBundle) error { return nil },
		ListTargets: func(profileID string) ([]workspace.OpenRuntimeTarget, error) {
			return []workspace.OpenRuntimeTarget{{
				TargetID:   "work-1",
				CurrentURL: "https://work.1688.com/?tracelog=login_target_is_blank_1688",
				PageTitle:  "1688-卖家工作台",
			}}, nil
		},
		ActivateTarget:     func(profileID string, targetID string) error { return nil },
		NavigateTarget:     func(profileID string, targetID string, targetURL string) error { return nil },
		CreateTarget:       func(profileID string, targetURL string) (string, error) { return "", nil },
		WaitForTargetReady: func(profileID string, targetID string, timeout time.Duration) error { return nil },
		CloseTarget:        func(profileID string, targetID string) error { return nil },
	})

	result, err := service.OpenManagedShop(managedinstance.OpenRequest{
		ShopID:      "b2b-2220216067652a3709",
		ProfileID:   profileID,
		ManagedMode: true,
		LaunchContext: workspace.ShopLaunchContext{
			SuccessURLPatterns: []string{"https://work.1688.com/"},
		},
	})
	if err != nil {
		t.Fatalf("open managed shop: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success result, got %+v", result)
	}
	if profile.CoreId != "core-1688" {
		t.Fatalf("expected profile core migrated to fingerprint core, got %s", profile.CoreId)
	}
}

func TestOpenManagedShopReusesRunningProfileInServiceLayer(t *testing.T) {
	mgr := newManagerWithCore(t, "core-1688", "fingerprint-core")
	profileID := "1688:b2b-222082061706256a1a"
	profile := &browser.Profile{
		ProfileId:  profileID,
		CoreId:     "core-1688",
		Running:    true,
		DebugReady: true,
		DebugPort:  9222,
		Pid:        12345,
	}
	mgr.Profiles[profileID] = profile

	service, err := managedinstance.NewService(managedinstance.Dependencies{BrowserMgr: mgr})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	var startCalls atomic.Int32
	var activateCalls atomic.Int32
	var importCalls atomic.Int32
	service.SetOpenRuntime(managedinstance.NativeOpenRuntime{
		EnsureManagedProfile: func(req managedinstance.OpenRequest) (*browser.Profile, error) {
			return profile, nil
		},
		FindRunningProfile: func(profileID string) (*browser.Profile, bool) {
			return profile, true
		},
		StartManagedProfile: func(profileID string, targetURL string, preferVisible bool) (*browser.Profile, error) {
			startCalls.Add(1)
			return nil, nil
		},
		ImportCookies: func(profileID string, bundle workspace.SessionBundle) error {
			importCalls.Add(1)
			return nil
		},
		ListTargets: func(profileID string) ([]workspace.OpenRuntimeTarget, error) {
			return []workspace.OpenRuntimeTarget{{
				TargetID:   "backend-1",
				CurrentURL: "https://work.1688.com/?tracelog=login_target_is_blank_1688",
				PageTitle:  "1688-卖家工作台",
			}}, nil
		},
		ActivateTarget: func(profileID string, targetID string) error {
			activateCalls.Add(1)
			return nil
		},
		NavigateTarget: func(profileID string, targetID string, targetURL string) error { return nil },
		CreateTarget:   func(profileID string, targetURL string) (string, error) { return "", nil },
		WaitForTargetReady: func(profileID string, targetID string, timeout time.Duration) error {
			return nil
		},
		CloseTarget: func(profileID string, targetID string) error { return nil },
	})

	result, err := service.OpenManagedShop(managedinstance.OpenRequest{
		ShopID:      "b2b-222082061706256a1a",
		ProfileID:   profileID,
		ManagedMode: true,
		TargetURL:   "https://work.1688.com/?shopId=b2b-222082061706256a1a",
		LaunchContext: workspace.ShopLaunchContext{
			SuccessURLPatterns: []string{"https://work.1688.com/"},
			LoginURLPatterns:   []string{"https://login.1688.com/"},
		},
	})
	if err != nil {
		t.Fatalf("open managed shop: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success result, got %+v", result)
	}
	if startCalls.Load() != 0 {
		t.Fatalf("expected no cold start, got=%d", startCalls.Load())
	}
	if activateCalls.Load() != 1 {
		t.Fatalf("expected activate path once, got=%d", activateCalls.Load())
	}
	if importCalls.Load() != 0 {
		t.Fatalf("expected no cookie import on activate path, got=%d", importCalls.Load())
	}
}

func TestFocusManagedShopActivatesRunningBackendTarget(t *testing.T) {
	mgr := newManagerWithCore(t, "core-1688", "fingerprint-core")
	profileID := "1688:b2b-222082061706256a1a"
	profile := &browser.Profile{
		ProfileId:  profileID,
		CoreId:     "core-1688",
		Running:    true,
		DebugReady: true,
		DebugPort:  9222,
		Pid:        12345,
	}
	mgr.Profiles[profileID] = profile

	service, err := managedinstance.NewService(managedinstance.Dependencies{BrowserMgr: mgr})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	var startCalls atomic.Int32
	var activateCalls atomic.Int32
	var activatedTarget string
	service.SetOpenRuntime(managedinstance.NativeOpenRuntime{
		FindRunningProfile: func(profileID string) (*browser.Profile, bool) {
			return profile, true
		},
		StartManagedProfile: func(profileID string, targetURL string, preferVisible bool) (*browser.Profile, error) {
			startCalls.Add(1)
			return nil, nil
		},
		ImportCookies: func(profileID string, bundle workspace.SessionBundle) error { return nil },
		ListTargets: func(profileID string) ([]workspace.OpenRuntimeTarget, error) {
			return []workspace.OpenRuntimeTarget{{
				TargetID:   "target-workbench",
				CurrentURL: "https://work.1688.com/?shopId=b2b-222082061706256a1a",
				PageTitle:  "1688-卖家工作台",
			}}, nil
		},
		ActivateTarget: func(profileID string, targetID string) error {
			activateCalls.Add(1)
			activatedTarget = targetID
			return nil
		},
		NavigateTarget:     func(profileID string, targetID string, targetURL string) error { return nil },
		CreateTarget:       func(profileID string, targetURL string) (string, error) { return "", nil },
		WaitForTargetReady: func(profileID string, targetID string, timeout time.Duration) error { return nil },
		CloseTarget:        func(profileID string, targetID string) error { return nil },
	})

	result, err := service.FocusManagedShop(managedinstance.OpenRequest{
		ShopID:      "b2b-222082061706256a1a",
		ProfileID:   profileID,
		ManagedMode: true,
		TargetURL:   "https://work.1688.com/?shopId=b2b-222082061706256a1a",
	})
	if err != nil {
		t.Fatalf("focus managed shop: %v", err)
	}
	if result == nil || !result.Success {
		t.Fatalf("expected focus success, got %#v", result)
	}
	if startCalls.Load() != 0 {
		t.Fatalf("expected focus to avoid starting browser, got %d starts", startCalls.Load())
	}
	if activateCalls.Load() != 1 || activatedTarget != "target-workbench" {
		t.Fatalf("expected target activation, calls=%d target=%q", activateCalls.Load(), activatedTarget)
	}
}

func TestFocusManagedShopFailsWhenNoBackendTargetIsOpen(t *testing.T) {
	mgr := newManagerWithCore(t, "core-1688", "fingerprint-core")
	profileID := "1688:b2b-222082061706256a1a"
	profile := &browser.Profile{
		ProfileId:  profileID,
		CoreId:     "core-1688",
		Running:    true,
		DebugReady: true,
		DebugPort:  9222,
		Pid:        12345,
	}
	mgr.Profiles[profileID] = profile

	service, err := managedinstance.NewService(managedinstance.Dependencies{BrowserMgr: mgr})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	var activateCalls atomic.Int32
	service.SetOpenRuntime(managedinstance.NativeOpenRuntime{
		FindRunningProfile: func(profileID string) (*browser.Profile, bool) {
			return profile, true
		},
		StartManagedProfile: func(profileID string, targetURL string, preferVisible bool) (*browser.Profile, error) {
			return nil, nil
		},
		ImportCookies: func(profileID string, bundle workspace.SessionBundle) error { return nil },
		ListTargets: func(profileID string) ([]workspace.OpenRuntimeTarget, error) {
			return []workspace.OpenRuntimeTarget{{
				TargetID:   "blank",
				CurrentURL: "about:blank",
				PageTitle:  "New Tab",
			}}, nil
		},
		ActivateTarget: func(profileID string, targetID string) error {
			activateCalls.Add(1)
			return nil
		},
		NavigateTarget:     func(profileID string, targetID string, targetURL string) error { return nil },
		CreateTarget:       func(profileID string, targetURL string) (string, error) { return "", nil },
		WaitForTargetReady: func(profileID string, targetID string, timeout time.Duration) error { return nil },
		CloseTarget:        func(profileID string, targetID string) error { return nil },
	})

	result, err := service.FocusManagedShop(managedinstance.OpenRequest{
		ShopID:      "b2b-222082061706256a1a",
		ProfileID:   profileID,
		ManagedMode: true,
	})
	if err != nil {
		t.Fatalf("focus managed shop: %v", err)
	}
	if result == nil || result.Success || result.Code != "ANT_TARGET_NOT_FOUND" {
		t.Fatalf("expected target-not-found result, got %#v", result)
	}
	if activateCalls.Load() != 0 {
		t.Fatalf("expected no activation, got %d", activateCalls.Load())
	}
}

func TestOpenManagedShopColdStartFlowRunsInsideServiceLayer(t *testing.T) {
	mgr := newManagerWithCore(t, "core-1688", "fingerprint-core")
	profileID := "1688:b2b-222082061706256a1a"
	profile := &browser.Profile{
		ProfileId: profileID,
		CoreId:    "core-1688",
		DebugPort: 9333,
		Pid:       54321,
	}
	mgr.Profiles[profileID] = profile

	service, err := managedinstance.NewService(managedinstance.Dependencies{BrowserMgr: mgr})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	var runningState atomic.Int32
	var listCalls atomic.Int32
	var startCalls atomic.Int32
	var importCalls atomic.Int32
	var navigateCalls atomic.Int32
	readyProfile := &browser.Profile{
		ProfileId:  profileID,
		CoreId:     "core-1688",
		Running:    true,
		DebugReady: true,
		DebugPort:  9333,
		Pid:        54321,
	}
	service.SetOpenRuntime(managedinstance.NativeOpenRuntime{
		EnsureManagedProfile: func(req managedinstance.OpenRequest) (*browser.Profile, error) {
			return profile, nil
		},
		FindRunningProfile: func(profileID string) (*browser.Profile, bool) {
			if runningState.Load() == 0 {
				return nil, false
			}
			return readyProfile, true
		},
		StartManagedProfile: func(profileID string, targetURL string, preferVisible bool) (*browser.Profile, error) {
			startCalls.Add(1)
			runningState.Store(1)
			return readyProfile, nil
		},
		ImportCookies: func(profileID string, bundle workspace.SessionBundle) error {
			importCalls.Add(1)
			return nil
		},
		ListTargets: func(profileID string) ([]workspace.OpenRuntimeTarget, error) {
			switch listCalls.Add(1) {
			case 1:
				return []workspace.OpenRuntimeTarget{{
					TargetID:   "blank-1",
					CurrentURL: "about:blank",
					PageTitle:  "新标签页",
				}}, nil
			default:
				return []workspace.OpenRuntimeTarget{{
					TargetID:   "blank-1",
					CurrentURL: "https://work.1688.com/?tracelog=login_target_is_blank_1688",
					PageTitle:  "1688-卖家工作台",
				}}, nil
			}
		},
		ActivateTarget: func(profileID string, targetID string) error { return nil },
		NavigateTarget: func(profileID string, targetID string, targetURL string) error {
			navigateCalls.Add(1)
			return nil
		},
		CreateTarget: func(profileID string, targetURL string) (string, error) { return "", nil },
		WaitForTargetReady: func(profileID string, targetID string, timeout time.Duration) error {
			return nil
		},
		CloseTarget: func(profileID string, targetID string) error { return nil },
	})

	result, err := service.OpenManagedShop(managedinstance.OpenRequest{
		ShopID:      "b2b-222082061706256a1a",
		ProfileID:   profileID,
		ManagedMode: true,
		TargetURL:   "about:blank",
		LaunchContext: workspace.ShopLaunchContext{
			SuccessURLPatterns: []string{"https://work.1688.com/"},
			LoginURLPatterns:   []string{"https://login.1688.com/"},
		},
		SessionBundle: workspace.SessionBundle{
			Cookies: []workspace.SessionCookie{{Name: "sid", Value: "1", Domain: ".1688.com", Path: "/"}},
		},
	})
	if err != nil {
		t.Fatalf("open managed shop: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success result, got %+v", result)
	}
	if startCalls.Load() != 1 {
		t.Fatalf("expected one cold start, got=%d", startCalls.Load())
	}
	if importCalls.Load() != 2 {
		t.Fatalf("expected import before reuse and navigate, got=%d", importCalls.Load())
	}
	if navigateCalls.Load() != 1 {
		t.Fatalf("expected one navigate, got=%d", navigateCalls.Load())
	}
	if result.CurrentURL != "https://work.1688.com/?tracelog=login_target_is_blank_1688" {
		t.Fatalf("unexpected current url: %s", result.CurrentURL)
	}
}

func TestOpenManagedShopIsIdempotentForRepeatedClicks(t *testing.T) {
	mgr := newManagerWithCore(t, "core-1688", "fingerprint-core")
	profileID := "1688:b2b-222082061706256a1a"
	profile := &browser.Profile{
		ProfileId:  profileID,
		CoreId:     "core-1688",
		Running:    true,
		DebugReady: true,
		DebugPort:  9222,
		Pid:        12345,
	}
	mgr.Profiles[profileID] = profile

	service, err := managedinstance.NewService(managedinstance.Dependencies{BrowserMgr: mgr})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	var startCalls atomic.Int32
	var activateCalls atomic.Int32
	service.SetOpenRuntime(managedinstance.NativeOpenRuntime{
		EnsureManagedProfile: func(req managedinstance.OpenRequest) (*browser.Profile, error) {
			return profile, nil
		},
		FindRunningProfile: func(profileID string) (*browser.Profile, bool) {
			return profile, true
		},
		StartManagedProfile: func(profileID string, targetURL string, preferVisible bool) (*browser.Profile, error) {
			startCalls.Add(1)
			return nil, nil
		},
		ImportCookies: func(profileID string, bundle workspace.SessionBundle) error { return nil },
		ListTargets: func(profileID string) ([]workspace.OpenRuntimeTarget, error) {
			return []workspace.OpenRuntimeTarget{{
				TargetID:   "backend-1",
				CurrentURL: "https://work.1688.com/?shopId=b2b-222082061706256a1a",
				PageTitle:  "壹级供应链 - 1688后台管理",
			}}, nil
		},
		ActivateTarget: func(profileID string, targetID string) error {
			activateCalls.Add(1)
			return nil
		},
		NavigateTarget: func(profileID string, targetID string, targetURL string) error { return nil },
		CreateTarget:   func(profileID string, targetURL string) (string, error) { return "", nil },
		WaitForTargetReady: func(profileID string, targetID string, timeout time.Duration) error {
			return nil
		},
		CloseTarget: func(profileID string, targetID string) error { return nil },
	})

	req := managedinstance.OpenRequest{
		ShopID:      "b2b-222082061706256a1a",
		ProfileID:   profileID,
		ManagedMode: true,
		TargetURL:   "https://work.1688.com/?shopId=b2b-222082061706256a1a",
		LaunchContext: workspace.ShopLaunchContext{
			SuccessURLPatterns: []string{"https://work.1688.com/"},
			LoginURLPatterns:   []string{"https://login.1688.com/"},
		},
	}

	first, err := service.OpenManagedShop(req)
	if err != nil {
		t.Fatalf("first open managed shop: %v", err)
	}
	second, err := service.OpenManagedShop(req)
	if err != nil {
		t.Fatalf("second open managed shop: %v", err)
	}
	if !first.Success || !second.Success {
		t.Fatalf("expected repeated open to succeed: first=%+v second=%+v", first, second)
	}
	if first.CurrentURL != second.CurrentURL {
		t.Fatalf("expected repeated open to reuse current page: first=%+v second=%+v", first, second)
	}
	if startCalls.Load() != 0 {
		t.Fatalf("expected no cold start on repeated clicks, got=%d", startCalls.Load())
	}
	if activateCalls.Load() != 2 {
		t.Fatalf("expected activate path for both clicks, got=%d", activateCalls.Load())
	}
}

func TestOpenManagedShopCreatesNewTargetWhenBrowserAliveButNoTabs(t *testing.T) {
	mgr := newManagerWithCore(t, "core-1688", "fingerprint-core")
	profileID := "1688:b2b-222082061706256a1a"
	profile := &browser.Profile{
		ProfileId:  profileID,
		CoreId:     "core-1688",
		Running:    true,
		DebugReady: true,
		DebugPort:  9222,
		Pid:        12345,
	}
	mgr.Profiles[profileID] = profile

	service, err := managedinstance.NewService(managedinstance.Dependencies{BrowserMgr: mgr})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	var listCalls atomic.Int32
	var createCalls atomic.Int32
	var waitCalls atomic.Int32
	var navigateCalls atomic.Int32
	var importCalls atomic.Int32
	service.SetOpenRuntime(managedinstance.NativeOpenRuntime{
		EnsureManagedProfile: func(req managedinstance.OpenRequest) (*browser.Profile, error) {
			return profile, nil
		},
		FindRunningProfile: func(profileID string) (*browser.Profile, bool) {
			return profile, true
		},
		StartManagedProfile: func(profileID string, targetURL string, preferVisible bool) (*browser.Profile, error) {
			return nil, nil
		},
		ImportCookies: func(profileID string, bundle workspace.SessionBundle) error {
			importCalls.Add(1)
			return nil
		},
		ListTargets: func(profileID string) ([]workspace.OpenRuntimeTarget, error) {
			switch listCalls.Add(1) {
			case 1:
				return []workspace.OpenRuntimeTarget{{
					TargetID:   "other-1",
					CurrentURL: "https://www.1688.com/",
					PageTitle:  "1688 首页",
				}}, nil
			default:
				return []workspace.OpenRuntimeTarget{{
					TargetID:   "backend-1",
					CurrentURL: "https://work.1688.com/?tracelog=login_target_is_blank_1688",
					PageTitle:  "1688-卖家工作台",
				}}, nil
			}
		},
		ActivateTarget: func(profileID string, targetID string) error { return nil },
		NavigateTarget: func(profileID string, targetID string, targetURL string) error {
			navigateCalls.Add(1)
			return nil
		},
		CreateTarget: func(profileID string, targetURL string) (string, error) {
			createCalls.Add(1)
			return "backend-1", nil
		},
		WaitForTargetReady: func(profileID string, targetID string, timeout time.Duration) error {
			waitCalls.Add(1)
			return nil
		},
		CloseTarget: func(profileID string, targetID string) error { return nil },
	})

	result, err := service.OpenManagedShop(managedinstance.OpenRequest{
		ShopID:      "b2b-222082061706256a1a",
		ProfileID:   profileID,
		ManagedMode: true,
		TargetURL:   "https://work.1688.com/?shopId=b2b-222082061706256a1a",
		LaunchContext: workspace.ShopLaunchContext{
			SuccessURLPatterns: []string{"https://work.1688.com/"},
			LoginURLPatterns:   []string{"https://login.1688.com/"},
		},
		SessionBundle: workspace.SessionBundle{
			Cookies: []workspace.SessionCookie{{Name: "sid", Value: "1", Domain: ".1688.com", Path: "/"}},
		},
	})
	if err != nil {
		t.Fatalf("open managed shop: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success result, got %+v", result)
	}
	if createCalls.Load() != 1 {
		t.Fatalf("expected one target creation, got=%d", createCalls.Load())
	}
	if waitCalls.Load() != 1 {
		t.Fatalf("expected one target ready wait, got=%d", waitCalls.Load())
	}
	if navigateCalls.Load() != 1 {
		t.Fatalf("expected one navigate after target creation, got=%d", navigateCalls.Load())
	}
	if importCalls.Load() != 1 {
		t.Fatalf("expected one cookie import after target creation, got=%d", importCalls.Load())
	}
}

func TestReconcileAuthorizedShopsCreatesAndReusesManagedProfiles(t *testing.T) {
	mgr := newManagerWithCore(t, "core-1688", "fingerprint-core")
	existingProfileID := "1688:shop-001"
	mgr.Profiles[existingProfileID] = &browser.Profile{
		ProfileId:   existingProfileID,
		ProfileName: "旧店铺名",
		UserDataDir: filepath.Join("managed-profiles", "1688__shop-001"),
		CoreId:      "core-1688",
		Tags:        []string{"managed", "managed:desktop", "managed:reclaim-pending", "shop:shop-001"},
	}

	service, err := managedinstance.NewService(managedinstance.Dependencies{BrowserMgr: mgr})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	result, err := service.ReconcileAuthorizedShops([]workspace.ShopRecord{
		{
			ShopID:       "shop-001",
			ShopName:     "一级供应链",
			PlatformCode: "1688",
		},
		{
			ShopID:       "shop-002",
			ShopName:     "二级供应链",
			PlatformCode: "1688",
		},
	})
	if err != nil {
		t.Fatalf("reconcile authorized shops: %v", err)
	}
	if result == nil {
		t.Fatal("expected reconcile result")
	}
	if len(result.UpdatedProfileIDs) != 1 || result.UpdatedProfileIDs[0] != existingProfileID {
		t.Fatalf("unexpected updated profiles: %#v", result.UpdatedProfileIDs)
	}
	if len(result.CreatedProfileIDs) != 1 || result.CreatedProfileIDs[0] != "1688:shop-002" {
		t.Fatalf("unexpected created profiles: %#v", result.CreatedProfileIDs)
	}

	existing := requireProfile(t, mgr, existingProfileID)
	if existing.ProfileName != "一级供应链" {
		t.Fatalf("expected existing profile name updated, got %s", existing.ProfileName)
	}
	if strings.Contains(strings.Join(existing.Tags, ","), "managed:reclaim-pending") {
		t.Fatalf("expected reclaim pending tag removed, got %#v", existing.Tags)
	}

	created := requireProfile(t, mgr, "1688:shop-002")
	if created.UserDataDir != filepath.Join("managed-profiles", "1688__shop-002") {
		t.Fatalf("unexpected user data dir: %s", created.UserDataDir)
	}
	if created.CoreId != "core-1688" {
		t.Fatalf("expected default core assigned, got %s", created.CoreId)
	}
}

func TestReconcileAuthorizedShopsMigratesSystemChromeCoreToFingerprintCore(t *testing.T) {
	appRoot := t.TempDir()
	systemRoot := t.TempDir()
	systemExe := filepath.Join(systemRoot, filepath.FromSlash(browser.CoreExecutableCandidates()[0]))
	if err := os.MkdirAll(filepath.Dir(systemExe), 0o755); err != nil {
		t.Fatalf("create system core directory: %v", err)
	}
	if err := os.WriteFile(systemExe, []byte("stub"), 0o755); err != nil {
		t.Fatalf("write system core executable: %v", err)
	}

	fingerprintCorePath := "chrome/fingerprint-core"
	fingerprintExe := filepath.Join(appRoot, fingerprintCorePath, filepath.FromSlash(browser.CoreExecutableCandidates()[0]))
	if err := os.MkdirAll(filepath.Dir(fingerprintExe), 0o755); err != nil {
		t.Fatalf("create fingerprint core directory: %v", err)
	}
	if err := os.WriteFile(fingerprintExe, []byte("stub"), 0o755); err != nil {
		t.Fatalf("write fingerprint core executable: %v", err)
	}

	cfg := testBrowserConfig()
	cfg.Browser.Cores = []config.BrowserCore{
		{
			CoreId:    "system-chrome",
			CoreName:  "System Chrome",
			CorePath:  systemRoot,
			IsDefault: true,
		},
		{
			CoreId:   "fingerprint-core",
			CoreName: "Fingerprint Chromium",
			CorePath: fingerprintCorePath,
		},
	}
	mgr := browser.NewManager(cfg, appRoot)
	profileID := "1688:shop-001"
	mgr.Profiles[profileID] = &browser.Profile{
		ProfileId:   profileID,
		ProfileName: "旧店铺名",
		UserDataDir: filepath.Join("managed-profiles", "1688__shop-001"),
		CoreId:      "system-chrome",
		Tags:        []string{"managed", "managed:desktop", "shop:shop-001"},
	}

	service, err := managedinstance.NewService(managedinstance.Dependencies{BrowserMgr: mgr})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if _, err := service.ReconcileAuthorizedShops([]workspace.ShopRecord{{
		ShopID:       "shop-001",
		ShopName:     "一级供应链",
		PlatformCode: "1688",
	}}); err != nil {
		t.Fatalf("reconcile authorized shops: %v", err)
	}

	profile := requireProfile(t, mgr, profileID)
	if profile.CoreId != "fingerprint-core" {
		t.Fatalf("expected fingerprint core after reconcile, got %s", profile.CoreId)
	}
}

func TestPreferredManagedCoreIDRegistersBundledFingerprintCoreWhenDAOOnlyHasSystemChrome(t *testing.T) {
	appRoot := t.TempDir()
	systemRoot := t.TempDir()
	systemExe := filepath.Join(systemRoot, filepath.FromSlash(browser.CoreExecutableCandidates()[0]))
	if err := os.MkdirAll(filepath.Dir(systemExe), 0o755); err != nil {
		t.Fatalf("create system core directory: %v", err)
	}
	if err := os.WriteFile(systemExe, []byte("stub"), 0o755); err != nil {
		t.Fatalf("write system core executable: %v", err)
	}

	bundledCorePath := filepath.Join(appRoot, "chrome", "fingerprint")
	bundledExe := filepath.Join(bundledCorePath, filepath.FromSlash(browser.CoreExecutableCandidates()[0]))
	if err := os.MkdirAll(filepath.Dir(bundledExe), 0o755); err != nil {
		t.Fatalf("create bundled core directory: %v", err)
	}
	if err := os.WriteFile(bundledExe, []byte("stub"), 0o755); err != nil {
		t.Fatalf("write bundled core executable: %v", err)
	}

	dataRoot := filepath.Join(appRoot, "data")
	if err := os.MkdirAll(dataRoot, 0o755); err != nil {
		t.Fatalf("create data directory: %v", err)
	}
	db, err := database.NewDB(filepath.Join(dataRoot, "app.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	cfg := testBrowserConfig()
	mgr := browser.NewManager(cfg, appRoot)
	mgr.CoreDAO = browser.NewSQLiteCoreDAO(db.GetConn())
	if err := mgr.CoreDAO.Upsert(browser.Core{
		CoreId:    "system-chrome",
		CoreName:  "System Chrome",
		CorePath:  systemRoot,
		IsDefault: true,
	}); err != nil {
		t.Fatalf("seed system core: %v", err)
	}

	service, err := managedinstance.NewService(managedinstance.Dependencies{BrowserMgr: mgr})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	coreID, ok := service.PreferredManagedCoreID("system-chrome")
	if !ok {
		t.Fatal("expected bundled managed core to be registered")
	}
	if coreID != "bundled-fingerprint" {
		t.Fatalf("expected bundled fingerprint core, got %s", coreID)
	}

	cores := mgr.ListCores()
	found := false
	for _, core := range cores {
		if core.CoreId == "bundled-fingerprint" {
			found = true
			if core.CorePath != filepath.ToSlash(filepath.Join("chrome", "fingerprint")) {
				t.Fatalf("unexpected bundled core path: %s", core.CorePath)
			}
			if !core.IsDefault {
				t.Fatalf("expected bundled core to become default: %+v", core)
			}
		}
	}
	if !found {
		t.Fatalf("expected bundled core in DAO list: %+v", cores)
	}
}

func TestReconcileAuthorizedShopsReclaimsRevokedManagedProfiles(t *testing.T) {
	mgr := newManagerWithCore(t, "core-1688", "fingerprint-core")
	keepProfileID := "1688:shop-keep"
	revokeProfileID := "1688:shop-revoke"
	revokeUserDataDir := filepath.Join("managed-profiles", "1688__shop-revoke")
	revokeUserDataRoot := mgr.ResolveRelativePath(revokeUserDataDir)
	if err := os.MkdirAll(revokeUserDataRoot, 0o755); err != nil {
		t.Fatalf("create user data dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(revokeUserDataRoot, "Cookies"), []byte("stub"), 0o644); err != nil {
		t.Fatalf("write user data marker: %v", err)
	}

	mgr.Profiles[keepProfileID] = &browser.Profile{
		ProfileId:   keepProfileID,
		ProfileName: "保留店铺",
		UserDataDir: filepath.Join("managed-profiles", "1688__shop-keep"),
		CoreId:      "core-1688",
		Tags:        []string{"managed", "managed:desktop", "shop:shop-keep"},
	}
	mgr.Profiles[revokeProfileID] = &browser.Profile{
		ProfileId:   revokeProfileID,
		ProfileName: "撤权店铺",
		UserDataDir: revokeUserDataDir,
		CoreId:      "core-1688",
		Running:     true,
		Tags:        []string{"managed", "managed:desktop", "shop:shop-revoke"},
	}

	service, err := managedinstance.NewService(managedinstance.Dependencies{BrowserMgr: mgr})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	var stopped []string
	service.SetOpenRuntime(managedinstance.NativeOpenRuntime{
		StopManagedProfile: func(profileID string) error {
			stopped = append(stopped, profileID)
			return nil
		},
	})

	result, err := service.ReconcileAuthorizedShops([]workspace.ShopRecord{{
		ShopID:       "shop-keep",
		ShopName:     "保留店铺",
		PlatformCode: "1688",
	}})
	if err != nil {
		t.Fatalf("reconcile authorized shops: %v", err)
	}
	if len(result.ReclaimedProfileIDs) != 1 || result.ReclaimedProfileIDs[0] != revokeProfileID {
		t.Fatalf("unexpected reclaimed profiles: %#v", result.ReclaimedProfileIDs)
	}
	if len(stopped) != 1 || stopped[0] != revokeProfileID {
		t.Fatalf("unexpected stopped profiles: %#v", stopped)
	}
	if _, ok := mgr.Profiles[revokeProfileID]; ok {
		t.Fatalf("expected revoked profile removed from manager")
	}
	if _, err := os.Stat(revokeUserDataRoot); !os.IsNotExist(err) {
		t.Fatalf("expected revoked user data dir removed, stat err=%v", err)
	}
	requireProfile(t, mgr, keepProfileID)
}

func TestReconcileAuthorizedShopsMarksPendingReclaimOnFailure(t *testing.T) {
	mgr := newManagerWithCore(t, "core-1688", "fingerprint-core")
	revokeProfileID := "1688:shop-revoke"
	mgr.Profiles[revokeProfileID] = &browser.Profile{
		ProfileId:   revokeProfileID,
		ProfileName: "撤权店铺",
		UserDataDir: filepath.Join("managed-profiles", "1688__shop-revoke"),
		CoreId:      "core-1688",
		Running:     true,
		Tags:        []string{"managed", "managed:desktop", "shop:shop-revoke"},
	}

	service, err := managedinstance.NewService(managedinstance.Dependencies{BrowserMgr: mgr})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	service.SetOpenRuntime(managedinstance.NativeOpenRuntime{
		StopManagedProfile: func(profileID string) error {
			return os.ErrPermission
		},
	})

	result, err := service.ReconcileAuthorizedShops(nil)
	if err != nil {
		t.Fatalf("expected pending reclaim to be reported in result, got err=%v", err)
	}
	if len(result.PendingReclaimProfileIDs) != 1 || result.PendingReclaimProfileIDs[0] != revokeProfileID {
		t.Fatalf("unexpected pending reclaim profiles: %#v", result.PendingReclaimProfileIDs)
	}

	profile := requireProfile(t, mgr, revokeProfileID)
	if !strings.Contains(strings.Join(profile.Tags, ","), "managed:reclaim-pending") {
		t.Fatalf("expected reclaim pending tag, got %#v", profile.Tags)
	}
}
