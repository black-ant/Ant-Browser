package backend

import (
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/database"
	"ant-chrome/backend/internal/proxy"
)

type proxySpeedPersistenceFailureDAO struct {
	proxies []browser.Proxy
	err     error
}

func (d *proxySpeedPersistenceFailureDAO) List() ([]browser.Proxy, error) {
	return append([]browser.Proxy{}, d.proxies...), nil
}

func (d *proxySpeedPersistenceFailureDAO) ListByGroup(string) ([]browser.Proxy, error) {
	return nil, nil
}

func (d *proxySpeedPersistenceFailureDAO) ListGroups() ([]string, error) { return nil, nil }

func (d *proxySpeedPersistenceFailureDAO) Upsert(browser.Proxy) error { return nil }

func (d *proxySpeedPersistenceFailureDAO) ReplaceAll([]browser.Proxy) error { return nil }

func (d *proxySpeedPersistenceFailureDAO) Delete(string) error { return nil }

func (d *proxySpeedPersistenceFailureDAO) DeleteAll() error { return nil }

func (d *proxySpeedPersistenceFailureDAO) UpdateSpeedResult(string, bool, int64, string) error {
	return nil
}

func (d *proxySpeedPersistenceFailureDAO) UpdateSpeedResultWithProvenance(string, bool, int64, string, string, string, string) error {
	return d.err
}

func (d *proxySpeedPersistenceFailureDAO) UpdateIPHealthResult(string, string) error { return nil }

func TestProxySpeedFingerprintIncludesProxyPoolChanges(t *testing.T) {
	app := NewApp("")
	app.config = config.DefaultConfig()
	proxies := []BrowserProxy{
		{ProxyId: "target", ProxyConfig: "http://127.0.0.1:8080"},
		{ProxyId: "unrelated", ProxyConfig: "http://127.0.0.2:8080"},
	}
	speedConfig := &proxy.SpeedTestConfig{Timeout: 3 * time.Second, TCPTimeout: 3 * time.Second, URLs: []string{"http://example.com/204"}, ExpectedStatus: []int{204}}
	initial := proxySpeedTestFingerprint("target", proxies, config.BrowserConnectorXray, speedConfig)

	proxies[1].ProxyConfig = "http://127.0.0.3:8080"
	if got := proxySpeedTestFingerprint("target", proxies, config.BrowserConnectorXray, speedConfig); got == initial {
		t.Fatal("a proxy pool change did not invalidate the target fingerprint")
	}
	proxies[0].DnsServers = "1.1.1.1"
	if got := proxySpeedTestFingerprint("target", proxies, config.BrowserConnectorXray, speedConfig); got == initial {
		t.Fatal("a target DNS change did not invalidate the fingerprint")
	}
	proxies[0].DnsServers = ""
	proxies[0].GroupName = "changed"
	if got := proxySpeedTestFingerprint("target", proxies, config.BrowserConnectorXray, speedConfig); got == initial {
		t.Fatal("a target group change did not invalidate the fingerprint")
	}
}

func TestProxySpeedFingerprintChangesWithTargetID(t *testing.T) {
	proxies := []BrowserProxy{{ProxyId: "target", ProxyConfig: "http://127.0.0.1:8080"}}
	first := proxy.DefaultSpeedTestConfig
	first.TargetID = "speed-a"
	second := first
	second.TargetID = "speed-b"

	firstFingerprint := proxySpeedTestFingerprint("target", proxies, config.BrowserConnectorXray, &first)
	secondFingerprint := proxySpeedTestFingerprint("target", proxies, config.BrowserConnectorXray, &second)
	if firstFingerprint == secondFingerprint {
		t.Fatalf("fingerprint did not change when target ID changed: %q", firstFingerprint)
	}
}

func TestProxySpeedResultsRejectLegacyAndChangedTargetContexts(t *testing.T) {
	app := NewApp("")
	app.config = config.DefaultConfig()
	testedAt := time.Now().Format(time.RFC3339)
	proxies := []BrowserProxy{{
		ProxyId: "proxy-1", ProxyConfig: "http://127.0.0.1:8080",
		LastTestOk: true, LastLatencyMs: 40, LastTestedAt: testedAt,
	}}

	if app.proxySpeedResultsWithCurrentContext(proxies)[0].LastTestOk {
		t.Fatal("legacy result without a fingerprint remained eligible")
	}
	proxies[0].LastTestFingerprint = proxySpeedTestFingerprint(proxies[0].ProxyId, proxies, app.defaultProxyConnectorType(), app.proxySpeedTestConfig())
	proxies[0].LastTestConnectorType = app.defaultProxyConnectorType()
	proxies[0].LastTestEngine = expectedProxySpeedEngine(proxies[0].ProxyId, proxies, app.defaultProxyConnectorType())
	if !app.proxySpeedResultsWithCurrentContext(proxies)[0].LastTestOk {
		t.Fatal("current result was rejected")
	}
	proxies[0].ProxyConfig = "http://127.0.0.2:8080"
	if app.proxySpeedResultsWithCurrentContext(proxies)[0].LastTestOk {
		t.Fatal("result for a previous proxy configuration remained eligible")
	}
}

func TestProxySpeedResultsRejectExpiredResults(t *testing.T) {
	app := NewApp("")
	app.config = config.DefaultConfig()
	proxies := []BrowserProxy{{
		ProxyId: "proxy-1", ProxyConfig: "http://127.0.0.1:8080",
		LastTestOk: true, LastLatencyMs: 40,
		LastTestedAt: time.Now().Add(-browser.DefaultProxySpeedInterval - time.Second).Format(time.RFC3339),
	}}
	proxies[0].LastTestFingerprint = proxySpeedTestFingerprint(proxies[0].ProxyId, proxies, app.defaultProxyConnectorType(), app.proxySpeedTestConfig())
	proxies[0].LastTestConnectorType = app.defaultProxyConnectorType()
	proxies[0].LastTestEngine = expectedProxySpeedEngine(proxies[0].ProxyId, proxies, app.defaultProxyConnectorType())
	if app.proxySpeedResultsWithCurrentContext(proxies)[0].LastTestOk {
		t.Fatal("expired speed result remained eligible")
	}
}

func TestProxySpeedScheduleUsesConfiguredInterval(t *testing.T) {
	app := NewApp("")
	app.config = config.DefaultConfig()
	app.config.ProxyCheck.SpeedIntervalMinutes = 120
	app.config.ProxyCheck.FailureRetryMinutes = 7
	app.config.ProxyCheck.SpeedConcurrency = 4

	interval, failureRetry, concurrency := app.proxySpeedSchedule()
	if interval != 120*time.Minute {
		t.Fatalf("speed interval = %s, want 2h", interval)
	}
	if failureRetry != 7*time.Minute {
		t.Fatalf("failure retry = %s, want 7m", failureRetry)
	}
	if concurrency != 4 {
		t.Fatalf("speed concurrency = %d, want 4", concurrency)
	}
}

func TestProxySpeedGateEnforcesGlobalConcurrency(t *testing.T) {
	app := NewApp("")
	app.config = config.DefaultConfig()
	app.config.ProxyCheck.SpeedConcurrency = 2
	entered := make(chan struct{}, 8)
	release := make(chan struct{})
	var active atomic.Int32
	var maximum atomic.Int32
	var workers sync.WaitGroup
	for range 8 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			app.withProxySpeedSlot(func() ProxyTestResult {
				current := active.Add(1)
				for previous := maximum.Load(); current > previous && !maximum.CompareAndSwap(previous, current); previous = maximum.Load() {
				}
				entered <- struct{}{}
				<-release
				active.Add(-1)
				return ProxyTestResult{}
			})
		}()
	}
	for range 2 {
		select {
		case <-entered:
		case <-time.After(time.Second):
			close(release)
			t.Fatal("global speed-test slots did not start")
		}
	}
	select {
	case <-entered:
		close(release)
		t.Fatal("speed-test concurrency exceeded configured limit")
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	workers.Wait()
	if got := maximum.Load(); got != 2 {
		t.Fatalf("maximum concurrent speed tests = %d, want 2", got)
	}
}

func TestPersistProxySpeedResultRejectsChangedContext(t *testing.T) {
	db, err := database.NewDB(filepath.Join(t.TempDir(), "database.db"))
	if err != nil {
		t.Fatalf("NewDB returned error: %v", err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}

	app := NewApp(t.TempDir())
	app.config = config.DefaultConfig()
	app.browserMgr = browser.NewManager(app.config, app.appRoot)
	app.browserMgr.ProxyDAO = browser.NewSQLiteProxyDAO(db.GetConn())
	initial := BrowserProxy{ProxyId: "proxy-1", ProxyName: "Proxy 1", ProxyConfig: "http://127.0.0.1:8080"}
	if err := app.browserMgr.ProxyDAO.Upsert(initial); err != nil {
		t.Fatalf("insert initial proxy failed: %v", err)
	}
	app.config.Browser.Proxies = []BrowserProxy{initial}

	connectorType := app.defaultProxyConnectorType()
	result := ProxyTestResult{
		ProxyId:       initial.ProxyId,
		Ok:            true,
		LatencyMs:     25,
		Engine:        expectedProxySpeedEngine(initial.ProxyId, []BrowserProxy{initial}, connectorType),
		TestedAt:      time.Now().Format(time.RFC3339),
		fingerprint:   proxySpeedTestFingerprint(initial.ProxyId, []BrowserProxy{initial}, connectorType, app.proxySpeedTestConfig()),
		connectorType: connectorType,
	}

	changed := initial
	changed.ProxyConfig = "http://127.0.0.2:8080"
	app.config.Browser.Proxies = []BrowserProxy{changed}
	if err := app.browserMgr.ProxyDAO.DeleteAll(); err != nil {
		t.Fatalf("replace proxy list failed: %v", err)
	}
	if err := app.browserMgr.ProxyDAO.Upsert(changed); err != nil {
		t.Fatalf("save changed proxy failed: %v", err)
	}

	if err := app.persistProxySpeedResult(result); err != nil {
		t.Fatalf("persistProxySpeedResult returned error: %v", err)
	}
	stored, err := app.browserMgr.ProxyDAO.List()
	if err != nil {
		t.Fatalf("read proxy list failed: %v", err)
	}
	for _, item := range stored {
		if item.ProxyId == initial.ProxyId && item.LastTestedAt != "" {
			t.Fatalf("stale speed result was persisted: %#v", item)
		}
	}
}

func TestPersistProxySpeedResultReportsStorageFailure(t *testing.T) {
	app := NewApp(t.TempDir())
	app.config = config.DefaultConfig()
	initial := BrowserProxy{ProxyId: "proxy-1", ProxyName: "Proxy 1", ProxyConfig: "http://127.0.0.1:8080"}
	app.browserMgr = browser.NewManager(app.config, app.appRoot)
	app.browserMgr.ProxyDAO = &proxySpeedPersistenceFailureDAO{
		proxies: []browser.Proxy{initial},
		err:     errors.New("database write failed"),
	}
	proxies := app.getLatestProxies()
	connectorType := app.defaultProxyConnectorType()
	result := ProxyTestResult{
		ProxyId:       initial.ProxyId,
		Ok:            true,
		LatencyMs:     25,
		Engine:        expectedProxySpeedEngine(initial.ProxyId, proxies, connectorType),
		TestedAt:      time.Now().Format(time.RFC3339),
		fingerprint:   proxySpeedTestFingerprint(initial.ProxyId, proxies, connectorType, app.proxySpeedTestConfig()),
		connectorType: connectorType,
	}
	if err := app.persistProxySpeedResult(result); err == nil {
		t.Fatal("persistProxySpeedResult succeeded despite storage failure")
	}
}
