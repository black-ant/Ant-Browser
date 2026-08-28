package backend

import (
	"ant-chrome/backend/internal/backup"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// BackupExportPackage 导出全量配置与数据到 ZIP。
func (a *App) BackupExportPackage() (map[string]interface{}, error) {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()

	if a.ctx == nil {
		return nil, fmt.Errorf("应用上下文未初始化")
	}
	a.backupEmitExportProgress("starting", 0, "正在检查运行中的实例...")
	if runningNames := a.backupRunningProfileNames(); len(runningNames) > 0 {
		message := fmt.Sprintf("请先停止运行中的实例再导出：%s", strings.Join(runningNames, "、"))
		a.backupEmitExportProgress("error", 100, message)
		return nil, fmt.Errorf("%s", message)
	}
	a.backupEmitExportProgress("starting", 2, "等待选择导出路径...")

	defaultName := fmt.Sprintf("ant-chrome-backup-%s.zip", time.Now().Format("20060102-150405"))
	savePath, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "导出全量备份",
		DefaultFilename: defaultName,
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "ZIP 文件 (*.zip)", Pattern: "*.zip"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("打开保存对话框失败: %w", err)
	}
	if strings.TrimSpace(savePath) == "" {
		a.backupEmitExportProgress("cancelled", 0, "已取消导出")
		return map[string]interface{}{
			"cancelled": true,
			"message":   "已取消导出",
		}, nil
	}
	savePath = backupEnsureZipSuffix(savePath)
	a.backupEmitExportProgress("preparing", 8, "正在收集导出范围...")

	scope, err := a.backupBuildScope()
	if err != nil {
		a.backupEmitExportProgress("error", 100, fmt.Sprintf("导出失败: %v", err))
		return nil, err
	}
	manifest := backup.BuildManifest(scope, a.appName(), a.appVersion(), time.Now())
	a.backupEmitExportProgress("preparing", 15, "开始写入备份包...")

	includedEntries, skippedEntries, fileCount, err := backupWritePackageZip(savePath, scope, manifest, a.backupEmitExportProgressMeta)
	if err != nil {
		a.backupEmitExportProgress("error", 100, fmt.Sprintf("导出失败: %v", err))
		return nil, err
	}
	a.backupEmitExportProgress("done", 100, "导出完成")

	return map[string]interface{}{
		"cancelled":       false,
		"zipPath":         savePath,
		"includedEntries": includedEntries,
		"skippedEntries":  skippedEntries,
		"fileCount":       fileCount,
		"message":         "导出完成",
	}, nil
}

// BackupImportPackage 从 ZIP 导入配置与数据，仅支持判重合并导入。
func (a *App) BackupImportPackage() (map[string]interface{}, error) {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()

	if a.ctx == nil {
		return nil, fmt.Errorf("应用上下文未初始化")
	}
	a.backupEmitImportProgress("starting", 0, "等待选择 ZIP 配置文件...")

	zipPath, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "导入全局备份",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "ZIP 文件 (*.zip)", Pattern: "*.zip"},
		},
	})
	if err != nil {
		a.backupEmitImportProgress("error", 100, fmt.Sprintf("打开文件对话框失败: %v", err))
		return nil, fmt.Errorf("打开文件对话框失败: %w", err)
	}
	if strings.TrimSpace(zipPath) == "" {
		a.backupEmitImportProgress("cancelled", 0, "已取消导入")
		return map[string]interface{}{
			"cancelled": true,
			"message":   "已取消导入",
		}, nil
	}
	a.backupEmitImportProgress("preparing", 5, "正在校验备份包...")

	result, importErr := a.backupImportFromPathLocked(zipPath)
	if importErr != nil {
		a.backupEmitImportProgress("error", 100, fmt.Sprintf("导入失败: %v", importErr))
		return nil, importErr
	}
	return result, nil
}

// BackupRestoreLocalPackage 从历史路径恢复本地 ZIP 备份，仅支持判重合并恢复。
func (a *App) BackupRestoreLocalPackage(zipPath string) (map[string]interface{}, error) {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()

	zipPath = strings.TrimSpace(zipPath)
	if zipPath == "" {
		err := fmt.Errorf("本地备份路径为空")
		a.backupEmitImportProgress("error", 100, fmt.Sprintf("本地备份恢复失败: %v", err))
		return nil, err
	}
	if !strings.EqualFold(filepath.Ext(zipPath), ".zip") {
		err := fmt.Errorf("本地备份必须是 ZIP 文件")
		a.backupEmitImportProgress("error", 100, fmt.Sprintf("本地备份恢复失败: %v", err))
		return nil, err
	}
	info, err := os.Stat(zipPath)
	if err != nil {
		if os.IsNotExist(err) {
			err = fmt.Errorf("本地备份文件不存在: %s", zipPath)
		} else {
			err = fmt.Errorf("读取本地备份文件失败: %w", err)
		}
		a.backupEmitImportProgress("error", 100, fmt.Sprintf("本地备份恢复失败: %v", err))
		return nil, err
	}
	if !info.Mode().IsRegular() {
		err := fmt.Errorf("本地备份路径不是普通文件: %s", zipPath)
		a.backupEmitImportProgress("error", 100, fmt.Sprintf("本地备份恢复失败: %v", err))
		return nil, err
	}

	a.backupEmitImportProgress("preparing", 5, "正在校验本地备份包...")
	result, importErr := a.backupImportFromPathLocked(zipPath)
	if importErr != nil {
		a.backupEmitImportProgress("error", 100, fmt.Sprintf("本地备份恢复失败: %v", importErr))
		return nil, importErr
	}
	return result, nil
}
