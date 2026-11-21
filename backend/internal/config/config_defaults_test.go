package config

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDefaultFingerprintArgsIncludeEffectiveRuntimeArgs(t *testing.T) {
	args := defaultFingerprintArgsForOS("windows")
	assertStringSliceContains(t, args, "--fingerprint-brand=Chrome")
	assertStringSliceContains(t, args, "--fingerprint-platform=windows")
	assertStringSliceContains(t, args, "--disable-non-proxied-udp")
	assertStringSliceContains(t, args, "--fingerprinting-canvas-image-data-noise")
	assertStringSliceContains(t, args, "--fingerprinting-client-rects-noise")
}

func TestNormalizeConfigUpgradesLegacyMinimalDefaultFingerprintArgs(t *testing.T) {
	config := &Config{}
	config.Browser.DefaultFingerprintArgs = []string{"--fingerprint-brand=Chrome", "--fingerprint-platform=windows"}

	normalizeConfig(config)

	assertStringSliceContains(t, config.Browser.DefaultFingerprintArgs, "--fingerprint-brand=Chrome")
	assertStringSliceContains(t, config.Browser.DefaultFingerprintArgs, "--fingerprint-platform=windows")
	assertStringSliceContains(t, config.Browser.DefaultFingerprintArgs, "--disable-non-proxied-udp")
	assertStringSliceContains(t, config.Browser.DefaultFingerprintArgs, "--fingerprinting-canvas-image-data-noise")
	assertStringSliceContains(t, config.Browser.DefaultFingerprintArgs, "--fingerprinting-client-rects-noise")
}

func TestNormalizeConfigDoesNotOverrideCustomDefaultFingerprintArgs(t *testing.T) {
	config := &Config{}
	config.Browser.DefaultFingerprintArgs = []string{"--fingerprint=123", "--fingerprint-brand=Chrome"}

	normalizeConfig(config)

	if got, want := len(config.Browser.DefaultFingerprintArgs), 2; got != want {
		t.Fatalf("default fingerprint args length = %d, want %d: %#v", got, want, config.Browser.DefaultFingerprintArgs)
	}
	assertStringSliceContains(t, config.Browser.DefaultFingerprintArgs, "--fingerprint=123")
	assertStringSliceContains(t, config.Browser.DefaultFingerprintArgs, "--fingerprint-brand=Chrome")
}

func TestNormalizeConfigSetsAndBoundsProxySpeedSettings(t *testing.T) {
	config := &Config{}
	normalizeConfig(config)
	if config.ProxyCheck.SpeedIntervalMinutes != DefaultProxySpeedIntervalMinutes {
		t.Fatalf("default speed interval = %d, want %d", config.ProxyCheck.SpeedIntervalMinutes, DefaultProxySpeedIntervalMinutes)
	}
	if config.ProxyCheck.FailureRetryMinutes != DefaultProxyFailureRetryMinutes {
		t.Fatalf("default failure retry = %d, want %d", config.ProxyCheck.FailureRetryMinutes, DefaultProxyFailureRetryMinutes)
	}
	if config.ProxyCheck.SpeedConcurrency != DefaultProxySpeedConcurrency {
		t.Fatalf("default speed concurrency = %d, want %d", config.ProxyCheck.SpeedConcurrency, DefaultProxySpeedConcurrency)
	}

	config.ProxyCheck.FailureRetryMinutes = 24*60 + 1
	config.ProxyCheck.SpeedIntervalMinutes = MaxProxySpeedIntervalMinutes + 1
	config.ProxyCheck.SpeedConcurrency = MaxProxySpeedConcurrency + 1
	normalizeConfig(config)
	if config.ProxyCheck.SpeedIntervalMinutes != MaxProxySpeedIntervalMinutes || config.ProxyCheck.FailureRetryMinutes != 24*60 || config.ProxyCheck.SpeedConcurrency != MaxProxySpeedConcurrency {
		t.Fatalf("bounded speed settings = interval %d, retry %d, concurrency %d", config.ProxyCheck.SpeedIntervalMinutes, config.ProxyCheck.FailureRetryMinutes, config.ProxyCheck.SpeedConcurrency)
	}
}

// 虚拟分组核验间隔必须有硬上限，"至少每 3 小时核验一次"不能靠配置自觉。
func TestNormalizeConfigBoundsVirtualGroupProbeInterval(t *testing.T) {
	config := &Config{}
	normalizeConfig(config)
	if config.ProxyCheck.VirtualGroupProbeIntervalMinutes != DefaultVirtualGroupProbeIntervalMinutes {
		t.Fatalf("default virtual group probe interval = %d, want %d",
			config.ProxyCheck.VirtualGroupProbeIntervalMinutes, DefaultVirtualGroupProbeIntervalMinutes)
	}
	if MaxVirtualGroupProbeIntervalMinutes > 3*60 {
		t.Fatalf("虚拟分组核验上限 = %d 分钟, 必须 <= 180", MaxVirtualGroupProbeIntervalMinutes)
	}

	config.ProxyCheck.VirtualGroupProbeIntervalMinutes = MaxProxySpeedIntervalMinutes
	normalizeConfig(config)
	if config.ProxyCheck.VirtualGroupProbeIntervalMinutes != MaxVirtualGroupProbeIntervalMinutes {
		t.Fatalf("bounded virtual group probe interval = %d, want %d",
			config.ProxyCheck.VirtualGroupProbeIntervalMinutes, MaxVirtualGroupProbeIntervalMinutes)
	}
}

func TestBridgeCustomDNSValue(t *testing.T) {
	if got := BridgeCustomDNSValue(nil); got != "" {
		t.Fatalf("nil 配置应返回空, got %q", got)
	}
	if got := BridgeCustomDNSValue(&Config{}); got != "" {
		t.Fatalf("未配置自定义 DNS 应返回空, got %q", got)
	}

	custom := "  1.1.1.1\nhttps://doh.example.com/dns-query  "
	config := &Config{Browser: BrowserConfig{BridgeCustomDNS: custom}}
	if got := BridgeCustomDNSValue(config); got != strings.TrimSpace(custom) {
		t.Fatalf("自定义 DNS 应 trim 后返回, got %q", got)
	}

	normalizeConfig(config)
	if BridgeCustomDNSValue(config) == "" {
		t.Fatal("normalizeConfig 不应清除用户自定义 DNS")
	}
}

func TestBridgeCustomDNSYAMLRoundTrip(t *testing.T) {
	custom := "1.1.1.1\nhttps://doh.example.com/dns-query"
	config := &Config{Browser: BrowserConfig{BridgeCustomDNS: custom}}
	data, err := yaml.Marshal(config)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if !strings.Contains(string(data), "bridge_custom_dns:") {
		t.Fatalf("自定义 DNS 未写入配置: %s", data)
	}

	var parsed Config
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	if BridgeCustomDNSValue(&parsed) != custom {
		t.Fatalf("往返后自定义 DNS 不一致: %q", BridgeCustomDNSValue(&parsed))
	}

	emptyData, err := yaml.Marshal(&Config{})
	if err != nil {
		t.Fatalf("marshal empty config: %v", err)
	}
	if strings.Contains(string(emptyData), "bridge_custom_dns") {
		t.Fatalf("空自定义 DNS 不应写入配置: %s", emptyData)
	}
}

func TestNormalizeConfigUpdateDefaults(t *testing.T) {
	config := &Config{}
	normalizeConfig(config)
	if config.Update.Repo != "black-ant/Ant-Browser" {
		t.Fatalf("update repo = %q", config.Update.Repo)
	}
	if config.Update.CheckIntervalHours != 24 {
		t.Fatalf("update interval = %d, want 24", config.Update.CheckIntervalHours)
	}
	if !config.Update.IsEnabled() || !config.Update.ShouldAutoCheck() || !config.Update.VerifySHA256() {
		t.Fatal("update defaults should be enabled with checksum verification")
	}

	off := false
	custom := &Config{Update: UpdateConfig{Enabled: &off, AutoCheck: &off, VerifyChecksum: &off}}
	normalizeConfig(custom)
	if custom.Update.IsEnabled() || custom.Update.ShouldAutoCheck() || custom.Update.VerifySHA256() {
		t.Fatal("explicitly disabled update settings must not be overridden")
	}
}

func assertStringSliceContains(t *testing.T, values []string, expected string) {
	t.Helper()
	for _, value := range values {
		if value == expected {
			return
		}
	}
	t.Fatalf("values %#v missing %q", values, expected)
}
