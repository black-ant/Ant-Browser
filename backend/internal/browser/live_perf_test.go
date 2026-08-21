package browser

import (
	"strings"
	"testing"
)

func TestLivePerfArgsMustHaves(t *testing.T) {
	joined := strings.Join(LivePerfArgs(), "\n")
	must := []string{
		"--process-per-site",
		"--renderer-process-limit=3",
		"--disable-background-networking",
		"--disable-component-update",
		"--disable-component-extensions-with-background-pages",
		"--disable-background-mode",
		"--metrics-recording-only",
		"--disable-ipc-flooding-protection",
		"--disk-cache-size=134217728",
		"--media-cache-size=33554432",
	}
	for _, m := range must {
		if !strings.Contains(joined, m) {
			t.Errorf("missing %s", m)
		}
	}
	for _, f := range []string{"SpareRendererForSitePerProcess", "AudioServiceOutOfProcess", "MediaRouter"} {
		if !strings.Contains(joined, f) {
			t.Errorf("missing feature %s", f)
		}
	}
}

// 看播时长命脉:遮挡窗口必须保持 visibilityState=visible。
func TestLivePerfArgsKeepsOcclusionDisabled(t *testing.T) {
	if !strings.Contains(strings.Join(LivePerfArgs(), "\n"), "CalculateNativeWinOcclusion") {
		t.Fatal("must keep CalculateNativeWinOcclusion disabled")
	}
}

// 红线守卫:A/B 两级参数集都不得碰指纹面 / 播放面 / CDP / 安全。
func TestLivePerfArgsRespectRedLines(t *testing.T) {
	for name, set := range map[string][]string{
		"A": LivePerfArgs(),
		"B": LivePerfAggressiveArgs(),
	} {
		if bad := AssertNoForbiddenArgs(set); len(bad) != 0 {
			t.Errorf("tier %s violates red lines: %v", name, bad)
		}
	}
}

// 整个参数集只允许有一条 --disable-features(否则自身就先自我覆盖了)。
func TestLivePerfArgsHasSingleFeatureSwitch(t *testing.T) {
	count := 0
	for _, a := range LivePerfArgs() {
		if strings.HasPrefix(a, "--disable-features=") {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly one --disable-features switch, got %d", count)
	}
}

func TestLivePerfAggressiveArgs(t *testing.T) {
	if !strings.Contains(strings.Join(LivePerfAggressiveArgs(), "\n"), "--disable-site-isolation-trials") {
		t.Error("aggressive set must merge cross-site iframes")
	}
}
