package browser

import "testing"

func TestNormalizeProxyVirtualGroupDefaultsStrategyAndTrimsSelection(t *testing.T) {
	group := NormalizeProxyVirtualGroup(ProxyVirtualGroup{
		GroupName:       " pool ",
		SelectedProxyId: " proxy-1 ",
	})
	if group.Strategy != ProxyVirtualGroupStrategyRoundRobin {
		t.Fatalf("default strategy = %q, want %q", group.Strategy, ProxyVirtualGroupStrategyRoundRobin)
	}
	if group.SelectedProxyId != "proxy-1" {
		t.Fatalf("selected proxy ID = %q, want proxy-1", group.SelectedProxyId)
	}
	group.Strategy = "STICKY"
	group = NormalizeProxyVirtualGroup(group)
	if group.Strategy != ProxyVirtualGroupStrategySticky {
		t.Fatalf("normalized sticky strategy = %q, want %q", group.Strategy, ProxyVirtualGroupStrategySticky)
	}
}

func TestNormalizeProxyVirtualGroup(t *testing.T) {
	group := NormalizeProxyVirtualGroup(ProxyVirtualGroup{
		GroupId:        " group-1 ",
		GroupName:      "  round-robin  ",
		MemberProxyIds: []string{" proxy-1 ", "Proxy-2", "proxy-1", " "},
	})

	if group.GroupId != "group-1" || group.GroupName != "round-robin" {
		t.Fatalf("normalized group identity = %#v", group)
	}
	want := []string{"proxy-1", "Proxy-2"}
	if len(group.MemberProxyIds) != len(want) {
		t.Fatalf("member proxy IDs = %#v, want %#v", group.MemberProxyIds, want)
	}
	for index, name := range want {
		if group.MemberProxyIds[index] != name {
			t.Fatalf("member proxy ID[%d] = %q, want %q", index, group.MemberProxyIds[index], name)
		}
	}
}

func TestExpandProxyVirtualGroupUsesVirtualGroupMemberOrder(t *testing.T) {
	proxies := []Proxy{
		{ProxyId: "mobile-1", ProxyName: "Mobile 1", GroupName: "mobile"},
		{ProxyId: "desktop-1", ProxyName: "Desktop 1", GroupName: "desktop"},
		{ProxyId: "mobile-2", ProxyName: "Mobile 2", GroupName: "mobile"},
	}
	group := ProxyVirtualGroup{GroupName: "pool", MemberProxyIds: []string{"mobile-2", "desktop-1", "mobile-2", "missing"}}

	got := ExpandProxyVirtualGroup(group, proxies)
	if len(got) != 2 {
		t.Fatalf("expanded candidate count = %d, want 2", len(got))
	}
	want := []string{"mobile-2", "desktop-1"}
	for index, proxyID := range want {
		if got[index].ProxyId != proxyID {
			t.Fatalf("expanded candidate[%d] = %q, want %q", index, got[index].ProxyId, proxyID)
		}
	}
}

func TestExpandProxyVirtualGroupMergesExplicitAndPhysicalGroupMembers(t *testing.T) {
	proxies := []Proxy{
		{ProxyId: "mobile-1", ProxyName: "Mobile 1", GroupName: "mobile"},
		{ProxyId: "desktop-1", ProxyName: "Desktop 1", GroupName: "desktop"},
		{ProxyId: "mobile-2", ProxyName: "Mobile 2", GroupName: "mobile"},
	}
	group := ProxyVirtualGroup{
		GroupName:        "pool",
		MemberProxyIds:   []string{"mobile-1"},
		MemberGroupNames: []string{"mobile"},
	}

	got := ExpandProxyVirtualGroup(group, proxies)
	want := []string{"mobile-1", "mobile-2"}
	if len(got) != len(want) {
		t.Fatalf("expanded candidate count = %d, want %d", len(got), len(want))
	}
	for index, proxyID := range want {
		if got[index].ProxyId != proxyID {
			t.Fatalf("expanded candidate[%d] = %q, want %q", index, got[index].ProxyId, proxyID)
		}
	}
}
