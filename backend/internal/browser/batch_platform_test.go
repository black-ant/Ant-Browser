package browser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/identity"
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

// 不支持的平台参数(如 "linux")必须在建任何环境之前就被拒绝——不允许静默用回退身份"成功"。
func TestCreateBatchRejectsInvalidPlatform(t *testing.T) {
	manager := NewManager(&config.Config{}, t.TempDir())
	manager.IdentityService = newTestIdentityService(t)

	created, err := manager.CreateBatch("bad", 3, 1, "linux", ProfileInput{})
	if err == nil {
		t.Fatal("expected error for unsupported platform \"linux\", got nil")
	}
	if len(created) != 0 {
		t.Fatalf("expected no profiles created on validation failure, got %d", len(created))
	}
	if len(manager.Profiles) != 0 {
		t.Fatalf("expected no profiles registered in manager, got %d", len(manager.Profiles))
	}
}

// 平台参数大小写应被归一化(如 "MacOS" -> "macos")后再校验/采样,而不是被当作非法值拒绝
// 或原样透传导致池过滤匹配不到任何记录。
func TestCreateBatchNormalizesPlatformCase(t *testing.T) {
	manager := NewManager(&config.Config{}, t.TempDir())
	manager.IdentityService = newTestIdentityService(t)

	created, err := manager.CreateBatch("mixedcase", 1, 1, "MacOS", ProfileInput{})
	if err != nil {
		t.Fatalf("expected \"MacOS\" to normalize to \"macos\" and succeed, got error: %v", err)
	}
	if len(created) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(created))
	}
	if val, ok := argValue(created[0].FingerprintArgs, "--fingerprint-platform="); !ok || val != "macos" {
		t.Fatalf("expected --fingerprint-platform=macos, got %v", created[0].FingerprintArgs)
	}
}

// 指定平台的池为空时(如身份池 overlay 只含 windows,却请求 macos 批量创建),必须整批硬失败,
// 不能静默回退成非唯一(未入 browser_identities 去重库)且平台不符(宿主 windows)的默认指纹。
// 这正是 review 发现的 bug:createProfileLocked 原来只 log.Warn 并 return (profile, nil)。
func TestCreateBatchHardFailsWhenPlatformPoolEmpty(t *testing.T) {
	recs := []identity.PoolRecord{
		{
			ID: "win1", Platform: "windows", PlatformVersion: "10.0", BrandVersion: "147.0.0.0",
			UAFull:              "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36",
			HardwareConcurrency: 12, DeviceMemory: 32,
			Screen:     identity.Screen{Width: 1536, Height: 864, DevicePixelRatio: 1.25, ColorDepth: 32},
			WindowSize: "1536,816", Languages: []string{"en-US"}, Locale: "en-US", Timezone: "America/New_York", Weight: 1,
		},
		{
			ID: "win2", Platform: "windows", PlatformVersion: "10.0", BrandVersion: "147.0.0.0",
			UAFull:              "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36",
			HardwareConcurrency: 8, DeviceMemory: 16,
			Screen:     identity.Screen{Width: 1920, Height: 1080, DevicePixelRatio: 1, ColorDepth: 24},
			WindowSize: "1920,1040", Languages: []string{"en-US"}, Locale: "en-US", Timezone: "America/New_York", Weight: 1,
		},
	}
	data, err := json.MarshalIndent(recs, "", "  ")
	if err != nil {
		t.Fatalf("marshal pool fixture: %v", err)
	}
	overlay := filepath.Join(t.TempDir(), "pool.json")
	if err := os.WriteFile(overlay, data, 0o644); err != nil {
		t.Fatalf("write pool fixture: %v", err)
	}

	manager := NewManager(&config.Config{}, t.TempDir())
	manager.IdentityService = newTestIdentityServiceWithOverlay(t, overlay)

	created, err := manager.CreateBatch("empty", 2, 1, "macos", ProfileInput{})
	if err == nil {
		t.Fatalf("expected error when macos pool is empty, got silent success with %d profiles", len(created))
	}
	if len(created) != 0 {
		t.Fatalf("expected no profiles created on hard-fail, got %d", len(created))
	}
	if len(manager.Profiles) != 0 {
		t.Fatalf("expected no fallback profiles registered in manager, got %d", len(manager.Profiles))
	}
}
