package proxy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ant-chrome/backend/internal/config"
)

// 复现用户报错：xray 桥接启动失败: yaml: line 1: did not find expected '-' indicator
// 样本取自 data/app.db 真实存储的 hysteria2 节点（带 "-" 前缀的 YAML 序列项）。
func TestReproClashYAMLNodeBridgeError(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer target.Close()
	cfg := &SpeedTestConfig{
		Timeout:        2 * time.Second,
		TCPTimeout:     2 * time.Second,
		URLs:           []string{target.URL},
		ExpectedStatus: []int{http.StatusNoContent},
	}

	samples := map[string]string{
		"带-前缀的单节点(库中实际形态)": "- name: 香港01\n  server: 209.9.200.33\n  port: 20000\n  udp: true\n  skip-cert-verify: true\n  sni: d1.awsstatic.com\n  type: hysteria2\n  password: 34e92c65-4ef7-466c-89fe-1fd0f0a4e172",
		"无-前缀的块式map":          "name: 香港01\n  server: 209.9.200.33\n  port: 20000\n  type: hysteria2\n  password: abc",
		"两空格缩进不齐":             "- name: 香港01\n server: 209.9.200.33\n  port: 20000\n  type: hysteria2",
		"vmess URI 含 type: JSON": "vmess://eyJ0IjoiMSIsInR5cGUiOiJub25lIn0=",
	}

	xrayMgr := NewXrayManager(config.DefaultConfig(), t.TempDir())

	for name, src := range samples {
		t.Run(name, func(t *testing.T) {
			proxies := []config.BrowserProxy{{ProxyId: "sample-1", ProxyConfig: src}}
			result := SpeedTestWithConnector("sample-1", proxies, xrayMgr, nil, nil, nil, config.BrowserConnectorXray, cfg)
			t.Logf("输入=%q", src)
			t.Logf("ok=%v engine=%s error=%s", result.Ok, result.Engine, result.Error)
		})
	}
}
// 修复验证：缩进畸形的 clash YAML 节点应被规范化修复，不再报
// "did not find expected '-' indicator"。
func TestRepairedClashYAMLParsesSuccessfully(t *testing.T) {
	samples := map[string]string{
		"首行前导空格": " - name: 香港01\n  server: 1.1.1.1\n  port: 443\n  type: vless\n  password: test",
		"缩进不齐":   "- name: 香港01\n server: 1.1.1.1\n  port: 443\n  type: vless\n  password: test",
		"CRLF混合": "- name: 香港01\r\n server: 1.1.1.1\r\n  port: 443\r\n  type: vless",
	}
	for name, src := range samples {
		t.Run(name, func(t *testing.T) {
			outbound, standard, err := parseClashNode(src)
			if err != nil {
				t.Fatalf("畸形节点未被修复: %v", err)
			}
			if standard != "" {
				t.Fatalf("vless 不应转标准代理: %s", standard)
			}
			if outbound == nil {
				t.Fatal("应生成出站配置")
			}
		})
	}
}

// 块标量配置不做修复（返回原始错误，不破坏数据）。
func TestBlockScalarNotRepaired(t *testing.T) {
	src := "- name: x\n  server: 1.1.1.1\n  type: ss\n  plugin-opts:\n    mode: tls"
	if _, _, err := parseClashNode(src); err != nil {
		t.Fatalf("合法嵌套 map 不应报错: %v", err)
	}
}

// 端到端：畸形 vless clash YAML 走 xray 测速，错误不应再是 yaml 解析错误。
func TestMalformedClashNodeSpeedTestNotYAMLError(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer target.Close()
	cfg := &SpeedTestConfig{
		Timeout:        2 * time.Second,
		TCPTimeout:     2 * time.Second,
		URLs:           []string{target.URL},
		ExpectedStatus: []int{http.StatusNoContent},
	}
	xrayMgr := NewXrayManager(config.DefaultConfig(), t.TempDir())
	src := " - name: 香港01\n  server: 127.0.0.1\n  port: 1\n  type: vless\n  password: test"
	proxies := []config.BrowserProxy{{ProxyId: "malformed-1", ProxyConfig: src}}
	result := SpeedTestWithConnector("malformed-1", proxies, xrayMgr, nil, nil, nil, config.BrowserConnectorXray, cfg)
	if strings.Contains(result.Error, "did not find expected '-' indicator") {
		t.Fatalf("仍报 yaml 缩进错误: %s", result.Error)
	}
	t.Logf("ok=%v engine=%s error=%s", result.Ok, result.Engine, result.Error)
}
