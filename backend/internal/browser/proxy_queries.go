package browser

import (
	"ant-chrome/backend/internal/logger"
	"fmt"
	"sort"
	"strings"
)

func ensureBuiltinDirectProxy(list []Proxy) []Proxy {
	for _, item := range list {
		if item.ProxyId == directProxyID {
			return list
		}
	}

	builtin := Proxy{
		ProxyId:     directProxyID,
		ProxyName:   `直连（不走代理）`,
		ProxyConfig: `direct://`,
	}
	return append([]Proxy{builtin}, list...)
}

func ListProxiesWithFallback(proxyDAO ProxyDAO, fallback []Proxy) []Proxy {
	list, err := LoadLatestProxiesWithFallback(proxyDAO, fallback)
	if err != nil {
		logger.New("BrowserProxy").Error("读取代理目录失败", logger.F("error", err))
	}
	return list
}

func LoadLatestProxiesWithFallback(proxyDAO ProxyDAO, fallback []Proxy) ([]Proxy, error) {
	if proxyDAO != nil {
		list, err := proxyDAO.List()
		if err != nil {
			return ensureBuiltinDirectProxy(nil), fmt.Errorf("读取代理目录失败: %w", err)
		}
		return ensureBuiltinDirectProxy(list), nil
	}
	return ensureBuiltinDirectProxy(append([]Proxy{}, fallback...)), nil
}

func ListProxyGroups(proxyDAO ProxyDAO) []string {
	groups, err := LoadProxyGroupsWithFallback(proxyDAO, nil)
	if err != nil {
		logger.New("BrowserProxy").Error("读取代理分组失败", logger.F("error", err))
	}
	return groups
}

func LoadProxyGroupsWithFallback(proxyDAO ProxyDAO, fallback []Proxy) ([]string, error) {
	if proxyDAO != nil {
		groups, err := proxyDAO.ListGroups()
		if err != nil {
			return nil, fmt.Errorf("读取代理分组失败: %w", err)
		}
		return groups, nil
	}
	return proxyGroupNames(fallback), nil
}

func ListProxiesByGroupWithFallback(proxyDAO ProxyDAO, groupName string, fallback []Proxy) []Proxy {
	list, err := LoadProxiesByGroupWithFallback(proxyDAO, groupName, fallback)
	if err != nil {
		logger.New("BrowserProxy").Error("按分组读取代理目录失败", logger.F("group", groupName), logger.F("error", err))
	}
	return list
}

func LoadProxiesByGroupWithFallback(proxyDAO ProxyDAO, groupName string, fallback []Proxy) ([]Proxy, error) {
	if proxyDAO != nil {
		list, err := proxyDAO.ListByGroup(groupName)
		if err != nil {
			return nil, fmt.Errorf("按分组读取代理目录失败: %w", err)
		}
		return list, nil
	}

	var result []Proxy
	for _, item := range fallback {
		if item.GroupName == groupName {
			result = append(result, item)
		}
	}
	return result, nil
}

func proxyGroupNames(proxies []Proxy) []string {
	seen := make(map[string]struct{})
	groups := make([]string, 0)
	for _, item := range proxies {
		name := strings.TrimSpace(item.GroupName)
		key := strings.ToLower(name)
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		groups = append(groups, name)
	}
	sort.SliceStable(groups, func(i, j int) bool {
		return strings.ToLower(groups[i]) < strings.ToLower(groups[j])
	})
	return groups
}

func LatestProxiesWithFallback(proxyDAO ProxyDAO, fallback []Proxy) []Proxy {
	list, err := LoadLatestProxiesWithFallback(proxyDAO, fallback)
	if err != nil {
		logger.New("BrowserProxy").Error("读取最新代理目录失败", logger.F("error", err))
	}
	return list
}
