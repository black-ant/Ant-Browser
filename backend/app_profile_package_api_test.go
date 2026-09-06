package backend

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
)

func TestProfilePackageImportSkipsMissingUserDataWithWarning(t *testing.T) {
	app, zipPath := newProfilePackageImportTestApp(t, []browser.Profile{{
		ProfileId:     "source-1",
		ProfileName:   "源实例",
		UserDataDir:   "source-1",
		ProxyBindName: "missing-proxy",
	}}, nil)

	result, err := app.importProfilePackageFromPath(zipPath)
	if err != nil {
		t.Fatalf("importProfilePackageFromPath returned error: %v", err)
	}
	if result.ImportedCount != 1 {
		t.Fatalf("imported count = %d, want 1", result.ImportedCount)
	}
	if len(result.Warnings) != 2 {
		t.Fatalf("warnings = %#v, want proxy and missing user-data warnings", result.Warnings)
	}
	joinedWarnings := strings.Join(result.Warnings, "\n")
	if !strings.Contains(joinedWarnings, "missing-proxy") || !strings.Contains(joinedWarnings, "没有用户数据目录") {
		t.Fatalf("unexpected warnings: %#v", result.Warnings)
	}
	newID := result.ProfileMappings["source-1"]
	if newID == "" {
		t.Fatalf("missing profile mapping: %#v", result.ProfileMappings)
	}
	if _, err := os.Stat(filepath.Join(app.config.Browser.UserDataRoot, newID)); !os.IsNotExist(err) {
		t.Fatalf("missing user-data import should not create final dir, stat err=%v", err)
	}
}

func TestProfilePackageImportCleansFinalDirWhenSaveFails(t *testing.T) {
	app, zipPath := newProfilePackageImportTestApp(t, []browser.Profile{{
		ProfileId:   "source-1",
		ProfileName: "源实例",
		UserDataDir: "source-1",
	}}, map[string]string{"source-1/Default/Preferences": "{}"})

	configPath := filepath.Join(app.appRoot, "config.yaml")
	if err := os.WriteFile(configPath, []byte("blocked"), 0o444); err != nil {
		t.Fatalf("prepare readonly config failed: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(configPath, 0o644) })

	_, err := app.importProfilePackageFromPath(zipPath)
	if err == nil {
		t.Fatal("expected import to fail when config save fails")
	}
	entries, readErr := os.ReadDir(app.config.Browser.UserDataRoot)
	if readErr != nil {
		t.Fatalf("read user data root failed: %v", readErr)
	}
	for _, entry := range entries {
		if entry.Name() == ".imports" {
			continue
		}
		t.Fatalf("expected final user-data dirs to be rolled back, found %s", entry.Name())
	}
}

func TestProfilePackagePrepareImportDetectsIDConflictWithoutDatabase(t *testing.T) {
	app, zipPath := newProfilePackageImportTestApp(t, []browser.Profile{{
		ProfileId:   "profile-1",
		ProfileName: "源实例",
		UserDataDir: "profile-1",
	}}, nil)
	addProfilePackageTestProfile(t, app, browser.Profile{
		ProfileId:   "profile-1",
		ProfileName: "目标实例",
		UserDataDir: "target-data",
	})

	preview, err := app.prepareProfilePackageImportFromPath(zipPath)
	if err != nil {
		t.Fatalf("prepareProfilePackageImportFromPath returned error: %v", err)
	}
	if preview.ProfileCount != 1 || preview.ConflictCount != 1 {
		t.Fatalf("unexpected preview counts: %#v", preview)
	}
	if preview.CanOverwrite {
		t.Fatal("overwrite must be disabled without database-backed trash")
	}
	conflict := preview.Conflicts[0]
	if conflict.MatchType != profilePackageImportMatchID || conflict.TargetProfileID != "profile-1" || conflict.TargetProfileName != "目标实例" {
		t.Fatalf("unexpected ID conflict: %#v", conflict)
	}
}

func TestProfilePackageImportOverwriteMovesTargetToTrash(t *testing.T) {
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
		ProfileId:   "profile-1",
		ProfileName: "新配置",
		UserDataDir: "profile-1",
	}
	zipPath := filepath.Join(root, "overwrite-profile.zip")
	writeTestProfilePackage(t, zipPath, []browser.Profile{source}, map[string]string{"profile-1/Default/Preferences": "new-data"})

	target := browser.Profile{
		ProfileId:   "profile-1",
		ProfileName: "旧配置",
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

	result, err := app.importProfilePackageFromPathWithMode(zipPath, profilePackageImportModeOverwrite)
	if err != nil {
		t.Fatalf("overwrite import returned error: %v", err)
	}
	if result.ImportedCount != 1 || result.CreatedCount != 0 || result.OverwrittenCount != 1 {
		t.Fatalf("unexpected overwrite result: %#v", result)
	}
	newID := result.ProfileMappings[source.ProfileId]
	if newID == "" || newID == target.ProfileId {
		t.Fatalf("overwrite must create a new profile ID: %#v", result.ProfileMappings)
	}

	active, err := app.browserMgr.ProfileDAO.GetById(newID)
	if err != nil {
		t.Fatalf("read imported profile failed: %v", err)
	}
	if active.DeletedAt != "" || active.ProfileName != source.ProfileName || active.UserDataDir != newID {
		t.Fatalf("imported active profile fields are incorrect: %#v", active)
	}
	deleted, err := app.browserMgr.ProfileDAO.GetById(target.ProfileId)
	if err != nil {
		t.Fatalf("read trashed target profile failed: %v", err)
	}
	if deleted.DeletedAt == "" || deleted.ProfileName != target.ProfileName || deleted.UserDataDir != target.UserDataDir {
		t.Fatalf("target profile was not preserved in trash: %#v", deleted)
	}
	trash, err := app.browserMgr.ProfileDAO.ListDeleted()
	if err != nil {
		t.Fatalf("list trashed profiles failed: %v", err)
	}
	if len(trash) != 1 || trash[0].ProfileId != target.ProfileId {
		t.Fatalf("unexpected trash contents: %#v", trash)
	}
	if content, err := os.ReadFile(filepath.Join(targetDir, "old.txt")); err != nil || string(content) != "old-data" {
		t.Fatalf("old user data backup was not preserved: content=%q err=%v", content, err)
	}
	if content, err := os.ReadFile(filepath.Join(cfg.Browser.UserDataRoot, newID, "Default", "Preferences")); err != nil || string(content) != "new-data" {
		t.Fatalf("new user data missing or incorrect: content=%q err=%v", content, err)
	}
}

func TestProfilePackageImportRepeatedOverwriteIgnoresTrashedTargets(t *testing.T) {
	root := t.TempDir()
	db := newProfilePackageDatabase(t, root)
	cfg := config.DefaultConfig()
	cfg.Browser.UserDataRoot = filepath.Join(root, "user-data")
	app := NewApp(root)
	app.config = cfg
	app.db = db
	app.browserMgr = browser.NewManager(cfg, root)
	app.browserMgr.ProfileDAO = browser.NewSQLiteProfileDAO(db.GetConn())

	zipPath := filepath.Join(root, "repeated-overwrite.zip")
	writeTestProfilePackage(t, zipPath, []browser.Profile{{
		ProfileId:   "source-1",
		ProfileName: "CPA",
		UserDataDir: "source-1",
	}}, nil)

	first, err := app.importProfilePackageFromPathWithMode(zipPath, profilePackageImportModeNew)
	if err != nil {
		t.Fatalf("first import returned error: %v", err)
	}
	firstID := first.ProfileMappings["source-1"]
	if firstID == "" {
		t.Fatalf("first import did not create a profile: %#v", first.ProfileMappings)
	}

	second, err := app.importProfilePackageFromPathWithMode(zipPath, profilePackageImportModeOverwrite)
	if err != nil {
		t.Fatalf("second import returned error: %v", err)
	}
	secondID := second.ProfileMappings["source-1"]
	if secondID == "" || secondID == firstID || second.OverwrittenCount != 1 {
		t.Fatalf("second import did not replace the active profile: %#v", second)
	}

	preview, err := app.prepareProfilePackageImportFromPath(zipPath)
	if err != nil {
		t.Fatalf("repeated import preview returned error: %v", err)
	}
	if preview.ConflictCount != 1 || !preview.CanOverwrite {
		t.Fatalf("trashed targets must not make the repeated import ambiguous: %#v", preview)
	}
	conflict := preview.Conflicts[0]
	if conflict.TargetProfileID != secondID || conflict.TargetMatches != 1 || conflict.Ambiguous {
		t.Fatalf("repeated import matched the wrong targets: %#v", conflict)
	}
	trash, err := app.browserMgr.ProfileDAO.ListDeleted()
	if err != nil {
		t.Fatalf("list trashed profiles failed: %v", err)
	}
	if len(trash) != 1 || trash[0].ProfileId != firstID {
		t.Fatalf("unexpected trashed profiles after repeated overwrite: %#v", trash)
	}
}

func TestProfilePackageImportNewModeRequiresConflictConfirmation(t *testing.T) {
	app, zipPath := newProfilePackageImportTestApp(t, []browser.Profile{
		{ProfileId: "target-1", ProfileName: "CPA", UserDataDir: "target-1"},
	}, nil)
	addProfilePackageTestProfile(t, app, browser.Profile{ProfileId: "target-1", ProfileName: "CPA", UserDataDir: "target-1"})

	if _, err := app.importProfilePackageFromPathWithModeAndConfirmation(zipPath, profilePackageImportModeNew, false); err == nil || !strings.Contains(err.Error(), "同名或 ID 冲突") {
		t.Fatalf("expected conflict confirmation error, got %v", err)
	}
	if _, err := app.importProfilePackageFromPathWithModeAndConfirmation(zipPath, profilePackageImportModeOverwrite, false); err == nil || !strings.Contains(err.Error(), "同名或 ID 冲突") {
		t.Fatalf("unconfirmed overwrite must also require a conflict choice, got %v", err)
	}

	result, err := app.importProfilePackageFromPathWithModeAndConfirmation(zipPath, profilePackageImportModeNew, true)
	if err != nil {
		t.Fatalf("confirmed new import returned error: %v", err)
	}
	newID := result.ProfileMappings["target-1"]
	if newID == "" || result.CreatedCount != 1 || result.OverwrittenCount != 0 {
		t.Fatalf("confirmed new import did not create a new profile: %#v", result)
	}
	app.browserMgr.Mutex.Lock()
	created := *app.browserMgr.Profiles[newID]
	app.browserMgr.Mutex.Unlock()
	if created.ProfileName != "CPA（导入）" {
		t.Fatalf("confirmed new import used unexpected name: %#v", created)
	}
}

func TestProfilePackageImportRejectsRunningOverwrite(t *testing.T) {
	app, zipPath := newProfilePackageImportTestApp(t, []browser.Profile{{
		ProfileId:   "profile-1",
		ProfileName: "源实例",
		UserDataDir: "profile-1",
	}}, nil)
	addProfilePackageTestProfile(t, app, browser.Profile{
		ProfileId:   "profile-1",
		ProfileName: "目标实例",
		UserDataDir: "target-data",
		Running:     true,
	})

	preview, err := app.prepareProfilePackageImportFromPath(zipPath)
	if err != nil {
		t.Fatalf("prepareProfilePackageImportFromPath returned error: %v", err)
	}
	if preview.CanOverwrite {
		t.Fatal("running target must not allow overwrite")
	}
	if _, err := app.importProfilePackageFromPathWithMode(zipPath, profilePackageImportModeOverwrite); err == nil || !strings.Contains(err.Error(), "正在运行") {
		t.Fatalf("expected running-target error, got %v", err)
	}
}

func TestProfilePackageImportRejectsAmbiguousNameOverwrite(t *testing.T) {
	app, zipPath := newProfilePackageImportTestApp(t, []browser.Profile{{
		ProfileId:   "source-1",
		ProfileName: "同名实例",
		UserDataDir: "source-1",
	}}, nil)
	addProfilePackageTestProfile(t, app, browser.Profile{ProfileId: "target-1", ProfileName: "同名实例", UserDataDir: "target-1"})
	addProfilePackageTestProfile(t, app, browser.Profile{ProfileId: "target-2", ProfileName: "同名实例", UserDataDir: "target-2"})

	preview, err := app.prepareProfilePackageImportFromPath(zipPath)
	if err != nil {
		t.Fatalf("prepareProfilePackageImportFromPath returned error: %v", err)
	}
	if preview.ConflictCount != 1 || preview.CanOverwrite || !preview.Conflicts[0].Ambiguous || preview.Conflicts[0].TargetMatches != 2 {
		t.Fatalf("unexpected ambiguous conflict preview: %#v", preview)
	}
	if _, err := app.importProfilePackageFromPathWithMode(zipPath, profilePackageImportModeOverwrite); err == nil || !strings.Contains(err.Error(), "多个同名目标") {
		t.Fatalf("expected ambiguous-target error, got %v", err)
	}
}

func TestProfilePackageImportRejectsMultipleSourcesForSameTarget(t *testing.T) {
	app, zipPath := newProfilePackageImportTestApp(t, []browser.Profile{
		{ProfileId: "source-1", ProfileName: "同名实例", UserDataDir: "source-1"},
		{ProfileId: "source-2", ProfileName: "同名实例", UserDataDir: "source-2"},
	}, nil)
	addProfilePackageTestProfile(t, app, browser.Profile{
		ProfileId:   "target-1",
		ProfileName: "同名实例",
		UserDataDir: "target-1",
	})

	preview, err := app.prepareProfilePackageImportFromPath(zipPath)
	if err != nil {
		t.Fatalf("prepareProfilePackageImportFromPath returned error: %v", err)
	}
	if preview.ConflictCount != 2 || preview.CanOverwrite || !preview.Conflicts[1].SourceTargetCollision {
		t.Fatalf("unexpected same-target preview: %#v", preview)
	}
	if _, err := app.importProfilePackageFromPathWithMode(zipPath, profilePackageImportModeOverwrite); err == nil || !strings.Contains(err.Error(), "实例包内存在多个同名实例") {
		t.Fatalf("expected same-target overwrite error, got %v", err)
	}
}

func TestProfilePackagePrepareImportDetectsRepeatedOriginalName(t *testing.T) {
	app, zipPath := newProfilePackageImportTestApp(t, []browser.Profile{{
		ProfileId:   "source-1",
		ProfileName: "CPA",
		UserDataDir: "source-1",
	}}, nil)

	first, err := app.importProfilePackageFromPath(zipPath)
	if err != nil {
		t.Fatalf("first import returned error: %v", err)
	}
	if first.ProfileMappings["source-1"] == "" {
		t.Fatalf("first import did not create profile mapping: %#v", first.ProfileMappings)
	}
	app.browserMgr.Mutex.Lock()
	firstProfile := *app.browserMgr.Profiles[first.ProfileMappings["source-1"]]
	app.browserMgr.Mutex.Unlock()
	if firstProfile.ProfileName != "CPA" {
		t.Fatalf("first import changed the original profile name: %#v", firstProfile)
	}

	preview, err := app.prepareProfilePackageImportFromPath(zipPath)
	if err != nil {
		t.Fatalf("prepare repeated import returned error: %v", err)
	}
	if preview.ConflictCount != 1 || preview.CanOverwrite {
		t.Fatalf("unexpected repeated-import preview without database: %#v", preview)
	}
	conflict := preview.Conflicts[0]
	if conflict.TargetProfileName != "CPA" || conflict.MatchType != profilePackageImportMatchName {
		t.Fatalf("repeated import did not match the original name: %#v", conflict)
	}
}

func TestProfilePackageImportAddsSuffixOnlyWhenNameConflicts(t *testing.T) {
	app, zipPath := newProfilePackageImportTestApp(t, []browser.Profile{{
		ProfileId:   "source-1",
		ProfileName: "CPA",
		UserDataDir: "source-1",
	}}, nil)

	first, err := app.importProfilePackageFromPathWithMode(zipPath, profilePackageImportModeNew)
	if err != nil {
		t.Fatalf("first import returned error: %v", err)
	}
	app.browserMgr.Mutex.Lock()
	firstProfile := *app.browserMgr.Profiles[first.ProfileMappings["source-1"]]
	app.browserMgr.Mutex.Unlock()
	if firstProfile.ProfileName != "CPA" {
		t.Fatalf("first import should preserve the source name: %#v", firstProfile)
	}

	second, err := app.importProfilePackageFromPathWithMode(zipPath, profilePackageImportModeNew)
	if err != nil {
		t.Fatalf("second import returned error: %v", err)
	}
	app.browserMgr.Mutex.Lock()
	secondProfile := *app.browserMgr.Profiles[second.ProfileMappings["source-1"]]
	app.browserMgr.Mutex.Unlock()
	if secondProfile.ProfileName != "CPA（导入）" {
		t.Fatalf("second import should add a suffix after the name conflict: %#v", secondProfile)
	}

	third, err := app.importProfilePackageFromPathWithMode(zipPath, profilePackageImportModeNew)
	if err != nil {
		t.Fatalf("third import returned error: %v", err)
	}
	app.browserMgr.Mutex.Lock()
	thirdProfile := *app.browserMgr.Profiles[third.ProfileMappings["source-1"]]
	app.browserMgr.Mutex.Unlock()
	if thirdProfile.ProfileName != "CPA（导入 2）" {
		t.Fatalf("third import should increment the suffix: %#v", thirdProfile)
	}
}

func TestProfilePackagePrepareImportDetectsDuplicateNamesInPackage(t *testing.T) {
	app, zipPath := newProfilePackageImportTestApp(t, []browser.Profile{
		{ProfileId: "source-1", ProfileName: "CPA", UserDataDir: "source-1"},
		{ProfileId: "source-2", ProfileName: "CPA", UserDataDir: "source-2"},
	}, nil)

	preview, err := app.prepareProfilePackageImportFromPath(zipPath)
	if err != nil {
		t.Fatalf("prepare duplicate-name import returned error: %v", err)
	}
	if preview.ConflictCount != 2 || preview.CanOverwrite {
		t.Fatalf("unexpected duplicate-name preview: %#v", preview)
	}
	for _, conflict := range preview.Conflicts {
		if !conflict.SourceNameCollision || conflict.TargetMatches != 2 {
			t.Fatalf("duplicate source name was not reported: %#v", conflict)
		}
	}
}

func TestProfilePackageImportNewModeDoesNotOverwriteConflict(t *testing.T) {
	app, zipPath := newProfilePackageImportTestApp(t, []browser.Profile{{
		ProfileId:   "profile-1",
		ProfileName: "目标实例",
		UserDataDir: "profile-1",
	}}, nil)
	target := browser.Profile{
		ProfileId:   "profile-1",
		ProfileName: "目标实例",
		UserDataDir: "target-data",
	}
	addProfilePackageTestProfile(t, app, target)

	result, err := app.importProfilePackageFromPathWithMode(zipPath, profilePackageImportModeNew)
	if err != nil {
		t.Fatalf("new-mode import returned error: %v", err)
	}
	newID := result.ProfileMappings["profile-1"]
	if newID == "" || newID == target.ProfileId || result.CreatedCount != 1 || result.OverwrittenCount != 0 {
		t.Fatalf("new-mode import overwrote conflict: result=%#v", result)
	}
	app.browserMgr.Mutex.Lock()
	storedTarget := *app.browserMgr.Profiles[target.ProfileId]
	app.browserMgr.Mutex.Unlock()
	if storedTarget.ProfileName != target.ProfileName || storedTarget.UserDataDir != target.UserDataDir {
		t.Fatalf("existing target was changed: %#v", storedTarget)
	}
}

func addProfilePackageTestProfile(t *testing.T, app *App, profile browser.Profile) {
	t.Helper()
	app.browserMgr.Mutex.Lock()
	copyProfile := profile
	app.browserMgr.Profiles[profile.ProfileId] = &copyProfile
	app.browserMgr.Mutex.Unlock()
}

func newProfilePackageImportTestApp(t *testing.T, profiles []browser.Profile, userDataFiles map[string]string) (*App, string) {
	t.Helper()
	root := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Browser.UserDataRoot = filepath.Join(root, "user-data")
	app := NewApp(root)
	app.config = cfg
	app.browserMgr = browser.NewManager(cfg, root)
	app.browserMgr.InitData()
	zipPath := filepath.Join(root, "profile-package.zip")
	writeTestProfilePackage(t, zipPath, profiles, userDataFiles)
	return app, zipPath
}

func writeTestProfilePackage(t *testing.T, zipPath string, profiles []browser.Profile, userDataFiles map[string]string) {
	t.Helper()
	file, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("create zip failed: %v", err)
	}
	zipWriter := zip.NewWriter(file)
	writeJSONToZip(t, zipWriter, "manifest.json", ProfilePackageManifest{Format: profilePackageFormat, Version: 1, ProfileCount: len(profiles)})
	writeJSONToZip(t, zipWriter, "profiles.json", profiles)
	for name, content := range userDataFiles {
		writer, err := zipWriter.Create("user-data/" + filepath.ToSlash(name))
		if err != nil {
			t.Fatalf("create zip entry failed: %v", err)
		}
		if _, err := writer.Write([]byte(content)); err != nil {
			t.Fatalf("write zip entry failed: %v", err)
		}
	}
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("close zip failed: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close file failed: %v", err)
	}
}

func writeJSONToZip(t *testing.T, zipWriter *zip.Writer, name string, value any) {
	t.Helper()
	writer, err := zipWriter.Create(name)
	if err != nil {
		t.Fatalf("create json entry failed: %v", err)
	}
	if err := json.NewEncoder(writer).Encode(value); err != nil {
		t.Fatalf("encode json failed: %v", err)
	}
}
