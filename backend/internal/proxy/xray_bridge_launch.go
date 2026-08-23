package proxy

import (
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/logger"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func (m *XrayManager) ensureBridge(proxyConfig string, proxies []config.BrowserProxy, proxyId string, pin bool) (string, string, error) {
	log := logger.New("Xray")
	src := resolveProxyConfig(proxyConfig, proxies, proxyId)
	dnsServers := ""
	if proxyId != "" {
		for _, item := range proxies {
			if strings.EqualFold(item.ProxyId, proxyId) {
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

	if IsChainProxy(src) {
		chainCfg, err := ParseChainProxyConfig(src)
		if err != nil {
			log.Error("通用链式节点解析失败", logger.F("error", err))
			return "", "", err
		}
		outbounds, routes, err = buildXrayChainOutbounds(chainCfg, proxies, proxyId)
		if err != nil {
			return "", "", err
		}
		preferredPort = chainCfg.LocalPort
	} else if IsChainSocks5Proxy(src) {
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
		directOutbound, shouldBridgeDirectProxy, err := buildDirectProxyBridgeOutbound(src)
		if err != nil {
			log.Error("直连代理桥接配置解析失败", logger.F("error", err))
			return "", "", err
		}
		if shouldBridgeDirectProxy {
			outbounds = []interface{}{directOutbound}
			routes = []interface{}{
				map[string]interface{}{
					"type":        "field",
					"inboundTag":  []string{"socks-in"},
					"outboundTag": "proxy-out",
				},
			}
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
	}
	key := computeNodeKey(src + "\x00" + dnsServers)

	if socksURL, reused := m.tryReuseBridge(key, pin); reused {
		log.Info("复用 xray 桥接进程", logger.F("engine", "xray"), logger.F("key", key), logger.F("socks_url", socksURL))
		return socksURL, key, nil
	}
	unlockLaunch := m.lockLaunchForKey(key)
	defer unlockLaunch()
	if socksURL, reused := m.tryReuseBridge(key, pin); reused {
		log.Info("复用 xray 桥接进程", logger.F("engine", "xray"), logger.F("key", key), logger.F("socks_url", socksURL))
		return socksURL, key, nil
	}

	binaryPath, err := m.resolveBinary()
	if err != nil {
		log.Error("xray 不可用", logger.F("error", err))
		return "", "", err
	}

	maxLaunchRetries := 2
	if preferredPort > 0 {
		maxLaunchRetries = 1
	}
	var lastErr error
	attemptsUsed := 0
	for attempt := 1; attempt <= maxLaunchRetries; attempt++ {
		attemptsUsed = attempt
		socksURL, bridge, err := m.launchBridgeAttempt(log, key, binaryPath, outbounds, routes, preferredPort, dnsServers, pin, attempt)
		if err == nil {
			return socksURL, key, nil
		}
		if bridge != nil && bridge.Running {
			go m.watchBridge(bridge, key)
		}
		lastErr = err
		if !isRetryableXrayLaunchError(err) {
			break
		}
	}
	if !isRetryableXrayLaunchError(lastErr) {
		return "", "", lastErr
	}
	return "", "", fmt.Errorf("xray 启动失败（已尝试 %d 次）: %w", attemptsUsed, lastErr)
}

type xrayLaunchError struct {
	err       error
	retryable bool
}

func (e *xrayLaunchError) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *xrayLaunchError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func isRetryableXrayLaunchError(err error) bool {
	if err == nil {
		return false
	}
	var launchErr *xrayLaunchError
	if errors.As(err, &launchErr) {
		return launchErr.retryable
	}
	return true
}

func (m *XrayManager) launchBridgeAttempt(log *logger.Logger, key string, binaryPath string, outbounds []interface{}, routes []interface{}, preferredPort int, dnsServers string, pin bool, attempt int) (string, *XrayBridge, error) {
	port := preferredPort
	if port <= 0 {
		var err error
		port, err = nextAvailablePort()
		if err != nil {
			log.Error("端口分配失败", logger.F("error", err), logger.F("attempt", attempt))
			return "", nil, err
		}
	}
	cfgPath, err := m.buildRuntimeConfigWithRoute(key, outbounds, routes, port, dnsServers)
	if err != nil {
		log.Error("xray 配置生成失败", logger.F("error", err))
		return "", nil, err
	}
	stderrPath := filepath.Join(filepath.Dir(cfgPath), "xray-stderr.log")
	if err := m.testRuntimeConfig(binaryPath, cfgPath, stderrPath); err != nil {
		log.Error("xray 配置预检失败", logger.F("error", err), logger.F("attempt", attempt), logger.F("config", cfgPath))
		return "", nil, err
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
		log.Error("xray 启动失败", logger.F("error", err), logger.F("attempt", attempt))
		return "", nil, &xrayLaunchError{err: err, retryable: false}
	}

	bridge := &XrayBridge{
		NodeKey:    key,
		Port:       port,
		Cmd:        cmd,
		Pid:        cmd.Process.Pid,
		Running:    true,
		RefCount:   0,
		LastUsedAt: time.Now(),
		Outbounds:  cloneInterfaceSlice(outbounds),
		Routes:     cloneInterfaceSlice(routes),
		DNSServers: dnsServers,
	}
	bridge.startExitWatcher()
	log.Info("xray 内核进程已启动", logger.F("engine", "xray"), logger.F("key", key), logger.F("pid", bridge.Pid), logger.F("port", bridge.Port), logger.F("attempt", attempt))

	if err := m.waitBridgeReady(log, bridge, cfgPath, stderrPath, stderrFile, attempt); err != nil {
		return "", nil, err
	}

	if socksURL, reused := m.registerBridge(key, bridge, pin); reused {
		log.Info("复用已就绪 xray 桥接进程", logger.F("engine", "xray"), logger.F("key", key), logger.F("socks_url", socksURL))
		bridge.Stopping = true
		m.stopBridgeProcess(bridge)
		return socksURL, nil, nil
	}

	return fmt.Sprintf("socks5://127.0.0.1:%d", port), bridge, nil
}

func chainSocks5Outbound(hop chainSocks5Hop, tag string, nextTag string) map[string]interface{} {
	protocol := normalizeChainHopProtocol(hop.Protocol)
	if protocol == "http" || protocol == "https" {
		if protocol == "https" {
			hop.TLS = true
		}
		return chainHTTPOutbound(hop, tag, nextTag)
	}

	user := map[string]interface{}{}
	if strings.TrimSpace(hop.Username) != "" {
		user["user"] = strings.TrimSpace(hop.Username)
		if strings.TrimSpace(hop.Password) != "" {
			user["pass"] = hop.Password
		}
	}

	server := map[string]interface{}{
		"address": strings.TrimSpace(hop.Server),
		"port":    hop.Port,
	}
	if len(user) > 0 {
		server["users"] = []interface{}{user}
	}

	outbound := map[string]interface{}{
		"protocol": "socks",
		"tag":      tag,
		"settings": map[string]interface{}{
			"servers": []interface{}{server},
		},
	}
	if strings.TrimSpace(nextTag) != "" {
		outbound["proxySettings"] = map[string]interface{}{
			"tag": strings.TrimSpace(nextTag),
		}
	}
	return outbound
}

func chainHTTPOutbound(hop chainSocks5Hop, tag string, nextTag string) map[string]interface{} {
	user := map[string]interface{}{}
	if strings.TrimSpace(hop.Username) != "" {
		user["user"] = strings.TrimSpace(hop.Username)
		if strings.TrimSpace(hop.Password) != "" {
			user["pass"] = hop.Password
		}
	}

	server := map[string]interface{}{
		"address": strings.TrimSpace(hop.Server),
		"port":    hop.Port,
	}
	if len(user) > 0 {
		server["users"] = []interface{}{user}
	}

	outbound := map[string]interface{}{
		"protocol": "http",
		"tag":      tag,
		"settings": map[string]interface{}{
			"servers": []interface{}{server},
		},
	}
	if hop.TLS {
		outbound["streamSettings"] = map[string]interface{}{
			"security": "tls",
			"tlsSettings": map[string]interface{}{
				"serverName": strings.TrimSpace(hop.Server),
			},
		}
	}
	if strings.TrimSpace(nextTag) != "" {
		outbound["proxySettings"] = map[string]interface{}{
			"tag": strings.TrimSpace(nextTag),
		}
	}
	return outbound
}

func (m *XrayManager) waitBridgeReady(log *logger.Logger, bridge *XrayBridge, cfgPath string, stderrPath string, stderrFile *os.File, attempt int) error {
	if err := m.waitBridgeSocksReady(bridge, m.bridgeStartTimeout()); err != nil {
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
		message := "xray 桥接未就绪"
		if retryable {
			message = "xray 桥接未就绪，重试"
		}
		log.Error(message, logger.F("key", bridge.NodeKey), logger.F("error", err), logger.F("port", bridge.Port), logger.F("attempt", attempt), logger.F("retryable", retryable))
		time.Sleep(200 * time.Millisecond)
		return &xrayLaunchError{err: fmt.Errorf("%s", bridge.LastError), retryable: retryable}
	}
	if stderrFile != nil {
		stderrFile.Close()
	}
	return nil
}

func (m *XrayManager) isRetryableBridgeReadyError(err error, cfgPath string, stderrPath string) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	if !strings.Contains(message, "提前退出") {
		return true
	}
	tail := strings.ToLower(readLogTail(stderrPath, 1200))
	if tail == "" && strings.TrimSpace(cfgPath) != "" {
		tail = strings.ToLower(readLogTail(filepath.Join(filepath.Dir(cfgPath), "xray-error.log"), 1200))
	}
	return strings.Contains(tail, "address already in use") ||
		strings.Contains(tail, "only one usage of each socket") ||
		strings.Contains(tail, "bind:") ||
		strings.Contains(tail, "bind ")
}

func (m *XrayManager) testRuntimeConfig(binaryPath string, cfgPath string, stderrPath string) error {
	cmd := exec.Command(binaryPath, "run", "-test", "-c", cfgPath)
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
	if len(output) > 0 && stderrFile != nil {
		_, _ = stderrFile.Write(output)
	}
	return &xrayLaunchError{
		err:       fmt.Errorf("Xray 配置错误：%s", m.describeBridgeReadyError(err, cfgPath, stderrPath)),
		retryable: false,
	}
}

func (m *XrayManager) waitBridgeSocksReady(bridge *XrayBridge, timeout time.Duration) error {
	if bridge == nil {
		return fmt.Errorf("xray 桥接进程不存在")
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ready := make(chan error, 1)
	go func() {
		ready <- waitSocks5Ready("127.0.0.1", bridge.Port, timeout)
	}()
	select {
	case err := <-ready:
		return err
	case <-bridge.ExitDone:
		if err := bridge.exitErr(); err != nil {
			return fmt.Errorf("xray 进程提前退出: %w", err)
		}
		return fmt.Errorf("xray 进程提前退出")
	case <-deadline.C:
		return fmt.Errorf("xray socks5 端口 %d 启动超时", bridge.Port)
	}
}

func (m *XrayManager) bridgeStartTimeout() time.Duration {
	if m != nil && m.Config != nil && m.Config.ProxyCheck.BridgeStartTimeoutMs > 0 {
		return time.Duration(m.Config.ProxyCheck.BridgeStartTimeoutMs) * time.Millisecond
	}
	return time.Duration(defaultBridgeStartTimeoutMs) * time.Millisecond
}

func (m *XrayManager) describeBridgeReadyError(err error, cfgPath string, stderrPath string) string {
	if tail := readLogTail(stderrPath, 1200); tail != "" {
		return summarizeXrayError(tail)
	} else if cfgPath != "" {
		if tail := readLogTail(filepath.Join(filepath.Dir(cfgPath), "xray-error.log"), 1200); tail != "" {
			return summarizeXrayError(tail)
		}
	}
	if err != nil {
		return summarizeXrayError(err.Error())
	}
	return "未知错误"
}

func summarizeXrayError(raw string) string {
	text := strings.TrimSpace(raw)
	if text == "" {
		return "未知错误"
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(text, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			text = line
			break
		}
	}
	text = strings.TrimSpace(strings.TrimPrefix(text, "Failed to start:"))
	if idx := strings.LastIndex(text, " > "); idx >= 0 && idx+3 < len(text) {
		text = strings.TrimSpace(text[idx+3:])
	}
	text = strings.TrimPrefix(text, "infra/conf: ")
	text = strings.TrimPrefix(text, "main: ")
	if idx := strings.Index(text, "Try "); idx > 0 {
		text = strings.TrimSpace(text[:idx])
	}
	if idx := strings.Index(text, "请检查"); idx > 0 {
		text = strings.TrimSpace(text[:idx])
	}
	text = strings.Trim(text, " .；。")
	if strings.Contains(text, "allowInsecure") && strings.Contains(strings.ToLower(text), "removed") {
		return "字段 allowInsecure 已被当前 Xray 移除"
	}
	if text == "" {
		return "未知错误"
	}
	return text
}

func readLogTail(path string, max int) string {
	if strings.TrimSpace(path) == "" || max <= 0 {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return ""
	}
	text := strings.TrimSpace(string(data))
	if len(text) <= max {
		return text
	}
	return text[len(text)-max:]
}

func (m *XrayManager) logBridgeStartupError(log *logger.Logger, cfgPath string, stderrPath string) {
	if stderrContent, readErr := os.ReadFile(stderrPath); readErr == nil && len(stderrContent) > 0 {
		log.Error("xray stderr", logger.F("output", string(stderrContent)))
		return
	}

	errLogPath := filepath.Join(filepath.Dir(cfgPath), "xray-error.log")
	if errContent, readErr := os.ReadFile(errLogPath); readErr == nil && len(errContent) > 0 {
		log.Error("xray error.log", logger.F("output", string(errContent)))
	}
}
