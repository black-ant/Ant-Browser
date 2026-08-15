package database

import (
	"path/filepath"
	"testing"
)

// 迁移后应存在 browser_identities 表及其关键列,且 schema 版本达到 15。
func TestMigrateCreatesBrowserIdentitiesTable(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	conn := db.GetConn()

	var v int
	if err := conn.QueryRow(`SELECT COALESCE(MAX(version),0) FROM schema_migrations`).Scan(&v); err != nil {
		t.Fatalf("query version: %v", err)
	}
	if v < 15 {
		t.Fatalf("expected schema version >= 15, got %d", v)
	}

	want := map[string]bool{
		"profile_id": false, "identity_json": false, "fingerprint_hash": false,
		"seed": false, "coherence_status": false, "proxy_geo_snapshot": false,
	}
	rows, err := conn.Query(`PRAGMA table_info(browser_identities)`)
	if err != nil {
		t.Fatalf("table_info: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notnull, pk int
		var name, ctype string
		var dflt any
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan col: %v", err)
		}
		if _, ok := want[name]; ok {
			want[name] = true
		}
	}
	for col, seen := range want {
		if !seen {
			t.Errorf("missing column %q in browser_identities", col)
		}
	}
}

// fingerprint_hash 必须在数据库层唯一(唯一性登记的硬保障)。
func TestMigrateEnforcesFingerprintHashUniqueness(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	conn := db.GetConn()

	if _, err := conn.Exec(`INSERT INTO browser_identities (profile_id, identity_json, fingerprint_hash, seed) VALUES ('p1','{}','abc',1)`); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	if _, err := conn.Exec(`INSERT INTO browser_identities (profile_id, identity_json, fingerprint_hash, seed) VALUES ('p2','{}','abc',2)`); err == nil {
		t.Fatal("expected UNIQUE constraint violation on duplicate fingerprint_hash")
	}
}
