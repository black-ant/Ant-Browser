package database

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 存量旧库升级：迁移前备份、数据保留、新版本表补齐，且不触发重建。
func TestOpenWithRecoveryUpgradesLegacyDatabase(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "database.db")

	legacy, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB returned error: %v", err)
	}
	if _, err := legacy.GetConn().Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    INTEGER PRIMARY KEY,
			desc       TEXT NOT NULL DEFAULT '',
			applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`); err != nil {
		t.Fatalf("create schema_migrations returned error: %v", err)
	}
	for _, migration := range migrations {
		if migration.version > 17 {
			break
		}
		if err := legacy.applyMigration(migration); err != nil {
			t.Fatalf("apply migration %d returned error: %v", migration.version, err)
		}
	}
	if _, err := legacy.GetConn().Exec(`
		INSERT INTO browser_profiles
			(profile_id, profile_name, user_data_dir, core_id, fingerprint_args, proxy_id, proxy_config, launch_args, tags, keywords, created_at, updated_at)
		VALUES
			('legacy-profile', '旧实例', '', '', '[]', '', '', '[]', '[]', '[]', datetime('now'), datetime('now'))`); err != nil {
		t.Fatalf("insert legacy profile returned error: %v", err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	db, info, err := OpenWithRecovery(dbPath, filepath.Join(dir, "db-backups"))
	if err != nil {
		t.Fatalf("OpenWithRecovery returned error: %v", err)
	}
	defer db.Close()
	if info.Recovered {
		t.Fatalf("旧库升级不应触发重建: %+v", info)
	}
	if info.MigrationBackup == "" {
		t.Fatal("存量库存在待执行迁移时应创建升级前备份")
	}
	if _, err := os.Stat(info.MigrationBackup); err != nil {
		t.Fatalf("升级前备份不存在: %v", err)
	}

	var tableCount int
	if err := db.GetConn().QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'browser_builtin_extension_superseded'`,
	).Scan(&tableCount); err != nil {
		t.Fatalf("查询新表失败: %v", err)
	}
	if tableCount != 1 {
		t.Fatal("升级后应存在内置插件取代标记表")
	}

	var profileName string
	if err := db.GetConn().QueryRow(
		`SELECT profile_name FROM browser_profiles WHERE profile_id = 'legacy-profile'`,
	).Scan(&profileName); err != nil {
		t.Fatalf("旧数据应保留: %v", err)
	}
	if profileName != "旧实例" {
		t.Fatalf("旧数据内容错误: %q", profileName)
	}
}

// 损坏库：自动备份 → 移走原文件 → 重建空库，应用可正常启动。
func TestOpenWithRecoveryRebuildsCorruptDatabase(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "database.db")
	garbage := make([]byte, 4096)
	copy(garbage, []byte("this is definitely not a sqlite database file"))
	if err := os.WriteFile(dbPath, garbage, 0o644); err != nil {
		t.Fatalf("写入损坏库失败: %v", err)
	}

	db, info, err := OpenWithRecovery(dbPath, filepath.Join(dir, "db-backups"))
	if err != nil {
		t.Fatalf("损坏库应自动恢复，实际报错: %v", err)
	}
	defer db.Close()
	if !info.Recovered {
		t.Fatalf("损坏库应标记为已重建: %+v", info)
	}
	if info.BrokenBackupPath == "" {
		t.Fatal("应保留损坏库备份")
	}
	backed, readErr := os.ReadFile(info.BrokenBackupPath)
	if readErr != nil || len(backed) != len(garbage) {
		t.Fatalf("损坏库备份内容异常: len=%d err=%v", len(backed), readErr)
	}
	if info.BrokenParkedPath == "" {
		t.Fatal("应保留损坏库现场文件")
	}
	if _, statErr := os.Stat(info.BrokenParkedPath); statErr != nil {
		t.Fatalf("损坏库现场文件不存在: %v", statErr)
	}

	var currentVersion int
	if err := db.GetConn().QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&currentVersion); err != nil {
		t.Fatalf("重建库不可用: %v", err)
	}
	latest := migrations[len(migrations)-1].version
	if currentVersion != latest {
		t.Fatalf("重建库版本 = %d, want %d", currentVersion, latest)
	}
	if _, err := db.GetConn().Exec(
		`INSERT INTO browser_proxies (proxy_id, proxy_name, proxy_config) VALUES ('probe', '直连', 'direct://')`,
	); err != nil {
		t.Fatalf("重建库写入失败: %v", err)
	}
}

// 另一个实例持有写锁时：不能重建数据库（会毁掉对方数据），必须原样报错。
func TestOpenWithRecoveryRefusesRebuildWhenDatabaseLocked(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "database.db")

	seed, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB returned error: %v", err)
	}
	if _, err := seed.GetConn().Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    INTEGER PRIMARY KEY,
			desc       TEXT NOT NULL DEFAULT '',
			applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`); err != nil {
		t.Fatalf("create schema_migrations returned error: %v", err)
	}
	for _, migration := range migrations {
		if migration.version > 17 {
			break
		}
		if err := seed.applyMigration(migration); err != nil {
			t.Fatalf("apply migration %d returned error: %v", migration.version, err)
		}
	}
	conn := seed.GetConn()
	if _, err := conn.Exec(`BEGIN IMMEDIATE`); err != nil {
		t.Fatalf("持有写锁失败: %v", err)
	}
	defer func() {
		_, _ = conn.Exec(`ROLLBACK`)
		_ = seed.Close()
	}()

	db, info, err := OpenWithRecovery(dbPath, filepath.Join(dir, "db-backups"))
	if err == nil {
		if db != nil {
			_ = db.Close()
		}
		t.Fatal("库被占用时应返回错误，而不是重建")
	}
	if info.Recovered {
		t.Fatalf("库被占用时不应触发重建: %+v", info)
	}
	if !strings.Contains(err.Error(), "占用") {
		t.Fatalf("错误信息应说明库被占用: %v", err)
	}
	if _, statErr := os.Stat(dbPath); statErr != nil {
		t.Fatalf("原库不应被移走: %v", statErr)
	}
}

// 库来自更高版本（用户装回旧版）：版本号超前不算错误，必须照常启动且不重建。
func TestOpenWithRecoveryHandlesDatabaseFromNewerVersion(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "database.db")

	seed, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB returned error: %v", err)
	}
	if err := seed.Migrate(); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}
	if _, err := seed.GetConn().Exec(
		`INSERT INTO schema_migrations (version, desc) VALUES (999, 'future-version')`,
	); err != nil {
		t.Fatalf("写入未来版本号失败: %v", err)
	}
	if err := seed.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	db, info, err := OpenWithRecovery(dbPath, filepath.Join(dir, "db-backups"))
	if err != nil {
		t.Fatalf("更高版本库应可启动，实际报错: %v", err)
	}
	defer db.Close()
	if info.Recovered {
		t.Fatalf("更高版本库不应触发重建: %+v", info)
	}
	var count int
	if err := db.GetConn().QueryRow(`SELECT COUNT(*) FROM browser_profiles`).Scan(&count); err != nil {
		t.Fatalf("更高版本库应可读: %v", err)
	}
}

// 用真实用户库副本做升级验证（默认跳过）：
// ANT_DB_UPGRADE_PROBE=<app.db 路径> go test ./internal/database -run TestOpenWithRecoveryOnRealUserDatabase -v
func TestOpenWithRecoveryOnRealUserDatabase(t *testing.T) {
	sourcePath := strings.TrimSpace(os.Getenv("ANT_DB_UPGRADE_PROBE"))
	if sourcePath == "" {
		t.Skip("未设置 ANT_DB_UPGRADE_PROBE，跳过真实库升级验证")
	}
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "app.db")
	for _, suffix := range []string{"", "-wal", "-shm"} {
		data, err := os.ReadFile(sourcePath + suffix)
		if err != nil {
			if suffix == "" {
				t.Fatalf("读取真实库失败: %v", err)
			}
			continue
		}
		if err := os.WriteFile(dbPath+suffix, data, 0o644); err != nil {
			t.Fatalf("复制真实库失败: %v", err)
		}
	}

	db, info, err := OpenWithRecovery(dbPath, filepath.Join(dir, "db-backups"))
	if err != nil {
		t.Fatalf("真实库升级失败: %v", err)
	}
	defer db.Close()
	if info.Recovered {
		t.Fatalf("真实库不应触发重建: %+v", info)
	}
	for _, table := range []string{"browser_profiles", "browser_proxies", "browser_extensions", "browser_cores", "browser_builtin_extension_superseded"} {
		var count int
		if err := db.GetConn().QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&count); err != nil {
			t.Fatalf("%s 不可读: %v", table, err)
		}
		t.Logf("%s rows=%d", table, count)
	}
}

// 全新安装：不产生升级前备份，正常建库。
func TestOpenWithRecoveryFreshDatabase(t *testing.T) {
	dir := t.TempDir()
	db, info, err := OpenWithRecovery(filepath.Join(dir, "database.db"), filepath.Join(dir, "db-backups"))
	if err != nil {
		t.Fatalf("OpenWithRecovery returned error: %v", err)
	}
	defer db.Close()
	if info.Recovered {
		t.Fatalf("全新安装不应触发重建: %+v", info)
	}
	if info.MigrationBackup != "" {
		t.Fatalf("全新安装不应产生升级前备份: %s", info.MigrationBackup)
	}
	var currentVersion int
	if err := db.GetConn().QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&currentVersion); err != nil {
		t.Fatalf("新库不可用: %v", err)
	}
	if currentVersion != migrations[len(migrations)-1].version {
		t.Fatalf("新库版本 = %d, want %d", currentVersion, migrations[len(migrations)-1].version)
	}
}
