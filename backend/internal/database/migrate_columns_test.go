package database

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 用户库的版本水位已达到内核字段迁移号（其他构建或分叉占用过同一个号）时，
// Migrate 会跳过这条迁移；启动期 Reconcile 必须按迁移语句补齐内核字段，且不丢历史数据。
func TestReconcileAddsCoreColumnsDespiteHigherVersionWatermark(t *testing.T) {
	db, err := NewDB(filepath.Join(t.TempDir(), "legacy.db"))
	if err != nil {
		t.Fatalf("NewDB returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	conn := db.GetConn()
	// 造出迁移前的内核表结构
	if _, err := conn.Exec(`CREATE TABLE browser_cores (
		core_id    TEXT PRIMARY KEY,
		core_name  TEXT NOT NULL,
		core_path  TEXT NOT NULL,
		is_default INTEGER NOT NULL DEFAULT 0,
		sort_order INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		t.Fatalf("create legacy table: %v", err)
	}
	if _, err := conn.Exec(`INSERT INTO browser_cores (core_id, core_name, core_path, is_default) VALUES ('legacy','Legacy','chrome',1)`); err != nil {
		t.Fatalf("insert legacy row: %v", err)
	}
	// 造出不低于本次迁移号的版本水位
	if _, err := conn.Exec(`CREATE TABLE schema_migrations (
		version    INTEGER PRIMARY KEY,
		desc       TEXT NOT NULL DEFAULT '',
		applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		t.Fatalf("create schema_migrations: %v", err)
	}
	coreMigration := coreColumnsMigrationVersion(t)
	if _, err := conn.Exec(`INSERT INTO schema_migrations (version, desc) VALUES (?, ?)`, coreMigration, "其他构建占用的迁移"); err != nil {
		t.Fatalf("seed version %d: %v", coreMigration, err)
	}

	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}
	if _, err := db.Reconcile(t.TempDir()); err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}
	assertCoreColumns(t, db)

	// 历史数据不能丢
	var coreName string
	if err := conn.QueryRow(`SELECT core_name FROM browser_cores WHERE core_id='legacy'`).Scan(&coreName); err != nil {
		t.Fatalf("legacy row lost: %v", err)
	}
	if coreName != "Legacy" {
		t.Fatalf("core_name = %q, want Legacy", coreName)
	}

	// 幂等：再次对齐不应有任何补齐语句
	report, err := db.Reconcile(t.TempDir())
	if err != nil {
		t.Fatalf("second Reconcile returned error: %v", err)
	}
	if report.AppliedStmts != 0 {
		t.Fatalf("second Reconcile applied %d stmts, want 0", report.AppliedStmts)
	}
}

// 全新库走正常迁移路径时同样要有新列。
func TestMigrateFreshDatabaseHasCoreColumns(t *testing.T) {
	db, err := NewDB(filepath.Join(t.TempDir(), "fresh.db"))
	if err != nil {
		t.Fatalf("NewDB returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}
	assertCoreColumns(t, db)
}

// 针对真实用户库的回归验证：如果仓库根目录存在 data/app.db，
// 用它的副本按启动顺序跑一次迁移与对齐，确认内核字段存在。
func TestMigrateRealDatabaseCopyIfPresent(t *testing.T) {
	source := filepath.Join("..", "..", "..", "data", "app.db")
	raw, err := os.ReadFile(source)
	if err != nil {
		t.Skipf("skip: no local data/app.db to verify (%v)", err)
	}

	target := filepath.Join(t.TempDir(), "app.db")
	if err := os.WriteFile(target, raw, 0o644); err != nil {
		t.Fatalf("copy database: %v", err)
	}

	db, err := NewDB(target)
	if err != nil {
		t.Fatalf("NewDB returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate on real database copy returned error: %v", err)
	}
	if _, err := db.Reconcile(t.TempDir()); err != nil {
		t.Fatalf("Reconcile on real database copy returned error: %v", err)
	}
	assertCoreColumns(t, db)
}

func coreColumnsMigrationVersion(t *testing.T) int {
	t.Helper()
	for _, m := range migrations {
		for _, stmt := range m.stmts {
			if strings.Contains(stmt, "browser_cores ADD COLUMN core_backend") {
				return m.version
			}
		}
	}
	t.Fatal("core_backend migration not found")
	return 0
}

func assertCoreColumns(t *testing.T, db *DB) {
	t.Helper()
	rows, err := db.GetConn().Query(`PRAGMA table_info(browser_cores)`)
	if err != nil {
		t.Fatalf("PRAGMA table_info returned error: %v", err)
	}
	defer rows.Close()

	found := map[string]bool{}
	for rows.Next() {
		var (
			cid, notNull, primaryKey int
			name, columnType         string
			dfltValue                any
		)
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &dfltValue, &primaryKey); err != nil {
			t.Fatalf("scan table_info: %v", err)
		}
		found[strings.ToLower(name)] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate table_info: %v", err)
	}
	for _, column := range []string{"core_backend", "core_env"} {
		if !found[column] {
			t.Fatalf("browser_cores missing column %s", column)
		}
	}
}
