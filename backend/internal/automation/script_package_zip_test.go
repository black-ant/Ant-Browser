package automation

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestWriteScriptPackageZipRoundTripsAdditionalFiles(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "demo-package.zip")

	if err := WriteScriptPackageZip(zipPath, ImportedBundle{
		Record: ScriptRecord{
			ID:           "zip-roundtrip",
			Name:         "ZIP 导出",
			Description:  "包含额外文件",
			Type:         "playwright-cdp",
			Status:       "ready",
			EntryFile:    "scripts/index.cjs",
			SelectorText: `{"code":"ZIP_DEMO"}`,
			ParamsText:   `{"url":"https://example.com"}`,
			ScriptText:   "const helper = require('./helpers/helper.cjs')\nmodule.exports.run = async () => helper.run()",
		},
		Files: []ImportedBundleFile{
			{
				Path:    "scripts/index.cjs",
				Content: []byte("const helper = require('./helpers/helper.cjs')\nmodule.exports.run = async () => helper.run()"),
			},
			{
				Path:    "scripts/helpers/helper.cjs",
				Content: []byte("module.exports.run = async () => ({ ok: true })"),
			},
			{
				Path:    "assets/raw.bin",
				Content: []byte{0x00, 0x01, 0x02, 0xff},
			},
		},
	}); err != nil {
		t.Fatalf("WriteScriptPackageZip returned error: %v", err)
	}

	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open zip failed: %v", err)
	}
	defer reader.Close()

	if !zipContainsEntry(reader.File, scriptPackageManifestName) {
		t.Fatalf("expected %s in zip", scriptPackageManifestName)
	}

	imported, err := ImportBundleFromZip(zipPath, "本地文件 "+zipPath)
	if err != nil {
		t.Fatalf("ImportBundleFromZip returned error: %v", err)
	}

	if imported.Record.EntryFile != "scripts/index.cjs" {
		t.Fatalf("unexpected entry file: %s", imported.Record.EntryFile)
	}
	if imported.Record.Source.Type != "local-file" {
		t.Fatalf("unexpected source: %+v", imported.Record.Source)
	}
	if !hasBundleFile(imported.Files, "scripts/helpers/helper.cjs", []byte("module.exports.run = async () => ({ ok: true })")) {
		t.Fatalf("expected helper file to round-trip, got %+v", imported.Files)
	}
	if !hasBundleFile(imported.Files, "assets/raw.bin", []byte{0x00, 0x01, 0x02, 0xff}) {
		t.Fatalf("expected binary file to round-trip, got %+v", imported.Files)
	}
}

func TestImportBundleFromBytesSupportsZipPackage(t *testing.T) {
	zipData := buildScriptPackageZipBytes(t, map[string]string{
		scriptPackageManifestName: `{
  "name": "远程 ZIP 脚本",
  "type": "playwright-cdp",
  "entryFile": "scripts/index.cjs"
}`,
		"scripts/index.cjs": "module.exports.run = async () => ({ ok: true, source: 'zip-bytes' })",
	})

	bundle, err := ImportBundleFromBytes("remote-package.zip", zipData, "远程地址 https://example.com/demo-package.zip")
	if err != nil {
		t.Fatalf("ImportBundleFromBytes returned error: %v", err)
	}

	if bundle.Record.Name != "远程 ZIP 脚本" {
		t.Fatalf("unexpected script name: %s", bundle.Record.Name)
	}
	if bundle.Record.Source.Type != "remote-url" {
		t.Fatalf("unexpected source: %+v", bundle.Record.Source)
	}
	if !strings.Contains(bundle.Record.ScriptText, "zip-bytes") {
		t.Fatalf("unexpected script text: %s", bundle.Record.ScriptText)
	}
}

func TestImportBundleFromZipPreservesPackageIdentityAndPublicAPI(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "proton-package.zip")
	if err := os.WriteFile(zipPath, buildScriptPackageZipBytes(t, map[string]string{
		scriptPackageManifestName: `{
  "format": "ant-automation-script",
  "packageFormat": "ant-automation-script",
  "manifestVersion": 1,
  "id": "proton-mail-first-message",
  "name": "Proton 邮件搜索并读取最新邮件",
  "type": "playwright-cdp",
  "status": "ready",
  "entryFile": "index.cjs",
  "targetConfig": {
    "mode": "manual"
  },
  "publicAPI": {
    "enabled": true,
    "method": "POST",
    "path": "mail/proton-first-message",
    "requestMode": "params-only",
    "responseMode": "envelope",
    "timeoutMs": 120000
  }
}`,
		"index.cjs": "module.exports.run = async () => ({ ok: true })",
	}), 0o644); err != nil {
		t.Fatalf("write zip failed: %v", err)
	}

	bundle, err := ImportBundleFromZip(zipPath, "本地文件 "+zipPath)
	if err != nil {
		t.Fatalf("ImportBundleFromZip returned error: %v", err)
	}

	if bundle.Record.ID != "proton-mail-first-message" {
		t.Fatalf("expected package id to round-trip, got %q", bundle.Record.ID)
	}
	if bundle.Record.Status != "ready" {
		t.Fatalf("expected package status to round-trip, got %q", bundle.Record.Status)
	}
	if !bundle.Record.PublicAPI.Enabled || bundle.Record.PublicAPI.Path != "mail/proton-first-message" {
		t.Fatalf("expected public api to round-trip, got %+v", bundle.Record.PublicAPI)
	}
	if bundle.Record.TargetConfig.Mode != "manual" {
		t.Fatalf("expected target config to round-trip, got %+v", bundle.Record.TargetConfig)
	}
}

func TestImportBundleFromZipSupportsSingleRootDirectory(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "nested.zip")
	if err := os.WriteFile(zipPath, buildScriptPackageZipBytes(t, map[string]string{
		"demo/automation.script.json": `{
  "name": "单根目录 ZIP",
  "type": "playwright-cdp",
  "entryFile": "scripts/index.cjs"
}`,
		"demo/scripts/index.cjs":          "const helper = require('./helpers/helper.cjs')\nmodule.exports.run = async () => helper.run()",
		"demo/scripts/helpers/helper.cjs": "module.exports.run = async () => ({ ok: true })",
		"__MACOSX/demo/._index.cjs":       "ignored",
		"demo/.DS_Store":                  "ignored",
	}), 0o644); err != nil {
		t.Fatalf("write nested zip failed: %v", err)
	}

	bundle, err := ImportBundleFromZip(zipPath, "本地文件 "+zipPath)
	if err != nil {
		t.Fatalf("ImportBundleFromZip returned error: %v", err)
	}

	if bundle.Record.Name != "单根目录 ZIP" {
		t.Fatalf("unexpected script name: %s", bundle.Record.Name)
	}
	if !hasBundleFile(bundle.Files, "scripts/helpers/helper.cjs", []byte("module.exports.run = async () => ({ ok: true })")) {
		t.Fatalf("expected nested helper file, got %+v", bundle.Files)
	}
}

func TestImportBundleFromZipRejectsZipSlip(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "zip-slip.zip")
	if err := os.WriteFile(zipPath, buildScriptPackageZipBytes(t, map[string]string{
		"../evil.cjs": "module.exports.run = async () => ({ ok: false })",
	}), 0o644); err != nil {
		t.Fatalf("write zip failed: %v", err)
	}

	if _, err := ImportBundleFromZip(zipPath, "本地文件 "+zipPath); err == nil || !strings.Contains(err.Error(), "invalid path") {
		t.Fatalf("expected zip slip error, got %v", err)
	}
}

func buildScriptPackageZipBytes(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)

	paths := make([]string, 0, len(files))
	for relativePath := range files {
		paths = append(paths, relativePath)
	}
	sort.Strings(paths)

	for _, relativePath := range paths {
		entry, err := writer.Create(relativePath)
		if err != nil {
			t.Fatalf("create zip entry failed: %v", err)
		}
		if _, err := entry.Write([]byte(files[relativePath])); err != nil {
			t.Fatalf("write zip entry failed: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip writer failed: %v", err)
	}
	return buf.Bytes()
}

func zipContainsEntry(files []*zip.File, target string) bool {
	for _, file := range files {
		if file.Name == target {
			return true
		}
	}
	return false
}
