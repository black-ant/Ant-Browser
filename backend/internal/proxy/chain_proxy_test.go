package proxy

import (
	"encoding/json"
	"net/url"
	"os"
	"strings"
	"testing"

	"ant-chrome/backend/internal/config"
)

func buildGenericChainTestConfig(t *testing.T, frontID string, protocol string) string {
	t.Helper()
	payload := chainProxyConfig{
		Version:      2,
		FrontProxyID: frontID,
		Landing:      chainSocks5Hop{Protocol: protocol, Server: "tr.example.com", Port: 1080, Username: "landing-user", Password: "landing-pass"},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return chainProxyPrefix + url.QueryEscape(string(data))
}

func TestGenericChainXrayUsesVLESSFrontToDialLanding(t *testing.T) {
	chain := buildGenericChainTestConfig(t, "front-vless", "socks5")
	proxies := []config.BrowserProxy{{
		ProxyId:     "front-vless",
		ProxyConfig: "vless://00000000-0000-0000-0000-000000000000@example.com:443?security=tls&sni=example.com&type=tcp",
	}}
	cfg, err := ParseChainProxyConfig(chain)
	if err != nil {
		t.Fatal(err)
	}
	outbounds, routes, err := buildXrayChainOutbounds(cfg, proxies, "chain-id")
	if err != nil {
		t.Fatal(err)
	}
	if len(outbounds) != 2 || len(routes) != 1 {
		t.Fatalf("outbounds/routes = %d/%d", len(outbounds), len(routes))
	}
	front := outbounds[0].(map[string]interface{})
	landing := outbounds[1].(map[string]interface{})
	if front["protocol"] != "vless" || front["tag"] != "front-hop" {
		t.Fatalf("front = %#v", front)
	}
	proxySettings, ok := landing["proxySettings"].(map[string]interface{})
	if !ok || proxySettings["tag"] != "front-hop" {
		t.Fatalf("landing proxySettings = %#v", landing["proxySettings"])
	}
	if routes[0].(map[string]interface{})["outboundTag"] != "landing-hop" {
		t.Fatalf("route = %#v", routes[0])
	}

	resolution, err := ResolveProxyKernel(chain, proxies, "chain-id", "")
	if err != nil {
		t.Fatal(err)
	}
	if resolution.Kernel != ProxyKernelXray {
		t.Fatalf("kernel = %s, want xray", resolution.Kernel)
	}
}

func TestGenericChainSingBoxDetoursLandingThroughHysteria2(t *testing.T) {
	chain := buildGenericChainTestConfig(t, "front-hy2", "https")
	proxies := []config.BrowserProxy{{ProxyId: "front-hy2", ProxyConfig: "hysteria2://password@hy.example.com:443?sni=hy.example.com"}}
	cfg, err := ParseChainProxyConfig(chain)
	if err != nil {
		t.Fatal(err)
	}
	outbounds, routeTag, err := buildSingBoxChainOutbounds(cfg, proxies, "chain-id")
	if err != nil {
		t.Fatal(err)
	}
	if routeTag != "landing-hop" || len(outbounds) != 2 {
		t.Fatalf("route/outbounds = %s/%d", routeTag, len(outbounds))
	}
	landing := outbounds[1].(map[string]interface{})
	if landing["type"] != "http" || landing["detour"] != "front-hop" {
		t.Fatalf("landing = %#v", landing)
	}
	resolution, err := ResolveProxyKernel(chain, proxies, "chain-id", "")
	if err != nil {
		t.Fatal(err)
	}
	if resolution.Kernel != ProxyKernelSingBox {
		t.Fatalf("kernel = %s, want sing-box", resolution.Kernel)
	}
}

func TestGenericChainRejectsMissingOrNestedFront(t *testing.T) {
	chain := buildGenericChainTestConfig(t, "missing", "socks5")
	if _, err := ResolveProxyKernel(chain, nil, "chain-id", ""); err == nil {
		t.Fatal("expected missing front to fail")
	}
	nested := []config.BrowserProxy{{ProxyId: "missing", ProxyConfig: chain}}
	if _, err := ResolveProxyKernel(chain, nested, "chain-id", ""); err == nil {
		t.Fatal("expected nested front to fail")
	}
}

func TestGenericChainMihomoUsesDialerProxy(t *testing.T) {
	chain := buildGenericChainTestConfig(t, "front-vless", "socks5")
	proxies := []config.BrowserProxy{{ProxyId: "front-vless", ProxyConfig: "name: vless-front\ntype: vless\nserver: example.com\nport: 443\nuuid: 00000000-0000-0000-0000-000000000000\ntls: true"}}
	cfg, err := ParseChainProxyConfig(chain)
	if err != nil {
		t.Fatal(err)
	}
	nodes, target, err := buildMihomoChainNodes(cfg, proxies, "chain-id")
	if err != nil {
		t.Fatal(err)
	}
	if target != "landing-hop" || len(nodes) != 2 {
		t.Fatalf("target/nodes = %s/%d", target, len(nodes))
	}
	landing := nodes[1].(map[string]interface{})
	if landing["dialer-proxy"] != "front-hop" {
		t.Fatalf("landing = %#v", landing)
	}
}

func TestGenericChainFallsBackFromStaleMihomoPreferenceToXray(t *testing.T) {
	chain := buildGenericChainTestConfig(t, "front-vless", "socks5")
	proxies := []config.BrowserProxy{{ProxyId: "front-vless", ProxyConfig: "vless://00000000-0000-0000-0000-000000000000@example.com:443"}}
	resolution, err := ResolveProxyKernel(chain, proxies, "chain-id", ProxyKernelMihomo)
	if err != nil {
		t.Fatal(err)
	}
	if resolution.Kernel != ProxyKernelXray {
		t.Fatalf("kernel = %s, want xray", resolution.Kernel)
	}
	if !strings.Contains(resolution.Reason, "回退") {
		t.Fatalf("reason = %q", resolution.Reason)
	}
}

func TestGenericChainFallsBackFromStaleSingBoxPreferenceToXray(t *testing.T) {
	chain := buildGenericChainTestConfig(t, "front-vless", "socks5")
	proxies := []config.BrowserProxy{{ProxyId: "front-vless", ProxyConfig: "vless://00000000-0000-0000-0000-000000000000@example.com:443"}}
	resolution, err := ResolveProxyKernel(chain, proxies, "chain-id", ProxyKernelSingBox)
	if err != nil {
		t.Fatal(err)
	}
	if resolution.Kernel != ProxyKernelXray {
		t.Fatalf("kernel = %s, want xray", resolution.Kernel)
	}
}

func TestGenericChainStartsRealXrayBridgeWhenConfigured(t *testing.T) {
	binary := strings.TrimSpace(os.Getenv("ANTIBROWSER_TEST_XRAY"))
	if binary == "" {
		t.Skip("ANTIBROWSER_TEST_XRAY is not set")
	}
	chain := buildGenericChainTestConfig(t, "front-vless", "socks5")
	proxies := []config.BrowserProxy{{ProxyId: "front-vless", ProxyConfig: "vless://00000000-0000-0000-0000-000000000000@example.com:443?security=tls&sni=example.com&type=tcp"}}
	cfg := config.DefaultConfig()
	cfg.Browser.XrayBinaryPath = binary
	cfg.Browser.UserDataRoot = t.TempDir()
	manager := NewXrayManager(cfg, t.TempDir())
	socksURL, key, err := manager.AcquireBridge(chain, proxies, "chain-id")
	if err != nil {
		t.Fatal(err)
	}
	defer manager.ReleaseBridge(key)
	defer manager.StopAll()
	if !strings.HasPrefix(socksURL, "socks5://127.0.0.1:") {
		t.Fatalf("socks URL = %s", socksURL)
	}
}

func TestGenericChainStartsRealSingBoxBridgeWhenConfigured(t *testing.T) {
	binary := strings.TrimSpace(os.Getenv("ANTIBROWSER_TEST_SINGBOX"))
	if binary == "" {
		t.Skip("ANTIBROWSER_TEST_SINGBOX is not set")
	}
	chain := buildGenericChainTestConfig(t, "front-hy2", "socks5")
	proxies := []config.BrowserProxy{{ProxyId: "front-hy2", ProxyConfig: "hysteria2://password@hy.example.com:443?sni=hy.example.com"}}
	cfg := config.DefaultConfig()
	cfg.Browser.SingBoxBinaryPath = binary
	cfg.Browser.UserDataRoot = t.TempDir()
	manager := NewSingBoxManager(cfg, t.TempDir())
	socksURL, key, err := manager.AcquireBridge(chain, proxies, "chain-id")
	if err != nil {
		t.Fatal(err)
	}
	defer manager.ReleaseBridge(key)
	defer manager.StopAll()
	if !strings.HasPrefix(socksURL, "socks5://127.0.0.1:") {
		t.Fatalf("socks URL = %s", socksURL)
	}
}
