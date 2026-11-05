package backend

import (
	"ant-chrome/backend/internal/backup"
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBackupListLocalBackupsScansZIPAndOptionalMetadata(t *testing.T) {
	root := t.TempDir()
	backupDir := filepath.Join(root, "backups")
	if err := os.MkdirAll(filepath.Join(backupDir, "nested"), 0o755); err != nil {
		t.Fatalf("create backup directory: %v", err)
	}

	fullPath := writeTestBackupZIP(t, backupDir, "full.zip")
	profilePath := writeTestBackupZIP(t, backupDir, "profile.zip")
	writeTestBackupZIP(t, filepath.Join(backupDir, "nested"), "nested.zip")
	if err := os.WriteFile(filepath.Join(backupDir, "notes.txt"), []byte("ignore"), 0o644); err != nil {
		t.Fatalf("write non-ZIP file: %v", err)
	}

	writeTestBackupMetadata(t, fullPath, backupMetadata{
		Format:      "ant-chrome-backup-metadata",
		Version:     backupMetadataVersion,
		BackupFile:  filepath.Base(fullPath),
		CreatedAt:   "2026-09-04T10:00:00Z",
		PackageType: "full",
	})
	writeTestBackupMetadata(t, profilePath, backupMetadata{
		Format:       "ant-chrome-backup-metadata",
		Version:      backupMetadataVersion,
		BackupFile:   filepath.Base(profilePath),
		CreatedAt:    "2026-09-04T11:00:00Z",
		PackageType:  "profile",
		ProfileCount: 2,
		ProfileNames: []string{"工作", "个人"},
	})

	app := NewApp(root)
	app.config = config.DefaultConfig()
	app.config.Backup.LocalDirectory = backupDir

	items, err := app.BackupListLocalBackups("")
	if err != nil {
		t.Fatalf("list local backups: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("backup count = %d, want 2: %+v", len(items), items)
	}
	if items[0].Name != "profile.zip" || items[1].Name != "full.zip" {
		t.Fatalf("backup order = [%s, %s], want newest first", items[0].Name, items[1].Name)
	}
	if !items[0].MetadataAvailable || items[0].PackageType != "profile" || items[0].ProfileCount != 2 || len(items[0].ProfileNames) != 2 {
		t.Fatalf("profile metadata = %+v, want parsed metadata", items[0])
	}
	if !items[1].MetadataAvailable || items[1].PackageType != "full" {
		t.Fatalf("full metadata = %+v, want parsed metadata", items[1])
	}
}

func TestBackupListLocalBackupsKeepsZIPWhenMetadataIsUnavailable(t *testing.T) {
	cases := []struct {
		name             string
		metadata         string
		wantError        bool
		wantErrorMessage string
	}{
		{name: "missing", wantError: true, wantErrorMessage: "读取备份包内部 manifest.json 失败"},
		{name: "corrupt", metadata: "{", wantError: true, wantErrorMessage: "解析备份元数据失败"},
		{name: "mismatch", metadata: `{"format":"ant-chrome-backup-metadata","version":1,"backupFile":"other.zip"}`, wantError: true, wantErrorMessage: "备份元数据与 ZIP 文件不匹配"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root := t.TempDir()
			backupDir := filepath.Join(root, "backups")
			zipPath := writeTestBackupZIP(t, backupDir, testCase.name+".zip")
			if testCase.metadata != "" {
				if err := os.WriteFile(backupMetadataPath(zipPath), []byte(testCase.metadata), 0o644); err != nil {
					t.Fatalf("write metadata: %v", err)
				}
			}

			app := NewApp(root)
			app.config = config.DefaultConfig()
			app.config.Backup.LocalDirectory = backupDir
			items, err := app.BackupListLocalBackups("")
			if err != nil {
				t.Fatalf("list local backups: %v", err)
			}
			if len(items) != 1 || items[0].Name != filepath.Base(zipPath) {
				t.Fatalf("items = %+v, want ZIP to remain listed", items)
			}
			if items[0].MetadataAvailable {
				t.Fatalf("metadata should be unavailable: %+v", items[0])
			}
			if testCase.wantError != (items[0].MetadataError != "") {
				t.Fatalf("metadata error = %q, want error = %v", items[0].MetadataError, testCase.wantError)
			}
			if testCase.wantErrorMessage != "" && !strings.Contains(items[0].MetadataError, testCase.wantErrorMessage) {
				t.Fatalf("metadata error = %q, want substring %q", items[0].MetadataError, testCase.wantErrorMessage)
			}
		})
	}
}

func TestBackupListLocalBackupsGeneratesMetadataFromFullManifest(t *testing.T) {
	root := t.TempDir()
	backupDir := filepath.Join(root, "backups")
	zipPath := filepath.Join(backupDir, "profile-looking-name.zip")
	writeTestFullBackupZIP(t, zipPath)

	app := NewApp(root)
	app.config = config.DefaultConfig()
	app.config.Backup.LocalDirectory = backupDir
	items, err := app.BackupListLocalBackups("")
	if err != nil {
		t.Fatalf("list local backups: %v", err)
	}
	if len(items) != 1 || !items[0].MetadataAvailable {
		t.Fatalf("items = %+v, want generated metadata", items)
	}
	if items[0].PackageType != "full" || items[0].AppName != "Test App" || items[0].AppVersion != "1.2.3" {
		t.Fatalf("item metadata = %+v, want full manifest metadata", items[0])
	}
	if _, err := os.Stat(backupMetadataPath(zipPath)); err != nil {
		t.Fatalf("generated metadata file missing: %v", err)
	}
}

func TestBackupListLocalBackupsGeneratesMetadataFromProfileManifest(t *testing.T) {
	root := t.TempDir()
	backupDir := filepath.Join(root, "backups")
	zipPath := filepath.Join(backupDir, "full-looking-name.zip")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatalf("create backup directory: %v", err)
	}
	writeTestProfilePackage(t, zipPath, []browser.Profile{{ProfileId: "profile-a", ProfileName: "实例 A"}}, nil)

	app := NewApp(root)
	app.config = config.DefaultConfig()
	app.config.Backup.LocalDirectory = backupDir
	items, err := app.BackupListLocalBackups("")
	if err != nil {
		t.Fatalf("list local backups: %v", err)
	}
	if len(items) != 1 || !items[0].MetadataAvailable {
		t.Fatalf("items = %+v, want generated metadata", items)
	}
	if items[0].PackageType != "profile" || items[0].ProfileCount != 1 || len(items[0].ProfileNames) != 1 || items[0].ProfileNames[0] != "实例 A" {
		t.Fatalf("item metadata = %+v, want profile manifest metadata", items[0])
	}
	if _, err := os.Stat(backupMetadataPath(zipPath)); err != nil {
		t.Fatalf("generated metadata file missing: %v", err)
	}
}

func TestBackupListLocalBackupsPairsMetadataWithZIPAndReportsOrphanMetadata(t *testing.T) {
	root := t.TempDir()
	backupDir := filepath.Join(root, "backups")
	zipPath := filepath.Join(backupDir, "full.zip")
	writeTestFullBackupZIP(t, zipPath)
	writeTestBackupMetadata(t, zipPath, backupMetadata{
		Format:      "ant-chrome-backup-metadata",
		Version:     backupMetadataVersion,
		BackupFile:  filepath.Base(zipPath),
		CreatedAt:   "2026-09-05T10:00:00Z",
		PackageType: "full",
	})
	orphanMetadataPath := filepath.Join(backupDir, "orphan.json")
	orphanMetadata := backupMetadata{
		Format:     "ant-chrome-backup-metadata",
		Version:    backupMetadataVersion,
		BackupFile: "orphan.zip",
		CreatedAt:  "2026-09-05T11:00:00Z",
	}
	orphanData, err := json.Marshal(orphanMetadata)
	if err != nil {
		t.Fatalf("marshal orphan metadata: %v", err)
	}
	if err := os.WriteFile(orphanMetadataPath, orphanData, 0o644); err != nil {
		t.Fatalf("write orphan metadata: %v", err)
	}

	app := NewApp(root)
	app.config = config.DefaultConfig()
	app.config.Backup.LocalDirectory = backupDir
	items, err := app.BackupListLocalBackups("")
	if err != nil {
		t.Fatalf("list local backups: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %+v, want one ZIP item and one orphan metadata error", items)
	}

	var zipItem, orphanItem *BackupLocalHistoryItem
	for index := range items {
		item := &items[index]
		switch item.Name {
		case filepath.Base(zipPath):
			zipItem = item
		case "orphan.zip":
			orphanItem = item
		}
	}
	if zipItem == nil || !zipItem.MetadataAvailable || zipItem.MetadataOrphan {
		t.Fatalf("ZIP item = %+v, want one metadata-backed ZIP item", zipItem)
	}
	if orphanItem == nil || !orphanItem.MetadataOrphan || orphanItem.MetadataAvailable || !strings.Contains(orphanItem.MetadataError, "同名 ZIP 文件不存在") {
		t.Fatalf("orphan item = %+v, want missing ZIP error", orphanItem)
	}
}

func TestBackupListLocalBackupsKeepsZIPWhenInternalManifestIsInvalid(t *testing.T) {
	root := t.TempDir()
	backupDir := filepath.Join(root, "backups")
	zipPath := filepath.Join(backupDir, "invalid-manifest.zip")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatalf("create backup directory: %v", err)
	}
	file, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("create ZIP: %v", err)
	}
	writer := zip.NewWriter(file)
	manifestWriter, err := writer.Create("manifest.json")
	if err != nil {
		_ = file.Close()
		t.Fatalf("create manifest: %v", err)
	}
	if _, err := manifestWriter.Write([]byte("{")); err != nil {
		_ = writer.Close()
		_ = file.Close()
		t.Fatalf("write manifest: %v", err)
	}
	if err := writer.Close(); err != nil {
		_ = file.Close()
		t.Fatalf("close ZIP: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close ZIP file: %v", err)
	}

	app := NewApp(root)
	app.config = config.DefaultConfig()
	app.config.Backup.LocalDirectory = backupDir
	items, err := app.BackupListLocalBackups("")
	if err != nil {
		t.Fatalf("list local backups: %v", err)
	}
	if len(items) != 1 || items[0].MetadataAvailable || !strings.Contains(items[0].MetadataError, "parse backup manifest failed") {
		t.Fatalf("items = %+v, want unavailable metadata", items)
	}
	if _, err := os.Stat(backupMetadataPath(zipPath)); !os.IsNotExist(err) {
		t.Fatalf("invalid internal manifest should not generate metadata, stat err=%v", err)
	}
}

func TestBackupListLocalBackupsRejectsMissingDirectory(t *testing.T) {
	app := NewApp(t.TempDir())
	app.config = config.DefaultConfig()
	app.config.Backup.LocalDirectory = filepath.Join(t.TempDir(), "missing")

	_, err := app.BackupListLocalBackups("")
	if err == nil || !strings.Contains(err.Error(), "本地备份目录不存在") {
		t.Fatalf("error = %v, want missing directory error", err)
	}
}

func TestBackupNextAvailablePackagePathReservesMetadataName(t *testing.T) {
	directory := t.TempDir()
	metadataPath := filepath.Join(directory, "backup.json")
	if err := os.WriteFile(metadataPath, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write metadata placeholder: %v", err)
	}

	path, err := backupNextAvailablePackagePath(directory, "backup.zip")
	if err != nil {
		t.Fatalf("next available backup path: %v", err)
	}
	if filepath.Base(path) != "backup-1.zip" {
		t.Fatalf("path = %q, want backup-1.zip", path)
	}
}

func writeTestBackupZIP(t *testing.T, directory, name string) string {
	t.Helper()
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatalf("create ZIP directory: %v", err)
	}
	path := filepath.Join(directory, name)
	if err := os.WriteFile(path, []byte("ZIP placeholder"), 0o644); err != nil {
		t.Fatalf("write ZIP placeholder: %v", err)
	}
	return path
}

func writeTestBackupMetadata(t *testing.T, zipPath string, metadata backupMetadata) {
	t.Helper()
	data, err := json.Marshal(metadata)
	if err != nil {
		t.Fatalf("marshal metadata: %v", err)
	}
	if err := os.WriteFile(backupMetadataPath(zipPath), data, 0o644); err != nil {
		t.Fatalf("write metadata: %v", err)
	}
}

func writeTestFullBackupZIP(t *testing.T, zipPath string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(zipPath), 0o755); err != nil {
		t.Fatalf("create ZIP directory: %v", err)
	}
	file, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("create ZIP: %v", err)
	}
	writer := zip.NewWriter(file)
	manifest := backup.Manifest{
		Format:          backup.PackageFormat,
		ManifestVersion: backup.ManifestVersion,
		CreatedAt:       time.Date(2026, time.September, 5, 10, 0, 0, 0, time.UTC).Format(time.RFC3339),
		App:             backup.ManifestAppInfo{Name: "Test App", Version: "1.2.3"},
		Entries: []backup.ManifestEntry{{
			ID:          "config",
			Category:    backup.CategorySystemConfig,
			EntryType:   backup.EntryTypeFile,
			Required:    true,
			ArchivePath: "payload/system/config.yaml",
		}},
	}
	manifestWriter, err := writer.Create("manifest.json")
	if err != nil {
		_ = file.Close()
		t.Fatalf("create manifest: %v", err)
	}
	if err := json.NewEncoder(manifestWriter).Encode(manifest); err != nil {
		_ = writer.Close()
		_ = file.Close()
		t.Fatalf("write manifest: %v", err)
	}
	configWriter, err := writer.Create("payload/system/config.yaml")
	if err != nil {
		_ = writer.Close()
		_ = file.Close()
		t.Fatalf("create payload: %v", err)
	}
	if _, err := configWriter.Write([]byte("app:\n  name: test\n")); err != nil {
		_ = writer.Close()
		_ = file.Close()
		t.Fatalf("write payload: %v", err)
	}
	if err := writer.Close(); err != nil {
		_ = file.Close()
		t.Fatalf("close ZIP: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close ZIP file: %v", err)
	}
}
