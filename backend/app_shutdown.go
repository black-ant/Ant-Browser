package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/logger"
	"fmt"
	"os/exec"
	stdruntime "runtime"
	"sync"
	"time"
)

type browserProcessSnapshot struct {
	profileID string
	cmd       *exec.Cmd
}

func (a *App) stopRuntimeServices() {
	a.stopServicesOnce.Do(func() {
		log := logger.New("App")

		a.stopAllBrowserProcessesForExit(log)

		if a.xrayMgr != nil {
			a.xrayMgr.StopAll()
		}
		a.clearProfileXrayBridges()

		if a.singboxMgr != nil {
			a.singboxMgr.StopAll()
		}

		if a.clashMgr != nil {
			a.clashMgr.StopAll()
		}

		if a.speedScheduler != nil {
			a.speedScheduler.Stop()
			a.speedScheduler = nil
		}

		if a.proxySourceScheduler != nil {
			a.proxySourceScheduler.Stop()
			a.proxySourceScheduler = nil
		}

		a.stopDashboardMonitor()

		if a.launchServer != nil {
			if err := a.launchServer.Stop(); err != nil {
				log.Error("LaunchServer 关闭失败", logger.F("error", err))
			}
			a.launchServer = nil
		}

		// 关闭所有 CDP sessions
		if a.cdpManager != nil {
			for _, sessionID := range a.cdpManager.ListSessions() {
				_ = a.cdpManager.CloseSession(sessionID)
			}
		}

		if err := killResidualRuntimeProcesses(a.appRoot); err != nil {
			log.Error("退出前清理残留进程失败", logger.F("error", err.Error()))
		}
	})
}

func (a *App) finalizeShutdown() {
	a.finalizeOnce.Do(func() {
		if a.db != nil {
			a.db.Close()
			a.db = nil
		}
		if err := logger.Close(); err != nil {
			fmt.Printf("关闭日志系统失败: %v\n", err)
		}
	})
}

func (a *App) stopAllBrowserProcessesForExit(log *logger.Logger) {
	if a.browserMgr == nil {
		return
	}

	stoppedAt := time.Now().Format(time.RFC3339)

	a.browserMgr.Mutex.Lock()
	processes := make([]browserProcessSnapshot, 0, len(a.browserMgr.BrowserProcesses))
	for profileID, cmd := range a.browserMgr.BrowserProcesses {
		if profile, ok := a.browserMgr.Profiles[profileID]; ok && profile != nil {
			profile.Running = false
			profile.Status = browser.StatusStopped
			profile.LastStopAt = stoppedAt
		}
		if cmd != nil && cmd.Process != nil {
			processes = append(processes, browserProcessSnapshot{
				profileID: profileID,
				cmd:       cmd,
			})
		}
	}
	a.browserMgr.BrowserProcesses = make(map[string]*exec.Cmd)
	a.browserMgr.Mutex.Unlock()

	if len(processes) == 0 {
		return
	}

	var wg sync.WaitGroup
	for _, item := range processes {
		wg.Add(1)
		go func(item browserProcessSnapshot) {
			defer wg.Done()

			pid := 0
			if item.cmd != nil && item.cmd.Process != nil {
				pid = item.cmd.Process.Pid
			}
			log.Info("退出前关闭浏览器窗口", logger.F("profile_id", item.profileID), logger.F("pid", pid))
			if err := stopProcessCmdForShutdown(item.cmd); err != nil {
				log.Error("退出前关闭浏览器窗口失败", logger.F("profile_id", item.profileID), logger.F("pid", pid), logger.F("error", err.Error()))
			}
		}(item)
	}
	wg.Wait()
}

func stopProcessCmdForShutdown(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	pid := cmd.Process.Pid
	if pid > 0 {
		if err := forceKillProcessTree(pid); err == nil || isProcessAlreadyFinished(err) {
			return nil
		}
	}

	err := cmd.Process.Kill()
	if err == nil || isProcessAlreadyFinished(err) {
		return nil
	}
	return err
}

func forceKillProcessTree(pid int) error {
	if pid <= 0 {
		return nil
	}
	if stdruntime.GOOS != "windows" {
		return fmt.Errorf("force kill process tree unsupported on %s", stdruntime.GOOS)
	}

	killCmd := exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", pid))
	hideWindow(killCmd)
	err := killCmd.Run()
	if err == nil {
		_ = waitProcessExitWindows(pid, 1500*time.Millisecond)
		return nil
	}
	if waitProcessExitWindows(pid, 300*time.Millisecond) {
		return nil
	}
	return err
}
