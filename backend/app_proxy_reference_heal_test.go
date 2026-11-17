package backend

import (
	"strings"
	"testing"

	"ant-chrome/backend/internal/browser"
)

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

func TestHealFetchProxyOnlyClearsRowWithInvalidFetchID(t *testing.T) {
	app := &App{}
	pool := []BrowserProxy{
		{
			ProxyId:                "proxy-a",
			SourceID:               "src-1",
			SourceURL:              "https://sub.example.com/list",
			ProxyName:              "节点A",
			SourceFetchProxyID:     "gone-proxy",
			SourceLastRefreshError: "",
		},
		{
			ProxyId:                "proxy-b",
			SourceID:               "src-1",
			SourceURL:              "https://sub.example.com/list",
			ProxyName:              "节点B",
			SourceFetchProxyID:     "proxy-a",
			SourceLastRefreshError: "",
		},
	}

	cleared, changed := app.healFetchProxyReferences(pool)

	if cleared != 1 || !changed {
		t.Fatalf("cleared=%d changed=%v", cleared, changed)
	}
	if pool[0].SourceFetchProxyID != "" || pool[0].SourceLastRefreshError == "" {
		t.Fatalf("失效行应清空并写错误: %+v", pool[0])
	}
	if pool[1].SourceFetchProxyID != "proxy-a" || pool[1].SourceLastRefreshError != "" {
		t.Fatalf("同源有效行不应被误清或写假错误: %+v", pool[1])
	}
}

func TestClearProfileProxyBindingClearsBindError(t *testing.T) {
	profile := &browser.Profile{
		ProxyMode:      browser.ProxyModeDynamic,
		ProxyBindError: "代理绑定已清理：旧节点已删除",
	}
	if !browser.ClearProfileProxyBinding(profile) {
		t.Fatal("期望清理变更")
	}
	if profile.ProxyBindError != "" {
		t.Fatalf("动态模式切换后应清除历史错误: %q", profile.ProxyBindError)
	}
}
