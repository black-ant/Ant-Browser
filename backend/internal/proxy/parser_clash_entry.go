package proxy

import (
	"fmt"
	"net/url"
	"strings"

	"gopkg.in/yaml.v3"
)

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
		// 历史数据可能存在缩进错位（如首行前导空格）导致序列项解析失败，
		// 这里做一次规范化修复重试，避免节点永远无法测速。
		if repaired, ok := repairClashNodeYAML(data); ok {
			if retryErr := yaml.Unmarshal([]byte(repaired), &payload); retryErr == nil {
				data = repaired
			} else {
				return nil, "", err
			}
		} else {
			return nil, "", err
		}
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

func pickClashNode(payload interface{}) map[string]interface{} {	if m := toStringMap(payload); m != nil {
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

// repairClashNodeYAML 规范化单节点 clash YAML 的缩进错位。
// 覆盖场景：首行 "- " 前的多余空格、字段行缩进不齐（如 1 空格混 2 空格）。
// 含块标量（| 或 > 值）的配置不修复（内容行缩进有意义，重排会破坏数据）。
func repairClashNodeYAML(src string) (string, bool) {
	norm := strings.ReplaceAll(src, "\r\n", "\n")
	lines := strings.Split(norm, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if isBlockScalarValue(line) {
			return "", false
		}
		if len(out) == 0 {
			if !strings.HasPrefix(trimmed, "- ") {
				return "", false
			}
			out = append(out, trimmed)
			continue
		}
		if strings.HasPrefix(trimmed, "- ") {
			// 多节点序列不属于单节点修复范围
			return "", false
		}
		out = append(out, "  "+trimmed)
	}
	if len(out) == 0 {
		return "", false
	}
	return strings.Join(out, "\n"), true
}

// isBlockScalarValue 判断行是否为块标量值（如 headers: |），其内容行缩进不可重排。
func isBlockScalarValue(line string) bool {
	trimmedRight := strings.TrimRight(line, " \t")
	if trimmedRight == "" {
		return false
	}
	if strings.HasSuffix(trimmedRight, "|") || strings.HasSuffix(trimmedRight, ">") {
		return strings.Contains(trimmedRight, ":")
	}
	return false
}
