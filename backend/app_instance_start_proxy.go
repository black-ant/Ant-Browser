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
	connectorType := config.BrowserConnectorXray
	if a.config != nil {
		connectorType = config.NormalizeBrowserConnectorType(a.config.Browser.DefaultConnectorType)
	}

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
		temporaryProxyID := input.TemporaryProxyID
		temporaryProxyConfig := input.TemporaryProxyConfig
		// 一次性代理也可以是虚拟分组（启动码/API 传参场景），按策略解析到成员。
		if strings.TrimSpace(temporaryProxyConfig) == "" {
			if memberID, memberConfig, picked := a.resolveVirtualGroupProxyTarget(strings.TrimSpace(temporaryProxyID), proxies); picked {
				temporaryProxyID = memberID
				temporaryProxyConfig = memberConfig
			}
		}
		resolvedProxyID, resolvedProxyConfig, err = resolveTemporaryBrowserStartProxy(temporaryProxyID, temporaryProxyConfig, proxies)
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
		if !matched && !browser.IsDynamicProxyMode(profile.ProxyMode) {
			// 固定绑定虚拟分组：按顺序尝试成员，任一桥接成功即启动。
			// 这样首选成员失效时不会导致整个实例启动失败。
			if group, ok := a.findProxyVirtualGroupByID(resolvedProxyID); ok {
				return a.startWithVirtualGroupMembers(group, profile, proxies, connectorType)
			}
		}
	}
	if !usingTemporaryProxy && browser.IsDynamicProxyMode(profile.ProxyMode) {
		return a.resolveDynamicBrowserStartProxy(profileID, profile, proxies)
	}

	// 防御：代理 ID 与配置快照同时为空时禁止静默直连（Native 内核会把空配置当成直连）。
	if strings.TrimSpace(resolvedProxyID) == "" && strings.TrimSpace(resolvedProxyConfig) == "" {
		startErr := fmt.Errorf("实例启动失败：实例未配置代理。请编辑实例并显式选择代理（直连也需要显式选择）后再启动")
		profile.LastError = startErr.Error()
		log.Error("实例未配置代理，拒绝启动",
			logger.F("profile_id", profileID),
			logger.F("reason", startErr.Error()),
		)
		return "", profileProxyBridgeRef{}, false, startErr
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
	proxyURL, bridgeRef, releaseBridge, err := a.acquireBrowserProxyBridge(profileID, profile.ProfileName, resolvedProxyID, resolvedProxyConfig, proxies, connectorType)
	if err != nil {
		profile.LastError = err.Error()
	}
	return proxyURL, bridgeRef, releaseBridge, err
}

// startWithVirtualGroupMembers 按候选顺序逐个尝试虚拟分组成员，任一桥接成功即启动。
// 全组失败时才返回错误，并在错误里逐条列出成员失败原因。
func (a *App) startWithVirtualGroupMembers(group browser.ProxyVirtualGroup, profile *BrowserProfile, proxies []BrowserProxy, connectorType string) (string, profileProxyBridgeRef, bool, error) {
	log := logger.New("Browser")
	profileID := strings.TrimSpace(profile.ProfileId)
	candidates := a.pickVirtualGroupStartProxies(group, proxies, profileID)
	if candidates.empty() {
		var startErr error
		if candidates.allDegraded {
			startErr = fmt.Errorf("实例启动失败：虚拟分组「%s」的成员全部处于降级状态，已拒绝启动。请先恢复至少一个节点，或在该分组执行「立即巡检」/「清除降级」后重试", group.GroupName)
		} else {
			startErr = fmt.Errorf("实例启动失败：虚拟分组「%s」没有可用成员代理，请检查分组配置", group.GroupName)
		}
		profile.LastError = startErr.Error()
		logger.New("Browser").Error("虚拟分组无法启动实例",
			logger.F("profile_id", profileID),
			logger.F("virtual_group", group.GroupName),
			logger.F("all_degraded", candidates.allDegraded),
			logger.F("reason", startErr.Error()),
		)
		return "", profileProxyBridgeRef{}, false, startErr
	}

	failures := make([]string, 0, len(candidates.ordered))
	for _, member := range candidates.ordered {
		proxyURL, bridgeRef, releaseBridge, err := a.acquireBrowserProxyBridge(profileID, profile.ProfileName, member.ProxyId, member.ProxyConfig, proxies, connectorType)
		if err == nil {
			a.commitVirtualGroupStartSelection(candidates, member.ProxyId)
			log.Info("虚拟分组代理已选择成员",
				logger.F("profile_id", profileID),
				logger.F("virtual_group", candidates.group.GroupName),
				logger.F("member_proxy_id", member.ProxyId),
			)
			return proxyURL, bridgeRef, releaseBridge, nil
		}
		label := strings.TrimSpace(member.ProxyName)
		if label == "" {
			label = strings.TrimSpace(member.ProxyId)
		}
		failures = append(failures, fmt.Sprintf("%s：%s", label, err.Error()))
		log.Warn("虚拟分组成员不可用，继续尝试下一个",
			logger.F("profile_id", profileID),
			logger.F("virtual_group", candidates.group.GroupName),
			logger.F("member_proxy_id", member.ProxyId),
			logger.F("error", err.Error()),
		)
	}

	const maxFailureDetails = 5
	if len(failures) > maxFailureDetails {
		failures = append(failures[:maxFailureDetails], fmt.Sprintf("其余 %d 个成员未展开", len(failures)-maxFailureDetails))
	}
	startErr := fmt.Errorf("实例启动失败：虚拟分组「%s」的成员均不可用（%s）。请重新测速或检查代理内核配置。", candidates.group.GroupName, strings.Join(failures, "；"))
	profile.LastError = startErr.Error()
	log.Error("虚拟分组成员全部不可用",
		logger.F("profile_id", profileID),
		logger.F("virtual_group", candidates.group.GroupName),
		logger.F("reason", startErr.Error()),
	)
	return "", profileProxyBridgeRef{}, false, startErr
}

// acquireBrowserProxyBridge 校验代理配置、选择内核并完成桥接。
// 返回浏览器代理地址、桥接引用、是否需要释放桥接、错误。
func (a *App) acquireBrowserProxyBridge(profileID string, profileName string, proxyID string, proxyConfig string, proxies []BrowserProxy, connectorType string) (string, profileProxyBridgeRef, bool, error) {
	log := logger.New("Browser")
	proxyID = strings.TrimSpace(proxyID)
	proxyConfig = strings.TrimSpace(proxyConfig)
	connectorType = config.NormalizeBrowserConnectorType(connectorType)

	if supported, errorMsg := proxy.ValidateProxyConfigWithConnector(proxyConfig, proxies, proxyID, connectorType); !supported {
		startErr := fmt.Errorf("实例启动失败：%s", errorMsg)
		log.Error("代理配置无效",
			logger.F("profile_id", profileID),
			logger.F("proxy_id", proxyID),
			logger.F("error", errorMsg),
			logger.F("reason", startErr.Error()),
		)
		return "", profileProxyBridgeRef{}, false, startErr
	}

	resolution, err := proxy.ResolveProxyKernelForConnector(proxyConfig, proxies, proxyID, connectorType)
	if err != nil {
		startErr := fmt.Errorf("实例启动失败：%s", err.Error())
		log.Error("代理内核选择失败",
			logger.F("profile_id", profileID),
			logger.F("proxy_id", proxyID),
			logger.F("error", err.Error()),
			logger.F("reason", startErr.Error()),
		)
		return "", profileProxyBridgeRef{}, false, startErr
	}
	log.Info("实际代理内核", logger.F("profile_id", profileID), logger.F("engine", resolution.Kernel), logger.F("protocol", resolution.Protocol), logger.F("proxy_id", proxyID), logger.F("reason", resolution.Reason))

	switch resolution.Kernel {
	case proxy.ProxyKernelMieru:
		if a.mieruMgr == nil {
			return "", profileProxyBridgeRef{}, false, fmt.Errorf("实例启动失败：mieru 管理器未初始化，无法启动该协议代理。请先下载 mieru 内核。")
		}
		socksURL, bridgeKey, bridgeErr := a.mieruMgr.AcquireBridge(proxyConfig, proxies, proxyID)
		if bridgeErr != nil {
			startErr := fmt.Errorf("实例启动失败：mieru 代理桥接失败：%v", bridgeErr)
			log.Error("代理桥接失败(mieru)", logger.F("error", bridgeErr.Error()), logger.F("reason", startErr.Error()))
			a.reportProxyBridgeFailure(profileID, profileName, proxyID, profileProxyBridgeEngineMieru, bridgeErr)
			return "", profileProxyBridgeRef{}, false, startErr
		}
		return socksURL, newProfileProxyBridgeRef(profileProxyBridgeEngineMieru, bridgeKey, proxyID), bridgeKey != "", nil
	case proxy.ProxyKernelMihomo:
		if a.clashMgr == nil {
			return "", profileProxyBridgeRef{}, false, fmt.Errorf("实例启动失败：mihomo 管理器未初始化，无法启动该协议代理。请先下载 Mihomo 内核。")
		}
		proxyURL, bridgeKey, bridgeErr := a.clashMgr.AcquireNodeBridge(proxyConfig, proxies, proxyID)
		if bridgeErr != nil {
			startErr := fmt.Errorf("实例启动失败：mihomo 代理桥接失败：%v", bridgeErr)
			log.Error("代理桥接失败(mihomo)", logger.F("error", bridgeErr.Error()), logger.F("reason", startErr.Error()))
			a.reportProxyBridgeFailure(profileID, profileName, proxyID, profileProxyBridgeEngineMihomo, bridgeErr)
			return "", profileProxyBridgeRef{}, false, startErr
		}
		return proxyURL, newProfileProxyBridgeRef(profileProxyBridgeEngineMihomo, bridgeKey, proxyID), bridgeKey != "", nil
	case proxy.ProxyKernelSingBox:
		if a.singboxMgr == nil {
			return "", profileProxyBridgeRef{}, false, fmt.Errorf("实例启动失败：sing-box 管理器未初始化，无法启动该协议代理。请检查 sing-box 内核配置。")
		}
		socksURL, bridgeKey, bridgeErr := a.singboxMgr.AcquireBridge(proxyConfig, proxies, proxyID)
		if bridgeErr != nil {
			startErr := fmt.Errorf("实例启动失败：sing-box 代理桥接失败：%v", bridgeErr)
			log.Error("代理桥接失败(sing-box)", logger.F("error", bridgeErr.Error()), logger.F("reason", startErr.Error()))
			a.reportProxyBridgeFailure(profileID, profileName, proxyID, profileProxyBridgeEngineSingBox, bridgeErr)
			return "", profileProxyBridgeRef{}, false, startErr
		}
		return socksURL, newProfileProxyBridgeRef(profileProxyBridgeEngineSingBox, bridgeKey, proxyID), bridgeKey != "", nil
	case proxy.ProxyKernelXray:
		if a.xrayMgr == nil {
			return "", profileProxyBridgeRef{}, false, fmt.Errorf("实例启动失败：xray 管理器未初始化，无法启动该协议代理。")
		}
		socksURL, bridgeKey, bridgeErr := a.xrayMgr.AcquireBridge(proxyConfig, proxies, proxyID)
		if bridgeErr != nil {
			startErr := fmt.Errorf("实例启动失败：Xray 代理桥接失败：%v", bridgeErr)
			log.Error("代理桥接失败(xray)", logger.F("error", bridgeErr.Error()), logger.F("reason", startErr.Error()))
			a.reportProxyBridgeFailure(profileID, profileName, proxyID, profileProxyBridgeEngineXray, bridgeErr)
			return "", profileProxyBridgeRef{}, false, startErr
		}
		return socksURL, newProfileProxyBridgeRef(profileProxyBridgeEngineXray, bridgeKey, proxyID), bridgeKey != "", nil
	case proxy.ProxyKernelNative:
		return proxyConfig, profileProxyBridgeRef{}, false, nil
	default:
		return "", profileProxyBridgeRef{}, false, fmt.Errorf("实例启动失败：无法为协议 %s 选择代理内核", resolution.Protocol)
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
	// 一次性代理必须能解析出代理池节点：丢弃 ID 改用配置属于静默改写启动参数。
	return "", "", fmt.Errorf("代理ID不存在（proxy id not found: %s），请重新选择代理后再启动", proxyID)
}
