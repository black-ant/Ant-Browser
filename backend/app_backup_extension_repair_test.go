package backend

import (
	"ant-chrome/backend/internal/database"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

const (
	backupTestExtensionID  = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	backupTestExtensionIDB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	backupTestProfileID    = "profile-a"
	backupTestPackageData  = "Cr24-test-package"
	backupTestTimestamp    = "20260809-000000"
)

func TestBackupRepairExtensionPathsAfterImportRelocatesManagedArtifacts(t *testing.T) {
	app := newBackupExtensionRepairTestApp(t)
	oldRoot := t.TempDir()
	oldInstallDir := filepath.Join(oldRoot, "data", "extensions", backupTestExtensionID)
	oldPackagePath := filepath.Join(oldRoot, "data", "extensions", "packages", backupTestExtensionID+".crx")
	oldBackupPath := filepath.Join(oldRoot, "data", "extension-backups", backupTestProfileID, backupTestExtensionID, backupTestTimestamp)
	packageData := []byte(backupTestPackageData)
	packageHash := sha256Hex(packageData)

	insertBackupTestExtension(t, app, backupTestExtensionID, oldInstallDir, oldPackagePath, "old-hash")
	insertBackupTestRuntime(t, app, backupTestExtensionID, oldBackupPath, "old-runtime-hash")
	payloadRoot := writeBackupExtensionPayload(t, t.TempDir(), backupTestExtensionID, packageData, true, oldBackupPath)

	stats := &backupMergeStats{}
	app.backupImportFileTrees(payloadRoot, nil, false, stats, nil)
	if err := app.backupRepairExtensionPathsAfterImport(); err != nil {
		t.Fatalf("backupRepairExtensionPathsAfterImport returned error: %v", err)
	}

	newInstallDir := filepath.Join(app.appRoot, "data", "extensions", backupTestExtensionID)
	newPackagePath := filepath.Join(app.appRoot, "data", "extensions", "packages", backupTestExtensionID+".crx")
	newBackupPath := filepath.Join(app.appRoot, "data", "extension-backups", backupTestProfileID, backupTestExtensionID, backupTestTimestamp)
	gotInstallDir, gotPackagePath, gotPackageHash := readBackupTestExtension(t, app, backupTestExtensionID)
	if !backupSamePath(gotInstallDir, newInstallDir) {
		t.Fatalf("install_dir = %q, want %q", gotInstallDir, newInstallDir)
	}
	if !backupSamePath(gotPackagePath, newPackagePath) {
		t.Fatalf("package_path = %q, want %q", gotPackagePath, newPackagePath)
	}
	if gotPackageHash != packageHash {
		t.Fatalf("package_hash = %q, want %q", gotPackageHash, packageHash)
	}

	var gotBackupPath string
	if err := app.db.GetConn().QueryRow(`
		SELECT backup_path
		FROM browser_profile_extension_runtime
		WHERE profile_id = ? AND extension_id = ?`, backupTestProfileID, backupTestExtensionID).Scan(&gotBackupPath); err != nil {
		t.Fatalf("read runtime backup_path returned error: %v", err)
	}
	if !backupSamePath(gotBackupPath, newBackupPath) {
		t.Fatalf("runtime backup_path = %q, want %q", gotBackupPath, newBackupPath)
	}
	if !backupExtensionManifestExists(newInstallDir) {
		t.Fatalf("relocated plugin manifest does not exist at %q", newInstallDir)
	}
	if !backupExtensionRegularFileExists(newPackagePath) {
		t.Fatalf("relocated plugin package does not exist at %q", newPackagePath)
	}
}

func TestBackupRepairExtensionPathsClearsMissingManagedPackage(t *testing.T) {
	app := newBackupExtensionRepairTestApp(t)
	oldRoot := t.TempDir()
	oldInstallDir := filepath.Join(oldRoot, "data", "extensions", backupTestExtensionID)
	oldPackagePath := filepath.Join(oldRoot, "data", "extensions", "packages", backupTestExtensionID+".crx")
	payloadRoot := filepath.Join(t.TempDir(), "payload")

	insertBackupTestExtension(t, app, backupTestExtensionID, oldInstallDir, oldPackagePath, "old-hash")
	writeBackupExtensionPayload(t, payloadRoot, backupTestExtensionID, nil, false, "")

	stats := &backupMergeStats{}
	app.backupImportFileTrees(payloadRoot, nil, false, stats, nil)
	if err := app.backupRepairExtensionPathsAfterImport(); err != nil {
		t.Fatalf("backupRepairExtensionPathsAfterImport returned error: %v", err)
	}

	gotInstallDir, gotPackagePath, gotPackageHash := readBackupTestExtension(t, app, backupTestExtensionID)
	wantInstallDir := filepath.Join(app.appRoot, "data", "extensions", backupTestExtensionID)
	if !backupSamePath(gotInstallDir, wantInstallDir) {
		t.Fatalf("install_dir = %q, want %q", gotInstallDir, wantInstallDir)
	}
	if gotPackagePath != "" || gotPackageHash != "" {
		t.Fatalf("missing package metadata = (%q, %q), want empty values", gotPackagePath, gotPackageHash)
	}
}

func TestBackupRepairExtensionPathsClearsMissingManagedDirectory(t *testing.T) {
	app := newBackupExtensionRepairTestApp(t)
	oldRoot := t.TempDir()
	oldInstallDir := filepath.Join(oldRoot, "data", "extensions", backupTestExtensionID)
	payloadRoot := filepath.Join(t.TempDir(), "payload")
	if err := os.MkdirAll(filepath.Join(payloadRoot, "app", "data"), 0o755); err != nil {
		t.Fatalf("MkdirAll payload data returned error: %v", err)
	}

	insertBackupTestExtension(t, app, backupTestExtensionID, oldInstallDir, "", "")
	stats := &backupMergeStats{}
	app.backupImportFileTrees(payloadRoot, nil, false, stats, nil)
	if err := app.backupRepairExtensionPathsAfterImport(); err != nil {
		t.Fatalf("backupRepairExtensionPathsAfterImport returned error: %v", err)
	}

	gotInstallDir, _, _ := readBackupTestExtension(t, app, backupTestExtensionID)
	if gotInstallDir != "" {
		t.Fatalf("missing managed install_dir = %q, want empty value", gotInstallDir)
	}
}

func TestBackupRepairExtensionPathsRollsBackAllUpdatesOnFailure(t *testing.T) {
	app := newBackupExtensionRepairTestApp(t)
	oldRoot := t.TempDir()
	payloadRoot := t.TempDir()
	oldInstallDirA := filepath.Join(oldRoot, "data", "extensions", backupTestExtensionID)
	oldInstallDirB := filepath.Join(oldRoot, "data", "extensions", backupTestExtensionIDB)
	insertBackupTestExtension(t, app, backupTestExtensionID, oldInstallDirA, "", "")
	insertBackupTestExtension(t, app, backupTestExtensionIDB, oldInstallDirB, "", "")
	writeBackupExtensionPayload(t, payloadRoot, backupTestExtensionID, nil, false, "")
	writeBackupExtensionPayload(t, payloadRoot, backupTestExtensionIDB, nil, false, "")

	triggerSQL := fmt.Sprintf(`
		CREATE TRIGGER backup_extension_path_repair_failure
		BEFORE UPDATE OF install_dir ON browser_extensions
		WHEN OLD.extension_id = '%s'
		BEGIN
			SELECT RAISE(ABORT, 'forced extension path repair failure');
		END`, backupTestExtensionIDB)
	if _, err := app.db.GetConn().Exec(triggerSQL); err != nil {
		t.Fatalf("create rollback trigger returned error: %v", err)
	}

	stats := &backupMergeStats{}
	app.backupImportFileTrees(payloadRoot, nil, false, stats, nil)
	if err := app.backupRepairExtensionPathsAfterImport(); err == nil {
		t.Fatal("backupRepairExtensionPathsAfterImport returned nil, want forced failure")
	}

	gotInstallDirA, _, _ := readBackupTestExtension(t, app, backupTestExtensionID)
	gotInstallDirB, _, _ := readBackupTestExtension(t, app, backupTestExtensionIDB)
	if !backupSamePath(gotInstallDirA, oldInstallDirA) || !backupSamePath(gotInstallDirB, oldInstallDirB) {
		t.Fatalf("transaction did not roll back: install dirs = (%q, %q)", gotInstallDirA, gotInstallDirB)
	}
}

func newBackupExtensionRepairTestApp(t *testing.T) *App {
	t.Helper()
	appRoot := t.TempDir()
	dataRoot := filepath.Join(appRoot, "data")
	if err := os.MkdirAll(dataRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll data root returned error: %v", err)
	}
	db, err := database.NewDB(filepath.Join(dataRoot, "app.db"))
	if err != nil {
		t.Fatalf("NewDB returned error: %v", err)
	}
	if err := db.Migrate(); err != nil {
		_ = db.Close()
		t.Fatalf("Migrate returned error: %v", err)
	}
	app := NewApp(appRoot)
	app.db = db
	t.Cleanup(func() { _ = db.Close() })
	return app
}

func insertBackupTestExtension(t *testing.T, app *App, extensionID string, installDir string, packagePath string, packageHash string) {
	t.Helper()
	_, err := app.db.GetConn().Exec(`
		INSERT INTO browser_extensions (
			extension_id, name, version, manifest_json, source_url, install_dir,
			install_mode, package_path, package_hash, enabled, default_install,
			installed_at, updated_at
		) VALUES (?, 'test', '1.0.0', '{}', '', ?, 'persistent', ?, ?, 1, 0, ?, ?)`,
		extensionID, installDir, packagePath, packageHash, backupTestTimestamp, backupTestTimestamp)
	if err != nil {
		t.Fatalf("insert extension returned error: %v", err)
	}
}

func insertBackupTestRuntime(t *testing.T, app *App, extensionID string, backupPath string, packageHash string) {
	t.Helper()
	_, err := app.db.GetConn().Exec(`
		INSERT INTO browser_profile_extension_runtime (
			profile_id, extension_id, runtime_extension_id, install_mode,
			installed_version, package_hash, status, backup_path,
			last_verified_at, last_error, created_at, updated_at
		) VALUES (?, ?, ?, 'persistent', '1.0.0', ?, 'installed', ?, '', '', ?, ?)`,
		backupTestProfileID, extensionID, extensionID, packageHash, backupPath, backupTestTimestamp, backupTestTimestamp)
	if err != nil {
		t.Fatalf("insert runtime returned error: %v", err)
	}
}

func writeBackupExtensionPayload(t *testing.T, payloadRoot string, extensionID string, packageData []byte, includePackage bool, backupPath string) string {
	t.Helper()
	extensionDir := filepath.Join(payloadRoot, "app", "data", "extensions", extensionID)
	if err := os.MkdirAll(extensionDir, 0o755); err != nil {
		t.Fatalf("MkdirAll extension payload returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(extensionDir, "manifest.json"), []byte(`{"name":"Test","version":"1.0.0"}`), 0o644); err != nil {
		t.Fatalf("WriteFile manifest returned error: %v", err)
	}
	if includePackage {
		packageDir := filepath.Join(payloadRoot, "app", "data", "extensions", "packages")
		if err := os.MkdirAll(packageDir, 0o755); err != nil {
			t.Fatalf("MkdirAll package payload returned error: %v", err)
		}
		if err := os.WriteFile(filepath.Join(packageDir, extensionID+".crx"), packageData, 0o644); err != nil {
			t.Fatalf("WriteFile package returned error: %v", err)
		}
	}
	if backupPath != "" {
		backupSuffix := filepath.Base(backupPath)
		backupDir := filepath.Join(payloadRoot, "app", "data", "extension-backups", backupTestProfileID, extensionID, backupSuffix)
		if err := os.MkdirAll(backupDir, 0o755); err != nil {
			t.Fatalf("MkdirAll backup payload returned error: %v", err)
		}
		if err := os.WriteFile(filepath.Join(backupDir, "state.bin"), []byte("state"), 0o644); err != nil {
			t.Fatalf("WriteFile backup state returned error: %v", err)
		}
	}
	return payloadRoot
}

func readBackupTestExtension(t *testing.T, app *App, extensionID string) (string, string, string) {
	t.Helper()
	var installDir string
	var packagePath string
	var packageHash string
	if err := app.db.GetConn().QueryRow(`
		SELECT install_dir, package_path, package_hash
		FROM browser_extensions WHERE extension_id = ?`, extensionID).Scan(&installDir, &packagePath, &packageHash); err != nil {
		t.Fatalf("read extension returned error: %v", err)
	}
	return installDir, packagePath, packageHash
}

func sha256Hex(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}
