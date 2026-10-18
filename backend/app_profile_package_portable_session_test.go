package backend

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
)

func TestPortableProfileSessionRoundTrip(t *testing.T) {
	cookies := []CookieInfo{
		{
			Name:         "session",
			Value:        "sensitive-cookie-value",
			Domain:       ".example.com",
			Path:         "/",
			Expires:      1_800_000_000,
			HttpOnly:     true,
			Secure:       true,
			SameSite:     "Lax",
			Priority:     "High",
			SourceScheme: "Secure",
			SourcePort:   443,
			PartitionKey: json.RawMessage(`{"topLevelSite":"https://example.com"}`),
		},
	}

	encrypted, err := encryptPortableProfileSession("profile-1", cookies, "portable-password")
	if err != nil {
		t.Fatalf("encryptPortableProfileSession returned error: %v", err)
	}
	if bytes.Contains(encrypted, []byte("sensitive-cookie-value")) {
		t.Fatal("encrypted portable session contains the plaintext cookie value")
	}

	decoded, err := decryptPortableProfileSession("profile-1", encrypted, "portable-password")
	if err != nil {
		t.Fatalf("decryptPortableProfileSession returned error: %v", err)
	}
	if !reflect.DeepEqual(decoded, cookies) {
		t.Fatalf("decoded cookies = %#v, want %#v", decoded, cookies)
	}

	if _, err := decryptPortableProfileSession("profile-1", encrypted, "wrong-password"); err == nil {
		t.Fatal("wrong password should fail to decrypt portable session")
	}
}

func TestPortableProfileSessionPasswordValidation(t *testing.T) {
	if err := validatePortableSessionPassword("1234567"); err == nil {
		t.Fatal("short migration password should be rejected")
	}
	if err := validatePortableSessionPassword("12345678"); err != nil {
		t.Fatalf("valid migration password returned error: %v", err)
	}
}

func TestOpenProfilePackageContentsDetectsPortableSession(t *testing.T) {
	root := t.TempDir()
	zipPath := filepath.Join(root, "portable-profile.zip")
	encrypted, err := encryptPortableProfileSession("profile-1", nil, "portable-password")
	if err != nil {
		t.Fatalf("encryptPortableProfileSession returned error: %v", err)
	}

	file, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("create zip failed: %v", err)
	}
	writer := zip.NewWriter(file)
	writeJSONToZip(t, writer, "manifest.json", ProfilePackageManifest{
		Format:       profilePackageFormat,
		Version:      1,
		ProfileCount: 1,
	})
	writeJSONToZip(t, writer, "profiles.json", []browser.Profile{{
		ProfileId:   "profile-1",
		ProfileName: "可迁移实例",
	}})
	entry, err := writer.Create(portableSessionArchivePath("profile-1"))
	if err != nil {
		t.Fatalf("create portable session entry failed: %v", err)
	}
	if _, err := entry.Write(encrypted); err != nil {
		t.Fatalf("write portable session entry failed: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip writer failed: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close zip file failed: %v", err)
	}

	contents, err := openProfilePackageContents(zipPath)
	if err != nil {
		t.Fatalf("openProfilePackageContents returned error: %v", err)
	}
	defer contents.Reader.Close()
	if len(contents.PortableSessionEntries) != 1 {
		t.Fatalf("portable session entries = %d, want 1", len(contents.PortableSessionEntries))
	}
	if _, ok := contents.PortableSessionEntries["profile-1"]; !ok {
		t.Fatalf("portable session entry keys = %v", contents.PortableSessionEntries)
	}

	previewApp := NewApp(root)
	previewApp.config = config.DefaultConfig()
	previewApp.browserMgr = browser.NewManager(previewApp.config, root)
	preview, err := previewApp.prepareProfilePackageImportFromPath(zipPath)
	if err != nil {
		t.Fatalf("prepareProfilePackageImportFromPath returned error: %v", err)
	}
	if !preview.PortableSessionAvailable || preview.PortableSessionProfileCount != 1 {
		t.Fatalf("portable session preview = %#v", preview)
	}
}
