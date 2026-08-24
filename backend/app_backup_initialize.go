package backend

import (
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/logger"
	"errors"
	"fmt"
	"os"
	"strings"
)

func (a *App) backupInitializeLocked(applyReload bool) (map[string]interface{}, error) {
	log := logger.New("Backup")
	if err := a.backupStopRuntimeForMaintenance(); err != nil {
		return nil, fmt.Errorf("停止运行时失败，已中止系统初始化: %w", err)
	}

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

	cleared := make([]string, 0, 3)
	var cleanupErrors []error
	dataRoot := a.resolveAppPath("data")
	if err := backupRemoveContentsExcept(dataRoot, keepFiles); err == nil {
		cleared = append(cleared, dataRoot)
	} else {
		cleanupErrors = append(cleanupErrors, fmt.Errorf("清理 %s 失败: %w", dataRoot, err))
	}
	oldUserRoot := a.backupResolveUserDataRoot(oldCfg)
	newUserRoot := a.backupResolveUserDataRoot(defaultCfg)
	for _, p := range backupUniqueNonEmpty([]string{oldUserRoot, newUserRoot}) {
		if backupSamePath(p, dataRoot) {
			continue
		}
		if err := backupRemoveContentsExcept(p, keepFiles); err == nil {
			cleared = append(cleared, p)
		} else {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("清理 %s 失败: %w", p, err))
		}
	}
	if err := errors.Join(cleanupErrors...); err != nil {
		return nil, fmt.Errorf("系统初始化未完成: %w", err)
	}
	proxiesPath := a.resolveAppPath("proxies.yaml")
	if err := os.Remove(proxiesPath); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("删除代理配置失败，已中止系统初始化: %w", err)
	}

	if err := a.backupClearBusinessTables(); err != nil {
		return nil, err
	}
	if err := defaultCfg.Save(a.resolveAppPath("config.yaml")); err != nil {
		return nil, fmt.Errorf("写入默认配置失败: %w", err)
	}
	a.config = defaultCfg
	a.applyRuntimeConfig(defaultCfg.Runtime)

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
