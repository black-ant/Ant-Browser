package proxy

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/metacubex/mihomo/adapter"

	"ant-chrome/backend/internal/config"
)

const mieruClashNode = `
name: 可乐云-HK01 Mieru
type: mieru
server: cursor.kaolun.cn
port: 40001
username: user-1
password: pass-1
transport: TCP
`

func TestMieruClashNodeIsMihomoOnlyProtocol(t *testing.T) {
	if !IsMihomoOnlyProtocol(mieruClashNode) {
		t.Fatalf("expected mieru Clash node to be treated as mihomo-only")
	}
	if IsSingBoxProtocol(mieruClashNode) {
		t.Fatalf("mieru must not be treated as sing-box protocol")
	}
	if RequiresBridge(mieruClashNode, nil, "") {
		t.Fatalf("mieru must not require xray bridge")
	}
	ok, msg := ValidateProxyConfig(mieruClashNode, nil, "")
	if !ok {
		t.Fatalf("ValidateProxyConfig rejected mieru node: %s", msg)
	}
}

func TestMieruClashNodeBuildsMihomoNode(t *testing.T) {
	node, err := buildMihomoNode(mieruClashNode)
	if err != nil {
		t.Fatalf("buildMihomoNode returned error: %v", err)
	}
	if node["type"] != "mieru" {
		t.Fatalf("type = %v, want mieru", node["type"])
	}
	if node["server"] != "cursor.kaolun.cn" {
		t.Fatalf("server = %v, want cursor.kaolun.cn", node["server"])
	}
	if node["port"] != 40001 {
		t.Fatalf("port = %v, want 40001", node["port"])
	}
}

func TestHTTPSStandardProxyBuildsMihomoHTTPNodeWithTLS(t *testing.T) {
	node, err := buildMihomoNode("https://user:pass@proxy.example.com:8443")
	if err != nil {
		t.Fatalf("buildMihomoNode returned error: %v", err)
	}
	if node["type"] != "http" {
		t.Fatalf("type = %v, want http", node["type"])
	}
	if node["server"] != "proxy.example.com" || node["port"] != 8443 {
		t.Fatalf("node endpoint = %+v, want proxy.example.com:8443", node)
	}
	if node["username"] != "user" || node["password"] != "pass" {
		t.Fatalf("auth = %+v, want user/pass", node)
	}
	if node["tls"] != true {
		t.Fatalf("tls = %v, want true", node["tls"])
	}
}

func TestMihomoNodeSupportsSingBoxURIs(t *testing.T) {
	cases := []struct {
		name       string
		source     string
		wantType   string
		wantServer string
		wantPort   int
	}{
		{
			name:       "hysteria2",
			source:     "hysteria2://password@example.com:443?sni=sni.example.com&insecure=1",
			wantType:   "hysteria2",
			wantServer: "example.com",
			wantPort:   443,
		},
		{
			name:       "tuic",
			source:     "tuic://uuid:password@example.com:443?sni=sni.example.com&insecure=1",
			wantType:   "tuic",
			wantServer: "example.com",
			wantPort:   443,
		},
		{
			name:       "anytls",
			source:     "anytls://password@example.com:443?sni=sni.example.com&insecure=1",
			wantType:   "anytls",
			wantServer: "example.com",
			wantPort:   443,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			node, err := buildMihomoNode(tc.source)
			if err != nil {
				t.Fatalf("buildMihomoNode returned error: %v", err)
			}
			if _, err := adapter.ParseProxy(node); err != nil {
				t.Fatalf("mihomo rejected converted node: %v", err)
			}
			if node["type"] != tc.wantType || node["server"] != tc.wantServer || node["port"] != tc.wantPort {
				t.Fatalf("node = %#v, want type=%s server=%s port=%d", node, tc.wantType, tc.wantServer, tc.wantPort)
			}
			if node["sni"] != "sni.example.com" || node["skip-cert-verify"] != true {
				t.Fatalf("TLS options = %#v, want sni and skip-cert-verify", node)
			}
		})
	}
}

func TestMihomoNodeSupportsV2RayURIs(t *testing.T) {
	vmessPayload := `{"add":"example.com","aid":"0","id":"00000000-0000-0000-0000-000000000000","net":"ws","path":"/ws","port":"443","ps":"vmess-test","tls":"tls","type":"none"}`
	cases := []struct {
		name       string
		source     string
		wantType   string
		wantServer string
		wantPort   int
	}{
		{
			name:       "vmess",
			source:     "vmess://" + base64.StdEncoding.EncodeToString([]byte(vmessPayload)),
			wantType:   "vmess",
			wantServer: "example.com",
			wantPort:   443,
		},
		{
			name:       "vless",
			source:     "vless://00000000-0000-0000-0000-000000000000@example.com:443?security=tls&sni=sni.example.com&type=ws&path=%2Fws&host=cdn.example.com",
			wantType:   "vless",
			wantServer: "example.com",
			wantPort:   443,
		},
		{
			name:       "trojan",
			source:     "trojan://password@example.com:443?sni=sni.example.com&type=ws&path=%2Fws&host=cdn.example.com",
			wantType:   "trojan",
			wantServer: "example.com",
			wantPort:   443,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			node, err := buildMihomoNode(tc.source)
			if err != nil {
				t.Fatalf("buildMihomoNode returned error: %v", err)
			}
			if _, err := adapter.ParseProxy(node); err != nil {
				t.Fatalf("mihomo rejected converted node: %v", err)
			}
			if node["type"] != tc.wantType || node["server"] != tc.wantServer || getMapInt(node, "port") != tc.wantPort {
				t.Fatalf("node = %#v, want type=%s server=%s port=%d", node, tc.wantType, tc.wantServer, tc.wantPort)
			}
		})
	}
}

func TestMihomoNodeSupportsHysteriaServerPortRanges(t *testing.T) {
	node, err := buildMihomoNode("hysteria2://password@example.com:443?mport=443-445&sni=sni.example.com")
	if err != nil {
		t.Fatalf("buildMihomoNode returned error: %v", err)
	}
	if _, err := adapter.ParseProxy(node); err != nil {
		t.Fatalf("mihomo rejected converted node: %v", err)
	}
	if node["ports"] != "443-445" {
		t.Fatalf("ports = %#v, want 443-445", node["ports"])
	}
}

func TestMieruSpeedTestRequiresMihomoConnector(t *testing.T) {
	proxyID := "mieru-proxy"
	result := SpeedTestWithConnector(
		proxyID,
		[]config.BrowserProxy{{ProxyId: proxyID, ProxyConfig: mieruClashNode}},
		nil,
		nil,
		nil,
		config.BrowserConnectorMihomo,
		&SpeedTestConfig{Timeout: 10, URLs: []string{"http://latency.test/generate_204"}},
	)
	if result.Ok {
		t.Fatalf("speed test should fail without mihomo connector, got success: %+v", result)
	}
	if result.Engine != config.BrowserConnectorMihomo {
		t.Fatalf("engine = %q, want mihomo; result=%+v", result.Engine, result)
	}
	if !strings.Contains(result.Error, "Mihomo") {
		t.Fatalf("error = %q, want Mihomo guidance", result.Error)
	}
}

func TestResolveMihomoBinaryFindsDownloadedCoreLayout(t *testing.T) {
	root := t.TempDir()
	binPath := filepath.Join(root, "bin", runtime.GOOS+"-"+runtime.GOARCH, "mihomo", "mihomo.exe")
	if runtime.GOOS != "windows" {
		binPath = filepath.Join(root, "bin", runtime.GOOS+"-"+runtime.GOARCH, "mihomo", "mihomo")
	}
	if err := os.MkdirAll(filepath.Dir(binPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binPath, []byte("test"), 0o755); err != nil {
		t.Fatal(err)
	}
	manager := &ClashManager{Config: &config.Config{}, AppRoot: root}
	got, err := manager.resolveMihomoBinary()
	if err != nil {
		t.Fatalf("resolveMihomoBinary returned error: %v", err)
	}
	if got != binPath {
		t.Fatalf("resolveMihomoBinary = %q, want %q", got, binPath)
	}
}
