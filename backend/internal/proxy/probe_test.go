package proxy

import "testing"

func TestParseBareProxy(t *testing.T) {
	cases := []struct {
		in       string
		host     string
		port     int
		username string
		password string
		wantErr  bool
	}{
		{"us.proxy001.com:7878:occmcl601376_custom_zone_US:pwd740836", "us.proxy001.com", 7878, "occmcl601376_custom_zone_US", "pwd740836", false},
		{"1.2.3.4:1080", "1.2.3.4", 1080, "", "", false},
		{"user:pass@host.example.com:8080", "host.example.com", 8080, "user", "pass", false},
		{"host.example.com:9000:user", "host.example.com", 9000, "user", "", false},
		{"host:7878:u:p:with:colons", "host", 7878, "u", "p:with:colons", false},
		{"1.2.3.4:1080#备注", "1.2.3.4", 1080, "", "", false},
		{"noport", "", 0, "", "", true},
		{"", "", 0, "", "", true},
	}
	for _, c := range cases {
		h, p, u, pw, err := parseBareProxy(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("%q: expected error, got none", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: unexpected error %v", c.in, err)
			continue
		}
		if h != c.host || p != c.port || u != c.username || pw != c.password {
			t.Errorf("%q: got (%q,%d,%q,%q), want (%q,%d,%q,%q)", c.in, h, p, u, pw, c.host, c.port, c.username, c.password)
		}
	}
}

func TestProbeProxyConfigWithScheme(t *testing.T) {
	r := ProbeProxyConfig("socks5://1.2.3.4:1080", 0)
	if r.Protocol != "socks5" {
		t.Errorf("expected socks5, got %q", r.Protocol)
	}
	r = ProbeProxyConfig("https://1.2.3.4:443", 0)
	if r.Protocol != "http" {
		t.Errorf("expected http for https scheme, got %q", r.Protocol)
	}
}
