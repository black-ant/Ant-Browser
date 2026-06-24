package proxy

import (
	"ant-chrome/backend/internal/apppath"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/fsutil"
	"ant-chrome/backend/internal/logger"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync"
	"time"
)

const (
	singboxBridgeIdleTTL         = 45 * time.Second
	singboxBridgeCleanupInterval = 15 * time.Second
)

// SingBoxBridge sing-box 桥接进程
type SingBoxBridge struct {
	NodeKey    string
	Port       int
	Cmd        *exec.Cmd
	Pid        int
	Running    bool
	Stopping   bool
	LastError  string
	RefCount   int       // 当前引用计数（有多少实例正使用此桥接）
	LastUsedAt time.Time // 上次被使用的时间（用于空闲回收）
}

// SingBoxManager sing-box 桥接管理器
type SingBoxManager struct {
	Config       *config.Config
	AppRoot      string // 应用根目录，所有相对路径基于此解析
	Bridges      map[string]*SingBoxBridge
	OnBridgeDied func(key string, err error)
	mu           sync.Mutex
	stopCh       chan struct{}
	stopOnce     sync.Once
}

// NewSingBoxManager 创建 sing-box 管理器
func NewSingBoxManager(cfg *config.Config, appRoot string) *SingBoxManager {
	manager := &SingBoxManager{
		Config:  cfg,
		AppRoot: appRoot,
		Bridges: make(map[string]*SingBoxBridge),
		stopCh:  make(chan struct{}),
	}
	go manager.cleanupLoop()
	return manager
}

// EnsureBridge 确保 sing-box 桥接进程运行，返回 socks5://127.0.0.1:port
func (m *SingBoxManager) EnsureBridge(proxyConfig string, proxies []config.BrowserProxy, proxyId string) (string, error) {
	log := logger.New("SingBox")
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
		return "", fmt.Errorf("未找到代理节点")
	}

	src = normalizeNodeScheme(src)
	outbound, err := BuildSingBoxOutbound(src)
	if err != nil {
		log.Error("节点解析失败", logger.F("error", err))
		return "", err
	}

	key := computeNodeKey(src)

	if socksURL, reused := m.tryReuseBridge(key); reused {
		log.Info("复用 sing-box 桥接", logger.F("key", key[:8]), logger.F("socks_url", socksURL))
		return socksURL, nil
	}

	binaryPath, err := m.resolveBinary()
	if err != nil {
		log.Error("sing-box 不可用", logger.F("error", err), logger.F("appRoot", m.AppRoot))
		return "", err
	}
	log.Debug("sing-box binary", logger.F("path", binaryPath))

	const maxRetries = 3
	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		port, err := nextAvailablePort()
		if err != nil {
			lastErr = err
			continue
		}

		cfgPath, err := m.buildConfig(key, outbound, port)
		if err != nil {
			return "", fmt.Errorf("sing-box 配置生成失败: %w", err)
		}

		cmd := exec.Command(binaryPath, "run", "-c", cfgPath)
		hideWindow(cmd)
		cmd.Dir = filepath.Dir(cfgPath)
		stderrPath := filepath.Join(filepath.Dir(cfgPath), "singbox-stderr.log")
		stderrFile, _ := os.Create(stderrPath)
		if stderrFile != nil {
			cmd.Stderr = stderrFile
		}

		if err := cmd.Start(); err != nil {
			if stderrFile != nil {
				stderrFile.Close()
			}
			log.Error("sing-box 启动失败", logger.F("error", err), logger.F("attempt", attempt))
			lastErr = err
			continue
		}

		bridge := &SingBoxBridge{
			NodeKey:    key,
			Port:       port,
			Cmd:        cmd,
			Pid:        cmd.Process.Pid,
			Running:    true,
			RefCount:   1,
			LastUsedAt: time.Now(),
		}
		log.Info("sing-box 启动", logger.F("key", key[:8]), logger.F("pid", bridge.Pid), logger.F("port", port))

		if err := waitPortReady("127.0.0.1", port, 10*time.Second); err != nil {
			if stderrFile != nil {
				stderrFile.Close()
			}
			if content, readErr := os.ReadFile(stderrPath); readErr == nil && len(content) > 0 {
				log.Error("sing-box stderr", logger.F("output", string(content)))
			}
			bridge.Stopping = true
			m.stopBridgeProcess(bridge)
			bridge.Running = false
			bridge.Pid = 0
			bridge.LastError = err.Error()
			log.Error("sing-box 端口不可用，重试", logger.F("error", err), logger.F("attempt", attempt))
			lastErr = err
			time.Sleep(200 * time.Millisecond)
			continue
		}

		if stderrFile != nil {
			stderrFile.Close()
		}

		if socksURL, reused := m.registerBridge(key, bridge); reused {
			log.Info("复用已就绪 sing-box 桥接", logger.F("key", key[:8]), logger.F("socks_url", socksURL))
			bridge.Stopping = true
			m.stopBridgeProcess(bridge)
			return socksURL, nil
		}

		go m.watchBridge(bridge, key)
		return fmt.Sprintf("socks5://127.0.0.1:%d", port), nil
	}

	return "", fmt.Errorf("sing-box 启动失败（已重试 %d 次）: %w", maxRetries, lastErr)
}

// StopAll 关闭所有 sing-box 桥接进程
func (m *SingBoxManager) StopAll() {
	m.mu.Lock()
	bridges := make([]*SingBoxBridge, 0, len(m.Bridges))
	for key, bridge := range m.Bridges {
		if bridge != nil {
			bridge.Stopping = true
			bridges = append(bridges, bridge)
		}
		delete(m.Bridges, key)
	}
	m.mu.Unlock()

	for _, bridge := range bridges {
		m.stopBridgeProcess(bridge)
	}

	// 通知回收循环退出
	m.stopOnce.Do(func() {
		close(m.stopCh)
	})
}

func (m *SingBoxManager) cleanupLoop() {
	ticker := time.NewTicker(singboxBridgeCleanupInterval)
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

func (m *SingBoxManager) recycleIdleBridges() {
	now := time.Now()
	var stale []*SingBoxBridge

	m.mu.Lock()
	for key, bridge := range m.Bridges {
		if bridge == nil {
			delete(m.Bridges, key)
			continue
		}
		if bridge.RefCount > 0 {
			continue
		}
		if now.Sub(bridge.LastUsedAt) < singboxBridgeIdleTTL {
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

	log := logger.New("SingBox")
	for _, bridge := range stale {
		log.Info("回收空闲桥接进程", logger.F("key", bridge.NodeKey), logger.F("pid", bridge.Pid))
		m.stopBridgeProcess(bridge)
	}
}

func (m *SingBoxManager) tryReuseBridge(key string) (string, bool) {
	var stale *SingBoxBridge

	m.mu.Lock()
	if bridge, ok := m.Bridges[key]; ok && bridge != nil {
		alive := bridge.Running && bridge.Cmd != nil && bridge.Cmd.Process != nil && bridge.Cmd.ProcessState == nil
		if alive && waitPortReady("127.0.0.1", bridge.Port, 800*time.Millisecond) == nil {
			bridge.RefCount++
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

func (m *SingBoxManager) registerBridge(key string, bridge *SingBoxBridge) (string, bool) {
	var duplicate *SingBoxBridge

	m.mu.Lock()
	if existing, ok := m.Bridges[key]; ok && existing != nil {
		if existing == bridge {
			m.mu.Unlock()
			return "", false
		}

		alive := existing.Running && existing.Cmd != nil && existing.Cmd.Process != nil && existing.Cmd.ProcessState == nil
		if alive && waitPortReady("127.0.0.1", existing.Port, 800*time.Millisecond) == nil {
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
	m.Bridges[key] = bridge
	m.mu.Unlock()

	if duplicate != nil {
		m.stopBridgeProcess(duplicate)
	}
	return "", false
}

func (m *SingBoxManager) watchBridge(bridge *SingBoxBridge, key string) {
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
		m.OnBridgeDied(key, fmt.Errorf("sing-box 桥接进程意外退出"))
	}
}

func (m *SingBoxManager) stopBridgeProcess(bridge *SingBoxBridge) {
	if bridge == nil || bridge.Cmd == nil || bridge.Cmd.Process == nil {
		return
	}
	_ = bridge.Cmd.Process.Kill()
}

func (m *SingBoxManager) resolveBinary() (string, error) {
	configPath := strings.TrimSpace(m.Config.Browser.SingBoxBinaryPath)
	if configPath != "" {
		resolved := resolveEnvPath(configPath, m.AppRoot)
		if resolved != "" {
			if _, err := os.Stat(resolved); err == nil {
				if err := fsutil.EnsureExecutable(resolved); err != nil {
					return "", fmt.Errorf("sing-box 文件不可执行: %s: %w", resolved, err)
				}
				return resolved, nil
			}
		}
	}
	if env := strings.TrimSpace(os.Getenv("SINGBOX_BINARY_PATH")); env != "" {
		if _, err := os.Stat(env); err == nil {
			if err := fsutil.EnsureExecutable(env); err != nil {
				return "", fmt.Errorf("sing-box 文件不可执行: %s: %w", env, err)
			}
			return env, nil
		}
	}

	binaryNames := []string{"sing-box"}
	if goruntime.GOOS == "windows" {
		binaryNames = []string{"sing-box.exe", "sing-box"}
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
					return "", fmt.Errorf("sing-box 文件不可执行: %s: %w", candidate, err)
				}
				return candidate, nil
			}
		}
	}

	for _, name := range binaryNames {
		if path, err := exec.LookPath(name); err == nil {
			if err := fsutil.EnsureExecutable(path); err != nil {
				return "", fmt.Errorf("sing-box 文件不可执行: %s: %w", path, err)
			}
			return path, nil
		}
	}

	return "", fmt.Errorf("未找到 sing-box 可执行文件。请将 sing-box 放到 bin/%s/ 或 bin/ 目录，或在配置中设置 SingBoxBinaryPath", platformDir)
}

func (m *SingBoxManager) buildConfig(key string, outbound map[string]interface{}, port int) (string, error) {
	baseDir := m.resolveWorkdir(key)
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return "", err
	}

	cfg := map[string]interface{}{
		"log": map[string]interface{}{
			"level":     "info",
			"output":    filepath.Join(baseDir, "singbox.log"),
			"timestamp": true,
		},
		"inbounds": []interface{}{
			map[string]interface{}{
				"type":        "socks",
				"tag":         "socks-in",
				"listen":      "127.0.0.1",
				"listen_port": port,
			},
		},
		"outbounds": []interface{}{
			outbound,
			map[string]interface{}{
				"type": "direct",
				"tag":  "direct",
			},
		},
		"route": map[string]interface{}{
			"rules": []interface{}{
				map[string]interface{}{
					"inbound":  []string{"socks-in"},
					"outbound": "proxy-out",
				},
			},
		},
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}

	cfgPath := filepath.Join(baseDir, "singbox-config.json")
	if err := os.WriteFile(cfgPath, data, 0600); err != nil {
		return "", err
	}
	return cfgPath, nil
}

func (m *SingBoxManager) resolveWorkdir(key string) string {
	root := strings.TrimSpace(m.Config.Browser.UserDataRoot)
	if root == "" {
		root = "data"
	}
	if !filepath.IsAbs(root) {
		root = apppath.Resolve(m.AppRoot, root)
	}
	return filepath.Join(root, "_singbox", key)
}
