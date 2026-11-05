package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/database"
	"path/filepath"
	"testing"
)

func newConfigOnlyVirtualGroupApp(appRoot string) *App {
	cfg := &config.Config{}
	app := NewApp(appRoot)
	app.config = cfg
	app.browserMgr = browser.NewManager(cfg, "")
	return app
}

func TestBrowserProxyVirtualGroupListResolvesConfigLegacyMembers(t *testing.T) {
	app := newConfigOnlyVirtualGroupApp(t.TempDir())
	app.config.Browser.Proxies = []config.BrowserProxy{
		{ProxyId: "mobile-1", ProxyName: "Mobile 1", ProxyConfig: "http://mobile:8080", GroupName: "mobile"},
	}
	app.config.Browser.ProxyVirtualGroups = []config.BrowserProxyVirtualGroup{
		{GroupName: "all-lines", MemberGroupNames: []string{"mobile"}},
	}

	groups := app.BrowserProxyVirtualGroupList()
	if len(groups) != 1 {
		t.Fatalf("virtual group count = %d, want 1", len(groups))
	}
	if len(groups[0].MemberProxyIds) != 1 || groups[0].MemberProxyIds[0] != "mobile-1" {
		t.Fatalf("resolved member proxy IDs = %#v, want [mobile-1]", groups[0].MemberProxyIds)
	}
}

func TestSaveBrowserProxyVirtualGroupValidatesMembers(t *testing.T) {
	app := newConfigOnlyVirtualGroupApp(t.TempDir())
	app.config.Browser.Proxies = []config.BrowserProxy{
		{ProxyId: "__direct__", ProxyName: "Direct", ProxyConfig: "direct://"},
		{ProxyId: "proxy-1", ProxyName: "Proxy 1", ProxyConfig: "http://proxy:8080"},
	}

	if err := app.SaveBrowserProxyVirtualGroup(BrowserProxyVirtualGroup{
		GroupName:      "invalid",
		MemberProxyIds: []string{"missing"},
	}); err == nil {
		t.Fatal("saving a virtual group with a missing proxy should fail")
	}
	if err := app.SaveBrowserProxyVirtualGroup(BrowserProxyVirtualGroup{
		GroupName:      "direct",
		MemberProxyIds: []string{"__direct__"},
	}); err == nil {
		t.Fatal("saving a virtual group with the direct node should fail")
	}
	if err := app.SaveBrowserProxyVirtualGroup(BrowserProxyVirtualGroup{
		GroupName:      "valid",
		MemberProxyIds: []string{" PROXY-1 "},
	}); err != nil {
		t.Fatalf("saving a valid virtual group returned error: %v", err)
	}
	groups := app.BrowserProxyVirtualGroupList()
	if len(groups) != 1 || len(groups[0].MemberProxyIds) != 1 || groups[0].MemberProxyIds[0] != "proxy-1" {
		t.Fatalf("saved virtual group = %#v, want canonical proxy-1 member", groups)
	}
}

func TestSaveBrowserProxyVirtualGroupPreservesDynamicMemberGroups(t *testing.T) {
	app := newConfigOnlyVirtualGroupApp(t.TempDir())
	app.config.Browser.Proxies = []config.BrowserProxy{
		{ProxyId: "mobile-1", ProxyName: "Mobile 1", ProxyConfig: "http://mobile-1:8080", GroupName: "mobile"},
	}

	if err := app.SaveBrowserProxyVirtualGroup(BrowserProxyVirtualGroup{
		GroupId:          "pool-1",
		GroupName:        "mobile-pool",
		MemberProxyIds:   []string{"mobile-1"},
		MemberGroupNames: []string{"mobile"},
	}); err != nil {
		t.Fatalf("saving dynamic virtual group returned error: %v", err)
	}

	app.config.Browser.Proxies = append(app.config.Browser.Proxies, config.BrowserProxy{
		ProxyId: "mobile-2", ProxyName: "Mobile 2", ProxyConfig: "http://mobile-2:8080", GroupName: "mobile",
	})
	groups := app.BrowserProxyVirtualGroupList()
	if len(groups) != 1 || len(groups[0].MemberGroupNames) != 1 || groups[0].MemberGroupNames[0] != "mobile" {
		t.Fatalf("dynamic member groups were not preserved: %#v", groups)
	}
	expanded := browser.ExpandProxyVirtualGroup(groups[0], app.config.Browser.Proxies)
	if len(expanded) != 2 || expanded[0].ProxyId != "mobile-1" || expanded[1].ProxyId != "mobile-2" {
		t.Fatalf("dynamic virtual group members = %#v, want [mobile-1 mobile-2]", expanded)
	}
}

func TestSaveBrowserProxiesPrunesDeletedVirtualGroupMembers(t *testing.T) {
	app := newConfigOnlyVirtualGroupApp(t.TempDir())
	app.config.Browser.Proxies = []config.BrowserProxy{
		{ProxyId: "proxy-1", ProxyName: "Proxy 1", ProxyConfig: "http://proxy:8080"},
	}
	app.config.Browser.ProxyVirtualGroups = []config.BrowserProxyVirtualGroup{
		{GroupId: "pool-1", GroupName: "pool", MemberProxyIds: []string{"proxy-1"}},
	}

	if err := app.SaveBrowserProxies(nil); err != nil {
		t.Fatalf("SaveBrowserProxies returned error: %v", err)
	}
	groups := app.BrowserProxyVirtualGroupList()
	if len(groups) != 1 || len(groups[0].MemberProxyIds) != 0 {
		t.Fatalf("virtual group members after proxy deletion = %#v, want empty", groups)
	}
}

func TestProxyVirtualGroupCursorKeyUsesNameWhenIDMissing(t *testing.T) {
	app := NewApp("")
	candidates := []BrowserProxy{{ProxyId: "proxy-1"}, {ProxyId: "proxy-2"}}
	firstKey := proxyVirtualGroupCursorKey("", "pool-a")
	secondKey := proxyVirtualGroupCursorKey("", "pool-b")

	_, firstStart := app.rotateVirtualProxyCandidates(firstKey, candidates, nil)
	app.commitVirtualProxyCursor(firstKey, "proxy-1", nil, candidates)
	_, secondStart := app.rotateVirtualProxyCandidates(secondKey, candidates, nil)
	if firstStart != 0 || secondStart != 0 {
		t.Fatalf("different unnamed virtual groups shared a cursor: first=%d second=%d", firstStart, secondStart)
	}

	_, nextStart := app.rotateVirtualProxyCandidates(firstKey, candidates, nil)
	if nextStart != 1 {
		t.Fatalf("named fallback cursor start = %d, want 1", nextStart)
	}
}

func TestMigrateToSQLiteAddsConfigVirtualGroupsWhenDAOHasExistingGroups(t *testing.T) {
	db, err := database.NewDB(filepath.Join(t.TempDir(), "virtual-group-migration.db"))
	if err != nil {
		t.Fatalf("NewDB returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}
	if _, err := db.GetConn().Exec(`
		INSERT INTO browser_proxy_virtual_groups (group_id, group_name, member_proxy_ids)
		VALUES ('existing-id', 'Existing Pool', '["proxy-1"]')`); err != nil {
		t.Fatalf("insert existing virtual group failed: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Browser.ProxyVirtualGroups = []config.BrowserProxyVirtualGroup{
		{GroupId: "new-id", GroupName: "Imported Pool", MemberProxyIds: []string{"proxy-2"}},
	}
	manager := browser.NewManager(cfg, "")
	manager.CoreDAO = browser.NewSQLiteCoreDAO(db.GetConn())
	manager.ProxyDAO = browser.NewSQLiteProxyDAO(db.GetConn())
	manager.ProxyVirtualGroupDAO = browser.NewSQLiteProxyVirtualGroupDAO(db.GetConn())
	manager.ProfileDAO = browser.NewSQLiteProfileDAO(db.GetConn())
	manager.BookmarkDAO = browser.NewSQLiteBookmarkDAO(db.GetConn())

	app := NewApp(t.TempDir())
	app.config = cfg
	app.browserMgr = manager
	app.migrateToSQLite()

	groups, err := manager.ProxyVirtualGroupDAO.List()
	if err != nil {
		t.Fatalf("list migrated virtual groups failed: %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("virtual group count = %d, want 2", len(groups))
	}
	var imported bool
	for _, group := range groups {
		if group.GroupName == "Imported Pool" && len(group.MemberProxyIds) == 1 && group.MemberProxyIds[0] == "proxy-2" {
			imported = true
			break
		}
	}
	if !imported {
		t.Fatalf("config virtual group was not incrementally migrated: %#v", groups)
	}
}
