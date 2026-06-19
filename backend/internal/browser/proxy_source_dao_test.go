package browser

import (
	"path/filepath"
	"testing"

	"ant-chrome/backend/internal/database"
)

func newTestSourceDAO(t *testing.T) *SQLiteProxySourceDAO {
	t.Helper()
	db, err := database.NewDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return NewSQLiteProxySourceDAO(db.GetConn())
}

func TestProxySourceDAO_CRUDAndOverrides(t *testing.T) {
	dao := newTestSourceDAO(t)

	// Upsert（新增，默认 importStrategy=merge）
	src := ProxySource{
		SourceID:         "src-1",
		SourceURL:        "https://example.com/sub",
		SourceName:       "example",
		NamePrefix:       "HK",
		AutoRefresh:      true,
		RefreshIntervalM: 30,
	}
	if err := dao.UpsertSource(src); err != nil {
		t.Fatalf("UpsertSource: %v", err)
	}

	got, err := dao.GetSource("src-1")
	if err != nil {
		t.Fatalf("GetSource: %v", err)
	}
	if got.SourceURL != src.SourceURL || !got.AutoRefresh || got.RefreshIntervalM != 30 {
		t.Errorf("GetSource mismatch: %+v", got)
	}
	if got.ImportStrategy != "merge" {
		t.Errorf("default ImportStrategy = %q, want merge", got.ImportStrategy)
	}

	// Upsert（更新同 id）
	src.AutoRefresh = false
	src.SourceName = "renamed"
	if err := dao.UpsertSource(src); err != nil {
		t.Fatalf("UpsertSource update: %v", err)
	}
	got, _ = dao.GetSource("src-1")
	if got.AutoRefresh || got.SourceName != "renamed" {
		t.Errorf("update not applied: %+v", got)
	}

	// UpdateRefreshResult
	if err := dao.UpdateRefreshResult("src-1", "2026-01-01T00:00:00Z", ""); err != nil {
		t.Fatalf("UpdateRefreshResult: %v", err)
	}
	got, _ = dao.GetSource("src-1")
	if got.LastRefreshAt != "2026-01-01T00:00:00Z" {
		t.Errorf("LastRefreshAt = %q", got.LastRefreshAt)
	}

	// List
	list, err := dao.ListSources()
	if err != nil || len(list) != 1 {
		t.Fatalf("ListSources = %d (err %v), want 1", len(list), err)
	}

	// Overrides
	if err := dao.UpsertOverride(ProxySourceOverride{SourceID: "src-1", NodeKey: "node-a", Action: "ignore"}); err != nil {
		t.Fatalf("UpsertOverride: %v", err)
	}
	if err := dao.UpsertOverride(ProxySourceOverride{SourceID: "src-1", NodeKey: "node-b", Action: "rename", CustomName: "我的节点"}); err != nil {
		t.Fatalf("UpsertOverride rename: %v", err)
	}
	ovs, err := dao.ListOverrides("src-1")
	if err != nil || len(ovs) != 2 {
		t.Fatalf("ListOverrides = %d (err %v), want 2", len(ovs), err)
	}
	if err := dao.DeleteOverride("src-1", "node-a"); err != nil {
		t.Fatalf("DeleteOverride: %v", err)
	}
	ovs, _ = dao.ListOverrides("src-1")
	if len(ovs) != 1 {
		t.Errorf("after delete ListOverrides = %d, want 1", len(ovs))
	}

	// DeleteSource 同时清理 overrides
	if err := dao.DeleteSource("src-1"); err != nil {
		t.Fatalf("DeleteSource: %v", err)
	}
	if list, _ := dao.ListSources(); len(list) != 0 {
		t.Errorf("after DeleteSource ListSources = %d, want 0", len(list))
	}
	if ovs, _ := dao.ListOverrides("src-1"); len(ovs) != 0 {
		t.Errorf("after DeleteSource ListOverrides = %d, want 0", len(ovs))
	}

	// GetSource 不存在
	if _, err := dao.GetSource("nope"); err == nil {
		t.Error("GetSource(nope) expected error")
	}
}
