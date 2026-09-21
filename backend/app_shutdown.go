package backend

import (
	"ant-chrome/backend/internal/logger"
	"context"
	"os/exec"
	goruntime "runtime"
	"strings"
	"time"
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
		a.stopAppOnlyRuntimeServices()
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
	a.stopRuntimeEvents()
	a.waitBackgroundTasks()
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

func (a *App) stopAppOnlyRuntimeServices() {
	a.stopRuntimeEvents()
	a.waitBackgroundTasks()
	if a.automationMgr != nil {
		a.automationMgr.StopAllTasks()
	}
	if a.speedScheduler != nil {
		a.speedScheduler.Stop()
		a.speedScheduler = nil
	}
}

func (a *App) stopTrackedBrowserProcesses() {
	if a.browserMgr == nil {
		return
	}

	type shutdownTarget struct {
		cmd       *exec.Cmd
		debugPort int
	}

	a.browserMgr.Mutex.Lock()
	targets := make([]shutdownTarget, 0, len(a.browserMgr.BrowserProcesses))
	seen := make(map[string]struct{}, len(a.browserMgr.BrowserProcesses))
	for profileID, cmd := range a.browserMgr.BrowserProcesses {
		debugPort := 0
		if profile := a.browserMgr.Profiles[profileID]; profile != nil {
			debugPort = profile.DebugPort
		}
		targets = append(targets, shutdownTarget{cmd: cmd, debugPort: debugPort})
		seen[profileID] = struct{}{}
	}
	for profileID, profile := range a.browserMgr.Profiles {
		if profile == nil || !profile.Running {
			continue
		}
		if _, ok := seen[profileID]; ok {
			continue
		}
		targets = append(targets, shutdownTarget{debugPort: profile.DebugPort})
	}
	a.browserMgr.Mutex.Unlock()

	for _, target := range targets {
		if target.debugPort > 0 && tryCloseBrowserViaCDP(target.debugPort, 5*time.Second) {
			continue
		}
		if target.cmd != nil {
			_ = a.stopProcessCmd(target.cmd)
		}
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
