package database

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestMigrateClearsLegacyPluginDefaultInstallFlags(t *testing.T) {
	db, err := NewDB(filepath.Join(t.TempDir(), "database.db"))
	if err != nil {
		t.Fatalf("NewDB returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := db.GetConn().Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    INTEGER PRIMARY KEY,
			desc       TEXT NOT NULL DEFAULT '',
			applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`); err != nil {
		t.Fatalf("create schema_migrations returned error: %v", err)
	}
	for _, migration := range migrations {
		if migration.version > 16 {
			break
		}
		if err := db.applyMigration(migration); err != nil {
			t.Fatalf("apply migration %d returned error: %v", migration.version, err)
		}
	}
	if _, err := db.GetConn().Exec(`
		INSERT INTO browser_extensions (extension_id, name, install_dir, enabled, default_install)
		VALUES (?, ?, ?, 1, 1)`, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "legacy", "legacy"); err != nil {
		t.Fatalf("insert legacy extension returned error: %v", err)
	}

	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}

	var defaultInstall int
	if err := db.GetConn().QueryRow(`
		SELECT default_install FROM browser_extensions WHERE extension_id = ?`, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa").Scan(&defaultInstall); err != nil {
		t.Fatalf("query default_install returned error: %v", err)
	}
	if defaultInstall != 0 {
		t.Fatalf("default_install = %d, want 0 after legacy cleanup", defaultInstall)
	}
}

func TestMigrateDefaultsNewPluginsToManualInstall(t *testing.T) {
	db, err := NewDB(filepath.Join(t.TempDir(), "database.db"))
	if err != nil {
		t.Fatalf("NewDB returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}

	if _, err := db.GetConn().Exec(`
		INSERT INTO browser_extensions (extension_id, name, install_dir, enabled)
		VALUES (?, ?, ?, 1)`, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "new", "new"); err != nil {
		t.Fatalf("insert new extension returned error: %v", err)
	}

	var defaultInstall int
	if err := db.GetConn().QueryRow(`
		SELECT default_install FROM browser_extensions WHERE extension_id = ?`, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb").Scan(&defaultInstall); err != nil {
		t.Fatalf("query default_install returned error: %v", err)
	}
	if defaultInstall != 0 {
		t.Fatalf("default_install = %d, want 0 for a newly stored plugin", defaultInstall)
	}
}

func TestMigrateRepairsCrossBranchSchemaGaps(t *testing.T) {
	db, err := NewDB(filepath.Join(t.TempDir(), "database.db"))
	if err != nil {
		t.Fatalf("NewDB returned error: %v", err)
	}
	defer db.Close()

	if _, err := db.GetConn().Exec(`
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
		if err := db.applyMigration(migration); err != nil {
			t.Fatalf("apply migration %d returned error: %v", migration.version, err)
		}
	}

	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}
	for _, column := range []string{"runtime_type", "runtime_config_json", "proxy_mode", "proxy_group_name"} {
		if !sqliteColumnExists(t, db.GetConn(), "browser_profiles", column) {
			t.Fatalf("browser_profiles.%s was not repaired", column)
		}
	}
	for _, column := range []string{"strategy", "selected_proxy_id", "member_proxy_ids", "member_group_names"} {
		if !sqliteColumnExists(t, db.GetConn(), "browser_proxy_virtual_groups", column) {
			t.Fatalf("browser_proxy_virtual_groups.%s was not repaired", column)
		}
	}
}

func sqliteColumnExists(t *testing.T, db *sql.DB, tableName string, columnName string) bool {
	t.Helper()
	rows, err := db.Query("PRAGMA table_info(" + tableName + ")")
	if err != nil {
		t.Fatalf("query table info for %s failed: %v", tableName, err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull, primaryKey int
		var defaultValue interface{}
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatalf("scan table info for %s failed: %v", tableName, err)
		}
		if name == columnName {
			return true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate table info for %s failed: %v", tableName, err)
	}
	return false
}
