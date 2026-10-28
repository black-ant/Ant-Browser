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
	changed, _, err := manager.ApplyDefaults(profile)
	if err != nil {
		t.Fatalf("ApplyDefaults returned error: %v", err)
	}
	if !changed {
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
		ProxyId:   directProxyID,
		FingerprintArgs: []string{
			`--fingerprint-brand=Chrome`,
			`--fingerprint-platform=windows`,
		},
	}

	changed, _, err := manager.ApplyDefaults(profile)
	if err != nil {
		t.Fatalf("ApplyDefaults returned error: %v", err)
	}
	if !changed {
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

	changed, _, err := manager.ApplyDefaults(profile)
	if err != nil {
		t.Fatalf("ApplyDefaults returned error: %v", err)
	}
	if !changed {
		t.Fatal("ApplyDefaults did not clear the local profile's pool binding")
	}
	if profile.ProxyMode != ProxyModeLocal || profile.ProxyId != "" || profile.ProxyConfig != "http://127.0.0.1:8080" || profile.ProxyGroupName != "" || profile.ProxyBindSourceID != "" || profile.ProxyBindSourceURL != "" || profile.ProxyBindName != "" {
		t.Fatalf("local proxy defaults = %#v", profile)
	}
}

// 代理 ID 与配置快照同时为空时必须报错，不允许自动绑定直连。
func TestApplyDefaultsRejectsEmptyProxyBinding(t *testing.T) {
	manager := NewManager(config.DefaultConfig(), "")
	profile := &Profile{ProfileId: "empty-binding", ProfileName: "空绑定"}

	if _, _, err := manager.ApplyDefaults(profile); err == nil {
		t.Fatal("双空绑定应报错，不能自动改为直连")
	}
	if profile.ProxyId == directProxyID || profile.ProxyConfig == "direct://" {
		t.Fatalf("双空绑定被自动改为直连: proxyId=%q proxyConfig=%q", profile.ProxyId, profile.ProxyConfig)
	}
}

// 保存时未知代理 ID 必须报错，不允许丢弃 ID 降级为裸配置。
func TestResolveProfileProxyInputRejectsUnknownProxyID(t *testing.T) {
	manager := NewManager(config.DefaultConfig(), "")
	if _, err := manager.resolveProfileProxyInput(ProfileInput{
		ProxyId:     "proxy-gone",
		ProxyConfig: "http://127.0.0.1:8080",
	}); err == nil {
		t.Fatal("未知代理 ID 应报错，不能静默改用 proxyConfig")
	}
}

// 导入后代理 ID 失效但配置快照可用：沿用快照继续（出口不变），但必须给出可见提示。
func TestApplyDefaultsUsesSnapshotAndReportsNoticeWhenProxyIDMissing(t *testing.T) {
	manager := NewManager(config.DefaultConfig(), "")
	profile := &Profile{
		ProfileId:   "snapshot-profile",
		ProfileName: "快照实例",
		ProxyMode:   ProxyModeFixed,
		ProxyId:     "proxy-gone",
		ProxyConfig: "http://127.0.0.1:8080",
	}

	_, notice, err := manager.ApplyDefaults(profile)
	if err != nil {
		t.Fatalf("ApplyDefaults returned error: %v", err)
	}
	if profile.ProxyId != "" || profile.ProxyConfig != "http://127.0.0.1:8080" {
		t.Fatalf("快照兜底结果异常: proxyId=%q proxyConfig=%q", profile.ProxyId, profile.ProxyConfig)
	}
	if profile.ProxyConfig == "direct://" {
		t.Fatal("快照兜底不得改为直连")
	}
	if notice == "" {
		t.Fatal("快照兜底必须给出可见提示，不能只写日志")
	}
}

// 订阅刷新导致节点 ID 变化：按绑定快照找回同节点，并给出可见提示。
func TestApplyDefaultsRebindsBySnapshotAndReportsNotice(t *testing.T) {
	manager := NewManager(config.DefaultConfig(), "")
	manager.Config.Browser.Proxies = []config.BrowserProxy{{
		ProxyId:     "proxy-new",
		ProxyName:   "节点A",
		ProxyConfig: "http://node-a:8080",
	}}
	profile := &Profile{
		ProfileId:     "rebind-profile",
		ProfileName:   "重关联实例",
		ProxyMode:     ProxyModeFixed,
		ProxyId:       "proxy-old",
		ProxyBindName: "节点A",
	}

	_, notice, err := manager.ApplyDefaults(profile)
	if err != nil {
		t.Fatalf("ApplyDefaults returned error: %v", err)
	}
	if profile.ProxyId != "proxy-new" {
		t.Fatalf("未按快照找回节点: %q", profile.ProxyId)
	}
	if profile.ProxyConfig != "http://node-a:8080" {
		t.Fatalf("找回节点后配置未同步: %q", profile.ProxyConfig)
	}
	if notice == "" {
		t.Fatal("自动重关联必须给出可见提示")
	}
}
