package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/database"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type proxyConnectorStateTestDAO struct {
	browser.ProxyDAO
	listCalled chan struct{}
	listOnce   sync.Once
}

func (d *proxyConnectorStateTestDAO) List() ([]browser.Proxy, error) {
	d.listOnce.Do(func() { close(d.listCalled) })
	return []browser.Proxy{}, nil
}

func TestSaveBrowserSettingsKeepsConnectionStackResults(t *testing.T) {
	db, err := database.NewDB(filepath.Join(t.TempDir(), "connector-state.db"))
	if err != nil {
		t.Fatalf("NewDB returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}

	app := NewApp(t.TempDir())
	app.config = config.DefaultConfig()
	app.browserMgr = browser.NewManager(app.config, app.appRoot)
	storage := browser.NewSQLiteProxyDAO(db.GetConn())
	schedulerDAO := &proxyConnectorStateTestDAO{
		ProxyDAO:   storage,
		listCalled: make(chan struct{}),
	}
	memberStateDAO := browser.NewSQLiteProxyVirtualGroupMemberStateDAO(db.GetConn())
	app.browserMgr.ProxyDAO = schedulerDAO
	app.browserMgr.ProxyVirtualGroupMemberStateDAO = memberStateDAO
	t.Cleanup(app.stopSpeedScheduler)

	testedAt := time.Now().Format(time.RFC3339)
	proxy := BrowserProxy{
		ProxyId:               "proxy-connector-state",
		ProxyName:             "Connector state",
		ProxyConfig:           "http://127.0.0.1:8080",
		LastLatencyMs:         47,
		LastTestOk:            true,
		LastTestedAt:          testedAt,
		LastTestEngine:        "xray",
		LastTestConnectorType: config.BrowserConnectorXray,
		LastTestFingerprint:   "test-fingerprint",
		LastIPHealthJSON:      `{"ip":"203.0.113.10"}`,
	}
	if err := storage.Upsert(proxy); err != nil {
		t.Fatalf("Upsert proxy returned error: %v", err)
	}
	if err := storage.UpdateSpeedResultWithProvenance(proxy.ProxyId, true, proxy.LastLatencyMs, testedAt, proxy.LastTestEngine, proxy.LastTestConnectorType, proxy.LastTestFingerprint); err != nil {
		t.Fatalf("UpdateSpeedResultWithProvenance returned error: %v", err)
	}
	if err := storage.UpdateIPHealthResult(proxy.ProxyId, proxy.LastIPHealthJSON); err != nil {
		t.Fatalf("UpdateIPHealthResult returned error: %v", err)
	}
	app.config.Browser.Proxies = []config.BrowserProxy{proxy}
	if err := memberStateDAO.Upsert(browser.ProxyVirtualGroupMemberState{
		GroupId:           "group-connector-state",
		ProxyId:           proxy.ProxyId,
		State:             browser.MemberStateDegraded,
		LastConnectorType: config.BrowserConnectorXray,
	}); err != nil {
		t.Fatalf("Upsert member state returned error: %v", err)
	}

	cacheExpiresAt := time.Now().Add(time.Hour)
	app.virtualGroupSnapshot.mu.Lock()
	app.virtualGroupSnapshot.groups = []browser.ProxyVirtualGroup{{GroupId: "group-connector-state"}}
	app.virtualGroupSnapshot.proxies = []browser.Proxy{proxy}
	app.virtualGroupSnapshot.byProxy = map[string][]browser.ProxyVirtualGroup{proxy.ProxyId: {{GroupId: "group-connector-state"}}}
	app.virtualGroupSnapshot.expiresAt = cacheExpiresAt
	app.virtualGroupSnapshot.mu.Unlock()
	app.virtualGroupMemberStateGate.mu.Lock()
	app.virtualGroupMemberStateGate.nextProbe = map[string]time.Time{proxy.ProxyId: cacheExpiresAt}
	app.virtualGroupMemberStateGate.expiresAt = cacheExpiresAt
	app.virtualGroupMemberStateGate.mu.Unlock()

	previousScheduler := browser.NewProxySpeedScheduler(nil, nil, time.Hour, 1)
	previousScheduler.Start()
	app.speedScheduler = previousScheduler

	settings := app.GetBrowserSettings()
	settings.DefaultConnectorType = config.BrowserConnectorMihomo
	if err := app.SaveBrowserSettings(settings); err != nil {
		t.Fatalf("SaveBrowserSettings returned error: %v", err)
	}
	if app.config.Browser.DefaultConnectorType != config.BrowserConnectorMihomo {
		t.Fatalf("connector type = %q, want %q", app.config.Browser.DefaultConnectorType, config.BrowserConnectorMihomo)
	}

	persisted, err := storage.List()
	if err != nil {
		t.Fatalf("List persisted proxies returned error: %v", err)
	}
	if len(persisted) != 1 {
		t.Fatalf("persisted proxies = %#v, want one proxy", persisted)
	}
	assertProxyConnectionResultsKept(t, persisted[0], proxy)
	assertProxyConnectionResultsKept(t, app.config.Browser.Proxies[0], proxy)
	if proxySpeedResultMatchesConnector(persisted[0], app.defaultProxyConnectorType()) {
		t.Fatal("previous connector stack result was accepted by the new stack")
	}

	states, err := memberStateDAO.List()
	if err != nil {
		t.Fatalf("List member states returned error: %v", err)
	}
	if len(states) != 0 {
		t.Fatalf("member states = %#v, want empty", states)
	}

	app.virtualGroupSnapshot.mu.Lock()
	snapshotCleared := app.virtualGroupSnapshot.expiresAt.IsZero() && app.virtualGroupSnapshot.groups == nil && app.virtualGroupSnapshot.proxies == nil && app.virtualGroupSnapshot.byProxy == nil
	app.virtualGroupSnapshot.mu.Unlock()
	if !snapshotCleared {
		t.Fatal("virtual group snapshot cache was not cleared")
	}
	app.virtualGroupMemberStateGate.mu.Lock()
	gateCleared := app.virtualGroupMemberStateGate.expiresAt.IsZero() && app.virtualGroupMemberStateGate.nextProbe == nil
	app.virtualGroupMemberStateGate.mu.Unlock()
	if !gateCleared {
		t.Fatal("virtual group member-state gate cache was not cleared")
	}

	if app.speedScheduler == nil || app.speedScheduler == previousScheduler {
		t.Fatal("speed scheduler was not rebuilt after connector stack switch")
	}
	select {
	case <-schedulerDAO.listCalled:
	case <-time.After(time.Second):
		t.Fatal("rebuilt speed scheduler did not run once")
	}
}

func assertProxyConnectionResultsKept(t *testing.T, got BrowserProxy, want BrowserProxy) {
	t.Helper()
	if got.LastLatencyMs != want.LastLatencyMs || got.LastTestOk != want.LastTestOk ||
		got.LastTestedAt != want.LastTestedAt || got.LastTestEngine != want.LastTestEngine ||
		got.LastTestConnectorType != want.LastTestConnectorType ||
		got.LastTestFingerprint != want.LastTestFingerprint ||
		got.LastIPHealthJSON != want.LastIPHealthJSON {
		t.Fatalf("proxy connection results were changed: %#v", got)
	}
}
