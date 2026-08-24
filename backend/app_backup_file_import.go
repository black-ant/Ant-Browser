package backend

import (
	"ant-chrome/backend/internal/backup"
	"ant-chrome/backend/internal/config"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func (a *App) backupImportFileTrees(payloadRoot string, incomingCfg *config.Config, manifest backup.Manifest, resetFirst bool, stats *backupMergeStats, onIssue func(componentID, componentName string, err error)) {
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
				if err := os.RemoveAll(userDataDst); err != nil {
					report("browser_user_data_root", "浏览器用户数据根目录（若与 data 重合则自动去重）", err)
				} else if err := os.MkdirAll(userDataDst, 0755); err != nil {
					report("browser_user_data_root", "浏览器用户数据根目录（若与 data 重合则自动去重）", err)
				} else if err := backupSyncDir(userDataSrc, userDataDst, true, stats, backupShouldSkipAppDBFile); err != nil {
					report("browser_user_data_root", "浏览器用户数据根目录（若与 data 重合则自动去重）", err)
				}
			} else if err := os.MkdirAll(userDataDst, 0755); err != nil {
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
			if err := os.RemoveAll(chromeDst); err != nil {
				report("browser_core_root", "默认内核目录", err)
			} else if err := os.MkdirAll(chromeDst, 0755); err != nil {
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

		targetExternal := a.backupCollectExternalCoreTargets(incomingCfg)
		sourceCoreIDs := backupExternalCoreIDsByFolder(manifest)
		usedTargets := make(map[string]struct{}, len(targetExternal))
		fallbackIndex := 0
		for _, folder := range sourceExternal {
			src := filepath.Join(externalSrcRoot, folder)
			componentID := "browser_core_external_" + folder
			coreID := sourceCoreIDs[folder]
			dst, ok := backupSelectExternalCoreTarget(targetExternal, coreID, usedTargets, &fallbackIndex)
			if !ok {
				stats.Skipped++
				report(componentID, "额外内核目录（来自配置 cores）", fmt.Errorf("目标配置缺失，无法导入该外部内核目录"))
				continue
			}
			if resetFirst {
				if err := os.RemoveAll(dst); err != nil {
					report(componentID, "额外内核目录（来自配置 cores）", err)
					continue
				}
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
	targets := a.backupCollectExternalCoreTargets(cfg)
	result := make([]string, 0, len(targets))
	for _, target := range targets {
		result = append(result, target.Path)
	}
	return result
}

type backupExternalCoreTarget struct {
	CoreId string
	Path   string
}

func (a *App) backupCollectExternalCoreTargets(cfg *config.Config) []backupExternalCoreTarget {
	if cfg == nil {
		return nil
	}
	defaultChromeRoot := a.resolveAppPath("chrome")
	seen := map[string]struct{}{}
	result := make([]backupExternalCoreTarget, 0)
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
		result = append(result, backupExternalCoreTarget{
			CoreId: strings.TrimSpace(core.CoreId),
			Path:   abs,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return backupNormalizePath(result[i].Path) < backupNormalizePath(result[j].Path)
	})
	return result
}

func backupExternalCoreIDsByFolder(manifest backup.Manifest) map[string]string {
	result := make(map[string]string)
	const prefix = "payload/browser/cores/external/"
	for _, entry := range manifest.Entries {
		coreID := strings.TrimSpace(entry.CoreId)
		if coreID == "" {
			continue
		}
		archivePath := filepath.ToSlash(strings.TrimSuffix(strings.TrimSpace(entry.ArchivePath), "/"))
		if !strings.HasPrefix(archivePath, prefix) {
			continue
		}
		folder := strings.TrimPrefix(archivePath, prefix)
		if folder == "" || strings.Contains(folder, "/") {
			continue
		}
		result[folder] = coreID
	}
	return result
}

func backupSelectExternalCoreTarget(targets []backupExternalCoreTarget, coreID string, used map[string]struct{}, fallbackIndex *int) (string, bool) {
	coreID = strings.TrimSpace(coreID)
	if coreID != "" {
		for _, target := range targets {
			if _, usedAlready := used[target.Path]; usedAlready {
				continue
			}
			if strings.EqualFold(strings.TrimSpace(target.CoreId), coreID) {
				used[target.Path] = struct{}{}
				return target.Path, true
			}
		}
	}

	for *fallbackIndex < len(targets) {
		target := targets[*fallbackIndex]
		*fallbackIndex = *fallbackIndex + 1
		if _, usedAlready := used[target.Path]; usedAlready {
			continue
		}
		used[target.Path] = struct{}{}
		return target.Path, true
	}
	return "", false
}
