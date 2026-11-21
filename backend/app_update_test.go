package backend

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckForUpdatesDetectsNewRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"tag_name":"v1.9.0","html_url":"https://example.com/release"}`))
	}))
	defer server.Close()

	result := checkForUpdates(context.Background(), "1.8.0", server.URL, server.Client())
	if !result.OK {
		t.Fatalf("checkForUpdates failed: %+v", result)
	}
	if !result.UpdateAvailable {
		t.Fatalf("expected update available: %+v", result)
	}
	if result.LatestVersion != "1.9.0" {
		t.Fatalf("latest version = %q, want 1.9.0", result.LatestVersion)
	}
}

func TestCheckForUpdatesReportsNoUpdate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"tag_name":"v1.8.0","name":"1.8.0"}`))
	}))
	defer server.Close()

	result := checkForUpdates(context.Background(), "1.8.0", server.URL, server.Client())
	if !result.OK {
		t.Fatalf("checkForUpdates failed: %+v", result)
	}
	if result.UpdateAvailable {
		t.Fatalf("expected no update: %+v", result)
	}
}

func TestCheckForUpdatesReportsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Error(writer, "service unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	result := checkForUpdates(context.Background(), "1.8.0", server.URL, server.Client())
	if result.OK {
		t.Fatalf("expected HTTP error: %+v", result)
	}
	if result.Error != "更新服务返回 HTTP 503" {
		t.Fatalf("error = %q, want HTTP 503 error", result.Error)
	}
}

func TestCheckForUpdatesReportsInvalidResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"tag_name":`))
	}))
	defer server.Close()

	result := checkForUpdates(context.Background(), "1.8.0", server.URL, server.Client())
	if result.OK {
		t.Fatalf("expected invalid response error: %+v", result)
	}
	if result.Error == "" {
		t.Fatal("expected invalid response error")
	}
}

func TestCheckForUpdatesDoesNotRequestWithUnknownCurrentVersion(t *testing.T) {
	requested := false
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requested = true
	}))
	defer server.Close()

	result := checkForUpdates(context.Background(), "unknown", server.URL, server.Client())
	if result.OK || result.Error != "当前应用版本未知" {
		t.Fatalf("expected unknown version error: %+v", result)
	}
	if requested {
		t.Fatal("update check requested release data with unknown current version")
	}
}

func TestFetchAppUpdateInfoParsesManifestAssets(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(request.URL.Path, "/manifest") {
			_, _ = writer.Write([]byte(`{"version":"1.9.6","notesUrl":"https://example.com/notes","force":true,"assets":{"windows-amd64":{"name":"setup.exe","sha256":"abc123"}}}`))
			return
		}
		base := strings.TrimSuffix(server.URL, "/")
		_, _ = writer.Write([]byte(fmt.Sprintf(
			`{"tag_name":"v1.9.6","html_url":"https://example.com/release","assets":[{"name":"setup.exe","browser_download_url":"%s/setup.exe"},{"name":"update-manifest.json","browser_download_url":"%s/manifest"}]}`,
			base, base,
		)))
	}))
	defer server.Close()

	info := fetchAppUpdateInfo(context.Background(), "1.9.5", server.URL, server.Client())
	if !info.OK || !info.UpdateAvailable {
		t.Fatalf("update info = %+v", info)
	}
	asset, ok := info.Assets["windows-amd64"]
	if !ok || asset.Name != "setup.exe" || asset.SHA256 != "abc123" {
		t.Fatalf("windows asset = %+v, want setup.exe/abc123", asset)
	}
	if !strings.HasSuffix(asset.URL, "/setup.exe") {
		t.Fatalf("asset url = %q", asset.URL)
	}
	if !info.Force || info.NotesURL != "https://example.com/notes" {
		t.Fatalf("manifest flags = force:%v notes:%q", info.Force, info.NotesURL)
	}
}

func TestSHA256File(t *testing.T) {
	path := filepath.Join(t.TempDir(), "payload.bin")
	if err := os.WriteFile(path, []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	sum, err := sha256File(path)
	if err != nil {
		t.Fatal(err)
	}
	if sum != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Fatalf("sha256 = %q", sum)
	}
}

func TestCompareUpdateVersionsHandlesPrerelease(t *testing.T) {
	cases := []struct {
		left  string
		right string
		want  int
	}{
		{left: "v1.8.0", right: "1.8.0", want: 0},
		{left: "1.8.0-beta.1", right: "1.8.0", want: -1},
		{left: "1.8.0", right: "1.8.1", want: -1},
		{left: "1.9.0", right: "1.8.1", want: 1},
	}

	for _, item := range cases {
		got, err := compareUpdateVersions(item.left, item.right)
		if err != nil {
			t.Fatalf("compareUpdateVersions(%q, %q) failed: %v", item.left, item.right, err)
		}
		if got != item.want {
			t.Fatalf("compareUpdateVersions(%q, %q) = %d, want %d", item.left, item.right, got, item.want)
		}
	}
}
