package backend

import (
	"ant-chrome/backend/internal/config"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func (a *App) backupImportFromPathLocked(zipPath string) (map[string]interface{}, error) {
	a.backupEmitImportProgress("preparing", 10, "正在解压并校验备份包...")
	extractRoot, manifest, err := backupExtractAndValidate(zipPath)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(extractRoot)
	if err := a.backupStopRuntimeForMaintenance(); err != nil {
		return nil, fmt.Errorf("停止运行时失败，已中止备份导入: %w", err)
	}
	runtimeStopped := true
	defer func() {
		if runtimeStopped {
			a.resumeBackgroundTasks()
			a.ensureSpeedScheduler()
		}
	}()
	rollback, err := a.beginBackupImportRollback()
	if err != nil {
		return nil, err
	}
	defer rollback.cleanup()
	a.backupEmitImportProgress("preparing", 20, "备份包校验通过，开始导入数据...")

	componentEntries := backupDetectPresentManifestEntries(extractRoot, manifest)
	issueTracker := newBackupImportTracker(componentEntries)

	stats := &backupMergeStats{}

	payloadRoot := filepath.Join(extractRoot, "payload")
	a.backupEmitImportProgress("importing", 50, "正在解析备份配置...")
	incomingCfg, hasIncomingCfg, err := backupLoadIncomingConfig(payloadRoot)
	if err != nil {
		issueTracker.RecordIssue("system_config_main", "主配置文件", fmt.Errorf("解析配置失败: %w", err))
		incomingCfg = nil
		hasIncomingCfg = false
	}
	if hasIncomingCfg {
		incomingCfg = a.backupNormalizeImportedConfigPaths(incomingCfg, a.config)
	}

	var importMappings *backupImportReferenceMappings
	if dbSrc := backupFindDatabaseFile(payloadRoot); dbSrc != "" {
		a.backupEmitImportProgress("importing", 76, "正在合并数据库数据...")
		importMappings, err = a.backupMergeDatabaseFromSource(dbSrc, incomingCfg, stats)
		if err != nil {
			issueTracker.RecordIssue("database_sqlite_main", "SQLite 主数据库", err)
		}
	} else if _, ok := componentEntries["database_sqlite_main"]; ok {
		issueTracker.RecordIssue("database_sqlite_main", "SQLite 主数据库", fmt.Errorf("备份包缺少数据库文件"))
	}

	if hasIncomingCfg {
		a.backupEmitImportProgress("importing", 82, "正在应用系统配置...")
		if err := a.backupApplyIncomingConfig(incomingCfg); err != nil {
			issueTracker.RecordIssue("system_config_main", "主配置文件", err)
		}
	}

	a.backupEmitImportProgress("importing", 84, "正在合并代理配置...")
	if err := a.backupMergeProxiesFile(payloadRoot, stats); err != nil {
		issueTracker.RecordIssue("system_config_proxies", "代理配置文件", err)
	}
	incomingProxies := make([]config.BrowserProxy, 0)
	if incomingCfg != nil {
		incomingProxies = append(incomingProxies, incomingCfg.Browser.Proxies...)
	}
	if loaded, loadErr := config.LoadProxies(filepath.Join(payloadRoot, "system", "proxies.yaml")); loadErr == nil {
		incomingProxies = append(incomingProxies, loaded...)
	}
	if err := a.backupReconcileProxyReferencesWithDatabase(incomingProxies); err != nil {
		issueTracker.RecordIssue("database_sqlite_main", "SQLite 主数据库", err)
	}

	a.backupEmitImportProgress("importing", 86, "正在同步文件数据...")
	a.backupImportFileTreesWithRollback(payloadRoot, incomingCfg, manifest, stats, issueTracker.RecordIssue, rollback, importMappings)

	a.backupEmitImportProgress("importing", 92, "正在修复插件迁移路径...")
	repairIssues, err := a.backupRepairExtensionPathsAfterImport()
	if err != nil {
		issueTracker.RecordIssue("browser_extension_paths", "插件路径迁移", err)
	}
	for _, issue := range repairIssues {
		issueTracker.RecordIssue("browser_extension_paths", "插件路径迁移", issue)
	}

	totalComponents, successCount, failedCount, partial := issueTracker.Summary()
	if partial {
		rollbackErr := rollback.rollback(a)
		reloadErr := a.backupReloadAfterImportRollback(rollback)
		runtimeStopped = false
		if rollbackErr != nil {
			if reloadErr != nil {
				return nil, backupImportError("备份导入失败且回滚、运行态恢复均失败", errors.Join(rollbackErr, reloadErr))
			}
			return nil, backupImportError("备份导入失败且回滚失败", rollbackErr)
		}
		if reloadErr != nil {
			return nil, backupImportError("备份导入已回滚，但运行态恢复失败", reloadErr)
		}
		message := fmt.Sprintf("导入未应用，已回滚：异常 %d 个模块", failedCount)
		a.backupEmitImportProgress("error", 100, message)
		return map[string]interface{}{
			"cancelled":        false,
			"zipPath":          zipPath,
			"imported":         0,
			"skipped":          stats.Skipped,
			"conflicts":        stats.Conflicts,
			"partial":          false,
			"rolledBack":       true,
			"componentTotal":   totalComponents,
			"componentSuccess": 0,
			"componentFailed":  failedCount,
			"failedComponents": issueTracker.FailedComponents(),
			"message":          message,
		}, nil
	}

	a.backupEmitImportProgress("importing", 94, "正在刷新运行时配置...")
	if err := a.backupReloadAfterMutation(); err != nil {
		rollbackErr := rollback.rollback(a)
		reloadErr := a.backupReloadAfterImportRollback(rollback)
		runtimeStopped = false
		if rollbackErr != nil {
			return nil, backupImportError("备份导入失败且回滚失败", rollbackErr)
		}
		if reloadErr != nil {
			return nil, backupImportError("备份导入已回滚，但运行态恢复失败", reloadErr)
		}
		return nil, backupImportError("备份导入失败，已回滚", err)
	}
	runtimeStopped = false

	message := "导入完成"
	if partial {
		message = fmt.Sprintf("导入完成（部分成功）：成功 %d 个模块，异常 %d 个模块", successCount, failedCount)
	}
	a.backupEmitImportProgress("done", 100, message)

	return map[string]interface{}{
		"cancelled":        false,
		"zipPath":          zipPath,
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
