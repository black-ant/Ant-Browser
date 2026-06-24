package backend

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	maxClashSubscriptionBytes = 8 * 1024 * 1024
	clashSubscriptionTimeout  = 25 * time.Second
)

// BrowserProxyFetchClashByURL 拉取 Clash 订阅 URL，并返回可直接导入的 YAML 文本与建议配置。
func (a *App) BrowserProxyFetchClashByURL(rawURL string) (map[string]interface{}, error) {
	parsedURL, content, payload, err := a.fetchClashSubscriptionPayload(rawURL)
	if err != nil {
		return nil, err
	}

	proxyCount := clashProxyCount(payload)
	if proxyCount <= 0 {
		return nil, fmt.Errorf("未检测到可导入的 proxies 节点")
	}

	dnsYAML := extractClashDNSYAML(payload)
	suggestedGroup := suggestClashGroupName(payload, parsedURL.Hostname())

	return map[string]interface{}{
		"url":            parsedURL.String(),
		"content":        content,
		"proxyCount":     proxyCount,
		"dnsServers":     dnsYAML,
		"suggestedGroup": suggestedGroup,
	}, nil
}

// fetchClashSubscriptionPayload 抓取订阅 URL 并归一化为可解析的 payload。
// 供前端导入预览（BrowserProxyFetchClashByURL）与后端自动刷新（refreshProxySource）共用。
func (a *App) fetchClashSubscriptionPayload(rawURL string) (*url.URL, string, interface{}, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, "", nil, fmt.Errorf("订阅 URL 不能为空")
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil || parsedURL.Host == "" {
		return nil, "", nil, fmt.Errorf("URL 格式无效")
	}
	scheme := strings.ToLower(strings.TrimSpace(parsedURL.Scheme))
	if scheme != "http" && scheme != "https" {
		return nil, "", nil, fmt.Errorf("仅支持 http/https URL")
	}
	if err := validateClashSubscriptionURL(parsedURL); err != nil {
		return nil, "", nil, err
	}

	req, err := http.NewRequest(http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return nil, "", nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "clash-verge/2.0 ant-chrome/1.0")
	req.Header.Set("Accept", "application/yaml,text/yaml,text/plain,*/*")
	req.Header.Set("Cache-Control", "no-cache")

	client := newClashSubscriptionHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", nil, fmt.Errorf("拉取订阅失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", nil, fmt.Errorf("拉取订阅失败: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxClashSubscriptionBytes+1))
	if err != nil {
		return nil, "", nil, fmt.Errorf("读取订阅内容失败: %w", err)
	}
	if len(body) > maxClashSubscriptionBytes {
		return nil, "", nil, fmt.Errorf("订阅内容过大（超过 8MB）")
	}

	content, payload, err := normalizeClashSubscriptionContent(body)
	if err != nil {
		return nil, "", nil, err
	}
	return parsedURL, content, payload, nil
}

func newClashSubscriptionHTTPClient() *http.Client {
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	transport := &http.Transport{
		Proxy:                 nil,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			if len(ips) == 0 {
				return nil, fmt.Errorf("订阅主机未解析到可用地址")
			}
			for _, item := range ips {
				if isBlockedSubscriptionIP(item.IP) {
					return nil, fmt.Errorf("订阅 URL 指向受限地址: %s", item.IP.String())
				}
			}

			var lastErr error
			for _, item := range ips {
				conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(item.IP.String(), port))
				if err == nil {
					return conn, nil
				}
				lastErr = err
			}
			if lastErr != nil {
				return nil, lastErr
			}
			return nil, fmt.Errorf("订阅主机连接失败")
		},
	}

	return &http.Client{
		Timeout:   clashSubscriptionTimeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("订阅 URL 重定向次数过多")
			}
			return validateClashSubscriptionURL(req.URL)
		},
	}
}

func validateClashSubscriptionURL(parsedURL *url.URL) error {
	if parsedURL == nil || parsedURL.Host == "" {
		return fmt.Errorf("URL 格式无效")
	}
	scheme := strings.ToLower(strings.TrimSpace(parsedURL.Scheme))
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("仅支持 http/https URL")
	}
	host := strings.TrimSpace(parsedURL.Hostname())
	if host == "" {
		return fmt.Errorf("URL 主机不能为空")
	}
	if ip := net.ParseIP(host); ip != nil && isBlockedSubscriptionIP(ip) {
		return fmt.Errorf("订阅 URL 指向受限地址: %s", ip.String())
	}
	return nil
}

func isBlockedSubscriptionIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() ||
		ip.IsMulticast()
}

func normalizeClashSubscriptionContent(body []byte) (string, interface{}, error) {
	baseText := strings.TrimSpace(strings.ReplaceAll(string(body), "\r\n", "\n"))
	if baseText == "" {
		return "", nil, fmt.Errorf("订阅内容为空")
	}

	tryTexts := make([]string, 0, 4)
	tryTexts = append(tryTexts, baseText)

	if unescaped, err := url.QueryUnescape(baseText); err == nil {
		unescaped = strings.TrimSpace(strings.ReplaceAll(unescaped, "\r\n", "\n"))
		if unescaped != "" && unescaped != baseText {
			tryTexts = append(tryTexts, unescaped)
		}
	}

	if decoded, ok := decodeBase64Text(baseText); ok {
		tryTexts = append(tryTexts, decoded)
	}

	for _, text := range tryTexts {
		payload, ok := parseClashPayload(text)
		if !ok {
			continue
		}
		if clashProxyCount(payload) > 0 {
			return text, payload, nil
		}
	}

	return "", nil, fmt.Errorf("URL 内容不是有效 Clash YAML（需包含 proxies）")
}

func decodeBase64Text(raw string) (string, bool) {
	candidate := strings.TrimSpace(raw)
	if candidate == "" {
		return "", false
	}
	// 一些订阅会返回 URL-safe base64 或缺少 padding，这里都尝试一遍。
	padded := candidate
	if mod := len(padded) % 4; mod != 0 {
		padded += strings.Repeat("=", 4-mod)
	}

	encoders := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	for _, enc := range encoders {
		if data, err := enc.DecodeString(candidate); err == nil {
			decoded := strings.TrimSpace(strings.ReplaceAll(string(data), "\r\n", "\n"))
			if decoded != "" {
				return decoded, true
			}
		}
		if data, err := enc.DecodeString(padded); err == nil {
			decoded := strings.TrimSpace(strings.ReplaceAll(string(data), "\r\n", "\n"))
			if decoded != "" {
				return decoded, true
			}
		}
	}
	return "", false
}

func parseClashPayload(text string) (interface{}, bool) {
	var payload interface{}
	if err := yaml.Unmarshal([]byte(text), &payload); err != nil {
		return nil, false
	}
	return payload, true
}

func clashProxyCount(payload interface{}) int {
	return len(extractClashProxyNodes(payload))
}

// extractClashProxyNodes 从已解析 payload 取出 proxies 数组，每个节点为字符串键 map。
func extractClashProxyNodes(payload interface{}) []map[string]interface{} {
	var arr []interface{}
	if m := toStringMap(payload); m != nil {
		if a, ok := m["proxies"].([]interface{}); ok {
			arr = a
		} else if a, ok := m["proxy"].([]interface{}); ok {
			arr = a
		} else if a, ok := m["Proxy"].([]interface{}); ok {
			arr = a
		}
	} else if a, ok := payload.([]interface{}); ok {
		arr = a
	}
	nodes := make([]map[string]interface{}, 0, len(arr))
	for _, item := range arr {
		if node := toStringMap(item); node != nil {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// proxyNodeToConfig 将单个 clash 节点序列化为应用内 proxyConfig（单元素 YAML 数组），
// 与前端 proxyToYaml(yaml.dump([node])) 等价；后端测速/桥接消费同一格式。
func proxyNodeToConfig(node map[string]interface{}) (string, error) {
	data, err := yaml.Marshal([]interface{}{node})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func extractClashDNSYAML(payload interface{}) string {
	m := toStringMap(payload)
	if m == nil {
		return ""
	}
	dnsRaw, exists := m["dns"]
	if !exists || dnsRaw == nil {
		return ""
	}
	data, err := yaml.Marshal(map[string]interface{}{
		"dns": dnsRaw,
	})
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func suggestClashGroupName(payload interface{}, fallbackHost string) string {
	fallbackHost = strings.TrimSpace(fallbackHost)
	m := toStringMap(payload)
	if m != nil {
		if groups, ok := m["proxy-groups"].([]interface{}); ok {
			for _, item := range groups {
				if groupMap := toStringMap(item); groupMap != nil {
					if name := strings.TrimSpace(getMapString(groupMap, "name")); name != "" {
						return name
					}
				}
			}
		}
	}
	if strings.HasPrefix(strings.ToLower(fallbackHost), "www.") {
		fallbackHost = fallbackHost[4:]
	}
	return fallbackHost
}

func toStringMap(value interface{}) map[string]interface{} {
	switch m := value.(type) {
	case map[string]interface{}:
		return m
	case map[interface{}]interface{}:
		out := make(map[string]interface{}, len(m))
		for k, v := range m {
			key := fmt.Sprint(k)
			out[key] = v
		}
		return out
	default:
		return nil
	}
}

func getMapString(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	value, ok := m[key]
	if !ok || value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	default:
		return fmt.Sprint(v)
	}
}
