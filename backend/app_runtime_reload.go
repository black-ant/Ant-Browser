package backend

import (
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/launchcode"
	"ant-chrome/backend/internal/logger"
	"fmt"
	"runtime/debug"
)

// ReloadConfig 开放给前端重新读取配置，用于应对手动修补后的配置重载
func (a *App) ReloadConfig() error {
	log := logger.New("App")
	cfg, err := LoadConfig(a.resolveAppPath("config.yaml"))
	if err != nil {
		log.Error("重载配置文件失败", logger.F("error", err))
		return fmt.Errorf("重载配置文件失败: %w", err)
	}

	previousConfig := a.config
	a.config = cfg
	a.applyRuntimeConfig(cfg.Runtime)
	if a.browserMgr != nil {
		a.browserMgr.Config = cfg
		a.browserMgr.ListCores()
		if err := a.loadProxies(); err != nil {
			a.config = previousConfig
			a.browserMgr.Config = previousConfig
			if previousConfig != nil {
				a.applyRuntimeConfig(previousConfig.Runtime)
			}
			return err
		}
		a.reconcileProfileProxyBindings()
	}
	if a.xrayMgr != nil {
		a.xrayMgr.Config = cfg
	}
	if a.clashMgr != nil {
		a.clashMgr.Config = cfg
	}
	if a.singboxMgr != nil {
		a.singboxMgr.Config = cfg
	}
	if a.launchServer != nil {
		a.launchServer.SetAPIAuthConfig(launchcode.APIAuthConfig{
			Enabled: cfg.LaunchServer.Auth.Enabled,
			APIKey:  cfg.LaunchServer.Auth.APIKey,
			Header:  cfg.LaunchServer.Auth.Header,
		})
	}
	if a.automationMgr != nil {
		a.automationMgr.SetConfig(cfg)
	}
	a.restartSpeedScheduler()

	log.Info("前端触发配置重载成功")
	return nil
}

func (a *App) applyRuntimeConfig(cfg config.RuntimeConfig) {
	if cfg.GCPercent > 0 {
		debug.SetGCPercent(cfg.GCPercent)
	}
	if cfg.MaxMemoryMB > 0 {
		maxMemoryBytes := int64(cfg.MaxMemoryMB) * 1024 * 1024
		debug.SetMemoryLimit(maxMemoryBytes)
		return
	}
	debug.SetMemoryLimit(1 << 60)
}
