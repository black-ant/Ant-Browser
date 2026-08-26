package backupremote

import (
	"context"
	"os"
	"testing"
)

func TestClientRejectsInvalidInput(t *testing.T) {
	if _, err := NewClient(Config{BaseURL: `ftp://example.test`}); err == nil {
		t.Fatal(`expected invalid scheme error`)
	}
	client, err := NewClient(Config{BaseURL: `https://example.test/dav`})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Upload(context.Background(), `backup.zip`, `../backup.zip`); err == nil {
		t.Fatal(`expected traversal error`)
	}
}

func TestClientUploadListDownload(t *testing.T) {
	store := newMemoryWebDAV()
	server := store.server()
	defer server.Close()
	client, err := NewClient(Config{
		BaseURL:    server.URL + `/dav`,
		RemotePath: `backups`,
		Username:   `user`,
		Password:   `secret`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Test(context.Background()); err != nil {
		t.Fatalf(`connection test failed: %v`, err)
	}
	localPath := t.TempDir() + `/source.zip`
	content := []byte(`backup-content`)
	if err := os.WriteFile(localPath, content, 0o644); err != nil {
		t.Fatal(err)
	}
	remoteFile, err := client.Upload(context.Background(), localPath, `ant-chrome-backup-20260825.zip`)
	if err != nil {
		t.Fatalf(`upload failed: %v`, err)
	}
	if remoteFile.Name != `ant-chrome-backup-20260825.zip` || remoteFile.Size != int64(len(content)) {
		t.Fatalf(`unexpected remote file: %+v`, remoteFile)
	}
	if store.hasFile(`backups/ant-chrome-backup-20260825.zip.uploading`) {
		t.Fatal(`temporary remote file was not finalized`)
	}
	items, err := client.List(context.Background())
	if err != nil {
		t.Fatalf(`list failed: %v`, err)
	}
	if len(items) != 1 || items[0].Name != remoteFile.Name {
		t.Fatalf(`unexpected remote list: %+v`, items)
	}
	downloadPath := t.TempDir() + `/nested/restore.zip`
	if err := client.Download(context.Background(), remoteFile.Name, downloadPath); err != nil {
		t.Fatalf(`download failed: %v`, err)
	}
	downloaded, err := os.ReadFile(downloadPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(downloaded) != string(content) {
		t.Fatalf(`downloaded content = %q, want %q`, downloaded, content)
	}
}
