package browser

import (
	"fmt"
	"testing"

	"ant-chrome/backend/internal/config"
)

type countingProfileDAO struct {
	upsertCalls int
	batchCalls  int
	batchCount  int
	failUpsert  bool
}

func (d *countingProfileDAO) List() ([]*Profile, error)                     { return nil, nil }
func (d *countingProfileDAO) ListDeleted() ([]*Profile, error)              { return nil, nil }
func (d *countingProfileDAO) GetById(string) (*Profile, error)              { return nil, fmt.Errorf("not found") }
func (d *countingProfileDAO) Delete(string) error                           { return nil }
func (d *countingProfileDAO) SoftDelete(string, string) error               { return nil }
func (d *countingProfileDAO) Restore(string) error                          { return nil }
func (d *countingProfileDAO) ListExpiredDeleted(string) ([]*Profile, error) { return nil, nil }

func (d *countingProfileDAO) Upsert(*Profile) error {
	d.upsertCalls++
	if d.failUpsert {
		return fmt.Errorf("stub upsert failure")
	}
	return nil
}

func (d *countingProfileDAO) UpsertMany(profiles []*Profile) error {
	d.batchCalls++
	d.batchCount += len(profiles)
	if d.failUpsert {
		return fmt.Errorf("stub upsert failure")
	}
	return nil
}

func newProfileTestManager(t *testing.T, existing int) (*Manager, *countingProfileDAO) {
	t.Helper()
	manager := NewManager(&config.Config{}, t.TempDir())
	manager.Profiles = make(map[string]*Profile, existing)
	for index := 0; index < existing; index++ {
		id := fmt.Sprintf("existing-%04d", index)
		manager.Profiles[id] = &Profile{ProfileId: id, ProfileName: id}
	}
	dao := &countingProfileDAO{}
	manager.ProfileDAO = dao
	return manager, dao
}

func TestCreateOnlyUpsertsNewProfile(t *testing.T) {
	manager, dao := newProfileTestManager(t, 500)

	if _, err := manager.Create(ProfileInput{ProfileName: "new-profile"}); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if dao.upsertCalls != 1 {
		t.Fatalf("Upsert calls = %d, want 1 (must not rewrite existing profiles)", dao.upsertCalls)
	}
	if dao.batchCalls != 0 {
		t.Fatalf("UpsertMany calls = %d, want 0", dao.batchCalls)
	}
	if len(manager.Profiles) != 501 {
		t.Fatalf("profiles in memory = %d, want 501", len(manager.Profiles))
	}
}

func TestCreateKeepsMemoryCleanWhenPersistFails(t *testing.T) {
	manager, dao := newProfileTestManager(t, 3)
	dao.failUpsert = true

	if _, err := manager.Create(ProfileInput{ProfileName: "broken"}); err == nil {
		t.Fatal("Create returned nil error on failing DAO")
	}
	if len(manager.Profiles) != 3 {
		t.Fatalf("profiles in memory = %d, want 3 (new profile must be rolled back)", len(manager.Profiles))
	}
}

func TestSaveProfilesUsesSingleBatchCall(t *testing.T) {
	manager, dao := newProfileTestManager(t, 200)

	if err := manager.SaveProfiles(); err != nil {
		t.Fatalf("SaveProfiles returned error: %v", err)
	}
	if dao.upsertCalls != 0 {
		t.Fatalf("Upsert calls = %d, want 0 (batch path must be used)", dao.upsertCalls)
	}
	if dao.batchCalls != 1 || dao.batchCount != 200 {
		t.Fatalf("UpsertMany calls = %d, profiles = %d, want 1 call with 200 profiles", dao.batchCalls, dao.batchCount)
	}
}

func TestUpdateOnlyUpsertsTargetProfile(t *testing.T) {
	manager, dao := newProfileTestManager(t, 50)
	target := manager.Profiles["existing-0000"]

	if _, err := manager.Update(target.ProfileId, ProfileInput{
		ProfileName:    "renamed",
		ProxyMode:      ProxyModeDynamic,
		ProxyGroupName: "mobile",
	}); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if dao.upsertCalls != 1 {
		t.Fatalf("Upsert calls = %d, want 1", dao.upsertCalls)
	}
	if dao.batchCalls != 0 {
		t.Fatalf("UpsertMany calls = %d, want 0", dao.batchCalls)
	}
}
