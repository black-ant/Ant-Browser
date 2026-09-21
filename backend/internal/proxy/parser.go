package proxy

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

const chainSocks5Prefix = "chain+socks5://"

type chainSocks5Hop struct {
	Protocol    string `json:"protocol,omitempty"`
	Server      string `json:"server,omitempty"`
	Port        int    `json:"port,omitempty"`
	Username    string `json:"username,omitempty"`
	Password    string `json:"password,omitempty"`
	TLS         bool   `json:"tls,omitempty"`
	ProxyConfig string `json:"proxyConfig,omitempty"`
}

type chainSocks5Config struct {
	LocalPort int            `json:"localPort,omitempty"`
	First     chainSocks5Hop `json:"first"`
	Second    chainSocks5Hop `json:"second"`
}

func IsChainSocks5Proxy(src string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(src)), chainSocks5Prefix)
}

func ParseChainSocks5Config(src string) (*chainSocks5Config, error) {
	raw := strings.TrimSpace(src)
	if !IsChainSocks5Proxy(raw) {
		return nil, fmt.Errorf("不是链式代理配置")
	}
	encoded := raw[len(chainSocks5Prefix):]
	if strings.TrimSpace(encoded) == "" {
		return nil, fmt.Errorf("链式代理配置为空")
	}

	decoded, err := url.QueryUnescape(encoded)
	if err != nil {
		return nil, fmt.Errorf("链式代理配置解码失败: %w", err)
	}

	var cfg chainSocks5Config
	if err := json.Unmarshal([]byte(decoded), &cfg); err != nil {
		return nil, fmt.Errorf("链式代理配置 JSON 解析失败: %w", err)
	}

	if err := validateChainSocks5Hop("第一层", cfg.First, true); err != nil {
		return nil, err
	}
	if err := validateChainSocks5Hop("第二层", cfg.Second, false); err != nil {
		return nil, err
	}
	if cfg.LocalPort < 0 || cfg.LocalPort > 65535 {
		return nil, fmt.Errorf("本地监听端口必须在 1-65535 之间")
	}
	cfg.First.Protocol = normalizeChainHopProtocol(cfg.First.Protocol)
	cfg.Second.Protocol = normalizeChainHopProtocol(cfg.Second.Protocol)
	return &cfg, nil
}

func normalizeChainHopProtocol(protocol string) string {
	normalized := strings.ToLower(strings.TrimSpace(protocol))
	if normalized == "" {
		return "socks5"
	}
	return normalized
}

func validateChainSocks5Hop(label string, hop chainSocks5Hop, allowProxyConfig bool) error {
	if proxyConfig := strings.TrimSpace(hop.ProxyConfig); proxyConfig != "" {
		if !allowProxyConfig {
			return fmt.Errorf("%s不支持高级节点配置", label)
		}
		if IsChainSocks5Proxy(proxyConfig) {
			return fmt.Errorf("%s不支持嵌套链式代理", label)
		}
		protocol := strings.ToLower(strings.TrimSpace(DetectProxyProtocol(proxyConfig)))
		if protocol == "hysteria2" || protocol == "hy2" {
			if _, err := BuildSingBoxOutbound(proxyConfig); err != nil {
				return fmt.Errorf("%s HY2 节点解析失败: %w", label, err)
			}
			return nil
		}
		switch protocol {
		case "vless", "vmess", "trojan", "ss", "shadowsocks":
		default:
			return fmt.Errorf("%s高级节点仅支持 VLESS、VMess、Trojan、SS、HY2 或兼容的 Clash 节点", label)
		}
		standardProxy, outbound, err := ParseProxyNode(proxyConfig)
		if err != nil {
			return fmt.Errorf("%s高级节点解析失败: %w", label, err)
		}
		if strings.TrimSpace(standardProxy) != "" || outbound == nil {
			return fmt.Errorf("%s高级节点仅支持 VLESS、VMess、Trojan、SS、HY2 或兼容的 Clash 节点", label)
		}
		return nil
	}
	if strings.TrimSpace(hop.Server) == "" {
		return fmt.Errorf("%s代理地址不能为空", label)
	}
	if hop.Port < 1 || hop.Port > 65535 {
		return fmt.Errorf("%s代理端口必须在 1-65535 之间", label)
	}
	protocol := strings.ToLower(strings.TrimSpace(hop.Protocol))
	if protocol != "" && protocol != "socks5" && protocol != "http" && protocol != "https" {
		return fmt.Errorf("%s协议仅支持 http、https 或 socks5", label)
	}
	if strings.TrimSpace(hop.Password) != "" && strings.TrimSpace(hop.Username) == "" {
		return fmt.Errorf("%s填写密码时请同时填写账号", label)
	}
	return nil
}

// ParseProxyNode 解析代理节点
func ParseProxyNode(node string) (string, map[string]interface{}, error) {
	src := strings.TrimSpace(node)
	if src == "" {
		return "", nil, fmt.Errorf("代理节点为空")
	}
	l := strings.ToLower(src)
	if strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "https://") || strings.HasPrefix(l, "socks5://") {
		return src, nil, nil
	}
	if strings.HasPrefix(l, "clash://") || strings.Contains(l, "type:") || strings.Contains(l, "proxies:") {
		outbound, standard, err := parseClashNode(src)
		if err != nil {
			return "", nil, err
		}
		if standard != "" {
			return standard, nil, nil
		}
		if outbound != nil {
			return "", outbound, nil
		}
	}
	outbound, err := buildXrayOutbound(src)
	if err != nil {
		return "", nil, err
	}
	return "", outbound, nil
}

func buildXrayOutbound(node string) (map[string]interface{}, error) {
	l := strings.ToLower(node)
	if strings.HasPrefix(l, "vmess://") {
		return buildOutboundVmess(node)
	}
	if strings.HasPrefix(l, "vless://") {
		return buildOutboundVless(node)
	}
	if strings.HasPrefix(l, "trojan://") {
		return buildOutboundTrojan(node)
	}
	if strings.HasPrefix(l, "ss://") {
		return buildOutboundSS(node)
	}
	if strings.HasPrefix(l, "ssr://") {
		return nil, fmt.Errorf("不支持 ShadowsocksR 协议，Xray 不支持 SSR，请使用 SS/vmess/vless/trojan")
	}
	if strings.HasPrefix(l, "hysteria2://") || strings.HasPrefix(l, "hysteria://") {
		return buildOutboundHysteria2(node)
	}
	return nil, fmt.Errorf("不支持的节点协议")
}
