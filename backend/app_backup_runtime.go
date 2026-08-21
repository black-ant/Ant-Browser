package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"os/exec"
)

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
	a.clearProfileProxyBridges()
	if a.singboxMgr != nil {
		a.singboxMgr.StopAll()
	}
	if a.speedScheduler != nil {
		a.speedScheduler.Stop()
		a.speedScheduler = nil
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
				connectorType := config.NormalizeBrowserConnectorType(a.config.Browser.DefaultConnectorType)
				r := a.testProxySpeedWithConnector(proxyID, a.getLatestProxies(), connectorType)
				return r.Ok, r.LatencyMs, r.Error
			},
			browser.DefaultProxySpeedInterval,
			browser.DefaultProxySpeedConcurrency,
		)
		// 与启动路径一致:默认不跑周期测速,手动 RunOnce 仍可用。
		if speedAutoTestEnabled(a.config) {
			a.speedScheduler.Start()
		}
	}
	return nil
}
