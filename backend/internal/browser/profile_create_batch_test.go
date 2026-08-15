package browser

import (
	"testing"

	"ant-chrome/backend/internal/config"
)

// 批量创建应按 prefix-编号(3位)递增命名,且每个环境获得独立、非零、互不相同的指纹 seed。
func TestCreateBatchGeneratesSequentialUniqueEnvironments(t *testing.T) {
	manager := NewManager(&config.Config{}, t.TempDir())
	manager.IdentityService = newTestIdentityService(t)

	created, err := manager.CreateBatch("test", 3, 1, ProfileInput{})
	if err != nil {
		t.Fatalf("CreateBatch: %v", err)
	}
	if len(created) != 3 {
		t.Fatalf("expected 3 profiles, got %d", len(created))
	}

	wantNames := []string{"test-001", "test-002", "test-003"}
	seeds := map[string]bool{}
	for i, p := range created {
		if p.ProfileName != wantNames[i] {
			t.Fatalf("profile[%d] name = %q, want %q", i, p.ProfileName, wantNames[i])
		}
		seed, ok := argValue(p.FingerprintArgs, "--fingerprint=")
		if !ok || seed == "" || seed == "0" {
			t.Fatalf("profile %q missing fresh non-zero seed: %v", p.ProfileName, p.FingerprintArgs)
		}
		if seeds[seed] {
			t.Fatalf("duplicate seed %q across batch — not unique", seed)
		}
		seeds[seed] = true
	}
}

// startIndex 生效且保持 3 位零填充(9 → 009,跨十位 10 → 010)。
func TestCreateBatchStartIndexAndPadding(t *testing.T) {
	manager := NewManager(&config.Config{}, t.TempDir())
	manager.IdentityService = newTestIdentityService(t)

	created, err := manager.CreateBatch("env", 2, 9, ProfileInput{})
	if err != nil {
		t.Fatalf("CreateBatch: %v", err)
	}
	want := []string{"env-009", "env-010"}
	for i := range want {
		if created[i].ProfileName != want[i] {
			t.Fatalf("name[%d] = %q, want %q", i, created[i].ProfileName, want[i])
		}
	}
}

func TestCreateBatchValidation(t *testing.T) {
	manager := NewManager(&config.Config{}, t.TempDir())
	manager.IdentityService = newTestIdentityService(t)

	if _, err := manager.CreateBatch("   ", 3, 1, ProfileInput{}); err == nil {
		t.Fatal("expected error for empty prefix")
	}
	if _, err := manager.CreateBatch("x", 0, 1, ProfileInput{}); err == nil {
		t.Fatal("expected error for count 0")
	}
	if _, err := manager.CreateBatch("x", MaxBatchCreateCount+1, 1, ProfileInput{}); err == nil {
		t.Fatal("expected error for count over max")
	}
}
