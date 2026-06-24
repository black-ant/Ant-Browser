package backend

import (
	"fmt"
	"os"
	"os/exec"
	stdruntime "runtime"
	"syscall"
	"time"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/logger"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	browserAsyncDebugAttachTimeout   = 45 * time.Second
	browserLauncherDetachGraceWindow = 15 * time.Second
)

func copyBrowserProfileSnapshot(profile *BrowserProfile) *BrowserProfile {
	if profile == nil {
		return nil
	}
	snapshot := *profile
	return &snapshot
}

func browserDebugPendingWarning(timeout time.Duration) string {
	return fmt.Sprintf("浏览器窗口已启动，但调试接口在 %s 内仍未就绪；系统会继续在后台连接。连接完成前，Cookie、自动化和统一 CDP 入口暂不可用。", formatBrowserWaitWindow(timeout))
}

func browserDebugPendingStartNotice(timeout time.Duration) string {
	return fmt.Sprintf("浏览器窗口已启动，但在 %s 内尚未完成接管；系统会继续在后台连接，请稍后查看实例状态。连接完成前，Cookie、自动化和统一 CDP 入口暂不可用。", formatBrowserWaitWindow(timeout))
}

func formatBrowserWaitWindow(timeout time.Duration) string {
	if timeout <= 0 {
		return "当前等待窗口"
	}

	rounded := timeout.Round(100 * time.Millisecond)
	if rounded%time.Second == 0 {
		return fmt.Sprintf("%d 秒", rounded/time.Second)
	}
	if rounded%time.Millisecond == 0 {
		return fmt.Sprintf("%d 毫秒", rounded/time.Millisecond)
	}
	return rounded.String()
}

func browserInstanceEventPayload(profile *BrowserProfile, reused bool) map[string]interface{} {
	if profile == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"profileId":      profile.ProfileId,
		"profileName":    profile.ProfileName,
		"status":         profile.Status,
		"debugPort":      profile.DebugPort,
		"debugReady":     profile.DebugReady,
		"pid":            profile.Pid,
		"reused":         reused,
		"running":        profile.Running,
		"runtimeWarning": profile.RuntimeWarning,
	}
}

func (a *App) emitBrowserInstanceStarted(profile *BrowserProfile, reused bool) {
	if a == nil || a.ctx == nil || profile == nil {
		return
	}
	runtime.EventsEmit(a.ctx, "browser:instance:started", browserInstanceEventPayload(profile, reused))
}

func (a *App) emitBrowserInstanceUpdated(profile *BrowserProfile) {
	if a == nil || a.ctx == nil || profile == nil {
		return
	}
	runtime.EventsEmit(a.ctx, "browser:instance:updated", browserInstanceEventPayload(profile, false))
}

func (a *App) markProfileRunningLocked(profileId string, profile *BrowserProfile, cmd *exec.Cmd, pid int, debugPort int, debugReady bool, runtimeWarning string) {
	if profile == nil {
		return
	}
	profile.Running = true
	profile.DebugPort = debugPort
	profile.DebugReady = debugReady
	profile.Pid = pid
	if debugReady {
		profile.Status = browser.StatusRunning
	} else {
		profile.Status = browser.StatusDebugPending
	}
	profile.LastStartAt = time.Now().Format(time.RFC3339)
	profile.RuntimeWarning = runtimeWarning
	profile.LastError = ""
	if cmd != nil {
		a.browserMgr.BrowserProcesses[profileId] = cmd
	}
	if debugReady && a.launchServer != nil {
		a.launchServer.SetActiveProfile(profile)
	}
}

func (a *App) markProfileDebugReadyLocked(profile *BrowserProfile, debugPort int) {
	if profile == nil {
		return
	}
	profile.DebugPort = debugPort
	profile.DebugReady = true
	profile.Status = browser.StatusRunning
	profile.RuntimeWarning = ""
	profile.LastError = ""
}

// commitBrowserStart 在短临界区内提交一次启动结果（Phase 3）。
// 若实例在启动期间被删除（Profiles 中不存在）或启动认领已被清除（被 Stop 取消），
// 返回 committed=false，调用方应杀掉已启动的进程并释放代理桥接等资源。
func (a *App) commitBrowserStart(profileId string, cmd *exec.Cmd, pid int, debugPort int, debugReady bool, runtimeWarning string) (*BrowserProfile, bool) {
	a.browserMgr.Mutex.Lock()
	defer a.browserMgr.Mutex.Unlock()

	profile, exists := a.browserMgr.Profiles[profileId]
	if !exists || profile == nil {
		delete(a.browserMgr.StartingProfiles, profileId)
		return nil, false
	}
	if !a.browserMgr.StartingProfiles[profileId] {
		// 认领已被清除（启动过程中被停止/取消）
		return nil, false
	}
	delete(a.browserMgr.StartingProfiles, profileId)
	a.markProfileRunningLocked(profileId, profile, cmd, pid, debugPort, debugReady, runtimeWarning)
	return copyBrowserProfileSnapshot(profile), true
}

// setProfileLastError 在短临界区内写回实例的 LastError（用于待接管提示等）。
func (a *App) setProfileLastError(profileId string, msg string) {
	a.browserMgr.Mutex.Lock()
	if p, ok := a.browserMgr.Profiles[profileId]; ok && p != nil {
		p.LastError = msg
	}
	a.browserMgr.Mutex.Unlock()
}

func (a *App) setProfileDebugReady(profileId string, debugPort int) (*BrowserProfile, bool) {
	if a == nil || a.browserMgr == nil {
		return nil, false
	}

	a.browserMgr.Mutex.Lock()
	profile, exists := a.browserMgr.Profiles[profileId]
	if !exists || profile == nil || !profile.Running || profile.DebugPort != debugPort {
		a.browserMgr.Mutex.Unlock()
		return nil, false
	}

	changed := !profile.DebugReady || profile.RuntimeWarning != ""
	if changed {
		a.markProfileDebugReadyLocked(profile, debugPort)
	}
	snapshot := copyBrowserProfileSnapshot(profile)
	a.browserMgr.Mutex.Unlock()

	if snapshot != nil && snapshot.DebugReady && a.launchServer != nil {
		a.launchServer.SetActiveProfile(snapshot)
	}
	return snapshot, changed
}

func (a *App) waitForBrowserDebugReady(profileId string, debugPort int, timeout time.Duration) (*BrowserProfile, bool) {
	if a == nil || a.browserMgr == nil || debugPort <= 0 || timeout <= 0 {
		return nil, false
	}

	deadline := time.Now().Add(timeout)
	for {
		a.browserMgr.Mutex.Lock()
		profile, exists := a.browserMgr.Profiles[profileId]
		if !exists || profile == nil || !profile.Running || profile.DebugPort != debugPort {
			a.browserMgr.Mutex.Unlock()
			return nil, false
		}
		if profile.DebugReady {
			snapshot := copyBrowserProfileSnapshot(profile)
			a.browserMgr.Mutex.Unlock()
			return snapshot, false
		}
		a.browserMgr.Mutex.Unlock()

		if err := probeBrowserDebugPort(debugPort, browserDebugProbeTimeout); err == nil {
			return a.setProfileDebugReady(profileId, debugPort)
		}
		if time.Now().After(deadline) {
			return nil, false
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func (a *App) waitBrowserDebugReadyAsync(profileId string, debugPort int, timeout time.Duration, overrides ...browserRuntimeOverrides) {
	snapshot, changed := a.waitForBrowserDebugReady(profileId, debugPort, timeout)
	if snapshot == nil || !changed {
		return
	}

	logger.New("Browser").Info("实例调试接口已就绪",
		logger.F("profile_id", profileId),
		logger.F("debug_port", debugPort),
	)
	if len(overrides) > 0 {
		if err := a.applyAndWatchBrowserRuntimeOverrides(profileId, debugPort, overrides[0]); err != nil {
			logger.New("Browser").Warn("运行时指纹覆盖应用失败",
				logger.F("profile_id", profileId),
				logger.F("geolocation_source", overrides[0].geolocationSource()),
				logger.F("timezone_source", overrides[0].timezoneSource()),
				logger.F("error", err.Error()))
		}
	}
	a.emitBrowserInstanceUpdated(snapshot)
}

func shouldKeepBrowserRunningPendingDebugReady(debugPort int, monitor *browserProcessMonitor) bool {
	return debugPort > 0 && monitor != nil && !monitor.HasExited()
}

func isBrowserProfileLive(profile *BrowserProfile, trackedCmd *exec.Cmd) bool {
	if profile == nil || !profile.Running {
		return false
	}
	if profile.DebugPort > 0 && canConnectDebugPort(profile.DebugPort, 250*time.Millisecond) {
		return true
	}
	if profile.Pid > 0 && isProcessAlive(profile.Pid) {
		return true
	}
	if trackedCmd != nil && trackedCmd.Process != nil && trackedCmd.Process.Pid > 0 {
		// Before exec.Cmd.Wait returns, ProcessState is nil. Treat this as alive so
		// a pending debug endpoint is not mistaken for a stopped browser instance.
		if trackedCmd.ProcessState == nil {
			return true
		}
		return !trackedCmd.ProcessState.Exited() || isProcessAlive(trackedCmd.Process.Pid)
	}
	return false
}

func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	if stdruntime.GOOS == "windows" {
		alive, err := isProcessAliveWindows(pid)
		return err == nil && alive
	}

	process, err := os.FindProcess(pid)
	if err != nil || process == nil {
		return false
	}
	return process.Signal(syscall.Signal(0)) == nil
}
