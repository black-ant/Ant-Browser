package backend

import (
	"ant-chrome/backend/internal/logger"
	"context"
	"os/exec"
	goruntime "runtime"
	"strings"
)

func (a *App) shutdown(ctx context.Context) {
	log := logger.New("App")
	a.stopRuntimeEvents()
	a.stopBackupScheduler()
	if a.shouldStopRuntimeServicesOnShutdown() {
		log.Info("应用正在关闭...")
		a.stopRuntimeServices()
	} else {
		log.Info("应用正在关闭（保留当前已打开的浏览器实例）...")
	}
	a.finalizeShutdown()
}

func (a *App) GetInterceptor() *logger.MethodInterceptor {
	return a.interceptor
}

// ForceQuit 设置强制退出标志并调用 runtime.Quit
func (a *App) ForceQuit() {
	a.setQuitMode(quitModeFull)
	a.stopRuntimeEvents()
	a.stopRuntimeServices()
	a.runtimeQuit()
}

// QuitAppOnly 仅退出应用本身，保留当前已打开的浏览器实例。
func (a *App) QuitAppOnly() {
	a.setQuitMode(quitModeAppOnly)
	a.stopRuntimeEvents()
	a.runtimeQuit()
}

func Start(a *App, ctx context.Context) {
	a.startup(ctx)
}

func Stop(a *App, ctx context.Context) {
	a.shutdown(ctx)
}

func platformSupportsTrayCloseFlow() bool {
	return platformSupportsTrayCloseFlowForOS(goruntime.GOOS)
}

func platformSupportsTrayCloseFlowForOS(goos string) bool {
	return strings.EqualFold(strings.TrimSpace(goos), "windows")
}

func (a *App) setQuitMode(mode quitMode) {
	a.quitMu.Lock()
	defer a.quitMu.Unlock()
	a.forceQuit = true
	a.quitMode = mode
}

func (a *App) isQuitRequested() bool {
	a.quitMu.RLock()
	defer a.quitMu.RUnlock()
	return a.forceQuit
}

func (a *App) shouldStopRuntimeServicesOnShutdown() bool {
	a.quitMu.RLock()
	defer a.quitMu.RUnlock()
	return a.quitMode != quitModeAppOnly
}

func ShouldBlockClose(a *App, ctx context.Context) bool {
	if a.isQuitRequested() {
		return false
	}
	if !platformSupportsTrayCloseFlow() {
		return false
	}
	a.emitRuntimeEventWithContext(ctx, "app:request-close")
	return true
}

func (a *App) stopRuntimeServices() {
	a.stopServicesOnce.Do(func() {
		if a.automationMgr != nil {
			a.automationMgr.StopAllTasks()
		}
		if a.speedScheduler != nil {
			a.speedScheduler.Stop()
			a.speedScheduler = nil
		}
		a.stopTrackedBrowserProcesses()
		if a.xrayMgr != nil {
			a.xrayMgr.StopAll()
		}
		a.clearProfileProxyBridges()
		if a.clashMgr != nil {
			a.clashMgr.StopAll()
		}
		if a.singboxMgr != nil {
			a.singboxMgr.StopAll()
		}
	})
}

func (a *App) stopTrackedBrowserProcesses() {
	if a.browserMgr == nil {
		return
	}

	a.browserMgr.Mutex.Lock()
	cmds := make([]*exec.Cmd, 0, len(a.browserMgr.BrowserProcesses))
	for _, cmd := range a.browserMgr.BrowserProcesses {
		cmds = append(cmds, cmd)
	}
	a.browserMgr.Mutex.Unlock()

	for _, cmd := range cmds {
		_ = a.stopProcessCmd(cmd)
	}

	a.browserMgr.Mutex.Lock()
	defer a.browserMgr.Mutex.Unlock()

	for profileID, profile := range a.browserMgr.Profiles {
		if profile == nil {
			continue
		}
		if profile.Running || a.browserMgr.BrowserProcesses[profileID] != nil {
			a.markProfileStoppedLocked(profileID, profile)
		}
	}
	a.browserMgr.BrowserProcesses = make(map[string]*exec.Cmd)
}

func (a *App) finalizeShutdown() {
	a.finalizeOnce.Do(func() {
		if a.launchServer != nil {
			_ = a.launchServer.Stop()
		}
		if a.db != nil {
			_ = a.db.Close()
		}
		_ = logger.Close()
	})
}
