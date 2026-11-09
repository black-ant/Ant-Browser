package backend

import (
	"testing"

	appconfig "ant-chrome/backend/internal/config"
)

func TestWebviewGPUPolicyNameMapping(t *testing.T) {
	cases := []struct {
		goos    string
		enabled bool
		want    string
	}{
		{goos: "windows", enabled: true, want: "webview2-gpu"},
		{goos: "windows", enabled: false, want: "webview2-software"},
		{goos: "Linux", enabled: true, want: "webkitgtk-on-demand"},
		{goos: "linux", enabled: false, want: "webkitgtk-never"},
		{goos: "darwin", enabled: true, want: "system-managed"},
		{goos: "darwin", enabled: false, want: "system-managed"},
		{goos: "freebsd", enabled: true, want: "unknown"},
	}

	for _, tc := range cases {
		if got := webviewGPUPolicyName(tc.goos, tc.enabled); got != tc.want {
			t.Fatalf("webviewGPUPolicyName(%q, %v) = %q, want %q", tc.goos, tc.enabled, got, tc.want)
		}
	}
}

func TestBuildWebviewGPUStatusMarksPlatformApplicability(t *testing.T) {
	cases := []struct {
		goos       string
		applicable bool
		policy     string
	}{
		{goos: "windows", applicable: true, policy: "webview2-gpu"},
		{goos: "linux", applicable: true, policy: "webkitgtk-on-demand"},
		{goos: "darwin", applicable: false, policy: "system-managed"},
	}

	for _, tc := range cases {
		status := buildWebviewGPUStatus(true, appconfig.WebviewGPUSourceConfig, tc.goos)
		if status.Applicable != tc.applicable {
			t.Fatalf("%s applicable = %v, want %v", tc.goos, status.Applicable, tc.applicable)
		}
		if status.Policy != tc.policy {
			t.Fatalf("%s policy = %q, want %q", tc.goos, status.Policy, tc.policy)
		}
	}
}

func TestResolveWebviewGPUReportsRestartRequired(t *testing.T) {
	status := ResolveWebviewGPU(appconfig.AppConfig{})

	if !status.RestartRequired {
		t.Fatal("webview gpu status must report restartRequired=true")
	}
	if status.Policy == "" || status.Platform == "" {
		t.Fatalf("webview gpu status missing policy/platform: %#v", status)
	}
}

func TestGetWebviewGPUStatusWithoutConfigFallsBackToDefault(t *testing.T) {
	var app *App

	status := app.GetWebviewGPUStatus()

	if status.Enabled {
		t.Fatalf("webview gpu enabled = true, want default false: %#v", status)
	}
	if status.Source != string(appconfig.WebviewGPUSourceDefault) {
		t.Fatalf("webview gpu source = %q, want %q", status.Source, appconfig.WebviewGPUSourceDefault)
	}
}

func TestGetWebviewGPUStatusReadsExplicitConfig(t *testing.T) {
	enabled := true
	app := &App{config: &Config{App: appconfig.AppConfig{WebviewGPU: &enabled}}}

	status := app.GetWebviewGPUStatus()

	if !status.Enabled {
		t.Fatalf("webview gpu enabled = false, want true: %#v", status)
	}
	if status.Source != string(appconfig.WebviewGPUSourceConfig) {
		t.Fatalf("webview gpu source = %q, want %q", status.Source, appconfig.WebviewGPUSourceConfig)
	}
}
