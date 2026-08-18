package backend

import "testing"

// 启动时 UA 覆盖:池里烘焙旧版本 UA,启动应以内核真实版本覆盖。
func TestOverrideUAToKernelVersion(t *testing.T) {
	args := []string{
		"--fingerprint=123",
		"--fingerprint-platform=windows",
		"--fingerprint-brand-version=145.0.0.0",
		"--user-agent=Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.36",
		"--fingerprint-hardware-concurrency=12",
	}
	out := overrideUAToKernelVersion(args, "148.0.7778.215")
	expectUA := "--user-agent=Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36"
	expectBrand := "--fingerprint-brand-version=148.0.0.0"
	assertContains(t, out, expectUA)
	assertContains(t, out, expectBrand)
	// 其它参数保持不变
	assertContains(t, out, "--fingerprint=123")
	assertContains(t, out, "--fingerprint-platform=windows")

	// macOS 平台:从 --fingerprint-platform=macos 推断
	mac := []string{
		"--fingerprint-platform=macos",
		"--fingerprint-brand-version=147.0.0.0",
		"--user-agent=Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36",
	}
	macOut := overrideUAToKernelVersion(mac, "144.0.7559.132")
	assertContains(t, macOut, "--user-agent=Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/144.0.0.0 Safari/537.36")
	assertContains(t, macOut, "--fingerprint-brand-version=144.0.0.0")
}

// 内核版本无法识别(major<=0)时不覆盖,保留原值。
func TestOverrideUASkipsUnknownVersion(t *testing.T) {
	args := []string{"--user-agent=Mozilla/5.0 ... Chrome/145.0.0.0 Safari/537.36"}
	out := overrideUAToKernelVersion(args, "")
	if out[0] != args[0] {
		t.Errorf("unknown version should be no-op, got %q", out[0])
	}
}

func assertContains(t *testing.T, slice []string, want string) {
	t.Helper()
	for _, s := range slice {
		if s == want {
			return
		}
	}
	t.Errorf("slice %v missing %q", slice, want)
}
