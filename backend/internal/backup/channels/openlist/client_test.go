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
	"time"
)

const testBaseURL = "https" + "://example.test/dav"

func TestClientRejectsInvalidInput(t *testing.T) {
	if _, err := NewClient(Config{BaseURL: `ftp://example.test`}); err == nil {
		t.Fatal(`expected invalid scheme error`)
	}
	if _, err := NewClient(Config{BaseURL: `https://example.test`, Token: `secret`}); err == nil || !strings.Contains(err.Error(), `site root`) {
		t.Fatalf(`site-root URL error = %v, want WebDAV endpoint error`, err)
	}
	client, err := NewClient(Config{BaseURL: `https://example.test/dav`, Token: `secret`})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Upload(context.Background(), `backup.zip`, `../backup.zip`); err == nil {
		t.Fatal(`expected traversal error`)
	}
}

func TestClientReportsWebDAVEndpointErrorBeforeRemoteDirectory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != `/dav/` {
			t.Fatalf(`request path = %q, want /dav/`, r.URL.Path)
		}
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`virtual disk path is missing a mapping ID`))
	}))
	defer server.Close()

	client, err := NewClient(Config{
		BaseURL:    server.URL + `/dav`,
		RemotePath: `ant-browser`,
		Token:      `secret`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Test(context.Background()); err == nil || !strings.Contains(err.Error(), `WebDAV endpoint check failed`) || !strings.Contains(err.Error(), `mapping ID`) {
		t.Fatalf(`connection test error = %v, want WebDAV endpoint error`, err)
	}
}

func TestClientDoesNotFollowWebDAVRedirectAsGet(t *testing.T) {
	methods := make([]string, 0, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		w.Header().Set(`Location`, `https://example.test/dav/`)
		w.WriteHeader(http.StatusMovedPermanently)
	}))
	defer server.Close()

	client, err := NewClient(Config{
		BaseURL: server.URL + `/dav`,
		Token:   `secret`,
	})
	if err != nil {
		t.Fatal(err)
	}
	err = client.Test(context.Background())
	if err == nil || !strings.Contains(err.Error(), `configure the final HTTPS WebDAV URL`) {
		t.Fatalf(`redirect error = %v, want HTTPS configuration guidance`, err)
	}
	if len(methods) != 1 || methods[0] != methodPROPFIND {
		t.Fatalf(`redirect methods = %v, want one PROPFIND request`, methods)
	}
}

func TestClientUsesBasicAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok || username != `webdav-user` || password != `webdav-password` {
			t.Fatalf(`basic auth = (%q, %q, %t), want configured credentials`, username, password, ok)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, err := NewClient(Config{
		BaseURL:  server.URL + `/dav`,
		AuthMode: authModeBasic,
		Username: `webdav-user`,
		Password: `webdav-password`,
	})
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.request(context.Background(), methodPROPFIND, ``, nil, -1, nil)
	if err != nil {
		t.Fatalf(`basic-auth request failed: %v`, err)
	}
	defer response.Body.Close()
}

func TestClientNormalizesBearerPrefix(t *testing.T) {
	client, err := NewClient(Config{
		BaseURL: testBaseURL,
		Token:   `  Bearer secret-token  `,
	})
	if err != nil {
		t.Fatal(err)
	}
	if client.config.AuthMode != authModeToken || client.config.Token != `secret-token` {
		t.Fatalf(`normalized token config = %+v, want token mode with stripped bearer prefix`, client.config)
	}
}

func TestClientFollowsGetRedirect(t *testing.T) {
	paths := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+` `+r.URL.Path)
		if r.Header.Get(`Authorization`) != `Bearer secret-token` {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case `/dav/backup.zip`:
			w.Header().Set(`Location`, `/download/backup.zip`)
			w.WriteHeader(http.StatusFound)
		case `/download/backup.zip`:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`redirected-backup`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := NewClient(Config{
		BaseURL: server.URL + `/dav`,
		Token:   `secret-token`,
	})
	if err != nil {
		t.Fatal(err)
	}
	localPath := t.TempDir() + `/backup.zip`
	if err := client.Download(context.Background(), `backup.zip`, localPath); err != nil {
		t.Fatalf(`download through redirect failed: %v`, err)
	}
	content, err := os.ReadFile(localPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != `redirected-backup` {
		t.Fatalf(`downloaded content = %q, want redirected response`, content)
	}
	if strings.Join(paths, `,`) != `GET /dav/backup.zip,GET /download/backup.zip` {
		t.Fatalf(`redirect request sequence = %v, want two GET requests`, paths)
	}
}

func TestClientUploadListDownload(t *testing.T) {
	store := newMemoryWebDAV()
	store.dirs[`backups`] = true
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
	metadataPath := t.TempDir() + `/source.json`
	metadata := []byte(`{format:ant-chrome-backup-metadata,version:1}`)
	if err := os.WriteFile(metadataPath, metadata, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := client.UploadMetadata(context.Background(), metadataPath, `ant-chrome-backup-20260825.json`); err != nil {
		t.Fatalf(`metadata upload failed: %v`, err)
	}
	metadataDownloadPath := t.TempDir() + `/nested/restore.json`
	if err := client.DownloadMetadata(context.Background(), `ant-chrome-backup-20260825.json`, metadataDownloadPath); err != nil {
		t.Fatalf(`metadata download failed: %v`, err)
	}
	metadataDownloaded, err := os.ReadFile(metadataDownloadPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(metadataDownloaded) != string(metadata) {
		t.Fatalf(`downloaded metadata = %q, want %q`, metadataDownloaded, metadata)
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

func TestClientTestAcceptsExistingDirectoryWhenMetadataMethodsReturn405(t *testing.T) {
	store := newMemoryWebDAV()
	store.dirs[`backups`] = true
	store.rejectPROPFIND = true
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
	if store.mkcolCalls != 0 {
		t.Fatalf(`MKCOL calls = %d, want 0`, store.mkcolCalls)
	}
}

func TestClientTestRejectsMissingDirectoryWithoutCreating(t *testing.T) {
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
	if err := client.Test(context.Background()); err == nil || !strings.Contains(err.Error(), `remote directory`) || !strings.Contains(err.Error(), `backups`) || !strings.Contains(err.Error(), `does not exist`) {
		t.Fatalf(`connection test error = %v, want missing-directory error`, err)
	}
	if store.mkcolCalls != 0 {
		t.Fatalf(`MKCOL calls = %d, want 0`, store.mkcolCalls)
	}
}

func TestClientFallsBackWhenDirectoryMetadataRejectsTrailingSlash(t *testing.T) {
	store := newMemoryWebDAV()
	store.dirs[`backups`] = true
	store.rejectDirectoryTrailingSlash = true
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
	if _, err := client.List(context.Background()); err != nil {
		t.Fatalf(`list failed after directory metadata fallback: %v`, err)
	}
}

func TestClientUsesOptionsWhenDirectoryMetadataMethodsAreUnavailable(t *testing.T) {
	store := newMemoryWebDAV()
	store.dirs[`backups`] = true
	store.rejectPROPFIND = true
	store.rejectHEAD = true
	store.rejectDirectoryTrailingSlash = true
	store.allowOptions = true
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
		t.Fatalf(`connection test failed through OPTIONS: %v`, err)
	}
	if store.mkcolCalls != 0 {
		t.Fatalf(`MKCOL calls = %d, want 0`, store.mkcolCalls)
	}
}

func TestClientUploadWithProgressReportsTransfer(t *testing.T) {
	store := newMemoryWebDAV()
	store.dirs[`backups`] = true
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
	if updates[len(updates)-1].Stage != channels.UploadProgressStageVerifying {
		t.Fatalf(`last progress stage = %q, want %q`, updates[len(updates)-1].Stage, channels.UploadProgressStageVerifying)
	}
}

func TestClientUploadSupportsVirtualDiskWithoutMove(t *testing.T) {
	store := newMemoryWebDAV()
	store.dirs[`backups`] = true
	store.rejectMoves = true
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
	content := []byte(`virtual-disk-backup`)
	if err := os.WriteFile(localPath, content, 0o644); err != nil {
		t.Fatal(err)
	}

	remoteFile, err := client.Upload(context.Background(), localPath, `ant-chrome-virtual-disk.zip`)
	if err != nil {
		t.Fatalf(`upload failed: %v`, err)
	}
	if remoteFile.Name != `ant-chrome-virtual-disk.zip` || remoteFile.Size != int64(len(content)) {
		t.Fatalf(`unexpected remote file: %+v`, remoteFile)
	}
	if !store.hasFile(`backups/ant-chrome-virtual-disk.zip`) {
		t.Fatal(`final remote file was not uploaded`)
	}
}

func TestClientTreatsWrittenVirtualDisk502AsCompletedWithWarning(t *testing.T) {
	store := newMemoryWebDAV()
	store.dirs[`backups`] = true
	store.putResponseStatus = http.StatusBadGateway
	store.putResponseBody = `{"conflictMode":"overwrite","message":"虚拟盘文件已写入，但部分目标同步失败","remotePath":"ant-chrome-virtual-disk-502.zip","status":"failed","targets":[{"destinationId":"openlist","destinationName":"OpenList","status":"failed","message":"上传超时","durationMs":31326}]}`
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
	content := []byte(`virtual-disk-502-backup`)
	if err := os.WriteFile(localPath, content, 0o644); err != nil {
		t.Fatal(err)
	}

	outcome, err := client.UploadWithProgressOutcome(context.Background(), localPath, `ant-chrome-virtual-disk-502.zip`, nil)
	if err != nil {
		t.Fatalf(`upload failed: %v`, err)
	}
	if outcome.File.Size != int64(len(content)) {
		t.Fatalf(`remote file size = %d, want %d`, outcome.File.Size, len(content))
	}
	if !strings.Contains(outcome.Warning, `虚拟盘文件已写入`) || !strings.Contains(outcome.Warning, `远端文件大小已校验`) {
		t.Fatalf(`upload warning = %q, want committed-write warning`, outcome.Warning)
	}
	if !store.hasFile(`backups/ant-chrome-virtual-disk-502.zip`) {
		t.Fatal(`committed virtual-disk file was deleted after HTTP 502`)
	}
}

func TestClientLeavesCommittedFileOnRemoteSizeMismatch(t *testing.T) {
	store := newMemoryWebDAV()
	store.dirs[`backups`] = true
	store.putResponseStatus = http.StatusBadGateway
	store.putStoredData = []byte(`partial-content`)
	store.putResponseBody = strings.ReplaceAll(`{'message':'虚拟盘文件已写入，但部分目标同步失败','remotePath':'ant-chrome-size-mismatch.zip','targets':[{'status':'failed'}]}`, `'`, string(rune(34)))
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
	if err := os.WriteFile(localPath, []byte(`complete-content`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err = client.UploadWithProgressOutcome(context.Background(), localPath, `ant-chrome-size-mismatch.zip`, nil)
	if err == nil || !strings.Contains(err.Error(), `remote size mismatch`) || !strings.Contains(err.Error(), `remote file was left in place`) {
		t.Fatalf(`upload error = %v, want committed size mismatch without deletion`, err)
	}
	if !store.hasFile(`backups/ant-chrome-size-mismatch.zip`) {
		t.Fatal(`committed virtual-disk file was deleted after size mismatch`)
	}
}

func TestClientTreatsTimedOutPutAsCompletedWhenRemoteFileExists(t *testing.T) {
	store := newMemoryWebDAV()
	store.dirs[`backups`] = true
	store.hangPutResponse = true
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
	client.controlTimeout = 100 * time.Millisecond
	transport, ok := client.httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatal(`expected cloned HTTP transport`)
	}
	transport.ResponseHeaderTimeout = 20 * time.Millisecond
	localPath := t.TempDir() + `/source.zip`
	content := []byte(`completed-before-response`)
	if err := os.WriteFile(localPath, content, 0o644); err != nil {
		t.Fatal(err)
	}

	startedAt := time.Now()
	remoteFile, err := client.Upload(context.Background(), localPath, `ant-chrome-timeout-completed.zip`)
	if err != nil {
		t.Fatalf(`upload failed: %v`, err)
	}
	if remoteFile.Size != int64(len(content)) {
		t.Fatalf(`remote file size = %d, want %d`, remoteFile.Size, len(content))
	}
	if elapsed := time.Since(startedAt); elapsed > time.Second {
		t.Fatalf(`upload took %s, expected timeout recovery`, elapsed)
	}
}

func TestClientUploadLimitsRemoteVerificationTimeout(t *testing.T) {
	store := newMemoryWebDAV()
	store.dirs[`backups`] = true
	store.hangFileStat = true
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
	client.controlTimeout = 20 * time.Millisecond
	localPath := t.TempDir() + `/source.zip`
	if err := os.WriteFile(localPath, []byte(`verification-timeout`), 0o644); err != nil {
		t.Fatal(err)
	}

	startedAt := time.Now()
	_, err = client.Upload(context.Background(), localPath, `ant-chrome-verification-timeout.zip`)
	if err == nil {
		t.Fatal(`expected remote verification timeout`)
	}
	if elapsed := time.Since(startedAt); elapsed > time.Second {
		t.Fatalf(`upload took %s, expected control timeout`, elapsed)
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
