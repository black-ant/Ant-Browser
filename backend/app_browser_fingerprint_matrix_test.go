package backend

import "testing"

func TestBuildBrowserFingerprintLaunchPlanChrome144ConvertsRemovedGPUArgs(t *testing.T) {
	plan := buildBrowserFingerprintLaunchPlan("profile-a", []string{
		"--fingerprint=123",
		"--fingerprint-gpu-vendor=NVIDIA",
		"--fingerprint-gpu-renderer=RTX",
		"--disable-gpu-fingerprint",
		"--disable-spoofing=font",
	}, "144.0.7559.132")

	assertArgsDoNotContainKey(t, plan.launchArgs, "--fingerprint-gpu-vendor")
	assertArgsDoNotContainKey(t, plan.launchArgs, "--fingerprint-gpu-renderer")
	assertArgsDoNotContainKey(t, plan.launchArgs, "--disable-gpu-fingerprint")
	assertArgsContain(t, plan.launchArgs, "--disable-spoofing=font,gpu")
	assertRowsContainStatus(t, plan.rows, "GPU Vendor", "removed")
	assertRowsContainStatus(t, plan.rows, "GPU Renderer", "removed")
	assertRowsContainStatus(t, plan.rows, "GPU 伪装开关", "converted")
}

func TestBuildBrowserFingerprintLaunchPlanMergesMultipleDisableSpoofingArgs(t *testing.T) {
	plan := buildBrowserFingerprintLaunchPlan("profile-a", []string{
		"--fingerprint=123",
		"--disable-spoofing=canvas,audio",
		"--disable-spoofing=font,clientrects",
	}, "144.0.7559.132")

	assertArgsContain(t, plan.launchArgs, "--disable-spoofing=canvas,audio,font,clientrects")
}

func TestBuildBrowserFingerprintLaunchPlanDisablesCrossPlatformFontSpoofing(t *testing.T) {
	plan := buildBrowserFingerprintLaunchPlanForHost("profile-a", []string{
		"--fingerprint=123",
		"--fingerprint-platform=macos",
	}, "148.0.7778.215", "windows")

	assertArgsContain(t, plan.launchArgs, "--disable-spoofing=font")
	assertRowsContainStatus(t, plan.rows, "跨平台字体渲染", "injected")
}

func TestBuildBrowserFingerprintLaunchPlanMergesCrossPlatformFontSpoofing(t *testing.T) {
	plan := buildBrowserFingerprintLaunchPlanForHost("profile-a", []string{
		"--fingerprint=123",
		"--fingerprint-platform=macos",
		"--disable-spoofing=canvas",
	}, "148.0.7778.215", "windows")

	assertArgsContain(t, plan.launchArgs, "--disable-spoofing=canvas,font")
}

func TestBuildBrowserFingerprintLaunchPlanKeepsFontSpoofingOnNativePlatform(t *testing.T) {
	plan := buildBrowserFingerprintLaunchPlanForHost("profile-a", []string{
		"--fingerprint=123",
		"--fingerprint-platform=windows",
	}, "148.0.7778.215", "windows")

	assertArgsDoNotContainKey(t, plan.launchArgs, "--disable-spoofing")
	assertRowsDoNotContainCapability(t, plan.rows, "跨平台字体渲染")
}

func TestBuildBrowserFingerprintLaunchPlanCoversAllHostAndTargetPlatforms(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		target   string
		wantFont bool
	}{
		{name: "windows native", host: "windows", target: "windows", wantFont: false},
		{name: "windows to macos", host: "windows", target: "macos", wantFont: true},
		{name: "windows to linux", host: "windows", target: "linux", wantFont: true},
		{name: "macos native", host: "darwin", target: "mac", wantFont: false},
		{name: "macos to windows", host: "darwin", target: "windows", wantFont: true},
		{name: "macos to linux", host: "darwin", target: "linux", wantFont: true},
		{name: "linux native", host: "linux", target: "linux", wantFont: false},
		{name: "linux to windows", host: "linux", target: "windows", wantFont: true},
		{name: "linux to macos", host: "linux", target: "macos", wantFont: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan := buildBrowserFingerprintLaunchPlanForHost("profile-a", []string{
				"--fingerprint=123",
				"--fingerprint-platform=" + test.target,
			}, "148.0.7778.215", test.host)

			if test.wantFont {
				assertArgsContain(t, plan.launchArgs, "--disable-spoofing=font")
				assertRowsContainStatus(t, plan.rows, "跨平台字体渲染", "injected")
				return
			}
			assertArgsDoNotContainKey(t, plan.launchArgs, "--disable-spoofing")
			assertRowsDoNotContainCapability(t, plan.rows, "跨平台字体渲染")
		})
	}
}

func TestBuildBrowserFingerprintLaunchPlanDoesNotDuplicateExistingFontSpoofingOverride(t *testing.T) {
	plan := buildBrowserFingerprintLaunchPlanForHost("profile-a", []string{
		"--fingerprint=123",
		"--fingerprint-platform=linux",
		"--disable-spoofing=canvas,font",
	}, "148.0.7778.215", "windows")

	assertArgsContain(t, plan.launchArgs, "--disable-spoofing=canvas,font")
	if countArgsWithKey(plan.launchArgs, "--disable-spoofing") != 1 {
		t.Fatalf("launch args contain duplicate --disable-spoofing values: %#v", plan.launchArgs)
	}
}

func TestBuildBrowserFingerprintLaunchPlanChrome143KeepsGPUArgs(t *testing.T) {
	plan := buildBrowserFingerprintLaunchPlan("profile-a", []string{
		"--fingerprint=123",
		"--fingerprint-gpu-vendor=NVIDIA",
		"--fingerprint-gpu-renderer=RTX",
		"--disable-gpu-fingerprint",
	}, "143.0.7499.10")

	assertArgsContain(t, plan.launchArgs, "--fingerprint-gpu-vendor=NVIDIA")
	assertArgsContain(t, plan.launchArgs, "--fingerprint-gpu-renderer=RTX")
	assertArgsContain(t, plan.launchArgs, "--disable-gpu-fingerprint")
	assertRowsContainStatus(t, plan.rows, "GPU Vendor", "kept_legacy")
	assertRowsContainStatus(t, plan.rows, "GPU Renderer", "kept_legacy")
	assertRowsContainStatus(t, plan.rows, "GPU 伪装开关", "kept_legacy")
}

func TestBuildBrowserFingerprintLaunchPlanChrome144ConvertsVerifiedNoiseArgs(t *testing.T) {
	plan := buildBrowserFingerprintLaunchPlan("profile-a", []string{
		"--fingerprint=123",
		"--fingerprint-canvas-noise=1",
		"--fingerprint-client-rects-noise=true",
	}, "148.0.7778.215")

	assertArgsContain(t, plan.launchArgs, "--fingerprinting-canvas-image-data-noise")
	assertArgsContain(t, plan.launchArgs, "--fingerprinting-client-rects-noise")
	assertRowsContainStatus(t, plan.rows, "Canvas ImageData", "converted")
	assertRowsContainStatus(t, plan.rows, "ClientRects", "converted")
}

func TestBuildBrowserFingerprintLaunchPlanChrome144RemovesDisabledLegacyNoiseArgs(t *testing.T) {
	plan := buildBrowserFingerprintLaunchPlan("profile-a", []string{
		"--fingerprint=123",
		"--fingerprint-canvas-noise=0",
		"--fingerprint-client-rects-noise=false",
		"--fingerprinting-canvas-image-data-noise=0",
		"--fingerprinting-client-rects-noise=false",
	}, "148.0.7778.215")

	assertArgsDoNotContainKey(t, plan.launchArgs, "--fingerprint-canvas-noise")
	assertArgsDoNotContainKey(t, plan.launchArgs, "--fingerprint-client-rects-noise")
	assertArgsDoNotContainKey(t, plan.launchArgs, "--fingerprinting-canvas-image-data-noise")
	assertArgsDoNotContainKey(t, plan.launchArgs, "--fingerprinting-client-rects-noise")
	assertRowsContainStatus(t, plan.rows, "Canvas ImageData", "disabled")
	assertRowsContainStatus(t, plan.rows, "ClientRects", "disabled")
}

func TestBuildBrowserFingerprintLaunchPlanChrome144UsesLatestNoiseArg(t *testing.T) {
	plan := buildBrowserFingerprintLaunchPlan("profile-a", []string{
		"--fingerprint=123",
		"--fingerprinting-canvas-image-data-noise",
		"--fingerprint-canvas-noise=0",
		"--fingerprint-client-rects-noise=0",
		"--fingerprinting-client-rects-noise",
	}, "148.0.7778.215")

	assertArgsDoNotContainKey(t, plan.launchArgs, "--fingerprinting-canvas-image-data-noise")
	assertArgsContain(t, plan.launchArgs, "--fingerprinting-client-rects-noise")
	assertRowsContainStatus(t, plan.rows, "Canvas ImageData", "overridden")
	assertRowsContainStatus(t, plan.rows, "Canvas ImageData", "disabled")
	assertRowsContainStatus(t, plan.rows, "ClientRects", "overridden")
	assertRowsContainStatus(t, plan.rows, "ClientRects", "kept")
}

func TestBuildBrowserFingerprintLaunchPlanChrome144RemovesNoEffectFineGrainedArgs(t *testing.T) {
	plan := buildBrowserFingerprintLaunchPlan("profile-a", []string{
		"--fingerprint=123",
		"--fingerprint-device-memory=8",
		"--fingerprint-touch-points=5",
		"--fingerprint-do-not-track=1",
		"--fingerprint-media-devices=2",
		"--fingerprint-webgl-vendor=Intel",
		"--fingerprint-audio-noise=1",
		"--fingerprint-font-list=Arial",
		"--fingerprint-color-depth=30",
	}, "148.0.7778.215")

	assertArgsDoNotContainKey(t, plan.launchArgs, "--fingerprint-device-memory")
	assertArgsDoNotContainKey(t, plan.launchArgs, "--fingerprint-touch-points")
	assertArgsDoNotContainKey(t, plan.launchArgs, "--fingerprint-do-not-track")
	assertArgsDoNotContainKey(t, plan.launchArgs, "--fingerprint-media-devices")
	assertArgsDoNotContainKey(t, plan.launchArgs, "--fingerprint-webgl-vendor")
	assertArgsDoNotContainKey(t, plan.launchArgs, "--fingerprint-audio-noise")
	assertArgsDoNotContainKey(t, plan.launchArgs, "--fingerprint-font-list")
	assertArgsDoNotContainKey(t, plan.launchArgs, "--fingerprint-color-depth")
	assertRowsContainStatus(t, plan.rows, "设备内存", "not_effective")
	assertRowsContainStatus(t, plan.rows, "WebGL Vendor", "not_effective")
	assertRowsContainStatus(t, plan.rows, "Audio", "not_effective")
}

func TestBuildBrowserFingerprintLaunchPlanInjectsSeedAndAcceptLanguage(t *testing.T) {
	plan := buildBrowserFingerprintLaunchPlan("profile-a", []string{"--lang=zh-CN"}, "148.0.7778.215")

	assertArgsContainKey(t, plan.launchArgs, "--fingerprint")
	assertArgsContain(t, plan.launchArgs, "--lang=zh-CN")
	assertArgsContain(t, plan.launchArgs, "--accept-lang=zh-CN,zh")
	assertRowsContainStatus(t, plan.rows, "指纹种子", "injected")
	assertRowsContainStatus(t, plan.rows, "Accept-Language", "inferred")
}

func TestBuildBrowserFingerprintLaunchPlanUnknownVersionKeepsLegacyArgs(t *testing.T) {
	plan := buildBrowserFingerprintLaunchPlan("profile-a", []string{
		"--fingerprint=123",
		"--fingerprint-gpu-vendor=NVIDIA",
		"--fingerprint-canvas-noise=true",
	}, "")

	assertArgsContain(t, plan.launchArgs, "--fingerprint-gpu-vendor=NVIDIA")
	assertArgsContain(t, plan.launchArgs, "--fingerprint-canvas-noise=true")
	assertRowsContainStatus(t, plan.rows, "GPU Vendor", "kept_unconfirmed")
	assertRowsContainStatus(t, plan.rows, "Canvas ImageData", "kept_unconfirmed")
	if len(plan.warnings) == 0 {
		t.Fatalf("expected warning for unknown version")
	}
}

func assertArgsContain(t *testing.T, args []string, want string) {
	t.Helper()
	for _, arg := range args {
		if arg == want {
			return
		}
	}
	t.Fatalf("args %#v do not contain %q", args, want)
}

func assertArgsContainKey(t *testing.T, args []string, key string) {
	t.Helper()
	for _, arg := range args {
		if browserFingerprintArgKey(arg) == key {
			return
		}
	}
	t.Fatalf("args %#v do not contain key %q", args, key)
}

func assertArgsDoNotContainKey(t *testing.T, args []string, key string) {
	t.Helper()
	for _, arg := range args {
		if browserFingerprintArgKey(arg) == key {
			t.Fatalf("args %#v contain key %q", args, key)
		}
	}
}

func countArgsWithKey(args []string, key string) int {
	count := 0
	for _, arg := range args {
		if browserFingerprintArgKey(arg) == key {
			count++
		}
	}
	return count
}

func assertRowsContainStatus(t *testing.T, rows []BrowserFingerprintCapabilityRow, capability string, status string) {
	t.Helper()
	for _, row := range rows {
		if row.Capability == capability && row.Status == status {
			return
		}
	}
	t.Fatalf("rows %#v do not contain capability %q with status %q", rows, capability, status)
}

func assertRowsDoNotContainCapability(t *testing.T, rows []BrowserFingerprintCapabilityRow, capability string) {
	t.Helper()
	for _, row := range rows {
		if row.Capability == capability {
			t.Fatalf("rows %#v contain capability %q", rows, capability)
		}
	}
}
