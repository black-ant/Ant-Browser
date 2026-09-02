package backend

import "testing"

func TestResolveProxyConfigForAppPrefersExplicitConfig(t *testing.T) {
	proxyID := "proxy-1"
	got := resolveProxyConfigForApp(
		"http://temporary.example.com:8080",
		[]BrowserProxy{{ProxyId: proxyID, ProxyConfig: "http://saved.example.com:8080"}},
		proxyID,
	)
	if got != "http://temporary.example.com:8080" {
		t.Fatalf("resolveProxyConfigForApp = %q, want explicit config", got)
	}
}
