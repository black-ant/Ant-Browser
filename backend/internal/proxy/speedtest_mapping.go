package proxy

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/metacubex/mihomo/common/convert"
	"gopkg.in/yaml.v3"
)

func proxyConfigToMapping(src string) (map[string]any, error) {
	src = strings.TrimSpace(src)
	l := strings.ToLower(src)

	if strings.HasPrefix(l, "http://") {
		return parseStandardProxy(src, "http", false)
	}
	if strings.HasPrefix(l, "https://") {
		return parseStandardProxy(src, "http", true)
	}
	if strings.HasPrefix(l, "socks5://") {
		return parseStandardProxy(src, "socks5", false)
	}
	if strings.HasPrefix(l, "ss://") {
		return parseSSURIToMapping(src)
	}
	if strings.HasPrefix(l, "vmess://") || strings.HasPrefix(l, "vless://") || strings.HasPrefix(l, "trojan://") {
		proxies, err := convert.ConvertsV2Ray([]byte(src))
		if err != nil || len(proxies) == 0 {
			if err != nil {
				return nil, fmt.Errorf("V2Ray URI 解析失败: %w", err)
			}
			return nil, fmt.Errorf("V2Ray URI 未生成 Mihomo 节点")
		}
		if name := getMapString(proxies[0], "name"); name == "" {
			proxies[0]["name"] = "speedtest-proxy"
		}
		return proxies[0], nil
	}
	if strings.HasPrefix(l, "hysteria://") || strings.HasPrefix(l, "hysteria2://") || strings.HasPrefix(l, "tuic://") || strings.HasPrefix(l, "anytls://") {
		outbound, err := BuildSingBoxOutbound(src)
		if err != nil {
			return nil, err
		}
		return singBoxOutboundToMihomoMapping(outbound)
	}

	if strings.Contains(l, "://") && !strings.Contains(l, "type:") {
		return nil, fmt.Errorf("URI 格式暂不支持: %s", l[:min(30, len(l))])
	}

	return parseClashYAMLToMapping(src)
}

func singBoxOutboundToMihomoMapping(outbound map[string]interface{}) (map[string]any, error) {
	proxyType := strings.ToLower(strings.TrimSpace(getMapString(outbound, "type")))
	if proxyType == "" {
		return nil, fmt.Errorf("sing-box 节点缺少 type")
	}
	supported := map[string]bool{"hysteria": true, "hysteria2": true, "tuic": true, "anytls": true}
	if !supported[proxyType] {
		return nil, fmt.Errorf("无法将 sing-box 节点转换为 Mihomo 节点: %s", proxyType)
	}

	node := map[string]any{
		"name": "speedtest-proxy",
		"type": proxyType,
	}
	if server := getMapString(outbound, "server"); server != "" {
		node["server"] = server
	}
	if port := getMapInt(outbound, "server_port"); port > 0 {
		node["port"] = port
	}
	if ports := getMapString(outbound, "server_ports"); ports != "" {
		node["ports"] = strings.ReplaceAll(ports, ":", "-")
	}

	for _, key := range []string{"password", "uuid", "auth_str", "up_mbps", "down_mbps", "obfs", "congestion_control"} {
		if value, ok := outbound[key]; ok {
			node[key] = value
		}
	}
	if auth, ok := node["auth_str"]; ok {
		delete(node, "auth_str")
		node["auth-str"] = auth
	}
	if up, ok := node["up_mbps"]; ok {
		delete(node, "up_mbps")
		node["up"] = fmt.Sprintf("%v Mbps", up)
	}
	if down, ok := node["down_mbps"]; ok {
		delete(node, "down_mbps")
		node["down"] = fmt.Sprintf("%v Mbps", down)
	}
	if congestion, ok := node["congestion_control"]; ok {
		delete(node, "congestion_control")
		node["congestion-controller"] = congestion
	}
	if minIdleSession, ok := outbound["min_idle_session"]; ok {
		node["min-idle-session"] = minIdleSession
	}
	if obfs, ok := node["obfs"].(map[string]interface{}); ok {
		if obfsType := getMapString(obfs, "type"); obfsType != "" {
			node["obfs"] = obfsType
		}
		if obfsPassword := getMapString(obfs, "password"); obfsPassword != "" {
			node["obfs-password"] = obfsPassword
		}
	}

	if tls, ok := outbound["tls"].(map[string]interface{}); ok {
		if serverName := getMapString(tls, "server_name"); serverName != "" {
			node["sni"] = serverName
		}
		if _, exists := tls["insecure"]; exists {
			node["skip-cert-verify"] = getMapBool(tls, "insecure")
		}
		if alpn, exists := tls["alpn"]; exists {
			node["alpn"] = alpn
		}
		if utls, ok := tls["utls"].(map[string]interface{}); ok {
			if fingerprint := getMapString(utls, "fingerprint"); fingerprint != "" && proxyType == "anytls" {
				node["client-fingerprint"] = fingerprint
			}
		}
	}
	for _, key := range []string{"idle_session_check_interval", "idle_session_timeout"} {
		if value, ok := outbound[key]; ok {
			if seconds := durationSeconds(value); seconds > 0 {
				node[strings.ReplaceAll(key, "_", "-")] = seconds
			}
		}
	}
	port := getMapInt(node, "port")
	ports := strings.TrimSpace(getMapString(node, "ports"))
	if getMapString(node, "server") == "" || (port <= 0 && (proxyType != "hysteria" && proxyType != "hysteria2" || ports == "")) {
		return nil, fmt.Errorf("sing-box 节点缺少 server 或 port")
	}
	return node, nil
}

func durationSeconds(value interface{}) int {
	switch item := value.(type) {
	case int:
		return item
	case int64:
		return int(item)
	case float64:
		return int(item)
	case string:
		item = strings.TrimSpace(item)
		if seconds, err := strconv.Atoi(item); err == nil {
			return seconds
		}
		if duration, err := time.ParseDuration(item); err == nil {
			return int(duration / time.Second)
		}
	}
	return 0
}

func parseStandardProxy(src string, proxyType string, tls bool) (map[string]any, error) {
	parsed, err := url.Parse(src)
	if err != nil {
		return nil, fmt.Errorf("代理地址解析失败: %w", err)
	}
	host := strings.TrimSpace(parsed.Hostname())
	port, err := strconv.Atoi(parsed.Port())
	if err != nil {
		port = 0
	}
	if host == "" || port == 0 {
		return nil, fmt.Errorf("无法解析地址: %s", src)
	}
	username := ""
	password := ""
	if parsed.User != nil {
		username = parsed.User.Username()
		password, _ = parsed.User.Password()
	}

	mapping := map[string]any{
		"name":   "speedtest-proxy",
		"type":   proxyType,
		"server": host,
		"port":   port,
	}
	if username != "" {
		mapping["username"] = username
		mapping["password"] = password
	}
	if tls {
		mapping["tls"] = true
	}
	return mapping, nil
}

func parseSSURIToMapping(src string) (map[string]any, error) {
	outbound, err := buildOutboundSS(src)
	if err != nil {
		return nil, err
	}
	settings, ok := outbound["settings"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("ss 节点缺少 settings")
	}
	server, err := firstShadowsocksServer(settings)
	if err != nil {
		return nil, err
	}
	mapping := map[string]any{
		"name":     "speedtest-proxy",
		"type":     "ss",
		"server":   server["address"],
		"port":     server["port"],
		"cipher":   server["method"],
		"password": server["password"],
	}
	return mapping, nil
}

func firstShadowsocksServer(settings map[string]interface{}) (map[string]interface{}, error) {
	servers, ok := settings["servers"].([]interface{})
	if !ok || len(servers) == 0 {
		return nil, fmt.Errorf("ss 节点缺少 servers")
	}
	server, ok := servers[0].(map[string]interface{})
	if !ok || server == nil {
		return nil, fmt.Errorf("ss server 格式无效")
	}
	return server, nil
}

func parseClashYAMLToMapping(src string) (map[string]any, error) {
	var payload interface{}
	if err := yaml.Unmarshal([]byte(src), &payload); err != nil {
		return nil, fmt.Errorf("YAML 解析失败: %v", err)
	}

	node := pickClashNode(payload)
	if node == nil {
		return nil, fmt.Errorf("无法提取 Clash 节点")
	}

	if _, ok := node["name"]; !ok {
		node["name"] = "speedtest-proxy"
	}

	return node, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
