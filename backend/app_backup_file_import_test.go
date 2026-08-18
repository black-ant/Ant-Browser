package backend

import (
	"ant-chrome/backend/internal/config"
	"os"
	"path/filepath"
	"testing"
)

func TestBackupImportFileTreesPreservesAppDataWhenUserDataOverlaps(t *testing.T) {
	root := t.TempDir()
	appRoot := filepath.Join(root, "app")
	payloadRoot := filepath.Join(root, "payload")
	if err := os.MkdirAll(filepath.Join(payloadRoot, "app", "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(payloadRoot, "browser", "user-data", "Profile"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(payloadRoot, "app", "data", "restored.txt"), []byte("restored"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(payloadRoot, "browser", "user-data", "Profile", "Preferences"), []byte("browser"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(appRoot, "data", "stale"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appRoot, "data", "stale", "old.txt"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	app := &App{appRoot: appRoot, config: config.DefaultConfig()}
	stats := &backupMergeStats{}
	var issues []error
	app.backupImportFileTrees(payloadRoot, app.config, true, stats, func(_ string, _ string, err error) {
		issues = append(issues, err)
	})
	if len(issues) > 0 {
		t.Fatalf("file import reported issues: %v", issues)
	}
	if _, err := os.Stat(filepath.Join(appRoot, "data", "restored.txt")); err != nil {
		t.Fatalf("restored app data was removed by overlapping user data import: %v", err)
	}
	if _, err := os.Stat(filepath.Join(appRoot, "data", "Profile", "Preferences")); err != nil {
		t.Fatalf("browser user data was not imported: %v", err)
	}
}
