package proxy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"ant-chrome/backend/internal/config"

	"gopkg.in/yaml.v3"
)

const issue85DNSYAML = `dns:
  enable: true
  ipv6: false
  default-nameserver:
    - 223.5.5.5
    - 119.29.29.29
  nameserver:
    - 223.5.5.5
    - https://doh.example.com/dns-query
  proxy-server-nameserver:
    - https://doh.example.com/dns-query
`

func TestParseClashBridgeDNSAcceptsLegacyList(t *testing.T) {
	cfg := parseClashBridgeDNS(" 1.1.1.1 , 8.8.8.8 ")
	if len(cfg.Nameserver) != 2 || cfg.Nameserver[0] != "1.1.1.1" || cfg.Nameserver[1] != "8.8.8.8" {
		t.Fatalf("legacy list parse = %#v", cfg.Nameserver)
	}
}

func TestBuildMihomoDNSConfigCarriesProxyServerNameserver(t *testing.T) {
	dns := buildMihomoDNSConfig(issue85DNSYAML)
	if dns == nil {
		t.Fatal("mihomo dns config is nil")
	}
	servers, ok := dns["proxy-server-nameserver"].([]string)
	if !ok || len(servers) != 1 || servers[0] != "https://doh.example.com/dns-query" {
		t.Fatalf("proxy-server-nameserver = %#v, want subscription DoH", dns["proxy-server-nameserver"])
	}
	if enable, ok := dns["enable"].(bool); !ok || !enable {
		t.Fatalf("dns.enable = %#v, want true", dns["enable"])
	}
	if _, exists := dns["listen"]; exists {
		t.Fatal("mihomo bridge dns must not bind a listen address")
	}
}

func TestBuildMihomoNodeConfigWritesSubscriptionDNS(t *testing.T) {
	root := t.TempDir()
	manager := NewClashManager(&config.Config{}, root)
	manager.Config.Browser.UserDataRoot = filepath.Join(root, "data")

	node := map[string]interface{}{
		"name":     "node",
		"type":     "anytls",
		"server":   "node.example.com",
		"port":     443,
		"password": "pw",
	}
	key := ComputeBridgeKey("anytls://pw@node.example.com:443", issue85DNSYAML, ProxyKernelMihomo)
	cfgPath, err := manager.buildMihomoNodeConfig(key, node, 17890, 17891, issue85DNSYAML)
	if err != nil {
		t.Fatalf("buildMihomoNodeConfig returned error: %v", err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var payload map[string]interface{}
	if err := yaml.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	dnsSection, ok := payload["dns"].(map[string]interface{})
	if !ok {
		t.Fatalf("generated mihomo config has no dns section: %s", string(data))
	}
	if _, exists := dnsSection["proxy-server-nameserver"]; !exists {
		t.Fatalf("generated mihomo config drops proxy-server-nameserver: %s", string(data))
	}
}

func TestBuildSingBoxConfigMapsNodeDomainResolver(t *testing.T) {
	root := t.TempDir()
	manager := &SingBoxManager{Config: &config.Config{}, AppRoot: root}
	manager.Config.Browser.UserDataRoot = filepath.Join(root, "data")

	outbound := map[string]interface{}{
		"type":        "anytls",
		"tag":         "proxy-out",
		"server":      "node.example.com",
		"server_port": 443,
		"password":    "pw",
	}
	key := ComputeBridgeKey("anytls://pw@node.example.com:443", issue85DNSYAML, ProxyKernelSingBox)
	cfgPath, err := manager.buildConfig(key, outbound, 17890, issue85DNSYAML)
	if err != nil {
		t.Fatalf("buildConfig returned error: %v", err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var cfg struct {
		DNS struct {
			Servers []map[string]interface{} `json:"servers"`
			Final   string                   `json:"final"`
		} `json:"dns"`
		Outbounds []map[string]interface{} `json:"outbounds"`
		Route     map[string]interface{}   `json:"route"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	var proxyResolver map[string]interface{}
	for _, server := range cfg.DNS.Servers {
		if server["tag"] == singBoxProxyResolverTag {
			proxyResolver = server
		}
	}
	if proxyResolver == nil {
		t.Fatalf("sing-box dns.servers missing %q: %s", singBoxProxyResolverTag, string(data))
	}
	if proxyResolver["type"] != "https" || proxyResolver["server"] != "doh.example.com" || proxyResolver["path"] != "/dns-query" {
		t.Fatalf("proxy resolver server = %#v", proxyResolver)
	}
	if proxyResolver["domain_resolver"] != "bootstrap-dns" {
		t.Fatalf("DoH bootstrap resolver = %#v, want %q", proxyResolver["domain_resolver"], "bootstrap-dns")
	}

	if len(cfg.Outbounds) == 0 || cfg.Outbounds[0]["domain_resolver"] != singBoxProxyResolverTag {
		t.Fatalf("node outbound domain_resolver = %#v, want %q", outbound["domain_resolver"], singBoxProxyResolverTag)
	}
	if cfg.Route["default_domain_resolver"] != singBoxPublicDNSTag {
		t.Fatalf("route.default_domain_resolver = %#v, want %q", cfg.Route["default_domain_resolver"], singBoxPublicDNSTag)
	}
	if _, exists := outbound["domain_resolver"]; exists {
		t.Fatal("buildConfig mutated the caller outbound map")
	}
}

func TestBuildSingBoxDNSConfigFallsBackWithoutSubscriptionDNS(t *testing.T) {
	dns, resolverTag := buildSingBoxDNSConfig("")
	if resolverTag != "" {
		t.Fatalf("resolver tag = %q, want empty", resolverTag)
	}
	if dns["final"] != singBoxPublicDNSTag {
		t.Fatalf("dns.final = %#v, want %q", dns["final"], singBoxPublicDNSTag)
	}
}

func TestComputeBridgeKeyIncludesDNSForEveryEngine(t *testing.T) {
	src := "anytls://pw@node.example.com:443"
	for _, engine := range []string{ProxyKernelXray, ProxyKernelSingBox, ProxyKernelMihomo} {
		if ComputeBridgeKey(src, "dns-a", engine) == ComputeBridgeKey(src, "dns-b", engine) {
			t.Fatalf("%s bridge key ignored subscription DNS", engine)
		}
	}
}
