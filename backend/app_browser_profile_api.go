package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/gateway"
	"ant-chrome/backend/internal/logger"
	"ant-chrome/backend/internal/proxy"
	"fmt"
	"strings"
	"time"
)

type BrowserProfile = browser.Profile
type BrowserProfileInput = browser.ProfileInput
type BrowserTab = browser.Tab
type BrowserSettings = browser.Settings
type BrowserProxy = browser.Proxy
type BrowserCore = browser.Core
type BrowserCoreInput = browser.CoreInput
type BrowserCoreValidateResult = browser.CoreValidateResult
type BrowserCoreExtendedInfo = browser.CoreExtendedInfo
type BrowserProfileCopyOptions = browser.ProfileCopyOptions

// BrowserProfileList 获取所有实例列表
func (a *App) BrowserProfileList() []BrowserProfile {
	a.reconcileProfileRuntimeFromGateway()
	return a.browserMgr.List()
}

func (a *App) reconcileProfileRuntimeFromGateway() {
	if a == nil || a.browserMgr == nil {
		return
	}
	statuses := a.existingProxyGatewayStatuses()
	if len(statuses) == 0 {
		return
	}
	type recoveredRuntime struct {
		profileID string
		pid       int
		debugPort int
	}
	recovered := make([]recoveredRuntime, 0, len(statuses))
	for _, status := range statuses {
		alive := status.BrowserDebugPort > 0 && canConnectDebugPort(status.BrowserDebugPort, 250*time.Millisecond)
		if !alive && status.BrowserPID > 0 {
			alive = isProcessAlive(status.BrowserPID)
		}
		if alive {
			recovered = append(recovered, recoveredRuntime{
				profileID: strings.TrimSpace(status.ProfileID),
				pid:       status.BrowserPID,
				debugPort: status.BrowserDebugPort,
			})
		}
	}
	if len(recovered) == 0 {
		return
	}
	a.browserMgr.InitData()
	a.browserMgr.Mutex.Lock()
	defer a.browserMgr.Mutex.Unlock()
	for _, runtimeState := range recovered {
		profile := a.browserMgr.Profiles[runtimeState.profileID]
		if profile == nil || profile.Running {
			continue
		}
		a.markProfileRunningLocked(runtimeState.profileID, profile, nil, runtimeState.pid, runtimeState.debugPort, runtimeState.debugPort > 0, "")
	}
}

// BrowserProfileListByTag 按标签筛选实例列表
func (a *App) BrowserProfileListByTag(tag string) []BrowserProfile {
	return a.browserMgr.ListByTag(tag)
}

// BrowserGetAllTags 获取所有已使用的标签
func (a *App) BrowserGetAllTags() []string {
	return a.browserMgr.GetAllTags()
}

// BrowserProfileSetKeywords 设置实例关键字
func (a *App) BrowserProfileSetKeywords(profileId string, keywords []string) (*BrowserProfile, error) {
	return a.browserMgr.SetKeywords(profileId, keywords)
}

func (a *App) BrowserProfileCreate(input BrowserProfileInput) (*BrowserProfile, error) {
	return a.browserMgr.Create(input)
}

func (a *App) BrowserProfileUpdate(profileId string, input BrowserProfileInput) (*BrowserProfile, error) {
	current := a.profileForGatewayUpdate(profileId)
	if current == nil {
		return nil, fmt.Errorf("profile not found")
	}
	proxyChanged := strings.TrimSpace(current.ProxyId) != strings.TrimSpace(input.ProxyId) ||
		strings.TrimSpace(current.ProxyConfig) != strings.TrimSpace(input.ProxyConfig)
	var switched bool
	if current.Running && proxyChanged {
		if _, err := a.switchProfileGateway(profileId, input.ProxyId, input.ProxyConfig, false); err != nil {
			return nil, fmt.Errorf("运行中代理切换失败，原代理保持不变：%w", err)
		}
		switched = true
	}
	updated, err := a.browserMgr.Update(profileId, input)
	if err != nil && switched {
		_, _ = a.switchProfileGateway(profileId, current.ProxyId, current.ProxyConfig, false)
	}
	return updated, err
}

func (a *App) BrowserProfileSwitchProxy(profileId string, input BrowserProxySwitchInput) (*BrowserProxySwitchResult, error) {
	current := a.profileForGatewayUpdate(profileId)
	if current == nil {
		return nil, fmt.Errorf("profile not found")
	}
	proxies := a.getLatestProxies()
	resolvedID, resolvedConfig, err := resolveTemporaryBrowserStartProxy(input.ProxyID, input.ProxyConfig, proxies)
	if err != nil {
		return nil, err
	}
	if supported, message := proxy.ValidateProxyConfig(resolvedConfig, proxies, resolvedID); !supported {
		return nil, fmt.Errorf("invalid proxy configuration: %s", message)
	}
	status := ProxyGatewayStatus{ProfileID: profileId, Mode: a.proxyRoutingConfig(profileId).Mode, CurrentRouteID: resolvedID}
	if current.Running {
		status, err = a.switchProfileGateway(profileId, resolvedID, resolvedConfig, input.Force)
		if err != nil {
			return nil, fmt.Errorf("热切换失败，原代理保持不变：%w", err)
		}
	}
	updated, err := a.browserMgr.Update(profileId, browserProfileInputWithProxy(current, resolvedID, resolvedConfig))
	if err != nil {
		if current.Running {
			_, _ = a.switchProfileGateway(profileId, current.ProxyId, current.ProxyConfig, false)
		}
		return nil, err
	}
	return &BrowserProxySwitchResult{Profile: updated, Gateway: status, AppliedLive: current.Running}, nil
}

func (a *App) BrowserProfileDelete(profileId string) error {
	profile := a.profileForGatewayUpdate(profileId)
	if profile == nil {
		return fmt.Errorf("profile not found")
	}
	if profile.Running {
		return fmt.Errorf("running profile cannot be deleted")
	}
	if err := a.browserMgr.Delete(profileId); err != nil {
		return err
	}
	a.stopProfileGateway(profileId)
	return nil
}

func (a *App) BrowserProxyRoutingGet(profileId string) ProxyRoutingConfig {
	return a.proxyRoutingConfig(profileId)
}

func (a *App) BrowserProxyRoutingSave(profileId string, input ProxyRoutingConfig, force bool) (*ProxyGatewayStatus, error) {
	profile := a.profileForGatewayUpdate(profileId)
	if profile == nil {
		return nil, fmt.Errorf("profile not found")
	}
	if err := gateway.ValidateRoutingConfig(input); err != nil {
		return nil, err
	}
	previous := a.proxyRoutingConfig(profileId)
	normalized := gateway.NormalizeRoutingConfig(input)
	status := ProxyGatewayStatus{ProfileID: profileId, Mode: normalized.Mode, CurrentRouteID: profile.ProxyId}
	if profile.Running {
		client, err := a.ensureProxyGatewayClient()
		if err != nil {
			return nil, err
		}
		status, err = client.updateRouting(proxyGatewayProfileRequest{ProfileID: profileId, Routing: normalized, Force: force})
		if err != nil {
			return nil, fmt.Errorf("运行中分流规则应用失败，原规则保持不变：%w", err)
		}
	}
	if _, err := a.saveProxyRoutingConfig(profileId, normalized); err != nil {
		if profile.Running {
			client, clientErr := a.ensureProxyGatewayClient()
			if clientErr == nil {
				_, _ = client.updateRouting(proxyGatewayProfileRequest{ProfileID: profileId, Routing: previous})
			}
		}
		return nil, err
	}
	return &status, nil
}

func (a *App) BrowserProxyGatewayStatus(profileId string) (*ProxyGatewayStatus, error) {
	client, err := a.ensureProxyGatewayClient()
	if err != nil {
		return nil, err
	}
	status, err := client.status(strings.TrimSpace(profileId))
	if err != nil {
		return nil, err
	}
	return &status, nil
}

// BrowserProfileTrashList 获取回收站实例列表
func (a *App) BrowserProfileTrashList() []BrowserProfile { return a.browserMgr.ListDeleted() }

// BrowserProfileRestore 从回收站恢复实例
func (a *App) BrowserProfileRestore(profileId string) (*BrowserProfile, error) {
	return a.browserMgr.Restore(profileId)
}

// BrowserProfilePermanentlyDelete 从回收站彻底删除实例
func (a *App) BrowserProfilePermanentlyDelete(profileId string) error {
	if err := a.browserMgr.PermanentlyDelete(profileId); err != nil {
		return err
	}
	a.deleteProxyRoutingConfig(profileId)
	return nil
}

// BrowserProfileTrashCleanup 清理超过保留期的回收站实例
func (a *App) BrowserProfileTrashCleanup() error { return a.browserMgr.CleanupExpiredTrash() }

// BrowserProfileCopy 复制实例配置（除指纹参数外全部复制）
func (a *App) BrowserProfileCopy(profileId string, newName string) (*BrowserProfile, error) {
	return a.browserMgr.Copy(profileId, newName)
}

// BrowserProfileCopyWithMode 按模式复制实例配置。
func (a *App) BrowserProfileCopyWithMode(profileId string, newName string, mode string) (*BrowserProfile, error) {
	return a.browserMgr.CopyWithMode(profileId, newName, mode)
}

// BrowserProfileCopyWithOptions 按结构化选项复制实例配置。
func (a *App) BrowserProfileCopyWithOptions(profileId string, newName string, options BrowserProfileCopyOptions) (*BrowserProfile, error) {
	return a.browserMgr.CopyWithOptions(profileId, newName, options)
}

// migrateToSQLite 一次性迁移：若 SQLite 表为空则从旧文件导入数据，或初始化默认数据
// 迁移顺序：cores → proxies → profiles → bookmarks
func (a *App) migrateToSQLite() {
	log := logger.New("Migration")

	if cores, err := a.browserMgr.CoreDAO.List(); err == nil && len(cores) == 0 {
		if len(a.config.Browser.Cores) > 0 {
			for _, c := range a.config.Browser.Cores {
				if err := a.browserMgr.CoreDAO.Upsert(c); err != nil {
					log.Error("内核迁移失败", logger.F("core_id", c.CoreId), logger.F("error", err))
				}
			}
			log.Info("内核数据已迁移", logger.F("count", len(a.config.Browser.Cores)))
		} else {
			log.Info("内核表为空，将通过自动检测初始化")
		}
	}

	if proxies, err := a.browserMgr.ProxyDAO.List(); err == nil && len(proxies) == 0 {
		var srcProxies []browser.Proxy
		if loaded, err := config.LoadProxies(a.resolveAppPath("proxies.yaml")); err == nil && len(loaded) > 0 {
			srcProxies = loaded
		} else if len(a.config.Browser.Proxies) > 0 {
			srcProxies = a.config.Browser.Proxies
		} else {
			srcProxies = []browser.Proxy{
				{ProxyId: "__direct__", ProxyName: "直连（不走代理）", ProxyConfig: "direct://"},
			}
			log.Info("代理表为空，初始化默认代理")
		}
		for _, p := range srcProxies {
			if err := a.browserMgr.ProxyDAO.Upsert(p); err != nil {
				log.Error("代理迁移失败", logger.F("proxy_id", p.ProxyId), logger.F("error", err))
			}
		}
		if len(srcProxies) > 0 {
			log.Info("代理数据已初始化", logger.F("count", len(srcProxies)))
		}
	}

	if profiles, err := a.browserMgr.ProfileDAO.List(); err == nil && len(profiles) == 0 {
		if len(a.config.Browser.Profiles) > 0 {
			for _, pc := range a.config.Browser.Profiles {
				coreId := strings.TrimSpace(pc.CoreId)
				if strings.EqualFold(coreId, "default") {
					coreId = ""
				}
				p := &browser.Profile{
					ProfileId:          pc.ProfileId,
					ProfileName:        pc.ProfileName,
					UserDataDir:        pc.UserDataDir,
					CoreId:             coreId,
					FingerprintArgs:    pc.FingerprintArgs,
					ProxyId:            pc.ProxyId,
					ProxyConfig:        pc.ProxyConfig,
					ProxyBindSourceID:  pc.ProxyBindSourceID,
					ProxyBindSourceURL: pc.ProxyBindSourceURL,
					ProxyBindName:      pc.ProxyBindName,
					ProxyBindUpdatedAt: pc.ProxyBindUpdatedAt,
					LaunchArgs:         pc.LaunchArgs,
					Tags:               pc.Tags,
					Keywords:           pc.Keywords,
					CreatedAt:          pc.CreatedAt,
					UpdatedAt:          pc.UpdatedAt,
				}
				if err := a.browserMgr.ProfileDAO.Upsert(p); err != nil {
					log.Error("实例迁移失败", logger.F("profile_id", pc.ProfileId), logger.F("error", err))
				}
			}
			log.Info("实例数据已迁移", logger.F("count", len(a.config.Browser.Profiles)))
		} else {
			log.Info("实例表为空，自动创建默认实例")
			defaultProfile := &browser.Profile{
				ProfileId:       generateUUID(),
				ProfileName:     "默认实例",
				UserDataDir:     "default",
				CoreId:          "",
				FingerprintArgs: a.config.Browser.DefaultFingerprintArgs,
				LaunchArgs:      a.config.Browser.DefaultLaunchArgs,
				Tags:            []string{"默认"},
				ProxyId:         "__direct__",
				ProxyConfig:     "direct://",
				CreatedAt:       time.Now().Format(time.RFC3339),
				UpdatedAt:       time.Now().Format(time.RFC3339),
			}
			if err := a.browserMgr.ProfileDAO.Upsert(defaultProfile); err != nil {
				log.Error("自动创建默认实例失败", logger.F("error", err))
			}
		}
	}

	if bookmarks, err := a.browserMgr.BookmarkDAO.List(); err == nil && len(bookmarks) == 0 {
		src := a.config.Browser.DefaultBookmarks
		if len(src) == 0 {
			src = []config.BrowserBookmark{
				{Name: "Google", URL: "https://www.google.com/"},
				{Name: "Gmail", URL: "https://mail.google.com/"},
				{Name: "Claude", URL: "https://claude.ai/"},
				{Name: "ChatGPT", URL: "https://chatgpt.com/"},
				{Name: "YouTube", URL: "https://www.youtube.com/"},
			}
		}
		if err := a.browserMgr.BookmarkDAO.ReplaceAll(src); err != nil {
			log.Error("书签迁移失败", logger.F("error", err))
		} else {
			log.Info("书签数据已迁移", logger.F("count", len(src)))
		}
	}
}
