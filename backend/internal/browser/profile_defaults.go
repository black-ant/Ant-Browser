package browser

import (
	"ant-chrome/backend/internal/logger"
	"strings"
)

const directProxyID = "__direct__"

// ApplyDefaults 应用默认配置
func (m *Manager) ApplyDefaults(profile *Profile) bool {
	log := logger.New("Browser")
	profile.CoreId = normalizeProfileCoreID(profile.CoreId)
	if profile.CoreId == "" {
		if defaultCore, ok := m.GetDefaultCore(); ok {
			profile.CoreId = defaultCore.CoreId
		}
	}
	if len(profile.FingerprintArgs) == 0 {
		// 全局 default_fingerprint_args 是按 fingerprint-chromium 写的，
		// 直接套到 Cloak 实例上会产生一批无效参数，所以按后端过滤。
		profile.FingerprintArgs = m.defaultFingerprintArgsForCore(profile.CoreId)
	}
	if profile.LaunchArgs == nil || len(profile.LaunchArgs) == 0 {
		profile.LaunchArgs = append([]string{}, m.Config.Browser.DefaultLaunchArgs...)
	}
	if strings.TrimSpace(profile.UserDataDir) == "" {
		profile.UserDataDir = profile.ProfileId
	}

	proxyChanged := false
	bindChanged, boundInPool, bindMode := m.ResolveProfileProxyBinding(profile)
	if bindChanged {
		proxyChanged = true
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
		} else if m.bindProfileToDirectProxy(profile) {
			proxyChanged = true
			log.Warn("实例代理未找到，已回退到直连",
				logger.F("profile_id", profile.ProfileId),
				logger.F("missing_proxy_id", missingProxyID),
			)
		}
	}

	return proxyChanged
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
