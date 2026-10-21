package backend

import (
	"ant-chrome/backend/internal/config"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type backupImportFileState struct {
	exists bool
	data   []byte
	mode   os.FileMode
}

type backupImportRollback struct {
	tempDir        string
	dbSnapshotPath string
	configPath     string
	proxiesPath    string
	configState    backupImportFileState
	proxiesState   backupImportFileState
	originalConfig *config.Config
	createdPaths   []string
	createdPathSet map[string]struct{}
}

func backupImportError(message string, err error) error {
	if err == nil {
		return errors.New(message)
	}
	return errors.New(message + ": " + err.Error())
}

func (a *App) beginBackupImportRollback() (*backupImportRollback, error) {
	tempDir, err := os.MkdirTemp("", "ant-chrome-import-rollback-")
	if err != nil {
		return nil, backupImportError("创建备份导入回滚目录失败", err)
	}
	rollback := &backupImportRollback{
		tempDir:        tempDir,
		configPath:     a.resolveAppPath("config.yaml"),
		proxiesPath:    a.resolveAppPath("proxies.yaml"),
		createdPathSet: make(map[string]struct{}),
		originalConfig: cloneBackupImportConfig(a.config),
	}
	cleanup := func(err error) (*backupImportRollback, error) {
		_ = os.RemoveAll(tempDir)
		return nil, err
	}

	rollback.configState, err = captureBackupImportFileState(rollback.configPath)
	if err != nil {
		return cleanup(backupImportError("保存原配置失败", err))
	}
	rollback.proxiesState, err = captureBackupImportFileState(rollback.proxiesPath)
	if err != nil {
		return cleanup(backupImportError("保存原代理配置失败", err))
	}

	if a.db != nil && a.db.GetConn() != nil {
		rollback.dbSnapshotPath = filepath.Join(tempDir, "app.db")
		if _, err := a.db.GetConn().Exec(`VACUUM INTO ?`, rollback.dbSnapshotPath); err != nil {
			return cleanup(backupImportError("保存原数据库快照失败", err))
		}
	}
	return rollback, nil
}

func captureBackupImportFileState(path string) (backupImportFileState, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return backupImportFileState{}, nil
	}
	if err != nil {
		return backupImportFileState{}, err
	}
	if !info.Mode().IsRegular() {
		return backupImportFileState{}, errors.New("路径不是普通文件: " + path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return backupImportFileState{}, err
	}
	return backupImportFileState{exists: true, data: data, mode: info.Mode().Perm()}, nil
}

func cloneBackupImportConfig(value *config.Config) *config.Config {
	if value == nil {
		return nil
	}
	data, err := yaml.Marshal(value)
	if err != nil {
		return nil
	}
	var clone config.Config
	if err := yaml.Unmarshal(data, &clone); err != nil {
		return nil
	}
	return &clone
}

func (r *backupImportRollback) recordCreatedPath(path string) {
	if r == nil {
		return
	}
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" {
		return
	}
	if _, exists := r.createdPathSet[path]; exists {
		return
	}
	r.createdPathSet[path] = struct{}{}
	r.createdPaths = append(r.createdPaths, path)
}

func (r *backupImportRollback) rollback(a *App) error {
	if r == nil {
		return nil
	}
	var rollbackErrors []error
	if r.dbSnapshotPath != "" {
		if err := restoreBackupImportDatabase(a, r.dbSnapshotPath); err != nil {
			rollbackErrors = append(rollbackErrors, err)
		}
	}
	if err := r.removeCreatedPaths(); err != nil {
		rollbackErrors = append(rollbackErrors, err)
	}
	if err := restoreBackupImportFileState(r.configPath, r.configState); err != nil {
		rollbackErrors = append(rollbackErrors, backupImportError("恢复原配置失败", err))
	}
	if err := restoreBackupImportFileState(r.proxiesPath, r.proxiesState); err != nil {
		rollbackErrors = append(rollbackErrors, backupImportError("恢复原代理配置失败", err))
	}
	return errors.Join(rollbackErrors...)
}

func (r *backupImportRollback) removeCreatedPaths() error {
	paths := append([]string(nil), r.createdPaths...)
	sort.Slice(paths, func(i, j int) bool {
		return len(paths[i]) > len(paths[j])
	})
	var removeErrors []error
	for _, path := range paths {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			removeErrors = append(removeErrors, errors.New("删除导入新增路径失败("+path+"): "+err.Error()))
		}
	}
	return errors.Join(removeErrors...)
}

func restoreBackupImportFileState(path string, state backupImportFileState) error {
	if !state.exists {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	tmpPath := path + ".rollback.tmp"
	if err := os.WriteFile(tmpPath, state.data, state.mode); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func restoreBackupImportDatabase(a *App, snapshotPath string) error {
	if a == nil || a.db == nil || a.db.GetConn() == nil {
		return errors.New("数据库未初始化，无法回滚备份导入")
	}
	conn := a.db.GetConn()
	if _, err := conn.Exec(`ATTACH DATABASE ? AS backup_restore`, snapshotPath); err != nil {
		return backupImportError("挂载数据库回滚快照失败", err)
	}
	defer conn.Exec(`DETACH DATABASE backup_restore`)

	tx, err := conn.Begin()
	if err != nil {
		return backupImportError("开启数据库回滚事务失败", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	tables, err := backupSQLiteTableNames(tx, "main")
	if err != nil {
		return backupImportError("读取当前数据库表失败", err)
	}
	for _, table := range tables {
		exists, err := backupSQLiteTableExists(tx, "backup_restore", table)
		if err != nil {
			return errors.New("检查数据库回滚表失败(" + table + "): " + err.Error())
		}
		if !exists {
			return errors.New("回滚快照缺少数据表: " + table)
		}
		quoted := backupQuoteSQLiteIdentifier(table)
		if _, err := tx.Exec(`DELETE FROM main.` + quoted); err != nil {
			return errors.New("清理数据库表失败(" + table + "): " + err.Error())
		}
		if _, err := tx.Exec(`INSERT INTO main.` + quoted + ` SELECT * FROM backup_restore.` + quoted); err != nil {
			return errors.New("恢复数据库表失败(" + table + "): " + err.Error())
		}
	}
	if exists, err := backupSQLiteTableExists(tx, "main", "sqlite_sequence"); err != nil {
		return backupImportError("检查 SQLite 序列表失败", err)
	} else if exists {
		if sourceExists, sourceErr := backupSQLiteTableExists(tx, "backup_restore", "sqlite_sequence"); sourceErr != nil {
			return backupImportError("检查 SQLite 回滚序列表失败", sourceErr)
		} else if sourceExists {
			if _, err := tx.Exec(`DELETE FROM main.sqlite_sequence`); err != nil {
				return backupImportError("清理 SQLite 序列表失败", err)
			}
			if _, err := tx.Exec(`INSERT INTO main.sqlite_sequence SELECT * FROM backup_restore.sqlite_sequence`); err != nil {
				return backupImportError("恢复 SQLite 序列表失败", err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return backupImportError("提交数据库回滚事务失败", err)
	}
	committed = true
	return nil
}

func backupSQLiteTableNames(tx *sql.Tx, schema string) ([]string, error) {
	rows, err := tx.Query(`SELECT name FROM ` + schema + `.sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

func backupSQLiteTableExists(tx *sql.Tx, schema, table string) (bool, error) {
	var found int
	err := tx.QueryRow(`SELECT 1 FROM `+schema+`.sqlite_master WHERE type='table' AND name=? LIMIT 1`, table).Scan(&found)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil && found == 1, err
}

func backupQuoteSQLiteIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func (r *backupImportRollback) cleanup() {
	if r == nil || r.tempDir == "" {
		return
	}
	_ = os.RemoveAll(r.tempDir)
}

func (a *App) backupReloadAfterImportRollback(rollback *backupImportRollback) error {
	if rollback == nil || rollback.configState.exists || rollback.originalConfig == nil {
		return a.backupReloadAfterMutation()
	}
	data, err := yaml.Marshal(rollback.originalConfig)
	if err != nil {
		return backupImportError("序列化回滚配置失败", err)
	}
	if err := os.MkdirAll(filepath.Dir(rollback.configPath), 0755); err != nil {
		return backupImportError("创建回滚配置目录失败", err)
	}
	if err := os.WriteFile(rollback.configPath, data, 0644); err != nil {
		return backupImportError("写入临时回滚配置失败", err)
	}
	defer os.Remove(rollback.configPath)
	return a.backupReloadAfterMutation()
}
