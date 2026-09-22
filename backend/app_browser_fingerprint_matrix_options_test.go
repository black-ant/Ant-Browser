package backend

import (
	"testing"

	"ant-chrome/backend/internal/config"
)

func TestBuildBrowserFingerprintLaunchPlanKeepsFontSpoofingWhenPassthroughDisabled(t *testing.T) {
	options := browserFingerprintPlanOptions{crossPlatformFontPassthrough: false}
	plan := buildBrowserFingerprintLaunchPlanForHostWithOptions("profile-a", []string{
		"--fingerprint=123",
		"--fingerprint-platform=windows",
	}, "148.0.7778.215", "linux", options)

	assertArgsDoNotContainKey(t, plan.launchArgs, "--disable-spoofing")
	assertRowsContainStatus(t, plan.rows, "跨平台字体渲染", "kept")
}

func TestBuildBrowserFingerprintLaunchPlanPassthroughDisabledKeepsExplicitDisableSpoofing(t *testing.T) {
	options := browserFingerprintPlanOptions{crossPlatformFontPassthrough: false}
	plan := buildBrowserFingerprintLaunchPlanForHostWithOptions("profile-a", []string{
		"--fingerprint=123",
		"--fingerprint-platform=windows",
		"--disable-spoofing=canvas",
	}, "148.0.7778.215", "linux", options)

	assertArgsContain(t, plan.launchArgs, "--disable-spoofing=canvas")
}

func TestBuildBrowserFingerprintLaunchPlanDefaultOptionsInjectPassthrough(t *testing.T) {
	plan := buildBrowserFingerprintLaunchPlanForHostWithOptions("profile-a", []string{
		"--fingerprint=123",
		"--fingerprint-platform=windows",
	}, "148.0.7778.215", "linux", defaultBrowserFingerprintPlanOptions())

	assertArgsContain(t, plan.launchArgs, "--disable-spoofing=font")
	assertRowsContainStatus(t, plan.rows, "跨平台字体渲染", "injected")
}

func TestBrowserCrossPlatformFontPassthroughDefaultsToTrue(t *testing.T) {
	if !browserCrossPlatformFontPassthrough(nil) {
		t.Fatal("nil config should keep passthrough enabled")
	}
	if !browserCrossPlatformFontPassthrough(&config.Config{}) {
		t.Fatal("unset value should keep passthrough enabled")
	}
	disabled := false
	cfg := &config.Config{}
	cfg.Browser.CrossPlatformFontPassthrough = &disabled
	if browserCrossPlatformFontPassthrough(cfg) {
		t.Fatal("explicit false should disable passthrough")
	}
}
