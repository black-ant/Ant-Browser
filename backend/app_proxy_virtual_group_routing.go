package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/logger"
	"strings"
	"time"
)

func (a *App) orderVirtualProxyCandidates(groupKey string, group browser.ProxyVirtualGroup, candidates []BrowserProxy, allProxies []BrowserProxy) ([]BrowserProxy, []string) {
	memberIDs := make([]string, 0, len(allProxies))
	seenIDs := make(map[string]struct{}, len(allProxies))
	for _, member := range browser.ExpandProxyVirtualGroup(group, allProxies) {
		proxyID := strings.ToLower(strings.TrimSpace(member.ProxyId))
		if proxyID == "" {
			continue
		}
		if _, exists := seenIDs[proxyID]; exists {
			continue
		}
		seenIDs[proxyID] = struct{}{}
		memberIDs = append(memberIDs, proxyID)
	}
	for _, candidate := range candidates {
		proxyID := strings.ToLower(strings.TrimSpace(candidate.ProxyId))
		if proxyID == "" {
			continue
		}
		if _, exists := seenIDs[proxyID]; exists {
			continue
		}
		seenIDs[proxyID] = struct{}{}
		memberIDs = append(memberIDs, proxyID)
	}
	if len(memberIDs) == 0 {
		return candidates, memberIDs
	}

	candidateByID := make(map[string]BrowserProxy, len(candidates))
	for _, candidate := range candidates {
		candidateByID[strings.ToLower(strings.TrimSpace(candidate.ProxyId))] = candidate
	}

	a.proxyVirtualGroupMu.Lock()
	startIndex := a.proxyVirtualGroupCursor[groupKey] % len(memberIDs)
	a.proxyVirtualGroupMu.Unlock()
	if browser.NormalizeProxyVirtualGroupStrategy(group.Strategy) == browser.ProxyVirtualGroupStrategySticky {
		startIndex = stickyVirtualGroupStartIndex(memberIDs, group, candidateByID, startIndex)
	}

	ordered := make([]BrowserProxy, 0, len(candidates))
	for offset := 0; offset < len(memberIDs); offset++ {
		memberID := memberIDs[(startIndex+offset)%len(memberIDs)]
		if candidate, available := candidateByID[memberID]; available {
			ordered = append(ordered, candidate)
		}
	}
	return ordered, memberIDs
}

// stickyVirtualGroupStartIndex 计算 sticky 策略的起始下标。
// 优先偏好节点（恢复后回切），其次当前选中节点；两者都不可用时顺延到下一个成员。
func stickyVirtualGroupStartIndex(memberIDs []string, group browser.ProxyVirtualGroup, candidateByID map[string]BrowserProxy, fallback int) int {
	for _, rawID := range []string{group.PreferredProxyId, group.SelectedProxyId} {
		proxyID := strings.ToLower(strings.TrimSpace(rawID))
		if proxyID == "" {
			continue
		}
		for index, memberID := range memberIDs {
			if memberID != proxyID {
				continue
			}
			if _, available := candidateByID[proxyID]; available {
				return index
			}
			return (index + 1) % len(memberIDs)
		}
	}
	return fallback
}

func (a *App) rotateVirtualProxyCandidates(groupKey string, candidates []BrowserProxy, memberIDs []string) ([]BrowserProxy, int) {
	if len(candidates) == 0 {
		return candidates, 0
	}
	a.proxyVirtualGroupMu.Lock()
	defer a.proxyVirtualGroupMu.Unlock()
	if a.proxyVirtualGroupCursor == nil {
		a.proxyVirtualGroupCursor = make(map[string]int)
	}
	orderedIDs := virtualProxyMemberOrder(memberIDs, candidates)
	if len(orderedIDs) == 0 {
		return candidates, 0
	}
	startIndex := a.proxyVirtualGroupCursor[groupKey] % len(orderedIDs)
	for offset := 0; offset < len(orderedIDs); offset++ {
		memberID := orderedIDs[(startIndex+offset)%len(orderedIDs)]
		for candidateIndex, candidate := range candidates {
			if strings.EqualFold(strings.TrimSpace(candidate.ProxyId), memberID) {
				ordered := append([]BrowserProxy{}, candidates[candidateIndex:]...)
				ordered = append(ordered, candidates[:candidateIndex]...)
				return ordered, candidateIndex
			}
		}
	}
	return candidates, 0
}

func (a *App) commitVirtualProxyCursor(groupKey string, selectedProxyID string, memberIDs []string, candidates []BrowserProxy) {
	selectedProxyID = strings.ToLower(strings.TrimSpace(selectedProxyID))
	if selectedProxyID == "" || len(memberIDs) == 0 && len(candidates) == 0 {
		return
	}
	orderedIDs := virtualProxyMemberOrder(memberIDs, candidates)
	if len(orderedIDs) == 0 {
		return
	}
	a.proxyVirtualGroupMu.Lock()
	defer a.proxyVirtualGroupMu.Unlock()
	if a.proxyVirtualGroupCursor == nil {
		a.proxyVirtualGroupCursor = make(map[string]int)
	}
	for index, memberID := range orderedIDs {
		if strings.EqualFold(strings.TrimSpace(memberID), selectedProxyID) {
			a.proxyVirtualGroupCursor[groupKey] = (index + 1) % len(orderedIDs)
			return
		}
	}
}

func virtualProxyMemberOrder(memberIDs []string, candidates []BrowserProxy) []string {
	orderedIDs := make([]string, 0, len(memberIDs)+len(candidates))
	seenIDs := make(map[string]struct{}, len(memberIDs)+len(candidates))
	for _, rawID := range memberIDs {
		proxyID := strings.ToLower(strings.TrimSpace(rawID))
		if proxyID == "" {
			continue
		}
		if _, exists := seenIDs[proxyID]; exists {
			continue
		}
		seenIDs[proxyID] = struct{}{}
		orderedIDs = append(orderedIDs, proxyID)
	}
	for _, candidate := range candidates {
		proxyID := strings.ToLower(strings.TrimSpace(candidate.ProxyId))
		if proxyID == "" {
			continue
		}
		if _, exists := seenIDs[proxyID]; exists {
			continue
		}
		seenIDs[proxyID] = struct{}{}
		orderedIDs = append(orderedIDs, proxyID)
	}
	return orderedIDs
}

// virtualGroupStartCandidates 固定绑定虚拟分组一次启动的候选集合（有序）。
type virtualGroupStartCandidates struct {
	group       browser.ProxyVirtualGroup
	ordered     []BrowserProxy
	memberOrder []string
	pool        []BrowserProxy
	// allDegraded 为 true 表示全组成员都处于降级/恢复观察状态，启动直接拒绝。
	allDegraded bool
}

func (c virtualGroupStartCandidates) empty() bool {
	return len(c.ordered) == 0
}

// pickVirtualGroupStartProxies 按策略返回本次启动可依次尝试的成员代理。
// 已降级（degraded）与恢复观察（recovering）的成员不进候选；全组降级时不再兜底，直接返回空候选。
func (a *App) pickVirtualGroupStartProxies(group browser.ProxyVirtualGroup, proxies []BrowserProxy, profileID string) virtualGroupStartCandidates {
	groupName := strings.TrimSpace(group.GroupName)
	if groupID := strings.TrimSpace(group.GroupId); groupID != "" {
		if latest, ok := a.findProxyVirtualGroupByID(groupID); ok {
			group = latest
		}
	}
	resolved := browser.ResolveProxyVirtualGroup(group, proxies)
	candidates := make([]BrowserProxy, 0, len(resolved.MemberProxyIds))
	memberSet := make(map[string]struct{}, len(resolved.MemberProxyIds))
	for _, memberID := range resolved.MemberProxyIds {
		memberSet[strings.ToLower(strings.TrimSpace(memberID))] = struct{}{}
	}
	for _, proxy := range proxies {
		if _, ok := memberSet[strings.ToLower(strings.TrimSpace(proxy.ProxyId))]; !ok {
			continue
		}
		if strings.TrimSpace(proxy.ProxyConfig) == "" {
			continue
		}
		candidates = append(candidates, proxy)
	}
	if len(candidates) == 0 {
		logger.New("Browser").Warn("虚拟分组没有可用成员代理",
			logger.F("profile_id", profileID),
			logger.F("virtual_group", groupName),
		)
		return virtualGroupStartCandidates{group: resolved}
	}

	groupKey := proxyVirtualGroupCursorKey(resolved.GroupId, resolved.GroupName)
	release := a.lockProxyVirtualGroup(groupKey)
	defer release()

	states := a.virtualGroupMemberStates(resolved.GroupId)
	usable := make([]BrowserProxy, 0, len(candidates))
	degradedCount := 0
	for _, candidate := range candidates {
		if a.virtualGroupMemberUsable(candidate, states) {
			usable = append(usable, candidate)
			continue
		}
		if a.memberStateDegraded(candidate.ProxyId, states) {
			degradedCount++
		}
	}

	// 只有"每个成员都被明确降级"才拒绝；全部未观测属于首次使用，仍按旧逻辑重试。
	if len(usable) == 0 && degradedCount == len(candidates) {
		a.reportVirtualGroupAllDegraded(resolved, profileID, candidates, states)
		return virtualGroupStartCandidates{group: resolved, allDegraded: true}
	}

	pool := usable
	if len(pool) == 0 {
		pool = candidates
	}
	ordered, memberOrder := a.orderVirtualProxyCandidates(groupKey, resolved, pool, proxies)
	return virtualGroupStartCandidates{
		group:       resolved,
		ordered:     ordered,
		memberOrder: memberOrder,
		pool:        pool,
	}
}

// pickVirtualGroupStartProxy 为固定绑定虚拟分组的实例挑选本次启动使用的成员代理。
// 复用动态代理相同的策略逻辑（round_robin 游标轮询 / sticky 固定选成员）。
func (a *App) pickVirtualGroupStartProxy(group browser.ProxyVirtualGroup, proxies []BrowserProxy, profileID string) (string, string, bool) {
	candidates := a.pickVirtualGroupStartProxies(group, proxies, profileID)
	if candidates.empty() {
		return "", "", false
	}
	chosen := candidates.ordered[0]
	a.commitVirtualGroupStartSelection(candidates, chosen.ProxyId)
	return strings.TrimSpace(chosen.ProxyId), strings.TrimSpace(chosen.ProxyConfig), true
}

// commitVirtualGroupStartSelection 在真正选中某个成员后推进游标并记录选点。
func (a *App) commitVirtualGroupStartSelection(candidates virtualGroupStartCandidates, proxyID string) {
	if candidates.empty() {
		return
	}
	groupKey := proxyVirtualGroupCursorKey(candidates.group.GroupId, candidates.group.GroupName)
	a.commitVirtualProxyCursor(groupKey, proxyID, candidates.memberOrder, candidates.pool)
	if browser.NormalizeProxyVirtualGroupStrategy(candidates.group.Strategy) == browser.ProxyVirtualGroupStrategySticky {
		a.rememberProxyVirtualGroupSelection(candidates.group, proxyID)
	}
}

// virtualGroupMemberStates 读取分组健康状态；健康开关关闭时返回 nil（保持旧行为）。
func (a *App) virtualGroupMemberStates(groupID string) map[string]browser.ProxyVirtualGroupMemberState {
	if !a.virtualGroupHealthEnabled() {
		return nil
	}
	return a.memberStatesForGroup(groupID)
}

// virtualGroupMemberUsable 判定成员是否可参与轮询。
// 有明确且属于当前连接栈的健康状态时以状态为准；否则回落到旧的测速新鲜度判定。
func (a *App) virtualGroupMemberUsable(candidate BrowserProxy, states map[string]browser.ProxyVirtualGroupMemberState) bool {
	if states != nil {
		proxyID := strings.ToLower(strings.TrimSpace(candidate.ProxyId))
		if row, exists := states[proxyID]; exists && row.State != browser.MemberStateUnknown {
			if a.memberStateMatchesConnector(row) {
				return browser.IsMemberStateUsable(row.State)
			}
			// 连接栈已变化：旧状态不可信，按未观测处理。
		}
	}
	if !candidate.LastTestOk || candidate.LastLatencyMs < 0 {
		return false
	}
	return isDynamicProxyTestFresh(candidate.LastTestedAt, time.Now(), a.dynamicProxyMaxTestAge())
}

// reportVirtualGroupAllDegraded 全组降级：记录明确告警并拒绝使用，不再兜底重试。
func (a *App) reportVirtualGroupAllDegraded(group browser.ProxyVirtualGroup, profileID string, members []BrowserProxy, states map[string]browser.ProxyVirtualGroupMemberState) {
	degradedIDs := make([]string, 0, len(members))
	for _, member := range members {
		proxyID := strings.ToLower(strings.TrimSpace(member.ProxyId))
		if row, exists := states[proxyID]; exists && row.State == browser.MemberStateDegraded {
			degradedIDs = append(degradedIDs, member.ProxyId)
		}
	}
	logger.New("Browser").Warn("虚拟分组成员全部降级，拒绝使用该分组",
		logger.F("profile_id", profileID),
		logger.F("group_id", group.GroupId),
		logger.F("virtual_group", group.GroupName),
		logger.F("member_count", len(members)),
		logger.F("degraded_count", len(degradedIDs)),
		logger.F("reason", "all_degraded"),
	)
	a.emitRuntimeEvent("proxy:virtual-group:health", map[string]interface{}{
		"groupId":     group.GroupId,
		"groupName":   group.GroupName,
		"allDegraded": true,
		"memberCount": len(members),
		"degradedIds": degradedIDs,
		"reason":      "all_degraded",
		"profileId":   profileID,
	})
}
