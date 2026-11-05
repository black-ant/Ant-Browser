package browser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ant-chrome/backend/internal/config"
)

func TestPermanentlyDeleteRemovesUserDataDirAndWritesAudit(t *testing.T) {
	appRoot := t.TempDir()
	manager := NewManager(&config.Config{}, appRoot)
	manager.Config.Browser.UserDataRoot = "data"

	profile := &Profile{
		ProfileId:   "profile-delete-audit",
		ProfileName: "待彻底删除实例",
		UserDataDir: "profile-delete-audit",
		DeletedAt:   time.Now().Add(-4 * 24 * time.Hour).Format(time.RFC3339),
	}
	dao := &profileDeleteAuditTestDAO{profiles: map[string]*Profile{profile.ProfileId: profile}}
	manager.ProfileDAO = dao

	userDataDir := filepath.Join(appRoot, "data", profile.UserDataDir)
	if err := os.MkdirAll(filepath.Join(userDataDir, "Default"), 0755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(userDataDir, "Default", "Preferences"), []byte("{}"), 0644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	fingerprintCheckDir := filepath.Join(appRoot, "data", "fingerprint-check", profile.ProfileId)
	if err := os.MkdirAll(fingerprintCheckDir, 0755); err != nil {
		t.Fatalf("MkdirAll fingerprint check dir returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(fingerprintCheckDir, "index.html"), []byte("<html></html>"), 0644); err != nil {
		t.Fatalf("WriteFile fingerprint check page returned error: %v", err)
	}

	if err := manager.PermanentlyDelete(profile.ProfileId); err != nil {
		t.Fatalf("PermanentlyDelete returned error: %v", err)
	}
	if _, err := os.Stat(userDataDir); !os.IsNotExist(err) {
		t.Fatalf("user data dir still exists after permanent delete: %v", err)
	}
	if _, exists := dao.profiles[profile.ProfileId]; exists {
		t.Fatalf("profile record still exists after permanent delete")
	}
	if _, err := os.Stat(fingerprintCheckDir); !os.IsNotExist(err) {
		t.Fatalf("fingerprint check dir still exists after permanent delete: %v", err)
	}

	auditPath := filepath.Join(appRoot, "data", "logs", "profile-delete-audit.jsonl")
	auditBytes, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("ReadFile audit returned error: %v", err)
	}
	audit := string(auditBytes)
	for _, expected := range []string{
		`"action":"permanent_delete"`,
		`"profileId":"profile-delete-audit"`,
		`"dataDirExistedBefore":true`,
		`"dataDirExistsAfter":false`,
		`"success":true`,
	} {
		if !strings.Contains(audit, expected) {
			t.Fatalf("audit log missing %s: %s", expected, audit)
		}
	}
}

type profileDeleteAuditTestDAO struct {
	profiles map[string]*Profile
}

func (d *profileDeleteAuditTestDAO) List() ([]*Profile, error) { return nil, nil }

func (d *profileDeleteAuditTestDAO) ListDeleted() ([]*Profile, error) { return nil, nil }

func (d *profileDeleteAuditTestDAO) GetById(profileId string) (*Profile, error) {
	profile, ok := d.profiles[profileId]
	if !ok {
		return nil, errProfileDeleteAuditTestNotFound(profileId)
	}
	return profile, nil
}

func (d *profileDeleteAuditTestDAO) Upsert(profile *Profile) error {
	d.profiles[profile.ProfileId] = profile
	return nil
}

func (d *profileDeleteAuditTestDAO) Delete(profileId string) error {
	delete(d.profiles, profileId)
	return nil
}

func (d *profileDeleteAuditTestDAO) SoftDelete(profileId string, deletedAt string) error {
	profile, ok := d.profiles[profileId]
	if !ok {
		return errProfileDeleteAuditTestNotFound(profileId)
	}
	profile.DeletedAt = deletedAt
	return nil
}

func (d *profileDeleteAuditTestDAO) Restore(profileId string) error {
	profile, ok := d.profiles[profileId]
	if !ok {
		return errProfileDeleteAuditTestNotFound(profileId)
	}
	profile.DeletedAt = ""
	return nil
}

func (d *profileDeleteAuditTestDAO) ListExpiredDeleted(expiredBefore string) ([]*Profile, error) {
	return nil, nil
}

type errProfileDeleteAuditTestNotFound string

func (e errProfileDeleteAuditTestNotFound) Error() string { return "profile not found: " + string(e) }
