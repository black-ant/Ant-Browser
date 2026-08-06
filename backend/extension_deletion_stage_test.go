package backend

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBrowserExtensionDeletionStageRestoresFiles(t *testing.T) {
	root := t.TempDir()
	originalPath := filepath.Join(root, "extensions", "example.crx")
	stagingRoot := filepath.Join(root, "staging", "extension-1")
	stagedPath := filepath.Join(stagingRoot, "000-example.crx")
	if err := os.MkdirAll(filepath.Dir(stagedPath), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(stagedPath, []byte("package"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	stage := &browserExtensionDeletionStage{
		root: stagingRoot,
		entries: []browserExtensionDeletionEntry{{
			originalPath: originalPath,
			stagedPath:   stagedPath,
		}},
	}
	if err := stage.restore(); err != nil {
		t.Fatalf("restore returned error: %v", err)
	}
	restored, err := os.ReadFile(originalPath)
	if err != nil {
		t.Fatalf("ReadFile restored file returned error: %v", err)
	}
	if string(restored) != "package" {
		t.Fatalf("restored content = %q, want package", restored)
	}
	if _, err := os.Stat(stagingRoot); !os.IsNotExist(err) {
		t.Fatalf("staging root still exists, stat error = %v", err)
	}
}
