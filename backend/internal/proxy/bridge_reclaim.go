package proxy

import (
	"strings"

	"ant-chrome/backend/internal/logger"
)

// 测速、IP 健康等短时探测通过引用计数占用桥接，探测结束后调用 ReclaimBridgeIfIdle：
// 桥接无人占用（RefCount==0）时立即结束进程，避免批量探测后大量内核进程滞留到空闲 TTL；
// 有浏览器实例或其他探测仍在使用时不动，交给既有空闲回收协程。

func (m *XrayManager) ReclaimBridgeIfIdle(key string) {
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

	logger.New("Xray").Info("探测结束，回收空闲桥接进程", logger.F("key", bridge.NodeKey), logger.F("pid", bridge.Pid))
	m.stopBridgeProcess(bridge)
}

func (m *SingBoxManager) ReclaimBridgeIfIdle(key string) {
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

	logKey := bridge.NodeKey
	if len(logKey) > 8 {
		logKey = logKey[:8]
	}
	logger.New("SingBox").Info("探测结束，回收空闲桥接进程", logger.F("key", logKey), logger.F("pid", bridge.Pid))
	m.stopBridgeProcess(bridge)
}

func (m *ClashManager) ReclaimBridgeIfIdle(key string) {
	key = strings.TrimSpace(key)
	if m == nil || key == "" {
		return
	}
	m.mu.Lock()
	bridge, ok := m.NodeBridges[key]
	if !ok || bridge == nil || !bridge.Running || bridge.RefCount > 0 {
		m.mu.Unlock()
		return
	}
	delete(m.NodeBridges, key)
	if bridge.Cmd != nil && bridge.Cmd.Process != nil {
		_ = bridge.Cmd.Process.Kill()
	}
	m.mu.Unlock()

	logKey := key
	if len(logKey) > 8 {
		logKey = logKey[:8]
	}
	logger.New("Mihomo").Info("探测结束，回收空闲桥接进程", logger.F("key", logKey), logger.F("pid", bridge.Pid))
}
