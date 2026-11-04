package browser

import (
	"ant-chrome/backend/internal/config"
	"os"
	"path/filepath"
	"testing"
)

func TestCloakVersionFromDirName(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{name: "chromium-146.0.7680.177.5", want: "146.0.7680.177.5"},
		{name: "chromium-148.0.7778.215", want: "148.0.7778.215"},
		{name: "CHROMIUM-146.0.1", want: "146.0.1"},
		{name: "chromium-foo", want: ""},
		{name: "chromium-", want: ""},
		{name: "chromium-148", want: ""}, // 无点号，不认为是版本目录
		{name: "chrome-142", want: ""},
		{name: "", want: ""},
	}
	for _, item := range cases {
		if got := cloakVersionFromDirName(item.name); got != item.want {
			t.Fatalf("cloakVersionFromDirName(%q) = %q, want %q", item.name, got, item.want)
		}
	}
}

func TestCloakCoreVersionPicksHighestNestedVersion(t *testing.T) {
	baseDir := t.TempDir()
	for _, dir := range []string{"chromium-146.0.7680.177", "chromium-148.0.7778.215", "chromium-99.0.1"} {
		if err := os.MkdirAll(filepath.Join(baseDir, dir), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	if got, want := cloakCoreVersion(baseDir), "148.0.7778.215"; got != want {
		t.Fatalf("cloakCoreVersion = %q, want %q", got, want)
	}
}

func TestCloakCoreVersionReadsDirNameItself(t *testing.T) {
	baseDir := filepath.Join(t.TempDir(), "chromium-146.0.7680.177.5")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if got, want := cloakCoreVersion(baseDir), "146.0.7680.177.5"; got != want {
		t.Fatalf("cloakCoreVersion = %q, want %q", got, want)
	}
}

func TestCompareDottedVersion(t *testing.T) {
	cases := []struct {
		left  string
		right string
		want  int
	}{
		{left: "148.0.1", right: "146.9.9", want: 1},
		{left: "146.0.7680.177", right: "146.0.7680.177.5", want: -1},
		{left: "146.0.1", right: "146.0.1", want: 0},
		{left: "10.0.0", right: "9.9.9", want: 1},
	}
	for _, item := range cases {
		if got := compareDottedVersion(item.left, item.right); got != item.want {
			t.Fatalf("compareDottedVersion(%q, %q) = %d, want %d", item.left, item.right, got, item.want)
		}
	}
}

func TestDetectCoreBackendIdentifiesCloakLayout(t *testing.T) {
	cloakRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cloakRoot, "chromium-148.0.7778.215"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if got, want := DetectCoreBackend(cloakRoot), config.CoreBackendCloak; got != want {
		t.Fatalf("DetectCoreBackend(cloak layout) = %q, want %q", got, want)
	}

	fpRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(fpRoot, "manifest.json"), []byte(`{"version":"144.0.1"}`), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if got, want := DetectCoreBackend(fpRoot), config.CoreBackendFingerprintChromium; got != want {
		t.Fatalf("DetectCoreBackend(fingerprint layout) = %q, want %q", got, want)
	}
}

func TestGetCoreVersionUsesBackendSpecificSource(t *testing.T) {
	appRoot := t.TempDir()
	manager := NewManager(&config.Config{}, appRoot)

	// fingerprint-chromium：读 manifest.json
	fpDir := filepath.Join(appRoot, "chrome", "fp")
	if err := os.MkdirAll(fpDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(fpDir, "manifest.json"), []byte(`{"version":"144.0.7559.132"}`), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if got, want := manager.GetCoreVersion("chrome/fp", config.CoreBackendFingerprintChromium), "144.0.7559.132"; got != want {
		t.Fatalf("fingerprint core version = %q, want %q", got, want)
	}

	// cloak：没有 manifest，版本在目录名里
	cloakDir := filepath.Join(appRoot, "chrome", "cloak", "chromium-148.0.7778.215")
	if err := os.MkdirAll(cloakDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if got, want := manager.GetCoreVersion("chrome/cloak", config.CoreBackendCloak), "148.0.7778.215"; got != want {
		t.Fatalf("cloak core version = %q, want %q", got, want)
	}
	// 同一目录按 fingerprint-chromium 解析应当拿不到版本号
	if got := manager.GetCoreVersion("chrome/cloak", config.CoreBackendFingerprintChromium); got != "" {
		t.Fatalf("cloak dir parsed as fingerprint core = %q, want empty", got)
	}
}

func TestCoreExecutableCandidatesAnyBackendIsSupersetAndDeduped(t *testing.T) {
	merged := CoreExecutableCandidatesAnyBackend()
	seen := make(map[string]int, len(merged))
	for _, candidate := range merged {
		seen[candidate]++
	}
	for candidate, count := range seen {
		if count > 1 {
			t.Fatalf("candidate %q appears %d times in merged list", candidate, count)
		}
	}
	for _, backend := range config.KnownCoreBackends() {
		for _, candidate := range CoreExecutableCandidatesForBackend(backend) {
			if _, ok := seen[candidate]; !ok {
				t.Fatalf("merged candidates missing %q from backend %q", candidate, backend)
			}
		}
	}
}

func TestDefaultFingerprintArgsForCoreDropsCloakIncompatibleArgs(t *testing.T) {
	manager := NewManager(&config.Config{}, t.TempDir())
	manager.Config.Browser.DefaultFingerprintArgs = []string{
		"--fingerprint-brand=Chrome",
		"--fingerprint-platform=windows",
		"--disable-non-proxied-udp",
		"--fingerprinting-canvas-image-data-noise",
		"--fingerprinting-client-rects-noise",
		"--timezone=Asia/Shanghai",
	}
	manager.Config.Browser.Cores = []Core{
		{CoreId: "cloak-core", CoreName: "Cloak", CorePath: "chrome/cloak", CoreBackend: config.CoreBackendCloak},
		{CoreId: "fp-core", CoreName: "FP", CorePath: "chrome/fp", CoreBackend: config.CoreBackendFingerprintChromium, IsDefault: true},
	}

	cloakArgs := manager.defaultFingerprintArgsForCore("cloak-core")
	assertStringSliceContains(t, cloakArgs, "--fingerprint-brand=Chrome")
	assertStringSliceContains(t, cloakArgs, "--fingerprint-platform=windows")
	for _, unwanted := range []string{
		"--fingerprinting-canvas-image-data-noise",
		"--fingerprinting-client-rects-noise",
		"--timezone=Asia/Shanghai",
	} {
		for _, arg := range cloakArgs {
			if arg == unwanted {
				t.Fatalf("cloak default args should not contain %q: %#v", unwanted, cloakArgs)
			}
		}
	}

	fpArgs := manager.defaultFingerprintArgsForCore("fp-core")
	if got, want := len(fpArgs), len(manager.Config.Browser.DefaultFingerprintArgs); got != want {
		t.Fatalf("fingerprint default args length = %d, want %d: %#v", got, want, fpArgs)
	}
}

func TestUpgradeLegacyMinimalFingerprintArgsSkippedForCloak(t *testing.T) {
	manager := NewManager(&config.Config{}, t.TempDir())
	manager.Config.Browser.Cores = []Core{
		{CoreId: "cloak-core", CoreName: "Cloak", CorePath: "chrome/cloak", CoreBackend: config.CoreBackendCloak, IsDefault: true},
	}

	legacyArgs := []string{"--fingerprint-brand=Chrome", "--fingerprint-platform=windows"}
	upgraded := manager.upgradeLegacyMinimalFingerprintArgsForProfile("cloak-core", legacyArgs)

	if got, want := len(upgraded), len(legacyArgs); got != want {
		t.Fatalf("cloak args length = %d, want %d: %#v", got, want, upgraded)
	}
	for _, arg := range upgraded {
		if arg == "--fingerprinting-canvas-image-data-noise" {
			t.Fatalf("cloak profile should not receive fingerprint-chromium noise args: %#v", upgraded)
		}
	}
}

func TestResolveProfileCoreFallsBackToDefault(t *testing.T) {
	manager := NewManager(&config.Config{}, t.TempDir())
	manager.Config.Browser.Cores = []Core{
		{CoreId: "fp-core", CoreName: "FP", CorePath: "chrome/fp"},
		{CoreId: "cloak-core", CoreName: "Cloak", CorePath: "chrome/cloak", CoreBackend: config.CoreBackendCloak, IsDefault: true},
	}

	explicit, ok := manager.ResolveProfileCore(&Profile{CoreId: "fp-core"})
	if !ok || explicit.CoreId != "fp-core" {
		t.Fatalf("explicit core = %#v, ok = %v", explicit, ok)
	}

	// CoreId 为空或写着 "default" 时都应回落到默认内核
	for _, coreId := range []string{"", "default"} {
		fallback, ok := manager.ResolveProfileCore(&Profile{CoreId: coreId})
		if !ok || fallback.CoreId != "cloak-core" {
			t.Fatalf("fallback core for %q = %#v, ok = %v", coreId, fallback, ok)
		}
	}
}
