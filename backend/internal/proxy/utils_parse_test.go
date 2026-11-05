package proxy

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestGetMapStringHandlesNilAndPreservesValues(t *testing.T) {
	values := map[string]interface{}{
		"nil":       nil,
		"name":      " test ",
		"port":      443,
		"enabled":   true,
		"nullText":  "null",
		"nilText":   "nil",
		"angleText": "<nil>",
	}

	tests := map[string]string{
		"nil":       "",
		"name":      "test",
		"port":      "443",
		"enabled":   "true",
		"nullText":  "null",
		"nilText":   "nil",
		"angleText": "<nil>",
	}
	for key, want := range tests {
		if got := getMapString(values, key); got != want {
			t.Errorf("getMapString(%q) = %q, want %q", key, got, want)
		}
	}
}

func TestBuildOutboundFromClashVlessOmitsNullRealityShortID(t *testing.T) {
	node := map[string]interface{}{
		"type":               "vless",
		"name":               "test-reality-node",
		"server":             "node.example.com",
		"port":               443,
		"uuid":               "11111111-2222-3333-4444-555555555555",
		"flow":               "xtls-rprx-vision",
		"network":            "tcp",
		"sni":                "example.com",
		"client-fingerprint": "chrome",
		"reality-opts": map[string]interface{}{
			"public-key": "test-public-key",
			"short-id":   nil,
		},
	}

	outbound, _, err := buildOutboundFromClashVless(node)
	if err != nil {
		t.Fatalf("buildOutboundFromClashVless returned error: %v", err)
	}
	streamSettings, ok := outbound["streamSettings"].(map[string]interface{})
	if !ok {
		t.Fatalf("streamSettings is %T, want map[string]interface{}", outbound["streamSettings"])
	}
	if got := streamSettings["security"]; got != "reality" {
		t.Fatalf("streamSettings.security = %v, want reality", got)
	}
	realitySettings, ok := streamSettings["realitySettings"].(map[string]interface{})
	if !ok {
		t.Fatalf("realitySettings is %T, want map[string]interface{}", streamSettings["realitySettings"])
	}
	if _, ok := realitySettings["shortId"]; ok {
		t.Fatalf("shortId should be omitted for a null Clash value: %+v", realitySettings)
	}
	if got := realitySettings["publicKey"]; got != "test-public-key" {
		t.Fatalf("publicKey = %v, want test-public-key", got)
	}
	encoded, err := json.Marshal(outbound)
	if err != nil {
		t.Fatalf("marshal outbound failed: %v", err)
	}
	if strings.Contains(string(encoded), "<nil>") {
		t.Fatalf("outbound contains invalid <nil> value: %s", encoded)
	}
}
