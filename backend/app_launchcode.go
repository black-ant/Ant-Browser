package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/launchcode"
	"ant-chrome/backend/internal/logger"
	"fmt"
	"strings"
)

// StartInstance 实现 launchcode.BrowserStarter 接口
func (a *App) StartInstance(profileId string) (*browser.Profile, error) {
	return a.BrowserInstanceStart(profileId)
}

// StartInstanceWithParams 实现 launchcode.BrowserStarterWithParams 接口
func (a *App) StartInstanceWithParams(profileId string, params launchcode.LaunchRequestParams) (*browser.Profile, error) {
	return a.BrowserInstanceStartWithParams(profileId, params.LaunchArgs, params.StartURLs, params.SkipDefaultStartURLs)
}

func (a *App) createBrowserProfileFromInput(input browser.ProfileInput) (*browser.Profile, error) {
	if a.browserMgr == nil {
		return nil, fmt.Errorf("browser manager not initialized")
	}

	profile, err := a.browserMgr.Create(input)
	if err != nil {
		return nil, err
	}

	if err := a.applyRequestedProfileLaunchCode(profile, input.LaunchCode); err != nil {
		if deleteErr := a.browserMgr.Delete(profile.ProfileId); deleteErr != nil {
			logger.New("Browser").Warn("创建实例后回滚失败",
				logger.F("profile_id", profile.ProfileId),
				logger.F("error", deleteErr.Error()),
			)
		}
		return nil, err
	}

	// 关联账号到实例
	if len(input.AccountIds) > 0 {
		if err := a.linkAccountsToProfile(profile.ProfileId, input.AccountIds); err != nil {
			logger.New("Browser").Warn("创建实例后关联账号失败",
				logger.F("profile_id", profile.ProfileId),
				logger.F("error", err.Error()),
			)
		}
	}

	return profile, nil
}

func (a *App) updateBrowserProfileFromInput(profileId string, input browser.ProfileInput) (*browser.Profile, error) {
	if a.browserMgr == nil {
		return nil, fmt.Errorf("browser manager not initialized")
	}

	var previous *browser.Profile
	if strings.TrimSpace(input.LaunchCode) != "" {
		a.browserMgr.InitData()
		a.browserMgr.Mutex.Lock()
		if existing := a.browserMgr.Profiles[strings.TrimSpace(profileId)]; existing != nil {
			snapshot := *existing
			previous = &snapshot
		}
		a.browserMgr.Mutex.Unlock()
	}

	profile, err := a.browserMgr.Update(profileId, input)
	if err != nil {
		return nil, err
	}

	if err := a.applyRequestedProfileLaunchCode(profile, input.LaunchCode); err != nil {
		if previous != nil {
			if _, rollbackErr := a.browserMgr.Update(profileId, browserProfileToInput(previous)); rollbackErr != nil {
				logger.New("Browser").Warn("更新实例后回滚失败",
					logger.F("profile_id", profileId),
					logger.F("error", rollbackErr.Error()),
				)
			}
		}
		return nil, err
	}

	// 关联账号到实例
	if len(input.AccountIds) > 0 {
		if err := a.linkAccountsToProfile(profile.ProfileId, input.AccountIds); err != nil {
			logger.New("Browser").Warn("更新实例后关联账号失败",
				logger.F("profile_id", profile.ProfileId),
				logger.F("error", err.Error()),
			)
		}
	}

	return profile, nil
}

func (a *App) applyRequestedProfileLaunchCode(profile *browser.Profile, requestedCode string) error {
	if profile == nil {
		return fmt.Errorf("profile is nil")
	}
	requestedCode = strings.TrimSpace(requestedCode)
	if requestedCode == "" {
		return nil
	}
	if a.launchCodeSvc == nil {
		return fmt.Errorf("launch code service not initialized")
	}

	code, err := a.launchCodeSvc.SetCode(profile.ProfileId, requestedCode)
	if err != nil {
		return err
	}
	profile.LaunchCode = code
	return nil
}

func browserProfileToInput(profile *browser.Profile) browser.ProfileInput {
	if profile == nil {
		return browser.ProfileInput{}
	}
	return browser.ProfileInput{
		ProfileName:     strings.TrimSpace(profile.ProfileName),
		UserDataDir:     strings.TrimSpace(profile.UserDataDir),
		CoreId:          strings.TrimSpace(profile.CoreId),
		FingerprintArgs: append([]string{}, profile.FingerprintArgs...),
		ProxyId:         strings.TrimSpace(profile.ProxyId),
		ProxyConfig:     strings.TrimSpace(profile.ProxyConfig),
		LaunchArgs:      append([]string{}, profile.LaunchArgs...),
		Tags:            append([]string{}, profile.Tags...),
		Keywords:        append([]string{}, profile.Keywords...),
		GroupId:         strings.TrimSpace(profile.GroupId),
		LaunchCode:      strings.TrimSpace(profile.LaunchCode),
		AccountIds:      nil, // 回滚时不恢复账号关联
	}
}

// BrowserProfileGetCode 获取实例的 LaunchCode（Wails 绑定）
func (a *App) BrowserProfileGetCode(profileId string) (string, error) {
	if a.launchCodeSvc == nil {
		return "", nil
	}
	return a.launchCodeSvc.EnsureCode(profileId)
}

// BrowserProfileRegenerateCode 重新生成实例的 LaunchCode（Wails 绑定）
func (a *App) BrowserProfileRegenerateCode(profileId string) (string, error) {
	if a.launchCodeSvc == nil {
		return "", nil
	}
	return a.launchCodeSvc.RegenerateCode(profileId)
}

// BrowserProfileSetCode 自定义设置实例 LaunchCode（Wails 绑定）
func (a *App) BrowserProfileSetCode(profileId string, code string) (string, error) {
	if a.launchCodeSvc == nil {
		return "", nil
	}
	return a.launchCodeSvc.SetCode(profileId, code)
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

// CreateProfile 实现 launchcode.profileCreator 接口
func (a *App) CreateProfile(input browser.ProfileInput) (*browser.Profile, error) {
	return a.createBrowserProfileFromInput(input)
}

// UpdateProfile 实现 launchcode.profileUpdater 接口
func (a *App) UpdateProfile(profileID string, input browser.ProfileInput) (*browser.Profile, error) {
	return a.updateBrowserProfileFromInput(profileID, input)
}

// DeleteProfile 实现 launchcode.profileDeleter 接口
func (a *App) DeleteProfile(profileID string) error {
	if a.browserMgr == nil {
		return fmt.Errorf("browser manager not initialized")
	}
	return a.browserMgr.Delete(profileID)
}

// StopInstance 实现 launchcode.BrowserStopper 接口
func (a *App) StopInstance(profileID string) error {
	_, err := a.BrowserInstanceStop(profileID)
	return err
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
		}
	} else {
		info["baseUrl"] = ""
		info["cdpUrl"] = ""
		info["activeDebugPort"] = 0
	}
	return info
}

// 确保编译器检查 App 实现了 BrowserStarter 接口
var _ launchcode.BrowserStarter = (*App)(nil)
var _ launchcode.BrowserStarterWithParams = (*App)(nil)
