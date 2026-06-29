package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsWailsDevExecutableDir(t *testing.T) {
	t.Run("detects temp executable", func(t *testing.T) {
		if !isWailsDevExecutableDir("/var/folders/ab/tmp/wails-build", "/var/folders/ab/tmp") {
			t.Fatalf("expected temp dir to be treated as wails dev executable")
		}
	})

	t.Run("detects build bin root", func(t *testing.T) {
		if !isWailsDevExecutableDir("/Users/robbin/project/build/bin", "/var/folders/tmp") {
			t.Fatalf("expected build/bin root to be treated as wails dev executable")
		}
	})

	t.Run("detects mac app under build bin", func(t *testing.T) {
		exeDir := "/Users/robbin/project/build/bin/Maka Browser.app/Contents/MacOS"
		if !isWailsDevExecutableDir(exeDir, "/var/folders/tmp") {
			t.Fatalf("expected mac .app under build/bin to be treated as wails dev executable")
		}
	})

	t.Run("treats build bin mac app with runtime manifest as packaged", func(t *testing.T) {
		root := t.TempDir()
		exeDir := filepath.Join(root, "build", "bin", "Maka Browser.app", "Contents", "MacOS")
		manifestPath := filepath.Join(exeDir, "publish", "runtime-manifest.json")
		if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
			t.Fatalf("create manifest dir: %v", err)
		}
		if err := os.WriteFile(manifestPath, []byte(`{"schemaVersion":2}`), 0o644); err != nil {
			t.Fatalf("write manifest: %v", err)
		}

		if isWailsDevExecutableDir(exeDir, filepath.Join(root, "tmp")) {
			t.Fatalf("expected build/bin mac .app with runtime manifest to be treated as packaged")
		}
	})

	t.Run("ignores packaged app outside build bin", func(t *testing.T) {
		exeDir := "/Applications/Maka Browser.app/Contents/MacOS"
		if isWailsDevExecutableDir(exeDir, "/var/folders/tmp") {
			t.Fatalf("expected packaged app outside build/bin to be treated as production")
		}
	})
}
