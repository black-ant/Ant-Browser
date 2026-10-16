package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/logger"
	"fmt"
	"strings"
	"sync"
)

func (a *App) BrowserProxyVirtualGroupList() []BrowserProxyVirtualGroup {
	if a == nil || a.browserMgr == nil {
		return []BrowserProxyVirtualGroup{}
	}
	if a.browserMgr.ProxyVirtualGroupDAO != nil {
		groups, err := a.browserMgr.ProxyVirtualGroupDAO.List()
		if err != nil {
			return []BrowserProxyVirtualGroup{}
		}
		return groups
	}
	if a.config != nil {
		proxies := a.latestProxiesForVirtualGroup()
		groups := make([]BrowserProxyVirtualGroup, 0, len(a.config.Browser.ProxyVirtualGroups))
		for _, group := range a.config.Browser.ProxyVirtualGroups {
			groups = append(groups, browser.ResolveProxyVirtualGroup(group, proxies))
		}
		return groups
	}
	return []BrowserProxyVirtualGroup{}
}

func (a *App) SaveBrowserProxyVirtualGroup(group BrowserProxyVirtualGroup) error {
	if a == nil || a.browserMgr == nil {
		return fmt.Errorf("应用未初始化，无法保存虚拟代理分组")
	}
	group = browser.NormalizeProxyVirtualGroup(group)
	group, err := validateProxyVirtualGroupMembers(group, a.latestProxiesForVirtualGroup())
	if err != nil {
		return err
	}
	previousCursorKey := proxyVirtualGroupCursorKey(group.GroupId, group.GroupName)
	if group.GroupId == "" {
		group.GroupId = generateUUID()
	}
	if group.GroupName == "" {
		return fmt.Errorf("虚拟代理分组名称不能为空")
	}
	if len(group.MemberProxyIds) == 0 {
		return fmt.Errorf("虚拟代理分组至少需要关联一个代理节点")
	}
	for _, existing := range a.BrowserProxyVirtualGroupList() {
		if existing.GroupId != group.GroupId && strings.EqualFold(strings.TrimSpace(existing.GroupName), group.GroupName) {
			return fmt.Errorf("虚拟代理分组名称已存在：%s", group.GroupName)
		}
	}
	for _, physicalGroupName := range a.listLatestProxyGroups() {
		if strings.EqualFold(strings.TrimSpace(physicalGroupName), group.GroupName) {
			return fmt.Errorf("虚拟分组名称不能与实体代理分组重名：%s", group.GroupName)
		}
	}

	if a.browserMgr.ProxyVirtualGroupDAO != nil {
		if err := a.browserMgr.ProxyVirtualGroupDAO.Upsert(group); err != nil {
			return err
		}
	} else if a.config != nil {
		groups := make([]BrowserProxyVirtualGroup, 0, len(a.config.Browser.ProxyVirtualGroups)+1)
		updated := false
		for _, existing := range a.config.Browser.ProxyVirtualGroups {
			if existing.GroupId == group.GroupId {
				groups = append(groups, group)
				updated = true
				continue
			}
			groups = append(groups, existing)
		}
		if !updated {
			groups = append(groups, group)
		}
		a.config.Browser.ProxyVirtualGroups = groups
		if err := a.config.Save(a.resolveAppPath("config.yaml")); err != nil {
			return fmt.Errorf("保存虚拟代理分组配置失败: %w", err)
		}
	} else {
		return fmt.Errorf("应用未初始化，无法保存虚拟代理分组")
	}

	a.resetProxyVirtualGroupCursorByKey(previousCursorKey)
	a.resetProxyVirtualGroupCursorForGroup(group)
	logger.New("Browser").Info("虚拟代理分组已保存", logger.F("group_id", group.GroupId), logger.F("group_name", group.GroupName), logger.F("member_count", len(group.MemberProxyIds)))
	return nil
}

func (a *App) DeleteBrowserProxyVirtualGroup(groupId string) error {
	if a == nil || a.browserMgr == nil {
		return fmt.Errorf("应用未初始化，无法删除虚拟代理分组")
	}
	groupId = strings.TrimSpace(groupId)
	if groupId == "" {
		return fmt.Errorf("虚拟代理分组 ID 不能为空")
	}
	if a.browserMgr.ProxyVirtualGroupDAO != nil {
		if err := a.browserMgr.ProxyVirtualGroupDAO.Delete(groupId); err != nil {
			return err
		}
	} else if a.config != nil {
		groups := make([]BrowserProxyVirtualGroup, 0, len(a.config.Browser.ProxyVirtualGroups))
		for _, group := range a.config.Browser.ProxyVirtualGroups {
			if group.GroupId != groupId {
				groups = append(groups, group)
			}
		}
		a.config.Browser.ProxyVirtualGroups = groups
		if err := a.config.Save(a.resolveAppPath("config.yaml")); err != nil {
			return fmt.Errorf("保存虚拟代理分组配置失败: %w", err)
		}
	} else {
		return fmt.Errorf("应用未初始化，无法删除虚拟代理分组")
	}
	a.resetProxyVirtualGroupCursor(groupId)
	return nil
}

func (a *App) findProxyVirtualGroup(groupName string) (browser.ProxyVirtualGroup, bool) {
	groupName = strings.TrimSpace(groupName)
	if groupName == "" {
		return browser.ProxyVirtualGroup{}, false
	}
	if a != nil && a.browserMgr != nil && a.browserMgr.ProxyVirtualGroupDAO != nil {
		if group, err := a.browserMgr.ProxyVirtualGroupDAO.GetByName(groupName); err == nil && group != nil {
			return *group, true
		}
	}
	for _, group := range a.BrowserProxyVirtualGroupList() {
		if strings.EqualFold(strings.TrimSpace(group.GroupName), groupName) {
			return browser.NormalizeProxyVirtualGroup(group), true
		}
	}
	return browser.ProxyVirtualGroup{}, false
}

func (a *App) rememberProxyVirtualGroupSelection(group browser.ProxyVirtualGroup, proxyID string) {
	if a == nil || a.browserMgr == nil || browser.NormalizeProxyVirtualGroupStrategy(group.Strategy) != browser.ProxyVirtualGroupStrategySticky {
		return
	}
	proxyID = strings.TrimSpace(proxyID)
	if proxyID == "" {
		return
	}
	group = browser.NormalizeProxyVirtualGroup(group)
	group.SelectedProxyId = proxyID
	if a.browserMgr.ProxyVirtualGroupDAO != nil {
		if err := a.browserMgr.ProxyVirtualGroupDAO.Upsert(group); err != nil {
			logger.New("Browser").Warn("保存虚拟代理分组保持节点失败", logger.F("group_id", group.GroupId), logger.F("proxy_id", proxyID), logger.F("error", err))
		}
		return
	}
	if a.config == nil {
		return
	}
	updated := false
	for index := range a.config.Browser.ProxyVirtualGroups {
		item := &a.config.Browser.ProxyVirtualGroups[index]
		if (group.GroupId != "" && item.GroupId == group.GroupId) || (group.GroupId == "" && strings.EqualFold(strings.TrimSpace(item.GroupName), group.GroupName)) {
			item.SelectedProxyId = proxyID
			updated = true
			break
		}
	}
	if !updated {
		return
	}
	if err := a.config.Save(a.resolveAppPath("config.yaml")); err != nil {
		logger.New("Browser").Warn("保存虚拟代理分组保持节点配置失败", logger.F("group_id", group.GroupId), logger.F("proxy_id", proxyID), logger.F("error", err))
	}
}

func (a *App) resetProxyVirtualGroupCursor(groupID string) {
	if a == nil {
		return
	}
	a.resetProxyVirtualGroupCursorByKey(proxyVirtualGroupCursorKey(groupID, ""))
}

func (a *App) resetProxyVirtualGroupCursorForGroup(group browser.ProxyVirtualGroup) {
	if a == nil {
		return
	}
	a.resetProxyVirtualGroupCursorByKey(proxyVirtualGroupCursorKey(group.GroupId, group.GroupName))
}

func (a *App) resetAllProxyVirtualGroupCursors() {
	if a == nil {
		return
	}
	for _, group := range a.BrowserProxyVirtualGroupList() {
		a.resetProxyVirtualGroupCursorForGroup(group)
	}
}

func (a *App) resetProxyVirtualGroupCursorByKey(key string) {
	if a == nil || strings.TrimSpace(key) == "" {
		return
	}
	groupLock := a.proxyVirtualGroupLock(key)
	groupLock.Lock()
	defer groupLock.Unlock()
	a.proxyVirtualGroupMu.Lock()
	defer a.proxyVirtualGroupMu.Unlock()
	if a.proxyVirtualGroupCursor == nil {
		a.proxyVirtualGroupCursor = make(map[string]int)
	}
	delete(a.proxyVirtualGroupCursor, key)
}

func (a *App) lockProxyVirtualGroup(key string) func() {
	groupLock := a.proxyVirtualGroupLock(key)
	groupLock.Lock()
	return groupLock.Unlock
}

func (a *App) proxyVirtualGroupLock(key string) *sync.Mutex {
	a.proxyVirtualGroupMu.Lock()
	defer a.proxyVirtualGroupMu.Unlock()
	if a.proxyVirtualGroupLocks == nil {
		a.proxyVirtualGroupLocks = make(map[string]*sync.Mutex)
	}
	if groupLock, exists := a.proxyVirtualGroupLocks[key]; exists {
		return groupLock
	}
	groupLock := &sync.Mutex{}
	a.proxyVirtualGroupLocks[key] = groupLock
	return groupLock
}

func proxyVirtualGroupCursorKey(groupID, groupName string) string {
	if groupID = strings.ToLower(strings.TrimSpace(groupID)); groupID != "" {
		return "id:" + groupID
	}
	if groupName = strings.ToLower(strings.TrimSpace(groupName)); groupName != "" {
		return "name:" + groupName
	}
	return "name:"
}

func (a *App) latestProxiesForVirtualGroup() []browser.Proxy {
	if a == nil || a.browserMgr == nil {
		return []browser.Proxy{}
	}
	var fallback []browser.Proxy
	if a.config != nil {
		fallback = a.config.Browser.Proxies
	}
	return browser.LatestProxiesWithFallback(a.browserMgr.ProxyDAO, fallback)
}

func (a *App) listLatestProxyGroups() []string {
	if a == nil || a.browserMgr == nil {
		return []string{}
	}
	if groups := browser.ListProxyGroups(a.browserMgr.ProxyDAO); len(groups) > 0 {
		return groups
	}
	seen := make(map[string]struct{})
	groups := make([]string, 0)
	for _, item := range a.latestProxiesForVirtualGroup() {
		name := strings.TrimSpace(item.GroupName)
		key := strings.ToLower(name)
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		groups = append(groups, name)
	}
	return groups
}

func validateProxyVirtualGroupMembers(group browser.ProxyVirtualGroup, proxies []browser.Proxy) (browser.ProxyVirtualGroup, error) {
	group = browser.ResolveProxyVirtualGroup(group, proxies)
	proxyByID := make(map[string]browser.Proxy, len(proxies))
	for _, item := range proxies {
		proxyID := strings.TrimSpace(item.ProxyId)
		if proxyID == "" {
			continue
		}
		proxyByID[strings.ToLower(proxyID)] = item
	}
	normalizedIDs := make([]string, 0, len(group.MemberProxyIds))
	for _, rawProxyID := range group.MemberProxyIds {
		proxyID := strings.TrimSpace(rawProxyID)
		item, exists := proxyByID[strings.ToLower(proxyID)]
		if !exists {
			return browser.ProxyVirtualGroup{}, fmt.Errorf("虚拟代理分组关联的代理节点不存在：%s", proxyID)
		}
		if strings.EqualFold(strings.TrimSpace(item.ProxyId), "__direct__") || strings.EqualFold(strings.TrimSpace(item.ProxyConfig), "direct://") {
			return browser.ProxyVirtualGroup{}, fmt.Errorf("虚拟代理分组不能关联直连节点：%s", proxyID)
		}
		normalizedIDs = append(normalizedIDs, strings.TrimSpace(item.ProxyId))
	}
	group.MemberProxyIds = normalizedIDs
	group.Strategy = browser.NormalizeProxyVirtualGroupStrategy(group.Strategy)
	if group.Strategy != browser.ProxyVirtualGroupStrategySticky {
		group.SelectedProxyId = ""
	} else {
		selectedProxyID := strings.ToLower(strings.TrimSpace(group.SelectedProxyId))
		selectedExists := false
		for _, memberProxyID := range group.MemberProxyIds {
			if strings.EqualFold(strings.TrimSpace(memberProxyID), selectedProxyID) {
				selectedExists = true
				break
			}
		}
		if !selectedExists {
			group.SelectedProxyId = ""
		}
	}
	return browser.NormalizeProxyVirtualGroup(group), nil
}
