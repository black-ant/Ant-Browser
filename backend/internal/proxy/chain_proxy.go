package proxy

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"ant-chrome/backend/internal/config"
)

const chainProxyPrefix = "chain+proxy://"

type chainProxyConfig struct {
	Version       int            `json:"version,omitempty"`
	FrontProxyID  string         `json:"frontProxyId"`
	Landing       chainSocks5Hop `json:"landing"`
	LocalPort     int            `json:"localPort,omitempty"`
	PreferredCore string         `json:"preferredKernel,omitempty"`
}

func IsChainProxy(src string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(src)), chainProxyPrefix)
}

func ParseChainProxyConfig(src string) (*chainProxyConfig, error) {
	raw := strings.TrimSpace(src)
	if !IsChainProxy(raw) {
		return nil, fmt.Errorf("不是通用链式代理配置")
	}
	decoded, err := url.QueryUnescape(raw[len(chainProxyPrefix):])
	if err != nil {
		return nil, fmt.Errorf("链式代理配置解码失败: %w", err)
	}
	var cfg chainProxyConfig
	if err := json.Unmarshal([]byte(decoded), &cfg); err != nil {
		return nil, fmt.Errorf("链式代理配置 JSON 解析失败: %w", err)
	}
	if strings.TrimSpace(cfg.FrontProxyID) == "" {
		return nil, fmt.Errorf("请选择前置代理节点")
	}
	if err := validateChainSocks5Hop("落地", cfg.Landing); err != nil {
		return nil, err
	}
	if cfg.LocalPort < 0 || cfg.LocalPort > 65535 {
		return nil, fmt.Errorf("本地监听端口必须在 1-65535 之间")
	}
	cfg.Version = 2
	cfg.FrontProxyID = strings.TrimSpace(cfg.FrontProxyID)
	cfg.Landing.Protocol = normalizeChainHopProtocol(cfg.Landing.Protocol)
	return &cfg, nil
}

func resolveChainFront(cfg *chainProxyConfig, proxies []config.BrowserProxy, currentProxyID string) (config.BrowserProxy, error) {
	if cfg == nil {
		return config.BrowserProxy{}, fmt.Errorf("链式代理配置为空")
	}
	if strings.EqualFold(cfg.FrontProxyID, strings.TrimSpace(currentProxyID)) {
		return config.BrowserProxy{}, fmt.Errorf("前置代理不能引用链式代理自身")
	}
	for _, item := range proxies {
		if !strings.EqualFold(strings.TrimSpace(item.ProxyId), cfg.FrontProxyID) {
			continue
		}
		frontSrc := strings.TrimSpace(item.ProxyConfig)
		if frontSrc == "" || strings.EqualFold(frontSrc, "direct://") {
			return config.BrowserProxy{}, fmt.Errorf("前置代理 %s 没有可用节点配置", cfg.FrontProxyID)
		}
		if IsChainProxy(frontSrc) || IsChainSocks5Proxy(frontSrc) {
			return config.BrowserProxy{}, fmt.Errorf("前置代理不能再次引用链式代理")
		}
		return item, nil
	}
	return config.BrowserProxy{}, fmt.Errorf("前置代理节点不存在或已删除: %s", cfg.FrontProxyID)
}

func chainHopFromStandardProxy(src string) (chainSocks5Hop, error) {
	parsed, err := url.Parse(strings.TrimSpace(src))
	if err != nil {
		return chainSocks5Hop{}, err
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil || port < 1 || port > 65535 || strings.TrimSpace(parsed.Hostname()) == "" {
		return chainSocks5Hop{}, fmt.Errorf("代理地址或端口无效")
	}
	hop := chainSocks5Hop{Protocol: strings.ToLower(parsed.Scheme), Server: parsed.Hostname(), Port: port}
	if parsed.User != nil {
		hop.Username = parsed.User.Username()
		hop.Password, _ = parsed.User.Password()
	}
	hop.TLS = hop.Protocol == "https"
	return hop, validateChainSocks5Hop("前置", hop)
}

func buildXrayChainOutbounds(cfg *chainProxyConfig, proxies []config.BrowserProxy, currentProxyID string) ([]interface{}, []interface{}, error) {
	front, err := resolveChainFront(cfg, proxies, currentProxyID)
	if err != nil {
		return nil, nil, err
	}
	frontSrc := normalizeNodeScheme(strings.TrimSpace(front.ProxyConfig))
	var frontOutbound map[string]interface{}
	protocol := DetectProxyProtocol(frontSrc)
	if protocol == "http" || protocol == "socks5" {
		hop, err := chainHopFromStandardProxy(frontSrc)
		if err != nil {
			return nil, nil, err
		}
		frontOutbound = chainSocks5Outbound(hop, "front-hop", "")
	} else {
		standard, outbound, err := ParseProxyNode(frontSrc)
		if err != nil {
			return nil, nil, fmt.Errorf("前置节点无法由 Xray 解析: %w", err)
		}
		if standard != "" {
			hop, err := chainHopFromStandardProxy(standard)
			if err != nil {
				return nil, nil, err
			}
			frontOutbound = chainSocks5Outbound(hop, "front-hop", "")
		} else {
			frontOutbound = outbound
			frontOutbound["tag"] = "front-hop"
		}
	}
	landing := chainSocks5Outbound(cfg.Landing, "landing-hop", "front-hop")
	routes := []interface{}{map[string]interface{}{"type": "field", "inboundTag": []string{"socks-in"}, "outboundTag": "landing-hop"}}
	return []interface{}{frontOutbound, landing}, routes, nil
}

func buildSingBoxChainOutbounds(cfg *chainProxyConfig, proxies []config.BrowserProxy, currentProxyID string) ([]interface{}, string, error) {
	front, err := resolveChainFront(cfg, proxies, currentProxyID)
	if err != nil {
		return nil, "", err
	}
	frontOutbound, err := BuildSingBoxOutbound(front.ProxyConfig)
	if err != nil {
		return nil, "", fmt.Errorf("前置节点无法由 sing-box 解析: %w", err)
	}
	frontOutbound["tag"] = "front-hop"
	landing := map[string]interface{}{
		"type": "socks", "tag": "landing-hop", "server": strings.TrimSpace(cfg.Landing.Server),
		"server_port": cfg.Landing.Port, "detour": "front-hop",
	}
	if cfg.Landing.Protocol == "http" || cfg.Landing.Protocol == "https" {
		landing["type"] = "http"
	}
	if strings.TrimSpace(cfg.Landing.Username) != "" {
		landing["username"] = strings.TrimSpace(cfg.Landing.Username)
		landing["password"] = cfg.Landing.Password
	}
	if cfg.Landing.Protocol == "https" || cfg.Landing.TLS {
		landing["tls"] = map[string]interface{}{"enabled": true, "server_name": strings.TrimSpace(cfg.Landing.Server)}
	}
	return []interface{}{frontOutbound, landing}, "landing-hop", nil
}

func chainLandingURL(hop chainSocks5Hop) string {
	scheme := normalizeChainHopProtocol(hop.Protocol)
	host := strings.TrimSpace(hop.Server)
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}
	u := &url.URL{Scheme: scheme, Host: host + ":" + strconv.Itoa(hop.Port)}
	if strings.TrimSpace(hop.Username) != "" {
		u.User = url.UserPassword(strings.TrimSpace(hop.Username), hop.Password)
	}
	return u.String()
}

func buildMihomoChainNodes(cfg *chainProxyConfig, proxies []config.BrowserProxy, currentProxyID string) ([]interface{}, string, error) {
	front, err := resolveChainFront(cfg, proxies, currentProxyID)
	if err != nil {
		return nil, "", err
	}
	frontNode, err := buildMihomoNode(front.ProxyConfig)
	if err != nil {
		return nil, "", fmt.Errorf("前置节点无法由 mihomo 解析: %w", err)
	}
	frontNode["name"] = "front-hop"
	landingNode, err := proxyConfigToMapping(chainLandingURL(cfg.Landing))
	if err != nil {
		return nil, "", err
	}
	landingNode["name"] = "landing-hop"
	landingNode["dialer-proxy"] = "front-hop"
	return []interface{}{frontNode, landingNode}, "landing-hop", nil
}
