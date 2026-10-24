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
	connectorType = config.NormalizeBrowserConnectorType(connectorType)
	explicitKernel := NormalizePreferredKernel(proxyPreferredKernel(proxies, proxyId))
	if explicitKernel != "" && !connectorAllowsKernel(connectorType, explicitKernel) {
		return ProxyKernelResolution{
			PreferredKernel: explicitKernel,
			Protocol:        DetectProxyProtocol(src),
		}, fmt.Errorf("代理指定内核 %s 不属于当前 %s 连接栈，请切换连接栈或修改该代理的内核设置", explicitKernel, connectorType)
	}
	if explicitKernel != "" {
		return ResolveProxyKernel(src, proxies, proxyId, explicitKernel)
	}

	supported := SupportedKernelsForProtocol(DetectProxyProtocol(src), src, proxies, proxyId)
	selectedKernel := selectKernelForConnector(supported, connectorType)
	if selectedKernel == "" {
		protocol := DetectProxyProtocol(src)
		if len(supported) == 0 {
			return ProxyKernelResolution{PreferredKernel: ProxyKernelAuto, Protocol: protocol}, fmt.Errorf("不支持的代理协议: %s", protocol)
		}
		return ProxyKernelResolution{
			PreferredKernel:  ProxyKernelAuto,
			Protocol:         protocol,
			SupportedKernels: supported,
		}, fmt.Errorf("协议 %s 不属于当前 %s 连接栈，请切换连接栈", protocol, connectorType)
	}
	return ResolveProxyKernel(src, proxies, proxyId, selectedKernel)
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

func proxyPreferredKernel(proxies []config.BrowserProxy, proxyId string) string {
	proxyId = strings.TrimSpace(proxyId)
	if proxyId == "" {
		return ""
	}
	for _, item := range proxies {
		if strings.EqualFold(strings.TrimSpace(item.ProxyId), proxyId) {
			return item.PreferredKernel
		}
	}
	return ""
}

func connectorAllowsKernel(connectorType string, kernel string) bool {
	connectorType = config.NormalizeBrowserConnectorType(connectorType)
	kernel = NormalizePreferredKernel(kernel)
	if kernel == ProxyKernelNative {
		return true
	}
	if connectorType == config.BrowserConnectorMihomo {
		return kernel == ProxyKernelMihomo
	}
	return kernel == ProxyKernelXray || kernel == ProxyKernelSingBox
}

func selectKernelForConnector(supported []string, connectorType string) string {
	connectorType = config.NormalizeBrowserConnectorType(connectorType)
	order := []string{ProxyKernelXray, ProxyKernelSingBox, ProxyKernelNative}
	if connectorType == config.BrowserConnectorMihomo {
		order = []string{ProxyKernelMihomo, ProxyKernelNative}
	}
	for _, candidate := range order {
		if containsKernel(supported, candidate) {
			return candidate
		}
	}
	return ""
}
