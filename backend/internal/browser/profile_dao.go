package browser

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ProfileDAO 窗口配置持久化接口
type ProfileDAO interface {
	List() ([]*Profile, error)
	GetById(profileId string) (*Profile, error)
	Upsert(profile *Profile) error
	Delete(profileId string) error
}

// SQLiteProfileDAO 基于 SQLite 的 ProfileDAO 实现
type SQLiteProfileDAO struct {
	db *sql.DB
}

// NewSQLiteProfileDAO 创建 SQLiteProfileDAO
func NewSQLiteProfileDAO(db *sql.DB) *SQLiteProfileDAO {
	return &SQLiteProfileDAO{db: db}
}

// List 查询所有窗口配置，按创建时间升序
func (d *SQLiteProfileDAO) List() ([]*Profile, error) {
	rows, err := d.db.Query(`
		SELECT profile_id, profile_name, user_data_dir, core_id,
		       fingerprint_args, proxy_id, proxy_config,
		       COALESCE(proxy_bind_source_id, ''), COALESCE(proxy_bind_source_url, ''),
		       COALESCE(proxy_bind_name, ''), COALESCE(proxy_bind_updated_at, ''),
		       launch_args,
		       COALESCE(profile_config, '{}'),
		       tags, keywords, group_id, created_at, updated_at
		FROM browser_profiles ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("查询窗口列表失败: %w", err)
	}
	defer rows.Close()

	var list []*Profile
	for rows.Next() {
		p, err := scanProfile(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

// GetById 根据 profileId 查询单个窗口
func (d *SQLiteProfileDAO) GetById(profileId string) (*Profile, error) {
	row := d.db.QueryRow(`
		SELECT profile_id, profile_name, user_data_dir, core_id,
		       fingerprint_args, proxy_id, proxy_config,
		       COALESCE(proxy_bind_source_id, ''), COALESCE(proxy_bind_source_url, ''),
		       COALESCE(proxy_bind_name, ''), COALESCE(proxy_bind_updated_at, ''),
		       launch_args,
		       COALESCE(profile_config, '{}'),
		       tags, keywords, group_id, created_at, updated_at
		FROM browser_profiles WHERE profile_id = ?`, profileId)
	p, err := scanProfile(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("窗口不存在: %s", profileId)
	}
	return p, err
}

// Upsert 新增或更新窗口配置
func (d *SQLiteProfileDAO) Upsert(profile *Profile) error {
	fingerprintArgs, _ := json.Marshal(profile.FingerprintArgs)
	launchArgs, _ := json.Marshal(profile.LaunchArgs)
	tags, _ := json.Marshal(profile.Tags)
	keywords, _ := json.Marshal(profile.Keywords)

	now := time.Now().Format(time.RFC3339)
	if profile.CreatedAt == "" {
		profile.CreatedAt = now
	}
	if profile.UpdatedAt == "" {
		profile.UpdatedAt = now
	}

	_, err := d.db.Exec(`
		INSERT INTO browser_profiles
		  (profile_id, profile_name, user_data_dir, core_id, fingerprint_args,
		   proxy_id, proxy_config, proxy_bind_source_id, proxy_bind_source_url, proxy_bind_name, proxy_bind_updated_at,
		   launch_args, profile_config, tags, keywords, group_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(profile_id) DO UPDATE SET
		  profile_name     = excluded.profile_name,
		  user_data_dir    = excluded.user_data_dir,
		  core_id          = excluded.core_id,
		  fingerprint_args = excluded.fingerprint_args,
		  proxy_id         = excluded.proxy_id,
		  proxy_config     = excluded.proxy_config,
		  proxy_bind_source_id = excluded.proxy_bind_source_id,
		  proxy_bind_source_url = excluded.proxy_bind_source_url,
		  proxy_bind_name = excluded.proxy_bind_name,
		  proxy_bind_updated_at = excluded.proxy_bind_updated_at,
		  launch_args      = excluded.launch_args,
		  profile_config   = excluded.profile_config,
		  tags             = excluded.tags,
		  keywords         = excluded.keywords,
		  group_id         = excluded.group_id,
		  updated_at       = excluded.updated_at`,
		profile.ProfileId, profile.ProfileName, profile.UserDataDir, profile.CoreId,
		string(fingerprintArgs), profile.ProxyId, profile.ProxyConfig,
		profile.ProxyBindSourceID, profile.ProxyBindSourceURL, profile.ProxyBindName, profile.ProxyBindUpdatedAt,
		string(launchArgs), normalizeProfileConfigJSON(profile.ProfileConfig), string(tags), string(keywords), profile.GroupId,
		profile.CreatedAt, profile.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("保存窗口配置失败: %w", err)
	}
	return nil
}

// Delete 删除窗口配置
func (d *SQLiteProfileDAO) Delete(profileId string) error {
	_, err := d.db.Exec(`DELETE FROM browser_profiles WHERE profile_id = ?`, profileId)
	if err != nil {
		return fmt.Errorf("删除窗口配置失败: %w", err)
	}
	return nil
}

// ListByGroup 按分组筛选窗口
// groupId 为空字符串时返回未分组的窗口
// includeChildren=true 时同时包含 childGroupIds 中的子分组窗口
func (d *SQLiteProfileDAO) ListByGroup(groupId string, includeChildren bool, childGroupIds []string) ([]*Profile, error) {
	var rows *sql.Rows
	var err error

	if includeChildren && len(childGroupIds) > 0 {
		// 构建 IN 子句，包含当前分组和所有子分组
		allIds := append([]string{groupId}, childGroupIds...)
		inClause := ""
		args := make([]interface{}, len(allIds))
		for i, id := range allIds {
			if i > 0 {
				inClause += ","
			}
			inClause += "?"
			args[i] = id
		}
		rows, err = d.db.Query(fmt.Sprintf(`
			SELECT profile_id, profile_name, user_data_dir, core_id,
			       fingerprint_args, proxy_id, proxy_config,
			       COALESCE(proxy_bind_source_id, ''), COALESCE(proxy_bind_source_url, ''),
			       COALESCE(proxy_bind_name, ''), COALESCE(proxy_bind_updated_at, ''),
			       launch_args,
			       COALESCE(profile_config, '{}'),
			       tags, keywords, group_id, created_at, updated_at
			FROM browser_profiles WHERE group_id IN (%s) ORDER BY created_at ASC`, inClause), args...)
	} else {
		// 仅查询指定分组
		rows, err = d.db.Query(`
			SELECT profile_id, profile_name, user_data_dir, core_id,
			       fingerprint_args, proxy_id, proxy_config,
			       COALESCE(proxy_bind_source_id, ''), COALESCE(proxy_bind_source_url, ''),
			       COALESCE(proxy_bind_name, ''), COALESCE(proxy_bind_updated_at, ''),
			       launch_args,
			       COALESCE(profile_config, '{}'),
			       tags, keywords, group_id, created_at, updated_at
			FROM browser_profiles WHERE group_id = ? ORDER BY created_at ASC`, groupId)
	}

	if err != nil {
		return nil, fmt.Errorf("按分组查询窗口失败: %w", err)
	}
	defer rows.Close()

	var list []*Profile
	for rows.Next() {
		p, err := scanProfile(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

// MoveToGroup 批量移动窗口到分组
func (d *SQLiteProfileDAO) MoveToGroup(profileIds []string, groupId string) error {
	if len(profileIds) == 0 {
		return nil
	}
	inClause := ""
	args := make([]interface{}, len(profileIds)+1)
	args[0] = groupId
	for i, id := range profileIds {
		if i > 0 {
			inClause += ","
		}
		inClause += "?"
		args[i+1] = id
	}
	_, err := d.db.Exec(fmt.Sprintf(`UPDATE browser_profiles SET group_id = ? WHERE profile_id IN (%s)`, inClause), args...)
	if err != nil {
		return fmt.Errorf("批量移动窗口失败: %w", err)
	}
	return nil
}

// scanner 统一扫描接口，兼容 *sql.Row 和 *sql.Rows
type scanner interface {
	Scan(dest ...any) error
}

func scanProfile(s scanner) (*Profile, error) {
	var (
		fingerprintArgsJSON, launchArgsJSON, profileConfigJSON, tagsJSON, keywordsJSON string
		p                                                                              Profile
	)
	err := s.Scan(
		&p.ProfileId, &p.ProfileName, &p.UserDataDir, &p.CoreId,
		&fingerprintArgsJSON, &p.ProxyId, &p.ProxyConfig,
		&p.ProxyBindSourceID, &p.ProxyBindSourceURL, &p.ProxyBindName, &p.ProxyBindUpdatedAt,
		&launchArgsJSON, &profileConfigJSON, &tagsJSON, &keywordsJSON, &p.GroupId,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(fingerprintArgsJSON), &p.FingerprintArgs)
	_ = json.Unmarshal([]byte(launchArgsJSON), &p.LaunchArgs)
	p.ProfileConfig = normalizeProfileConfigJSON(profileConfigJSON)
	_ = json.Unmarshal([]byte(tagsJSON), &p.Tags)
	_ = json.Unmarshal([]byte(keywordsJSON), &p.Keywords)
	if p.FingerprintArgs == nil {
		p.FingerprintArgs = []string{}
	}
	if p.LaunchArgs == nil {
		p.LaunchArgs = []string{}
	}
	if strings.TrimSpace(p.ProfileConfig) == "" {
		p.ProfileConfig = "{}"
	}
	if p.Tags == nil {
		p.Tags = []string{}
	}
	if p.Keywords == nil {
		p.Keywords = []string{}
	}
	return &p, nil
}
