package identity

import (
	"path/filepath"
	"testing"

	"ant-chrome/backend/internal/database"
)

func newTestStore(t *testing.T) *SQLiteStore {
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
	return NewSQLiteStore(db.GetConn())
}

// Seen 应能同时按 fingerprint hash 与 seed 命中已登记的身份。
func TestSQLiteStoreSeenDetectsHashAndSeed(t *testing.T) {
	store := newTestStore(t)
	id := sampleIdentity()

	if seen, err := store.Seen(id.FingerprintHash(), id.Seed); err != nil {
		t.Fatalf("Seen: %v", err)
	} else if seen {
		t.Fatal("expected not seen before save")
	}

	if err := store.Save("p1", id); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if seen, _ := store.Seen(id.FingerprintHash(), id.Seed+999); !seen {
		t.Fatal("expected seen by matching fingerprint hash")
	}
	if seen, _ := store.Seen("different-hash", id.Seed); !seen {
		t.Fatal("expected seen by matching seed")
	}
	if seen, _ := store.Seen("brand-new-hash", id.Seed+12345); seen {
		t.Fatal("expected not seen for a fresh hash+seed")
	}
}

// Save 后 Load 应还原出等价身份(以指纹 hash 判等)。
func TestSQLiteStoreSaveLoadRoundTrip(t *testing.T) {
	store := newTestStore(t)
	id := sampleIdentity()
	if err := store.Save("p1", id); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, ok, err := store.Load("p1")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !ok {
		t.Fatal("expected identity to exist")
	}
	if got.FingerprintHash() != id.FingerprintHash() {
		t.Fatal("loaded identity differs from saved")
	}
}

// 同一 profile 再次 Save 应为覆盖(upsert),不因唯一约束报错。
func TestSQLiteStoreSaveIsUpsertByProfile(t *testing.T) {
	store := newTestStore(t)
	a := sampleIdentity()
	if err := store.Save("p1", a); err != nil {
		t.Fatalf("Save a: %v", err)
	}
	b := sampleIdentity()
	b.Seed = a.Seed + 1
	if err := store.Save("p1", b); err != nil {
		t.Fatalf("Save b (upsert): %v", err)
	}
	got, _, _ := store.Load("p1")
	if got.Seed != b.Seed {
		t.Fatalf("expected upsert to latest identity, got seed %d", got.Seed)
	}
}
