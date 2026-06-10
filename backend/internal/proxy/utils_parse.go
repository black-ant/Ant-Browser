package proxy

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// proxyEndpoint 从代理配置中提取 server:port，用于 TCP ping
func proxyEndpoint(src string) (string, error) {
	src = strings.TrimSpace(src)
	l := strings.ToLower(src)

	if strings.HasPrefix(l, "socks5://") || strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "https://") {
		parsed, err := url.Parse(src)
		if err != nil {
			return "", err
		}
		if parsed.Host == "" {
			return "", fmt.Errorf("缺少代理地址")
		}
		return parsed.Host, nil
	}

	if strings.HasPrefix(l, "vmess://") {
		raw := strings.TrimPrefix(src, "vmess://")
		decoded, err := decodeBase64String(strings.TrimSpace(raw))
		if err == nil {
			var v struct {
				Add  string      `json:"add"`
				Port interface{} `json:"port"`
			}
			if jsonErr := json.Unmarshal(decoded, &v); jsonErr == nil && v.Add != "" {
				return fmt.Sprintf("%s:%v", v.Add, v.Port), nil
			}
		}
	}

	if strings.HasPrefix(l, "vless://") {
		rest := src[len("vless://"):]
		if at := strings.LastIndex(rest, "@"); at >= 0 {
			hostport := strings.SplitN(rest[at+1:], "?", 2)[0]
			hostport = strings.SplitN(hostport, "#", 2)[0]
			return hostport, nil
		}
	}

	var payload interface{}
	if err := yaml.Unmarshal([]byte(src), &payload); err == nil {
		node := pickClashNode(payload)
		if node != nil {
			server := getMapString(node, "server")
			port := getMapInt(node, "port")
			if server != "" && port > 0 {
				return fmt.Sprintf("%s:%d", server, port), nil
			}
		}
	}

	return "", fmt.Errorf("无法解析代理地址")
}

func toStringMap(input interface{}) map[string]interface{} {
	switch v := input.(type) {
	case map[string]interface{}:
		return v
	case map[interface{}]interface{}:
		out := map[string]interface{}{}
		for k, val := range v {
			out[fmt.Sprint(k)] = val
		}
		return out
	}
	return nil
}

func getMapString(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch s := v.(type) {
	case string:
		return strings.TrimSpace(s)
	case int:
		return strconv.Itoa(s)
	case int64:
		return strconv.FormatInt(s, 10)
	case float64:
		return strconv.Itoa(int(s))
	case bool:
		if s {
			return "true"
		}
		return "false"
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

func getMapInt(m map[string]interface{}, key string) int {
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch s := v.(type) {
	case int:
		return s
	case int64:
		return int(s)
	case float64:
		return int(s)
	case string:
		value, _ := strconv.Atoi(s)
		return value
	}
	return 0
}

func getMapBool(m map[string]interface{}, key string) bool {
	v, ok := m[key]
	if !ok {
		return false
	}
	switch s := v.(type) {
	case bool:
		return s
	case string:
		return strings.ToLower(s) == "true"
	case int:
		return s != 0
	case float64:
		return int(s) != 0
	}
	return false
}

func decodeBase64String(raw string) ([]byte, error) {
	if raw == "" {
		return nil, fmt.Errorf("base64 内容为空")
	}
	if data, err := base64.StdEncoding.DecodeString(raw); err == nil {
		return data, nil
	}
	if data, err := base64.RawStdEncoding.DecodeString(raw); err == nil {
		return data, nil
	}
	if data, err := base64.URLEncoding.DecodeString(raw); err == nil {
		return data, nil
	}
	if data, err := base64.RawURLEncoding.DecodeString(raw); err == nil {
		return data, nil
	}
	return nil, fmt.Errorf("base64 解析失败")
}

func isUnsupportedProtocol(src string) bool {
	l := strings.ToLower(strings.TrimSpace(src))
	return strings.HasPrefix(l, "hysteria://") || strings.HasPrefix(l, "hysteria2://")
}
