package backend

import (
	"errors"
	"testing"
	"time"

	"ant-chrome/backend/internal/browser"
)

func TestRuntimeFailoverCooldownBlocksImmediateRetry(t *testing.T) {
	app := newVirtualGroupHealthTestApp(t)

	attempt, allowed := app.beginRuntimeFailover("profile-1")
	if !allowed || attempt != 1 {
		t.Fatalf("first attempt = (%d,%v), want (1,true)", attempt, allowed)
	}
	if _, allowed := app.beginRuntimeFailover("profile-1"); allowed {
		t.Fatal("冷却窗口内的第二次尝试应被拒绝")
	}

	// 其他实例互不影响。
	if _, allowed := app.beginRuntimeFailover("profile-2"); !allowed {
		t.Fatal("不同实例不应共享冷却窗口")
	}
}

func TestRuntimeFailoverMaxAttempts(t *testing.T) {
	app := newVirtualGroupHealthTestApp(t)
	app.config.ProxyCheck.RuntimeFailoverCooldownMinutes = 1
	app.config.ProxyCheck.RuntimeFailoverMaxAttempts = 2

	for i := 1; i <= 2; i++ {
		attempt, allowed := app.beginRuntimeFailover("profile-max")
		if !allowed || attempt != i {
			t.Fatalf("attempt %d = (%d,%v), want (%d,true)", i, attempt, allowed, i)
		}
		// 跳过冷却窗口。
		app.proxyFailoverMu.Lock()
		app.proxyFailoverLastAt["profile-max"] = time.Now().Add(-2 * time.Minute)
		app.proxyFailoverMu.Unlock()
	}

	if _, allowed := app.beginRuntimeFailover("profile-max"); allowed {
		t.Fatal("超过最大尝试次数后应停止故障切换")
	}
}

func TestRuntimeFailoverResetsAfterLongStableWindow(t *testing.T) {
	app := newVirtualGroupHealthTestApp(t)
	app.config.ProxyCheck.RuntimeFailoverCooldownMinutes = 1
	app.config.ProxyCheck.RuntimeFailoverMaxAttempts = 1

	if _, allowed := app.beginRuntimeFailover("profile-reset"); !allowed {
		t.Fatal("首次尝试应允许")
	}
	// 超过 cooldown*3 后视为恢复稳定，计数清零。
	app.proxyFailoverMu.Lock()
	app.proxyFailoverLastAt["profile-reset"] = time.Now().Add(-10 * time.Minute)
	app.proxyFailoverMu.Unlock()
	if attempt, allowed := app.beginRuntimeFailover("profile-reset"); !allowed || attempt != 1 {
		t.Fatalf("稳定窗口后 attempt = (%d,%v), want (1,true)", attempt, allowed)
	}
}

func TestMarkProxyDegradedInAllGroups(t *testing.T) {
	app := newVirtualGroupHealthTestApp(t)
	group := browser.ProxyVirtualGroup{
		GroupId:        "vg-degrade-all",
		GroupName:      "降级强信号",
		MemberProxyIds: []string{"proxy-a", "proxy-b"},
	}
	if err := app.browserMgr.ProxyVirtualGroupDAO.Upsert(group); err != nil {
		t.Fatalf("seed group failed: %v", err)
	}
	if err := app.browserMgr.ProxyDAO.ReplaceAll([]BrowserProxy{
		{ProxyId: "proxy-a", ProxyName: "A", ProxyConfig: "http://a:8080"},
		{ProxyId: "proxy-b", ProxyName: "B", ProxyConfig: "http://b:8080"},
	}); err != nil {
		t.Fatalf("seed proxies failed: %v", err)
	}
	app.invalidateVirtualGroupSnapshot()

	app.markProxyDegradedInAllGroups("proxy-a", errors.New("bridge exited"))

	states, err := app.browserMgr.ProxyVirtualGroupMemberStateDAO.ListByGroup(group.GroupId)
	if err != nil {
		t.Fatalf("ListByGroup failed: %v", err)
	}
	if len(states) != 1 || states[0].ProxyId != "proxy-a" || states[0].State != browser.MemberStateDegraded {
		t.Fatalf("states = %#v, want proxy-a degraded", states)
	}
	if states[0].LastError != "bridge exited" {
		t.Fatalf("last error = %q", states[0].LastError)
	}

	// 再次标记不应重复写事件或翻转计数。
	app.markProxyDegradedInAllGroups("proxy-a", errors.New("bridge exited"))
}

func TestVirtualGroupBoundToProfile(t *testing.T) {
	app := newVirtualGroupHealthTestApp(t)
	fixed := browser.ProxyVirtualGroup{
		GroupId:        "vg-fixed",
		GroupName:      "固定绑定",
		MemberProxyIds: []string{"proxy-a"},
	}
	dynamic := browser.ProxyVirtualGroup{
		GroupId:        "vg-dynamic",
		GroupName:      "动态绑定",
		MemberProxyIds: []string{"proxy-b"},
	}
	for _, group := range []browser.ProxyVirtualGroup{fixed, dynamic} {
		if err := app.browserMgr.ProxyVirtualGroupDAO.Upsert(group); err != nil {
			t.Fatalf("seed group failed: %v", err)
		}
	}
	app.invalidateVirtualGroupSnapshot()

	app.browserMgr.Mutex.Lock()
	app.browserMgr.Profiles["profile-fixed"] = &BrowserProfile{ProfileId: "profile-fixed", ProxyId: fixed.GroupId}
	app.browserMgr.Profiles["profile-dynamic"] = &BrowserProfile{
		ProfileId:      "profile-dynamic",
		ProxyMode:      browser.ProxyModeDynamic,
		ProxyGroupName: dynamic.GroupName,
	}
	app.browserMgr.Profiles["profile-none"] = &BrowserProfile{ProfileId: "profile-none", ProxyId: "proxy-a"}
	app.browserMgr.Mutex.Unlock()

	if group, ok := app.virtualGroupBoundToProfile("profile-fixed"); !ok || group.GroupId != fixed.GroupId {
		t.Fatalf("fixed binding = (%q,%v)", group.GroupId, ok)
	}
	if group, ok := app.virtualGroupBoundToProfile("profile-dynamic"); !ok || group.GroupId != dynamic.GroupId {
		t.Fatalf("dynamic binding = (%q,%v)", group.GroupId, ok)
	}
	if _, ok := app.virtualGroupBoundToProfile("profile-none"); ok {
		t.Fatal("普通代理绑定不应识别为虚拟分组")
	}
	if _, ok := app.virtualGroupBoundToProfile("profile-missing"); ok {
		t.Fatal("不存在的实例不应识别为虚拟分组")
	}
}

func TestFirstVirtualGroupCandidateExcept(t *testing.T) {
	candidates := []BrowserProxy{{ProxyId: "proxy-a"}, {ProxyId: "proxy-b"}}
	next := firstVirtualGroupCandidateExcept(candidates, "Proxy-A")
	if next == nil || next.ProxyId != "proxy-b" {
		t.Fatalf("next = %#v, want proxy-b", next)
	}
	if got := firstVirtualGroupCandidateExcept(candidates[:1], "proxy-a"); got != nil {
		t.Fatalf("仅剩失败节点时应返回 nil, got %#v", got)
	}
}

// 故障切换必须能把下一次启动固定到指定成员。
func TestSetVirtualProxyCursorPinsNextStart(t *testing.T) {
	app := NewApp("")
	group := browser.ProxyVirtualGroup{
		GroupId:        "vg-pin",
		GroupName:      "pin",
		Strategy:       browser.ProxyVirtualGroupStrategyRoundRobin,
		MemberProxyIds: []string{"proxy-a", "proxy-b", "proxy-c"},
	}
	candidates := []BrowserProxy{
		{ProxyId: "proxy-a"},
		{ProxyId: "proxy-b"},
		{ProxyId: "proxy-c"},
	}
	key := proxyVirtualGroupCursorKey(group.GroupId, group.GroupName)
	if !app.setVirtualProxyCursor(key, "proxy-c", nil, candidates) {
		t.Fatal("固定到组内成员应成功")
	}
	ordered, _ := app.orderVirtualProxyCandidates(key, group, candidates, candidates)
	if len(ordered) != 3 || ordered[0].ProxyId != "proxy-c" {
		t.Fatalf("ordered = %#v, want proxy-c 优先", ordered)
	}

	if app.setVirtualProxyCursor(key, "proxy-missing", nil, candidates) {
		t.Fatal("非成员不应固定成功")
	}
	ordered, _ = app.orderVirtualProxyCandidates(key, group, candidates, candidates)
	if len(ordered) == 0 || ordered[0].ProxyId != "proxy-c" {
		t.Fatalf("固定失败不应改变游标，实际 %#v", ordered)
	}
}

// 桥接引用读取用于故障切换事件上报实际生效成员。
func TestBridgeProxyIDForProfile(t *testing.T) {
	app := NewApp("")
	app.bindProfileProxyBridge("profile-1", newProfileProxyBridgeRef("xray", "bridge-key", "proxy-a"))
	if got := app.bridgeProxyIDForProfile("profile-1"); got != "proxy-a" {
		t.Fatalf("bridge proxy id = %q, want proxy-a", got)
	}
	app.releaseProfileProxyBridge("profile-1")
	if got := app.bridgeProxyIDForProfile("profile-1"); got != "" {
		t.Fatalf("released bridge proxy id = %q, want empty", got)
	}
}

// 启动期桥接失败是弱信号：一次失败只进入 suspect，连续两次才降级。
func TestStartBridgeFailureRequiresThresholdBeforeDegrade(t *testing.T) {
	app := newVirtualGroupHealthTestApp(t)
	if err := app.browserMgr.ProxyDAO.ReplaceAll([]BrowserProxy{
		{ProxyId: "proxy-a", ProxyName: "A", ProxyConfig: "http://a:8080"},
	}); err != nil {
		t.Fatalf("seed proxies failed: %v", err)
	}
	group := browser.ProxyVirtualGroup{GroupId: "vg-start-fail", GroupName: "start-fail", MemberProxyIds: []string{"proxy-a"}}
	if err := app.browserMgr.ProxyVirtualGroupDAO.Upsert(group); err != nil {
		t.Fatalf("seed group failed: %v", err)
	}
	app.invalidateVirtualGroupSnapshot()

	app.reportProxyBridgeFailure("profile-1", "profile", "proxy-a", profileProxyBridgeEngineXray, errors.New("dial failed"))
	states, err := app.browserMgr.ProxyVirtualGroupMemberStateDAO.ListByGroup(group.GroupId)
	if err != nil {
		t.Fatalf("ListByGroup returned error: %v", err)
	}
	if len(states) != 1 || states[0].State != browser.MemberStateSuspect {
		t.Fatalf("state after first start failure = %#v, want suspect", states)
	}

	app.reportProxyBridgeFailure("profile-1", "profile", "proxy-a", profileProxyBridgeEngineXray, errors.New("dial failed"))
	states, err = app.browserMgr.ProxyVirtualGroupMemberStateDAO.ListByGroup(group.GroupId)
	if err != nil {
		t.Fatalf("ListByGroup returned error: %v", err)
	}
	if len(states) != 1 || states[0].State != browser.MemberStateDegraded {
		t.Fatalf("state after second start failure = %#v, want degraded", states)
	}
}
