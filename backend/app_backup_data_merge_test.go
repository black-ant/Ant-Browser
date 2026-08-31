package backend

import (
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
proxy_bind_source_id, proxy_bind_source_url, proxy_bind_name, proxy_bind_updated_at,
memory_limit_mb, launch_args, tags, keywords, group_id, created_at, updated_at,
restore_last_session, deleted_at
) VALUES (
'profile-source', 'Source Profile', 'C:\\old\\profiles\\profile-source', 'core-source', '["--flag"]',
'proxy-source', 'socks5://127.0.0.1:1080', 'source-1', 'https://example.com/proxies', 'Source Proxy',
'2026-08-24T00:03:00Z', 512, '["--headless"]', '["tag"]', '["keyword"]', 'group-source',
'2026-08-24T00:00:00Z', '2026-08-24T00:04:00Z', 'never', '2026-08-24T00:05:00Z'
)`); err != nil {
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

	var bindSourceID, bindSourceURL, bindName, bindUpdatedAt, groupID, restoreLastSession, deletedAt string
	var memoryLimit int
	if err := targetDB.GetConn().QueryRow(`SELECT proxy_bind_source_id, proxy_bind_source_url, proxy_bind_name, proxy_bind_updated_at,
memory_limit_mb, group_id, restore_last_session, deleted_at
FROM browser_profiles WHERE profile_id = 'profile-source'`).Scan(
		&bindSourceID, &bindSourceURL, &bindName, &bindUpdatedAt, &memoryLimit, &groupID, &restoreLastSession, &deletedAt,
	); err != nil {
		t.Fatal(err)
	}
	if bindSourceID != "source-1" || bindSourceURL != "https://example.com/proxies" || bindName != "Source Proxy" || bindUpdatedAt != "2026-08-24T00:03:00Z" || memoryLimit != 512 || groupID != "group-source" || restoreLastSession != "never" || deletedAt != "2026-08-24T00:05:00Z" {
		t.Fatalf("profile extended fields were not preserved: sourceID=%q sourceURL=%q name=%q updated=%q memory=%d group=%q restore=%q deleted=%q", bindSourceID, bindSourceURL, bindName, bindUpdatedAt, memoryLimit, groupID, restoreLastSession, deletedAt)
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
