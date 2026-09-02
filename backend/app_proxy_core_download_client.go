package backend

import (
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/proxy"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func (a *App) unifiedProxyCoreHTTPClient(timeout time.Duration, proxyConfig string) (*http.Client, string, error) {
	if a == nil {
		return nil, "", fmt.Errorf("\u5e94\u7528\u5b9e\u4f8b\u4e3a\u7a7a")
	}
	rawProxyConfig := strings.TrimSpace(proxyConfig)
	if rawProxyConfig == "" || rawProxyConfig == "__direct__" || strings.EqualFold(rawProxyConfig, "direct://") {
		return &http.Client{Timeout: timeout, Transport: proxyCoreDirectTransport()}, "\u76f4\u8fde", nil
	}
	if parsed, err := url.Parse(rawProxyConfig); err == nil && isBadLocalHTTPSProxy(parsed) {
		return nil, "", fmt.Errorf("\u4e0b\u8f7d\u4ee3\u7406\u4e0d\u80fd\u586b %s\uff0c127.0.0.1:443 \u901a\u5e38\u4e0d\u662f\u672c\u673a\u4ee3\u7406\u7aef\u53e3\uff1b\u8bf7\u6539\u6210\u771f\u5b9e\u4ee3\u7406\u7aef\u53e3\uff0c\u5982 socks5://127.0.0.1:7890\uff0c\u6216\u7559\u7a7a\u76f4\u8fde", parsed.Host)
	}

	connectorType := config.BrowserConnectorXray
	var proxies []config.BrowserProxy
	if a.config != nil {
		connectorType = config.NormalizeBrowserConnectorType(a.config.Browser.DefaultConnectorType)
		if a.browserMgr != nil {
			proxies = a.getLatestProxies()
		} else {
			proxies = a.config.Browser.Proxies
		}
	}
	client, err := proxy.BuildProxyHTTPClient(rawProxyConfig, "", proxies, a.xrayMgr, a.singboxMgr, a.clashMgr, connectorType, timeout)
	if err != nil {
		return nil, "", err
	}
	return client, "\u7edf\u4e00\u4ee3\u7406\u8fde\u63a5\u94fe\u8def", nil
}
