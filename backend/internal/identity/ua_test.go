package identity

import "testing"

func TestBuildReducedUA(t *testing.T) {
	tests := []struct {
		platform string
		major    int
		want     string
	}{
		{"windows", 148, "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36"},
		{"macos", 148, "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36"},
		{"windows", 144, "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/144.0.0.0 Safari/537.36"},
		{"macos", 144, "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/144.0.0.0 Safari/537.36"},
		{"", 148, "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36"},
		{"MAC", 148, "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36"},
		{"windows", 0, ""}, // 0 不应产出 UA
	}
	for _, tt := range tests {
		if got := BuildReducedUA(tt.platform, tt.major); got != tt.want {
			t.Errorf("BuildReducedUA(%q,%d)=%q want %q", tt.platform, tt.major, got, tt.want)
		}
	}
}

func TestBrandVersionForMajor(t *testing.T) {
	if got := BrandVersionForMajor(148); got != "148.0.0.0" {
		t.Errorf("got %q", got)
	}
	if got := BrandVersionForMajor(0); got != "" {
		t.Errorf("0 should yield empty, got %q", got)
	}
}

func TestMajorFromVersion(t *testing.T) {
	tests := map[string]int{
		"148":                148,
		"148.0.7778.215":     148,
		"144.0.7559.132":     144,
		"":                   0,
		"abc":                0,
		"148.0.0.0":          148,
	}
	for in, want := range tests {
		if got := MajorFromVersion(in); got != want {
			t.Errorf("MajorFromVersion(%q)=%d want %d", in, got, want)
		}
	}
}

func TestApplyKernelVersion(t *testing.T) {
	id := Identity{Platform: "windows", UAFull: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.36", BrandVersion: "145.0.0.0"}
	out := ApplyKernelVersion(id, 148)
	if out.UAFull != BuildReducedUA("windows", 148) {
		t.Errorf("UA not rewritten to 148: %q", out.UAFull)
	}
	if out.BrandVersion != "148.0.0.0" {
		t.Errorf("brand not rewritten: %q", out.BrandVersion)
	}
	// major<=0 不改动
	orig := id
	if out2 := ApplyKernelVersion(orig, 0); out2.UAFull != orig.UAFull || out2.BrandVersion != orig.BrandVersion {
		t.Errorf("major=0 should be no-op")
	}
}
