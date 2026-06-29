package backup

import (
	"ant-chrome/backend/internal/config"
	"path/filepath"
	"testing"
	"time"
)

func TestBuildScope_DefaultConfigKeepsCoreEntries(t *testing.T) {
	tempDir := t.TempDir()
	cfg := config.DefaultConfig()

	scope, err := BuildScope(BuildOptions{
		AppRoot: tempDir,
		Config:  cfg,
	})
	if err != nil {
		t.Fatalf("BuildScope 返回错误: %v", err)
	}

	if scope.Format != PackageFormat {
		t.Fatalf("format 不正确: %s", scope.Format)
	}
	if scope.ManifestVersion != ManifestVersion {
		t.Fatalf("manifestVersion 不正确: %d", scope.ManifestVersion)
	}

	ids := make(map[string]ScopeEntry)
	for _, e := range scope.Entries {
		ids[e.ID] = e
	}

	assertEntry(t, ids, "system_config_main")
	assertEntry(t, ids, "system_config_proxies")
	assertEntry(t, ids, "app_data_root")
	assertEntry(t, ids, "browser_core_root")

	if _, ok := ids["database_sqlite_main"]; ok {
		t.Fatalf("默认配置下 database_sqlite_main 应被 app_data_root 覆盖，不应单独出现")
	}
	if _, ok := ids["browser_user_data_root"]; ok {
		t.Fatalf("默认配置下 browser_user_data_root 与 app_data_root 重合，不应重复出现")
	}
}

func TestBuildScope_CustomPathsIncludeNonOverlappingEntries(t *testing.T) {
	tempDir := t.TempDir()
	cfg := config.DefaultConfig()

	cfg.Browser.UserDataRoot = "profiles"
	cfg.Database.SQLite.Path = "db/main.db"
	cfg.Logging.FilePath = "runtime/logs/app.log"
	cfg.Browser.Cores = []config.BrowserCore{
		{
			CoreId:   "core-external-a",
			CoreName: "External Core A",
			CorePath: "external-core-a",
		},
	}

	scope, err := BuildScope(BuildOptions{
		AppRoot: tempDir,
		Config:  cfg,
	})
	if err != nil {
		t.Fatalf("BuildScope 返回错误: %v", err)
	}

	ids := make(map[string]ScopeEntry)
	for _, e := range scope.Entries {
		ids[e.ID] = e
	}

	assertEntry(t, ids, "browser_user_data_root")
	assertEntry(t, ids, "database_sqlite_main")
	assertEntry(t, ids, "database_sqlite_wal")
	assertEntry(t, ids, "database_sqlite_shm")
	assertEntry(t, ids, "logs_root")
	assertEntry(t, ids, "browser_core_external_external-01")

	dbEntry := ids["database_sqlite_main"]
	expectedDB := filepath.Join(tempDir, "db", "main.db")
	if dbEntry.SourcePath != expectedDB {
		t.Fatalf("database source path 不匹配: got=%s want=%s", dbEntry.SourcePath, expectedDB)
	}
}

func TestBuildManifest_StripsSourcePath(t *testing.T) {
	tempDir := t.TempDir()
	scope, err := BuildScope(BuildOptions{
		AppRoot: tempDir,
		Config:  config.DefaultConfig(),
	})
	if err != nil {
		t.Fatalf("BuildScope 返回错误: %v", err)
	}

	at := time.Date(2026, 3, 2, 12, 0, 0, 0, time.UTC)
	manifest := BuildManifest(scope, "Maka Browser", "1.1.0", at)

	if manifest.CreatedAt != "2026-03-02T12:00:00Z" {
		t.Fatalf("CreatedAt 不匹配: %s", manifest.CreatedAt)
	}
	if manifest.App.Name != "Maka Browser" {
		t.Fatalf("manifest app name 不正确: %s", manifest.App.Name)
	}
	if manifest.App.Version != "1.1.0" {
		t.Fatalf("manifest app version 不正确: %s", manifest.App.Version)
	}

	for _, item := range manifest.Entries {
		if item.ArchivePath == "" {
			t.Fatalf("manifest entry 缺少 archivePath: %+v", item)
		}
	}
}

func assertEntry(t *testing.T, entries map[string]ScopeEntry, id string) {
	t.Helper()
	if _, ok := entries[id]; !ok {
		t.Fatalf("缺少 scope entry: %s", id)
	}
}
