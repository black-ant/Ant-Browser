package proxy

import "testing"

// 回归:cloudflare trace 的国家码在 loc=CC 字段,parser 必须把它 normalize 到标准
// countryCode 键,否则下游 resolveProxyLocationCountryCode 不认 → 无法按代理匹配定位。
func TestParseIPHealthBodyCloudflareTraceMapsLocToCountryCode(t *testing.T) {
	body := []byte("fl=445f240\nh=www.cloudflare.com\nip=42.51.46.89\nloc=CN\ncolo=LAX\n")
	got, err := parseIPHealthBody(body, "cloudflare_trace")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["ip"] != "42.51.46.89" {
		t.Fatalf("ip: got %q want 42.51.46.89", got["ip"])
	}
	if got["countryCode"] != "CN" {
		t.Fatalf("countryCode: got %q want CN (应从 loc 映射)", got["countryCode"])
	}
}

// 回归:默认测速回退链必须含国内可达目标(miui/baidu),否则中国出口代理测速必失败。
// 回归:ip-api.com 的出口 IP 在 query 字段而非 ip,parser 必须把它提升为 ip 键,
// 否则回退链以 ip 非空判成功 → 地理字段最全的 ip-api 目标永远被误判为失败。
func TestParseIPHealthBodyPromotesIPAPIQueryToIP(t *testing.T) {
	body := []byte(`{"status":"success","country":"China","countryCode":"CN","city":"Suzhou","query":"36.103.196.79"}`)
	got, err := parseIPHealthBody(body, "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["ip"] != "36.103.196.79" {
		t.Fatalf("ip: got %q want 36.103.196.79 (应从 query 提升)", got["ip"])
	}
	if got["countryCode"] != "CN" {
		t.Fatalf("countryCode: got %q want CN", got["countryCode"])
	}
}

func TestDefaultSpeedTestURLChainContainsDomesticTargets(t *testing.T) {
	urls := speedTestTargetURLs(nil)
	wantAny := []string{"http://connect.rom.miui.com/generate_204", "http://www.baidu.com/favicon.ico"}
	for _, want := range wantAny {
		found := false
		for _, u := range urls {
			if u == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("默认测速回退链缺少国内目标 %q: %v", want, urls)
		}
	}
}

// 回归:默认 IP 健康回退链至少含 ip-api(返回标准 countryCode+city),且全部非 my.ippure.com。
func TestDefaultIPHealthChainNoBlockedOverseasOnly(t *testing.T) {
	if len(defaultIPHealthChain) == 0 {
		t.Fatal("默认 IP 健康回退链为空")
	}
	for _, tgt := range defaultIPHealthChain {
		if tgt.URL == "" {
			t.Fatalf("回退链项 URL 为空: %+v", tgt)
		}
		// 不应再依赖被中国出口代理墙掉的海外站
		if tgt.URL == "https://my.ippure.com/v1/info" {
			t.Fatalf("回退链不应包含被墙的 my.ippure.com: %q", tgt.URL)
		}
	}
	// 第一项应是 ip-api(最标准:countryCode+city)
	if defaultIPHealthChain[0].URL != "http://ip-api.com/json" {
		t.Fatalf("回退链首项应为 ip-api,实际 %q", defaultIPHealthChain[0].URL)
	}
}
