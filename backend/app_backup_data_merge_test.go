package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/database"
	"path/filepath"
	"testing"
)

func TestBackupMergeDatabasePreservesExtendedFieldsAndNormalizesPaths(t *testing.T) {
	root := t.TempDir()
	targetPath := filepath.Join(root, "target.db")
	sourcePath := filepath.Join(root, "source.db")

	targetDB, err := database.NewDB(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	defer targetDB.Close()
	if err := targetDB.Migrate(); err != nil {
		t.Fatal(err)
	}

	sourceDB, err := database.NewDB(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := sourceDB.Migrate(); err != nil {
		sourceDB.Close()
		t.Fatal(err)
	}
	if _, err := sourceDB.GetConn().Exec(`INSERT INTO browser_cores (core_id, core_name, core_path, is_default, sort_order, created_at)
VALUES ('core-source', 'Source Core', 'C:\\old\\chrome\\core-source', 0, 1, '2026-08-24T00:00:00Z')`); err != nil {
		sourceDB.Close()
		t.Fatal(err)
	}
	if _, err := sourceDB.GetConn().Exec(`INSERT INTO browser_proxies (
proxy_id, proxy_name, proxy_config, preferred_kernel, dns_servers, group_name,
source_id, source_url, source_name_prefix, source_auto_refresh, source_refresh_interval_m,
source_last_refresh_at, last_latency_ms, last_test_ok, last_tested_at, last_ip_health_json,
sort_order, created_at
) VALUES (
'proxy-source', 'Source Proxy', 'socks5://127.0.0.1:1080', 'sing-box', '1.1.1.1', 'source-group',
'source-1', 'https://example.com/proxies', 'Source', 1, 30,
'2026-08-24T00:01:00Z', 123, 1, '2026-08-24T00:02:00Z', '{"ok":true}',
2, '2026-08-24T00:00:00Z'
)`); err != nil {
		sourceDB.Close()
		t.Fatal(err)
	}
	if _, err := sourceDB.GetConn().Exec(`INSERT INTO browser_profiles (
profile_id, profile_name, user_data_dir, core_id, fingerprint_args, proxy_id, proxy_config,
proxy_mode, proxy_group_name, proxy_bind_source_id, proxy_bind_source_url, proxy_bind_name, proxy_bind_updated_at,
memory_limit_mb, launch_args, tags, keywords, group_id, created_at, updated_at,
restore_last_session, deleted_at
) VALUES (
'profile-source', 'Source Profile', 'C:\\old\\profiles\\profile-source', 'core-source', '["--flag"]',
'proxy-source', 'socks5://127.0.0.1:1080', 'fixed', '', 'source-1', 'https://example.com/proxies', 'Source Proxy',
'2026-08-24T00:03:00Z', 512, '["--headless"]', '["tag"]', '["keyword"]', 'group-source',
'2026-08-24T00:00:00Z', '2026-08-24T00:04:00Z', 'never', '2026-08-24T00:05:00Z'
)`); err != nil {
		sourceDB.Close()
		t.Fatal(err)
	}
	if _, err := sourceDB.GetConn().Exec(`INSERT INTO browser_profiles (
profile_id, profile_name, user_data_dir, proxy_mode, proxy_group_name, created_at, updated_at
) VALUES ('profile-dynamic-source', 'Dynamic Profile', 'profile-dynamic-source', 'dynamic', 'mobile', '2026-08-24T00:00:00Z', '2026-08-24T00:00:00Z')`); err != nil {
		sourceDB.Close()
		t.Fatal(err)
	}
	if err := sourceDB.Close(); err != nil {
		t.Fatal(err)
	}

	incoming := config.DefaultConfig()
	incoming.Browser.Cores = []config.BrowserCore{{
		CoreId:   "core-source",
		CorePath: filepath.Join("chrome", "external", "core-source"),
	}}
	incoming.Browser.Profiles = []config.BrowserProfileConfig{{
		ProfileId:   "profile-source",
		UserDataDir: "profile-source",
	}, {
		ProfileId:   "profile-dynamic-source",
		UserDataDir: "profile-dynamic-source",
	}}
	app := NewApp(root)
	app.db = targetDB
	stats := &backupMergeStats{}
	if _, err := app.backupMergeDatabaseFromSource(sourcePath, incoming, stats); err != nil {
		t.Fatal(err)
	}

	var preferredKernel, sourceID, sourceURL string
	var latency int64
	var testOK int
	if err := targetDB.GetConn().QueryRow(`SELECT preferred_kernel, source_id, source_url, last_latency_ms, last_test_ok
FROM browser_proxies WHERE proxy_id = 'proxy-source'`).Scan(&preferredKernel, &sourceID, &sourceURL, &latency, &testOK); err != nil {
		t.Fatal(err)
	}
	if preferredKernel != "sing-box" || sourceID != "source-1" || sourceURL != "https://example.com/proxies" || latency != 123 || testOK != 1 {
		t.Fatalf("proxy extended fields were not preserved: kernel=%q sourceID=%q sourceURL=%q latency=%d testOK=%d", preferredKernel, sourceID, sourceURL, latency, testOK)
	}

	var proxyMode, proxyGroupName, bindSourceID, bindSourceURL, bindName, bindUpdatedAt, groupID, restoreLastSession, deletedAt string
	var memoryLimit int
	if err := targetDB.GetConn().QueryRow(`SELECT proxy_mode, proxy_group_name, proxy_bind_source_id, proxy_bind_source_url, proxy_bind_name, proxy_bind_updated_at,
memory_limit_mb, group_id, restore_last_session, deleted_at
FROM browser_profiles WHERE profile_id = 'profile-source'`).Scan(
		&proxyMode, &proxyGroupName, &bindSourceID, &bindSourceURL, &bindName, &bindUpdatedAt, &memoryLimit, &groupID, &restoreLastSession, &deletedAt,
	); err != nil {
		t.Fatal(err)
	}
	if proxyMode != "fixed" || proxyGroupName != "" || bindSourceID != "source-1" || bindSourceURL != "https://example.com/proxies" || bindName != "Source Proxy" || bindUpdatedAt != "2026-08-24T00:03:00Z" || memoryLimit != 512 || groupID != "group-source" || restoreLastSession != "never" || deletedAt != "2026-08-24T00:05:00Z" {
		t.Fatalf("profile extended fields were not preserved: proxyMode=%q proxyGroup=%q sourceID=%q sourceURL=%q name=%q updated=%q memory=%d group=%q restore=%q deleted=%q", proxyMode, proxyGroupName, bindSourceID, bindSourceURL, bindName, bindUpdatedAt, memoryLimit, groupID, restoreLastSession, deletedAt)
	}
	var dynamicMode, dynamicGroup string
	if err := targetDB.GetConn().QueryRow(`SELECT proxy_mode, proxy_group_name FROM browser_profiles WHERE profile_id = 'profile-dynamic-source'`).Scan(&dynamicMode, &dynamicGroup); err != nil {
		t.Fatal(err)
	}
	if dynamicMode != "dynamic" || dynamicGroup != "mobile" {
		t.Fatalf("dynamic proxy selection was not preserved: mode=%q group=%q", dynamicMode, dynamicGroup)
	}

	var corePath, userDataDir string
	if err := targetDB.GetConn().QueryRow(`SELECT core_path FROM browser_cores WHERE core_id = 'core-source'`).Scan(&corePath); err != nil {
		t.Fatal(err)
	}
	if err := targetDB.GetConn().QueryRow(`SELECT user_data_dir FROM browser_profiles WHERE profile_id = 'profile-source'`).Scan(&userDataDir); err != nil {
		t.Fatal(err)
	}
	if corePath != filepath.Join("chrome", "external", "core-source") || userDataDir != "profile-source" {
		t.Fatalf("imported paths were not normalized: core=%q profile=%q", corePath, userDataDir)
	}
}

func TestBackupMergeMapsVirtualGroupMembersAndRenamesConflicts(t *testing.T) {
	root := t.TempDir()
	targetPath := filepath.Join(root, "target.db")
	sourcePath := filepath.Join(root, "source.db")
	targetDB, err := database.NewDB(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	defer targetDB.Close()
	if err := targetDB.Migrate(); err != nil {
		t.Fatal(err)
	}
	sourceDB, err := database.NewDB(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := sourceDB.Migrate(); err != nil {
		sourceDB.Close()
		t.Fatal(err)
	}
	if _, err := sourceDB.GetConn().Exec(`INSERT INTO browser_proxies (proxy_id, proxy_name, proxy_config, sort_order, created_at)
VALUES ('shared-proxy-id', '源代理', 'socks5://source:1080', 0, '2026-08-30T00:00:00Z')`); err != nil {
		sourceDB.Close()
		t.Fatal(err)
	}
	if _, err := sourceDB.GetConn().Exec(`INSERT INTO browser_proxy_virtual_groups (
	group_id, group_name, strategy, selected_proxy_id, member_proxy_ids, member_group_names, sort_order, created_at, updated_at
) VALUES ('source-virtual-group', 'mobile', 'sticky', 'shared-proxy-id', '["shared-proxy-id"]', '[]', 0, '2026-08-30T00:00:00Z', '2026-08-30T00:00:00Z')`); err != nil {
		sourceDB.Close()
		t.Fatal(err)
	}
	if _, err := sourceDB.GetConn().Exec(`INSERT INTO browser_profiles (
	profile_id, profile_name, user_data_dir, proxy_mode, proxy_group_name, created_at, updated_at
) VALUES ('dynamic-source-profile', '动态实例', 'dynamic-source-profile', 'dynamic', 'mobile', '2026-08-30T00:00:00Z', '2026-08-30T00:00:00Z')`); err != nil {
		sourceDB.Close()
		t.Fatal(err)
	}
	if err := sourceDB.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := targetDB.GetConn().Exec(`INSERT INTO browser_proxies (proxy_id, proxy_name, proxy_config, sort_order, created_at)
VALUES ('shared-proxy-id', '本地代理', 'socks5://local:1080', 0, '2026-08-30T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if _, err := targetDB.GetConn().Exec(`INSERT INTO browser_proxy_virtual_groups (
	group_id, group_name, strategy, selected_proxy_id, member_proxy_ids, member_group_names, sort_order, created_at, updated_at
) VALUES ('target-local-group', 'mobile', 'round_robin', '', '["local-proxy"]', '[]', 0, '2026-08-30T00:00:00Z', '2026-08-30T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}

	incoming := config.DefaultConfig()
	incoming.Browser.Profiles = []config.BrowserProfileConfig{{
		ProfileId: "dynamic-source-profile", UserDataDir: "dynamic-source-profile",
		ProxyMode: "dynamic", ProxyGroupName: "mobile",
	}}
	incoming.Browser.ProxyVirtualGroups = []config.BrowserProxyVirtualGroup{{
		GroupId: "source-virtual-group", GroupName: "mobile",
		SelectedProxyId: "shared-proxy-id", MemberProxyIds: []string{"shared-proxy-id"},
	}}
	app := NewApp(root)
	app.db = targetDB
	stats := &backupMergeStats{}
	mappings, err := app.backupMergeDatabaseFromSource(sourcePath, incoming, stats)
	if err != nil {
		t.Fatal(err)
	}
	backupApplyImportReferenceMappings(incoming, mappings)

	groupName := mappings.ProxyVirtualGroups[backupImportTextKey("mobile")].TargetID
	proxyID := mappings.Proxies[backupImportIDKey("shared-proxy-id")].TargetID
	if groupName != "mobile (导入)" || proxyID == "" || proxyID == "shared-proxy-id" {
		t.Fatalf("unexpected imported mappings: group=%q proxy=%q", groupName, proxyID)
	}
	var localMembers string
	if err := targetDB.GetConn().QueryRow(`SELECT member_proxy_ids FROM browser_proxy_virtual_groups WHERE group_name = 'mobile'`).Scan(&localMembers); err != nil {
		t.Fatal(err)
	}
	if localMembers != `["local-proxy"]` {
		t.Fatalf("local virtual group was modified: %s", localMembers)
	}
	var importedMembers, importedSelectedProxyID string
	if err := targetDB.GetConn().QueryRow(`SELECT member_proxy_ids, selected_proxy_id FROM browser_proxy_virtual_groups WHERE group_name = ?`, groupName).Scan(&importedMembers, &importedSelectedProxyID); err != nil {
		t.Fatal(err)
	}
	if importedMembers != `["`+proxyID+`"]` || importedSelectedProxyID != proxyID {
		t.Fatalf("imported virtual group refs = (%s, %q), want mapped proxy %q", importedMembers, importedSelectedProxyID, proxyID)
	}
	var storedProfileGroup string
	if err := targetDB.GetConn().QueryRow(`SELECT proxy_group_name FROM browser_profiles WHERE profile_id = 'dynamic-source-profile'`).Scan(&storedProfileGroup); err != nil {
		t.Fatal(err)
	}
	if storedProfileGroup != groupName || incoming.Browser.Profiles[0].ProxyGroupName != groupName || incoming.Browser.ProxyVirtualGroups[0].GroupName != groupName {
		t.Fatalf("virtual group references were not remapped: db=%q configProfile=%q configGroup=%q want=%q", storedProfileGroup, incoming.Browser.Profiles[0].ProxyGroupName, incoming.Browser.ProxyVirtualGroups[0].GroupName, groupName)
	}
	if len(incoming.Browser.ProxyVirtualGroups[0].MemberProxyIds) != 1 || incoming.Browser.ProxyVirtualGroups[0].MemberProxyIds[0] != proxyID || incoming.Browser.ProxyVirtualGroups[0].SelectedProxyId != proxyID {
		t.Fatalf("config virtual group proxy refs were not remapped: %#v", incoming.Browser.ProxyVirtualGroups[0])
	}
	if stats.Conflicts == 0 {
		t.Fatal("expected same-name virtual group conflict to be reported")
	}
}

func TestBackupMergeDatabaseDetachesSourceConnection(t *testing.T) {
	root := t.TempDir()
	targetPath := filepath.Join(root, "target.db")
	sourcePath := filepath.Join(root, "source.db")

	targetDB, err := database.NewDB(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	defer targetDB.Close()
	if err := targetDB.Migrate(); err != nil {
		t.Fatal(err)
	}

	sourceDB, err := database.NewDB(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := sourceDB.Migrate(); err != nil {
		sourceDB.Close()
		t.Fatal(err)
	}
	if _, err := sourceDB.GetConn().Exec(`INSERT INTO browser_cores (core_id, core_name, core_path)
VALUES ('core-source', 'Source Core', 'chrome/external/core-source')`); err != nil {
		sourceDB.Close()
		t.Fatal(err)
	}
	if err := sourceDB.Close(); err != nil {
		t.Fatal(err)
	}

	app := NewApp(root)
	app.db = targetDB
	if _, err := app.backupMergeDatabaseFromSource(sourcePath, nil, &backupMergeStats{}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.backupMergeDatabaseFromSource(sourcePath, nil, &backupMergeStats{}); err != nil {
		t.Fatalf("second database merge failed after source detach: %v", err)
	}
}

func TestBackupMergeDatabaseDoesNotRestoreDeletedBookmarks(t *testing.T) {
	root := t.TempDir()
	targetDB, err := database.NewDB(filepath.Join(root, "target.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer targetDB.Close()
	if err := targetDB.Migrate(); err != nil {
		t.Fatal(err)
	}

	targetBookmarks := browser.NewSQLiteBookmarkDAO(targetDB.GetConn())
	deletedURL := "https://ping0.cc/"
	if err := targetBookmarks.ReplaceAll([]config.BrowserBookmark{{Name: "Ping0", URL: deletedURL}}); err != nil {
		t.Fatal(err)
	}
	if err := targetBookmarks.ReplaceAll(nil); err != nil {
		t.Fatal(err)
	}

	sourcePath := filepath.Join(root, "old-backup.db")
	sourceDB, err := database.NewDB(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := sourceDB.Migrate(); err != nil {
		sourceDB.Close()
		t.Fatal(err)
	}
	if _, err := sourceDB.GetConn().Exec(`INSERT INTO browser_bookmarks (name, url, sort_order) VALUES
		('Ping0', ?, 0), ('New', 'https://new.example/', 1)`, deletedURL); err != nil {
		sourceDB.Close()
		t.Fatal(err)
	}
	if err := sourceDB.Close(); err != nil {
		t.Fatal(err)
	}

	app := NewApp(root)
	app.db = targetDB
	for attempt := 0; attempt < 2; attempt++ {
		if _, err := app.backupMergeDatabaseFromSource(sourcePath, nil, &backupMergeStats{}); err != nil {
			t.Fatalf("backup merge attempt %d failed: %v", attempt+1, err)
		}
	}

	var deletedCount, newCount int
	if err := targetDB.GetConn().QueryRow(`SELECT COUNT(*) FROM browser_bookmarks WHERE lower(trim(url)) = lower(?)`, deletedURL).Scan(&deletedCount); err != nil {
		t.Fatal(err)
	}
	if err := targetDB.GetConn().QueryRow(`SELECT COUNT(*) FROM browser_bookmarks WHERE url = 'https://new.example/'`).Scan(&newCount); err != nil {
		t.Fatal(err)
	}
	if deletedCount != 0 || newCount != 1 {
		t.Fatalf("bookmark counts after repeated import: deleted=%d new=%d, want deleted=0 new=1", deletedCount, newCount)
	}
	deletedURLs, err := targetBookmarks.DeletedURLs()
	if err != nil {
		t.Fatal(err)
	}
	if len(deletedURLs) != 1 || deletedURLs[0] != deletedURL {
		t.Fatalf("deletion markers after import = %#v, want %q to remain authoritative", deletedURLs, deletedURL)
	}
}

func TestBackupMergeDatabaseHonorsConfigDeletionMarkers(t *testing.T) {
	root := t.TempDir()
	targetDB, err := database.NewDB(filepath.Join(root, "target.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer targetDB.Close()
	if err := targetDB.Migrate(); err != nil {
		t.Fatal(err)
	}

	sourcePath := filepath.Join(root, "source.db")
	sourceDB, err := database.NewDB(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := sourceDB.Migrate(); err != nil {
		sourceDB.Close()
		t.Fatal(err)
	}
	if _, err := sourceDB.GetConn().Exec(`INSERT INTO browser_bookmarks (name, url, sort_order) VALUES
		('Ping0', 'https://ping0.cc/', 0), ('New', 'https://new.example/', 1)`); err != nil {
		sourceDB.Close()
		t.Fatal(err)
	}
	if err := sourceDB.Close(); err != nil {
		t.Fatal(err)
	}

	app := NewApp(root)
	app.db = targetDB
	app.config = config.DefaultConfig()
	app.config.Browser.DeletedBookmarkURLs = []string{"https://ping0.cc/"}
	for attempt := 0; attempt < 2; attempt++ {
		if _, err := app.backupMergeDatabaseFromSource(sourcePath, nil, &backupMergeStats{}); err != nil {
			t.Fatalf("backup merge attempt %d failed: %v", attempt+1, err)
		}
	}

	var deletedCount, newCount int
	if err := targetDB.GetConn().QueryRow(`SELECT COUNT(*) FROM browser_bookmarks WHERE url = 'https://ping0.cc/'`).Scan(&deletedCount); err != nil {
		t.Fatal(err)
	}
	if err := targetDB.GetConn().QueryRow(`SELECT COUNT(*) FROM browser_bookmarks WHERE url = 'https://new.example/'`).Scan(&newCount); err != nil {
		t.Fatal(err)
	}
	if deletedCount != 0 || newCount != 1 {
		t.Fatalf("bookmark counts after repeated config-marked import: deleted=%d new=%d, want deleted=0 new=1", deletedCount, newCount)
	}
}

func TestBackupMergeDatabaseUsesBookmarkURLKeyRules(t *testing.T) {
	root := t.TempDir()
	targetDB, err := database.NewDB(filepath.Join(root, "target.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer targetDB.Close()
	if err := targetDB.Migrate(); err != nil {
		t.Fatal(err)
	}
	targetBookmarks := browser.NewSQLiteBookmarkDAO(targetDB.GetConn())
	if err := targetBookmarks.ReplaceAll([]config.BrowserBookmark{{
		Name: "Deleted",
		URL:  "https://EXAMPLE.com/Path?Query=One",
	}}); err != nil {
		t.Fatal(err)
	}
	if err := targetBookmarks.ReplaceAll(nil); err != nil {
		t.Fatal(err)
	}

	sourcePath := filepath.Join(root, "source.db")
	sourceDB, err := database.NewDB(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := sourceDB.Migrate(); err != nil {
		sourceDB.Close()
		t.Fatal(err)
	}
	if _, err := sourceDB.GetConn().Exec(`INSERT INTO browser_bookmarks (name, url, sort_order) VALUES
		('Same host/path/query', 'https://example.com/Path?Query=One', 0),
		('Different path', 'https://example.com/path?Query=One', 1),
		('Different query', 'https://example.com/Path?query=One', 2)`); err != nil {
		sourceDB.Close()
		t.Fatal(err)
	}
	if err := sourceDB.Close(); err != nil {
		t.Fatal(err)
	}

	app := NewApp(root)
	app.db = targetDB
	for attempt := 0; attempt < 2; attempt++ {
		if _, err := app.backupMergeDatabaseFromSource(sourcePath, nil, &backupMergeStats{}); err != nil {
			t.Fatalf("backup merge attempt %d failed: %v", attempt+1, err)
		}
	}

	bookmarks, err := targetBookmarks.List()
	if err != nil {
		t.Fatal(err)
	}
	if containsBookmarkURL(bookmarks, "https://example.com/Path?Query=One") {
		t.Fatalf("deleted bookmark was imported: %#v", bookmarks)
	}
	if countBookmarkURL(bookmarks, "https://example.com/path?Query=One") != 1 {
		t.Fatalf("different path bookmark count = %d, want 1: %#v", countBookmarkURL(bookmarks, "https://example.com/path?Query=One"), bookmarks)
	}
	if countBookmarkURL(bookmarks, "https://example.com/Path?query=One") != 1 {
		t.Fatalf("different query bookmark count = %d, want 1: %#v", countBookmarkURL(bookmarks, "https://example.com/Path?query=One"), bookmarks)
	}
	deletedURLs, err := targetBookmarks.DeletedURLs()
	if err != nil {
		t.Fatal(err)
	}
	if len(deletedURLs) != 1 || deletedURLs[0] != "https://example.com/Path?Query=One" {
		t.Fatalf("deletion markers = %#v, want the same host/path/query URL", deletedURLs)
	}
}

func countBookmarkURL(bookmarks []config.BrowserBookmark, expectedURL string) int {
	count := 0
	for _, bookmark := range bookmarks {
		if browser.NormalizeBookmarkURL(bookmark.URL) == browser.NormalizeBookmarkURL(expectedURL) {
			count++
		}
	}
	return count
}

func TestBackupMergeDatabaseSupportsLegacyProxyAndProfileTables(t *testing.T) {
	root := t.TempDir()
	targetPath := filepath.Join(root, "target.db")
	sourcePath := filepath.Join(root, "legacy-source.db")

	targetDB, err := database.NewDB(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	defer targetDB.Close()
	if err := targetDB.Migrate(); err != nil {
		t.Fatal(err)
	}

	sourceDB, err := database.NewDB(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := sourceDB.Migrate(); err != nil {
		sourceDB.Close()
		t.Fatal(err)
	}
	legacyStatements := []string{
		`DROP TABLE browser_proxies`,
		`CREATE TABLE browser_proxies (
proxy_id TEXT PRIMARY KEY,
proxy_name TEXT NOT NULL,
proxy_config TEXT NOT NULL,
dns_servers TEXT NOT NULL DEFAULT '',
sort_order INTEGER NOT NULL DEFAULT 0,
created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
)`,
		`DROP TABLE browser_profiles`,
		`CREATE TABLE browser_profiles (
profile_id TEXT PRIMARY KEY,
profile_name TEXT NOT NULL,
user_data_dir TEXT NOT NULL DEFAULT '',
core_id TEXT NOT NULL DEFAULT '',
fingerprint_args TEXT NOT NULL DEFAULT '[]',
proxy_id TEXT NOT NULL DEFAULT '',
proxy_config TEXT NOT NULL DEFAULT '',
launch_args TEXT NOT NULL DEFAULT '[]',
tags TEXT NOT NULL DEFAULT '[]',
keywords TEXT NOT NULL DEFAULT '[]',
created_at DATETIME NOT NULL,
updated_at DATETIME NOT NULL
)`,
		`INSERT INTO browser_proxies (proxy_id, proxy_name, proxy_config) VALUES ('legacy-proxy', 'Legacy Proxy', 'http://127.0.0.1:8080')`,
		`INSERT INTO browser_profiles (profile_id, profile_name, user_data_dir, created_at, updated_at) VALUES ('legacy-profile', 'Legacy Profile', 'legacy-profile', '2026-08-24T00:00:00Z', '2026-08-24T00:00:00Z')`,
	}
	for _, statement := range legacyStatements {
		if _, err := sourceDB.GetConn().Exec(statement); err != nil {
			sourceDB.Close()
			t.Fatalf("legacy source setup failed: %v", err)
		}
	}
	if err := sourceDB.Close(); err != nil {
		t.Fatal(err)
	}

	app := NewApp(root)
	app.db = targetDB
	if _, err := app.backupMergeDatabaseFromSource(sourcePath, nil, &backupMergeStats{}); err != nil {
		t.Fatal(err)
	}

	var preferredKernel, bindSourceID string
	var memoryLimit int
	if err := targetDB.GetConn().QueryRow(`SELECT preferred_kernel FROM browser_proxies WHERE proxy_id = 'legacy-proxy'`).Scan(&preferredKernel); err != nil {
		t.Fatal(err)
	}
	if err := targetDB.GetConn().QueryRow(`SELECT proxy_bind_source_id, memory_limit_mb FROM browser_profiles WHERE profile_id = 'legacy-profile'`).Scan(&bindSourceID, &memoryLimit); err != nil {
		t.Fatal(err)
	}
	if preferredKernel != "" || bindSourceID != "" || memoryLimit != 0 {
		t.Fatalf("legacy defaults were not applied: kernel=%q bindSourceID=%q memory=%d", preferredKernel, bindSourceID, memoryLimit)
	}
}
