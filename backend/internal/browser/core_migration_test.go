package browser

import (
	"ant-chrome/backend/internal/config"
	"database/sql"
	"path/filepath"
	"testing"

	"ant-chrome/backend/internal/database"
)

// 内核后端迁移给 browser_cores 增加了 core_backend / core_env 两列。
// 这里验证从"迁移前结构"升级时历史数据不丢、且新列可用。
func TestCoreBackendMigrationUpgradesLegacyCoreTable(t *testing.T) {
	db, err := database.NewDB(filepath.Join(t.TempDir(), "legacy.db"))
	if err != nil {
		t.Fatalf("NewDB returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	conn := db.GetConn()
	// 手工建出 v15 之前的表结构，并写入一行历史数据
	if _, err := conn.Exec(`CREATE TABLE browser_cores (
		core_id    TEXT PRIMARY KEY,
		core_name  TEXT NOT NULL,
		core_path  TEXT NOT NULL,
		is_default INTEGER NOT NULL DEFAULT 0,
		sort_order INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		t.Fatalf("create legacy table: %v", err)
	}
	if _, err := conn.Exec(
		`INSERT INTO browser_cores (core_id, core_name, core_path, is_default) VALUES (?, ?, ?, 1)`,
		"legacy", "Legacy Core", "chrome",
	); err != nil {
		t.Fatalf("insert legacy row: %v", err)
	}

	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}

	// 新列存在，历史行取到空后端标记
	if !coreColumnExists(t, conn, "core_backend") {
		t.Fatal("core_backend column missing after migration")
	}
	if !coreColumnExists(t, conn, "core_env") {
		t.Fatal("core_env column missing after migration")
	}

	// 通过 DAO 读取时，空后端应被归一化为 fingerprint-chromium，历史配置行为不变
	cores, err := NewSQLiteCoreDAO(conn).List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(cores) != 1 {
		t.Fatalf("cores length = %d, want 1", len(cores))
	}
	if cores[0].CoreName != "Legacy Core" || cores[0].CorePath != "chrome" || !cores[0].IsDefault {
		t.Fatalf("legacy row was not preserved: %#v", cores[0])
	}
	if got, want := cores[0].CoreBackend, config.CoreBackendFingerprintChromium; got != want {
		t.Fatalf("legacy CoreBackend = %q, want %q", got, want)
	}
}

func coreColumnExists(t *testing.T, conn *sql.DB, column string) bool {
	t.Helper()
	rows, err := conn.Query(`PRAGMA table_info(browser_cores)`)
	if err != nil {
		t.Fatalf("PRAGMA table_info returned error: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid        int
			name       string
			columnType string
			notNull    int
			dfltValue  sql.NullString
			primaryKey int
		)
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &dfltValue, &primaryKey); err != nil {
			t.Fatalf("scan table_info row: %v", err)
		}
		if name == column {
			return true
		}
	}
	return false
}
