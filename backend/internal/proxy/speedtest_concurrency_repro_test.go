package proxy

// 并发复现测试：模拟代理池"测试全部"在并发 5 与 20 下的行为差异。
// 场景 A：本地 HTTP 代理（可测通，native 引擎）
// 场景 B：vless 死端口（真实 xray 桥接启动 + 请求失败）
// 目标：验证高并发是否导致桥接启动失败 / 大面积 ok=false。

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ant-chrome/backend/internal/config"
)

const reproVlessUUID = "3b9410a0-7f6e-4c1e-8a2d-000000000000"

func reproSpeedConfig(t *testing.T, targetURL string) *SpeedTestConfig {
	return &SpeedTestConfig{
		Timeout:        3 * time.Second,
		TCPTimeout:     3 * time.Second,
		URLs:           []string{targetURL},
		ExpectedStatus: []int{http.StatusNoContent},
	}
}

func reproStartTargetServer(t *testing.T) *httptest.Server {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)
	return server
}

// 最简 HTTP 正向代理：http 客户端经代理请求 http://target 时，requestURI 为绝对地址。
func reproStartForwardProxy(t *testing.T) *httptest.Server {
	upstream := &http.Client{Timeout: 5 * time.Second}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		outReq := r.Clone(r.Context())
		outReq.RequestURI = ""
		resp, err := upstream.Do(outReq)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		for key, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write([]byte{})
	}))
	t.Cleanup(server.Close)
	return server
}

func reproBatchSpeed(t *testing.T, proxyConfigs []string, concurrency int, cfg *SpeedTestConfig, xrayMgr *XrayManager) (okCount int, startFailures int, elapsed time.Duration) {
	proxies := make([]config.BrowserProxy, 0, len(proxyConfigs))
	for i, src := range proxyConfigs {
		proxies = append(proxies, config.BrowserProxy{
			ProxyId:     fmt.Sprintf("repro-%d", i),
			ProxyConfig: src,
		})
	}
	results := make([]TestResult, len(proxies))
	jobs := make(chan int, len(proxies))
	var wg sync.WaitGroup
	start := time.Now()
	for worker := 0; worker < concurrency; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				results[idx] = SpeedTestWithConnector(proxies[idx].ProxyId, proxies, xrayMgr, nil, nil, config.BrowserConnectorXray, cfg)
			}
		}()
	}
	for i := range proxies {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	elapsed = time.Since(start)
	for _, r := range results {
		if r.Ok {
			okCount++
		}
		if containsAny(r.Error, []string{"xray 启动失败", "sing-box 启动失败", "mihomo 启动失败", "未找到 xray 可执行文件", "未找到 sing-box 可执行文件"}) {
			startFailures++
		}
	}
	return
}

func containsAny(s string, needles []string) bool {
	for _, n := range needles {
		if s != "" && n != "" && len(s) >= len(n) {
			if idx := indexOf(s, n); idx >= 0 {
				return true
			}
		}
	}
	return false
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func reproXrayManager(t *testing.T) *XrayManager {
	bin := filepath.Join("..", "..", "..", "bin", "xray.exe")
	abs, err := filepath.Abs(bin)
	if err != nil {
		t.Fatalf("resolve xray path: %v", err)
	}
	if _, err := os.Stat(abs); err != nil {
		t.Skipf("本机无 xray.exe（%s），跳过桥接场景", abs)
	}
	t.Setenv("XRAY_BINARY_PATH", abs)
	cfg := config.DefaultConfig()
	// AppRoot 指向临时目录，桥接运行时文件不落到包目录。
	return NewXrayManager(cfg, t.TempDir())
}

// 场景 A：本地 HTTP 代理，可全部测通。并发 5 vs 20 应当全部成功。
func TestReproBatchSpeedHTTPProxyConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	target := reproStartTargetServer(t)
	forward := reproStartForwardProxy(t)
	cfg := reproSpeedConfig(t, target.URL)

	var mgr *XrayManager
	const total = 60
	configs := make([]string, total)
	for i := range configs {
		configs[i] = forward.URL
	}

	var ok5, ok20, startFail5, startFail20 int
	var elapsed5, elapsed20 time.Duration

	ok5, startFail5, elapsed5 = reproBatchSpeed(t, configs, 5, cfg, mgr)
	ok20, startFail20, elapsed20 = reproBatchSpeed(t, configs, 20, cfg, mgr)

	t.Logf("并发5 : 成功 %d/%d, 桥接启动失败 %d, 耗时 %s", ok5, total, startFail5, elapsed5)
	t.Logf("并发20: 成功 %d/%d, 桥接启动失败 %d, 耗时 %s", ok20, total, startFail20, elapsed20)

	if ok5 != total {
		t.Errorf("并发 5 应全部成功，实际成功 %d/%d", ok5, total)
	}
	if ok20 != total {
		t.Errorf("并发 20 出现失败，实际成功 %d/%d", ok20, total)
	}
}

// 场景 B：vless 死端口，走真实 xray 桥接。关注"桥接启动失败"数量（应为 0）。
func TestReproBatchSpeedVlessBridgeConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	target := reproStartTargetServer(t)
	cfg := reproSpeedConfig(t, target.URL)
	xrayMgr := reproXrayManager(t)

	const total = 40
	configs := make([]string, total)
	for i := range configs {
		configs[i] = fmt.Sprintf("vless://%s@127.0.0.1:1?encryption=none&security=none&type=tcp#dead-%d", reproVlessUUID, i)
	}

	var bridgeErr5, bridgeErr20 atomic.Int32
	_ = bridgeErr5
	_ = bridgeErr20

	ok5, startFail5, elapsed5 := reproBatchSpeed(t, configs, 5, cfg, xrayMgr)
	ok20, startFail20, elapsed20 := reproBatchSpeed(t, configs, 20, cfg, xrayMgr)

	t.Logf("并发5 : 成功 %d/%d, 桥接启动失败 %d, 耗时 %s", ok5, total, startFail5, elapsed5)
	t.Logf("并发20: 成功 %d/%d, 桥接启动失败 %d, 耗时 %s", ok20, total, startFail20, elapsed20)

	if startFail5 > 0 {
		t.Errorf("并发 5 出现桥接启动失败 %d 条", startFail5)
	}
	if startFail20 > 0 {
		t.Errorf("并发 20 出现桥接启动失败 %d 条（高并发导致桥接启动问题）", startFail20)
	}
}