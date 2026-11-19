package browser

import (
	"ant-chrome/backend/internal/database"
	"path/filepath"
	"testing"
)

func TestSQLiteProfileDAOPersistsMemoryLimitMB(t *testing.T) {
	db, err := database.NewDB(filepath.Join(t.TempDir(), "profiles.db"))
	if err != nil {
		t.Fatalf("NewDB returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}

	dao := NewSQLiteProfileDAO(db.GetConn())
	profile := &Profile{
		ProfileId:       "profile-memory-limit",
		ProfileName:     "memory limit",
		ProxyMode:       ProxyModeDynamic,
		ProxyGroupName:  " mobile ",
		MemoryLimitMB:   768,
		FingerprintArgs: []string{},
		LaunchArgs:      []string{},
		Tags:            []string{},
		Keywords:        []string{},
		CreatedAt:       "2026-07-26T00:00:00Z",
		UpdatedAt:       "2026-07-26T00:00:00Z",
	}

	if err := dao.Upsert(profile); err != nil {
		t.Fatalf("Upsert returned error: %v", err)
	}

	stored, err := dao.GetById(profile.ProfileId)
	if err != nil {
		t.Fatalf("GetById returned error: %v", err)
	}
	if stored.MemoryLimitMB != profile.MemoryLimitMB {
		t.Fatalf("MemoryLimitMB = %d, want %d", stored.MemoryLimitMB, profile.MemoryLimitMB)
	}
	if stored.ProxyMode != ProxyModeDynamic || stored.ProxyGroupName != "mobile" {
		t.Fatalf("proxy selection = (%q, %q), want (%q, %q)", stored.ProxyMode, stored.ProxyGroupName, ProxyModeDynamic, "mobile")
	}

	listed, err := dao.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("List length = %d, want 1", len(listed))
	}
	if listed[0].MemoryLimitMB != profile.MemoryLimitMB {
		t.Fatalf("listed MemoryLimitMB = %d, want %d", listed[0].MemoryLimitMB, profile.MemoryLimitMB)
	}
}

func TestSQLiteProfileDAOUpsertManyPersistsAllRecords(t *testing.T) {
	db, err := database.NewDB(filepath.Join(t.TempDir(), "profiles-batch.db"))
	if err != nil {
		t.Fatalf("NewDB returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}

	dao := NewSQLiteProfileDAO(db.GetConn())
	batch := []*Profile{
		{ProfileId: "batch-1", ProfileName: "one", FingerprintArgs: []string{}, LaunchArgs: []string{}, Tags: []string{}, Keywords: []string{}},
		{ProfileId: "batch-2", ProfileName: "two", FingerprintArgs: []string{}, LaunchArgs: []string{}, Tags: []string{}, Keywords: []string{}},
		{ProfileId: "batch-3", ProfileName: "three", FingerprintArgs: []string{}, LaunchArgs: []string{}, Tags: []string{}, Keywords: []string{}},
	}
	if err := dao.UpsertMany(batch); err != nil {
		t.Fatalf("UpsertMany returned error: %v", err)
	}

	listed, err := dao.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(listed) != len(batch) {
		t.Fatalf("List length = %d, want %d", len(listed), len(batch))
	}

	batch[0].ProfileName = "one-updated"
	if err := dao.UpsertMany(batch[:1]); err != nil {
		t.Fatalf("UpsertMany (update) returned error: %v", err)
	}
	stored, err := dao.GetById("batch-1")
	if err != nil {
		t.Fatalf("GetById returned error: %v", err)
	}
	if stored.ProfileName != "one-updated" {
		t.Fatalf("ProfileName = %q, want %q", stored.ProfileName, "one-updated")
	}
}
