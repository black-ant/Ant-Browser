package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/database"
	"path/filepath"
	"testing"
	"time"
)

func newVirtualGroupHealthTestApp(t *testing.T) *App {
	t.Helper()
	db, err := database.NewDB(filepath.Join(t.TempDir(), "virtual-group-health.db"))
	if err != nil {
		t.Fatalf("NewDB returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}

	app := NewApp(t.TempDir())
	app.config = config.DefaultConfig()
	app.config.ProxyCheck.VirtualGroupHealthEnabled = boolPtrForTest(true)
	app.browserMgr = browser.NewManager(app.config, app.appRoot)
	app.browserMgr.ProxyDAO = browser.NewSQLiteProxyDAO(db.GetConn())
	app.browserMgr.ProxyVirtualGroupDAO = browser.NewSQLiteProxyVirtualGroupDAO(db.GetConn())
	app.browserMgr.ProxyVirtualGroupMemberStateDAO = browser.NewSQLiteProxyVirtualGroupMemberStateDAO(db.GetConn())
	return app
}

func boolPtrForTest(value bool) *bool {
	v := value
	return &v
}

func seedMemberState(t *testing.T, app *App, groupID string, proxyID string, state browser.MemberHealthState) {
	t.Helper()
	if err := app.browserMgr.ProxyVirtualGroupMemberStateDAO.Upsert(browser.ProxyVirtualGroupMemberState{
		GroupId: groupID,
		ProxyId: proxyID,
		State:   state,
	}); err != nil {
		t.Fatalf("seed member state failed: %v", err)
	}
}

func TestSelectVirtualDynamicBrowserProxyCandidates(t *testing.T) {
	testedAt := time.Now().Format(time.RFC3339)
	proxies := []BrowserProxy{
		{ProxyId: "proxy-slow", ProxyConfig: "http://slow:8080", LastTestOk: true, LastTestedAt: testedAt, LastLatencyMs: 300},
		{ProxyId: "proxy-failed", ProxyConfig: "http://failed:8080", LastTestOk: false, LastTestedAt: testedAt, LastLatencyMs: 10},
		{ProxyId: "proxy-fast", ProxyConfig: "http://fast:8080", LastTestOk: true, LastTestedAt: testedAt, LastLatencyMs: 100},
	}
	group := browser.ProxyVirtualGroup{MemberProxyIds: []string{"proxy-slow", "proxy-failed", "proxy-fast"}}
	candidates, allDegraded := NewApp("").selectVirtualDynamicBrowserProxyCandidates(proxies, group, defaultDynamicProxyMaxTestAge, nil)
	if allDegraded {
		t.Fatal("未启用降级过滤时不应标记全部降级")
	}
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

// 固定绑定优先在测速健康的成员间轮询，跳过已失效成员。
func TestPickVirtualGroupStartProxyPrefersHealthyMembers(t *testing.T) {
	app := NewApp("")
	testedAt := time.Now().Format(time.RFC3339)
	group := browser.ProxyVirtualGroup{
		GroupId:        "vg-healthy",
		GroupName:      "healthy",
		Strategy:       browser.ProxyVirtualGroupStrategyRoundRobin,
		MemberProxyIds: []string{"proxy-dead", "proxy-alive"},
	}
	proxies := []BrowserProxy{
		{ProxyId: "proxy-dead", ProxyConfig: "http://dead:8080", LastTestOk: false, LastTestedAt: testedAt, LastLatencyMs: 0},
		{ProxyId: "proxy-alive", ProxyConfig: "http://alive:8080", LastTestOk: true, LastTestedAt: testedAt, LastLatencyMs: 120},
	}
	for i := 0; i < 3; i++ {
		memberID, _, ok := app.pickVirtualGroupStartProxy(group, proxies, "profile-healthy")
		if !ok || memberID != "proxy-alive" {
			t.Fatalf("pick #%d = (%q,%v), want proxy-alive（应跳过失效成员）", i, memberID, ok)
		}
	}
}

// 全部成员都无健康记录时，仍退回配置非空成员（可启动，由实例启动校验兜底）。
func TestPickVirtualGroupStartProxyFallsBackWhenNoHealthy(t *testing.T) {
	app := NewApp("")
	group := browser.ProxyVirtualGroup{
		GroupId:        "vg-no-healthy",
		GroupName:      "no-healthy",
		MemberProxyIds: []string{"proxy-a"},
	}
	proxies := []BrowserProxy{{ProxyId: "proxy-a", ProxyConfig: "http://a:8080"}}
	if memberID, _, ok := app.pickVirtualGroupStartProxy(group, proxies, "profile-fallback"); !ok || memberID != "proxy-a" {
		t.Fatalf("无健康记录时应退回普通成员: (%q,%v)", memberID, ok)
	}
}

// 已降级成员不得进入启动候选。
func TestVirtualGroupDegradedMemberExcludedFromStart(t *testing.T) {
	app := newVirtualGroupHealthTestApp(t)
	testedAt := time.Now().Format(time.RFC3339)
	group := browser.ProxyVirtualGroup{
		GroupId:        "vg-degraded",
		GroupName:      "降级排除",
		MemberProxyIds: []string{"proxy-a", "proxy-b"},
	}
	proxies := []BrowserProxy{
		{ProxyId: "proxy-a", ProxyConfig: "http://a:8080", LastTestOk: true, LastTestedAt: testedAt, LastLatencyMs: 80},
		{ProxyId: "proxy-b", ProxyConfig: "http://b:8080", LastTestOk: true, LastTestedAt: testedAt, LastLatencyMs: 90},
	}
	seedMemberState(t, app, group.GroupId, "proxy-a", browser.MemberStateDegraded)

	candidates := app.pickVirtualGroupStartProxies(group, proxies, "profile-degraded")
	if candidates.allDegraded {
		t.Fatal("存在健康成员时不应标记全部降级")
	}
	if len(candidates.ordered) != 1 || candidates.ordered[0].ProxyId != "proxy-b" {
		t.Fatalf("candidates = %#v, want 仅 proxy-b", candidates.ordered)
	}
}

// 全组降级时直接拒绝，不再兜底。
func TestVirtualGroupAllDegradedRejectsStart(t *testing.T) {
	app := newVirtualGroupHealthTestApp(t)
	testedAt := time.Now().Format(time.RFC3339)
	group := browser.ProxyVirtualGroup{
		GroupId:        "vg-all-degraded",
		GroupName:      "全组降级",
		MemberProxyIds: []string{"proxy-a", "proxy-b"},
	}
	proxies := []BrowserProxy{
		{ProxyId: "proxy-a", ProxyConfig: "http://a:8080", LastTestOk: true, LastTestedAt: testedAt, LastLatencyMs: 80},
		{ProxyId: "proxy-b", ProxyConfig: "http://b:8080", LastTestOk: true, LastTestedAt: testedAt, LastLatencyMs: 90},
	}
	seedMemberState(t, app, group.GroupId, "proxy-a", browser.MemberStateDegraded)
	seedMemberState(t, app, group.GroupId, "proxy-b", browser.MemberStateDegraded)

	candidates := app.pickVirtualGroupStartProxies(group, proxies, "profile-all-degraded")
	if !candidates.allDegraded {
		t.Fatal("全组降级必须标记 allDegraded")
	}
	if !candidates.empty() {
		t.Fatalf("全组降级不应返回候选，实际 %#v", candidates.ordered)
	}
	if _, _, ok := app.pickVirtualGroupStartProxy(group, proxies, "profile-all-degraded"); ok {
		t.Fatal("全组降级时 pickVirtualGroupStartProxy 必须返回 false")
	}
}

// sticky 策略：偏好节点健康时优先于当前选中节点（用于恢复后回切）。
func TestVirtualGroupStickyPrefersPreferredMember(t *testing.T) {
	app := newVirtualGroupHealthTestApp(t)
	testedAt := time.Now().Format(time.RFC3339)
	group := browser.ProxyVirtualGroup{
		GroupId:          "vg-preferred",
		GroupName:        "偏好回切",
		Strategy:         browser.ProxyVirtualGroupStrategySticky,
		SelectedProxyId:  "proxy-a",
		PreferredProxyId: "proxy-b",
		MemberProxyIds:   []string{"proxy-a", "proxy-b"},
	}
	proxies := []BrowserProxy{
		{ProxyId: "proxy-a", ProxyConfig: "http://a:8080", LastTestOk: true, LastTestedAt: testedAt, LastLatencyMs: 80},
		{ProxyId: "proxy-b", ProxyConfig: "http://b:8080", LastTestOk: true, LastTestedAt: testedAt, LastLatencyMs: 90},
	}
	seedMemberState(t, app, group.GroupId, "proxy-a", browser.MemberStateHealthy)
	seedMemberState(t, app, group.GroupId, "proxy-b", browser.MemberStateHealthy)

	candidates := app.pickVirtualGroupStartProxies(group, proxies, "profile-preferred")
	if len(candidates.ordered) == 0 || candidates.ordered[0].ProxyId != "proxy-b" {
		t.Fatalf("sticky 首选 = %#v, want proxy-b（偏好节点优先）", candidates.ordered)
	}
}

// 动态代理候选池排除已降级成员。
func TestSelectVirtualDynamicCandidatesExcludesDegraded(t *testing.T) {
	testedAt := time.Now().Format(time.RFC3339)
	proxies := []BrowserProxy{
		{ProxyId: "proxy-a", ProxyConfig: "http://a:8080", LastTestOk: true, LastTestedAt: testedAt, LastLatencyMs: 80},
		{ProxyId: "proxy-b", ProxyConfig: "http://b:8080", LastTestOk: true, LastTestedAt: testedAt, LastLatencyMs: 90},
	}
	group := browser.ProxyVirtualGroup{MemberProxyIds: []string{"proxy-a", "proxy-b"}}
	states := map[string]browser.ProxyVirtualGroupMemberState{
		"proxy-a": {ProxyId: "proxy-a", State: browser.MemberStateDegraded},
	}

	candidates, allDegraded := NewApp("").selectVirtualDynamicBrowserProxyCandidates(proxies, group, defaultDynamicProxyMaxTestAge, states)
	if allDegraded {
		t.Fatal("仍有健康成员时不应标记全部降级")
	}
	if len(candidates) != 1 || candidates[0].ProxyId != "proxy-b" {
		t.Fatalf("candidates = %#v, want 仅 proxy-b", candidates)
	}
}

// 全组降级时动态代理直接拒绝，不再兜底。
func TestSelectVirtualDynamicCandidatesRejectsAllDegraded(t *testing.T) {
	testedAt := time.Now().Format(time.RFC3339)
	proxies := []BrowserProxy{
		{ProxyId: "proxy-a", ProxyConfig: "http://a:8080", LastTestOk: true, LastTestedAt: testedAt, LastLatencyMs: 80},
		{ProxyId: "proxy-b", ProxyConfig: "http://b:8080", LastTestOk: true, LastTestedAt: testedAt, LastLatencyMs: 90},
	}
	group := browser.ProxyVirtualGroup{MemberProxyIds: []string{"proxy-a", "proxy-b"}}
	states := map[string]browser.ProxyVirtualGroupMemberState{
		"proxy-a": {ProxyId: "proxy-a", State: browser.MemberStateDegraded},
		"proxy-b": {ProxyId: "proxy-b", State: browser.MemberStateDegraded},
	}

	candidates, allDegraded := NewApp("").selectVirtualDynamicBrowserProxyCandidates(proxies, group, defaultDynamicProxyMaxTestAge, states)
	if !allDegraded {
		t.Fatal("全组降级必须标记 allDegraded")
	}
	if len(candidates) != 0 {
		t.Fatalf("全组降级不应返回候选，实际 %d 个", len(candidates))
	}
}

// 切换连接栈后，其他栈留下的健康状态不得被采信。
func TestVirtualGroupStateIgnoredAcrossConnectorStack(t *testing.T) {
	app := newVirtualGroupHealthTestApp(t)
	group := browser.ProxyVirtualGroup{
		GroupId:        "vg-cross-stack",
		GroupName:      "跨栈失效",
		MemberProxyIds: []string{"proxy-a"},
	}
	candidate := BrowserProxy{ProxyId: "proxy-a", ProxyConfig: "http://a:8080", LastTestOk: false, LastLatencyMs: -1}

	upsert := func(connectorType string) {
		t.Helper()
		if err := app.browserMgr.ProxyVirtualGroupMemberStateDAO.Upsert(browser.ProxyVirtualGroupMemberState{
			GroupId:           group.GroupId,
			ProxyId:           "proxy-a",
			State:             browser.MemberStateHealthy,
			LastConnectorType: connectorType,
		}); err != nil {
			t.Fatalf("seed state failed: %v", err)
		}
	}

	upsert("mihomo")
	if app.virtualGroupMemberUsable(candidate, app.virtualGroupMemberStates(group.GroupId)) {
		t.Fatal("其他连接栈留下的健康状态不应被采信")
	}

	upsert("xray")
	if !app.virtualGroupMemberUsable(candidate, app.virtualGroupMemberStates(group.GroupId)) {
		t.Fatal("当前连接栈下的健康状态应被采信")
	}
}


