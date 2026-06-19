package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/logger"
	"ant-chrome/backend/internal/proxy"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	stdruntime "runtime"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ============================================================================
// 浏览器实例管理 API
// ============================================================================

func (a *App) BrowserInstanceStart(profileId string) (*BrowserProfile, error) {
	return a.browserInstanceStartInternal(profileId, nil, nil, false, false)
}

// BrowserInstanceStartWithParams 通过额外参数启动实例（仅本次启动生效，不落库）
func (a *App) BrowserInstanceStartWithParams(profileId string, extraLaunchArgs []string, startURLs []string, skipDefaultStartURLs bool) (*BrowserProfile, error) {
	return a.browserInstanceStartInternal(profileId, extraLaunchArgs, startURLs, skipDefaultStartURLs, true)
}

func (a *App) browserInstanceStartInternal(profileId string, extraLaunchArgs []string, startURLs []string, skipDefaultStartURLs bool, preferVisibleWindow bool) (*BrowserProfile, error) {
	log := logger.New("Browser")

	normalizedExtraLaunchArgs := normalizeNonEmptyStrings(extraLaunchArgs)
	normalizedStartURLs := normalizeNonEmptyStrings(startURLs)
	if preferVisibleWindow {
		normalizedExtraLaunchArgs = ensureNewWindowLaunchArg(normalizedExtraLaunchArgs)
	}

	// ===== Phase 1：认领（短临界区，持大锁）=====
	a.browserMgr.Mutex.Lock()
	profile, exists := a.browserMgr.Profiles[profileId]
	if !exists {
		a.browserMgr.Mutex.Unlock()
		err := fmt.Errorf("实例启动失败：未找到实例配置（ID=%s）。请刷新列表后重试。", profileId)
		log.Error("实例不存在", logger.F("profile_id", profileId), logger.F("reason", err.Error()))
		return nil, err
	}
	if profile.Running {
		if !isBrowserProfileLive(profile, a.browserMgr.BrowserProcesses[profileId]) {
			log.Info("检测到实例运行状态已失效，准备重新启动",
				logger.F("profile_id", profileId),
				logger.F("pid", profile.Pid),
				logger.F("debug_port", profile.DebugPort),
			)
			a.markProfileStoppedLocked(profileId, profile)
		} else {
			if preferVisibleWindow {
				if err := a.openBrowserWindowForRunningProfile(profile, normalizedExtraLaunchArgs, normalizedStartURLs); err != nil {
					startErr := fmt.Errorf("实例已在运行，但窗口唤起失败：%w", err)
					profile.LastError = startErr.Error()
					snapshot := copyBrowserProfileSnapshot(profile)
					a.browserMgr.Mutex.Unlock()
					log.Error("运行中实例窗口唤起失败",
						logger.F("profile_id", profileId),
						logger.F("debug_port", snapshot.DebugPort),
						logger.F("error", err.Error()),
						logger.F("reason", startErr.Error()),
					)
					return snapshot, startErr
				}
			}
			if a.launchServer != nil && profile.DebugReady {
				a.launchServer.SetActiveProfile(profile)
			}
			snapshot := copyBrowserProfileSnapshot(profile)
			a.browserMgr.Mutex.Unlock()
			a.emitBrowserInstanceStarted(snapshot, true)
			return snapshot, nil
		}
	}

	// 防止同一实例并发重复启动
	if a.browserMgr.StartingProfiles[profileId] {
		snapshot := copyBrowserProfileSnapshot(profile)
		a.browserMgr.Mutex.Unlock()
		err := fmt.Errorf("实例正在启动中，请稍候。")
		log.Warn("重复启动被拒绝（实例正在启动中）", logger.F("profile_id", profileId))
		return snapshot, err
	}

	// 应用默认配置（可能修改 profile）；仅在代理变化时持久化，DAO 模式用单条 upsert，避免遍历全表拖慢临界区
	if proxyChanged := a.browserMgr.ApplyDefaults(profile); proxyChanged {
		if a.browserMgr.ProfileDAO != nil {
			if err := a.browserMgr.ProfileDAO.Upsert(profile); err != nil {
				log.Error("启动前持久化代理变更失败", logger.F("profile_id", profileId), logger.F("error", err.Error()))
			}
		} else {
			_ = a.browserMgr.SaveProfiles()
		}
	}

	// 认领 + 置 starting，并拷贝快照供锁外慢操作只读
	a.browserMgr.StartingProfiles[profileId] = true
	profile.Status = browser.StatusStarting
	profile.LastError = ""
	snap := *profile
	a.browserMgr.Mutex.Unlock()

	// 兜底清理：函数退出时若认领仍在（未走到明确成功/失败落地），清除并把 starting 回落为 stopped
	defer func() {
		a.browserMgr.Mutex.Lock()
		if a.browserMgr.StartingProfiles[profileId] {
			delete(a.browserMgr.StartingProfiles, profileId)
			if p, ok := a.browserMgr.Profiles[profileId]; ok && p != nil && p.Status == browser.StatusStarting {
				p.Status = browser.StatusStopped
			}
		}
		a.browserMgr.Mutex.Unlock()
	}()

	// 立即广播 starting 状态，前端无需等待轮询
	startingSnapshot := snap
	a.emitBrowserInstanceUpdated(&startingSnapshot)

	// commitFailure：失败时写回 LastError + 置 stopped + 清认领，返回快照
	commitFailure := func(startErr error) *BrowserProfile {
		a.recordActivity("start", "error", startErr.Error(), snap.ProfileName)
		a.browserMgr.Mutex.Lock()
		defer a.browserMgr.Mutex.Unlock()
		delete(a.browserMgr.StartingProfiles, profileId)
		if p, ok := a.browserMgr.Profiles[profileId]; ok && p != nil {
			p.Status = browser.StatusStopped
			p.LastError = startErr.Error()
			return copyBrowserProfileSnapshot(p)
		}
		return nil
	}

	// ===== Phase 2：慢操作（不持大锁，受启动队列限流）=====
	if err := a.startupQueue.Acquire(profileId); err != nil {
		startErr := fmt.Errorf("实例启动失败：%v", err)
		log.Error("启动队列等待失败", logger.F("profile_id", profileId), logger.F("error", err.Error()))
		return commitFailure(startErr), startErr
	}
	defer a.startupQueue.Release()

	// 启动前检查：内核路径、用户数据目录权限、磁盘空间
	if pre := a.browserMgr.ValidateStartupPreconditions(&snap); !pre.OK {
		startErr := fmt.Errorf("实例启动失败：%s检查未通过 - %s", pre.Stage, pre.Message)
		log.Error("启动前检查未通过", logger.F("profile_id", profileId), logger.F("stage", pre.Stage), logger.F("reason", pre.Message))
		return commitFailure(startErr), startErr
	}

	sanitizedProfileLaunchArgs, managedProfileArgs := sanitizeManagedLaunchArgs(snap.LaunchArgs)
	sanitizedExtraLaunchArgs, managedExtraArgs := sanitizeManagedLaunchArgs(normalizedExtraLaunchArgs)
	logManagedLaunchArgOverrides(log, profileId, "profile.launchArgs", managedProfileArgs)
	logManagedLaunchArgOverrides(log, profileId, "start.extraLaunchArgs", managedExtraArgs)

	chromeBinaryPath, err := a.browserMgr.ResolveChromeBinary(&snap)
	if err != nil {
		startErr := fmt.Errorf("实例启动失败：%w", err)
		log.Error("内核路径解析失败", logger.F("profile_id", profileId), logger.F("error", err.Error()), logger.F("reason", startErr.Error()))
		return commitFailure(startErr), startErr
	}

	userDataDir := a.browserMgr.ResolveUserDataDir(&snap)
	if err := os.MkdirAll(userDataDir, 0755); err != nil {
		startErr := fmt.Errorf("实例启动失败：无法创建用户数据目录 %s。原因：%w。请检查目录权限或路径配置。", userDataDir, err)
		log.Error("用户数据目录创建失败", logger.F("profile_id", profileId), logger.F("dir", userDataDir), logger.F("error", err.Error()), logger.F("reason", startErr.Error()))
		return commitFailure(startErr), startErr
	}
	// 每次启动时合并默认书签（已存在的 URL 不重复添加）
	if err := browser.EnsureDefaultBookmarks(userDataDir, a.BookmarkList()); err != nil {
		log.Error("默认书签写入失败", logger.F("error", err.Error()))
	}

	proxies := a.getLatestProxies()
	acquiredXrayBridgeKey := ""
	releaseXrayBridge := false
	defer func() {
		if releaseXrayBridge && acquiredXrayBridgeKey != "" && a.xrayMgr != nil {
			a.xrayMgr.ReleaseBridge(acquiredXrayBridgeKey)
		}
	}()

	// 解析实际代理配置（可能来自 proxyId 引用）
	resolvedProxyConfig := strings.TrimSpace(snap.ProxyConfig)
	if snap.ProxyId != "" {
		for _, item := range proxies {
			if strings.EqualFold(item.ProxyId, snap.ProxyId) {
				resolvedProxyConfig = strings.TrimSpace(item.ProxyConfig)
				break
			}
		}
	}
	effectiveProxy := resolvedProxyConfig
	log.Info("代理配置检查",
		logger.F("profile_id", profileId),
		logger.F("proxy_id", snap.ProxyId),
		logger.F("profile_proxy_config", snap.ProxyConfig),
		logger.F("resolved_proxy_config", resolvedProxyConfig),
	)
	if supported, errorMsg := proxy.ValidateProxyConfig(resolvedProxyConfig, proxies, snap.ProxyId); !supported {
		startErr := fmt.Errorf("实例启动失败：%s", errorMsg)
		log.Error("代理配置无效", logger.F("profile_id", profileId), logger.F("proxy_id", snap.ProxyId), logger.F("error", errorMsg), logger.F("reason", startErr.Error()))
		return commitFailure(startErr), startErr
	}

	if proxy.IsSingBoxProtocol(resolvedProxyConfig) {
		// hysteria2 / tuic → sing-box 桥接
		socksURL, bridgeErr := a.singboxMgr.EnsureBridge(resolvedProxyConfig, proxies, snap.ProxyId)
		if bridgeErr != nil {
			startErr := fmt.Errorf("实例启动失败：代理桥接启动失败（sing-box）。原因：%v。请检查代理节点配置、sing-box 可执行文件是否存在，以及本地端口是否被占用。", bridgeErr)
			log.Error("代理桥接失败(sing-box)", logger.F("error", bridgeErr.Error()), logger.F("reason", startErr.Error()))
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, "proxy:bridge:failed", map[string]interface{}{
					"profileId":   profileId,
					"profileName": snap.ProfileName,
					"error":       startErr.Error(),
				})
			}
			return commitFailure(startErr), startErr
		}
		effectiveProxy = socksURL
		log.Info("sing-box 桥接成功", logger.F("socks_url", socksURL))
	} else if proxy.RequiresBridge(resolvedProxyConfig, proxies, snap.ProxyId) {
		// vmess / vless / trojan / ss → xray 桥接
		socksURL, bridgeKey, bridgeErr := a.xrayMgr.AcquireBridge(resolvedProxyConfig, proxies, snap.ProxyId)
		if bridgeErr != nil {
			startErr := fmt.Errorf("实例启动失败：代理桥接启动失败（xray）。原因：%v。请检查代理节点配置、xray 可执行文件是否存在，以及本地端口是否被占用。", bridgeErr)
			log.Error("代理桥接失败(xray)", logger.F("error", bridgeErr.Error()), logger.F("reason", startErr.Error()))
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, "proxy:bridge:failed", map[string]interface{}{
					"profileId":   profileId,
					"profileName": snap.ProfileName,
					"error":       startErr.Error(),
				})
			}
			return commitFailure(startErr), startErr
		}
		acquiredXrayBridgeKey = bridgeKey
		releaseXrayBridge = bridgeKey != ""
		effectiveProxy = socksURL
		log.Info("xray 桥接成功", logger.F("socks_url", socksURL))
	}

	startReadyTimeout, startStableWindow := a.browserStartTimingSettings()
	maxStartAttempts := browserStartAttemptCount()
	totalReadyTimeout := time.Duration(maxStartAttempts) * startReadyTimeout
	assignedDebugPort, err := nextAvailablePort()
	if err != nil {
		startErr := fmt.Errorf("实例启动失败：本地调试端口分配失败。原因：%v。请关闭占用端口的程序后重试。", err)
		log.Error("调试端口分配失败", logger.F("profile_id", profileId), logger.F("error", err.Error()), logger.F("reason", startErr.Error()))
		return commitFailure(startErr), startErr
	}

	args := []string{
		fmt.Sprintf("--user-data-dir=%s", userDataDir),
		fmt.Sprintf("--remote-debugging-port=%d", assignedDebugPort),
		"--disable-session-crashed-bubble",
	}

	hasFingerprint := false
	for _, arg := range snap.FingerprintArgs {
		if strings.HasPrefix(arg, "--fingerprint=") {
			hasFingerprint = true
			break
		}
	}
	if !hasFingerprint {
		seed := 0
		for _, char := range snap.ProfileId {
			seed = (seed << 5) - seed + int(char)
		}
		if seed < 0 {
			seed = -seed
		}
		args = append(args, fmt.Sprintf("--fingerprint=%d", seed))
	}

	if effectiveProxy == "direct://" {
		// 强制直连，覆盖系统全局代理
		args = append(args, "--proxy-server=direct://")
	} else if effectiveProxy != "" {
		args = append(args, fmt.Sprintf("--proxy-server=%s", effectiveProxy))
	}
	args = append(args, snap.FingerprintArgs...)
	args = append(args, sanitizedProfileLaunchArgs...)
	args = append(args, sanitizedExtraLaunchArgs...)
	// 注入绑定到该实例的本地扩展（启用 + 目录含 manifest.json）
	if extPaths := a.extensionLoadPathsForProfile(profileId); len(extPaths) > 0 {
		args = append(args, fmt.Sprintf("--load-extension=%s", strings.Join(extPaths, ",")))
		log.Info("注入扩展", logger.F("profile_id", profileId), logger.F("count", len(extPaths)))
	}
	args = appendLaunchTargets(args, &snap, normalizedStartURLs, skipDefaultStartURLs)

	cmd := exec.Command(chromeBinaryPath, args...)
	cmd.Dir = filepath.Dir(chromeBinaryPath)
	monitor, err := newBrowserProcessMonitor(cmd)
	if err != nil {
		startErr := fmt.Errorf("实例启动失败：无法建立浏览器错误输出捕获。可执行文件：%s。原因：%v。", chromeBinaryPath, err)
		log.Error("浏览器错误输出捕获初始化失败", logger.F("profile_id", profileId), logger.F("chrome", chromeBinaryPath), logger.F("error", err.Error()), logger.F("reason", startErr.Error()))
		return commitFailure(startErr), startErr
	}
	if err := cmd.Start(); err != nil {
		startErr := fmt.Errorf("%s", describeChromeProcessStartError(chromeBinaryPath, err))
		log.Error("浏览器进程启动失败", logger.F("profile_id", profileId), logger.F("chrome", chromeBinaryPath), logger.F("error", err.Error()), logger.F("reason", startErr.Error()))
		return commitFailure(startErr), startErr
	}
	monitor.Start()

	var lastStartErr error
	for attempt := 1; attempt <= maxStartAttempts; attempt++ {
		stableDebugPort, readyErr := waitBrowserDebugPortStable(assignedDebugPort, userDataDir, startReadyTimeout, startStableWindow, monitor)
		if readyErr == nil {
			// ===== Phase 3：提交成功（短临界区，持大锁）=====
			snapshot, committed := a.commitBrowserStart(profileId, cmd, cmd.Process.Pid, stableDebugPort, true, "")
			if !committed {
				_ = a.stopProcessCmd(cmd)
				startErr := fmt.Errorf("实例启动已取消（实例在启动过程中被移除或停止）")
				log.Warn("启动提交被取消", logger.F("profile_id", profileId), logger.F("debug_port", stableDebugPort))
				return nil, startErr
			}
			if acquiredXrayBridgeKey != "" {
				a.bindProfileXrayBridge(profileId, acquiredXrayBridgeKey)
				releaseXrayBridge = false
			}

			log.Info("实例启动",
				logger.F("profile_id", profileId),
				logger.F("debug_port", stableDebugPort),
				logger.F("pid", snapshot.Pid),
				logger.F("proxy", effectiveProxy),
				logger.F("attempt", attempt),
				logger.F("max_attempts", maxStartAttempts),
				logger.F("args", strings.Join(args, " ")),
			)
			a.emitBrowserInstanceStarted(snapshot, false)
			a.recordActivity("start", "info", "实例启动成功", snapshot.ProfileName)

			go a.waitBrowserProcess(profileId, monitor)
			return snapshot, nil
		}

		startErr := fmt.Errorf("%s", describeBrowserReadyFailure(chromeBinaryPath, assignedDebugPort, totalReadyTimeout, readyErr))
		lastStartErr = startErr
		log.Error("浏览器启动未就绪",
			logger.F("profile_id", profileId),
			logger.F("chrome", chromeBinaryPath),
			logger.F("debug_port", assignedDebugPort),
			logger.F("attempt", attempt),
			logger.F("max_attempts", maxStartAttempts),
			logger.F("error", readyErr.Error()),
			logger.F("reason", startErr.Error()),
		)

		if attempt < maxStartAttempts && shouldRetryBrowserReadyFailure(readyErr) {
			log.Warn("浏览器启动未就绪，继续检测",
				logger.F("profile_id", profileId),
				logger.F("debug_port", assignedDebugPort),
				logger.F("attempt", attempt),
				logger.F("next_attempt", attempt+1),
				logger.F("max_attempts", maxStartAttempts),
				logger.F("timeout_ms", startReadyTimeout.Milliseconds()),
			)
			continue
		}

		break
	}

	// 调试端口未就绪，但进程仍存活 → 转入后台附着（debug_pending）
	if shouldKeepBrowserRunningPendingDebugReady(assignedDebugPort, monitor) {
		runtimeWarning := browserDebugPendingWarning(totalReadyTimeout)
		pendingStartNotice := browserDebugPendingStartNotice(totalReadyTimeout)
		snapshot, committed := a.commitBrowserStart(profileId, cmd, cmd.Process.Pid, assignedDebugPort, false, runtimeWarning)
		if !committed {
			_ = a.stopProcessCmd(cmd)
			startErr := fmt.Errorf("实例启动已取消（实例在启动过程中被移除或停止）")
			log.Warn("启动提交被取消（待接管阶段）", logger.F("profile_id", profileId), logger.F("debug_port", assignedDebugPort))
			return nil, startErr
		}
		if acquiredXrayBridgeKey != "" {
			a.bindProfileXrayBridge(profileId, acquiredXrayBridgeKey)
			releaseXrayBridge = false
		}

		log.Warn("浏览器窗口已启动，但调试接口在等待窗口内未就绪，转入后台附着",
			logger.F("profile_id", profileId),
			logger.F("debug_port", assignedDebugPort),
			logger.F("pid", snapshot.Pid),
			logger.F("max_attempts", maxStartAttempts),
			logger.F("warning", runtimeWarning),
		)
		a.emitBrowserInstanceStarted(snapshot, false)
		a.recordActivity("start", "warn", "实例已启动，调试接口后台接管中", snapshot.ProfileName)
		go a.waitBrowserProcess(profileId, monitor)
		go a.waitBrowserDebugReadyAsync(profileId, assignedDebugPort, browserAsyncDebugAttachTimeout)

		a.setProfileLastError(profileId, pendingStartNotice)
		snapshot.LastError = pendingStartNotice
		return snapshot, fmt.Errorf("%s", pendingStartNotice)
	}

	// 进程已退出且未就绪 → 失败
	if lastStartErr == nil {
		lastStartErr = fmt.Errorf("实例启动失败：浏览器在等待窗口内仍未就绪")
	}
	return commitFailure(lastStartErr), lastStartErr
}

func (a *App) BrowserInstanceStop(profileId string) (*BrowserProfile, error) {
	log := logger.New("Browser")
	a.browserMgr.Mutex.Lock()
	defer a.browserMgr.Mutex.Unlock()

	profile, exists := a.browserMgr.Profiles[profileId]
	if !exists {
		return nil, fmt.Errorf("profile not found")
	}
	profile.Status = browser.StatusStopping

	cmd := a.browserMgr.BrowserProcesses[profileId]
	debugPort := profile.DebugPort
	if tryCloseBrowserViaCDP(debugPort, 5*time.Second) {
		a.markProfileStoppedLocked(profileId, profile)
		log.Info("实例停止", logger.F("profile_id", profileId), logger.F("method", "cdp"), logger.F("debug_port", debugPort))
		a.recordActivity("stop", "info", "实例已停止", profile.ProfileName)
		return profile, nil
	}

	if cmd != nil && cmd.Process != nil {
		if err := a.stopBrowserProcess(cmd); err != nil {
			log.Error("实例停止失败", logger.F("profile_id", profileId), logger.F("error", err))
			profile.LastError = err.Error()
			return profile, err
		}
	}

	if debugPort > 0 && canConnectDebugPort(debugPort, 250*time.Millisecond) {
		err := fmt.Errorf("实例停止失败：浏览器仍在运行（调试端口 %d 仍可访问）", debugPort)
		log.Error("实例停止失败", logger.F("profile_id", profileId), logger.F("debug_port", debugPort), logger.F("reason", err.Error()))
		profile.LastError = err.Error()
		return profile, err
	}

	a.markProfileStoppedLocked(profileId, profile)
	log.Info("实例停止", logger.F("profile_id", profileId))
	a.recordActivity("stop", "info", "实例已停止", profile.ProfileName)
	return profile, nil
}

func (a *App) BrowserInstanceRestart(profileId string) (*BrowserProfile, error) {
	if _, err := a.BrowserInstanceStop(profileId); err != nil {
		return nil, err
	}
	return a.BrowserInstanceStart(profileId)
}

// BrowserProfileBatchSetTags 批量为实例设置标签（追加模式：将 tags 加入已有标签；replace 模式：直接替换）
func (a *App) BrowserProfileBatchSetTags(profileIds []string, tags []string, replace bool) error {
	log := logger.New("Browser")
	a.browserMgr.Mutex.Lock()
	defer a.browserMgr.Mutex.Unlock()

	for _, profileId := range profileIds {
		profile, exists := a.browserMgr.Profiles[profileId]
		if !exists {
			continue
		}
		if replace {
			profile.Tags = tags
		} else {
			// 追加去重
			existing := make(map[string]struct{})
			for _, t := range profile.Tags {
				existing[t] = struct{}{}
			}
			for _, t := range tags {
				if _, ok := existing[t]; !ok {
					profile.Tags = append(profile.Tags, t)
					existing[t] = struct{}{}
				}
			}
		}
		profile.UpdatedAt = time.Now().Format(time.RFC3339)
		if a.browserMgr.ProfileDAO != nil {
			if err := a.browserMgr.ProfileDAO.Upsert(profile); err != nil {
				log.Error("批量设置标签失败", logger.F("profile_id", profileId), logger.F("error", err))
				return err
			}
		}
	}
	return nil
}

// BrowserProfileBatchRemoveTags 批量从实例移除指定标签
func (a *App) BrowserProfileBatchRemoveTags(profileIds []string, tags []string) error {
	log := logger.New("Browser")
	a.browserMgr.Mutex.Lock()
	defer a.browserMgr.Mutex.Unlock()

	removeSet := make(map[string]struct{})
	for _, t := range tags {
		removeSet[t] = struct{}{}
	}

	for _, profileId := range profileIds {
		profile, exists := a.browserMgr.Profiles[profileId]
		if !exists {
			continue
		}
		filtered := profile.Tags[:0]
		for _, t := range profile.Tags {
			if _, ok := removeSet[t]; !ok {
				filtered = append(filtered, t)
			}
		}
		profile.Tags = filtered
		profile.UpdatedAt = time.Now().Format(time.RFC3339)
		if a.browserMgr.ProfileDAO != nil {
			if err := a.browserMgr.ProfileDAO.Upsert(profile); err != nil {
				log.Error("批量移除标签失败", logger.F("profile_id", profileId), logger.F("error", err))
				return err
			}
		}
	}
	return nil
}

// BrowserRenameTag 重命名所有实例中的指定标签
func (a *App) BrowserRenameTag(oldName string, newName string) error {
	log := logger.New("Browser")
	oldName = strings.TrimSpace(oldName)
	newName = strings.TrimSpace(newName)
	if oldName == "" || newName == "" {
		return fmt.Errorf("标签名称不能为空")
	}

	a.browserMgr.Mutex.Lock()
	defer a.browserMgr.Mutex.Unlock()

	changedCount := 0
	for profileId, profile := range a.browserMgr.Profiles {
		tagChanged := false
		var newTags []string
		for _, t := range profile.Tags {
			if strings.EqualFold(t, oldName) {
				newTags = append(newTags, newName)
				tagChanged = true
			} else {
				newTags = append(newTags, t)
			}
		}

		if tagChanged {
			// 去重
			uniqueTags := make([]string, 0)
			seen := make(map[string]struct{})
			for _, t := range newTags {
				if _, ok := seen[t]; !ok {
					uniqueTags = append(uniqueTags, t)
					seen[t] = struct{}{}
				}
			}

			profile.Tags = uniqueTags
			profile.UpdatedAt = time.Now().Format(time.RFC3339)
			if a.browserMgr.ProfileDAO != nil {
				if err := a.browserMgr.ProfileDAO.Upsert(profile); err != nil {
					log.Error("重命名标签保存失败", logger.F("profile_id", profileId), logger.F("error", err))
					return err
				}
			}
			changedCount++
		}
	}

	if changedCount > 0 && a.browserMgr.ProfileDAO == nil {
		if err := a.browserMgr.SaveProfiles(); err != nil {
			return err
		}
	}

	if changedCount > 0 {
		log.Info("重命名标签成功", logger.F("old", oldName), logger.F("new", newName), logger.F("changed_profiles", changedCount))
	}
	return nil
}

func (a *App) BrowserInstanceStatus(profileId string) (*BrowserProfile, error) {
	a.browserMgr.Mutex.Lock()
	defer a.browserMgr.Mutex.Unlock()
	profile, exists := a.browserMgr.Profiles[profileId]
	if !exists {
		return nil, fmt.Errorf("profile not found")
	}
	return profile, nil
}

func (a *App) BrowserInstanceOpenUrl(profileId string, targetUrl string) bool {
	a.browserMgr.Mutex.Lock()
	profile, exists := a.browserMgr.Profiles[profileId]
	a.browserMgr.Mutex.Unlock()
	if !exists || !profile.Running {
		return false
	}
	return true
}

func (a *App) BrowserInstanceGetTabs(profileId string) []BrowserTab {
	return []BrowserTab{
		{TabId: "tab-1", Title: "新标签页", Url: "about:blank", Active: true},
		{TabId: "tab-2", Title: "示例站点", Url: "https://example.com", Active: false},
	}
}

func (a *App) waitBrowserProcess(profileId string, monitor *browserProcessMonitor) {
	err := monitor.Wait()

	log := logger.New("Browser")
	debugPort := 0
	profileName := profileId
	shouldMonitorDetached := false

	a.browserMgr.Mutex.Lock()
	profile, exists := a.browserMgr.Profiles[profileId]
	wasRunning := exists && profile.Running
	if exists {
		profileName = profile.ProfileName
		debugPort = profile.DebugPort
	}
	a.browserMgr.Mutex.Unlock()

	if wasRunning && debugPort > 0 {
		snapshot, changed := a.waitForBrowserDebugReady(profileId, debugPort, browserLauncherDetachGraceWindow)
		if snapshot != nil {
			if changed {
				log.Info("浏览器启动器进程退出后，调试接口延迟就绪",
					logger.F("profile_id", profileId),
					logger.F("debug_port", debugPort),
				)
				a.emitBrowserInstanceUpdated(snapshot)
			}
		}

		a.browserMgr.Mutex.Lock()
		profile, exists = a.browserMgr.Profiles[profileId]
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
			a.waitDetachedBrowser(profileId, debugPort)
			return
		}
	}

	a.browserMgr.Mutex.Lock()
	profile, exists = a.browserMgr.Profiles[profileId]
	wasRunning = exists && profile.Running
	if exists {
		profileName = profile.ProfileName
		a.markProfileStoppedLocked(profileId, profile)
		// 区分异常崩溃与正常退出：崩溃时置 crashed 状态并记录原因（在锁内写回，避免竞态）
		if wasRunning && err != nil && profile != nil {
			profile.Status = browser.StatusCrashed
			profile.LastError = fmt.Sprintf("实例运行异常退出：%s", err.Error())
		}
	}
	a.browserMgr.Mutex.Unlock()

	if a.ctx == nil {
		return
	}

	// 进程是正常退出（用户手动关闭）还是异常崩溃
	if wasRunning && err != nil {
		// 异常退出，推送崩溃通知
		log.Error("浏览器进程异常退出", logger.F("profile_id", profileId), logger.F("profile_name", profileName), logger.F("error", err))
		runtime.EventsEmit(a.ctx, "browser:instance:crashed", map[string]interface{}{
			"profileId":   profileId,
			"profileName": profileName,
			"error":       err.Error(),
		})
		a.recordActivity("crash", "error", fmt.Sprintf("实例异常退出：%s", err.Error()), profileName)
	} else {
		runtime.EventsEmit(a.ctx, "browser:instance:stopped", profileId)
	}
}

func (a *App) waitDetachedBrowser(profileId string, debugPort int) {
	const (
		pollInterval = 500 * time.Millisecond
		maxMisses    = 3
	)

	log := logger.New("Browser")
	misses := 0
	for {
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
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "browser:instance:stopped", profileId)
		}
		return
	}
}

func tryCloseBrowserViaCDP(debugPort int, timeout time.Duration) bool {
	if debugPort <= 0 || !canConnectDebugPort(debugPort, 250*time.Millisecond) {
		return false
	}

	_ = cdpBrowserCall(debugPort, "Browser.close", nil)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !canConnectDebugPort(debugPort, 250*time.Millisecond) {
			return true
		}
		time.Sleep(150 * time.Millisecond)
	}
	return false
}

func normalizeNonEmptyStrings(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		v := strings.TrimSpace(item)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

func ensureNewWindowLaunchArg(args []string) []string {
	for _, arg := range args {
		if strings.EqualFold(strings.TrimSpace(arg), "--new-window") {
			return args
		}
	}
	return append(args, "--new-window")
}

func appendLaunchTargets(args []string, profile *BrowserProfile, startURLs []string, skipDefaultStartURLs bool) []string {
	if len(startURLs) > 0 {
		return append(args, startURLs...)
	}
	if !skipDefaultStartURLs {
		return browser.BuildLaunchArgs(args, profile)
	}
	return args
}

func (a *App) markProfileStoppedLocked(profileId string, profile *BrowserProfile) {
	if profile == nil {
		return
	}
	profile.Running = false
	profile.DebugReady = false
	profile.Status = browser.StatusStopped
	profile.Pid = 0
	profile.DebugPort = 0
	profile.RuntimeWarning = ""
	profile.LastStopAt = time.Now().Format(time.RFC3339)
	delete(a.browserMgr.BrowserProcesses, profileId)
	// 统一在此取消启动认领：若实例在启动过程中被停止，进行中的启动会在提交阶段
	// 检测到认领丢失而中止（见 browserInstanceStartInternal 的 Phase 3）。
	delete(a.browserMgr.StartingProfiles, profileId)
	a.releaseProfileXrayBridge(profileId)
	if a.launchServer != nil {
		a.launchServer.ClearActiveProfile(profileId)
	}
}

func (a *App) openBrowserWindowForRunningProfile(profile *BrowserProfile, extraLaunchArgs []string, startURLs []string) error {
	chromeBinaryPath, err := a.browserMgr.ResolveChromeBinary(profile)
	if err != nil {
		return err
	}

	userDataDir := a.browserMgr.ResolveUserDataDir(profile)
	if err := os.MkdirAll(userDataDir, 0755); err != nil {
		return fmt.Errorf("无法创建用户数据目录 %s：%w", userDataDir, err)
	}

	args := []string{
		fmt.Sprintf("--user-data-dir=%s", userDataDir),
	}
	sanitizedExtraLaunchArgs, managedExtraArgs := sanitizeManagedLaunchArgs(extraLaunchArgs)
	logManagedLaunchArgOverrides(logger.New("Browser"), profile.ProfileId, "running-window.extraLaunchArgs", managedExtraArgs)
	args = append(args, sanitizedExtraLaunchArgs...)
	if len(startURLs) > 0 {
		args = append(args, startURLs...)
	} else {
		args = append(args, "about:blank")
	}

	cmd := exec.Command(chromeBinaryPath, args...)
	cmd.Dir = filepath.Dir(chromeBinaryPath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%s", describeChromeProcessStartError(chromeBinaryPath, err))
	}

	go func() {
		_ = cmd.Wait()
	}()
	return nil
}

func (a *App) stopBrowserProcess(cmd *exec.Cmd) error {
	return a.stopProcessCmd(cmd)
}

func (a *App) stopProcessCmd(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	// Windows 下优先非强制 taskkill，尽量让 Chromium 走正常退出路径，减少“恢复页面”提示。
	if stdruntime.GOOS == "windows" {
		pid := cmd.Process.Pid
		if pid > 0 {
			softKillCmd := exec.Command("taskkill", "/PID", fmt.Sprintf("%d", pid), "/T")
			hideWindow(softKillCmd)
			if err := softKillCmd.Run(); err == nil {
				if waitProcessExitWindows(pid, 3*time.Second) {
					return nil
				}
				forceKillCmd := exec.Command("taskkill", "/F", "/PID", fmt.Sprintf("%d", pid), "/T")
				hideWindow(forceKillCmd)
				if forceErr := forceKillCmd.Run(); forceErr == nil {
					_ = waitProcessExitWindows(pid, 2*time.Second)
					return nil
				}
			}
		}
	}

	err := cmd.Process.Kill()
	if err == nil || isProcessAlreadyFinished(err) {
		return nil
	}
	return err
}

func isProcessAlreadyFinished(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	if msg == "" {
		return false
	}
	if strings.Contains(msg, "process already finished") {
		return true
	}
	if strings.Contains(msg, "not found") {
		return true
	}
	if strings.Contains(msg, "no process") {
		return true
	}
	if strings.Contains(msg, "不存在") {
		return true
	}
	return false
}

func waitProcessExitWindows(pid int, timeout time.Duration) bool {
	if pid <= 0 {
		return true
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		alive, err := isProcessAliveWindows(pid)
		if err == nil && !alive {
			return true
		}
		time.Sleep(150 * time.Millisecond)
	}
	alive, err := isProcessAliveWindows(pid)
	if err != nil {
		return false
	}
	return !alive
}

func isProcessAliveWindows(pid int) (bool, error) {
	cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH")
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return false, err
	}
	line := strings.TrimSpace(string(out))
	if line == "" {
		return false, nil
	}
	if strings.HasPrefix(strings.ToUpper(line), "INFO:") {
		return false, nil
	}
	token := fmt.Sprintf("\",\"%d\",", pid)
	return strings.Contains(line, token), nil
}
