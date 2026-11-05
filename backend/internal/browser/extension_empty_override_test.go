package browser

import (
	"ant-chrome/backend/internal/config"
	"testing"
)

func TestSetProfileSettingsFoldsEmptyManualListToInherit(t *testing.T) {
	appRoot := t.TempDir()
	dao := newTestExtensionDAO(t, appRoot)

	settings, err := dao.SetProfileSettings("profile-empty", nil, true)
	if err != nil {
		t.Fatalf("SetProfileSettings returned error: %v", err)
	}
	if settings.Configured {
		t.Fatal("空的手动列表不应被记录为手动配置")
	}

	stored, err := dao.GetProfileSettings("profile-empty")
	if err != nil {
		t.Fatalf("GetProfileSettings returned error: %v", err)
	}
	if stored.Configured || len(stored.ExtensionIDs) != 0 {
		t.Fatalf("stored settings = %#v, want inherit without bindings", stored)
	}
}

func TestEffectiveProfileExtensionIDsFallsBackToDefaultForLegacyEmptyOverride(t *testing.T) {
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

	// 模拟升级前遗留的脏数据：configured=true 但没有任何绑定。
	legacy := ProfileExtensionSettings{ProfileID: "profile-legacy", Configured: true}
	ids, err := manager.effectiveProfileExtensionIDs(legacy)
	if err != nil {
		t.Fatalf("effectiveProfileExtensionIDs returned error: %v", err)
	}
	if len(ids) != 1 || ids[0] != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("effective ids = %#v, want the default install extension", ids)
	}
}
