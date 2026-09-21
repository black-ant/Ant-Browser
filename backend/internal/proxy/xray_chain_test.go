package proxy

import (
	"encoding/json"
	"net/url"
	"os"
	"testing"

	"ant-chrome/backend/internal/config"
)

func TestChainSocks5RuntimeConfigRoutesThroughSecondHop(t *testing.T) {
	chainConfig := buildTestChainSocks5Config(t, 19090)
	chainCfg, err := ParseChainSocks5Config(chainConfig)
	if err != nil {
		t.Fatalf("ParseChainSocks5Config returned error: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Browser.UserDataRoot = t.TempDir()
	manager := &XrayManager{
		Config:  cfg,
		AppRoot: t.TempDir(),
	}

	cfgPath, err := manager.buildRuntimeConfigWithRoute(
		"chain-test",
		[]interface{}{
			chainSocks5Outbound(chainCfg.First, "first-hop", ""),
			chainSocks5Outbound(chainCfg.Second, "second-hop", "first-hop"),
		},
		[]interface{}{
			map[string]interface{}{
				"type":        "field",
				"inboundTag":  []string{"socks-in"},
				"outboundTag": "second-hop",
			},
		},
		chainCfg.LocalPort,
		"",
	)
	if err != nil {
		t.Fatalf("buildRuntimeConfigWithRoute returned error: %v", err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read runtime config failed: %v", err)
	}
	var runtimeConfig map[string]interface{}
	if err := json.Unmarshal(data, &runtimeConfig); err != nil {
		t.Fatalf("unmarshal runtime config failed: %v", err)
	}

	inbounds := runtimeConfig["inbounds"].([]interface{})
	inbound := inbounds[0].(map[string]interface{})
	if got := int(inbound["port"].(float64)); got != 19090 {
		t.Fatalf("inbound port = %d, want 19090", got)
	}

	outbounds := runtimeConfig["outbounds"].([]interface{})
	byTag := map[string]map[string]interface{}{}
	for _, item := range outbounds {
		outbound := item.(map[string]interface{})
		if tag, ok := outbound["tag"].(string); ok {
			byTag[tag] = outbound
		}
	}
	secondHop := byTag["second-hop"]
	if secondHop == nil {
		t.Fatalf("second-hop outbound is missing: %+v", byTag)
	}
	proxySettings, ok := secondHop["proxySettings"].(map[string]interface{})
	if !ok {
		t.Fatalf("second-hop proxySettings is missing: %+v", secondHop)
	}
	if got := proxySettings["tag"]; got != "first-hop" {
		t.Fatalf("second-hop proxy tag = %v, want first-hop", got)
	}

	routing := runtimeConfig["routing"].(map[string]interface{})
	rules := routing["rules"].([]interface{})
	rule := rules[0].(map[string]interface{})
	if got := rule["outboundTag"]; got != "second-hop" {
		t.Fatalf("route outboundTag = %v, want second-hop", got)
	}
}

func TestChainVlessFirstHopBuildsXrayTwoHopRoute(t *testing.T) {
	raw := `{"first":{"proxyConfig":"vless://00000000-0000-0000-0000-000000000001@first-hop.invalid:443?security=tls&sni=first-hop.invalid&type=tcp"},"second":{"protocol":"socks5","server":"127.0.0.2","port":1080,"username":"u2","password":"p2"}}`
	chainConfig := chainSocks5Prefix + url.QueryEscape(raw)
	chainCfg, err := ParseChainSocks5Config(chainConfig)
	if err != nil {
		t.Fatalf("ParseChainSocks5Config returned error: %v", err)
	}
	first, err := chainFirstHopOutbound(chainCfg.First, "first-hop")
	if err != nil {
		t.Fatalf("chainFirstHopOutbound returned error: %v", err)
	}
	if first["protocol"] != "vless" || first["tag"] != "first-hop" {
		t.Fatalf("unexpected first hop: %+v", first)
	}
	second := chainSocks5Outbound(chainCfg.Second, "second-hop", "first-hop")
	proxySettings, ok := second["proxySettings"].(map[string]interface{})
	if !ok || proxySettings["tag"] != "first-hop" {
		t.Fatalf("second hop proxySettings invalid: %+v", second["proxySettings"])
	}
}

func TestChainAdvancedNodeRejectedOnSecondHop(t *testing.T) {
	raw := `{"first":{"protocol":"socks5","server":"127.0.0.1","port":1080},"second":{"proxyConfig":"vless://00000000-0000-0000-0000-000000000001@second-hop.invalid:443"}}`
	if _, err := ParseChainSocks5Config(chainSocks5Prefix + url.QueryEscape(raw)); err == nil {
		t.Fatal("expected second-hop advanced node config to be rejected")
	}
}

func TestAuthenticatedSocks5RuntimeConfigUsesLocalBridgeOutbound(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Browser.UserDataRoot = t.TempDir()
	manager := &XrayManager{
		Config:  cfg,
		AppRoot: t.TempDir(),
	}

	outbound, ok, err := buildDirectProxyBridgeOutbound("socks5://user:pass@first-hop.invalid:1080")
	if err != nil {
		t.Fatalf("buildDirectProxyBridgeOutbound returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected bridge outbound for authenticated socks5 proxy")
	}

	cfgPath, err := manager.buildRuntimeConfigWithRoute(
		"direct-auth-socks-test",
		[]interface{}{outbound},
		[]interface{}{
			map[string]interface{}{
				"type":        "field",
				"inboundTag":  []string{"socks-in"},
				"outboundTag": "proxy-out",
			},
		},
		19091,
		"",
	)
	if err != nil {
		t.Fatalf("buildRuntimeConfigWithRoute returned error: %v", err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read runtime config failed: %v", err)
	}
	var runtimeConfig map[string]interface{}
	if err := json.Unmarshal(data, &runtimeConfig); err != nil {
		t.Fatalf("unmarshal runtime config failed: %v", err)
	}

	outbounds := runtimeConfig["outbounds"].([]interface{})
	byTag := map[string]map[string]interface{}{}
	for _, item := range outbounds {
		current := item.(map[string]interface{})
		if tag, ok := current["tag"].(string); ok {
			byTag[tag] = current
		}
	}

	proxyOut := byTag["proxy-out"]
	if proxyOut == nil {
		t.Fatalf("proxy-out outbound is missing: %+v", byTag)
	}
	if proxyOut["protocol"] != "socks" {
		t.Fatalf("proxy-out protocol = %v, want socks", proxyOut["protocol"])
	}
}

func TestChainHTTPOutboundUsesAuthenticatedHTTPServer(t *testing.T) {
	outbound := chainSocks5Outbound(chainSocks5Hop{
		Protocol: "http",
		Server:   "first-hop.invalid",
		Port:     1080,
		Username: "user",
		Password: "pass",
	}, "first-hop", "")

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
	if server["address"] != "first-hop.invalid" {
		t.Fatalf("address = %v, want first-hop.invalid", server["address"])
	}
	if server["port"] != 1080 {
		t.Fatalf("port = %v, want 1080", server["port"])
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

func TestChainMixedHTTPAndSocksRuntimeConfig(t *testing.T) {
	chainConfig := "chain+socks5://%7B%22first%22%3A%7B%22protocol%22%3A%22http%22%2C%22server%22%3A%22127.0.0.1%22%2C%22port%22%3A8080%2C%22username%22%3A%22u1%22%2C%22password%22%3A%22p1%22%7D%2C%22second%22%3A%7B%22protocol%22%3A%22socks5%22%2C%22server%22%3A%22127.0.0.2%22%2C%22port%22%3A1080%2C%22username%22%3A%22u2%22%2C%22password%22%3A%22p2%22%7D%7D"
	chainCfg, err := ParseChainSocks5Config(chainConfig)
	if err != nil {
		t.Fatalf("ParseChainSocks5Config returned error: %v", err)
	}

	first := chainSocks5Outbound(chainCfg.First, "first-hop", "")
	second := chainSocks5Outbound(chainCfg.Second, "second-hop", "first-hop")
	if first["protocol"] != "http" {
		t.Fatalf("first protocol = %v, want http", first["protocol"])
	}
	if second["protocol"] != "socks" {
		t.Fatalf("second protocol = %v, want socks", second["protocol"])
	}
	proxySettings, ok := second["proxySettings"].(map[string]interface{})
	if !ok || proxySettings["tag"] != "first-hop" {
		t.Fatalf("second proxySettings invalid: %+v", second["proxySettings"])
	}
}
