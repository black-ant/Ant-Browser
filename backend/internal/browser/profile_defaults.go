package browser

import (
	"ant-chrome/backend/internal/logger"
	"strings"
)

const directProxyID = "__direct__"

// ApplyDefaults 应用默认配置
func (m *Manager) ApplyDefaults(profile *Profile) bool {
	defaultsChanged := false
	log := logger.New("Browser")
	if profile.FingerprintArgs == nil || len(profile.FingerprintArgs) == 0 {
		if m.Config != nil {
			profile.FingerprintArgs = append([]string{}, m.Config.Browser.DefaultFingerprintArgs...)
			defaultsChanged = len(profile.FingerprintArgs) > 0
		}
	}
	upgradedFingerprintArgs := upgradeLegacyMinimalFingerprintArgs(profile.FingerprintArgs)
	if !stringSlicesEqual(profile.FingerprintArgs, upgradedFingerprintArgs) {
		profile.FingerprintArgs = upgradedFingerprintArgs
		defaultsChanged = true
	}
	if profile.LaunchArgs == nil || len(profile.LaunchArgs) == 0 {
		if m.Config != nil {
			profile.LaunchArgs = append([]string{}, m.Config.Browser.DefaultLaunchArgs...)
			defaultsChanged = defaultsChanged || len(profile.LaunchArgs) > 0
		}
	}
	if strings.TrimSpace(profile.UserDataDir) == "" {
		profile.UserDataDir = profile.ProfileId
	}
	profile.CoreId = normalizeProfileCoreID(profile.CoreId)
	if profile.CoreId == "" {
		if defaultCore, ok := m.GetDefaultCore(); ok {
			profile.CoreId = defaultCore.CoreId
		}
	}

	proxyMode := NormalizeProxyMode(profile.ProxyMode)
	if proxyMode == ProxyModeDynamic || proxyMode == ProxyModeLocal {
		proxyChanged := false
		if strings.TrimSpace(profile.ProxyId) != "" {
			profile.ProxyId = ""
			proxyChanged = true
		}
		if proxyMode == ProxyModeDynamic && strings.TrimSpace(profile.ProxyConfig) != "" {
			profile.ProxyConfig = ""
			proxyChanged = true
		}
		if ClearProfileProxyBinding(profile) {
			proxyChanged = true
		}
		if proxyMode == ProxyModeDynamic {
			profile.ProxyGroupName = NormalizeProxyGroupName(profile.ProxyGroupName)
		} else if profile.ProxyGroupName != "" {
			profile.ProxyGroupName = ""
			proxyChanged = true
		}
		return defaultsChanged || proxyChanged
	}

	proxyChanged := false
	bindChanged, boundInPool, bindMode := m.ResolveProfileProxyBinding(profile)
	if bindChanged {
		proxyChanged = true
	}
	// 虚拟分组绑定是合法形态：ProxyId 为分组 ID、ProxyConfig 特意保持为空，
	// 启动时按分组策略解析成员，绝不能在此时回退直连。
	if !boundInPool && strings.TrimSpace(profile.ProxyId) != "" {
		if _, isVirtualGroup := m.getProxyVirtualGroupByIDOrName(profile.ProxyId); isVirtualGroup {
			boundInPool = true
		}
	}
	if bindMode != "" && bindMode != "proxy_id" {
		log.Info("实例代理自动重关联",
			logger.F("profile_id", profile.ProfileId),
			logger.F("proxy_id", profile.ProxyId),
			logger.F("mode", bindMode),
		)
	}

	if strings.TrimSpace(profile.ProxyId) == "" {
		if proxy, ok := m.resolvePoolProxyByConfig(profile.ProxyConfig); ok {
			if BindProfileToProxy(profile, proxy, true) {
				proxyChanged = true
			}
			boundInPool = true
		} else if strings.TrimSpace(profile.ProxyConfig) == "" && m.bindProfileToDirectProxy(profile) {
			proxyChanged = true
			boundInPool = true
		}
	}

	if profile.ProxyId != "" && !boundInPool {
		missingProxyID := profile.ProxyId
		if strings.TrimSpace(profile.ProxyConfig) != "" {
			profile.ProxyId = ""
			proxyChanged = true
			if ClearProfileProxyBinding(profile) {
				proxyChanged = true
			}
			log.Warn("实例代理ID未找到，已改为使用实例代理配置",
				logger.F("profile_id", profile.ProfileId),
				logger.F("missing_proxy_id", missingProxyID),
			)
		} else {
			// 不允许自动改写用户绑定：失效 ID 保留原样（启动时代理解析会明确报错），
			// 自动回退直连会暴露真实 IP 且覆盖用户选择。
			log.Warn("实例代理未找到，保留原绑定（不会自动改为直连）",
				logger.F("profile_id", profile.ProfileId),
				logger.F("missing_proxy_id", missingProxyID),
			)
		}
	}

	return defaultsChanged || proxyChanged
}

func stringSlicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func (m *Manager) bindProfileToDirectProxy(profile *Profile) bool {
	if profile == nil {
		return false
	}
	if proxy, ok := m.GetProxyByID(directProxyID); ok {
		return BindProfileToProxy(profile, proxy, true)
	}

	changed := false
	if strings.TrimSpace(profile.ProxyId) != "" {
		profile.ProxyId = ""
		changed = true
	}
	if strings.TrimSpace(profile.ProxyConfig) != "" {
		profile.ProxyConfig = ""
		changed = true
	}
	if ClearProfileProxyBinding(profile) {
		changed = true
	}
	return changed
}

func (m *Manager) resolvePoolProxyByConfig(proxyConfig string) (Proxy, bool) {
	target := normalizeProxyBindValue(proxyConfig)
	if target == "" {
		return Proxy{}, false
	}
	proxies := m.listProxyCatalog()
	return uniqueProxyMatch(proxies, func(item Proxy) bool {
		return normalizeProxyBindValue(item.ProxyConfig) == target
	})
}

// copyKeywords 深拷贝 keywords map
func copyKeywords(src map[string]string) map[string]string {
	if src == nil {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
