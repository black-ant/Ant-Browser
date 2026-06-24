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
