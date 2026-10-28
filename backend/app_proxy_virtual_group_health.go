package backend

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/logger"
)

// virtualGroupSnapshotCache 缓存"分组 → 成员代理"与"代理 → 所属分组"两个索引。
// 探测是并发执行的，每次探测都全量 List 分组与代理会放大成 N 次查询。
type virtualGroupSnapshotCache struct {
	mu        sync.Mutex
	groups    []browser.ProxyVirtualGroup
	proxies   []browser.Proxy
	byProxy   map[string][]browser.ProxyVirtualGroup
	expiresAt time.Time
}

const virtualGroupSnapshotTTL = 30 * time.Second

const (
	// 分组成员优先于全池；正在被实例使用的成员再优先一级。
	probePriorityVirtualGroupMember = 100
	probePriorityActiveMember       = 200
)

// isProxyInUse 该代理当前是否被某个运行中的实例占用（桥接引用仍然存在）。
func (a *App) isProxyInUse(proxyID string) bool {
	if a == nil {
		return false
	}
	proxyID = strings.ToLower(strings.TrimSpace(proxyID))
	if proxyID == "" {
		return false
	}
	a.bridgeMu.Lock()
	defer a.bridgeMu.Unlock()
	for _, ref := range a.profileBridgeRefs {
		if strings.EqualFold(strings.TrimSpace(ref.ProxyID), proxyID) {
			return true
		}
	}
	return false
}

// virtualGroupProbePriority 供测速调度器排序：分组成员优先，其中被实例占用的最优先。
func (a *App) virtualGroupProbePriority(proxyID string) int {
	proxyID = strings.ToLower(strings.TrimSpace(proxyID))
	if proxyID == "" {
		return 0
	}
	_, byProxy := a.virtualGroupSnapshotData()
	if len(byProxy[proxyID]) == 0 {
		return 0
	}
	if a.isProxyInUse(proxyID) {
		return probePriorityActiveMember
	}
	return probePriorityVirtualGroupMember
}

// virtualGroupProbeIntervalFor 分组成员使用独立探测间隔，其余返回 0 走全局设置。
func (a *App) virtualGroupProbeIntervalFor(proxyID string) time.Duration {
	proxyID = strings.ToLower(strings.TrimSpace(proxyID))
	if proxyID == "" {
		return 0
	}
	_, byProxy := a.virtualGroupSnapshotData()
	if len(byProxy[proxyID]) == 0 {
		return 0
	}
	return a.virtualGroupProbeInterval()
}

func (a *App) virtualGroupHealthEnabled() bool {
	if a == nil || a.config == nil {
		return false
	}
	return a.config.ProxyCheck.IsVirtualGroupHealthEnabled()
}

// memberHealthPolicy 从配置读取降级/升级阈值，零值由状态机自身回落默认值。
func (a *App) memberHealthPolicy() browser.MemberHealthPolicy {
	settings := config.DefaultConfig().ProxyCheck
	if a != nil && a.config != nil {
		settings = a.config.ProxyCheck
	}
	failureRetry := time.Duration(settings.FailureRetryMinutes) * time.Minute
	if failureRetry <= 0 {
		failureRetry = time.Duration(config.DefaultProxyFailureRetryMinutes) * time.Minute
	}
	return browser.MemberHealthPolicy{
		DegradeFailureThreshold: settings.DegradeFailureThreshold,
		RestoreSuccessThreshold: settings.RestoreSuccessThreshold,
		FailureRetry:            failureRetry,
	}
}

// virtualGroupProbeInterval 分组成员的独立探测间隔；0 表示跟随全局测速间隔。
func (a *App) virtualGroupProbeInterval() time.Duration {
	if a == nil || a.config == nil {
		return 0
	}
	minutes := a.config.ProxyCheck.VirtualGroupProbeIntervalMinutes
	if minutes <= 0 {
		return 0
	}
	return time.Duration(minutes) * time.Minute
}

// invalidateVirtualGroupSnapshot 分组或代理发生变化时主动失效缓存。
func (a *App) invalidateVirtualGroupSnapshot() {
	if a == nil {
		return
	}
	a.virtualGroupSnapshot.mu.Lock()
	a.virtualGroupSnapshot.expiresAt = time.Time{}
	a.virtualGroupSnapshot.groups = nil
	a.virtualGroupSnapshot.proxies = nil
	a.virtualGroupSnapshot.byProxy = nil
	a.virtualGroupSnapshot.mu.Unlock()
}

func (a *App) virtualGroupSnapshotData() ([]browser.ProxyVirtualGroup, map[string][]browser.ProxyVirtualGroup) {
	if a == nil || a.browserMgr == nil {
		return nil, nil
	}
	a.virtualGroupSnapshot.mu.Lock()
	defer a.virtualGroupSnapshot.mu.Unlock()
	now := time.Now()
	if a.virtualGroupSnapshot.byProxy != nil && now.Before(a.virtualGroupSnapshot.expiresAt) {
		return a.virtualGroupSnapshot.groups, a.virtualGroupSnapshot.byProxy
	}

	proxies := a.getLatestProxies()
	groups := a.BrowserProxyVirtualGroupList()
	resolved := make([]browser.ProxyVirtualGroup, 0, len(groups))
	byProxy := make(map[string][]browser.ProxyVirtualGroup)
	for _, group := range groups {
		item := browser.ResolveProxyVirtualGroup(group, proxies)
		resolved = append(resolved, item)
		for _, member := range browser.ExpandProxyVirtualGroup(item, proxies) {
			proxyID := strings.ToLower(strings.TrimSpace(member.ProxyId))
			if proxyID == "" {
				continue
			}
			byProxy[proxyID] = append(byProxy[proxyID], item)
		}
	}

	a.virtualGroupSnapshot.groups = resolved
	a.virtualGroupSnapshot.proxies = proxies
	a.virtualGroupSnapshot.byProxy = byProxy
	a.virtualGroupSnapshot.expiresAt = now.Add(virtualGroupSnapshotTTL)
	return resolved, byProxy
}

// ProxyVirtualGroupMemberHealth 单个成员的健康快照。
type ProxyVirtualGroupMemberHealth struct {
	ProxyId     string `json:"proxyId"`
	ProxyName   string `json:"proxyName"`
	GroupName   string `json:"groupName"`
	State       string `json:"state"`
	LatencyMs   int64  `json:"latencyMs"`
	LastProbeAt string `json:"lastProbeAt"`
	LastError   string `json:"lastError"`
	Active      bool   `json:"active"`
	Preferred   bool   `json:"preferred"`
}

// ProxyVirtualGroupHealth 分组级聚合健康快照。
type ProxyVirtualGroupHealth struct {
	GroupId          string                        `json:"groupId"`
	GroupName        string                        `json:"groupName"`
	Strategy         string                        `json:"strategy"`
	ActiveProxyId    string                        `json:"activeProxyId"`
	PreferredProxyId string                        `json:"preferredProxyId"`
	Total            int                           `json:"total"`
	Healthy          int                           `json:"healthy"`
	Suspect          int                           `json:"suspect"`
	Degraded         int                           `json:"degraded"`
	// AllDegraded 为 true 表示整组成员全部降级，启动会被直接拒绝。
	AllDegraded bool                          `json:"allDegraded"`
	Members     []ProxyVirtualGroupMemberHealth `json:"members"`
	UpdatedAt   string                        `json:"updatedAt"`
}

// ProxyVirtualGroupProbeResult 手动巡检结果。
type ProxyVirtualGroupProbeResult struct {
	GroupId  string `json:"groupId"`
	Tested   int    `json:"tested"`
	Healthy  int    `json:"healthy"`
	Degraded int    `json:"degraded"`
}

// BrowserProxyVirtualGroupHealthList 返回所有虚拟分组的健康聚合。
func (a *App) BrowserProxyVirtualGroupHealthList() []ProxyVirtualGroupHealth {
	if a == nil || a.browserMgr == nil {
		return []ProxyVirtualGroupHealth{}
	}
	groups := a.BrowserProxyVirtualGroupList()
	proxies := a.getLatestProxies()
	result := make([]ProxyVirtualGroupHealth, 0, len(groups))
	for _, group := range groups {
		resolved := browser.ResolveProxyVirtualGroup(group, proxies)
		result = append(result, a.buildVirtualGroupHealth(resolved, proxies))
	}
	return result
}

func (a *App) buildVirtualGroupHealth(group browser.ProxyVirtualGroup, proxies []BrowserProxy) ProxyVirtualGroupHealth {
	stateRows := a.memberStateRows(group.GroupId)
	members := browser.ExpandProxyVirtualGroup(group, proxies)
	entry := ProxyVirtualGroupHealth{
		GroupId:          group.GroupId,
		GroupName:        group.GroupName,
		Strategy:         browser.NormalizeProxyVirtualGroupStrategy(group.Strategy),
		ActiveProxyId:    strings.TrimSpace(group.SelectedProxyId),
		PreferredProxyId: strings.TrimSpace(group.PreferredProxyId),
		Total:            len(members),
		Members:          make([]ProxyVirtualGroupMemberHealth, 0, len(members)),
		UpdatedAt:        time.Now().Format(time.RFC3339),
	}
	for _, member := range members {
		proxyID := strings.ToLower(strings.TrimSpace(member.ProxyId))
		row := stateRows[proxyID]
		state := row.State
		if state == "" {
			state = browser.MemberStateUnknown
		}
		// 记录来自其他连接栈时按未观测展示，避免误报为健康。
		if !a.memberStateMatchesConnector(row) {
			state = browser.MemberStateUnknown
		}
		switch state {
		case browser.MemberStateHealthy:
			entry.Healthy++
		case browser.MemberStateSuspect:
			entry.Suspect++
		case browser.MemberStateDegraded, browser.MemberStateRecovering:
			entry.Degraded++
		}
		entry.Members = append(entry.Members, ProxyVirtualGroupMemberHealth{
			ProxyId:     member.ProxyId,
			ProxyName:   member.ProxyName,
			GroupName:   member.GroupName,
			State:       string(state),
			LatencyMs:   member.LastLatencyMs,
			LastProbeAt: row.LastProbeAt,
			LastError:   row.LastError,
			Active:      strings.EqualFold(strings.TrimSpace(group.SelectedProxyId), member.ProxyId),
			Preferred:   strings.EqualFold(strings.TrimSpace(group.PreferredProxyId), member.ProxyId),
		})
	}
	// 全部成员都已降级（含恢复观察）才算整组不可用；全未观测不算。
	entry.AllDegraded = entry.Total > 0 && entry.Degraded == entry.Total
	return entry
}

// memberStateRows 按 proxyID(lower) 返回完整状态行。
func (a *App) memberStateRows(groupID string) map[string]browser.ProxyVirtualGroupMemberState {
	result := make(map[string]browser.ProxyVirtualGroupMemberState)
	if a == nil || a.browserMgr == nil || a.browserMgr.ProxyVirtualGroupMemberStateDAO == nil {
		return result
	}
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return result
	}
	rows, err := a.browserMgr.ProxyVirtualGroupMemberStateDAO.ListByGroup(groupID)
	if err != nil {
		return result
	}
	for _, row := range rows {
		proxyID := strings.ToLower(strings.TrimSpace(row.ProxyId))
		if proxyID == "" {
			continue
		}
		result[proxyID] = row
	}
	return result
}

// BrowserProxyVirtualGroupResetHealth 手动清除分组的降级状态，便于运营兜底。
func (a *App) BrowserProxyVirtualGroupResetHealth(groupId string) error {
	if a == nil || a.browserMgr == nil || a.browserMgr.ProxyVirtualGroupMemberStateDAO == nil {
		return fmt.Errorf("虚拟分组健康状态存储不可用")
	}
	groupId = strings.TrimSpace(groupId)
	if groupId == "" {
		return fmt.Errorf("虚拟分组 ID 不能为空")
	}
	return a.browserMgr.ProxyVirtualGroupMemberStateDAO.ResetByGroup(groupId)
}

// BrowserProxyVirtualGroupProbe 手动触发一轮分组巡检（串行，避免内核并发过高）。
func (a *App) BrowserProxyVirtualGroupProbe(groupId string) (ProxyVirtualGroupProbeResult, error) {
	groupId = strings.TrimSpace(groupId)
	result := ProxyVirtualGroupProbeResult{GroupId: groupId}
	if a == nil || a.browserMgr == nil {
		return result, fmt.Errorf("应用未初始化")
	}
	group, ok := a.findProxyVirtualGroupByID(groupId)
	if !ok {
		return result, fmt.Errorf("虚拟分组不存在：%s", groupId)
	}
	proxies := a.getLatestProxies()
	resolved := browser.ResolveProxyVirtualGroup(group, proxies)
	members := browser.ExpandProxyVirtualGroup(resolved, proxies)
	if len(members) == 0 {
		return result, fmt.Errorf("虚拟分组「%s」没有可用成员代理", resolved.GroupName)
	}
	for _, member := range members {
		a.testSharedProxySpeed(strings.TrimSpace(member.ProxyId), true)
		result.Tested++
	}
	health := a.buildVirtualGroupHealth(resolved, proxies)
	result.Healthy = health.Healthy
	result.Degraded = health.Degraded
	return result, nil
}

// recordVirtualGroupMemberProbe 把一次成员探测结果写入分组健康状态机。
// 探测来源可以是定时调度、手动测速或桥接失败，调用方只需提供结果。
func (a *App) recordVirtualGroupMemberProbe(proxyID string, outcome browser.MemberProbeOutcome) {
	if a == nil || a.browserMgr == nil || a.browserMgr.ProxyVirtualGroupMemberStateDAO == nil {
		return
	}
	if !a.virtualGroupHealthEnabled() {
		return
	}
	proxyID = strings.TrimSpace(proxyID)
	if proxyID == "" {
		return
	}
	_, byProxy := a.virtualGroupSnapshotData()
	groups := byProxy[strings.ToLower(proxyID)]
	if len(groups) == 0 {
		return
	}

	policy := a.memberHealthPolicy()
	dao := a.browserMgr.ProxyVirtualGroupMemberStateDAO
	for _, group := range groups {
		// 纯 config 模式下的历史分组可能没有 group_id，无法作为主键落库。
		if strings.TrimSpace(group.GroupId) == "" {
			continue
		}
		states, err := dao.ListByGroup(group.GroupId)
		if err != nil {
			logger.New("VirtualGroup").Warn("读取虚拟分组成员健康状态失败",
				logger.F("group_id", group.GroupId),
				logger.F("proxy_id", proxyID),
				logger.F("error", err.Error()),
			)
			continue
		}
		previous := previousMemberState(states, group.GroupId, proxyID)
		next := browser.NextMemberState(previous, outcome, policy)
		if err := dao.Upsert(next); err != nil {
			logger.New("VirtualGroup").Warn("保存虚拟分组成员健康状态失败",
				logger.F("group_id", group.GroupId),
				logger.F("proxy_id", proxyID),
				logger.F("error", err.Error()),
			)
			continue
		}
		if previous.State != next.State {
			logger.New("VirtualGroup").Info("虚拟分组成员健康状态变化",
				logger.F("group_id", group.GroupId),
				logger.F("group_name", group.GroupName),
				logger.F("proxy_id", proxyID),
				logger.F("from", string(previous.State)),
				logger.F("to", string(next.State)),
				logger.F("consecutive_failures", next.ConsecutiveFailures),
				logger.F("error", next.LastError),
			)
			a.emitRuntimeEvent("proxy:virtual-group:health", map[string]interface{}{
				"groupId":      group.GroupId,
				"groupName":    group.GroupName,
				"proxyId":      proxyID,
				"previousState": string(previous.State),
				"state":        string(next.State),
				"error":        next.LastError,
				"at":           next.LastProbeAt,
			})
		}
	}
}

func previousMemberState(states []browser.ProxyVirtualGroupMemberState, groupID string, proxyID string) browser.ProxyVirtualGroupMemberState {
	for _, item := range states {
		if strings.EqualFold(strings.TrimSpace(item.ProxyId), proxyID) {
			return item
		}
	}
	// 首次观测：必须带上分组 ID，否则 Upsert 会因主键不全而失败。
	return browser.ProxyVirtualGroupMemberState{
		GroupId: groupID,
		ProxyId: proxyID,
		State:   browser.MemberStateUnknown,
	}
}

// memberStatesForGroup 返回 groupID 下 proxyID(lower) → 完整状态行。
func (a *App) memberStatesForGroup(groupID string) map[string]browser.ProxyVirtualGroupMemberState {
	return a.memberStateRows(groupID)
}

// memberStateMatchesConnector 状态是否属于当前连接栈。
// 切换 xray/mihomo 栈后旧结果不可信，按未观测处理；空值视为历史数据，保持兼容。
func (a *App) memberStateMatchesConnector(row browser.ProxyVirtualGroupMemberState) bool {
	recorded := strings.TrimSpace(row.LastConnectorType)
	if recorded == "" {
		return true
	}
	return strings.EqualFold(recorded, a.defaultProxyConnectorType())
}

// applyManualProxyProbe 手动测速是强信号：成功立即恢复健康，失败立即降级，
// 不再等连续失败/成功阈值（自动调度仍按阈值防抖）。
func (a *App) applyManualProxyProbe(result ProxyTestResult) {
	if a == nil || a.browserMgr == nil || a.browserMgr.ProxyVirtualGroupMemberStateDAO == nil {
		return
	}
	proxyID := strings.TrimSpace(result.ProxyId)
	if proxyID == "" {
		return
	}
	_, byProxy := a.virtualGroupSnapshotData()
	groups := byProxy[strings.ToLower(proxyID)]
	if len(groups) == 0 {
		return
	}
	var cause error
	if !result.Ok {
		message := strings.TrimSpace(result.Error)
		if message == "" {
			message = "手动测速失败"
		}
		cause = fmt.Errorf("%s", message)
	}
	for _, group := range groups {
		if strings.TrimSpace(group.GroupId) == "" {
			continue
		}
		if result.Ok {
			a.markVirtualGroupMemberHealthy(group, proxyID, result.Engine, result.connectorType)
			continue
		}
		a.markVirtualGroupMemberDegraded(group, proxyID, cause)
	}
}

// markVirtualGroupMemberHealthy 强信号直接恢复健康，用于手动测速成功。
func (a *App) markVirtualGroupMemberHealthy(group browser.ProxyVirtualGroup, proxyID string, engine string, connectorType string) {
	if a == nil || a.browserMgr == nil || a.browserMgr.ProxyVirtualGroupMemberStateDAO == nil {
		return
	}
	proxyID = strings.TrimSpace(proxyID)
	if proxyID == "" || strings.TrimSpace(group.GroupId) == "" {
		return
	}
	dao := a.browserMgr.ProxyVirtualGroupMemberStateDAO
	states, err := dao.ListByGroup(group.GroupId)
	if err != nil {
		return
	}
	previous := previousMemberState(states, group.GroupId, proxyID)
	if previous.State == browser.MemberStateHealthy {
		return
	}
	next := previous
	next.GroupId = group.GroupId
	next.ProxyId = proxyID
	next.State = browser.MemberStateHealthy
	next.ConsecutiveFailures = 0
	if next.ConsecutiveSuccesses < 1 {
		next.ConsecutiveSuccesses = 1
	}
	now := time.Now().Format(time.RFC3339)
	next.LastProbeAt = now
	next.LastSuccessAt = now
	next.UpdatedAt = now
	next.NextProbeAt = ""
	next.LastError = ""
	if engine != "" {
		next.LastEngine = engine
	}
	if connectorType != "" {
		next.LastConnectorType = connectorType
	}
	if err := dao.Upsert(next); err != nil {
		return
	}
	logger.New("VirtualGroup").Info("虚拟分组成员恢复健康",
		logger.F("group_id", group.GroupId),
		logger.F("group_name", group.GroupName),
		logger.F("proxy_id", proxyID),
		logger.F("from", string(previous.State)),
	)
	a.emitRuntimeEvent("proxy:virtual-group:health", map[string]interface{}{
		"groupId":       group.GroupId,
		"groupName":     group.GroupName,
		"proxyId":       proxyID,
		"previousState": string(previous.State),
		"state":         string(browser.MemberStateHealthy),
		"at":            now,
	})
}

// markVirtualGroupMemberDegraded 强信号（桥接进程死亡）直接降级，跳过连续失败阈值。
func (a *App) markVirtualGroupMemberDegraded(group browser.ProxyVirtualGroup, proxyID string, cause error) {
	if a == nil || a.browserMgr == nil || a.browserMgr.ProxyVirtualGroupMemberStateDAO == nil {
		return
	}
	proxyID = strings.TrimSpace(proxyID)
	if proxyID == "" || strings.TrimSpace(group.GroupId) == "" {
		return
	}
	dao := a.browserMgr.ProxyVirtualGroupMemberStateDAO
	states, err := dao.ListByGroup(group.GroupId)
	if err != nil {
		return
	}
	previous := previousMemberState(states, group.GroupId, proxyID)
	if previous.State == browser.MemberStateDegraded {
		return
	}
	next := previous
	next.GroupId = group.GroupId
	next.ProxyId = proxyID
	next.State = browser.MemberStateDegraded
	next.ConsecutiveSuccesses = 0
	now := time.Now().Format(time.RFC3339)
	next.LastProbeAt = now
	next.LastFailureAt = now
	next.UpdatedAt = now
	if cause != nil {
		next.LastError = cause.Error()
	}
	if next.ConsecutiveFailures < 1 {
		next.ConsecutiveFailures = 1
	}
	next.NextProbeAt = time.Now().Add(a.memberHealthPolicy().FailureRetry).Format(time.RFC3339)
	_ = dao.Upsert(next)
	a.emitRuntimeEvent("proxy:virtual-group:health", map[string]interface{}{
		"groupId":       group.GroupId,
		"groupName":     group.GroupName,
		"proxyId":       proxyID,
		"previousState": string(previous.State),
		"state":         string(browser.MemberStateDegraded),
		"error":         next.LastError,
		"at":            now,
	})
}
