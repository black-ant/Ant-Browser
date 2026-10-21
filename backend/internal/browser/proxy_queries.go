package browser

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
	if proxyDAO != nil {
		if list, err := proxyDAO.List(); err == nil {
			return ensureBuiltinDirectProxy(list)
		}
		return ensureBuiltinDirectProxy(nil)
	}
	return ensureBuiltinDirectProxy(append([]Proxy{}, fallback...))
}

func ListProxyGroups(proxyDAO ProxyDAO) []string {
	if proxyDAO != nil {
		if groups, err := proxyDAO.ListGroups(); err == nil {
			return groups
		}
	}
	return nil
}

func ListProxiesByGroupWithFallback(proxyDAO ProxyDAO, groupName string, fallback []Proxy) []Proxy {
	if proxyDAO != nil {
		if list, err := proxyDAO.ListByGroup(groupName); err == nil {
			return list
		}
		return nil
	}

	var result []Proxy
	for _, item := range fallback {
		if item.GroupName == groupName {
			result = append(result, item)
		}
	}
	return result
}

func LatestProxiesWithFallback(proxyDAO ProxyDAO, fallback []Proxy) []Proxy {
	if proxyDAO != nil {
		if list, err := proxyDAO.List(); err == nil {
			return ensureBuiltinDirectProxy(list)
		}
		return ensureBuiltinDirectProxy(nil)
	}
	return ensureBuiltinDirectProxy(fallback)
}
