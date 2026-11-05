package proxy

import (
	"ant-chrome/backend/internal/logger"
	"time"
)

func (m *SingBoxManager) cleanupLoop() {
	ticker := time.NewTicker(singBoxBridgeCleanupInterval)
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
		if now.Sub(bridge.LastUsedAt) < singBoxBridgeIdleTTL {
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
		log.Info("回收空闲 sing-box 桥接进程", logger.F("key", bridge.NodeKey[:8]), logger.F("pid", bridge.Pid))
		m.stopBridgeProcess(bridge)
	}
}
