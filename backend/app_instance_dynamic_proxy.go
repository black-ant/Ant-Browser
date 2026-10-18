package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/logger"
	"fmt"
	"sort"
	"strings"
	"time"
)

const dynamicProxyMaxTestAge = browser.DefaultProxySpeedInterval

func (a *App) resolveDynamicBrowserStartProxy(profileID string, profile *BrowserProfile, proxies []BrowserProxy) (string, profileProxyBridgeRef, bool, error) {
	proxies = a.proxySpeedResultsWithCurrentContext(proxies)
	groupName := browser.NormalizeProxyGroupName(profile.ProxyGroupName)
	virtualGroup, isVirtualGroup := a.findProxyVirtualGroup(groupName)
	candidates := selectDynamicBrowserProxyCandidates(proxies, groupName)
	if isVirtualGroup {
		virtualGroup = browser.ResolveProxyVirtualGroup(virtualGroup, proxies)
		candidates = selectVirtualDynamicBrowserProxyCandidates(proxies, virtualGroup)
	}
	log := logger.New("Browser")
	if len(candidates) == 0 {
		groupLabel := "全部分组"
		if groupName != "" {
			groupLabel = fmt.Sprintf("分组「%s」", groupName)
		}
		startErr := fmt.Errorf("实例启动失败：动态代理没有可用节点（%s）。请先对代理池节点测速，并确保至少一个节点在 %d 分钟内测速成功。", groupLabel, int(dynamicProxyMaxTestAge.Minutes()))
		profile.LastError = startErr.Error()
		return "", profileProxyBridgeRef{}, false, startErr
	}

	orderedCandidates := candidates
	virtualGroupKey := ""
	memberOrder := []string(nil)
	var releaseVirtualGroup func()
	selectionMode := "latency_priority"
	if isVirtualGroup {
		virtualGroupKey = proxyVirtualGroupCursorKey(virtualGroup.GroupId, virtualGroup.GroupName)
		releaseVirtualGroup = a.lockProxyVirtualGroup(virtualGroupKey)
		defer releaseVirtualGroup()
		if latestGroup, found := a.findProxyVirtualGroup(groupName); found {
			virtualGroup = browser.ResolveProxyVirtualGroup(latestGroup, proxies)
		}
		orderedCandidates, memberOrder = a.orderVirtualProxyCandidates(virtualGroupKey, virtualGroup, candidates, proxies)
		if browser.NormalizeProxyVirtualGroupStrategy(virtualGroup.Strategy) == browser.ProxyVirtualGroupStrategySticky {
			selectionMode = "virtual_sticky"
		} else {
			selectionMode = "virtual_round_robin"
		}
	}

	log.Info("动态代理开始尝试候选节点",
		logger.F("profile_id", profileID),
		logger.F("proxy_group_name", groupName),
		logger.F("selection_mode", selectionMode),
		logger.F("candidate_count", len(candidates)),
	)
	failures := make([]string, 0, len(candidates))
	for _, candidate := range orderedCandidates {
		candidateProfile := *profile
		candidateProfile.ProxyMode = browser.ProxyModeFixed
		candidateProfile.ProxyGroupName = ""
		candidateProfile.ProxyId = strings.TrimSpace(candidate.ProxyId)
		candidateProfile.ProxyConfig = strings.TrimSpace(candidate.ProxyConfig)
		proxyURL, bridgeRef, releaseBridge, err := a.resolveBrowserStartProxy(browserStartInput{ProfileID: profileID}, &candidateProfile)
		if err == nil {
			if isVirtualGroup {
				a.commitVirtualProxyCursor(virtualGroupKey, candidate.ProxyId, memberOrder, candidates)
				if browser.NormalizeProxyVirtualGroupStrategy(virtualGroup.Strategy) == browser.ProxyVirtualGroupStrategySticky {
					a.rememberProxyVirtualGroupSelection(virtualGroup, candidate.ProxyId)
				}
			}
			log.Info("动态代理节点已选择",
				logger.F("profile_id", profileID),
				logger.F("proxy_id", candidate.ProxyId),
				logger.F("proxy_group_name", candidate.GroupName),
				logger.F("last_latency_ms", candidate.LastLatencyMs),
			)
			return proxyURL, bridgeRef, releaseBridge, nil
		}
		label := strings.TrimSpace(candidate.ProxyName)
		if label == "" {
			label = strings.TrimSpace(candidate.ProxyId)
		}
		failures = append(failures, fmt.Sprintf("%s：%s", label, err.Error()))
		log.Warn("动态代理候选节点不可用，继续尝试下一个",
			logger.F("profile_id", profileID),
			logger.F("proxy_id", candidate.ProxyId),
			logger.F("error", err.Error()),
		)
	}

	const maxFailureDetails = 5
	if len(failures) > maxFailureDetails {
		failures = append(failures[:maxFailureDetails], fmt.Sprintf("其余 %d 个节点未展开", len(failures)-maxFailureDetails))
	}
	startErr := fmt.Errorf("实例启动失败：动态代理候选节点均不可用（%s）。请重新测速或检查代理内核配置。", strings.Join(failures, "；"))
	profile.LastError = startErr.Error()
	return "", profileProxyBridgeRef{}, false, startErr
}

func selectVirtualDynamicBrowserProxyCandidates(proxies []BrowserProxy, group browser.ProxyVirtualGroup) []BrowserProxy {
	memberIDs := make(map[string]struct{})
	for _, member := range browser.ExpandProxyVirtualGroup(group, proxies) {
		proxyID := strings.ToLower(strings.TrimSpace(member.ProxyId))
		if proxyID != "" {
			memberIDs[proxyID] = struct{}{}
		}
	}
	if len(memberIDs) == 0 {
		return []BrowserProxy{}
	}

	available := selectDynamicBrowserProxyCandidates(proxies, "")
	candidates := make([]BrowserProxy, 0, len(memberIDs))
	for _, candidate := range available {
		if _, exists := memberIDs[strings.ToLower(strings.TrimSpace(candidate.ProxyId))]; exists {
			candidates = append(candidates, candidate)
		}
	}
	return candidates
}

func selectDynamicBrowserProxyCandidates(proxies []BrowserProxy, groupName string) []BrowserProxy {
	groupName = browser.NormalizeProxyGroupName(groupName)
	checkedAt := time.Now()
	candidates := make([]BrowserProxy, 0, len(proxies))
	for _, item := range proxies {
		proxyID := strings.TrimSpace(item.ProxyId)
		if proxyID == "" || strings.EqualFold(proxyID, temporaryDirectProxyID) || strings.EqualFold(strings.TrimSpace(item.ProxyConfig), "direct://") {
			continue
		}
		if groupName != "" && !strings.EqualFold(strings.TrimSpace(item.GroupName), groupName) {
			continue
		}
		if !item.LastTestOk || item.LastLatencyMs < 0 || !isDynamicProxyTestFresh(item.LastTestedAt, checkedAt) {
			continue
		}
		candidates = append(candidates, item)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].LastLatencyMs != candidates[j].LastLatencyMs {
			return candidates[i].LastLatencyMs < candidates[j].LastLatencyMs
		}
		if candidates[i].SortOrder != candidates[j].SortOrder {
			return candidates[i].SortOrder < candidates[j].SortOrder
		}
		return strings.ToLower(strings.TrimSpace(candidates[i].ProxyId)) < strings.ToLower(strings.TrimSpace(candidates[j].ProxyId))
	})
	return candidates
}

func isDynamicProxyTestFresh(testedAt string, checkedAt time.Time) bool {
	testedTime, err := time.Parse(time.RFC3339, strings.TrimSpace(testedAt))
	if err != nil {
		return false
	}
	age := checkedAt.Sub(testedTime)
	return age >= 0 && age <= dynamicProxyMaxTestAge
}
