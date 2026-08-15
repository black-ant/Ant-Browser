package identity

import "testing"

func findOverride(ovs []CDPOverride, method string) *CDPOverride {
	for i := range ovs {
		if ovs[i].Method == method {
			return &ovs[i]
		}
	}
	return nil
}

// 有地理坐标时应产出 setGeolocationOverride,含经纬度与精度。
func TestCDPOverridesIncludeGeolocationWhenSet(t *testing.T) {
	ovs := sampleIdentity().CDPOverrides()
	geo := findOverride(ovs, "Emulation.setGeolocationOverride")
	if geo == nil {
		t.Fatal("expected a geolocation override")
	}
	if geo.Params["latitude"] != 40.7128 || geo.Params["longitude"] != -74.0060 {
		t.Fatalf("unexpected geo params: %+v", geo.Params)
	}
}

// 无地理坐标时不应产出 geolocation 覆盖(避免注入 0,0)。
func TestCDPOverridesOmitGeolocationWhenZero(t *testing.T) {
	id := sampleIdentity()
	id.Geo = Geo{}
	if findOverride(id.CDPOverrides(), "Emulation.setGeolocationOverride") != nil {
		t.Fatal("should not emit geolocation override when coords are zero")
	}
}

// 应产出时区覆盖(CDP 兜底,与 --timezone 双保险)。
func TestCDPOverridesIncludeTimezone(t *testing.T) {
	tz := findOverride(sampleIdentity().CDPOverrides(), "Emulation.setTimezoneOverride")
	if tz == nil || tz.Params["timezoneId"] != "America/New_York" {
		t.Fatalf("expected timezone override America/New_York, got %+v", tz)
	}
}

// 应产出 locale 覆盖。
func TestCDPOverridesIncludeLocale(t *testing.T) {
	loc := findOverride(sampleIdentity().CDPOverrides(), "Emulation.setLocaleOverride")
	if loc == nil || loc.Params["locale"] != "en-US" {
		t.Fatalf("expected locale override en-US, got %+v", loc)
	}
}
