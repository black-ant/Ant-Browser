package backend

import (
	"ant-chrome/backend/internal/browser"
	"testing"
	"time"
)

func TestSelectVirtualDynamicBrowserProxyCandidates(t *testing.T) {
	testedAt := time.Now().Format(time.RFC3339)
	proxies := []BrowserProxy{
		{ProxyId: "proxy-slow", ProxyConfig: "http://slow:8080", LastTestOk: true, LastTestedAt: testedAt, LastLatencyMs: 300},
		{ProxyId: "proxy-failed", ProxyConfig: "http://failed:8080", LastTestOk: false, LastTestedAt: testedAt, LastLatencyMs: 10},
		{ProxyId: "proxy-fast", ProxyConfig: "http://fast:8080", LastTestOk: true, LastTestedAt: testedAt, LastLatencyMs: 100},
	}
	group := browser.ProxyVirtualGroup{MemberProxyIds: []string{"proxy-slow", "proxy-failed", "proxy-fast"}}
	candidates := selectVirtualDynamicBrowserProxyCandidates(proxies, group, defaultDynamicProxyMaxTestAge)
	if len(candidates) != 2 || candidates[0].ProxyId != "proxy-fast" || candidates[1].ProxyId != "proxy-slow" {
		t.Fatalf("virtual candidates = %#v, want healthy members ordered by latency", candidates)
	}
}

func TestVirtualProxyGroupRoundRobinAdvancesAfterSelection(t *testing.T) {
	app := NewApp("")
	group := browser.ProxyVirtualGroup{
		GroupId:        "virtual-1",
		GroupName:      "round-robin",
		Strategy:       browser.ProxyVirtualGroupStrategyRoundRobin,
		MemberProxyIds: []string{"proxy-a", "proxy-b", "proxy-c"},
	}
	proxies := []BrowserProxy{
		{ProxyId: "proxy-a"},
		{ProxyId: "proxy-b"},
		{ProxyId: "proxy-c"},
	}
	key := proxyVirtualGroupCursorKey(group.GroupId, group.GroupName)

	firstOrder, memberIDs := app.orderVirtualProxyCandidates(key, group, proxies, proxies)
	if firstOrder[0].ProxyId != "proxy-a" {
		t.Fatalf("first selected member = %q, want proxy-a", firstOrder[0].ProxyId)
	}
	app.commitVirtualProxyCursor(key, firstOrder[0].ProxyId, memberIDs, proxies)
	secondOrder, _ := app.orderVirtualProxyCandidates(key, group, proxies, proxies)
	if secondOrder[0].ProxyId != "proxy-b" {
		t.Fatalf("second selected member = %q, want proxy-b", secondOrder[0].ProxyId)
	}
}

func TestStickyVirtualProxyGroupKeepsMemberAndFallsForwardWhenUnavailable(t *testing.T) {
	app := NewApp("")
	group := browser.ProxyVirtualGroup{
		GroupId:         "virtual-2",
		GroupName:       "sticky",
		Strategy:        browser.ProxyVirtualGroupStrategySticky,
		SelectedProxyId: "proxy-b",
		MemberProxyIds:  []string{"proxy-a", "proxy-b", "proxy-c"},
	}
	allProxies := []BrowserProxy{{ProxyId: "proxy-a"}, {ProxyId: "proxy-b"}, {ProxyId: "proxy-c"}}
	key := proxyVirtualGroupCursorKey(group.GroupId, group.GroupName)

	stickyOrder, _ := app.orderVirtualProxyCandidates(key, group, allProxies, allProxies)
	if stickyOrder[0].ProxyId != "proxy-b" {
		t.Fatalf("sticky selected member = %q, want proxy-b", stickyOrder[0].ProxyId)
	}

	availableProxies := []BrowserProxy{{ProxyId: "proxy-a"}, {ProxyId: "proxy-c"}}
	fallbackOrder, _ := app.orderVirtualProxyCandidates(key, group, availableProxies, allProxies)
	if fallbackOrder[0].ProxyId != "proxy-c" {
		t.Fatalf("sticky fallback member = %q, want next member proxy-c", fallbackOrder[0].ProxyId)
	}
}

// 固定绑定虚拟分组：round_robin 每次启动轮询下一个成员。
func TestPickVirtualGroupStartProxyRoundRobin(t *testing.T) {
	app := NewApp("")
	group := browser.ProxyVirtualGroup{
		GroupId:        "vg-fixed-1",
		GroupName:      "fixed-rr",
		Strategy:       browser.ProxyVirtualGroupStrategyRoundRobin,
		MemberProxyIds: []string{"proxy-a", "proxy-b"},
	}
	proxies := []BrowserProxy{
		{ProxyId: "proxy-a", ProxyConfig: "http://a:8080"},
		{ProxyId: "proxy-b", ProxyConfig: "http://b:8080"},
	}

	firstID, firstConfig, ok := app.pickVirtualGroupStartProxy(group, proxies, "profile-1")
	if !ok || firstID != "proxy-a" || firstConfig != "http://a:8080" {
		t.Fatalf("first pick = (%q,%q,%v), want proxy-a", firstID, firstConfig, ok)
	}
	secondID, _, ok := app.pickVirtualGroupStartProxy(group, proxies, "profile-2")
	if !ok || secondID != "proxy-b" {
		t.Fatalf("second pick = (%q,%v), want proxy-b", secondID, ok)
	}
	thirdID, _, ok := app.pickVirtualGroupStartProxy(group, proxies, "profile-3")
	if !ok || thirdID != "proxy-a" {
		t.Fatalf("third pick = (%q,%v), want proxy-a", thirdID, ok)
	}
}

// sticky 策略固定使用 selectedProxyId 成员。
func TestPickVirtualGroupStartProxySticky(t *testing.T) {
	app := NewApp("")
	group := browser.ProxyVirtualGroup{
		GroupId:         "vg-fixed-2",
		GroupName:       "fixed-sticky",
		Strategy:        browser.ProxyVirtualGroupStrategySticky,
		SelectedProxyId: "proxy-b",
		MemberProxyIds:  []string{"proxy-a", "proxy-b"},
	}
	proxies := []BrowserProxy{
		{ProxyId: "proxy-a", ProxyConfig: "http://a:8080"},
		{ProxyId: "proxy-b", ProxyConfig: "http://b:8080"},
	}
	for i := 0; i < 3; i++ {
		memberID, _, ok := app.pickVirtualGroupStartProxy(group, proxies, "profile-sticky")
		if !ok || memberID != "proxy-b" {
			t.Fatalf("sticky pick #%d = (%q,%v), want proxy-b", i, memberID, ok)
		}
	}
}

// 成员配置为空的代理不参与固定绑定选择。
func TestPickVirtualGroupStartProxySkipsEmptyConfig(t *testing.T) {
	app := NewApp("")
	group := browser.ProxyVirtualGroup{
		GroupId:        "vg-fixed-3",
		GroupName:      "fixed-empty",
		MemberProxyIds: []string{"proxy-empty", "proxy-ok"},
	}
	proxies := []BrowserProxy{
		{ProxyId: "proxy-empty", ProxyConfig: ""},
		{ProxyId: "proxy-ok", ProxyConfig: "http://ok:8080"},
	}
	memberID, config, ok := app.pickVirtualGroupStartProxy(group, proxies, "profile-empty")
	if !ok || memberID != "proxy-ok" || config != "http://ok:8080" {
		t.Fatalf("pick = (%q,%q,%v), want proxy-ok", memberID, config, ok)
	}
}

// 虚拟分组无可用成员时返回 false（实例启动会报错而非静默直连）。
func TestPickVirtualGroupStartProxyNoMembers(t *testing.T) {
	app := NewApp("")
	group := browser.ProxyVirtualGroup{GroupId: "vg-fixed-4", GroupName: "empty", MemberProxyIds: []string{"missing"}}
	proxies := []BrowserProxy{{ProxyId: "proxy-a", ProxyConfig: "http://a:8080"}}
	if _, _, ok := app.pickVirtualGroupStartProxy(group, proxies, "profile-none"); ok {
		t.Fatal("expected false when no member exists")
	}
}

// 测速目标解析：实体代理 ID 原样返回，虚拟分组 ID 解析为成员。
func TestResolveVirtualGroupSpeedTestTarget(t *testing.T) {
	app := NewApp("")
	proxies := []BrowserProxy{
		{ProxyId: "proxy-a", ProxyConfig: "http://a:8080"},
		{ProxyId: "proxy-b", ProxyConfig: "http://b:8080"},
	}
	if id, ok := app.resolveVirtualGroupSpeedTestTarget("proxy-a", proxies); ok || id != "" {
		t.Fatalf("实体代理不应被解析: (%q,%v)", id, ok)
	}
	if id, ok := app.resolveVirtualGroupSpeedTestTarget("unknown", proxies); ok || id != "" {
		t.Fatalf("未知 ID 不应被解析: (%q,%v)", id, ok)
	}
}
