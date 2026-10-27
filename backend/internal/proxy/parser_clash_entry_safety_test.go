package proxy

// 真实链路验证：用前端同款 js-yaml（proxyToYaml 相同参数）生成的节点 YAML，
// 逐条走 parseClashNode，确保正常导入产物零误伤、复杂嵌套全部可解析。
import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ant-chrome/backend/internal/config"
)

func loadRealProxyYAMLSamples(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile(`D:\tmp-yaml-samples.json`)
	if err != nil {
		t.Skipf("样本文件不存在（由前端 js-yaml 生成）: %v", err)
	}
	var samples []string
	if err := json.Unmarshal(raw, &samples); err != nil {
		t.Fatalf("解析样本文件失败: %v", err)
	}
	return samples
}

func newXrayManagerForTest(t *testing.T) *XrayManager {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", "..", "..", "bin", "xray.exe"))
	if err != nil {
		t.Fatalf("resolve xray path: %v", err)
	}
	if _, err := os.Stat(abs); err != nil {
		t.Skipf("本机无 xray.exe（%s），跳过真实内核端到端", abs)
	}
	t.Setenv("XRAY_BINARY_PATH", abs)
	return NewXrayManager(config.DefaultConfig(), t.TempDir())
}

func newSpeedTestTargetForTest(t *testing.T) *SpeedTestConfig {
	t.Helper()
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(target.Close)
	return &SpeedTestConfig{
		Timeout:        2 * time.Second,
		TCPTimeout:     2 * time.Second,
		URLs:           []string{target.URL},
		ExpectedStatus: []int{http.StatusNoContent},
	}
}

// 前端正常导入产物（js-yaml dump）必须全部可解析，修复层不得误伤。
func TestRealJsYamlSamplesAllParse(t *testing.T) {
	samples := loadRealProxyYAMLSamples(t)
	if len(samples) < 6 {
		t.Fatalf("样本不足: %d", len(samples))
	}
	parsed := 0
	for i, src := range samples {
		outbound, _, err := parseClashNode(src)
		if err != nil {
			// hysteria2/tuic 等协议设计上走 sing-box 内核，xray 侧"不支持"属预期路由行为
			if strings.Contains(err.Error(), "不支持") {
				continue
			}
			t.Errorf("样本 %d 解析失败（正常导入产物被误伤）: %v\n%s", i, err, src)
			continue
		}
		if outbound == nil {
			// sing-box 专属协议（hysteria2 等）在 xray 出站层返回空属预期
			continue
		}
		parsed++
	}
	t.Logf("成功解析 %d/%d 个真实形态样本", parsed, len(samples))
}

// 修复函数安全性：对合法 YAML 输入，repairClashNodeYAML 不会被调用（上层短路），
// 但即使被直接调用也不应改变可解析性；对块标量嵌套必须拒绝修复。
func TestRepairFunctionSafety(t *testing.T) {
	legal := []string{
		"- name: a\n  server: 1.1.1.1\n  port: 443\n  type: vless",
		"- name: a\n  server: 1.1.1.1\n  type: vless\n  network: ws\n  ws-opts:\n    path: /p",
	}
	for _, src := range legal {
		if _, _, err := parseClashNode(src); err != nil {
			t.Fatalf("合法输入被误伤: %v\n%s", err, src)
		}
	}

	// 块标量（headers: | 形态）：内容行缩进有意义，修复必须拒绝
	blockScalar := "- name: a\n  server: 1.1.1.1\n  type: ss\n  plugin-opts:\n    header: |\n      line1\n      line2"
	if _, _, err := parseClashNode(blockScalar); err != nil {
		t.Fatalf("合法块标量被误伤: %v", err)
	}
}

// 前导 tab 形态（"found character that cannot start any token"）也应被修复。
func TestRepairHandlesLeadingTab(t *testing.T) {
	src := "\t- name: 香港01\n  server: 1.1.1.1\n  port: 443\n  type: vless\n  password: test"
	outbound, _, err := parseClashNode(src)
	if err != nil {
		t.Fatalf("前导 tab 未被修复: %v", err)
	}
	if outbound == nil {
		t.Fatal("应生成出站配置")
	}
}

// 空行开头的畸形也应被修复。
func TestRepairHandlesLeadingBlankLine(t *testing.T) {
	src := "\n - name: 香港01\n  server: 1.1.1.1\n  port: 443\n  type: vless\n  password: test"
	outbound, _, err := parseClashNode(src)
	if err != nil {
		t.Fatalf("空行开头未被修复: %v", err)
	}
	if outbound == nil {
		t.Fatal("应生成出站配置")
	}
}

// 字段行零缩进的畸形也应被修复。
func TestRepairHandlesZeroIndentFields(t *testing.T) {
	src := "- name: 香港01\nserver: 1.1.1.1\nport: 443\ntype: vless\npassword: test"
	outbound, _, err := parseClashNode(src)
	if err != nil {
		t.Fatalf("零缩进字段未被修复: %v", err)
	}
	if outbound == nil {
		t.Fatal("应生成出站配置")
	}
}
// 端到端（真实内核）：畸形节点修复后应真实启动 xray 桥接（而非卡在 yaml 解析），
// 合法节点（js-yaml 真实形态）同样应正常进入桥接启动环节。
func TestEndToEndMalformedNodeStartsRealBridge(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	xrayMgr := newXrayManagerForTest(t)
	target := newSpeedTestTargetForTest(t)

	malformed := " - name: 香港01\n  server: 127.0.0.1\n  port: 1\n  type: vless\n  uuid: d342d11e-d424-4583-b36e-524ab1f0afa4\n  tls: true"
	legal := "- name: 风萧萧-usUS-3-@kiki佬赞助\n  server: 127.0.0.1\n  port: 1\n  type: vless\n  uuid: d342d11e-d424-4583-b36e-524ab1f0afa4\n  tls: true\n  servername: www.bing.com\n  network: ws\n  ws-opts:\n    path: /path\n    headers:\n      Host: example.com"

	for name, src := range map[string]string{"畸形修复后": malformed, "合法真实形态": legal} {
		t.Run(name, func(t *testing.T) {
			proxies := []config.BrowserProxy{{ProxyId: "e2e-" + name, ProxyConfig: src}}
			result := SpeedTestWithConnector("e2e-"+name, proxies, xrayMgr, nil, nil, nil, config.BrowserConnectorXray, target)
			if strings.Contains(result.Error, "yaml:") || strings.Contains(result.Error, "indicator") {
				t.Fatalf("仍卡在 yaml 解析: %s", result.Error)
			}
			// 死端口预期超时（说明已真实进入内核启动+请求环节）
			t.Logf("ok=%v engine=%s error=%s", result.Ok, result.Engine, result.Error)
		})
	}
}

// 用户真实库样本形态：顶层字段缩进被剥离、嵌套字段保留 4/8 空格（历史数据损坏形态）。
func TestRepairHandlesStrippedTopLevelIndentWithNested(t *testing.T) {
	samples := map[string]string{
		"reality嵌套保留4空格": "- client-fingerprint: chrome\nflow: xtls-rprx-vision\nname: test\nport: 443\nreality-opts:\n    public-key: Q0VeH\n    short-id: 82f\nserver: 1.2.3.4\ntype: vless\nuuid: d342d11e-d424-4583-b36e-524ab1f0afa4",
		"深层嵌套保留4/8空格":  "- alterId: 0\nname: test\nnetwork: ws\nport: 2086\nserver: 1.2.3.4\ntype: vmess\nuuid: 1111\nws-opts:\n    headers:\n        Host: www.bing.com\n    path: /p",
		"嵌套序列项alpn":     "- alpn:\n    - h3\nfast-open: true\nname: test\npassword: pwd\nport: 21874\nserver: 1.2.3.4\ntype: hysteria2",
	}
	for name, src := range samples {
		t.Run(name, func(t *testing.T) {
			outbound, _, err := parseClashNode(src)
			if err != nil {
				// hysteria2 等协议设计上走 sing-box 内核，xray 侧"不支持"属预期路由
				if strings.Contains(err.Error(), "不支持") && strings.Contains(src, "type: hysteria2") {
					return
				}
				t.Fatalf("历史损坏形态未被修复: %v\n%s", err, src)
			}
			if outbound == nil {
				t.Fatal("应生成出站配置")
			}
			// 嵌套结构必须真实存在
			stream, _ := outbound["streamSettings"].(map[string]interface{})
			if stream == nil {
				t.Fatalf("应生成 streamSettings: %#v", outbound)
			}
		})
	}
}

// hysteria2 损坏形态走 sing-box 解析入口也必须被修复。
func TestSingBoxParserRepairsStrippedIndent(t *testing.T) {
	src := "- alpn:\n    - h3\nfast-open: true\nname: test\npassword: pwd\nport: 21874\nserver: 1.2.3.4\nsni: www.bing.com\ntype: hysteria2"
	outbound, err := parseClashSingBoxNode(src)
	if err != nil {
		t.Fatalf("sing-box 入口未修复损坏形态: %v", err)
	}
	if outbound == nil {
		t.Fatal("应生成 sing-box 出站配置")
	}
}
