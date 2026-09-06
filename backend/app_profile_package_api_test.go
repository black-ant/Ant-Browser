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

func TestProfilePackagePrepareImportDetectsIDConflict(t *testing.T) {
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
	if !preview.CanOverwrite {
		t.Fatal("expected safe ID conflict to allow overwrite")
	}
	conflict := preview.Conflicts[0]
	if conflict.MatchType != profilePackageImportMatchID || conflict.TargetProfileID != "profile-1" || conflict.TargetProfileName != "目标实例" {
		t.Fatalf("unexpected ID conflict: %#v", conflict)
	}
}

func TestProfilePackageImportOverwritePreservesTargetIDAndUserDataDir(t *testing.T) {
	app, zipPath := newProfilePackageImportTestApp(t, []browser.Profile{{
		ProfileId:   "profile-1",
		ProfileName: "新配置",
		UserDataDir: "profile-1",
	}}, map[string]string{"profile-1/Default/Preferences": "new-data"})
	target := browser.Profile{
		ProfileId:   "profile-1",
		ProfileName: "旧配置",
		UserDataDir: "target-data",
		CreatedAt:   "2026-09-01T00:00:00Z",
	}
	addProfilePackageTestProfile(t, app, target)
	targetDir := filepath.Join(app.config.Browser.UserDataRoot, target.UserDataDir)
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
	if result.ProfileMappings["profile-1"] != "profile-1" {
		t.Fatalf("overwrite changed profile ID: %#v", result.ProfileMappings)
	}
	app.browserMgr.Mutex.Lock()
	stored := *app.browserMgr.Profiles["profile-1"]
	app.browserMgr.Mutex.Unlock()
	if stored.ProfileName != "新配置" || stored.UserDataDir != "target-data" || stored.CreatedAt != target.CreatedAt {
		t.Fatalf("target profile fields were not preserved correctly: %#v", stored)
	}
	if content, err := os.ReadFile(filepath.Join(targetDir, "Default", "Preferences")); err != nil || string(content) != "new-data" {
		t.Fatalf("overwritten user data missing or incorrect: content=%q err=%v", content, err)
	}
	if _, err := os.Stat(filepath.Join(targetDir, "old.txt")); !os.IsNotExist(err) {
		t.Fatalf("old user data was not replaced, stat err=%v", err)
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

func TestProfilePackagePrepareImportDetectsPreviouslyImportedName(t *testing.T) {
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

	preview, err := app.prepareProfilePackageImportFromPath(zipPath)
	if err != nil {
		t.Fatalf("prepare repeated import returned error: %v", err)
	}
	if preview.ConflictCount != 1 || !preview.CanOverwrite {
		t.Fatalf("unexpected repeated-import preview: %#v", preview)
	}
	conflict := preview.Conflicts[0]
	if conflict.TargetProfileName != "CPA（导入）" || conflict.MatchType != profilePackageImportMatchName {
		t.Fatalf("repeated import did not match generated name: %#v", conflict)
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
