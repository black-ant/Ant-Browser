package s3

import (
	"ant-chrome/backend/internal/backup/channels"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewClientRequiresCredentials(t *testing.T) {
	if _, err := NewClient(Config{Bucket: "backup-bucket"}); err == nil || !strings.Contains(err.Error(), "access key ID") {
		t.Fatalf("NewClient error = %v, want access key validation", err)
	}
}

func TestClientTestUsesPathStyleAndSignature(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodHead {
			t.Errorf("request method = %s, want HEAD", request.Method)
		}
		if request.URL.Path != "/backup-bucket" {
			t.Errorf("request path = %s, want /backup-bucket", request.URL.Path)
		}
		if request.Header.Get("x-amz-content-sha256") == "" {
			t.Error("missing x-amz-content-sha256 header")
		}
		if !strings.HasPrefix(request.Header.Get("Authorization"), "AWS4-HMAC-SHA256 ") {
			t.Error("missing AWS Signature V4 authorization")
		}
		response.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewClient(Config{
		Endpoint:        server.URL,
		Region:          "us-east-1",
		Bucket:          "backup-bucket",
		AccessKeyID:     "access-key",
		SecretAccessKey: "secret-key",
		ForcePathStyle:  true,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if err := client.Test(context.Background()); err != nil {
		t.Fatalf("Test: %v", err)
	}
}

func TestBucketURLUsesVirtualHostStyle(t *testing.T) {
	client, err := NewClient(Config{
		Endpoint:        "https://s3.example.com",
		Region:          "us-east-1",
		Bucket:          "backup-bucket",
		AccessKeyID:     "access-key",
		SecretAccessKey: "secret-key",
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	target, err := client.bucketURL()
	if err != nil {
		t.Fatalf("bucketURL: %v", err)
	}
	if target.Host != "backup-bucket.s3.example.com" || target.Path != "/" {
		t.Fatalf("bucket URL = %s, want virtual host style", target.String())
	}
}

func TestClientUploadListDownloadWithPrefixAndPagination(t *testing.T) {
	store := newMemoryS3()
	server := httptest.NewServer(store.handler())
	defer server.Close()

	client, err := NewClient(Config{
		Endpoint:        server.URL,
		Region:          "us-east-1",
		Bucket:          "backup-bucket",
		Prefix:          "ant-chrome/backups/",
		AccessKeyID:     "access-key",
		SecretAccessKey: "secret-key",
		ForcePathStyle:  true,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if client.ID() != channels.S3 {
		t.Fatalf("client ID = %q, want %q", client.ID(), channels.S3)
	}
	if err := client.Test(context.Background()); err != nil {
		t.Fatalf("Test: %v", err)
	}

	localPath := filepath.Join(t.TempDir(), "source.zip")
	content := []byte("s3-backup-content")
	if err := os.WriteFile(localPath, content, 0o644); err != nil {
		t.Fatal(err)
	}
	updates := make([]channels.UploadProgress, 0, 1)
	remoteFile, err := client.UploadWithProgress(context.Background(), localPath, "ant-chrome-backup-20260831.zip", func(progress channels.UploadProgress) {
		updates = append(updates, progress)
	})
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if remoteFile.Name != "ant-chrome-backup-20260831.zip" || remoteFile.Size != int64(len(content)) {
		t.Fatalf("remote file = %+v, want uploaded ZIP", remoteFile)
	}
	if len(updates) == 0 || updates[len(updates)-1].BytesTransferred != int64(len(content)) {
		t.Fatalf("progress updates = %+v, want completed transfer", updates)
	}

	metadataPath := filepath.Join(t.TempDir(), "source.json")
	metadata := []byte(`{"format":"ant-chrome-backup-metadata"}`)
	if err := os.WriteFile(metadataPath, metadata, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := client.UploadMetadata(context.Background(), metadataPath, "ant-chrome-backup-20260831.json"); err != nil {
		t.Fatalf("upload metadata: %v", err)
	}

	items, err := client.List(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 2 || items[0].Name != "ant-chrome-backup-20260831.zip" || items[1].Name != "ant-chrome-backup-20260830.zip" {
		t.Fatalf("listed items = %+v, want paginated ZIP objects sorted by time", items)
	}
	if store.payloadHashMismatch {
		t.Fatal("PUT request used an incorrect payload hash")
	}

	downloadPath := filepath.Join(t.TempDir(), "nested", "restore.zip")
	if err := client.Download(context.Background(), remoteFile.Name, downloadPath); err != nil {
		t.Fatalf("download: %v", err)
	}
	downloaded, err := os.ReadFile(downloadPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(downloaded, content) {
		t.Fatalf("downloaded content = %q, want %q", downloaded, content)
	}
}

func TestNewClientRejectsPrefixTraversal(t *testing.T) {
	_, err := NewClient(Config{
		Endpoint:        "https://s3.example.com",
		Bucket:          "backup-bucket",
		Prefix:          "backups/../other",
		AccessKeyID:     "access-key",
		SecretAccessKey: "secret-key",
	})
	if err == nil || !strings.Contains(err.Error(), "traversal") {
		t.Fatalf("NewClient error = %v, want prefix traversal validation", err)
	}
}

type memoryS3 struct {
	mu                  sync.Mutex
	objects             map[string][]byte
	payloadHashMismatch bool
}

func newMemoryS3() *memoryS3 {
	return &memoryS3{
		objects: map[string][]byte{
			"ant-chrome/backups/ant-chrome-backup-20260830.zip": []byte("older"),
			"ant-chrome/backups/ignore.json":                    []byte("metadata"),
		},
	}
}

func (store *memoryS3) handler() http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/backup-bucket" {
			if request.Method == http.MethodGet && request.URL.Query().Get("list-type") == "2" {
				store.writeList(response, request.URL.Query().Get("continuation-token"))
				return
			}
			if request.Method != http.MethodHead {
				response.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			response.WriteHeader(http.StatusOK)
			return
		}
		key := strings.TrimPrefix(request.URL.Path, "/backup-bucket/")
		key, _ = urlPathUnescape(key)
		switch request.Method {
		case http.MethodGet:
			if request.URL.Query().Get("list-type") == "2" {
				store.writeList(response, request.URL.Query().Get("continuation-token"))
				return
			}
			store.writeObject(response, key)
		case http.MethodPut:
			payload, err := io.ReadAll(request.Body)
			if err != nil {
				response.WriteHeader(http.StatusBadRequest)
				return
			}
			digest := sha256.Sum256(payload)
			if request.Header.Get("x-amz-content-sha256") != hex.EncodeToString(digest[:]) {
				store.mu.Lock()
				store.payloadHashMismatch = true
				store.mu.Unlock()
			}
			store.mu.Lock()
			store.objects[key] = append([]byte(nil), payload...)
			store.mu.Unlock()
			response.WriteHeader(http.StatusOK)
		case http.MethodHead:
			store.writeHead(response, key)
		default:
			response.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
}

func (store *memoryS3) writeList(response http.ResponseWriter, token string) {
	response.Header().Set("Content-Type", "application/xml")
	if token == "page-2" {
		_, _ = fmt.Fprint(response, `<ListBucketResult><IsTruncated>false</IsTruncated><Contents><Key>ant-chrome/backups/ant-chrome-backup-20260830.zip</Key><LastModified>2026-08-30T10:00:00Z</LastModified><Size>5</Size></Contents><Contents><Key>ant-chrome/backups/ignore.json</Key><LastModified>2026-08-30T10:00:00Z</LastModified><Size>8</Size></Contents></ListBucketResult>`)
		return
	}
	_, _ = fmt.Fprint(response, `<ListBucketResult><IsTruncated>true</IsTruncated><NextContinuationToken>page-2</NextContinuationToken><Contents><Key>ant-chrome/backups/ant-chrome-backup-20260831.zip</Key><LastModified>2026-08-31T10:00:00Z</LastModified><Size>17</Size></Contents></ListBucketResult>`)
}

func (store *memoryS3) writeObject(response http.ResponseWriter, key string) {
	store.mu.Lock()
	payload, exists := store.objects[key]
	store.mu.Unlock()
	if !exists {
		response.WriteHeader(http.StatusNotFound)
		return
	}
	response.Header().Set("Content-Length", strconv.Itoa(len(payload)))
	_, _ = response.Write(payload)
}

func (store *memoryS3) writeHead(response http.ResponseWriter, key string) {
	store.mu.Lock()
	payload, exists := store.objects[key]
	store.mu.Unlock()
	if !exists {
		response.WriteHeader(http.StatusNotFound)
		return
	}
	response.Header().Set("Content-Length", strconv.Itoa(len(payload)))
	response.Header().Set("Last-Modified", time.Date(2026, 8, 31, 10, 0, 0, 0, time.UTC).Format(http.TimeFormat))
	response.WriteHeader(http.StatusOK)
}

func urlPathUnescape(value string) (string, error) {
	return url.PathUnescape(value)
}
