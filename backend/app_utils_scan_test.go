package backend

import (
	"os"
	"path/filepath"
	"testing"

	"ant-chrome/backend/internal/config"
)

// scanChromeDir 多内核场景:默认内核应取"最高版本"(148),而非字典序首个(144)。
func TestScanChromeDirHighestVersionDefault(t *testing.T) {
	appRoot := t.TempDir()
	// chrome/144 + chrome/148 各放 fake chrome.exe + *.manifest
	for _, ver := range []string{"144", "148"} {
		dir := filepath.Join(appRoot, "chrome", ver)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		// 跨平台候选:Windows 用 chrome.exe,darwin/linux 用 chrome。各写一份,
		// FindCoreExecutable 按 CoreExecutableCandidates() 任一命中即识别为内核目录。
		for _, name := range []string{"chrome.exe", "chrome"} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte("fake"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(dir, ver+".0.0.0.manifest"), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	a := NewApp(appRoot)
	a.config = &config.Config{}
	cores := a.scanChromeDir("chrome")
	if len(cores) != 2 {
		t.Fatalf("expected 2 cores, got %d: %+v", len(cores), cores)
	}
	var def string
	for _, c := range cores {
		if c.IsDefault {
			def = c.CoreId
		}
	}
	if def != "core-148" {
		t.Errorf("default core = %q, want core-148", def)
	}
}

func TestCoreDirVersionRank(t *testing.T) {
	if coreDirVersionRank("chrome/148") <= coreDirVersionRank("chrome/144") {
		t.Errorf("148 should rank higher than 144")
	}
	if coreDirVersionRank("148.0.7778.215") <= coreDirVersionRank("144.0.7559.132") {
		t.Errorf("full 148 should rank higher than full 144")
	}
	// 真实目录名形如 fingerprint-chromium-148:末尾数字段必须被解析。
	if coreDirVersionRank("fingerprint-chromium-148") <= coreDirVersionRank("fingerprint-chromium-144") {
		t.Errorf("fingerprint-chromium-148 should rank higher than -144")
	}
	if coreDirVersionRank("abc") != 0 {
		t.Errorf("non-numeric should rank 0")
	}
}
