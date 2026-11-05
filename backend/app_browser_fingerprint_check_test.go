package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"strings"
	"testing"
)

func TestFingerprintCheckRuntimeProfileSnapshotSyncsDetectedRunningProfile(t *testing.T) {
	app := NewApp(t.TempDir())
	app.config = &config.Config{}
	app.browserMgr = browser.NewManager(app.config, app.appRoot)
	app.browserMgr.Profiles["profile-detected"] = &browser.Profile{
		ProfileId: "profile-detected",
		Running:   false,
	}

	var detectedUserDataDir string
	snapshot, err := app.fingerprintCheckRuntimeProfileSnapshot("profile-detected", func(userDataDir string) (browserRuntimeDetection, bool) {
		detectedUserDataDir = userDataDir
		return browserRuntimeDetection{PID: 4321, DebugPort: 9333, DebugReady: true}, true
	}, nil)
	if err != nil {
		t.Fatalf("fingerprintCheckRuntimeProfileSnapshot error = %v", err)
	}
	if strings.TrimSpace(detectedUserDataDir) == "" {
		t.Fatalf("detector did not receive user data dir")
	}
	if snapshot == nil || !snapshot.Running || !snapshot.DebugReady || snapshot.Pid != 4321 || snapshot.DebugPort != 9333 {
		t.Fatalf("snapshot runtime = %+v, want detected running state", snapshot)
	}

	stored := app.browserMgr.Profiles["profile-detected"]
	if stored == nil || !stored.Running || !stored.DebugReady || stored.Pid != 4321 || stored.DebugPort != 9333 {
		t.Fatalf("stored runtime = %+v, want detected running state", stored)
	}
}

func TestFingerprintCheckRuntimeProfileSnapshotMarksDebugReadyWhenPortProbes(t *testing.T) {
	app := NewApp(t.TempDir())
	app.config = &config.Config{}
	app.browserMgr = browser.NewManager(app.config, app.appRoot)
	app.browserMgr.Profiles["profile-pending"] = &browser.Profile{
		ProfileId:  "profile-pending",
		Running:    true,
		DebugPort:  9444,
		DebugReady: false,
	}

	snapshot, err := app.fingerprintCheckRuntimeProfileSnapshot("profile-pending", nil, func(debugPort int) bool {
		return debugPort == 9444
	})
	if err != nil {
		t.Fatalf("fingerprintCheckRuntimeProfileSnapshot error = %v", err)
	}
	if snapshot == nil || !snapshot.Running || !snapshot.DebugReady || snapshot.DebugPort != 9444 {
		t.Fatalf("snapshot runtime = %+v, want debug ready state", snapshot)
	}

	stored := app.browserMgr.Profiles["profile-pending"]
	if stored == nil || !stored.DebugReady || stored.DebugPort != 9444 {
		t.Fatalf("stored runtime = %+v, want debug ready state", stored)
	}
}

func TestBuildBrowserFingerprintExpected(t *testing.T) {
	expected := buildBrowserFingerprintExpected([]string{
		"--fingerprint=12345",
		"--fingerprint-brand=Chrome",
		"--fingerprint-brand-version=144.0.7559.132",
		"--fingerprint-platform=windows",
		"--fingerprint-platform-version=10.0.0",
		"--lang=ja-JP",
		"--timezone=Asia/Tokyo",
		"--fingerprint-hardware-concurrency=3",
		"--fingerprint-device-memory=8",
		"--fingerprint-color-depth=24",
		"--fingerprint-touch-points=5",
		"--window-size=1111,777",
		"--disable-non-proxied-udp",
		"--disable-spoofing=font,gpu",
		"--fingerprint-do-not-track=1",
		"--fingerprint-media-devices=2",
		"--fingerprinting-canvas-image-data-noise",
		"--fingerprint-audio-noise=1",
		"--fingerprinting-client-rects-noise",
		"--fingerprint-fonts=Arial,Calibri",
		"--fingerprint-webgl-vendor=Intel Inc.",
		"--fingerprint-webgl-renderer=Intel Iris OpenGL Engine",
	})

	if expected.Language != "ja-JP" {
		t.Fatalf("language = %q", expected.Language)
	}
	if expected.AcceptLanguage != "ja-JP,ja" {
		t.Fatalf("acceptLanguage = %q", expected.AcceptLanguage)
	}
	if expected.Timezone != "Asia/Tokyo" {
		t.Fatalf("timezone = %q", expected.Timezone)
	}
	if expected.HardwareConcurrency != "3" {
		t.Fatalf("hardwareConcurrency = %q", expected.HardwareConcurrency)
	}
	if expected.WindowSize != "1111,777" {
		t.Fatalf("windowSize = %q", expected.WindowSize)
	}
	if expected.DeviceMemory != "" || expected.ColorDepth != "" || expected.TouchPoints != "" {
		t.Fatalf("hardware details = %q / %q / %q", expected.DeviceMemory, expected.ColorDepth, expected.TouchPoints)
	}
	if expected.Brand != "Chrome" || expected.Platform != "windows" {
		t.Fatalf("identity = %q / %q", expected.Brand, expected.Platform)
	}
	if expected.BrandVersion != "144.0.7559.132" || expected.PlatformVersion != "10.0.0" {
		t.Fatalf("identity versions = %q / %q", expected.BrandVersion, expected.PlatformVersion)
	}
	if expected.Seed != "12345" {
		t.Fatalf("seed = %q", expected.Seed)
	}
	if expected.WebRTCPolicy != "disable_non_proxied_udp" {
		t.Fatalf("webrtcPolicy = %q", expected.WebRTCPolicy)
	}
	if expected.DisableSpoofing != "font,gpu" {
		t.Fatalf("disableSpoofing = %q", expected.DisableSpoofing)
	}
	if expected.DoNotTrack != "" || expected.MediaDevices != "" {
		t.Fatalf("privacy/media = %q / %q", expected.DoNotTrack, expected.MediaDevices)
	}
	if expected.CanvasNoise != "1" || expected.AudioNoise != "" || expected.ClientRectsNoise != "1" {
		t.Fatalf("noise = %q / %q / %q", expected.CanvasNoise, expected.AudioNoise, expected.ClientRectsNoise)
	}
	if expected.FontList != "" {
		t.Fatalf("fontList = %q", expected.FontList)
	}
	if expected.WebGLVendor != "" || expected.WebGLRenderer != "" {
		t.Fatalf("webgl = %q / %q", expected.WebGLVendor, expected.WebGLRenderer)
	}
}

func TestBuildBrowserFingerprintExpectedNoiseSwitchDisabledValues(t *testing.T) {
	expected := buildBrowserFingerprintExpected([]string{
		"--fingerprinting-canvas-image-data-noise=0",
		"--fingerprint-client-rects-noise=false",
	})

	if expected.CanvasNoise != "" {
		t.Fatalf("canvasNoise = %q, want empty for disabled flag", expected.CanvasNoise)
	}
	if expected.ClientRectsNoise != "" {
		t.Fatalf("clientRectsNoise = %q, want empty for disabled legacy flag", expected.ClientRectsNoise)
	}
}

func TestBuildBrowserFingerprintExpectedNoiseSwitchEnabledValues(t *testing.T) {
	expected := buildBrowserFingerprintExpected([]string{
		"--fingerprinting-canvas-image-data-noise=yes",
		"--fingerprint-client-rects-noise=on",
	})

	if expected.CanvasNoise != "1" {
		t.Fatalf("canvasNoise = %q, want 1", expected.CanvasNoise)
	}
	if expected.ClientRectsNoise != "1" {
		t.Fatalf("clientRectsNoise = %q, want 1", expected.ClientRectsNoise)
	}
}

func TestBuildBrowserFingerprintExpectedNoiseSwitchUsesLatestEquivalentArg(t *testing.T) {
	expected := buildBrowserFingerprintExpected([]string{
		"--fingerprinting-canvas-image-data-noise",
		"--fingerprint-canvas-noise=0",
		"--fingerprint-client-rects-noise=0",
		"--fingerprinting-client-rects-noise",
	})

	if expected.CanvasNoise != "" {
		t.Fatalf("canvasNoise = %q, want latest disabled value", expected.CanvasNoise)
	}
	if expected.ClientRectsNoise != "1" {
		t.Fatalf("clientRectsNoise = %q, want latest enabled flag", expected.ClientRectsNoise)
	}
}
