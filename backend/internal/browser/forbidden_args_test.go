package browser

import "testing"

func TestAssertNoForbiddenArgsCatchesPrefixes(t *testing.T) {
	bad := AssertNoForbiddenArgs([]string{"--process-per-site", "--js-flags=--max-old-space-size=512"})
	if len(bad) != 1 || bad[0] != "--js-flags=--max-old-space-size=512" {
		t.Fatalf("expected js-flags flagged, got %v", bad)
	}
}

func TestAssertNoForbiddenArgsCatchesFeatureNames(t *testing.T) {
	bad := AssertNoForbiddenArgs([]string{"--disable-features=Translate,WebBluetooth"})
	if len(bad) != 1 {
		t.Fatalf("expected WebBluetooth flagged, got %v", bad)
	}
}

func TestAssertNoForbiddenArgsAllowsCleanSet(t *testing.T) {
	if bad := AssertNoForbiddenArgs([]string{"--process-per-site", "--disable-features=Translate,MediaRouter"}); len(bad) != 0 {
		t.Fatalf("clean set should pass, got %v", bad)
	}
}

// 每一条红线都要有覆盖:一类漏了,以后就会有人加进来。
func TestAssertNoForbiddenArgsCoversEachCategory(t *testing.T) {
	cases := map[string]string{
		"指纹面":            "--fingerprint-platform=windows",
		"GPU管线":          "--disable-gpu-compositing",
		"JS可见API":        "--disable-notifications",
		"插件面":            "--disable-plugins",
		"网络指纹":           "--disable-quic",
		"字体":             "--disable-remote-fonts",
		"播放行为面":          "--autoplay-policy=no-user-gesture-required",
		"打断CDP":          "--single-process",
		"自动化标识":          "--enable-automation",
		"安全":             "--no-sandbox",
		"codec特性":        "--disable-features=PlatformHEVCDecoderSupport",
		"PrivacySandbox": "--disable-features=BrowsingTopics",
	}
	for name, arg := range cases {
		if bad := AssertNoForbiddenArgs([]string{arg}); len(bad) == 0 {
			t.Errorf("红线类别 %s 未被拦截: %s", name, arg)
		}
	}
}

// 必须保留的开关不能被误伤 —— 它们是看播时长与直播可播性的命脉。
func TestAssertNoForbiddenArgsDoesNotFlagRequiredSwitches(t *testing.T) {
	required := []string{
		"--disable-backgrounding-occluded-windows",
		"--disable-renderer-backgrounding",
		"--disable-background-timer-throttling",
		"--disable-features=CalculateNativeWinOcclusion",
		"--mute-audio",
		"--disable-non-proxied-udp",
	}
	if bad := AssertNoForbiddenArgs(required); len(bad) != 0 {
		t.Fatalf("必须保留的开关被误伤: %v", bad)
	}
}
