package backend

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
)

func TestProfilePackageExportImportRestoresExtensionArtifacts(t *testing.T) {
	zipPath, extensionID := createProfilePackageWithExtensionArtifacts(t)
	targetRoot := t.TempDir()
	targetDB := newProfilePackageDatabase(t, targetRoot)
	targetCfg := config.DefaultConfig()
	targetCfg.Browser.UserDataRoot = filepath.Join(targetRoot, "user-data")
	targetApp := NewApp(targetRoot)
	targetApp.config = targetCfg
	targetApp.db = targetDB
	targetApp.browserMgr = browser.NewManager(targetCfg, targetRoot)
	targetApp.browserMgr.ProfileDAO = browser.NewSQLiteProfileDAO(targetDB.GetConn())
	targetApp.browserMgr.ProxyDAO = browser.NewSQLiteProxyDAO(targetDB.GetConn())
	targetApp.browserMgr.CoreDAO = browser.NewSQLiteCoreDAO(targetDB.GetConn())
	targetApp.browserMgr.GroupDAO = browser.NewSQLiteGroupDAO(targetDB.GetConn())
	targetApp.browserMgr.ExtensionDAO = browser.NewSQLiteExtensionDAO(targetDB.GetConn())

	result, err := targetApp.importProfilePackageFromPath(zipPath)
	if err != nil {
		t.Fatalf("importProfilePackageFromPath returned error: %v", err)
	}
	if result.ImportedCount != 1 {
		t.Fatalf("imported count = %d, want 1", result.ImportedCount)
	}

	stored, err := browser.NewSQLiteExtensionDAO(targetDB.GetConn()).Get(extensionID)
	if err != nil {
		t.Fatalf("read imported extension failed: %v", err)
	}
	expectedInstallDir := filepath.Join(targetRoot, "data", "extensions", extensionID)
	expectedPackagePath := filepath.Join(targetRoot, "data", "extensions", "packages", extensionID+".crx")
	if filepath.Clean(stored.InstallDir) != filepath.Clean(expectedInstallDir) {
		t.Fatalf("install_dir = %q, want %q", stored.InstallDir, expectedInstallDir)
	}
	if filepath.Clean(stored.PackagePath) != filepath.Clean(expectedPackagePath) {
		t.Fatalf("package_path = %q, want %q", stored.PackagePath, expectedPackagePath)
	}
	manifest, err := os.ReadFile(filepath.Join(expectedInstallDir, "manifest.json"))
	if err != nil {
		t.Fatalf("read restored extension manifest failed: %v", err)
	}
	if string(manifest) != `{"name":"source extension"}` {
		t.Fatalf("restored manifest = %q", manifest)
	}
	nested, err := os.ReadFile(filepath.Join(expectedInstallDir, "nested", "settings.json"))
	if err != nil {
		t.Fatalf("read restored nested extension file failed: %v", err)
	}
	if string(nested) != `{"enabled":true}` {
		t.Fatalf("restored nested file = %q", nested)
	}
	packageData, err := os.ReadFile(expectedPackagePath)
	if err != nil {
		t.Fatalf("read restored extension package failed: %v", err)
	}
	if string(packageData) != "source-package" {
		t.Fatalf("restored package = %q", packageData)
	}
}

func TestProfilePackageImportPreservesExistingExtensionArtifacts(t *testing.T) {
	zipPath, extensionID := createProfilePackageWithExtensionArtifacts(t)
	targetRoot := t.TempDir()
	targetDB := newProfilePackageDatabase(t, targetRoot)
	targetCfg := config.DefaultConfig()
	targetCfg.Browser.UserDataRoot = filepath.Join(targetRoot, "user-data")
	targetApp := NewApp(targetRoot)
	targetApp.config = targetCfg
	targetApp.db = targetDB
	targetApp.browserMgr = browser.NewManager(targetCfg, targetRoot)
	targetApp.browserMgr.ProfileDAO = browser.NewSQLiteProfileDAO(targetDB.GetConn())
	targetApp.browserMgr.ProxyDAO = browser.NewSQLiteProxyDAO(targetDB.GetConn())
	targetApp.browserMgr.CoreDAO = browser.NewSQLiteCoreDAO(targetDB.GetConn())
	targetApp.browserMgr.GroupDAO = browser.NewSQLiteGroupDAO(targetDB.GetConn())
	targetApp.browserMgr.ExtensionDAO = browser.NewSQLiteExtensionDAO(targetDB.GetConn())

	installDir := filepath.Join(targetRoot, "data", "extensions", extensionID)
	packagePath := filepath.Join(targetRoot, "data", "extensions", "packages", extensionID+".crx")
	if err := os.MkdirAll(filepath.Join(installDir, "nested"), 0o755); err != nil {
		t.Fatalf("create existing extension directory failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(packagePath), 0o755); err != nil {
		t.Fatalf("create existing extension package directory failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(installDir, "manifest.json"), []byte("local-manifest"), 0o644); err != nil {
		t.Fatalf("write existing extension manifest failed: %v", err)
	}
	if err := os.WriteFile(packagePath, []byte("local-package"), 0o644); err != nil {
		t.Fatalf("write existing extension package failed: %v", err)
	}

	if _, err := targetApp.importProfilePackageFromPath(zipPath); err != nil {
		t.Fatalf("importProfilePackageFromPath returned error: %v", err)
	}
	manifest, err := os.ReadFile(filepath.Join(installDir, "manifest.json"))
	if err != nil {
		t.Fatalf("read existing extension manifest failed: %v", err)
	}
	if string(manifest) != "local-manifest" {
		t.Fatalf("existing extension manifest was overwritten: %q", manifest)
	}
	packageData, err := os.ReadFile(packagePath)
	if err != nil {
		t.Fatalf("read existing extension package failed: %v", err)
	}
	if string(packageData) != "local-package" {
		t.Fatalf("existing extension package was overwritten: %q", packageData)
	}
}

func createProfilePackageWithExtensionArtifacts(t *testing.T) (string, string) {
	t.Helper()
	sourceRoot := t.TempDir()
	sourceDB := newProfilePackageDatabase(t, sourceRoot)
	sourceCfg := config.DefaultConfig()
	sourceCfg.Browser.UserDataRoot = filepath.Join(sourceRoot, "user-data")
	sourceApp := NewApp(sourceRoot)
	sourceApp.config = sourceCfg
	sourceApp.db = sourceDB
	sourceApp.browserMgr = browser.NewManager(sourceCfg, sourceRoot)

	profileID := "source-extension-profile"
	extensionID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	profile := browser.Profile{
		ProfileId:   profileID,
		ProfileName: "插件实例",
		UserDataDir: profileID,
		CreatedAt:   "2026-08-31T00:00:00Z",
		UpdatedAt:   "2026-08-31T00:00:00Z",
	}
	insertProfilePackageDatabaseFixtures(t, sourceDB.GetConn(), profile, "source-extension-parent", extensionID)
	installDir := filepath.Join(sourceRoot, "data", "extensions", extensionID)
	packagePath := filepath.Join(sourceRoot, "data", "extensions", "packages", extensionID+".crx")
	if _, err := sourceDB.GetConn().Exec(`UPDATE browser_extensions SET install_dir = ?, package_path = ? WHERE extension_id = ?`, filepath.ToSlash(filepath.Join("data", "extensions", extensionID)), filepath.ToSlash(filepath.Join("data", "extensions", "packages", extensionID+".crx")), extensionID); err != nil {
		t.Fatalf("update extension paths failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(installDir, "nested"), 0o755); err != nil {
		t.Fatalf("create source extension directory failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(packagePath), 0o755); err != nil {
		t.Fatalf("create source extension package directory failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(installDir, "manifest.json"), []byte(`{"name":"source extension"}`), 0o644); err != nil {
		t.Fatalf("write source extension manifest failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(installDir, "nested", "settings.json"), []byte(`{"enabled":true}`), 0o644); err != nil {
		t.Fatalf("write source nested extension file failed: %v", err)
	}
	if err := os.WriteFile(packagePath, []byte("source-package"), 0o644); err != nil {
		t.Fatalf("write source extension package failed: %v", err)
	}

	zipPath := filepath.Join(sourceRoot, "extension-profile.zip")
	if _, err := sourceApp.writeProfilePackage(zipPath, []browser.Profile{profile}); err != nil {
		t.Fatalf("writeProfilePackage returned error: %v", err)
	}
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open exported profile package failed: %v", err)
	}
	defer reader.Close()
	var snapshot ProfilePackageDatabase
	for _, entry := range reader.File {
		if entry.Name != profilePackageDatabasePath {
			continue
		}
		file, err := entry.Open()
		if err != nil {
			t.Fatalf("open database snapshot failed: %v", err)
		}
		if err := json.NewDecoder(file).Decode(&snapshot); err != nil {
			_ = file.Close()
			t.Fatalf("decode database snapshot failed: %v", err)
		}
		_ = file.Close()
	}
	if len(snapshot.ExtensionArtifacts) != 1 {
		t.Fatalf("extension artifacts = %#v, want one artifact", snapshot.ExtensionArtifacts)
	}
	if snapshot.ExtensionArtifacts[0].InstallDirArchivePath == "" || snapshot.ExtensionArtifacts[0].PackageArchivePath == "" {
		t.Fatalf("extension artifact paths are incomplete: %#v", snapshot.ExtensionArtifacts[0])
	}
	return zipPath, extensionID
}
