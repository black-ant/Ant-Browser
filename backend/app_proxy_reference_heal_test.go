package backend

import (
	"strings"
	"testing"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
)

func newHealManager() *browser.Manager {
	return browser.NewManager(config.DefaultConfig(), "")
}

func TestBindProfileToProxyClearsProxyBindError(t *testing.T) {
	profile := &browser.Profile{
		ProfileId:      "p1",
		ProxyMode:      browser.ProxyModeFixed,
		ProxyId:        "gone",
		ProxyBindError: "代理绑定已清理：旧节点已删除",
	}
	proxy := browser.Proxy{ProxyId: "p2", ProxyName: "新节点", ProxyConfig: "socks5://127.0.0.1:1080"}

	changed := browser.BindProfileToProxy(profile, proxy, true)

	if !changed {
		t.Fatal("期望绑定变更")
	}
	if profile.ProxyBindError != "" {
		t.Fatalf("重新绑定后应清除 proxyBindError: %q", profile.ProxyBindError)
	}
	if profile.ProxyId != "p2" {
		t.Fatalf("ProxyId = %q", profile.ProxyId)
	}
}

func TestHealBindErrorPrefix(t *testing.T) {
	if !strings.Contains(proxyBindClearedPrefix, "代理绑定已清理") {
		t.Fatalf("错误文案前缀不正确: %q", proxyBindClearedPrefix)
	}
}
