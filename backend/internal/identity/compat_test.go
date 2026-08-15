package identity

import "testing"

func equalArgs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// 应能从既有 fingerprint_args 反解出结构化身份(老环境兼容)。
func TestFromLaunchArgsParsesCoreFlags(t *testing.T) {
	args := []string{
		"--fingerprint=42",
		"--fingerprint-platform=macos",
		"--fingerprint-brand=Chrome",
		"--fingerprint-brand-version=142.0.0.0",
		"--fingerprint-hardware-concurrency=12",
		"--window-size=1440,900",
		"--lang=de-DE",
		"--accept-lang=de-DE,de,en",
		"--timezone=Europe/Berlin",
		"--fingerprinting-canvas-image-data-noise",
		"--fingerprinting-client-rects-noise",
		"--disable-non-proxied-udp",
	}
	id := FromLaunchArgs(args)
	if id.Seed != 42 || id.Platform != "macos" || id.BrowserBrand != "Chrome" || id.BrandVersion != "142.0.0.0" {
		t.Fatalf("core fields not parsed: %+v", id)
	}
	if id.HardwareConcurrency != 12 || id.WindowSize != "1440,900" {
		t.Fatalf("hardware/window not parsed: %+v", id)
	}
	if id.Locale != "de-DE" || len(id.Languages) != 3 || id.Timezone != "Europe/Berlin" {
		t.Fatalf("locale/lang/tz not parsed: %+v", id)
	}
	if !id.CanvasNoise || !id.ClientRectsNoise || id.WebRTCPolicy != "disable_non_proxied_udp" {
		t.Fatalf("noise/webrtc flags not parsed: %+v", id)
	}
}

// flag 可表示的字段应能往返(LaunchArgs -> FromLaunchArgs -> LaunchArgs 不变)。
func TestFromLaunchArgsRoundTrips(t *testing.T) {
	orig := sampleIdentity()
	got := FromLaunchArgs(orig.LaunchArgs())
	if !equalArgs(orig.LaunchArgs(), got.LaunchArgs()) {
		t.Fatalf("round-trip mismatch:\n orig=%v\n got =%v", orig.LaunchArgs(), got.LaunchArgs())
	}
}

// 未知 flag 应被忽略,不影响已识别项。
func TestFromLaunchArgsIgnoresUnknownFlags(t *testing.T) {
	id := FromLaunchArgs([]string{"--some-unknown=1", "--fingerprint=7", "--no-first-run"})
	if id.Seed != 7 {
		t.Fatalf("expected seed 7, got %d", id.Seed)
	}
}
