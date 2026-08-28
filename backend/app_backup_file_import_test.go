package backend

import (
	"ant-chrome/backend/internal/backup"
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
	app.backupImportFileTrees(payloadRoot, app.config, backup.Manifest{}, stats, func(_ string, _ string, err error) {
		issues = append(issues, err)
	})
	if len(issues) > 0 {
		t.Fatalf("file import reported issues: %v", issues)
	}
	if _, err := os.Stat(filepath.Join(appRoot, "data", "restored.txt")); err != nil {
		t.Fatalf("restored app data was removed by overlapping user data import: %v", err)
	}
	if _, err := os.Stat(filepath.Join(appRoot, "data", "stale", "old.txt")); err != nil {
		t.Fatalf("existing app data was removed by backup import: %v", err)
	}
	if _, err := os.Stat(filepath.Join(appRoot, "data", "Profile", "Preferences")); err != nil {
		t.Fatalf("browser user data was not imported: %v", err)
	}
}

func TestBackupImportExternalCoresUsesManifestCoreIDs(t *testing.T) {
	root := t.TempDir()
	appRoot := filepath.Join(root, "app")
	payloadRoot := filepath.Join(root, "payload")
	for _, folder := range []string{"external-01", "external-02"} {
		if err := os.MkdirAll(filepath.Join(payloadRoot, "browser", "cores", "external", folder), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(payloadRoot, "browser", "cores", "external", "external-01", "marker.txt"), []byte("core-b"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(payloadRoot, "browser", "cores", "external", "external-02", "marker.txt"), []byte("core-a"), 0o644); err != nil {
		t.Fatal(err)
	}

	incoming := config.DefaultConfig()
	incoming.Browser.Cores = []config.BrowserCore{
		{CoreId: "core-b", CorePath: "z-core"},
		{CoreId: "core-a", CorePath: "a-core"},
	}
	manifest := backup.Manifest{Entries: []backup.ManifestEntry{
		{ArchivePath: "payload/browser/cores/external/external-01/", CoreId: "core-b"},
		{ArchivePath: "payload/browser/cores/external/external-02/", CoreId: "core-a"},
	}}

	app := &App{appRoot: appRoot, config: config.DefaultConfig()}
	stats := &backupMergeStats{}
	var issues []error
	app.backupImportFileTrees(payloadRoot, incoming, manifest, stats, func(_ string, _ string, err error) {
		issues = append(issues, err)
	})
	if len(issues) > 0 {
		t.Fatalf("file import reported issues: %v", issues)
	}

	coreB, err := os.ReadFile(filepath.Join(appRoot, "z-core", "marker.txt"))
	if err != nil {
		t.Fatal(err)
	}
	coreA, err := os.ReadFile(filepath.Join(appRoot, "a-core", "marker.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(coreB) != "core-b" || string(coreA) != "core-a" {
		t.Fatalf("external cores were mapped by order instead of core ID: z-core=%q a-core=%q", coreB, coreA)
	}
}
