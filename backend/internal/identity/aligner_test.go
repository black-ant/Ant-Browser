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
