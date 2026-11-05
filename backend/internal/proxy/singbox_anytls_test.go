package proxy

import "testing"

func TestBuildSingBoxAnyTLSFromClash(t *testing.T) {
	src := `
proxies:
  - name: anytls-main
    type: anytls
    server: anytls.example.com
    port: 443
    password: test-password
    sni: sni.example.com
    servername: fallback.example.com
    skip-cert-verify: true
    alpn:
      - h2
      - http/1.1
    client-fingerprint: chrome
    idle-session-check-interval: 30
    idle-session-timeout: 45
    min-idle-session: 5
`

	if !IsSingBoxProtocol(src) {
		t.Fatalf("expected anytls Clash YAML to be treated as sing-box protocol")
	}
	if RequiresBridge(src, nil, "") {
		t.Fatalf("anytls Clash YAML must not require Xray bridge")
	}

	out, err := BuildSingBoxOutbound(src)
	if err != nil {
		t.Fatalf("BuildSingBoxOutbound returned error: %v", err)
	}
	if got := out["type"]; got != "anytls" {
		t.Fatalf("type = %v, want anytls", got)
	}
	if got := out["tag"]; got != "proxy-out" {
		t.Fatalf("tag = %v, want proxy-out", got)
	}
	if got := out["server"]; got != "anytls.example.com" {
		t.Fatalf("server = %v, want anytls.example.com", got)
	}
	if got := out["server_port"]; got != 443 {
		t.Fatalf("server_port = %v, want 443", got)
	}
	if got := out["password"]; got != "test-password" {
		t.Fatalf("password = %v, want test-password", got)
	}
	if got := out["idle_session_check_interval"]; got != "30s" {
		t.Fatalf("idle_session_check_interval = %v, want 30s", got)
	}
	if got := out["idle_session_timeout"]; got != "45s" {
		t.Fatalf("idle_session_timeout = %v, want 45s", got)
	}
	if got := out["min_idle_session"]; got != 5 {
		t.Fatalf("min_idle_session = %v, want 5", got)
	}

	tls, ok := out["tls"].(map[string]interface{})
	if !ok {
		t.Fatalf("tls is %T, want map[string]interface{}", out["tls"])
	}
	if got := tls["enabled"]; got != true {
		t.Fatalf("tls.enabled = %v, want true", got)
	}
	if got := tls["insecure"]; got != true {
		t.Fatalf("tls.insecure = %v, want true", got)
	}
	if got := tls["server_name"]; got != "sni.example.com" {
		t.Fatalf("tls.server_name = %v, want sni.example.com", got)
	}
	alpn, ok := tls["alpn"].([]string)
	if !ok {
		t.Fatalf("tls.alpn is %T, want []string", tls["alpn"])
	}
	if len(alpn) != 2 || alpn[0] != "h2" || alpn[1] != "http/1.1" {
		t.Fatalf("tls.alpn = %#v, want [h2 http/1.1]", alpn)
	}
	utls, ok := tls["utls"].(map[string]interface{})
	if !ok {
		t.Fatalf("tls.utls is %T, want map[string]interface{}", tls["utls"])
	}
	if got := utls["enabled"]; got != true {
		t.Fatalf("tls.utls.enabled = %v, want true", got)
	}
	if got := utls["fingerprint"]; got != "chrome" {
		t.Fatalf("tls.utls.fingerprint = %v, want chrome", got)
	}
}

func TestBuildSingBoxAnyTLSServernameFallbackAndDurationStrings(t *testing.T) {
	src := `
type: anytls
server: anytls.example.com
port: 8443
password: test-password
servername: fallback.example.com
idle-session-check-interval: 1m
idle-session-timeout: "30"
min-idle-session: 0
`

	out, err := BuildSingBoxOutbound(src)
	if err != nil {
		t.Fatalf("BuildSingBoxOutbound returned error: %v", err)
	}
	if got := out["idle_session_check_interval"]; got != "1m" {
		t.Fatalf("idle_session_check_interval = %v, want 1m", got)
	}
	if got := out["idle_session_timeout"]; got != "30s" {
		t.Fatalf("idle_session_timeout = %v, want 30s", got)
	}
	if _, ok := out["min_idle_session"]; ok {
		t.Fatalf("min_idle_session should be omitted when Clash value is 0")
	}
	tls, ok := out["tls"].(map[string]interface{})
	if !ok {
		t.Fatalf("tls is %T, want map[string]interface{}", out["tls"])
	}
	if got := tls["server_name"]; got != "fallback.example.com" {
		t.Fatalf("tls.server_name = %v, want fallback.example.com", got)
	}
	if got := tls["insecure"]; got != false {
		t.Fatalf("tls.insecure = %v, want false", got)
	}
	if _, ok := tls["utls"]; ok {
		t.Fatalf("tls.utls should be omitted without client-fingerprint")
	}
}

func TestBuildSingBoxAnyTLSRequiresServerAndPort(t *testing.T) {
	src := `
type: anytls
password: test-password
`

	if _, err := BuildSingBoxOutbound(src); err == nil {
		t.Fatalf("expected missing server and port to fail")
	}
}

func TestBuildSingBoxAnyTLSRequiresPassword(t *testing.T) {
	src := `
type: anytls
server: anytls.example.com
port: 443
`

	if _, err := BuildSingBoxOutbound(src); err == nil {
		t.Fatalf("expected missing password to fail")
	}
}

func TestBuildSingBoxAnyTLSFromURI(t *testing.T) {
	src := "anytls://test-password@anytls.example.com:25535?sni=ai.gitee.com&insecure=1&idle-session-timeout=30#AnyTLS"
	if !IsSingBoxProtocol(src) {
		t.Fatalf("expected anytls URI to be treated as sing-box protocol")
	}
	out, err := BuildSingBoxOutbound(src)
	if err != nil {
		t.Fatalf("BuildSingBoxOutbound returned error: %v", err)
	}
	if got := out["type"]; got != "anytls" {
		t.Fatalf("type = %v, want anytls", got)
	}
	if got := out["server"]; got != "anytls.example.com" {
		t.Fatalf("server = %v, want anytls.example.com", got)
	}
	if got := out["server_port"]; got != 25535 {
		t.Fatalf("server_port = %v, want 25535", got)
	}
	if got := out["password"]; got != "test-password" {
		t.Fatalf("password = %v, want test-password", got)
	}
	if got := out["idle_session_timeout"]; got != "30s" {
		t.Fatalf("idle_session_timeout = %v, want 30s", got)
	}
	tls, ok := out["tls"].(map[string]interface{})
	if !ok {
		t.Fatalf("tls is %T, want map[string]interface{}", out["tls"])
	}
	if got := tls["server_name"]; got != "ai.gitee.com" {
		t.Fatalf("tls.server_name = %v, want ai.gitee.com", got)
	}
	if got := tls["insecure"]; got != true {
		t.Fatalf("tls.insecure = %v, want true", got)
	}
}
