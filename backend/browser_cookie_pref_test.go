package backend

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func readPrefs(t *testing.T, dir string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "Default", "Preferences"))
	if err != nil {
		t.Fatal(err)
	}
	var prefs map[string]any
	if err := json.Unmarshal(data, &prefs); err != nil {
		t.Fatal(err)
	}
	return prefs
}

func TestEnsureLaunchPreferencesLivePerf(t *testing.T) {
	dir := t.TempDir()
	ensureLaunchPreferences(dir, true)
	prefs := readPrefs(t, dir)

	if sb, _ := prefs["safebrowsing"].(map[string]any); sb == nil || sb["enabled"] != false {
		t.Error("safebrowsing.enabled should be false")
	}
	if net, _ := prefs["net"].(map[string]any); net == nil || net["network_prediction_options"] != float64(2) {
		t.Error("net.network_prediction_options should be 2 (no prefetch/preconnect)")
	}
	if search, _ := prefs["search"].(map[string]any); search == nil || search["suggest_enabled"] != false {
		t.Error("search.suggest_enabled should be false")
	}
	if prefs["credentials_enable_service"] != false {
		t.Error("credentials_enable_service should be false")
	}
	profile, _ := prefs["profile"].(map[string]any)
	if profile == nil || profile["password_manager_leak_detection"] != false {
		t.Error("password_manager_leak_detection should be false")
	}
}

// 红线:权限状态(Notification.permission 等)JS 可读,绝不能写内容设置。
func TestEnsureLaunchPreferencesNeverWritesContentSettings(t *testing.T) {
	dir := t.TempDir()
	ensureLaunchPreferences(dir, true)
	prefs := readPrefs(t, dir)

	if _, has := prefs["default_content_setting_values"]; has {
		t.Error("must never write top-level content settings")
	}
	if profile, _ := prefs["profile"].(map[string]any); profile != nil {
		if _, has := profile["default_content_setting_values"]; has {
			t.Error("must never write profile.default_content_setting_values (JS-visible permission states)")
		}
	}
}

// 原有行为必须保留:Cookie 放开、去掉崩溃气泡与默认浏览器提示。
func TestEnsureLaunchPreferencesKeepsBaseBehaviour(t *testing.T) {
	for _, livePerf := range []bool{true, false} {
		dir := t.TempDir()
		ensureLaunchPreferences(dir, livePerf)
		prefs := readPrefs(t, dir)

		profile, _ := prefs["profile"].(map[string]any)
		if profile == nil || profile["cookie_controls_mode"] != float64(0) {
			t.Errorf("livePerf=%v: cookie_controls_mode should stay 0", livePerf)
		}
		if profile["exit_type"] != "Normal" || profile["exited_cleanly"] != true {
			t.Errorf("livePerf=%v: exit state prefs lost", livePerf)
		}
		browser, _ := prefs["browser"].(map[string]any)
		if browser == nil || browser["check_default_browser"] != false {
			t.Errorf("livePerf=%v: check_default_browser should stay false", livePerf)
		}
	}
}

func TestEnsureLaunchPreferencesLivePerfOff(t *testing.T) {
	dir := t.TempDir()
	ensureLaunchPreferences(dir, false)
	prefs := readPrefs(t, dir)

	if _, ok := prefs["safebrowsing"]; ok {
		t.Error("perf prefs must not be written when live perf off")
	}
	if _, ok := prefs["net"]; ok {
		t.Error("perf prefs must not be written when live perf off")
	}
}

// 已有 Preferences 的其他字段不得被破坏(读改写合并)。
func TestEnsureLaunchPreferencesPreservesExistingFields(t *testing.T) {
	dir := t.TempDir()
	prefsDir := filepath.Join(dir, "Default")
	if err := os.MkdirAll(prefsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	existing := `{"profile":{"name":"keep-me"},"extensions":{"settings":{"abc":1}}}`
	if err := os.WriteFile(filepath.Join(prefsDir, "Preferences"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	ensureLaunchPreferences(dir, true)
	prefs := readPrefs(t, dir)

	profile, _ := prefs["profile"].(map[string]any)
	if profile == nil || profile["name"] != "keep-me" {
		t.Error("existing profile fields must be preserved")
	}
	if _, ok := prefs["extensions"]; !ok {
		t.Error("unrelated top-level sections must be preserved")
	}
}
