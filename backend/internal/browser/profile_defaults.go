package browser

import (
	"ant-chrome/backend/internal/logger"
	"fmt"
	"strings"
)

const directProxyID = "__direct__"

// ApplyDefaults 应用默认配置。
// 返回 (是否有字段变更, 代理绑定提示, 错误)。代理绑定处于不可用状态时返回错误而不是静默修正：
// 任何自动回退直连都会暴露真实 IP 并覆盖用户选择。代理绑定提示用于把「按快照找回节点」
// 「改用配置快照」这类自动行为明确告知用户，而不是只写日志。
func (m *Manager) ApplyDefaults(profile *Profile) (bool, string, error) {
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
		return defaultsChanged || proxyChanged, "", nil
	}

	proxyNotice := ""
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
		proxyNotice = fmt.Sprintf("代理绑定已自动重新关联到节点「%s」（原节点 ID 已失效，按绑定快照找回，出口流量未变）", strings.TrimSpace(profile.ProxyBindName))
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
		} else if strings.TrimSpace(profile.ProxyConfig) == "" {
			// 代理 ID 与配置快照同时为空属于异常状态（导入时被清空、数据损坏等）。
			// 这里禁止自动绑定直连：直连会暴露真实 IP，且会把用户的失效绑定永久覆盖掉。
			return defaultsChanged || proxyChanged, proxyNotice, fmt.Errorf(
				"实例「%s」未配置代理，也没有可用的代理配置快照。请编辑实例并显式选择代理（直连也需要显式选择）后再启动",
				profile.ProfileName,
			)
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
			proxyNotice = fmt.Sprintf("实例绑定的代理节点已失效（proxyId=%s），本次启动改用实例内的代理配置快照，出口节点未变；请尽快重新选择代理", missingProxyID)
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

	return defaultsChanged || proxyChanged, proxyNotice, nil
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

// bindProfileToDirectProxy 仅在显式选择直连时使用。
// 拿不到内置直连代理时返回 false 并保留原绑定：清空绑定会让实例在启动时解析成
// Native 内核直连，属于最隐蔽的静默直连路径。
func (m *Manager) bindProfileToDirectProxy(profile *Profile) bool {
	if profile == nil {
		return false
	}
	proxy, ok := m.GetProxyByID(directProxyID)
	if !ok {
		return false
	}
	return BindProfileToProxy(profile, proxy, true)
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
