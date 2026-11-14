package backend

import "ant-chrome/backend/internal/logger"

var detectBrowserRuntimeForStartup = detectBrowserRuntimeByActivePort

func (a *App) recoverBrowserRuntimeStates() {
	if a == nil || a.browserMgr == nil {
		return
	}

	recovered := 0
	a.browserMgr.Mutex.Lock()
	for profileId, profile := range a.browserMgr.Profiles {
		if profile == nil || profile.Running {
			continue
		}
		userDataDir := a.browserMgr.ResolveUserDataDir(profile)
		detection, ok := detectBrowserRuntimeForStartup(userDataDir)
		if !ok || !detection.DebugReady || detection.DebugPort <= 0 {
			continue
		}
		a.markProfileRunningLocked(profileId, profile, nil, detection.PID, detection.DebugPort, true, "")
		recovered++
	}
	a.browserMgr.Mutex.Unlock()

	if recovered > 0 {
		logger.New("Browser").Info("已恢复保留中的浏览器实例状态", logger.F("count", recovered))
	}
}
