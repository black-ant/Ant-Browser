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
// 覆盖场景：
//   - 首行 "- " 前的多余空白、前导空行
//   - 字段行缩进被剥离（如顶层字段顶格而嵌套字段保留 4/8 空格的历史数据）
//   - 字段行缩进噪声（如 1 空格混 2 空格）
//
// 算法为缩进树重建：容器行（"key:" 无值）之后的更深行进入子层级，
// 叶子行（"key: value"）之后的更深行视为缩进噪声归入同级。
// 含块标量（| 或 > 值）的配置不修复（内容行缩进有意义，重排会破坏数据）。
func repairClashNodeYAML(src string) (string, bool) {
	norm := strings.ReplaceAll(src, "\r\n", "\n")
	rawLines := strings.Split(norm, "\n")


	type nodeLine struct {
		indent      int
		content     string
		isContainer bool
	}
	var parsed []nodeLine
	for _, line := range rawLines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if isBlockScalarValue(line) {
			return "", false
		}
		trimmed := strings.TrimSpace(line)
		if len(parsed) == 0 {
			if !strings.HasPrefix(trimmed, "- ") {
				return "", false
			}
			body := strings.TrimPrefix(trimmed, "- ")
			parsed = append(parsed, nodeLine{indent: 0, content: body, isContainer: isContainerLine(body)})
			continue
		}
		if strings.HasPrefix(trimmed, "- ") {
			// 更深缩进的 "- " 是容器字段（如 alpn:）的嵌套序列项，允许保留；
			// 同层/更浅的 "- " 属于多节点序列，超出单节点修复范围。
			indent := len(line) - len(strings.TrimLeft(line, " \t"))
			prev := parsed[len(parsed)-1]
			if !(prev.isContainer && indent > prev.indent) {
				return "", false
			}
			parsed = append(parsed, nodeLine{indent: indent, content: trimmed, isContainer: false})
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		parsed = append(parsed, nodeLine{indent: indent, content: trimmed, isContainer: isContainerLine(trimmed)})
	}
	if len(parsed) == 0 {
		return "", false
	}

	// 缩进树重建：分配层级（0=顶层字段），输出按 2*(level+1) 空格缩进，首行加 "- "。
	indentLevels := map[int]int{}
	levels := make([]int, len(parsed))
	prevLevel := 0
	for i := range parsed {
		if i == 0 {
			levels[i] = 0
		} else {
			prev := parsed[i-1]
			switch {
			case prev.isContainer && parsed[i].indent > prev.indent:
				levels[i] = prevLevel + 1
			case parsed[i].indent == prev.indent:
				levels[i] = prevLevel
			default:
				if lvl, ok := indentLevels[parsed[i].indent]; ok {
					levels[i] = lvl
				} else if parsed[i].indent > prev.indent {
					// 未记录的更深缩进且上一行是叶子：缩进噪声，归入上一行层级
					levels[i] = prevLevel
				} else {
					// 更浅的未记录缩进：回退到不超过该缩进的最深已记录层级，兜底顶层
					levels[i] = 0
					for ind, lvl := range indentLevels {
						if ind <= parsed[i].indent && lvl > levels[i] {
							levels[i] = lvl
						}
					}
				}
			}
		}
		if i > 0 {
			indentLevels[parsed[i].indent] = levels[i]
		}
		prevLevel = levels[i]
	}

	buf := make([]string, len(parsed))
	for i, item := range parsed {
		if i == 0 {
			buf[i] = "- " + item.content
		} else {
			buf[i] = strings.Repeat("  ", levels[i]+1) + item.content
		}
	}
	return strings.Join(buf, "\n"), true
}

// isContainerLine 判断行是否为容器（key: 后无值），其更深缩进行为子项。
func isContainerLine(trimmed string) bool {
	if !strings.Contains(trimmed, ":") {
		return false
	}
	idx := strings.LastIndex(trimmed, ":")
	return strings.TrimSpace(trimmed[idx+1:]) == ""
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


