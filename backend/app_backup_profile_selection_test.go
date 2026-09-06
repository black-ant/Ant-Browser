package backend

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/database"
)

func TestBackupProfileIDsFromInput(t *testing.T) {
	ids, err := backupProfileIDsFromInput(map[string]string{
		"profileIds": `["profile-b", "profile-a", "profile-b", ""]`,
	})
	if err != nil {
		t.Fatalf("backupProfileIDsFromInput returned error: %v", err)
	}
	if want := []string{"profile-a", "profile-b"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("profile IDs = %#v, want %#v", ids, want)
	}

	if _, err := backupProfileIDsFromInput(map[string]string{"profileIds": `{"profileId":"profile-a"}`}); err == nil {
		t.Fatal("expected invalid profile IDs to return an error")
	}
}

func TestBackupProfilePackageFileName(t *testing.T) {
	now := time.Date(2026, time.September, 1, 17, 36, 16, 0, time.UTC)
	if got := backupProfilePackageFileName([]string{"测试账号 A"}, now, false); got != "ant-chrome-profile-backup-single--测试账号 A--20260901-173616.zip" {
		t.Fatalf("single profile backup name = %q", got)
	}
	if got := backupProfilePackageFileName([]string{"实例 A", "实例 B"}, now, true); got != "ant-chrome-profile-backup-multi-2--20260901-173616.000000000.zip" {
		t.Fatalf("multi profile backup name = %q", got)
	}
}

func TestDetectBackupPackageFormat(t *testing.T) {
	for _, format := range []string{"ant-chrome-full-backup", profilePackageFormat} {
		t.Run(format, func(t *testing.T) {
			zipPath := filepath.Join(t.TempDir(), "backup.zip")
			file, err := os.Create(zipPath)
			if err != nil {
				t.Fatalf("create zip failed: %v", err)
			}
			writer := zip.NewWriter(file)
			manifest, err := writer.Create("manifest.json")
			if err != nil {
				_ = file.Close()
				t.Fatalf("create manifest failed: %v", err)
			}
			if err := json.NewEncoder(manifest).Encode(map[string]string{"format": format}); err != nil {
				_ = writer.Close()
				_ = file.Close()
				t.Fatalf("write manifest failed: %v", err)
			}
			if err := writer.Close(); err != nil {
				_ = file.Close()
				t.Fatalf("close zip failed: %v", err)
			}
			if err := file.Close(); err != nil {
				t.Fatalf("close file failed: %v", err)
			}

			got, err := detectBackupPackageFormat(zipPath)
			if err != nil {
				t.Fatalf("detectBackupPackageFormat returned error: %v", err)
			}
			if got != format {
				t.Fatalf("package format = %q, want %q", got, format)
			}
		})
	}
}

func TestInspectBackupPackageUsesInternalManifestInsteadOfFileName(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "ant-chrome-backup-full-looking-name.zip")
	writeTestProfilePackage(t, zipPath, []browser.Profile{{ProfileId: "profile-a", ProfileName: "实例 A"}}, nil)

	info, err := inspectBackupPackageInfo(zipPath)
	if err != nil {
		t.Fatalf("inspectBackupPackageInfo returned error: %v", err)
	}
	if info.PackageType != "profile" || info.ProfileCount != 1 || len(info.ProfileNames) != 1 || info.ProfileNames[0] != "实例 A" {
		t.Fatalf("package info = %#v, want profile manifest metadata", info)
	}
}

func TestBackupExportProfilePackageToPath(t *testing.T) {
	root := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Browser.UserDataRoot = filepath.Join(root, "user-data")
	app := NewApp(root)
	app.config = cfg
	if err := os.MkdirAll(filepath.Join(root, "data"), 0o755); err != nil {
		t.Fatalf("create test database directory failed: %v", err)
	}
	db, err := database.NewDB(filepath.Join(root, "data", "app.db"))
	if err != nil {
		t.Fatalf("create test database failed: %v", err)
	}
	if err := db.Migrate(); err != nil {
		_ = db.Close()
		t.Fatalf("migrate test database failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	app.db = db
	app.browserMgr = browser.NewManager(cfg, root)
	app.browserMgr.Profiles["profile-a"] = &browser.Profile{
		ProfileId:   "profile-a",
		ProfileName: "实例 A",
		UserDataDir: "profile-a",
	}

	userDataDir := filepath.Join(cfg.Browser.UserDataRoot, "profile-a")
	if err := os.MkdirAll(userDataDir, 0o755); err != nil {
		t.Fatalf("create user-data directory failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(userDataDir, "Preferences"), []byte(`{"homepage":"https://example.com"}`), 0o644); err != nil {
		t.Fatalf("write user-data file failed: %v", err)
	}

	zipPath := filepath.Join(root, "profile-backup.zip")
	result, err := app.backupExportProfilePackageToPath(zipPath, []string{"profile-a"})
	if err != nil {
		t.Fatalf("backupExportProfilePackageToPath returned error: %v", err)
	}
	if result["packageType"] != "profile" || result["profileCount"] != 1 {
		t.Fatalf("unexpected export result: %#v", result)
	}
	packageInfo, err := inspectBackupPackageInfo(zipPath)
	if err != nil {
		t.Fatalf("inspect exported profile package failed: %v", err)
	}
	if packageInfo.PackageType != "profile" || packageInfo.ProfileCount != 1 || !reflect.DeepEqual(packageInfo.ProfileNames, []string{"实例 A"}) {
		t.Fatalf("unexpected exported package info: %#v", packageInfo)
	}
	fileInfo, err := app.GetBackupFileInfo(zipPath)
	if err != nil {
		t.Fatalf("read exported profile file info failed: %v", err)
	}
	if fileInfo["packageType"] != "profile" || fileInfo["profileCount"] != 1 {
		t.Fatalf("unexpected exported file info: %#v", fileInfo)
	}

	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open exported profile package failed: %v", err)
	}
	defer reader.Close()
	entries := make(map[string]bool, len(reader.File))
	for _, entry := range reader.File {
		entries[filepath.ToSlash(entry.Name)] = true
	}
	for _, name := range []string{"manifest.json", "profiles.json", "database.json", "user-data/profile-a/Preferences"} {
		if !entries[name] {
			t.Fatalf("exported package is missing %s", name)
		}
	}
}

func TestBackupRestorePackageFromPathRoutesProfilePackage(t *testing.T) {
	app, zipPath := newProfilePackageImportTestApp(t, []browser.Profile{
		{ProfileId: "source-1", ProfileName: "源实例", UserDataDir: "source-1"},
	}, map[string]string{"source-1/Default/Preferences": "{}"})

	result, err := app.backupRestorePackageFromPathLocked(zipPath)
	if err != nil {
		t.Fatalf("backupRestorePackageFromPathLocked returned error: %v", err)
	}
	if result["packageType"] != "profile" || result["requiresProfileImportConfirmation"] != true {
		t.Fatalf("unexpected restore result: %#v", result)
	}
	preview, ok := result["profileImportPreview"].(ProfilePackageImportPreview)
	if !ok || preview.ProfileCount != 1 || len(preview.Profiles) != 1 || preview.Profiles[0].SuggestedProfileName != "源实例" {
		t.Fatalf("unexpected restore preview: %#v", result["profileImportPreview"])
	}
	imported, err := app.importProfilePackageFromPathWithModeAndActions(zipPath, profilePackageImportModeNew, true, []ProfilePackageImportAction{{
		SourceProfileID: "source-1",
		SourceIndex:     0,
		Mode:            profilePackageImportModeNew,
	}})
	if err != nil || imported.ImportedCount != 1 || imported.CreatedCount != 1 {
		t.Fatalf("confirmed restore failed: result=%#v err=%v", imported, err)
	}
}

func TestBackupRestoreProfilePackageRequiresConflictChoice(t *testing.T) {
	root := t.TempDir()
	db := newProfilePackageDatabase(t, root)
	cfg := config.DefaultConfig()
	cfg.Browser.UserDataRoot = filepath.Join(root, "user-data")
	app := NewApp(root)
	app.config = cfg
	app.db = db
	app.browserMgr = browser.NewManager(cfg, root)
	app.browserMgr.ProfileDAO = browser.NewSQLiteProfileDAO(db.GetConn())

	source := browser.Profile{
		ProfileId:   "source-1",
		ProfileName: "CPA",
		UserDataDir: "source-1",
	}
	zipPath := filepath.Join(root, "profile-conflict.zip")
	writeTestProfilePackage(t, zipPath, []browser.Profile{source}, nil)

	target := browser.Profile{
		ProfileId:   "target-1",
		ProfileName: "CPA",
		UserDataDir: "target-data",
		CreatedAt:   "2026-09-01T00:00:00Z",
	}
	if _, err := db.GetConn().Exec(`INSERT INTO browser_profiles (profile_id, profile_name, user_data_dir, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, target.ProfileId, target.ProfileName, target.UserDataDir, target.CreatedAt, target.CreatedAt); err != nil {
		t.Fatalf("insert target profile failed: %v", err)
	}
	targetDir := filepath.Join(cfg.Browser.UserDataRoot, target.UserDataDir)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("create target user data failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "old.txt"), []byte("old-data"), 0o644); err != nil {
		t.Fatalf("write target user data failed: %v", err)
	}
	app.browserMgr.InitData()

	pending, err := app.backupRestorePackageFromPathLocked(zipPath)
	if err != nil {
		t.Fatalf("backupRestorePackageFromPathLocked returned error: %v", err)
	}
	if pending["requiresProfileImportConfirmation"] != true {
		t.Fatalf("conflicting restore must require a choice: %#v", pending)
	}
	preview, ok := pending["profileImportPreview"].(ProfilePackageImportPreview)
	if !ok || preview.ConflictCount != 1 || preview.ZipPath != zipPath {
		t.Fatalf("unexpected conflict preview: %#v", pending["profileImportPreview"])
	}
	app.browserMgr.Mutex.Lock()
	for _, profile := range app.browserMgr.Profiles {
		if profile != nil && profile.ProfileName == "CPA（导入）" {
			t.Fatalf("restore created a suffixed profile before a choice: %#v", profile)
		}
	}
	app.browserMgr.Mutex.Unlock()

	created, err := app.importProfilePackageFromPathWithModeAndConfirmation(zipPath, profilePackageImportModeNew, true)
	if err != nil {
		t.Fatalf("confirmed new import returned error: %v", err)
	}
	createdID := created.ProfileMappings[source.ProfileId]
	if createdID == "" || created.CreatedCount != 1 || created.OverwrittenCount != 0 {
		t.Fatalf("confirmed new import returned unexpected result: %#v", created)
	}
	createdProfile, err := app.browserMgr.ProfileDAO.GetById(createdID)
	if err != nil {
		t.Fatalf("read created profile failed: %v", err)
	}
	if createdProfile.ProfileName != "CPA（导入）" || createdProfile.DeletedAt != "" {
		t.Fatalf("new choice did not create the expected active profile: %#v", createdProfile)
	}

	overwritten, err := app.importProfilePackageFromPathWithModeAndConfirmation(zipPath, profilePackageImportModeOverwrite, true)
	if err != nil {
		t.Fatalf("confirmed overwrite import returned error: %v", err)
	}
	overwrittenID := overwritten.ProfileMappings[source.ProfileId]
	if overwrittenID == "" || overwrittenID == createdID || overwritten.CreatedCount != 0 || overwritten.OverwrittenCount != 1 {
		t.Fatalf("confirmed overwrite returned unexpected result: %#v", overwritten)
	}
	active, err := app.browserMgr.ProfileDAO.GetById(overwrittenID)
	if err != nil {
		t.Fatalf("read overwritten profile failed: %v", err)
	}
	if active.ProfileName != "CPA" || active.DeletedAt != "" {
		t.Fatalf("overwrite did not create the expected active profile: %#v", active)
	}
	deleted, err := app.browserMgr.ProfileDAO.GetById(target.ProfileId)
	if err != nil {
		t.Fatalf("read trashed target failed: %v", err)
	}
	if deleted.DeletedAt == "" || deleted.ProfileName != target.ProfileName || deleted.UserDataDir != target.UserDataDir {
		t.Fatalf("overwrite did not preserve the old profile in trash: %#v", deleted)
	}
}
