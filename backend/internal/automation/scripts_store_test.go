package automation

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScriptStoreSaveListAndDelete(t *testing.T) {
	store := NewScriptStore(filepath.Join(t.TempDir(), "data", "automation", "scripts"))

	saved, err := store.Save(ScriptRecord{
		ID:           "buyer-script",
		Name:         "买家脚本",
		Description:  "用于接管页面并截图",
		Type:         "playwright-cdp",
		Status:       "ready",
		EntryFile:    "index.cjs",
		Tags:         []string{"Playwright", "CDP", "Playwright"},
		SelectorText: `{"code":"BUYER_001"}`,
		ParamsText:   `{"url":"https://example.com"}`,
		ScriptText:   "module.exports.run = async () => ({ ok: true })\r\n",
		Notes:        "stable",
	})
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	if saved.ID != "buyer-script" {
		t.Fatalf("expected saved id buyer-script, got %q", saved.ID)
	}
	if saved.PackageFormat != defaultScriptPackageFormat {
		t.Fatalf("expected package format %q, got %q", defaultScriptPackageFormat, saved.PackageFormat)
	}
	if saved.ManifestVersion != defaultScriptManifestVersion {
		t.Fatalf("expected manifest version %d, got %d", defaultScriptManifestVersion, saved.ManifestVersion)
	}
	if len(saved.Tags) != 2 {
		t.Fatalf("expected deduped tags, got %#v", saved.Tags)
	}
	if saved.CreatedAt == "" || saved.UpdatedAt == "" {
		t.Fatalf("expected timestamps to be populated, got %+v", saved)
	}
	if saved.ScriptText != "module.exports.run = async () => ({ ok: true })\n" {
		t.Fatalf("expected normalized script line endings, got %q", saved.ScriptText)
	}

	configPath := filepath.Join(store.rootDir, "buyer-script", scriptStoreConfigFileName)
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("expected config to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(store.rootDir, "buyer-script", scriptStoreLegacyConfigName)); !os.IsNotExist(err) {
		t.Fatalf("expected legacy manifest to be absent, got %v", err)
	}
	scriptPath := filepath.Join(store.rootDir, "buyer-script", "index.cjs")
	if _, err := os.Stat(scriptPath); err != nil {
		t.Fatalf("expected script file to exist: %v", err)
	}

	items, err := store.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one item, got %d", len(items))
	}
	if items[0].ScriptText == "" {
		t.Fatalf("expected script text to be loaded from file")
	}
	if items[0].PackageFormat != defaultScriptPackageFormat {
		t.Fatalf("expected loaded package format %q, got %q", defaultScriptPackageFormat, items[0].PackageFormat)
	}

	if err := store.Delete("buyer-script"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(store.rootDir, "buyer-script")); !os.IsNotExist(err) {
		t.Fatalf("expected script dir to be removed, got %v", err)
	}
}

func TestScriptStoreSaveRenamedEntryRemovesOldFile(t *testing.T) {
	store := NewScriptStore(filepath.Join(t.TempDir(), "data", "automation", "scripts"))

	original, err := store.Save(ScriptRecord{
		ID:         "rename-script",
		Name:       "重命名入口",
		EntryFile:  "index.cjs",
		ScriptText: "module.exports.run = async () => ({ ok: true })",
	})
	if err != nil {
		t.Fatalf("initial Save returned error: %v", err)
	}

	updated, err := store.Save(ScriptRecord{
		ID:         original.ID,
		Name:       original.Name,
		Type:       original.Type,
		Status:     original.Status,
		EntryFile:  "runner.cjs",
		ScriptText: "module.exports.run = async () => ({ ok: false })",
		CreatedAt:  original.CreatedAt,
	})
	if err != nil {
		t.Fatalf("second Save returned error: %v", err)
	}

	if updated.EntryFile != "runner.cjs" {
		t.Fatalf("expected entry file runner.cjs, got %q", updated.EntryFile)
	}
	if updated.PackageFormat != defaultScriptPackageFormat {
		t.Fatalf("expected package format %q, got %q", defaultScriptPackageFormat, updated.PackageFormat)
	}
	if updated.CreatedAt != original.CreatedAt {
		t.Fatalf("expected createdAt to be preserved, got %q want %q", updated.CreatedAt, original.CreatedAt)
	}
	if _, err := os.Stat(filepath.Join(store.rootDir, original.ID, "runner.cjs")); err != nil {
		t.Fatalf("expected new entry file to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(store.rootDir, original.ID, "index.cjs")); !os.IsNotExist(err) {
		t.Fatalf("expected old entry file to be removed, got %v", err)
	}
}

func TestScriptStoreSaveGeneratesIDForNewScript(t *testing.T) {
	store := NewScriptStore(filepath.Join(t.TempDir(), "data", "automation", "scripts"))

	saved, err := store.Save(ScriptRecord{
		Name:       "自动生成 ID",
		ScriptText: "module.exports.run = async () => ({ ok: true })",
	})
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if saved.ID == "" {
		t.Fatalf("expected generated id to be populated")
	}
	if _, err := os.Stat(filepath.Join(store.rootDir, saved.ID)); err != nil {
		t.Fatalf("expected generated script dir to exist: %v", err)
	}
}

func TestScriptStorePersistsTargetConfig(t *testing.T) {
	store := NewScriptStore(filepath.Join(t.TempDir(), "data", "automation", "scripts"))

	saved, err := store.Save(ScriptRecord{
		ID:         "target-config-script",
		Name:       "实例策略脚本",
		EntryFile:  "index.cjs",
		ScriptText: "module.exports.run = async () => ({ ok: true })",
		TargetConfig: ScriptTargetConfig{
			Mode: "rotate",
			Selector: ScriptTargetSelector{
				Tags:     []string{"pool", "pool"},
				Keywords: []string{"buyer"},
			},
		},
	})
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	if saved.TargetConfig.Mode != "rotate" {
		t.Fatalf("expected target mode rotate, got %+v", saved.TargetConfig)
	}
	if len(saved.TargetConfig.Selector.Tags) != 1 {
		t.Fatalf("expected normalized target tags, got %+v", saved.TargetConfig.Selector.Tags)
	}

	loaded, err := store.Get(saved.ID)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if loaded.TargetConfig.Mode != "rotate" {
		t.Fatalf("expected persisted target mode rotate, got %+v", loaded.TargetConfig)
	}
	if len(loaded.TargetConfig.Selector.Tags) != 1 || loaded.TargetConfig.Selector.Tags[0] != "pool" {
		t.Fatalf("unexpected persisted target tags: %+v", loaded.TargetConfig.Selector.Tags)
	}
}

func TestScriptStorePersistsPublicAPIConfig(t *testing.T) {
	store := NewScriptStore(filepath.Join(t.TempDir(), "data", "automation", "scripts"))

	saved, err := store.Save(ScriptRecord{
		ID:         "public-api-script",
		Name:       "对外接口脚本",
		EntryFile:  "index.cjs",
		ScriptText: "module.exports.run = async () => ({ ok: true })",
		PublicAPI: ScriptPublicAPIConfig{
			Enabled:          true,
			Method:           "post",
			Path:             "/API/Automation/Hooks/Mail/Proton First Message",
			RequestMode:      "params-only",
			ResponseMode:     "result-only",
			TimeoutMs:        999,
			RequestBodyText:  "{\n  \"recipientQuery\": \"target@example.com\"\n}",
			ResponseBodyText: "{\n  \"verificationCode\": \"429792\"\n}",
			Variables: []ScriptPublicAPIVariable{
				{Name: "recipientQuery", DefaultValue: "target@example.com", Description: "收件人", Required: true},
				{Name: "recipientQuery", DefaultValue: "duplicate"},
				{Name: "senderEmail", DefaultValue: "otp@tm1.openai.com, noreply@tm.openai.com"},
			},
		},
	})
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	if !saved.PublicAPI.Enabled {
		t.Fatalf("expected public api to be enabled")
	}
	if saved.PublicAPI.Method != "POST" {
		t.Fatalf("expected public api method POST, got %+v", saved.PublicAPI)
	}
	if saved.PublicAPI.Path != "mail/proton-first-message" {
		t.Fatalf("expected normalized public api path, got %+v", saved.PublicAPI)
	}
	if saved.PublicAPI.TimeoutMs != 1000 {
		t.Fatalf("expected normalized public api timeout 1000, got %+v", saved.PublicAPI)
	}
	if len(saved.PublicAPI.Variables) != 2 || saved.PublicAPI.Variables[0].Name != "recipientQuery" || !saved.PublicAPI.Variables[0].Required {
		t.Fatalf("expected normalized public api variables, got %+v", saved.PublicAPI.Variables)
	}

	loaded, err := store.Get(saved.ID)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if loaded.PublicAPI.Path != "mail/proton-first-message" {
		t.Fatalf("unexpected persisted public api path: %+v", loaded.PublicAPI)
	}
	if loaded.PublicAPI.RequestMode != "params-only" || loaded.PublicAPI.ResponseMode != "result-only" {
		t.Fatalf("unexpected persisted public api modes: %+v", loaded.PublicAPI)
	}
	if len(loaded.PublicAPI.Variables) != 2 || loaded.PublicAPI.Variables[1].Name != "senderEmail" {
		t.Fatalf("unexpected persisted public api variables: %+v", loaded.PublicAPI.Variables)
	}
}

func TestScriptStoreRejectsDuplicatePublicAPIPath(t *testing.T) {
	store := NewScriptStore(filepath.Join(t.TempDir(), "data", "automation", "scripts"))

	if _, err := store.Save(ScriptRecord{
		ID:         "hook-a",
		Name:       "Hook A",
		EntryFile:  "index.cjs",
		ScriptText: "module.exports.run = async () => ({ ok: true })",
		PublicAPI: ScriptPublicAPIConfig{
			Enabled: true,
			Path:    "mail/proton-first-message",
		},
	}); err != nil {
		t.Fatalf("initial Save returned error: %v", err)
	}

	if _, err := store.Save(ScriptRecord{
		ID:         "hook-b",
		Name:       "Hook B",
		EntryFile:  "index.cjs",
		ScriptText: "module.exports.run = async () => ({ ok: true })",
		PublicAPI: ScriptPublicAPIConfig{
			Enabled: false,
			Path:    "/api/automation/hooks/mail/proton-first-message",
		},
	}); err == nil {
		t.Fatalf("expected duplicate public api path to fail")
	}
}

func TestScriptStoreReadsLegacyManifestAndMigratesOnSave(t *testing.T) {
	store := NewScriptStore(filepath.Join(t.TempDir(), "data", "automation", "scripts"))
	scriptDir := filepath.Join(store.rootDir, "legacy-script")
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		t.Fatalf("create script dir failed: %v", err)
	}

	legacyConfig := `{
  "packageFormat": "ant-automation-script",
  "manifestVersion": 1,
  "id": "legacy-script",
  "name": "旧结构脚本",
  "type": "playwright-cdp",
  "status": "ready",
  "entryFile": "index.cjs",
  "createdAt": "2026-04-03T00:00:00Z",
  "updatedAt": "2026-04-03T00:00:00Z"
}`
	if err := os.WriteFile(filepath.Join(scriptDir, scriptStoreLegacyConfigName), []byte(legacyConfig), 0o644); err != nil {
		t.Fatalf("write legacy manifest failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(scriptDir, "index.cjs"), []byte("module.exports.run = async () => ({ ok: true })"), 0o644); err != nil {
		t.Fatalf("write script failed: %v", err)
	}

	loaded, err := store.Get("legacy-script")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if loaded.Name != "旧结构脚本" {
		t.Fatalf("unexpected loaded record: %+v", loaded)
	}

	loaded.ScriptText = "module.exports.run = async () => ({ ok: false })"
	saved, err := store.Save(loaded)
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if saved.ID != "legacy-script" {
		t.Fatalf("unexpected saved record: %+v", saved)
	}
	if _, err := os.Stat(filepath.Join(scriptDir, scriptStoreConfigFileName)); err != nil {
		t.Fatalf("expected new config to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(scriptDir, scriptStoreLegacyConfigName)); !os.IsNotExist(err) {
		t.Fatalf("expected legacy manifest to be removed, got %v", err)
	}
}

func TestScriptStoreKeepsManifestAsRegularFile(t *testing.T) {
	store := NewScriptStore(filepath.Join(t.TempDir(), "data", "automation", "scripts"))

	record, err := store.ImportBundle(ImportedBundle{
		Record: ScriptRecord{
			ID:         "regular-manifest",
			Name:       "普通 manifest 文件",
			Type:       "playwright-cdp",
			Status:     "ready",
			EntryFile:  "index.cjs",
			ScriptText: "module.exports.run = async () => ({ ok: true })",
		},
		Files: []ImportedBundleFile{
			{
				Path:    "index.cjs",
				Content: []byte("module.exports.run = async () => ({ ok: true })"),
			},
			{
				Path:    "manifest.json",
				Content: []byte(`{"keep":true}`),
			},
		},
	})
	if err != nil {
		t.Fatalf("ImportBundle returned error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(store.rootDir, record.ID, scriptStoreConfigFileName)); err != nil {
		t.Fatalf("expected system config to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(store.rootDir, record.ID, "manifest.json")); err != nil {
		t.Fatalf("expected manifest.json to be kept as a regular file: %v", err)
	}

	record.ScriptText = "module.exports.run = async () => ({ ok: false })"
	if _, err := store.Save(record); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(store.rootDir, record.ID, "manifest.json")); err != nil {
		t.Fatalf("expected manifest.json to survive Save: %v", err)
	}

	exported, err := store.ExportBundle(record.ID)
	if err != nil {
		t.Fatalf("ExportBundle returned error: %v", err)
	}
	if !hasBundleFile(exported.Files, "manifest.json", []byte(`{"keep":true}`)) {
		t.Fatalf("expected manifest.json in exported files, got %+v", exported.Files)
	}
}
