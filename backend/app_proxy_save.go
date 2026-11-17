package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/logger"
	"ant-chrome/backend/internal/proxy"
	"fmt"
	"strings"
)

func (a *App) SaveBrowserProxies(proxies []BrowserProxy) error {
	log := logger.New("Browser")
	normalized := proxy.NormalizeBrowserProxies(proxies, generateUUID)
	// 在持久化前完成拉取代理自愈：一次 ReplaceAll 写回，避免二次全量写与竞态窗口。
	a.healFetchProxyReferences(normalized)

	if a.browserMgr.ProxyDAO != nil {
		if err := a.browserMgr.ProxyDAO.ReplaceAll(normalized); err != nil {
			log.Error("代理列表事务保存失败", logger.F("error", err))
			return err
		}
		log.Info("代理列表已保存到数据库", logger.F("count", len(normalized)))
	} else {
		if err := config.SaveProxies(a.resolveAppPath("proxies.yaml"), normalized); err != nil {
			log.Error("代理列表保存失败", logger.F("error", err))
			return err
		}
	}
	a.config.Browser.Proxies = normalized
	a.healProfileProxyBindings()
	if err := a.pruneProxyVirtualGroupMembers(normalized); err != nil {
		return err
	}
	return nil
}

func (a *App) pruneProxyVirtualGroupMembers(proxies []BrowserProxy) error {
	if a == nil || a.browserMgr == nil {
		return nil
	}
	validProxyIDs := make(map[string]string, len(proxies))
	for _, item := range proxies {
		proxyID := strings.TrimSpace(item.ProxyId)
		if proxyID == "" || strings.EqualFold(proxyID, "__direct__") || strings.EqualFold(strings.TrimSpace(item.ProxyConfig), "direct://") {
			continue
		}
		validProxyIDs[strings.ToLower(proxyID)] = proxyID
	}

	groups := a.BrowserProxyVirtualGroupList()
	if len(groups) == 0 {
		return nil
	}
	configGroups := make([]BrowserProxyVirtualGroup, 0, len(groups))
	configChanged := false
	for _, group := range groups {
		group = browser.NormalizeProxyVirtualGroup(group)
		// 动态分组（MemberGroupNames）的成员由代理池实时展开，不能把展开结果写回
		// MemberProxyIds，否则下次刷新时动态组会退化成静态快照。
		isDynamicMembers := len(group.MemberGroupNames) > 0
		filteredIDs := make([]string, 0, len(group.MemberProxyIds))
		for _, memberProxyID := range group.MemberProxyIds {
			if canonicalID, exists := validProxyIDs[strings.ToLower(strings.TrimSpace(memberProxyID))]; exists {
				filteredIDs = append(filteredIDs, canonicalID)
			}
		}
		selectedProxyID := strings.ToLower(strings.TrimSpace(group.SelectedProxyId))
		_, selectedExists := validProxyIDs[selectedProxyID]
		preferredProxyID := strings.ToLower(strings.TrimSpace(group.PreferredProxyId))
		_, preferredExists := validProxyIDs[preferredProxyID]
		changed := len(filteredIDs) != len(group.MemberProxyIds) ||
			(selectedProxyID != "" && !selectedExists) ||
			(preferredProxyID != "" && !preferredExists)
		if !changed {
			// 动态组即使成员列表未变，也要按展开结果清理孤儿状态行。
			if isDynamicMembers {
				effective := browser.ExpandProxyVirtualGroup(group, proxies)
				stateGroup := group
				stateGroup.MemberProxyIds = make([]string, 0, len(effective))
				for _, item := range effective {
					stateGroup.MemberProxyIds = append(stateGroup.MemberProxyIds, item.ProxyId)
				}
				a.pruneVirtualGroupMemberStates(stateGroup)
			}
			configGroups = append(configGroups, group)
			continue
		}
		if isDynamicMembers {
			// 保留空 MemberProxyIds，让 ResolveProxyVirtualGroup 继续按组名动态展开。
			group.MemberProxyIds = nil
		} else {
			group.MemberProxyIds = filteredIDs
		}
		if !selectedExists {
			group.SelectedProxyId = ""
		}
		if !preferredExists {
			group.PreferredProxyId = ""
		}
		configGroups = append(configGroups, group)
		a.resetProxyVirtualGroupCursorForGroup(group)
		if a.browserMgr.ProxyVirtualGroupDAO != nil {
			if err := a.browserMgr.ProxyVirtualGroupDAO.Upsert(group); err != nil {
				return fmt.Errorf("清理虚拟代理分组「%s」失效关联失败: %w", group.GroupName, err)
			}
			stateGroup := group
			if isDynamicMembers {
				effective := browser.ExpandProxyVirtualGroup(group, proxies)
				stateGroup.MemberProxyIds = make([]string, 0, len(effective))
				for _, item := range effective {
					stateGroup.MemberProxyIds = append(stateGroup.MemberProxyIds, item.ProxyId)
				}
			}
			a.pruneVirtualGroupMemberStates(stateGroup)
		} else {
			configChanged = true
		}
	}
	if configChanged && a.config != nil {
		a.config.Browser.ProxyVirtualGroups = configGroups
		if err := a.config.Save(a.resolveAppPath("config.yaml")); err != nil {
			return fmt.Errorf("保存清理后的虚拟代理分组失败: %w", err)
		}
	}
	a.invalidateVirtualGroupSnapshot()
	return nil
}
