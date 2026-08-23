package proxy

import (
	"ant-chrome/backend/internal/apppath"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/fsutil"
	"ant-chrome/backend/internal/logger"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type MihomoNodeBridge struct {
	NodeKey        string
	Port           int
	ControllerPort int
	Cmd            *exec.Cmd
	Pid            int
	ConfigPath     string
	Running        bool
	RefCount       int
	LastUsedAt     time.Time
	ExitDone       chan struct{}
	ExitErr        error
}

func (m *ClashManager) EnsureNodeBridge(proxyConfig string, proxies []config.BrowserProxy, proxyId string) (string, error) {
	proxyURL, _, err := m.ensureNodeBridge(proxyConfig, proxies, proxyId, false)
	return proxyURL, err
}

func (m *ClashManager) AcquireNodeBridge(proxyConfig string, proxies []config.BrowserProxy, proxyId string) (string, string, error) {
	return m.ensureNodeBridge(proxyConfig, proxies, proxyId, true)
}

func (m *ClashManager) ReleaseNodeBridge(key string) {
	key = strings.TrimSpace(key)
	if key == "" || m == nil {
		return
	}

	var bridgeToStop *MihomoNodeBridge
	m.mu.Lock()
	bridge := m.NodeBridges[key]
	if bridge != nil {
		if bridge.RefCount > 0 {
			bridge.RefCount--
		}
		bridge.LastUsedAt = time.Now()
		if bridge.RefCount <= 0 {
			bridge.Running = false
			delete(m.NodeBridges, key)
			bridgeToStop = bridge
		}
	}
	m.mu.Unlock()

	if bridgeToStop != nil && bridgeToStop.Cmd != nil && bridgeToStop.Cmd.Process != nil {
		_ = bridgeToStop.Cmd.Process.Kill()
	}
}

func (m *ClashManager) ensureNodeBridge(proxyConfig string, proxies []config.BrowserProxy, proxyId string, pin bool) (string, string, error) {
	log := logger.New("Mihomo")
	src := strings.TrimSpace(resolveProxyConfig(proxyConfig, proxies, proxyId))
	if src == "" {
		return "", "", fmt.Errorf("未找到代理节点")
	}
	if strings.EqualFold(src, "direct://") {
		return "direct://", "", nil
	}

	key := computeNodeKey(src + "\x00mihomo")
	unlock := m.lockLaunchForKey(key)
	defer unlock()

	if proxyURL, reused := m.tryReuseMihomoNodeBridge(key, pin); reused {
		log.Info("复用 mihomo 桥接", logger.F("engine", "mihomo"), logger.F("key", key[:8]), logger.F("proxy_url", proxyURL))
		return proxyURL, key, nil
	}

	binaryPath, err := m.resolveMihomoBinary()
	if err != nil {
		return "", "", err
	}
	var nodes []interface{}
	targetName := ""
	preferredPort := 0
	if IsChainProxy(src) {
		chainCfg, err := ParseChainProxyConfig(src)
		if err != nil {
			return "", "", err
		}
		nodes, targetName, err = buildMihomoChainNodes(chainCfg, proxies, proxyId)
		if err != nil {
			return "", "", err
		}
		preferredPort = chainCfg.LocalPort
	} else {
		node, err := buildMihomoNode(src)
		if err != nil {
			return "", "", err
		}
		nodes = []interface{}{node}
		targetName = strings.TrimSpace(getMapString(node, "name"))
	}
	port := preferredPort
	if port <= 0 {
		var err error
		port, err = nextAvailablePort()
		if err != nil {
			return "", "", err
		}
	}
	controllerPort, err := nextAvailablePort()
	if err != nil {
		return "", "", err
	}
	cfgPath, err := m.buildMihomoNodesConfig(key, nodes, targetName, port, controllerPort)
	if err != nil {
		return "", "", err
	}

	cmd := exec.Command(binaryPath, "-f", cfgPath, "-d", filepath.Dir(cfgPath))
	hideWindow(cmd)
	cmd.Dir = filepath.Dir(cfgPath)
	stderrPath := filepath.Join(filepath.Dir(cfgPath), "mihomo-stderr.log")
	stderrFile, _ := os.Create(stderrPath)
	if stderrFile != nil {
		cmd.Stderr = stderrFile
	}
	if err := cmd.Start(); err != nil {
		if stderrFile != nil {
			stderrFile.Close()
		}
		return "", "", fmt.Errorf("mihomo 启动失败: %w", err)
	}
	bridge := &MihomoNodeBridge{NodeKey: key, Port: port, ControllerPort: controllerPort, Cmd: cmd, Pid: cmd.Process.Pid, ConfigPath: cfgPath, Running: true, LastUsedAt: time.Now(), ExitDone: make(chan struct{})}
	if pin {
		bridge.RefCount = 1
	}
	m.watchMihomoNodeBridge(bridge)
	if err := waitTCPPortReady("127.0.0.1", port, 10*time.Second); err != nil {
		if stderrFile != nil {
			stderrFile.Close()
		}
		_ = cmd.Process.Kill()
		return "", "", fmt.Errorf("mihomo mixed-port 未就绪: %w", err)
	}
	if err := waitTCPPortReady("127.0.0.1", controllerPort, 10*time.Second); err != nil {
		if stderrFile != nil {
			stderrFile.Close()
		}
		_ = cmd.Process.Kill()
		return "", "", fmt.Errorf("mihomo 控制端口未就绪: %w", err)
	}
	if stderrFile != nil {
		stderrFile.Close()
	}
	m.registerMihomoNodeBridge(key, bridge)
	log.Info("mihomo 内核进程已启动", logger.F("engine", "mihomo"), logger.F("key", key[:8]), logger.F("pid", bridge.Pid), logger.F("port", port))
	return fmt.Sprintf("http://127.0.0.1:%d", port), key, nil
}

func (m *ClashManager) tryReuseMihomoNodeBridge(key string, pin bool) (string, bool) {
	if m == nil {
		return "", false
	}
	m.mu.Lock()
	if m.NodeBridges == nil {
		m.NodeBridges = map[string]*MihomoNodeBridge{}
	}
	bridge := m.NodeBridges[key]
	if bridge == nil || !bridge.Running || bridge.Cmd == nil || bridge.Cmd.Process == nil || bridge.Cmd.ProcessState != nil {
		delete(m.NodeBridges, key)
		m.mu.Unlock()
		return "", false
	}
	m.mu.Unlock()
	if waitTCPPortReady("127.0.0.1", bridge.Port, 800*time.Millisecond) != nil {
		_ = bridge.Cmd.Process.Kill()
		m.mu.Lock()
		if m.NodeBridges[key] == bridge {
			bridge.Running = false
			delete(m.NodeBridges, key)
		}
		m.mu.Unlock()
		return "", false
	}
	m.mu.Lock()
	if m.NodeBridges[key] != bridge || !bridge.Running {
		m.mu.Unlock()
		return "", false
	}
	if pin {
		bridge.RefCount++
	}
	bridge.LastUsedAt = time.Now()
	m.mu.Unlock()
	return fmt.Sprintf("http://127.0.0.1:%d", bridge.Port), true
}

func (m *ClashManager) TestNodeDelay(proxyId string, proxies []config.BrowserProxy, cfg *SpeedTestConfig) TestResult {
	src := strings.TrimSpace(resolveProxyConfig("", proxies, proxyId))
	if src == "" {
		return TestResult{ProxyId: proxyId, Ok: false, Engine: "mihomo", Error: "代理配置为空"}
	}
	if strings.EqualFold(src, "direct://") {
		return TestResult{ProxyId: proxyId, Ok: true, LatencyMs: 0, Engine: "direct"}
	}
	if m == nil {
		return TestResult{ProxyId: proxyId, Ok: false, Engine: "mihomo", Error: "mihomo 管理器未初始化"}
	}
	if _, err := m.EnsureNodeBridge(src, proxies, proxyId); err != nil {
		return TestResult{ProxyId: proxyId, Ok: false, Engine: "mihomo", Error: err.Error()}
	}
	key := computeNodeKey(src + "\x00mihomo")
	m.mu.Lock()
	bridge := m.NodeBridges[key]
	m.mu.Unlock()
	if bridge == nil || !bridge.Running || bridge.ControllerPort <= 0 {
		return TestResult{ProxyId: proxyId, Ok: false, Engine: "mihomo", Error: "mihomo 控制端口未就绪"}
	}
	timeout := 10 * time.Second
	testURL := DefaultSpeedTestURL
	if cfg != nil {
		if cfg.Timeout > 0 {
			timeout = cfg.Timeout
		}
		if urls := normalizeSpeedTestURLs(cfg.URLs); len(urls) > 0 {
			testURL = urls[0]
		}
	}
	apiURL := fmt.Sprintf(
		"http://127.0.0.1:%d/proxies/%s/delay?timeout=%d&url=%s",
		bridge.ControllerPort,
		url.PathEscape("proxy-out"),
		int(timeout.Milliseconds()),
		url.QueryEscape(testURL),
	)
	client := &http.Client{Timeout: timeout + time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		return TestResult{ProxyId: proxyId, Ok: false, Engine: "mihomo", Error: "mihomo 延迟测试失败: " + err.Error()}
	}
	defer resp.Body.Close()
	var payload struct {
		Delay int64  `json:"delay"`
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return TestResult{ProxyId: proxyId, Ok: false, Engine: "mihomo", Error: "mihomo 延迟结果解析失败: " + err.Error()}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if payload.Error != "" {
			return TestResult{ProxyId: proxyId, Ok: false, Engine: "mihomo", Error: payload.Error}
		}
		return TestResult{ProxyId: proxyId, Ok: false, Engine: "mihomo", Error: fmt.Sprintf("mihomo 延迟测试失败: HTTP %d", resp.StatusCode)}
	}
	if payload.Delay <= 0 {
		if payload.Error != "" {
			return TestResult{ProxyId: proxyId, Ok: false, Engine: "mihomo", Error: payload.Error}
		}
		return TestResult{ProxyId: proxyId, Ok: false, Engine: "mihomo", Error: "mihomo 延迟测试无结果"}
	}
	return TestResult{ProxyId: proxyId, Ok: true, LatencyMs: payload.Delay, Engine: "mihomo"}
}

func (m *ClashManager) registerMihomoNodeBridge(key string, bridge *MihomoNodeBridge) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.NodeBridges == nil {
		m.NodeBridges = map[string]*MihomoNodeBridge{}
	}
	if old := m.NodeBridges[key]; old != nil && old.Cmd != nil && old.Cmd.Process != nil {
		old.Running = false
		_ = old.Cmd.Process.Kill()
	}
	m.NodeBridges[key] = bridge
}

func (m *ClashManager) watchMihomoNodeBridge(bridge *MihomoNodeBridge) {
	if m == nil || bridge == nil || bridge.Cmd == nil {
		return
	}
	go func() {
		err := bridge.Cmd.Wait()
		m.mu.Lock()
		bridge.Running = false
		bridge.ExitErr = err
		if current := m.NodeBridges[bridge.NodeKey]; current == bridge {
			delete(m.NodeBridges, bridge.NodeKey)
		}
		m.mu.Unlock()
		close(bridge.ExitDone)
	}()
}

func (m *ClashManager) lockLaunchForKey(key string) func() {
	if m == nil {
		return func() {}
	}
	m.mu.Lock()
	if m.launchLocks == nil {
		m.launchLocks = make(map[string]*bridgeLaunchLock)
	}
	lock := m.launchLocks[key]
	if lock == nil {
		lock = &bridgeLaunchLock{}
		m.launchLocks[key] = lock
	}
	lock.refs++
	m.mu.Unlock()

	lock.mu.Lock()
	return func() {
		lock.mu.Unlock()
		m.mu.Lock()
		lock.refs--
		if lock.refs <= 0 && m.launchLocks[key] == lock {
			delete(m.launchLocks, key)
		}
		m.mu.Unlock()
	}
}

func (m *ClashManager) buildMihomoNodeConfig(key string, node map[string]interface{}, port int, controllerPort int) (string, error) {
	return m.buildMihomoNodesConfig(key, []interface{}{node}, strings.TrimSpace(getMapString(node, "name")), port, controllerPort)
}

func (m *ClashManager) buildMihomoNodesConfig(key string, nodes []interface{}, targetName string, port int, controllerPort int) (string, error) {
	baseDir := m.resolveMihomoWorkdir(key)
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return "", err
	}
	if len(nodes) == 0 {
		return "", fmt.Errorf("mihomo 节点配置为空")
	}
	name := strings.TrimSpace(targetName)
	if name == "" {
		if node, ok := nodes[0].(map[string]interface{}); ok {
			name = strings.TrimSpace(getMapString(node, "name"))
			if name == "" {
				name = "node"
				node["name"] = name
			}
		}
	}
	payload := map[string]interface{}{
		"mixed-port":          port,
		"external-controller": fmt.Sprintf("127.0.0.1:%d", controllerPort),
		"allow-lan":           false,
		"mode":                "rule",
		"log-level":           "warning",
		"ipv6":                true,
		"unified-delay":       true,
		"tcp-concurrent":      false,
		"proxies":             nodes,
		"proxy-groups": []interface{}{
			map[string]interface{}{
				"name":    "proxy-out",
				"type":    "select",
				"proxies": []interface{}{name},
			},
		},
		"rules": []interface{}{"MATCH,proxy-out"},
	}
	cfgPath := filepath.Join(baseDir, "mihomo-config.yaml")
	data, err := yaml.Marshal(payload)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
		return "", err
	}
	return cfgPath, nil
}

func buildMihomoNode(src string) (map[string]interface{}, error) {
	var payload interface{}
	if err := yaml.Unmarshal([]byte(src), &payload); err == nil {
		if node := pickClashNode(payload); node != nil {
			return cloneStringInterfaceMap(node), nil
		}
	}
	mapping, err := proxyConfigToMapping(src)
	if err != nil {
		return nil, fmt.Errorf("mihomo 节点解析失败: %w", err)
	}
	out := map[string]interface{}{}
	for key, value := range mapping {
		out[key] = value
	}
	return out, nil
}

func (m *ClashManager) resolveMihomoBinary() (string, error) {
	if m == nil || m.Config == nil {
		return "", fmt.Errorf("mihomo 管理器未初始化")
	}
	candidates := []string{}
	if configured := strings.TrimSpace(m.Config.Browser.ClashBinaryPath); configured != "" {
		candidates = append(candidates, resolveEnvPath(configured, m.AppRoot))
	}
	for _, name := range []string{"mihomo.exe", "mihomo", "clash-meta.exe", "clash-meta", "clash.exe", "clash"} {
		if path, err := exec.LookPath(name); err == nil {
			candidates = append(candidates, path)
		}
	}
	if runtime.GOOS == "windows" {
		if appData := strings.TrimSpace(os.Getenv("APPDATA")); appData != "" {
			candidates = append(candidates,
				filepath.Join(appData, "mihomo-party", "mihomo.exe"),
				filepath.Join(appData, "mihomo-party", "core", "mihomo.exe"),
				filepath.Join(appData, "mihomo-party", "core", "mihomo-windows-amd64.exe"),
			)
		}
	}
	if m.AppRoot != "" {
		candidates = append(candidates,
			filepath.Join(m.AppRoot, "bin", "mihomo.exe"),
			filepath.Join(m.AppRoot, "bin", "mihomo"),
			filepath.Join(m.AppRoot, "bin", runtime.GOOS+"-"+runtime.GOARCH, "mihomo", "mihomo.exe"),
			filepath.Join(m.AppRoot, "bin", runtime.GOOS+"-"+runtime.GOARCH, "mihomo", "mihomo"),
			filepath.Join(m.AppRoot, "bin", runtime.GOOS+"-"+runtime.GOARCH, "mihomo.exe"),
			filepath.Join(m.AppRoot, "bin", runtime.GOOS+"-"+runtime.GOARCH, "mihomo"),
		)
	}
	for _, candidate := range candidates {
		candidate = fsutil.NormalizePathInput(candidate)
		if candidate == "" {
			continue
		}
		if !filepath.IsAbs(candidate) && m.AppRoot != "" {
			candidate = apppath.Resolve(m.AppRoot, candidate)
		}
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			if err := fsutil.EnsureExecutable(candidate); err != nil {
				return "", fmt.Errorf("mihomo 文件不可执行: %s: %w", candidate, err)
			}
			return candidate, nil
		}
	}
	return "", fmt.Errorf("mihomo/clash 可执行文件未找到，请配置 browser.clash_binary_path")
}

func (m *ClashManager) resolveMihomoWorkdir(key string) string {
	root := "data"
	if m != nil && m.Config != nil {
		root = strings.TrimSpace(m.Config.Browser.UserDataRoot)
		if root == "" {
			root = "data"
		}
	}
	if !filepath.IsAbs(root) && m != nil {
		root = apppath.Resolve(m.AppRoot, root)
	}
	return filepath.Join(root, "_mihomo", key)
}

func waitTCPPortReady(host string, port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	address := net.JoinHostPort(host, strconv.Itoa(port))
	for {
		conn, err := net.DialTimeout("tcp", address, 200*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		if time.Now().After(deadline) {
			return err
		}
		time.Sleep(100 * time.Millisecond)
	}
}
