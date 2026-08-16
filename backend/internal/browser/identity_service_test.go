package browser

import (
	"path/filepath"
	"strings"
	"testing"

	"ant-chrome/backend/internal/database"
	"ant-chrome/backend/internal/identity"
)

func newTestIdentityService(t *testing.T) *IdentityService {
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
	svc, err := NewIdentityService(db.GetConn(), "")
	if err != nil {
		t.Fatalf("NewIdentityService: %v", err)
	}
	return svc
}

func argValue(args []string, prefix string) (string, bool) {
	for _, a := range args {
		if strings.HasPrefix(a, prefix) {
			return strings.TrimPrefix(a, prefix), true
		}
	}
	return "", false
}

// 新建环境应自动获得唯一、自洽的指纹参数。
func TestIdentityServiceAssignsUniqueFingerprint(t *testing.T) {
	svc := newTestIdentityService(t)
	p1 := &Profile{ProfileId: "p1"}
	p2 := &Profile{ProfileId: "p2"}
	if err := svc.AssignToProfile(p1); err != nil {
		t.Fatalf("assign p1: %v", err)
	}
	if err := svc.AssignToProfile(p2); err != nil {
		t.Fatalf("assign p2: %v", err)
	}
	seed1, ok1 := argValue(p1.FingerprintArgs, "--fingerprint=")
	if !ok1 {
		t.Fatalf("expected --fingerprint on p1, got %v", p1.FingerprintArgs)
	}
	if _, ok := argValue(p1.FingerprintArgs, "--fingerprint-platform="); !ok {
		t.Fatalf("expected --fingerprint-platform on p1, got %v", p1.FingerprintArgs)
	}
	seed2, _ := argValue(p2.FingerprintArgs, "--fingerprint=")
	if seed1 == seed2 {
		t.Fatal("expected different seeds for different profiles")
	}
}

// 回归:新建环境即使已预填静态默认 flag,Regenerate 也必须产出全新池身份
// （带非零 seed 与完整维度),而不是把默认 flag 反解成 seed=0 的身份。
func TestRegenerateProducesFreshIdentityIgnoringExistingArgs(t *testing.T) {
	svc := newTestIdentityService(t)
	p := &Profile{ProfileId: "p1", FingerprintArgs: []string{"--fingerprint-platform=windows", "--fingerprint-brand=Chrome"}}
	if err := svc.Regenerate(p); err != nil {
		t.Fatalf("regenerate: %v", err)
	}
	seed, ok := argValue(p.FingerprintArgs, "--fingerprint=")
	if !ok || seed == "" || seed == "0" {
		t.Fatalf("expected a fresh non-zero seed, got %q in %v", seed, p.FingerprintArgs)
	}
	if _, ok := argValue(p.FingerprintArgs, "--window-size="); !ok {
		t.Fatalf("expected a full pool identity (window-size present), got %v", p.FingerprintArgs)
	}
}

// 按代理地理对齐后,时区/语言应更新为目标国家(自洽)。
func TestIdentityServiceAlignProfileToGeo(t *testing.T) {
	svc := newTestIdentityService(t)
	p := &Profile{ProfileId: "p1"}
	if err := svc.AssignToProfile(p); err != nil {
		t.Fatalf("assign: %v", err)
	}
	if err := svc.AlignProfileToGeo(p, identity.GeoInfo{CountryCode: "JP", Latitude: 35.68, Longitude: 139.69}); err != nil {
		t.Fatalf("align: %v", err)
	}
	if tz, _ := argValue(p.FingerprintArgs, "--timezone="); tz != "Asia/Tokyo" {
		t.Fatalf("expected Asia/Tokyo after align, got %q", tz)
	}
	if lang, _ := argValue(p.FingerprintArgs, "--accept-lang="); !strings.HasPrefix(lang, "ja-JP") {
		t.Fatalf("expected ja-JP accept-lang after align, got %q", lang)
	}
}

// 老环境(仅有 fingerprint_args、无结构化身份)应反解补齐,保留原 seed。
func TestIdentityServiceReverseDerivesLegacyArgs(t *testing.T) {
	svc := newTestIdentityService(t)
	legacy := &Profile{ProfileId: "old", FingerprintArgs: []string{
		"--fingerprint=555", "--fingerprint-platform=windows", "--fingerprint-brand=Chrome",
	}}
	if err := svc.AssignToProfile(legacy); err != nil {
		t.Fatalf("assign legacy: %v", err)
	}
	if seed, _ := argValue(legacy.FingerprintArgs, "--fingerprint="); seed != "555" {
		t.Fatalf("expected legacy seed 555 preserved, got %q in %v", seed, legacy.FingerprintArgs)
	}
}
