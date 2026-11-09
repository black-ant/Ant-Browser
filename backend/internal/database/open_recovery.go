package database

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"modernc.org/sqlite"
)

// ─── 启动期数据库自愈 ───
//
// 目标：任何历史版本留下的库（旧 schema、跨分支异构、损坏文件、锁未释放）
// 都不能让应用起不来。策略分三级：
//  1. 打开 + 迁移，锁竞争先重试；
//  2. 迁移前存在待执行版本时，先做一致性快照备份；
//  3. 打开或迁移仍失败时，备份原库、移走原文件、重建空库并迁移。
//
// 第 3 级会丢业务数据，因此原库一律保留两份（备份目录 + 同目录 .broken-*），
// 由调用方把路径透出给用户，便于人工找回。

// RecoveryInfo 一次启动期数据库自愈的结果。
type RecoveryInfo struct {
	Recovered        bool   // 是否重建了空库（业务数据未恢复）
	Reason           string // 触发自愈的原始错误
	MigrationBackup  string // 升级前的数据快照（正常升级路径）
	BrokenBackupPath string // 损坏库的文件级备份
	BrokenParkedPath string // 损坏库的现场保留路径（同目录 .broken-*）
}

const (
	dbRecoveryMigrateAttempts = 3
	dbRecoveryRetryInterval   = 300 * time.Millisecond
)

// OpenWithRecovery 打开数据库并完成版本迁移，失败时自动备份并重建空库。
// 返回的 RecoveryInfo.Recovered=true 表示使用了兜底重建，调用方应显式告知用户。
func OpenWithRecovery(dbPath string, backupDir string) (*DB, RecoveryInfo, error) {
	info := RecoveryInfo{}
	dbPath = strings.TrimSpace(dbPath)
	if dbPath == "" {
		return nil, info, fmt.Errorf("数据库路径为空")
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, info, fmt.Errorf("创建数据库目录失败: %w", err)
	}

	// 打开前先判定是否存量库：NewDB 会创建库文件，之后再判断会把新装误判为升级。
	hadContent := databaseFileHasContent(dbPath)
	db, err := NewDB(dbPath)
	if err != nil {
		info.Reason = err.Error()
		// 锁竞争（另一个实例正在打开同一个库）不能走重建：会尝试搬走对方正在用的文件。
		if isDatabaseBusyError(err) {
			return nil, info, fmt.Errorf("数据库正被其他进程占用，请关闭另一个应用实例后重试: %w", err)
		}
		return recoverDatabase(dbPath, backupDir, info, err)
	}

	// 升级前快照：失败不阻断（迁移语句幂等且按版本事务化）。
	// 全新安装（库文件不存在或为空）没有可备份的数据，跳过以免产生垃圾备份。
	if hadContent && db.HasPendingMigrations() {
		if backupPath, backupErr := db.BackupTo(backupDir, "app-pre-migrate"); backupErr == nil {
			info.MigrationBackup = backupPath
		}
	}

	if err := migrateWithRetry(db, dbRecoveryMigrateAttempts); err != nil {
		info.Reason = err.Error()
		_ = db.Close()
		// 锁竞争说明另一个实例正在使用同一个库：重建会毁掉对方的数据，必须直接报错。
		if isDatabaseBusyError(err) {
			return nil, info, fmt.Errorf("数据库正被其他进程占用，请关闭另一个应用实例后重试: %w", err)
		}
		return recoverDatabase(dbPath, backupDir, info, err)
	}
	return db, info, nil
}

// recoverDatabase 备份并移走当前库文件，重建空库后完成迁移。
func recoverDatabase(dbPath string, backupDir string, info RecoveryInfo, cause error) (*DB, RecoveryInfo, error) {
	// 文件级备份最可靠：库损坏时 VACUUM INTO 往往已经不可用。
	if _, statErr := os.Stat(dbPath); statErr == nil {
		if dst, backupErr := backupDatabaseFiles(dbPath, backupDir, "app-db-broken"); backupErr == nil {
			info.BrokenBackupPath = dst
		}
	}
	parked, parkErr := parkDatabaseFiles(dbPath)
	if parkErr != nil {
		return nil, info, fmt.Errorf("数据库无法使用且无法移走原文件: %w（原始错误: %v）", parkErr, cause)
	}
	info.BrokenParkedPath = parked

	db, err := NewDB(dbPath)
	if err != nil {
		return nil, info, fmt.Errorf("重建数据库失败: %w（原始错误: %v，原库备份 %q，现场保留 %q）",
			err, cause, info.BrokenBackupPath, info.BrokenParkedPath)
	}
	if err := migrateWithRetry(db, 2); err != nil {
		_ = db.Close()
		return nil, info, fmt.Errorf("重建数据库迁移失败: %w（原始错误: %v，原库备份 %q，现场保留 %q）",
			err, cause, info.BrokenBackupPath, info.BrokenParkedPath)
	}
	info.Recovered = true
	return db, info, nil
}

// databaseFileHasContent 判断库文件是否已存在且有内容（区分全新安装与存量升级）。
func databaseFileHasContent(dbPath string) bool {
	info, err := os.Stat(dbPath)
	return err == nil && info.Size() > 0
}

// migrateWithRetry 对 database is locked / busy 做有限重试，其余错误立即返回。
func migrateWithRetry(db *DB, attempts int) error {
	if attempts < 1 {
		attempts = 1
	}
	var err error
	for attempt := 0; attempt < attempts; attempt++ {
		err = db.Migrate()
		if err == nil {
			return nil
		}
		if !isDatabaseBusyError(err) || attempt == attempts-1 {
			return err
		}
		time.Sleep(dbRecoveryRetryInterval)
	}
	return err
}

func isDatabaseBusyError(err error) bool {
	if err == nil {
		return false
	}
	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) {
		// SQLITE_BUSY(5) / SQLITE_LOCKED(6)：另一个连接或进程正在写。
		if code := sqliteErr.Code(); code == 5 || code == 6 {
			return true
		}
	}
	// 兜底：错误被跨层转述时按文本判断（避免路径/文案误判依赖错误码优先）。
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "locked") || strings.Contains(message, "busy")
}

// backupDatabaseFiles 按文件复制 db/-wal/-shm，兼容库已损坏无法 VACUUM 的场景。
func backupDatabaseFiles(dbPath string, backupDir string, prefix string) (string, error) {
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return "", err
	}
	stamp := time.Now().Format("20060102-150405")
	dst := filepath.Join(backupDir, fmt.Sprintf("%s-%s-%s", prefix, stamp, filepath.Base(dbPath)))
	if err := copyFileIfExists(dbPath, dst); err != nil {
		return "", err
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		_ = copyFileIfExists(dbPath+suffix, dst+suffix)
	}
	pruneBackups(backupDir)
	return dst, nil
}

func copyFileIfExists(src string, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer source.Close()
	target, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer target.Close()
	if _, err := io.Copy(target, source); err != nil {
		return err
	}
	return target.Sync()
}

// parkDatabaseFiles 把无法使用的库移到同目录 .broken-*，避免新库复用坏文件。
// 原库本来就不存在时返回空路径且不报错。
// Windows 上连接关闭后句柄可能延迟释放，rename 需要少量重试。
func parkDatabaseFiles(dbPath string) (string, error) {
	stamp := time.Now().Format("20060102-150405")
	parked := fmt.Sprintf("%s.broken-%s", dbPath, stamp)
	var err error
	for attempt := 0; attempt < 5; attempt++ {
		err = os.Rename(dbPath, parked)
		if err == nil || os.IsNotExist(err) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil {
		if os.IsNotExist(err) {
			_ = os.Remove(dbPath + "-wal")
			_ = os.Remove(dbPath + "-shm")
			return "", nil
		}
		return "", err
	}
	_ = os.Remove(dbPath + "-wal")
	_ = os.Remove(dbPath + "-shm")
	return parked, nil
}
