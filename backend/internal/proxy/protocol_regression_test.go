package proxy

import (
	"testing"

	"ant-chrome/backend/internal/config"
)

func TestVlessRealityURIUsesRealitySettings(t *testing.T) {
	src := "vless://00000000-0000-0000-0000-000000000001@reality.example.com:443?security=reality&sni=sni.example.com&fp=chrome&pbk=public-key-1&sid=abcd&spx=%2F&type=tcp&flow=xtls-rprx-vision#Reality"

	_, outbound, err := ParseProxyNode(src)
	if err != nil {
		t.Fatalf("ParseProxyNode returned error: %v", err)
	}
	stream, ok := outbound["streamSettings"].(map[string]interface{})
	if !ok {
		t.Fatalf("streamSettings is %T, want map[string]interface{}", outbound["streamSettings"])
	}
	if got := stream["security"]; got != "reality" {
		t.Fatalf("security = %v, want reality", got)
	}
	if _, ok := stream["tlsSettings"]; ok {
		t.Fatalf("reality URI must not be downgraded to tlsSettings: %#v", stream)
	}
	reality, ok := stream["realitySettings"].(map[string]interface{})
	if !ok {
		t.Fatalf("realitySettings is %T, want map[string]interface{}", stream["realitySettings"])
	}
	if got := reality["serverName"]; got != "sni.example.com" {
		t.Fatalf("serverName = %v, want sni.example.com", got)
	}
	if got := reality["fingerprint"]; got != "chrome" {
		t.Fatalf("fingerprint = %v, want chrome", got)
	}
	if got := reality["publicKey"]; got != "public-key-1" {
		t.Fatalf("publicKey = %v, want public-key-1", got)
	}
	if got := reality["shortId"]; got != "abcd" {
		t.Fatalf("shortId = %v, want abcd", got)
	}
	if got := reality["spiderX"]; got != "/" {
		t.Fatalf("spiderX = %v, want /", got)
	}
}

func TestHysteriaURIStaysHysteriaV1(t *testing.T) {
	src := "hysteria://hy.example.com:443?auth=secret&peer=sni.example.com&insecure=1&upmbps=100&downmbps=200&obfs=obfs-token#HY1"
	if normalized := normalizeNodeScheme(src); normalized != src {
		t.Fatalf("normalizeNodeScheme = %q, want unchanged hysteria v1 URI", normalized)
	}

	out, err := BuildSingBoxOutbound(src)
	if err != nil {
		t.Fatalf("BuildSingBoxOutbound returned error: %v", err)
	}
	if got := out["type"]; got != "hysteria" {
		t.Fatalf("type = %v, want hysteria", got)
	}
	if _, ok := out["password"]; ok {
		t.Fatalf("hysteria v1 must not use hysteria2 password field: %#v", out)
	}
	if got := out["auth_str"]; got != "secret" {
		t.Fatalf("auth_str = %v, want secret", got)
	}
	if got := out["up_mbps"]; got != 100 {
		t.Fatalf("up_mbps = %v, want 100", got)
	}
	if got := out["down_mbps"]; got != 200 {
		t.Fatalf("down_mbps = %v, want 200", got)
	}
	if got := out["obfs"]; got != "obfs-token" {
		t.Fatalf("obfs = %v, want obfs-token", got)
	}
	tls, ok := out["tls"].(map[string]interface{})
	if !ok {
		t.Fatalf("tls is %T, want map[string]interface{}", out["tls"])
	}
	if got := tls["server_name"]; got != "sni.example.com" {
		t.Fatalf("tls.server_name = %v, want sni.example.com", got)
	}
	if got := tls["insecure"]; got != true {
		t.Fatalf("tls.insecure = %v, want true", got)
	}
}

func TestClashHysteriaV1BuildsHysteriaNotHysteria2(t *testing.T) {
	src := `
name: hy1
type: hysteria
server: hy.example.com
port: 443
auth-str: secret
sni: sni.example.com
skip-cert-verify: true
up: 50 Mbps
down: 100 Mbps
obfs: obfs-token
`
	out, err := BuildSingBoxOutbound(src)
	if err != nil {
		t.Fatalf("BuildSingBoxOutbound returned error: %v", err)
	}
	if got := out["type"]; got != "hysteria" {
		t.Fatalf("type = %v, want hysteria", got)
	}
	if _, ok := out["password"]; ok {
		t.Fatalf("hysteria v1 Clash YAML must not use hysteria2 password field: %#v", out)
	}
	if got := out["auth_str"]; got != "secret" {
		t.Fatalf("auth_str = %v, want secret", got)
	}
}

func TestShadowsocksPluginRoutesToMihomoOnly(t *testing.T) {
	src := `
name: ss-obfs
type: ss
server: ss.example.com
port: 8388
cipher: aes-128-gcm
password: secret
plugin: obfs
plugin-opts:
  mode: tls
  host: cdn.example.com
`
	if !IsMihomoOnlyProtocol(src) {
		t.Fatalf("expected SS plugin Clash node to be mihomo-only")
	}
	kernels := SupportedKernelsForProtocol(DetectProxyProtocol(src), src, nil, "")
	if len(kernels) != 1 || kernels[0] != ProxyKernelMihomo {
		t.Fatalf("kernels = %#v, want [mihomo]", kernels)
	}
	if RequiresBridge(src, nil, "") {
		t.Fatalf("SS plugin node must not require Xray bridge")
	}
	ok, msg := ValidateProxyConfig(src, nil, "")
	if !ok {
		t.Fatalf("ValidateProxyConfig rejected SS plugin node: %s", msg)
	}
	if _, _, err := ParseProxyNode(src); err == nil {
		t.Fatalf("ParseProxyNode should reject SS plugin for Xray instead of silently stripping plugin")
	}
}

func TestWireGuardClashNodeIsMihomoOnly(t *testing.T) {
	src := `
name: wg
type: wireguard
server: wg.example.com
port: 51820
ip: 172.16.0.2
private-key: private
public-key: public
`
	if !IsMihomoOnlyProtocol(src) {
		t.Fatalf("expected WireGuard Clash node to be mihomo-only")
	}
	kernels := SupportedKernelsForProtocol(DetectProxyProtocol(src), src, nil, "")
	if len(kernels) != 1 || kernels[0] != ProxyKernelMihomo {
		t.Fatalf("kernels = %#v, want [mihomo]", kernels)
	}
	if RequiresBridge(src, nil, "") {
		t.Fatalf("WireGuard node must not require Xray bridge")
	}
	ok, msg := ValidateProxyConfig(src, nil, "")
	if !ok {
		t.Fatalf("ValidateProxyConfig rejected WireGuard node: %s", msg)
	}
	node, err := buildMihomoNode(src)
	if err != nil {
		t.Fatalf("buildMihomoNode returned error: %v", err)
	}
	if got := node["type"]; got != "wireguard" {
		t.Fatalf("type = %v, want wireguard", got)
	}
}

func TestMihomoOnlyDiagnosticsUseMihomoEngine(t *testing.T) {
	ssPlugin := `
name: ss-obfs
type: ss
server: ss.example.com
port: 8388
cipher: aes-128-gcm
password: secret
plugin: obfs
`
	diagnostic := BuildProxyDiagnostic(ssPlugin, nil, "", BuildDiagnosticOptions{ConnectorType: config.BrowserConnectorMihomo})
	if !diagnostic.Ok {
		t.Fatalf("SS plugin diagnostic failed: %+v", diagnostic)
	}
	if diagnostic.Engine != ProxyKernelMihomo {
		t.Fatalf("SS plugin diagnostic engine = %q, want mihomo", diagnostic.Engine)
	}
	if diagnostic.Outbound["type"] != "ss" {
		t.Fatalf("SS plugin diagnostic outbound = %#v", diagnostic.Outbound)
	}

	wireGuard := `
name: wg
type: wireguard
server: wg.example.com
port: 51820
ip: 172.16.0.2
private-key: private
public-key: public
`
	diagnostic = BuildProxyDiagnostic(wireGuard, nil, "", BuildDiagnosticOptions{ConnectorType: config.BrowserConnectorMihomo})
	if !diagnostic.Ok {
		t.Fatalf("WireGuard diagnostic failed: %+v", diagnostic)
	}
	if diagnostic.Engine != ProxyKernelMihomo {
		t.Fatalf("WireGuard diagnostic engine = %q, want mihomo", diagnostic.Engine)
	}
	if diagnostic.Outbound["type"] != "wireguard" {
		t.Fatalf("WireGuard diagnostic outbound = %#v", diagnostic.Outbound)
	}
}
