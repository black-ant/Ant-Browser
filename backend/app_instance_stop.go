package backend

import (
	"ant-chrome/backend/internal/logger"
	"fmt"
	"time"
)

var (
	detectBrowserRuntimeForStop = detectBrowserRuntimeByUserDataDir
	closeBrowserViaCDPForStop   = tryCloseBrowserViaCDP
	debugPortReachableForStop   = canConnectDebugPort
)

func (a *App) BrowserInstanceStop(profileId string) (*BrowserProfile, error) {
	log := logger.New("Browser")
	a.browserMgr.Mutex.Lock()
	defer a.browserMgr.Mutex.Unlock()

	profile, exists := a.browserMgr.Profiles[profileId]
	if !exists {
		return nil, fmt.Errorf("profile not found")
	}

	cmd := a.browserMgr.BrowserProcesses[profileId]
	if cmd == nil && (!profile.Running || profile.DebugPort <= 0 || !debugPortReachableForStop(profile.DebugPort, 250*time.Millisecond)) {
		userDataDir := a.browserMgr.ResolveUserDataDir(profile)
		if detection, detected := detectBrowserRuntimeForStop(userDataDir); detected {
			if !detection.DebugReady || detection.DebugPort <= 0 {
				err := fmt.Errorf("实例停止失败：检测到实例浏览器仍在运行，但调试接口尚不可用，请等待浏览器启动完成后重试")
				profile.LastError = err.Error()
				return profile, err
			}
			a.markProfileRunningLocked(profileId, profile, nil, detection.PID, detection.DebugPort, true, "")
			log.Warn("停止前发现同一用户数据目录浏览器仍在运行，已同步实例状态",
				logger.F("profile_id", profileId),
				logger.F("user_data_dir", userDataDir),
				logger.F("pid", detection.PID),
				logger.F("debug_port", detection.DebugPort),
			)
		}
	}
	debugPort := profile.DebugPort
	if closeBrowserViaCDPForStop(debugPort, 5*time.Second) {
		a.markProfileStoppedLocked(profileId, profile)
		log.Info("实例停止", logger.F("profile_id", profileId), logger.F("method", "cdp"), logger.F("debug_port", debugPort))
		return profile, nil
	}

	if cmd != nil && cmd.Process != nil {
		if err := a.stopBrowserProcess(cmd); err != nil {
			log.Error("实例停止失败", logger.F("profile_id", profileId), logger.F("error", err))
			profile.LastError = err.Error()
			return profile, err
		}
	}

	if debugPort > 0 && debugPortReachableForStop(debugPort, 250*time.Millisecond) {
		err := fmt.Errorf("实例停止失败：浏览器仍在运行（调试端口 %d 仍可访问）", debugPort)
		log.Error("实例停止失败", logger.F("profile_id", profileId), logger.F("debug_port", debugPort), logger.F("reason", err.Error()))
		profile.LastError = err.Error()
		return profile, err
	}

	a.markProfileStoppedLocked(profileId, profile)
	log.Info("实例停止", logger.F("profile_id", profileId))
	return profile, nil
}

func (a *App) BrowserInstanceRestart(profileId string) (*BrowserProfile, error) {
	if _, err := a.BrowserInstanceStop(profileId); err != nil {
		return nil, err
	}
	return a.BrowserInstanceStart(profileId)
}
