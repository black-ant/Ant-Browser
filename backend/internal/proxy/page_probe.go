package proxy

import (
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"ant-chrome/backend/internal/config"
)

const defaultBrowserPageProbeConcurrency = 8

var DefaultBrowserPageProbeConfig = BrowserPageProbeConfig{
	URLs:        []string{DefaultSpeedTestURL},
	Timeout:     15 * time.Second,
	Concurrency: defaultBrowserPageProbeConcurrency,
}

type BrowserPageProbeConfig struct {
	URLs        []string
	Timeout     time.Duration
	Concurrency int
}

type BrowserPageProbeResult struct {
	ProxyId     string
	Ok          bool
	TotalMs     int64
	AverageMs   int64
	P95Ms       int64
	Bytes       int64
	Completed   int
	Failed      int
	Concurrency int
	Error       string
}

func ProbeBrowserPageConnectivity(
	proxyId string,
	proxies []config.BrowserProxy,
	xrayMgr *XrayManager,
	singboxMgr *SingBoxManager,
	clashMgr *ClashManager,
	cfg *BrowserPageProbeConfig,
) BrowserPageProbeResult {
	return ProbeBrowserPageConnectivityWithConnector(proxyId, proxies, xrayMgr, singboxMgr, clashMgr, config.BrowserConnectorXray, cfg)
}

func ProbeBrowserPageConnectivityWithConnector(
	proxyId string,
	proxies []config.BrowserProxy,
	xrayMgr *XrayManager,
	singboxMgr *SingBoxManager,
	clashMgr *ClashManager,
	connectorType string,
	cfg *BrowserPageProbeConfig,
) BrowserPageProbeResult {
	normalized := normalizeBrowserPageProbeConfig(cfg)
	client, err := buildProxyHTTPClient("", proxyId, proxies, xrayMgr, singboxMgr, clashMgr, connectorType, normalized.Timeout)
	if err != nil {
		return BrowserPageProbeResult{ProxyId: proxyId, Ok: false, Error: err.Error(), Concurrency: normalized.Concurrency}
	}
	return runBrowserPageProbe(proxyId, client, normalized)
}

func normalizeBrowserPageProbeConfig(cfg *BrowserPageProbeConfig) BrowserPageProbeConfig {
	normalized := DefaultBrowserPageProbeConfig
	normalized.URLs = append([]string{}, DefaultBrowserPageProbeConfig.URLs...)
	if cfg == nil {
		return normalized
	}
	urls := make([]string, 0, len(cfg.URLs))
	for _, rawURL := range cfg.URLs {
		if url := strings.TrimSpace(rawURL); url != "" {
			urls = append(urls, url)
		}
	}
	if len(urls) > 0 {
		normalized.URLs = urls
	}
	if cfg.Timeout > 0 {
		normalized.Timeout = cfg.Timeout
	}
	if cfg.Concurrency > 0 {
		normalized.Concurrency = cfg.Concurrency
	}
	return normalized
}

func runBrowserPageProbe(proxyId string, client *http.Client, cfg BrowserPageProbeConfig) BrowserPageProbeResult {
	startedAt := time.Now()
	latencies := make([]int64, 0, cfg.Concurrency)
	var totalBytes int64
	var firstError string
	var failed int
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < cfg.Concurrency; i++ {
		url := cfg.URLs[i%len(cfg.URLs)]
		wg.Add(1)
		go func(targetURL string) {
			defer wg.Done()
			requestStartedAt := time.Now()
			resp, err := client.Get(targetURL)
			latencyMs := time.Since(requestStartedAt).Milliseconds()
			if err != nil {
				mu.Lock()
				failed++
				if firstError == "" {
					firstError = err.Error()
				}
				mu.Unlock()
				return
			}
			defer resp.Body.Close()
			bytesRead, readErr := io.Copy(io.Discard, resp.Body)
			mu.Lock()
			defer mu.Unlock()
			if readErr != nil || resp.StatusCode >= http.StatusBadRequest {
				failed++
				if firstError == "" {
					if readErr != nil {
						firstError = readErr.Error()
					} else {
						firstError = resp.Status
					}
				}
				return
			}
			latencies = append(latencies, latencyMs)
			totalBytes += bytesRead
		}(url)
	}
	wg.Wait()

	completed := len(latencies)
	result := BrowserPageProbeResult{
		ProxyId:     proxyId,
		Ok:          completed > 0 && failed == 0,
		TotalMs:     time.Since(startedAt).Milliseconds(),
		Bytes:       totalBytes,
		Completed:   completed,
		Failed:      failed,
		Concurrency: cfg.Concurrency,
		Error:       firstError,
	}
	if completed == 0 {
		if result.Error == "" {
			result.Error = "并发探测全部失败"
		}
		return result
	}
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	var sum int64
	for _, latency := range latencies {
		sum += latency
	}
	result.AverageMs = sum / int64(completed)
	result.P95Ms = percentileLatency(latencies, 0.95)
	return result
}

func percentileLatency(sortedLatencies []int64, percentile float64) int64 {
	if len(sortedLatencies) == 0 {
		return 0
	}
	if percentile <= 0 {
		return sortedLatencies[0]
	}
	idx := int(float64(len(sortedLatencies))*percentile + 0.5)
	if idx < 1 {
		idx = 1
	}
	if idx > len(sortedLatencies) {
		idx = len(sortedLatencies)
	}
	return sortedLatencies[idx-1]
}
