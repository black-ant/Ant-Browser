package proxy

import (
	"strings"

	"ant-chrome/backend/internal/config"
)

func resolveProxyConfig(proxyConfig string, proxies []config.BrowserProxy, proxyId string) string {
	src := strings.TrimSpace(proxyConfig)
	if src != "" {
		return src
	}
	if proxyId == "" {
		return src
	}
	for _, item := range proxies {
		if strings.EqualFold(item.ProxyId, proxyId) {
			return strings.TrimSpace(item.ProxyConfig)
		}
	}
	return src
}
