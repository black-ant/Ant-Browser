package browser

import (
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/database"
	"encoding/binary"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestPersistentExtensionArtifactMatchesVersionedDirectory(t *testing.T) {
	userDataDir := t.TempDir()
	manifestDir := filepath.Join(userDataDir, "Default", "Extensions", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "1.2.3_0")
	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(manifestDir, "manifest.json"), []byte(`{"name":"Test","version":"1.2.3"}`), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if !persistentExtensionArtifactMatches(userDataDir, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "1.2.3") {
		t.Fatal("persistentExtensionArtifactMatches = false, want true for Chromium version directory suffix")
	}
}

func TestMigrateExtensionStoragePrefersLegacyData(t *testing.T) {
	userDataDir := t.TempDir()
	oldRuntimeID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	newRuntimeID := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	oldPath := filepath.Join(userDataDir, "Default", "Local Extension Settings", oldRuntimeID)
	newPath := filepath.Join(userDataDir, "Default", "Local Extension Settings", newRuntimeID)
	if err := os.MkdirAll(oldPath, 0o755); err != nil {
		t.Fatalf("MkdirAll old path returned error: %v", err)
	}
	if err := os.MkdirAll(newPath, 0o755); err != nil {
		t.Fatalf("MkdirAll new path returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(oldPath, "legacy.log"), []byte("legacy"), 0o644); err != nil {
		t.Fatalf("WriteFile old path returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(newPath, "default.log"), []byte("default"), 0o644); err != nil {
		t.Fatalf("WriteFile new path returned error: %v", err)
	}

	if err := migrateExtensionStorage(userDataDir, oldRuntimeID, newRuntimeID); err != nil {
		t.Fatalf("migrateExtensionStorage returned error: %v", err)
	}
	legacyData, err := os.ReadFile(filepath.Join(newPath, "legacy.log"))
	if err != nil {
		t.Fatalf("ReadFile migrated data returned error: %v", err)
	}
	if string(legacyData) != "legacy" {
		t.Fatalf("migrated data = %q, want legacy data", legacyData)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("old storage path still exists, stat error = %v", err)
	}
}

func TestResolveExtensionPackageUsesCurrentFileHash(t *testing.T) {
	appRoot := t.TempDir()
	packagePath := filepath.Join(appRoot, "extension.crx")
	packageData := []byte("Cr24-current-package")
	if err := os.WriteFile(packagePath, packageData, 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	manager := NewManager(config.DefaultConfig(), appRoot)
	_, packageHash, err := manager.resolveExtensionPackage(Extension{
		PackagePath: packagePath,
		PackageHash: "stale-hash-from-database",
	}, "")
	if err != nil {
		t.Fatalf("resolveExtensionPackage returned error: %v", err)
	}
	if packageHash != extensionPackageHash(packageData) {
		t.Fatalf("package hash = %q, want current file hash %q", packageHash, extensionPackageHash(packageData))
	}
}

func TestBackupProfileExtensionStateIncludesIndexedDBAndServiceWorker(t *testing.T) {
	appRoot := t.TempDir()
	userDataDir := filepath.Join(appRoot, "profile")
	runtimeID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	indexedDBPath := filepath.Join(userDataDir, "Default", "IndexedDB", "https_example_"+runtimeID+"_0.indexeddb.leveldb")
	serviceWorkerPath := filepath.Join(userDataDir, "Default", "Service Worker", "Database", runtimeID+"-worker")
	for _, path := range []string{indexedDBPath, serviceWorkerPath} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll returned error: %v", err)
		}
		if err := os.WriteFile(path, []byte(path), 0o644); err != nil {
			t.Fatalf("WriteFile returned error: %v", err)
		}
	}

	manager := NewManager(config.DefaultConfig(), appRoot)
	backupPath, err := manager.backupProfileExtensionState("profile", runtimeID, userDataDir, []string{runtimeID})
	if err != nil {
		t.Fatalf("backupProfileExtensionState returned error: %v", err)
	}
	for _, relativePath := range []string{
		filepath.Join("IndexedDB", filepath.Base(indexedDBPath)),
		filepath.Join("Service Worker", "Database", filepath.Base(serviceWorkerPath)),
	} {
		if _, err := os.Stat(filepath.Join(backupPath, relativePath)); err != nil {
			t.Fatalf("backup entry %q not found: %v", relativePath, err)
		}
	}

	if err := os.RemoveAll(filepath.Join(userDataDir, "Default", "IndexedDB")); err != nil {
		t.Fatalf("RemoveAll IndexedDB returned error: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(userDataDir, "Default", "Service Worker")); err != nil {
		t.Fatalf("RemoveAll Service Worker returned error: %v", err)
	}
	if err := restoreProfileExtensionState(userDataDir, backupPath, []string{runtimeID}, ""); err != nil {
		t.Fatalf("restoreProfileExtensionState returned error: %v", err)
	}
	for _, path := range []string{indexedDBPath, serviceWorkerPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("restored entry %q not found: %v", path, err)
		}
	}
}

func TestRemoveExtensionFromStoppedProfilesRemovesPersistentCodeWithoutRuntimeState(t *testing.T) {
	appRoot := t.TempDir()
	userDataDir := filepath.Join(appRoot, "profile")
	runtimeID := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	codeDir := filepath.Join(userDataDir, "Default", "Extensions", runtimeID)
	if err := os.MkdirAll(codeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	manager := NewManager(config.DefaultConfig(), appRoot)
	manager.Profiles["profile"] = &Profile{
		ProfileId:   "profile",
		ProfileName: "profile",
		UserDataDir: userDataDir,
		Running:     false,
	}
	manager.ExtensionDAO = newTestExtensionDAO(t, appRoot)
	if err := manager.ExtensionDAO.Upsert(Extension{
		ExtensionID: runtimeID,
		Name:        "Persistent test",
		Version:     "1.0.0",
		InstallMode: ExtensionInstallModePersistent,
		Enabled:     false,
	}); err != nil {
		t.Fatalf("Upsert returned error: %v", err)
	}

	if err := manager.RemoveExtensionFromStoppedProfiles(runtimeID); err != nil {
		t.Fatalf("RemoveExtensionFromStoppedProfiles returned error: %v", err)
	}
	if _, err := os.Stat(codeDir); !os.IsNotExist(err) {
		t.Fatalf("persistent code directory still exists, stat error = %v", err)
	}
}

func TestExtensionIDFromCRX2PublicKey(t *testing.T) {
	publicKey := []byte("test-public-key")
	data := make([]byte, 16+len(publicKey)+4)
	copy(data[:4], []byte("Cr24"))
	binary.LittleEndian.PutUint32(data[4:8], 2)
	binary.LittleEndian.PutUint32(data[8:12], uint32(len(publicKey)))
	binary.LittleEndian.PutUint32(data[12:16], 4)
	copy(data[16:], publicKey)
	if extensionIDFromCRX(data) == "" {
		t.Fatal("extensionIDFromCRX returned empty ID for valid CRX2 header")
	}
}

func TestExtensionIDFromCRX3DeclaredID(t *testing.T) {
	rawID, err := hex.DecodeString("8d5ff66de04dc50615ad5a0d2f1b9220")
	if err != nil {
		t.Fatalf("DecodeString returned error: %v", err)
	}
	signedHeaderData := appendCRX3LengthDelimitedField(nil, 1, rawID)
	header := appendCRX3LengthDelimitedField(nil, 10000, signedHeaderData)
	data := make([]byte, 12+len(header))
	copy(data[:4], []byte("Cr24"))
	binary.LittleEndian.PutUint32(data[4:8], 3)
	binary.LittleEndian.PutUint32(data[8:12], uint32(len(header)))
	copy(data[12:], header)

	const want = "infppggnoaenmfagbfknfkancpbljcca"
	if got := extensionIDFromCRX(data); got != want {
		t.Fatalf("extensionIDFromCRX = %q, want %q", got, want)
	}
}

func appendCRX3LengthDelimitedField(data []byte, fieldNumber uint64, value []byte) []byte {
	data = appendCRX3Varint(data, fieldNumber<<3|2)
	data = appendCRX3Varint(data, uint64(len(value)))
	return append(data, value...)
}

func appendCRX3Varint(data []byte, value uint64) []byte {
	for value >= 0x80 {
		data = append(data, byte(value)|0x80)
		value >>= 7
	}
	return append(data, byte(value))
}

func TestLegacyDirectorySourceRemainsCommandline(t *testing.T) {
	appRoot := t.TempDir()
	directorySource := filepath.Join(appRoot, "source-extension")
	if err := os.MkdirAll(directorySource, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	manager := NewManager(config.DefaultConfig(), appRoot)
	if manager == nil {
		t.Fatal("NewManager returned nil")
	}

	dao := newTestExtensionDAO(t, appRoot)
	manager.ExtensionDAO = dao
	legacy := Extension{
		ExtensionID: "cccccccccccccccccccccccccccccccc",
		Name:        "legacy directory",
		Version:     "1.0.0",
		SourceURL:   directorySource,
		InstallDir:  filepath.Join(appRoot, "stored-extension"),
		Enabled:     true,
	}
	if err := dao.Upsert(legacy); err != nil {
		t.Fatalf("Upsert returned error: %v", err)
	}
	stored, err := dao.Get(legacy.ExtensionID)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if stored.InstallMode != ExtensionInstallModeCommandline {
		t.Fatalf("legacy InstallMode = %q, want commandline", stored.InstallMode)
	}
}

func newTestExtensionDAO(t *testing.T, appRoot string) *SQLiteExtensionDAO {
	t.Helper()
	db, err := database.NewDB(filepath.Join(appRoot, "extensions.db"))
	if err != nil {
		t.Fatalf("NewDB returned error: %v", err)
	}
	if err := db.Migrate(); err != nil {
		db.Close()
		t.Fatalf("Migrate returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewSQLiteExtensionDAO(db.GetConn())
}
