package backend

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/logger"
)

const (
	// 实例停止后的主动校验窗口：单次停止最多等这么久，仍未确认停止就再停一次。
	automationProfileStopVerifyTimeout = 12 * time.Second
	automationProfileStopVerifyStep    = 250 * time.Millisecond
)

// Chromium 单例锁文件名。强制结束进程后这些文件会残留，下次启动会被 Chrome 误判为"配置目录已被占用"。
var chromiumSingletonLockNames = []string{"SingletonLock", "SingletonSocket", "SingletonCookie"}

// automationTargetRunning 判断自动化目标实例在任务开始前是否已经在运行。
// 只有"任务自己拉起来的实例"才在任务结束时停止，避免打断用户手动打开的实例。
func (a *App) automationTargetRunning(profileID string, launchCode string) bool {
	if a == nil || a.browserMgr == nil {
		return false
	}
	profileID = strings.TrimSpace(profileID)
	launchCode = strings.ToUpper(strings.TrimSpace(launchCode))
	if profileID == "" && launchCode == "" {
		return false
	}

	a.browserMgr.Mutex.Lock()
	defer a.browserMgr.Mutex.Unlock()
	for _, profile := range a.browserMgr.Profiles {
		if profile == nil {
			continue
		}
		if profileID != "" {
			if strings.TrimSpace(profile.ProfileId) != profileID {
				continue
			}
		} else if !strings.EqualFold(strings.TrimSpace(profile.LaunchCode), launchCode) {
			continue
		}
		return profile.Running
	}
	return false
}

func (a *App) profileRunningByID(profileID string) bool {
	profile := a.automationProfileByID(profileID)
	return profile != nil && profile.Running
}

func (a *App) automationProfileByID(profileID string) *browser.Profile {
	if a == nil || a.browserMgr == nil {
		return nil
	}
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return nil
	}
	a.browserMgr.Mutex.Lock()
	defer a.browserMgr.Mutex.Unlock()
	profile := a.browserMgr.Profiles[profileID]
	if profile == nil {
		return nil
	}
	snapshot := *profile
	return &snapshot
}

// stopAutomationTargetProfile 停止自动化任务启动的实例并清理运行痕迹。
// 返回 ok=false 时调用方必须把 detail 写回运行记录，不能静默。
func (a *App) stopAutomationTargetProfile(profileID string) (bool, string) {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return true, ""
	}
	if !a.profileRuntimeAlive(profileID) {
		a.cleanupAutomationProfileLocksByID(profileID)
		return true, ""
	}

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if _, err := a.BrowserInstanceStop(profileID); err != nil {
			lastErr = err
		} else {
			lastErr = nil
		}
		deadline := time.Now().Add(automationProfileStopVerifyTimeout / 2)
		for time.Now().Before(deadline) {
			if !a.profileRuntimeAlive(profileID) {
				a.cleanupAutomationProfileLocksByID(profileID)
				return true, ""
			}
			time.Sleep(automationProfileStopVerifyStep)
		}
	}

	detail := "实例在任务结束后仍在运行"
	if lastErr != nil {
		detail += "：" + lastErr.Error()
	}
	logger.New("Automation").Error("自动化任务结束后实例未能停止",
		logger.F("profile_id", profileID),
		logger.F("detail", detail),
	)
	return false, detail
}

// profileRuntimeAlive 同时核对内存状态和调试端口，避免只信标记位。
//
// 这里不用 detectBrowserRuntimeByUserDataDir：它会扫描全部进程，Chrome 的
// zygote/GPU 子进程也带 --user-data-dir，主进程退出后这些子进程还会残留一小段时间，
// 巡检查询（ps -axo）又会把停止成功的实例误判成“仍在运行”。
func (a *App) profileRuntimeAlive(profileID string) bool {
	if a.profileRunningByID(profileID) {
		return true
	}
	profile := a.automationProfileByID(profileID)
	if profile == nil || a.browserMgr == nil {
		return false
	}
	userDataDir := strings.TrimSpace(a.browserMgr.ResolveUserDataDir(profile))
	if userDataDir == "" {
		return false
	}
	debugPort, err := readBrowserDebugPortFile(userDataDir)
	if err != nil || debugPort <= 0 {
		return false
	}
	return probeBrowserDebugPort(debugPort, browserDebugProbeTimeout) == nil
}

func (a *App) cleanupAutomationProfileLocksByID(profileID string) []string {
	profile := a.automationProfileByID(profileID)
	if profile == nil || a.browserMgr == nil {
		return nil
	}
	userDataDir := strings.TrimSpace(a.browserMgr.ResolveUserDataDir(profile))
	if userDataDir == "" {
		return nil
	}
	return removeChromiumSingletonLocks(userDataDir)
}

func removeChromiumSingletonLocks(userDataDir string) []string {
	userDataDir = strings.TrimSpace(userDataDir)
	if userDataDir == "" {
		return nil
	}
	removed := make([]string, 0, len(chromiumSingletonLockNames))
	for _, name := range chromiumSingletonLockNames {
		lockPath := filepath.Join(userDataDir, name)
		if _, err := os.Lstat(lockPath); err != nil {
			continue
		}
		if err := os.Remove(lockPath); err == nil {
			removed = append(removed, name)
		}
	}
	return removed
}

// automationRunKeepProfile 允许脚本显式要求保留实例（调试用），默认必须停止。
// 兼容既有脚本的 stopProfileAfterRun=false 语义，避免历史调用方行为突变。
func automationRunKeepProfile(params map[string]any) bool {
	if len(params) == 0 {
		return false
	}
	if value, ok := params["keepProfileAfterRun"].(bool); ok && value {
		return true
	}
	if value, ok := params["stopProfileAfterRun"].(bool); ok && !value {
		return true
	}
	return false
}

func appendAutomationCleanupError(existing string, detail string) string {
	existing = strings.TrimSpace(existing)
	detail = strings.TrimSpace(detail)
	if detail == "" {
		return existing
	}
	if existing == "" {
		return "实例清理失败：" + detail
	}
	return existing + "；实例清理失败：" + detail
}
