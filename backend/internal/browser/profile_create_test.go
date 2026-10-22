package browser

import (
	"ant-chrome/backend/internal/config"
	"testing"
)

func TestCreateAppliesDefaultFingerprintArgsWhenInputIsEmpty(t *testing.T) {
	manager := NewManager(&config.Config{}, t.TempDir())
	manager.Config.Browser.DefaultFingerprintArgs = []string{
		"--fingerprint-brand=Chrome",
		"--fingerprint-platform=windows",
		"--disable-non-proxied-udp",
		"--fingerprinting-canvas-image-data-noise",
		"--fingerprinting-client-rects-noise",
	}

	profile, err := manager.Create(ProfileInput{ProfileName: "test"})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	assertStringSliceContains(t, profile.FingerprintArgs, "--disable-non-proxied-udp")
	assertStringSliceContains(t, profile.FingerprintArgs, "--fingerprinting-canvas-image-data-noise")
	assertStringSliceContains(t, profile.FingerprintArgs, "--fingerprinting-client-rects-noise")
}

func TestCreateKeepsExplicitFingerprintArgs(t *testing.T) {
	manager := NewManager(&config.Config{}, t.TempDir())
	manager.Config.Browser.DefaultFingerprintArgs = []string{"--fingerprint-brand=Chrome"}

	profile, err := manager.Create(ProfileInput{
		ProfileName:     "test",
		FingerprintArgs: []string{"--fingerprint=123"},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if got, want := len(profile.FingerprintArgs), 1; got != want {
		t.Fatalf("fingerprint args length = %d, want %d: %#v", got, want, profile.FingerprintArgs)
	}
	assertStringSliceContains(t, profile.FingerprintArgs, "--fingerprint=123")
}

func TestCreateUpgradesLegacyMinimalFingerprintArgs(t *testing.T) {
	manager := NewManager(&config.Config{}, t.TempDir())

	profile, err := manager.Create(ProfileInput{
		ProfileName: `legacy`,
		FingerprintArgs: []string{
			`--fingerprint-brand=Chrome`,
			`--fingerprint-platform=windows`,
		},
	})
	if err != nil {
		t.Fatalf(`Create returned error: %v`, err)
	}

	assertStringSliceContains(t, profile.FingerprintArgs, `--disable-non-proxied-udp`)
	assertStringSliceContains(t, profile.FingerprintArgs, `--fingerprinting-canvas-image-data-noise`)
	assertStringSliceContains(t, profile.FingerprintArgs, `--fingerprinting-client-rects-noise`)
}

func TestCreateNormalizesRestoreLastSessionMode(t *testing.T) {
	manager := NewManager(&config.Config{}, t.TempDir())
	profile, err := manager.Create(ProfileInput{ProfileName: "test", RestoreLastSession: "true"})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if profile.RestoreLastSession != RestoreLastSessionEnabled {
		t.Fatalf("restoreLastSession = %q, want %q", profile.RestoreLastSession, RestoreLastSessionEnabled)
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
