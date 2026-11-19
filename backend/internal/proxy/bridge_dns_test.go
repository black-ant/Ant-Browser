package proxy

import (
	"ant-chrome/backend/internal/config"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

const sampleSubscriptionDNSYAML = `dns:
  enable: true
  ipv6: false
  listen: 0.0.0.0:53
  enhanced-mode: fake-ip
  nameserver-policy:
    geosite:cn:
      - 223.5.5.5
  default-nameserver:
    - 223.5.5.5
    - 119.29.29.29
  nameserver:
    - https://doh.example.com/dns-query#🚀 节点选择
  proxy-server-nameserver:
    - https://resolver.example.com:8443/custom-token/dns-query#h3=true`

func TestComputeBridgeKeyIncludesDNSForMihomoAndSingBox(t *testing.T) {
	proxyConfig := "anytls://secret@anytls.example.com:443?sni=sni.example.com#node-1"

	mihomoKeyA := ComputeBridgeKey(proxyConfig, "dns:\n  proxy-server-nameserver:\n    - https://dns-a.example.com/dns-query", ProxyKernelMihomo)
	mihomoKeyB := ComputeBridgeKey(proxyConfig, "dns:\n  proxy-server-nameserver:\n    - https://dns-b.example.com/dns-query", ProxyKernelMihomo)
	if mihomoKeyA == mihomoKeyB {
		t.Fatalf("mihomo bridge key ignored subscription DNS changes: %s", mihomoKeyA)
	}

	singBoxKeyA := ComputeBridgeKey(proxyConfig, "dns:\n  proxy-server-nameserver:\n    - https://dns-a.example.com/dns-query", ProxyKernelSingBox)
	singBoxKeyB := ComputeBridgeKey(proxyConfig, "dns:\n  proxy-server-nameserver:\n    - https://dns-b.example.com/dns-query", ProxyKernelSingBox)
	if singBoxKeyA == singBoxKeyB {
		t.Fatalf("sing-box bridge key ignored subscription DNS changes: %s", singBoxKeyA)
	}

	xrayKeyA := ComputeBridgeKey(proxyConfig, "dns:\n  proxy-server-nameserver:\n    - https://dns-a.example.com/dns-query", ProxyKernelXray)
	if singBoxKeyA == xrayKeyA || mihomoKeyA == xrayKeyA || singBoxKeyA == mihomoKeyA {
		t.Fatalf("distinct kernels must not share bridge keys: xray=%s singbox=%s mihomo=%s", xrayKeyA, singBoxKeyA, mihomoKeyA)
	}
}

func TestMihomoEnsureNodeBridgeWritesSubscriptionDNSConfig(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell stub helper requires unix sh")
	}

	tempDir := t.TempDir()
	fakeMihomo := filepath.Join(tempDir, "fake-mihomo.sh")
	if err := os.WriteFile(fakeMihomo, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatalf("write fake mihomo failed: %v", err)
	}

	appCfg := config.DefaultConfig()
	appCfg.Browser.UserDataRoot = filepath.Join(tempDir, "userdata")
	appCfg.Browser.ClashBinaryPath = fakeMihomo

	nodeSrc := "anytls://secret@anytls.example.com:443?sni=sni.example.com#anytls-1"
	proxies := []config.BrowserProxy{
		{
			ProxyId:     "proxy-anytls-1",
			ProxyName:   "anytls-1",
			ProxyConfig: nodeSrc,
			DnsServers:  sampleSubscriptionDNSYAML,
		},
	}

	mgr := &ClashManager{
		Config:  appCfg,
		AppRoot: tempDir,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	_, _ = mgr.EnsureNodeBridgeContext(ctx, "", proxies, "proxy-anytls-1")

	expectedKey := ComputeBridgeKey(nodeSrc, sampleSubscriptionDNSYAML, ProxyKernelMihomo)
	cfgPath := filepath.Join(appCfg.Browser.UserDataRoot, "_mihomo", expectedKey, "mihomo-config.yaml")
	rawBytes, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("expected mihomo config at %s: %v", cfgPath, err)
	}

	var parsed map[string]interface{}
	if err := yaml.Unmarshal(rawBytes, &parsed); err != nil {
		t.Fatalf("unmarshal mihomo config failed: %v", err)
	}

	dnsSection, ok := parsed["dns"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected dns section in mihomo-config.yaml, got: %s", string(rawBytes))
	}
	if dnsSection["enable"] != true {
		t.Fatalf("dns.enable = %v, want true", dnsSection["enable"])
	}
	if _, hasListen := dnsSection["listen"]; hasListen {
		t.Fatalf("dns.listen must be stripped to avoid port conflicts across node bridges: %#v", dnsSection["listen"])
	}
	if _, hasPolicy := dnsSection["nameserver-policy"]; hasPolicy {
		t.Fatalf("dns.nameserver-policy must be stripped to avoid missing geodata failures: %#v", dnsSection["nameserver-policy"])
	}

	proxyNS, ok := dnsSection["proxy-server-nameserver"].([]interface{})
	if !ok || len(proxyNS) != 1 || proxyNS[0] != "https://resolver.example.com:8443/custom-token/dns-query#h3=true" {
		t.Fatalf("dns.proxy-server-nameserver = %#v", dnsSection["proxy-server-nameserver"])
	}

	ns, ok := dnsSection["nameserver"].([]interface{})
	if !ok || len(ns) != 1 || ns[0] != "https://doh.example.com/dns-query#proxy-out" {
		t.Fatalf("dns.nameserver = %#v, want group rewritten to #proxy-out", dnsSection["nameserver"])
	}

	diag := BuildProxyDiagnostic("", proxies, "proxy-anytls-1", BuildDiagnosticOptions{
		ClashMgr:      mgr,
		ConnectorType: config.BrowserConnectorMihomo,
	})
	if diag.NodeKey != expectedKey {
		t.Fatalf("mihomo diagnostic NodeKey = %s, want %s", diag.NodeKey, expectedKey)
	}
}

func TestSingBoxEnsureBridgeWritesValidSubscriptionDNSConfig(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell stub helper requires unix sh")
	}

	tempDir := t.TempDir()
	fakeSingBox := filepath.Join(tempDir, "fake-singbox.sh")
	stubScript := "#!/bin/sh\nif [ \"$1\" = \"check\" ]; then exit 0; fi\nexit 1\n"
	if err := os.WriteFile(fakeSingBox, []byte(stubScript), 0o755); err != nil {
		t.Fatalf("write fake sing-box failed: %v", err)
	}

	appCfg := config.DefaultConfig()
	appCfg.Browser.UserDataRoot = filepath.Join(tempDir, "userdata")
	appCfg.Browser.SingBoxBinaryPath = fakeSingBox
	appCfg.ProxyCheck.BridgeStartTimeoutMs = 300

	nodeSrc := "anytls://secret@anytls.example.com:443?sni=sni.example.com#anytls-1"
	proxies := []config.BrowserProxy{
		{
			ProxyId:     "proxy-anytls-1",
			ProxyName:   "anytls-1",
			ProxyConfig: nodeSrc,
			DnsServers:  sampleSubscriptionDNSYAML,
		},
	}

	mgr := &SingBoxManager{
		Config:      appCfg,
		AppRoot:     tempDir,
		Bridges:     make(map[string]*SingBoxBridge),
		launchLocks: make(map[string]*bridgeLaunchLock),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _ = mgr.EnsureBridgeContext(ctx, "", proxies, "proxy-anytls-1")

	expectedKey := ComputeBridgeKey(nodeSrc, sampleSubscriptionDNSYAML, ProxyKernelSingBox)
	cfgPath := filepath.Join(appCfg.Browser.UserDataRoot, "_singbox", expectedKey, "singbox-config.json")
	rawBytes, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("expected sing-box config at %s: %v", cfgPath, err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(rawBytes, &parsed); err != nil {
		t.Fatalf("unmarshal sing-box config failed: %v", err)
	}

	route, ok := parsed["route"].(map[string]interface{})
	if !ok {
		t.Fatalf("route missing: %#v", parsed["route"])
	}
	resolverTag, _ := route["default_domain_resolver"].(string)
	if resolverTag == "" || resolverTag == "public-dns" {
		t.Fatalf("route.default_domain_resolver = %q, want subscription proxy-server-nameserver tag", resolverTag)
	}

	dnsSection, ok := parsed["dns"].(map[string]interface{})
	if !ok {
		t.Fatalf("dns missing: %#v", parsed["dns"])
	}
	servers, ok := dnsSection["servers"].([]interface{})
	if !ok {
		t.Fatalf("dns.servers missing: %#v", dnsSection["servers"])
	}

	foundProxyResolver := false
	for _, rawSrv := range servers {
		srv, ok := rawSrv.(map[string]interface{})
		if !ok || srv["tag"] != resolverTag {
			continue
		}
		foundProxyResolver = true
		if srv["type"] != "h3" {
			t.Fatalf("proxy resolver type = %v, want h3 (from #h3=true)", srv["type"])
		}
		if srv["server"] != "resolver.example.com" {
			t.Fatalf("proxy resolver server = %v, want resolver.example.com", srv["server"])
		}
		if srv["path"] != "/custom-token/dns-query" {
			t.Fatalf("proxy resolver path = %v, want /custom-token/dns-query", srv["path"])
		}
		if srv["domain_resolver"] != "public-dns" {
			t.Fatalf("proxy resolver domain_resolver = %v, want public-dns", srv["domain_resolver"])
		}
	}
	if !foundProxyResolver {
		t.Fatalf("did not find server with tag %q in dns.servers: %s", resolverTag, string(rawBytes))
	}

	// Validate with bundled real sing-box binary when present in repo.
	realSingBox := filepath.Join("..", "..", "..", "bin", runtime.GOOS+"-"+runtime.GOARCH, "sing-box")
	if info, statErr := os.Stat(realSingBox); statErr == nil && !info.IsDir() {
		cmd := exec.Command(realSingBox, "check", "-c", cfgPath)
		if out, checkErr := cmd.CombinedOutput(); checkErr != nil {
			t.Fatalf("real sing-box check rejected generated config: %v\n%s\nconfig:\n%s", checkErr, strings.TrimSpace(string(out)), string(rawBytes))
		}
	}

	diag := BuildProxyDiagnostic("", proxies, "proxy-anytls-1", BuildDiagnosticOptions{
		SingBoxMgr:    mgr,
		ConnectorType: config.BrowserConnectorXray,
	})
	if diag.NodeKey != expectedKey {
		t.Fatalf("sing-box diagnostic NodeKey = %s, want %s", diag.NodeKey, expectedKey)
	}
}

func TestSubscriptionDNSEdgeCasesAndSingBoxValidation(t *testing.T) {
	if got := buildMihomoDNSConfig("dns:\n  enable: false\n  nameserver:\n    - 8.8.8.8"); got != nil {
		t.Fatalf("expected disabled dns block to return nil, got %#v", got)
	}

	onlyProxyNS := `dns:
  proxy-server-nameserver:
    - https://doh.example.com/dns-query#ProxyGroup`
	mihomoDNS := buildMihomoDNSConfig(onlyProxyNS)
	if mihomoDNS == nil {
		t.Fatal("expected mihomo DNS map for proxy-server-nameserver only input")
	}
	proxyList, _ := mihomoDNS["proxy-server-nameserver"].([]string)
	if len(proxyList) != 1 || proxyList[0] != "https://doh.example.com/dns-query" {
		t.Fatalf("proxy-server-nameserver should strip unknown group to avoid cycle: %#v", mihomoDNS["proxy-server-nameserver"])
	}
	defaultList, _ := mihomoDNS["default-nameserver"].([]string)
	if len(defaultList) == 0 {
		t.Fatalf("expected IP default-nameserver bootstrap for domain DoH: %#v", mihomoDNS)
	}
	nsList, _ := mihomoDNS["nameserver"].([]string)
	if len(nsList) == 0 {
		t.Fatalf("expected fallback nameserver when omitted: %#v", mihomoDNS)
	}

	flatDNS := "1.1.1.1, tls://dns.google:853, quic://dns.adguard-dns.com, tcp://8.8.4.4:5353, system, 2400:3200::1"
	appCfg := config.DefaultConfig()
	appCfg.Browser.UserDataRoot = t.TempDir()
	sbMgr := &SingBoxManager{Config: appCfg, AppRoot: t.TempDir()}
	cfgPath, err := sbMgr.buildConfigWithDNS("multi-scheme-test", map[string]interface{}{
		"type": "direct",
		"tag":  "proxy-out",
	}, 20808, flatDNS)
	if err != nil {
		t.Fatalf("buildConfigWithDNS failed: %v", err)
	}

	realSingBox := filepath.Join("..", "..", "..", "bin", runtime.GOOS+"-"+runtime.GOARCH, "sing-box")
	if info, statErr := os.Stat(realSingBox); statErr == nil && !info.IsDir() {
		cmd := exec.Command(realSingBox, "check", "-c", cfgPath)
		if out, checkErr := cmd.CombinedOutput(); checkErr != nil {
			raw, _ := os.ReadFile(cfgPath)
			t.Fatalf("real sing-box check failed on multi-scheme DNS config: %v\n%s\n%s", checkErr, strings.TrimSpace(string(out)), string(raw))
		}
	}
}
