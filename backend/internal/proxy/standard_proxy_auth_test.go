package proxy

import "testing"

func TestProxyURLHasCredentials(t *testing.T) {
	cases := map[string]bool{
		"http://user:pass@host:1080":   true,
		"socks5://user:pass@host:1080": true,
		"https://user:pass@host:443":   true,
		"http://host:1080":             false,
		"socks5://host:1080":           false,
		"direct://":                    false,
	}
	for in, want := range cases {
		if got := proxyURLHasCredentials(in); got != want {
			t.Errorf("proxyURLHasCredentials(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestRequiresBridgeForAuthedStandardProxy(t *testing.T) {
	// 带账号密码 → 需桥接（Chrome 无法鉴权）
	for _, src := range []string{"http://u:p@h:1080", "socks5://u:p@h:1080"} {
		if !RequiresBridge(src, nil, "") {
			t.Errorf("RequiresBridge(%q) = false, want true", src)
		}
	}
	// 无凭据 → 直连，不桥接
	for _, src := range []string{"http://h:1080", "socks5://h:1080"} {
		if RequiresBridge(src, nil, "") {
			t.Errorf("RequiresBridge(%q) = true, want false", src)
		}
	}
}

func TestParseProxyNodeAuthedStandardProducesOutbound(t *testing.T) {
	// 无凭据：返回标准字符串、不构造出站
	std, ob, err := ParseProxyNode("socks5://h:1080")
	if err != nil || std != "socks5://h:1080" || ob != nil {
		t.Fatalf("no-auth socks5: std=%q ob=%v err=%v", std, ob, err)
	}

	// 带凭据：构造 xray socks 出站，含 users
	std, ob, err = ParseProxyNode("socks5://user:secret@h:1080")
	if err != nil {
		t.Fatalf("authed socks5 parse error: %v", err)
	}
	if std != "" || ob == nil {
		t.Fatalf("authed socks5 should produce outbound, got std=%q ob=%v", std, ob)
	}
	if ob["protocol"] != "socks" || ob["tag"] != "proxy-out" {
		t.Errorf("outbound protocol/tag wrong: %v", ob)
	}
	settings, _ := ob["settings"].(map[string]interface{})
	servers, _ := settings["servers"].([]interface{})
	if len(servers) != 1 {
		t.Fatalf("expected 1 server, got %v", servers)
	}
	server, _ := servers[0].(map[string]interface{})
	if server["address"] != "h" || server["port"] != 1080 {
		t.Errorf("server addr/port wrong: %v", server)
	}
	users, _ := server["users"].([]interface{})
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %v", users)
	}
	u, _ := users[0].(map[string]interface{})
	if u["user"] != "user" || u["pass"] != "secret" {
		t.Errorf("user creds wrong: %v", u)
	}

	// http 带凭据 → http 出站
	_, ob, err = ParseProxyNode("http://user:secret@h:8080")
	if err != nil || ob == nil || ob["protocol"] != "http" {
		t.Errorf("authed http should produce http outbound: ob=%v err=%v", ob, err)
	}
}
