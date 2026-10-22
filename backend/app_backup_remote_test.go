package backend

import (
	"ant-chrome/backend/internal/backup/channels"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

type backupRemoteMetadataTestClient struct {
	metadata map[string][]byte
}

func (client *backupRemoteMetadataTestClient) DownloadMetadata(_ context.Context, fileName, localPath string) error {
	data, ok := client.metadata[fileName]
	if !ok {
		return os.ErrNotExist
	}
	return os.WriteFile(localPath, data, 0o644)
}

type backupRemoteRestoreTestClient struct {
	backupRemoteMetadataTestClient
	downloadCalled bool
}

func (client *backupRemoteRestoreTestClient) ID() channels.ID { return channels.OpenList }

func (client *backupRemoteRestoreTestClient) Test(context.Context) error { return nil }

func (client *backupRemoteRestoreTestClient) List(context.Context) ([]channels.File, error) {
	return nil, nil
}

func (client *backupRemoteRestoreTestClient) Upload(context.Context, string, string) (channels.File, error) {
	return channels.File{}, nil
}

func (client *backupRemoteRestoreTestClient) UploadMetadata(context.Context, string, string) (channels.File, error) {
	return channels.File{}, nil
}

func (client *backupRemoteRestoreTestClient) Download(context.Context, string, string) error {
	client.downloadCalled = true
	return os.ErrNotExist
}

func TestFormatBackupFileSize(t *testing.T) {
	tests := []struct {
		size int64
		want string
	}{
		{size: 1023, want: `1023 B`},
		{size: 1024, want: `1.00 KB`},
		{size: 1024 * 1024, want: `1.00 MB`},
		{size: 29631128, want: `28.26 MB`},
	}

	for _, test := range tests {
		if got := formatBackupFileSize(test.size); got != test.want {
			t.Errorf(`formatBackupFileSize(%d) = %q, want %q`, test.size, got, test.want)
		}
	}
}

func TestLockBackupMaintenanceWaitsBriefly(t *testing.T) {
	app := NewApp(t.TempDir())
	app.maintenanceMu.Lock()
	released := make(chan struct{})
	go func() {
		time.Sleep(20 * time.Millisecond)
		app.maintenanceMu.Unlock()
		close(released)
	}()

	if err := app.lockBackupMaintenance(); err != nil {
		t.Fatalf(`lockBackupMaintenance returned error: %v`, err)
	}
	app.maintenanceMu.Unlock()
	<-released
}

func TestBackupRemoteHistoryEntriesReadsMetadata(t *testing.T) {
	metadata, err := json.Marshal(backupMetadata{
		Format:       `ant-chrome-backup-metadata`,
		Version:      backupMetadataVersion,
		BackupFile:   `ant-chrome-profile-backup-multi-2--20260912-093115.zip`,
		PackageType:  `profile`,
		ProfileCount: 2,
		ProfileNames: []string{`工作`, `个人`},
	})
	if err != nil {
		t.Fatalf(`marshal metadata: %v`, err)
	}

	app := NewApp(t.TempDir())
	items := app.backupRemoteHistoryEntries(&backupRemoteMetadataTestClient{
		metadata: map[string][]byte{
			`ant-chrome-profile-backup-multi-2--20260912-093115.json`: metadata,
		},
	}, []channels.File{{
		Name:       `ant-chrome-profile-backup-multi-2--20260912-093115.zip`,
		Size:       86 * 1024 * 1024,
		ModifiedAt: `2026-09-12T09:31:15Z`,
	}}, time.Second)

	if len(items) != 1 {
		t.Fatalf(`remote history item count = %d, want 1`, len(items))
	}
	item := items[0]
	if item[`packageType`] != `profile` || item[`profileCount`] != 2 {
		t.Fatalf(`remote history metadata = %+v, want profile metadata`, item)
	}
	profileNames, ok := item[`profileNames`].([]string)
	if !ok || len(profileNames) != 2 || profileNames[0] != `工作` || profileNames[1] != `个人` {
		t.Fatalf(`remote history profile names = %#v, want [工作 个人]`, item[`profileNames`])
	}
}

func TestBackupRemoteHistoryEntriesFallsBackToFileName(t *testing.T) {
	app := NewApp(t.TempDir())
	items := app.backupRemoteHistoryEntries(&backupRemoteMetadataTestClient{}, []channels.File{
		{
			Name: `ant-chrome-profile-backup-single--ChatGPT-已登录--20260913-161549.751207500.zip`,
		},
		{
			Name: `ant-chrome-profile-backup-multi-3--20260913-161549.751207500.zip`,
		},
		{
			Name: `ant-chrome-backup-20260913-161549.zip`,
		},
	}, time.Second)

	if len(items) != 3 {
		t.Fatalf(`remote history item count = %d, want 3`, len(items))
	}
	if items[0][`packageType`] != `profile` || items[0][`profileCount`] != 1 {
		t.Fatalf(`single profile fallback = %#v, want profile count 1`, items[0])
	}
	profileNames, ok := items[0][`profileNames`].([]string)
	if !ok || len(profileNames) != 1 || profileNames[0] != `ChatGPT-已登录` {
		t.Fatalf(`single profile names fallback = %#v, want ChatGPT-已登录`, items[0][`profileNames`])
	}
	if items[1][`packageType`] != `profile` || items[1][`profileCount`] != 3 {
		t.Fatalf(`multi profile fallback = %#v, want profile count 3`, items[1])
	}
	if items[2][`packageType`] != `full` {
		t.Fatalf(`full backup fallback = %#v, want full`, items[2])
	}
}

func TestBackupRemoteHistoryEntriesFallsBackWhenMetadataMismatches(t *testing.T) {
	metadata, err := json.Marshal(backupMetadata{
		Format:      `ant-chrome-backup-metadata`,
		Version:     backupMetadataVersion,
		BackupFile:  `other-backup.zip`,
		PackageType: `full`,
	})
	if err != nil {
		t.Fatalf(`marshal metadata: %v`, err)
	}

	backupName := `ant-chrome-profile-backup-single--ChatGPT-已登录--20260913-161549.751207500.zip`
	app := NewApp(t.TempDir())
	items := app.backupRemoteHistoryEntries(&backupRemoteMetadataTestClient{
		metadata: map[string][]byte{
			`ant-chrome-profile-backup-single--ChatGPT-已登录--20260913-161549.751207500.json`: metadata,
		},
	}, []channels.File{{Name: backupName}}, time.Second)

	if len(items) != 1 {
		t.Fatalf(`remote history item count = %d, want 1`, len(items))
	}
	if items[0][`packageType`] != `profile` || items[0][`profileCount`] != 1 {
		t.Fatalf(`mismatched metadata fallback = %#v, want profile count 1`, items[0])
	}
	profileNames, ok := items[0][`profileNames`].([]string)
	if !ok || len(profileNames) != 1 || profileNames[0] != `ChatGPT-已登录` {
		t.Fatalf(`mismatched metadata profile names = %#v, want ChatGPT-已登录`, items[0][`profileNames`])
	}
}

func TestNormalizeRemoteBackupRestoreName(t *testing.T) {
	tests := []struct {
		name    string
		want    string
		wantErr bool
	}{
		{name: `backup.zip`, want: `backup.zip`},
		{name: ` backup.ZIP `, want: `backup.ZIP`},
		{name: `../backup.zip`, wantErr: true},
		{name: `folder/backup.zip`, wantErr: true},
		{name: `folder\\backup.zip`, wantErr: true},
		{name: `backup.tar`, wantErr: true},
		{name: `backup.zip` + "\n", wantErr: true},
	}

	for _, test := range tests {
		got, err := normalizeRemoteBackupRestoreName(test.name)
		if test.wantErr {
			if err == nil {
				t.Errorf(`normalizeRemoteBackupRestoreName(%q) returned nil error`, test.name)
			}
			continue
		}
		if err != nil {
			t.Errorf(`normalizeRemoteBackupRestoreName(%q) returned error: %v`, test.name, err)
			continue
		}
		if got != test.want {
			t.Errorf(`normalizeRemoteBackupRestoreName(%q) = %q, want %q`, test.name, got, test.want)
		}
	}
}

func TestBackupRestoreRemoteRejectsMismatchedMetadata(t *testing.T) {
	metadata, err := json.Marshal(backupMetadata{
		Format:     `ant-chrome-backup-metadata`,
		Version:    backupMetadataVersion,
		BackupFile: `other-backup.zip`,
	})
	if err != nil {
		t.Fatalf(`marshal metadata: %v`, err)
	}
	client := &backupRemoteRestoreTestClient{
		backupRemoteMetadataTestClient: backupRemoteMetadataTestClient{
			metadata: map[string][]byte{`backup.json`: metadata},
		},
	}

	app := NewApp(t.TempDir())
	_, err = app.backupRestoreRemoteLocked(client, `OpenList`, time.Second, `backup.zip`, `ant-chrome-test-restore-`)
	if err == nil || !strings.Contains(err.Error(), `备份元数据与 ZIP 文件不匹配`) {
		t.Fatalf(`backupRestoreRemoteLocked error = %v, want metadata mismatch`, err)
	}
	if client.downloadCalled {
		t.Fatal(`backupRestoreRemoteLocked downloaded ZIP after metadata mismatch`)
	}
}

func TestBackupDownloadRemoteMetadataKeepsExistingFileOnDownloadFailure(t *testing.T) {
	metadataPath := t.TempDir() + string(os.PathSeparator) + `backup.json`
	original := []byte(`existing metadata`)
	if err := os.WriteFile(metadataPath, original, 0o644); err != nil {
		t.Fatalf(`write existing metadata: %v`, err)
	}

	app := NewApp(t.TempDir())
	err := app.backupDownloadRemoteMetadata(&backupRemoteMetadataTestClient{}, time.Second, `backup.json`, metadataPath, `backup.zip`)
	if err == nil {
		t.Fatal(`backupDownloadRemoteMetadata returned nil error for missing metadata`)
	}
	content, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf(`read existing metadata: %v`, err)
	}
	if string(content) != string(original) {
		t.Fatalf(`existing metadata changed after failed download: %q`, content)
	}
}

func TestBackupDownloadRemoteMetadataKeepsExistingFileOnInvalidMetadata(t *testing.T) {
	metadataPath := t.TempDir() + string(os.PathSeparator) + `backup.json`
	original := []byte(`existing metadata`)
	if err := os.WriteFile(metadataPath, original, 0o644); err != nil {
		t.Fatalf(`write existing metadata: %v`, err)
	}

	app := NewApp(t.TempDir())
	err := app.backupDownloadRemoteMetadata(&backupRemoteMetadataTestClient{
		metadata: map[string][]byte{`backup.json`: []byte(`{`)},
	}, time.Second, `backup.json`, metadataPath, `backup.zip`)
	if err == nil {
		t.Fatal(`backupDownloadRemoteMetadata returned nil error for invalid metadata`)
	}
	content, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf(`read existing metadata: %v`, err)
	}
	if string(content) != string(original) {
		t.Fatalf(`existing metadata changed after invalid download: %q`, content)
	}
}

func TestBackupDownloadRemoteMetadataPublishesValidatedMetadata(t *testing.T) {
	metadataPath := t.TempDir() + string(os.PathSeparator) + `backup.json`
	if err := os.WriteFile(metadataPath, []byte(`existing metadata`), 0o644); err != nil {
		t.Fatalf(`write existing metadata: %v`, err)
	}
	metadata, err := json.Marshal(backupMetadata{
		Format:  `ant-chrome-backup-metadata`,
		Version: backupMetadataVersion,
	})
	if err != nil {
		t.Fatalf(`marshal metadata: %v`, err)
	}

	app := NewApp(t.TempDir())
	if err := app.backupDownloadRemoteMetadata(&backupRemoteMetadataTestClient{
		metadata: map[string][]byte{`backup.json`: metadata},
	}, time.Second, `backup.json`, metadataPath, `backup.zip`); err != nil {
		t.Fatalf(`backupDownloadRemoteMetadata returned error: %v`, err)
	}
	content, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf(`read published metadata: %v`, err)
	}
	var loaded backupMetadata
	if err := json.Unmarshal(content, &loaded); err != nil {
		t.Fatalf(`parse published metadata: %v`, err)
	}
	if loaded.BackupFile != `backup.zip` {
		t.Fatalf(`published backup file = %q, want backup.zip`, loaded.BackupFile)
	}
}
