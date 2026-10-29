package browser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSyncBuiltinExtensionsInstallsAsDefaultInstall(t *testing.T) {
	manager := newIsolationManager(t)
	spec := builtinExtensionSpecs[0]

	if err := manager.SyncBuiltinExtensions(); err != nil {
		t.Fatalf("SyncBuiltinExtensions returned error: %v", err)
	}

	extension, err := manager.ExtensionDAO.Get(spec.ID)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if !extension.Enabled || !extension.DefaultInstall {
		t.Fatalf("extension = %#v, want enabled and default install", extension)
	}
	manifestPath := filepath.Join(extension.InstallDir, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("内置插件未解包: %v", err)
	}
	var manifest struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("manifest 解析失败: %v", err)
	}
	if extension.Version != manifest.Version {
		t.Fatalf("version = %q, want %q", extension.Version, manifest.Version)
	}
}

// 重复启动不能覆盖用户对内置插件的开关设置。
func TestSyncBuiltinExtensionsKeepsUserSettings(t *testing.T) {
	manager := newIsolationManager(t)
	spec := builtinExtensionSpecs[0]

	if err := manager.SyncBuiltinExtensions(); err != nil {
		t.Fatalf("SyncBuiltinExtensions returned error: %v", err)
	}
	if err := manager.ExtensionDAO.SetDefaultInstall(spec.ID, false); err != nil {
		t.Fatalf("SetDefaultInstall returned error: %v", err)
	}
	if err := manager.ExtensionDAO.SetEnabled(spec.ID, false); err != nil {
		t.Fatalf("SetEnabled returned error: %v", err)
	}

	if err := manager.SyncBuiltinExtensions(); err != nil {
		t.Fatalf("SyncBuiltinExtensions (second) returned error: %v", err)
	}

	extension, err := manager.ExtensionDAO.Get(spec.ID)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if extension.DefaultInstall {
		t.Fatal("重复同步把用户的默认安装设置改回去了")
	}
	if extension.Enabled {
		t.Fatal("重复同步把用户的启用状态改回去了")
	}
}
