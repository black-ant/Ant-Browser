package backend

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/logger"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ============================================================================
// 代理订阅源管理（独立建模 + 后端自动刷新）
// ============================================================================

// BrowserProxySourceList 列出所有订阅源
func (a *App) BrowserProxySourceList() []browser.ProxySource {
	if a.browserMgr.ProxySourceDAO == nil {
		return []browser.ProxySource{}
	}
	list, err := a.browserMgr.ProxySourceDAO.ListSources()
	if err != nil {
		logger.New("ProxySource").Error("查询订阅源失败", logger.F("error", err))
		return []browser.ProxySource{}
	}
	if list == nil {
		return []browser.ProxySource{}
	}
	return list
}

// BrowserProxySourceListOverrides 列出某订阅源的忽略/重命名记录
func (a *App) BrowserProxySourceListOverrides(sourceId string) []browser.ProxySourceOverride {
	if a.browserMgr.ProxySourceDAO == nil {
		return []browser.ProxySourceOverride{}
	}
	list, err := a.browserMgr.ProxySourceDAO.ListOverrides(strings.TrimSpace(sourceId))
	if err != nil || list == nil {
		return []browser.ProxySourceOverride{}
	}
	return list
}

// BrowserProxySourceUpsert 新增或更新订阅源
func (a *App) BrowserProxySourceUpsert(input browser.ProxySource) (*browser.ProxySource, error) {
	if a.browserMgr.ProxySourceDAO == nil {
		return nil, fmt.Errorf("数据库未就绪")
	}
	input.SourceURL = strings.TrimSpace(input.SourceURL)
	if input.SourceURL == "" {
		return nil, fmt.Errorf("订阅 URL 不能为空")
	}
	if strings.TrimSpace(input.SourceID) == "" {
		input.SourceID = "src-" + generateUUID()
	}
	if input.ImportStrategy != "replace" {
		input.ImportStrategy = "merge"
	}
	if strings.TrimSpace(input.SourceName) == "" {
		input.SourceName = hostFromURL(input.SourceURL)
	}
	input.RefreshIntervalM = clampRefreshInterval(input.RefreshIntervalM, input.AutoRefresh)
	if err := a.browserMgr.ProxySourceDAO.UpsertSource(input); err != nil {
		return nil, err
	}

	// 审计日志：记录代理源添加/更新
	log := logger.New("ProxySource")
	log.Info("安全审计：代理源配置已更新",
		logger.F("source_id", input.SourceID),
		logger.F("source_name", input.SourceName),
		logger.F("source_url", input.SourceURL))

	return a.browserMgr.ProxySourceDAO.GetSource(input.SourceID)
}

// BrowserProxySourceDelete 删除订阅源；deleteProxies=true 时一并删除该来源下的代理
func (a *App) BrowserProxySourceDelete(sourceId string, deleteProxies bool) error {
	if a.browserMgr.ProxySourceDAO == nil {
		return fmt.Errorf("数据库未就绪")
	}
	sourceId = strings.TrimSpace(sourceId)

	// 审计日志：记录代理源删除
	log := logger.New("ProxySource")
	log.Info("安全审计：代理源已删除",
		logger.F("source_id", sourceId),
		logger.F("delete_proxies", deleteProxies))

	if deleteProxies && a.browserMgr.ProxyDAO != nil {
		if proxies, err := a.browserMgr.ProxyDAO.List(); err == nil {
			for _, p := range proxies {
				if strings.TrimSpace(p.SourceID) == sourceId {
					_ = a.browserMgr.ProxyDAO.Delete(p.ProxyId)
				}
			}
		}
		if refreshed, err := a.browserMgr.ProxyDAO.List(); err == nil {
			a.config.Browser.Proxies = refreshed
		}
	}
	return a.browserMgr.ProxySourceDAO.DeleteSource(sourceId)
}

// BrowserProxySourceRefresh 手动刷新单个订阅源
func (a *App) BrowserProxySourceRefresh(sourceId string) error {
	return a.refreshProxySource(strings.TrimSpace(sourceId))
}

// BrowserProxySourceRefreshAll 手动刷新所有订阅源
func (a *App) BrowserProxySourceRefreshAll() {
	for _, s := range a.BrowserProxySourceList() {
		_ = a.refreshProxySource(s.SourceID)
	}
}

// BrowserProxySourceSetOverride 设置某节点的忽略/重命名记录
func (a *App) BrowserProxySourceSetOverride(sourceId string, nodeKey string, action string, customName string) error {
	if a.browserMgr.ProxySourceDAO == nil {
		return fmt.Errorf("数据库未就绪")
	}
	sourceId = strings.TrimSpace(sourceId)
	nodeKey = strings.TrimSpace(nodeKey)
	if sourceId == "" || nodeKey == "" {
		return fmt.Errorf("参数无效")
	}
	if action != "rename" {
		action = "ignore"
	}
	return a.browserMgr.ProxySourceDAO.UpsertOverride(browser.ProxySourceOverride{
		SourceID:   sourceId,
		NodeKey:    nodeKey,
		Action:     action,
		CustomName: strings.TrimSpace(customName),
	})
}

// BrowserProxySourceRemoveOverride 移除某节点的覆盖记录
func (a *App) BrowserProxySourceRemoveOverride(sourceId string, nodeKey string) error {
	if a.browserMgr.ProxySourceDAO == nil {
		return fmt.Errorf("数据库未就绪")
	}
	return a.browserMgr.ProxySourceDAO.DeleteOverride(strings.TrimSpace(sourceId), strings.TrimSpace(nodeKey))
}

// refreshProxySource 刷新单个订阅源并记录结果、发事件
func (a *App) refreshProxySource(sourceId string) error {
	log := logger.New("ProxySource")
	if sourceId == "" {
		return fmt.Errorf("订阅源 ID 不能为空")
	}
	if a.browserMgr.ProxySourceDAO == nil || a.browserMgr.ProxyDAO == nil {
		return fmt.Errorf("数据库未就绪")
	}

	// per-source 串行化：同一订阅源同时只允许一个刷新在执行（调度器与手动刷新会并发）。
	// 已在刷新中则跳过——进行中的那次完成后会发 proxy:source:refreshed 事件，调用方据此重载。
	if _, inFlight := a.proxyRefreshInFlight.LoadOrStore(sourceId, struct{}{}); inFlight {
		log.Info("订阅源正在刷新中，跳过本次重复刷新", logger.F("source_id", sourceId))
		return nil
	}
	defer a.proxyRefreshInFlight.Delete(sourceId)

	source, err := a.browserMgr.ProxySourceDAO.GetSource(sourceId)
	if err != nil {
		return err
	}

	refreshErr := a.doRefreshProxySource(source)
	refreshedAt := time.Now().Format(time.RFC3339)
	errMsg := ""
	if refreshErr != nil {
		errMsg = refreshErr.Error()
	}
	_ = a.browserMgr.ProxySourceDAO.UpdateRefreshResult(sourceId, refreshedAt, errMsg)

	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "proxy:source:refreshed", map[string]interface{}{
			"sourceId":      sourceId,
			"ok":            refreshErr == nil,
			"error":         errMsg,
			"lastRefreshAt": refreshedAt,
		})
	}
	if refreshErr != nil {
		log.Error("订阅刷新失败", logger.F("source_id", sourceId), logger.F("url", source.SourceURL), logger.F("error", refreshErr))
		a.recordActivity("import", "error", fmt.Sprintf("订阅刷新失败：%s", refreshErr.Error()), source.SourceName)
		return refreshErr
	}
	log.Info("订阅刷新成功", logger.F("source_id", sourceId), logger.F("url", source.SourceURL))
	a.recordActivity("import", "info", "订阅刷新成功", source.SourceName)
	return nil
}

// doRefreshProxySource 抓取 → 解析节点 → 套用覆盖 → 按策略合并 → 持久化（只触碰本来源代理）
func (a *App) doRefreshProxySource(source *browser.ProxySource) error {
	_, _, payload, err := a.fetchClashSubscriptionPayload(source.SourceURL)
	if err != nil {
		return err
	}
	nodes := extractClashProxyNodes(payload)
	if len(nodes) == 0 {
		return fmt.Errorf("订阅内容未解析到可用代理")
	}

	overrides := map[string]browser.ProxySourceOverride{}
	if list, err := a.browserMgr.ProxySourceDAO.ListOverrides(source.SourceID); err == nil {
		for _, o := range list {
			overrides[o.NodeKey] = o
		}
	}

	allProxies, err := a.browserMgr.ProxyDAO.List()
	if err != nil {
		return err
	}
	oldSourceProxies := make([]browser.Proxy, 0)
	for _, p := range allProxies {
		if strings.TrimSpace(p.SourceID) == source.SourceID {
			oldSourceProxies = append(oldSourceProxies, p)
		}
	}
	picker := newExistingProxyIDPicker(oldSourceProxies)

	// 保留存活代理原有 sort_order，新节点追加到该源末尾（避免刷新打乱排序）
	oldSortById := make(map[string]int, len(oldSourceProxies))
	nextSort := 0
	for _, p := range oldSourceProxies {
		oldSortById[p.ProxyId] = p.SortOrder
		if p.SortOrder >= nextSort {
			nextSort = p.SortOrder + 1
		}
	}

	prefix := strings.TrimSpace(source.NamePrefix)
	refreshedAt := time.Now().Format(time.RFC3339)
	newSourceProxies := make([]browser.Proxy, 0, len(nodes))

	for _, node := range nodes {
		nodeName := strings.TrimSpace(getMapString(node, "name"))
		if nodeName == "" {
			continue
		}
		nodeKey := nodeName // 订阅内原始名称作为节点稳定标识

		if ov, ok := overrides[nodeKey]; ok && ov.Action == "ignore" {
			continue // 用户忽略：刷新时不再加入
		}

		displayName := nodeName
		if ov, ok := overrides[nodeKey]; ok && ov.Action == "rename" && strings.TrimSpace(ov.CustomName) != "" {
			displayName = strings.TrimSpace(ov.CustomName)
		} else if prefix != "" {
			displayName = prefix + "-" + nodeName
		}

		configText, err := proxyNodeToConfig(node)
		if err != nil || strings.TrimSpace(configText) == "" {
			continue
		}

		proxyId := ""
		if source.ImportStrategy != "replace" {
			proxyId = picker(displayName, configText)
		}
		if proxyId == "" {
			proxyId = "proxy-" + generateUUID()
		}

		sortOrder, ok := oldSortById[proxyId]
		if !ok {
			sortOrder = nextSort
			nextSort++
		}

		newSourceProxies = append(newSourceProxies, browser.Proxy{
			ProxyId:                proxyId,
			ProxyName:              displayName,
			ProxyConfig:            configText,
			DnsServers:             strings.TrimSpace(source.DnsServers),
			GroupName:              strings.TrimSpace(source.GroupName),
			SourceID:               source.SourceID,
			SourceURL:              source.SourceURL,
			SourceNamePrefix:       prefix,
			SourceAutoRefresh:      source.AutoRefresh,
			SourceRefreshIntervalM: source.RefreshIntervalM,
			SourceLastRefreshAt:    refreshedAt,
			SortOrder:              sortOrder,
		})
	}

	if len(newSourceProxies) == 0 {
		return fmt.Errorf("订阅刷新后无可用代理（可能全部被忽略）")
	}

	// 事务化替换本来源代理：删除已消失项 + Upsert 新集合（存活行保留 proxyId 与测速/IP健康列）。
	// 整体原子，部分失败不会丢数据；不触碰其他来源或手动添加的代理。
	if err := a.browserMgr.ProxyDAO.ReplaceSourceProxies(source.SourceID, newSourceProxies); err != nil {
		return err
	}

	if refreshed, err := a.browserMgr.ProxyDAO.List(); err == nil {
		a.config.Browser.Proxies = refreshed
	}
	a.reconcileProfileProxyBindings()
	return nil
}

// seedProxySourcesFromProxies 首次运行时，从现有代理行的 source_* 字段聚合出受管订阅源。
// 仅在 proxy_sources 为空时执行，保留现有订阅为受管源，不丢数据。
func (a *App) seedProxySourcesFromProxies() {
	if a.browserMgr.ProxySourceDAO == nil || a.browserMgr.ProxyDAO == nil {
		return
	}
	existing, err := a.browserMgr.ProxySourceDAO.ListSources()
	if err != nil || len(existing) > 0 {
		return
	}
	proxies, err := a.browserMgr.ProxyDAO.List()
	if err != nil {
		return
	}
	seen := map[string]bool{}
	for _, p := range proxies {
		sid := strings.TrimSpace(p.SourceID)
		surl := strings.TrimSpace(p.SourceURL)
		if sid == "" || surl == "" || seen[sid] {
			continue
		}
		seen[sid] = true
		_ = a.browserMgr.ProxySourceDAO.UpsertSource(browser.ProxySource{
			SourceID:         sid,
			SourceURL:        surl,
			SourceName:       hostFromURL(surl),
			GroupName:        strings.TrimSpace(p.GroupName),
			NamePrefix:       strings.TrimSpace(p.SourceNamePrefix),
			DnsServers:       strings.TrimSpace(p.DnsServers),
			AutoRefresh:      p.SourceAutoRefresh,
			RefreshIntervalM: clampRefreshInterval(p.SourceRefreshIntervalM, p.SourceAutoRefresh),
			ImportStrategy:   "merge",
			LastRefreshAt:    strings.TrimSpace(p.SourceLastRefreshAt),
		})
	}
	if len(seen) > 0 {
		logger.New("ProxySource").Info("已从现有代理聚合订阅源", logger.F("count", len(seen)))
	}
}

// newExistingProxyIDPicker 按 (name+config) 优先、其次 name 复用旧 proxyId，保留实例绑定。
// 与前端 createExistingProxyIDPicker 行为一致。
func newExistingProxyIDPicker(old []browser.Proxy) func(name string, config string) string {
	exact := map[string][]string{}
	byName := map[string][]string{}
	for _, p := range old {
		ek := p.ProxyName + "|||" + p.ProxyConfig
		exact[ek] = append(exact[ek], p.ProxyId)
		byName[p.ProxyName] = append(byName[p.ProxyName], p.ProxyId)
	}
	return func(name string, config string) string {
		ek := name + "|||" + config
		if ids := exact[ek]; len(ids) > 0 {
			exact[ek] = ids[1:]
			return ids[0]
		}
		if ids := byName[name]; len(ids) > 0 {
			byName[name] = ids[1:]
			return ids[0]
		}
		return ""
	}
}

func clampRefreshInterval(intervalM int, autoRefresh bool) int {
	if intervalM < 0 {
		intervalM = 0
	}
	if intervalM > 24*60 {
		intervalM = 24 * 60
	}
	if autoRefresh && intervalM <= 0 {
		intervalM = 60
	}
	return intervalM
}

func hostFromURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Hostname() == "" {
		return strings.TrimSpace(raw)
	}
	host := parsed.Hostname()
	if strings.HasPrefix(strings.ToLower(host), "www.") {
		host = host[4:]
	}
	return host
}
