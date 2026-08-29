package openlist

import (
	"ant-chrome/backend/internal/backup/channels"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestClientRejectsInvalidInput(t *testing.T) {
	if _, err := NewClient(Config{BaseURL: `ftp://example.test`}); err == nil {
		t.Fatal(`expected invalid scheme error`)
	}
	client, err := NewClient(Config{BaseURL: `https://example.test/dav`, Token: `secret`})
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
		Token:      `secret`,
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

func TestClientUploadWithProgressReportsTransfer(t *testing.T) {
	store := newMemoryWebDAV()
	server := store.server()
	defer server.Close()
	client, err := NewClient(Config{
		BaseURL:    server.URL + `/dav`,
		RemotePath: `backups`,
		Token:      `secret`,
	})
	if err != nil {
		t.Fatal(err)
	}
	localPath := t.TempDir() + `/source.zip`
	content := bytes.Repeat([]byte("x"), 256*1024)
	if err := os.WriteFile(localPath, content, 0o644); err != nil {
		t.Fatal(err)
	}
	updates := make([]channels.UploadProgress, 0, 1)
	_, err = client.UploadWithProgress(context.Background(), localPath, `ant-chrome-progress.zip`, func(progress channels.UploadProgress) {
		updates = append(updates, progress)
	})
	if err != nil {
		t.Fatalf(`upload failed: %v`, err)
	}
	if len(updates) == 0 || updates[len(updates)-1].BytesTransferred != int64(len(content)) {
		t.Fatalf(`progress updates = %+v, want completed transfer`, updates)
	}
}

func TestClientUploadExplainsRequestEntityTooLarge(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPut {
			response.WriteHeader(http.StatusRequestEntityTooLarge)
			_, _ = response.Write([]byte(`<html><center>openresty</center></html>`))
			return
		}
		response.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	client, err := NewClient(Config{BaseURL: server.URL + `/dav`, Token: `secret`})
	if err != nil {
		t.Fatal(err)
	}
	localPath := t.TempDir() + `/source.zip`
	if err := os.WriteFile(localPath, []byte(`content`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = client.Upload(context.Background(), localPath, `ant-chrome-413.zip`)
	if err == nil || !strings.Contains(err.Error(), `client_max_body_size`) {
		t.Fatalf(`upload error = %v, want client_max_body_size guidance`, err)
	}
}
