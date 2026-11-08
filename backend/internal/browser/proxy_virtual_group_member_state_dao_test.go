package browser

import (
	"path/filepath"
	"testing"

	"ant-chrome/backend/internal/database"
)

func newMemberStateTestDAO(t *testing.T) *SQLiteProxyVirtualGroupMemberStateDAO {
	t.Helper()
	db, err := database.NewDB(filepath.Join(t.TempDir(), "member-state.db"))
	if err != nil {
		t.Fatalf("NewDB returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}
	return NewSQLiteProxyVirtualGroupMemberStateDAO(db.GetConn())
}

func TestSQLiteProxyVirtualGroupMemberStateUpsertAndList(t *testing.T) {
	dao := newMemberStateTestDAO(t)

	state := ProxyVirtualGroupMemberState{
		GroupId:             "group-1",
		ProxyId:             "proxy-1",
		State:               MemberStateHealthy,
		ConsecutiveFailures: 0,
		LastProbeAt:         "2026-10-28T12:00:00Z",
		LastSuccessAt:       "2026-10-28T12:00:00Z",
		LastEngine:          "xray",
		LastConnectorType:   "xray",
	}
	if err := dao.Upsert(state); err != nil {
		t.Fatalf("Upsert returned error: %v", err)
	}

	// 重复写入应更新而不是插入新行。
	state.State = MemberStateDegraded
	state.ConsecutiveFailures = 2
	state.LastError = "timeout"
	if err := dao.Upsert(state); err != nil {
		t.Fatalf("second Upsert returned error: %v", err)
	}

	states, err := dao.ListByGroup("group-1")
	if err != nil {
		t.Fatalf("ListByGroup returned error: %v", err)
	}
	if len(states) != 1 {
		t.Fatalf("state count = %d, want 1", len(states))
	}
	if states[0].State != MemberStateDegraded || states[0].ConsecutiveFailures != 2 || states[0].LastError != "timeout" {
		t.Fatalf("stored state = %#v", states[0])
	}
	if states[0].LastConnectorType != "xray" {
		t.Fatalf("connector type = %q, want xray", states[0].LastConnectorType)
	}

	all, err := dao.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("List count = %d, want 1", len(all))
	}
}

func TestSQLiteProxyVirtualGroupMemberStateResetAndDelete(t *testing.T) {
	dao := newMemberStateTestDAO(t)

	for _, proxyID := range []string{"proxy-1", "proxy-2"} {
		if err := dao.Upsert(ProxyVirtualGroupMemberState{
			GroupId:             "group-1",
			ProxyId:             proxyID,
			State:               MemberStateDegraded,
			ConsecutiveFailures: 3,
			LastError:           "refused",
			NextProbeAt:         "2026-10-28T12:05:00Z",
		}); err != nil {
			t.Fatalf("Upsert %s returned error: %v", proxyID, err)
		}
	}

	if err := dao.ResetByGroup("group-1"); err != nil {
		t.Fatalf("ResetByGroup returned error: %v", err)
	}
	states, err := dao.ListByGroup("group-1")
	if err != nil {
		t.Fatalf("ListByGroup returned error: %v", err)
	}
	for _, item := range states {
		if item.State != MemberStateUnknown || item.ConsecutiveFailures != 0 || item.NextProbeAt != "" {
			t.Fatalf("state after reset = %#v", item)
		}
	}

	if err := dao.DeleteByGroup("group-1"); err != nil {
		t.Fatalf("DeleteByGroup returned error: %v", err)
	}
	states, err = dao.ListByGroup("group-1")
	if err != nil {
		t.Fatalf("ListByGroup after delete returned error: %v", err)
	}
	if len(states) != 0 {
		t.Fatalf("state count after delete = %d, want 0", len(states))
	}
}

func TestSQLiteProxyVirtualGroupMemberStateRejectsMissingIDs(t *testing.T) {
	dao := newMemberStateTestDAO(t)
	if err := dao.Upsert(ProxyVirtualGroupMemberState{GroupId: "group-1"}); err == nil {
		t.Fatal("Upsert without proxy id should fail")
	}
}

// v29 迁移必须同时补齐 preferred_proxy_id 列。
func TestVirtualGroupPreferredProxyIDPersists(t *testing.T) {
	db, err := database.NewDB(filepath.Join(t.TempDir(), "preferred.db"))
	if err != nil {
		t.Fatalf("NewDB returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}

	dao := NewSQLiteProxyVirtualGroupDAO(db.GetConn())
	if err := dao.Upsert(ProxyVirtualGroup{
		GroupId:          "group-1",
		GroupName:        "主线路",
		Strategy:         ProxyVirtualGroupStrategySticky,
		SelectedProxyId:  "proxy-2",
		PreferredProxyId: "proxy-1",
		MemberProxyIds:   []string{"proxy-1", "proxy-2"},
	}); err != nil {
		t.Fatalf("Upsert returned error: %v", err)
	}

	stored, err := dao.GetByName("主线路")
	if err != nil {
		t.Fatalf("GetByName returned error: %v", err)
	}
	if stored.PreferredProxyId != "proxy-1" || stored.SelectedProxyId != "proxy-2" {
		t.Fatalf("stored preferred/selected = %q/%q, want proxy-1/proxy-2", stored.PreferredProxyId, stored.SelectedProxyId)
	}
}

func TestSQLiteProxyVirtualGroupMemberStateDeleteByGroupExcept(t *testing.T) {
	dao := newMemberStateTestDAO(t)

	for _, proxyID := range []string{"proxy-1", "proxy-2", "proxy-3"} {
		if err := dao.Upsert(ProxyVirtualGroupMemberState{
			GroupId: "group-1",
			ProxyId: proxyID,
			State:   MemberStateHealthy,
		}); err != nil {
			t.Fatalf("seed %s failed: %v", proxyID, err)
		}
	}
	if err := dao.Upsert(ProxyVirtualGroupMemberState{
		GroupId: "group-2",
		ProxyId: "proxy-3",
		State:   MemberStateDegraded,
	}); err != nil {
		t.Fatalf("seed other group failed: %v", err)
	}

	// 大小写与重复项都要归一化，且只影响指定分组。
	if err := dao.DeleteByGroupExcept("group-1", []string{" PROXY-1 ", "proxy-2", "proxy-2"}); err != nil {
		t.Fatalf("DeleteByGroupExcept returned error: %v", err)
	}
	states, err := dao.ListByGroup("group-1")
	if err != nil {
		t.Fatalf("ListByGroup returned error: %v", err)
	}
	if len(states) != 2 || states[0].ProxyId != "proxy-1" || states[1].ProxyId != "proxy-2" {
		t.Fatalf("states after prune = %#v, want proxy-1/proxy-2", states)
	}
	other, err := dao.ListByGroup("group-2")
	if err != nil {
		t.Fatalf("ListByGroup(group-2) returned error: %v", err)
	}
	if len(other) != 1 || other[0].State != MemberStateDegraded {
		t.Fatalf("other group states = %#v, want untouched degraded row", other)
	}

	// 保留列表为空表示清空该分组（成员全部被移除）。
	if err := dao.DeleteByGroupExcept("group-1", nil); err != nil {
		t.Fatalf("DeleteByGroupExcept(nil) returned error: %v", err)
	}
	states, err = dao.ListByGroup("group-1")
	if err != nil {
		t.Fatalf("ListByGroup after clear returned error: %v", err)
	}
	if len(states) != 0 {
		t.Fatalf("states after clear = %#v, want empty", states)
	}
}
