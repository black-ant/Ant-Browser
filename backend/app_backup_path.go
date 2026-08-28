package backend

import (
	"ant-chrome/backend/internal/fsutil"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// OpenBackupPath 在系统文件管理器中打开本地备份文件并选中该文件。
func (a *App) OpenBackupPath(backupPath string) error {
	resolvedPath, _, err := a.resolveBackupFile(backupPath)
	if err != nil {
		return err
	}

	absPath, err := filepath.Abs(resolvedPath)
	if err != nil {
		return fmt.Errorf("获取备份文件绝对路径失败: %w", err)
	}
	return openPathInFileManager(absPath)
}

func (a *App) GetBackupFileInfo(backupPath string) (map[string]interface{}, error) {
	_, info, err := a.resolveBackupFile(backupPath)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"size":       info.Size(),
		"modifiedAt": info.ModTime().UTC().Format(time.RFC3339Nano),
	}, nil
}

func (a *App) resolveBackupFile(backupPath string) (string, os.FileInfo, error) {
	resolvedPath, err := fsutil.ResolveExistingPath(a.resolveAppPath, backupPath, "备份路径不能为空")
	if err != nil {
		return "", nil, err
	}
	if !strings.EqualFold(filepath.Ext(resolvedPath), ".zip") {
		return "", nil, fmt.Errorf("备份文件必须是 ZIP 文件")
	}

	info, err := os.Stat(resolvedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil, fmt.Errorf("备份文件不存在: %s", resolvedPath)
		}
		return "", nil, fmt.Errorf("读取备份文件失败: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", nil, fmt.Errorf("备份路径不是普通文件: %s", resolvedPath)
	}
	return resolvedPath, info, nil
}
