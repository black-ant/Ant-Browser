package backend

import (
	"fmt"
	"strings"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/logger"
)

// 异常节点被跳过的原因：仍被实例绑定或虚拟分组引用时不允许自动删除，
// 否则实例会变成悬空绑定（只能靠配置快照启动），分组也会静默少成员。
const (
	proxyMissingSkipInstance     = "instance"
	proxyMissingSkipVirtualGroup = "virtual_group"
	proxyMissingSkipFetchProxy   = "fetch_proxy"
)

// ProxyMissingCleanupSkip 清理异常节点时被跳过的行及引用来源。
type ProxyMissingCleanupSkip struct {
	ProxyId   string `json:"proxyId"`
	ProxyName string `json:"proxyName"`
	SourceId  string `json:"sourceId"`
	Reason    string `json:"reason"`
	Detail    string `json:"detail"`
}

// ProxyMissingCleanupResult 一次异常节点清理的统计结果。
type ProxyMissingCleanupResult struct {
	SourceId  string                    `json:"sourceId"`
	Scanned   int                       `json:"scanned"`   // 标记异常的行数（按 sourceId 过滤后）
	Deletable int                       `json:"deletable"` // 无任何引用的行数
	Deleted   int                       `json:"deleted"`   // 实际删除的行数（dryRun 时为 0）
	Skipped   []ProxyMissingCleanupSkip `json:"skipped"`   // 仍被实例/虚拟分组引用、已跳过的行
	DryRun    bool                      `json:"dryRun"`
}

// BrowserProxyCleanupMissing 清理订阅刷新后标记为异常的节点（订阅里已不存在的行）。
//
// 安全边界：只删除没有被任何实例绑定、也没有被任何虚拟分组引用的行。
// 被引用的行会跳过并返回原因，避免制造悬空绑定或让分组静默少成员。
// sourceId 为空表示清理全部订阅源；dryRun=true 只统计不落盘，供前端确认弹窗预览。
func (a *App) BrowserProxyCleanupMissing(sourceId string, dryRun bool) (ProxyMissingCleanupResult, error) {
	sourceId = strings.TrimSpace(sourceId)
	empty := ProxyMissingCleanupResult{SourceId: sourceId, DryRun: dryRun, Skipped: []ProxyMissingCleanupSkip{}}
	if a == nil {
		return empty, fmt.Errorf("应用未初始化")
	}

	// 与订阅刷新共用一把锁：清理是"读池 → 过滤 → 全量保存"，
	// 与定时刷新交错会互相覆盖对方的合并结果。
	a.proxySourceRefreshMu.Lock()
	defer a.proxySourceRefreshMu.Unlock()

	pool := a.getLatestProxies()
	instanceRefs, groupRefs, fetchRefs := a.collectProxyMissingReferences(pool)
	remaining, result := planProxyMissingCleanup(pool, sourceId, instanceRefs, groupRefs, fetchRefs)
	result.SourceId = sourceId
	result.DryRun = dryRun

	if dryRun || result.Deletable == 0 {
		return result, nil
	}
	if err := a.SaveBrowserProxies(remaining); err != nil {
		return result, fmt.Errorf("清理异常节点失败: %w", err)
	}
	result.Deleted = result.Deletable
	logger.New("ProxySource").Info("订阅异常节点已清理",
		logger.F("source_id", sourceId),
		logger.F("scanned", result.Scanned),
		logger.F("deleted", result.Deleted),
		logger.F("skipped", len(result.Skipped)),
	)
	a.emitRuntimeEvent("proxy:sources:missing:cleaned", map[string]interface{}{
		"sourceId": result.SourceId,
		"scanned":  result.Scanned,
		"deleted":  result.Deleted,
		"skipped":  len(result.Skipped),
	})
	return result, nil
}

// planProxyMissingCleanup 纯函数：按引用情况决定哪些异常行可以删除。
// 返回保留后的代理池（全部行，含未被选中的源与正常行）与统计结果。
func planProxyMissingCleanup(
	pool []BrowserProxy,
	sourceId string,
	instanceRefs map[string]string,
	groupRefs map[string]string,
	fetchRefs map[string]string,
) ([]BrowserProxy, ProxyMissingCleanupResult) {
	result := ProxyMissingCleanupResult{Skipped: []ProxyMissingCleanupSkip{}}
	remaining := make([]BrowserProxy, 0, len(pool))

	for _, row := range pool {
		if !row.SourceMissing {
			remaining = append(remaining, row)
			continue
		}
		if sourceId != "" && !proxyRowMatchesCleanupSource(row, sourceId) {
			remaining = append(remaining, row)
			continue
		}
		result.Scanned++
		key := strings.ToLower(strings.TrimSpace(row.ProxyId))
		if key == "" {
			// 缺少主键的行无法判定归属，保留以免误删手工节点。
			remaining = append(remaining, row)
			continue
		}
		if name, ok := instanceRefs[key]; ok {
			result.Skipped = append(result.Skipped, ProxyMissingCleanupSkip{
				ProxyId: row.ProxyId, ProxyName: row.ProxyName, SourceId: row.SourceID,
				Reason: proxyMissingSkipInstance, Detail: name,
			})
			remaining = append(remaining, row)
			continue
		}
		if name, ok := groupRefs[key]; ok {
			result.Skipped = append(result.Skipped, ProxyMissingCleanupSkip{
				ProxyId: row.ProxyId, ProxyName: row.ProxyName, SourceId: row.SourceID,
				Reason: proxyMissingSkipVirtualGroup, Detail: name,
			})
			remaining = append(remaining, row)
			continue
		}
		if name, ok := fetchRefs[key]; ok {
			result.Skipped = append(result.Skipped, ProxyMissingCleanupSkip{
				ProxyId: row.ProxyId, ProxyName: row.ProxyName, SourceId: row.SourceID,
				Reason: proxyMissingSkipFetchProxy, Detail: name,
			})
			remaining = append(remaining, row)
			continue
		}
		result.Deletable++
	}
	return remaining, result
}

// collectProxyMissingReferences 收集代理 ID 的引用来源：实例、虚拟分组、订阅拉取代理。
// 分组引用必须用 ExpandProxyVirtualGroup 展开：成员既可能来自 MemberProxyIds，
// 也可能来自 MemberGroupNames（按代理分组名批量纳入），只查前者会漏判。
// 拉取代理同样必须收集：定时/手动刷新都用它建客户端，删掉会让订阅彻底刷不动。
func (a *App) collectProxyMissingReferences(proxies []BrowserProxy) (instanceRefs map[string]string, groupRefs map[string]string, fetchRefs map[string]string) {
	instanceRefs = make(map[string]string)
	groupRefs = make(map[string]string)
	fetchRefs = make(map[string]string)

	for _, row := range proxies {
		id := strings.TrimSpace(row.SourceFetchProxyID)
		if id == "" {
			continue
		}
		key := strings.ToLower(id)
		if _, exists := fetchRefs[key]; exists {
			continue
		}
		label := strings.TrimSpace(row.SourceID)
		if label == "" {
			label = strings.TrimSpace(row.SourceURL)
		}
		if label == "" {
			label = strings.TrimSpace(row.ProxyName)
		}
		fetchRefs[key] = label
	}

	if a == nil || a.browserMgr == nil {
		return
	}

	a.browserMgr.Mutex.Lock()
	for _, profile := range a.browserMgr.Profiles {
		if profile == nil {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(profile.ProxyId))
		if key == "" {
			continue
		}
		if _, exists := instanceRefs[key]; exists {
			continue
		}
		label := strings.TrimSpace(profile.ProfileName)
		if label == "" {
			label = strings.TrimSpace(profile.ProfileId)
		}
		instanceRefs[key] = label
	}
	a.browserMgr.Mutex.Unlock()

	for _, group := range a.BrowserProxyVirtualGroupList() {
		name := strings.TrimSpace(group.GroupName)
		if name == "" {
			name = strings.TrimSpace(group.GroupId)
		}
		addRef := func(proxyID string) {
			key := strings.ToLower(strings.TrimSpace(proxyID))
			if key == "" {
				return
			}
			if _, exists := groupRefs[key]; exists {
				return
			}
			groupRefs[key] = name
		}
		resolved := browser.ResolveProxyVirtualGroup(group, proxies)
		for _, member := range browser.ExpandProxyVirtualGroup(resolved, proxies) {
			addRef(member.ProxyId)
		}
		addRef(resolved.SelectedProxyId)
		addRef(resolved.PreferredProxyId)
	}
	return
}

// proxyRowMatchesCleanupSource 与订阅刷新同一口径：sourceId 或 source_url 都可用于定位源。
func proxyRowMatchesCleanupSource(row BrowserProxy, sourceId string) bool {
	if strings.TrimSpace(sourceId) == "" {
		return true
	}
	return proxySourceKey(row) == sourceId || rowConfigMatchesSourceId(row, sourceId)
}
