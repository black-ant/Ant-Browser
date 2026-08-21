package browser

import (
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/database"
	"path/filepath"
	"testing"
)

func newMigratedCoreDAO(t *testing.T) *SQLiteCoreDAO {
	t.Helper()
	db, err := database.NewDB(filepath.Join(t.TempDir(), "cores.db"))
	if err != nil {
		t.Fatalf("NewDB returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}
	return NewSQLiteCoreDAO(db.GetConn())
}

func TestSQLiteCoreDAOPersistsBackendAndEnv(t *testing.T) {
	dao := newMigratedCoreDAO(t)

	core := Core{
		CoreId:      "cloak-core",
		CoreName:    "Cloak 148",
		CorePath:    "chrome/cloak",
		IsDefault:   true,
		CoreBackend: config.CoreBackendCloak,
		CoreEnv:     []string{"CLOAKBROWSER_LICENSE_KEY=abc123", "CLOAKBROWSER_CACHE_DIR=D:/cloak-cache"},
	}
	if err := dao.Upsert(core); err != nil {
		t.Fatalf("Upsert returned error: %v", err)
	}

	listed, err := dao.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("List length = %d, want 1", len(listed))
	}
	stored := listed[0]
	if stored.CoreBackend != config.CoreBackendCloak {
		t.Fatalf("CoreBackend = %q, want %q", stored.CoreBackend, config.CoreBackendCloak)
	}
	if len(stored.CoreEnv) != 2 {
		t.Fatalf("CoreEnv = %#v, want 2 entries", stored.CoreEnv)
	}
	assertStringSliceContains(t, stored.CoreEnv, "CLOAKBROWSER_LICENSE_KEY=abc123")
	assertStringSliceContains(t, stored.CoreEnv, "CLOAKBROWSER_CACHE_DIR=D:/cloak-cache")
}

func TestSQLiteCoreDAODefaultsLegacyRowsToFingerprintChromium(t *testing.T) {
	dao := newMigratedCoreDAO(t)

	// 模拟迁移前写入的历史行：core_backend 为空串
	if err := dao.Upsert(Core{CoreId: "legacy", CoreName: "Legacy", CorePath: "chrome"}); err != nil {
		t.Fatalf("Upsert returned error: %v", err)
	}

	listed, err := dao.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("List length = %d, want 1", len(listed))
	}
	if got, want := listed[0].CoreBackend, config.CoreBackendFingerprintChromium; got != want {
		t.Fatalf("legacy CoreBackend = %q, want %q", got, want)
	}
	if listed[0].CoreEnv != nil {
		t.Fatalf("legacy CoreEnv = %#v, want nil", listed[0].CoreEnv)
	}
}

func TestSQLiteCoreDAOUpsertOverwritesBackendAndEnv(t *testing.T) {
	dao := newMigratedCoreDAO(t)

	if err := dao.Upsert(Core{
		CoreId:      "core-1",
		CoreName:    "Cloak",
		CorePath:    "chrome/cloak",
		CoreBackend: config.CoreBackendCloak,
		CoreEnv:     []string{"CLOAKBROWSER_LICENSE_KEY=old"},
	}); err != nil {
		t.Fatalf("first Upsert returned error: %v", err)
	}
	if err := dao.Upsert(Core{
		CoreId:      "core-1",
		CoreName:    "Cloak",
		CorePath:    "chrome/cloak",
		CoreBackend: config.CoreBackendCloak,
		CoreEnv:     []string{"CLOAKBROWSER_LICENSE_KEY=new"},
	}); err != nil {
		t.Fatalf("second Upsert returned error: %v", err)
	}

	listed, err := dao.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("List length = %d, want 1", len(listed))
	}
	assertStringSliceContains(t, listed[0].CoreEnv, "CLOAKBROWSER_LICENSE_KEY=new")
	for _, arg := range listed[0].CoreEnv {
		if arg == "CLOAKBROWSER_LICENSE_KEY=old" {
			t.Fatalf("stale env entry survived upsert: %#v", listed[0].CoreEnv)
		}
	}
}

func TestUnmarshalCoreEnvHandlesBadData(t *testing.T) {
	if got := unmarshalCoreEnv("not json"); got != nil {
		t.Fatalf("unmarshalCoreEnv(bad json) = %#v, want nil", got)
	}
	if got := unmarshalCoreEnv(""); got != nil {
		t.Fatalf("unmarshalCoreEnv(empty) = %#v, want nil", got)
	}
	if got := unmarshalCoreEnv(`["", "  ", "A=1"]`); len(got) != 1 || got[0] != "A=1" {
		t.Fatalf("unmarshalCoreEnv(blank entries) = %#v, want [A=1]", got)
	}
}

func TestMarshalCoreEnvNormalizesEmpty(t *testing.T) {
	if got, want := marshalCoreEnv(nil), "[]"; got != want {
		t.Fatalf("marshalCoreEnv(nil) = %q, want %q", got, want)
	}
	if got, want := marshalCoreEnv([]string{"", "   "}), "[]"; got != want {
		t.Fatalf("marshalCoreEnv(blank) = %q, want %q", got, want)
	}
}

func TestSaveCoreNormalizesBackendInConfigFallback(t *testing.T) {
	manager := NewManager(&config.Config{}, t.TempDir())

	// CoreDAO 未注入时走 config.yaml 降级路径
	if err := manager.SaveCore(CoreInput{
		CoreName:    "Cloak",
		CorePath:    "chrome/cloak",
		CoreBackend: "CloakBrowser", // 非规范写法，应被归一化
		CoreEnv:     []string{" CLOAKBROWSER_LICENSE_KEY=k ", ""},
	}); err != nil {
		t.Fatalf("SaveCore returned error: %v", err)
	}

	cores := manager.Config.Browser.Cores
	if len(cores) != 1 {
		t.Fatalf("cores length = %d, want 1", len(cores))
	}
	if got, want := cores[0].CoreBackend, config.CoreBackendCloak; got != want {
		t.Fatalf("CoreBackend = %q, want %q", got, want)
	}
	if len(cores[0].CoreEnv) != 1 || cores[0].CoreEnv[0] != "CLOAKBROWSER_LICENSE_KEY=k" {
		t.Fatalf("CoreEnv = %#v, want trimmed single entry", cores[0].CoreEnv)
	}
}
