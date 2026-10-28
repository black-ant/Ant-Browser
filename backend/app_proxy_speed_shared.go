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

func (a *App) testSharedProxySpeed(proxyID string, forceRefresh bool) ProxyTestResult {
	proxies := a.getLatestProxies()
	// 实例固定绑定虚拟分组时，测速针对本次选中的成员代理，结果写回该成员。
	if resolvedID, ok := a.resolveVirtualGroupSpeedTestTarget(strings.TrimSpace(proxyID), proxies); ok {
		proxyID = resolvedID
	}
	return a.testSharedProxySpeedWithOptions(proxyID, forceRefresh, proxies, a.defaultProxyConnectorType(), a.proxySpeedTestConfig())
}

// resolveVirtualGroupProxyTarget 当 proxyID 是虚拟分组时，挑选一个成员代理（ID 与配置）。
// 实体代理 ID 与未知 ID 返回 false，调用方按原有逻辑继续处理。
func (a *App) resolveVirtualGroupProxyTarget(proxyID string, proxies []BrowserProxy) (string, string, bool) {
	if proxyID == "" {
		return "", "", false
	}
	for _, proxy := range proxies {
		if strings.EqualFold(strings.TrimSpace(proxy.ProxyId), proxyID) {
			return "", "", false
		}
	}
	group, ok := a.findProxyVirtualGroupByID(proxyID)
	if !ok {
		return "", "", false
	}
	if memberID, memberConfig, picked := a.pickVirtualGroupStartProxy(group, proxies, "virtual-target"); picked {
		return memberID, memberConfig, true
	}
	// 全员降级时选点会返回空，但手动测速必须仍可用，否则用户无法把节点测回来。
	resolved := browser.ResolveProxyVirtualGroup(group, proxies)
	memberSet := make(map[string]struct{}, len(resolved.MemberProxyIds))
	for _, memberID := range resolved.MemberProxyIds {
		memberSet[strings.ToLower(strings.TrimSpace(memberID))] = struct{}{}
	}
	for _, proxy := range proxies {
		if strings.TrimSpace(proxy.ProxyConfig) == "" {
			continue
		}
		if _, isMember := memberSet[strings.ToLower(strings.TrimSpace(proxy.ProxyId))]; !isMember {
			continue
		}
		return strings.TrimSpace(proxy.ProxyId), strings.TrimSpace(proxy.ProxyConfig), true
	}
	return "", "", false
}

// resolveVirtualGroupSpeedTestTarget 当 proxyID 是虚拟分组时，挑选一个成员代理作为测速目标。
func (a *App) resolveVirtualGroupSpeedTestTarget(proxyID string, proxies []BrowserProxy) (string, bool) {
	memberID, _, picked := a.resolveVirtualGroupProxyTarget(proxyID, proxies)
	if !picked {
		return "", false
	}
	return memberID, true
}

func (a *App) testSharedProxySpeedWithOptions(proxyID string, forceRefresh bool, proxies []BrowserProxy, connectorType string, speedConfig *proxy.SpeedTestConfig) ProxyTestResult {
	proxyID = strings.TrimSpace(proxyID)
	if proxyID == "" {
		return ProxyTestResult{ProxyId: proxyID, Error: "代理 ID 为空"}
	}
	connectorType = config.NormalizeBrowserConnectorType(connectorType)
	if !forceRefresh {
		if cached, ok := a.readFreshProxySpeedResult(proxyID, connectorType, proxies); ok {
			return cached
		}
	}

	item := buildProxyTestResult(a.testProxySpeedWithConnector(proxyID, proxies, connectorType, speedConfig))
	item.TestedAt = time.Now().Format(time.RFC3339)
	item.connectorType = connectorType
	if err := a.persistProxySpeedResult(item); err != nil {
		// 网络测速结果是真实的，持久化失败不应翻转 Ok，避免把存储故障伪装成“测速全部失败”。
		logger.New("SpeedTest").Warn("测速结果保存失败",
			logger.F("proxy_id", proxyID),
			logger.F("error", err.Error()),
		)
	}
	// 单点汇聚：定时调度与手动测速都经过这里，统一喂给虚拟分组健康状态机。
	// 缓存命中路径在上面提前返回，不会重复计数。
	a.recordVirtualGroupMemberProbe(proxyID, browser.MemberProbeOutcome{
		Ok:            item.Ok,
		LatencyMs:     item.LatencyMs,
		Error:         item.Error,
		Engine:        item.Engine,
		ConnectorType: connectorType,
		At:            time.Now(),
	})
	return item
}

func (a *App) readFreshProxySpeedResult(proxyID string, connectorType string, proxies []BrowserProxy) (ProxyTestResult, bool) {
	if a == nil {
		return ProxyTestResult{}, false
	}
	for _, item := range proxies {
		if !strings.EqualFold(strings.TrimSpace(item.ProxyId), proxyID) {
			continue
		}
		if item.LastTestConnectorType != connectorType || !isProxySpeedResultFresh(item.LastTestedAt, time.Now(), a.proxySpeedResultMaxAge()) {
			return ProxyTestResult{}, false
		}
		return ProxyTestResult{
			ProxyId: proxyID, Ok: item.LastTestOk, LatencyMs: item.LastLatencyMs,
			Engine: item.LastTestEngine, TestedAt: item.LastTestedAt,
			connectorType: item.LastTestConnectorType,
		}, true
	}
	return ProxyTestResult{}, false
}

func isProxySpeedResultFresh(testedAt string, checkedAt time.Time, maxAge time.Duration) bool {
	testedTime, err := time.Parse(time.RFC3339, strings.TrimSpace(testedAt))
	if err != nil {
		return false
	}
	age := checkedAt.Sub(testedTime)
	return age >= 0 && age <= maxAge
}

func (a *App) persistProxySpeedResult(result ProxyTestResult) error {
	if a == nil || a.browserMgr == nil || a.browserMgr.ProxyDAO == nil || strings.TrimSpace(result.ProxyId) == "" {
		return fmt.Errorf(`代理测速结果持久化不可用`)
	}
	testedAt := result.TestedAt
	if testedAt == "" {
		testedAt = time.Now().Format(time.RFC3339)
	}
	updater, ok := a.browserMgr.ProxyDAO.(interface {
		UpdateSpeedResultWithProvenance(string, bool, int64, string, string, string, string) error
	})
	if ok {
		// connectorType 必须落库：读取侧（readFreshProxySpeedResult）与健康状态判定都依赖它区分连接栈。
		// fingerprint 目前没有生产来源（proxy.TestResult 未提供），保持空串，不参与任何判定。
		if err := updater.UpdateSpeedResultWithProvenance(result.ProxyId, result.Ok, result.LatencyMs, testedAt, result.Engine, result.connectorType, result.fingerprint); err != nil {
			// 兼容跨分支旧 schema（缺 last_test_engine 等列）：降级为基础列写入，保留可用的测速结果。
			if fallbackErr := a.browserMgr.ProxyDAO.UpdateSpeedResult(result.ProxyId, result.Ok, result.LatencyMs, testedAt); fallbackErr == nil {
				return nil
			}
			return err
		}
		return nil
	}
	return a.browserMgr.ProxyDAO.UpdateSpeedResult(result.ProxyId, result.Ok, result.LatencyMs, testedAt)
}

func (a *App) proxySpeedResultIsCurrent(item BrowserProxy, proxies []BrowserProxy) bool {
	_ = proxies
	return isProxySpeedResultFresh(item.LastTestedAt, time.Now(), a.proxySpeedResultMaxAge())
}

func (a *App) proxySpeedConcurrencyLimit() int {
	limit := config.DefaultProxySpeedConcurrency
	if a != nil && a.config != nil && a.config.ProxyCheck.SpeedConcurrency > 0 {
		limit = a.config.ProxyCheck.SpeedConcurrency
	}
	if limit > config.MaxProxySpeedConcurrency {
		limit = config.MaxProxySpeedConcurrency
	}
	if limit < 1 {
		limit = 1
	}
	return limit
}

func (a *App) proxySpeedResultMaxAge() time.Duration {
	interval, _, _ := a.proxySpeedSchedule()
	return interval
}
