package proxy

import "testing"

func TestBuildSingBoxTUICFromURI(t *testing.T) {
	src := "tuic://00000000-0000-0000-0000-000000000001:test-password@tuic.example.com:443?sni=sni.example.com&alpn=h3&congestion_control=bbr&insecure=1#TUIC"

	if !IsSingBoxProtocol(src) {
		t.Fatalf("expected tuic URI to be treated as sing-box protocol")
	}
	if DetectProxyProtocol(src) != "tuic" {
		t.Fatalf("DetectProxyProtocol = %v, want tuic", DetectProxyProtocol(src))
	}

	out, err := BuildSingBoxOutbound(src)
	if err != nil {
		t.Fatalf("BuildSingBoxOutbound returned error: %v", err)
	}
	if got := out["type"]; got != "tuic" {
		t.Fatalf("type = %v, want tuic", got)
	}
	if got := out["tag"]; got != "proxy-out" {
		t.Fatalf("tag = %v, want proxy-out", got)
	}
	if got := out["server"]; got != "tuic.example.com" {
		t.Fatalf("server = %v, want tuic.example.com", got)
	}
	if got := out["server_port"]; got != 443 {
		t.Fatalf("server_port = %v, want 443", got)
	}
	if got := out["uuid"]; got != "00000000-0000-0000-0000-000000000001" {
		t.Fatalf("uuid = %v, want the provided uuid", got)
	}
	if got := out["password"]; got != "test-password" {
		t.Fatalf("password = %v, want test-password", got)
	}
	if got := out["congestion_control"]; got != "bbr" {
		t.Fatalf("congestion_control = %v, want bbr", got)
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
	if !ok || len(alpn) != 1 || alpn[0] != "h3" {
		t.Fatalf("tls.alpn = %#v, want [h3]", tls["alpn"])
	}
}

func TestBuildSingBoxTUICFromURIDefaultsCongestionBBR(t *testing.T) {
	src := "tuic://uuid-xyz:pass@tuic.example.com:8443?sni=a.example.com"
	out, err := BuildSingBoxOutbound(src)
	if err != nil {
		t.Fatalf("BuildSingBoxOutbound returned error: %v", err)
	}
	if got := out["congestion_control"]; got != "bbr" {
		t.Fatalf("congestion_control = %v, want default bbr", got)
	}
}

func TestBuildSingBoxTUICFromURIRequiresServerPortUUID(t *testing.T) {
	if _, err := BuildSingBoxOutbound("tuic://:pass@tuic.example.com:443"); err == nil {
		t.Fatalf("expected missing uuid to fail")
	}
}
