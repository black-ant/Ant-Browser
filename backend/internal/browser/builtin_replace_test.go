package browser

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

// 从内置资源读取 spec 的名称与版本，用于构造同名同版本的手动安装目录。
func builtinSpecNameVersion(t *testing.T) (string, string) {
	t.Helper()
	spec := builtinExtensionSpecs[0]
	data, err := builtinExtensionFS.ReadFile(path.Join(builtinExtensionAssetRoot, spec.Dir, "manifest.json"))
	if err != nil {
		t.Fatalf("读取内置清单失败: %v", err)
	}
	var manifest struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("解析内置清单失败: %v", err)
	}
	return manifest.Name, manifest.Version
}

func writeLocalExtensionDir(t *testing.T, name string, version string) string {
	t.Helper()
	dir := t.TempDir()
	manifest := fmt.Sprintf(`{"name":%q,"version":%q,"manifest_version":3,"description":"manual copy"}`, name, version)
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("写入 manifest 失败: %v", err)
	}
	return dir
}

// 手动安装同名同版本插件：未确认时必须取消且不落任何文件/记录；
// 确认后取代内置版本，实例绑定迁移，内置同步跳过；取代者被删除后内置恢复登记。
func TestInstallDirectoryReplacesConflictingBuiltin(t *testing.T) {
	manager := newIsolationManager(t)
	spec := builtinExtensionSpecs[0]
	if err := manager.SyncBuiltinExtensions(); err != nil {
		t.Fatalf("SyncBuiltinExtensions returned error: %v", err)
	}
	name, version := builtinSpecNameVersion(t)
	sourceDir := writeLocalExtensionDir(t, name, version)

	// 未提供确认回调 → 冲突取消，内置记录与文件保留。
	if _, err := manager.InstallExtensionDirectory(sourceDir, nil); !errors.Is(err, ErrBuiltinExtensionConflict) {
		t.Fatalf("未确认时应返回冲突错误，实际: %v", err)
	}
	if _, err := manager.ExtensionDAO.Get(spec.ID); err != nil {
		t.Fatalf("取消安装后内置记录应保留: %v", err)
	}

	// 用户拒绝替换 → 同样取消。
	if _, err := manager.InstallExtensionDirectory(sourceDir, func(Extension) bool { return false }); !errors.Is(err, ErrBuiltinExtensionConflict) {
		t.Fatalf("拒绝替换时应返回冲突错误，实际: %v", err)
	}

	// 实例绑定内置插件，验证替换后引用迁移。
	const profileID = "profile-replace-1"
	if _, err := manager.ExtensionDAO.SetProfileSettings(profileID, []string{spec.ID}, true); err != nil {
		t.Fatalf("SetProfileSettings returned error: %v", err)
	}

	installed, err := manager.InstallExtensionDirectory(sourceDir, func(Extension) bool { return true })
	if err != nil {
		t.Fatalf("确认替换后安装失败: %v", err)
	}
	if installed.ExtensionID == spec.ID {
		t.Fatal("新插件 ID 不应与内置 ID 相同")
	}
	if _, err := manager.ExtensionDAO.Get(spec.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("内置记录应已删除: %v", err)
	}
	if _, err := os.Stat(filepath.Join(manager.ResolveRelativePath(filepath.Join("data", extensionsRootDir)), spec.ID)); !os.IsNotExist(err) {
		t.Fatalf("内置插件目录应已清理: %v", err)
	}
	successor, err := manager.ExtensionDAO.GetBuiltinSupersededBy(spec.ID)
	if err != nil {
		t.Fatalf("读取取代标记失败: %v", err)
	}
	if successor != installed.ExtensionID {
		t.Fatalf("取代标记指向 %q, want %q", successor, installed.ExtensionID)
	}
	settings, err := manager.ExtensionDAO.GetProfileSettings(profileID)
	if err != nil {
		t.Fatalf("GetProfileSettings returned error: %v", err)
	}
	if !settings.Configured || len(settings.ExtensionIDs) != 1 || settings.ExtensionIDs[0] != installed.ExtensionID {
		t.Fatalf("实例绑定应迁移到新插件: %#v", settings.ExtensionIDs)
	}

	// 再次同步：内置不应被重新登记。
	if err := manager.SyncBuiltinExtensions(); err != nil {
		t.Fatalf("SyncBuiltinExtensions returned error: %v", err)
	}
	if _, err := manager.ExtensionDAO.Get(spec.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("被取代的内置插件不应重新登记: %v", err)
	}

	// 删除取代者 → 标记清除 → 下次同步内置恢复登记。
	if err := manager.ExtensionDAO.Delete(installed.ExtensionID); err != nil {
		t.Fatalf("删除取代者失败: %v", err)
	}
	if successor, err := manager.ExtensionDAO.GetBuiltinSupersededBy(spec.ID); err != nil || successor != "" {
		t.Fatalf("删除取代者后标记应清除: successor=%q err=%v", successor, err)
	}
	if err := manager.SyncBuiltinExtensions(); err != nil {
		t.Fatalf("SyncBuiltinExtensions returned error: %v", err)
	}
	if _, err := manager.ExtensionDAO.Get(spec.ID); err != nil {
		t.Fatalf("取代者删除后内置应重新登记: %v", err)
	}
}

// 运行中的实例仍通过 --load-extension 加载内置目录时，替换必须被拒绝，且不做任何改动。
func TestInstallDirectoryRejectsReplacementWhileProfileRunning(t *testing.T) {
	manager := newIsolationManager(t)
	spec := builtinExtensionSpecs[0]
	if err := manager.SyncBuiltinExtensions(); err != nil {
		t.Fatalf("SyncBuiltinExtensions returned error: %v", err)
	}
	const profileID = "profile-replace-running"
	if _, err := manager.ExtensionDAO.SetProfileSettings(profileID, []string{spec.ID}, true); err != nil {
		t.Fatalf("SetProfileSettings returned error: %v", err)
	}
	manager.InitData()
	manager.Profiles[profileID] = &Profile{ProfileId: profileID, ProfileName: "运行中实例", Running: true}
	if err := manager.EnsureExtensionUnusedByRunningProfiles(spec.ID); err == nil {
		t.Fatal("前置条件失败：运行中实例应被视为正在使用内置插件")
	}

	name, version := builtinSpecNameVersion(t)
	sourceDir := writeLocalExtensionDir(t, name, version)

	_, err := manager.InstallExtensionDirectory(sourceDir, func(Extension) bool { return true })
	if err == nil || !strings.Contains(err.Error(), "运行中的实例") {
		t.Fatalf("运行中实例占用内置插件时替换应被拒绝，实际: %v", err)
	}
	if _, err := manager.ExtensionDAO.Get(spec.ID); err != nil {
		t.Fatalf("拒绝替换后内置记录应保留: %v", err)
	}
	builtinDir := filepath.Join(manager.ResolveRelativePath(filepath.Join("data", extensionsRootDir)), spec.ID)
	if _, err := os.Stat(builtinDir); err != nil {
		t.Fatalf("拒绝替换后内置目录应保留: %v", err)
	}
	settings, err := manager.ExtensionDAO.GetProfileSettings(profileID)
	if err != nil {
		t.Fatalf("GetProfileSettings returned error: %v", err)
	}
	if len(settings.ExtensionIDs) != 1 || settings.ExtensionIDs[0] != spec.ID {
		t.Fatalf("拒绝替换后绑定不应变化: %#v", settings.ExtensionIDs)
	}
}

// 替换的结算必须发生在 successor 落库之后：只确认不结算时，绑定与内置记录保持原样。
func TestReplacementKeepsBindingUntilSettled(t *testing.T) {
	manager := newIsolationManager(t)
	spec := builtinExtensionSpecs[0]
	if err := manager.SyncBuiltinExtensions(); err != nil {
		t.Fatalf("SyncBuiltinExtensions returned error: %v", err)
	}
	const profileID = "profile-settle-order"
	if _, err := manager.ExtensionDAO.SetProfileSettings(profileID, []string{spec.ID}, true); err != nil {
		t.Fatalf("SetProfileSettings returned error: %v", err)
	}
	const successorID = "manualcopyidmanualcopyidmanualco"
	name, version := builtinSpecNameVersion(t)
	pending, err := manager.prepareBuiltinReplacement(&Extension{
		ExtensionID: successorID,
		Name:        name,
		Version:     version,
	}, func(Extension) bool { return true })
	if err != nil {
		t.Fatalf("prepareBuiltinReplacement returned error: %v", err)
	}
	if pending == nil {
		t.Fatal("应返回待结算的内置插件记录")
	}
	// 尚未结算（successor 未落库）：绑定必须仍指向内置版本，否则实例会绑定到不存在的插件。
	settings, err := manager.ExtensionDAO.GetProfileSettings(profileID)
	if err != nil {
		t.Fatalf("GetProfileSettings returned error: %v", err)
	}
	if len(settings.ExtensionIDs) != 1 || settings.ExtensionIDs[0] != spec.ID {
		t.Fatalf("结算前绑定不应迁移: %#v", settings.ExtensionIDs)
	}
	if _, err := manager.ExtensionDAO.Get(spec.ID); err != nil {
		t.Fatalf("结算前内置记录应保留: %v", err)
	}

	// successor 落库后结算：绑定迁移、内置记录删除、取代标记写入。
	if err := manager.ExtensionDAO.Upsert(Extension{
		ExtensionID: successorID,
		Name:        name,
		Version:     version,
		InstallMode: ExtensionInstallModePersistent,
		Enabled:     true,
	}); err != nil {
		t.Fatalf("Upsert returned error: %v", err)
	}
	if err := manager.settleBuiltinReplacement(pending, successorID); err != nil {
		t.Fatalf("settleBuiltinReplacement returned error: %v", err)
	}
	settings, err = manager.ExtensionDAO.GetProfileSettings(profileID)
	if err != nil {
		t.Fatalf("GetProfileSettings returned error: %v", err)
	}
	if len(settings.ExtensionIDs) != 1 || settings.ExtensionIDs[0] != successorID {
		t.Fatalf("结算后绑定应迁移到新插件: %#v", settings.ExtensionIDs)
	}
	if _, err := manager.ExtensionDAO.Get(spec.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("结算后内置记录应删除: %v", err)
	}
	if successor, err := manager.ExtensionDAO.GetBuiltinSupersededBy(spec.ID); err != nil || successor != successorID {
		t.Fatalf("结算后应写入取代标记: successor=%q err=%v", successor, err)
	}
}
