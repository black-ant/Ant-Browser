package identity

import "testing"

// 代理地理应覆盖身份的时区/语言/locale/坐标,同时保留平台/UA/硬件等非地理字段。
func TestAlignToProxyGeoOverridesTimezoneAndLocale(t *testing.T) {
	id := sampleIdentity() // windows / America/New_York / en-US
	geo := GeoInfo{CountryCode: "DE", City: "Berlin", Latitude: 52.52, Longitude: 13.405, Timezone: "Europe/Berlin", Accuracy: 50}

	out := AlignToProxyGeo(id, geo)

	if out.Timezone != "Europe/Berlin" {
		t.Errorf("timezone not aligned: %q", out.Timezone)
	}
	if out.Locale != "de-DE" {
		t.Errorf("locale not aligned: %q", out.Locale)
	}
	if out.Geo.Latitude != 52.52 || out.Geo.Longitude != 13.405 {
		t.Errorf("geo coords not set: %+v", out.Geo)
	}
	if out.Platform != id.Platform || out.UAFull != id.UAFull || out.HardwareConcurrency != id.HardwareConcurrency {
		t.Error("non-geo identity fields must be preserved")
	}
}

// GeoIP 缺时区时,应回退到国家→时区兜底表。
func TestAlignFallsBackToCountryTableWhenTimezoneMissing(t *testing.T) {
	id := sampleIdentity()
	geo := GeoInfo{CountryCode: "JP", Latitude: 35.68, Longitude: 139.69} // 无 Timezone

	out := AlignToProxyGeo(id, geo)

	if out.Timezone != "Asia/Tokyo" {
		t.Errorf("expected country-table tz Asia/Tokyo, got %q", out.Timezone)
	}
	if out.Locale != "ja-JP" {
		t.Errorf("expected locale ja-JP, got %q", out.Locale)
	}
}

// 直连(无代理)按本地国家对齐:时区/语言/locale 应变为该国默认,
// 但设备指纹字段(seed / 平台 / UA / 硬件)必须保持不变,地理坐标不改。
func TestAlignToCountryOverridesLocaleKeepsDevice(t *testing.T) {
	id := sampleIdentity() // windows / America/New_York / en-US
	id.Seed = 1326051714
	id.Geo = Geo{Latitude: 40.71, Longitude: -74.0, Accuracy: 50}

	out := AlignToCountry(id, "CN")

	if out.Timezone != "Asia/Shanghai" {
		t.Errorf("timezone not aligned to CN: %q", out.Timezone)
	}
	if out.Locale != "zh-CN" {
		t.Errorf("locale not aligned to CN: %q", out.Locale)
	}
	if len(out.Languages) == 0 || out.Languages[0] != "zh-CN" {
		t.Errorf("languages not aligned to CN: %v", out.Languages)
	}
	// 设备指纹稳定:seed / 平台 / UA / 硬件不变。
	if out.Seed != id.Seed || out.Platform != id.Platform || out.UAFull != id.UAFull || out.HardwareConcurrency != id.HardwareConcurrency {
		t.Error("device fingerprint fields must be preserved on country align")
	}
	// 直连不伪造精确定位:坐标保持不变。
	if out.Geo.Latitude != id.Geo.Latitude || out.Geo.Longitude != id.Geo.Longitude {
		t.Errorf("country align must not change geo coords: %+v", out.Geo)
	}
	// 序列化后的 flag 应反映中文环境。
	args := out.LaunchArgs()
	if !hasArg(args, "--timezone=Asia/Shanghai") {
		t.Errorf("timezone flag not aligned: %v", args)
	}
	if !hasArgPrefix(args, "--accept-lang=zh-CN") {
		t.Errorf("accept-lang not aligned to zh-CN: %v", args)
	}
}

// 未收录的国家码应原样返回,不破坏已有自洽。
func TestAlignToCountryUnknownIsNoop(t *testing.T) {
	id := sampleIdentity()
	out := AlignToCountry(id, "ZZ")
	if out.Timezone != id.Timezone || out.Locale != id.Locale {
		t.Errorf("unknown country must be no-op, got tz=%q locale=%q", out.Timezone, out.Locale)
	}
}

// 对齐后序列化的 flag 中,时区与 accept-language 应与代理国家一致。
func TestAlignReflectedInLaunchArgs(t *testing.T) {
	id := sampleIdentity()
	geo := GeoInfo{CountryCode: "FR", Timezone: "Europe/Paris", Latitude: 48.85, Longitude: 2.35}

	args := AlignToProxyGeo(id, geo).LaunchArgs()

	if !hasArg(args, "--timezone=Europe/Paris") {
		t.Errorf("timezone flag not aligned: %v", args)
	}
	if !hasArgPrefix(args, "--accept-lang=fr-FR") {
		t.Errorf("accept-lang not aligned: %v", args)
	}
}
