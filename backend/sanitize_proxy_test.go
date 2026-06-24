package backend

import (
	"testing"
)

func TestSanitizeProxyForLog(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty",
			input:    "",
			expected: "",
		},
		{
			name:     "direct",
			input:    "direct://",
			expected: "direct://",
		},
		{
			name:     "socks5 with credentials",
			input:    "socks5://user:pass@192.168.1.1:1080",
			expected: "socks5://***:***@192.168.1.1:1080",
		},
		{
			name:     "http with credentials",
			input:    "http://admin:secret123@proxy.example.com:8080",
			expected: "http://***:***@proxy.example.com:8080",
		},
		{
			name:     "https with credentials and path",
			input:    "https://user:pass@proxy.com:443/path",
			expected: "https://***:***@proxy.com:443/path",
		},
		{
			name:     "http with credentials and sensitive query",
			input:    "http://user:pass@proxy.com:8080?token=secret123&password=admin",
			expected: "http://***:***@proxy.com:8080?***",
		},
		{
			name:     "socks5 with query containing obfs-password",
			input:    "socks5://proxy.com:1080?plugin=obfs&obfs-password=secret",
			expected: "socks5://proxy.com:1080?***",
		},
		{
			name:     "http with query containing secret",
			input:    "http://proxy.com:8080/path?secret=mysecret&api_key=12345",
			expected: "http://proxy.com:8080/path?***",
		},
		{
			name:     "socks5 without credentials",
			input:    "socks5://192.168.1.1:1080",
			expected: "socks5://192.168.1.1:1080",
		},
		{
			name:     "vmess protocol",
			input:    "vmess://base64encodedconfigwithsecrets",
			expected: "vmess://***",
		},
		{
			name:     "vless protocol",
			input:    "vless://uuid@server:port?encryption=none&security=tls",
			expected: "vless://***",
		},
		{
			name:     "trojan protocol",
			input:    "trojan://password@server:port",
			expected: "trojan://***",
		},
		{
			name:     "ss protocol",
			input:    "ss://base64method:password@server:port",
			expected: "ss://***",
		},
		{
			name:     "hysteria2 with token",
			input:    "hysteria2://token@server.com:443?upmbps=100&downmbps=200",
			expected: "hysteria2://***@server.com:443",
		},
		{
			name:     "tuic with password",
			input:    "tuic://uuid:password@server.com:8443",
			expected: "tuic://***@server.com:8443",
		},
		{
			name:     "malformed url",
			input:    "not-a-valid-url",
			expected: "***",
		},
		{
			name:     "protocol only malformed",
			input:    "http://",
			expected: "http://***",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeProxyForLog(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeProxyForLog(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSanitizeLaunchArgs(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "empty",
			input:    []string{},
			expected: []string{},
		},
		{
			name: "no proxy arg",
			input: []string{
				"--user-data-dir=/path",
				"--remote-debugging-port=9222",
			},
			expected: []string{
				"--user-data-dir=/path",
				"--remote-debugging-port=9222",
			},
		},
		{
			name: "proxy server with credentials",
			input: []string{
				"--user-data-dir=/path",
				"--proxy-server=socks5://user:pass@192.168.1.1:1080",
				"--remote-debugging-port=9222",
			},
			expected: []string{
				"--user-data-dir=/path",
				"--proxy-server=socks5://***:***@192.168.1.1:1080",
				"--remote-debugging-port=9222",
			},
		},
		{
			name: "proxy server without credentials",
			input: []string{
				"--proxy-server=socks5://192.168.1.1:1080",
			},
			expected: []string{
				"--proxy-server=socks5://192.168.1.1:1080",
			},
		},
		{
			name: "proxy server direct",
			input: []string{
				"--proxy-server=direct://",
			},
			expected: []string{
				"--proxy-server=direct://",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeLaunchArgs(tt.input)
			if len(result) != len(tt.expected) {
				t.Fatalf("sanitizeLaunchArgs() returned %d items, want %d", len(result), len(tt.expected))
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("sanitizeLaunchArgs()[%d] = %q, want %q", i, result[i], tt.expected[i])
				}
			}
		})
	}
}

func TestSanitizeProxyConfigField(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "nil",
			input:    nil,
			expected: "",
		},
		{
			name:     "string with credentials",
			input:    "socks5://user:pass@host:1080",
			expected: "socks5://***:***@host:1080",
		},
		{
			name:     "non-string value",
			input:    12345,
			expected: "12345",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeProxyConfigField(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeProxyConfigField(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
