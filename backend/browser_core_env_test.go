package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"strings"
	"testing"
)

func TestBuildBrowserProcessEnvReturnsNilWithoutOverrides(t *testing.T) {
	// 没有内核环境变量时必须返回 nil，让 cmd.Env 保持继承父进程环境。
	if env := buildBrowserProcessEnv(browser.Core{CoreId: "c1"}, "p1"); env != nil {
		t.Fatalf("env = %#v, want nil", env)
	}
	if env := buildBrowserProcessEnv(browser.Core{CoreId: "c1", CoreEnv: []string{"  "}}, "p1"); env != nil {
		t.Fatalf("env for blank entries = %#v, want nil", env)
	}
}

func TestBuildBrowserProcessEnvIncludesAllowedKeys(t *testing.T) {
	core := browser.Core{
		CoreId:      "cloak",
		CoreBackend: config.CoreBackendCloak,
		CoreEnv:     []string{"CLOAKBROWSER_LICENSE_KEY=secret", "CLOAKBROWSER_CACHE_DIR=D:/cache"},
	}
	env := buildBrowserProcessEnv(core, "p1")
	if env == nil {
		t.Fatal("env = nil, want merged environment")
	}
	assertEnvContains(t, env, "CLOAKBROWSER_LICENSE_KEY=secret")
	assertEnvContains(t, env, "CLOAKBROWSER_CACHE_DIR=D:/cache")
	// 合并后的环境必须仍然包含父进程条目，否则会丢掉 PATH 等必要变量
	if len(env) < 2 {
		t.Fatalf("env length = %d, want parent environment plus overrides", len(env))
	}
}

func TestBuildBrowserProcessEnvRejectsNonWhitelistedKeys(t *testing.T) {
	core := browser.Core{
		CoreId:  "cloak",
		CoreEnv: []string{"PATH=/evil", "HTTP_PROXY=http://evil", "CLOAKBROWSER_LICENSE_KEY=ok"},
	}
	env := buildBrowserProcessEnv(core, "p1")
	if env == nil {
		t.Fatal("env = nil, want merged environment for the allowed key")
	}
	assertEnvContains(t, env, "CLOAKBROWSER_LICENSE_KEY=ok")
	assertEnvNotContains(t, env, "PATH=/evil")
	assertEnvNotContains(t, env, "HTTP_PROXY=http://evil")
}

func TestSanitizeCoreEnvEntriesReportsRejections(t *testing.T) {
	accepted, rejected := sanitizeCoreEnvEntries([]string{
		"CLOAKBROWSER_LICENSE_KEY=ok",
		"PATH=/evil",
		"no-equals-sign",
		"=novalue",
		"CLOAKBROWSER_=empty-suffix",
	})
	if len(accepted) != 1 || accepted[0] != "CLOAKBROWSER_LICENSE_KEY=ok" {
		t.Fatalf("accepted = %#v, want single allowed entry", accepted)
	}
	if len(rejected) != 4 {
		t.Fatalf("rejected = %#v, want 4 entries", rejected)
	}
}

func TestMergeProcessEnvOverridesCaseInsensitively(t *testing.T) {
	// Windows 环境变量名不区分大小写，合并后不应出现同名双份条目。
	merged := mergeProcessEnv(
		[]string{"cloakbrowser_license_key=old", "OTHER=keep"},
		[]string{"CLOAKBROWSER_LICENSE_KEY=new"},
	)

	if len(merged) != 2 {
		t.Fatalf("merged = %#v, want 2 entries", merged)
	}
	assertEnvContains(t, merged, "CLOAKBROWSER_LICENSE_KEY=new")
	assertEnvContains(t, merged, "OTHER=keep")
	assertEnvNotContains(t, merged, "cloakbrowser_license_key=old")
}

func TestMergeProcessEnvAppendsNewKeys(t *testing.T) {
	merged := mergeProcessEnv([]string{"OTHER=keep"}, []string{"CLOAKBROWSER_CACHE_DIR=D:/cache"})
	if len(merged) != 2 {
		t.Fatalf("merged = %#v, want 2 entries", merged)
	}
	assertEnvContains(t, merged, "OTHER=keep")
	assertEnvContains(t, merged, "CLOAKBROWSER_CACHE_DIR=D:/cache")
}

func assertEnvContains(t *testing.T, env []string, expected string) {
	t.Helper()
	for _, item := range env {
		if item == expected {
			return
		}
	}
	t.Fatalf("env missing %q (len=%d)", expected, len(env))
}

func assertEnvNotContains(t *testing.T, env []string, unexpected string) {
	t.Helper()
	for _, item := range env {
		if item == unexpected {
			t.Fatalf("env unexpectedly contains %q", unexpected)
		}
	}
}

func TestBuildCloakFingerprintLaunchPlanKeepsBackendSpecificArgs(t *testing.T) {
	// 这些参数在 fingerprint-chromium 矩阵里会被剔除（removed / not_effective），
	// 在 Cloak 下必须原样保留，否则用户配置会被静默丢掉。
	args := []string{
		"--fingerprint=54321",
		"--fingerprint-gpu-vendor=Intel Inc.",
		"--fingerprint-gpu-renderer=Intel Iris",
		"--fingerprint-device-memory=8",
		"--fingerprint-screen-width=1920",
		"--fingerprint-screen-height=1080",
		"--fingerprint-locale=ja-JP",
		"--fingerprint-timezone=Asia/Tokyo",
	}
	plan := buildCloakFingerprintLaunchPlan("profile-1", args, "148.0.7778.215")

	for _, arg := range args {
		assertStringSliceContainsBackend(t, plan.launchArgs, arg)
	}
}

func TestBuildCloakFingerprintLaunchPlanInjectsSeedInRange(t *testing.T) {
	plan := buildCloakFingerprintLaunchPlan("profile-seed", nil, "148.0.7778.215")

	seedArg := ""
	for _, arg := range plan.launchArgs {
		if strings.HasPrefix(arg, "--fingerprint=") {
			seedArg = arg
		}
	}
	if seedArg == "" {
		t.Fatalf("launch args missing injected seed: %#v", plan.launchArgs)
	}
	value := strings.TrimPrefix(seedArg, "--fingerprint=")
	seed := 0
	for _, char := range value {
		if char < '0' || char > '9' {
			t.Fatalf("seed %q is not numeric", value)
		}
		seed = seed*10 + int(char-'0')
	}
	if seed < cloakSeedMin || seed > cloakSeedMax {
		t.Fatalf("seed = %d, want within [%d, %d]", seed, cloakSeedMin, cloakSeedMax)
	}

	// 同一实例 ID 必须得到稳定种子
	again := buildCloakFingerprintLaunchPlan("profile-seed", nil, "148.0.7778.215")
	if got, want := again.launchArgs[0], plan.launchArgs[0]; got != want {
		t.Fatalf("seed not stable across calls: %q vs %q", got, want)
	}
}

func TestBuildCloakFingerprintLaunchPlanDoesNotOverrideExplicitSeed(t *testing.T) {
	plan := buildCloakFingerprintLaunchPlan("profile-1", []string{"--fingerprint=777"}, "148.0.7778.215")

	seedCount := 0
	for _, arg := range plan.launchArgs {
		if strings.HasPrefix(arg, "--fingerprint=") {
			seedCount++
		}
	}
	if seedCount != 1 {
		t.Fatalf("seed arg count = %d, want 1: %#v", seedCount, plan.launchArgs)
	}
	assertStringSliceContainsBackend(t, plan.launchArgs, "--fingerprint=777")
}

func TestBuildCloakFingerprintLaunchPlanFlagsForeignArgs(t *testing.T) {
	plan := buildCloakFingerprintLaunchPlan("profile-1", []string{
		"--fingerprint=1",
		"--disable-spoofing=font,gpu",
		"--fingerprinting-canvas-image-data-noise",
	}, "148.0.7778.215")

	// 原样传递，但状态标记为 foreign_backend 并给出替代建议
	assertStringSliceContainsBackend(t, plan.launchArgs, "--disable-spoofing=font,gpu")
	assertStringSliceContainsBackend(t, plan.launchArgs, "--fingerprinting-canvas-image-data-noise")

	foreignRows := 0
	for _, row := range plan.rows {
		if row.Status == "foreign_backend" {
			foreignRows++
			if row.Note == "" {
				t.Fatalf("foreign row %q missing note", row.Capability)
			}
		}
	}
	if foreignRows != 2 {
		t.Fatalf("foreign_backend row count = %d, want 2", foreignRows)
	}
	if len(plan.warnings) == 0 {
		t.Fatal("expected a warning about fingerprint-chromium-only args")
	}
}

func TestBuildCloakFingerprintLaunchPlanWarnsOnUnknownVersion(t *testing.T) {
	plan := buildCloakFingerprintLaunchPlan("profile-1", []string{"--fingerprint=1"}, "")
	if len(plan.warnings) == 0 {
		t.Fatal("expected a warning when the Cloak core version is unknown")
	}
}

func TestBuildBrowserFingerprintExpectedForBackendDiffersByBackend(t *testing.T) {
	args := []string{
		"--fingerprint-device-memory=8",
		"--fingerprint-gpu-vendor=Intel Inc.",
		"--fingerprint-gpu-renderer=Intel Iris",
		"--fingerprint-locale=ja-JP",
		"--fingerprint-timezone=Asia/Tokyo",
	}

	// fingerprint-chromium：这些参数实测无效，不作为期望值
	fpExpected := buildBrowserFingerprintExpectedForBackend(args, config.CoreBackendFingerprintChromium)
	if fpExpected.DeviceMemory != "" || fpExpected.WebGLVendor != "" || fpExpected.WebGLRenderer != "" {
		t.Fatalf("fingerprint-chromium expected should ignore these args: %#v", fpExpected)
	}
	if fpExpected.Language != "" || fpExpected.Timezone != "" {
		t.Fatalf("fingerprint-chromium should not read Cloak locale/timezone keys: %#v", fpExpected)
	}

	// Cloak：同样的参数是受支持的，必须作为期望值
	cloakExpected := buildBrowserFingerprintExpectedForBackend(args, config.CoreBackendCloak)
	if cloakExpected.DeviceMemory != "8" {
		t.Fatalf("cloak deviceMemory = %q, want 8", cloakExpected.DeviceMemory)
	}
	if cloakExpected.WebGLVendor != "Intel Inc." || cloakExpected.WebGLRenderer != "Intel Iris" {
		t.Fatalf("cloak GPU expectations = %q / %q", cloakExpected.WebGLVendor, cloakExpected.WebGLRenderer)
	}
	if cloakExpected.Language != "ja-JP" || cloakExpected.Timezone != "Asia/Tokyo" {
		t.Fatalf("cloak locale expectations = %q / %q", cloakExpected.Language, cloakExpected.Timezone)
	}
}

func TestBuildBrowserFingerprintExpectedForBackendPrefersNativeLangKeys(t *testing.T) {
	// Cloak 后端下若同时存在 --lang 和 --fingerprint-locale，以 Chromium 原生 --lang 为准
	expected := buildBrowserFingerprintExpectedForBackend([]string{
		"--lang=en-US",
		"--fingerprint-locale=ja-JP",
	}, config.CoreBackendCloak)

	if expected.Language != "en-US" {
		t.Fatalf("language = %q, want en-US", expected.Language)
	}
}

func assertStringSliceContainsBackend(t *testing.T, values []string, expected string) {
	t.Helper()
	for _, value := range values {
		if value == expected {
			return
		}
	}
	t.Fatalf("values %#v missing %q", values, expected)
}
