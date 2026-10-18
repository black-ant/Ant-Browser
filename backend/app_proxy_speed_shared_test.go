package backend

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/proxy"
)

func TestProxySpeedFingerprintIgnoresUnrelatedProxyChanges(t *testing.T) {
	app := NewApp("")
	app.config = config.DefaultConfig()
	proxies := []BrowserProxy{
		{ProxyId: "target", ProxyConfig: "http://127.0.0.1:8080"},
		{ProxyId: "unrelated", ProxyConfig: "http://127.0.0.2:8080"},
	}
	speedConfig := &proxy.SpeedTestConfig{Timeout: 3 * time.Second, TCPTimeout: 3 * time.Second, URLs: []string{"http://example.com/204"}, ExpectedStatus: []int{204}}
	initial := proxySpeedTestFingerprint("target", proxies, config.BrowserConnectorXray, speedConfig)

	proxies[1].ProxyConfig = "http://127.0.0.3:8080"
	if got := proxySpeedTestFingerprint("target", proxies, config.BrowserConnectorXray, speedConfig); got != initial {
		t.Fatal("an unrelated proxy change invalidated the target fingerprint")
	}
	proxies[0].DnsServers = "1.1.1.1"
	if got := proxySpeedTestFingerprint("target", proxies, config.BrowserConnectorXray, speedConfig); got == initial {
		t.Fatal("a target DNS change did not invalidate the fingerprint")
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
