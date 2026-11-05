package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

// buildConflictingBranchDB 复刻跨分支版本号冲突的存量库现场：
// 执行主线 v1-v17 迁移 + 模拟 feature/android-engine 分支的 v18，
// 使版本号停在 18（>= 主线 v17，跳过 v24+），但缺 v24-28 引入的列。
func buildConflictingBranchDB(t *testing.T, dir string) *DB {
	t.Helper()
	db, err := NewDB(filepath.Join(dir, "app.db"))
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    INTEGER PRIMARY KEY,
			desc       TEXT NOT NULL DEFAULT '',
			applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`); err != nil {
		t.Fatalf("create schema_migrations failed: %v", err)
	}
	for _, m := range migrations {
		if m.version > 17 {
			continue
		}
		for _, stmt := range m.stmts {
			if _, err := db.conn.Exec(stmt); err != nil && !isColumnExistsError(err) {
				t.Fatalf("apply migration v%d failed: %v", m.version, err)
			}
		}
		if _, err := db.conn.Exec(
			`INSERT INTO schema_migrations (version, desc) VALUES (?, ?)`, m.version, m.desc,
		); err != nil {
			t.Fatalf("record migration v%d failed: %v", m.version, err)
		}
	}
	// 模拟 feature/android-engine 分支的 v18（版本号冲突区）。
	for _, stmt := range []string{
		`ALTER TABLE browser_profiles ADD COLUMN runtime_type TEXT NOT NULL DEFAULT 'desktop_chromium'`,
		`ALTER TABLE browser_profiles ADD COLUMN runtime_config_json TEXT NOT NULL DEFAULT '{}'`,
	} {
		if _, err := db.conn.Exec(stmt); err != nil {
			t.Fatalf("apply android v18 stmt failed: %v", err)
		}
	}
	if _, err := db.conn.Exec(
		`INSERT INTO schema_migrations (version, desc) VALUES (18, '添加实例运行时配置')`,
	); err != nil {
		t.Fatalf("record android v18 failed: %v", err)
	}
	return db
}

func hasColumn(t *testing.T, db *DB, table, column string) bool {
	t.Helper()
	rows, err := db.conn.Query(`SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		t.Fatalf("pragma_table_info(%s) failed: %v", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan column failed: %v", err)
		}
		if name == column {
			return true
		}
	}
	return false
}

// 场景一（用户现场复刻）：版本号停在冲突区 v18、缺主线 v26 测速来源列，
// Reconcile 应补齐缺失列且不触碰既有数据。
func TestReconcileRepairsConflictingBranchSchema(t *testing.T) {
	dir := t.TempDir()
	db := buildConflictingBranchDB(t, dir)

	// 预置业务数据（用户存量），对齐后必须原样保留。
	seedRows := [][]any{
		{"proxy-1", "节点一", "http://127.0.0.1:8080", "", 0},
		{"proxy-2", "节点二", "vless://uuid@1.2.3.4:443", "分组A", 1},
	}
	for _, row := range seedRows {
		if _, err := db.conn.Exec(
			`INSERT INTO browser_proxies (proxy_id, proxy_name, proxy_config, group_name, sort_order) VALUES (?, ?, ?, ?, ?)`,
			row...,
		); err != nil {
			t.Fatalf("seed proxy failed: %v", err)
		}
	}

	// 现场断言：测速来源列缺失（正是 no such column 报错根源）。
	if hasColumn(t, db, "browser_proxies", "last_test_engine") {
		t.Fatal("预置条件失败：last_test_engine 不应存在")
	}

	report, err := db.Reconcile(filepath.Join(dir, "db-backups"))
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	// 主线 v26 的三列必须补齐。
	for _, col := range []string{"last_test_engine", "last_test_connector_type", "last_test_fingerprint"} {
		if !hasColumn(t, db, "browser_proxies", col) {
			t.Errorf("对齐后 browser_proxies.%s 仍缺失", col)
		}
	}
	// 主线 v25 的虚拟分组表必须补齐。
	for _, col := range []string{"strategy", "member_proxy_ids", "member_group_names"} {
		if !hasColumn(t, db, "browser_proxy_virtual_groups", col) {
			t.Errorf("对齐后 browser_proxy_virtual_groups.%s 仍缺失", col)
		}
	}
	if !report.BackupCreated || report.BackupPath == "" {
		t.Errorf("有差异时必须先备份: %+v", report)
	}
	if report.AppliedStmts == 0 {
		t.Errorf("应执行补齐语句: %+v", report)
	}

	// 数据保全：行数与内容逐字段对比。
	rows, err := db.conn.Query(`SELECT proxy_id, proxy_name, proxy_config, group_name, sort_order FROM browser_proxies ORDER BY sort_order`)
	if err != nil {
		t.Fatalf("query proxies failed: %v", err)
	}
	defer rows.Close()
	var got [][]any
	for rows.Next() {
		var id, name, cfg, group string
		var sort int
		if err := rows.Scan(&id, &name, &cfg, &group, &sort); err != nil {
			t.Fatalf("scan proxy failed: %v", err)
		}
		got = append(got, []any{id, name, cfg, group, sort})
	}
	if len(got) != len(seedRows) {
		t.Fatalf("行数 = %d, want %d（数据被刷错）", len(got), len(seedRows))
	}
	for i := range seedRows {
		for j := range seedRows[i] {
			if got[i][j] != seedRows[i][j] {
				t.Errorf("行 %d 字段 %d = %v, want %v（数据被改动）", i, j, got[i][j], seedRows[i][j])
			}
		}
	}
}

// 场景二：幂等性——重复对齐零操作、零备份。
func TestReconcileIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	db := buildConflictingBranchDB(t, dir)

	first, err := db.Reconcile(filepath.Join(dir, "db-backups"))
	if err != nil {
		t.Fatalf("first Reconcile failed: %v", err)
	}
	if first.AppliedStmts == 0 {
		t.Fatalf("首次对齐应执行语句: %+v", first)
	}

	second, err := db.Reconcile(filepath.Join(dir, "db-backups"))
	if err != nil {
		t.Fatalf("second Reconcile failed: %v", err)
	}
	if second.AppliedStmts != 0 || len(second.AddedTables) != 0 || len(second.AddedColumns) != 0 {
		t.Fatalf("第二次对齐应零操作: %+v", second)
	}
	if second.BackupCreated {
		t.Fatalf("零差异时不应再备份: %+v", second)
	}
}

// 场景三：正常最新库（先 Migrate 再 Reconcile）零操作。
func TestReconcileNoopOnFreshMigratedDB(t *testing.T) {
	dir := t.TempDir()
	db, err := NewDB(filepath.Join(dir, "app.db"))
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	report, err := db.Reconcile(filepath.Join(dir, "db-backups"))
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}
	if report.AppliedStmts != 0 || len(report.AddedTables) != 0 || len(report.AddedColumns) != 0 {
		t.Fatalf("最新库对齐应零操作: %+v", report)
	}
}

// 场景四：空库直接 Reconcile 也能建全（自愈建库能力）。
func TestReconcileBuildsEmptyDatabase(t *testing.T) {
	dir := t.TempDir()
	db, err := NewDB(filepath.Join(dir, "app.db"))
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer db.Close()

	report, err := db.Reconcile(filepath.Join(dir, "db-backups"))
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}
	if len(report.AddedTables) == 0 {
		t.Fatalf("空库应建全表: %+v", report)
	}
	// 建出的表必须可用：插入并读回。
	if _, err := db.conn.Exec(
		`INSERT INTO browser_proxies (proxy_id, proxy_name, proxy_config) VALUES ('p1', '节点', 'http://127.0.0.1:1')`,
	); err != nil {
		t.Fatalf("insert after reconcile failed: %v", err)
	}
	var count int
	if err := db.conn.QueryRow(`SELECT COUNT(*) FROM browser_proxies`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("count = %d, err = %v", count, err)
	}
}

// 场景五：备份可用性——VACUUM INTO 快照可被完整打开并读出数据。
func TestBackupToProducesReadableSnapshot(t *testing.T) {
	dir := t.TempDir()
	db := buildConflictingBranchDB(t, dir)
	if _, err := db.conn.Exec(
		`INSERT INTO browser_proxies (proxy_id, proxy_name, proxy_config) VALUES ('p9', '快照节点', 'http://127.0.0.1:9')`,
	); err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	backupPath, err := db.BackupTo(filepath.Join(dir, "db-backups"), "app-pre-reconcile")
	if err != nil {
		t.Fatalf("BackupTo failed: %v", err)
	}

	snapshot, err := sql.Open("sqlite", backupPath)
	if err != nil {
		t.Fatalf("open snapshot failed: %v", err)
	}
	defer snapshot.Close()
	var name string
	if err := snapshot.QueryRow(`SELECT proxy_name FROM browser_proxies WHERE proxy_id='p9'`).Scan(&name); err != nil {
		t.Fatalf("snapshot query failed: %v", err)
	}
	if name != "快照节点" {
		t.Fatalf("snapshot name = %q, want 快照节点", name)
	}
}

// 场景六：备份轮换——超过保留份数后删旧留新。
func TestPruneBackupsKeepsRecentOnly(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "db-backups")

	for i := 0; i < reconcileBackupKeep+2; i++ {
		prefix := "app-pre-reconcile"
		if i%2 == 0 {
			prefix = "app-pre-migrate"
		}
		// 用可排序的固定后缀替代时间戳以保证确定性。
		path := filepath.Join(backupDir, prefix+"-20000101-00000"+string(rune('0'+i))+".db")
		if err := writeBackupFile(path); err != nil {
			t.Fatalf("write backup %d failed: %v", i, err)
		}
	}
	pruneBackups(backupDir)
	count := countBackupFiles(t, backupDir)
	if count != reconcileBackupKeep {
		t.Fatalf("备份份数 = %d, want %d", count, reconcileBackupKeep)
	}
}

func writeBackupFile(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte("backup"), 0o644)
}

func countBackupFiles(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read backup dir failed: %v", err)
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			count++
		}
	}
	return count
}