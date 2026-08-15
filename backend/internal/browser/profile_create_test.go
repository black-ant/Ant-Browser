package browser

import (
	"ant-chrome/backend/internal/config"
	"testing"
)

func TestCreateAppliesDefaultFingerprintArgsWhenInputIsEmpty(t *testing.T) {
	manager := NewManager(&config.Config{}, t.TempDir())
	manager.Config.Browser.DefaultFingerprintArgs = []string{
		"--fingerprint-brand=Chrome",
		"--fingerprint-platform=windows",
		"--disable-non-proxied-udp",
		"--fingerprinting-canvas-image-data-noise",
		"--fingerprinting-client-rects-noise",
	}

	profile, err := manager.Create(ProfileInput{ProfileName: "test"})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	assertStringSliceContains(t, profile.FingerprintArgs, "--disable-non-proxied-udp")
	assertStringSliceContains(t, profile.FingerprintArgs, "--fingerprinting-canvas-image-data-noise")
	assertStringSliceContains(t, profile.FingerprintArgs, "--fingerprinting-client-rects-noise")
}

func TestCreateKeepsExplicitFingerprintArgs(t *testing.T) {
	manager := NewManager(&config.Config{}, t.TempDir())
	manager.Config.Browser.DefaultFingerprintArgs = []string{"--fingerprint-brand=Chrome"}

	profile, err := manager.Create(ProfileInput{
		ProfileName:     "test",
		FingerprintArgs: []string{"--fingerprint=123"},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if got, want := len(profile.FingerprintArgs), 1; got != want {
		t.Fatalf("fingerprint args length = %d, want %d: %#v", got, want, profile.FingerprintArgs)
	}
	assertStringSliceContains(t, profile.FingerprintArgs, "--fingerprint=123")
}

func TestCreateNormalizesRestoreLastSessionMode(t *testing.T) {
	manager := NewManager(&config.Config{}, t.TempDir())
	profile, err := manager.Create(ProfileInput{ProfileName: "test", RestoreLastSession: "true"})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if profile.RestoreLastSession != RestoreLastSessionEnabled {
		t.Fatalf("restoreLastSession = %q, want %q", profile.RestoreLastSession, RestoreLastSessionEnabled)
	}
}

// 回归(核心需求):GUI 新建“只填名称”会提交无种子的静态默认模板参数(非空)。
// 此时必须为每个环境生成全新、唯一、自洽的池身份,而不是共用同一套默认指纹。
func TestCreateGeneratesUniqueIdentityForNameOnlyWithStaticDefaults(t *testing.T) {
	manager := NewManager(&config.Config{}, t.TempDir())
	manager.IdentityService = newTestIdentityService(t)
	staticDefaults := []string{"--fingerprint-brand=Chrome", "--fingerprint-platform=mac", "--disable-non-proxied-udp"}

	p1, err := manager.Create(ProfileInput{ProfileName: "a", FingerprintArgs: append([]string{}, staticDefaults...)})
	if err != nil {
		t.Fatalf("create a: %v", err)
	}
	p2, err := manager.Create(ProfileInput{ProfileName: "b", FingerprintArgs: append([]string{}, staticDefaults...)})
	if err != nil {
		t.Fatalf("create b: %v", err)
	}

	seed1, ok1 := argValue(p1.FingerprintArgs, "--fingerprint=")
	if !ok1 || seed1 == "" || seed1 == "0" {
		t.Fatalf("expected fresh non-zero seed for name-only create, got %q in %v", seed1, p1.FingerprintArgs)
	}
	seed2, _ := argValue(p2.FingerprintArgs, "--fingerprint=")
	if seed1 == seed2 {
		t.Fatalf("expected different seeds for two name-only creates, both %q", seed1)
	}
	// 自洽:身份是整套采样的(窗口尺寸等维度齐全),而非仅有静态默认 flag。
	if _, ok := argValue(p1.FingerprintArgs, "--window-size="); !ok {
		t.Fatalf("expected a full pool identity (window-size present), got %v", p1.FingerprintArgs)
	}
}

// 显式指定 --fingerprint=<seed>(Launch API/复现场景)应被尊重,即使自洽引擎在位也不覆盖。
func TestCreateRespectsExplicitSeedWithIdentityService(t *testing.T) {
	manager := NewManager(&config.Config{}, t.TempDir())
	manager.IdentityService = newTestIdentityService(t)

	p, err := manager.Create(ProfileInput{
		ProfileName:     "x",
		FingerprintArgs: []string{"--fingerprint=424242", "--fingerprint-platform=windows"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if seed, _ := argValue(p.FingerprintArgs, "--fingerprint="); seed != "424242" {
		t.Fatalf("expected explicit seed 424242 preserved, got %q in %v", seed, p.FingerprintArgs)
	}
}

func TestHasExplicitFingerprintSeed(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want bool
	}{
		{"nil", nil, false},
		{"static defaults only", []string{"--fingerprint-brand=Chrome", "--fingerprint-platform=mac"}, false},
		{"explicit seed", []string{"--fingerprint=123", "--fingerprint-platform=mac"}, true},
		{"zero seed treated as unset", []string{"--fingerprint=0"}, false},
		{"non-numeric seed ignored", []string{"--fingerprint=abc"}, false},
		{"whitespace around arg", []string{"  --fingerprint=42  "}, true},
	}
	for _, tc := range cases {
		if got := hasExplicitFingerprintSeed(tc.args); got != tc.want {
			t.Errorf("%s: hasExplicitFingerprintSeed(%v) = %v, want %v", tc.name, tc.args, got, tc.want)
		}
	}
}

func assertStringSliceContains(t *testing.T, values []string, expected string) {
	t.Helper()
	for _, value := range values {
		if value == expected {
			return
		}
	}
	t.Fatalf("values %#v missing %q", values, expected)
}
