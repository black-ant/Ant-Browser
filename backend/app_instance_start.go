package backend

import "errors"

var errBrowserStartRuntimeStopped = errors.New("应用正在关闭，不能启动实例")

func (a *App) BrowserInstanceStart(profileId string) (*BrowserProfile, error) {
	return a.browserInstanceStartInternal(profileId, nil, nil, false, false, false, "", "")
}

func shouldPreferVisibleWindowForStartWithParams(startURLs []string) bool {
	return len(normalizeNonEmptyStrings(startURLs)) > 0
}

// BrowserInstanceStartDirect 仅本次启动走直连，不落库修改实例代理配置。
func (a *App) BrowserInstanceStartDirect(profileId string) (*BrowserProfile, error) {
	return a.browserInstanceStartInternal(profileId, nil, nil, false, false, true, "", "")
}

// BrowserInstanceStartWithParams 通过额外参数启动实例（仅本次启动生效，不落库）
func (a *App) BrowserInstanceStartWithParams(profileId string, extraLaunchArgs []string, startURLs []string, skipDefaultStartURLs bool) (*BrowserProfile, error) {
	preferVisibleWindow := shouldPreferVisibleWindowForStartWithParams(startURLs)
	return a.browserInstanceStartInternal(profileId, extraLaunchArgs, startURLs, skipDefaultStartURLs, preferVisibleWindow, false, "", "")
}

func (a *App) browserInstanceStartAllowed() bool {
	if a == nil {
		return false
	}
	a.runtimeMu.RLock()
	defer a.runtimeMu.RUnlock()
	return !a.runtimeStopped
}

func (a *App) shouldAbortBrowserStartForShutdown() bool {
	if a == nil {
		return true
	}
	a.runtimeMu.RLock()
	runtimeStopped := a.runtimeStopped
	a.runtimeMu.RUnlock()
	return runtimeStopped && a.shouldStopRuntimeServicesOnShutdown()
}

func (a *App) browserInstanceStartInternal(profileId string, extraLaunchArgs []string, startURLs []string, skipDefaultStartURLs bool, preferVisibleWindow bool, forceDirectProxy bool, proxyId string, proxyConfig string) (*BrowserProfile, error) {
	if !a.browserInstanceStartAllowed() {
		return nil, errBrowserStartRuntimeStopped
	}
	release, _ := a.acquireInstanceLaunchSlot(profileId)
	admissionKept := false
	defer func() {
		if !admissionKept {
			release()
		}
	}()
	input := newBrowserStartInput(profileId, extraLaunchArgs, startURLs, skipDefaultStartURLs, preferVisibleWindow, forceDirectProxy, proxyId, proxyConfig)
	profile, err := a.startBrowserInstanceWithInput(input)
	if err != nil {
		return profile, err
	}
	// 启动成功（含"已在运行被复用"）：名额保持占用，直到实例停止时释放。
	admissionKept = true
	return profile, nil
}

func (a *App) browserInstanceStartFreshInternal(profileId string, extraLaunchArgs []string, startURLs []string, skipDefaultStartURLs bool, preferVisibleWindow bool, forceDirectProxy bool, proxyId string, proxyConfig string) (*BrowserProfile, error) {
	if !a.browserInstanceStartAllowed() {
		return nil, errBrowserStartRuntimeStopped
	}
	release, _ := a.acquireInstanceLaunchSlot(profileId)
	admissionKept := false
	defer func() {
		if !admissionKept {
			release()
		}
	}()
	input := newBrowserStartInput(profileId, extraLaunchArgs, startURLs, skipDefaultStartURLs, preferVisibleWindow, forceDirectProxy, proxyId, proxyConfig)
	input.RejectExistingRuntime = true
	profile, err := a.startBrowserInstanceWithInput(input)
	if err != nil {
		return profile, err
	}
	admissionKept = true
	return profile, nil
}

func (a *App) startBrowserInstanceWithInput(input browserStartInput) (*BrowserProfile, error) {
	a.browserMgr.Mutex.Lock()
	defer a.browserMgr.Mutex.Unlock()
	if !a.browserInstanceStartAllowed() {
		return nil, errBrowserStartRuntimeStopped
	}

	profile, handled, err := a.resolveBrowserStartProfile(input)
	if err != nil || handled {
		return profile, err
	}

	plan, err := a.prepareBrowserStartPlan(input, profile)
	if err == errBrowserStartHandledByRecoveredRuntime {
		a.emitBrowserInstanceStarted(profile, true)
		return profile, nil
	}
	if err != nil {
		return profile, err
	}
	defer plan.releaseBridgeIfNeeded(a)

	return a.startBrowserProfileWithPlan(input, plan)
}
