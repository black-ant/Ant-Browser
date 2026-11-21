package proxy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func TestBridgeDNSServersPrefersCustomDNS(t *testing.T) {
	wantNodeDNS := strings.TrimSpace(issue85DNSYAML)
	proxies := []config.BrowserProxy{{ProxyId: "p1", DnsServers: issue85DNSYAML}}

	// 未配置自定义：走节点/订阅 DNS
	if got := BridgeDNSServers(nil, "", proxies, "p1"); got != wantNodeDNS {
		t.Fatalf("未配置自定义时应使用订阅 DNS，got %q", got)
	}
	if got := BridgeDNSServers(&config.Config{}, "", proxies, "p1"); got != wantNodeDNS {
		t.Fatalf("空配置应使用订阅 DNS，got %q", got)
	}

	// 配置了自定义：自定义优先
	custom := "1.1.1.1\nhttps://doh.example.com/dns-query"
	customCfg := &config.Config{Browser: config.BrowserConfig{BridgeCustomDNS: custom}}
	if got := BridgeDNSServers(customCfg, "", proxies, "p1"); got != custom {
		t.Fatalf("自定义 DNS 应覆盖订阅 DNS，got %q", got)
	}

	// 既无自定义也无节点 DNS：空（桥接回退公共/系统 DNS）
	if got := BridgeDNSServers(&config.Config{}, "", nil, ""); got != "" {
		t.Fatalf("无任何 DNS 时应返回空，got %q", got)
	}
}

func TestParseClashBridgeDNSAcceptsMultilineList(t *testing.T) {
	cfg := parseClashBridgeDNS("1.1.1.1\nhttps://doh.example.com/dns-query")
	if len(cfg.Nameserver) != 2 || cfg.Nameserver[0] != "1.1.1.1" || cfg.Nameserver[1] != "https://doh.example.com/dns-query" {
		t.Fatalf("多行 DNS 列表解析失败: %#v", cfg.Nameserver)
	}
}

func TestBuildMihomoDNSConfigEmptyWhenNoSubscriptionDNS(t *testing.T) {
	if dns := buildMihomoDNSConfig(""); dns != nil {
		t.Fatalf("无订阅 DNS 时不应生成 dns 段: %#v", dns)
	}
}

func TestHasUsableDNS(t *testing.T) {
	cases := []struct {
		raw  string
		want bool
	}{
		{"", false},
		{"abc", false},
		{"1.1.1.1", true},
		{"https://doh.example.com/dns-query", true},
		{"dns:\n  enable: false\n  nameserver:\n    - 1.1.1.1", false},
		{"dns:\n  nameserver:\n    - bad-host\n  proxy-server-nameserver:\n    - https://doh.example.com/dns-query", true},
	}
	for _, tc := range cases {
		if got := HasUsableDNS(tc.raw); got != tc.want {
			t.Fatalf("HasUsableDNS(%q) = %v, want %v", tc.raw, got, tc.want)
		}
	}
}

func TestBuildMihomoDNSConfigFiltersInvalidAddresses(t *testing.T) {
	dns := buildMihomoDNSConfig("dns:\n  nameserver:\n    - bad-host\n    - 1.1.1.1")
	if dns == nil {
		t.Fatal("有效的 DNS 项应保留")
	}
	servers, ok := dns["nameserver"].([]string)
	if !ok || len(servers) != 1 || servers[0] != "1.1.1.1" {
		t.Fatalf("nameserver = %#v, want [1.1.1.1]", dns["nameserver"])
	}
	if got := buildMihomoDNSConfig("abc"); got != nil {
		t.Fatalf("全部无效时不应生成 dns 段: %#v", got)
	}
}

func TestDNSConfigDisabledWhenEnableFalse(t *testing.T) {
	raw := "dns:\n  enable: false\n  nameserver:\n    - 1.1.1.1\n  proxy-server-nameserver:\n    - https://doh.example.com/dns-query"
	if got := buildMihomoDNSConfig(raw); got != nil {
		t.Fatalf("enable:false 不应注入 mihomo dns 段: %#v", got)
	}
	cfg, resolver := buildSingBoxDNSConfig(raw)
	if resolver != "" || cfg["final"] != singBoxPublicDNSTag {
		t.Fatalf("enable:false 不应影响 sing-box 节点解析: resolver=%q cfg=%#v", resolver, cfg)
	}
}

func TestBuildDnsDiagnosticSummaryCountsProxyServerNameserver(t *testing.T) {
	raw := "dns:\n  nameserver:\n    - 223.5.5.5\n  proxy-server-nameserver:\n    - https://doh.example.com/dns-query"
	summary := buildDnsDiagnosticSummary(raw)
	if summary.SourceFormat != "clash" {
		t.Fatalf("source format = %q, want clash", summary.SourceFormat)
	}
	if summary.ProxyServerNameserverCount != 1 {
		t.Fatalf("proxy-server-nameserver count = %d, want 1", summary.ProxyServerNameserverCount)
	}
	if summary.XrayServerCount != 2 {
		t.Fatalf("xray server count = %d, want 2 (nameserver + proxy-server-nameserver)", summary.XrayServerCount)
	}
}

func TestComputeBridgeKeyFallsBackToLegacyKeyWithoutDNS(t *testing.T) {
	src := "anytls://pw@node.example.com:443"
	if got, want := ComputeBridgeKey(src, "", ProxyKernelMihomo), computeNodeKey(src+"\x00mihomo"); got != want {
		t.Fatalf("mihomo legacy key = %q, want %q", got, want)
	}
	if got, want := ComputeBridgeKey(src, "", ProxyKernelSingBox), computeNodeKey(src); got != want {
		t.Fatalf("sing-box legacy key = %q, want %q", got, want)
	}
}

func TestBuildProxyDiagnosticNodeKeyMatchesBridgeKey(t *testing.T) {
	src := "anytls://pw@node.example.com:443?sni=node.example.com"
	proxies := []config.BrowserProxy{{
		ProxyId:         "p1",
		ProxyConfig:     src,
		DnsServers:      issue85DNSYAML,
		PreferredKernel: ProxyKernelMihomo,
	}}

	diag := BuildProxyDiagnostic("", proxies, "p1", BuildDiagnosticOptions{ConnectorType: config.BrowserConnectorMihomo})
	if diag.NodeKey != ComputeBridgeKey(src, issue85DNSYAML, ProxyKernelMihomo) {
		t.Fatalf("诊断 NodeKey 与桥接 key 不一致（开启）: %q", diag.NodeKey)
	}

	customDNS := "1.1.1.1"
	customCfg := &config.Config{Browser: config.BrowserConfig{BridgeCustomDNS: customDNS}}
	diagCustom := BuildProxyDiagnostic("", proxies, "p1", BuildDiagnosticOptions{
		ConnectorType: config.BrowserConnectorMihomo,
		Config:        customCfg,
	})
	if diagCustom.NodeKey != ComputeBridgeKey(src, customDNS, ProxyKernelMihomo) {
		t.Fatalf("诊断 NodeKey 未使用自定义 DNS: %q", diagCustom.NodeKey)
	}
}
