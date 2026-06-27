package proxy

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/metacubex/mihomo/adapter"
	C "github.com/metacubex/mihomo/constant"
	"gopkg.in/yaml.v3"

	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/logger"
)

// ─── Clash 标准测速 URL ───
// 使用 HTTP 与 Clash 客户端保持一致

const defaultTestURL = "http://www.gstatic.com/generate_204"

// SpeedTestConfig 测速参数
type SpeedTestConfig struct {
	Timeout    time.Duration
	TCPTimeout time.Duration
	URLs       []string
}

var DefaultSpeedTestConfig = SpeedTestConfig{
	Timeout:    10 * time.Second,
	TCPTimeout: 5 * time.Second,
}

// ─── 对外入口 ───

// SpeedTest 使用 mihomo 代理适配器进行测速。
// 采用 unified-delay 策略：先建立连接（预热），再单独计时 HTTP 往返，
// 与 Clash 客户端 unified-delay: true 的延迟结果一致。
func SpeedTest(
	proxyId string,
	proxies []config.BrowserProxy,
	xrayMgr *XrayManager,
	singboxMgr *SingBoxManager,
	cfg *SpeedTestConfig,
) TestResult {
	log := logger.New("SpeedTest")

	if cfg == nil {
		c := DefaultSpeedTestConfig
		cfg = &c
	}

	// 查找代理配置
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

	if strings.ToLower(src) == "direct://" {
		return TestResult{ProxyId: proxyId, Ok: true, LatencyMs: 0}
	}

	testURL := defaultTestURL
	if len(cfg.URLs) > 0 {
		testURL = cfg.URLs[0]
	}

	if shouldUseBridgeHTTPDelayTest(src, proxies, proxyId, xrayMgr) {
		return httpClientDelayTest(proxyId, src, proxies, xrayMgr, singboxMgr, testURL, cfg.Timeout)
	}

	resolvedSrc := src
	if IsChainSocks5Proxy(src) {
		if xrayMgr == nil {
			log.Warn("链式代理测速缺少 Xray 管理器，降级到 TCP ping",
				logger.F("proxy_id", proxyId),
			)
			return tcpPingFallback(proxyId, src, cfg.TCPTimeout, log)
		}
		bridgeSocksURL, bridgeErr := xrayMgr.EnsureBridge(src, proxies, proxyId)
		if bridgeErr != nil {
			log.Warn("链式代理桥接失败，降级到 TCP ping",
				logger.F("proxy_id", proxyId),
				logger.F("error", bridgeErr.Error()),
			)
			return tcpPingFallback(proxyId, src, cfg.TCPTimeout, log)
		}
		resolvedSrc = strings.TrimSpace(bridgeSocksURL)
	}

	// 将代理配置转换为 mihomo mapping
	mapping, err := proxyConfigToMapping(resolvedSrc)
	if err != nil {
		log.Warn("代理配置解析失败，降级到 TCP ping",
			logger.F("proxy_id", proxyId),
			logger.F("error", err.Error()),
		)
		return tcpPingFallback(proxyId, resolvedSrc, cfg.TCPTimeout, log)
	}

	// 使用 mihomo adapter.ParseProxy 创建代理窗口
	proxyInstance, err := adapter.ParseProxy(mapping)
	if err != nil {
		log.Warn("mihomo 代理创建失败，降级到 TCP ping",
			logger.F("proxy_id", proxyId),
			logger.F("error", err.Error()),
			logger.F("type", mapping["type"]),
		)
		return tcpPingFallback(proxyId, resolvedSrc, cfg.TCPTimeout, log)
	}

	// unified-delay 测速：分离连接建立和 HTTP 往返计时
	return unifiedDelayTest(proxyId, proxyInstance, testURL, cfg.Timeout)
}

func shouldUseBridgeHTTPDelayTest(
	src string,
	proxies []config.BrowserProxy,
	proxyId string,
	xrayMgr *XrayManager,
) bool {
	if xrayMgr == nil || !isStandardProxyConfig(src) {
		return false
	}
	// 带账密的标准代理启动时会走 xray 桥接；测速也走同一链路，
	// 这样全局前置代理才能参与延迟测试，避免 UI 与实际启动路径不一致。
	return RequiresBridge(src, proxies, proxyId)
}

func httpClientDelayTest(
	proxyId string,
	src string,
	proxies []config.BrowserProxy,
	xrayMgr *XrayManager,
	singboxMgr *SingBoxManager,
	testURL string,
	timeout time.Duration,
) TestResult {
	client, err := buildProxyHTTPClient(src, proxyId, proxies, xrayMgr, singboxMgr, timeout)
	if err != nil {
		return TestResult{ProxyId: proxyId, Ok: false, Error: err.Error()}
	}
	defer client.CloseIdleConnections()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, testURL, nil)
	if err != nil {
		return TestResult{ProxyId: proxyId, Ok: false, Error: fmt.Sprintf("URL 解析失败: %v", err)}
	}

	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return TestResult{ProxyId: proxyId, Ok: false, LatencyMs: latency, Error: err.Error()}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return TestResult{ProxyId: proxyId, Ok: false, LatencyMs: latency,
			Error: fmt.Sprintf("HTTP %d", resp.StatusCode)}
	}

	return TestResult{ProxyId: proxyId, Ok: true, LatencyMs: latency}
}

// unifiedDelayTest 模拟 Clash unified-delay 模式：
// 1. 通过代理建立到目标的 TCP 连接（预热，不计入延迟）
// 2. 发送第一次 HTTP 请求预热连接（不计入延迟）
// 3. 在已建立的连接上发送第二次 HTTP 请求，只计这次的 RTT
// 这样测出的延迟 = 纯 HTTP 往返时间，和 Clash unified-delay: true 一致。
func unifiedDelayTest(proxyId string, px C.Proxy, testURL string, timeout time.Duration) TestResult {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// 解析目标地址
	addr, err := urlToMeta(testURL)
	if err != nil {
		return TestResult{ProxyId: proxyId, Ok: false, Error: fmt.Sprintf("URL 解析失败: %v", err)}
	}

	// 步骤 1：通过代理 DialContext 建立连接（预热）
	conn, err := px.DialContext(ctx, &addr)
	if err != nil {
		return TestResult{ProxyId: proxyId, Ok: false, Error: fmt.Sprintf("代理连接失败: %v", err)}
	}
	defer conn.Close()

	// 构造复用此连接的 HTTP client
	transport := &http.Transport{
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			return conn, nil
		},
		DisableKeepAlives: false,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	defer client.CloseIdleConnections()

	// 步骤 2：第一次请求预热（不计时）
	req1, _ := http.NewRequestWithContext(ctx, http.MethodHead, testURL, nil)
	resp1, err := client.Do(req1)
	if err != nil {
		return TestResult{ProxyId: proxyId, Ok: false, Error: err.Error()}
	}
	resp1.Body.Close()

	// 步骤 3：第二次请求计时（纯 HTTP RTT）
	start := time.Now()
	req2, _ := http.NewRequestWithContext(ctx, http.MethodHead, testURL, nil)
	resp2, err := client.Do(req2)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		return TestResult{ProxyId: proxyId, Ok: false, LatencyMs: latency, Error: err.Error()}
	}
	resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK && resp2.StatusCode != http.StatusNoContent {
		return TestResult{ProxyId: proxyId, Ok: false, LatencyMs: latency,
			Error: fmt.Sprintf("HTTP %d", resp2.StatusCode)}
	}

	return TestResult{ProxyId: proxyId, Ok: true, LatencyMs: latency}
}

// urlToMeta 将 URL 转换为 mihomo Metadata
func urlToMeta(rawURL string) (C.Metadata, error) {
	var host string
	var portNum uint16
	if strings.HasPrefix(rawURL, "https://") {
		host = rawURL[len("https://"):]
		portNum = 443
	} else if strings.HasPrefix(rawURL, "http://") {
		host = rawURL[len("http://"):]
		portNum = 80
	} else {
		return C.Metadata{}, fmt.Errorf("不支持的 URL scheme")
	}
	// 去掉 path
	if idx := strings.Index(host, "/"); idx >= 0 {
		host = host[:idx]
	}
	// 检查是否有自定义端口
	if h, p, err := net.SplitHostPort(host); err == nil {
		host = h
		fmt.Sscanf(p, "%d", &portNum)
	}

	meta := C.Metadata{
		Host:    host,
		DstPort: portNum,
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		meta.DstIP = addr
	}
	return meta, nil
}

// ─── 代理配置转换为 mihomo mapping ───

func proxyConfigToMapping(src string) (map[string]any, error) {
	src = strings.TrimSpace(src)
	l := strings.ToLower(src)

	// http/https 直连代理
	if strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "https://") {
		return parseStandardProxy(src, "http")
	}
	// socks5 直连代理
	if strings.HasPrefix(l, "socks5://") {
		return parseStandardProxy(src, "socks5")
	}

	// URI 格式（vmess:// vless:// 等）暂不支持直接转 mapping，降级
	if strings.Contains(l, "://") && !strings.Contains(l, "type:") {
		return nil, fmt.Errorf("URI 格式暂不支持: %s", l[:min(30, len(l))])
	}

	// Clash YAML 格式 → 直接解析
	return parseClashYAMLToMapping(src)
}

func isStandardProxyConfig(src string) bool {
	l := strings.ToLower(strings.TrimSpace(src))
	return strings.HasPrefix(l, "http://") ||
		strings.HasPrefix(l, "https://") ||
		strings.HasPrefix(l, "socks5://")
}

func parseStandardProxy(src string, proxyType string) (map[string]any, error) {
	rest := src[strings.Index(src, "://")+3:]

	var username, password, hostport string
	if atIdx := strings.LastIndex(rest, "@"); atIdx >= 0 {
		userInfo := rest[:atIdx]
		hostport = rest[atIdx+1:]
		parts := strings.SplitN(userInfo, ":", 2)
		username = parts[0]
		if len(parts) > 1 {
			password = parts[1]
		}
	} else {
		hostport = rest
	}
	hostport = strings.SplitN(hostport, "/", 2)[0]

	host, port := splitHostPort(hostport)
	if host == "" || port == 0 {
		return nil, fmt.Errorf("无法解析地址: %s", src)
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
	return mapping, nil
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

func splitHostPort(hostport string) (string, int) {
	if strings.HasPrefix(hostport, "[") {
		if idx := strings.LastIndex(hostport, "]:"); idx >= 0 {
			host := hostport[1:idx]
			port := 0
			fmt.Sscanf(hostport[idx+2:], "%d", &port)
			return host, port
		}
		return strings.Trim(hostport, "[]"), 0
	}
	idx := strings.LastIndex(hostport, ":")
	if idx < 0 {
		return hostport, 0
	}
	host := hostport[:idx]
	port := 0
	fmt.Sscanf(hostport[idx+1:], "%d", &port)
	return host, port
}

// ─── TCP Ping 降级 ───

func tcpPingFallback(proxyId, src string, timeout time.Duration, log *logger.Logger) TestResult {
	endpoint, err := proxyEndpoint(src)
	if err != nil {
		return TestResult{ProxyId: proxyId, Ok: false, Error: fmt.Sprintf("无法解析代理地址: %v", err)}
	}

	start := time.Now()
	conn, err := net.DialTimeout("tcp", endpoint, timeout)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		return TestResult{ProxyId: proxyId, Ok: false, LatencyMs: latency, Error: fmt.Sprintf("TCP 连接失败: %v", err)}
	}
	conn.Close()
	return TestResult{ProxyId: proxyId, Ok: true, LatencyMs: latency}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
