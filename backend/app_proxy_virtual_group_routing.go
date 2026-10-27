package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/logger"
	"strings"
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
		selectedID := strings.ToLower(strings.TrimSpace(group.SelectedProxyId))
		if selectedID != "" {
			for index, memberID := range memberIDs {
				if memberID == selectedID {
					startIndex = index
					if _, available := candidateByID[selectedID]; !available {
						startIndex = (index + 1) % len(memberIDs)
					}
					break
				}
			}
		}
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

// pickVirtualGroupStartProxy 为固定绑定虚拟分组的实例挑选本次启动使用的成员代理。
// 复用动态代理相同的策略逻辑（round_robin 游标轮询 / sticky 固定选成员）。
func (a *App) pickVirtualGroupStartProxy(group browser.ProxyVirtualGroup, proxies []BrowserProxy, profileID string) (string, string, bool) {
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
		return "", "", false
	}

	groupKey := proxyVirtualGroupCursorKey(resolved.GroupId, resolved.GroupName)
	release := a.lockProxyVirtualGroup(groupKey)
	defer release()

	ordered, memberOrder := a.orderVirtualProxyCandidates(groupKey, resolved, candidates, proxies)
	if len(ordered) == 0 {
		return "", "", false
	}
	chosen := ordered[0]
	a.commitVirtualProxyCursor(groupKey, chosen.ProxyId, memberOrder, candidates)
	if browser.NormalizeProxyVirtualGroupStrategy(resolved.Strategy) == browser.ProxyVirtualGroupStrategySticky {
		a.rememberProxyVirtualGroupSelection(resolved, chosen.ProxyId)
	}
	return strings.TrimSpace(chosen.ProxyId), strings.TrimSpace(chosen.ProxyConfig), true
}
