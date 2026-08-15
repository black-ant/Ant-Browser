//go:build darwin

package browser

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// 端到端验证 macOS .dmg 解包:用 hdiutil 造一个内含最小 Chromium.app 的 dmg,
// 解包后 FindCoreExecutable 必须能定位到 Contents/MacOS/Chromium。
func TestExtractDmgAndStripRootDarwin(t *testing.T) {
	if _, err := exec.LookPath("hdiutil"); err != nil {
		t.Skip("hdiutil not available")
	}

	// 造最小 .app:Chromium.app/Contents/MacOS/Chromium(可执行)。
	src := t.TempDir()
	macOSDir := filepath.Join(src, "Chromium.app", "Contents", "MacOS")
	if err := os.MkdirAll(macOSDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(macOSDir, "Chromium"), []byte("#!/bin/sh\necho chromium\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	dmgPath := filepath.Join(t.TempDir(), "fake-core.dmg")
	create := exec.Command("hdiutil", "create",
		"-srcfolder", src, "-volname", "FakeCore",
		"-fs", "HFS+", "-format", "UDZO", "-ov", dmgPath)
	if out, err := create.CombinedOutput(); err != nil {
		t.Skipf("hdiutil create unavailable in this environment: %v (%s)", err, string(out))
	}

	dest := filepath.Join(t.TempDir(), "core")
	if err := extractCoreArchiveAndStripRoot(dmgPath, dest, func(int, string) {}); err != nil {
		t.Fatalf("extract dmg: %v", err)
	}

	binPath, candidate, ok := FindCoreExecutable(dest)
	if !ok {
		t.Fatalf("FindCoreExecutable did not locate an executable under %s", dest)
	}
	if filepath.Base(binPath) != "Chromium" {
		t.Fatalf("unexpected executable %q (candidate %q)", binPath, candidate)
	}
	if info, err := os.Stat(binPath); err != nil || info.IsDir() {
		t.Fatalf("extracted executable missing or is dir: %v", err)
	}
}
