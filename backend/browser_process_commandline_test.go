package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"testing"
)

func TestParseBrowserProcessCommandLineArgs(t *testing.T) {
	args := parseBrowserProcessCommandLineArgs(`"C:\Program Files\Chrome\chrome.exe" --user-data-dir="D:\profiles\one" --lang=zh-CN --timezone=Etc/GMT-8 --fingerprint-hardware-concurrency=26 --window-size=1689,1243`)
	assertArgsContain(t, args, `--user-data-dir=D:\profiles\one`)
	assertArgsContain(t, args, "--lang=zh-CN")
	assertArgsContain(t, args, "--timezone=Etc/GMT-8")
	assertArgsContain(t, args, "--fingerprint-hardware-concurrency=26")
	assertArgsContain(t, args, "--window-size=1689,1243")
}

func TestFingerprintCheckExpectedArgsRecoversRunningCommandLine(t *testing.T) {
	originalFinder := findBrowserUserDataProcesses
	defer func() { findBrowserUserDataProcesses = originalFinder }()

	app := NewApp(t.TempDir())
	app.config = &config.Config{}
	app.browserMgr = browser.NewManager(app.config, app.appRoot)
	profile := &browser.Profile{
		ProfileId:       "profile-running",
		FingerprintArgs: []string{"--fingerprint-brand=Chrome", "--fingerprint-platform=windows"},
		LaunchArgs:      []string{"--disable-sync", "--no-first-run"},
		Running:         true,
		Pid:             1234,
		DebugPort:       9222,
	}
	app.browserMgr.Profiles[profile.ProfileId] = profile
	findBrowserUserDataProcesses = func(userDataDir string) ([]browserUserDataProcess, error) {
		return []browserUserDataProcess{{
			PID:         1234,
			DebugPort:   9222,
			CommandLine: `"C:\Program Files\Chrome\chrome.exe" --user-data-dir="` + app.browserMgr.ResolveUserDataDir(profile) + `" --remote-debugging-port=9222 --fingerprint=676448312360042767 --fingerprint-brand=Chrome --fingerprint-platform=windows --lang=zh-CN --timezone=Etc/GMT-8 --fingerprint-hardware-concurrency=26 --window-size=1689,1243`,
		}}, nil
	}

	expected := buildBrowserFingerprintExpected(app.fingerprintCheckExpectedArgsFromProfile(profile))
	if expected.Seed != "676448312360042767" {
		t.Fatalf("seed = %q", expected.Seed)
	}
	if expected.Language != "zh-CN" {
		t.Fatalf("language = %q", expected.Language)
	}
	if expected.AcceptLanguage != "zh-CN,zh" {
		t.Fatalf("acceptLanguage = %q", expected.AcceptLanguage)
	}
	if expected.Timezone != "Etc/GMT-8" {
		t.Fatalf("timezone = %q", expected.Timezone)
	}
	if expected.HardwareConcurrency != "26" {
		t.Fatalf("hardwareConcurrency = %q", expected.HardwareConcurrency)
	}
	if expected.WindowSize != "1689,1243" {
		t.Fatalf("windowSize = %q", expected.WindowSize)
	}
}
