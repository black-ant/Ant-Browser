package proxy

import (
	"fmt"
	"strings"

	"ant-chrome/backend/internal/config"
)

const (
	ProxyKernelAuto    = "auto"
	ProxyKernelNative  = "native"
	ProxyKernelXray    = "xray"
	ProxyKernelSingBox = "sing-box"
	ProxyKernelMihomo  = "mihomo"
)

type ProxyKernelResolution struct {
	Protocol         string   `json:"protocol"`
	PreferredKernel  string   `json:"preferredKernel"`
	Kernel           string   `json:"kernel"`
	SupportedKernels []string `json:"supportedKernels"`
	MissingCore      string   `json:"missingCore,omitempty"`
	Reason           string   `json:"reason"`
}

func NormalizePreferredKernel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", ProxyKernelAuto:
		return ""
	case ProxyKernelXray:
		return ProxyKernelXray
	case ProxyKernelSingBox, "singbox", "sing_box":
		return ProxyKernelSingBox
	case ProxyKernelMihomo, "clash", "clash-meta":
		return ProxyKernelMihomo
	case ProxyKernelNative:
		return ProxyKernelNative
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func ResolveProxyKernel(proxyConfig string, proxies []config.BrowserProxy, proxyId string, preferredKernel string) (ProxyKernelResolution, error) {
	src := strings.TrimSpace(resolveProxyConfig(proxyConfig, proxies, proxyId))
	if strings.TrimSpace(preferredKernel) == "" && strings.TrimSpace(proxyId) != "" {
		for _, item := range proxies {
			if strings.EqualFold(strings.TrimSpace(item.ProxyId), strings.TrimSpace(proxyId)) {
				preferredKernel = item.PreferredKernel
				break
			}
		}
	}
	preferred := NormalizePreferredKernel(preferredKernel)
	if preferred == "" {
		preferred = ProxyKernelAuto
	}
	resolution := ProxyKernelResolution{PreferredKernel: preferred}
	if src == "" || strings.EqualFold(src, "direct://") {
		resolution.Protocol = "direct"
		resolution.Kernel = ProxyKernelNative
		resolution.SupportedKernels = []string{ProxyKernelNative}
		resolution.Reason = "直连无需代理内核"
		return resolution, validatePreferredKernel(resolution, preferred)
	}

	protocol := DetectProxyProtocol(src)
	resolution.Protocol = protocol
	resolution.SupportedKernels = SupportedKernelsForProtocol(protocol, src, proxies, proxyId)
	if len(resolution.SupportedKernels) == 0 {
		return resolution, fmt.Errorf("不支持的代理协议: %s", protocol)
	}
	if preferred != ProxyKernelAuto {
		if !containsKernel(resolution.SupportedKernels, preferred) {
			if protocol == "chain+proxy" {
				resolution.Kernel = resolution.SupportedKernels[0]
				resolution.Reason = fmt.Sprintf("链式代理指定内核 %s 不可用，已回退到 %s", preferred, resolution.Kernel)
				return resolution, nil
			}
			return resolution, fmt.Errorf("协议 %s 不支持指定内核 %s", protocol, preferred)
		}
		resolution.Kernel = preferred
		resolution.Reason = "使用代理指定内核"
		return resolution, nil
	}
	resolution.Kernel = resolution.SupportedKernels[0]
	resolution.Reason = "按默认内核优先级自动选择"
	return resolution, nil
}

func ResolveProxyKernelForConnector(proxyConfig string, proxies []config.BrowserProxy, proxyId string, connectorType string) (ProxyKernelResolution, error) {
	src := strings.TrimSpace(resolveProxyConfig(proxyConfig, proxies, proxyId))
	preferredKernel := preferredKernelForConnector(src, proxies, proxyId, connectorType)
	return ResolveProxyKernel(src, proxies, proxyId, preferredKernel)
}

func DetectProxyProtocol(proxyConfig string) string {
	src := strings.TrimSpace(proxyConfig)
	l := strings.ToLower(src)
	if src == "" || strings.EqualFold(src, "direct://") {
		return "direct"
	}
	if strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "https://") {
		return "http"
	}
	if strings.HasPrefix(l, "socks5://") {
		return "socks5"
	}
	if IsChainSocks5Proxy(src) {
		return "chain+socks5"
	}
	if IsChainProxy(src) {
		return "chain+proxy"
	}
	if nodeType := clashNodeType(src); nodeType != "" {
		return nodeType
	}
	for _, prefix := range []string{"vmess://", "vless://", "trojan://", "ss://", "ssr://", "hysteria2://", "hysteria://", "tuic://", "anytls://"} {
		if strings.HasPrefix(l, prefix) {
			return strings.TrimSuffix(prefix, "://")
		}
	}
	return "unknown"
}

func SupportedKernelsForProtocol(protocol string, proxyConfig string, proxies []config.BrowserProxy, proxyId string) []string {
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case "direct":
		return []string{ProxyKernelNative}
	case "http", "https", "socks5":
		// 带账号密码鉴权的 socks5/http 代理：Chromium 的 --proxy-server 无法携带凭据，
		// 浏览器 native 会静默丢弃鉴权信息导致连接失败。这类代理必须通过 xray / mihomo
		// 桥接成本地无鉴权 socks5 再交给浏览器。无鉴权的代理仍走 native。
		if RequiresLocalProxyBridgeForBrowser(proxyConfig) {
			return []string{ProxyKernelXray, ProxyKernelMihomo}
		}
		return []string{ProxyKernelNative}
	case "vmess", "vless", "trojan", "chain+socks5":
		return []string{ProxyKernelXray, ProxyKernelMihomo}
	case "chain+proxy":
		cfg, err := ParseChainProxyConfig(proxyConfig)
		if err != nil {
			return nil
		}
		front, err := resolveChainFront(cfg, proxies, proxyId)
		if err != nil {
			return nil
		}
		switch DetectProxyProtocol(front.ProxyConfig) {
		case "http", "socks5", "vmess", "vless", "trojan":
			return []string{ProxyKernelXray}
		case "ss", "shadowsocks":
			return []string{ProxyKernelXray}
		case "hysteria", "hysteria2", "tuic", "anytls":
			return []string{ProxyKernelSingBox}
		case "mieru", "wireguard":
			return []string{ProxyKernelMihomo}
		default:
			if IsSingBoxProtocol(front.ProxyConfig) {
				return []string{ProxyKernelSingBox}
			}
			if IsMihomoOnlyProtocol(front.ProxyConfig) {
				return []string{ProxyKernelMihomo}
			}
		}
		return nil
	case "ss", "shadowsocks":
		if IsMihomoOnlyProtocol(proxyConfig) {
			return []string{ProxyKernelMihomo}
		}
		return []string{ProxyKernelXray, ProxyKernelMihomo}
	case "hysteria", "hysteria2", "tuic", "anytls":
		return []string{ProxyKernelSingBox, ProxyKernelMihomo}
	case "mieru", "wireguard":
		return []string{ProxyKernelMihomo}
	default:
		if RequiresLocalProxyBridgeForBrowser(proxyConfig) || RequiresBridge(proxyConfig, proxies, proxyId) {
			return []string{ProxyKernelXray, ProxyKernelMihomo}
		}
		if IsSingBoxProtocol(proxyConfig) {
			return []string{ProxyKernelSingBox, ProxyKernelMihomo}
		}
		if IsMihomoOnlyProtocol(proxyConfig) {
			return []string{ProxyKernelMihomo}
		}
		return nil
	}
}

func validatePreferredKernel(resolution ProxyKernelResolution, preferred string) error {
	if preferred == "" || preferred == ProxyKernelAuto {
		return nil
	}
	if !containsKernel(resolution.SupportedKernels, preferred) {
		return fmt.Errorf("协议 %s 不支持指定内核 %s", resolution.Protocol, preferred)
	}
	return nil
}

func containsKernel(kernels []string, kernel string) bool {
	kernel = NormalizePreferredKernel(kernel)
	for _, item := range kernels {
		if item == kernel {
			return true
		}
	}
	return false
}

func preferredKernelForConnector(src string, proxies []config.BrowserProxy, proxyId string, connectorType string) string {
	if config.NormalizeBrowserConnectorType(connectorType) != config.BrowserConnectorMihomo {
		return ""
	}
	if proxyHasExplicitPreferredKernel(proxies, proxyId) {
		return ""
	}
	src = strings.TrimSpace(resolveProxyConfig(src, proxies, proxyId))
	protocol := DetectProxyProtocol(src)
	if containsKernel(SupportedKernelsForProtocol(protocol, src, proxies, proxyId), ProxyKernelMihomo) {
		return ProxyKernelMihomo
	}
	return ""
}

func proxyHasExplicitPreferredKernel(proxies []config.BrowserProxy, proxyId string) bool {
	proxyId = strings.TrimSpace(proxyId)
	if proxyId == "" {
		return false
	}
	for _, item := range proxies {
		if strings.EqualFold(strings.TrimSpace(item.ProxyId), proxyId) {
			return NormalizePreferredKernel(item.PreferredKernel) != ""
		}
	}
	return false
}
