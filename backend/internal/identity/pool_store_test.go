package identity

import (
	"os"
	"path/filepath"
	"testing"
)

func validPoolRecord() PoolRecord {
	return PoolRecord{
		Platform:            "macos",
		BrandVersion:        "146.0.0.0",
		UAFull:              "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36",
		HardwareConcurrency: 10,
		DeviceMemory:        16,
		Screen:              Screen{Width: 1512, Height: 982, DevicePixelRatio: 2, ColorDepth: 30},
		WindowSize:          "1512,900",
		Languages:           []string{"en-US", "en"},
		Locale:              "en-US",
		Timezone:            "America/New_York",
		Weight:              1,
	}
}

// 覆盖文件不存在时:用内嵌默认物化并落盘,记录都赋了 ID。
func TestPoolStoreMaterializesFromEmbedded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity_pool.json")
	s, err := NewPoolStore(path)
	if err != nil {
		t.Fatalf("NewPoolStore: %v", err)
	}
	if s.Count() < 100 {
		t.Fatalf("物化后记录数过少: %d", s.Count())
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("overlay 文件应已落盘: %v", err)
	}
	for _, r := range s.Records()[:5] {
		if r.ID == "" {
			t.Error("物化后每条应有 ID")
		}
	}
}

// 增删改后重新打开,数据应持久;Pool() 随之重建。
func TestPoolStoreCRUDRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity_pool.json")
	s, err := NewPoolStore(path)
	if err != nil {
		t.Fatal(err)
	}
	base := s.Count()

	added, err := s.Add(validPoolRecord())
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if added.ID == "" {
		t.Fatal("Add 应赋 ID")
	}
	if s.Count() != base+1 {
		t.Fatalf("Add 后应 %d,实为 %d", base+1, s.Count())
	}

	upd := validPoolRecord()
	upd.HardwareConcurrency = 24
	if _, err := s.Update(added.ID, upd); err != nil {
		t.Fatalf("Update: %v", err)
	}

	// 重新打开,验证持久化 + Update 生效。
	s2, err := NewPoolStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if s2.Count() != base+1 {
		t.Fatalf("重开后应 %d,实为 %d", base+1, s2.Count())
	}
	var found *PoolRecord
	for _, r := range s2.Records() {
		if r.ID == added.ID {
			rr := r
			found = &rr
		}
	}
	if found == nil || found.HardwareConcurrency != 24 {
		t.Fatalf("Update 未持久化: %+v", found)
	}

	if err := s2.Delete(added.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if s2.Count() != base {
		t.Fatalf("Delete 后应回到 %d,实为 %d", base, s2.Count())
	}
	if s2.Pool().Len() != base {
		t.Fatalf("Pool 应随删除重建为 %d,实为 %d", base, s2.Pool().Len())
	}
}

// RestoreDefaults 恢复到内嵌默认数量。
func TestPoolStoreRestoreDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity_pool.json")
	s, err := NewPoolStore(path)
	if err != nil {
		t.Fatal(err)
	}
	base := s.Count()
	_, _ = s.Add(validPoolRecord())
	_, _ = s.Add(validPoolRecord())
	if s.Count() != base+2 {
		t.Fatalf("预期 %d", base+2)
	}
	if err := s.RestoreDefaults(); err != nil {
		t.Fatalf("RestoreDefaults: %v", err)
	}
	if s.Count() != base {
		t.Fatalf("恢复默认后应回到 %d,实为 %d", base, s.Count())
	}
}

// 坏的 overlay 文件:备份为 .bad 后回退内嵌默认(始终可用)。
func TestPoolStoreRecoversFromCorruptOverlay(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity_pool.json")
	if err := os.WriteFile(path, []byte("{ not valid json"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := NewPoolStore(path)
	if err != nil {
		t.Fatalf("坏文件应恢复而非报错: %v", err)
	}
	if s.Count() < 100 {
		t.Fatalf("恢复后记录数过少: %d", s.Count())
	}
	if _, err := os.Stat(path + ".bad"); err != nil {
		t.Errorf("坏文件应被备份为 .bad: %v", err)
	}
}

func TestValidatePoolRecord(t *testing.T) {
	if r := ValidatePoolRecord(validPoolRecord()); !r.OK {
		t.Errorf("合法记录应通过,却有问题: %+v", r.Issues)
	}
	// UA 版本与 brandVersion 不一致 → error。
	bad := validPoolRecord()
	bad.BrandVersion = "145.0.0.0" // UA 是 146
	r := ValidatePoolRecord(bad)
	if r.OK {
		t.Error("UA↔brandVersion 不一致应判 error")
	}
	// UA 与平台不一致 → error。
	bad2 := validPoolRecord()
	bad2.Platform = "windows" // UA 是 Mac
	if ValidatePoolRecord(bad2).OK {
		t.Error("UA↔平台不一致应判 error")
	}
}
