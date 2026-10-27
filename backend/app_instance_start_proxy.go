package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/logger"
	"ant-chrome/backend/internal/proxy"
	"fmt"
	"strings"
)

const temporaryDirectProxyID = "__direct__"

func (a *App) resolveBrowserStartProxy(input browserStartInput, profile *BrowserProfile) (string, profileProxyBridgeRef, bool, error) {
	log := logger.New("Browser")
	proxies := a.getLatestProxies()
	profileID := input.ProfileID

	if input.ForceDirectProxy {
		log.Warn("按请求直连启动实例",
			logger.F("profile_id", profileID),
			logger.F("proxy_id", profile.ProxyId),
		)
		return "direct://", profileProxyBridgeRef{}, false, nil
	}

	resolvedProxyID := strings.TrimSpace(profile.ProxyId)
	resolvedProxyConfig := strings.TrimSpace(profile.ProxyConfig)
	usingTemporaryProxy := input.hasTemporaryProxy()
	if usingTemporaryProxy {
		var err error
		resolvedProxyID, resolvedProxyConfig, err = resolveTemporaryBrowserStartProxy(input.TemporaryProxyID, input.TemporaryProxyConfig, proxies)
		if err != nil {
			startErr := fmt.Errorf("实例启动失败：%s", err.Error())
			profile.LastError = startErr.Error()
			log.Error("一次性代理配置无效",
				logger.F("profile_id", profileID),
				logger.F("temporary_proxy_id", input.TemporaryProxyID),
				logger.F("error", err.Error()),
				logger.F("reason", startErr.Error()),
			)
			return "", profileProxyBridgeRef{}, false, startErr
		}
	} else if resolvedProxyID != "" {
		matched := false
		for _, item := range proxies {
			if strings.EqualFold(item.ProxyId, resolvedProxyID) {
				resolvedProxyID = strings.TrimSpace(item.ProxyId)
				resolvedProxyConfig = strings.TrimSpace(item.ProxyConfig)
				matched = true
				break
			}
		}
		if !matched {
			// 固定绑定虚拟分组：按策略挑选本次启动的成员代理
			if group, ok := a.findProxyVirtualGroupByID(resolvedProxyID); ok {
				if memberID, memberConfig, picked := a.pickVirtualGroupStartProxy(group, proxies, profileID); picked {
					log.Info("虚拟分组代理已选择成员",
						logger.F("profile_id", profileID),
						logger.F("virtual_group", group.GroupName),
						logger.F("member_proxy_id", memberID),
					)
					resolvedProxyID = memberID
					resolvedProxyConfig = memberConfig
				}
			}
		}
	}
	if !usingTemporaryProxy && browser.IsDynamicProxyMode(profile.ProxyMode) {
		return a.resolveDynamicBrowserStartProxy(profileID, profile, proxies)
	}

	log.Info("代理配置检查",
		logger.F("profile_id", profileID),
		logger.F("proxy_id", profile.ProxyId),
		logger.F("profile_proxy_config", profile.ProxyConfig),
		logger.F("temporary_proxy", usingTemporaryProxy),
		logger.F("temporary_proxy_id", input.TemporaryProxyID),
		logger.F("temporary_proxy_config", input.TemporaryProxyConfig),
		logger.F("resolved_proxy_config", resolvedProxyConfig),
	)
	connectorType := config.BrowserConnectorXray
	if a.config != nil {
		connectorType = config.NormalizeBrowserConnectorType(a.config.Browser.DefaultConnectorType)
	}
	if supported, errorMsg := proxy.ValidateProxyConfigWithConnector(resolvedProxyConfig, proxies, resolvedProxyID, connectorType); !supported {
		startErr := fmt.Errorf("实例启动失败：%s", errorMsg)
		profile.LastError = startErr.Error()
		log.Error("代理配置无效",
			logger.F("profile_id", profileID),
			logger.F("proxy_id", resolvedProxyID),
			logger.F("error", errorMsg),
			logger.F("reason", startErr.Error()),
		)
		return "", profileProxyBridgeRef{}, false, startErr
	}

	resolution, err := proxy.ResolveProxyKernelForConnector(resolvedProxyConfig, proxies, resolvedProxyID, connectorType)
	if err != nil {
		startErr := fmt.Errorf("实例启动失败：%s", err.Error())
		profile.LastError = startErr.Error()
		log.Error("代理内核选择失败",
			logger.F("profile_id", profileID),
			logger.F("proxy_id", resolvedProxyID),
			logger.F("error", err.Error()),
			logger.F("reason", startErr.Error()),
		)
		return "", profileProxyBridgeRef{}, false, startErr
	}
	log.Info("实际代理内核", logger.F("profile_id", profileID), logger.F("engine", resolution.Kernel), logger.F("protocol", resolution.Protocol), logger.F("proxy_id", resolvedProxyID), logger.F("reason", resolution.Reason))

	switch resolution.Kernel {
	case proxy.ProxyKernelMieru:
		if a.mieruMgr == nil {
			startErr := fmt.Errorf("实例启动失败：mieru 管理器未初始化，无法启动该协议代理。请先下载 mieru 内核。")
			profile.LastError = startErr.Error()
			return "", profileProxyBridgeRef{}, false, startErr
		}
		socksURL, bridgeKey, bridgeErr := a.mieruMgr.AcquireBridge(resolvedProxyConfig, proxies, resolvedProxyID)
		if bridgeErr != nil {
			startErr := fmt.Errorf("实例启动失败：mieru 代理桥接失败：%v", bridgeErr)
			log.Error("代理桥接失败(mieru)", logger.F("error", bridgeErr.Error()), logger.F("reason", startErr.Error()))
			profile.LastError = startErr.Error()
			a.reportProxyBridgeFailure(profileID, profile.ProfileName, resolvedProxyID, profileProxyBridgeEngineMieru, bridgeErr)
			return "", profileProxyBridgeRef{}, false, startErr
		}
		return socksURL, newProfileProxyBridgeRef(profileProxyBridgeEngineMieru, bridgeKey, resolvedProxyID), bridgeKey != "", nil
	case proxy.ProxyKernelMihomo:
		if a.clashMgr == nil {
			startErr := fmt.Errorf("实例启动失败：mihomo 管理器未初始化，无法启动该协议代理。请先下载 Mihomo 内核。")
			profile.LastError = startErr.Error()
			return "", profileProxyBridgeRef{}, false, startErr
		}
		proxyURL, bridgeKey, bridgeErr := a.clashMgr.AcquireNodeBridge(resolvedProxyConfig, proxies, resolvedProxyID)
		if bridgeErr != nil {
			startErr := fmt.Errorf("实例启动失败：mihomo 代理桥接失败：%v", bridgeErr)
			log.Error("代理桥接失败(mihomo)", logger.F("error", bridgeErr.Error()), logger.F("reason", startErr.Error()))
			profile.LastError = startErr.Error()
			a.reportProxyBridgeFailure(profileID, profile.ProfileName, resolvedProxyID, profileProxyBridgeEngineMihomo, bridgeErr)
			return "", profileProxyBridgeRef{}, false, startErr
		}
		return proxyURL, newProfileProxyBridgeRef(profileProxyBridgeEngineMihomo, bridgeKey, resolvedProxyID), bridgeKey != "", nil
	case proxy.ProxyKernelSingBox:
		if a.singboxMgr == nil {
			startErr := fmt.Errorf("实例启动失败：sing-box 管理器未初始化，无法启动该协议代理。请检查 sing-box 内核配置。")
			profile.LastError = startErr.Error()
			return "", profileProxyBridgeRef{}, false, startErr
		}
		socksURL, bridgeKey, bridgeErr := a.singboxMgr.AcquireBridge(resolvedProxyConfig, proxies, resolvedProxyID)
		if bridgeErr != nil {
			startErr := fmt.Errorf("实例启动失败：sing-box 代理桥接失败：%v", bridgeErr)
			log.Error("代理桥接失败(sing-box)", logger.F("error", bridgeErr.Error()), logger.F("reason", startErr.Error()))
			profile.LastError = startErr.Error()
			a.reportProxyBridgeFailure(profileID, profile.ProfileName, resolvedProxyID, profileProxyBridgeEngineSingBox, bridgeErr)
			return "", profileProxyBridgeRef{}, false, startErr
		}
		return socksURL, newProfileProxyBridgeRef(profileProxyBridgeEngineSingBox, bridgeKey, resolvedProxyID), bridgeKey != "", nil
	case proxy.ProxyKernelXray:
		if a.xrayMgr == nil {
			startErr := fmt.Errorf("实例启动失败：xray 管理器未初始化，无法启动该协议代理。")
			profile.LastError = startErr.Error()
			return "", profileProxyBridgeRef{}, false, startErr
		}
		socksURL, bridgeKey, bridgeErr := a.xrayMgr.AcquireBridge(resolvedProxyConfig, proxies, resolvedProxyID)
		if bridgeErr != nil {
			startErr := fmt.Errorf("实例启动失败：Xray 代理桥接失败：%v", bridgeErr)
			log.Error("代理桥接失败(xray)", logger.F("error", bridgeErr.Error()), logger.F("reason", startErr.Error()))
			profile.LastError = startErr.Error()
			a.reportProxyBridgeFailure(profileID, profile.ProfileName, resolvedProxyID, profileProxyBridgeEngineXray, bridgeErr)
			return "", profileProxyBridgeRef{}, false, startErr
		}
		return socksURL, newProfileProxyBridgeRef(profileProxyBridgeEngineXray, bridgeKey, resolvedProxyID), bridgeKey != "", nil
	case proxy.ProxyKernelNative:
		return resolvedProxyConfig, profileProxyBridgeRef{}, false, nil
	default:
		startErr := fmt.Errorf("实例启动失败：无法为协议 %s 选择代理内核", resolution.Protocol)
		profile.LastError = startErr.Error()
		return "", profileProxyBridgeRef{}, false, startErr
	}
}

func resolveTemporaryBrowserStartProxy(proxyID string, proxyConfig string, proxies []BrowserProxy) (string, string, error) {
	proxyID = strings.TrimSpace(proxyID)
	proxyConfig = strings.TrimSpace(proxyConfig)
	if proxyID == "" {
		return "", proxyConfig, nil
	}

	for _, item := range proxies {
		if strings.EqualFold(item.ProxyId, proxyID) {
			return strings.TrimSpace(item.ProxyId), strings.TrimSpace(item.ProxyConfig), nil
		}
	}
	if strings.EqualFold(proxyID, temporaryDirectProxyID) {
		return temporaryDirectProxyID, "direct://", nil
	}
	if proxyConfig != "" {
		return "", proxyConfig, nil
	}
	return "", "", fmt.Errorf("代理ID不存在（proxy id not found: %s），且未提供 proxyConfig", proxyID)
}
