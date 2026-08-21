package backend

import (
	"strings"
	"testing"

	"ant-chrome/backend/internal/config"
)

func TestLivePerfArgsAppliedByDefault(t *testing.T) {
	joined := strings.Join(appendLivePerfArgs(nil, &config.Config{}), "\n")
	if !strings.Contains(joined, "--process-per-site") {
		t.Fatal("live perf args should apply when unset (default on)")
	}
	if strings.Contains(joined, "--disable-site-isolation-trials") {
		t.Fatal("aggressive args must stay off by default")
	}
}

func TestLivePerfArgsAppliedWhenConfigNil(t *testing.T) {
	if len(appendLivePerfArgs(nil, nil)) == 0 {
		t.Fatal("nil config should still default to on")
	}
}

func TestLivePerfArgsDisabled(t *testing.T) {
	off := false
	cfg := &config.Config{}
	cfg.Browser.LivePerfEnabled = &off
	if got := appendLivePerfArgs(nil, cfg); len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

func TestLivePerfAggressiveOptIn(t *testing.T) {
	cfg := &config.Config{}
	cfg.Browser.LivePerfAggressive = true
	if !strings.Contains(strings.Join(appendLivePerfArgs(nil, cfg), "\n"), "--disable-site-isolation-trials") {
		t.Fatal("aggressive opt-in should append aggressive args")
	}
}

func TestBuildArgsDisablesExtensionsWhenNone(t *testing.T) {
	args := buildBrowserLaunchArgs("/tmp/x", 9222, "", nil, nil, nil, nil, nil, false)
	if !argsContainExact(args, "--disable-extensions") {
		t.Fatal("expected --disable-extensions when no extension dirs")
	}
	withExt := buildBrowserLaunchArgs("/tmp/x", 9222, "", []string{"/ext/a"}, nil, nil, nil, nil, false)
	if argsContainExact(withExt, "--disable-extensions") {
		t.Fatal("must not disable extensions when extension dirs exist")
	}
}

// 启动参数出口必须只剩一条 --disable-features(合并器生效),否则各层互相覆盖。
func TestBuildArgsMergesFeatureSwitches(t *testing.T) {
	args := buildBrowserLaunchArgs(
		"/tmp/x", 9222, "", nil,
		[]string{"--disable-features=FingerprintLayer"},
		[]string{"--disable-features=ProfileLayer"},
		[]string{"--disable-features=ExtraLayer"},
		nil, false,
	)
	count := 0
	merged := ""
	for _, a := range args {
		if strings.HasPrefix(a, "--disable-features=") {
			count++
			merged = a
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly one --disable-features, got %d: %v", count, args)
	}
	for _, want := range []string{"FingerprintLayer", "ProfileLayer", "ExtraLayer"} {
		if !strings.Contains(merged, want) {
			t.Errorf("merged switch lost %s: %s", want, merged)
		}
	}
}

func argsContainExact(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}
