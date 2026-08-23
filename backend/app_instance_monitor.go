package backend

import (
	"ant-chrome/backend/internal/logger"
	"fmt"
	"time"
)

func (a *App) waitBrowserProcess(profileId string, monitor *browserProcessMonitor) {
	err := monitor.Wait()
	if !a.browserProcessMonitorIsCurrent(profileId, monitor) {
		return
	}

	log := logger.New("Browser")
	debugPort := 0
	profileName := profileId
	shouldMonitorDetached := false

	a.browserMgr.Mutex.Lock()
	if !a.browserProcessMonitorIsCurrentLocked(profileId, monitor) {
		a.browserMgr.Mutex.Unlock()
		return
	}
	profile, exists := a.browserMgr.Profiles[profileId]
	wasRunning := exists && profile.Running
	if exists {
		profileName = profile.ProfileName
		debugPort = profile.DebugPort
	}
	a.browserMgr.Mutex.Unlock()

	if wasRunning && debugPort > 0 {
		snapshot, changed := a.waitForBrowserDebugReadyForMonitor(profileId, debugPort, browserLauncherDetachGraceWindow, monitor)
		if warningSnapshot, warningChanged := a.finalizeDeferredStartTargetsForMonitor(profileId, debugPort, monitor); warningSnapshot != nil {
			snapshot = warningSnapshot
			changed = changed || warningChanged
		}
		if snapshot != nil && changed {
			if !a.browserProcessMonitorIsCurrent(profileId, monitor) {
				return
			}
			log.Info("浏览器启动器进程退出后，调试接口延迟就绪",
				logger.F("profile_id", profileId),
				logger.F("debug_port", debugPort),
			)
			a.emitBrowserInstanceUpdated(snapshot)
		}

		a.browserMgr.Mutex.Lock()
		profile, exists = a.browserMgr.Profiles[profileId]
		if !a.browserProcessMonitorIsCurrentLocked(profileId, monitor) {
			a.browserMgr.Mutex.Unlock()
			return
		}
		if exists && profile.Running && profile.DebugPort == debugPort && profile.DebugReady && canConnectDebugPort(debugPort, 250*time.Millisecond) {
			delete(a.browserMgr.BrowserProcesses, profileId)
			profile.Pid = 0
			shouldMonitorDetached = true
		}
		a.browserMgr.Mutex.Unlock()
		if shouldMonitorDetached {
			log.Info("浏览器启动器进程已退出，切换为调试端口存活监控",
				logger.F("profile_id", profileId),
				logger.F("profile_name", profileName),
				logger.F("debug_port", debugPort),
			)
			a.waitDetachedBrowser(profileId, debugPort, monitor)
			return
		}
	}

	a.browserMgr.Mutex.Lock()
	if !a.browserProcessMonitorIsCurrentLocked(profileId, monitor) {
		a.browserMgr.Mutex.Unlock()
		return
	}
	profile, exists = a.browserMgr.Profiles[profileId]
	wasRunning = exists && profile.Running
	if exists {
		profileName = profile.ProfileName
		a.markProfileStoppedLocked(profileId, profile)
	}
	if wasRunning && err != nil && exists && profile != nil {
		profile.LastError = fmt.Sprintf("实例运行异常退出：%s", err.Error())
	}
	a.browserMgr.Mutex.Unlock()

	if wasRunning && err != nil {
		log.Error("浏览器进程异常退出", logger.F("profile_id", profileId), logger.F("profile_name", profileName), logger.F("error", err))
		a.emitRuntimeEvent("browser:instance:crashed", map[string]interface{}{
			"profileId":   profileId,
			"profileName": profileName,
			"error":       err.Error(),
		})
	} else {
		a.emitRuntimeEvent("browser:instance:stopped", profileId)
	}
}

func (a *App) waitDetachedBrowser(profileId string, debugPort int, monitor *browserProcessMonitor) {
	const (
		pollInterval = 500 * time.Millisecond
		maxMisses    = 3
	)

	log := logger.New("Browser")
	misses := 0
	for {
		if !a.browserProcessMonitorIsCurrent(profileId, monitor) {
			return
		}
		if canConnectDebugPort(debugPort, 250*time.Millisecond) {
			misses = 0
			time.Sleep(pollInterval)
			continue
		}

		misses++
		if misses < maxMisses {
			time.Sleep(pollInterval)
			continue
		}

		profileName := profileId
		a.browserMgr.Mutex.Lock()
		if !a.browserProcessMonitorIsCurrentLocked(profileId, monitor) {
			a.browserMgr.Mutex.Unlock()
			return
		}
		profile, exists := a.browserMgr.Profiles[profileId]
		if !exists || !profile.Running || profile.DebugPort != debugPort {
			a.browserMgr.Mutex.Unlock()
			return
		}
		profileName = profile.ProfileName
		a.markProfileStoppedLocked(profileId, profile)
		a.browserMgr.Mutex.Unlock()

		log.Info("检测到浏览器调试端口关闭，实例已停止",
			logger.F("profile_id", profileId),
			logger.F("profile_name", profileName),
			logger.F("debug_port", debugPort),
		)
		a.emitRuntimeEvent("browser:instance:stopped", profileId)
		return
	}
}
