package automation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportBundleFromBytesSupportsInlineJSONPackage(t *testing.T) {
	bundle, err := ImportBundleFromBytes("demo-script.json", []byte(`{
  "manifest": {
    "name": "远程示例",
    "description": "用于导入测试",
    "type": "playwright-cdp",
    "entryFile": "index.cjs",
    "tags": ["demo", "remote"]
  },
  "selector": {
    "code": "DEMO_001"
  },
  "params": {
    "url": "https://example.com"
  },
  "publicAPI": {
    "enabled": true,
    "path": "mail/demo-script",
    "requestMode": "params-only",
    "responseMode": "result-only",
    "variables": [
      {"name": "recipientQuery", "defaultValue": "target@example.com", "description": "收件人", "required": true},
      {"name": "senderEmail", "defaultValue": "otp@tm1.openai.com, noreply@tm.openai.com"}
    ]
  },
  "script": "module.exports.run = async () => ({ ok: true })"
}`), "远程地址 https://example.com/demo-script.json")
	if err != nil {
		t.Fatalf("ImportBundleFromBytes returned error: %v", err)
	}

	if bundle.Record.Name != "远程示例" {
		t.Fatalf("unexpected script name: %s", bundle.Record.Name)
	}
	if bundle.Record.PackageFormat != defaultScriptPackageFormat {
		t.Fatalf("unexpected package format: %s", bundle.Record.PackageFormat)
	}
	if bundle.Record.ManifestVersion != defaultScriptManifestVersion {
		t.Fatalf("unexpected manifest version: %d", bundle.Record.ManifestVersion)
	}
	if bundle.Record.Type != "playwright-cdp" {
		t.Fatalf("unexpected script type: %s", bundle.Record.Type)
	}
	if bundle.Record.Status != "draft" {
		t.Fatalf("expected imported script to be draft, got %s", bundle.Record.Status)
	}
	if strings.TrimSpace(bundle.Record.SelectorText) != "{\n  \"code\": \"DEMO_001\"\n}" {
		t.Fatalf("unexpected selector text: %s", bundle.Record.SelectorText)
	}
	if len(bundle.Files) != 1 || bundle.Files[0].Path != "index.cjs" {
		t.Fatalf("unexpected bundle files: %+v", bundle.Files)
	}
	if bundle.Record.PublicAPI.Path != "mail/demo-script" || !bundle.Record.PublicAPI.Enabled {
		t.Fatalf("unexpected public api import result: %+v", bundle.Record.PublicAPI)
	}
	if len(bundle.Record.PublicAPI.Variables) != 2 || bundle.Record.PublicAPI.Variables[0].Name != "recipientQuery" || !bundle.Record.PublicAPI.Variables[0].Required {
		t.Fatalf("unexpected public api variables: %+v", bundle.Record.PublicAPI.Variables)
	}
	if bundle.Record.Source.Type != "remote-url" {
		t.Fatalf("expected remote-url source, got %+v", bundle.Record.Source)
	}
	if bundle.Record.Source.URI != "https://example.com/demo-script.json" {
		t.Fatalf("unexpected source uri: %+v", bundle.Record.Source)
	}
	if !strings.Contains(bundle.Record.Notes, "来源: 远程地址 https://example.com/demo-script.json") {
		t.Fatalf("expected notes to include source, got %q", bundle.Record.Notes)
	}
}

func TestImportBundleFromBytesRejectsInvalidJSONTemplate(t *testing.T) {
	if _, err := ImportBundleFromBytes("broken-template.json", []byte(`{"manifest":`), "文本导入"); err == nil {
		t.Fatalf("expected invalid JSON template to fail")
	}
}

func TestImportBundleFromDirectorySupportsNestedEntryAndExtraFiles(t *testing.T) {
	rootDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(rootDir, "scripts", "helpers"), 0o755); err != nil {
		t.Fatalf("create script dir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "automation.script.json"), []byte(`{
  "name": "Git 示例",
  "description": "包含额外依赖文件",
  "type": "playwright-cdp",
  "entryFile": "scripts/index.cjs",
  "selector": {
    "code": "DEMO_GIT"
  }
}`), 0o644); err != nil {
		t.Fatalf("write manifest failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "scripts", "index.cjs"), []byte(`const helper = require('./helpers/helper.cjs')

module.exports.run = async () => helper.run()`), 0o644); err != nil {
		t.Fatalf("write entry file failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "scripts", "helpers", "helper.cjs"), []byte(`module.exports.run = async () => ({ ok: true })`), 0o644); err != nil {
		t.Fatalf("write helper file failed: %v", err)
	}

	bundle, err := ImportBundleFromDirectory(rootDir, "", "Git https://example.com/demo.git")
	if err != nil {
		t.Fatalf("ImportBundleFromDirectory returned error: %v", err)
	}

	if bundle.Record.EntryFile != "scripts/index.cjs" {
		t.Fatalf("unexpected entry file: %s", bundle.Record.EntryFile)
	}
	if bundle.Record.Source.Type != "git" {
		t.Fatalf("expected git source, got %+v", bundle.Record.Source)
	}
	if !strings.Contains(bundle.Record.ScriptText, "require('./helpers/helper.cjs')") {
		t.Fatalf("unexpected script text: %s", bundle.Record.ScriptText)
	}

	paths := make([]string, 0, len(bundle.Files))
	for _, file := range bundle.Files {
		paths = append(paths, file.Path)
	}
	if !containsString(paths, "scripts/index.cjs") || !containsString(paths, "scripts/helpers/helper.cjs") {
		t.Fatalf("expected nested files to be included, got %+v", paths)
	}
}

func TestScriptStoreImportBundlePersistsNestedFiles(t *testing.T) {
	store := NewScriptStore(filepath.Join(t.TempDir(), "data", "automation", "scripts"))

	record, err := store.ImportBundle(ImportedBundle{
		Record: ScriptRecord{
			ID:         "git-imported",
			Name:       "Git 导入脚本",
			Type:       "playwright-cdp",
			Status:     "draft",
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
	})
	if err != nil {
		t.Fatalf("ImportBundle returned error: %v", err)
	}

	if record.EntryFile != "scripts/index.cjs" {
		t.Fatalf("unexpected entry file: %s", record.EntryFile)
	}
	if record.PackageFormat != defaultScriptPackageFormat {
		t.Fatalf("unexpected package format: %s", record.PackageFormat)
	}
	if _, err := os.Stat(filepath.Join(store.rootDir, "git-imported", "scripts", "helpers", "helper.cjs")); err != nil {
		t.Fatalf("expected helper file to exist: %v", err)
	}

	loaded, err := store.Get("git-imported")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if loaded.EntryFile != "scripts/index.cjs" {
		t.Fatalf("unexpected loaded entry file: %s", loaded.EntryFile)
	}
	if !strings.Contains(loaded.ScriptText, "helper.run") {
		t.Fatalf("unexpected loaded script text: %s", loaded.ScriptText)
	}
}

func TestImportBundleFromDirectoryRejectsNodeModules(t *testing.T) {
	rootDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(rootDir, "node_modules", "left-pad"), 0o755); err != nil {
		t.Fatalf("create node_modules failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "automation.script.json"), []byte(`{
  "name": "Bad Package",
  "type": "playwright-cdp",
  "entryFile": "index.cjs"
}`), 0o644); err != nil {
		t.Fatalf("write manifest failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "index.cjs"), []byte(`module.exports.run = async () => ({ ok: true })`), 0o644); err != nil {
		t.Fatalf("write entry file failed: %v", err)
	}

	if _, err := ImportBundleFromDirectory(rootDir, "", "本地目录 "+rootDir); err == nil || !strings.Contains(err.Error(), "node_modules") {
		t.Fatalf("expected node_modules validation error, got %v", err)
	}
}

func TestImportBundleFromDirectoryRejectsPackageJSONDependencies(t *testing.T) {
	rootDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootDir, "automation.script.json"), []byte(`{
  "name": "Bad Package",
  "type": "playwright-cdp",
  "entryFile": "index.cjs"
}`), 0o644); err != nil {
		t.Fatalf("write manifest failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "package.json"), []byte(`{
  "name": "bad-package",
  "dependencies": {
    "axios": "^1.0.0"
  }
}`), 0o644); err != nil {
		t.Fatalf("write package.json failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "index.cjs"), []byte(`module.exports.run = async () => ({ ok: true })`), 0o644); err != nil {
		t.Fatalf("write entry file failed: %v", err)
	}

	if _, err := ImportBundleFromDirectory(rootDir, "", "本地目录 "+rootDir); err == nil || !strings.Contains(err.Error(), "dependencies") {
		t.Fatalf("expected package dependencies validation error, got %v", err)
	}
}

func TestImportBundleFromDirectoryRejectsExternalDependencySpecifier(t *testing.T) {
	rootDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootDir, "automation.script.json"), []byte(`{
  "name": "External Dependency",
  "type": "playwright-cdp",
  "entryFile": "index.cjs"
}`), 0o644); err != nil {
		t.Fatalf("write manifest failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "index.cjs"), []byte(`const axios = require('axios')
module.exports.run = async () => ({ ok: !!axios })`), 0o644); err != nil {
		t.Fatalf("write entry file failed: %v", err)
	}

	if _, err := ImportBundleFromDirectory(rootDir, "", "本地目录 "+rootDir); err == nil || !strings.Contains(err.Error(), `axios`) {
		t.Fatalf("expected external dependency validation error, got %v", err)
	}
}

func TestImportBundleFromDirectoryRejectsMissingLocalDependency(t *testing.T) {
	rootDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootDir, "automation.script.json"), []byte(`{
  "name": "Missing Local Dependency",
  "type": "playwright-cdp",
  "entryFile": "index.cjs"
}`), 0o644); err != nil {
		t.Fatalf("write manifest failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "index.cjs"), []byte(`const helper = require('./helpers/helper.cjs')
module.exports.run = async () => helper.run()`), 0o644); err != nil {
		t.Fatalf("write entry file failed: %v", err)
	}

	if _, err := ImportBundleFromDirectory(rootDir, "", "本地目录 "+rootDir); err == nil || !strings.Contains(err.Error(), "本地依赖") {
		t.Fatalf("expected missing local dependency validation error, got %v", err)
	}
}

func TestImportBundleFromDirectoryRejectsTypeScriptEntryWhenBuildDisabled(t *testing.T) {
	rootDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootDir, "automation.script.json"), []byte(`{
  "name": "TypeScript Entry",
  "type": "playwright-cdp",
  "entryFile": "index.ts"
}`), 0o644); err != nil {
		t.Fatalf("write manifest failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "index.ts"), []byte(`export async function run() { return { ok: true } }`), 0o644); err != nil {
		t.Fatalf("write entry file failed: %v", err)
	}

	if _, err := ImportBundleFromDirectory(rootDir, "", "本地目录 "+rootDir); err == nil || !strings.Contains(err.Error(), "未开启 TypeScript 脚本构建支持") {
		t.Fatalf("expected disabled TypeScript build error, got %v", err)
	}
}

func TestImportBundleFromBytesBuildsTypeScriptWhenEnabled(t *testing.T) {
	bundle, err := ImportBundleFromBytesWithOptions("demo-script.ts", []byte(`export async function run() { return { ok: true, source: 'ts-single-file' } }`), "本地文件 demo-script.ts", ImportOptions{
		AllowTypeScriptBuild: true,
	})
	if err != nil {
		t.Fatalf("ImportBundleFromBytesWithOptions returned error: %v", err)
	}

	if bundle.Record.EntryFile != "demo-script.cjs" {
		t.Fatalf("unexpected compiled entry file: %s", bundle.Record.EntryFile)
	}
	if !strings.Contains(bundle.Record.ScriptText, "ts-single-file") {
		t.Fatalf("unexpected compiled script text: %s", bundle.Record.ScriptText)
	}
	if hasTypeScriptSource(bundle.Files) {
		t.Fatalf("expected built bundle to omit TypeScript sources, got %+v", bundle.Files)
	}
}

func TestImportBundleFromDirectoryBuildsTypeScriptEntryWhenEnabled(t *testing.T) {
	rootDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(rootDir, "scripts", "helpers"), 0o755); err != nil {
		t.Fatalf("create helper dir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "automation.script.json"), []byte(`{
  "name": "TypeScript Entry",
  "type": "playwright-cdp",
  "entryFile": "scripts/index.ts"
}`), 0o644); err != nil {
		t.Fatalf("write manifest failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "scripts", "index.ts"), []byte(`import { helperValue } from './helpers/helper'

export async function run() {
  return { ok: helperValue, source: 'ts-dir' }
}`), 0o644); err != nil {
		t.Fatalf("write entry file failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "scripts", "helpers", "helper.ts"), []byte(`export const helperValue = true`), 0o644); err != nil {
		t.Fatalf("write helper file failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "assets.json"), []byte(`{"ok":true}`), 0o644); err != nil {
		t.Fatalf("write asset file failed: %v", err)
	}

	bundle, err := ImportBundleFromDirectoryWithOptions(rootDir, "", "本地目录 "+rootDir, ImportOptions{
		AllowTypeScriptBuild: true,
	})
	if err != nil {
		t.Fatalf("ImportBundleFromDirectoryWithOptions returned error: %v", err)
	}

	if bundle.Record.EntryFile != "scripts/index.cjs" {
		t.Fatalf("unexpected compiled entry file: %s", bundle.Record.EntryFile)
	}
	if !strings.Contains(bundle.Record.ScriptText, "ts-dir") {
		t.Fatalf("unexpected compiled script text: %s", bundle.Record.ScriptText)
	}
	if hasTypeScriptSource(bundle.Files) {
		t.Fatalf("expected built bundle to omit TypeScript sources, got %+v", bundle.Files)
	}
	if !hasBundleFile(bundle.Files, "assets.json", []byte(`{"ok":true}`)) {
		t.Fatalf("expected non-TypeScript asset to be preserved, got %+v", bundle.Files)
	}
}

func TestImportBundleFromDirectoryAllowsBuiltinsAndPlaywrightModules(t *testing.T) {
	rootDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(rootDir, "helpers"), 0o755); err != nil {
		t.Fatalf("create helper dir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "automation.script.json"), []byte(`{
  "name": "Supported Dependencies",
  "type": "playwright-cdp",
  "entryFile": "index.cjs"
}`), 0o644); err != nil {
		t.Fatalf("write manifest failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "index.cjs"), []byte(`const fs = require('fs')
const path = require('node:path')
const playwright = require('playwright')
const core = require('playwright-core')
const helper = require('./helpers/helper.cjs')

module.exports.run = async () => ({
  ok: !!fs && !!path && !!playwright && !!core && helper.ok,
})`), 0o644); err != nil {
		t.Fatalf("write entry file failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "helpers", "helper.cjs"), []byte(`module.exports = { ok: true }`), 0o644); err != nil {
		t.Fatalf("write helper file failed: %v", err)
	}

	bundle, err := ImportBundleFromDirectory(rootDir, "", "本地目录 "+rootDir)
	if err != nil {
		t.Fatalf("expected supported package to import, got %v", err)
	}
	if bundle.Record.Name != "Supported Dependencies" {
		t.Fatalf("unexpected imported record: %+v", bundle.Record)
	}
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func hasTypeScriptSource(files []ImportedBundleFile) bool {
	for _, file := range files {
		if isTypeScriptSourceFile(file.Path) {
			return true
		}
	}
	return false
}
