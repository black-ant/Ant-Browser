package backend

import (
	"ant-chrome/backend/internal/config"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

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
		userDataOverlapsAppData := backupSamePath(userDataDst, appDataDst) ||
			backupPathWithin(userDataDst, appDataDst) ||
			backupPathWithin(appDataDst, userDataDst)
		if resetFirst {
			if !userDataOverlapsAppData {
				_ = os.RemoveAll(userDataDst)
			}
			if err := os.MkdirAll(userDataDst, 0755); err != nil {
				report("browser_user_data_root", "浏览器用户数据根目录（若与 data 重合则自动去重）", err)
			} else if err := backupSyncDir(userDataSrc, userDataDst, true, stats, backupShouldSkipAppDBFile); err != nil {
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
