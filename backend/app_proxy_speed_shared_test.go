package backend

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
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
	return d.err
}

func (d *proxySpeedPersistenceFailureDAO) UpdateSpeedResultWithProvenance(string, bool, int64, string, string, string, string) error {
	return d.err
}

func (d *proxySpeedPersistenceFailureDAO) UpdateIPHealthResult(string, string) error { return nil }

func TestProxySpeedResultIsCurrentFollowsTimeWindow(t *testing.T) {
	app := NewApp("")
	app.config = config.DefaultConfig()
	fresh := BrowserProxy{
		ProxyId: "proxy-1", ProxyConfig: "http://127.0.0.1:8080",
		LastTestOk: true, LastLatencyMs: 40, LastTestedAt: time.Now().Format(time.RFC3339),
	}
	if !app.proxySpeedResultIsCurrent(fresh, []BrowserProxy{fresh}) {
		t.Fatal("fresh speed result was not considered current")
	}
	expired := fresh
	expired.LastTestedAt = time.Now().Add(-app.proxySpeedResultMaxAge() - time.Second).Format(time.RFC3339)
	if app.proxySpeedResultIsCurrent(expired, []BrowserProxy{expired}) {
		t.Fatal("expired speed result remained current")
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

func TestPersistProxySpeedResultWritesResult(t *testing.T) {
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

	result := ProxyTestResult{
		ProxyId:   initial.ProxyId,
		Ok:        true,
		LatencyMs: 25,
		Engine:    "xray",
		TestedAt:  time.Now().Format(time.RFC3339),
	}
	if err := app.persistProxySpeedResult(result); err != nil {
		t.Fatalf("persistProxySpeedResult returned error: %v", err)
	}
	stored, err := app.browserMgr.ProxyDAO.List()
	if err != nil {
		t.Fatalf("read proxy list failed: %v", err)
	}
	found := false
	for _, item := range stored {
		if item.ProxyId != initial.ProxyId {
			continue
		}
		found = true
		if !item.LastTestOk || item.LastLatencyMs != 25 || item.LastTestedAt == "" {
			t.Fatalf("speed result was not persisted: %#v", item)
		}
	}
	if !found {
		t.Fatal("persisted proxy row not found")
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
	result := ProxyTestResult{
		ProxyId:   initial.ProxyId,
		Ok:        true,
		LatencyMs: 25,
		TestedAt:  time.Now().Format(time.RFC3339),
	}
	if err := app.persistProxySpeedResult(result); err == nil {
		t.Fatal("persistProxySpeedResult succeeded despite storage failure")
	}
}

// 持久化失败不应把真实测速成功结果伪装成失败。
func TestTestSharedProxySpeedKeepsResultWhenPersistenceFails(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer target.Close()
	upstream := &http.Client{Timeout: 5 * time.Second}
	forward := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		outReq := r.Clone(r.Context())
		outReq.RequestURI = ""
		resp, err := upstream.Do(outReq)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		w.WriteHeader(resp.StatusCode)
	}))
	defer forward.Close()

	app := NewApp(t.TempDir())
	app.config = config.DefaultConfig()
	initial := BrowserProxy{ProxyId: "proxy-1", ProxyName: "Proxy 1", ProxyConfig: forward.URL}
	app.browserMgr = browser.NewManager(app.config, app.appRoot)
	app.browserMgr.ProxyDAO = &proxySpeedPersistenceFailureDAO{
		proxies: []browser.Proxy{initial},
		err:     errors.New("database write failed"),
	}
	speedConfig := &proxy.SpeedTestConfig{
		Timeout:        3 * time.Second,
		TCPTimeout:     3 * time.Second,
		URLs:           []string{target.URL},
		ExpectedStatus: []int{http.StatusNoContent},
	}
	result := app.testSharedProxySpeedWithOptions("proxy-1", true, []BrowserProxy{initial}, config.BrowserConnectorXray, speedConfig)
	if !result.Ok {
		t.Fatalf("persist failure flipped Ok to false: %#v", result)
	}
	if result.LatencyMs < 0 {
		t.Fatalf("latency = %d, want non-negative", result.LatencyMs)
	}
}

// provenance 写入失败但基础列可用时应降级成功。
type proxySpeedProvenanceFallbackDAO struct {
	proxySpeedPersistenceFailureDAO
	fallbackErr error
}

func (d *proxySpeedProvenanceFallbackDAO) UpdateSpeedResult(string, bool, int64, string) error {
	return d.fallbackErr
}

func TestPersistProxySpeedResultFallsBackToBasicColumns(t *testing.T) {
	app := NewApp(t.TempDir())
	app.config = config.DefaultConfig()
	initial := BrowserProxy{ProxyId: "proxy-1", ProxyName: "Proxy 1", ProxyConfig: "http://127.0.0.1:8080"}
	app.browserMgr = browser.NewManager(app.config, app.appRoot)
	app.browserMgr.ProxyDAO = &proxySpeedProvenanceFallbackDAO{
		proxySpeedPersistenceFailureDAO: proxySpeedPersistenceFailureDAO{
			proxies: []browser.Proxy{initial},
			err:     errors.New("no such column: last_test_engine"),
		},
		fallbackErr: nil,
	}
	result := ProxyTestResult{
		ProxyId:   initial.ProxyId,
		Ok:        true,
		LatencyMs: 30,
		Engine:    "xray",
		TestedAt:  time.Now().Format(time.RFC3339),
	}
	if err := app.persistProxySpeedResult(result); err != nil {
		t.Fatalf("persistProxySpeedResult should fall back to basic columns: %v", err)
	}
}
