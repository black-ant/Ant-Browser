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

	a.config.Browser.Proxies = normalized

	if a.browserMgr.ProxyDAO != nil {
		if err := a.browserMgr.ProxyDAO.DeleteAll(); err != nil {
			log.Error("清空代理表失败", logger.F("error", err))
			return err
		}
		for _, item := range normalized {
			if err := a.browserMgr.ProxyDAO.Upsert(item); err != nil {
				log.Error("代理保存失败", logger.F("proxy_id", item.ProxyId), logger.F("error", err))
				return err
			}
		}
		log.Info("代理列表已保存到数据库", logger.F("count", len(normalized)))
	} else {
		if err := config.SaveProxies(a.resolveAppPath("proxies.yaml"), normalized); err != nil {
			log.Error("代理列表保存失败", logger.F("error", err))
			return err
		}
	}
	a.reconcileProfileProxyBindings()
	return a.pruneProxyVirtualGroupMembers(normalized)
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
		filteredIDs := make([]string, 0, len(group.MemberProxyIds))
		for _, memberProxyID := range group.MemberProxyIds {
			if canonicalID, exists := validProxyIDs[strings.ToLower(strings.TrimSpace(memberProxyID))]; exists {
				filteredIDs = append(filteredIDs, canonicalID)
			}
		}
		selectedProxyID := strings.ToLower(strings.TrimSpace(group.SelectedProxyId))
		_, selectedExists := validProxyIDs[selectedProxyID]
		if len(filteredIDs) == len(group.MemberProxyIds) && (selectedProxyID == "" || selectedExists) {
			configGroups = append(configGroups, group)
			continue
		}
		group.MemberProxyIds = filteredIDs
		if !selectedExists {
			group.SelectedProxyId = ""
		}
		configGroups = append(configGroups, group)
		a.resetProxyVirtualGroupCursorForGroup(group)
		if a.browserMgr.ProxyVirtualGroupDAO != nil {
			if err := a.browserMgr.ProxyVirtualGroupDAO.Upsert(group); err != nil {
				return fmt.Errorf("清理虚拟代理分组「%s」失效关联失败: %w", group.GroupName, err)
			}
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
	return nil
}
