package browser

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ProxySourceDAO 代理订阅源与节点覆盖记录的持久化接口
type ProxySourceDAO interface {
	ListSources() ([]ProxySource, error)
	GetSource(sourceId string) (*ProxySource, error)
	UpsertSource(source ProxySource) error
	DeleteSource(sourceId string) error
	UpdateRefreshResult(sourceId string, lastRefreshAt string, lastRefreshError string) error

	ListOverrides(sourceId string) ([]ProxySourceOverride, error)
	UpsertOverride(o ProxySourceOverride) error
	DeleteOverride(sourceId string, nodeKey string) error
}

// SQLiteProxySourceDAO 基于 SQLite 的 ProxySourceDAO 实现
type SQLiteProxySourceDAO struct {
	db *sql.DB
}

// NewSQLiteProxySourceDAO 创建 SQLiteProxySourceDAO
func NewSQLiteProxySourceDAO(db *sql.DB) *SQLiteProxySourceDAO {
	return &SQLiteProxySourceDAO{db: db}
}

const proxySourceColumns = `source_id, source_url, source_name, group_name, name_prefix, dns_servers,
	auto_refresh, refresh_interval_m, import_strategy, last_refresh_at, last_refresh_error, created_at`

// ListSources 查询所有订阅源，按创建时间升序
func (d *SQLiteProxySourceDAO) ListSources() ([]ProxySource, error) {
	rows, err := d.db.Query(`SELECT ` + proxySourceColumns + ` FROM proxy_sources ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("查询订阅源列表失败: %w", err)
	}
	defer rows.Close()

	var list []ProxySource
	for rows.Next() {
		s, err := scanProxySource(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	return list, rows.Err()
}

// GetSource 查询单个订阅源
func (d *SQLiteProxySourceDAO) GetSource(sourceId string) (*ProxySource, error) {
	row := d.db.QueryRow(`SELECT `+proxySourceColumns+` FROM proxy_sources WHERE source_id = ?`, sourceId)
	s, err := scanProxySource(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("订阅源不存在: %s", sourceId)
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// UpsertSource 新增或更新订阅源
func (d *SQLiteProxySourceDAO) UpsertSource(source ProxySource) error {
	autoRefreshInt := 0
	if source.AutoRefresh {
		autoRefreshInt = 1
	}
	if source.ImportStrategy == "" {
		source.ImportStrategy = "merge"
	}
	if source.CreatedAt == "" {
		source.CreatedAt = time.Now().Format(time.RFC3339)
	}
	_, err := d.db.Exec(`
		INSERT INTO proxy_sources (
		  source_id, source_url, source_name, group_name, name_prefix, dns_servers,
		  auto_refresh, refresh_interval_m, import_strategy, last_refresh_at, last_refresh_error, created_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(source_id) DO UPDATE SET
		  source_url         = excluded.source_url,
		  source_name        = excluded.source_name,
		  group_name         = excluded.group_name,
		  name_prefix        = excluded.name_prefix,
		  dns_servers        = excluded.dns_servers,
		  auto_refresh       = excluded.auto_refresh,
		  refresh_interval_m = excluded.refresh_interval_m,
		  import_strategy    = excluded.import_strategy`,
		source.SourceID, source.SourceURL, source.SourceName, source.GroupName, source.NamePrefix, source.DnsServers,
		autoRefreshInt, source.RefreshIntervalM, source.ImportStrategy, source.LastRefreshAt, source.LastRefreshError, source.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("保存订阅源失败: %w", err)
	}
	return nil
}

// DeleteSource 删除订阅源及其覆盖记录（代理行由调用方按 source_id 处理）
func (d *SQLiteProxySourceDAO) DeleteSource(sourceId string) error {
	if _, err := d.db.Exec(`DELETE FROM proxy_source_overrides WHERE source_id = ?`, sourceId); err != nil {
		return fmt.Errorf("删除订阅源覆盖记录失败: %w", err)
	}
	if _, err := d.db.Exec(`DELETE FROM proxy_sources WHERE source_id = ?`, sourceId); err != nil {
		return fmt.Errorf("删除订阅源失败: %w", err)
	}
	return nil
}

// UpdateRefreshResult 更新订阅源最后刷新时间与错误信息
func (d *SQLiteProxySourceDAO) UpdateRefreshResult(sourceId string, lastRefreshAt string, lastRefreshError string) error {
	_, err := d.db.Exec(`
		UPDATE proxy_sources SET last_refresh_at = ?, last_refresh_error = ?
		WHERE source_id = ?`, lastRefreshAt, lastRefreshError, sourceId)
	if err != nil {
		return fmt.Errorf("更新订阅源刷新结果失败: %w", err)
	}
	return nil
}

// ListOverrides 查询某订阅源的所有覆盖记录
func (d *SQLiteProxySourceDAO) ListOverrides(sourceId string) ([]ProxySourceOverride, error) {
	rows, err := d.db.Query(`
		SELECT source_id, node_key, action, custom_name
		FROM proxy_source_overrides WHERE source_id = ?`, sourceId)
	if err != nil {
		return nil, fmt.Errorf("查询订阅源覆盖记录失败: %w", err)
	}
	defer rows.Close()

	var list []ProxySourceOverride
	for rows.Next() {
		var o ProxySourceOverride
		if err := rows.Scan(&o.SourceID, &o.NodeKey, &o.Action, &o.CustomName); err != nil {
			return nil, fmt.Errorf("读取订阅源覆盖记录失败: %w", err)
		}
		list = append(list, o)
	}
	return list, rows.Err()
}

// UpsertOverride 新增或更新一条覆盖记录
func (d *SQLiteProxySourceDAO) UpsertOverride(o ProxySourceOverride) error {
	if o.Action == "" {
		o.Action = "ignore"
	}
	_, err := d.db.Exec(`
		INSERT INTO proxy_source_overrides (source_id, node_key, action, custom_name)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(source_id, node_key) DO UPDATE SET
		  action      = excluded.action,
		  custom_name = excluded.custom_name`,
		o.SourceID, o.NodeKey, o.Action, o.CustomName,
	)
	if err != nil {
		return fmt.Errorf("保存订阅源覆盖记录失败: %w", err)
	}
	return nil
}

// DeleteOverride 删除一条覆盖记录
func (d *SQLiteProxySourceDAO) DeleteOverride(sourceId string, nodeKey string) error {
	_, err := d.db.Exec(`DELETE FROM proxy_source_overrides WHERE source_id = ? AND node_key = ?`, sourceId, nodeKey)
	if err != nil {
		return fmt.Errorf("删除订阅源覆盖记录失败: %w", err)
	}
	return nil
}

func scanProxySource(s scanner) (ProxySource, error) {
	var src ProxySource
	var autoRefreshInt int
	if err := s.Scan(
		&src.SourceID, &src.SourceURL, &src.SourceName, &src.GroupName, &src.NamePrefix, &src.DnsServers,
		&autoRefreshInt, &src.RefreshIntervalM, &src.ImportStrategy, &src.LastRefreshAt, &src.LastRefreshError, &src.CreatedAt,
	); err != nil {
		return ProxySource{}, err
	}
	src.AutoRefresh = autoRefreshInt == 1
	return src, nil
}
