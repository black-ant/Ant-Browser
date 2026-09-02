package backend

import (
	"ant-chrome/backend/internal/browser"
	"errors"
	"os/exec"
	"sort"
	"strings"
	"time"
)

var backupTryCloseBrowserViaCDP = tryCloseBrowserViaCDP
var backupTerminateBrowserProcessesByUserDataDir = terminateBrowserProcessesByUserDataDir

func (a *App) backupRunningProfileNames() []string {
	if a.browserMgr == nil {
		return nil
	}

	profiles := a.browserMgr.List()
	names := make([]string, 0)
	for _, profile := range profiles {
		if !profile.Running {
			continue
		}
		name := strings.TrimSpace(profile.ProfileName)
		if name == "" {
			name = strings.TrimSpace(profile.ProfileId)
		}
		if name != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func (a *App) backupStopRuntimeForMaintenance() error {
	// 维护期间停止会访问数据库或代理内核的运行态服务，
	// 自动化任务由应用生命周期继续管理。
	type runtimeProfile struct {
		debugPort   int
		userDataDir string
	}

	profiles := make([]runtimeProfile, 0)
	commands := make([]*exec.Cmd, 0)
	seenCommands := make(map[*exec.Cmd]struct{})
	var stopErrors []error
	if a.browserMgr != nil {
		a.browserMgr.Mutex.Lock()
		profileIDs := make(map[string]struct{}, len(a.browserMgr.Profiles)+len(a.browserMgr.BrowserProcesses))
		for profileID := range a.browserMgr.Profiles {
			profileIDs[profileID] = struct{}{}
		}
		for profileID := range a.browserMgr.BrowserProcesses {
			profileIDs[profileID] = struct{}{}
		}
		for profileID := range profileIDs {
			profile := a.browserMgr.Profiles[profileID]
			cmd := a.browserMgr.BrowserProcesses[profileID]
			if profile == nil && cmd == nil {
				continue
			}
			if cmd == nil && (profile == nil || (!profile.Running && profile.Pid <= 0 && profile.DebugPort <= 0)) {
				continue
			}
			item := runtimeProfile{}
			if profile != nil {
				item.debugPort = profile.DebugPort
				item.userDataDir = a.browserMgr.ResolveUserDataDir(profile)
			}
			profiles = append(profiles, item)
			if cmd != nil && cmd.Process != nil {
				if _, exists := seenCommands[cmd]; !exists {
					seenCommands[cmd] = struct{}{}
					commands = append(commands, cmd)
				}
			}
		}
		a.browserMgr.Mutex.Unlock()
	}

	for _, profile := range profiles {
		if profile.debugPort > 0 {
			_ = backupTryCloseBrowserViaCDP(profile.debugPort, 5*time.Second)
		}
	}
	for _, cmd := range commands {
		if err := a.stopProcessCmd(cmd); err != nil {
			stopErrors = append(stopErrors, err)
		}
	}
	for _, profile := range profiles {
		if profile.userDataDir != "" {
			if _, err := backupTerminateBrowserProcessesByUserDataDir(profile.userDataDir, 5*time.Second); err != nil {
				stopErrors = append(stopErrors, err)
			}
		}
	}

	if err := errors.Join(stopErrors...); err != nil {
		return err
	}
	a.cancelBackgroundTasksAndWait()
	if a.speedScheduler != nil {
		a.speedScheduler.Stop()
		a.speedScheduler = nil
	}

	if a.xrayMgr != nil {
		a.xrayMgr.StopBridges()
	}
	if a.clashMgr != nil {
		a.clashMgr.StopAll()
	}
	if a.singboxMgr != nil {
		a.singboxMgr.StopBridges()
	}
	a.clearProfileProxyBridges()

	if a.browserMgr != nil {
		a.browserMgr.Mutex.Lock()
		for profileID, profile := range a.browserMgr.Profiles {
			if profile == nil {
				continue
			}
			if profile.Running || profile.Pid > 0 || profile.DebugPort > 0 || profile.WindowMarkerCode != "" || a.browserMgr.BrowserProcesses[profileID] != nil {
				a.markProfileStoppedLocked(profileID, profile)
			}
		}
		a.browserMgr.BrowserProcesses = make(map[string]*exec.Cmd)
		a.browserProcessMonitors = make(map[string]*browserProcessMonitor)
		a.browserMgr.Mutex.Unlock()
	}
	return nil
}

func (a *App) backupReloadAfterMutation() error {
	if err := a.ReloadConfig(); err != nil {
		a.resumeBackgroundTasks()
		a.ensureSpeedScheduler()
		return err
	}

	if a.browserMgr != nil {
		a.browserMgr.Config = a.config
		a.browserMgr.Mutex.Lock()
		a.browserMgr.Profiles = make(map[string]*browser.Profile)
		a.browserMgr.BrowserProcesses = make(map[string]*exec.Cmd)
		a.browserMgr.XrayBridges = make(map[string]*browser.XrayBridge)
		a.browserProcessMonitors = make(map[string]*browserProcessMonitor)
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

	a.ensureSpeedScheduler()
	a.resumeBackgroundTasks()
	return nil
}
