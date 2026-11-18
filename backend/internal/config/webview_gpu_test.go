package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveWebviewGPUDefaultsToDisabledWhenUnset(t *testing.T) {
	// 默认必须与 1.9.0 一致：软件渲染，不让升级改变渲染路径。
	app := AppConfig{}

	enabled, source := resolveWebviewGPU(app, func(string) string { return "" })

	if enabled {
		t.Fatalf("webview gpu enabled = true, want default %v", DefaultWebviewGPUEnabled)
	}
	if DefaultWebviewGPUEnabled {
		t.Fatal("DefaultWebviewGPUEnabled = true, want false")
	}
	if source != WebviewGPUSourceDefault {
		t.Fatalf("webview gpu source = %q, want %q", source, WebviewGPUSourceDefault)
	}
}

func TestResolveWebviewGPUHonorsExplicitConfig(t *testing.T) {
	cases := []struct {
		name string
		file bool
		want bool
	}{
		{name: "explicit enable", file: true, want: true},
		{name: "explicit disable", file: false, want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := AppConfig{WebviewGPU: boolPtr(tc.file)}

			enabled, source := resolveWebviewGPU(app, func(string) string { return "" })

			if enabled != tc.want {
				t.Fatalf("webview gpu enabled = %v, want %v", enabled, tc.want)
			}
			if source != WebviewGPUSourceConfig {
				t.Fatalf("webview gpu source = %q, want %q", source, WebviewGPUSourceConfig)
			}
		})
	}
}

func TestResolveWebviewGPUEnvironmentOverridesConfig(t *testing.T) {
	cases := []struct {
		raw  string
		want bool
	}{
		{raw: "0", want: false},
		{raw: "false", want: false},
		{raw: "OFF", want: false},
		{raw: "1", want: true},
		{raw: "true", want: true},
		{raw: " on ", want: true},
	}

	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			// 配置文件里写死 true，环境变量应当能把它压回 false（反之亦然）。
			app := AppConfig{WebviewGPU: boolPtr(!tc.want)}

			enabled, source := resolveWebviewGPU(app, func(key string) string {
				if key != WebviewGPUEnvVar {
					t.Fatalf("unexpected env key %q", key)
				}
				return tc.raw
			})

			if enabled != tc.want {
				t.Fatalf("webview gpu enabled = %v, want %v (raw=%q)", enabled, tc.want, tc.raw)
			}
			if source != WebviewGPUSourceEnv {
				t.Fatalf("webview gpu source = %q, want %q", source, WebviewGPUSourceEnv)
			}
		})
	}
}

func TestResolveWebviewGPUIgnoresUnparsableEnvironmentValue(t *testing.T) {
	// 拼写错误不应该被当成“关闭”，否则一个 typo 会让用户退回软件渲染还找不到原因。
	app := AppConfig{WebviewGPU: boolPtr(false)}

	enabled, source := resolveWebviewGPU(app, func(string) string { return "maybe" })

	if enabled {
		t.Fatal("webview gpu enabled = true, want config value false")
	}
	if source != WebviewGPUSourceConfig {
		t.Fatalf("webview gpu source = %q, want %q", source, WebviewGPUSourceConfig)
	}
}

func TestDefaultConfigKeepsWebviewGPUDisabled(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.App.WebviewGPU == nil {
		t.Fatal("default app.webview_gpu is nil, want explicit default")
	}
	if *cfg.App.WebviewGPU {
		t.Fatal("default app.webview_gpu = true, want false")
	}
}

func TestLoadConfigKeepsLegacyConfigOnDefaultWebviewGPU(t *testing.T) {
	// 老用户的 config.yaml 没有这一项：必须落到默认值，而不是被当成 false。
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	legacy := strings.Join([]string{
		"app:",
		"    name: Ant Browser",
		"    window:",
		"        width: 1600",
		"        height: 900",
		"        min_width: 1200",
		"        min_height: 700",
	}, "\n")
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.App.WebviewGPU != nil {
		t.Fatalf("legacy config parsed webview_gpu = %v, want nil", *cfg.App.WebviewGPU)
	}
	enabled, source := resolveWebviewGPU(cfg.App, func(string) string { return "" })
	if enabled || source != WebviewGPUSourceDefault {
		t.Fatalf("legacy resolve = (%v, %q), want (false, %q)", enabled, source, WebviewGPUSourceDefault)
	}
}

func TestSaveAndLoadRoundTripWebviewGPU(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := DefaultConfig()
	cfg.App.WebviewGPU = boolPtr(true)
	if err := cfg.Save(path); err != nil {
		t.Fatalf("save config: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	if !strings.Contains(string(raw), "webview_gpu: true") {
		t.Fatalf("saved config does not contain webview_gpu: true:\n%s", raw)
	}

	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if reloaded.App.WebviewGPU == nil || !*reloaded.App.WebviewGPU {
		t.Fatalf("reloaded app.webview_gpu = %v, want true", reloaded.App.WebviewGPU)
	}
}
