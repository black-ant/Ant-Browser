package backend

import (
	"ant-chrome/backend/internal/logger"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

func (a *App) startBrowserProfileWithPlan(input browserStartInput, plan *browserStartPlan) (*BrowserProfile, error) {
	log := logger.New("Browser")
	profile := plan.profile
	a.clearDeferredStartTargets(input.ProfileID)
	a.markProfileLastLaunchArgsLocked(profile, plan.args)

	cmd := exec.Command(plan.chromeBinaryPath, plan.args...)
	cmd.Dir = filepath.Dir(plan.chromeBinaryPath)

	monitor, err := newBrowserProcessMonitor(cmd)
	if err != nil {
		startErr := fmt.Errorf("实例启动失败：无法建立浏览器错误输出捕获。可执行文件：%s。原因：%v。", plan.chromeBinaryPath, err)
		log.Error("浏览器错误输出捕获初始化失败",
			logger.F("profile_id", input.ProfileID),
			logger.F("chrome", plan.chromeBinaryPath),
			logger.F("error", err.Error()),
			logger.F("reason", startErr.Error()),
		)
		profile.LastError = startErr.Error()
		return profile, startErr
	}
	if err := cmd.Start(); err != nil {
		startErr := fmt.Errorf("%s", describeChromeProcessStartError(plan.chromeBinaryPath, err))
		log.Error("浏览器进程启动失败",
			logger.F("profile_id", input.ProfileID),
			logger.F("chrome", plan.chromeBinaryPath),
			logger.F("error", err.Error()),
			logger.F("reason", startErr.Error()),
		)
		profile.LastError = startErr.Error()
		return profile, startErr
	}
	memoryLimitCleanup, err := applyBrowserProcessMemoryLimit(cmd, profile.MemoryLimitMB)
	if err != nil {
		_ = a.stopProcessCmd(cmd)
		startErr := fmt.Errorf("实例启动失败：无法应用实例内存限制 %d MB。原因：%v。", profile.MemoryLimitMB, err)
		log.Error("浏览器进程内存限制应用失败",
			logger.F("profile_id", input.ProfileID),
			logger.F("chrome", plan.chromeBinaryPath),
			logger.F("memory_limit_mb", profile.MemoryLimitMB),
			logger.F("error", err.Error()),
			logger.F("reason", startErr.Error()),
		)
		profile.LastError = startErr.Error()
		return profile, startErr
	}
	defer func() {
		if memoryLimitCleanup != nil {
			memoryLimitCleanup()
		}
	}()
	monitor.Start()

	var lastStartErr error
	for attempt := 1; attempt <= plan.maxStartAttempts; attempt++ {
		stableDebugPort, readyErr := waitBrowserDebugPortStable(plan.assignedDebugPort, plan.userDataDir, plan.startReadyTimeout, plan.startStableWindow, monitor)
		if readyErr == nil {
			a.markProfileRunningLocked(input.ProfileID, profile, cmd, cmd.Process.Pid, stableDebugPort, true, "")
			if plan.extensionWarning != "" {
				profile.RuntimeWarning = plan.extensionWarning
			}
			a.setBrowserProcessMonitorLocked(input.ProfileID, monitor)
			if plan.acquiredProxyBridge.valid() {
				a.bindProfileProxyBridge(input.ProfileID, plan.acquiredProxyBridge)
				plan.releaseProxyBridge = false
			}
			if len(plan.deferredStartTargets) > 0 {
				deferredPlan := deferredStartTargetsPlan{targets: plan.deferredStartTargets, newTabs: plan.deferredStartNewTabs}
				if err := openDeferredStartTargets(stableDebugPort, deferredPlan); err != nil {
					warning := deferredStartTargetsWarning(plan.deferredStartTargets, err)
					if plan.extensionWarning != "" {
						warning = plan.extensionWarning + "；" + warning
					}
					profile.RuntimeWarning = warning
					profile.LastError = ""
					log.Warn("浏览器已就绪，但启动页延后打开失败",
						logger.F("profile_id", input.ProfileID),
						logger.F("debug_port", stableDebugPort),
						logger.F("target_count", len(plan.deferredStartTargets)),
						logger.F("error", err.Error()),
						logger.F("warning", warning),
					)
				}
			}

			log.Info("实例启动",
				logger.F("profile_id", input.ProfileID),
				logger.F("debug_port", stableDebugPort),
				logger.F("pid", profile.Pid),
				logger.F("proxy", plan.effectiveProxy),
				logger.F("memory_limit_mb", profile.MemoryLimitMB),
				logger.F("attempt", attempt),
				logger.F("max_attempts", plan.maxStartAttempts),
				logger.F("args", strings.Join(plan.args, " ")),
			)
			a.emitBrowserInstanceStarted(profile, false)

			cleanup := memoryLimitCleanup
			memoryLimitCleanup = nil
			go func() {
				defer func() {
					if cleanup != nil {
						cleanup()
					}
				}()
				a.waitBrowserProcess(input.ProfileID, monitor)
			}()
			return profile, nil
		}

		startErr := fmt.Errorf("%s", describeBrowserReadyFailure(plan.chromeBinaryPath, plan.assignedDebugPort, plan.totalReadyTimeout, readyErr))
		lastStartErr = startErr
		log.Error("浏览器启动未就绪",
			logger.F("profile_id", input.ProfileID),
			logger.F("chrome", plan.chromeBinaryPath),
			logger.F("debug_port", plan.assignedDebugPort),
			logger.F("attempt", attempt),
			logger.F("max_attempts", plan.maxStartAttempts),
			logger.F("error", readyErr.Error()),
			logger.F("reason", startErr.Error()),
		)

		if attempt < plan.maxStartAttempts && shouldRetryBrowserReadyFailure(readyErr) {
			log.Warn("浏览器启动未就绪，继续检测",
				logger.F("profile_id", input.ProfileID),
				logger.F("debug_port", plan.assignedDebugPort),
				logger.F("attempt", attempt),
				logger.F("next_attempt", attempt+1),
				logger.F("max_attempts", plan.maxStartAttempts),
				logger.F("timeout_ms", plan.startReadyTimeout.Milliseconds()),
			)
			continue
		}

		break
	}

	pendingAttach := shouldKeepBrowserRunningPendingDebugReady(plan.assignedDebugPort, monitor)
	if pendingAttach {
		runtimeWarning := browserDebugPendingWarning(plan.totalReadyTimeout)
		if plan.extensionWarning != "" {
			runtimeWarning = runtimeWarning + "；" + plan.extensionWarning
		}
		a.markProfileRunningLocked(input.ProfileID, profile, cmd, cmd.Process.Pid, plan.assignedDebugPort, false, runtimeWarning)
		a.setBrowserProcessMonitorLocked(input.ProfileID, monitor)
		if len(plan.deferredStartTargets) > 0 {
			a.storeDeferredStartTargets(input.ProfileID, plan.deferredStartTargets, plan.deferredStartNewTabs)
		}
		if plan.acquiredProxyBridge.valid() {
			a.bindProfileProxyBridge(input.ProfileID, plan.acquiredProxyBridge)
			plan.releaseProxyBridge = false
		}

		log.Warn("浏览器窗口已启动，但调试接口在等待窗口内未就绪，转入后台附着",
			logger.F("profile_id", input.ProfileID),
			logger.F("debug_port", plan.assignedDebugPort),
			logger.F("pid", profile.Pid),
			logger.F("max_attempts", plan.maxStartAttempts),
			logger.F("warning", runtimeWarning),
		)
		a.emitBrowserInstanceStarted(profile, false)
		cleanup := memoryLimitCleanup
		memoryLimitCleanup = nil
		go func() {
			defer func() {
				if cleanup != nil {
					cleanup()
				}
			}()
			a.waitBrowserProcess(input.ProfileID, monitor)
		}()
		go a.waitBrowserDebugReadyAsync(input.ProfileID, plan.assignedDebugPort, browserAsyncDebugAttachTimeout, monitor)
	}

	if pendingAttach {
		return profile, nil
	}

	if lastStartErr != nil {
		a.clearDeferredStartTargets(input.ProfileID)
		profile.LastError = lastStartErr.Error()
		return profile, lastStartErr
	}

	a.clearDeferredStartTargets(input.ProfileID)
	startErr := fmt.Errorf("实例启动失败：浏览器在等待窗口内仍未就绪")
	profile.LastError = startErr.Error()
	return profile, startErr
}
