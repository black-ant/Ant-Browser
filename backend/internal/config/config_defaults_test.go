package config

import "testing"

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
	if config.ProxyCheck.FailureRetryMinutes != DefaultProxyFailureRetryMinutes {
		t.Fatalf("default failure retry = %d, want %d", config.ProxyCheck.FailureRetryMinutes, DefaultProxyFailureRetryMinutes)
	}
	if config.ProxyCheck.SpeedConcurrency != DefaultProxySpeedConcurrency {
		t.Fatalf("default speed concurrency = %d, want %d", config.ProxyCheck.SpeedConcurrency, DefaultProxySpeedConcurrency)
	}

	config.ProxyCheck.FailureRetryMinutes = 24*60 + 1
	config.ProxyCheck.SpeedConcurrency = MaxProxySpeedConcurrency + 1
	normalizeConfig(config)
	if config.ProxyCheck.FailureRetryMinutes != 24*60 || config.ProxyCheck.SpeedConcurrency != MaxProxySpeedConcurrency {
		t.Fatalf("bounded speed settings = retry %d, concurrency %d", config.ProxyCheck.FailureRetryMinutes, config.ProxyCheck.SpeedConcurrency)
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
