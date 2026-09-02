package proxy

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/logger"

	xproxy "golang.org/x/net/proxy"
)

// buildProxyHTTPClient 根据代理配置构建 HTTP 客户端，统一用于测速/健康检测场景。
func BuildProxyHTTPClient(
	src string,
	proxyId string,
	proxies []config.BrowserProxy,
	xrayMgr *XrayManager,
	singboxMgr *SingBoxManager,
	clashMgr *ClashManager,
	connectorType string,
	timeout time.Duration,
) (*http.Client, error) {
	return buildProxyHTTPClientContext(context.Background(), src, proxyId, proxies, xrayMgr, singboxMgr, clashMgr, connectorType, timeout)
}

func buildProxyHTTPClient(
	src string,
	proxyId string,
	proxies []config.BrowserProxy,
	xrayMgr *XrayManager,
	singboxMgr *SingBoxManager,
	clashMgr *ClashManager,
	connectorType string,
	timeout time.Duration,
) (*http.Client, error) {
	return buildProxyHTTPClientContext(context.Background(), src, proxyId, proxies, xrayMgr, singboxMgr, clashMgr, connectorType, timeout)
}

func buildProxyHTTPClientContext(
	ctx context.Context,
	src string,
	proxyId string,
	proxies []config.BrowserProxy,
	xrayMgr *XrayManager,
	singboxMgr *SingBoxManager,
	clashMgr *ClashManager,
	connectorType string,
	timeout time.Duration,
) (*http.Client, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	src = strings.TrimSpace(resolveProxyConfig(src, proxies, proxyId))
	log := logger.New("ProxyHTTPClient")
	resolution, err := ResolveProxyKernelForConnector(src, proxies, proxyId, connectorType)
	if err != nil {
		log.Warn("代理内核解析失败",
			logger.F("proxy_id", proxyId),
			logger.F("error", err.Error()),
		)
		return nil, err
	}
	log.Info("代理 HTTP 客户端内核选择",
		logger.F("proxy_id", proxyId),
		logger.F("protocol", resolution.Protocol),
		logger.F("kernel", resolution.Kernel),
		logger.F("preferred_kernel", resolution.PreferredKernel),
		logger.F("supported_kernels", strings.Join(resolution.SupportedKernels, ",")),
		logger.F("reason", resolution.Reason),
	)
	l := strings.ToLower(strings.TrimSpace(src))
	if resolution.Kernel == ProxyKernelNative || l == "" || l == "direct://" {
		if strings.HasPrefix(l, "socks5://") {
			u, err := url.Parse(src)
			if err != nil {
				return nil, fmt.Errorf("SOCKS5 地址解析失败: %w", err)
			}
			var auth *xproxy.Auth
			if u.User != nil {
				pass, _ := u.User.Password()
				auth = &xproxy.Auth{User: u.User.Username(), Password: pass}
			}
			dialer, err := xproxy.SOCKS5("tcp", u.Host, auth, xproxy.Direct)
			if err != nil {
				return nil, fmt.Errorf("SOCKS5 dialer 创建失败: %w", err)
			}
			contextDialer, ok := dialer.(xproxy.ContextDialer)
			if !ok {
				return nil, fmt.Errorf("SOCKS5 dialer 不支持 ContextDialer")
			}
			return &http.Client{Transport: &http.Transport{DialContext: contextDialer.DialContext}, Timeout: timeout}, nil
		}
		if strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "https://") {
			proxyURL, err := url.Parse(src)
			if err != nil {
				return nil, fmt.Errorf("代理地址解析失败: %w", err)
			}
			return &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}, Timeout: timeout}, nil
		}
		return &http.Client{Timeout: timeout}, nil
	}

	switch resolution.Kernel {
	case ProxyKernelMihomo:
		if clashMgr == nil {
			log.Warn("Mihomo 管理器未初始化", logger.F("proxy_id", proxyId))
			return nil, fmt.Errorf("Mihomo 管理器未初始化")
		}
		proxyAddr, err := clashMgr.EnsureNodeBridgeContext(ctx, src, proxies, proxyId)
		if err != nil {
			log.Warn("Mihomo 桥接启动失败", logger.F("proxy_id", proxyId), logger.F("error", err.Error()))
			return nil, fmt.Errorf("Mihomo 桥接启动失败: %w", err)
		}
		log.Info("Mihomo 桥接已就绪", logger.F("proxy_id", proxyId), logger.F("proxy_addr", proxyAddr))
		return buildHTTPProxyClient(proxyAddr, timeout)
	case ProxyKernelSingBox:
		if singboxMgr == nil {
			log.Warn("sing-box 管理器未初始化", logger.F("proxy_id", proxyId))
			return nil, fmt.Errorf("sing-box 管理器未初始化")
		}
		socks5Addr, err := singboxMgr.EnsureBridgeContext(ctx, src, proxies, proxyId)
		if err != nil {
			log.Warn("sing-box 桥接启动失败", logger.F("proxy_id", proxyId), logger.F("error", err.Error()))
			return nil, fmt.Errorf("sing-box 桥接启动失败: %w", err)
		}
		log.Info("sing-box 桥接已就绪", logger.F("proxy_id", proxyId), logger.F("socks5_addr", socks5Addr))
		return buildSocks5HTTPClient(strings.TrimPrefix(socks5Addr, "socks5://"), timeout)
	case ProxyKernelXray:
		if xrayMgr == nil {
			log.Warn("xray 管理器未初始化", logger.F("proxy_id", proxyId))
			return nil, fmt.Errorf("xray 管理器未初始化")
		}
		socks5Addr, err := xrayMgr.EnsureBridgeContext(ctx, src, proxies, proxyId)
		if err != nil {
			log.Warn("xray 桥接启动失败", logger.F("proxy_id", proxyId), logger.F("error", err.Error()))
			return nil, fmt.Errorf("xray 桥接启动失败: %w", err)
		}
		log.Info("xray 桥接已就绪", logger.F("proxy_id", proxyId), logger.F("socks5_addr", socks5Addr))
		return buildSocks5HTTPClient(strings.TrimPrefix(socks5Addr, "socks5://"), timeout)
	default:
		return nil, fmt.Errorf("无法为协议 %s 选择代理内核", resolution.Protocol)
	}
}

func buildHTTPProxyClient(proxyAddr string, timeout time.Duration) (*http.Client, error) {
	proxyURL, err := url.Parse(proxyAddr)
	if err != nil {
		return nil, fmt.Errorf("HTTP 代理地址解析失败: %w", err)
	}
	transport := &http.Transport{Proxy: http.ProxyURL(proxyURL)}
	return &http.Client{Transport: transport, Timeout: timeout}, nil
}

func buildSocks5HTTPClient(socks5Host string, timeout time.Duration) (*http.Client, error) {
	dialer, err := xproxy.SOCKS5("tcp", socks5Host, nil, xproxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("SOCKS5 dialer 创建失败: %w", err)
	}
	contextDialer, ok := dialer.(xproxy.ContextDialer)
	if !ok {
		return nil, fmt.Errorf("SOCKS5 dialer 不支持 ContextDialer")
	}
	transport := &http.Transport{DialContext: contextDialer.DialContext}
	return &http.Client{Transport: transport, Timeout: timeout}, nil
}
