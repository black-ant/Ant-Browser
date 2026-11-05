package proxy

import (
	"ant-chrome/backend/internal/logger"
	"errors"
	"fmt"
	"time"
)

var errXrayBridgeRestartNotNeeded = errors.New("xray 桥接已无须恢复")

func cloneInterfaceSlice(items []interface{}) []interface{} {
	if len(items) == 0 {
		return nil
	}
	cloned := make([]interface{}, len(items))
	copy(cloned, items)
	return cloned
}

func (m *XrayManager) restartPinnedBridge(log *logger.Logger, key string, bridge *XrayBridge) error {
	if bridge == nil {
		return fmt.Errorf("xray 桥接不存在")
	}
	unlockLaunch := m.lockLaunchForKey(key)
	defer unlockLaunch()

	m.mu.Lock()
	current := m.Bridges[key]
	if current != bridge || bridge.Stopping || bridge.RefCount <= 0 {
		m.mu.Unlock()
		return errXrayBridgeRestartNotNeeded
	}
	refCount := bridge.RefCount
	m.mu.Unlock()

	if refCount <= 0 {
		return fmt.Errorf("xray 桥接无活动引用")
	}
	if len(bridge.Outbounds) == 0 || len(bridge.Routes) == 0 {
		return fmt.Errorf("xray 桥接缺少重启上下文")
	}

	log.Warn("xray 桥接进程退出，尝试同端口恢复",
		logger.F("key", key),
		logger.F("port", bridge.Port),
		logger.F("ref_count", refCount),
	)
	binaryPath, err := m.resolveBinary()
	if err != nil {
		return err
	}
	socksURL, restarted, err := m.launchBridgeAttempt(
		log,
		key,
		binaryPath,
		cloneInterfaceSlice(bridge.Outbounds),
		cloneInterfaceSlice(bridge.Routes),
		bridge.Port,
		bridge.DNSServers,
		false,
		1,
	)
	if err != nil {
		return err
	}
	if restarted == nil {
		return fmt.Errorf("xray 桥接恢复异常: 未返回新进程")
	}
	restarted.LastUsedAt = time.Now()
	log.Info("xray 桥接已同端口恢复",
		logger.F("key", key),
		logger.F("port", restarted.Port),
		logger.F("pid", restarted.Pid),
		logger.F("socks_url", socksURL),
		logger.F("ref_count", refCount),
	)
	go m.watchBridge(restarted, key)
	return nil
}
