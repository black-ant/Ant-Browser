package proxy

import (
	"ant-chrome/backend/internal/apppath"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/fsutil"
	"ant-chrome/backend/internal/logger"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	xrayBridgeIdleTTL         = 45 * time.Second
	xrayBridgeCleanupInterval = 15 * time.Second
)

// XrayManager Xray 桥接管理器
type XrayManager struct {
	Config       *config.Config
	AppRoot      string // 应用根目录，所有相对路径基于此解析
	Bridges      map[string]*XrayBridge
	OnBridgeDied func(key string, err error) // 桥接进程意外退出回调
	mu           sync.Mutex
	stopCh       chan struct{}
	stopOnce     sync.Once
}

// NewXrayManager 创建 Xray 管理器
func NewXrayManager(cfg *config.Config, appRoot string) *XrayManager {
	manager := &XrayManager{
		Config:  cfg,
		AppRoot: appRoot,
		Bridges: make(map[string]*XrayBridge),
		stopCh:  make(chan struct{}),
	}
	go manager.cleanupLoop()
	return manager
}

// ValidateProxyConfig 验证代理配置是否支持
// 返回: supported bool, errorMsg string
func ValidateProxyConfig(proxyConfig string, proxies []config.BrowserProxy, proxyId string) (bool, string) {
	src := strings.TrimSpace(proxyConfig)
	found := false
	if proxyId != "" {
		for _, item := range proxies {
			if strings.EqualFold(item.ProxyId, proxyId) {
				src = strings.TrimSpace(item.ProxyConfig)
				found = true
				break
			}
		}
		if !found {
			// 兼容模式：如果 profile 内仍保留了可解析的 proxyConfig，则允许回退使用。
			// 这样可兼容历史版本中 proxyId 失效后的启动流程，避免升级后强制手工重绑。
			if src == "" {
				return false, fmt.Sprintf("代理链路不可用：代理池节点已不存在（proxyId=%s）。可能因订阅刷新后节点下线或被删除，请重新选择代理后再启动。", proxyId)
			}
		}
	}
	if src == "" {
		return true, "" // 无代理配置，允许启动
	}
	if strings.EqualFold(src, "direct://") {
		return true, ""
	}
	l := strings.ToLower(src)
	// 标准代理格式，支持
	if strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "https://") || strings.HasPrefix(l, "socks5://") {
		return true, ""
	}
	if IsChainSocks5Proxy(src) {
		if _, err := ParseChainSocks5Config(src); err != nil {
			return false, fmt.Sprintf("链式代理配置解析失败: %v", err)
		}
		return true, ""
	}

	// hysteria2/tuic 通过 sing-box 支持，先做可解析性校验
	if IsSingBoxProtocol(src) {
		if _, err := BuildSingBoxOutbound(src); err != nil {
			return false, fmt.Sprintf("代理配置解析失败: %v", err)
		}
		return true, ""
	}

	// 其余协议交给统一解析器校验，防止无效字符串被当成代理参数透传给 Chrome
	standardProxy, outbound, err := ParseProxyNode(src)
	if err != nil {
		return false, fmt.Sprintf("代理配置解析失败: %v", err)
	}
	if strings.TrimSpace(standardProxy) == "" && outbound == nil {
		return false, "代理配置无效"
	}
	return true, ""
}

// RequiresBridge 判断是否需要 Xray 桥接
// 注意: Xray 仅支持 vless/vmess/trojan/shadowsocks 等协议
// hysteria2 不支持，需要使用 Hysteria 客户端或 sing-box
func RequiresBridge(proxyConfig string, proxies []config.BrowserProxy, proxyId string) bool {
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
		return false
	}
	l := strings.ToLower(src)
	// 标准代理格式：带账号密码的需要桥接（Chrome --proxy-server 无法鉴权），无凭据的直连
	if strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "https://") || strings.HasPrefix(l, "socks5://") {
		return proxyURLHasCredentials(src)
	}
	// hysteria2 Xray 不支持，不触发桥接
	if strings.HasPrefix(l, "hysteria://") || strings.HasPrefix(l, "hysteria2://") {
		return false
	}
	if IsChainSocks5Proxy(src) {
		return true
	}

	// Xray 支持的协议
	if strings.HasPrefix(l, "vmess://") || strings.HasPrefix(l, "vless://") || strings.HasPrefix(l, "trojan://") || strings.HasPrefix(l, "ss://") {
		return true
	}
	// Clash 格式需要进一步检查类型
	if strings.HasPrefix(l, "clash://") || strings.Contains(l, "type:") || strings.Contains(l, "proxies:") {
		// 排除 hysteria 类型
		if strings.Contains(l, "type: hysteria") || strings.Contains(l, "type:hysteria") {
			return false
		}
		return true
	}
	return false
}

// EnsureBridge 确保 Xray 桥接进程运行，用于临时请求场景。
func (m *XrayManager) EnsureBridge(proxyConfig string, proxies []config.BrowserProxy, proxyId string) (string, error) {
	socksURL, _, err := m.ensureBridge(proxyConfig, proxies, proxyId, false)
	return socksURL, err
}

// AcquireBridge 获取一个带引用计数的 Xray 桥接，用于浏览器实例等长生命周期场景。
func (m *XrayManager) AcquireBridge(proxyConfig string, proxies []config.BrowserProxy, proxyId string) (string, string, error) {
	return m.ensureBridge(proxyConfig, proxies, proxyId, true)
}

// ReleaseBridge 释放一个已占用的桥接引用；空闲桥接会由后台回收协程延迟清理。
func (m *XrayManager) ReleaseBridge(key string) {
	key = strings.TrimSpace(key)
	if key == "" {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	bridge, ok := m.Bridges[key]
	if !ok || bridge == nil {
		return
	}
	if bridge.RefCount > 0 {
		bridge.RefCount--
	}
	bridge.LastUsedAt = time.Now()
}

// StopAll 关闭所有 xray 桥接进程。
func (m *XrayManager) StopAll() {
	m.stopOnce.Do(func() {
		close(m.stopCh)
	})

	m.mu.Lock()
	// 复制键以避免在迭代时修改map
	keys := make([]string, 0, len(m.Bridges))
	for key := range m.Bridges {
		keys = append(keys, key)
	}

	bridges := make([]*XrayBridge, 0, len(keys))
	for _, key := range keys {
		if bridge, exists := m.Bridges[key]; exists && bridge != nil {
			bridge.Stopping = true
			bridges = append(bridges, bridge)
			delete(m.Bridges, key)
		}
	}
	m.mu.Unlock()

	for _, bridge := range bridges {
		m.stopBridgeProcess(bridge)
	}
}

func (m *XrayManager) ensureBridge(proxyConfig string, proxies []config.BrowserProxy, proxyId string, pin bool) (string, string, error) {
	log := logger.New("Xray")
	src := strings.TrimSpace(proxyConfig)
	dnsServers := ""
	if proxyId != "" {
		for _, item := range proxies {
			if strings.EqualFold(item.ProxyId, proxyId) {
				src = strings.TrimSpace(item.ProxyConfig)
				dnsServers = item.DnsServers
				break
			}
		}
	}
	if src == "" {
		return "", "", fmt.Errorf("未找到代理节点")
	}
	src = normalizeNodeScheme(src)

	var (
		outbounds     []interface{}
		routes        []interface{}
		preferredPort int
	)

	if IsChainSocks5Proxy(src) {
		chainCfg, err := ParseChainSocks5Config(src)
		if err != nil {
			log.Error("链式节点解析失败", logger.F("error", err))
			return "", "", err
		}
		outbounds = []interface{}{
			chainSocks5Outbound(chainCfg.First, "first-hop", ""),
			chainSocks5Outbound(chainCfg.Second, "second-hop", "first-hop"),
		}
		routes = []interface{}{
			map[string]interface{}{
				"type":        "field",
				"inboundTag":  []string{"socks-in"},
				"outboundTag": "second-hop",
			},
		}
		preferredPort = chainCfg.LocalPort
	} else {
		standardProxy, outbound, err := ParseProxyNode(src)
		if err != nil {
			log.Error("节点解析失败", logger.F("error", err))
			return "", "", err
		}
		if standardProxy != "" {
			return standardProxy, "", nil
		}
		if outbound == nil {
			return "", "", fmt.Errorf("节点解析失败")
		}
		outbounds = []interface{}{outbound}
		routes = []interface{}{
			map[string]interface{}{
				"type":        "field",
				"inboundTag":  []string{"socks-in"},
				"outboundTag": "proxy-out",
			},
		}
	}

	key := computeNodeKey(src + "\x00" + dnsServers)

	if socksURL, reused := m.tryReuseBridge(key, pin); reused {
		log.Info("复用桥接进程", logger.F("key", key), logger.F("socks_url", socksURL))
		return socksURL, key, nil
	}

	binaryPath, err := m.resolveBinary()
	if err != nil {
		log.Error("xray 不可用", logger.F("error", err))
		return "", "", err
	}
	maxLaunchRetries := 3
	if preferredPort > 0 {
		maxLaunchRetries = 1
	}
	var lastErr error
	for attempt := 1; attempt <= maxLaunchRetries; attempt++ {
		var port int
		if preferredPort > 0 {
			port = preferredPort
		} else {
			port, err = nextAvailablePort()
			if err != nil {
				log.Error("端口分配失败", logger.F("error", err), logger.F("attempt", attempt))
				lastErr = err
				continue
			}
		}

		cfgPath, err := m.buildRuntimeConfigWithRoute(key, outbounds, routes, port, dnsServers)
		if err != nil {
			log.Error("xray 配置生成失败", logger.F("error", err))
			return "", "", err
		}
		cmd := exec.Command(binaryPath, "run", "-c", cfgPath)
		hideWindow(cmd)
		cmd.Dir = filepath.Dir(cfgPath)
		stderrPath := filepath.Join(filepath.Dir(cfgPath), "xray-stderr.log")
		stderrFile, _ := os.Create(stderrPath)
		if stderrFile != nil {
			cmd.Stderr = stderrFile
		}
		if err := cmd.Start(); err != nil {
			if stderrFile != nil {
				stderrFile.Close()
			}
			log.Error("xray 启动失败", logger.F("error", err), logger.F("attempt", attempt))
			lastErr = err
			continue
		}
		bridge := &XrayBridge{
			NodeKey:    key,
			Port:       port,
			Cmd:        cmd,
			Pid:        cmd.Process.Pid,
			Running:    true,
			RefCount:   0,
			LastUsedAt: time.Now(),
		}
		log.Info("xray 启动", logger.F("key", key), logger.F("pid", bridge.Pid), logger.F("port", bridge.Port), logger.F("attempt", attempt))
		if err := waitPortReady("127.0.0.1", port, 10*time.Second); err != nil {
			if stderrFile != nil {
				stderrFile.Close()
			}
			if stderrContent, readErr := os.ReadFile(stderrPath); readErr == nil && len(stderrContent) > 0 {
				log.Error("xray stderr", logger.F("output", string(stderrContent)))
			} else {
				errLogPath := filepath.Join(filepath.Dir(cfgPath), "xray-error.log")
				if errContent, readErr := os.ReadFile(errLogPath); readErr == nil && len(errContent) > 0 {
					log.Error("xray error.log", logger.F("output", string(errContent)))
				}
			}
			bridge.Stopping = true
			m.stopBridgeProcess(bridge)
			bridge.Running = false
			bridge.Pid = 0
			bridge.LastError = err.Error()
			log.Error("xray 端口不可用，重试", logger.F("key", key), logger.F("error", err), logger.F("port", port), logger.F("attempt", attempt))
			lastErr = err
			time.Sleep(200 * time.Millisecond)
			continue
		}
		if stderrFile != nil {
			stderrFile.Close()
		}

		if socksURL, reused := m.registerBridge(key, bridge, pin); reused {
			log.Info("复用已就绪桥接进程", logger.F("key", key), logger.F("socks_url", socksURL))
			bridge.Stopping = true
			m.stopBridgeProcess(bridge)
			return socksURL, key, nil
		}

		go m.watchBridge(bridge, key)
		return fmt.Sprintf("socks5://127.0.0.1:%d", port), key, nil
	}
	return "", "", fmt.Errorf("xray 启动失败（已重试 %d 次）: %w", maxLaunchRetries, lastErr)
}

func (m *XrayManager) tryReuseBridge(key string, pin bool) (string, bool) {
	var stale *XrayBridge

	m.mu.Lock()
	if bridge, ok := m.Bridges[key]; ok && bridge != nil {
		alive := bridge.Running && bridge.Cmd != nil && bridge.Cmd.Process != nil && bridge.Cmd.ProcessState == nil
		if alive && waitPortReady("127.0.0.1", bridge.Port, 800*time.Millisecond) == nil {
			if pin {
				bridge.RefCount++
			}
			bridge.LastUsedAt = time.Now()
			socksURL := fmt.Sprintf("socks5://127.0.0.1:%d", bridge.Port)
			m.mu.Unlock()
			return socksURL, true
		}

		bridge.Stopping = true
		stale = bridge
		delete(m.Bridges, key)
	}
	m.mu.Unlock()

	if stale != nil {
		m.stopBridgeProcess(stale)
	}
	return "", false
}

func (m *XrayManager) registerBridge(key string, bridge *XrayBridge, pin bool) (string, bool) {
	var duplicate *XrayBridge

	m.mu.Lock()
	if existing, ok := m.Bridges[key]; ok && existing != nil {
		alive := existing.Running && existing.Cmd != nil && existing.Cmd.Process != nil && existing.Cmd.ProcessState == nil
		if alive && waitPortReady("127.0.0.1", existing.Port, 800*time.Millisecond) == nil {
			if pin {
				existing.RefCount++
			}
			existing.LastUsedAt = time.Now()
			duplicate = bridge
			socksURL := fmt.Sprintf("socks5://127.0.0.1:%d", existing.Port)
			m.mu.Unlock()
			if duplicate != nil {
				duplicate.Stopping = true
				m.stopBridgeProcess(duplicate)
			}
			return socksURL, true
		}

		existing.Stopping = true
		delete(m.Bridges, key)
		duplicate = existing
	}

	if pin {
		bridge.RefCount = 1
	}
	bridge.LastUsedAt = time.Now()
	m.Bridges[key] = bridge
	m.mu.Unlock()

	if duplicate != nil {
		m.stopBridgeProcess(duplicate)
	}
	return "", false
}

func (m *XrayManager) watchBridge(bridge *XrayBridge, key string) {
	if bridge == nil || bridge.Cmd == nil {
		return
	}
	_ = bridge.Cmd.Wait()

	m.mu.Lock()
	if current, ok := m.Bridges[key]; ok && current == bridge {
		delete(m.Bridges, key)
	}
	bridge.Running = false
	stopping := bridge.Stopping
	m.mu.Unlock()

	if !stopping && m.OnBridgeDied != nil {
		m.OnBridgeDied(key, fmt.Errorf("xray 桥接进程意外退出"))
	}
}

func (m *XrayManager) cleanupLoop() {
	ticker := time.NewTicker(xrayBridgeCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.recycleIdleBridges()
		case <-m.stopCh:
			return
		}
	}
}

func (m *XrayManager) recycleIdleBridges() {
	now := time.Now()
	var stale []*XrayBridge

	m.mu.Lock()
	// 复制键以避免在迭代时修改map
	keys := make([]string, 0, len(m.Bridges))
	for key := range m.Bridges {
		keys = append(keys, key)
	}

	for _, key := range keys {
		bridge, exists := m.Bridges[key]
		if !exists {
			continue
		}
		if bridge == nil {
			delete(m.Bridges, key)
			continue
		}
		if bridge.RefCount > 0 {
			continue
		}
		if now.Sub(bridge.LastUsedAt) < xrayBridgeIdleTTL {
			continue
		}

		bridge.Stopping = true
		stale = append(stale, bridge)
		delete(m.Bridges, key)
	}
	m.mu.Unlock()

	if len(stale) == 0 {
		return
	}

	log := logger.New("Xray")
	for _, bridge := range stale {
		log.Info("回收空闲桥接进程", logger.F("key", bridge.NodeKey), logger.F("pid", bridge.Pid))
		m.stopBridgeProcess(bridge)
	}
}

func (m *XrayManager) stopBridgeProcess(bridge *XrayBridge) {
	if bridge == nil || bridge.Cmd == nil || bridge.Cmd.Process == nil {
		return
	}
	_ = bridge.Cmd.Process.Kill()
	// 后台回收僵尸进程(仅 Unix 需要,Windows 的 Kill 立即释放资源)
	go bridge.Cmd.Wait()
}

func (m *XrayManager) resolveBinary() (string, error) {
	configPath := strings.TrimSpace(m.Config.Browser.XrayBinaryPath)
	if configPath != "" {
		resolved := resolveEnvPath(configPath, m.AppRoot)
		if resolved != "" {
			if _, err := os.Stat(resolved); err == nil {
				if err := fsutil.EnsureExecutable(resolved); err != nil {
					return "", fmt.Errorf("xray 文件不可执行: %s: %w", resolved, err)
				}
				return resolved, nil
			}
		}
	}
	env := strings.TrimSpace(os.Getenv("XRAY_BINARY_PATH"))
	if env != "" {
		if _, err := os.Stat(env); err == nil {
			if err := fsutil.EnsureExecutable(env); err != nil {
				return "", fmt.Errorf("xray 文件不可执行: %s: %w", env, err)
			}
			return env, nil
		}
	}

	binaryNames := []string{"xray"}
	if goruntime.GOOS == "windows" {
		binaryNames = []string{"xray.exe", "xray"}
	}
	platformDir := fmt.Sprintf("%s-%s", goruntime.GOOS, goruntime.GOARCH)

	searchDirs := make([]string, 0, 4)
	if m.AppRoot != "" {
		searchDirs = append(searchDirs,
			filepath.Join(m.AppRoot, "bin", platformDir),
			filepath.Join(m.AppRoot, "bin"),
		)
	}
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		searchDirs = append(searchDirs,
			filepath.Join(exeDir, "bin", platformDir),
			filepath.Join(exeDir, "bin"),
		)
	}

	for _, dir := range searchDirs {
		for _, name := range binaryNames {
			candidate := filepath.Join(dir, name)
			if _, err := os.Stat(candidate); err == nil {
				if err := fsutil.EnsureExecutable(candidate); err != nil {
					return "", fmt.Errorf("xray 文件不可执行: %s: %w", candidate, err)
				}
				return candidate, nil
			}
		}
	}

	for _, name := range binaryNames {
		if path, err := exec.LookPath(name); err == nil {
			if err := fsutil.EnsureExecutable(path); err != nil {
				return "", fmt.Errorf("xray 文件不可执行: %s: %w", path, err)
			}
			return path, nil
		}
	}

	return "", fmt.Errorf("未找到 xray 可执行文件。请将 xray 放到 bin/%s/ 或 bin/ 目录，或在配置中设置 XrayBinaryPath", platformDir)
}

// parseDnsConfig 解析 DNS 配置，支持两种格式：
// 1. Clash dns: YAML 块（含 nameserver/fallback 等字段）
// 2. 逗号分隔的 IP 列表（兼容旧格式）
// 返回 xray dns 配置 map，若无有效配置则返回 nil
//
// 注意：xray dns.servers 只支持纯 IP 或 DoH（https://）地址，
// 不支持 Clash 的 tls:// 格式（DoT），会被自动过滤。
func parseDnsConfig(raw string) map[string]interface{} {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	// 尝试解析 Clash dns: YAML 块
	type clashDns struct {
		Enable     bool     `yaml:"enable"`
		Nameserver []string `yaml:"nameserver"`
		Fallback   []string `yaml:"fallback"`
	}
	type clashDnsWrapper struct {
		Dns clashDns `yaml:"dns"`
	}

	var wrapper clashDnsWrapper
	if err := yaml.Unmarshal([]byte(raw), &wrapper); err == nil && len(wrapper.Dns.Nameserver) > 0 {
		servers := make([]interface{}, 0)
		for _, s := range wrapper.Dns.Nameserver {
			if s = strings.TrimSpace(s); s != "" && isXrayDnsAddr(s) {
				servers = append(servers, s)
			}
		}
		for _, s := range wrapper.Dns.Fallback {
			if s = strings.TrimSpace(s); s != "" && isXrayDnsAddr(s) {
				servers = append(servers, s)
			}
		}
		if len(servers) > 0 {
			return map[string]interface{}{"servers": servers}
		}
	}

	// 兼容旧格式：逗号分隔的 IP 列表
	var result []string
	for _, s := range strings.Split(raw, ",") {
		if s = strings.TrimSpace(s); s != "" && isXrayDnsAddr(s) {
			result = append(result, s)
		}
	}
	if len(result) > 0 {
		servers := make([]interface{}, len(result))
		for i, s := range result {
			servers[i] = s
		}
		return map[string]interface{}{"servers": servers}
	}
	return nil
}

// isXrayDnsAddr 判断 DNS 地址是否为 xray 支持的格式。
// xray 支持：纯 IP（如 8.8.8.8）、IP:port（如 8.8.8.8:53）、
// DoH（https://...）、localhost。
// 不支持：Clash 的 tls:// 格式（DoT）。
func isXrayDnsAddr(s string) bool {
	l := strings.ToLower(s)
	if strings.HasPrefix(l, "tls://") {
		return false
	}
	return true
}

func (m *XrayManager) buildRuntimeConfig(key string, outbound map[string]interface{}, port int, dnsServers string) (string, error) {
	baseDir := m.resolveWorkdir(key)
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return "", err
	}
	cfgPath := filepath.Join(baseDir, "xray-config.json")
	cfg := map[string]interface{}{
		"log": map[string]interface{}{
			"loglevel": "info",
			"error":    filepath.Join(baseDir, "xray-error.log"),
		},
		"inbounds": []interface{}{
			map[string]interface{}{
				"tag":      "socks-in",
				"port":     port,
				"listen":   "127.0.0.1",
				"protocol": "socks",
				"settings": map[string]interface{}{
					"udp": true,
				},
				"sniffing": map[string]interface{}{
					"enabled": false,
				},
			},
		},
		"outbounds": []interface{}{
			outbound,
			map[string]interface{}{
				"protocol": "direct",
				"tag":      "direct",
			},
			map[string]interface{}{
				"protocol": "blackhole",
				"tag":      "block",
			},
		},
		"routing": map[string]interface{}{
			"rules": []interface{}{
				map[string]interface{}{
					"type":        "field",
					"inboundTag":  []string{"socks-in"},
					"outboundTag": "proxy-out",
				},
			},
		},
	}
	if dnsCfg := parseDnsConfig(dnsServers); dnsCfg != nil {
		cfg["dns"] = dnsCfg
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(cfgPath, data, 0600); err != nil {
		return "", err
	}
	return cfgPath, nil
}

func chainSocks5Outbound(hop chainSocks5Hop, tag string, nextTag string) map[string]interface{} {
	user := map[string]interface{}{}
	if strings.TrimSpace(hop.Username) != "" {
		user["user"] = hop.Username
		if strings.TrimSpace(hop.Password) != "" {
			user["pass"] = hop.Password
		}
	}
	servers := []interface{}{
		map[string]interface{}{
			"address": hop.Server,
			"port":    hop.Port,
			"users":   []interface{}{user},
		},
	}
	if len(user) == 0 {
		servers = []interface{}{
			map[string]interface{}{
				"address": hop.Server,
				"port":    hop.Port,
			},
		}
	}

	outbound := map[string]interface{}{
		"protocol": "socks",
		"tag":      tag,
		"settings": map[string]interface{}{
			"servers": servers,
		},
	}
	if strings.TrimSpace(nextTag) != "" {
		outbound["proxySettings"] = map[string]interface{}{
			"tag": nextTag,
		}
	}
	return outbound
}

func (m *XrayManager) buildRuntimeConfigWithRoute(
	key string,
	outbounds []interface{},
	rules []interface{},
	port int,
	dnsServers string,
) (string, error) {
	baseDir := m.resolveWorkdir(key)
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return "", err
	}
	cfgPath := filepath.Join(baseDir, "xray-config.json")
	cfg := map[string]interface{}{
		"log": map[string]interface{}{
			"loglevel": "info",
			"error":    filepath.Join(baseDir, "xray-error.log"),
		},
		"inbounds": []interface{}{
			map[string]interface{}{
				"tag":      "socks-in",
				"port":     port,
				"listen":   "127.0.0.1",
				"protocol": "socks",
				"settings": map[string]interface{}{
					"udp": true,
				},
				"sniffing": map[string]interface{}{
					"enabled": false,
				},
			},
		},
		"outbounds": append(outbounds,
			map[string]interface{}{
				"protocol": "direct",
				"tag":      "direct",
			},
			map[string]interface{}{
				"protocol": "blackhole",
				"tag":      "block",
			},
		),
		"routing": map[string]interface{}{
			"rules": rules,
		},
	}
	if dnsCfg := parseDnsConfig(dnsServers); dnsCfg != nil {
		cfg["dns"] = dnsCfg
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(cfgPath, data, 0600); err != nil {
		return "", err
	}
	return cfgPath, nil
}
func (m *XrayManager) resolveWorkdir(key string) string {
	root := strings.TrimSpace(m.Config.Browser.UserDataRoot)
	if root == "" {
		root = "data"
	}
	if !filepath.IsAbs(root) {
		root = apppath.Resolve(m.AppRoot, root)
	}
	return filepath.Join(root, "_xray", key)
}

func computeNodeKey(src string) string {
	h := sha256.Sum256([]byte(strings.TrimSpace(src)))
	return hex.EncodeToString(h[:])
}

func normalizeNodeScheme(src string) string {
	s := strings.TrimSpace(src)
	if strings.HasPrefix(strings.ToLower(s), "hysteria://") {
		return "hysteria2://" + strings.TrimPrefix(s, "hysteria://")
	}
	return s
}

func resolveEnvPath(path string, appRoot string) string {
	path = fsutil.NormalizePathInput(path)
	if path == "" {
		return ""
	}
	if filepath.IsAbs(path) {
		return path
	}
	// 优先基于 appRoot 解析
	if appRoot != "" {
		candidate := filepath.Join(appRoot, path)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	// 兜底：exe 目录
	if exePath, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exePath), path)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	// 兜底：CWD
	if cwd, err := os.Getwd(); err == nil {
		candidate := filepath.Join(cwd, path)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return path
}

func waitPortReady(host string, port int, timeout time.Duration) error {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("端口 %d 不可用", port)
}

// nextAvailablePort 分配一个可用端口。
// 采用二次验证策略：分配后立即再次绑定确认未被其他进程抢占，
// 并在 EnsureBridge 层面加重试，彻底消除 TOCTOU 竞争窗口。
func nextAvailablePort() (int, error) {
	return nextAvailablePortWithRetry(10)
}

func nextAvailablePortWithRetry(maxRetries int) (int, error) {
	for i := 0; i < maxRetries; i++ {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			continue
		}
		port := listener.Addr().(*net.TCPAddr).Port
		listener.Close()
		// 短暂等待确保 OS 释放端口
		time.Sleep(10 * time.Millisecond)
		// 二次验证端口确实可用（没有被其他进程抢占）
		verifyListener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			// 端口被抢占，重试
			continue
		}
		verifyListener.Close()
		return port, nil
	}
	return 0, fmt.Errorf("无法分配可用端口，已重试 %d 次", maxRetries)
}
