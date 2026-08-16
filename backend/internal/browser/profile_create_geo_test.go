package browser

import (
	"strings"
	"testing"

	"ant-chrome/backend/internal/config"
)

// 直连(无代理)新建的实例,人设必须对齐到本地国家(默认 CN):
// 时区 Asia/Shanghai、语言 zh-CN。否则“真机中国 IP + 境外时区/语言”会被平台风控判废登录。
func TestCreateDirectProfileAlignsToLocalCountryCN(t *testing.T) {
	cfg := &config.Config{}
	cfg.Browser.LocalCountry = "CN"
	manager := NewManager(cfg, t.TempDir())
	manager.IdentityService = newTestIdentityService(t)

	p, err := manager.Create(ProfileInput{ProfileName: "direct-cn"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if tz, _ := argValue(p.FingerprintArgs, "--timezone="); tz != "Asia/Shanghai" {
		t.Fatalf("direct profile timezone = %q, want Asia/Shanghai; args=%v", tz, p.FingerprintArgs)
	}
	if al, _ := argValue(p.FingerprintArgs, "--accept-lang="); !strings.HasPrefix(al, "zh-CN") {
		t.Fatalf("direct profile accept-lang = %q, want zh-CN prefix; args=%v", al, p.FingerprintArgs)
	}
	// 仍须保留独立、非零的设备种子(对齐国家不影响唯一性)。
	if seed, ok := argValue(p.FingerprintArgs, "--fingerprint="); !ok || seed == "" || seed == "0" {
		t.Fatalf("direct profile missing non-zero seed: %v", p.FingerprintArgs)
	}
}

// LocalCountry 缺省(空)时应兜底为 CN。
func TestCreateDirectProfileDefaultsToCNWhenUnset(t *testing.T) {
	manager := NewManager(&config.Config{}, t.TempDir()) // LocalCountry 空
	manager.IdentityService = newTestIdentityService(t)

	p, err := manager.Create(ProfileInput{ProfileName: "direct-default"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if tz, _ := argValue(p.FingerprintArgs, "--timezone="); tz != "Asia/Shanghai" {
		t.Fatalf("expected CN fallback timezone Asia/Shanghai, got %q; args=%v", tz, p.FingerprintArgs)
	}
}
