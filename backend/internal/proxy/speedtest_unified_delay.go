package proxy

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"

	C "github.com/metacubex/mihomo/constant"
)

// unifiedDelayTest 通过代理访问测速 URL，记录从代理拨号到首个 HTTP 响应完成的端到端耗时。
// master 版本只统计第二次复用连接 HEAD 的 RTT，会漏掉代理拨号/握手耗时，导致显示延迟偏低。
func unifiedDelayTest(proxyId string, px C.Proxy, testURL string, timeout time.Duration) TestResult {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	addr, err := urlToMeta(testURL)
	if err != nil {
		return TestResult{ProxyId: proxyId, Ok: false, Error: fmt.Sprintf("URL 解析失败: %v", err)}
	}

	start := time.Now()
	conn, err := px.DialContext(ctx, &addr)
	if err != nil {
		return TestResult{ProxyId: proxyId, Ok: false, Error: fmt.Sprintf("代理连接失败: %v", err)}
	}
	defer conn.Close()

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

	req, _ := http.NewRequestWithContext(ctx, http.MethodHead, testURL, nil)
	resp, err := client.Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return TestResult{ProxyId: proxyId, Ok: false, LatencyMs: latency, Error: err.Error()}
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return TestResult{
			ProxyId:   proxyId,
			Ok:        false,
			LatencyMs: latency,
			Error:     fmt.Sprintf("HTTP %d", resp.StatusCode),
		}
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

	if idx := strings.Index(host, "/"); idx >= 0 {
		host = host[:idx]
	}
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
