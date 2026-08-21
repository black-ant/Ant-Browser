package backend

import (
	"os"
	"path/filepath"
	"testing"
)

func writeCacheFixture(t *testing.T, dir string, rel string) string {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("xxxx"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCleanProfileCachesRemovesCacheDirs(t *testing.T) {
	dir := t.TempDir()
	gone := []string{
		writeCacheFixture(t, dir, "Default/Cache/Cache_Data/f_000001"),
		writeCacheFixture(t, dir, "Default/Code Cache/js/a_0"),
		writeCacheFixture(t, dir, "Default/GPUCache/data_0"),
		writeCacheFixture(t, dir, "GrShaderCache/data_0"),
	}

	freed, err := cleanProfileCaches(dir)
	if err != nil {
		t.Fatal(err)
	}
	if freed <= 0 {
		t.Fatal("expected freed bytes > 0")
	}
	for _, p := range gone {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("cache path should be removed: %s", p)
		}
	}
}

// 红线:账号状态与站点可见存储绝不能碰 —— 删了轻则掉登录,重则形成行为异常面。
func TestCleanProfileCachesKeepsAccountState(t *testing.T) {
	dir := t.TempDir()
	writeCacheFixture(t, dir, "Default/Cache/Cache_Data/f_000001") // 保证有东西可删
	keep := []string{
		writeCacheFixture(t, dir, "Default/Cookies"),
		writeCacheFixture(t, dir, "Default/Local Storage/leveldb/000003.log"),
		writeCacheFixture(t, dir, "Default/Service Worker/CacheStorage/x/index"),
		writeCacheFixture(t, dir, "Default/IndexedDB/https_x.com_0.indexeddb.leveldb/1.log"),
		writeCacheFixture(t, dir, "Default/Preferences"),
		writeCacheFixture(t, dir, "Default/Network/Cookies"),
	}

	if _, err := cleanProfileCaches(dir); err != nil {
		t.Fatal(err)
	}
	for _, p := range keep {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("account/site state must be untouched: %s", p)
		}
	}
}

func TestCleanProfileCachesEmptyDirIsNoOp(t *testing.T) {
	dir := t.TempDir()
	freed, err := cleanProfileCaches(dir)
	if err != nil {
		t.Fatal(err)
	}
	if freed != 0 {
		t.Fatalf("expected 0 freed on empty profile, got %d", freed)
	}
}
