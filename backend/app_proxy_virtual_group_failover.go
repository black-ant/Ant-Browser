package backend

import (
	"context"
	"strings"
	"time"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/logger"
)

const (
	defaultRuntimeFailoverCooldownMinutes = 10
	defaultRuntimeFailoverMaxAttempts     = 3
	// 超过该倍数的冷却窗口没有新的失败，视为恢复稳定，重置尝试计数。
	runtimeFailoverCountResetFactor = 3
)

func (a *App) runtimeFailoverCooldown() time.Duration {
	minutes := defaultRuntimeFailoverCooldownMinutes
	if a != nil && a.config != nil && a.config.ProxyCheck.RuntimeFailoverCooldownMinutes > 0 {
		minutes = a.config.ProxyCheck.RuntimeFailoverCooldownMinutes
	}
	if minutes <= 0 {
		minutes = defaultRuntimeFailoverCooldownMinutes
	}
	return time.Duration(minutes) * time.Minute
}

func (a *App) runtimeFailoverMaxAttempts() int {
	if a != nil && a.config != nil && a.config.ProxyCheck.RuntimeFailoverMaxAttempts > 0 {
		return a.config.ProxyCheck.RuntimeFailoverMaxAttempts
	}
	return defaultRuntimeFailoverMaxAttempts
}

// scheduleVirtualGroupProxyFailure 桥接失效后的异步处置入口：降级失败成员并尝试切到健康成员重启实例。
func (a *App) scheduleVirtualGroupProxyFailure(profileID string, proxyID string, trigger string, cause error) {
	if a == nil || a.config == nil {
		return
	}
	if !a.config.ProxyCheck.IsRuntimeFailoverEnabled() {
		return
	}
	profileID = strings.TrimSpace(profileID)
	proxyID = strings.TrimSpace(proxyID)
	if profileID == "" || proxyID == "" {
		return
	}
	if err := a.startBackgroundTask(func(ctx context.Context) {
		a.handleVirtualGroupProxyFailure(profileID, proxyID, trigger, cause)
	}); err != nil {
		logger.New("VirtualGroup").Warn("提交虚拟分组故障切换任务失败",
			logger.F("profile_id", profileID),
			logger.F("proxy_id", proxyID),
			logger.F("error", err.Error()),
		)
	}
}

// handleVirtualGroupProxyFailure 只在实例当前绑定的虚拟分组内做故障切换。
// 非虚拟分组绑定、没有其他健康成员、处于冷却窗口时都直接跳过。
func (a *App) handleVirtualGroupProxyFailure(profileID string, proxyID string, trigger string, cause error) {
	log := logger.New("VirtualGroup")
	// 节点确实挂了：无论实例是否在跑，都先降级，避免下一次启动再撞上它。
	a.markProxyDegradedInAllGroups(proxyID, cause)

	group, ok := a.virtualGroupBoundToProfile(profileID)
	if !ok {
		return
	}
	// 实例已停止时不重启，避免"故障切换"变成意外启动浏览器。
	if !a.isProfileRunning(profileID) {
		return
	}

	proxies := a.getLatestProxies()
	candidates := a.pickVirtualGroupStartProxies(group, proxies, profileID)
	next := firstVirtualGroupCandidateExcept(candidates.ordered, proxyID)
	if next == nil {
		log.Warn("虚拟分组没有可切换的健康成员，保留当前实例",
			logger.F("profile_id", profileID),
			logger.F("group_id", group.GroupId),
			logger.F("virtual_group", group.GroupName),
			logger.F("failed_proxy_id", proxyID),
			logger.F("trigger", trigger),
		)
		return
	}

	attempt, allowed := a.beginRuntimeFailover(profileID)
	if !allowed {
		log.Warn("虚拟分组故障切换被冷却或次数上限拦截",
			logger.F("profile_id", profileID),
			logger.F("group_id", group.GroupId),
			logger.F("failed_proxy_id", proxyID),
			logger.F("attempt", attempt),
			logger.F("max_attempts", a.runtimeFailoverMaxAttempts()),
			logger.F("trigger", trigger),
		)
		return
	}

	log.Info("虚拟分组开始故障切换",
		logger.F("profile_id", profileID),
		logger.F("group_id", group.GroupId),
		logger.F("virtual_group", group.GroupName),
		logger.F("from_proxy_id", proxyID),
		logger.F("to_proxy_id", next.ProxyId),
		logger.F("attempt", attempt),
		logger.F("trigger", trigger),
	)

	if _, err := a.BrowserInstanceRestart(profileID); err != nil {
		log.Error("虚拟分组故障切换重启实例失败",
			logger.F("profile_id", profileID),
			logger.F("group_id", group.GroupId),
			logger.F("error", err.Error()),
		)
		return
	}

	a.emitRuntimeEvent("proxy:virtual-group:failover", map[string]interface{}{
		"profileId":   profileID,
		"groupId":     group.GroupId,
		"groupName":   group.GroupName,
		"fromProxyId": proxyID,
		"toProxyId":   strings.TrimSpace(next.ProxyId),
		"trigger":     trigger,
		"attempt":     attempt,
		"at":          time.Now().Format(time.RFC3339),
	})
}

// isProfileRunning 实例当前是否在运行。
func (a *App) isProfileRunning(profileID string) bool {
	if a == nil || a.browserMgr == nil {
		return false
	}
	a.browserMgr.Mutex.Lock()
	defer a.browserMgr.Mutex.Unlock()
	profile, exists := a.browserMgr.Profiles[profileID]
	return exists && profile != nil && profile.Running
}

// virtualGroupBoundToProfile 返回实例当前绑定的虚拟分组：
// 动态代理模式按 proxy_group_name 查，固定绑定按 proxy_id 查。
func (a *App) virtualGroupBoundToProfile(profileID string) (browser.ProxyVirtualGroup, bool) {
	if a == nil || a.browserMgr == nil {
		return browser.ProxyVirtualGroup{}, false
	}
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return browser.ProxyVirtualGroup{}, false
	}

	a.browserMgr.Mutex.Lock()
	profile, exists := a.browserMgr.Profiles[profileID]
	dynamic := false
	pinnedProxyID := ""
	groupName := ""
	if exists && profile != nil {
		dynamic = browser.IsDynamicProxyMode(profile.ProxyMode)
		pinnedProxyID = strings.TrimSpace(profile.ProxyId)
		groupName = strings.TrimSpace(profile.ProxyGroupName)
	}
	a.browserMgr.Mutex.Unlock()
	if !exists || profile == nil {
		return browser.ProxyVirtualGroup{}, false
	}

	if dynamic {
		return a.findProxyVirtualGroup(groupName)
	}
	return a.findProxyVirtualGroupByID(pinnedProxyID)
}

// beginRuntimeFailover 冷却与次数控制；返回本次是第几次尝试。
func (a *App) beginRuntimeFailover(profileID string) (int, bool) {
	cooldown := a.runtimeFailoverCooldown()
	maxAttempts := a.runtimeFailoverMaxAttempts()
	now := time.Now()

	a.proxyFailoverMu.Lock()
	defer a.proxyFailoverMu.Unlock()
	if a.proxyFailoverLastAt == nil {
		a.proxyFailoverLastAt = make(map[string]time.Time)
	}
	if a.proxyFailoverCount == nil {
		a.proxyFailoverCount = make(map[string]int)
	}

	count := a.proxyFailoverCount[profileID]
	if last := a.proxyFailoverLastAt[profileID]; !last.IsZero() {
		if now.Sub(last) > cooldown*time.Duration(runtimeFailoverCountResetFactor) {
			count = 0
		} else if now.Sub(last) < cooldown {
			return count, false
		}
	}
	if count >= maxAttempts {
		return count, false
	}

	a.proxyFailoverLastAt[profileID] = now
	a.proxyFailoverCount[profileID] = count + 1
	return count + 1, true
}

// markProxyDegradedInAllGroups 把某个代理在其所属的所有虚拟分组里直接降级。
// 桥接进程死亡是强信号，不再等待连续失败阈值。
func (a *App) markProxyDegradedInAllGroups(proxyID string, cause error) {
	proxyID = strings.TrimSpace(proxyID)
	if proxyID == "" || a == nil {
		return
	}
	_, byProxy := a.virtualGroupSnapshotData()
	for _, group := range byProxy[strings.ToLower(proxyID)] {
		a.markVirtualGroupMemberDegraded(group, proxyID, cause)
	}
}

func firstVirtualGroupCandidateExcept(candidates []BrowserProxy, excludeProxyID string) *BrowserProxy {
	excludeProxyID = strings.ToLower(strings.TrimSpace(excludeProxyID))
	for index := range candidates {
		if strings.EqualFold(strings.TrimSpace(candidates[index].ProxyId), excludeProxyID) {
			continue
		}
		return &candidates[index]
	}
	return nil
}
