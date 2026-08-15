package identity

import (
	"database/sql"
	"encoding/json"
)

// SQLiteStore 基于 browser_identities 表实现身份持久化与唯一性登记。
// 它同时满足 UniquenessStore 接口(Seen)。
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore 用已打开的数据库连接创建 store。
func NewSQLiteStore(db *sql.DB) *SQLiteStore {
	return &SQLiteStore{db: db}
}

// Seen 报告是否已存在 fingerprint hash 或 seed 相同的身份(空 hash / 零 seed 不算命中)。
func (s *SQLiteStore) Seen(fingerprintHash string, seed int64) (bool, error) {
	var exists int
	err := s.db.QueryRow(
		`SELECT EXISTS(
			SELECT 1 FROM browser_identities
			WHERE (fingerprint_hash = ? AND fingerprint_hash != '')
			   OR (seed = ? AND seed != 0)
		)`,
		fingerprintHash, seed,
	).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists == 1, nil
}

// Save 以 profile_id 为主键 upsert 一套身份。
func (s *SQLiteStore) Save(profileID string, id Identity) error {
	data, err := json.Marshal(id)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(
		`INSERT INTO browser_identities
			(profile_id, identity_json, fingerprint_hash, seed, coherence_status, proxy_geo_snapshot, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(profile_id) DO UPDATE SET
			identity_json      = excluded.identity_json,
			fingerprint_hash   = excluded.fingerprint_hash,
			seed               = excluded.seed,
			coherence_status   = excluded.coherence_status,
			proxy_geo_snapshot = excluded.proxy_geo_snapshot,
			updated_at         = CURRENT_TIMESTAMP`,
		profileID, string(data), id.FingerprintHash(), id.Seed, id.CoherenceStatus, id.ProxyGeoSnapshot,
	)
	return err
}

// Load 读取某 profile 的身份;不存在时返回 (Identity{}, false, nil)。
func (s *SQLiteStore) Load(profileID string) (Identity, bool, error) {
	var data string
	err := s.db.QueryRow(
		`SELECT identity_json FROM browser_identities WHERE profile_id = ?`, profileID,
	).Scan(&data)
	if err == sql.ErrNoRows {
		return Identity{}, false, nil
	}
	if err != nil {
		return Identity{}, false, err
	}
	var id Identity
	if err := json.Unmarshal([]byte(data), &id); err != nil {
		return Identity{}, false, err
	}
	return id, true, nil
}
