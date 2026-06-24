package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLoadBackfillsLegacyConfig(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	legacyConfig := `
app:
  used_cd_keys:
    - GITHUB_STAR_REWARD
logging: {}
browser: {}
`
	if err := os.WriteFile(configPath, []byte(legacyConfig), 0o644); err != nil {
		t.Fatalf("写入测试配置失败: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	if cfg.Database.Type != "sqlite" {
		t.Fatalf("Database.Type 未补齐: got=%q", cfg.Database.Type)
	}
	if cfg.Database.SQLite.Path != "data/app.db" {
		t.Fatalf("Database.SQLite.Path 未补齐: got=%q", cfg.Database.SQLite.Path)
	}
	if cfg.App.Name != "Ant Browser" {
		t.Fatalf("App.Name 未补齐: got=%q", cfg.App.Name)
	}
	if cfg.App.MaxProfileLimit != GithubStarProfileTotal {
		t.Fatalf("MaxProfileLimit 计算错误: got=%d want=%d", cfg.App.MaxProfileLimit, GithubStarProfileTotal)
	}
	if cfg.Runtime.MaxMemoryMB != 0 || cfg.Runtime.GCPercent != 100 {
		t.Fatalf("Runtime 未补齐: got=%+v", cfg.Runtime)
	}
	if cfg.Logging.Level != "info" || cfg.Logging.FilePath != "data/logs/app.log" {
		t.Fatalf("Logging 基础字段未补齐: got=%+v", cfg.Logging)
	}
	if !cfg.Logging.Interceptor.Enabled || !cfg.Logging.Interceptor.LogParameters || !cfg.Logging.Interceptor.LogResults {
		t.Fatalf("Interceptor 默认值未补齐: got=%+v", cfg.Logging.Interceptor)
	}
	if len(cfg.Logging.Interceptor.SensitiveFields) == 0 {
		t.Fatalf("Interceptor.SensitiveFields 未补齐")
	}
	if cfg.Browser.UserDataRoot != "data" {
		t.Fatalf("Browser.UserDataRoot 未补齐: got=%q", cfg.Browser.UserDataRoot)
	}
	if len(cfg.Browser.DefaultFingerprintArgs) == 0 || len(cfg.Browser.DefaultLaunchArgs) == 0 {
		t.Fatalf("Browser 默认启动参数未补齐")
	}
	if cfg.Browser.Cores == nil || cfg.Browser.Proxies == nil || cfg.Browser.Profiles == nil {
		t.Fatalf("Browser 列表字段应初始化为空切片")
	}
	if cfg.LaunchServer.Port != DefaultLaunchServerPort {
		t.Fatalf("LaunchServer.Port 未补齐: got=%d", cfg.LaunchServer.Port)
	}
	if cfg.LaunchServer.Auth.Enabled {
		t.Fatalf("LaunchServer.Auth.Enabled 默认应为 false: got=%v", cfg.LaunchServer.Auth.Enabled)
	}
	if cfg.LaunchServer.Auth.APIKey != "" {
		t.Fatalf("LaunchServer.Auth.APIKey 默认应为空: got=%q", cfg.LaunchServer.Auth.APIKey)
	}
	if cfg.LaunchServer.Auth.Header != DefaultLaunchServerAPIKeyHeader {
		t.Fatalf("LaunchServer.Auth.Header 未补齐: got=%q", cfg.LaunchServer.Auth.Header)
	}
}

func TestDefaultFingerprintArgsForOS(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"windows": "--fingerprint-platform=windows",
		"linux":   "--fingerprint-platform=linux",
		"darwin":  "--fingerprint-platform=mac",
		"freebsd": "--fingerprint-platform=windows",
	}

	for goos, want := range tests {
		got := defaultFingerprintArgsForOS(goos)
		if len(got) != 3 {
			t.Fatalf("%s: unexpected args length: got=%v", goos, got)
		}
		if got[1] != want {
			t.Fatalf("%s: platform arg mismatch: got=%q want=%q", goos, got[1], want)
		}
		if got[2] != "--webrtc-ip-handling-policy=disable_non_proxied_udp" {
			t.Fatalf("%s: 缺少默认 WebRTC 防泄露策略: got=%v", goos, got)
		}
	}
}

func TestDefaultConfigUsesCurrentOSFingerprintPlatform(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	want := defaultFingerprintArgsForOS(runtime.GOOS)
	if len(cfg.Browser.DefaultFingerprintArgs) != len(want) {
		t.Fatalf("默认指纹参数数量不符: got=%v want=%v", cfg.Browser.DefaultFingerprintArgs, want)
	}
	for i := range want {
		if cfg.Browser.DefaultFingerprintArgs[i] != want[i] {
			t.Fatalf("默认指纹参数不符: got=%v want=%v", cfg.Browser.DefaultFingerprintArgs, want)
		}
	}
}

func TestLoadPreservesExplicitConfig(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	customConfig := `
database:
  type: sqlite
  sqlite:
    path: custom/app.db
app:
  name: Custom App
  window:
    width: 1400
    height: 800
    min_width: 900
    min_height: 600
  max_profile_limit: 120
  used_cd_keys: []
runtime:
  max_memory_mb: 2048
  gc_percent: 80
logging:
  level: debug
  file_enabled: true
  file_path: custom.log
  format: json
  buffer_size: 8
  async_queue_size: 2000
  flush_interval_ms: 500
  rotation:
    enabled: true
    max_size_mb: 10
    max_age: 3
    max_backups: 2
    time_interval: hourly
  interceptor:
    enabled: false
    log_parameters: false
    log_results: false
    sensitive_fields: []
browser:
  user_data_root: custom_data
  default_fingerprint_args:
    - --fingerprint-brand=Edge
  default_launch_args:
    - --start-maximized
  default_proxy: direct://
  default_bookmarks: []
  cores: []
  proxies: []
  profiles: []
launch_server:
  port: 30000
  auth:
    enabled: true
    api_key: secret-key
    header: X-Custom-Ant-Key
`
	if err := os.WriteFile(configPath, []byte(customConfig), 0o644); err != nil {
		t.Fatalf("写入测试配置失败: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	if cfg.App.Name != "Custom App" || cfg.App.MaxProfileLimit != 120 {
		t.Fatalf("App 显式配置被覆盖: got=%+v", cfg.App)
	}
	if cfg.Database.SQLite.Path != "custom/app.db" {
		t.Fatalf("Database.SQLite.Path 显式配置被覆盖: got=%q", cfg.Database.SQLite.Path)
	}
	if cfg.Runtime.MaxMemoryMB != 2048 || cfg.Runtime.GCPercent != 80 {
		t.Fatalf("Runtime 显式配置被覆盖: got=%+v", cfg.Runtime)
	}
	if cfg.Logging.Level != "debug" || cfg.Logging.Format != "json" || !cfg.Logging.FileEnabled {
		t.Fatalf("Logging 显式配置被覆盖: got=%+v", cfg.Logging)
	}
	if cfg.Logging.Interceptor.Enabled {
		t.Fatalf("Interceptor.Enabled 显式 false 被覆盖")
	}
	if len(cfg.Browser.DefaultFingerprintArgs) != 1 || cfg.Browser.DefaultFingerprintArgs[0] != "--fingerprint-brand=Edge" {
		t.Fatalf("Browser.DefaultFingerprintArgs 显式配置被覆盖: got=%v", cfg.Browser.DefaultFingerprintArgs)
	}
	if cfg.Browser.UserDataRoot != "custom_data" || cfg.Browser.DefaultProxy != "direct://" {
		t.Fatalf("Browser 显式配置被覆盖: got=%+v", cfg.Browser)
	}
	if cfg.LaunchServer.Port != 30000 {
		t.Fatalf("LaunchServer.Port 显式配置被覆盖: got=%d", cfg.LaunchServer.Port)
	}
	if !cfg.LaunchServer.Auth.Enabled {
		t.Fatalf("LaunchServer.Auth.Enabled 显式配置被覆盖")
	}
	if cfg.LaunchServer.Auth.APIKey != "secret-key" {
		t.Fatalf("LaunchServer.Auth.APIKey 显式配置被覆盖: got=%q", cfg.LaunchServer.Auth.APIKey)
	}
	if cfg.LaunchServer.Auth.Header != "X-Custom-Ant-Key" {
		t.Fatalf("LaunchServer.Auth.Header 显式配置被覆盖: got=%q", cfg.LaunchServer.Auth.Header)
	}
}

func TestLoadMigratesLegacyRootLogPath(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	legacyConfig := `
logging:
  file_path: logs/app.log
`
	if err := os.WriteFile(configPath, []byte(legacyConfig), 0o644); err != nil {
		t.Fatalf("写入测试配置失败: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	if cfg.Logging.FilePath != "data/logs/app.log" {
		t.Fatalf("legacy 根目录日志路径未迁移: got=%q", cfg.Logging.FilePath)
	}
}
