package proxy

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/metacubex/mihomo/adapter"
	"github.com/metacubex/mihomo/common/utils"
	C "github.com/metacubex/mihomo/constant"

	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/logger"
)

// ─── Clash 标准测速 URL ───
// 使用 HTTP 与 Clash 客户端保持一致

const DefaultSpeedTestURL = "http://www.gstatic.com/generate_204"

// SpeedTestConfig 测速参数
type SpeedTestConfig struct {
	Timeout        time.Duration
	TCPTimeout     time.Duration
	URLs           []string
	ExpectedStatus []int
}

var DefaultSpeedTestConfig = SpeedTestConfig{
	Timeout:    3 * time.Second,
	TCPTimeout: 3 * time.Second,
}

// ─── 对外入口 ───

// SpeedTest 按单个代理的内核决策执行轻量 HTTP 延迟测试。
func SpeedTest(
	proxyId string,
	proxies []config.BrowserProxy,
	xrayMgr *XrayManager,
	singboxMgr *SingBoxManager,
	cfg *SpeedTestConfig,
) TestResult {
	return SpeedTestWithConnector(proxyId, proxies, xrayMgr, singboxMgr, nil, config.BrowserConnectorXray, cfg)
}

// SpeedTestWithConnector 保留 connectorType 参数用于旧调用兼容。
// 实际测速内核由 ResolveProxyKernel 按单个代理决定。
func SpeedTestWithConnector(
	proxyId string,
	proxies []config.BrowserProxy,
	xrayMgr *XrayManager,
	singboxMgr *SingBoxManager,
	clashMgr *ClashManager,
	connectorType string,
	cfg *SpeedTestConfig,
) TestResult {
	connectorType = config.NormalizeBrowserConnectorType(connectorType)
	return lightHTTPDelayTestWithConnector(proxyId, proxies, xrayMgr, singboxMgr, clashMgr, connectorType, cfg)
}

func lightHTTPDelayTestWithConnector(
	proxyId string,
	proxies []config.BrowserProxy,
	xrayMgr *XrayManager,
	singboxMgr *SingBoxManager,
	clashMgr *ClashManager,
	connectorType string,
	cfg *SpeedTestConfig,
) TestResult {
	log := logger.New("SpeedTest")

	if cfg == nil {
		c := DefaultSpeedTestConfig
		cfg = &c
	}

	src := resolveProxyConfig("", proxies, proxyId)
	if src == "" {
		return TestResult{ProxyId: proxyId, Ok: false, Engine: connectorType, Error: "代理配置为空"}
	}

	if strings.ToLower(src) == "direct://" {
		return TestResult{ProxyId: proxyId, Ok: true, LatencyMs: 0, Engine: "direct"}
	}

	testURLs := speedTestTargetURLs(cfg)
	if len(testURLs) == 0 {
		return TestResult{ProxyId: proxyId, Ok: false, Engine: connectorType, Error: "测速目标 URL 为空"}
	}
	engine := speedTestProbeEngine(src, proxies, proxyId, connectorType)
	log.Info("开始代理测速",
		logger.F("proxy_id", proxyId),
		logger.F("engine", engine),
		logger.F("timeout_ms", cfg.Timeout.Milliseconds()),
		logger.F("tcp_timeout_ms", cfg.TCPTimeout.Milliseconds()),
		logger.F("targets", strings.Join(testURLs, ",")),
	)

	client, err := buildSpeedTestHTTPClient(src, proxyId, proxies, xrayMgr, singboxMgr, clashMgr, connectorType, cfg)
	if err != nil {
		log.Warn("代理测速 HTTP 客户端创建失败",
			logger.F("proxy_id", proxyId),
			logger.F("error", err.Error()),
		)
		return TestResult{ProxyId: proxyId, Ok: false, Engine: engine, Error: err.Error()}
	}

	var lastErr error
	var lastLatency int64
	for _, testURL := range testURLs {
		latency, statusCode, err := doSpeedTestRequest(client, testURL)
		lastLatency = latency
		if err != nil {
			lastErr = err
			log.Warn("代理测速请求失败",
				logger.F("proxy_id", proxyId),
				logger.F("engine", engine),
				logger.F("url", testURL),
				logger.F("latency_ms", latency),
				logger.F("error", err.Error()),
			)
			continue
		}
		if speedTestStatusOK(statusCode, cfg) {
			log.Info("代理测速成功",
				logger.F("proxy_id", proxyId),
				logger.F("engine", engine),
				logger.F("url", testURL),
				logger.F("status", statusCode),
				logger.F("latency_ms", latency),
			)
			return TestResult{ProxyId: proxyId, Ok: true, LatencyMs: latency, Engine: engine}
		}
		lastErr = fmt.Errorf("HTTP %d", statusCode)
		log.Warn("代理测速状态码不符合预期",
			logger.F("proxy_id", proxyId),
			logger.F("engine", engine),
			logger.F("url", testURL),
			logger.F("status", statusCode),
			logger.F("latency_ms", latency),
		)
	}

	if lastErr != nil {
		errorMessage := lastErr.Error()
		if runtimeError := speedTestRuntimeError(engine, src, proxies, proxyId, xrayMgr); runtimeError != "" {
			errorMessage = runtimeError
		}
		log.Warn("代理测速失败",
			logger.F("proxy_id", proxyId),
			logger.F("engine", engine),
			logger.F("latency_ms", lastLatency),
			logger.F("error", errorMessage),
		)
		return TestResult{ProxyId: proxyId, Ok: false, LatencyMs: lastLatency, Engine: engine, Error: errorMessage}
	}
	log.Warn("代理测速失败", logger.F("proxy_id", proxyId), logger.F("engine", engine), logger.F("error", "测速失败"))
	return TestResult{ProxyId: proxyId, Ok: false, LatencyMs: lastLatency, Engine: engine, Error: "测速失败"}
}

func buildSpeedTestHTTPClient(
	src string,
	proxyId string,
	proxies []config.BrowserProxy,
	xrayMgr *XrayManager,
	singboxMgr *SingBoxManager,
	clashMgr *ClashManager,
	connectorType string,
	cfg *SpeedTestConfig,
) (*http.Client, error) {
	timeout := DefaultSpeedTestConfig.Timeout
	prepareTimeout := DefaultSpeedTestConfig.TCPTimeout
	if cfg != nil {
		if cfg.Timeout > 0 {
			timeout = cfg.Timeout
		}
		if cfg.TCPTimeout > 0 {
			prepareTimeout = cfg.TCPTimeout
		}
	}
	if prepareTimeout <= 0 {
		prepareTimeout = timeout
	}

	type clientResult struct {
		client *http.Client
		err    error
	}
	ctx, cancel := context.WithTimeout(context.Background(), prepareTimeout)
	defer cancel()
	resultCh := make(chan clientResult, 1)
	go func() {
		client, err := buildProxyHTTPClientContext(ctx, src, proxyId, proxies, xrayMgr, singboxMgr, clashMgr, connectorType, timeout)
		resultCh <- clientResult{client: client, err: err}
	}()

	select {
	case result := <-resultCh:
		if result.err != nil && errors.Is(result.err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("代理准备超时（%dms）", prepareTimeout.Milliseconds())
		}
		return result.client, result.err
	case <-ctx.Done():
		return nil, fmt.Errorf("代理准备超时（%dms）", prepareTimeout.Milliseconds())
	}
}

func primarySpeedTestURL(cfg *SpeedTestConfig) string {
	if cfg != nil {
		if urls := normalizeSpeedTestURLs(cfg.URLs); len(urls) > 0 {
			return urls[0]
		}
	}
	return strings.TrimSpace(DefaultSpeedTestURL)
}

func speedTestTargetURLs(cfg *SpeedTestConfig) []string {
	if cfg != nil {
		if urls := uniqueSpeedTestURLs(cfg.URLs); len(urls) > 0 {
			return urls
		}
	}
	return []string{DefaultSpeedTestURL}
}

func speedTestProbeEngine(src string, proxies []config.BrowserProxy, proxyId string, connectorType string) string {
	resolution, err := ResolveProxyKernelForConnector(src, proxies, proxyId, connectorType)
	if err != nil {
		if resolution.Kernel != "" {
			return resolution.Kernel
		}
		return config.NormalizeBrowserConnectorType(connectorType)
	}
	if resolution.Kernel == ProxyKernelNative {
		return "native"
	}
	return resolution.Kernel
}

func speedTestRuntimeError(engine string, src string, proxies []config.BrowserProxy, proxyId string, xrayMgr *XrayManager) string {
	if engine != ProxyKernelXray || xrayMgr == nil {
		return ""
	}
	dnsServers := ""
	if proxyId != "" {
		for _, item := range proxies {
			if strings.EqualFold(item.ProxyId, proxyId) {
				dnsServers = item.DnsServers
				break
			}
		}
	}
	key := computeNodeKey(normalizeNodeScheme(src) + "\x00" + dnsServers)
	return latestXrayErrorSummary(filepath.Join(xrayMgr.resolveWorkdir(key), "xray-error.log"))
}

func latestXrayErrorSummary(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" || !strings.Contains(line, "[Error]") {
			continue
		}
		if idx := strings.Index(line, "[Error]"); idx >= 0 {
			line = strings.TrimSpace(line[idx+len("[Error]"):])
		}
		if len([]rune(line)) > 240 {
			line = string([]rune(line)[:240]) + "..."
		}
		return "xray 转发失败: " + line
	}
	return ""
}

func doSpeedTestRequest(client *http.Client, testURL string) (int64, int, error) {
	latency, statusCode, err := doSpeedTestRequestWithMethod(client, http.MethodHead, testURL)
	if err != nil || statusCode != http.StatusMethodNotAllowed {
		if err != nil {
			return latency, statusCode, err
		}
		secondLatency, secondStatusCode, secondErr := doSpeedTestRequestWithMethod(client, http.MethodHead, testURL)
		if secondErr == nil {
			return secondLatency, secondStatusCode, nil
		}
		return latency, statusCode, nil
	}
	return doSpeedTestRequestWithMethod(client, http.MethodGet, testURL)
}

func doSpeedTestRequestWithMethod(client *http.Client, method string, testURL string) (int64, int, error) {
	start := time.Now()
	req, err := http.NewRequest(method, testURL, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("测速请求创建失败: %w", err)
	}
	resp, err := client.Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return latency, 0, err
	}
	_ = resp.Body.Close()
	return latency, resp.StatusCode, nil
}

func speedTestStatusOK(statusCode int, cfg *SpeedTestConfig) bool {
	if cfg != nil && len(cfg.ExpectedStatus) > 0 {
		for _, expected := range cfg.ExpectedStatus {
			if statusCode == expected {
				return true
			}
		}
		return false
	}
	return isSpeedTestSuccessStatus(statusCode)
}

func mihomoURLTest(proxyId string, proxyInstance C.Proxy, testURL string, cfg *SpeedTestConfig) TestResult {
	timeout := DefaultSpeedTestConfig.Timeout
	if cfg != nil && cfg.Timeout > 0 {
		timeout = cfg.Timeout
	}

	expectedStatus, err := speedTestExpectedStatus(cfg)
	if err != nil {
		return TestResult{ProxyId: proxyId, Ok: false, Engine: "mihomo", Error: err.Error()}
	}

	adapter.UnifiedDelay.Store(true)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	delay, err := proxyInstance.URLTest(ctx, testURL, expectedStatus)
	latency := int64(delay)
	if ctx.Err() != nil {
		return TestResult{ProxyId: proxyId, Ok: false, LatencyMs: latency, Engine: "mihomo", Error: "测速超时"}
	}
	if err != nil || delay == 0 {
		if err != nil {
			return TestResult{ProxyId: proxyId, Ok: false, LatencyMs: latency, Engine: "mihomo", Error: err.Error()}
		}
		return TestResult{ProxyId: proxyId, Ok: false, LatencyMs: latency, Engine: "mihomo", Error: "mihomo 延迟测试无结果"}
	}

	return TestResult{ProxyId: proxyId, Ok: true, LatencyMs: latency, Engine: "mihomo"}
}

func speedTestExpectedStatus(cfg *SpeedTestConfig) (utils.IntRanges[uint16], error) {
	if cfg == nil || len(cfg.ExpectedStatus) == 0 {
		return nil, nil
	}
	items := make([]string, 0, len(cfg.ExpectedStatus))
	for _, status := range cfg.ExpectedStatus {
		if status <= 0 || status > 65535 {
			return nil, fmt.Errorf("无效测速状态码: %d", status)
		}
		items = append(items, strconv.Itoa(status))
	}
	return utils.NewUnsignedRangesFromList[uint16](items)
}
