package browser

import (
	"database/sql"
	"fmt"
	"time"
)

// ProxyDAO 代理列表持久化接口
type ProxyDAO interface {
	List() ([]Proxy, error)
	ListByGroup(groupName string) ([]Proxy, error)
	ListGroups() ([]string, error)
	Upsert(proxy Proxy) error
	Delete(proxyId string) error
	DeleteAll() error
	// ReplaceAllProxies 在单个事务内整表替换（DeleteAll + 重新插入），并在事务内回填测速/IP健康列，
	// 避免部分失败丢数据，也避免重插把健康结果重置为默认值。
	ReplaceAllProxies(proxies []Proxy) error
	// ReplaceSourceProxies 在单个事务内用给定集合替换某订阅源的全部代理：
	// 删除该源中已消失的代理、Upsert 新集合（存活行经 Upsert 保留 proxyId 与健康列）。仅触碰该源。
	ReplaceSourceProxies(sourceId string, proxies []Proxy) error
	UpdateSpeedResult(proxyId string, ok bool, latencyMs int64, testedAt string) error
	UpdateIPHealthResult(proxyId string, healthJSON string) error
}

// sqlExecer 抽象 *sql.DB 与 *sql.Tx 的 Exec，使 Upsert 可在事务内复用
type sqlExecer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

// SQLiteProxyDAO 基于 SQLite 的 ProxyDAO 实现
type SQLiteProxyDAO struct {
	db *sql.DB
}

// NewSQLiteProxyDAO 创建 SQLiteProxyDAO
func NewSQLiteProxyDAO(db *sql.DB) *SQLiteProxyDAO {
	return &SQLiteProxyDAO{db: db}
}

// List 查询所有代理，按 sort_order 升序
func (d *SQLiteProxyDAO) List() ([]Proxy, error) {
	rows, err := d.db.Query(`
		SELECT proxy_id, proxy_name, proxy_config, dns_servers, COALESCE(group_name, ''),
		       COALESCE(source_id, ''), COALESCE(source_url, ''), COALESCE(source_name_prefix, ''),
		       COALESCE(source_auto_refresh, 0), COALESCE(source_refresh_interval_m, 0), COALESCE(source_last_refresh_at, ''),
		       COALESCE(last_latency_ms, -1), COALESCE(last_test_ok, 0), COALESCE(last_tested_at, ''),
		       COALESCE(last_ip_health_json, ''),
		       sort_order
		FROM browser_proxies ORDER BY sort_order ASC, created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("查询代理列表失败: %w", err)
	}
	defer rows.Close()
	return scanProxies(rows)
}

// ListByGroup 按分组名称查询代理
func (d *SQLiteProxyDAO) ListByGroup(groupName string) ([]Proxy, error) {
	rows, err := d.db.Query(`
		SELECT proxy_id, proxy_name, proxy_config, dns_servers, COALESCE(group_name, ''),
		       COALESCE(source_id, ''), COALESCE(source_url, ''), COALESCE(source_name_prefix, ''),
		       COALESCE(source_auto_refresh, 0), COALESCE(source_refresh_interval_m, 0), COALESCE(source_last_refresh_at, ''),
		       COALESCE(last_latency_ms, -1), COALESCE(last_test_ok, 0), COALESCE(last_tested_at, ''),
		       COALESCE(last_ip_health_json, ''),
		       sort_order
		FROM browser_proxies WHERE group_name = ?
		ORDER BY sort_order ASC, created_at ASC`, groupName)
	if err != nil {
		return nil, fmt.Errorf("按分组查询代理失败: %w", err)
	}
	defer rows.Close()
	return scanProxies(rows)
}

// ListGroups 获取所有非空分组名称（去重）
func (d *SQLiteProxyDAO) ListGroups() ([]string, error) {
	rows, err := d.db.Query(`
		SELECT DISTINCT group_name FROM browser_proxies
		WHERE group_name != '' ORDER BY group_name ASC`)
	if err != nil {
		return nil, fmt.Errorf("查询代理分组失败: %w", err)
	}
	defer rows.Close()

	var groups []string
	for rows.Next() {
		var g string
		if err := rows.Scan(&g); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

// Upsert 新增或更新代理
func (d *SQLiteProxyDAO) Upsert(proxy Proxy) error {
	return upsertProxyExec(d.db, proxy)
}

// upsertProxyExec 在给定执行器（*sql.DB 或 *sql.Tx）上 Upsert 单个代理。
// 注意：ON CONFLICT 不触碰 last_latency_ms/last_test_ok/last_tested_at/last_ip_health_json，
// 因此对已存在行的 Upsert 会保留其测速/IP 健康结果。
func upsertProxyExec(ex sqlExecer, proxy Proxy) error {
	now := time.Now().Format(time.RFC3339)
	autoRefreshInt := 0
	if proxy.SourceAutoRefresh {
		autoRefreshInt = 1
	}
	_, err := ex.Exec(`
		INSERT INTO browser_proxies (
		  proxy_id, proxy_name, proxy_config, dns_servers, group_name,
		  source_id, source_url, source_name_prefix, source_auto_refresh, source_refresh_interval_m, source_last_refresh_at,
		  sort_order, created_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(proxy_id) DO UPDATE SET
		  proxy_name   = excluded.proxy_name,
		  proxy_config = excluded.proxy_config,
		  dns_servers  = excluded.dns_servers,
		  group_name   = excluded.group_name,
		  source_id    = excluded.source_id,
		  source_url   = excluded.source_url,
		  source_name_prefix = excluded.source_name_prefix,
		  source_auto_refresh = excluded.source_auto_refresh,
		  source_refresh_interval_m = excluded.source_refresh_interval_m,
		  source_last_refresh_at = excluded.source_last_refresh_at,
		  sort_order   = excluded.sort_order`,
		proxy.ProxyId, proxy.ProxyName, proxy.ProxyConfig, proxy.DnsServers, proxy.GroupName,
		proxy.SourceID, proxy.SourceURL, proxy.SourceNamePrefix, autoRefreshInt, proxy.SourceRefreshIntervalM, proxy.SourceLastRefreshAt,
		proxy.SortOrder, now,
	)
	if err != nil {
		return fmt.Errorf("保存代理失败: %w", err)
	}
	return nil
}

// ReplaceAllProxies 事务化整表替换，并在事务内回填测速/IP 健康结果
func (d *SQLiteProxyDAO) ReplaceAllProxies(proxies []Proxy) error {
	// 替换前快照测速/IP 健康（这些列仅由 Update* 写入，重新 INSERT 会重置为默认值）
	existing, _ := d.List()
	healthById := make(map[string]Proxy, len(existing))
	for _, p := range existing {
		healthById[p.ProxyId] = p
	}

	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM browser_proxies`); err != nil {
		return fmt.Errorf("清空代理表失败: %w", err)
	}
	for _, p := range proxies {
		if err := upsertProxyExec(tx, p); err != nil {
			return err
		}
	}
	// 回填健康结果（仅对仍存在的代理）
	for _, p := range proxies {
		old, ok := healthById[p.ProxyId]
		if !ok {
			continue
		}
		if old.LastTestedAt != "" {
			okInt := 0
			if old.LastTestOk {
				okInt = 1
			}
			if _, err := tx.Exec(`UPDATE browser_proxies SET last_latency_ms=?, last_test_ok=?, last_tested_at=? WHERE proxy_id=?`,
				old.LastLatencyMs, okInt, old.LastTestedAt, p.ProxyId); err != nil {
				return fmt.Errorf("回填测速结果失败: %w", err)
			}
		}
		if old.LastIPHealthJSON != "" {
			if _, err := tx.Exec(`UPDATE browser_proxies SET last_ip_health_json=? WHERE proxy_id=?`,
				old.LastIPHealthJSON, p.ProxyId); err != nil {
				return fmt.Errorf("回填 IP 健康结果失败: %w", err)
			}
		}
	}
	return tx.Commit()
}

// ReplaceSourceProxies 事务化替换某订阅源的代理集合，仅触碰该源
func (d *SQLiteProxyDAO) ReplaceSourceProxies(sourceId string, proxies []Proxy) error {
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer tx.Rollback()

	// 该源现有 proxyId（在事务内读取，避免 TOCTOU）
	rows, err := tx.Query(`SELECT proxy_id FROM browser_proxies WHERE source_id = ?`, sourceId)
	if err != nil {
		return fmt.Errorf("查询订阅源代理失败: %w", err)
	}
	existingIds := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("读取订阅源代理失败: %w", err)
		}
		existingIds = append(existingIds, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	newIds := make(map[string]bool, len(proxies))
	for _, p := range proxies {
		newIds[p.ProxyId] = true
	}
	// 删除该源中已消失的代理
	for _, id := range existingIds {
		if newIds[id] {
			continue
		}
		if _, err := tx.Exec(`DELETE FROM browser_proxies WHERE proxy_id = ?`, id); err != nil {
			return fmt.Errorf("删除订阅源代理失败: %w", err)
		}
	}
	// Upsert 新集合（存活行保留健康列与 proxyId）
	for _, p := range proxies {
		if err := upsertProxyExec(tx, p); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Delete 删除单个代理
func (d *SQLiteProxyDAO) Delete(proxyId string) error {
	_, err := d.db.Exec(`DELETE FROM browser_proxies WHERE proxy_id = ?`, proxyId)
	if err != nil {
		return fmt.Errorf("删除代理失败: %w", err)
	}
	return nil
}

// DeleteAll 清空代理表（批量保存前使用）
func (d *SQLiteProxyDAO) DeleteAll() error {
	_, err := d.db.Exec(`DELETE FROM browser_proxies`)
	if err != nil {
		return fmt.Errorf("清空代理表失败: %w", err)
	}
	return nil
}

// UpdateSpeedResult 更新单个代理的测速结果
func (d *SQLiteProxyDAO) UpdateSpeedResult(proxyId string, ok bool, latencyMs int64, testedAt string) error {
	okInt := 0
	if ok {
		okInt = 1
	}
	_, err := d.db.Exec(`
		UPDATE browser_proxies SET last_latency_ms=?, last_test_ok=?, last_tested_at=?
		WHERE proxy_id=?`, latencyMs, okInt, testedAt, proxyId)
	if err != nil {
		return fmt.Errorf("更新测速结果失败: %w", err)
	}
	return nil
}

// UpdateIPHealthResult 更新单个代理的 IP 健康检测结果（JSON 字符串）
func (d *SQLiteProxyDAO) UpdateIPHealthResult(proxyId string, healthJSON string) error {
	_, err := d.db.Exec(`
		UPDATE browser_proxies SET last_ip_health_json=?
		WHERE proxy_id=?`, healthJSON, proxyId)
	if err != nil {
		return fmt.Errorf("更新 IP 健康结果失败: %w", err)
	}
	return nil
}

func scanProxies(rows *sql.Rows) ([]Proxy, error) {
	var list []Proxy
	for rows.Next() {
		var p Proxy
		var okInt int
		var autoRefreshInt int
		if err := rows.Scan(
			&p.ProxyId, &p.ProxyName, &p.ProxyConfig, &p.DnsServers, &p.GroupName,
			&p.SourceID, &p.SourceURL, &p.SourceNamePrefix, &autoRefreshInt, &p.SourceRefreshIntervalM, &p.SourceLastRefreshAt,
			&p.LastLatencyMs, &okInt, &p.LastTestedAt, &p.LastIPHealthJSON, &p.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("读取代理行失败: %w", err)
		}
		p.LastTestOk = okInt == 1
		p.SourceAutoRefresh = autoRefreshInt == 1
		list = append(list, p)
	}
	return list, rows.Err()
}
