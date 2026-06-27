package backend

import (
	"ant-chrome/backend/internal/backup"
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/logger"
	"ant-chrome/backend/internal/proxy"
	"archive/zip"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// BackupInitializeSystem 初始化系统到最开始状态。
func (a *App) BackupInitializeSystem() (map[string]interface{}, error) {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()

	return a.backupInitializeLocked(true)
}

// BackupExportPackage 导出全量配置与数据到 ZIP。
func (a *App) BackupExportPackage() (map[string]interface{}, error) {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()

	if a.ctx == nil {
		return nil, fmt.Errorf("应用上下文未初始化")
	}
	a.backupEmitExportProgress("starting", 0, "等待选择导出路径...")

	defaultName := fmt.Sprintf("ant-chrome-backup-%s.zip", time.Now().Format("20060102-150405"))
	savePath, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "导出配置",
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

	scope, err := backup.BuildScope(backup.BuildOptions{AppRoot: a.appRoot, Config: a.config})
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

	return map[string]interface{}{
		"cancelled":       false,
		"zipPath":         savePath,
		"includedEntries": includedEntries,
		"skippedEntries":  skippedEntries,
		"fileCount":       fileCount,
		"message":         "导出完成",
	}, nil
}

// BackupImportPackage 从 ZIP 加载配置与数据。
// resetFirst=true: 先初始化，再全量导入。
// resetFirst=false: 直接导入并执行判重合并。
func (a *App) BackupImportPackage(resetFirst bool) (map[string]interface{}, error) {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()

	if a.ctx == nil {
		return nil, fmt.Errorf("应用上下文未初始化")
	}
	a.backupEmitImportProgress("starting", 0, "等待选择 ZIP 配置文件...")

	zipPath, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "加载配置",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "ZIP 文件 (*.zip)", Pattern: "*.zip"},
		},
	})
	if err != nil {
		a.backupEmitImportProgress("error", 100, fmt.Sprintf("打开文件对话框失败: %v", err))
		return nil, fmt.Errorf("打开文件对话框失败: %w", err)
	}
	if strings.TrimSpace(zipPath) == "" {
		a.backupEmitImportProgress("cancelled", 0, "已取消加载")
		return map[string]interface{}{
			"cancelled": true,
			"message":   "已取消加载",
		}, nil
	}
	a.backupEmitImportProgress("preparing", 5, "正在校验备份包...")

	result, importErr := a.backupImportFromPathLocked(zipPath, resetFirst)
	if importErr != nil {
		a.backupEmitImportProgress("error", 100, fmt.Sprintf("加载失败: %v", importErr))
		return nil, importErr
	}
	return result, nil
}

type backupMergeStats struct {
	Imported  int
	Skipped   int
	Conflicts int
}

type backupImportIssue struct {
	ComponentID   string `json:"componentId"`
	ComponentName string `json:"componentName"`
	Error         string `json:"error"`
}

type backupProgressMeta struct {
	ComponentID   string
	ComponentName string
	EntryIndex    int
	EntryTotal    int
}

type backupProgressEvent struct {
	Phase         string `json:"phase"`
	Progress      int    `json:"progress"`
	Message       string `json:"message"`
	ComponentID   string `json:"componentId,omitempty"`
	ComponentName string `json:"componentName,omitempty"`
	EntryIndex    int    `json:"entryIndex,omitempty"`
	EntryTotal    int    `json:"entryTotal,omitempty"`
	Timestamp     string `json:"timestamp,omitempty"`
}

func (a *App) backupEmitExportProgress(phase string, progress int, message string) {
	a.backupEmitExportProgressMeta(phase, progress, message, nil)
}

func (a *App) backupEmitExportProgressMeta(phase string, progress int, message string, meta *backupProgressMeta) {
	a.backupEmitProgress("backup:export:progress", phase, progress, message, meta)
}

func (a *App) backupEmitImportProgress(phase string, progress int, message string) {
	a.backupEmitImportProgressMeta(phase, progress, message, nil)
}

func (a *App) backupEmitImportProgressMeta(phase string, progress int, message string, meta *backupProgressMeta) {
	a.backupEmitProgress("backup:import:progress", phase, progress, message, meta)
}

func (a *App) backupEmitProgress(eventName, phase string, progress int, message string, meta *backupProgressMeta) {
	if a == nil || a.ctx == nil {
		return
	}
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	evt := backupProgressEvent{
		Phase:     strings.TrimSpace(phase),
		Progress:  progress,
		Message:   strings.TrimSpace(message),
		Timestamp: time.Now().Format("15:04:05"),
	}
	if meta != nil {
		evt.ComponentID = strings.TrimSpace(meta.ComponentID)
		evt.ComponentName = strings.TrimSpace(meta.ComponentName)
		evt.EntryIndex = meta.EntryIndex
		evt.EntryTotal = meta.EntryTotal
	}
	wailsruntime.EventsEmit(a.ctx, eventName, backupProgressEvent{
		Phase:         evt.Phase,
		Progress:      evt.Progress,
		Message:       evt.Message,
		ComponentID:   evt.ComponentID,
		ComponentName: evt.ComponentName,
		EntryIndex:    evt.EntryIndex,
		EntryTotal:    evt.EntryTotal,
		Timestamp:     evt.Timestamp,
	})
}

func (a *App) backupInitializeLocked(applyReload bool) (map[string]interface{}, error) {
	log := logger.New("Backup")
	a.backupStopRuntimeForMaintenance()

	defaultCfg := config.DefaultConfig()
	oldCfg := a.config
	if oldCfg == nil {
		oldCfg = config.DefaultConfig()
	}
	activeDBPath := a.backupResolveDBPath(oldCfg)
	keepFiles := map[string]struct{}{
		backupNormalizePath(activeDBPath):          {},
		backupNormalizePath(activeDBPath + "-wal"): {},
		backupNormalizePath(activeDBPath + "-shm"): {},
	}

	if err := defaultCfg.Save(a.resolveAppPath("config.yaml")); err != nil {
		return nil, fmt.Errorf("写入默认配置失败: %w", err)
	}
	a.config = defaultCfg
	a.applyRuntimeConfig(defaultCfg.Runtime)
	_ = os.Remove(a.resolveAppPath("proxies.yaml"))

	if err := a.backupClearBusinessTables(); err != nil {
		return nil, err
	}

	cleared := make([]string, 0, 3)
	dataRoot := a.resolveAppPath("data")
	if err := backupRemoveContentsExcept(dataRoot, keepFiles); err == nil {
		cleared = append(cleared, dataRoot)
	}
	oldUserRoot := a.backupResolveUserDataRoot(oldCfg)
	newUserRoot := a.backupResolveUserDataRoot(defaultCfg)
	for _, p := range backupUniqueNonEmpty([]string{oldUserRoot, newUserRoot}) {
		if backupSamePath(p, dataRoot) {
			continue
		}
		if err := backupRemoveContentsExcept(p, keepFiles); err == nil {
			cleared = append(cleared, p)
		}
	}

	if applyReload {
		if err := a.backupReloadAfterMutation(); err != nil {
			return nil, err
		}
	}

	log.Info("系统初始化完成", logger.F("cleared_dirs", strings.Join(cleared, ";")))
	return map[string]interface{}{
		"cancelled":   false,
		"resetDone":   true,
		"clearedDirs": cleared,
		"message":     "系统已初始化到默认状态",
	}, nil
}

func (a *App) backupImportFromPathLocked(zipPath string, resetFirst bool) (map[string]interface{}, error) {
	a.backupStopRuntimeForMaintenance()
	a.backupEmitImportProgress("preparing", 10, "正在解压并校验备份包...")

	extractRoot, manifest, err := backupExtractAndValidate(zipPath)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(extractRoot)
	a.backupEmitImportProgress("preparing", 20, "备份包校验通过，开始加载数据...")

	componentEntries := backupDetectPresentManifestEntries(extractRoot, manifest)
	componentUniverse := make(map[string]struct{}, len(componentEntries))
	for id := range componentEntries {
		componentUniverse[id] = struct{}{}
	}
	failedComponentIDs := map[string]struct{}{}
	issues := make([]backupImportIssue, 0)
	recordIssue := func(componentID, componentName string, err error) {
		if err == nil {
			return
		}
		componentID = strings.TrimSpace(componentID)
		componentName = strings.TrimSpace(componentName)
		if componentID != "" {
			componentUniverse[componentID] = struct{}{}
			failedComponentIDs[componentID] = struct{}{}
			if componentName == "" {
				if entry, ok := componentEntries[componentID]; ok {
					componentName = backupResolveManifestComponentName(entry)
				}
			}
		}
		if componentName == "" {
			componentName = "未知模块"
		}
		issues = append(issues, backupImportIssue{
			ComponentID:   componentID,
			ComponentName: componentName,
			Error:         err.Error(),
		})
	}

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
		recordIssue("system_config_main", "主配置文件", fmt.Errorf("解析配置失败: %w", err))
		incomingCfg = nil
		hasIncomingCfg = false
	}
	if resetFirst && !hasIncomingCfg {
		recordIssue("system_config_main", "主配置文件", fmt.Errorf("备份包缺少 payload/system/config.yaml，已保留默认配置继续加载其余模块"))
	}

	if hasIncomingCfg {
		a.backupEmitImportProgress("importing", 58, "正在应用系统配置...")
		if err := a.backupApplyIncomingConfig(incomingCfg, resetFirst); err != nil {
			recordIssue("system_config_main", "主配置文件", err)
		}
	}

	a.backupEmitImportProgress("importing", 66, "正在合并代理配置...")
	if err := a.backupMergeProxiesFile(payloadRoot, resetFirst, stats); err != nil {
		recordIssue("system_config_proxies", "代理配置文件", err)
	}

	if dbSrc := backupFindDatabaseFile(payloadRoot); dbSrc != "" {
		a.backupEmitImportProgress("importing", 76, "正在合并数据库数据...")
		if err := a.backupMergeDatabaseFromSource(dbSrc, resetFirst, stats); err != nil {
			recordIssue("database_sqlite_main", "SQLite 主数据库", err)
		}
	} else if _, ok := componentEntries["database_sqlite_main"]; ok {
		recordIssue("database_sqlite_main", "SQLite 主数据库", fmt.Errorf("备份包缺少数据库文件"))
	}

	a.backupEmitImportProgress("importing", 86, "正在同步文件数据...")
	a.backupImportFileTrees(payloadRoot, incomingCfg, resetFirst, stats, recordIssue)

	a.backupEmitImportProgress("importing", 94, "正在刷新运行时配置...")
	if err := a.backupReloadAfterMutation(); err != nil {
		return nil, err
	}

	totalComponents := len(componentUniverse)
	failedCount := len(failedComponentIDs)
	successCount := totalComponents - failedCount
	if successCount < 0 {
		successCount = 0
	}
	partial := failedCount > 0
	message := "加载完成"
	if partial {
		message = fmt.Sprintf("加载完成（部分成功）：成功 %d 个模块，异常 %d 个模块", successCount, failedCount)
	}
	a.backupEmitImportProgress("done", 100, message)

	failedComponents := make([]map[string]string, 0, len(issues))
	for _, item := range issues {
		failedComponents = append(failedComponents, map[string]string{
			"componentId":   item.ComponentID,
			"componentName": item.ComponentName,
			"error":         item.Error,
		})
	}

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
		"failedComponents": failedComponents,
		"message":          message,
	}, nil
}

func (a *App) backupStopRuntimeForMaintenance() {
	if a.browserMgr != nil {
		a.browserMgr.Mutex.Lock()
		for _, cmd := range a.browserMgr.BrowserProcesses {
			if cmd != nil && cmd.Process != nil {
				_ = a.stopProcessCmd(cmd)
			}
		}
		a.browserMgr.BrowserProcesses = make(map[string]*exec.Cmd)
		a.browserMgr.Mutex.Unlock()
	}

	if a.xrayMgr != nil {
		a.xrayMgr.StopAll()
	}
	a.clearProfileXrayBridges()
	if a.singboxMgr != nil {
		a.singboxMgr.StopAll()
	}
	if a.speedScheduler != nil {
		a.speedScheduler.Stop()
		a.speedScheduler = nil
	}
	if a.proxySourceScheduler != nil {
		a.proxySourceScheduler.Stop()
		a.proxySourceScheduler = nil
	}
}

func (a *App) backupReloadAfterMutation() error {
	if err := a.ReloadConfig(); err != nil {
		return err
	}

	if a.browserMgr != nil {
		a.browserMgr.Config = a.config
		a.browserMgr.Mutex.Lock()
		a.browserMgr.Profiles = make(map[string]*browser.Profile)
		a.browserMgr.BrowserProcesses = make(map[string]*exec.Cmd)
		a.browserMgr.XrayBridges = make(map[string]*browser.XrayBridge)
		a.browserMgr.Mutex.Unlock()
	}
	if a.xrayMgr != nil {
		a.xrayMgr.Config = a.config
	}
	if a.clashMgr != nil {
		a.clashMgr.Config = a.config
	}
	if a.singboxMgr != nil {
		a.singboxMgr.Config = a.config
	}

	a.migrateToSQLite()
	if a.browserMgr != nil {
		a.browserMgr.InitData()
	}
	a.autoDetectCores()
	a.loadProxies()

	if a.launchCodeSvc != nil {
		_ = a.launchCodeSvc.LoadAll()
	}
	if a.browserMgr != nil {
		a.browserMgr.CodeProvider = a.launchCodeSvc
	}

	if a.browserMgr != nil && a.browserMgr.ProxyDAO != nil {
		a.speedScheduler = browser.NewProxySpeedScheduler(
			a.browserMgr.ProxyDAO,
			func(proxyID string) (bool, int64, string) {
				r := proxy.SpeedTest(proxyID, a.config.Browser.Proxies, a.xrayMgr, a.singboxMgr, nil)
				return r.Ok, r.LatencyMs, r.Error
			},
			5*time.Minute,
			5,
		)
		a.speedScheduler.Start()
	}

	// 恢复后重新聚合订阅源并重启自动刷新调度器
	if a.browserMgr != nil && a.browserMgr.ProxySourceDAO != nil {
		a.seedProxySourcesFromProxies()
		a.proxySourceScheduler = browser.NewProxySourceScheduler(
			a.browserMgr.ProxySourceDAO,
			a.refreshProxySource,
			60*time.Second,
		)
		a.proxySourceScheduler.Start()
	}
	return nil
}
func (a *App) backupResolveDBPath(cfg *config.Config) string {
	if cfg == nil {
		return a.resolveAppPath("data/app.db")
	}
	path := strings.TrimSpace(cfg.Database.SQLite.Path)
	if path == "" {
		path = "data/app.db"
	}
	return a.resolveAppPath(path)
}

func (a *App) backupResolveUserDataRoot(cfg *config.Config) string {
	if cfg == nil {
		return a.resolveAppPath("data")
	}
	root := strings.TrimSpace(cfg.Browser.UserDataRoot)
	if root == "" {
		root = "data"
	}
	return a.resolveAppPath(root)
}

func (a *App) backupClearBusinessTables() error {
	if a.db == nil || a.db.GetConn() == nil {
		return fmt.Errorf("数据库未初始化")
	}
	tx, err := a.db.GetConn().Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer tx.Rollback()

	tables := []string{"launch_codes", "browser_profiles", "browser_proxies", "browser_cores", "browser_bookmarks", "browser_groups", "browser_templates"}
	for _, table := range tables {
		if _, err := tx.Exec("DELETE FROM " + table); err != nil && !backupIsNoSuchTableError(err) {
			return fmt.Errorf("清空数据表失败(%s): %w", table, err)
		}
	}
	_, _ = tx.Exec(`DELETE FROM sqlite_sequence WHERE name IN ('browser_bookmarks')`)
	return tx.Commit()
}

func backupWritePackageZip(zipPath string, scope backup.Scope, manifest backup.Manifest, emitProgress func(phase string, progress int, message string, meta *backupProgressMeta)) (int, int, int, error) {
	emit := func(phase string, progress int, message string, meta *backupProgressMeta) {
		if emitProgress != nil {
			emitProgress(phase, progress, message, meta)
		}
	}
	if err := os.MkdirAll(filepath.Dir(zipPath), 0755); err != nil {
		return 0, 0, 0, fmt.Errorf("创建导出目录失败: %w", err)
	}
	emit("writing", 18, "正在创建导出文件...", nil)

	tmpPath := zipPath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("创建导出文件失败: %w", err)
	}
	w := zip.NewWriter(f)

	includedEntries := 0
	skippedEntries := 0
	fileCount := 0

	writeErr := func() error {
		emit("writing", 20, "正在写入备份清单...", nil)
		manifestData, err := json.MarshalIndent(manifest, "", "  ")
		if err != nil {
			return err
		}
		mw, err := w.Create("manifest.json")
		if err != nil {
			return err
		}
		if _, err := mw.Write(manifestData); err != nil {
			return err
		}
		fileCount++

		totalEntries := len(scope.Entries)
		if totalEntries == 0 {
			emit("writing", 90, "没有可导出的目录条目", nil)
		}
		for i, entry := range scope.Entries {
			meta := &backupProgressMeta{
				ComponentID:   entry.ID,
				ComponentName: backupResolveEntryComponentName(entry),
				EntryIndex:    i + 1,
				EntryTotal:    totalEntries,
			}
			startProgress := 20 + int(float64(i)/float64(totalEntries)*70)
			emit("writing", startProgress, fmt.Sprintf("开始处理组件 %d/%d：%s", i+1, totalEntries, meta.ComponentName), meta)

			info, err := os.Stat(entry.SourcePath)
			if err != nil {
				if os.IsNotExist(err) && !entry.Required {
					skippedEntries++
					progress := 20 + int(float64(i+1)/float64(totalEntries)*70)
					emit("writing", progress, fmt.Sprintf("组件跳过：%s（源路径不存在）", meta.ComponentName), meta)
					continue
				}
				return fmt.Errorf("读取导出源失败(%s): %w", entry.ID, err)
			}
			entryAddedFiles := 0
			if info.IsDir() {
				n, err := backupZipAddDir(w, entry.SourcePath, entry.ArchivePath, zipPath)
				if err != nil {
					return fmt.Errorf("写入目录失败(%s): %w", entry.ID, err)
				}
				fileCount += n
				entryAddedFiles = n
			} else {
				if backupSamePath(entry.SourcePath, zipPath) {
					skippedEntries++
					progress := 20 + int(float64(i+1)/float64(totalEntries)*70)
					emit("writing", progress, fmt.Sprintf("组件跳过：%s（导出文件本身）", meta.ComponentName), meta)
					continue
				}
				if err := backupZipAddFile(w, entry.SourcePath, strings.TrimSuffix(entry.ArchivePath, "/")); err != nil {
					return fmt.Errorf("写入文件失败(%s): %w", entry.ID, err)
				}
				fileCount++
				entryAddedFiles = 1
			}
			includedEntries++
			progress := 20 + int(float64(i+1)/float64(totalEntries)*70)
			emit("writing", progress, fmt.Sprintf("组件完成：%s（新增 %d 个文件）", meta.ComponentName, entryAddedFiles), meta)
		}
		return nil
	}()

	closeErr := w.Close()
	fileCloseErr := f.Close()
	if writeErr != nil {
		emit("error", 100, writeErr.Error(), nil)
		_ = os.Remove(tmpPath)
		return 0, 0, 0, writeErr
	}
	if closeErr != nil {
		emit("error", 100, closeErr.Error(), nil)
		_ = os.Remove(tmpPath)
		return 0, 0, 0, closeErr
	}
	if fileCloseErr != nil {
		emit("error", 100, fileCloseErr.Error(), nil)
		_ = os.Remove(tmpPath)
		return 0, 0, 0, fileCloseErr
	}
	if err := os.Rename(tmpPath, zipPath); err != nil {
		emit("error", 100, err.Error(), nil)
		_ = os.Remove(tmpPath)
		return 0, 0, 0, fmt.Errorf("写入导出文件失败: %w", err)
	}
	emit("done", 100, "导出完成", nil)
	return includedEntries, skippedEntries, fileCount, nil
}

func backupResolveEntryComponentName(entry backup.ScopeEntry) string {
	if desc := strings.TrimSpace(entry.Description); desc != "" {
		return desc
	}
	if entry.ID != "" {
		return entry.ID
	}
	switch entry.Category {
	case backup.CategorySystemConfig:
		return "系统配置"
	case backup.CategoryAppData:
		return "应用数据"
	case backup.CategoryBrowserData:
		return "浏览器数据"
	case backup.CategoryCoreData:
		return "内核数据"
	case backup.CategoryLogs:
		return "日志数据"
	default:
		return "未知组件"
	}
}

func backupZipAddDir(w *zip.Writer, srcDir, archiveBase, outputZipPath string) (int, error) {
	base := strings.TrimSuffix(filepath.ToSlash(strings.TrimSpace(archiveBase)), "/")
	if base == "" {
		return 0, fmt.Errorf("archive base 不能为空")
	}
	fileCount := 0
	err := filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if backupSamePath(path, outputZipPath) {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		targetName := base + "/" + rel
		if d.IsDir() {
			_, err := w.Create(strings.TrimSuffix(targetName, "/") + "/")
			return err
		}
		if err := backupZipAddFile(w, path, targetName); err != nil {
			return err
		}
		fileCount++
		return nil
	})
	return fileCount, err
}

func backupZipAddFile(w *zip.Writer, srcFile, archivePath string) error {
	info, err := os.Stat(srcFile)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("不支持将目录按文件写入: %s", srcFile)
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = strings.TrimPrefix(filepath.ToSlash(strings.TrimSpace(archivePath)), "/")
	header.Method = zip.Deflate
	if header.Name == "" {
		return fmt.Errorf("archivePath 不能为空")
	}
	writer, err := w.CreateHeader(header)
	if err != nil {
		return err
	}
	in, err := os.Open(srcFile)
	if err != nil {
		return err
	}
	defer in.Close()
	_, err = io.Copy(writer, in)
	return err
}

func backupExtractAndValidate(zipPath string) (string, backup.Manifest, error) {
	tmpDir, err := os.MkdirTemp("", "ant-chrome-import-*")
	if err != nil {
		return "", backup.Manifest{}, err
	}
	if err := unzipTo(zipPath, tmpDir); err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", backup.Manifest{}, fmt.Errorf("解压备份包失败: %w", err)
	}

	manifestPath := filepath.Join(tmpDir, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", backup.Manifest{}, fmt.Errorf("备份包缺少 manifest.json")
	}
	var manifest backup.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", backup.Manifest{}, fmt.Errorf("manifest.json 解析失败: %w", err)
	}
	if manifest.Format != backup.PackageFormat {
		_ = os.RemoveAll(tmpDir)
		return "", backup.Manifest{}, fmt.Errorf("不支持的备份格式: %s", manifest.Format)
	}
	if manifest.ManifestVersion != backup.ManifestVersion {
		_ = os.RemoveAll(tmpDir)
		return "", backup.Manifest{}, fmt.Errorf("不支持的 manifest 版本: %d", manifest.ManifestVersion)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "payload")); err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", backup.Manifest{}, fmt.Errorf("备份包缺少 payload 目录")
	}
	return tmpDir, manifest, nil
}

func backupLoadIncomingConfig(payloadRoot string) (*config.Config, bool, error) {
	cfgPath := filepath.Join(payloadRoot, "system", "config.yaml")
	if _, err := os.Stat(cfgPath); err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, false, err
	}
	return cfg, true, nil
}

func backupDetectPresentManifestEntries(extractRoot string, manifest backup.Manifest) map[string]backup.ManifestEntry {
	result := make(map[string]backup.ManifestEntry, len(manifest.Entries))
	for _, entry := range manifest.Entries {
		id := strings.TrimSpace(entry.ID)
		if id == "" {
			continue
		}
		archivePath := strings.TrimSpace(strings.TrimSuffix(entry.ArchivePath, "/"))
		if archivePath == "" {
			continue
		}
		absPath := filepath.Join(extractRoot, filepath.FromSlash(archivePath))
		if _, err := os.Stat(absPath); err == nil {
			result[id] = entry
		}
	}
	return result
}

func backupResolveManifestComponentName(entry backup.ManifestEntry) string {
	if desc := strings.TrimSpace(entry.Description); desc != "" {
		return desc
	}
	if id := strings.TrimSpace(entry.ID); id != "" {
		return id
	}
	return "未知模块"
}

func (a *App) backupApplyIncomingConfig(incoming *config.Config, resetFirst bool) error {
	if incoming == nil {
		return nil
	}
	current := a.config
	if current == nil {
		current = config.DefaultConfig()
	}

	var target *config.Config
	if resetFirst {
		cloned := *incoming
		target = &cloned
	} else {
		target = backupMergeConfig(current, incoming)
	}
	target.Database = current.Database
	target.App.MaxProfileLimit = current.App.MaxProfileLimit
	target.App.UsedCDKeys = append([]string{}, current.App.UsedCDKeys...)

	if err := target.Save(a.resolveAppPath("config.yaml")); err != nil {
		return fmt.Errorf("保存导入配置失败: %w", err)
	}
	a.config = target
	a.applyRuntimeConfig(target.Runtime)
	return nil
}

func backupMergeConfig(current, incoming *config.Config) *config.Config {
	if current == nil {
		cp := *incoming
		return &cp
	}
	if incoming == nil {
		cp := *current
		return &cp
	}
	merged := *current
	if strings.TrimSpace(merged.App.Name) == "" {
		merged.App.Name = incoming.App.Name
	}
	merged.Browser.DefaultBookmarks = backupMergeBookmarks(merged.Browser.DefaultBookmarks, incoming.Browser.DefaultBookmarks)
	merged.Browser.Cores = backupMergeCores(merged.Browser.Cores, incoming.Browser.Cores)
	merged.Browser.Proxies = backupMergeProxies(merged.Browser.Proxies, incoming.Browser.Proxies)
	merged.Browser.Profiles = backupMergeProfiles(merged.Browser.Profiles, incoming.Browser.Profiles)
	return &merged
}
func (a *App) backupMergeProxiesFile(payloadRoot string, resetFirst bool, stats *backupMergeStats) error {
	srcPath := filepath.Join(payloadRoot, "system", "proxies.yaml")
	dstPath := a.resolveAppPath("proxies.yaml")

	if _, err := os.Stat(srcPath); err != nil {
		if os.IsNotExist(err) {
			if resetFirst {
				_ = os.Remove(dstPath)
			}
			return nil
		}
		return err
	}

	if resetFirst {
		return backupCopyFile(srcPath, dstPath)
	}

	incoming, err := config.LoadProxies(srcPath)
	if err != nil {
		return err
	}
	current, err := config.LoadProxies(dstPath)
	if err != nil {
		return err
	}

	merged := append([]config.BrowserProxy{}, current...)
	existingID := make(map[string]struct{}, len(current))
	existingCfg := make(map[string]struct{}, len(current))
	for _, p := range current {
		existingID[strings.ToLower(strings.TrimSpace(p.ProxyId))] = struct{}{}
		existingCfg[strings.ToLower(strings.TrimSpace(p.ProxyConfig))] = struct{}{}
	}
	for _, p := range incoming {
		idKey := strings.ToLower(strings.TrimSpace(p.ProxyId))
		cfgKey := strings.ToLower(strings.TrimSpace(p.ProxyConfig))
		if _, ok := existingID[idKey]; ok {
			stats.Skipped++
			continue
		}
		if cfgKey != "" {
			if _, ok := existingCfg[cfgKey]; ok {
				stats.Skipped++
				continue
			}
		}
		merged = append(merged, p)
		existingID[idKey] = struct{}{}
		if cfgKey != "" {
			existingCfg[cfgKey] = struct{}{}
		}
		stats.Imported++
	}

	return config.SaveProxies(dstPath, merged)
}

func backupFindDatabaseFile(payloadRoot string) string {
	candidates := []string{
		filepath.Join(payloadRoot, "app", "database", "app.db"),
		filepath.Join(payloadRoot, "app", "data", "app.db"),
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

func (a *App) backupMergeDatabaseFromSource(srcDBPath string, resetFirst bool, stats *backupMergeStats) error {
	if a.db == nil || a.db.GetConn() == nil {
		return fmt.Errorf("数据库未初始化")
	}
	tx, err := a.db.GetConn().Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`ATTACH DATABASE ? AS src`, srcDBPath); err != nil {
		return fmt.Errorf("挂载备份数据库失败: %w", err)
	}
	defer tx.Exec(`DETACH DATABASE src`)

	profileConfigInsertAll := "'{}'"
	profileConfigInsertSafe := "'{}'"
	if exists, err := backupSrcColumnExists(tx, "browser_profiles", "profile_config"); err != nil {
		return err
	} else if exists {
		profileConfigInsertAll = "COALESCE(profile_config,'{}')"
		profileConfigInsertSafe = "COALESCE(s.profile_config,'{}')"
	}

	mergeTables := []struct {
		name       string
		insertAll  string
		insertSafe string
	}{
		{
			name: "browser_templates",
			insertAll: `INSERT INTO browser_templates (template_id, template_name, profile_config, created_at, updated_at)
SELECT template_id, template_name, COALESCE(profile_config,'{}'), created_at, updated_at FROM src.browser_templates`,
			insertSafe: `INSERT INTO browser_templates (template_id, template_name, profile_config, created_at, updated_at)
SELECT s.template_id, s.template_name, COALESCE(s.profile_config,'{}'), s.created_at, s.updated_at
FROM src.browser_templates s
WHERE NOT EXISTS (
  SELECT 1 FROM browser_templates t
  WHERE t.template_id = s.template_id OR lower(t.template_name) = lower(s.template_name)
)`,
		},
		{
			name: "browser_groups",
			insertAll: `INSERT INTO browser_groups (group_id, group_name, parent_id, sort_order, created_at, updated_at)
SELECT group_id, group_name, parent_id, sort_order, created_at, updated_at FROM src.browser_groups`,
			insertSafe: `INSERT INTO browser_groups (group_id, group_name, parent_id, sort_order, created_at, updated_at)
SELECT s.group_id, s.group_name, s.parent_id, s.sort_order, s.created_at, s.updated_at
FROM src.browser_groups s
WHERE NOT EXISTS (
  SELECT 1 FROM browser_groups t
  WHERE t.group_id = s.group_id OR (t.parent_id = s.parent_id AND lower(t.group_name) = lower(s.group_name))
)`,
		},
		{
			name: "browser_cores",
			insertAll: `INSERT INTO browser_cores (core_id, core_name, core_path, is_default, sort_order, created_at)
SELECT core_id, core_name, core_path, is_default, sort_order, created_at FROM src.browser_cores`,
			insertSafe: `INSERT INTO browser_cores (core_id, core_name, core_path, is_default, sort_order, created_at)
SELECT s.core_id, s.core_name, s.core_path, s.is_default, s.sort_order, s.created_at
FROM src.browser_cores s
WHERE NOT EXISTS (
  SELECT 1 FROM browser_cores t
  WHERE t.core_id = s.core_id OR lower(t.core_path) = lower(s.core_path)
)`,
		},
		{
			name: "browser_proxies",
			insertAll: `INSERT INTO browser_proxies (proxy_id, proxy_name, proxy_config, dns_servers, group_name, source_id, source_url, source_name_prefix, source_auto_refresh, source_refresh_interval_m, source_last_refresh_at, last_latency_ms, last_test_ok, last_tested_at, last_ip_health_json, sort_order, created_at)
SELECT proxy_id, proxy_name, proxy_config, dns_servers, COALESCE(group_name,''), COALESCE(source_id,''), COALESCE(source_url,''), COALESCE(source_name_prefix,''), COALESCE(source_auto_refresh,0), COALESCE(source_refresh_interval_m,0), COALESCE(source_last_refresh_at,''), COALESCE(last_latency_ms,-1), COALESCE(last_test_ok,0), COALESCE(last_tested_at,''), COALESCE(last_ip_health_json,''), sort_order, created_at
FROM src.browser_proxies`,
			insertSafe: `INSERT INTO browser_proxies (proxy_id, proxy_name, proxy_config, dns_servers, group_name, source_id, source_url, source_name_prefix, source_auto_refresh, source_refresh_interval_m, source_last_refresh_at, last_latency_ms, last_test_ok, last_tested_at, last_ip_health_json, sort_order, created_at)
SELECT s.proxy_id, s.proxy_name, s.proxy_config, s.dns_servers, COALESCE(s.group_name,''), COALESCE(s.source_id,''), COALESCE(s.source_url,''), COALESCE(s.source_name_prefix,''), COALESCE(s.source_auto_refresh,0), COALESCE(s.source_refresh_interval_m,0), COALESCE(s.source_last_refresh_at,''), COALESCE(s.last_latency_ms,-1), COALESCE(s.last_test_ok,0), COALESCE(s.last_tested_at,''), COALESCE(s.last_ip_health_json,''), s.sort_order, s.created_at
FROM src.browser_proxies s
WHERE NOT EXISTS (
  SELECT 1 FROM browser_proxies t
  WHERE t.proxy_id = s.proxy_id OR lower(t.proxy_config) = lower(s.proxy_config)
)`,
		},
		{
			name: "browser_profiles",
			insertAll: fmt.Sprintf(`INSERT INTO browser_profiles (profile_id, profile_name, user_data_dir, core_id, fingerprint_args, proxy_id, proxy_config, launch_args, profile_config, tags, keywords, group_id, created_at, updated_at)
SELECT profile_id, profile_name, user_data_dir, core_id, fingerprint_args, proxy_id, proxy_config, launch_args, %s, tags, keywords, COALESCE(group_id,''), created_at, updated_at
FROM src.browser_profiles`, profileConfigInsertAll),
			insertSafe: fmt.Sprintf(`INSERT INTO browser_profiles (profile_id, profile_name, user_data_dir, core_id, fingerprint_args, proxy_id, proxy_config, launch_args, profile_config, tags, keywords, group_id, created_at, updated_at)
SELECT s.profile_id, s.profile_name, s.user_data_dir, s.core_id, s.fingerprint_args, s.proxy_id, s.proxy_config, s.launch_args, %s, s.tags, s.keywords, COALESCE(s.group_id,''), s.created_at, s.updated_at
FROM src.browser_profiles s
WHERE NOT EXISTS (
  SELECT 1 FROM browser_profiles t
  WHERE t.profile_id = s.profile_id OR lower(t.user_data_dir) = lower(s.user_data_dir)
)`, profileConfigInsertSafe),
		},
		{
			name: "browser_bookmarks",
			insertAll: `INSERT INTO browser_bookmarks (name, url, sort_order)
SELECT name, url, sort_order FROM src.browser_bookmarks`,
			insertSafe: `INSERT INTO browser_bookmarks (name, url, sort_order)
SELECT s.name, s.url, s.sort_order
FROM src.browser_bookmarks s
WHERE NOT EXISTS (
  SELECT 1 FROM browser_bookmarks t WHERE lower(t.url) = lower(s.url)
)`,
		},
		{
			name: "launch_codes",
			insertAll: `INSERT INTO launch_codes (profile_id, code, created_at, updated_at)
SELECT profile_id, code, created_at, updated_at FROM src.launch_codes`,
			insertSafe: `INSERT INTO launch_codes (profile_id, code, created_at, updated_at)
SELECT s.profile_id, s.code, s.created_at, s.updated_at
FROM src.launch_codes s
WHERE NOT EXISTS (
  SELECT 1 FROM launch_codes t
  WHERE t.profile_id = s.profile_id OR t.code = s.code
)`,
		},
	}

	for _, item := range mergeTables {
		exists, err := backupSrcTableExists(tx, item.name)
		if err != nil {
			return err
		}
		if !exists {
			continue
		}

		total, err := backupCountRows(tx, "src."+item.name)
		if err != nil {
			return err
		}
		if total == 0 {
			continue
		}

		sqlText := item.insertAll
		if !resetFirst {
			sqlText = item.insertSafe
		}
		res, err := tx.Exec(sqlText)
		if err != nil {
			return fmt.Errorf("导入数据表失败(%s): %w", item.name, err)
		}
		affected, _ := res.RowsAffected()
		inserted := int(affected)
		if inserted < 0 {
			inserted = total
		}
		stats.Imported += inserted
		if !resetFirst && total > inserted {
			stats.Skipped += total - inserted
		}
	}

	return tx.Commit()
}

func (a *App) backupImportFileTrees(payloadRoot string, incomingCfg *config.Config, resetFirst bool, stats *backupMergeStats, onIssue func(componentID, componentName string, err error)) {
	report := func(componentID, componentName string, err error) {
		if onIssue != nil && err != nil {
			onIssue(componentID, componentName, err)
		}
	}

	appDataSrc := filepath.Join(payloadRoot, "app", "data")
	appDataDst := a.resolveAppPath("data")
	dbPath := a.backupResolveDBPath(a.config)
	keepDB := map[string]struct{}{
		backupNormalizePath(dbPath):          {},
		backupNormalizePath(dbPath + "-wal"): {},
		backupNormalizePath(dbPath + "-shm"): {},
	}

	if backupPathExists(appDataSrc) {
		if resetFirst {
			if err := backupRemoveContentsExcept(appDataDst, keepDB); err != nil {
				report("app_data_root", "应用数据目录（含数据库、快照及默认浏览器数据）", err)
			} else if err := backupSyncDir(appDataSrc, appDataDst, true, stats, backupShouldSkipAppDBFile); err != nil {
				report("app_data_root", "应用数据目录（含数据库、快照及默认浏览器数据）", err)
			}
		} else {
			if err := backupSyncDir(appDataSrc, appDataDst, false, stats, backupShouldSkipAppDBFile); err != nil {
				report("app_data_root", "应用数据目录（含数据库、快照及默认浏览器数据）", err)
			}
		}
	}

	userDataSrc := filepath.Join(payloadRoot, "browser", "user-data")
	userDataDst := a.backupResolveUserDataRoot(a.config)
	if backupPathExists(userDataSrc) {
		if resetFirst {
			_ = os.RemoveAll(userDataDst)
			if err := os.MkdirAll(userDataDst, 0755); err != nil {
				report("browser_user_data_root", "浏览器用户数据根目录（若与 data 重合则自动去重）", err)
			} else if err := backupSyncDir(userDataSrc, userDataDst, true, stats, nil); err != nil {
				report("browser_user_data_root", "浏览器用户数据根目录（若与 data 重合则自动去重）", err)
			}
		} else {
			if err := backupSyncDir(userDataSrc, userDataDst, false, stats, nil); err != nil {
				report("browser_user_data_root", "浏览器用户数据根目录（若与 data 重合则自动去重）", err)
			}
		}
	}

	chromeSrc := filepath.Join(payloadRoot, "browser", "cores", "chrome")
	chromeDst := a.resolveAppPath("chrome")
	if backupPathExists(chromeSrc) {
		if resetFirst {
			_ = os.RemoveAll(chromeDst)
			if err := os.MkdirAll(chromeDst, 0755); err != nil {
				report("browser_core_root", "默认内核目录", err)
			} else if err := backupSyncDir(chromeSrc, chromeDst, true, stats, nil); err != nil {
				report("browser_core_root", "默认内核目录", err)
			}
		} else {
			if err := backupSyncDir(chromeSrc, chromeDst, false, stats, nil); err != nil {
				report("browser_core_root", "默认内核目录", err)
			}
		}
	}

	externalSrcRoot := filepath.Join(payloadRoot, "browser", "cores", "external")
	if backupPathExists(externalSrcRoot) {
		sourceExternal := make([]string, 0)
		entries, err := os.ReadDir(externalSrcRoot)
		if err != nil {
			report("browser_core_external", "额外内核目录（来自配置 cores）", err)
			return
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			sourceExternal = append(sourceExternal, entry.Name())
		}
		sort.Strings(sourceExternal)

		if incomingCfg == nil {
			for _, folder := range sourceExternal {
				componentID := "browser_core_external_" + folder
				report(componentID, "额外内核目录（来自配置 cores）", fmt.Errorf("缺少可用配置，无法映射目标路径"))
			}
			return
		}

		targetExternal := a.backupCollectExternalCorePaths(incomingCfg)
		for i, folder := range sourceExternal {
			src := filepath.Join(externalSrcRoot, folder)
			componentID := "browser_core_external_" + folder
			if i >= len(targetExternal) {
				stats.Skipped++
				report(componentID, "额外内核目录（来自配置 cores）", fmt.Errorf("目标配置缺失，无法导入该外部内核目录"))
				continue
			}
			dst := targetExternal[i]
			if resetFirst {
				_ = os.RemoveAll(dst)
				if err := os.MkdirAll(dst, 0755); err != nil {
					report(componentID, "额外内核目录（来自配置 cores）", err)
					continue
				}
				if err := backupSyncDir(src, dst, true, stats, nil); err != nil {
					report(componentID, "额外内核目录（来自配置 cores）", err)
					continue
				}
			} else {
				if err := backupSyncDir(src, dst, false, stats, nil); err != nil {
					report(componentID, "额外内核目录（来自配置 cores）", err)
					continue
				}
			}
		}
	}
}

func (a *App) backupCollectExternalCorePaths(cfg *config.Config) []string {
	if cfg == nil {
		return nil
	}
	defaultChromeRoot := a.resolveAppPath("chrome")
	seen := map[string]struct{}{}
	result := make([]string, 0)
	for _, core := range cfg.Browser.Cores {
		p := strings.TrimSpace(core.CorePath)
		if p == "" {
			continue
		}
		abs := a.resolveAppPath(p)
		if backupPathWithin(abs, defaultChromeRoot) {
			continue
		}
		norm := backupNormalizePath(abs)
		if _, ok := seen[norm]; ok {
			continue
		}
		seen[norm] = struct{}{}
		result = append(result, abs)
	}
	sort.Strings(result)
	return result
}
func backupSyncDir(src, dst string, overwrite bool, stats *backupMergeStats, shouldSkip func(rel string) bool) error {
	if !backupPathExists(src) {
		return nil
	}
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			stats.Skipped++
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if shouldSkip != nil && shouldSkip(rel) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			stats.Skipped++
			return nil
		}

		target := filepath.Join(dst, filepath.FromSlash(rel))
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}

		if overwrite {
			if err := backupCopyFile(path, target); err != nil {
				return err
			}
			stats.Imported++
			return nil
		}

		if _, err := os.Stat(target); os.IsNotExist(err) {
			if err := backupCopyFile(path, target); err != nil {
				return err
			}
			stats.Imported++
			return nil
		} else if err != nil {
			return err
		}

		same, err := backupFilesSame(path, target)
		if err != nil {
			return err
		}
		if same {
			stats.Skipped++
		} else {
			stats.Conflicts++
		}
		return nil
	})
}

func backupCopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	tmpPath := dst + ".tmp"
	out, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, dst); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func backupFilesSame(a, b string) (bool, error) {
	ainfo, err := os.Stat(a)
	if err != nil {
		return false, err
	}
	binfo, err := os.Stat(b)
	if err != nil {
		return false, err
	}
	if ainfo.Size() != binfo.Size() {
		return false, nil
	}
	ah, err := backupSHA256File(a)
	if err != nil {
		return false, err
	}
	bh, err := backupSHA256File(b)
	if err != nil {
		return false, err
	}
	return ah == bh, nil
}

func backupSHA256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func backupShouldSkipAppDBFile(rel string) bool {
	r := strings.TrimSpace(filepath.ToSlash(rel))
	return r == "app.db" || r == "app.db-wal" || r == "app.db-shm"
}

func backupRemoveContentsExcept(dir string, keep map[string]struct{}) error {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		p := filepath.Join(dir, entry.Name())
		if backupPathInSet(p, keep) {
			continue
		}
		if err := os.RemoveAll(p); err != nil {
			return err
		}
	}
	return nil
}

func backupPathInSet(path string, set map[string]struct{}) bool {
	if len(set) == 0 {
		return false
	}
	_, ok := set[backupNormalizePath(path)]
	return ok
}

func backupNormalizePath(path string) string {
	return strings.ToLower(filepath.Clean(strings.TrimSpace(path)))
}

func backupEnsureZipSuffix(path string) string {
	if strings.EqualFold(filepath.Ext(path), ".zip") {
		return path
	}
	return path + ".zip"
}

func backupPathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func backupSamePath(a, b string) bool {
	return backupNormalizePath(a) == backupNormalizePath(b)
}

func backupPathWithin(path, root string) bool {
	p := backupNormalizePath(path)
	r := backupNormalizePath(root)
	if p == r {
		return true
	}
	if !strings.HasSuffix(r, string(filepath.Separator)) {
		r += string(filepath.Separator)
	}
	return strings.HasPrefix(p, r)
}

func backupIsNoSuchTableError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "no such table")
}

func backupUniqueNonEmpty(list []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(list))
	for _, item := range list {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		key := backupNormalizePath(item)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}

func backupUnionStrings(a, b []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(a)+len(b))
	for _, item := range append(append([]string{}, a...), b...) {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		key := strings.ToLower(item)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}

func backupMergeBookmarks(a, b []config.BrowserBookmark) []config.BrowserBookmark {
	seen := map[string]struct{}{}
	out := make([]config.BrowserBookmark, 0, len(a)+len(b))
	appendOne := func(item config.BrowserBookmark) {
		urlKey := strings.ToLower(strings.TrimSpace(item.URL))
		if urlKey == "" {
			return
		}
		if _, ok := seen[urlKey]; ok {
			return
		}
		seen[urlKey] = struct{}{}
		out = append(out, item)
	}
	for _, item := range a {
		appendOne(item)
	}
	for _, item := range b {
		appendOne(item)
	}
	return out
}

func backupMergeCores(a, b []config.BrowserCore) []config.BrowserCore {
	seenID := map[string]struct{}{}
	seenPath := map[string]struct{}{}
	out := make([]config.BrowserCore, 0, len(a)+len(b))
	appendOne := func(item config.BrowserCore) {
		idKey := strings.ToLower(strings.TrimSpace(item.CoreId))
		pathKey := strings.ToLower(strings.TrimSpace(item.CorePath))
		if idKey == "" && pathKey == "" {
			return
		}
		if idKey != "" {
			if _, ok := seenID[idKey]; ok {
				return
			}
		}
		if pathKey != "" {
			if _, ok := seenPath[pathKey]; ok {
				return
			}
		}
		if idKey != "" {
			seenID[idKey] = struct{}{}
		}
		if pathKey != "" {
			seenPath[pathKey] = struct{}{}
		}
		out = append(out, item)
	}
	for _, item := range a {
		appendOne(item)
	}
	for _, item := range b {
		appendOne(item)
	}
	return out
}

func backupMergeProxies(a, b []config.BrowserProxy) []config.BrowserProxy {
	seenID := map[string]struct{}{}
	seenCfg := map[string]struct{}{}
	out := make([]config.BrowserProxy, 0, len(a)+len(b))
	appendOne := func(item config.BrowserProxy) {
		idKey := strings.ToLower(strings.TrimSpace(item.ProxyId))
		cfgKey := strings.ToLower(strings.TrimSpace(item.ProxyConfig))
		if idKey == "" && cfgKey == "" {
			return
		}
		if idKey != "" {
			if _, ok := seenID[idKey]; ok {
				return
			}
		}
		if cfgKey != "" {
			if _, ok := seenCfg[cfgKey]; ok {
				return
			}
		}
		if idKey != "" {
			seenID[idKey] = struct{}{}
		}
		if cfgKey != "" {
			seenCfg[cfgKey] = struct{}{}
		}
		out = append(out, item)
	}
	for _, item := range a {
		appendOne(item)
	}
	for _, item := range b {
		appendOne(item)
	}
	return out
}

func backupMergeProfiles(a, b []config.BrowserProfileConfig) []config.BrowserProfileConfig {
	seenID := map[string]struct{}{}
	seenDir := map[string]struct{}{}
	out := make([]config.BrowserProfileConfig, 0, len(a)+len(b))
	appendOne := func(item config.BrowserProfileConfig) {
		idKey := strings.ToLower(strings.TrimSpace(item.ProfileId))
		dirKey := strings.ToLower(strings.TrimSpace(item.UserDataDir))
		if idKey == "" && dirKey == "" {
			return
		}
		if idKey != "" {
			if _, ok := seenID[idKey]; ok {
				return
			}
		}
		if dirKey != "" {
			if _, ok := seenDir[dirKey]; ok {
				return
			}
		}
		if idKey != "" {
			seenID[idKey] = struct{}{}
		}
		if dirKey != "" {
			seenDir[dirKey] = struct{}{}
		}
		out = append(out, item)
	}
	for _, item := range a {
		appendOne(item)
	}
	for _, item := range b {
		appendOne(item)
	}
	return out
}

func backupSrcTableExists(tx *sql.Tx, table string) (bool, error) {
	var cnt int
	err := tx.QueryRow(`SELECT COUNT(1) FROM src.sqlite_master WHERE type='table' AND name=?`, table).Scan(&cnt)
	if err != nil {
		return false, err
	}
	return cnt > 0, nil
}

func backupSrcColumnExists(tx *sql.Tx, table string, column string) (bool, error) {
	if !backupSafeSQLiteIdent(table) {
		return false, fmt.Errorf("invalid sqlite table name: %s", table)
	}
	rows, err := tx.Query(fmt.Sprintf("PRAGMA src.table_info(%s)", table))
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var typeName string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &typeName, &notNull, &defaultValue, &pk); err != nil {
			return false, err
		}
		if strings.EqualFold(name, column) {
			return true, nil
		}
	}
	return false, rows.Err()
}

func backupSafeSQLiteIdent(value string) bool {
	if value == "" {
		return false
	}
	for _, ch := range value {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' {
			continue
		}
		return false
	}
	return true
}

func backupCountRows(tx *sql.Tx, tableName string) (int, error) {
	var cnt int
	row := tx.QueryRow("SELECT COUNT(1) FROM " + tableName)
	if err := row.Scan(&cnt); err != nil {
		return 0, err
	}
	return cnt, nil
}
