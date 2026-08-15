package browser

import "testing"

// 下载临时文件必须保留正确后缀,解包分支才能据此路由(.dmg 尤其关键)。
func TestCoreArchiveTempPatternPreservesSuffix(t *testing.T) {
	cases := map[string]string{
		"https://host/path/ungoogled-chromium_148_macos.dmg":     "download_*.dmg",
		"https://host/path/ungoogled-chromium_148_windows_x64.zip": "download_*.zip",
		"https://host/path/ungoogled-chromium_148_linux.tar.xz":    "download_*.tar.xz",
		"https://host/path/kernel.tgz":                             "download_*.tgz",
		"https://host/path/no-extension":                           "download_*",
	}
	for url, want := range cases {
		if got := coreArchiveTempPattern(url); got != want {
			t.Errorf("coreArchiveTempPattern(%q) = %q, want %q", url, got, want)
		}
	}
}

// .dmg / .zip 各有独立解包分支,不能被误判为 tar 家族。
func TestIsTarArchivePathExcludesZipAndDmg(t *testing.T) {
	tar := []string{"a.tar", "a.tar.gz", "a.tgz", "a.tar.xz", "a.txz", "a.tar.bz2", "a.tbz2"}
	notTar := []string{"a.zip", "a.dmg"}
	for _, p := range tar {
		if !isTarArchivePath(p) {
			t.Errorf("isTarArchivePath(%q) = false, want true", p)
		}
	}
	for _, p := range notTar {
		if isTarArchivePath(p) {
			t.Errorf("isTarArchivePath(%q) = true, want false", p)
		}
	}
}
