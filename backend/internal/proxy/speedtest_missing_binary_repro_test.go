package proxy

// 复现"内核二进制缺失"场景：所有测速应立即失败（毫秒级），错误指向可执行文件未找到。
import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"
	"time"

	"ant-chrome/backend/internal/config"
)

func TestReproBatchSpeedMissingBinaryFailsInstantly(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	// 确保测试进程不解析到任何 xray（exeDir 临时目录与 PATH 均无内核）。
	_ = os.Unsetenv("XRAY_BINARY_PATH")
	for _, name := range []string{"xray", "xray.exe"} {
		if _, err := exec.LookPath(name); err == nil {
			t.Skipf("PATH 中存在 %s，无法构造内核缺失场景", name)
		}
	}

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer target.Close()
	cfg := &SpeedTestConfig{
		Timeout:        3 * time.Second,
		TCPTimeout:     3 * time.Second,
		URLs:           []string{target.URL},
		ExpectedStatus: []int{http.StatusNoContent},
	}
	xrayMgr := NewXrayManager(config.DefaultConfig(), t.TempDir())

	const total = 40
	proxies := make([]config.BrowserProxy, total)
	configs := make([]string, total)
	for i := range configs {
		configs[i] = fmt.Sprintf("vless://%s@127.0.0.1:1?encryption=none&security=none&type=tcp#missing-%d", reproVlessUUID, i)
		proxies[i] = config.BrowserProxy{ProxyId: fmt.Sprintf("missing-%d", i), ProxyConfig: configs[i]}
	}

	start := time.Now()
	results := make([]TestResult, total)
	for i := range proxies {
		results[i] = SpeedTestWithConnector(proxies[i].ProxyId, proxies, xrayMgr, nil, nil, config.BrowserConnectorXray, cfg)
	}
	elapsed := time.Since(start)

	ok, missing := 0, 0
	for _, r := range results {
		if r.Ok {
			ok++
		}
		if containsAny(r.Error, []string{"未找到 xray 可执行文件"}) {
			missing++
		}
	}
	t.Logf("内核缺失: 成功 %d/%d, 错误含\"未找到 xray 可执行文件\" %d, 总耗时 %s（单条平均 %s）", ok, total, missing, elapsed, elapsed/total)

	if ok != 0 {
		t.Errorf("内核缺失时不应有任何成功，实际成功 %d", ok)
	}
	if missing != total {
		t.Errorf("全部应报\"未找到 xray 可执行文件\"，实际 %d/%d；示例错误: %q", missing, total, firstError(results))
	}
}

func firstError(results []TestResult) string {
	for _, r := range results {
		if r.Error != "" {
			return r.Error
		}
	}
	return ""
}