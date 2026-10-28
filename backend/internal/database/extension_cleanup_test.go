package database

import (
	"path/filepath"
	"testing"
)

// v30 只清理「手动设置但一个插件都没绑」的空配置，正常配置必须原样保留。
func TestMigrateClearsEmptyManualProfileExtensionSettings(t *testing.T) {
	db, err := NewDB(filepath.Join(t.TempDir(), "database.db"))
	if err != nil {
		t.Fatalf("NewDB returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}

	conn := db.GetConn()
	if _, err := conn.Exec(`INSERT INTO browser_profile_extension_settings (profile_id, configured) VALUES ('profile-empty', 1)`); err != nil {
		t.Fatalf("seed empty settings: %v", err)
	}
	if _, err := conn.Exec(`INSERT INTO browser_profile_extension_settings (profile_id, configured) VALUES ('profile-bound', 1)`); err != nil {
		t.Fatalf("seed bound settings: %v", err)
	}
	if _, err := conn.Exec(`INSERT INTO browser_profile_extensions (profile_id, extension_id) VALUES ('profile-bound', 'extension-x')`); err != nil {
		t.Fatalf("seed binding: %v", err)
	}

	// 删掉版本记录，模拟从 v29 升级上来时重放该迁移。
	if _, err := conn.Exec(`DELETE FROM schema_migrations WHERE version = 30`); err != nil {
		t.Fatalf("reset migration marker: %v", err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate (replay) returned error: %v", err)
	}

	assertProfileSettingCount(t, db, "profile-empty", 0)
	assertProfileSettingCount(t, db, "profile-bound", 1)

	// 再次执行迁移不应有任何变化（幂等）。
	if _, err := conn.Exec(`DELETE FROM schema_migrations WHERE version = 30`); err != nil {
		t.Fatalf("reset migration marker: %v", err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate (second replay) returned error: %v", err)
	}
	assertProfileSettingCount(t, db, "profile-bound", 1)
}

func assertProfileSettingCount(t *testing.T, db *DB, profileID string, want int) {
	t.Helper()
	var got int
	if err := db.GetConn().QueryRow(
		`SELECT COUNT(*) FROM browser_profile_extension_settings WHERE profile_id = ?`, profileID,
	).Scan(&got); err != nil {
		t.Fatalf("count settings for %s: %v", profileID, err)
	}
	if got != want {
		t.Fatalf("settings for %s = %d, want %d", profileID, got, want)
	}
}
