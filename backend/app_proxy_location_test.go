package backend

import "testing"

func TestBuildProxyLocationResolveResultUsesRawCountryCode(t *testing.T) {
	health := ProxyIPHealthResult{
		ProxyId: "proxy-1",
		Ok:      true,
		Country: "Hong Kong SAR China",
		City:    "Hong Kong",
		RawData: map[string]interface{}{
			"countryCode": "HK",
		},
	}

	result := buildProxyLocationResolveResult("proxy-1", health, "ip_health", "2026-07-20T00:00:00Z")
	if !result.Ok {
		t.Fatalf("result.Ok = false, want true: %+v", result)
	}
	if result.Lang != "zh-HK" {
		t.Fatalf("result.Lang = %q, want %q", result.Lang, "zh-HK")
	}
	if result.Timezone != "Asia/Hong_Kong" {
		t.Fatalf("result.Timezone = %q, want %q", result.Timezone, "Asia/Hong_Kong")
	}
	if result.Error != "" {
		t.Fatalf("result.Error = %q, want empty", result.Error)
	}
}
