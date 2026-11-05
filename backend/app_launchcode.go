package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/launchcode"
	"fmt"
	"time"
)

// StartInstance 实现 launchcode.BrowserStarter 接口
func (a *App) StartInstance(profileId string) (*browser.Profile, error) {
	return a.BrowserInstanceStart(profileId)
}

// StartInstanceWithParams 实现 launchcode.BrowserStarterWithParams 接口
func (a *App) StartInstanceWithParams(profileId string, params launchcode.LaunchRequestParams) (*browser.Profile, error) {
	preferVisibleWindow := shouldPreferVisibleWindowForStartWithParams(params.StartURLs)
	return a.browserInstanceStartInternal(profileId, params.LaunchArgs, params.StartURLs, params.SkipDefaultStartURLs, preferVisibleWindow, false, params.ProxyId, params.ProxyConfig)
}

// StatusInstance 实现 launchcode.BrowserStatusProvider 接口
func (a *App) StatusInstance(profileId string) (*browser.Profile, error) {
	return a.BrowserInstanceStatus(profileId)
}

// StopInstance 实现 launchcode.BrowserStopper 接口
func (a *App) StopInstance(profileId string) (*browser.Profile, error) {
	return a.BrowserInstanceStop(profileId)
}

// WaitInstanceDebugReady 实现 launchcode.BrowserDebugWaiter 接口
func (a *App) WaitInstanceDebugReady(profileId string, debugPort int, timeout time.Duration) (*browser.Profile, bool, error) {
	if timeout <= 0 {
		profile, err := a.BrowserInstanceStatus(profileId)
		if err != nil {
			return nil, false, err
		}
		return profile, profile != nil && profile.DebugReady, nil
	}

	snapshot, _ := a.waitForBrowserDebugReady(profileId, debugPort, timeout)
	if snapshot != nil {
		return snapshot, snapshot.DebugReady, nil
	}

	profile, err := a.BrowserInstanceStatus(profileId)
	if err != nil {
		return nil, false, err
	}
	return profile, profile != nil && profile.DebugReady, nil
}

// BrowserProfileGetCode 获取实例的 LaunchCode（Wails 绑定）
func (a *App) BrowserProfileGetCode(profileId string) (string, error) {
	if a.launchCodeSvc == nil {
		return "", nil
	}
	code, err := a.launchCodeSvc.EnsureCode(profileId)
	if err == nil {
		a.setManagedProfileLaunchCode(profileId, code)
	}
	return code, err
}

// BrowserProfileRegenerateCode 重新生成实例的 LaunchCode（Wails 绑定）
func (a *App) BrowserProfileRegenerateCode(profileId string) (string, error) {
	if a.launchCodeSvc == nil {
		return "", nil
	}
	code, err := a.launchCodeSvc.RegenerateCode(profileId)
	if err == nil {
		a.setManagedProfileLaunchCode(profileId, code)
	}
	return code, err
}

// BrowserProfileSetCode 自定义设置实例 LaunchCode（Wails 绑定）
func (a *App) BrowserProfileSetCode(profileId string, code string) (string, error) {
	if a.launchCodeSvc == nil {
		return "", nil
	}
	updated, err := a.launchCodeSvc.SetCode(profileId, code)
	if err == nil {
		a.setManagedProfileLaunchCode(profileId, updated)
	}
	return updated, err
}

// BrowserInstanceStartByCode 通过 LaunchCode 启动实例（Wails 绑定）
func (a *App) BrowserInstanceStartByCode(code string) (*browser.Profile, error) {
	if a.launchCodeSvc == nil {
		return nil, fmt.Errorf("launch code service not initialized")
	}
	profileId, err := a.launchCodeSvc.Resolve(code)
	if err != nil {
		return nil, err
	}
	return a.BrowserInstanceStart(profileId)
}

// GetLaunchServerInfo 返回 LaunchServer 的当前监听信息（Wails 绑定）
func (a *App) GetLaunchServerInfo() map[string]interface{} {
	preferredPort := 0
	authRequested := false
	authConfigured := false
	authEnabled := false
	authHeader := launchcode.DefaultAPIKeyHeader
	if a.config != nil {
		preferredPort = a.config.LaunchServer.Port
		authRequested = a.config.LaunchServer.Auth.Enabled
		authConfigured = a.config.LaunchServer.Auth.APIKey != ""
		if header := a.config.LaunchServer.Auth.Header; header != "" {
			authHeader = header
		}
	}

	actualPort := 0
	if a.launchServer != nil {
		actualPort = a.launchServer.Port()
		authRequested = a.launchServer.APIAuthRequested()
		authConfigured = a.launchServer.APIAuthConfigured()
		authEnabled = a.launchServer.APIAuthEnabled()
		authHeader = a.launchServer.APIAuthHeader()
	}

	info := map[string]interface{}{
		"host":          "127.0.0.1",
		"preferredPort": preferredPort,
		"port":          actualPort,
		"ready":         actualPort > 0,
		"apiAuth": map[string]interface{}{
			"requested":  authRequested,
			"configured": authConfigured,
			"enabled":    authEnabled,
			"header":     authHeader,
		},
	}
	if actualPort > 0 {
		info["baseUrl"] = fmt.Sprintf("http://127.0.0.1:%d", actualPort)
		info["cdpUrl"] = fmt.Sprintf("http://127.0.0.1:%d", actualPort)
		if a.launchServer != nil {
			info["activeDebugPort"] = a.launchServer.ActiveDebugPort()
			activeProfileID, activeProfileName, _ := a.launchServer.ActiveProfile()
			info["activeProfileId"] = activeProfileID
			info["activeProfileName"] = activeProfileName
		}
	} else {
		info["baseUrl"] = ""
		info["cdpUrl"] = ""
		info["activeDebugPort"] = 0
		info["activeProfileId"] = ""
		info["activeProfileName"] = ""
	}
	return info
}

// 确保编译器检查 App 实现了 BrowserStarter 接口
var _ launchcode.BrowserStarter = (*App)(nil)
var _ launchcode.BrowserStarterWithParams = (*App)(nil)
var _ launchcode.BrowserStatusProvider = (*App)(nil)
var _ launchcode.BrowserStopper = (*App)(nil)
var _ launchcode.BrowserDebugWaiter = (*App)(nil)
var _ launchcode.AutomationScriptLister = (*App)(nil)
var _ launchcode.AutomationScriptGetter = (*App)(nil)
var _ launchcode.AutomationScriptRunner = (*App)(nil)
var _ launchcode.AutomationScriptRunLister = (*App)(nil)
