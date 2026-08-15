package identity

import (
	"strings"
	"testing"
)

func hasArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

func hasArgPrefix(args []string, prefix string) bool {
	for _, a := range args {
		if strings.HasPrefix(a, prefix) {
			return true
		}
	}
	return false
}

// 完整身份应序列化出内核可识别的核心指纹 flag。
func TestLaunchArgsEmitsCoreFlags(t *testing.T) {
	args := sampleIdentity().LaunchArgs()
	want := []string{
		"--fingerprint=123456789",
		"--fingerprint-platform=windows",
		"--fingerprint-brand=Chrome",
		"--fingerprint-brand-version=142.0.0.0",
		"--fingerprint-hardware-concurrency=8",
		"--window-size=1280,720",
		"--lang=en-US",
		"--accept-lang=en-US,en",
		"--timezone=America/New_York",
		"--fingerprinting-canvas-image-data-noise",
		"--fingerprinting-client-rects-noise",
		"--disable-non-proxied-udp",
	}
	for _, w := range want {
		if !hasArg(args, w) {
			t.Errorf("missing expected flag %q in %v", w, args)
		}
	}
}

// 可选字段为空时不应产出对应 flag。
func TestLaunchArgsOmitsEmptyOptionalFields(t *testing.T) {
	id := sampleIdentity()
	id.Timezone = ""
	id.HardwareConcurrency = 0
	id.CanvasNoise = false
	id.BrandVersion = ""
	args := id.LaunchArgs()
	if hasArgPrefix(args, "--timezone") {
		t.Error("should not emit --timezone when empty")
	}
	if hasArgPrefix(args, "--fingerprint-hardware-concurrency") {
		t.Error("should not emit hardware-concurrency when 0")
	}
	if hasArg(args, "--fingerprinting-canvas-image-data-noise") {
		t.Error("should not emit canvas noise when disabled")
	}
	if hasArgPrefix(args, "--fingerprint-brand-version") {
		t.Error("should not emit brand-version when empty")
	}
}

// 地理定位必须经 CDP 注入(Chrome 144+ 已废弃相关启动 flag),不得作为 flag 产出。
func TestLaunchArgsDoesNotEmitGeolocation(t *testing.T) {
	args := sampleIdentity().LaunchArgs()
	for _, a := range args {
		if strings.Contains(a, "location") || strings.Contains(a, "geo") {
			t.Errorf("geolocation must be injected via CDP, not a launch flag; found %q", a)
		}
	}
}
