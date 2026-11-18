package backend

import (
	"fmt"
	"strings"
	"time"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/logger"
	"ant-chrome/backend/internal/proxy"
)

func nowRFC3339() string {
	return time.Now().Format(time.RFC3339)
}

func browserNormalizeProxies(proxies []BrowserProxy) []BrowserProxy {
	return proxy.NormalizeBrowserProxies(proxies, generateUUID)
}

// 持久错误文案前缀，便于前端识别与将来统一替换。
const proxyBindClearedPrefix = "代理绑定已清理："

// ProxyReferenceHealReport 一次引用自愈的统计结果。
type ProxyReferenceHealReport struct {
	ProfileBindingsCleared int `json:"profileBindingsCleared"`
	FetchProxiesCleared    int `json:"fetchProxiesCleared"`
	ProfileGroupsCleared   int `json:"profileGroupsCleared"`
}

// loadProxyPoolForHeal 读取用于引用自愈的代理池。
// 读取失败时返回 ok=false：此时"空池"不代表代理真的被删除，
// 绝不能据此清空实例绑定或拉取代理，否则数据库瞬时故障会抹掉全部绑定配置。
func (a *App) loadProxyPoolForHeal() ([]BrowserProxy, bool) {
	if a == nil || a.browserMgr == nil {
		return nil, false
	}
	if a.browserMgr.ProxyDAO != nil {
		list, err := a.browserMgr.ProxyDAO.List()
		if err != nil {
			logger.New("Browser").Error("读取代理目录失败，跳过代理引用自愈", logger.F("error", err.Error()))
			return nil, false
		}
		return list, true
	}
	if a.config == nil {
		return nil, false
	}
	return a.config.Browser.Proxies, true
}

// ReconcileProxyReferences 幂等地把指向已删除对象的引用修复成明确态。
// 不拦截删除、不回退直连、不猜测替代节点；先按快照重绑，失败才清理并写持久错误。
func (a *App) ReconcileProxyReferences() ProxyReferenceHealReport {
	report := ProxyReferenceHealReport{}
	if a == nil || a.browserMgr == nil {
		return report
	}

	report.ProfileBindingsCleared = a.healProfileProxyBindings()
	// 拉取代理自愈只修改传入的池；由本入口负责持久化。
	// SaveBrowserProxies 内已包含同样的自愈，这里只在启动/重载路径需要通过总入口触发。
	if pool, ok := a.loadProxyPoolForHeal(); ok {
		report.FetchProxiesCleared, _ = a.healFetchProxyReferences(pool)
		if report.FetchProxiesCleared > 0 {
			_ = a.saveProxiesWithoutHeal(pool)
		}
	}
	report.ProfileGroupsCleared = a.healProfileGroupRefs()

	if report.ProfileBindingsCleared > 0 || report.FetchProxiesCleared > 0 || report.ProfileGroupsCleared > 0 {
		logger.New("Browser").Info("代理引用自愈完成",
			logger.F("bindings_cleared", report.ProfileBindingsCleared),
			logger.F("fetch_cleared", report.FetchProxiesCleared),
			logger.F("groups_cleared", report.ProfileGroupsCleared),
		)
		a.emitRuntimeEvent("proxy:references:healed", map[string]interface{}{
			"profileBindingsCleared": report.ProfileBindingsCleared,
			"fetchProxiesCleared":    report.FetchProxiesCleared,
			"profileGroupsCleared":   report.ProfileGroupsCleared,
		})
	}
	return report
}

// healProfileProxyBindings 修复实例的代理绑定。
// 返回清理的实例数。虚拟分组绑定是合法形态，必须先排除。
func (a *App) healProfileProxyBindings() int {
	a.browserMgr.Mutex.Lock()
	defer a.browserMgr.Mutex.Unlock()

	pool, ok := a.loadProxyPoolForHeal()
	if !ok {
		return 0
	}
	proxyIDs := make(map[string]struct{}, len(pool))
	for _, row := range pool {
		id := strings.ToLower(strings.TrimSpace(row.ProxyId))
		if id != "" {
			proxyIDs[id] = struct{}{}
		}
	}

	changedCount := 0
	clearedCount := 0
	for _, profile := range a.browserMgr.Profiles {
		if profile == nil {
			continue
		}
		if browser.IsDynamicProxyMode(profile.ProxyMode) || strings.EqualFold(strings.TrimSpace(profile.ProxyMode), browser.ProxyModeLocal) {
			// 模式已切走固定代理时，历史失效错误态不再适用。
			if profile.ProxyBindError != "" {
				profile.ProxyBindError = ""
				profile.UpdatedAt = nowRFC3339()
				changedCount++
			}
			continue
		}
		currentID := strings.TrimSpace(profile.ProxyId)
		if currentID == "" || currentID == "__direct__" || currentID == "__local__" {
			continue
		}
		if _, exists := proxyIDs[strings.ToLower(currentID)]; exists {
			if profile.ProxyBindError != "" {
				profile.ProxyBindError = ""
				profile.UpdatedAt = nowRFC3339()
				changedCount++
			}
			continue
		}
		if _, isGroup := a.findProxyVirtualGroupByID(currentID); isGroup {
			if profile.ProxyBindError != "" {
				profile.ProxyBindError = ""
				profile.UpdatedAt = nowRFC3339()
				changedCount++
			}
			continue
		}

		// 先走现有 7 级快照重绑；能找回的节点不清理。
		changed, boundInPool, _ := a.browserMgr.ResolveProfileProxyBinding(profile)
		if changed {
			changedCount++
		}
		if boundInPool {
			if profile.ProxyBindError != "" {
				profile.ProxyBindError = ""
				profile.UpdatedAt = nowRFC3339()
				changedCount++
			}
			continue
		}

		oldName := strings.TrimSpace(profile.ProxyBindName)
		if oldName == "" {
			oldName = currentID
		}
		profile.ProxyId = ""
		profile.ProxyConfig = ""
		profile.ProxyBindError = fmt.Sprintf("%s代理「%s」已删除或不可用，绑定已清理，请重新选择代理（不会自动改为直连）", proxyBindClearedPrefix, oldName)
		profile.UpdatedAt = nowRFC3339()
		changedCount++
		clearedCount++
		logger.New("Browser").Info("实例代理绑定已清理",
			logger.F("profile_id", profile.ProfileId),
			logger.F("profile_name", profile.ProfileName),
			logger.F("old_proxy", currentID),
		)
	}
	if changedCount > 0 {
		if err := a.browserMgr.SaveProfiles(); err != nil {
			logger.New("Browser").Error("实例代理绑定自愈持久化失败", logger.F("error", err.Error()))
			return 0
		}
	}
	return clearedCount
}

// healFetchProxyReferences 清理失效的订阅拉取代理。
// 拉取代理只影响订阅内容获取，回退直连是安全的；失败原因写入 SourceLastRefreshError。
func (a *App) healFetchProxyReferences(pool []BrowserProxy) (int, bool) {
	proxyIDs := make(map[string]struct{}, len(pool))
	for _, row := range pool {
		id := strings.ToLower(strings.TrimSpace(row.ProxyId))
		if id != "" {
			proxyIDs[id] = struct{}{}
		}
	}

	clearedCount := 0
	for index, row := range pool {
		fetchID := strings.TrimSpace(row.SourceFetchProxyID)
		if fetchID == "" {
			continue
		}
		if _, exists := proxyIDs[strings.ToLower(fetchID)]; exists {
			continue
		}
		// 只清理本行自己的失效引用：同源各行可能配置了不同的拉取代理，
		// 不能按 sourceKey 连带清空其它行的有效配置或写假错误。
		sourceKey := proxySourceKey(row)
		if sourceKey == "" {
			sourceKey = strings.TrimSpace(row.SourceURL)
		}
		pool[index].SourceFetchProxyID = ""
		pool[index].SourceLastRefreshError = "拉取代理已删除，已回退直连拉取；如订阅无法访问请重新选择拉取代理"
		clearedCount++
		logger.New("ProxySource").Info("订阅拉取代理已失效，回退直连",
			logger.F("source", sourceKey),
			logger.F("fetch_proxy_id", fetchID),
		)
	}
	return clearedCount, clearedCount > 0
}

// healProfileGroupRefs 清理实例上指向已删业务分组的 GroupId。
func (a *App) healProfileGroupRefs() int {
	if a.browserMgr.GroupDAO == nil {
		return 0
	}
	groups, err := a.browserMgr.GroupDAO.List()
	if err != nil {
		return 0
	}
	groupIDs := make(map[string]struct{}, len(groups))
	for _, group := range groups {
		id := strings.TrimSpace(group.GroupId)
		if id != "" {
			groupIDs[id] = struct{}{}
		}
	}

	a.browserMgr.Mutex.Lock()
	defer a.browserMgr.Mutex.Unlock()
	changedCount := 0
	clearedCount := 0
	for _, profile := range a.browserMgr.Profiles {
		if profile == nil {
			continue
		}
		groupID := strings.TrimSpace(profile.GroupId)
		if groupID == "" {
			continue
		}
		if _, exists := groupIDs[groupID]; exists {
			continue
		}
		profile.GroupId = ""
		profile.UpdatedAt = nowRFC3339()
		changedCount++
		clearedCount++
	}
	if changedCount > 0 {
		if err := a.browserMgr.SaveProfiles(); err != nil {
			logger.New("Browser").Error("实例分组引用自愈持久化失败", logger.F("error", err.Error()))
			return 0
		}
	}
	return clearedCount
}

// saveProxiesWithoutHeal 保存代理池而不触发 ReconcileProxyReferences，避免自愈内部递归。
func (a *App) saveProxiesWithoutHeal(proxies []BrowserProxy) error {
	normalized := browserNormalizeProxies(proxies)
	if a.browserMgr.ProxyDAO != nil {
		if err := a.browserMgr.ProxyDAO.ReplaceAll(normalized); err != nil {
			return err
		}
	} else if a.config != nil {
		if err := config.SaveProxies(a.resolveAppPath("proxies.yaml"), normalized); err != nil {
			return err
		}
	}
	if a.config != nil {
		a.config.Browser.Proxies = normalized
	}
	return nil
}
