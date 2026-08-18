package proxy

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"ant-chrome/backend/internal/config"
)

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
		return TestResult{ProxyId: proxyId, Ok: false, Engine: "tcp", Error: "代理配置为空"}
	}

	endpoint, err := proxyEndpoint(src)
	if err != nil {
		return TestResult{ProxyId: proxyId, Ok: false, Engine: "tcp", Error: fmt.Sprintf("地址解析失败: %v", err)}
	}

	start := time.Now()
	conn, err := net.DialTimeout("tcp", endpoint, 10*time.Second)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		return TestResult{ProxyId: proxyId, Ok: false, LatencyMs: latency, Engine: "tcp", Error: err.Error()}
	}
	conn.Close()
	return TestResult{ProxyId: proxyId, Ok: true, LatencyMs: latency, Engine: "tcp"}
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
	return TestRealConnectivityWithConfig(proxyId, proxies, xrayMgr, singboxMgr, nil)
}

func TestRealConnectivityWithConfig(
	proxyId string,
	proxies []config.BrowserProxy,
	xrayMgr *XrayManager,
	singboxMgr *SingBoxManager,
	cfg *SpeedTestConfig,
) TestResult {
	return TestRealConnectivityWithRuntimeConfig(proxyId, proxies, xrayMgr, singboxMgr, nil, config.BrowserConnectorXray, cfg)
}

func TestRealConnectivityWithRuntimeConfig(
	proxyId string,
	proxies []config.BrowserProxy,
	xrayMgr *XrayManager,
	singboxMgr *SingBoxManager,
	clashMgr *ClashManager,
	connectorType string,
	cfg *SpeedTestConfig,
) TestResult {
	src := resolveProxyConfig("", proxies, proxyId)
	engine := speedTestProbeEngine(src, proxies, proxyId, connectorType)
	if src == "" {
		return TestResult{ProxyId: proxyId, Ok: false, Engine: engine, Error: "代理配置为空"}
	}

	targetURLs := defaultRealConnectivityTargets()
	timeout := 15 * time.Second
	if cfg != nil {
		if len(cfg.URLs) > 0 {
			configuredURLs := normalizeSpeedTestURLs(cfg.URLs)
			if len(configuredURLs) > 0 {
				targetURLs = append(configuredURLs, targetURLs...)
			}
		}
		if cfg.Timeout > 0 {
			timeout = cfg.Timeout
		}
	}
	targetURLs = uniqueSpeedTestURLs(targetURLs)
	if len(targetURLs) == 0 {
		return TestResult{ProxyId: proxyId, Ok: false, Engine: engine, Error: "真实连通性测试目标 URL 为空"}
	}

	client, err := buildProxyHTTPClient(src, proxyId, proxies, xrayMgr, singboxMgr, clashMgr, connectorType, timeout)
	if err != nil {
		return TestResult{ProxyId: proxyId, Ok: false, Engine: engine, Error: err.Error()}
	}

	var lastErr error
	var lastLatency int64
	for _, targetURL := range targetURLs {
		start := time.Now()
		resp, err := client.Get(targetURL)
		latency := time.Since(start).Milliseconds()
		lastLatency = latency
		if err != nil {
			lastErr = err
			continue
		}
		_ = resp.Body.Close()
		if isSpeedTestSuccessStatus(resp.StatusCode) {
			return TestResult{ProxyId: proxyId, Ok: true, LatencyMs: latency, Engine: engine}
		}
		lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	if lastErr != nil {
		return TestResult{ProxyId: proxyId, Ok: false, LatencyMs: lastLatency, Engine: engine, Error: "真实访问失败: " + lastErr.Error()}
	}
	return TestResult{ProxyId: proxyId, Ok: false, LatencyMs: lastLatency, Engine: engine, Error: "真实连通性测试失败"}
}

func defaultRealConnectivityTargets() []string {
	return []string{
		DefaultSpeedTestURL,
		"http://connect.rom.miui.com/generate_204",
		"https://cp.cloudflare.com/generate_204",
		"https://www.cloudflare.com/cdn-cgi/trace",
		"http://www.baidu.com/favicon.ico",
		"http://www.msftconnecttest.com/connecttest.txt",
	}
}

func normalizeSpeedTestURLs(urls []string) []string {
	result := make([]string, 0, len(urls))
	for _, item := range urls {
		if item = strings.TrimSpace(item); item != "" {
			result = append(result, item)
		}
	}
	return result
}

func uniqueSpeedTestURLs(urls []string) []string {
	result := make([]string, 0, len(urls))
	seen := map[string]struct{}{}
	for _, item := range normalizeSpeedTestURLs(urls) {
		key := strings.ToLower(item)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, item)
	}
	return result
}

func isSpeedTestSuccessStatus(statusCode int) bool {
	return statusCode == http.StatusNoContent || (statusCode >= 200 && statusCode < 300)
}
