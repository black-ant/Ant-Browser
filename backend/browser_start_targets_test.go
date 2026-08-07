package backend

import (
	"slices"
	"testing"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
)

func TestBuildBrowserLaunchTargetsRestoresSessionThenDefersStartPages(t *testing.T) {
	launchTargets, deferredTargets, deferredNewTabs := buildBrowserLaunchTargets(nil, []string{"https://example.com"}, false, true, true)
	if len(launchTargets) != 0 {
		t.Fatalf("launchTargets = %#v, want empty so Chromium can restore session first", launchTargets)
	}
	if len(deferredTargets) != 1 || deferredTargets[0] != "https://example.com" {
		t.Fatalf("deferredTargets = %#v, want startup page deferred", deferredTargets)
	}
	if !deferredNewTabs {
		t.Fatal("deferredNewTabs = false, want true so startup pages do not replace restored tabs")
	}
}

func TestBuildBrowserLaunchArgsAddsRestoreLastSessionWhenEnabled(t *testing.T) {
	args := buildBrowserLaunchArgs("profile-dir", 9222, "direct://", nil, nil, nil, nil, true)
	if !slices.Contains(args, "--restore-last-session") {
		t.Fatalf("args = %#v, want --restore-last-session", args)
	}
}

func TestBuildBrowserLaunchArgsOmitsRestoreLastSessionWhenDisabled(t *testing.T) {
	sanitizedProfileArgs, _ := sanitizeManagedLaunchArgs([]string{"--restore-last-session"})
	args := buildBrowserLaunchArgs("profile-dir", 9222, "direct://", nil, sanitizedProfileArgs, nil, nil, false)
	if slices.Contains(args, "--restore-last-session") {
		t.Fatalf("args = %#v, want --restore-last-session omitted when disabled", args)
	}
}

func TestBuildBrowserLaunchTargetsKeepsStartupPagesWhenRestoreDisabled(t *testing.T) {
	launchTargets, deferredTargets, deferredNewTabs := buildBrowserLaunchTargets(nil, []string{"https://example.com"}, false, false, false)
	if len(launchTargets) != 1 || launchTargets[0] != "https://example.com" {
		t.Fatalf("launchTargets = %#v, want startup page", launchTargets)
	}
	if len(deferredTargets) != 0 {
		t.Fatalf("deferredTargets = %#v, want empty", deferredTargets)
	}
	if deferredNewTabs {
		t.Fatal("deferredNewTabs = true, want false when startup pages are launch targets")
	}
}

func TestBuildBrowserLaunchTargetsLightStartDefersWithoutNewTabs(t *testing.T) {
	launchTargets, deferredTargets, deferredNewTabs := buildBrowserLaunchTargets(nil, []string{"https://example.com"}, false, false, true)
	if len(launchTargets) != 1 || launchTargets[0] != "about:blank" {
		t.Fatalf("launchTargets = %#v, want about:blank", launchTargets)
	}
	if len(deferredTargets) != 1 || deferredTargets[0] != "https://example.com" {
		t.Fatalf("deferredTargets = %#v, want startup page deferred", deferredTargets)
	}
	if deferredNewTabs {
		t.Fatal("deferredNewTabs = true, want false so light start can replace about:blank")
	}
}

func TestProfileRestoreLastSessionOverridesBrowserDefault(t *testing.T) {
	cfg := &config.Config{}
	cfg.Browser.RestoreLastSession = false
	if !profileRestoreLastSession(&browser.Profile{RestoreLastSession: browser.RestoreLastSessionEnabled}, cfg) {
		t.Fatal("enabled profile override should restore session")
	}

	cfg.Browser.RestoreLastSession = true
	if profileRestoreLastSession(&browser.Profile{RestoreLastSession: browser.RestoreLastSessionDisabled}, cfg) {
		t.Fatal("disabled profile override should not restore session")
	}
	if !profileRestoreLastSession(&browser.Profile{RestoreLastSession: browser.RestoreLastSessionFollow}, cfg) {
		t.Fatal("follow profile should use browser default")
	}
}
