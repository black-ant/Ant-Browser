package browser

import "testing"

func TestUpgradeLegacyMinimalFingerprintArgsAddsEffectiveRuntimeArgs(t *testing.T) {
	args := upgradeLegacyMinimalFingerprintArgs([]string{"--fingerprint-brand=Chrome", "--fingerprint-platform=windows"})

	assertStringSliceContains(t, args, "--disable-non-proxied-udp")
	assertStringSliceContains(t, args, "--fingerprinting-canvas-image-data-noise")
	assertStringSliceContains(t, args, "--fingerprinting-client-rects-noise")
}

func TestUpgradeLegacyMinimalFingerprintArgsKeepsCustomArgs(t *testing.T) {
	args := upgradeLegacyMinimalFingerprintArgs([]string{"--fingerprint=123", "--fingerprint-brand=Chrome"})

	if got, want := len(args), 2; got != want {
		t.Fatalf("fingerprint args length = %d, want %d: %#v", got, want, args)
	}
	assertStringSliceContains(t, args, "--fingerprint=123")
	assertStringSliceContains(t, args, "--fingerprint-brand=Chrome")
}
