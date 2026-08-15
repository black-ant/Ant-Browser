package identity

import "testing"

func hasIssueField(res ValidationResult, field string) bool {
	for _, is := range res.Issues {
		if is.Field == field {
			return true
		}
	}
	return false
}

// 自洽身份应通过校验。
func TestValidateAcceptsCoherentIdentity(t *testing.T) {
	res := Validate(sampleIdentity())
	if !res.OK {
		t.Fatalf("expected coherent identity to pass, issues=%+v", res.Issues)
	}
}

// 平台与 UA 不一致应判为错误(硬拦截)。
func TestValidateFlagsPlatformUAMismatch(t *testing.T) {
	id := sampleIdentity()
	id.Platform = "macos" // UA 仍是 Windows
	res := Validate(id)
	if res.OK {
		t.Fatal("expected platform/UA mismatch to fail")
	}
	if !hasIssueField(res, "ua") {
		t.Errorf("expected an issue on the ua field, got %+v", res.Issues)
	}
}

// 缺 seed 应判为错误。
func TestValidateFlagsMissingSeed(t *testing.T) {
	id := sampleIdentity()
	id.Seed = 0
	if Validate(id).OK {
		t.Fatal("expected missing seed to fail")
	}
}

// 绑定了代理国家却没有对齐时区,应判为错误。
func TestValidateFlagsTimezoneNotAlignedToProxyCountry(t *testing.T) {
	id := sampleIdentity()
	id.ProxyGeoSnapshot = "DE"
	id.Timezone = ""
	res := Validate(id)
	if res.OK {
		t.Fatal("expected missing timezone with a bound proxy country to fail")
	}
	if !hasIssueField(res, "timezone") {
		t.Errorf("expected an issue on the timezone field, got %+v", res.Issues)
	}
}

// 屏幕尺寸非法应判为错误。
func TestValidateFlagsInvalidScreen(t *testing.T) {
	id := sampleIdentity()
	id.Screen.Width = 0
	if Validate(id).OK {
		t.Fatal("expected invalid screen to fail")
	}
}
