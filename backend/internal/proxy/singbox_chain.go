package proxy

import (
	"fmt"
	"strings"
)

func buildSingBoxChainOutbounds(cfg *chainSocks5Config) ([]interface{}, string, error) {
	if cfg == nil {
		return nil, "", fmt.Errorf("链式代理配置为空")
	}
	firstConfig := strings.TrimSpace(cfg.First.ProxyConfig)
	if firstConfig == "" {
		return nil, "", fmt.Errorf("sing-box 链式代理第一层必须为 HY2 节点")
	}
	protocol := strings.ToLower(strings.TrimSpace(DetectProxyProtocol(firstConfig)))
	if protocol != "hysteria2" && protocol != "hy2" {
		return nil, "", fmt.Errorf("sing-box 链式代理第一层当前仅支持 HY2")
	}

	first, err := BuildSingBoxOutbound(firstConfig)
	if err != nil {
		return nil, "", fmt.Errorf("第一层 HY2 节点解析失败: %w", err)
	}
	firstHop := cloneStringInterfaceMap(first)
	firstHop["tag"] = "first-hop"
	delete(firstHop, "detour")

	secondHop, err := buildSingBoxChainHopOutbound(cfg.Second, "second-hop", "first-hop")
	if err != nil {
		return nil, "", err
	}
	return []interface{}{firstHop, secondHop}, "second-hop", nil
}

func buildSingBoxChainHopOutbound(hop chainSocks5Hop, tag string, detour string) (map[string]interface{}, error) {
	protocol := normalizeChainHopProtocol(hop.Protocol)
	server := strings.TrimSpace(hop.Server)
	if server == "" || hop.Port < 1 || hop.Port > 65535 {
		return nil, fmt.Errorf("第二层落地代理地址或端口无效")
	}

	var outbound map[string]interface{}
	switch protocol {
	case "socks5":
		outbound = map[string]interface{}{
			"type":        "socks",
			"tag":         tag,
			"server":      server,
			"server_port": hop.Port,
			"version":     "5",
		}
	case "http", "https":
		outbound = map[string]interface{}{
			"type":        "http",
			"tag":         tag,
			"server":      server,
			"server_port": hop.Port,
		}
		if protocol == "https" || hop.TLS {
			outbound["tls"] = map[string]interface{}{"enabled": true}
		}
	default:
		return nil, fmt.Errorf("第二层协议仅支持 HTTP、HTTPS 或 SOCKS5")
	}
	if username := strings.TrimSpace(hop.Username); username != "" {
		outbound["username"] = username
		if hop.Password != "" {
			outbound["password"] = hop.Password
		}
	}
	if strings.TrimSpace(detour) != "" {
		outbound["detour"] = strings.TrimSpace(detour)
	}
	return outbound, nil
}
