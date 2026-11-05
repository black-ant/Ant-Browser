package proxy

import "fmt"

func buildOutboundFromClashTrojan(node map[string]interface{}) (map[string]interface{}, string, error) {
	host := getMapString(node, "server")
	port := getMapInt(node, "port")
	password := getMapString(node, "password")
	sni := getMapString(node, "sni")
	if sni == "" {
		sni = getMapString(node, "servername")
	}
	network := getMapString(node, "network")
	out := map[string]interface{}{
		"protocol": "trojan",
		"tag":      "proxy-out",
		"settings": map[string]interface{}{
			"servers": []interface{}{
				map[string]interface{}{
					"address":  host,
					"port":     port,
					"password": password,
				},
			},
		},
	}
	stream := map[string]interface{}{
		"security": "tls",
		"tlsSettings": map[string]interface{}{
			"serverName": sni,
		},
	}
	applyClashTLSClientOptions(node, stream["tlsSettings"].(map[string]interface{}))
	if network == "ws" {
		stream["network"] = "ws"
		stream["wsSettings"] = buildClashWSSettings(node)
	} else if network == "grpc" {
		stream["network"] = "grpc"
		if grpc := buildClashGRPCSettings(node); len(grpc) > 0 {
			stream["grpcSettings"] = grpc
		}
	}
	out["streamSettings"] = stream
	return out, "", nil
}

func buildOutboundFromClashHysteria2(node map[string]interface{}) (map[string]interface{}, string, error) {
	return nil, "", fmt.Errorf("Xray 不支持 hysteria2 协议，请使用 vless/vmess/socks5/http 格式的代理")
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
			"servers": []interface{}{
				map[string]interface{}{
					"address":  host,
					"port":     port,
					"method":   cipher,
					"password": password,
				},
			},
		},
	}
	if plugin := getMapString(node, "plugin"); plugin != "" {
		return nil, "", fmt.Errorf("Shadowsocks plugin=%s 需要使用 Mihomo 内核，不能按裸 SS 走 Xray", plugin)
	}
	return out, "", nil
}
