package launchcode

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// LaunchCodeDAO Launch Code 持久化接口
type LaunchCodeDAO interface {
	// FindProfileId 根据 code 查询 profileId
	FindProfileId(code string) (string, error)
	// FindCode 根据 profileId 查询 code
	FindCode(profileId string) (string, error)
	// Upsert 保存或更新映射
	Upsert(profileId, code string) error
	// Delete 删除映射（窗口删除时调用）
	Delete(profileId string) error
	// LoadAll 加载所有映射（启动时用），返回 profileId -> code 的 map
	LoadAll() (map[string]string, error)
}

// SQLiteLaunchCodeDAO 基于 SQLite 的 LaunchCodeDAO 实现
type SQLiteLaunchCodeDAO struct {
	db *sql.DB
}

// NewSQLiteLaunchCodeDAO 创建 SQLiteLaunchCodeDAO
func NewSQLiteLaunchCodeDAO(db *sql.DB) *SQLiteLaunchCodeDAO {
	return &SQLiteLaunchCodeDAO{db: db}
}

// FindProfileId 根据 code 查询 profileId
func (d *SQLiteLaunchCodeDAO) FindProfileId(code string) (string, error) {
	var profileId string
	err := d.db.QueryRow(
		`SELECT profile_id FROM launch_codes WHERE code = ?`, code,
	).Scan(&profileId)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("launch code not found: %s", code)
	}
	if err != nil {
		return "", fmt.Errorf("查询 launch code 失败: %w", err)
	}
	return profileId, nil
}

// FindCode 根据 profileId 查询 code
func (d *SQLiteLaunchCodeDAO) FindCode(profileId string) (string, error) {
	var code string
	err := d.db.QueryRow(
		`SELECT code FROM launch_codes WHERE profile_id = ?`, profileId,
	).Scan(&code)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("profile not found: %s", profileId)
	}
	if err != nil {
		return "", fmt.Errorf("查询 profile code 失败: %w", err)
	}
	return code, nil
}

// Upsert 保存或更新 profileId <-> code 映射
func (d *SQLiteLaunchCodeDAO) Upsert(profileId, code string) error {
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	_, err := d.db.Exec(
		`INSERT INTO launch_codes (profile_id, code, created_at, updated_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(profile_id) DO UPDATE SET
		   code       = excluded.code,
		   updated_at = excluded.updated_at`,
		profileId, code, now, now,
	)
	if err != nil {
		return fmt.Errorf("保存 launch code 失败: %w", err)
	}
	return nil
}

// Delete 删除 profileId 对应的映射
func (d *SQLiteLaunchCodeDAO) Delete(profileId string) error {
	_, err := d.db.Exec(
		`DELETE FROM launch_codes WHERE profile_id = ?`, profileId,
	)
	if err != nil {
		return fmt.Errorf("删除 launch code 失败: %w", err)
	}
	return nil
}

// LoadAll 加载所有映射，返回 profileId -> code 的 map
func (d *SQLiteLaunchCodeDAO) LoadAll() (map[string]string, error) {
	rows, err := d.db.Query(`SELECT profile_id, code FROM launch_codes`)
	if err != nil {
		return nil, fmt.Errorf("加载 launch codes 失败: %w", err)
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var profileId, code string
		if err := rows.Scan(&profileId, &code); err != nil {
			return nil, fmt.Errorf("读取 launch code 行失败: %w", err)
		}
		result[profileId] = code
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历 launch codes 失败: %w", err)
	}
	return result, nil
}
