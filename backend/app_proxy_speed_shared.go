package backend

import (
	"fmt"
	"strings"
	"time"

	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/proxy"
)

func (a *App) testSharedProxySpeed(proxyID string, forceRefresh bool) ProxyTestResult {
	proxies := a.getLatestProxies()
	return a.testSharedProxySpeedWithOptions(proxyID, forceRefresh, proxies, a.defaultProxyConnectorType(), a.proxySpeedTestConfig())
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
		item.Ok = false
		item.LatencyMs = -1
		if strings.TrimSpace(item.Error) == "" {
			item.Error = fmt.Sprintf(`测速结果保存失败: %v`, err)
		} else {
			item.Error = fmt.Sprintf(`%s；测速结果保存失败: %v`, item.Error, err)
		}
	}
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
		return updater.UpdateSpeedResultWithProvenance(result.ProxyId, result.Ok, result.LatencyMs, testedAt, result.Engine, "", "")
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
