package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/fsutil"
	"ant-chrome/backend/internal/logger"
	"gopkg.in/yaml.v3"
)

// mieru 桥接：启动官方 mieru 客户端进程（mieru run，前台模式），
// 以本地 socks5 入口提供代理能力，用于测速与实例代理。
// 配置通过环境变量 MIERU_CONFIG_JSON_FILE 注入，每个桥接独立配置文件与端口。

const (
	mieruBridgeIdleTTL   = 30 * time.Second
	mieruBridgeStartTTL  = 8 * time.Second
	mieruBridgeStopTTL   = 3 * time.Second
	mieruKernelEnvBinary = "MIERU_BINARY_PATH"
	mieruConfigEnvVar    = "MIERU_CONFIG_JSON_FILE"
)

type MieruBridge struct {
	NodeKey    string
	SocksPort  int
	RpcPort    int
	Cmd        *exec.Cmd
	Pid        int
	Running    bool
	Stopping   bool
	RefCount   int
	LastUsedAt time.Time
	ExitDone   chan struct{}
	exitOnce   sync.Once
	ExitErr    error
	exitMu     sync.Mutex
}

type MieruManager struct {
	Config      *config.Config
	AppRoot     string
	Bridges     map[string]*MieruBridge
	mu          sync.Mutex
	launchLocks map[string]*bridgeLaunchLock
	stopCh      chan struct{}
	stopOnce    sync.Once
}

func NewMieruManager(cfg *config.Config, appRoot string) *MieruManager {
	manager := &MieruManager{
		Config:  cfg,
		AppRoot: appRoot,
		Bridges: make(map[string]*MieruBridge),
		stopCh:  make(chan struct{}),
	}
	go manager.cleanupLoop()
	return manager
}

// Stop 停止全部桥接与清理协程。
func (m *MieruManager) Stop() {
	if m == nil {
		return
	}
	m.stopOnce.Do(func() { close(m.stopCh) })
	m.mu.Lock()
	bridges := make([]*MieruBridge, 0, len(m.Bridges))
	for _, bridge := range m.Bridges {
		bridge.Stopping = true
		bridges = append(bridges, bridge)
	}
	m.Bridges = make(map[string]*MieruBridge)
	m.mu.Unlock()
	for _, bridge := range bridges {
		m.stopBridgeProcess(bridge)
	}
}

func (m *MieruManager) resolveBinary() (string, error) {
	if m == nil || m.Config == nil {
		return "", fmt.Errorf("mieru 管理器未初始化")
	}
	if configured := strings.TrimSpace(m.Config.Browser.MieruBinaryPath); configured != "" {
		if resolved := resolveEnvPath(configured, m.AppRoot); resolved != "" {
			if info, err := os.Stat(resolved); err == nil && !info.IsDir() {
				return resolved, nil
			}
		}
	}
	if env := strings.TrimSpace(os.Getenv(mieruKernelEnvBinary)); env != "" {
		if info, err := os.Stat(env); err == nil && !info.IsDir() {
			return env, nil
		}
	}

	binaryNames := []string{"mieru"}
	if runtime.GOOS == "windows" {
		binaryNames = []string{"mieru.exe", "mieru"}
	}
	platformDir := fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
	// 搜索顺序：下载框架的安装目录（bin/{platform}/mieru/，见 proxyCoreInstallDir）优先，
	// 兼容用户平铺放置（bin/{platform}/、bin/）与 exe 同目录。
	searchDirs := make([]string, 0, 6)
	if m.AppRoot != "" {
		searchDirs = append(searchDirs,
			filepath.Join(m.AppRoot, "bin", platformDir, "mieru"),
			filepath.Join(m.AppRoot, "bin", platformDir),
			filepath.Join(m.AppRoot, "bin"),
		)
	}
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		searchDirs = append(searchDirs,
			filepath.Join(exeDir, "bin", platformDir, "mieru"),
			filepath.Join(exeDir, "bin", platformDir),
			filepath.Join(exeDir, "bin"),
			exeDir,
		)
	}
	for _, dir := range searchDirs {
		for _, name := range binaryNames {
			candidate := filepath.Join(dir, name)
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				if err := fsutil.EnsureExecutable(candidate); err != nil {
					return "", fmt.Errorf("mieru 文件不可执行: %s: %w", candidate, err)
				}
				return candidate, nil
			}
		}
	}
	for _, name := range binaryNames {
		if path, err := exec.LookPath(name); err == nil {
			if err := fsutil.EnsureExecutable(path); err != nil {
				return "", fmt.Errorf("mieru 文件不可执行: %s: %w", path, err)
			}
			return path, nil
		}
	}
	return "", fmt.Errorf("未找到 mieru 可执行文件。请将 mieru 放到 bin/%s/mieru/ 或 bin/ 目录，或在配置中设置 MieruBinaryPath", platformDir)
}

// ─── 协议解析 ───

// ParseMieruClashNode 解析 clash YAML 格式的 mieru 节点。
func ParseMieruClashNode(src string) (map[string]interface{}, error) {
	data := strings.TrimSpace(src)
	var payload interface{}
	if err := yaml.Unmarshal([]byte(data), &payload); err != nil {
		if repaired, ok := repairClashNodeYAML(data); ok {
			if retryErr := yaml.Unmarshal([]byte(repaired), &payload); retryErr != nil {
				return nil, fmt.Errorf("mieru 节点 YAML 解析失败: %v", retryErr)
			}
		} else {
			return nil, fmt.Errorf("mieru 节点 YAML 解析失败: %v", err)
		}
	}
	node := pickClashNode(payload)
	if node == nil {
		return nil, fmt.Errorf("mieru 节点解析失败")
	}
	if strings.ToLower(getMapString(node, "type")) != "mieru" {
		return nil, fmt.Errorf("节点类型不是 mieru: %s", getMapString(node, "type"))
	}
	if getMapString(node, "server") == "" {
		return nil, fmt.Errorf("mieru 节点缺少 server")
	}
	if getMapInt(node, "port") == 0 && getMapString(node, "port-range") == "" {
		return nil, fmt.Errorf("mieru 节点缺少 port/port-range")
	}
	return node, nil
}

// buildMieruClientConfig 将 clash mieru 节点转换为 mieru 客户端 JSON 配置。
// 字段映射：server→ipAddress/domainName（按 IP/域名自动区分）、port/port-range→portBindings、
// transport→protocol、username/password→user、multiplexing/handshake-mode 原样透传。
// traffic-pattern 为 base64 protobuf 编码，JSON 需要对象形式，暂不透传（不影响基础连通性）。
func buildMieruClientConfig(node map[string]interface{}, socksPort int, rpcPort int) (map[string]interface{}, error) {
	server := strings.TrimSpace(getMapString(node, "server"))
	if server == "" {
		return nil, fmt.Errorf("mieru 节点缺少 server")
	}

	serverEntry := map[string]interface{}{}
	if ip := net.ParseIP(server); ip != nil {
		serverEntry["ipAddress"] = server
	} else {
		serverEntry["domainName"] = server
	}

	protocol := strings.ToUpper(strings.TrimSpace(getMapString(node, "transport")))
	if protocol == "" {
		protocol = "TCP"
	}

	// mieru 不允许同时使用 port 与 port-range，优先 port。
	portBindings := make([]interface{}, 0, 1)
	if port := getMapInt(node, "port"); port > 0 {
		portBindings = append(portBindings, map[string]interface{}{
			"port":     port,
			"protocol": protocol,
		})
	} else if portRange := strings.TrimSpace(getMapString(node, "port-range")); portRange != "" {
		portBindings = append(portBindings, map[string]interface{}{
			"portRange": portRange,
			"protocol":  protocol,
		})
	}
	if len(portBindings) == 0 {
		return nil, fmt.Errorf("mieru 节点缺少有效端口")
	}
	serverEntry["portBindings"] = portBindings

	profile := map[string]interface{}{
		"profileName": "default",
		"servers":     []interface{}{serverEntry},
	}
	if username := strings.TrimSpace(getMapString(node, "username")); username != "" {
		user := map[string]interface{}{"name": username}
		if password := strings.TrimSpace(getMapString(node, "password")); password != "" {
			user["password"] = password
		}
		profile["user"] = user
	}
	if multiplexing := strings.TrimSpace(getMapString(node, "multiplexing")); multiplexing != "" {
		profile["multiplexing"] = map[string]interface{}{"level": multiplexing}
	}
	if handshake := strings.TrimSpace(getMapString(node, "handshake-mode")); handshake != "" {
		profile["handshakeMode"] = handshake
	}

	return map[string]interface{}{
		"profiles":        []interface{}{profile},
		"activeProfile":   "default",
		"rpcPort":         rpcPort,
		"socks5Port":      socksPort,
		"loggingLevel":    "ERROR",
		"socks5ListenLAN": false,
	}, nil
}

// ─── 桥接生命周期 ───

// EnsureBridgeContext 确保桥接运行（临时请求场景）。
func (m *MieruManager) EnsureBridgeContext(ctx context.Context, proxyConfig string, proxies []config.BrowserProxy, proxyId string) (string, error) {
	socksURL, _, err := m.ensureBridgeContext(ctx, proxyConfig, proxies, proxyId, false)
	return socksURL, err
}

// AcquireBridge 获取带引用计数的桥接（实例等长生命周期场景）。
func (m *MieruManager) AcquireBridge(proxyConfig string, proxies []config.BrowserProxy, proxyId string) (string, string, error) {
	return m.ensureBridgeContext(context.Background(), proxyConfig, proxies, proxyId, true)
}

// ReleaseBridge 释放桥接引用。
func (m *MieruManager) ReleaseBridge(key string) {
	key = strings.TrimSpace(key)
	if m == nil || key == "" {
		return
	}
	m.mu.Lock()
	bridge, ok := m.Bridges[key]
	if !ok || bridge == nil {
		m.mu.Unlock()
		return
	}
	if bridge.RefCount > 0 {
		bridge.RefCount--
	}
	bridge.LastUsedAt = time.Now()
	m.mu.Unlock()
}

// ReclaimBridgeIfIdle 空闲桥接立即回收，避免批量探测后内核进程滞留。
func (m *MieruManager) ReclaimBridgeIfIdle(key string) {
	key = strings.TrimSpace(key)
	if m == nil || key == "" {
		return
	}
	m.mu.Lock()
	bridge, ok := m.Bridges[key]
	if !ok || bridge == nil || bridge.RefCount > 0 {
		m.mu.Unlock()
		return
	}
	bridge.Stopping = true
	delete(m.Bridges, key)
	m.mu.Unlock()

	logger.New("Mieru").Info("探测结束，回收空闲 mieru 桥接进程", logger.F("key", shortBridgeKey(key)), logger.F("pid", bridge.Pid))
	m.stopBridgeProcess(bridge)
}

func (m *MieruManager) ensureBridgeContext(ctx context.Context, proxyConfig string, proxies []config.BrowserProxy, proxyId string, pin bool) (string, string, error) {
	log := logger.New("Mieru")
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return "", "", err
	}
	src := resolveProxyConfig(proxyConfig, proxies, proxyId)
	if src == "" {
		return "", "", fmt.Errorf("未找到代理节点")
	}

	node, err := ParseMieruClashNode(src)
	if err != nil {
		return "", "", err
	}

	key := ComputeBridgeKey(src, "", ProxyKernelMieru)

	// 同一节点并发启动串行化，避免批量测速/实例启动并发时产生孤儿 mieru 进程。
	unlockLaunch := m.lockLaunchForKey(key)
	defer unlockLaunch()

	if socksURL, reused := m.tryReuseBridge(key, pin); reused {
		log.Info("复用 mieru 桥接", logger.F("key", shortBridgeKey(key)), logger.F("socks_url", socksURL))
		return socksURL, key, nil
	}

	binaryPath, err := m.resolveBinary()
	if err != nil {
		log.Error("mieru 不可用", logger.F("error", err.Error()), logger.F("appRoot", m.AppRoot))
		return "", "", err
	}

	const maxRetries = 2
	var lastErr error
	attemptsUsed := 0
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return "", "", err
		}
		attemptsUsed = attempt
		socksPort, rpcPort, err := m.allocatePorts()
		if err != nil {
			lastErr = err
			continue
		}

		bridge, err := m.launchBridgeOnPort(ctx, log, key, binaryPath, node, socksPort, rpcPort, attempt)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return "", "", ctxErr
			}
			lastErr = err
			continue
		}

		if socksURL, reused := m.registerBridge(key, bridge, pin); reused {
			log.Info("复用已就绪 mieru 桥接", logger.F("key", shortBridgeKey(key)), logger.F("socks_url", socksURL))
			bridge.Stopping = true
			m.stopBridgeProcess(bridge)
			return socksURL, key, nil
		}
		if err := ctx.Err(); err != nil {
			m.discardBridge(key, bridge)
			return "", "", err
		}

		go m.watchBridge(bridge, key)
		return fmt.Sprintf("socks5://127.0.0.1:%d", socksPort), key, nil
	}
	return "", "", fmt.Errorf("mieru 启动失败（已尝试 %d 次）: %w", attemptsUsed, lastErr)
}

// shortBridgeKey 安全截断桥接键用于日志（防短键越界）。
func shortBridgeKey(key string) string {
	if len(key) > 8 {
		return key[:8]
	}
	return key
}

func (m *MieruManager) launchBridgeOnPort(ctx context.Context, log *logger.Logger, key string, binaryPath string, node map[string]interface{}, socksPort int, rpcPort int, attempt int) (*MieruBridge, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	clientConfig, err := buildMieruClientConfig(node, socksPort, rpcPort)
	if err != nil {
		return nil, err
	}
	cfgPath, err := m.writeClientConfig(key, clientConfig)
	if err != nil {
		return nil, fmt.Errorf("mieru 配置写入失败: %w", err)
	}

	cmd := exec.Command(binaryPath, "run")
	hideWindow(cmd)
	cmd.Dir = filepath.Dir(cfgPath)
	cmd.Env = append(os.Environ(), fmt.Sprintf("%s=%s", mieruConfigEnvVar, cfgPath))
	stderrPath := filepath.Join(filepath.Dir(cfgPath), "mieru-stderr.log")
	stderrFile, err := os.Create(stderrPath)
	if err == nil {
		cmd.Stderr = stderrFile
	}

	if err := cmd.Start(); err != nil {
		if stderrFile != nil {
			stderrFile.Close()
		}
		log.Error("mieru 启动失败", logger.F("error", err.Error()), logger.F("attempt", attempt))
		return nil, err
	}
	bridge := &MieruBridge{
		NodeKey:    key,
		SocksPort:  socksPort,
		RpcPort:    rpcPort,
		Cmd:        cmd,
		Pid:        cmd.Process.Pid,
		Running:    true,
		LastUsedAt: time.Now(),
		ExitDone:   make(chan struct{}),
	}
	go bridge.startExitWatcher()
	log.Info("mieru 内核进程已启动", logger.F("key", shortBridgeKey(key)), logger.F("pid", bridge.Pid), logger.F("socks_port", socksPort), logger.F("rpc_port", rpcPort))

	if err := waitSocks5ReadyContext(ctx, "127.0.0.1", socksPort, mieruBridgeStartTTL); err != nil {
		if stderrFile != nil {
			stderrFile.Close()
		}
		bridge.Stopping = true
		m.stopBridgeProcess(bridge)
		return nil, fmt.Errorf("mieru socks5 端口 %d 启动超时: %w", socksPort, err)
	}
	if stderrFile != nil {
		stderrFile.Close()
	}
	return bridge, nil
}

func (b *MieruBridge) startExitWatcher() {
	if b == nil || b.Cmd == nil {
		return
	}
	err := b.Cmd.Wait()
	b.exitMu.Lock()
	b.ExitErr = err
	b.exitMu.Unlock()
	b.Running = false
	b.exitOnce.Do(func() { close(b.ExitDone) })
}

func (b *MieruBridge) exitErr() error {
	b.exitMu.Lock()
	defer b.exitMu.Unlock()
	return b.ExitErr
}

func (m *MieruManager) writeClientConfig(key string, clientConfig map[string]interface{}) (string, error) {
	baseDir := m.resolveWorkdir(key)
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(clientConfig, "", "  ")
	if err != nil {
		return "", err
	}
	cfgPath := filepath.Join(baseDir, "client.json")
	if err := os.WriteFile(cfgPath, data, 0o600); err != nil {
		return "", err
	}
	return cfgPath, nil
}

func (m *MieruManager) resolveWorkdir(key string) string {
	root := m.AppRoot
	if root == "" {
		root = "."
	}
	return filepath.Join(root, "data", "tmp", "mieru", key)
}

func (m *MieruManager) allocatePorts() (int, int, error) {
	socksPort, err := nextAvailablePort()
	if err != nil {
		return 0, 0, err
	}
	rpcPort, err := nextAvailablePort()
	if err != nil {
		return 0, 0, err
	}
	if rpcPort == socksPort {
		return 0, 0, fmt.Errorf("mieru 端口分配冲突: %d", socksPort)
	}
	return socksPort, rpcPort, nil
}

func (m *MieruManager) tryReuseBridge(key string, pin bool) (string, bool) {
	m.mu.Lock()
	bridge, ok := m.Bridges[key]
	if !ok || bridge == nil || !bridge.Running || bridge.Stopping {
		m.mu.Unlock()
		return "", false
	}
	if pin {
		bridge.RefCount++
	}
	bridge.LastUsedAt = time.Now()
	port := bridge.SocksPort
	m.mu.Unlock()
	return fmt.Sprintf("socks5://127.0.0.1:%d", port), true
}

func (m *MieruManager) registerBridge(key string, bridge *MieruBridge, pin bool) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, ok := m.Bridges[key]
	if ok && existing != nil && existing.Running && !existing.Stopping {
		if pin {
			existing.RefCount++
		}
		existing.LastUsedAt = time.Now()
		return fmt.Sprintf("socks5://127.0.0.1:%d", existing.SocksPort), true
	}
	if pin {
		bridge.RefCount = 1
	}
	bridge.LastUsedAt = time.Now()
	m.Bridges[key] = bridge
	return "", false
}

func (m *MieruManager) discardBridge(key string, bridge *MieruBridge) {
	if bridge == nil {
		return
	}
	m.mu.Lock()
	if current, ok := m.Bridges[key]; ok && current == bridge {
		delete(m.Bridges, key)
	}
	m.mu.Unlock()
	bridge.Stopping = true
	m.stopBridgeProcess(bridge)
}

func (m *MieruManager) watchBridge(bridge *MieruBridge, key string) {
	if bridge == nil {
		return
	}
	<-bridge.ExitDone
	m.mu.Lock()
	if current, ok := m.Bridges[key]; ok && current == bridge {
		delete(m.Bridges, key)
	}
	m.mu.Unlock()
}

func (m *MieruManager) stopBridgeProcess(bridge *MieruBridge) {
	if bridge == nil || bridge.Cmd == nil || bridge.Cmd.Process == nil {
		return
	}
	_ = bridge.Cmd.Process.Kill()
	select {
	case <-bridge.ExitDone:
	case <-time.After(mieruBridgeStopTTL):
	}
}

func (m *MieruManager) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
		}
		now := time.Now()
		m.mu.Lock()
		var stale []*MieruBridge
		for key, bridge := range m.Bridges {
			if bridge == nil || bridge.Stopping || bridge.RefCount > 0 {
				continue
			}
			if now.Sub(bridge.LastUsedAt) < mieruBridgeIdleTTL {
				continue
			}
			bridge.Stopping = true
			delete(m.Bridges, key)
			stale = append(stale, bridge)
		}
		m.mu.Unlock()
		for _, bridge := range stale {
			logger.New("Mieru").Info("回收空闲 mieru 桥接进程", logger.F("key", bridge.NodeKey), logger.F("pid", bridge.Pid))
			m.stopBridgeProcess(bridge)
		}
	}
}

// ResolveBinaryPath 解析 mieru 可执行文件路径（供诊断与测试使用）。
func (m *MieruManager) ResolveBinaryPath() (string, error) {
	if m == nil {
		return "", fmt.Errorf("mieru 管理器未初始化")
	}
	return m.resolveBinary()
}

// MieruBinaryAvailable 检查 mieru 内核是否已安装（供前端提示下载使用）。
func MieruBinaryAvailable(cfg *config.Config, appRoot string) bool {
	mgr := &MieruManager{Config: cfg, AppRoot: appRoot}
	_, err := mgr.resolveBinary()
	return err == nil
}

// lockLaunchForKey 同 key 启动互斥（复用 sing-box 的 bridgeLaunchLock 机制）。
func (m *MieruManager) lockLaunchForKey(key string) func() {
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
