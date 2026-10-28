package database

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// ─── 启动期 schema 自愈对齐 ───
//
// 背景：版本号驱动的 Migrate() 在多分支并行开发时存在版本号冲突
// （如 feature 分支占用 18-23，主线从 17 跳到 24），存量库可能出现
// "版本号已达标但列实际缺失"的异构状态，运行期报 no such column。
//
// 本层不再信任版本号，直接对比"实际表结构"与"migrations 推导出的期望结构"，
// 缺表补表、缺列补列。安全约束：
//   - 单一事实源：期望结构完全从 migrations 语句推导，不另维护副本
//   - 白名单：仅执行 CREATE TABLE IF NOT EXISTS / ADD COLUMN，绝不删改
//   - 先备份后动手：VACUUM INTO 一致性快照，备份失败则放弃本次对齐
//   - 原子性：所有补齐语句在单事务内执行，任一失败全部回滚
//   - 幂等：重复启动零副作用

type expectedColumn struct {
	name string
	stmt string
}

type expectedTable struct {
	createStmt string
	columns    []expectedColumn
}

var (
	createTablePattern    = regexp.MustCompile(`(?is)^CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(\w+)\s*\((.*)\)$`)
	alterAddColumnPattern = regexp.MustCompile(`(?is)^ALTER\s+TABLE\s+(\w+)\s+ADD\s+COLUMN\s+(\w+)\s+(.+)$`)
	ifNotExistsPattern    = regexp.MustCompile(`(?i)^CREATE\s+TABLE\s+IF\s+NOT\s+EXISTS\b`)
	createTableOnlyPattern = regexp.MustCompile(`(?i)^CREATE\s+TABLE\s+`)
)

// ReconcileReport 一次自愈对齐的结果，用于日志与生命周期事件。
type ReconcileReport struct {
	AddedTables   []string
	AddedColumns  []string // "表名.列名" 形式
	BackupPath    string
	BackupCreated bool
	AppliedStmts  int
	DurationMs    int64
}

// BackupTo 使用 VACUUM INTO 生成库的一致性快照（含 WAL 内容），并轮换保留最近 keepBackups 份。
func (db *DB) BackupTo(backupDir, prefix string) (string, error) {
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return "", fmt.Errorf("准备备份目录失败: %w", err)
	}
	path := filepath.Join(backupDir, fmt.Sprintf("%s-%s.db", prefix, time.Now().Format("20060102-150405")))
	if _, err := db.conn.Exec(`VACUUM INTO ?`, path); err != nil {
		return "", fmt.Errorf("数据库备份失败: %w", err)
	}
	pruneBackups(backupDir)
	return path, nil
}

// HasPendingMigrations 是否存在尚未执行的迁移版本（版本表不可读时保守返回 true）。
func (db *DB) HasPendingMigrations() bool {
	var currentVersion int
	if err := db.conn.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&currentVersion); err != nil {
		return true
	}
	for _, m := range migrations {
		if m.version > currentVersion {
			return true
		}
	}
	return false
}

// Reconcile 对齐实际 schema 到 migrations 推导出的期望结构。
// 无差异时零开销；有差异时先备份再单事务补齐。
// 失败时返回错误，调用方应仅记录并放行启动（运行时已有降级容错）。
func (db *DB) Reconcile(backupDir string) (*ReconcileReport, error) {
	start := time.Now()
	expected := deriveExpectedSchema(migrations)
	stmts, addedTables, addedColumns, err := db.schemaDiff(expected)
	if err != nil {
		return nil, err
	}

	report := &ReconcileReport{AddedTables: addedTables, AddedColumns: addedColumns}
	if len(stmts) == 0 {
		report.DurationMs = time.Since(start).Milliseconds()
		return report, nil
	}

	backupPath, err := db.BackupTo(backupDir, "app-pre-reconcile")
	if err != nil {
		return nil, fmt.Errorf("自愈对齐前备份失败（已跳过本次对齐）: %w", err)
	}
	report.BackupPath = backupPath
	report.BackupCreated = true

	if err := db.applyReconcileStmts(stmts); err != nil {
		return nil, err
	}
	report.AppliedStmts = len(stmts)
	report.DurationMs = time.Since(start).Milliseconds()
	return report, nil
}

// ─── 期望 schema 推导 ───

func deriveExpectedSchema(migs []migration) map[string]*expectedTable {
	tables := map[string]*expectedTable{}
	getTable := func(name string) *expectedTable {
		t, ok := tables[name]
		if !ok {
			t = &expectedTable{}
			tables[name] = t
		}
		return t
	}

	for _, m := range migs {
		for _, raw := range m.stmts {
			stmt := strings.TrimSpace(strings.TrimRight(strings.TrimSpace(raw), ";"))
			if stmt == "" {
				continue
			}
			if matches := createTablePattern.FindStringSubmatch(stmt); matches != nil {
				tableName, body := matches[1], matches[2]
				t := getTable(tableName)
				if t.createStmt == "" {
					t.createStmt = ensureCreateIfNotExists(stmt)
				}
				for _, colDef := range splitTopLevel(body) {
					def := strings.TrimSpace(colDef)
					if def == "" || isTableConstraint(def) {
						continue
					}
					colName := firstWord(def)
					if colName == "" || t.hasColumn(colName) {
						continue
					}
					// 建表语句推导的列：剥离 PRIMARY KEY/UNIQUE 等表级约束，
					// 仅用于"表存在但缺列"的跨分支补列场景。
					t.columns = append(t.columns, expectedColumn{
						name: colName,
						stmt: fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s", tableName, sanitizeColumnDefinition(def)),
					})
				}
			} else if matches := alterAddColumnPattern.FindStringSubmatch(stmt); matches != nil {
				tableName, colName, colDef := matches[1], matches[2], strings.TrimSpace(matches[3])
				t := getTable(tableName)
				if t.hasColumn(colName) {
					continue
				}
				t.columns = append(t.columns, expectedColumn{
					name: colName,
					stmt: fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", tableName, colName, colDef),
				})
			}
			// 其余语句（CREATE INDEX / UPDATE 等）不参与对齐：索引缺失只影响性能，数据语句绝不能重放。
		}
	}
	return tables
}

func (t *expectedTable) hasColumn(name string) bool {
	for _, col := range t.columns {
		if col.name == name {
			return true
		}
	}
	return false
}

// ensureCreateIfNotExists 保证建表语句可安全重复执行。
func ensureCreateIfNotExists(stmt string) string {
	if ifNotExistsPattern.MatchString(stmt) {
		return stmt
	}
	return createTableOnlyPattern.ReplaceAllString(stmt, "CREATE TABLE IF NOT EXISTS ")
}

// splitTopLevel 按顶层逗号切分（忽略括号内的逗号，如 PRIMARY KEY (a, b)）。
func splitTopLevel(s string) []string {
	var parts []string
	depth := 0
	var current strings.Builder
	for _, r := range s {
		switch r {
		case '(':
			depth++
			current.WriteRune(r)
		case ')':
			depth--
			current.WriteRune(r)
		case ',':
			if depth == 0 {
				parts = append(parts, current.String())
				current.Reset()
			} else {
				current.WriteRune(r)
			}
		default:
			current.WriteRune(r)
		}
	}
	if strings.TrimSpace(current.String()) != "" {
		parts = append(parts, current.String())
	}
	return parts
}

// isTableConstraint 判断列定义段是否为表级约束（非列定义）。
func isTableConstraint(def string) bool {
	fields := strings.Fields(strings.TrimSpace(def))
	if len(fields) == 0 {
		return false
	}
	switch strings.ToUpper(fields[0]) {
	case "PRIMARY", "UNIQUE", "CHECK", "FOREIGN", "CONSTRAINT":
		return true
	}
	return false
}

func firstWord(def string) string {
	fields := strings.Fields(strings.TrimSpace(def))
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// sanitizeColumnDefinition 剥离 ADD COLUMN 不允许或可能触碰存量数据的约束
// （PRIMARY KEY / UNIQUE / AUTOINCREMENT / REFERENCES / CHECK），
// 保留类型与 NOT NULL/DEFAULT。仅用于从 CREATE TABLE 推导的补列语句。
func sanitizeColumnDefinition(def string) string {
	for {
		upper := strings.ToUpper(def)
		minIdx, found := -1, false
		for _, kw := range []string{"PRIMARY KEY", "UNIQUE", "AUTOINCREMENT", "REFERENCES", "CHECK"} {
			if idx := strings.Index(upper, kw); idx >= 0 && (minIdx < 0 || idx < minIdx) {
				minIdx, found = idx, true
			}
		}
		if !found {
			return strings.TrimSpace(def)
		}
		def = strings.TrimSpace(def[:minIdx])
	}
}

// ─── 实际结构对账 ───

func (db *DB) schemaDiff(expected map[string]*expectedTable) (stmts []string, addedTables []string, addedColumns []string, err error) {
	existingTables := map[string]bool{}
	rows, err := db.conn.Query(`SELECT name FROM sqlite_master WHERE type='table'`)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("读取实际表结构失败: %w", err)
	}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return nil, nil, nil, fmt.Errorf("读取实际表结构失败: %w", err)
		}
		existingTables[name] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, nil, nil, fmt.Errorf("读取实际表结构失败: %w", err)
	}

	tableNames := make([]string, 0, len(expected))
	for name := range expected {
		tableNames = append(tableNames, name)
	}
	sort.Strings(tableNames)

	for _, tableName := range tableNames {
		table := expected[tableName]
		if !existingTables[tableName] {
			stmts = append(stmts, table.createStmt)
			addedTables = append(addedTables, tableName)
			// 不跳过列比对：期望结构里的列可能来自后续版本的 ALTER，
			// 只执行建表语句会漏掉这些列，导致对齐后仍缺字段。
		}
		existingCols := map[string]bool{}
		colRows, err := db.conn.Query(fmt.Sprintf(`PRAGMA table_info(%q)`, tableName))
		if err != nil {
			return nil, nil, nil, fmt.Errorf("读取表 %s 列结构失败: %w", tableName, err)
		}
		for colRows.Next() {
			var cid int
			var name, colType string
			var notNull, pk int
			var dfltValue any
			if err := colRows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
				colRows.Close()
				return nil, nil, nil, fmt.Errorf("读取表 %s 列结构失败: %w", tableName, err)
			}
			existingCols[name] = true
		}
		colRows.Close()
		if err := colRows.Err(); err != nil {
			return nil, nil, nil, fmt.Errorf("读取表 %s 列结构失败: %w", tableName, err)
		}
		for _, col := range table.columns {
			if existingCols[col.name] {
				continue
			}
			stmts = append(stmts, col.stmt)
			addedColumns = append(addedColumns, tableName+"."+col.name)
		}
	}
	return stmts, addedTables, addedColumns, nil
}

// applyReconcileStmts 在单事务内执行补齐语句，任一失败整体回滚。
func (db *DB) applyReconcileStmts(stmts []string) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("开启自愈事务失败: %w", err)
	}
	defer tx.Rollback()
	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			// 幂等容错：并发或重复定义导致的"列已存在"跳过，其余错误整体回滚。
			if isColumnExistsError(err) {
				continue
			}
			return fmt.Errorf("自愈语句执行失败 [%s]: %w", truncate(stmt, 60), err)
		}
	}
	return tx.Commit()
}

const reconcileBackupKeep = 5

// pruneBackups 轮换删除旧备份（文件名含时间戳，字典序即时间序），保留最近 reconcileBackupKeep 份。
func pruneBackups(backupDir string) {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return
	}
	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, "app-") && strings.HasSuffix(name, ".db") {
			files = append(files, name)
		}
	}
	sort.Strings(files)
	for len(files) > reconcileBackupKeep {
		_ = os.Remove(filepath.Join(backupDir, files[0]))
		files = files[1:]
	}
}