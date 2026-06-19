package proxy

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const chainSocks5Prefix = "chain+socks5://"

type chainSocks5Hop struct {
	Protocol string `json:"protocol"`
	Server   string `json:"server"`
	Port     int    `json:"port"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
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

	if err := validateChainSocks5Hop("第一层", cfg.First); err != nil {
		return nil, err
	}
	if err := validateChainSocks5Hop("第二层", cfg.Second); err != nil {
		return nil, err
	}
	if cfg.LocalPort < 0 || cfg.LocalPort > 65535 {
		return nil, fmt.Errorf("本地监听端口必须在 1-65535 之间")
	}
	if cfg.First.Protocol == "" {
		cfg.First.Protocol = "socks5"
	}
	if cfg.Second.Protocol == "" {
		cfg.Second.Protocol = "socks5"
	}
	return &cfg, nil
}

func validateChainSocks5Hop(label string, hop chainSocks5Hop) error {
	if strings.TrimSpace(hop.Server) == "" {
		return fmt.Errorf("%s代理地址不能为空", label)
	}
	if hop.Port < 1 || hop.Port > 65535 {
		return fmt.Errorf("%s代理端口必须在 1-65535 之间", label)
	}
	protocol := strings.ToLower(strings.TrimSpace(hop.Protocol))
	if protocol != "" && protocol != "socks5" {
		return fmt.Errorf("%s协议仅支持 socks5", label)
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
		// 带账号密码的直连代理：Chrome 的 --proxy-server 无法做代理鉴权（407 challenge），
		// 必须经 xray 桥接成本地无鉴权 socks5。无凭据的仍按原样直连，避免无谓开销。
		if proxyURLHasCredentials(src) {
			ob, err := buildStandardProxyOutbound(src)
			if err != nil {
				return "", nil, err
			}
			return "", ob, nil
		}
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

// proxyURLHasCredentials 判断 http(s)/socks5 代理 URI 是否带账号密码（userinfo）。
func proxyURLHasCredentials(src string) bool {
	u, err := url.Parse(strings.TrimSpace(src))
	if err != nil || u.User == nil {
		return false
	}
	return u.User.Username() != ""
}

// buildStandardProxyOutbound 将带鉴权的 http(s)/socks5 直连代理构造为 xray 出站，
// 使其经本地无鉴权 socks5 暴露给 Chrome（解决 Chrome --proxy-server 无法鉴权问题）。
func buildStandardProxyOutbound(src string) (map[string]interface{}, error) {
	u, err := url.Parse(strings.TrimSpace(src))
	if err != nil {
		return nil, fmt.Errorf("代理地址解析失败: %w", err)
	}
	host := u.Hostname()
	port, _ := strconv.Atoi(u.Port())
	if host == "" || port <= 0 {
		return nil, fmt.Errorf("代理地址缺少主机或端口")
	}
	scheme := strings.ToLower(u.Scheme)
	protocol := "http"
	if scheme == "socks5" || scheme == "socks" {
		protocol = "socks"
	}

	server := map[string]interface{}{"address": host, "port": port}
	if u.User != nil {
		if username := u.User.Username(); username != "" {
			password, _ := u.User.Password()
			server["users"] = []interface{}{map[string]interface{}{"user": username, "pass": password}}
		}
	}

	outbound := map[string]interface{}{
		"protocol": protocol,
		"settings": map[string]interface{}{"servers": []interface{}{server}},
		"tag":      "proxy-out",
	}
	// https 代理：在 http 出站上附加 TLS
	if scheme == "https" {
		outbound["streamSettings"] = map[string]interface{}{
			"security":    "tls",
			"tlsSettings": map[string]interface{}{"serverName": host},
		}
	}
	return outbound, nil
}

func parseClashNode(src string) (map[string]interface{}, string, error) {
	data := strings.TrimSpace(src)
	if strings.HasPrefix(strings.ToLower(data), "clash://") {
		raw := strings.TrimPrefix(data, "clash://")
		raw, _ = url.QueryUnescape(raw)
		decoded, err := decodeBase64String(raw)
		if err != nil {
			return nil, "", err
		}
		data = string(decoded)
	}
	var payload interface{}
	if err := yaml.Unmarshal([]byte(data), &payload); err != nil {
		return nil, "", err
	}
	nodeMap := pickClashNode(payload)
	if nodeMap == nil {
		return nil, "", fmt.Errorf("clash 节点解析失败")
	}
	nodeType := strings.ToLower(getMapString(nodeMap, "type"))
	switch nodeType {
	case "socks5", "http", "https":
		return nil, buildStandardProxyFromClash(nodeMap, nodeType), nil
	case "vmess":
		return buildOutboundFromClashVmess(nodeMap)
	case "vless":
		return buildOutboundFromClashVless(nodeMap)
	case "trojan":
		return buildOutboundFromClashTrojan(nodeMap)
	case "ss", "shadowsocks":
		return buildOutboundFromClashSS(nodeMap)
	case "ssr":
		return nil, "", fmt.Errorf("不支持 ShadowsocksR 协议，Xray 不支持 SSR，请使用 SS/vmess/vless/trojan")
	case "hysteria2", "hysteria":
		return buildOutboundFromClashHysteria2(nodeMap)
	}
	return nil, "", fmt.Errorf("不支持的节点类型")
}

func pickClashNode(payload interface{}) map[string]interface{} {
	if m := toStringMap(payload); m != nil {
		if proxies, ok := m["proxies"]; ok {
			if arr, ok := proxies.([]interface{}); ok && len(arr) > 0 {
				return toStringMap(arr[0])
			}
		}
		if proxyItem, ok := m["proxy"]; ok {
			if node := toStringMap(proxyItem); node != nil {
				return node
			}
		}
		return m
	}
	if arr, ok := payload.([]interface{}); ok && len(arr) > 0 {
		return toStringMap(arr[0])
	}
	return nil
}

func buildStandardProxyFromClash(node map[string]interface{}, scheme string) string {
	host := getMapString(node, "server")
	port := getMapInt(node, "port")
	username := getMapString(node, "username")
	password := getMapString(node, "password")
	if host == "" || port == 0 {
		return ""
	}
	address := fmt.Sprintf("%s:%d", host, port)
	if username != "" {
		user := url.UserPassword(username, password)
		return fmt.Sprintf("%s://%s@%s", scheme, user.String(), address)
	}
	return fmt.Sprintf("%s://%s", scheme, address)
}

func buildOutboundFromClashVless(node map[string]interface{}) (map[string]interface{}, string, error) {
	host := getMapString(node, "server")
	port := getMapInt(node, "port")
	id := getMapString(node, "uuid")
	flow := getMapString(node, "flow")
	// sni 和 servername 都要读
	sni := getMapString(node, "sni")
	if sni == "" {
		sni = getMapString(node, "servername")
	}
	network := getMapString(node, "network")
	out := map[string]interface{}{
		"protocol": "vless",
		"tag":      "proxy-out",
		"settings": map[string]interface{}{
			"vnext": []interface{}{
				map[string]interface{}{
					"address": host,
					"port":    port,
					"users": []interface{}{
						map[string]interface{}{
							"id":         id,
							"flow":       flow,
							"encryption": "none",
						},
					},
				},
			},
		},
	}
	stream := map[string]interface{}{}
	tlsVal := strings.ToLower(getMapString(node, "tls"))
	_, hasRealityOpts := node["reality-opts"]

	if hasRealityOpts {
		// Reality 模式：network 必须显式为 tcp，否则 xray 校验失败
		stream["network"] = "tcp"
		realityOpts := map[string]interface{}{
			"spiderX": "",
		}
		if sni != "" {
			realityOpts["serverName"] = sni
		}
		fingerprint := getMapString(node, "client-fingerprint")
		if fingerprint == "" {
			fingerprint = "chrome"
		}
		realityOpts["fingerprint"] = fingerprint
		if rm := toStringMap(node["reality-opts"]); rm != nil {
			if pbk := getMapString(rm, "public-key"); pbk != "" {
				realityOpts["publicKey"] = pbk
			}
			if sid := getMapString(rm, "short-id"); sid != "" {
				realityOpts["shortId"] = sid
			}
		}
		stream["security"] = "reality"
		stream["realitySettings"] = realityOpts
	} else if getMapBool(node, "tls") || tlsVal == "true" || tlsVal == "tls" {
		// 普通 TLS 模式
		tlsSettings := map[string]interface{}{}
		if sni != "" {
			tlsSettings["serverName"] = sni
		}
		tlsSettings["allowInsecure"] = getMapBool(node, "skip-cert-verify")
		stream["security"] = "tls"
		stream["tlsSettings"] = tlsSettings
	}
	if network == "ws" {
		stream["network"] = "ws"
		ws := map[string]interface{}{}
		if wsOpts, ok := node["ws-opts"]; ok {
			if wsMap := toStringMap(wsOpts); wsMap != nil {
				path := getMapString(wsMap, "path")
				// path 为 "/" 也要设置
				if path != "" {
					ws["path"] = path
				}
				if headers, ok := wsMap["headers"]; ok {
					if headerMap := toStringMap(headers); headerMap != nil {
						if hostH := getMapString(headerMap, "Host"); hostH != "" {
							ws["headers"] = map[string]interface{}{"Host": hostH}
						}
					}
				}
			}
		}
		stream["wsSettings"] = ws
	}
	if network == "grpc" {
		stream["network"] = "grpc"
		if grpcOpts, ok := node["grpc-opts"]; ok {
			if grpcMap := toStringMap(grpcOpts); grpcMap != nil {
				serviceName := getMapString(grpcMap, "grpc-service-name")
				if serviceName != "" {
					stream["grpcSettings"] = map[string]interface{}{"serviceName": serviceName}
				}
			}
		}
	}
	if len(stream) > 0 {
		out["streamSettings"] = stream
	}
	return out, "", nil
}

func buildOutboundFromClashVmess(node map[string]interface{}) (map[string]interface{}, string, error) {
	host := getMapString(node, "server")
	port := getMapInt(node, "port")
	id := getMapString(node, "uuid")
	cipher := getMapString(node, "cipher")
	if cipher == "" {
		cipher = "auto"
	}
	network := getMapString(node, "network")
	// sni 和 servername 都要读
	sni := getMapString(node, "sni")
	if sni == "" {
		sni = getMapString(node, "servername")
	}
	out := map[string]interface{}{
		"protocol": "vmess",
		"tag":      "proxy-out",
		"settings": map[string]interface{}{
			"vnext": []interface{}{
				map[string]interface{}{
					"address": host,
					"port":    port,
					"users": []interface{}{
						map[string]interface{}{
							"id":       id,
							"security": cipher,
						},
					},
				},
			},
		},
	}
	stream := map[string]interface{}{}
	if getMapBool(node, "tls") || strings.ToLower(getMapString(node, "tls")) == "true" {
		tlsSettings := map[string]interface{}{}
		if sni != "" {
			tlsSettings["serverName"] = sni
		}
		skipVerify := getMapBool(node, "skip-cert-verify")
		tlsSettings["allowInsecure"] = skipVerify
		stream["security"] = "tls"
		stream["tlsSettings"] = tlsSettings
	}
	if network == "ws" {
		stream["network"] = "ws"
		ws := map[string]interface{}{}
		if wsOpts, ok := node["ws-opts"]; ok {
			if wsMap := toStringMap(wsOpts); wsMap != nil {
				path := getMapString(wsMap, "path")
				// path 为 "/" 也要设置
				if path != "" {
					ws["path"] = path
				}
				if headers, ok := wsMap["headers"]; ok {
					if headerMap := toStringMap(headers); headerMap != nil {
						if hostH := getMapString(headerMap, "Host"); hostH != "" {
							ws["headers"] = map[string]interface{}{"Host": hostH}
						}
					}
				}
			}
		}
		stream["wsSettings"] = ws
	}
	if network == "grpc" {
		stream["network"] = "grpc"
		if grpcOpts, ok := node["grpc-opts"]; ok {
			if grpcMap := toStringMap(grpcOpts); grpcMap != nil {
				serviceName := getMapString(grpcMap, "grpc-service-name")
				if serviceName != "" {
					stream["grpcSettings"] = map[string]interface{}{"serviceName": serviceName}
				}
			}
		}
	}
	if len(stream) > 0 {
		out["streamSettings"] = stream
	}
	return out, "", nil
}

func buildOutboundFromClashTrojan(node map[string]interface{}) (map[string]interface{}, string, error) {
	host := getMapString(node, "server")
	port := getMapInt(node, "port")
	password := getMapString(node, "password")
	sni := getMapString(node, "sni")
	if sni == "" {
		sni = getMapString(node, "servername")
	}
	network := getMapString(node, "network")
	skipVerify := getMapBool(node, "skip-cert-verify")

	out := map[string]interface{}{
		"protocol": "trojan",
		"tag":      "proxy-out",
		"settings": map[string]interface{}{
			"address":  host,
			"port":     port,
			"password": password,
		},
	}
	stream := map[string]interface{}{
		"security": "tls",
		"tlsSettings": map[string]interface{}{
			"serverName":    sni,
			"allowInsecure": skipVerify,
		},
	}
	if network == "ws" {
		stream["network"] = "ws"
		ws := map[string]interface{}{}
		if wsOpts, ok := node["ws-opts"]; ok {
			if wsMap := toStringMap(wsOpts); wsMap != nil {
				if path := getMapString(wsMap, "path"); path != "" {
					ws["path"] = path
				}
				if headers := toStringMap(wsMap["headers"]); headers != nil {
					if h := getMapString(headers, "Host"); h != "" {
						ws["headers"] = map[string]interface{}{"Host": h}
					}
				}
			}
		}
		stream["wsSettings"] = ws
	} else if network == "grpc" {
		stream["network"] = "grpc"
		if grpcOpts, ok := node["grpc-opts"]; ok {
			if grpcMap := toStringMap(grpcOpts); grpcMap != nil {
				if svcName := getMapString(grpcMap, "grpc-service-name"); svcName != "" {
					stream["grpcSettings"] = map[string]interface{}{"serviceName": svcName}
				}
			}
		}
	}
	out["streamSettings"] = stream
	return out, "", nil
}

func buildOutboundFromClashHysteria2(node map[string]interface{}) (map[string]interface{}, string, error) {
	// 支持的协议: vless, vmess, trojan, shadowsocks, socks, http, wireguard
	// hysteria2 需要使用 Hysteria 客户端或 sing-box
	return nil, "", fmt.Errorf("Xray 不支持 hysteria2 协议，请使用 vless/vmess/socks5/http 格式的代理")
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

func buildOutboundVmess(node string) (map[string]interface{}, error) {
	raw := strings.TrimPrefix(node, "vmess://")
	decoded, err := decodeBase64String(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("vmess 解析失败: %v", err)
	}
	var v struct {
		Add  string `json:"add"`
		Port string `json:"port"`
		ID   string `json:"id"`
		Net  string `json:"net"`
		Type string `json:"type"`
		Host string `json:"host"`
		Path string `json:"path"`
		TLS  string `json:"tls"`
		Sni  string `json:"sni"`
		Alpn string `json:"alpn"`
	}
	if err := json.Unmarshal(decoded, &v); err != nil {
		return nil, fmt.Errorf("vmess 配置解析失败: %v", err)
	}
	p, _ := strconv.Atoi(v.Port)
	out := map[string]interface{}{
		"protocol": "vmess",
		"tag":      "proxy-out",
		"settings": map[string]interface{}{
			"vnext": []interface{}{
				map[string]interface{}{
					"address": v.Add,
					"port":    p,
					"users": []interface{}{
						map[string]interface{}{
							"id":       v.ID,
							"security": "auto",
						},
					},
				},
			},
		},
	}
	stream := map[string]interface{}{}
	if v.TLS == "tls" {
		stream["security"] = "tls"
		if v.Sni != "" {
			stream["tlsSettings"] = map[string]interface{}{"serverName": v.Sni}
		}
	}
	if v.Net == "ws" {
		stream["network"] = "ws"
		ws := map[string]interface{}{}
		if v.Path != "" {
			ws["path"] = v.Path
		}
		if v.Host != "" {
			ws["headers"] = map[string]interface{}{"Host": v.Host}
		}
		if len(ws) > 0 {
			stream["wsSettings"] = ws
		}
	}
	if len(stream) > 0 {
		out["streamSettings"] = stream
	}
	return out, nil
}

func buildOutboundVless(node string) (map[string]interface{}, error) {
	u, err := url.Parse(node)
	if err != nil {
		return nil, fmt.Errorf("vless 解析失败: %v", err)
	}
	host := u.Hostname()
	portStr := u.Port()
	p, _ := strconv.Atoi(portStr)
	id := u.User.Username()
	q := u.Query()
	flow := q.Get("flow")
	sec := strings.ToLower(q.Get("security"))
	sni := q.Get("sni")
	out := map[string]interface{}{
		"protocol": "vless",
		"tag":      "proxy-out",
		"settings": map[string]interface{}{
			"vnext": []interface{}{
				map[string]interface{}{
					"address": host,
					"port":    p,
					"users": []interface{}{
						map[string]interface{}{
							"id":         id,
							"flow":       flow,
							"encryption": "none",
						},
					},
				},
			},
		},
	}
	stream := map[string]interface{}{}
	if sec == "tls" || sec == "reality" {
		stream["security"] = "tls"
		if sni != "" {
			stream["tlsSettings"] = map[string]interface{}{"serverName": sni}
		}
	}
	network := q.Get("type")
	if network == "" {
		network = q.Get("network")
	}
	if network == "ws" {
		stream["network"] = "ws"
		ws := map[string]interface{}{}
		if pth := q.Get("path"); pth != "" {
			ws["path"] = pth
		}
		hostH := q.Get("host")
		if hostH == "" {
			hostH = u.Hostname()
		}
		if hostH != "" {
			ws["headers"] = map[string]interface{}{"Host": hostH}
		}
		stream["wsSettings"] = ws
	}
	if len(stream) > 0 {
		out["streamSettings"] = stream
	}
	return out, nil
}

func buildOutboundHysteria2(node string) (map[string]interface{}, error) {
	// Xray 不支持 hysteria2 作为 outbound 协议
	// 支持的协议: vless, vmess, trojan, shadowsocks, socks, http, wireguard
	// hysteria2 需要使用 Hysteria 客户端或 sing-box
	return nil, fmt.Errorf("Xray 不支持 hysteria2 协议，请使用 vless/vmess/socks5/http 格式的代理")
}

// buildOutboundTrojan 解析 trojan:// URI 格式
func buildOutboundTrojan(node string) (map[string]interface{}, error) {
	u, err := url.Parse(node)
	if err != nil {
		return nil, fmt.Errorf("trojan 解析失败: %v", err)
	}
	host := u.Hostname()
	portStr := u.Port()
	p, _ := strconv.Atoi(portStr)
	password := u.User.Username()
	q := u.Query()
	sni := q.Get("sni")
	if sni == "" {
		sni = q.Get("peer")
	}
	skipVerify := q.Get("allowInsecure") == "1" || strings.ToLower(q.Get("allowInsecure")) == "true"
	network := q.Get("type")

	out := map[string]interface{}{
		"protocol": "trojan",
		"tag":      "proxy-out",
		"settings": map[string]interface{}{
			"address":  host,
			"port":     p,
			"password": password,
		},
	}
	stream := map[string]interface{}{
		"security": "tls",
		"tlsSettings": map[string]interface{}{
			"serverName":    sni,
			"allowInsecure": skipVerify,
		},
	}
	if network == "ws" {
		stream["network"] = "ws"
		ws := map[string]interface{}{}
		if pth := q.Get("path"); pth != "" {
			ws["path"] = pth
		}
		if h := q.Get("host"); h != "" {
			ws["headers"] = map[string]interface{}{"Host": h}
		}
		stream["wsSettings"] = ws
	}
	out["streamSettings"] = stream
	return out, nil
}

// buildOutboundFromClashSS 从 Clash YAML 格式解析 Shadowsocks outbound
func buildOutboundFromClashSS(node map[string]interface{}) (map[string]interface{}, string, error) {
	host := getMapString(node, "server")
	port := getMapInt(node, "port")
	password := getMapString(node, "password")
	cipher := getMapString(node, "cipher")
	if cipher == "" {
		cipher = getMapString(node, "method")
	}
	if cipher == "" {
		cipher = "aes-256-gcm"
	}
	out := map[string]interface{}{
		"protocol": "shadowsocks",
		"tag":      "proxy-out",
		"settings": map[string]interface{}{
			"address":  host,
			"port":     port,
			"method":   cipher,
			"password": password,
		},
	}
	// plugin 支持（obfs/v2ray-plugin）
	if plugin := getMapString(node, "plugin"); plugin != "" {
		pluginOpts := getMapString(node, "plugin-opts")
		_ = pluginOpts // xray 原生不支持 plugin，忽略
	}
	return out, "", nil
}

// buildOutboundSS 解析 ss:// URI 格式
// 支持两种格式：
// 1. ss://BASE64(method:password)@host:port
// 2. ss://BASE64(method:password@host:port)
func buildOutboundSS(node string) (map[string]interface{}, error) {
	raw := strings.TrimPrefix(node, "ss://")
	// 去掉 fragment（#备注）
	if idx := strings.Index(raw, "#"); idx >= 0 {
		raw = raw[:idx]
	}
	raw = strings.TrimSpace(raw)

	var host, method, password string
	var port int

	// 格式1：method:password@host:port（SIP002）
	if strings.Contains(raw, "@") {
		u, err := url.Parse("ss://" + raw)
		if err != nil {
			return nil, fmt.Errorf("ss 解析失败: %v", err)
		}
		host = u.Hostname()
		port, _ = strconv.Atoi(u.Port())
		userInfo := u.User.String()
		// userInfo 可能是 base64 编码的 method:password
		if decoded, err := decodeBase64String(userInfo); err == nil {
			parts := strings.SplitN(string(decoded), ":", 2)
			if len(parts) == 2 {
				method = parts[0]
				password = parts[1]
			}
		} else {
			// 明文 method:password
			parts := strings.SplitN(userInfo, ":", 2)
			if len(parts) == 2 {
				method = parts[0]
				password = parts[1]
			}
		}
	} else {
		// 格式2：整体 base64
		decoded, err := decodeBase64String(raw)
		if err != nil {
			return nil, fmt.Errorf("ss base64 解析失败: %v", err)
		}
		// method:password@host:port
		s := string(decoded)
		atIdx := strings.LastIndex(s, "@")
		if atIdx < 0 {
			return nil, fmt.Errorf("ss 格式错误")
		}
		userPart := s[:atIdx]
		hostPart := s[atIdx+1:]
		parts := strings.SplitN(userPart, ":", 2)
		if len(parts) == 2 {
			method = parts[0]
			password = parts[1]
		}
		hostPort := strings.Split(hostPart, ":")
		if len(hostPort) == 2 {
			host = hostPort[0]
			port, _ = strconv.Atoi(hostPort[1])
		}
	}

	if host == "" || port == 0 || method == "" {
		return nil, fmt.Errorf("ss 节点信息不完整")
	}

	return map[string]interface{}{
		"protocol": "shadowsocks",
		"tag":      "proxy-out",
		"settings": map[string]interface{}{
			"address":  host,
			"port":     port,
			"method":   method,
			"password": password,
		},
	}, nil
}
