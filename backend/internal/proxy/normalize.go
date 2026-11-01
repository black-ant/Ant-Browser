package proxy

import (
	"ant-chrome/backend/internal/config"
	"strings"
)

const defaultSourceRefreshIntervalM = 60
const maxSourceRefreshIntervalM = 24 * 60

func NormalizeBrowserProxies(proxies []config.BrowserProxy, generateID func() string) []config.BrowserProxy {
	normalized := make([]config.BrowserProxy, 0, len(proxies)+1)
	for i, item := range proxies {
		proxyName := strings.TrimSpace(item.ProxyName)
		proxyConfig := strings.TrimSpace(item.ProxyConfig)
		if proxyName == "" || proxyConfig == "" {
			continue
		}

		proxyID := strings.TrimSpace(item.ProxyId)
		if proxyID == "" && generateID != nil {
			proxyID = generateID()
		}

		sourceURL := strings.TrimSpace(item.SourceURL)
		sourceID := strings.TrimSpace(item.SourceID)
		sourceNamePrefix := strings.TrimSpace(item.SourceNamePrefix)
		sourceLastRefreshAt := strings.TrimSpace(item.SourceLastRefreshAt)
		sourceFetchProxyID := strings.TrimSpace(item.SourceFetchProxyID)
		sourceLastRefreshError := strings.TrimSpace(item.SourceLastRefreshError)
		sourceRefreshIntervalM := item.SourceRefreshIntervalM
		if sourceRefreshIntervalM < 0 {
			sourceRefreshIntervalM = 0
		}
		if sourceRefreshIntervalM > maxSourceRefreshIntervalM {
			sourceRefreshIntervalM = maxSourceRefreshIntervalM
		}

		sourceAutoRefresh := item.SourceAutoRefresh && sourceURL != ""
		if sourceAutoRefresh && sourceRefreshIntervalM <= 0 {
			sourceRefreshIntervalM = defaultSourceRefreshIntervalM
		}
		if !sourceAutoRefresh {
			sourceRefreshIntervalM = 0
		}
		if sourceURL == "" {
			sourceID = ""
			sourceNamePrefix = ""
			sourceLastRefreshAt = ""
			sourceFetchProxyID = ""
			sourceLastRefreshError = ""
			sourceAutoRefresh = false
			sourceRefreshIntervalM = 0
		}

		normalized = append(normalized, config.BrowserProxy{
			ProxyId:                proxyID,
			ProxyName:              proxyName,
			ProxyConfig:            proxyConfig,
			PreferredKernel:        NormalizePreferredKernel(item.PreferredKernel),
			DnsServers:             strings.TrimSpace(item.DnsServers),
			GroupName:              strings.TrimSpace(item.GroupName),
			SourceID:               sourceID,
			SourceURL:              sourceURL,
			SourceNamePrefix:       sourceNamePrefix,
			SourceAutoRefresh:      sourceAutoRefresh,
			SourceRefreshIntervalM: sourceRefreshIntervalM,
			SourceLastRefreshAt:    sourceLastRefreshAt,
			SourceFetchProxyID:     sourceFetchProxyID,
			SourceLastRefreshError: sourceLastRefreshError,
			SourceMissing:          item.SourceMissing,
			SourceMissingSince:     strings.TrimSpace(item.SourceMissingSince),
			SortOrder:              i,
		})
	}

	return ensureBuiltinDirectProxy(normalized)
}

func ensureBuiltinDirectProxy(proxies []config.BrowserProxy) []config.BrowserProxy {
	const directProxyID = "__direct__"
	for _, item := range proxies {
		if item.ProxyId == directProxyID {
			return proxies
		}
	}

	builtin := config.BrowserProxy{
		ProxyId:     directProxyID,
		ProxyName:   "直连（不走代理）",
		ProxyConfig: "direct://",
	}
	return append([]config.BrowserProxy{builtin}, proxies...)
}
