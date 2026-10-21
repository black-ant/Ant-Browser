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
