package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/database"
	"path/filepath"
	"testing"
)

func TestBackupReconcileProxyReferencesWithDatabaseImportsAndRemapsProxies(t *testing.T) {
	root := t.TempDir()
	db, err := database.NewDB(filepath.Join(root, "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	if _, err := db.GetConn().Exec(`
		INSERT INTO browser_proxies (proxy_id, proxy_name, proxy_config)
		VALUES ('target-proxy', 'Target Proxy', 'http://same:8080')`); err != nil {
		t.Fatal(err)
	}

	cfg := config.DefaultConfig()
	cfg.Browser.Proxies = []config.BrowserProxy{
		{ProxyId: "source-proxy", ProxyName: "Source Proxy", ProxyConfig: "http://same:8080"},
		{ProxyId: "new-proxy", ProxyName: "New Proxy", ProxyConfig: "socks5://new:1080"},
	}
	cfg.Browser.Profiles = []config.BrowserProfileConfig{{
		ProfileId: "profile-1",
		ProxyId:   "source-proxy",
	}}
	cfg.Browser.ProxyVirtualGroups = []config.BrowserProxyVirtualGroup{{
		GroupId:         "virtual-1",
		GroupName:       "mixed-pool",
		SelectedProxyId: "source-proxy",
		MemberProxyIds:  []string{"source-proxy", "new-proxy"},
	}}

	app := NewApp(root)
	app.config = cfg
	app.browserMgr = browser.NewManager(cfg, root)
	app.browserMgr.ProxyDAO = browser.NewSQLiteProxyDAO(db.GetConn())
	if err := app.backupReconcileProxyReferencesWithDatabase(cfg.Browser.Proxies); err != nil {
		t.Fatal(err)
	}

	proxies, err := app.browserMgr.ProxyDAO.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(proxies) != 2 {
		t.Fatalf("proxy count = %d, want 2", len(proxies))
	}
	var newProxyFound bool
	for _, item := range proxies {
		if item.ProxyId == "new-proxy" {
			newProxyFound = true
		}
		if item.ProxyConfig == "http://same:8080" && item.ProxyId != "target-proxy" {
			t.Fatalf("same proxy config was duplicated with ID %q", item.ProxyId)
		}
	}
	if !newProxyFound {
		t.Fatal("proxy from the imported config was not added to the database")
	}
	if got := cfg.Browser.Profiles[0].ProxyId; got != "target-proxy" {
		t.Fatalf("profile proxy ID = %q, want target-proxy", got)
	}
	group := cfg.Browser.ProxyVirtualGroups[0]
	if len(group.MemberProxyIds) != 2 || group.MemberProxyIds[0] != "target-proxy" || group.MemberProxyIds[1] != "new-proxy" || group.SelectedProxyId != "target-proxy" {
		t.Fatalf("virtual group proxy references = %#v, selected %q", group.MemberProxyIds, group.SelectedProxyId)
	}
}
