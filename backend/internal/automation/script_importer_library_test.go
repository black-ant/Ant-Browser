package automation

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDiscoverImportableScriptDirectoriesWithOptions(t *testing.T) {
	rootDir := t.TempDir()

	writeLibraryPackageFile(t, filepath.Join(rootDir, "manifest-script", "automation.script.json"), `{
  "name": "Manifest Script",
  "type": "playwright-cdp",
  "entryFile": "index.cjs"
}`)
	writeLibraryPackageFile(t, filepath.Join(rootDir, "manifest-script", "index.cjs"), "module.exports.run = async () => ({ ok: true, source: 'manifest' })")

	writeLibraryPackageFile(t, filepath.Join(rootDir, "entry-script", "index.cjs"), "module.exports.run = async () => ({ ok: true, source: 'entry' })")
	writeLibraryPackageFile(t, filepath.Join(rootDir, "entry-no-run", "index.ts"), "export const value = 'not-a-script'")

	writeLibraryPackageFile(t, filepath.Join(rootDir, "nested", "child-script", "automation.script.json"), `{
  "name": "Nested Script",
  "type": "playwright-cdp",
  "entryFile": "index.cjs"
}`)
	writeLibraryPackageFile(t, filepath.Join(rootDir, "nested", "child-script", "index.cjs"), "module.exports.run = async () => ({ ok: true, source: 'nested' })")

	writeLibraryPackageFile(t, filepath.Join(rootDir, "parent-script", "automation.script.json"), `{
  "name": "Parent Script",
  "type": "playwright-cdp",
  "entryFile": "index.cjs"
}`)
	writeLibraryPackageFile(t, filepath.Join(rootDir, "parent-script", "index.cjs"), "module.exports.run = async () => ({ ok: true, source: 'parent' })")
	writeLibraryPackageFile(t, filepath.Join(rootDir, "parent-script", "child-script", "automation.script.json"), `{
  "name": "Child Script",
  "type": "playwright-cdp",
  "entryFile": "index.cjs"
}`)
	writeLibraryPackageFile(t, filepath.Join(rootDir, "parent-script", "child-script", "index.cjs"), "module.exports.run = async () => ({ ok: true, source: 'child' })")

	writeLibraryPackageFile(t, filepath.Join(rootDir, ".git", "ignored-script", "automation.script.json"), `{"name":"Ignored","entryFile":"index.cjs"}`)
	writeLibraryPackageFile(t, filepath.Join(rootDir, "node_modules", "ignored-script", "automation.script.json"), `{"name":"Ignored","entryFile":"index.cjs"}`)

	directories, err := DiscoverImportableScriptDirectoriesWithOptions(rootDir)
	if err != nil {
		t.Fatalf("DiscoverImportableScriptDirectoriesWithOptions returned error: %v", err)
	}

	expected := []string{
		filepath.Join(rootDir, "entry-script"),
		filepath.Join(rootDir, "manifest-script"),
		filepath.Join(rootDir, "nested", "child-script"),
		filepath.Join(rootDir, "parent-script"),
	}
	if !reflect.DeepEqual(directories, expected) {
		t.Fatalf("unexpected discovered directories:\nwant: %#v\ngot:  %#v", expected, directories)
	}
}

func writeLibraryPackageFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create package directory failed: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write package file failed: %v", err)
	}
}
