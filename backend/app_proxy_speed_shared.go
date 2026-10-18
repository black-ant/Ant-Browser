package backend

import (
	"strings"
	"sync"
	"time"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/proxy"
)

type proxySpeedTask struct {
	done   chan struct{}
	result ProxyTestResult
}

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
	fingerprint := proxySpeedTestFingerprint(proxyID, proxies, connectorType, speedConfig)
	key := strings.ToLower(proxyID) + ":" + fingerprint

	a.proxySpeedMu.Lock()
	if a.proxySpeedTasks == nil {
		a.proxySpeedTasks = make(map[string]*proxySpeedTask)
	}
	if task := a.proxySpeedTasks[key]; task != nil {
		a.proxySpeedMu.Unlock()
		<-task.done
		return task.result
	}
	if !forceRefresh {
		if cached, ok := a.readFreshProxySpeedResult(proxyID, fingerprint, connectorType, proxies); ok {
			a.proxySpeedMu.Unlock()
			return cached
		}
	}
	task := &proxySpeedTask{done: make(chan struct{})}
	a.proxySpeedTasks[key] = task
	a.proxySpeedMu.Unlock()

	result := a.withProxySpeedSlot(func() ProxyTestResult {
		raw := a.testProxySpeedWithConnector(proxyID, proxies, connectorType, speedConfig)
		item := buildProxyTestResult(raw)
		item.testedAt = time.Now().Format(time.RFC3339)
		item.fingerprint = fingerprint
		item.connectorType = connectorType
		return item
	})
	if result.testedAt == "" {
		result.testedAt = time.Now().Format(time.RFC3339)
		result.fingerprint = fingerprint
		result.connectorType = connectorType
	}
	_ = a.persistProxySpeedResult(result)

	a.proxySpeedMu.Lock()
	task.result = result
	delete(a.proxySpeedTasks, key)
	close(task.done)
	a.proxySpeedMu.Unlock()
	return result
}

func (a *App) readFreshProxySpeedResult(proxyID string, fingerprint string, connectorType string, proxies []BrowserProxy) (ProxyTestResult, bool) {
	if a == nil {
		return ProxyTestResult{}, false
	}
	for _, item := range proxies {
		if !strings.EqualFold(strings.TrimSpace(item.ProxyId), proxyID) {
			continue
		}
		if item.LastTestFingerprint != fingerprint || item.LastTestConnectorType != connectorType || item.LastTestEngine != expectedProxySpeedEngine(proxyID, proxies, connectorType) {
			return ProxyTestResult{}, false
		}
		if !isProxySpeedResultFresh(item.LastTestedAt, time.Now(), a.proxySpeedResultMaxAge()) {
			return ProxyTestResult{}, false
		}
		return ProxyTestResult{
			ProxyId: proxyID, Ok: item.LastTestOk, LatencyMs: item.LastLatencyMs,
			Engine: item.LastTestEngine, testedAt: item.LastTestedAt,
			fingerprint: item.LastTestFingerprint, connectorType: item.LastTestConnectorType,
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
		return nil
	}
	testedAt := result.testedAt
	if testedAt == "" {
		testedAt = time.Now().Format(time.RFC3339)
	}
	updater, ok := a.browserMgr.ProxyDAO.(interface {
		UpdateSpeedResultWithProvenance(string, bool, int64, string, string, string, string) error
	})
	if ok {
		return updater.UpdateSpeedResultWithProvenance(result.ProxyId, result.Ok, result.LatencyMs, testedAt, result.Engine, result.connectorType, result.fingerprint)
	}
	return a.browserMgr.ProxyDAO.UpdateSpeedResult(result.ProxyId, result.Ok, result.LatencyMs, testedAt)
}

func (a *App) proxySpeedResultsWithCurrentContext(proxies []BrowserProxy) []BrowserProxy {
	connectorType := a.defaultProxyConnectorType()
	speedConfig := a.proxySpeedTestConfig()
	result := append([]BrowserProxy{}, proxies...)
	for index, item := range result {
		fingerprint := proxySpeedTestFingerprint(item.ProxyId, proxies, connectorType, speedConfig)
		if item.LastTestFingerprint != fingerprint || item.LastTestConnectorType != connectorType || item.LastTestEngine != expectedProxySpeedEngine(item.ProxyId, proxies, connectorType) {
			result[index].LastTestOk = false
			result[index].LastLatencyMs = -1
		}
	}
	return result
}

func (a *App) proxySpeedResultIsCurrent(item BrowserProxy, proxies []BrowserProxy) bool {
	connectorType := a.defaultProxyConnectorType()
	fingerprint := proxySpeedTestFingerprint(item.ProxyId, proxies, connectorType, a.proxySpeedTestConfig())
	return item.LastTestFingerprint == fingerprint && item.LastTestConnectorType == connectorType && item.LastTestEngine == expectedProxySpeedEngine(item.ProxyId, proxies, connectorType)
}

func (a *App) withProxySpeedSlot(testFn func() ProxyTestResult) ProxyTestResult {
	if testFn == nil {
		return ProxyTestResult{}
	}
	if a == nil {
		return testFn()
	}
	limit := a.proxySpeedConcurrencyLimit()
	a.proxySpeedGateMu.Lock()
	if a.proxySpeedGateCond == nil {
		a.proxySpeedGateCond = sync.NewCond(&a.proxySpeedGateMu)
	}
	for a.proxySpeedActive >= limit {
		a.proxySpeedGateCond.Wait()
		limit = a.proxySpeedConcurrencyLimit()
	}
	a.proxySpeedActive++
	a.proxySpeedGateMu.Unlock()
	defer func() {
		a.proxySpeedGateMu.Lock()
		a.proxySpeedActive--
		a.proxySpeedGateCond.Broadcast()
		a.proxySpeedGateMu.Unlock()
	}()
	return testFn()
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
	return browser.DefaultProxySpeedInterval
}
