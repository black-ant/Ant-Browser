package browser

import "testing"

func TestDeriveProxyGeoEmptyOrBad(t *testing.T) {
	t.Parallel()
	if _, ok := DeriveProxyGeo(""); ok {
		t.Fatalf("空 JSON 应返回 ok=false")
	}
	if _, ok := DeriveProxyGeo("not-json"); ok {
		t.Fatalf("坏 JSON 应返回 ok=false")
	}
	if _, ok := DeriveProxyGeo(`{"ok":true,"country":"ZZ","rawData":{}}`); ok {
		t.Fatalf("未知国家且无原始时区应返回 ok=false")
	}
}

func TestDeriveProxyGeoPrefersRawTimezone(t *testing.T) {
	t.Parallel()
	geo, ok := DeriveProxyGeo(`{"ok":true,"country":"US","region":"California","rawData":{"timezone":"America/Chicago"}}`)
	if !ok {
		t.Fatalf("应解析成功")
	}
	if geo.Timezone != "America/Chicago" {
		t.Fatalf("应优先采用原始响应时区: got=%q", geo.Timezone)
	}
	if geo.Language != "en-US" {
		t.Fatalf("US 语言应为 en-US: got=%q", geo.Language)
	}
}

func TestDeriveProxyGeoUSRegionFallback(t *testing.T) {
	t.Parallel()
	geo, ok := DeriveProxyGeo(`{"ok":true,"country":"US","region":"California","rawData":{}}`)
	if !ok {
		t.Fatalf("应解析成功")
	}
	if geo.Timezone != "America/Los_Angeles" {
		t.Fatalf("加州应映射到太平洋时区: got=%q", geo.Timezone)
	}
}

func TestDeriveProxyGeoCountryNameAndCode(t *testing.T) {
	t.Parallel()
	cases := map[string]struct{ tz, lang string }{
		`{"ok":true,"country":"JP","rawData":{}}`:             {"Asia/Tokyo", "ja-JP"},
		`{"ok":true,"country":"Japan","rawData":{}}`:          {"Asia/Tokyo", "ja-JP"},
		`{"ok":true,"country":"GB","rawData":{}}`:             {"Europe/London", "en-GB"},
		`{"ok":true,"country":"United Kingdom","rawData":{}}`: {"Europe/London", "en-GB"},
		`{"ok":true,"country":"DE","rawData":{}}`:             {"Europe/Berlin", "de-DE"},
	}
	for in, want := range cases {
		geo, ok := DeriveProxyGeo(in)
		if !ok {
			t.Fatalf("%s 应解析成功", in)
		}
		if geo.Timezone != want.tz || geo.Language != want.lang {
			t.Fatalf("%s 推导错误: got tz=%q lang=%q, want tz=%q lang=%q", in, geo.Timezone, geo.Language, want.tz, want.lang)
		}
	}
}

func TestDeriveProxyGeoLatLon(t *testing.T) {
	t.Parallel()
	geo, ok := DeriveProxyGeo(`{"ok":true,"country":"US","region":"NY","rawData":{"latitude":40.71,"longitude":-74.0}}`)
	if !ok || !geo.HasLatLon {
		t.Fatalf("应提取经纬度: %+v", geo)
	}
	if geo.Lat != 40.71 || geo.Lon != -74.0 {
		t.Fatalf("经纬度错误: got lat=%v lon=%v", geo.Lat, geo.Lon)
	}

	geo2, ok2 := DeriveProxyGeo(`{"ok":true,"country":"US","region":"NY","rawData":{"loc":"37.75,-97.82"}}`)
	if !ok2 || !geo2.HasLatLon || geo2.Lat != 37.75 || geo2.Lon != -97.82 {
		t.Fatalf("应从 loc 字段提取经纬度: %+v", geo2)
	}
}

func TestUSRegionToTimezoneAddedStates(t *testing.T) {
	t.Parallel()
	// 新补的「单一时区」中部/山区州：缺失时本会错误回落东部，补全后映射正确。
	cases := map[string]string{
		"alabama":     "America/Chicago",
		"al":          "America/Chicago",
		"mississippi": "America/Chicago",
		"arkansas":    "America/Chicago",
		"oklahoma":    "America/Chicago",
		"iowa":        "America/Chicago",
		"wyoming":     "America/Denver",
		"wy":          "America/Denver",
	}
	for region, want := range cases {
		if got := usRegionToTimezone(region); got != want {
			t.Fatalf("usRegionToTimezone(%q)=%q, want %q", region, got, want)
		}
	}
}

func TestUSRegionToTimezoneSplitStatesUnmapped(t *testing.T) {
	t.Parallel()
	// 跨时区州刻意不进映射表：单一 state→zone 猜测必错，交由第三方接口 timezone 字段 / 经纬度决定。
	for _, region := range []string{"tennessee", "kentucky", "indiana", "kansas", "nebraska"} {
		if got := usRegionToTimezone(region); got != "" {
			t.Fatalf("跨时区州 %q 不应映射到单一时区, got=%q", region, got)
		}
	}
}

func TestDeriveProxyGeoUSUnknownRegionDefaultsEastern(t *testing.T) {
	t.Parallel()
	// 无 rawData.timezone、region 为未覆盖（跨时区）州时，回落美国默认东部时区。
	geo, ok := DeriveProxyGeo(`{"ok":true,"country":"US","region":"Tennessee","rawData":{}}`)
	if !ok {
		t.Fatalf("应解析成功")
	}
	if geo.Timezone != "America/New_York" {
		t.Fatalf("未覆盖州应回落东部时区: got=%q", geo.Timezone)
	}
}

func TestDeriveProxyGeoRawTimezoneOverridesRegion(t *testing.T) {
	t.Parallel()
	// 复现本次场景：IP 落地美东，接口直接给出 America/New_York，应直接采用而非依赖州映射。
	geo, ok := DeriveProxyGeo(`{"ok":true,"country":"US","region":"Tennessee","rawData":{"timezone":"America/New_York"}}`)
	if !ok {
		t.Fatalf("应解析成功")
	}
	if geo.Timezone != "America/New_York" {
		t.Fatalf("应采用接口原始时区: got=%q", geo.Timezone)
	}
}
