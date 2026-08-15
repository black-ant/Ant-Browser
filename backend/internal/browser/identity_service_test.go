package browser

import (
	"path/filepath"
	"strings"
	"testing"

	"ant-chrome/backend/internal/database"
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
	svc, err := NewIdentityService(db.GetConn())
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
