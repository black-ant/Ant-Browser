package browser

import (
	"strings"
	"testing"

	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/identity"
)

// 批量按代理出口地理对齐:多个环境一次对齐,时区/语言全部切到目标地区,设备种子不变。
func TestAlignFingerprintsToProxyGeoBatch(t *testing.T) {
	manager := NewManager(&config.Config{}, t.TempDir())
	manager.IdentityService = newTestIdentityService(t)

	p1, err := manager.Create(ProfileInput{ProfileName: "px-1"})
	if err != nil {
		t.Fatalf("Create p1: %v", err)
	}
	p2, err := manager.Create(ProfileInput{ProfileName: "px-2"})
	if err != nil {
		t.Fatalf("Create p2: %v", err)
	}
	seed1, _ := argValue(p1.FingerprintArgs, "--fingerprint=")
	seed2, _ := argValue(p2.FingerprintArgs, "--fingerprint=")

	geo := identity.GeoInfo{CountryCode: "US", Timezone: "America/Los_Angeles", Latitude: 34.05, Longitude: -118.24}
	aligned, err := manager.AlignFingerprintsToProxyGeo([]string{p1.ProfileId, p2.ProfileId, "not-exist"}, geo)
	if err != nil {
		t.Fatalf("AlignFingerprintsToProxyGeo: %v", err)
	}
	if aligned != 2 {
		t.Fatalf("aligned = %d, want 2", aligned)
	}
	for _, p := range []*Profile{p1, p2} {
		if tz, _ := argValue(p.FingerprintArgs, "--timezone="); tz != "America/Los_Angeles" {
			t.Fatalf("profile %s timezone = %q, want America/Los_Angeles; args=%v", p.ProfileName, tz, p.FingerprintArgs)
		}
		if al, _ := argValue(p.FingerprintArgs, "--accept-lang="); !strings.HasPrefix(al, "en-US") {
			t.Fatalf("profile %s accept-lang = %q, want en-US prefix", p.ProfileName, al)
		}
	}
	if s, _ := argValue(p1.FingerprintArgs, "--fingerprint="); s != seed1 {
		t.Fatalf("p1 seed changed after align: %q -> %q", seed1, s)
	}
	if s, _ := argValue(p2.FingerprintArgs, "--fingerprint="); s != seed2 {
		t.Fatalf("p2 seed changed after align: %q -> %q", seed2, s)
	}
}

// 换绑回直连后按本地国家(CN)重对齐:先模拟挂美国代理被对齐到 US,再调用应回到中国人设。
func TestAlignFingerprintToLocalCountryRealignsDirectProfile(t *testing.T) {
	cfg := &config.Config{}
	cfg.Browser.LocalCountry = "CN"
	manager := NewManager(cfg, t.TempDir())
	manager.IdentityService = newTestIdentityService(t)

	p, err := manager.Create(ProfileInput{ProfileName: "back-to-direct"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := manager.AlignFingerprintsToProxyGeo([]string{p.ProfileId}, identity.GeoInfo{CountryCode: "US", Timezone: "America/New_York"}); err != nil {
		t.Fatalf("pre-align to US: %v", err)
	}
	if tz, _ := argValue(p.FingerprintArgs, "--timezone="); tz != "America/New_York" {
		t.Fatalf("precondition failed: timezone = %q, want America/New_York", tz)
	}

	if _, err := manager.AlignFingerprintToLocalCountry(p.ProfileId); err != nil {
		t.Fatalf("AlignFingerprintToLocalCountry: %v", err)
	}
	if tz, _ := argValue(p.FingerprintArgs, "--timezone="); tz != "Asia/Shanghai" {
		t.Fatalf("timezone = %q, want Asia/Shanghai; args=%v", tz, p.FingerprintArgs)
	}
	if al, _ := argValue(p.FingerprintArgs, "--accept-lang="); !strings.HasPrefix(al, "zh-CN") {
		t.Fatalf("accept-lang = %q, want zh-CN prefix", al)
	}
}

// ProfileProxyBinding 返回当前代理绑定快照;不存在的实例 ok=false。
func TestProfileProxyBindingSnapshot(t *testing.T) {
	manager := NewManager(&config.Config{}, t.TempDir())
	manager.IdentityService = newTestIdentityService(t)
	p, err := manager.Create(ProfileInput{ProfileName: "snap"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	proxyId, proxyConfig, ok := manager.ProfileProxyBinding(p.ProfileId)
	if !ok {
		t.Fatalf("ProfileProxyBinding not found")
	}
	if proxyId != p.ProxyId || proxyConfig != p.ProxyConfig {
		t.Fatalf("snapshot mismatch: got (%q,%q), want (%q,%q)", proxyId, proxyConfig, p.ProxyId, p.ProxyConfig)
	}
	if _, _, ok := manager.ProfileProxyBinding("missing"); ok {
		t.Fatalf("expected ok=false for missing profile")
	}
}
