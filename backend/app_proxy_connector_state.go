package backend

import (
	"errors"
	"fmt"
	"strings"
)

type proxyVirtualGroupMemberStateAllClearer interface {
	DeleteAll() error
}

// invalidateProxyConnectorRuntimeState 连接栈切换时只清理运行时派生的健康状态。
// 持久化与内存中的测速结果、IP 健康是已有数据，保持原样；读取侧按
// last_test_connector_type 判定归属，显式标记为其它连接栈的结果不被采信，
// 无来源的历史结果按当前栈兼容。
func (a *App) invalidateProxyConnectorRuntimeState() error {
	if a == nil {
		return nil
	}

	a.stopSpeedScheduler()
	defer func() {
		a.ensureSpeedScheduler()
		if a.speedScheduler != nil {
			a.speedScheduler.RunOnce()
		}
	}()

	err := a.clearPersistedVirtualGroupMemberStates()
	a.invalidateVirtualGroupSnapshot()
	a.invalidateVirtualGroupMemberStateGate()
	return err
}

func (a *App) clearPersistedVirtualGroupMemberStates() error {
	if a == nil || a.browserMgr == nil || a.browserMgr.ProxyVirtualGroupMemberStateDAO == nil {
		return nil
	}
	dao := a.browserMgr.ProxyVirtualGroupMemberStateDAO
	if clearer, ok := dao.(proxyVirtualGroupMemberStateAllClearer); ok {
		return clearer.DeleteAll()
	}

	states, err := dao.List()
	if err != nil {
		return fmt.Errorf("读取虚拟分组成员健康状态失败: %w", err)
	}
	groupIDs := make(map[string]string)
	for _, state := range states {
		groupID := strings.TrimSpace(state.GroupId)
		if groupID == "" {
			continue
		}
		groupIDs[strings.ToLower(groupID)] = groupID
	}
	failures := make([]error, 0)
	for _, groupID := range groupIDs {
		if err := dao.DeleteByGroup(groupID); err != nil {
			failures = append(failures, fmt.Errorf("清除虚拟分组 %s 成员健康状态失败: %w", groupID, err))
		}
	}
	return errors.Join(failures...)
}
