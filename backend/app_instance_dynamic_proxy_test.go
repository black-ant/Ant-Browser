package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"testing"
	"time"
)

func TestSelectDynamicBrowserProxyCandidatesFiltersAndSorts(t *testing.T) {
	testedAt := time.Now().Format(time.RFC3339)
	proxies := []BrowserProxy{
		{ProxyId: "direct", ProxyConfig: "direct://", GroupName: "mobile", LastTestOk: true, LastTestedAt: testedAt, LastLatencyMs: 1},
		{ProxyId: "failed", ProxyConfig: "http://failed:8080", GroupName: "mobile", LastTestOk: false, LastTestedAt: testedAt, LastLatencyMs: 2},
		{ProxyId: "stale", ProxyConfig: "http://stale:8080", GroupName: "mobile", LastTestOk: true, LastTestedAt: time.Now().Add(-defaultDynamicProxyMaxTestAge - time.Minute).Format(time.RFC3339), LastLatencyMs: 3},
		{ProxyId: "slow", ProxyConfig: "http://slow:8080", GroupName: "mobile", LastTestOk: true, LastTestedAt: testedAt, LastLatencyMs: 400},
		{ProxyId: "fast", ProxyConfig: "http://fast:8080", GroupName: "mobile", LastTestOk: true, LastTestedAt: testedAt, LastLatencyMs: 100},
		{ProxyId: "other-group", ProxyConfig: "http://other:8080", GroupName: "desktop", LastTestOk: true, LastTestedAt: testedAt, LastLatencyMs: 50},
	}
	candidates := selectDynamicBrowserProxyCandidates(proxies, " MOBILE ", defaultDynamicProxyMaxTestAge)
	if len(candidates) != 2 || candidates[0].ProxyId != "fast" || candidates[1].ProxyId != "slow" {
		t.Fatalf("dynamic candidates = %#v, want [fast slow]", candidates)
	}
}

func TestDynamicProxyCandidatesUseConfiguredSpeedInterval(t *testing.T) {
	app := NewApp("")
	app.config = config.DefaultConfig()
	app.config.ProxyCheck.SpeedIntervalMinutes = 120
	proxies := []BrowserProxy{{
		ProxyId:       "proxy-1",
		ProxyConfig:   "http://proxy:8080",
		LastTestOk:    true,
		LastLatencyMs: 50,
		LastTestedAt:  time.Now().Add(-45 * time.Minute).Format(time.RFC3339),
	}}

	if got := selectDynamicBrowserProxyCandidates(proxies, "", app.dynamicProxyMaxTestAge()); len(got) != 1 {
		t.Fatalf("configured 120-minute interval rejected a 45-minute result: %#v", got)
	}

	app.config.ProxyCheck.SpeedIntervalMinutes = 30
	if got := selectDynamicBrowserProxyCandidates(proxies, "", app.dynamicProxyMaxTestAge()); len(got) != 0 {
		t.Fatalf("configured 30-minute interval accepted a 45-minute result: %#v", got)
	}
}

func TestResolveDynamicBrowserStartProxyFallsBackToNextCandidate(t *testing.T) {
	testedAt := time.Now().Format(time.RFC3339)
	app := NewApp("")
	app.config = config.DefaultConfig()
	app.config.Browser.Proxies = []config.BrowserProxy{
		{ProxyId: "invalid", ProxyConfig: "bad://invalid", LastTestOk: true, LastTestedAt: testedAt, LastLatencyMs: 10},
		{ProxyId: "valid", ProxyConfig: "http://127.0.0.1:8080", LastTestOk: true, LastTestedAt: testedAt, LastLatencyMs: 20},
	}
	for index := range app.config.Browser.Proxies {
		item := &app.config.Browser.Proxies[index]
		item.LastTestConnectorType = app.defaultProxyConnectorType()
	}
	app.browserMgr = browser.NewManager(app.config, "")
	profile := &BrowserProfile{ProfileId: "dynamic-profile", ProxyMode: browser.ProxyModeDynamic}
	proxyURL, _, _, err := app.resolveDynamicBrowserStartProxy(profile.ProfileId, profile, app.config.Browser.Proxies)
	if err != nil {
		t.Fatalf("dynamic proxy fallback returned error: %v", err)
	}
	if proxyURL != "http://127.0.0.1:8080" {
		t.Fatalf("resolved proxy URL = %q, want http://127.0.0.1:8080", proxyURL)
	}
}
