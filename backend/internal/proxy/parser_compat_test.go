package proxy

import "testing"

func TestClashVlessTransportKeepsTLSFingerprintAndHeaders(t *testing.T) {
	src := `
name: vless-ws
type: vless
server: edge.example.com
port: 443
uuid: 00000000-0000-0000-0000-000000000001
tls: true
sni: sni.example.com
skip-cert-verify: true
client-fingerprint: chrome
alpn:
  - h2
network: ws
ws-opts:
  path: /ray
  headers:
    Host: cdn.example.com
    User-Agent: ant
`

	_, outbound, err := ParseProxyNode(src)
	if err != nil {
		t.Fatalf("ParseProxyNode returned error: %v", err)
	}
	stream := outbound["streamSettings"].(map[string]interface{})
	tlsSettings := stream["tlsSettings"].(map[string]interface{})
	if _, ok := tlsSettings["allowInsecure"]; ok {
		t.Fatalf("tlsSettings must not include deprecated allowInsecure: %#v", tlsSettings)
	}
	if tlsSettings[xrayTLSInsecurePinKey] != true {
		t.Fatalf("tlsSettings missing insecure pin marker: %#v", tlsSettings)
	}
	if tlsSettings["fingerprint"] != "chrome" {
		t.Fatalf("fingerprint = %v, want chrome", tlsSettings["fingerprint"])
	}
	alpn := tlsSettings["alpn"].([]string)
	if len(alpn) != 1 || alpn[0] != "h2" {
		t.Fatalf("alpn = %#v, want [h2]", alpn)
	}
	ws := stream["wsSettings"].(map[string]interface{})
	headers := ws["headers"].(map[string]interface{})
	if headers["Host"] != "cdn.example.com" || headers["User-Agent"] != "ant" {
		t.Fatalf("headers = %#v", headers)
	}
}

func TestClashTrojanSkipCertVerifyUsesXrayPinMarker(t *testing.T) {
	src := `
name: trojan
type: trojan
server: edge.example.com
port: 443
password: pass
sni: sni.example.com
skip-cert-verify: true
`

	_, outbound, err := ParseProxyNode(src)
	if err != nil {
		t.Fatalf("ParseProxyNode returned error: %v", err)
	}
	stream := outbound["streamSettings"].(map[string]interface{})
	tlsSettings := stream["tlsSettings"].(map[string]interface{})
	if _, ok := tlsSettings["allowInsecure"]; ok {
		t.Fatalf("tlsSettings must not include deprecated allowInsecure: %#v", tlsSettings)
	}
	if tlsSettings[xrayTLSInsecurePinKey] != true {
		t.Fatalf("tlsSettings missing insecure pin marker: %#v", tlsSettings)
	}
}

func TestClashVmessGRPCFallbackServiceName(t *testing.T) {
	src := `
name: vmess-grpc
type: vmess
server: edge.example.com
port: 443
uuid: 00000000-0000-0000-0000-000000000002
cipher: auto
tls: true
servername: sni.example.com
network: grpc
serviceName: svc
`

	_, outbound, err := ParseProxyNode(src)
	if err != nil {
		t.Fatalf("ParseProxyNode returned error: %v", err)
	}
	stream := outbound["streamSettings"].(map[string]interface{})
	grpc := stream["grpcSettings"].(map[string]interface{})
	if grpc["serviceName"] != "svc" {
		t.Fatalf("grpc serviceName = %v, want svc", grpc["serviceName"])
	}
}

func TestClashVmessWSDoesNotEnableBrowserMuxByDefault(t *testing.T) {
	src := `
name: vmess-ws
type: vmess
server: edge.example.com
port: 443
uuid: 00000000-0000-0000-0000-000000000004
cipher: auto
network: ws
ws-opts:
  path: /ray
`

	_, outbound, err := ParseProxyNode(src)
	if err != nil {
		t.Fatalf("ParseProxyNode returned error: %v", err)
	}
	if _, ok := outbound["mux"]; ok {
		t.Fatalf("mux should not be enabled by default after stability validation: %#v", outbound["mux"])
	}
}

func TestClashVmessWSEnablesBrowserMuxWhenExplicit(t *testing.T) {
	src := `
name: vmess-ws
type: vmess
server: edge.example.com
port: 443
uuid: 00000000-0000-0000-0000-000000000004
cipher: auto
network: ws
browser-mux: true
ws-opts:
  path: /ray
`

	_, outbound, err := ParseProxyNode(src)
	if err != nil {
		t.Fatalf("ParseProxyNode returned error: %v", err)
	}
	mux := outbound["mux"].(map[string]interface{})
	if mux["enabled"] != true {
		t.Fatalf("mux.enabled = %v, want true", mux["enabled"])
	}
	if mux["concurrency"] != defaultXrayMuxConcurrency {
		t.Fatalf("mux.concurrency = %v, want %d", mux["concurrency"], defaultXrayMuxConcurrency)
	}
}

func TestClashVlessWSEnablesBrowserMuxWhenExplicit(t *testing.T) {
	src := `
name: vless-ws
type: vless
server: edge.example.com
port: 443
uuid: 00000000-0000-0000-0000-000000000005
network: ws
browser-mux: true
`

	_, outbound, err := ParseProxyNode(src)
	if err != nil {
		t.Fatalf("ParseProxyNode returned error: %v", err)
	}
	if outbound["mux"] == nil {
		t.Fatalf("expected mux for vless ws outbound")
	}
}

func TestClashVmessGRPCDoesNotEnableBrowserMux(t *testing.T) {
	src := `
name: vmess-grpc
type: vmess
server: edge.example.com
port: 443
uuid: 00000000-0000-0000-0000-000000000006
cipher: auto
network: grpc
serviceName: svc
`

	_, outbound, err := ParseProxyNode(src)
	if err != nil {
		t.Fatalf("ParseProxyNode returned error: %v", err)
	}
	if _, ok := outbound["mux"]; ok {
		t.Fatalf("grpc outbound must not enable browser mux by default: %#v", outbound["mux"])
	}
}

func TestClashVmessWSAllowsExplicitMuxDisable(t *testing.T) {
	src := `
name: vmess-ws
type: vmess
server: edge.example.com
port: 443
uuid: 00000000-0000-0000-0000-000000000007
cipher: auto
network: ws
browser-mux: false
`

	_, outbound, err := ParseProxyNode(src)
	if err != nil {
		t.Fatalf("ParseProxyNode returned error: %v", err)
	}
	if _, ok := outbound["mux"]; ok {
		t.Fatalf("explicit mux disable should be respected: %#v", outbound["mux"])
	}
}

func TestClashVmessWSAllowsNestedMuxDisable(t *testing.T) {
	src := `
name: vmess-ws
type: vmess
server: edge.example.com
port: 443
uuid: 00000000-0000-0000-0000-000000000008
cipher: auto
network: ws
mux:
  enabled: false
`

	_, outbound, err := ParseProxyNode(src)
	if err != nil {
		t.Fatalf("ParseProxyNode returned error: %v", err)
	}
	if _, ok := outbound["mux"]; ok {
		t.Fatalf("nested mux disable should be respected: %#v", outbound["mux"])
	}
}

func TestSingBoxHysteria2ClashKeepsTLSFingerprintAndCongestion(t *testing.T) {
	src := `
type: hysteria2
server: hy.example.com
port: 443
ports: 20000-50000, 51000-52000
password: test-password
sni: sni.example.com
alpn:
  - h3
client-fingerprint: chrome
congestion-control: bbr
`

	out, err := BuildSingBoxOutbound(src)
	if err != nil {
		t.Fatalf("BuildSingBoxOutbound returned error: %v", err)
	}
	if out["congestion_control"] != "bbr" {
		t.Fatalf("congestion_control = %v, want bbr", out["congestion_control"])
	}
	serverPorts, ok := out["server_ports"].(string)
	if !ok {
		t.Fatalf("server_ports is %T, want string", out["server_ports"])
	}
	if serverPorts != "20000:50000,51000:52000" {
		t.Fatalf("server_ports = %#v, want converted ranges", serverPorts)
	}
	tls := out["tls"].(map[string]interface{})
	alpn := tls["alpn"].([]string)
	if len(alpn) != 1 || alpn[0] != "h3" {
		t.Fatalf("tls.alpn = %#v, want [h3]", alpn)
	}
	utls := tls["utls"].(map[string]interface{})
	if utls["fingerprint"] != "chrome" {
		t.Fatalf("utls fingerprint = %v, want chrome", utls["fingerprint"])
	}
}

func TestSingBoxTUICClashKeepsServernameFingerprintAndCongestion(t *testing.T) {
	src := `
type: tuic
server: tuic.example.com
port: 443
uuid: 00000000-0000-0000-0000-000000000003
password: test-password
servername: sni.example.com
client-fingerprint: chrome
congestion-controller: cubic
`

	out, err := BuildSingBoxOutbound(src)
	if err != nil {
		t.Fatalf("BuildSingBoxOutbound returned error: %v", err)
	}
	if out["congestion_control"] != "cubic" {
		t.Fatalf("congestion_control = %v, want cubic", out["congestion_control"])
	}
	tls := out["tls"].(map[string]interface{})
	if tls["server_name"] != "sni.example.com" {
		t.Fatalf("tls.server_name = %v, want sni.example.com", tls["server_name"])
	}
	utls := tls["utls"].(map[string]interface{})
	if utls["fingerprint"] != "chrome" {
		t.Fatalf("utls fingerprint = %v, want chrome", utls["fingerprint"])
	}
}

func TestClashSSBuildsXrayServersArray(t *testing.T) {
	src := `
name: ss-node
type: ss
server: ss.example.com
port: 8388
cipher: aes-128-gcm
password: test-password
`

	_, outbound, err := ParseProxyNode(src)
	if err != nil {
		t.Fatalf("ParseProxyNode returned error: %v", err)
	}
	if outbound["protocol"] != "shadowsocks" {
		t.Fatalf("protocol = %v, want shadowsocks", outbound["protocol"])
	}
	settings := outbound["settings"].(map[string]interface{})
	servers, ok := settings["servers"].([]interface{})
	if !ok || len(servers) != 1 {
		t.Fatalf("servers invalid: %#v", settings["servers"])
	}
	server := servers[0].(map[string]interface{})
	if server["address"] != "ss.example.com" || server["port"] != 8388 || server["method"] != "aes-128-gcm" || server["password"] != "test-password" {
		t.Fatalf("server invalid: %#v", server)
	}
	if _, ok := settings["address"]; ok {
		t.Fatalf("legacy flat shadowsocks settings should not be present: %#v", settings)
	}
}

func TestSSURIBuildsXrayServersArray(t *testing.T) {
	_, outbound, err := ParseProxyNode("ss://YWVzLTEyOC1nY206cGFzc3dvcmQ@example.com:8388")
	if err != nil {
		t.Fatalf("ParseProxyNode returned error: %v", err)
	}
	settings := outbound["settings"].(map[string]interface{})
	servers, ok := settings["servers"].([]interface{})
	if !ok || len(servers) != 1 {
		t.Fatalf("servers invalid: %#v", settings["servers"])
	}
	server := servers[0].(map[string]interface{})
	if server["address"] != "example.com" || server["port"] != 8388 || server["method"] != "aes-128-gcm" || server["password"] != "password" {
		t.Fatalf("server invalid: %#v", server)
	}
}
