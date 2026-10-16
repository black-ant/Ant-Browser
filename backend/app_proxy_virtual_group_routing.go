package backend

import (
	"ant-chrome/backend/internal/browser"
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
