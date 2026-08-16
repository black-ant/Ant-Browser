package browser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"ant-chrome/backend/internal/database"
	"ant-chrome/backend/internal/identity"
)

func newTestIdentityServiceWithOverlay(t *testing.T, overlay string) *IdentityService {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := database.NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	svc, err := NewIdentityService(db.GetConn(), overlay)
	if err != nil {
		t.Fatalf("NewIdentityService: %v", err)
	}
	return svc
}

// 端到端:overlay 身份池被 IdentityService 采样;删除某平台后,新建环境立刻不再出现该平台。
func TestPoolEditAffectsSampling(t *testing.T) {
	overlay := filepath.Join(t.TempDir(), "pool.json")
	recs := []identity.PoolRecord{
		{ID: "mac", Platform: "macos", BrandVersion: "147.0.0.0", UAFull: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36", HardwareConcurrency: 8, DeviceMemory: 16, Screen: identity.Screen{Width: 1512, Height: 982, DevicePixelRatio: 2, ColorDepth: 30}, WindowSize: "1512,900", Languages: []string{"en-US"}, Locale: "en-US", Timezone: "America/New_York", Weight: 1},
		{ID: "win", Platform: "windows", BrandVersion: "147.0.0.0", UAFull: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36", HardwareConcurrency: 12, DeviceMemory: 32, Screen: identity.Screen{Width: 1536, Height: 864, DevicePixelRatio: 1.25, ColorDepth: 32}, WindowSize: "1536,816", Languages: []string{"en-US"}, Locale: "en-US", Timezone: "America/New_York", Weight: 1},
	}
	data, _ := json.MarshalIndent(recs, "", "  ")
	if err := os.WriteFile(overlay, data, 0o644); err != nil {
		t.Fatal(err)
	}

	svc := newTestIdentityServiceWithOverlay(t, overlay)
	if svc.PoolCount() != 2 {
		t.Fatalf("应加载 overlay 的 2 条,实为 %d", svc.PoolCount())
	}

	// 删除 windows 记录。
	for _, r := range svc.PoolRecords() {
		if r.Platform == "windows" {
			if err := svc.DeletePoolRecord(r.ID); err != nil {
				t.Fatal(err)
			}
		}
	}
	// 之后生成的环境应全部是 macos(采样反映了删除)。
	for i := 0; i < 15; i++ {
		id, err := svc.GenerateUnique()
		if err != nil {
			t.Fatalf("GenerateUnique: %v", err)
		}
		if id.Platform != "macos" {
			t.Fatalf("删掉 windows 后仍采样到 %s", id.Platform)
		}
	}
}
