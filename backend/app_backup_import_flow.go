package backend

import (
	"fmt"
	"os"
	"path/filepath"
)

func (a *App) backupImportFromPathLocked(zipPath string, resetFirst bool) (map[string]interface{}, error) {
	a.backupEmitImportProgress("preparing", 10, "正在解压并校验备份包...")
	extractRoot, manifest, err := backupExtractAndValidate(zipPath)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(extractRoot)
	a.backupStopRuntimeForMaintenance()
	a.backupEmitImportProgress("preparing", 20, "备份包校验通过，开始导入数据...")

	componentEntries := backupDetectPresentManifestEntries(extractRoot, manifest)
	issueTracker := newBackupImportTracker(componentEntries)

	stats := &backupMergeStats{}

	if resetFirst {
		a.backupEmitImportProgress("preparing", 30, "正在初始化系统数据...")
		if _, err := a.backupInitializeLocked(false); err != nil {
			return nil, err
		}
		a.backupEmitImportProgress("preparing", 40, "初始化完成，继续加载备份内容...")
	}

	payloadRoot := filepath.Join(extractRoot, "payload")
	a.backupEmitImportProgress("importing", 50, "正在解析备份配置...")
	incomingCfg, hasIncomingCfg, err := backupLoadIncomingConfig(payloadRoot)
	if err != nil {
		issueTracker.RecordIssue("system_config_main", "主配置文件", fmt.Errorf("解析配置失败: %w", err))
		incomingCfg = nil
		hasIncomingCfg = false
	}
	if resetFirst && !hasIncomingCfg {
		issueTracker.RecordIssue("system_config_main", "主配置文件", fmt.Errorf("备份包缺少 payload/system/config.yaml，已保留默认配置继续加载其余模块"))
	}

	if hasIncomingCfg {
		a.backupEmitImportProgress("importing", 58, "正在应用系统配置...")
		incomingCfg = a.backupNormalizeImportedConfigPaths(incomingCfg, a.config)
		if err := a.backupApplyIncomingConfig(incomingCfg, resetFirst); err != nil {
			issueTracker.RecordIssue("system_config_main", "主配置文件", err)
		}
	}

	a.backupEmitImportProgress("importing", 66, "正在合并代理配置...")
	if err := a.backupMergeProxiesFile(payloadRoot, resetFirst, stats); err != nil {
		issueTracker.RecordIssue("system_config_proxies", "代理配置文件", err)
	}

	if dbSrc := backupFindDatabaseFile(payloadRoot); dbSrc != "" {
		a.backupEmitImportProgress("importing", 76, "正在合并数据库数据...")
		if err := a.backupMergeDatabaseFromSource(dbSrc, resetFirst, stats); err != nil {
			issueTracker.RecordIssue("database_sqlite_main", "SQLite 主数据库", err)
		}
	} else if _, ok := componentEntries["database_sqlite_main"]; ok {
		issueTracker.RecordIssue("database_sqlite_main", "SQLite 主数据库", fmt.Errorf("备份包缺少数据库文件"))
	}

	a.backupEmitImportProgress("importing", 86, "正在同步文件数据...")
	a.backupImportFileTrees(payloadRoot, incomingCfg, resetFirst, stats, issueTracker.RecordIssue)

	a.backupEmitImportProgress("importing", 92, "正在修复插件迁移路径...")
	if err := a.backupRepairExtensionPathsAfterImport(); err != nil {
		issueTracker.RecordIssue("browser_extension_paths", "插件路径迁移", err)
	}

	a.backupEmitImportProgress("importing", 94, "正在刷新运行时配置...")
	if err := a.backupReloadAfterMutation(); err != nil {
		return nil, err
	}

	totalComponents, successCount, failedCount, partial := issueTracker.Summary()
	message := "导入完成"
	if partial {
		message = fmt.Sprintf("导入完成（部分成功）：成功 %d 个模块，异常 %d 个模块", successCount, failedCount)
	}
	a.backupEmitImportProgress("done", 100, message)

	return map[string]interface{}{
		"cancelled":        false,
		"zipPath":          zipPath,
		"resetFirst":       resetFirst,
		"imported":         stats.Imported,
		"skipped":          stats.Skipped,
		"conflicts":        stats.Conflicts,
		"partial":          partial,
		"componentTotal":   totalComponents,
		"componentSuccess": successCount,
		"componentFailed":  failedCount,
		"failedComponents": issueTracker.FailedComponents(),
		"message":          message,
	}, nil
}
