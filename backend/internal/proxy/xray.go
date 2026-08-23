package proxy

import (
	"ant-chrome/backend/internal/config"
	"fmt"
	"strings"
	"sync"
	"time"
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
	launchLocks  map[string]*bridgeLaunchLock
	stopCh       chan struct{}
	stopOnce     sync.Once
}

// NewXrayManager 创建 Xray 管理器
func NewXrayManager(cfg *config.Config, appRoot string) *XrayManager {
	manager := &XrayManager{
		Config:      cfg,
		AppRoot:     appRoot,
		Bridges:     make(map[string]*XrayBridge),
		launchLocks: make(map[string]*bridgeLaunchLock),
		stopCh:      make(chan struct{}),
	}
	go manager.cleanupLoop()
	return manager
}

// ValidateProxyConfig 验证代理配置是否支持
// 返回: supported bool, errorMsg string
func ValidateProxyConfig(proxyConfig string, proxies []config.BrowserProxy, proxyId string) (bool, string) {
	src := strings.TrimSpace(proxyConfig)
	preferredKernel := ""
	if proxyId != "" {
		found := false
		for _, item := range proxies {
			if strings.EqualFold(item.ProxyId, proxyId) {
				src = strings.TrimSpace(item.ProxyConfig)
				preferredKernel = strings.TrimSpace(item.PreferredKernel)
				found = true
				break
			}
		}
		if !found {
			if src == "" {
				return false, fmt.Sprintf("代理链路不可用：代理池节点已不存在（proxyId=%s）。可能因订阅刷新后节点下线或被删除，请重新选择代理后再启动。", proxyId)
			}
		}
	}
	if resolution, err := ResolveProxyKernel(src, proxies, "", preferredKernel); err != nil {
		return false, fmt.Sprintf("代理配置解析失败: %v", err)
	} else if len(resolution.SupportedKernels) == 0 {
		return false, "代理配置无效"
	}
	if src == "" {
		return true, ""
	}
	if strings.EqualFold(src, "direct://") {
		return true, ""
	}
	l := strings.ToLower(src)
	if strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "https://") || strings.HasPrefix(l, "socks5://") {
		return true, ""
	}
	if IsChainSocks5Proxy(src) {
		if _, err := ParseChainSocks5Config(src); err != nil {
			return false, fmt.Sprintf("链式代理配置解析失败: %v", err)
		}
		return true, ""
	}
	if IsChainProxy(src) {
		cfg, err := ParseChainProxyConfig(src)
		if err != nil {
			return false, fmt.Sprintf("链式代理配置解析失败: %v", err)
		}
		if _, err := resolveChainFront(cfg, proxies, proxyId); err != nil {
			return false, err.Error()
		}
		return true, ""
	}
	if IsSingBoxProtocol(src) {
		if _, err := BuildSingBoxOutbound(src); err != nil {
			return false, fmt.Sprintf("代理配置解析失败: %v", err)
		}
		return true, ""
	}
	if IsMihomoOnlyProtocol(src) {
		if err := validateMihomoOnlyProtocol(src); err != nil {
			return false, fmt.Sprintf("代理配置解析失败: %v", err)
		}
		return true, ""
	}

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
	src := resolveProxyConfig(proxyConfig, proxies, proxyId)
	if src == "" {
		return false
	}
	l := strings.ToLower(src)
	if strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "https://") || strings.HasPrefix(l, "socks5://") {
		return false
	}
	if IsChainSocks5Proxy(src) {
		return true
	}
	if IsChainProxy(src) {
		return true
	}
	if IsSingBoxProtocol(src) {
		return false
	}
	if IsMihomoOnlyProtocol(src) {
		return false
	}
	if strings.HasPrefix(l, "hysteria://") || strings.HasPrefix(l, "hysteria2://") {
		return false
	}
	if strings.HasPrefix(l, "vmess://") || strings.HasPrefix(l, "vless://") || strings.HasPrefix(l, "trojan://") || strings.HasPrefix(l, "ss://") {
		return true
	}
	if strings.HasPrefix(l, "clash://") || strings.Contains(l, "type:") || strings.Contains(l, "proxies:") {
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
	bridges := make([]*XrayBridge, 0, len(m.Bridges))
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
