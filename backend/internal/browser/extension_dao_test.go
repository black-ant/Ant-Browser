package browser

import (
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/database"
	"os"
	"path/filepath"
	"testing"
)

func TestSQLiteExtensionDAOPersistsInstallMetadataAndRuntime(t *testing.T) {
	db, err := database.NewDB(filepath.Join(t.TempDir(), "extensions.db"))
	if err != nil {
		t.Fatalf("NewDB returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}

	dao := NewSQLiteExtensionDAO(db.GetConn())
	extension := Extension{
		ExtensionID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Name:        "persistent",
		Version:     "1.0.0",
		InstallDir:  filepath.Join(t.TempDir(), "extension"),
		InstallMode: ExtensionInstallModePersistent,
		PackagePath: filepath.Join(t.TempDir(), "extension.crx"),
		PackageHash: "package-hash",
		Enabled:     true,
	}
	if err := dao.Upsert(extension); err != nil {
		t.Fatalf("Upsert returned error: %v", err)
	}
	stored, err := dao.Get(extension.ExtensionID)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if stored.InstallMode != ExtensionInstallModePersistent || stored.PackagePath != extension.PackagePath || stored.PackageHash != extension.PackageHash {
		t.Fatalf("stored metadata = %#v, want persistent package metadata", stored)
	}

	runtimeState := ProfileExtensionRuntime{
		ProfileID:          "profile-a",
		ExtensionID:        extension.ExtensionID,
		RuntimeExtensionID: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		InstallMode:        ExtensionInstallModePersistent,
		InstalledVersion:   extension.Version,
		PackageHash:        extension.PackageHash,
		Status:             ExtensionRuntimeStatusInstalled,
	}
	if err := dao.UpsertProfileExtensionRuntime(runtimeState); err != nil {
		t.Fatalf("UpsertProfileExtensionRuntime returned error: %v", err)
	}
	storedRuntime, err := dao.GetProfileExtensionRuntime(runtimeState.ProfileID, runtimeState.ExtensionID)
	if err != nil {
		t.Fatalf("GetProfileExtensionRuntime returned error: %v", err)
	}
	if storedRuntime.RuntimeExtensionID != runtimeState.RuntimeExtensionID || storedRuntime.Status != ExtensionRuntimeStatusInstalled {
		t.Fatalf("stored runtime = %#v, want installed runtime", storedRuntime)
	}
}

func TestPrepareProfileExtensionsReturnsOnlyCommandlinePlugins(t *testing.T) {
	appRoot := t.TempDir()
	manager := NewManager(config.DefaultConfig(), appRoot)
	db, err := database.NewDB(filepath.Join(appRoot, "extensions.db"))
	if err != nil {
		t.Fatalf("NewDB returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}
	manager.ExtensionDAO = NewSQLiteExtensionDAO(db.GetConn())

	commandlineDir := filepath.Join(appRoot, "dev-extension")
	if err := os.MkdirAll(commandlineDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(commandlineDir, "manifest.json"), []byte(`{"name":"Dev","version":"1.0.0"}`), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if err := manager.ExtensionDAO.Upsert(Extension{
		ExtensionID: "cccccccccccccccccccccccccccccccc",
		Name:        "Dev",
		Version:     "1.0.0",
		InstallDir:  commandlineDir,
		InstallMode: ExtensionInstallModeCommandline,
		Enabled:     true,
	}); err != nil {
		t.Fatalf("Upsert commandline extension returned error: %v", err)
	}
	if err := manager.ExtensionDAO.Upsert(Extension{
		ExtensionID: "dddddddddddddddddddddddddddddddd",
		Name:        "Production",
		Version:     "1.0.0",
		InstallDir:  commandlineDir,
		InstallMode: ExtensionInstallModePersistent,
		PackagePath: filepath.Join(appRoot, "production.crx"),
		Enabled:     true,
	}); err != nil {
		t.Fatalf("Upsert persistent extension returned error: %v", err)
	}

	dirs := manager.EnabledExtensionDirsForProfile("profile-a")
	if len(dirs) != 1 || dirs[0] != commandlineDir {
		t.Fatalf("EnabledExtensionDirsForProfile = %#v, want only commandline directory", dirs)
	}
}
