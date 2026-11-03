package browser

import (
	"ant-chrome/backend/internal/config"
	"testing"
)

// 「手动设置本实例」但一个插件都不勾选，是合法且有意义的状态：
// 用户明确要求这个实例不加载任何插件。它必须被原样保存，
// 不能折叠成「未配置」——否则启动时会回落到默认安装集合，插件凭空出现。
func TestSetProfileSettingsKeepsExplicitEmptyOverride(t *testing.T) {
	appRoot := t.TempDir()
	dao := newTestExtensionDAO(t, appRoot)

	settings, err := dao.SetProfileSettings("profile-empty", nil, true)
	if err != nil {
		t.Fatalf("SetProfileSettings returned error: %v", err)
	}
	if !settings.Configured {
		t.Fatal("显式选择「零插件」应被记录为手动配置")
	}
	if len(settings.ExtensionIDs) != 0 {
		t.Fatalf("settings.ExtensionIDs = %#v, want empty", settings.ExtensionIDs)
	}

	stored, err := dao.GetProfileSettings("profile-empty")
	if err != nil {
		t.Fatalf("GetProfileSettings returned error: %v", err)
	}
	if !stored.Configured || len(stored.ExtensionIDs) != 0 {
		t.Fatalf("stored settings = %#v, want configured=true without bindings", stored)
	}
}

// 同一份「零插件」配置，在"期望集合"这条路径上也必须是 0 个，
// 与启动加载（ProfileExtensionLoadPaths）保持一致，不能回落到默认安装。
func TestEffectiveProfileExtensionIDsHonorsEmptyOverride(t *testing.T) {
	appRoot := t.TempDir()
	manager := NewManager(config.DefaultConfig(), appRoot)
	manager.ExtensionDAO = newTestExtensionDAO(t, appRoot)

	if err := manager.ExtensionDAO.Upsert(Extension{
		ExtensionID:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Name:           "default-install",
		Version:        "1.0.0",
		InstallMode:    ExtensionInstallModePersistent,
		Enabled:        true,
		DefaultInstall: true,
	}); err != nil {
		t.Fatalf("Upsert returned error: %v", err)
	}

	// configured=true 且没有任何绑定：用户主动选择零插件（也兼容历史脏数据）
	empty := ProfileExtensionSettings{ProfileID: "profile-empty-override", Configured: true}
	ids, err := manager.effectiveProfileExtensionIDs(empty)
	if err != nil {
		t.Fatalf("effectiveProfileExtensionIDs returned error: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("effective ids = %#v, want none", ids)
	}

	// 未配置的实例仍然跟随默认安装集合
	inherit := ProfileExtensionSettings{ProfileID: "profile-inherit"}
	ids, err = manager.effectiveProfileExtensionIDs(inherit)
	if err != nil {
		t.Fatalf("effectiveProfileExtensionIDs returned error: %v", err)
	}
	if len(ids) != 1 || ids[0] != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("effective ids = %#v, want the default install extension", ids)
	}
}
