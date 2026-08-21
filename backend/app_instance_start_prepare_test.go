package backend

import (
	"ant-chrome/backend/internal/browser"
	"errors"
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
