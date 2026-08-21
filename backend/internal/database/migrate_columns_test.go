package database

import (
	"os"
	"path/filepath"
	"testing"
)

// 复现真实故障：用户库里已记录 15/16/17（历史插件迁移），
// 版本水位 MAX(version)=17 会让新追加的迁移被当成"已执行"而跳过。
// 迁移完成后必须保证代码依赖的列真实存在。
func TestMigrateAddsCoreColumnsDespiteHigherVersionWatermark(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "legacy.db")
	db, err := NewDB(dbPath)
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
	// 造出高于本次迁移号的版本水位
	if _, err := conn.Exec(`CREATE TABLE schema_migrations (
		version    INTEGER PRIMARY KEY,
		desc       TEXT NOT NULL DEFAULT '',
		applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		t.Fatalf("create schema_migrations: %v", err)
	}
	for _, version := range []int{15, 16, 17} {
		if _, err := conn.Exec(`INSERT INTO schema_migrations (version, desc) VALUES (?, ?)`, version, "历史插件迁移"); err != nil {
			t.Fatalf("seed version %d: %v", version, err)
		}
	}

	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}

	for _, column := range []string{"core_backend", "core_env"} {
		exists, err := db.columnExists("browser_cores", column)
		if err != nil {
			t.Fatalf("columnExists(%s) returned error: %v", column, err)
		}
		if !exists {
			t.Fatalf("column %s missing after Migrate despite version watermark", column)
		}
	}

	// 历史数据不能丢
	var coreName string
	if err := conn.QueryRow(`SELECT core_name FROM browser_cores WHERE core_id='legacy'`).Scan(&coreName); err != nil {
		t.Fatalf("legacy row lost: %v", err)
	}
	if coreName != "Legacy" {
		t.Fatalf("core_name = %q, want Legacy", coreName)
	}

	// 幂等：重复迁移不应报错
	if err := db.Migrate(); err != nil {
		t.Fatalf("second Migrate returned error: %v", err)
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
	for _, column := range []string{"core_backend", "core_env"} {
		exists, err := db.columnExists("browser_cores", column)
		if err != nil {
			t.Fatalf("columnExists(%s) returned error: %v", column, err)
		}
		if !exists {
			t.Fatalf("fresh database missing column %s", column)
		}
	}
}

// 针对真实用户库的回归验证：如果仓库根目录存在 data/app.db，
// 用它的副本跑一次迁移，确认能自愈出缺失列。
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
	for _, column := range []string{"core_backend", "core_env"} {
		exists, err := db.columnExists("browser_cores", column)
		if err != nil {
			t.Fatalf("columnExists(%s) returned error: %v", column, err)
		}
		if !exists {
			t.Fatalf("real database copy missing column %s after Migrate", column)
		}
	}
}
