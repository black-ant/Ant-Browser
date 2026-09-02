package proxy

import (
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/logger"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// EnsureBridge 确保 sing-box 桥接进程运行，用于临时请求场景。
func (m *SingBoxManager) EnsureBridge(proxyConfig string, proxies []config.BrowserProxy, proxyId string) (string, error) {
	return m.EnsureBridgeContext(context.Background(), proxyConfig, proxies, proxyId)
}

func (m *SingBoxManager) EnsureBridgeContext(ctx context.Context, proxyConfig string, proxies []config.BrowserProxy, proxyId string) (string, error) {
	socksURL, _, err := m.ensureBridgeContext(ctx, proxyConfig, proxies, proxyId, false)
	return socksURL, err
}

// AcquireBridge 获取一个带引用计数的 sing-box 桥接，用于浏览器实例等长生命周期场景。
func (m *SingBoxManager) AcquireBridge(proxyConfig string, proxies []config.BrowserProxy, proxyId string) (string, string, error) {
	return m.ensureBridgeContext(context.Background(), proxyConfig, proxies, proxyId, true)
}

// ReleaseBridge 释放一个已占用的桥接引用；空闲桥接会由后台回收协程延迟清理。
func (m *SingBoxManager) ReleaseBridge(key string) {
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

func (m *SingBoxManager) ensureBridge(proxyConfig string, proxies []config.BrowserProxy, proxyId string, pin bool) (string, string, error) {
	return m.ensureBridgeContext(context.Background(), proxyConfig, proxies, proxyId, pin)
}

func (m *SingBoxManager) ensureBridgeContext(ctx context.Context, proxyConfig string, proxies []config.BrowserProxy, proxyId string, pin bool) (string, string, error) {
	log := logger.New("SingBox")
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

	src = normalizeNodeScheme(src)
	outbound, err := BuildSingBoxOutbound(src)
	if err != nil {
		log.Error("节点解析失败", logger.F("error", err))
		return "", "", err
	}

	key := computeNodeKey(src)

	if socksURL, reused := m.tryReuseBridge(key, pin); reused {
		log.Info("复用 sing-box 桥接", logger.F("engine", "sing-box"), logger.F("key", key[:8]), logger.F("socks_url", socksURL))
		return socksURL, key, nil
	}
	unlockLaunch := m.lockLaunchForKey(key)
	defer unlockLaunch()
	if err := ctx.Err(); err != nil {
		return "", "", err
	}
	if socksURL, reused := m.tryReuseBridge(key, pin); reused {
		log.Info("复用 sing-box 桥接", logger.F("engine", "sing-box"), logger.F("key", key[:8]), logger.F("socks_url", socksURL))
		return socksURL, key, nil
	}

	binaryPath, err := m.resolveBinary()
	if err != nil {
		log.Error("sing-box 不可用", logger.F("error", err), logger.F("appRoot", m.AppRoot))
		return "", "", err
	}
	log.Debug("sing-box binary", logger.F("path", binaryPath))

	const maxRetries = 2
	var lastErr error
	attemptsUsed := 0
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return "", "", err
		}
		attemptsUsed = attempt
		port, err := nextAvailablePort()
		if err != nil {
			lastErr = err
			continue
		}

		bridge, err := m.launchBridgeOnPortContext(ctx, log, key, binaryPath, outbound, port, attempt)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return "", "", ctxErr
			}
			lastErr = err
			if !isRetryableSingBoxLaunchError(err) {
				break
			}
			continue
		}

		if socksURL, reused := m.registerBridge(key, bridge, pin); reused {
			log.Info("复用已就绪 sing-box 桥接", logger.F("engine", "sing-box"), logger.F("key", key[:8]), logger.F("socks_url", socksURL))
			bridge.Stopping = true
			m.stopBridgeProcess(bridge)
			return socksURL, key, nil
		}
		if err := ctx.Err(); err != nil {
			m.discardBridge(key, bridge)
			return "", "", err
		}

		go m.watchBridge(bridge, key)
		return fmt.Sprintf("socks5://127.0.0.1:%d", port), key, nil
	}

	return "", "", fmt.Errorf("sing-box 启动失败（已尝试 %d 次）: %w", attemptsUsed, lastErr)
}

func (m *SingBoxManager) launchBridgeOnPort(log *logger.Logger, key string, binaryPath string, outbound map[string]interface{}, port int, attempt int) (*SingBoxBridge, error) {
	return m.launchBridgeOnPortContext(context.Background(), log, key, binaryPath, outbound, port, attempt)
}

func (m *SingBoxManager) launchBridgeOnPortContext(ctx context.Context, log *logger.Logger, key string, binaryPath string, outbound map[string]interface{}, port int, attempt int) (*SingBoxBridge, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cfgPath, err := m.buildConfig(key, outbound, port)
	if err != nil {
		return nil, fmt.Errorf("sing-box 配置生成失败: %w", err)
	}
	stderrPath := filepath.Join(filepath.Dir(cfgPath), "singbox-stderr.log")
	if err := m.testRuntimeConfigContext(ctx, binaryPath, cfgPath, stderrPath); err != nil {
		log.Error("sing-box 配置预检失败", logger.F("error", err), logger.F("attempt", attempt), logger.F("config", cfgPath))
		return nil, err
	}

	cmd := exec.Command(binaryPath, "run", "-c", cfgPath)
	hideWindow(cmd)
	cmd.Dir = filepath.Dir(cfgPath)
	stderrFile, _ := os.Create(stderrPath)
	if stderrFile != nil {
		cmd.Stderr = stderrFile
	}

	if err := cmd.Start(); err != nil {
		if stderrFile != nil {
			stderrFile.Close()
		}
		log.Error("sing-box 启动失败", logger.F("error", err), logger.F("attempt", attempt))
		return nil, &singBoxLaunchError{err: err, retryable: false}
	}
	if err := ctx.Err(); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		if stderrFile != nil {
			stderrFile.Close()
		}
		return nil, &singBoxLaunchError{err: err, retryable: false}
	}

	bridge := &SingBoxBridge{
		NodeKey:    key,
		Port:       port,
		Cmd:        cmd,
		Pid:        cmd.Process.Pid,
		Running:    true,
		Outbound:   cloneStringInterfaceMap(outbound),
		LastUsedAt: time.Now(),
	}
	bridge.startExitWatcher()
	log.Info("sing-box 内核进程已启动", logger.F("engine", "sing-box"), logger.F("key", key[:8]), logger.F("pid", bridge.Pid), logger.F("port", port))

	if err := m.waitBridgeSocksReadyContext(ctx, bridge, m.bridgeStartTimeout()); err != nil {
		if stderrFile != nil {
			stderrFile.Close()
		}
		m.logBridgeStartupError(log, cfgPath, stderrPath)
		bridge.Stopping = true
		m.stopBridgeProcess(bridge)
		bridge.Running = false
		bridge.Pid = 0
		bridge.LastError = m.describeBridgeReadyError(err, cfgPath, stderrPath)
		retryable := m.isRetryableBridgeReadyError(err, cfgPath, stderrPath)
		message := "sing-box 桥接未就绪"
		if retryable {
			message = "sing-box 桥接未就绪，重试"
		}
		log.Error(message, logger.F("error", err), logger.F("attempt", attempt), logger.F("port", port), logger.F("retryable", retryable))
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, &singBoxLaunchError{err: ctxErr, retryable: false}
		}
		time.Sleep(200 * time.Millisecond)
		return nil, &singBoxLaunchError{err: fmt.Errorf("%s", bridge.LastError), retryable: retryable}
	}

	if stderrFile != nil {
		stderrFile.Close()
	}
	if err := ctx.Err(); err != nil {
		m.discardBridge(key, bridge)
		return nil, &singBoxLaunchError{err: err, retryable: false}
	}
	return bridge, nil
}

func (m *SingBoxManager) bridgeStartTimeout() time.Duration {
	if m != nil && m.Config != nil && m.Config.ProxyCheck.BridgeStartTimeoutMs > 0 {
		return time.Duration(m.Config.ProxyCheck.BridgeStartTimeoutMs) * time.Millisecond
	}
	return time.Duration(defaultBridgeStartTimeoutMs) * time.Millisecond
}

type singBoxLaunchError struct {
	err       error
	retryable bool
}

func (e *singBoxLaunchError) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *singBoxLaunchError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func isRetryableSingBoxLaunchError(err error) bool {
	if err == nil {
		return false
	}
	var launchErr *singBoxLaunchError
	if errors.As(err, &launchErr) {
		return launchErr.retryable
	}
	return true
}

func (m *SingBoxManager) testRuntimeConfig(binaryPath string, cfgPath string, stderrPath string) error {
	return m.testRuntimeConfigContext(context.Background(), binaryPath, cfgPath, stderrPath)
}

func (m *SingBoxManager) testRuntimeConfigContext(ctx context.Context, binaryPath string, cfgPath string, stderrPath string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := exec.CommandContext(ctx, binaryPath, "check", "-c", cfgPath)
	hideWindow(cmd)
	cmd.Dir = filepath.Dir(cfgPath)
	stderrFile, _ := os.Create(stderrPath)
	if stderrFile != nil {
		defer stderrFile.Close()
		cmd.Stderr = stderrFile
	}
	output, err := cmd.Output()
	if err == nil {
		return nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return &singBoxLaunchError{err: ctxErr, retryable: false}
	}
	if len(output) > 0 && stderrFile != nil {
		_, _ = stderrFile.Write(output)
	}
	return &singBoxLaunchError{
		err:       fmt.Errorf("sing-box 配置预检失败: %w；%s", err, m.describeBridgeReadyError(err, cfgPath, stderrPath)),
		retryable: false,
	}
}

func (m *SingBoxManager) waitBridgeSocksReady(bridge *SingBoxBridge, timeout time.Duration) error {
	return m.waitBridgeSocksReadyContext(context.Background(), bridge, timeout)
}

func (m *SingBoxManager) waitBridgeSocksReadyContext(ctx context.Context, bridge *SingBoxBridge, timeout time.Duration) error {
	if bridge == nil {
		return fmt.Errorf("sing-box 桥接进程不存在")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ready := make(chan error, 1)
	go func() {
		ready <- waitSocks5ReadyContext(ctx, "127.0.0.1", bridge.Port, timeout)
	}()
	select {
	case err := <-ready:
		return err
	case <-bridge.ExitDone:
		if err := bridge.exitErr(); err != nil {
			return fmt.Errorf("sing-box 进程提前退出: %w", err)
		}
		return fmt.Errorf("sing-box 进程提前退出")
	case <-ctx.Done():
		return ctx.Err()
	case <-deadline.C:
		return fmt.Errorf("sing-box socks5 端口 %d 启动超时", bridge.Port)
	}
}

func (m *SingBoxManager) discardBridge(key string, bridge *SingBoxBridge) {
	if bridge == nil {
		return
	}
	m.mu.Lock()
	bridge.Stopping = true
	if current, ok := m.Bridges[key]; ok && current == bridge {
		delete(m.Bridges, key)
	}
	m.mu.Unlock()
	m.stopBridgeProcess(bridge)
}

func (m *SingBoxManager) isRetryableBridgeReadyError(err error, cfgPath string, stderrPath string) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	if !strings.Contains(message, "提前退出") {
		return true
	}
	tail := strings.ToLower(readLogTail(stderrPath, 1200))
	if tail == "" && strings.TrimSpace(cfgPath) != "" {
		tail = strings.ToLower(readLogTail(filepath.Join(filepath.Dir(cfgPath), "singbox-error.log"), 1200))
	}
	return strings.Contains(tail, "address already in use") ||
		strings.Contains(tail, "only one usage of each socket") ||
		strings.Contains(tail, "bind:") ||
		strings.Contains(tail, "bind ")
}

func (m *SingBoxManager) describeBridgeReadyError(err error, cfgPath string, stderrPath string) string {
	parts := []string{err.Error()}
	if strings.TrimSpace(cfgPath) != "" {
		parts = append(parts, "配置文件: "+cfgPath)
	}
	if tail := readLogTail(stderrPath, 1200); tail != "" {
		parts = append(parts, "stderr: "+tail)
	}
	return strings.Join(parts, "；")
}

func (m *SingBoxManager) logBridgeStartupError(log *logger.Logger, cfgPath string, stderrPath string) {
	if stderrContent, readErr := os.ReadFile(stderrPath); readErr == nil && len(stderrContent) > 0 {
		log.Error("sing-box stderr", logger.F("output", string(stderrContent)))
	}
}

// StopBridges closes all sing-box bridge processes while keeping the manager cleanup loop alive.
func (m *SingBoxManager) StopBridges() {
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
}

// StopAll closes all sing-box bridge processes and stops the manager cleanup loop.
func (m *SingBoxManager) StopAll() {
	m.stopOnce.Do(func() {
		close(m.stopCh)
	})
	m.StopBridges()
}

func (m *SingBoxManager) tryReuseBridge(key string, pin bool) (string, bool) {
	var stale *SingBoxBridge

	m.mu.Lock()
	if bridge, ok := m.Bridges[key]; ok && bridge != nil {
		alive := bridge.Running && bridge.Cmd != nil && bridge.Cmd.Process != nil && bridge.Cmd.ProcessState == nil
		if alive && waitSocks5Ready("127.0.0.1", bridge.Port, 800*time.Millisecond) == nil {
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

func (m *SingBoxManager) registerBridge(key string, bridge *SingBoxBridge, pin bool) (string, bool) {
	var duplicate *SingBoxBridge

	m.mu.Lock()
	if existing, ok := m.Bridges[key]; ok && existing != nil {
		if existing == bridge {
			m.mu.Unlock()
			return "", false
		}

		alive := existing.Running && existing.Cmd != nil && existing.Cmd.Process != nil && existing.Cmd.ProcessState == nil
		if alive && waitSocks5Ready("127.0.0.1", existing.Port, 800*time.Millisecond) == nil {
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

		transferredRefCount := 0
		if existing.Restarting && existing.RefCount > 0 {
			transferredRefCount = existing.RefCount
		}
		existing.Stopping = true
		delete(m.Bridges, key)
		duplicate = existing
		if transferredRefCount > 0 && !pin {
			bridge.RefCount = transferredRefCount
		}
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

func (m *SingBoxManager) watchBridge(bridge *SingBoxBridge, key string) {
	if bridge == nil || bridge.Cmd == nil {
		return
	}
	_ = bridge.waitExit()

	var shouldRestart bool
	var refCount int
	m.mu.Lock()
	if current, ok := m.Bridges[key]; ok && current == bridge {
		refCount = bridge.RefCount
		if !bridge.Stopping && refCount > 0 && !bridge.Restarting {
			bridge.Restarting = true
			shouldRestart = true
		} else {
			delete(m.Bridges, key)
		}
	}
	bridge.Running = false
	stopping := bridge.Stopping
	m.mu.Unlock()

	if shouldRestart {
		log := logger.New("SingBox")
		if err := m.restartBridgeOnSamePort(log, key, bridge, refCount); err == nil {
			return
		} else if errors.Is(err, errSingBoxBridgeRestartNotNeeded) {
			return
		} else {
			log.Error("sing-box 桥接同端口恢复失败", logger.F("key", key[:8]), logger.F("port", bridge.Port), logger.F("error", err.Error()))
			m.mu.Lock()
			if current, ok := m.Bridges[key]; ok && current == bridge {
				delete(m.Bridges, key)
			}
			m.mu.Unlock()
		}
	}

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
