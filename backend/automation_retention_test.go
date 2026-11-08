package backend

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeRetentionTestFile(t *testing.T, path string, modTime time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("chtimes %s: %v", path, err)
	}
}

func TestPruneAutomationRunRecords(t *testing.T) {
	root := t.TempDir()
	oldTime := time.Now().AddDate(0, 0, -10)
	newTime := time.Now().AddDate(0, 0, -1)
	writeRetentionTestFile(t, filepath.Join(root, "old.json"), oldTime)
	writeRetentionTestFile(t, filepath.Join(root, "new.json"), newTime)
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write notes: %v", err)
	}

	count, _, err := pruneAutomationRunRecords(root, time.Now().AddDate(0, 0, -7))
	if err != nil {
		t.Fatalf("prune runs: %v", err)
	}
	if count != 1 {
		t.Fatalf("removed = %d, want 1", count)
	}
	if _, err := os.Stat(filepath.Join(root, "old.json")); !os.IsNotExist(err) {
		t.Fatalf("old.json should be removed, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "new.json")); err != nil {
		t.Fatalf("new.json should be kept: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "notes.txt")); err != nil {
		t.Fatalf("non-json file should be kept: %v", err)
	}
}

func TestPruneAutomationArtifacts(t *testing.T) {
	root := t.TempDir()
	oldTime := time.Now().AddDate(0, 0, -10)
	newTime := time.Now().AddDate(0, 0, -1)
	writeRetentionTestFile(t, filepath.Join(root, "gpt-image-v2", "old-run", "a.png"), oldTime)
	writeRetentionTestFile(t, filepath.Join(root, "gpt-image-v2", "new-run", "b.png"), newTime)
	writeRetentionTestFile(t, filepath.Join(root, "other", "old-run", "c.png"), oldTime)
	for _, dir := range []string{
		filepath.Join(root, "gpt-image-v2", "old-run"),
		filepath.Join(root, "other", "old-run"),
	} {
		if err := os.Chtimes(dir, oldTime, oldTime); err != nil {
			t.Fatalf("chtimes %s: %v", dir, err)
		}
	}

	count, _, err := pruneAutomationArtifacts(root, time.Now().AddDate(0, 0, -7))
	if err != nil {
		t.Fatalf("prune artifacts: %v", err)
	}
	if count != 2 {
		t.Fatalf("removed = %d, want 2", count)
	}
	if _, err := os.Stat(filepath.Join(root, "gpt-image-v2", "old-run")); !os.IsNotExist(err) {
		t.Fatalf("old run dir should be removed, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "gpt-image-v2", "new-run", "b.png")); err != nil {
		t.Fatalf("new run file should be kept: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "other")); !os.IsNotExist(err) {
		t.Fatalf("empty script dir should be removed, stat err = %v", err)
	}
}

func TestPruneProxyWorkdirsSkipsInUse(t *testing.T) {
	root := t.TempDir()
	oldTime := time.Now().AddDate(0, 0, -10)
	newTime := time.Now().AddDate(0, 0, -1)
	oldUnused := filepath.Join(root, "old-unused")
	oldInUse := filepath.Join(root, "old-in-use")
	writeRetentionTestFile(t, filepath.Join(oldUnused, "xray-config.json"), oldTime)
	writeRetentionTestFile(t, filepath.Join(oldInUse, "xray-config.json"), oldTime)
	writeRetentionTestFile(t, filepath.Join(root, "new", "xray-config.json"), newTime)
	for _, dir := range []string{oldUnused, oldInUse} {
		if err := os.Chtimes(dir, oldTime, oldTime); err != nil {
			t.Fatalf("chtimes %s: %v", dir, err)
		}
	}

	count, _, err := pruneProxyWorkdirs(root, time.Now().AddDate(0, 0, -3), func(path string) bool {
		return filepath.Clean(path) == filepath.Clean(oldInUse)
	})
	if err != nil {
		t.Fatalf("prune workdirs: %v", err)
	}
	if count != 1 {
		t.Fatalf("removed = %d, want 1", count)
	}
	if _, err := os.Stat(oldUnused); !os.IsNotExist(err) {
		t.Fatalf("unused old dir should be removed, stat err = %v", err)
	}
	if _, err := os.Stat(oldInUse); err != nil {
		t.Fatalf("in-use dir should be kept: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "new")); err != nil {
		t.Fatalf("new dir should be kept: %v", err)
	}
}

func TestRemoveChromiumSingletonLocks(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"SingletonLock", "SingletonSocket", "SingletonCookie", "Other"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	removed := removeChromiumSingletonLocks(dir)
	if len(removed) != 3 {
		t.Fatalf("removed = %#v, want 3 lock files", removed)
	}
	for _, name := range []string{"SingletonLock", "SingletonSocket", "SingletonCookie"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Fatalf("%s should be removed, stat err = %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "Other")); err != nil {
		t.Fatalf("Other should be kept: %v", err)
	}
}

func TestCmdlineContainsPathBoundary(t *testing.T) {
	cases := []struct {
		name    string
		cmdline string
		needle  string
		want    bool
	}{
		{"命令行末尾", "/opt/xray run -c /data/_xray/abc", "/data/_xray/abc", true},
		{"后接配置文件名", "/opt/xray run -c /data/_xray/abc/xray-config.json", "/data/_xray/abc", true},
		{"user-data-dir 空格分隔", "/opt/chrome --user-data-dir=/data/p1 --x", "/data/p1", true},
		{"带引号", "/opt/chrome --user-data-dir=\"/data/p1\"", "/data/p1", true},
		{"前缀不算命中", "/opt/chrome --user-data-dir=/data/p10", "/data/p1", false},
		{"子串不算命中", "/opt/chrome --user-data-dir=/data/xxp1", "/data/p1", false},
	}
	for _, tc := range cases {
		if got := cmdlineContainsPath([]byte(tc.cmdline), []byte(tc.needle)); got != tc.want {
			t.Fatalf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestAutomationRunKeepProfile(t *testing.T) {
	if automationRunKeepProfile(nil) {
		t.Fatal("nil params must not keep profile")
	}
	if automationRunKeepProfile(map[string]any{}) {
		t.Fatal("empty params must not keep profile")
	}
	if automationRunKeepProfile(map[string]any{"keepProfileAfterRun": false}) {
		t.Fatal("false flag must not keep profile")
	}
	if !automationRunKeepProfile(map[string]any{"keepProfileAfterRun": true}) {
		t.Fatal("true flag should keep profile")
	}
	if !automationRunKeepProfile(map[string]any{"stopProfileAfterRun": false}) {
		t.Fatal("stopProfileAfterRun=false should keep profile for compatibility")
	}
	if automationRunKeepProfile(map[string]any{"stopProfileAfterRun": true}) {
		t.Fatal("stopProfileAfterRun=true must not keep profile")
	}
}
