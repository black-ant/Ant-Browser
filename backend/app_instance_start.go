package backend

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

func (a *App) browserInstanceStartInternal(profileId string, extraLaunchArgs []string, startURLs []string, skipDefaultStartURLs bool, preferVisibleWindow bool, forceDirectProxy bool, proxyId string, proxyConfig string) (*BrowserProfile, error) {
	input := newBrowserStartInput(profileId, extraLaunchArgs, startURLs, skipDefaultStartURLs, preferVisibleWindow, forceDirectProxy, proxyId, proxyConfig)
	return a.startBrowserInstanceWithInput(input)
}

func (a *App) browserInstanceStartFreshInternal(profileId string, extraLaunchArgs []string, startURLs []string, skipDefaultStartURLs bool, preferVisibleWindow bool, forceDirectProxy bool, proxyId string, proxyConfig string) (*BrowserProfile, error) {
	input := newBrowserStartInput(profileId, extraLaunchArgs, startURLs, skipDefaultStartURLs, preferVisibleWindow, forceDirectProxy, proxyId, proxyConfig)
	input.RejectExistingRuntime = true
	return a.startBrowserInstanceWithInput(input)
}

func (a *App) startBrowserInstanceWithInput(input browserStartInput) (*BrowserProfile, error) {
	a.browserMgr.Mutex.Lock()
	defer a.browserMgr.Mutex.Unlock()

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
