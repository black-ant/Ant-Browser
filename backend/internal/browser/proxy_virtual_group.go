package browser

import "strings"

type ProxyVirtualGroupDAO interface {
	List() ([]ProxyVirtualGroup, error)
	GetByName(groupName string) (*ProxyVirtualGroup, error)
	Upsert(group ProxyVirtualGroup) error
	Delete(groupId string) error
}

const (
	ProxyVirtualGroupStrategyRoundRobin = "round_robin"
	ProxyVirtualGroupStrategySticky     = "sticky"
)

func NormalizeProxyVirtualGroup(group ProxyVirtualGroup) ProxyVirtualGroup {
	group.GroupId = strings.TrimSpace(group.GroupId)
	group.GroupName = strings.TrimSpace(group.GroupName)
	group.Strategy = NormalizeProxyVirtualGroupStrategy(group.Strategy)
	group.SelectedProxyId = strings.TrimSpace(group.SelectedProxyId)
	group.MemberProxyIds = normalizeProxyVirtualGroupProxyIDs(group.MemberProxyIds)
	group.MemberGroupNames = normalizeProxyVirtualGroupMemberNames(group.MemberGroupNames)
	return group
}

func NormalizeProxyVirtualGroupStrategy(strategy string) string {
	switch strings.ToLower(strings.TrimSpace(strategy)) {
	case ProxyVirtualGroupStrategySticky:
		return ProxyVirtualGroupStrategySticky
	default:
		return ProxyVirtualGroupStrategyRoundRobin
	}
}

func normalizeProxyVirtualGroupProxyIDs(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	result := make([]string, 0, len(ids))
	for _, rawID := range ids {
		id := strings.TrimSpace(rawID)
		if id == "" {
			continue
		}
		key := strings.ToLower(id)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, id)
	}
	return result
}

func normalizeProxyVirtualGroupMemberNames(names []string) []string {
	seen := make(map[string]struct{}, len(names))
	result := make([]string, 0, len(names))
	for _, rawName := range names {
		name := strings.TrimSpace(rawName)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, name)
	}
	return result
}

func ExpandProxyVirtualGroup(group ProxyVirtualGroup, proxies []Proxy) []Proxy {
	group = NormalizeProxyVirtualGroup(group)
	if len(proxies) == 0 {
		return []Proxy{}
	}

	if len(group.MemberProxyIds) == 0 && len(group.MemberGroupNames) == 0 {
		return []Proxy{}
	}

	proxyByID := make(map[string]Proxy, len(proxies))
	for _, item := range proxies {
		proxyID := strings.ToLower(strings.TrimSpace(item.ProxyId))
		if proxyID == "" {
			continue
		}
		proxyByID[proxyID] = item
	}
	result := make([]Proxy, 0, len(group.MemberProxyIds))
	seenProxyIDs := make(map[string]struct{}, len(group.MemberProxyIds))
	appendProxy := func(item Proxy) {
		proxyID := strings.ToLower(strings.TrimSpace(item.ProxyId))
		if proxyID == "" {
			return
		}
		if _, exists := seenProxyIDs[proxyID]; exists {
			return
		}
		seenProxyIDs[proxyID] = struct{}{}
		result = append(result, item)
	}
	for _, memberProxyID := range group.MemberProxyIds {
		if item, exists := proxyByID[strings.ToLower(strings.TrimSpace(memberProxyID))]; exists {
			appendProxy(item)
		}
	}
	if len(group.MemberGroupNames) == 0 {
		return result
	}

	selectedGroupNames := make(map[string]struct{}, len(group.MemberGroupNames))
	for _, memberGroupName := range group.MemberGroupNames {
		selectedGroupNames[strings.ToLower(strings.TrimSpace(memberGroupName))] = struct{}{}
	}
	for _, item := range proxies {
		groupName := strings.ToLower(strings.TrimSpace(item.GroupName))
		if _, selected := selectedGroupNames[groupName]; !selected {
			continue
		}
		appendProxy(item)
	}
	return result
}

func ResolveProxyVirtualGroup(group ProxyVirtualGroup, proxies []Proxy) ProxyVirtualGroup {
	group = NormalizeProxyVirtualGroup(group)
	if len(group.MemberProxyIds) == 0 && len(group.MemberGroupNames) > 0 {
		for _, item := range ExpandProxyVirtualGroup(group, proxies) {
			if proxyID := strings.TrimSpace(item.ProxyId); proxyID != "" {
				group.MemberProxyIds = append(group.MemberProxyIds, proxyID)
			}
		}
		group.MemberProxyIds = normalizeProxyVirtualGroupProxyIDs(group.MemberProxyIds)
	}
	return group
}
