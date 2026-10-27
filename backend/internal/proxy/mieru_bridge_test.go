package proxy

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"ant-chrome/backend/internal/config"
)

// mieru 节点解析：clash YAML → 节点 map（含缩进损坏形态修复）。
func TestParseMieruClashNode(t *testing.T) {
	samples := map[string]string{
		"标准形态": "- name: HK01\n  server: b.woyoufan.cn\n  port: 40001\n  transport: TCP\n  type: mieru\n  udp: true\n  username: u1\n  password: p1\n  multiplexing: MULTIPLEXING_LOW\n  handshake-mode: HANDSHAKE_STANDARD",
		"损坏缩进": "- name: HK01\nserver: b.woyoufan.cn\nport: 40001\ntransport: TCP\ntype: mieru\nusername: u1\npassword: p1",
	}
	for name, src := range samples {
		t.Run(name, func(t *testing.T) {
			node, err := ParseMieruClashNode(src)
			if err != nil {
				t.Fatalf("解析失败: %v", err)
			}
			if getMapString(node, "server") != "b.woyoufan.cn" || getMapInt(node, "port") != 40001 {
				t.Fatalf("字段错误: %#v", node)
			}
		})
	}

	if _, err := ParseMieruClashNode("- name: x\n  type: vless\n  server: 1.1.1.1\n  port: 443"); err == nil {
		t.Fatal("非 mieru 类型应报错")
	}
	if _, err := ParseMieruClashNode("- name: x\n  type: mieru\n  port: 443"); err == nil {
		t.Fatal("缺 server 应报错")
	}
}

// mieru 客户端配置生成：字段映射正确性。
func TestBuildMieruClientConfig(t *testing.T) {
	node := map[string]interface{}{
		"server":         "b.woyoufan.cn",
		"port":           40001,
		"transport":      "TCP",
		"type":           "mieru",
		"username":       "u1",
		"password":       "p1",
		"multiplexing":   "MULTIPLEXING_LOW",
		"handshake-mode": "HANDSHAKE_STANDARD",
	}
	cfg, err := buildMieruClientConfig(node, 51001, 51002)
	if err != nil {
		t.Fatalf("生成失败: %v", err)
	}
	profiles := cfg["profiles"].([]interface{})
	profile := profiles[0].(map[string]interface{})
	server := profile["servers"].([]interface{})[0].(map[string]interface{})

	if _, ok := server["domainName"]; !ok || server["domainName"] != "b.woyoufan.cn" {
		t.Fatalf("域名应写入 domainName: %#v", server)
	}
	bindings := server["portBindings"].([]interface{})
	binding := bindings[0].(map[string]interface{})
	if binding["port"] != 40001 || binding["protocol"] != "TCP" {
		t.Fatalf("端口绑定错误: %#v", binding)
	}
	user := profile["user"].(map[string]interface{})
	if user["name"] != "u1" || user["password"] != "p1" {
		t.Fatalf("用户信息错误: %#v", user)
	}
	if profile["multiplexing"].(map[string]interface{})["level"] != "MULTIPLEXING_LOW" {
		t.Fatal("multiplexing 未透传")
	}
	if profile["handshakeMode"] != "HANDSHAKE_STANDARD" {
		t.Fatal("handshakeMode 未透传")
	}
	if cfg["socks5Port"] != 51001 || cfg["rpcPort"] != 51002 {
		t.Fatal("端口注入错误")
	}

	// IP 服务器走 ipAddress
	ipNode := map[string]interface{}{"server": "1.2.3.4", "port": 8964, "type": "mieru"}
	ipCfg, err := buildMieruClientConfig(ipNode, 51001, 51002)
	if err != nil {
		t.Fatalf("IP 节点生成失败: %v", err)
	}
	ipServer := ipCfg["profiles"].([]interface{})[0].(map[string]interface{})["servers"].([]interface{})[0].(map[string]interface{})
	if ipServer["ipAddress"] != "1.2.3.4" {
		t.Fatalf("IP 应写入 ipAddress: %#v", ipServer)
	}

	// port-range 形态
	rangeNode := map[string]interface{}{"server": "b.example.com", "port-range": "2012-2022", "type": "mieru"}
	rangeCfg, err := buildMieruClientConfig(rangeNode, 51001, 51002)
	if err != nil {
		t.Fatalf("port-range 节点生成失败: %v", err)
	}
	rangeBinding := rangeCfg["profiles"].([]interface{})[0].(map[string]interface{})["servers"].([]interface{})[0].(map[string]interface{})["portBindings"].([]interface{})[0].(map[string]interface{})
	if rangeBinding["portRange"] != "2012-2022" {
		t.Fatalf("portRange 错误: %#v", rangeBinding)
	}
}

// 内核路由：mieru 协议应选择 mieru 内核（两栈均可用）。
func TestMieruKernelResolution(t *testing.T) {
	src := "- name: HK01\n  server: b.woyoufan.cn\n  port: 40001\n  type: mieru\n  username: u1\n  password: p1"
	for _, connector := range []string{"xray", "mihomo"} {
		resolution, err := ResolveProxyKernelForConnector(src, nil, "", connector)
		if err != nil {
			t.Fatalf("连接栈 %s 路由失败: %v", connector, err)
		}
		if resolution.Kernel != ProxyKernelMieru {
			t.Fatalf("连接栈 %s 应路由 mieru 内核，实际 %s", connector, resolution.Kernel)
		}
		if resolution.Protocol != "mieru" {
			t.Fatalf("协议识别错误: %s", resolution.Protocol)
		}
	}
}

// 端到端（真实内核）：mieru 桥接启动 → socks5 就绪 → 测速链路完整执行。
func TestMieruBridgeEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	// 查找本地 mieru 二进制：环境变量优先，其次常见下载位置
	candidates := []string{
		os.Getenv("MIERU_BINARY_PATH"),
		`D:\tmp-mieru\mieru.exe`,
		filepath.Join("..", "..", "..", "bin", "mieru.exe"),
	}
	abs := ""
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, statErr := os.Stat(candidate); statErr == nil {
			if resolved, resolveErr := filepath.Abs(candidate); resolveErr == nil {
				abs = resolved
				break
			}
		}
	}
	if abs == "" {
		t.Skipf("本机无 mieru.exe（查找了 %v），跳过真实内核端到端", candidates)
	}
	t.Setenv(mieruKernelEnvBinary, abs)

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer target.Close()

	mgr := NewMieruManager(config.DefaultConfig(), t.TempDir())
	defer mgr.Stop()

	proxies := []config.BrowserProxy{{
		ProxyId:     "mieru-e2e",
		ProxyConfig: "- name: HK01\n  server: 127.0.0.1\n  port: 1\n  transport: TCP\n  type: mieru\n  username: u1\n  password: p1",
	}}
	speedConfig := &SpeedTestConfig{
		Timeout:        3 * time.Second,
		TCPTimeout:     3 * time.Second,
		URLs:           []string{target.URL},
		ExpectedStatus: []int{http.StatusNoContent},
	}

	result := SpeedTestWithConnector("mieru-e2e", proxies, nil, nil, nil, mgr, config.BrowserConnectorXray, speedConfig)

	// 死端口上游预期失败，但必须已进入 mieru 桥接环节（而非解析/启动错误）
	if strings.Contains(result.Error, "yaml") || strings.Contains(result.Error, "未找到 mieru") {
		t.Fatalf("未进入 mieru 桥接环节: %s", result.Error)
	}
	if result.Engine != "mieru" {
		t.Fatalf("engine 应为 mieru: %s", result.Engine)
	}
	t.Logf("ok=%v engine=%s error=%s", result.Ok, result.Engine, result.Error)
	if result.Ok {
		t.Fatal("死端口上游不应测通")
	}
}

// 并发安全：同一节点多路并发启动时，启动锁应保证只起一个进程（不产生孤儿）。
func TestMieruConcurrentLaunchSameKey(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	abs := ""
	for _, candidate := range []string{os.Getenv("MIERU_BINARY_PATH"), `D:\tmp-mieru\mieru.exe`} {
		if candidate == "" {
			continue
		}
		if _, statErr := os.Stat(candidate); statErr == nil {
			resolved, _ := filepath.Abs(candidate)
			abs = resolved
			break
		}
	}
	if abs == "" {
		t.Skip("本机无 mieru.exe，跳过并发启动测试")
	}
	t.Setenv(mieruKernelEnvBinary, abs)

	mgr := NewMieruManager(config.DefaultConfig(), t.TempDir())
	defer mgr.Stop()

	node := "- name: HK01\n  server: 127.0.0.1\n  port: 1\n  transport: TCP\n  type: mieru\n  username: u1\n  password: p1"
	proxies := []config.BrowserProxy{{ProxyId: "same-key", ProxyConfig: node}}

	const workers = 5
	var wg sync.WaitGroup
	keys := make([]string, workers)
	errs := make([]error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, key, err := mgr.AcquireBridge(node, proxies, "same-key")
			keys[idx] = key
			errs[idx] = err
			if err == nil {
				mgr.ReleaseBridge(key)
			}
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("worker %d 启动失败: %v", i, err)
		}
	}
	for i := 1; i < workers; i++ {
		if keys[i] != keys[0] {
			t.Errorf("并发启动产生了不同 key: %q vs %q（可能存在孤儿进程）", keys[0], keys[i])
		}
	}
	mgr.mu.Lock()
	bridgeCount := len(mgr.Bridges)
	mgr.mu.Unlock()
	if bridgeCount != 1 {
		t.Errorf("并发后桥接数 = %d, want 1（同一节点只应存在一个桥接）", bridgeCount)
	}
}
