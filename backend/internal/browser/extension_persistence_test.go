package browser

import (
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/database"
	"archive/zip"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
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
	if got := persistentExtensionArtifactPath(userDataDir, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "1.2.3"); got != manifestDir {
		t.Fatalf("persistentExtensionArtifactPath = %q, want %q", got, manifestDir)
	}
}

func TestPersistentExtensionCodePathUsesChromiumVersionDirectory(t *testing.T) {
	userDataDir := t.TempDir()
	want := filepath.Join(userDataDir, "Default", "Extensions", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "1.2.3_0")
	if got := persistentExtensionCodePath(userDataDir, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "1.2.3"); got != want {
		t.Fatalf("persistentExtensionCodePath = %q, want %q", got, want)
	}
}

func TestInstallExtensionPackageIntoProfileWritesProfileScopedCode(t *testing.T) {
	userDataDir := t.TempDir()
	packagePath := filepath.Join(t.TempDir(), "test.crx")
	const version = "1.2.3"
	publicKey := []byte("test-public-key")
	extensionID := extensionIDFromPublicKey(publicKey)

	var archive bytes.Buffer
	zipWriter := zip.NewWriter(&archive)
	manifestWriter, err := zipWriter.Create("manifest.json")
	if err != nil {
		t.Fatalf("Create manifest returned error: %v", err)
	}
	if _, err := manifestWriter.Write([]byte(`{"name":"Test","version":"1.2.3"}`)); err != nil {
		t.Fatalf("Write manifest returned error: %v", err)
	}
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("Close archive returned error: %v", err)
	}
	signature := []byte("test-signature")
	packageData := make([]byte, 16+len(publicKey)+len(signature))
	copy(packageData[:4], []byte("Cr24"))
	binary.LittleEndian.PutUint32(packageData[4:8], 2)
	binary.LittleEndian.PutUint32(packageData[8:12], uint32(len(publicKey)))
	binary.LittleEndian.PutUint32(packageData[12:16], uint32(len(signature)))
	copy(packageData[16:], publicKey)
	copy(packageData[16+len(publicKey):], signature)
	packageData = append(packageData, archive.Bytes()...)
	if err := os.WriteFile(packagePath, packageData, 0o644); err != nil {
		t.Fatalf("WriteFile package returned error: %v", err)
	}

	runtimeID, err := installExtensionPackageIntoProfile(userDataDir, packagePath, Extension{
		ExtensionID: extensionID,
		Name:        "Test",
		Version:     version,
	})
	if err != nil {
		t.Fatalf("installExtensionPackageIntoProfile returned error: %v", err)
	}
	if runtimeID != extensionID {
		t.Fatalf("runtime ID = %q, want %q", runtimeID, extensionID)
	}
	targetPath := persistentExtensionCodePath(userDataDir, runtimeID, version)
	manifestData, err := os.ReadFile(filepath.Join(targetPath, "manifest.json"))
	if err != nil {
		t.Fatalf("Read installed manifest returned error: %v", err)
	}
	var installedManifest map[string]any
	if err := json.Unmarshal(manifestData, &installedManifest); err != nil {
		t.Fatalf("Unmarshal installed manifest returned error: %v", err)
	}
	if installedManifest["name"] != "Test" || installedManifest["version"] != version || installedManifest["key"] == nil {
		t.Fatalf("installed manifest = %s", manifestData)
	}
	if !persistentExtensionArtifactMatches(userDataDir, runtimeID, version) {
		t.Fatal("persistentExtensionArtifactMatches = false after profile-scoped installation")
	}
}

func TestProfileExtensionRegistrationUsesChromeRelativePathAndClearsMAC(t *testing.T) {
	userDataDir := t.TempDir()
	runtimeID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	version := "1.2.3"
	codePath := persistentExtensionCodePath(userDataDir, runtimeID, version)
	if err := os.MkdirAll(codePath, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(codePath, "manifest.json"), []byte(`{"name":"Test","version":"1.2.3"}`), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	root := profileExtensionJSON{
		"extensions": profileExtensionJSON{
			"settings": profileExtensionJSON{
				runtimeID: profileExtensionJSON{
					"location": 8,
					"path":     `E:\\shared\\extension`,
				},
			},
		},
		"protection": profileExtensionJSON{
			"macs": profileExtensionJSON{
				"extensions": profileExtensionJSON{
					"settings":                profileExtensionJSON{runtimeID: "old-mac"},
					"settings_encrypted_hash": profileExtensionJSON{runtimeID: "old-hash"},
				},
			},
		},
	}
	if err := writeProfileJSON(filepath.Join(userDataDir, "Default", "Secure Preferences"), root); err != nil {
		t.Fatalf("writeProfileJSON returned error: %v", err)
	}

	if err := ensureProfileScopedExtensionRegistration(userDataDir, codePath, runtimeID, ""); err != nil {
		t.Fatalf("ensureProfileScopedExtensionRegistration returned error: %v", err)
	}
	storedRoot, err := readProfileJSON(filepath.Join(userDataDir, "Default", "Secure Preferences"), false)
	if err != nil {
		t.Fatalf("readProfileJSON returned error: %v", err)
	}
	storedSetting := storedRoot["extensions"].(map[string]any)["settings"].(map[string]any)[runtimeID].(map[string]any)
	if storedSetting["location"] != float64(1) {
		t.Fatalf("location = %#v, want 1", storedSetting["location"])
	}
	expectedPath, err := profileExtensionRelativePath(userDataDir, codePath)
	if err != nil {
		t.Fatalf("profileExtensionRelativePath returned error: %v", err)
	}
	if storedSetting["path"] != expectedPath {
		t.Fatalf("path = %#v, want %q", storedSetting["path"], expectedPath)
	}
	macs := storedRoot["protection"].(map[string]any)["macs"].(map[string]any)["extensions"].(map[string]any)
	if _, exists := macs["settings"].(map[string]any)[runtimeID]; exists {
		t.Fatal("old settings MAC was not removed")
	}
	if _, exists := macs["settings_encrypted_hash"].(map[string]any)[runtimeID]; exists {
		t.Fatal("old encrypted settings hash was not removed")
	}
	if !profileExtensionSettingMatches(userDataDir, runtimeID, version) {
		t.Fatal("profileExtensionSettingMatches = false for a valid profile registration")
	}
	storedSetting["location"] = float64(3)
	storedSetting["path"] = expectedPath
	if err := writeProfileJSON(filepath.Join(userDataDir, "Default", "Secure Preferences"), storedRoot); err != nil {
		t.Fatalf("writeProfileJSON external registration returned error: %v", err)
	}
	if !profileExtensionSettingMatches(userDataDir, runtimeID, version) {
		t.Fatal("profileExtensionSettingMatches = false for a valid external profile registration")
	}
	if err := ensureProfileExtensionRegistration(userDataDir, codePath, runtimeID, ""); err != nil {
		t.Fatalf("ensureProfileExtensionRegistration returned error: %v", err)
	}
	storedRoot, err = readProfileJSON(filepath.Join(userDataDir, "Default", "Secure Preferences"), false)
	if err != nil {
		t.Fatalf("readProfileJSON external registration returned error: %v", err)
	}
	storedSetting = storedRoot["extensions"].(map[string]any)["settings"].(map[string]any)[runtimeID].(map[string]any)
	if storedSetting["location"] != float64(3) {
		t.Fatalf("external registration location = %#v, want 3", storedSetting["location"])
	}

	storedSetting["path"] = `wrong\\path`
	if err := writeProfileJSON(filepath.Join(userDataDir, "Default", "Secure Preferences"), storedRoot); err != nil {
		t.Fatalf("writeProfileJSON wrong path returned error: %v", err)
	}
	if profileExtensionSettingMatches(userDataDir, runtimeID, version) {
		t.Fatal("profileExtensionSettingMatches = true for a wrong profile path")
	}
}

func TestCleanupProfileExtensionRuntimeKeepsWorkflowStorageAndRemovesRegistration(t *testing.T) {
	userDataDir := t.TempDir()
	runtimeID := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	codePath := persistentExtensionCodePath(userDataDir, runtimeID, "1.0.0")
	workflowPath := filepath.Join(userDataDir, "Default", "Local Extension Settings", runtimeID, "workflow.json")
	if err := os.MkdirAll(codePath, 0o755); err != nil {
		t.Fatalf("MkdirAll code returned error: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(workflowPath), 0o755); err != nil {
		t.Fatalf("MkdirAll workflow returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(codePath, "manifest.json"), []byte(`{"name":"Test","version":"1.0.0"}`), 0o644); err != nil {
		t.Fatalf("WriteFile manifest returned error: %v", err)
	}
	if err := os.WriteFile(workflowPath, []byte("workflow"), 0o644); err != nil {
		t.Fatalf("WriteFile workflow returned error: %v", err)
	}
	if err := ensureProfileScopedExtensionRegistration(userDataDir, codePath, runtimeID, ""); err != nil {
		t.Fatalf("ensureProfileScopedExtensionRegistration returned error: %v", err)
	}
	if err := cleanupProfileExtensionRuntime(userDataDir, runtimeID); err != nil {
		t.Fatalf("cleanupProfileExtensionRuntime returned error: %v", err)
	}
	if _, err := os.Stat(codePath); !os.IsNotExist(err) {
		t.Fatalf("code path still exists, stat error = %v", err)
	}
	if _, err := os.Stat(workflowPath); err != nil {
		t.Fatalf("workflow storage was removed: %v", err)
	}
	if profileExtensionSettingMatches(userDataDir, runtimeID, "1.0.0") {
		t.Fatal("profile extension registration still matches after cleanup")
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

func TestLegacyDirectorySourceUsesPersistentInstallMode(t *testing.T) {
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
	if stored.InstallMode != ExtensionInstallModePersistent {
		t.Fatalf("legacy InstallMode = %q, want persistent", stored.InstallMode)
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
