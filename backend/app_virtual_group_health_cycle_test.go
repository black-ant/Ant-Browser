package backend

import (
	"testing"
	"time"

	"ant-chrome/backend/internal/browser"
)

type proxySpeedProvenanceUpdater interface {
	UpdateSpeedResultWithProvenance(proxyID string, ok bool, latencyMs int64, testedAt string, engine string, connectorType string, fingerprint string) error
}

func seedHealthyProxy(t *testing.T, app *App, proxyID string) {
	t.Helper()
	updater, ok := app.browserMgr.ProxyDAO.(proxySpeedProvenanceUpdater)
	if !ok {
		t.Fatal("ProxyDAO 不支持写入测速来源")
	}
	if err := updater.UpdateSpeedResultWithProvenance(proxyID, true, 120, time.Now().Format(time.RFC3339), "xray", "xray", ""); err != nil {
		t.Fatalf("seed proxy %s speed result failed: %v", proxyID, err)
	}
}

func probeMember(app *App, proxyID string, ok bool) {
	outcome := browser.MemberProbeOutcome{
		Ok:            ok,
		LatencyMs:     100,
		Engine:        "xray",
		ConnectorType: "xray",
		At:            time.Now(),
	}
	if !ok {
		outcome.LatencyMs = -1
		outcome.Error = "probe timeout"
	}
	app.recordVirtualGroupMemberProbe(proxyID, outcome)
}

func newStickyHealthGroup(t *testing.T) (*App, browser.ProxyVirtualGroup) {
	t.Helper()
	app := newVirtualGroupHealthTestApp(t)
	if err := app.browserMgr.ProxyDAO.ReplaceAll([]BrowserProxy{
		{ProxyId: "proxy-a", ProxyName: "A", ProxyConfig: "http://a:8080"},
		{ProxyId: "proxy-b", ProxyName: "B", ProxyConfig: "http://b:8080"},
	}); err != nil {
		t.Fatalf("seed proxies failed: %v", err)
	}
	seedHealthyProxy(t, app, "proxy-a")
	seedHealthyProxy(t, app, "proxy-b")

	group := browser.ProxyVirtualGroup{
		GroupId:          "vg-cycle",
		GroupName:        "生命周期",
		Strategy:         browser.ProxyVirtualGroupStrategySticky,
		SelectedProxyId:  "proxy-a",
		PreferredProxyId: "proxy-a",
		MemberProxyIds:   []string{"proxy-a", "proxy-b"},
	}
	if err := app.browserMgr.ProxyVirtualGroupDAO.Upsert(group); err != nil {
		t.Fatalf("seed group failed: %v", err)
	}
	app.invalidateVirtualGroupSnapshot()
	return app, group
}

func currentStartMember(t *testing.T, app *App, group browser.ProxyVirtualGroup) virtualGroupStartCandidates {
	t.Helper()
	return app.pickVirtualGroupStartProxies(group, app.getLatestProxies(), "profile-cycle")
}

// 完整链路：失败 → 降级 → 切走 → 恢复 → 升回 → 回切。
func TestVirtualGroupHealthFullCycle(t *testing.T) {
	app, group := newStickyHealthGroup(t)

	// 1. 初始：偏好节点可用。
	initial := currentStartMember(t, app, group)
	if initial.empty() || initial.ordered[0].ProxyId != "proxy-a" {
		t.Fatalf("初始首选 = %#v, want proxy-a", initial.ordered)
	}
	// 全未观测时不应误报全部降级。
	if initialHealth := app.BrowserProxyVirtualGroupHealthList(); initialHealth[0].AllDegraded {
		t.Fatalf("未观测状态不应标记全部降级: %#v", initialHealth[0])
	}

	// 2. 第一次失败：可疑，仍然参与轮询。
	probeMember(app, "proxy-a", false)
	suspect := currentStartMember(t, app, group)
	if suspect.allDegraded || suspect.empty() || suspect.ordered[0].ProxyId != "proxy-a" {
		t.Fatalf("一次失败后首选 = %#v allDegraded=%v, want 仍为 proxy-a", suspect.ordered, suspect.allDegraded)
	}

	// 3. 第二次失败：降级，移出候选。
	probeMember(app, "proxy-a", false)
	degraded := currentStartMember(t, app, group)
	if degraded.allDegraded {
		t.Fatal("仍有健康成员时不应标记全部降级")
	}
	if degraded.empty() || degraded.ordered[0].ProxyId != "proxy-b" {
		t.Fatalf("降级后首选 = %#v, want proxy-b", degraded.ordered)
	}
	for _, candidate := range degraded.ordered {
		if candidate.ProxyId == "proxy-a" {
			t.Fatal("降级成员不应出现在候选内")
		}
	}

	health := app.BrowserProxyVirtualGroupHealthList()
	if len(health) != 1 {
		t.Fatalf("health 数量 = %d, want 1", len(health))
	}
	if health[0].Degraded != 1 || health[0].Healthy != 0 {
		t.Fatalf("聚合 = %#v, want degraded=1 healthy=0", health[0])
	}

	// 4. 备节点连续成功两次后升为健康。
	probeMember(app, "proxy-b", true)
	probeMember(app, "proxy-b", true)

	// 5. 原节点恢复：第一次进入恢复观察，仍不参与轮询。
	probeMember(app, "proxy-a", true)
	recovering := currentStartMember(t, app, group)
	if recovering.empty() || recovering.ordered[0].ProxyId != "proxy-b" {
		t.Fatalf("恢复观察期首选 = %#v, want proxy-b", recovering.ordered)
	}

	// 6. 第二次成功：升为健康并回切偏好节点。
	probeMember(app, "proxy-a", true)
	restored := currentStartMember(t, app, group)
	if restored.empty() || restored.ordered[0].ProxyId != "proxy-a" {
		t.Fatalf("恢复后首选 = %#v, want proxy-a（回切偏好）", restored.ordered)
	}

	finalHealth := app.BrowserProxyVirtualGroupHealthList()
	if finalHealth[0].Total != 2 || finalHealth[0].Healthy != 2 || finalHealth[0].Degraded != 0 || finalHealth[0].AllDegraded {
		t.Fatalf("最终聚合 = %#v, want total=2 healthy=2 degraded=0 allDegraded=false", finalHealth[0])
	}
	if finalHealth[0].PreferredProxyId != "proxy-a" || finalHealth[0].ActiveProxyId != "proxy-a" {
		t.Fatalf("最终偏好/当前 = %q/%q, want proxy-a/proxy-a",
			finalHealth[0].PreferredProxyId, finalHealth[0].ActiveProxyId)
	}
}

// 全组降级：拒绝启动，不再兜底。
func TestVirtualGroupHealthAllDegradedRejected(t *testing.T) {
	app, group := newStickyHealthGroup(t)
	for i := 0; i < 2; i++ {
		probeMember(app, "proxy-a", false)
		probeMember(app, "proxy-b", false)
	}

	candidates := currentStartMember(t, app, group)
	if !candidates.allDegraded {
		t.Fatal("全组降级应标记 allDegraded")
	}
	if !candidates.empty() {
		t.Fatalf("全组降级不应返回候选，实际 %#v", candidates.ordered)
	}
	if _, _, ok := app.pickVirtualGroupStartProxy(group, app.getLatestProxies(), "profile-cycle"); ok {
		t.Fatal("全组降级时不允许选出成员")
	}

	health := app.BrowserProxyVirtualGroupHealthList()
	if !health[0].AllDegraded || health[0].Degraded != 2 {
		t.Fatalf("聚合 = %#v, want allDegraded=true degraded=2", health[0])
	}
}

// 手动清除降级后应恢复可观测状态。
func TestVirtualGroupHealthResetRestoresCandidates(t *testing.T) {
	app, group := newStickyHealthGroup(t)
	for i := 0; i < 2; i++ {
		probeMember(app, "proxy-a", false)
	}
	if candidates := currentStartMember(t, app, group); candidates.ordered[0].ProxyId != "proxy-b" {
		t.Fatalf("降级后首选 = %#v, want proxy-b", candidates.ordered)
	}

	if err := app.BrowserProxyVirtualGroupResetHealth(group.GroupId); err != nil {
		t.Fatalf("ResetHealth failed: %v", err)
	}
	// 重置后状态为 unknown，回落旧的测速新鲜度判定，偏好节点重新可用。
	candidates := currentStartMember(t, app, group)
	if candidates.empty() || candidates.ordered[0].ProxyId != "proxy-a" {
		t.Fatalf("重置后首选 = %#v, want proxy-a", candidates.ordered)
	}
}

// 手动测速是强信号：一次失败立即降级、一次成功立即恢复。
func TestManualProbeImmediatelyDegradesAndRestores(t *testing.T) {
	app, group := newStickyHealthGroup(t)

	app.applyManualProxyProbe(ProxyTestResult{ProxyId: "proxy-a", Ok: false, Error: "timeout"})
	candidates := currentStartMember(t, app, group)
	if candidates.allDegraded || candidates.empty() || candidates.ordered[0].ProxyId != "proxy-b" {
		t.Fatalf("手动测速失败后首选 = %#v, want proxy-b（一次失败即降级）", candidates.ordered)
	}

	app.applyManualProxyProbe(ProxyTestResult{ProxyId: "proxy-a", Ok: true, LatencyMs: 120, Engine: "xray"})
	candidates = currentStartMember(t, app, group)
	if candidates.empty() || candidates.ordered[0].ProxyId != "proxy-a" {
		t.Fatalf("手动测速成功后首选 = %#v, want proxy-a（一次成功即恢复）", candidates.ordered)
	}
}

func TestBrowserProxyVirtualGroupProbeRejectsUnknownGroup(t *testing.T) {
	app := newVirtualGroupHealthTestApp(t)
	if _, err := app.BrowserProxyVirtualGroupProbe("missing-group"); err == nil {
		t.Fatal("未知分组应返回错误")
	}
}
