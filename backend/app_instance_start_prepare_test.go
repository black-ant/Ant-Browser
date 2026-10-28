package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunningProfileFingerprintExpectedArgsIgnoreExtraLaunchArgs(t *testing.T) {
	app := NewApp(t.TempDir())
	app.browserMgr = browser.NewManager(nil, app.appRoot)
	profile := &browser.Profile{
		ProfileId: "profile-running",
		Running:   true,
		LastLaunchArgs: []string{
			"--fingerprint=123",
			"--lang=zh-CN",
		},
	}

	expectedArgs := app.fingerprintCheckExpectedArgsForRunningProfile(profile, []string{"--timezone=Asia/Tokyo"})
	actual := buildBrowserFingerprintExpected(expectedArgs)
	if actual.Language != "zh-CN" {
		t.Fatalf("language = %q, want zh-CN", actual.Language)
	}
	if actual.Timezone != "" {
		t.Fatalf("timezone = %q, want no extra launch arg in running profile expected args", actual.Timezone)
	}
}

func TestJoinBrowserStartExtensionWarnings(t *testing.T) {
	if joined := joinBrowserStartExtensionWarnings(nil); joined != "" {
		t.Fatalf("nil warnings = %q, want empty", joined)
	}
	warnings := []error{
		errors.New("插件 A 安装失败"),
		nil,
		errors.New("插件 B 安装失败"),
	}
	joined := joinBrowserStartExtensionWarnings(warnings)
	if !strings.Contains(joined, "插件 A 安装失败") || !strings.Contains(joined, "插件 B 安装失败") {
		t.Fatalf("joined warnings = %q, want both entries", joined)
	}

	longWarning := errors.New(strings.Repeat("x", 600))
	joined = joinBrowserStartExtensionWarnings([]error{longWarning})
	if len(joined) != 503 || !strings.HasSuffix(joined, "…") {
		t.Fatalf("long warning not truncated: len=%d suffix=%q", len(joined), joined[len(joined)-3:])
	}
}

func TestPrepareBrowserLaunchContextRejectsRecoveredRuntimeForPortableSession(t *testing.T) {
	root := t.TempDir()
	coreDir := filepath.Join(root, "core")
	if err := os.MkdirAll(coreDir, 0o755); err != nil {
		t.Fatalf("create core directory failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(coreDir, "chrome.exe"), []byte("stub"), 0o755); err != nil {
		t.Fatalf("create fake browser executable failed: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Browser.UserDataRoot = filepath.Join(root, "user-data")
	cfg.Browser.Cores = []config.BrowserCore{{CoreId: "core-1", CorePath: coreDir, IsDefault: true}}
	app := NewApp(root)
	app.config = cfg
	app.browserMgr = browser.NewManager(cfg, root)
	profile := &browser.Profile{ProfileId: "profile-1", ProfileName: "已有运行态", CoreId: "core-1", ProxyId: "__direct__", ProxyConfig: "direct://"}
	input := newBrowserStartInput(profile.ProfileId, nil, []string{"about:blank"}, true, false, true, "", "")
	input.RejectExistingRuntime = true

	oldFind := findBrowserUserDataProcesses
	findBrowserUserDataProcesses = func(userDataDir string) ([]browserUserDataProcess, error) {
		if userDataDir != app.browserMgr.ResolveUserDataDir(profile) {
			return nil, nil
		}
		return []browserUserDataProcess{{PID: 1001, CommandLine: `"C:\\chrome.exe" --user-data-dir="` + userDataDir + `"`}}, nil
	}
	t.Cleanup(func() { findBrowserUserDataProcesses = oldFind })

	_, _, _, _, _, err := app.prepareBrowserLaunchContext(input, profile, nil)
	if !errors.Is(err, errBrowserStartRejectedExistingRuntime) {
		t.Fatalf("portable session launch error = %v, want existing-runtime rejection", err)
	}
}
