package browser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ant-chrome/backend/internal/config"
)

func newIsolationManager(t *testing.T) *Manager {
	t.Helper()
	appRoot := t.TempDir()
	manager := NewManager(config.DefaultConfig(), appRoot)
	manager.ExtensionDAO = newTestExtensionDAO(t, appRoot)
	return manager
}

func writeUnpackedExtension(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(`{"name":"Test","version":"1.0.0","manifest_version":3}`), 0o644); err != nil {
		t.Fatalf("WriteFile manifest returned error: %v", err)
	}
}

// 取消某个实例的插件，不能影响其他实例的绑定。
func TestDisablingExtensionForOneProfileKeepsOthers(t *testing.T) {
	manager := newIsolationManager(t)
	const extensionID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	if err := manager.ExtensionDAO.Upsert(Extension{
		ExtensionID: extensionID,
		Name:        "Shared",
		Version:     "1.0.0",
		InstallMode: ExtensionInstallModePersistent,
		Enabled:     true,
	}); err != nil {
		t.Fatalf("Upsert returned error: %v", err)
	}
	manager.Profiles["profile-a"] = &Profile{ProfileId: "profile-a", ProfileName: "A"}
	manager.Profiles["profile-b"] = &Profile{ProfileId: "profile-b", ProfileName: "B"}
	for _, profileID := range []string{"profile-a", "profile-b"} {
		if _, err := manager.ExtensionDAO.SetProfileSettings(profileID, []string{extensionID}, true); err != nil {
			t.Fatalf("SetProfileSettings returned error: %v", err)
		}
	}

	if _, err := manager.SaveProfileExtensionSettings("profile-a", nil, false); err != nil {
		t.Fatalf("SaveProfileExtensionSettings returned error: %v", err)
	}

	if manager.profileUsesExtension("profile-a", extensionID) {
		t.Fatal("profile-a 仍在使用已取消的插件")
	}
	if !manager.profileUsesExtension("profile-b", extensionID) {
		t.Fatal("profile-b 的插件绑定被其他实例的操作影响了")
	}
}

// 插件正被运行中的实例使用时，禁止删除/禁用，避免打断该实例。
func TestRemoveExtensionRefusedWhileRunningProfileUsesIt(t *testing.T) {
	manager := newIsolationManager(t)
	const extensionID = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	if err := manager.ExtensionDAO.Upsert(Extension{
		ExtensionID: extensionID,
		Name:        "InUse",
		Version:     "1.0.0",
		InstallMode: ExtensionInstallModePersistent,
		Enabled:     true,
	}); err != nil {
		t.Fatalf("Upsert returned error: %v", err)
	}
	manager.Profiles["profile-run"] = &Profile{ProfileId: "profile-run", ProfileName: "运行中"}
	manager.Profiles["profile-idle"] = &Profile{ProfileId: "profile-idle", ProfileName: "空闲"}
	if _, err := manager.ExtensionDAO.SetProfileSettings("profile-run", []string{extensionID}, true); err != nil {
		t.Fatalf("SetProfileSettings returned error: %v", err)
	}
	manager.ProfileRuntimeDetector = func(profile *Profile) bool {
		return profile != nil && profile.ProfileId == "profile-run"
	}

	err := manager.RemoveExtensionFromStoppedProfiles(extensionID)
	if err == nil {
		t.Fatal("运行中的实例正在使用该插件，删除应被拒绝")
	}
	if !strings.Contains(err.Error(), "运行中") {
		t.Fatalf("error = %v, want a running-instance hint", err)
	}
}

// 启动参数只加载该实例自己配置的插件。
func TestProfileExtensionLoadPathsArePerProfile(t *testing.T) {
	manager := newIsolationManager(t)
	dirA := filepath.Join(t.TempDir(), "ext-a")
	dirB := filepath.Join(t.TempDir(), "ext-b")
	writeUnpackedExtension(t, dirA)
	writeUnpackedExtension(t, dirB)

	for _, item := range []struct {
		id  string
		dir string
	}{
		{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", dirA},
		{"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", dirB},
	} {
		if err := manager.ExtensionDAO.Upsert(Extension{
			ExtensionID: item.id,
			Name:        item.id[:4],
			Version:     "1.0.0",
			InstallDir:  item.dir,
			InstallMode: ExtensionInstallModePersistent,
			Enabled:     true,
		}); err != nil {
			t.Fatalf("Upsert returned error: %v", err)
		}
	}

	manager.Profiles["profile-a"] = &Profile{ProfileId: "profile-a", ProfileName: "A"}
	manager.Profiles["profile-b"] = &Profile{ProfileId: "profile-b", ProfileName: "B"}
	if _, err := manager.ExtensionDAO.SetProfileSettings("profile-a", []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}, true); err != nil {
		t.Fatalf("SetProfileSettings returned error: %v", err)
	}
	if _, err := manager.ExtensionDAO.SetProfileSettings("profile-b", []string{"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}, true); err != nil {
		t.Fatalf("SetProfileSettings returned error: %v", err)
	}

	pathsA := manager.ProfileExtensionLoadPaths(manager.Profiles["profile-a"])
	pathsB := manager.ProfileExtensionLoadPaths(manager.Profiles["profile-b"])
	if len(pathsA) != 1 || filepath.Clean(pathsA[0]) != filepath.Clean(dirA) {
		t.Fatalf("pathsA = %#v, want only %s", pathsA, dirA)
	}
	if len(pathsB) != 1 || filepath.Clean(pathsB[0]) != filepath.Clean(dirB) {
		t.Fatalf("pathsB = %#v, want only %s", pathsB, dirB)
	}
}
