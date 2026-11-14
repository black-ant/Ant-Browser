package browser

import (
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/database"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeleteCoreRejectsExplicitlyReferencedCore(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Browser.Cores = []config.BrowserCore{
		{CoreId: "core-a", CoreName: "A", CorePath: "a", IsDefault: true},
		{CoreId: "core-b", CoreName: "B", CorePath: "b"},
	}
	manager := NewManager(cfg, t.TempDir())
	manager.Profiles["profile-1"] = &Profile{ProfileId: "profile-1", CoreId: "core-b"}

	err := manager.DeleteCore("core-b")
	if err == nil || !strings.Contains(err.Error(), "正在被 1 个实例使用") {
		t.Fatalf("DeleteCore error = %v, want referenced-core rejection", err)
	}
	if _, found := manager.GetCore("core-b"); !found {
		t.Fatal("referenced core was deleted")
	}
}

func TestDeleteCoreRejectsDefaultCoreReferencedImplicitly(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Browser.Cores = []config.BrowserCore{
		{CoreId: "core-a", CoreName: "A", CorePath: "a", IsDefault: true},
		{CoreId: "core-b", CoreName: "B", CorePath: "b"},
	}
	manager := NewManager(cfg, t.TempDir())
	manager.Profiles["profile-1"] = &Profile{ProfileId: "profile-1", CoreId: ""}

	err := manager.DeleteCore("core-a")
	if err == nil || !strings.Contains(err.Error(), "正在被 1 个实例使用") {
		t.Fatalf("DeleteCore error = %v, want implicit-default reference rejection", err)
	}
	if _, found := manager.GetCore("core-a"); !found {
		t.Fatal("implicitly referenced default core was deleted")
	}
}

func TestDeleteCoreRejectsDatabaseReferencesWhenProfilesAreNotLoaded(t *testing.T) {
	db, err := database.NewDB(filepath.Join(t.TempDir(), "core-reference.db"))
	if err != nil {
		t.Fatalf("NewDB returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Browser.Cores = []config.BrowserCore{
		{CoreId: "core-a", CoreName: "A", CorePath: "a", IsDefault: true},
		{CoreId: "core-b", CoreName: "B", CorePath: "b"},
	}
	manager := NewManager(cfg, t.TempDir())
	manager.ProfileDAO = NewSQLiteProfileDAO(db.GetConn())
	for _, profile := range []*Profile{
		{ProfileId: "profile-explicit", ProfileName: "explicit", CoreId: "core-b"},
		{ProfileId: "profile-default", ProfileName: "default", CoreId: ""},
	} {
		if err := manager.ProfileDAO.Upsert(profile); err != nil {
			t.Fatalf("Upsert profile %s returned error: %v", profile.ProfileId, err)
		}
	}
	if len(manager.Profiles) != 0 {
		t.Fatalf("manager profiles = %#v, want no in-memory profiles", manager.Profiles)
	}

	for _, coreId := range []string{"core-a", "core-b"} {
		err := manager.DeleteCore(coreId)
		if err == nil || !strings.Contains(err.Error(), "正在被 1 个实例使用") {
			t.Fatalf("DeleteCore(%q) error = %v, want database reference rejection", coreId, err)
		}
		if _, found := manager.GetCore(coreId); !found {
			t.Fatalf("database-referenced core %q was deleted", coreId)
		}
	}
}

func TestResolveChromeBinaryRejectsMissingExplicitCore(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Browser.Cores = []config.BrowserCore{{
		CoreId: "default-core", CoreName: "Default", CorePath: "missing", IsDefault: true,
	}}
	manager := NewManager(cfg, t.TempDir())

	_, err := manager.ResolveChromeBinary(&Profile{ProfileId: "profile-1", CoreId: "deleted-core"})
	if err == nil || !strings.Contains(err.Error(), "实例绑定的浏览器内核不存在") {
		t.Fatalf("ResolveChromeBinary error = %v, want missing bound core rejection", err)
	}
}
