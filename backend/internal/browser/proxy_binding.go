package browser

import (
	"strings"
	"time"
)

func normalizeProxyBindValue(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}

func (m *Manager) listProxyCatalog() []Proxy {
	if m.ProxyDAO != nil {
		if list, err := m.ProxyDAO.List(); err == nil && len(list) > 0 {
			return append([]Proxy{}, list...)
		}
	}
	return append([]Proxy{}, m.Config.Browser.Proxies...)
}

func findProxyByID(list []Proxy, proxyID string) (Proxy, bool) {
	target := normalizeProxyBindValue(proxyID)
	if target == "" {
		return Proxy{}, false
	}
	for _, item := range list {
		if normalizeProxyBindValue(item.ProxyId) == target {
			return item, true
		}
	}
	return Proxy{}, false
}

func uniqueProxyMatch(list []Proxy, match func(Proxy) bool) (Proxy, bool) {
	var hit Proxy
	matched := 0
	for _, item := range list {
		if !match(item) {
			continue
		}
		hit = item
		matched++
		if matched > 1 {
			return Proxy{}, false
		}
	}
	return hit, matched == 1
}

// GetProxyByID 根据代理 ID 获取代理对象（优先 DAO）。
func (m *Manager) GetProxyByID(proxyID string) (Proxy, bool) {
	return findProxyByID(m.listProxyCatalog(), proxyID)
}

// BindProfileToProxy 将窗口绑定到指定代理并同步绑定快照。
// syncProxyConfig=true 时会同步更新 profile.ProxyConfig。
func BindProfileToProxy(profile *Profile, proxy Proxy, syncProxyConfig bool) bool {
	if profile == nil {
		return false
	}

	changed := false
	if profile.ProxyId != strings.TrimSpace(proxy.ProxyId) {
		profile.ProxyId = strings.TrimSpace(proxy.ProxyId)
		changed = true
	}
	if syncProxyConfig {
		proxyConfig := strings.TrimSpace(proxy.ProxyConfig)
		if proxyConfig != "" && profile.ProxyConfig != proxyConfig {
			profile.ProxyConfig = proxyConfig
			changed = true
		}
	}

	sourceID := strings.TrimSpace(proxy.SourceID)
	sourceURL := strings.TrimSpace(proxy.SourceURL)
	proxyName := strings.TrimSpace(proxy.ProxyName)
	if profile.ProxyBindSourceID != sourceID {
		profile.ProxyBindSourceID = sourceID
		changed = true
	}
	if profile.ProxyBindSourceURL != sourceURL {
		profile.ProxyBindSourceURL = sourceURL
		changed = true
	}
	if profile.ProxyBindName != proxyName {
		profile.ProxyBindName = proxyName
		changed = true
	}
	if changed {
		profile.ProxyBindUpdatedAt = time.Now().Format(time.RFC3339)
	}
	return changed
}

// ClearProfileProxyBinding 清空窗口的代理绑定快照。
func ClearProfileProxyBinding(profile *Profile) bool {
	if profile == nil {
		return false
	}
	changed := false
	if profile.ProxyBindSourceID != "" {
		profile.ProxyBindSourceID = ""
		changed = true
	}
	if profile.ProxyBindSourceURL != "" {
		profile.ProxyBindSourceURL = ""
		changed = true
	}
	if profile.ProxyBindName != "" {
		profile.ProxyBindName = ""
		changed = true
	}
	if changed {
		profile.ProxyBindUpdatedAt = time.Now().Format(time.RFC3339)
	}
	return changed
}

// ResolveProfileProxyBinding 尝试修复窗口代理绑定。
// 返回值: changed 是否修改窗口, boundInPool 是否在代理池成功定位, mode 重关联命中模式
func (m *Manager) ResolveProfileProxyBinding(profile *Profile) (bool, bool, string) {
	if profile == nil {
		return false, false, ""
	}
	proxies := m.listProxyCatalog()
	if len(proxies) == 0 {
		return false, false, ""
	}

	if proxy, ok := findProxyByID(proxies, profile.ProxyId); ok {
		changed := BindProfileToProxy(profile, proxy, true)
		return changed, true, "proxy_id"
	}

	allowConfigFallback := strings.TrimSpace(profile.ProxyId) != "" ||
		strings.TrimSpace(profile.ProxyBindSourceID) != "" ||
		strings.TrimSpace(profile.ProxyBindSourceURL) != "" ||
		strings.TrimSpace(profile.ProxyBindName) != ""

	if proxy, ok, mode := matchProxyBySnapshot(profile, proxies, allowConfigFallback); ok {
		changed := BindProfileToProxy(profile, proxy, true)
		return changed, true, mode
	}

	return false, false, ""
}

func matchProxyBySnapshot(profile *Profile, proxies []Proxy, allowConfigFallback bool) (Proxy, bool, string) {
	nameKey := normalizeProxyBindValue(profile.ProxyBindName)
	sourceIDKey := normalizeProxyBindValue(profile.ProxyBindSourceID)
	sourceURLKey := normalizeProxyBindValue(profile.ProxyBindSourceURL)
	cfgKey := normalizeProxyBindValue(profile.ProxyConfig)

	if sourceIDKey != "" && nameKey != "" {
		if hit, ok := uniqueProxyMatch(proxies, func(item Proxy) bool {
			return normalizeProxyBindValue(item.SourceID) == sourceIDKey &&
				normalizeProxyBindValue(item.ProxyName) == nameKey
		}); ok {
			return hit, true, "source_id+name"
		}
	}

	if sourceURLKey != "" && nameKey != "" {
		if hit, ok := uniqueProxyMatch(proxies, func(item Proxy) bool {
			return normalizeProxyBindValue(item.SourceURL) == sourceURLKey &&
				normalizeProxyBindValue(item.ProxyName) == nameKey
		}); ok {
			return hit, true, "source_url+name"
		}
	}

	if nameKey != "" {
		if hit, ok := uniqueProxyMatch(proxies, func(item Proxy) bool {
			return normalizeProxyBindValue(item.ProxyName) == nameKey
		}); ok {
			return hit, true, "name"
		}
	}

	if sourceIDKey != "" && cfgKey != "" {
		if hit, ok := uniqueProxyMatch(proxies, func(item Proxy) bool {
			return normalizeProxyBindValue(item.SourceID) == sourceIDKey &&
				normalizeProxyBindValue(item.ProxyConfig) == cfgKey
		}); ok {
			return hit, true, "source_id+config"
		}
	}

	if sourceURLKey != "" && cfgKey != "" {
		if hit, ok := uniqueProxyMatch(proxies, func(item Proxy) bool {
			return normalizeProxyBindValue(item.SourceURL) == sourceURLKey &&
				normalizeProxyBindValue(item.ProxyConfig) == cfgKey
		}); ok {
			return hit, true, "source_url+config"
		}
	}

	if allowConfigFallback && cfgKey != "" {
		if hit, ok := uniqueProxyMatch(proxies, func(item Proxy) bool {
			return normalizeProxyBindValue(item.ProxyConfig) == cfgKey
		}); ok {
			return hit, true, "config"
		}
	}

	return Proxy{}, false, ""
}
