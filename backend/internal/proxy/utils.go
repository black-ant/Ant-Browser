package proxy

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"ant-chrome/backend/internal/config"
	xproxy "golang.org/x/net/proxy"
	"gopkg.in/yaml.v3"
)

// TestResult 代理测试结果
type TestResult struct {
	ProxyId   string
	Ok        bool
	LatencyMs int64
	Error     string
}

// proxyEndpoint 从代理配置中提取 server:port，用于 TCP ping
func proxyEndpoint(src string) (string, error) {
	src = strings.TrimSpace(src)
	l := strings.ToLower(src)

	// 标准 URL 格式: socks5://host:port, http://host:port
	if strings.HasPrefix(l, "socks5://") || strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "https://") {
		hostport := src[strings.Index(src, "//")+2:]
		hostport = strings.SplitN(hostport, "/", 2)[0]
		return hostport, nil
	}

	// vmess:// URL (base64 encoded JSON)
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

	// vless:// URL: vless://uuid@host:port?...
	if strings.HasPrefix(l, "vless://") {
		rest := src[len("vless://"):]
		if at := strings.LastIndex(rest, "@"); at >= 0 {
			hostport := strings.SplitN(rest[at+1:], "?", 2)[0]
			hostport = strings.SplitN(hostport, "#", 2)[0]
			return hostport, nil
		}
	}

	// Clash YAML 格式
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

// TestConnectivity 通过 TCP 握手测试代理服务器的可达性和延迟
// 直接对 server:port 建立 TCP 连接测量 RTT，无需启动外部进程
func TestConnectivity(proxyId string, proxyConfig string, proxies []config.BrowserProxy, _ interface{}) TestResult {
	src := strings.TrimSpace(proxyConfig)
	if proxyId != "" {
		for _, item := range proxies {
			if strings.EqualFold(item.ProxyId, proxyId) {
				src = strings.TrimSpace(item.ProxyConfig)
				break
			}
		}
	}
	if src == "" {
		return TestResult{ProxyId: proxyId, Ok: false, Error: "代理配置为空"}
	}

	endpoint, err := proxyEndpoint(src)
	if err != nil {
		return TestResult{ProxyId: proxyId, Ok: false, Error: fmt.Sprintf("地址解析失败: %v", err)}
	}

	start := time.Now()
	conn, err := net.DialTimeout("tcp", endpoint, 10*time.Second)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		return TestResult{ProxyId: proxyId, Ok: false, LatencyMs: latency, Error: err.Error()}
	}
	conn.Close()
	return TestResult{ProxyId: proxyId, Ok: true, LatencyMs: latency}
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
	if !ok {
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

// isUnsupportedProtocol 判断是否为不支持的协议（hysteria/hysteria2）
func isUnsupportedProtocol(src string) bool {
	l := strings.ToLower(strings.TrimSpace(src))
	return strings.HasPrefix(l, "hysteria://") || strings.HasPrefix(l, "hysteria2://")
}

// TestRealConnectivity 通过代理链路发起真实 HTTP 请求测量端到端延迟。
// - DirectProxy (http/https/socks5)：直接通过该代理发送请求
// - BridgeProxy (vmess/vless/Clash)：调用 EnsureBridge 获取 socks5 地址后发送请求
// - SingBoxProxy (hysteria2/tuic)：调用 SingBoxManager.EnsureBridge 后发送请求
func TestRealConnectivity(
	proxyId string,
	proxies []config.BrowserProxy,
	xrayMgr *XrayManager,
) TestResult {
	return TestRealConnectivityWithSingBox(proxyId, proxies, xrayMgr, nil)
}

// TestRealConnectivityWithSingBox 支持 sing-box 的真实连通性测试
func TestRealConnectivityWithSingBox(
	proxyId string,
	proxies []config.BrowserProxy,
	xrayMgr *XrayManager,
	singboxMgr *SingBoxManager,
) TestResult {
	src := ""
	for _, item := range proxies {
		if strings.EqualFold(item.ProxyId, proxyId) {
			src = strings.TrimSpace(item.ProxyConfig)
			break
		}
	}
	if src == "" {
		return TestResult{ProxyId: proxyId, Ok: false, Error: "代理配置为空"}
	}

	const targetURL = "http://www.gstatic.com/generate_204"
	const timeout = 15 * time.Second

	var client *http.Client

	if IsSingBoxProtocol(src) {
		// hysteria2/tuic → sing-box 桥接
		if singboxMgr == nil {
			return TestResult{ProxyId: proxyId, Ok: false, Error: "sing-box 管理器未初始化，无法测试 hysteria2"}
		}
		socks5Addr, err := singboxMgr.EnsureBridge(src, proxies, proxyId)
		if err != nil {
			return TestResult{ProxyId: proxyId, Ok: false, Error: fmt.Sprintf("sing-box 桥接启动失败: %v", err)}
		}
		socks5Host := strings.TrimPrefix(socks5Addr, "socks5://")
		dialer, err := xproxy.SOCKS5("tcp", socks5Host, nil, xproxy.Direct)
		if err != nil {
			return TestResult{ProxyId: proxyId, Ok: false, Error: fmt.Sprintf("SOCKS5 dialer 创建失败: %v", err)}
		}
		contextDialer, ok := dialer.(xproxy.ContextDialer)
		if !ok {
			return TestResult{ProxyId: proxyId, Ok: false, Error: "SOCKS5 dialer 不支持 ContextDialer"}
		}
		transport := &http.Transport{DialContext: contextDialer.DialContext}
		client = &http.Client{Transport: transport, Timeout: timeout}
	} else if RequiresBridge(src, proxies, proxyId) {
		// BridgeProxy：通过 xray socks5 桥接
		if xrayMgr == nil {
			return TestResult{ProxyId: proxyId, Ok: false, Error: "xray 管理器未初始化"}
		}
		socks5Addr, err := xrayMgr.EnsureBridge(src, proxies, proxyId)
		if err != nil {
			return TestResult{ProxyId: proxyId, Ok: false, Error: fmt.Sprintf("桥接启动失败: %v", err)}
		}
		// 解析 socks5://127.0.0.1:port
		socks5Host := strings.TrimPrefix(socks5Addr, "socks5://")
		dialer, err := xproxy.SOCKS5("tcp", socks5Host, nil, xproxy.Direct)
		if err != nil {
			return TestResult{ProxyId: proxyId, Ok: false, Error: fmt.Sprintf("SOCKS5 dialer 创建失败: %v", err)}
		}
		contextDialer, ok := dialer.(xproxy.ContextDialer)
		if !ok {
			return TestResult{ProxyId: proxyId, Ok: false, Error: "SOCKS5 dialer 不支持 ContextDialer"}
		}
		transport := &http.Transport{DialContext: contextDialer.DialContext}
		client = &http.Client{Transport: transport, Timeout: timeout}
	} else {
		// DirectProxy：http/https/socks5 直接代理
		proxyURL, err := parseAndValidateProxyURL(src)
		if err != nil {
			return TestResult{ProxyId: proxyId, Ok: false, Error: err.Error()}
		}
		transport := &http.Transport{Proxy: http.ProxyURL(proxyURL)}
		client = &http.Client{Transport: transport, Timeout: timeout}
	}

	start := time.Now()
	resp, err := client.Get(targetURL)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return TestResult{ProxyId: proxyId, Ok: false, LatencyMs: latency, Error: err.Error()}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return TestResult{ProxyId: proxyId, Ok: false, LatencyMs: latency, Error: fmt.Sprintf("HTTP %d", resp.StatusCode)}
	}
	return TestResult{ProxyId: proxyId, Ok: true, LatencyMs: latency}
}

// parseAndValidateProxyURL 解析并验证代理URL，防止SSRF和注入攻击
func parseAndValidateProxyURL(src string) (*url.URL, error) {
	proxyURL, err := url.Parse(src)
	if err != nil {
		return nil, fmt.Errorf("代理地址解析失败: %w", err)
	}

	// 验证scheme
	scheme := strings.ToLower(proxyURL.Scheme)
	if scheme != "http" && scheme != "https" && scheme != "socks5" {
		return nil, fmt.Errorf("不支持的代理协议: %s", scheme)
	}

	// 验证host
	host := proxyURL.Hostname()
	if host == "" {
		return nil, fmt.Errorf("代理地址无效: 缺少主机名")
	}

	// 防止SSRF到本地/内网
	if isLocalOrPrivateHost(host) {
		return nil, fmt.Errorf("代理地址不能指向本地或内网")
	}

	// 验证端口
	port := proxyURL.Port()
	if port != "" {
		portNum, err := strconv.Atoi(port)
		if err != nil || portNum < 1 || portNum > 65535 {
			return nil, fmt.Errorf("无效的端口号: %s", port)
		}
	}

	return proxyURL, nil
}

// isLocalOrPrivateHost 检查是否为本地或内网地址
func isLocalOrPrivateHost(host string) bool {
	// 检查localhost
	if host == "localhost" || host == "127.0.0.1" || host == "[::1]" {
		return true
	}

	// 解析IP并检查私有地址段
	ip := net.ParseIP(host)
	if ip != nil {
		return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()
	}

	return false
}
