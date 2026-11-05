package proxy

import (
	"ant-chrome/backend/internal/config"
	"fmt"
	"net/url"
	"strings"
	"testing"
)

func TestValidateProxyConfigInvalidRawString(t *testing.T) {
	ok, msg := ValidateProxyConfig("not-a-proxy-config", nil, "")
	if ok {
		t.Fatalf("expected invalid raw string to fail validation")
	}
	if !strings.Contains(msg, "解析失败") {
		t.Fatalf("unexpected message: %s", msg)
	}
}

func TestValidateProxyConfigMissingProxyId(t *testing.T) {
	ok, msg := ValidateProxyConfig("", []config.BrowserProxy{
		{ProxyId: "p1", ProxyConfig: "http://proxy.invalid:8080"},
	}, "missing-proxy")
	if ok {
		t.Fatalf("expected missing proxyId to fail validation")
	}
	if !strings.Contains(msg, "不存在") {
		t.Fatalf("unexpected message: %s", msg)
	}
}

func TestValidateProxyConfigMissingProxyIdFallbackToRawConfig(t *testing.T) {
	ok, msg := ValidateProxyConfig("socks5://127.0.0.1:1080", []config.BrowserProxy{
		{ProxyId: "p1", ProxyConfig: "http://proxy.invalid:8080"},
	}, "missing-proxy")
	if !ok {
		t.Fatalf("expected fallback proxyConfig to pass, msg=%s", msg)
	}
}

func TestValidateProxyConfigStandardProxy(t *testing.T) {
	ok, msg := ValidateProxyConfig("socks5://127.0.0.1:1080", nil, "")
	if !ok {
		t.Fatalf("expected standard proxy to pass: %s", msg)
	}
}

func TestValidateProxyConfigChainSocks5Proxy(t *testing.T) {
	chainConfig := buildTestChainSocks5Config(t, 0)
	ok, msg := ValidateProxyConfig(chainConfig, nil, "")
	if !ok {
		t.Fatalf("expected chain proxy to pass: %s", msg)
	}
	if !RequiresBridge(chainConfig, nil, "") {
		t.Fatalf("expected chain proxy to require bridge")
	}
}

func TestValidateProxyConfigChainHTTPProxy(t *testing.T) {
	chainConfig := buildTestChainHTTPConfig(t)
	ok, msg := ValidateProxyConfig(chainConfig, nil, "")
	if !ok {
		t.Fatalf("expected chain http proxy to pass: %s", msg)
	}
	if !RequiresBridge(chainConfig, nil, "") {
		t.Fatalf("expected chain http proxy to require bridge")
	}
}

func TestRequiresLocalProxyBridgeForBrowserAuthenticatedSocks5(t *testing.T) {
	if !RequiresLocalProxyBridgeForBrowser("socks5://user:pass@127.0.0.1:1080") {
		t.Fatal("expected authenticated socks5 proxy to require browser bridge")
	}
	if !RequiresLocalProxyBridgeForBrowser("http://user:pass@127.0.0.1:8080") {
		t.Fatal("expected authenticated http proxy to require browser bridge")
	}
	if !RequiresLocalProxyBridgeForBrowser("https://user:pass@127.0.0.1:8443") {
		t.Fatal("expected authenticated https proxy to require browser bridge")
	}
	if RequiresLocalProxyBridgeForBrowser("socks5://127.0.0.1:1080") {
		t.Fatal("expected unauthenticated socks5 proxy not to require browser bridge")
	}
	if RequiresLocalProxyBridgeForBrowser("http://127.0.0.1:8080") {
		t.Fatal("expected unauthenticated http proxy not to require browser bridge")
	}
	if RequiresLocalProxyBridgeForBrowser("https://127.0.0.1:8443") {
		t.Fatal("expected unauthenticated https proxy not to require browser bridge")
	}
}

func TestBuildDirectProxyBridgeOutboundAuthenticatedSocks5(t *testing.T) {
	outbound, ok, err := buildDirectProxyBridgeOutbound("socks5://user:pass@127.0.0.1:1080")
	if err != nil {
		t.Fatalf("buildDirectProxyBridgeOutbound returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected authenticated socks5 proxy to build bridge outbound")
	}

	if outbound["protocol"] != "socks" {
		t.Fatalf("protocol = %v, want socks", outbound["protocol"])
	}
	settings, ok := outbound["settings"].(map[string]interface{})
	if !ok {
		t.Fatalf("settings missing: %+v", outbound)
	}
	servers, ok := settings["servers"].([]interface{})
	if !ok || len(servers) != 1 {
		t.Fatalf("servers invalid: %+v", settings["servers"])
	}
	server, ok := servers[0].(map[string]interface{})
	if !ok {
		t.Fatalf("server invalid: %+v", servers[0])
	}
	if server["address"] != "127.0.0.1" {
		t.Fatalf("address = %v, want 127.0.0.1", server["address"])
	}
	if got := int(server["port"].(int)); got != 1080 {
		t.Fatalf("port = %d, want 1080", got)
	}
	users, ok := server["users"].([]interface{})
	if !ok || len(users) != 1 {
		t.Fatalf("users invalid: %+v", server["users"])
	}
	user, ok := users[0].(map[string]interface{})
	if !ok {
		t.Fatalf("user invalid: %+v", users[0])
	}
	if user["user"] != "user" || user["pass"] != "pass" {
		t.Fatalf("unexpected user payload: %+v", user)
	}
}

func TestBuildDirectProxyBridgeOutboundAuthenticatedHTTP(t *testing.T) {
	outbound, ok, err := buildDirectProxyBridgeOutbound("http://user:pass@127.0.0.1:8080")
	if err != nil {
		t.Fatalf("buildDirectProxyBridgeOutbound returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected bridge outbound for authenticated http proxy")
	}
	if outbound["protocol"] != "http" {
		t.Fatalf("protocol = %v, want http", outbound["protocol"])
	}
	settings, ok := outbound["settings"].(map[string]interface{})
	if !ok {
		t.Fatalf("settings missing: %+v", outbound)
	}
	servers, ok := settings["servers"].([]interface{})
	if !ok || len(servers) != 1 {
		t.Fatalf("servers invalid: %+v", settings["servers"])
	}
	server, ok := servers[0].(map[string]interface{})
	if !ok {
		t.Fatalf("server invalid: %+v", servers[0])
	}
	users, ok := server["users"].([]interface{})
	if !ok || len(users) != 1 {
		t.Fatalf("users invalid: %+v", server["users"])
	}
	user, ok := users[0].(map[string]interface{})
	if !ok {
		t.Fatalf("user invalid: %+v", users[0])
	}
	if user["user"] != "user" || user["pass"] != "pass" {
		t.Fatalf("unexpected user payload: %+v", user)
	}
}

func TestBuildDirectProxyBridgeOutboundAuthenticatedHTTPS(t *testing.T) {
	outbound, ok, err := buildDirectProxyBridgeOutbound("https://user:pass@proxy.example.com:8443")
	if err != nil {
		t.Fatalf("buildDirectProxyBridgeOutbound returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected bridge outbound for authenticated https proxy")
	}
	if outbound["protocol"] != "http" {
		t.Fatalf("protocol = %v, want http", outbound["protocol"])
	}
	streamSettings, ok := outbound["streamSettings"].(map[string]interface{})
	if !ok {
		t.Fatalf("streamSettings missing: %+v", outbound)
	}
	if streamSettings["security"] != "tls" {
		t.Fatalf("security = %v, want tls", streamSettings["security"])
	}
	settings, ok := outbound["settings"].(map[string]interface{})
	if !ok {
		t.Fatalf("settings missing: %+v", outbound)
	}
	servers, ok := settings["servers"].([]interface{})
	if !ok || len(servers) != 1 {
		t.Fatalf("servers invalid: %+v", settings["servers"])
	}
	server, ok := servers[0].(map[string]interface{})
	if !ok {
		t.Fatalf("server invalid: %+v", servers[0])
	}
	if server["address"] != "proxy.example.com" || server["port"] != 8443 {
		t.Fatalf("server = %+v, want proxy.example.com:8443", server)
	}
}

func buildTestChainSocks5Config(t *testing.T, localPort int) string {
	t.Helper()
	localPortField := ""
	if localPort > 0 {
		localPortField = fmt.Sprintf(`,"localPort":%d`, localPort)
	}
	raw := fmt.Sprintf(`{"first":{"protocol":"socks5","server":"127.0.0.1","port":1081,"username":"u1","password":"p1"},"second":{"protocol":"socks5","server":"127.0.0.2","port":1082}%s}`, localPortField)
	return "chain+socks5://" + url.QueryEscape(raw)
}

func buildTestChainHTTPConfig(t *testing.T) string {
	t.Helper()
	raw := `{"first":{"protocol":"http","server":"first-hop.invalid","port":8080,"username":"u1","password":"p1"},"second":{"protocol":"http","server":"second-hop.invalid","port":8081,"username":"u2","password":"p2"}}`
	return "chain+socks5://" + url.QueryEscape(raw)
}

func TestDirectProxyBridgeIgnoresClashYAML(t *testing.T) {
	t.Parallel()

	src := "name: JP-Vmess(Dmit) 1.5x\n" +
		"type: vmess\n" +
		"server: example.com\n" +
		"port: 10091\n" +
		"uuid: 5ef4299f-a5eb-4aaa-9bf5-60b541b19294\n" +
		"alterId: 0\n" +
		"cipher: auto\n" +
		"tls: true\n" +
		"network: ws\n" +
		"ws-opts:\n" +
		"  path: /\n" +
		"  headers:\n" +
		"    Host: example.com\n"

	outbound, ok, err := buildDirectProxyBridgeOutbound(src)
	if err != nil {
		t.Fatalf("buildDirectProxyBridgeOutbound returned error: %v", err)
	}
	if ok || outbound != nil {
		t.Fatalf("clash yaml must not be treated as direct proxy bridge: ok=%v outbound=%v", ok, outbound)
	}

	standard, parsedOutbound, err := ParseProxyNode(src)
	if err != nil {
		t.Fatalf("ParseProxyNode returned error: %v", err)
	}
	if standard != "" {
		t.Fatalf("expected xray outbound, got standard proxy %q", standard)
	}
	if parsedOutbound == nil || parsedOutbound["protocol"] != "vmess" {
		t.Fatalf("expected vmess outbound, got %#v", parsedOutbound)
	}
}
