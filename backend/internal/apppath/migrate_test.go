package apppath

import (
	"os"
	"path/filepath"
	"testing"
)

// darwin 下:旧状态目录 ant-browser 应被原子迁移到 ZwBrowser,数据随之搬迁,旧目录消失。
func TestMigrateLegacyStateRootDarwin(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	appRoot := filepath.Join(t.TempDir(), "ZwBrowser.app", "Contents", "MacOS")

	appSupport := filepath.Join(home, "Library", "Application Support")
	legacy := filepath.Join(appSupport, legacyStateDirName)
	if err := os.MkdirAll(filepath.Join(legacy, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "data", "app.db"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	migrated, err := migrateLegacyStateRootForOS(appRoot, "darwin")
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if !migrated {
		t.Fatal("expected migration to happen")
	}
	newRoot := filepath.Join(appSupport, appStateDirName)
	if _, err := os.Stat(filepath.Join(newRoot, "data", "app.db")); err != nil {
		t.Errorf("data 未迁移到新目录: %v", err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Error("迁移后旧目录应不存在")
	}
}

// 新目录已存在时必须跳过,不得覆盖已有数据。
func TestMigrateLegacyStateRootSkipsWhenNewExists(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	appRoot := filepath.Join(t.TempDir(), "ZwBrowser.app", "Contents", "MacOS")
	appSupport := filepath.Join(home, "Library", "Application Support")
	if err := os.MkdirAll(filepath.Join(appSupport, legacyStateDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(appSupport, appStateDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(appSupport, appStateDirName, "keep")
	if err := os.WriteFile(keep, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}

	migrated, err := migrateLegacyStateRootForOS(appRoot, "darwin")
	if err != nil {
		t.Fatal(err)
	}
	if migrated {
		t.Error("新目录已存在时不应迁移")
	}
	if _, err := os.Stat(keep); err != nil {
		t.Error("已存在的新目录必须被保留")
	}
}

// 无旧目录时不迁移(全新安装)。
func TestMigrateLegacyStateRootNoLegacy(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	appRoot := filepath.Join(t.TempDir(), "ZwBrowser.app", "Contents", "MacOS")
	migrated, err := migrateLegacyStateRootForOS(appRoot, "darwin")
	if err != nil {
		t.Fatal(err)
	}
	if migrated {
		t.Error("无旧目录时不应迁移")
	}
}
