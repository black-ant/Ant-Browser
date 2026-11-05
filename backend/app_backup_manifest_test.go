package backend

import (
	"ant-chrome/backend/internal/backup"
	"ant-chrome/backend/internal/config"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBackupPackageManifestDetectsTampering(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "source")
	if err := os.MkdirAll(filepath.Join(sourceDir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(sourceDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("app:\\n  name: test\\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "nested", "state.txt"), []byte("stable state"), 0o644); err != nil {
		t.Fatal(err)
	}

	scope := backup.Scope{
		Format:          backup.PackageFormat,
		ManifestVersion: backup.ManifestVersion,
		AppRoot:         sourceDir,
		Entries: []backup.ScopeEntry{
			{
				ID:          "system_config_main",
				Category:    backup.CategorySystemConfig,
				EntryType:   backup.EntryTypeFile,
				Required:    true,
				SourcePath:  configPath,
				ArchivePath: "payload/system/config.yaml",
			},
			{
				ID:          "app_data_root",
				Category:    backup.CategoryAppData,
				EntryType:   backup.EntryTypeDir,
				Required:    true,
				SourcePath:  sourceDir,
				ArchivePath: "payload/app/data/",
			},
		},
	}
	manifest := backup.BuildManifest(scope, "test", "1.0.0", time.Unix(0, 0))
	zipPath := filepath.Join(root, "backup.zip")
	if _, _, _, err := backupWritePackageZip(zipPath, scope, manifest, nil); err != nil {
		t.Fatal(err)
	}

	extractRoot, importedManifest, err := backupExtractAndValidate(zipPath)
	if err != nil {
		t.Fatalf("valid backup rejected: %v", err)
	}
	defer os.RemoveAll(extractRoot)

	if len(importedManifest.Entries) != 2 {
		t.Fatalf("manifest entries = %d, want 2", len(importedManifest.Entries))
	}
	tamperedPath := filepath.Join(extractRoot, "payload", "app", "data", "nested", "state.txt")
	if err := os.WriteFile(tamperedPath, []byte("tampered state"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := backupValidateManifest(extractRoot, importedManifest); err == nil {
		t.Fatal("tampered backup passed manifest validation")
	}
}

func TestBackupNormalizeImportedConfigPaths(t *testing.T) {
	root := t.TempDir()
	app := &App{appRoot: filepath.Join(root, "current")}
	current := config.DefaultConfig()
	incoming := config.DefaultConfig()
	incoming.Browser.UserDataRoot = filepath.Join(root, "old", "profiles")
	incoming.Browser.CoreRoot = filepath.Join(root, "old", "chrome")
	incoming.Browser.Cores = []config.BrowserCore{{
		CoreId:   "core-1",
		CorePath: filepath.Join(root, "old", "chrome", "core-1"),
	}}
	incoming.Browser.Profiles = []config.BrowserProfileConfig{{
		ProfileId:   "profile-1",
		UserDataDir: filepath.Join(root, "old", "profiles", "profile-1"),
	}}

	normalized := app.backupNormalizeImportedConfigPaths(incoming, current)
	if normalized.Browser.UserDataRoot != "data" {
		t.Fatalf("user data root = %q, want data", normalized.Browser.UserDataRoot)
	}
	if normalized.Browser.CoreRoot != "chrome" {
		t.Fatalf("core root = %q, want chrome", normalized.Browser.CoreRoot)
	}
	if normalized.Browser.Cores[0].CorePath != filepath.Join("chrome", "external", "core-1") {
		t.Fatalf("core path = %q, want local fallback", normalized.Browser.Cores[0].CorePath)
	}
	if normalized.Browser.Profiles[0].UserDataDir != "profile-1" {
		t.Fatalf("profile path = %q, want user-data-root relative fallback", normalized.Browser.Profiles[0].UserDataDir)
	}
}

func TestBackupNormalizeImportedRelativeProfilePath(t *testing.T) {
	root := t.TempDir()
	app := &App{appRoot: filepath.Join(root, "current")}
	incoming := config.DefaultConfig()
	incoming.Browser.UserDataRoot = "data"
	incoming.Browser.Profiles = []config.BrowserProfileConfig{{
		ProfileId:   "profile-1",
		UserDataDir: filepath.Join("data", "profile-1"),
	}}

	normalized := app.backupNormalizeImportedConfigPaths(incoming, config.DefaultConfig())
	if normalized.Browser.Profiles[0].UserDataDir != "profile-1" {
		t.Fatalf("profile path = %q, want profile-1", normalized.Browser.Profiles[0].UserDataDir)
	}
}

func TestBackupNormalizeImportedProfilePathRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	app := &App{appRoot: filepath.Join(root, "current")}
	incoming := config.DefaultConfig()
	incoming.Browser.UserDataRoot = "data"
	incoming.Browser.Profiles = []config.BrowserProfileConfig{{
		ProfileId:   "profile-1",
		UserDataDir: filepath.Join("..", "outside"),
	}}

	normalized := app.backupNormalizeImportedConfigPaths(incoming, config.DefaultConfig())
	if normalized.Browser.Profiles[0].UserDataDir != "outside" {
		t.Fatalf("profile path = %q, want fallback outside", normalized.Browser.Profiles[0].UserDataDir)
	}
}

func TestBackupNormalizeImportedCorePathRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	app := &App{appRoot: filepath.Join(root, "current")}
	incoming := config.DefaultConfig()
	incoming.Browser.CoreRoot = "chrome"
	incoming.Browser.Cores = []config.BrowserCore{{
		CoreId:   "core-1",
		CorePath: filepath.Join("..", "outside"),
	}}

	normalized := app.backupNormalizeImportedConfigPaths(incoming, config.DefaultConfig())
	if normalized.Browser.Cores[0].CorePath != filepath.Join("chrome", "external", "core-1") {
		t.Fatalf("core path = %q, want chrome/external/core-1", normalized.Browser.Cores[0].CorePath)
	}
}

func TestBackupPackageAcceptsEmptyRequiredDirectory(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "empty")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	scope := backup.Scope{
		Format:          backup.PackageFormat,
		ManifestVersion: backup.ManifestVersion,
		AppRoot:         root,
		Entries: []backup.ScopeEntry{{
			ID:          "empty_required_dir",
			Category:    backup.CategoryAppData,
			EntryType:   backup.EntryTypeDir,
			Required:    true,
			SourcePath:  sourceDir,
			ArchivePath: "payload/empty/",
		}},
	}
	manifest := backup.BuildManifest(scope, "test", "1.0.0", time.Unix(0, 0))
	zipPath := filepath.Join(root, "empty.zip")
	if _, _, _, err := backupWritePackageZip(zipPath, scope, manifest, nil); err != nil {
		t.Fatal(err)
	}

	extractRoot, _, err := backupExtractAndValidate(zipPath)
	if err != nil {
		t.Fatalf("empty required directory backup rejected: %v", err)
	}
	defer os.RemoveAll(extractRoot)
	info, err := os.Stat(filepath.Join(extractRoot, "payload", "empty"))
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Fatal("empty required directory was not restored as a directory")
	}
}

func TestBackupManifestStoresExternalCoreID(t *testing.T) {
	root := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Browser.Cores = []config.BrowserCore{
		{CoreId: "core-b", CorePath: filepath.Join(root, "z-core")},
		{CoreId: "core-a", CorePath: filepath.Join(root, "a-core")},
	}

	scope, err := backup.BuildScope(backup.BuildOptions{AppRoot: root, Config: cfg})
	if err != nil {
		t.Fatal(err)
	}
	manifest := backup.BuildManifest(scope, "test", "1.7.0", time.Unix(0, 0))
	coreIDs := make(map[string]string)
	for _, entry := range manifest.Entries {
		if !strings.HasPrefix(entry.ArchivePath, "payload/browser/cores/external/") {
			continue
		}
		coreIDs[entry.ArchivePath] = entry.CoreId
	}
	if len(coreIDs) != 2 {
		t.Fatalf("external core manifest entries = %d, want 2", len(coreIDs))
	}
	for archivePath, coreID := range coreIDs {
		if coreID == "" {
			t.Fatalf("external core manifest entry %q has no core ID", archivePath)
		}
	}
}
