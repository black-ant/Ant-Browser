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

// 版本自洽:必须下发 --user-agent,且其 Chrome 主版本与 brand-version 一致,
// 使 UA 与 Client Hints 报同一版本(实现"版本多样性且自洽")。
func TestLaunchArgsEmitsConsistentUserAgent(t *testing.T) {
	id := sampleIdentity() // BrandVersion 142 / UAFull Chrome/142
	args := id.LaunchArgs()
	uaArg := ""
	for _, a := range args {
		if strings.HasPrefix(a, "--user-agent=") {
			uaArg = a
		}
	}
	if uaArg == "" {
		t.Fatalf("必须下发 --user-agent 以统一 UA 版本;args=%v", args)
	}
	// UA 里的 Chrome 主版本必须与 brand-version 主版本一致
	uaMajor := chromeMajorFromUA(uaArg)
	brandMajor := strings.SplitN(id.BrandVersion, ".", 2)[0]
	if uaMajor == "" || uaMajor != brandMajor {
		t.Errorf("UA 版本(%s)与 brand-version 版本(%s)不一致:%s", uaMajor, brandMajor, uaArg)
	}
}

// UAFull 为空时不产出 --user-agent(回退内核默认,避免产出空 UA)。
func TestLaunchArgsOmitsUserAgentWhenUAFullEmpty(t *testing.T) {
	id := sampleIdentity()
	id.UAFull = ""
	if hasArgPrefix(id.LaunchArgs(), "--user-agent") {
		t.Error("UAFull 为空时不应产出 --user-agent")
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
