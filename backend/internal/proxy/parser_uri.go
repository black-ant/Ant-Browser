package proxy

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

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
	fingerprint := firstNonEmptyQueryValue(q, "fp", "client-fingerprint", "fingerprint")
	insecure := queryBool(q, "insecure", "allowInsecure")
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
	if sec == "reality" {
		stream["security"] = "reality"
		realitySettings := map[string]interface{}{"spiderX": ""}
		if sni != "" {
			realitySettings["serverName"] = sni
		}
		if fingerprint != "" {
			realitySettings["fingerprint"] = fingerprint
		}
		if publicKey := firstNonEmptyQueryValue(q, "pbk", "public-key", "publicKey"); publicKey != "" {
			realitySettings["publicKey"] = publicKey
		}
		if shortID := firstNonEmptyQueryValue(q, "sid", "short-id", "shortId"); shortID != "" {
			realitySettings["shortId"] = shortID
		}
		if spiderX := firstNonEmptyQueryValue(q, "spx", "spider-x", "spiderX"); spiderX != "" {
			realitySettings["spiderX"] = spiderX
		}
		stream["realitySettings"] = realitySettings
	} else if sec == "tls" {
		stream["security"] = "tls"
		tlsSettings := map[string]interface{}{}
		if sni != "" {
			tlsSettings["serverName"] = sni
		}
		if fingerprint != "" {
			tlsSettings["fingerprint"] = fingerprint
		}
		if insecure {
			tlsSettings[xrayTLSInsecurePinKey] = true
		}
		if len(tlsSettings) > 0 {
			stream["tlsSettings"] = tlsSettings
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
	fingerprint := firstNonEmptyQueryValue(q, "fp", "client-fingerprint", "fingerprint")
	insecure := queryBool(q, "insecure", "allowInsecure")
	network := q.Get("type")

	out := map[string]interface{}{
		"protocol": "trojan",
		"tag":      "proxy-out",
		"settings": map[string]interface{}{
			"servers": []interface{}{
				map[string]interface{}{
					"address":  host,
					"port":     p,
					"password": password,
				},
			},
		},
	}
	stream := map[string]interface{}{
		"security":    "tls",
		"tlsSettings": map[string]interface{}{},
	}
	tlsSettings := stream["tlsSettings"].(map[string]interface{})
	if sni != "" {
		tlsSettings["serverName"] = sni
	}
	if fingerprint != "" {
		tlsSettings["fingerprint"] = fingerprint
	}
	if insecure {
		tlsSettings[xrayTLSInsecurePinKey] = true
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

// buildOutboundSS 解析 ss:// URI 格式
// 支持两种格式：
// 1. ss://BASE64(method:password)@host:port
// 2. ss://BASE64(method:password@host:port)
func buildOutboundSS(node string) (map[string]interface{}, error) {
	raw := strings.TrimPrefix(node, "ss://")
	if idx := strings.Index(raw, "#"); idx >= 0 {
		raw = raw[:idx]
	}
	raw = strings.TrimSpace(raw)

	var host, method, password string
	var port int

	if strings.Contains(raw, "@") {
		u, err := url.Parse("ss://" + raw)
		if err != nil {
			return nil, fmt.Errorf("ss 解析失败: %v", err)
		}
		host = u.Hostname()
		port, _ = strconv.Atoi(u.Port())
		userInfo := u.User.String()
		if decoded, err := decodeBase64String(userInfo); err == nil {
			parts := strings.SplitN(string(decoded), ":", 2)
			if len(parts) == 2 {
				method = parts[0]
				password = parts[1]
			}
		} else {
			parts := strings.SplitN(userInfo, ":", 2)
			if len(parts) == 2 {
				method = parts[0]
				password = parts[1]
			}
		}
	} else {
		decoded, err := decodeBase64String(raw)
		if err != nil {
			return nil, fmt.Errorf("ss base64 解析失败: %v", err)
		}
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
		parsedHost, parsedPort, splitErr := splitHostPortLenient(hostPart)
		if splitErr == nil {
			host = parsedHost
			port = parsedPort
		}
	}

	if host == "" || port == 0 || method == "" {
		return nil, fmt.Errorf("ss 节点信息不完整")
	}

	return map[string]interface{}{
		"protocol": "shadowsocks",
		"tag":      "proxy-out",
		"settings": map[string]interface{}{
			"servers": []interface{}{
				map[string]interface{}{
					"address":  host,
					"port":     port,
					"method":   method,
					"password": password,
				},
			},
		},
	}, nil
}
