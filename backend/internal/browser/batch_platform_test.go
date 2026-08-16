package browser

import (
	"testing"

	"ant-chrome/backend/internal/config"
)

// 组一个仅含 IdentityService 的最小场景,批量生成后校验平台一致且不重复。
func TestCreateBatchPlatformFilterUniqueAndConsistent(t *testing.T) {
	idSvc := newTestIdentityService(t)

	// 直接驱动 IdentityService:对每个 profile RegenerateForPlatform("macos")。
	seen := map[string]bool{}
	for i := 0; i < 20; i++ {
		p := &Profile{ProfileId: "b" + string(rune('a'+i))}
		if err := idSvc.RegenerateForPlatform(p, "macos"); err != nil {
			t.Fatalf("RegenerateForPlatform: %v", err)
		}
		var hasMac bool
		for _, a := range p.FingerprintArgs {
			if a == "--fingerprint-platform=macos" {
				hasMac = true
			}
			if a == "--fingerprint-platform=windows" {
				t.Fatalf("macOS 批不应出现 windows 平台参数: %v", p.FingerprintArgs)
			}
		}
		if !hasMac {
			t.Fatalf("应含 --fingerprint-platform=macos: %v", p.FingerprintArgs)
		}
		key := p.FingerprintArgs[0] // --fingerprint=<seed> 在首位,seed 全局唯一
		if seen[key] {
			t.Fatalf("身份重复: %s", key)
		}
		seen[key] = true
	}
}

// 端到端校验 Manager.CreateBatch 把 platform 形参正确透传进每个 profile 的身份生成
// (createProfileLocked -> RegenerateForPlatform(profile, input.IdentityPlatform))。
func TestCreateBatchThreadsPlatformIntoProfiles(t *testing.T) {
	manager := NewManager(&config.Config{}, t.TempDir())
	manager.IdentityService = newTestIdentityService(t)

	created, err := manager.CreateBatch("mac", 5, 1, "macos", ProfileInput{})
	if err != nil {
		t.Fatalf("CreateBatch: %v", err)
	}
	if len(created) != 5 {
		t.Fatalf("expected 5 profiles, got %d", len(created))
	}
	seeds := map[string]bool{}
	for _, p := range created {
		var hasMac bool
		for _, a := range p.FingerprintArgs {
			if a == "--fingerprint-platform=macos" {
				hasMac = true
			}
			if a == "--fingerprint-platform=windows" {
				t.Fatalf("macOS 批不应出现 windows 平台参数: %v", p.FingerprintArgs)
			}
		}
		if !hasMac {
			t.Fatalf("profile %q 应含 --fingerprint-platform=macos: %v", p.ProfileName, p.FingerprintArgs)
		}
		seed := p.FingerprintArgs[0]
		if seeds[seed] {
			t.Fatalf("身份重复: %s", seed)
		}
		seeds[seed] = true
	}

	// platform="" 时应等价全平台(不强制 macos),回归验证不影响既有无平台批量创建路径。
	createdAny, err := manager.CreateBatch("any", 3, 1, "", ProfileInput{})
	if err != nil {
		t.Fatalf("CreateBatch(platform=\"\"): %v", err)
	}
	if len(createdAny) != 3 {
		t.Fatalf("expected 3 profiles, got %d", len(createdAny))
	}
}
