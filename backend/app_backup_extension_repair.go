package backend

import (
	"ant-chrome/backend/internal/browser"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	backupManagedExtensionsRoot = "data/extensions"
	backupExtensionBackupsRoot  = "data/extension-backups"
)

type backupExtensionPathRecord struct {
	extensionID string
	installDir  string
	packagePath string
	packageHash string
}

type backupProfileExtensionRuntimePathRecord struct {
	profileID   string
	extensionID string
	backupPath  string
}

func (a *App) backupRepairExtensionPathsAfterImport() ([]error, error) {
	if a == nil || a.db == nil || a.db.GetConn() == nil {
		return nil, fmt.Errorf("数据库未初始化，无法修复插件迁移路径")
	}

	tx, err := a.db.GetConn().Begin()
	if err != nil {
		return nil, fmt.Errorf("开启插件路径修复事务失败: %w", err)
	}
	defer tx.Rollback()

	issues := make([]error, 0)
	extensions, err := backupLoadExtensionPathRecords(tx)
	if err != nil {
		return issues, err
	}
	for _, extension := range extensions {
		if err := a.backupRepairExtensionPathRecord(tx, extension, &issues); err != nil {
			return issues, err
		}
	}

	runtimeStates, err := backupLoadProfileExtensionRuntimePathRecords(tx)
	if err != nil {
		return issues, err
	}
	for _, runtimeState := range runtimeStates {
		if err := a.backupRepairProfileExtensionRuntimePath(tx, runtimeState); err != nil {
			return issues, err
		}
	}

	if err := tx.Commit(); err != nil {
		return issues, fmt.Errorf("提交插件路径修复事务失败: %w", err)
	}
	return issues, nil
}

func backupLoadExtensionPathRecords(tx *sql.Tx) ([]backupExtensionPathRecord, error) {
	rows, err := tx.Query(`
		SELECT extension_id, install_dir, package_path, package_hash
		FROM browser_extensions ORDER BY extension_id`)
	if err != nil {
		return nil, fmt.Errorf("读取插件路径记录失败: %w", err)
	}
	defer rows.Close()

	items := make([]backupExtensionPathRecord, 0)
	for rows.Next() {
		var item backupExtensionPathRecord
		if err := rows.Scan(&item.extensionID, &item.installDir, &item.packagePath, &item.packageHash); err != nil {
			return nil, fmt.Errorf("读取插件路径记录失败: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历插件路径记录失败: %w", err)
	}
	return items, nil
}

func backupLoadProfileExtensionRuntimePathRecords(tx *sql.Tx) ([]backupProfileExtensionRuntimePathRecord, error) {
	rows, err := tx.Query(`
		SELECT profile_id, extension_id, backup_path
		FROM browser_profile_extension_runtime
		ORDER BY profile_id, extension_id`)
	if err != nil {
		return nil, fmt.Errorf("读取插件运行态备份路径失败: %w", err)
	}
	defer rows.Close()

	items := make([]backupProfileExtensionRuntimePathRecord, 0)
	for rows.Next() {
		var item backupProfileExtensionRuntimePathRecord
		if err := rows.Scan(&item.profileID, &item.extensionID, &item.backupPath); err != nil {
			return nil, fmt.Errorf("读取插件运行态备份路径失败: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历插件运行态备份路径失败: %w", err)
	}
	return items, nil
}

func (a *App) backupRepairExtensionPathRecord(tx *sql.Tx, record backupExtensionPathRecord, issues *[]error) error {
	extensionID := browser.NormalizeExtensionID(record.extensionID)
	if extensionID == "" {
		if strings.TrimSpace(record.installDir) != "" || strings.TrimSpace(record.packagePath) != "" {
			*issues = append(*issues, fmt.Errorf("插件 ID 无效，无法重建迁移路径: %q", record.extensionID))
		}
		return nil
	}

	managedRoot := a.resolveAppPath(backupManagedExtensionsRoot)
	installDir := filepath.Join(managedRoot, extensionID)
	packagePath := filepath.Join(managedRoot, "packages", extensionID+".crx")

	newInstallDir := strings.TrimSpace(record.installDir)
	newPackagePath := strings.TrimSpace(record.packagePath)
	newPackageHash := strings.TrimSpace(record.packageHash)
	changed := false

	if backupExtensionManifestExists(installDir) {
		if !backupSamePath(newInstallDir, installDir) {
			newInstallDir = installDir
			changed = true
		}
		if !backupExtensionManifestValid(installDir) {
			*issues = append(*issues, fmt.Errorf("插件 manifest.json 内容损坏(%s): %s", extensionID, filepath.Join(installDir, "manifest.json")))
		}
	} else if backupPathHasSuffix(newInstallDir, filepath.ToSlash(filepath.Join(backupManagedExtensionsRoot, extensionID))) {
		if newInstallDir != "" {
			newInstallDir = ""
			changed = true
		}
	}

	if newPackagePath != "" {
		if backupExtensionRegularFileExists(packagePath) {
			if !backupValidCRXFile(packagePath) {
				if newPackagePath != "" || newPackageHash != "" {
					newPackagePath = ""
					newPackageHash = ""
					changed = true
				}
				*issues = append(*issues, fmt.Errorf("插件包不是合法 CRX(%s): %s", extensionID, packagePath))
			} else {
				packageHash, err := backupSHA256File(packagePath)
				if err != nil {
					return fmt.Errorf("计算插件包哈希失败(%s): %w", extensionID, err)
				}
				if !backupSamePath(newPackagePath, packagePath) || newPackageHash != packageHash {
					newPackagePath = packagePath
					newPackageHash = packageHash
					changed = true
				}
			}
		} else if backupPathHasSuffix(newPackagePath, filepath.ToSlash(filepath.Join(backupManagedExtensionsRoot, "packages", extensionID+".crx"))) {
			newPackagePath = ""
			newPackageHash = ""
			changed = true
		}
	}

	if !changed {
		return nil
	}
	if _, err := tx.Exec(`
		UPDATE browser_extensions
		SET install_dir = ?, package_path = ?, package_hash = ?, updated_at = ?
		WHERE extension_id = ?`,
		newInstallDir,
		newPackagePath,
		newPackageHash,
		time.Now().Format(time.RFC3339),
		record.extensionID,
	); err != nil {
		return fmt.Errorf("更新插件迁移路径失败(%s): %w", extensionID, err)
	}
	return nil
}

func (a *App) backupRepairProfileExtensionRuntimePath(tx *sql.Tx, record backupProfileExtensionRuntimePathRecord) error {
	oldPath := strings.TrimSpace(record.backupPath)
	if oldPath == "" {
		return nil
	}

	mappedPath, recognized := backupMapExtensionBackupPath(a, oldPath)
	newPath := ""
	if recognized && backupPathExists(mappedPath) {
		newPath = mappedPath
	}
	if backupSamePath(oldPath, newPath) {
		return nil
	}
	if _, err := tx.Exec(`
		UPDATE browser_profile_extension_runtime
		SET backup_path = ?, updated_at = ?
		WHERE profile_id = ? AND extension_id = ?`,
		newPath,
		time.Now().Format(time.RFC3339),
		record.profileID,
		record.extensionID,
	); err != nil {
		return fmt.Errorf("更新插件运行态备份路径失败(%s/%s): %w", record.profileID, record.extensionID, err)
	}
	return nil
}

func backupExtensionManifestExists(installDir string) bool {
	return backupExtensionRegularFileExists(filepath.Join(installDir, "manifest.json"))
}

func backupExtensionManifestValid(installDir string) bool {
	data, err := os.ReadFile(filepath.Join(installDir, "manifest.json"))
	if err != nil {
		return false
	}
	var manifest map[string]interface{}
	return json.Unmarshal(data, &manifest) == nil
}

func backupValidCRXFile(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	magic := make([]byte, 4)
	read, err := io.ReadFull(file, magic)
	if err != nil || read != len(magic) {
		return false
	}
	return string(magic) == "Cr24"
}

func backupExtensionRegularFileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func backupPathHasSuffix(path string, suffix string) bool {
	normalizedPath := strings.TrimRight(strings.ReplaceAll(strings.TrimSpace(path), "\\", "/"), "/")
	normalizedSuffix := strings.Trim(strings.ReplaceAll(strings.TrimSpace(suffix), "\\", "/"), "/")
	if normalizedPath == "" || normalizedSuffix == "" {
		return false
	}
	normalizedPath = strings.ToLower(normalizedPath)
	normalizedSuffix = strings.ToLower(normalizedSuffix)
	return normalizedPath == normalizedSuffix || strings.HasSuffix(normalizedPath, "/"+normalizedSuffix)
}

func backupMapExtensionBackupPath(a *App, oldPath string) (string, bool) {
	normalized := strings.TrimRight(strings.ReplaceAll(strings.TrimSpace(oldPath), "\\", "/"), "/")
	lowerNormalized := strings.ToLower(normalized)
	marker := backupExtensionBackupsRoot + "/"
	markerIndex := backupFindPathMarker(lowerNormalized, marker)
	if markerIndex < 0 {
		return "", false
	}

	suffix := normalized[markerIndex+len(marker):]
	if !backupSafeRelativePathSuffix(suffix) {
		return "", false
	}

	backupRoot := a.resolveAppPath(backupExtensionBackupsRoot)
	candidate := filepath.Join(backupRoot, filepath.FromSlash(suffix))
	if !backupPathWithin(candidate, backupRoot) {
		return "", false
	}
	return candidate, true
}

func backupFindPathMarker(path string, marker string) int {
	for offset := 0; offset < len(path); {
		index := strings.Index(path[offset:], marker)
		if index < 0 {
			return -1
		}
		index += offset
		if index == 0 || path[index-1] == '/' {
			return index
		}
		offset = index + 1
	}
	return -1
}

func backupSafeRelativePathSuffix(value string) bool {
	value = strings.Trim(value, "/")
	if value == "" || filepath.IsAbs(filepath.FromSlash(value)) || strings.Contains(value, ":") {
		return false
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	return true
}
