package backend

import (
	"ant-chrome/backend/internal/proxy"
	"strings"
	"time"
)

// BrowserProxyBuildDiagnostic 构建代理桥接诊断信息，不启动代理进程。
func (a *App) BrowserProxyBuildDiagnostic(proxyId string, proxyConfig string) ProxyBuildDiagnostic {
	proxies := a.getLatestProxies()
	return proxy.BuildProxyDiagnostic(proxyConfig, proxies, proxyId, proxy.BuildDiagnosticOptions{
		XrayMgr:       a.xrayMgr,
		SingBoxMgr:    a.singboxMgr,
		ClashMgr:      a.clashMgr,
		ConnectorType: a.defaultProxyConnectorType(),
	})
}

// BrowserProxyProbeBrowserPage 运行浏览器式并发探测，用于诊断真实页面并发访问效果。
func (a *App) BrowserProxyProbeBrowserPage(request ProxyBrowserProbeRequest) ProxyBrowserProbeResult {
	request.ProxyId = strings.TrimSpace(request.ProxyId)
	proxies := a.getLatestProxies()
	cfg := buildProxyBrowserProbeConfig(request)
	result := proxy.ProbeBrowserPageConnectivityWithConnector(request.ProxyId, proxies, a.xrayMgr, a.singboxMgr, a.clashMgr, a.defaultProxyConnectorType(), &cfg)
	return ProxyBrowserProbeResult{
		ProxyId:     result.ProxyId,
		Ok:          result.Ok,
		TotalMs:     result.TotalMs,
		AverageMs:   result.AverageMs,
		P95Ms:       result.P95Ms,
		Bytes:       result.Bytes,
		Completed:   result.Completed,
		Failed:      result.Failed,
		Concurrency: result.Concurrency,
		Error:       result.Error,
	}
}

func buildProxyBrowserProbeConfig(request ProxyBrowserProbeRequest) proxy.BrowserPageProbeConfig {
	cfg := proxy.DefaultBrowserPageProbeConfig
	cfg.URLs = append([]string{}, proxy.DefaultBrowserPageProbeConfig.URLs...)
	if len(request.URLs) > 0 {
		urls := make([]string, 0, len(request.URLs))
		for _, rawURL := range request.URLs {
			if url := strings.TrimSpace(rawURL); url != "" {
				urls = append(urls, url)
			}
		}
		if len(urls) > 0 {
			cfg.URLs = urls
		}
	}
	if request.TimeoutMs > 0 {
		cfg.Timeout = time.Duration(request.TimeoutMs) * time.Millisecond
	}
	if request.Concurrency > 0 {
		cfg.Concurrency = request.Concurrency
	}
	if cfg.Concurrency > 16 {
		cfg.Concurrency = 16
	}
	return cfg
}
