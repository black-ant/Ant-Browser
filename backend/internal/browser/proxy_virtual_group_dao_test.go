package browser

import (
	"ant-chrome/backend/internal/database"
	"path/filepath"
	"testing"
)

func TestSQLiteProxyVirtualGroupDAOPersistsOrderedMembers(t *testing.T) {
	db, err := database.NewDB(filepath.Join(t.TempDir(), "virtual-groups.db"))
	if err != nil {
		t.Fatalf("NewDB returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}

	dao := NewSQLiteProxyVirtualGroupDAO(db.GetConn())
	if err := dao.Upsert(ProxyVirtualGroup{
		GroupId:         "virtual-1",
		GroupName:       "轮询线路",
		Strategy:        ProxyVirtualGroupStrategySticky,
		SelectedProxyId: "proxy-2",
		MemberProxyIds:  []string{"proxy-2", "proxy-1", "proxy-2"},
	}); err != nil {
		t.Fatalf("Upsert returned error: %v", err)
	}

	stored, err := dao.GetByName("  轮询线路 ")
	if err != nil {
		t.Fatalf("GetByName returned error: %v", err)
	}
	if stored.Strategy != ProxyVirtualGroupStrategySticky || stored.SelectedProxyId != "proxy-2" {
		t.Fatalf("strategy state = %q/%q, want sticky/proxy-2", stored.Strategy, stored.SelectedProxyId)
	}
	want := []string{"proxy-2", "proxy-1"}
	if len(stored.MemberProxyIds) != len(want) {
		t.Fatalf("member proxy IDs = %#v, want %#v", stored.MemberProxyIds, want)
	}
	for index, name := range want {
		if stored.MemberProxyIds[index] != name {
			t.Fatalf("member proxy ID[%d] = %q, want %q", index, stored.MemberProxyIds[index], name)
		}
	}
}

func TestSQLiteProxyVirtualGroupDAOHandlesNullJSON(t *testing.T) {
	db, err := database.NewDB(filepath.Join(t.TempDir(), "virtual-groups-null.db"))
	if err != nil {
		t.Fatalf("NewDB returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}
	if _, err := db.GetConn().Exec(`DROP TABLE browser_proxy_virtual_groups`); err != nil {
		t.Fatalf("drop virtual groups table failed: %v", err)
	}
	if _, err := db.GetConn().Exec(`
		CREATE TABLE browser_proxy_virtual_groups (
			group_id TEXT,
			group_name TEXT,
			strategy TEXT,
			selected_proxy_id TEXT,
			preferred_proxy_id TEXT,
			member_proxy_ids TEXT,
			member_group_names TEXT,
			sort_order INTEGER,
			created_at TEXT,
			updated_at TEXT
		)`); err != nil {
		t.Fatalf("create nullable virtual groups table failed: %v", err)
	}
	if _, err := db.GetConn().Exec(`
		INSERT INTO browser_proxy_virtual_groups (group_id, group_name, member_proxy_ids, member_group_names)
		VALUES ('nullable-group', 'Nullable Pool', NULL, NULL)`); err != nil {
		t.Fatalf("insert nullable virtual group failed: %v", err)
	}

	groups, err := NewSQLiteProxyVirtualGroupDAO(db.GetConn()).List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(groups) != 1 || groups[0].GroupName != "Nullable Pool" || len(groups[0].MemberProxyIds) != 0 {
		t.Fatalf("nullable virtual group = %#v, want empty member lists", groups)
	}
}

func TestSQLiteProxyVirtualGroupDAOKeepsDynamicGroupMembers(t *testing.T) {
	db, err := database.NewDB(filepath.Join(t.TempDir(), "dynamic-virtual-groups.db"))
	if err != nil {
		t.Fatalf("NewDB returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}
	if _, err := db.GetConn().Exec(`
		INSERT INTO browser_proxies (proxy_id, proxy_name, proxy_config, group_name, sort_order)
		VALUES ('mobile-1', 'Mobile 1', 'http://mobile-1:8080', 'mobile', 1)`); err != nil {
		t.Fatalf("insert initial proxy failed: %v", err)
	}

	dao := NewSQLiteProxyVirtualGroupDAO(db.GetConn())
	if err := dao.Upsert(ProxyVirtualGroup{
		GroupId:          "dynamic-1",
		GroupName:        "Mobile Pool",
		MemberProxyIds:   []string{"mobile-1"},
		MemberGroupNames: []string{"mobile"},
	}); err != nil {
		t.Fatalf("Upsert returned error: %v", err)
	}
	if _, err := db.GetConn().Exec(`
		INSERT INTO browser_proxies (proxy_id, proxy_name, proxy_config, group_name, sort_order)
		VALUES ('mobile-2', 'Mobile 2', 'http://mobile-2:8080', 'mobile', 2)`); err != nil {
		t.Fatalf("insert new proxy failed: %v", err)
	}

	stored, err := dao.GetByName("Mobile Pool")
	if err != nil {
		t.Fatalf("GetByName returned error: %v", err)
	}
	if len(stored.MemberGroupNames) != 1 || stored.MemberGroupNames[0] != "mobile" {
		t.Fatalf("dynamic member groups = %#v, want [mobile]", stored.MemberGroupNames)
	}
	expanded := ExpandProxyVirtualGroup(*stored, []Proxy{
		{ProxyId: "mobile-1", GroupName: "mobile"},
		{ProxyId: "mobile-2", GroupName: "mobile"},
	})
	if len(expanded) != 2 || expanded[0].ProxyId != "mobile-1" || expanded[1].ProxyId != "mobile-2" {
		t.Fatalf("dynamic DAO group members = %#v, want [mobile-1 mobile-2]", expanded)
	}
}
