package backend

import (
	"archive/zip"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/database"
)

func TestProfilePackageDatabaseRoundTripSelectedProfile(t *testing.T) {
	sourceRoot := t.TempDir()
	sourceDB := newProfilePackageDatabase(t, sourceRoot)
	sourceCfg := config.DefaultConfig()
	sourceCfg.Browser.UserDataRoot = filepath.Join(sourceRoot, "user-data")
	sourceApp := NewApp(sourceRoot)
	sourceApp.config = sourceCfg
	sourceApp.db = sourceDB
	sourceApp.browserMgr = browser.NewManager(sourceCfg, sourceRoot)

	profileID := "source-profile"
	groupID := "source-group"
	parentGroupID := "source-parent-group"
	coreID := "source-core"
	proxyID := "source-proxy"
	extensionID := "source-extension"
	profile := browser.Profile{
		ProfileId:       profileID,
		ProfileName:     "源实例",
		UserDataDir:     profileID,
		CoreId:          coreID,
		ProxyId:         proxyID,
		ProxyConfig:     "http://source-proxy:8080",
		ProxyBindName:   "源代理",
		GroupId:         groupID,
		FingerprintArgs: []string{"--fingerprint-platform=Windows"},
		LaunchArgs:      []string{"--start-maximized"},
		Tags:            []string{"测试"},
		Keywords:        []string{"源实例"},
		CreatedAt:       "2026-08-31T00:00:00Z",
		UpdatedAt:       "2026-08-31T00:00:00Z",
	}
	insertProfilePackageDatabaseFixtures(t, sourceDB.GetConn(), profile, parentGroupID, extensionID)
	userDataDir := filepath.Join(sourceCfg.Browser.UserDataRoot, profileID)
	if err := os.MkdirAll(userDataDir, 0o755); err != nil {
		t.Fatalf("create source user data failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(userDataDir, "Preferences"), []byte(`{"selected":true}`), 0o644); err != nil {
		t.Fatalf("write source user data failed: %v", err)
	}

	zipPath := filepath.Join(sourceRoot, "selected-profile.zip")
	if _, err := sourceApp.writeProfilePackage(zipPath, []browser.Profile{profile}); err != nil {
		t.Fatalf("writeProfilePackage returned error: %v", err)
	}
	assertProfilePackageDatabaseContents(t, zipPath, profileID, groupID, parentGroupID, coreID, proxyID, extensionID)
	_ = sourceDB.Close()

	targetRoot := t.TempDir()
	targetDB := newProfilePackageDatabase(t, targetRoot)
	targetCfg := config.DefaultConfig()
	targetCfg.Browser.UserDataRoot = filepath.Join(targetRoot, "user-data")
	targetApp := NewApp(targetRoot)
	targetApp.config = targetCfg
	targetApp.db = targetDB
	targetApp.browserMgr = browser.NewManager(targetCfg, targetRoot)
	targetApp.browserMgr.ProfileDAO = browser.NewSQLiteProfileDAO(targetDB.GetConn())
	targetApp.browserMgr.ProxyDAO = browser.NewSQLiteProxyDAO(targetDB.GetConn())
	targetApp.browserMgr.CoreDAO = browser.NewSQLiteCoreDAO(targetDB.GetConn())
	targetApp.browserMgr.GroupDAO = browser.NewSQLiteGroupDAO(targetDB.GetConn())
	targetApp.browserMgr.ExtensionDAO = browser.NewSQLiteExtensionDAO(targetDB.GetConn())
	if _, err := targetDB.GetConn().Exec(`INSERT INTO browser_profiles (profile_id, profile_name, user_data_dir, created_at, updated_at) VALUES ('other-profile', '其他实例', 'other-profile', '2026-08-31T00:00:00Z', '2026-08-31T00:00:00Z')`); err != nil {
		t.Fatalf("insert target profile failed: %v", err)
	}

	result, err := targetApp.importProfilePackageFromPath(zipPath)
	if err != nil {
		t.Fatalf("importProfilePackageFromPath returned error: %v", err)
	}
	if result.ImportedCount != 1 {
		t.Fatalf("imported count = %d, want 1", result.ImportedCount)
	}
	newProfileID := result.ProfileMappings[profileID]
	if newProfileID == "" || newProfileID == profileID {
		t.Fatalf("unexpected profile mapping: %#v", result.ProfileMappings)
	}

	stored, err := browser.NewSQLiteProfileDAO(targetDB.GetConn()).GetById(newProfileID)
	if err != nil {
		t.Fatalf("read imported profile failed: %v", err)
	}
	if stored.GroupId == "" || stored.CoreId == "" || stored.ProxyId == "" {
		t.Fatalf("profile references were lost: %#v", stored)
	}
	if stored.ProfileName != "源实例（导入）" || stored.ProxyConfig != "http://source-proxy:8080" {
		t.Fatalf("imported profile fields lost: %#v", stored)
	}
	if _, err := browser.NewSQLiteProfileDAO(targetDB.GetConn()).GetById("other-profile"); err != nil {
		t.Fatalf("target existing profile was not preserved: %v", err)
	}

	var groupCount, coreCount, proxyCount, extensionCount, bindingCount, runtimeCount int
	for _, query := range []struct {
		name  string
		query string
		count *int
	}{
		{"group", `SELECT COUNT(*) FROM browser_groups WHERE group_name IN ('源分组', '源父分组')`, &groupCount},
		{"core", `SELECT COUNT(*) FROM browser_cores WHERE core_name = '源内核'`, &coreCount},
		{"proxy", `SELECT COUNT(*) FROM browser_proxies WHERE proxy_name = '源代理'`, &proxyCount},
		{"extension", `SELECT COUNT(*) FROM browser_extensions WHERE extension_id = ?`, &extensionCount},
		{"binding", `SELECT COUNT(*) FROM browser_profile_extensions WHERE profile_id = ?`, &bindingCount},
		{"runtime", `SELECT COUNT(*) FROM browser_profile_extension_runtime WHERE profile_id = ?`, &runtimeCount},
	} {
		var err error
		if query.name == "extension" {
			err = targetDB.GetConn().QueryRow(query.query, extensionID).Scan(query.count)
		} else if query.name == "binding" || query.name == "runtime" {
			err = targetDB.GetConn().QueryRow(query.query, newProfileID).Scan(query.count)
		} else {
			err = targetDB.GetConn().QueryRow(query.query).Scan(query.count)
		}
		if err != nil {
			t.Fatalf("count imported %s rows failed: %v", query.name, err)
		}
	}
	if groupCount != 2 || coreCount != 1 || proxyCount != 1 || extensionCount != 1 || bindingCount != 1 || runtimeCount != 1 {
		t.Fatalf("unexpected imported related row counts: groups=%d cores=%d proxies=%d extensions=%d bindings=%d runtime=%d", groupCount, coreCount, proxyCount, extensionCount, bindingCount, runtimeCount)
	}
	if _, err := os.Stat(filepath.Join(targetCfg.Browser.UserDataRoot, newProfileID, "Preferences")); err != nil {
		t.Fatalf("imported user data missing: %v", err)
	}
}

func newProfilePackageDatabase(t *testing.T, root string) *database.DB {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, "data"), 0o755); err != nil {
		t.Fatalf("create database directory failed: %v", err)
	}
	db, err := database.NewDB(filepath.Join(root, "data", "app.db"))
	if err != nil {
		t.Fatalf("create database failed: %v", err)
	}
	if err := db.Migrate(); err != nil {
		_ = db.Close()
		t.Fatalf("migrate database failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func insertProfilePackageDatabaseFixtures(t *testing.T, conn *sql.DB, profile browser.Profile, parentGroupID, extensionID string) {
	t.Helper()
	if _, err := conn.Exec(`INSERT INTO browser_groups (group_id, group_name, parent_id, sort_order, created_at, updated_at) VALUES (?, ?, '', 0, ?, ?)`, parentGroupID, "源父分组", profile.CreatedAt, profile.UpdatedAt); err != nil {
		t.Fatalf("insert parent group failed: %v", err)
	}
	if _, err := conn.Exec(`INSERT INTO browser_groups (group_id, group_name, parent_id, sort_order, created_at, updated_at) VALUES (?, ?, ?, 0, ?, ?)`, profile.GroupId, "源分组", parentGroupID, profile.CreatedAt, profile.UpdatedAt); err != nil {
		t.Fatalf("insert group failed: %v", err)
	}
	if _, err := conn.Exec(`INSERT INTO browser_cores (core_id, core_name, core_path, is_default, sort_order, created_at) VALUES (?, ?, ?, 0, 0, ?)`, profile.CoreId, "源内核", "chrome/source", profile.CreatedAt); err != nil {
		t.Fatalf("insert core failed: %v", err)
	}
	if _, err := conn.Exec(`INSERT INTO browser_proxies (proxy_id, proxy_name, proxy_config, sort_order, created_at) VALUES (?, ?, ?, 0, ?)`, profile.ProxyId, profile.ProxyBindName, profile.ProxyConfig, profile.CreatedAt); err != nil {
		t.Fatalf("insert proxy failed: %v", err)
	}
	if _, err := conn.Exec(`INSERT INTO browser_profiles (profile_id, profile_name, user_data_dir, core_id, fingerprint_args, proxy_id, proxy_config, proxy_bind_name, launch_args, tags, keywords, group_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, profile.ProfileId, profile.ProfileName, profile.UserDataDir, profile.CoreId, `[
"--fingerprint-platform=Windows"
]`, profile.ProxyId, profile.ProxyConfig, profile.ProxyBindName, `["--start-maximized"]`, `["测试"]`, `["源实例"]`, profile.GroupId, profile.CreatedAt, profile.UpdatedAt); err != nil {
		t.Fatalf("insert profile failed: %v", err)
	}
	extensionDAO := browser.NewSQLiteExtensionDAO(conn)
	if err := extensionDAO.Upsert(browser.Extension{ExtensionID: extensionID, Name: "源插件", Version: "1.0.0", ManifestJSON: `{}`, InstallDir: "data/extensions/source-extension", Enabled: true, InstalledAt: profile.CreatedAt, UpdatedAt: profile.UpdatedAt}); err != nil {
		t.Fatalf("insert extension failed: %v", err)
	}
	if _, err := extensionDAO.SetProfileSettings(profile.ProfileId, []string{extensionID}, true); err != nil {
		t.Fatalf("insert extension settings failed: %v", err)
	}
	if err := extensionDAO.UpsertProfileExtensionRuntime(browser.ProfileExtensionRuntime{ProfileID: profile.ProfileId, ExtensionID: extensionID, RuntimeExtensionID: "runtime-source-extension", Status: browser.ExtensionRuntimeStatusInstalled, CreatedAt: profile.CreatedAt, UpdatedAt: profile.UpdatedAt}); err != nil {
		t.Fatalf("insert extension runtime failed: %v", err)
	}
}

func assertProfilePackageDatabaseContents(t *testing.T, zipPath, profileID, groupID, parentGroupID, coreID, proxyID, extensionID string) {
	t.Helper()
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open profile package failed: %v", err)
	}
	defer reader.Close()
	var snapshot ProfilePackageDatabase
	for _, entry := range reader.File {
		if entry.Name != profilePackageDatabasePath {
			continue
		}
		file, err := entry.Open()
		if err != nil {
			t.Fatalf("open database snapshot failed: %v", err)
		}
		if err := json.NewDecoder(file).Decode(&snapshot); err != nil {
			_ = file.Close()
			t.Fatalf("decode database snapshot failed: %v", err)
		}
		_ = file.Close()
	}
	if snapshot.Format != profilePackageDatabaseFormat || len(snapshot.Profiles) != 1 || len(snapshot.Groups) != 2 || len(snapshot.Cores) != 1 || len(snapshot.Proxies) != 1 || len(snapshot.Extensions) != 1 || len(snapshot.ProfileExtensions) != 1 || len(snapshot.ProfileExtensionRuntime) != 1 {
		t.Fatalf("unexpected database snapshot: %#v", snapshot)
	}
	if snapshot.Profiles[0].ProfileId != profileID || snapshot.Groups[0].GroupId == "" || snapshot.Groups[1].GroupId == "" || snapshot.Cores[0].CoreId != coreID || snapshot.Proxies[0].ProxyId != proxyID || snapshot.ProfileExtensions[0].ExtensionID != extensionID || snapshot.Groups[0].GroupId != groupID && snapshot.Groups[1].GroupId != parentGroupID {
		t.Fatalf("database snapshot references are incomplete: %#v", snapshot)
	}
}
