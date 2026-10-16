package browser

import (
	"testing"

	"ant-chrome/backend/internal/config"
)

func TestNormalizeCopyAutomationTargetsMapsLegacyTargetsToSeed(t *testing.T) {
	targets, err := normalizeCopyAutomationTargets([]string{"render", "fonts", "devices"})
	if err != nil {
		t.Fatalf("normalizeCopyAutomationTargets returned error: %v", err)
	}
	if len(targets) != 1 || targets[0] != copyAutomationTargetSeed {
		t.Fatalf("targets = %#v, want seed alias", targets)
	}
}

func TestBuildAutoFingerprintArgsKeepsLegacyFineGrainedArgs(t *testing.T) {
	sourceArgs := []string{
		"--fingerprint=111",
		"--fingerprint-canvas-noise=true",
		"--fingerprint-webgl-vendor=Intel",
	}
	defaultArgs := []string{"--fingerprint=222"}

	got := buildAutoFingerprintArgs(sourceArgs, defaultArgs, []string{copyAutomationTargetSeed})
	want := []string{"--fingerprint=222", "--fingerprint-canvas-noise=true", "--fingerprint-webgl-vendor=Intel"}
	if len(got) != len(want) {
		t.Fatalf("len(got) = %d, want %d; got=%#v", len(got), len(want), got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("got[%d] = %q, want %q; got=%#v", index, got[index], want[index], got)
		}
	}
}

func TestCopyDynamicProfilePreservesGroupWithoutFixedBinding(t *testing.T) {
	manager := NewManager(&config.Config{}, t.TempDir())
	manager.Profiles["source-profile"] = &Profile{
		ProfileId:          "source-profile",
		ProfileName:        "动态实例",
		ProxyMode:          ProxyModeDynamic,
		ProxyGroupName:     " mobile ",
		ProxyId:            "stale-proxy",
		ProxyConfig:        "http://127.0.0.1:8080",
		ProxyBindSourceID:  "stale-source",
		ProxyBindSourceURL: "https://example.com/proxies",
		ProxyBindName:      "旧代理",
		ProxyBindUpdatedAt: "2026-08-24T00:00:00Z",
	}

	profile, err := manager.copyProfile("source-profile", "复制实例", func(*Profile) []string { return nil })
	if err != nil {
		t.Fatalf("copyProfile returned error: %v", err)
	}
	if profile.ProxyMode != ProxyModeDynamic || profile.ProxyGroupName != "mobile" {
		t.Fatalf("copied dynamic selection = (%q, %q), want (%q, %q)", profile.ProxyMode, profile.ProxyGroupName, ProxyModeDynamic, "mobile")
	}
	if profile.ProxyId != "" || profile.ProxyConfig != "" || profile.ProxyBindSourceID != "" || profile.ProxyBindSourceURL != "" || profile.ProxyBindName != "" || profile.ProxyBindUpdatedAt != "" {
		t.Fatalf("copied dynamic profile retained a fixed binding: %#v", profile)
	}
}
