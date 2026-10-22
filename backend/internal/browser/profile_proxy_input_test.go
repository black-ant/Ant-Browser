package browser

import (
	"ant-chrome/backend/internal/config"
	"testing"
)

func TestResolveProfileProxyInputDynamicDropsFixedBinding(t *testing.T) {
	manager := NewManager(config.DefaultConfig(), "")
	resolved, err := manager.resolveProfileProxyInput(ProfileInput{
		ProxyMode:      ProxyModeDynamic,
		ProxyId:        "fixed-proxy",
		ProxyConfig:    "http://127.0.0.1:8080",
		ProxyGroupName: "  mobile  ",
	})
	if err != nil {
		t.Fatalf("resolveProfileProxyInput returned error: %v", err)
	}
	if resolved.ProxyMode != ProxyModeDynamic || resolved.ProxyId != "" || resolved.ProxyConfig != "" || resolved.ProxyGroupName != "mobile" {
		t.Fatalf("resolved dynamic proxy = %#v", resolved)
	}
}

func TestResolveProfileProxyInputLocalRequiresConfig(t *testing.T) {
	manager := NewManager(config.DefaultConfig(), "")
	if _, err := manager.resolveProfileProxyInput(ProfileInput{ProxyMode: ProxyModeLocal}); err == nil {
		t.Fatal("expected missing local proxy config error")
	}
}

func TestResolveProfileProxyInputInfersLegacyLocalMode(t *testing.T) {
	manager := NewManager(config.DefaultConfig(), "")
	resolved, err := manager.resolveProfileProxyInput(ProfileInput{ProxyConfig: "http://127.0.0.1:8080"})
	if err != nil {
		t.Fatalf("resolveProfileProxyInput returned error: %v", err)
	}
	if resolved.ProxyMode != ProxyModeLocal || resolved.ProxyConfig != "http://127.0.0.1:8080" {
		t.Fatalf("resolved legacy local proxy = %#v", resolved)
	}
}

func TestApplyDefaultsKeepsDynamicProfileUnbound(t *testing.T) {
	manager := NewManager(config.DefaultConfig(), "")
	profile := &Profile{
		ProfileId:         "dynamic-profile",
		ProxyMode:         ProxyModeDynamic,
		ProxyGroupName:    " mobile ",
		ProxyId:           "__direct__",
		ProxyConfig:       "direct://",
		ProxyBindSourceID: "old-source",
	}
	if !manager.ApplyDefaults(profile) {
		t.Fatal("ApplyDefaults did not clear the fixed proxy binding")
	}
	if profile.ProxyId != "" || profile.ProxyConfig != "" || profile.ProxyBindSourceID != "" || profile.ProxyGroupName != "mobile" {
		t.Fatalf("dynamic profile defaults = %#v", profile)
	}
}

func TestApplyDefaultsUpgradesLegacyFingerprintArgsAndReportsChange(t *testing.T) {
	manager := NewManager(&config.Config{}, t.TempDir())
	profile := &Profile{
		ProfileId: `legacy`,
		FingerprintArgs: []string{
			`--fingerprint-brand=Chrome`,
			`--fingerprint-platform=windows`,
		},
	}

	if !manager.ApplyDefaults(profile) {
		t.Fatal(`ApplyDefaults reported no change`)
	}
	assertStringSliceContains(t, profile.FingerprintArgs, `--disable-non-proxied-udp`)
	assertStringSliceContains(t, profile.FingerprintArgs, `--fingerprinting-canvas-image-data-noise`)
	assertStringSliceContains(t, profile.FingerprintArgs, `--fingerprinting-client-rects-noise`)
}

func TestApplyDefaultsKeepsLocalProxyOutOfPool(t *testing.T) {
	manager := NewManager(config.DefaultConfig(), "")
	manager.Config.Browser.Proxies = []config.BrowserProxy{{
		ProxyId:     "pool-proxy",
		ProxyConfig: "http://127.0.0.1:8080",
		GroupName:   "mobile",
	}}
	profile := &Profile{
		ProfileId:          "local-profile",
		ProxyMode:          ProxyModeLocal,
		ProxyConfig:        "http://127.0.0.1:8080",
		ProxyId:            "pool-proxy",
		ProxyGroupName:     "mobile",
		ProxyBindSourceID:  "old-source",
		ProxyBindSourceURL: "https://example.com/proxies",
		ProxyBindName:      "旧代理",
	}

	if !manager.ApplyDefaults(profile) {
		t.Fatal("ApplyDefaults did not clear the local profile's pool binding")
	}
	if profile.ProxyMode != ProxyModeLocal || profile.ProxyId != "" || profile.ProxyConfig != "http://127.0.0.1:8080" || profile.ProxyGroupName != "" || profile.ProxyBindSourceID != "" || profile.ProxyBindSourceURL != "" || profile.ProxyBindName != "" {
		t.Fatalf("local proxy defaults = %#v", profile)
	}
}
