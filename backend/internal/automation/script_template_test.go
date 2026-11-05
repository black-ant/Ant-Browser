package automation

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestMarshalScriptTemplateRoundTripsAdditionalFiles(t *testing.T) {
	templateData, err := MarshalScriptTemplate(ImportedBundle{
		Record: ScriptRecord{
			ID:           "template-roundtrip",
			Name:         "模板导出",
			Description:  "包含额外文件",
			Type:         "playwright-cdp",
			Status:       "ready",
			EntryFile:    "scripts/index.cjs",
			SelectorText: `{"code":"DEMO_TEMPLATE"}`,
			ParamsText:   `{"url":"https://example.com"}`,
			ScriptText:   "const helper = require('./helpers/helper.cjs')\nmodule.exports.run = async () => helper.run()",
			PublicAPI: ScriptPublicAPIConfig{
				Enabled:          true,
				Path:             "mail/template-roundtrip",
				RequestMode:      "params-only",
				ResponseMode:     "result-only",
				RequestBodyText:  "{\n  \"recipientQuery\": \"target@example.com\"\n}",
				ResponseBodyText: "{\n  \"verificationCode\": \"429792\"\n}",
				Variables: []ScriptPublicAPIVariable{
					{Name: "recipientQuery", DefaultValue: "target@example.com", Description: "收件人", Required: true},
				},
			},
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
				Path:    "manifest.json",
				Content: []byte(`{"custom":true}`),
			},
			{
				Path:    "assets/raw.bin",
				Content: []byte{0x00, 0x01, 0x02, 0xff},
			},
		},
	})
	if err != nil {
		t.Fatalf("MarshalScriptTemplate returned error: %v", err)
	}

	imported, err := ImportBundleFromBytes("template.json", templateData, "文本导入")
	if err != nil {
		t.Fatalf("ImportBundleFromBytes returned error: %v", err)
	}

	if imported.Record.EntryFile != "scripts/index.cjs" {
		t.Fatalf("unexpected entry file: %s", imported.Record.EntryFile)
	}
	if !bytes.Equal([]byte(imported.Record.ScriptText), []byte("const helper = require('./helpers/helper.cjs')\nmodule.exports.run = async () => helper.run()")) {
		t.Fatalf("unexpected script text: %q", imported.Record.ScriptText)
	}
	if !hasBundleFile(imported.Files, "scripts/helpers/helper.cjs", []byte("module.exports.run = async () => ({ ok: true })")) {
		t.Fatalf("expected helper file to round-trip, got %+v", imported.Files)
	}
	if !hasBundleFile(imported.Files, "manifest.json", []byte(`{"custom":true}`)) {
		t.Fatalf("expected manifest.json to round-trip as a regular file, got %+v", imported.Files)
	}
	if !hasBundleFile(imported.Files, "assets/raw.bin", []byte{0x00, 0x01, 0x02, 0xff}) {
		t.Fatalf("expected binary file to round-trip, got %+v", imported.Files)
	}
	if imported.Record.PublicAPI.Path != "mail/template-roundtrip" || !imported.Record.PublicAPI.Enabled {
		t.Fatalf("expected public api to round-trip, got %+v", imported.Record.PublicAPI)
	}
	if len(imported.Record.PublicAPI.Variables) != 1 || imported.Record.PublicAPI.Variables[0].Name != "recipientQuery" {
		t.Fatalf("expected public api variables to round-trip, got %+v", imported.Record.PublicAPI.Variables)
	}
}

func TestScriptStoreExportBundleIncludesNestedFiles(t *testing.T) {
	store := NewScriptStore(filepath.Join(t.TempDir(), "data", "automation", "scripts"))

	if _, err := store.ImportBundle(ImportedBundle{
		Record: ScriptRecord{
			ID:         "export-bundle",
			Name:       "导出 bundle",
			Type:       "playwright-cdp",
			Status:     "ready",
			EntryFile:  "scripts/index.cjs",
			ScriptText: "const helper = require('./helpers/helper.cjs')\nmodule.exports.run = async () => helper.run()",
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
		},
	}); err != nil {
		t.Fatalf("ImportBundle returned error: %v", err)
	}

	exported, err := store.ExportBundle("export-bundle")
	if err != nil {
		t.Fatalf("ExportBundle returned error: %v", err)
	}

	if exported.Record.ID != "export-bundle" {
		t.Fatalf("unexpected exported record: %+v", exported.Record)
	}
	if !hasBundleFile(exported.Files, "scripts/index.cjs", []byte("const helper = require('./helpers/helper.cjs')\nmodule.exports.run = async () => helper.run()")) {
		t.Fatalf("expected entry file in exported bundle, got %+v", exported.Files)
	}
	if !hasBundleFile(exported.Files, "scripts/helpers/helper.cjs", []byte("module.exports.run = async () => ({ ok: true })")) {
		t.Fatalf("expected helper file in exported bundle, got %+v", exported.Files)
	}
}

func hasBundleFile(files []ImportedBundleFile, targetPath string, expectedContent []byte) bool {
	for _, file := range files {
		if file.Path == targetPath && bytes.Equal(file.Content, expectedContent) {
			return true
		}
	}
	return false
}
