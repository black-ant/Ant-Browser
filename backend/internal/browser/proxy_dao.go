package browser

import (
	"database/sql"
	"fmt"
	"time"
)

// ProxyDAO 代理列表持久化接口
type ProxyDAO interface {
	List() ([]Proxy, error)
	ReplaceAll(proxies []Proxy) error
	ListByGroup(groupName string) ([]Proxy, error)
	ListGroups() ([]string, error)
	Upsert(proxy Proxy) error
	Delete(proxyId string) error
	DeleteAll() error
	UpdateSpeedResult(proxyId string, ok bool, latencyMs int64, testedAt string) error
	UpdateIPHealthResult(proxyId string, healthJSON string) error
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
		SELECT proxy_id, proxy_name, proxy_config, COALESCE(preferred_kernel, ''), dns_servers, COALESCE(group_name, ''),
		       COALESCE(source_id, ''), COALESCE(source_url, ''), COALESCE(source_name_prefix, ''),
		       COALESCE(source_auto_refresh, 0), COALESCE(source_refresh_interval_m, 0), COALESCE(source_last_refresh_at, ''),
		       COALESCE(last_latency_ms, -1), COALESCE(last_test_ok, 0), COALESCE(last_tested_at, ''),
		       COALESCE(last_test_engine, ''), COALESCE(last_test_connector_type, ''), COALESCE(last_test_fingerprint, ''),
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
		SELECT proxy_id, proxy_name, proxy_config, COALESCE(preferred_kernel, ''), dns_servers, COALESCE(group_name, ''),
		       COALESCE(source_id, ''), COALESCE(source_url, ''), COALESCE(source_name_prefix, ''),
		       COALESCE(source_auto_refresh, 0), COALESCE(source_refresh_interval_m, 0), COALESCE(source_last_refresh_at, ''),
		       COALESCE(last_latency_ms, -1), COALESCE(last_test_ok, 0), COALESCE(last_tested_at, ''),
		       COALESCE(last_test_engine, ''), COALESCE(last_test_connector_type, ''), COALESCE(last_test_fingerprint, ''),
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
	if err := upsertProxy(d.db, proxy); err != nil {
		return err
	}
	return nil
}

type proxySQLExecutor interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func upsertProxy(exec proxySQLExecutor, proxy Proxy) error {
	now := time.Now().Format(time.RFC3339)
	autoRefreshInt := 0
	if proxy.SourceAutoRefresh {
		autoRefreshInt = 1
	}
	_, err := exec.Exec(`
		INSERT INTO browser_proxies (
		  proxy_id, proxy_name, proxy_config, preferred_kernel, dns_servers, group_name,
		  source_id, source_url, source_name_prefix, source_auto_refresh, source_refresh_interval_m, source_last_refresh_at,
		  sort_order, created_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(proxy_id) DO UPDATE SET
		  proxy_name   = excluded.proxy_name,
		  proxy_config = excluded.proxy_config,
		  preferred_kernel = excluded.preferred_kernel,
		  dns_servers  = excluded.dns_servers,
		  group_name   = excluded.group_name,
		  source_id    = excluded.source_id,
		  source_url   = excluded.source_url,
		  source_name_prefix = excluded.source_name_prefix,
		  source_auto_refresh = excluded.source_auto_refresh,
		  source_refresh_interval_m = excluded.source_refresh_interval_m,
		  source_last_refresh_at = excluded.source_last_refresh_at,
		  sort_order   = excluded.sort_order`,
		proxy.ProxyId, proxy.ProxyName, proxy.ProxyConfig, proxy.PreferredKernel, proxy.DnsServers, proxy.GroupName,
		proxy.SourceID, proxy.SourceURL, proxy.SourceNamePrefix, autoRefreshInt, proxy.SourceRefreshIntervalM, proxy.SourceLastRefreshAt,
		proxy.SortOrder, now,
	)
	if err != nil {
		return fmt.Errorf("保存代理失败: %w", err)
	}
	return nil
}

func (d *SQLiteProxyDAO) ReplaceAll(proxies []Proxy) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM browser_proxies`); err != nil {
		return err
	}
	for _, proxy := range proxies {
		if err := upsertProxy(tx, proxy); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
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
		UPDATE browser_proxies SET last_latency_ms=?, last_test_ok=?, last_tested_at=?,
		last_test_engine='', last_test_connector_type='', last_test_fingerprint=''
		WHERE proxy_id=?`, latencyMs, okInt, testedAt, proxyId)
	if err != nil {
		return fmt.Errorf("更新测速结果失败: %w", err)
	}
	return nil
}

func (d *SQLiteProxyDAO) UpdateSpeedResultWithProvenance(proxyId string, ok bool, latencyMs int64, testedAt string, engine string, connectorType string, fingerprint string) error {
	okInt := 0
	if ok {
		okInt = 1
	}
	_, err := d.db.Exec(`
		UPDATE browser_proxies SET last_latency_ms=?, last_test_ok=?, last_tested_at=?,
		last_test_engine=?, last_test_connector_type=?, last_test_fingerprint=?
		WHERE proxy_id=?`, latencyMs, okInt, testedAt, engine, connectorType, fingerprint, proxyId)
	if err != nil {
		return fmt.Errorf("更新测速结果来源失败: %w", err)
	}
	return nil
}

func (d *SQLiteProxyDAO) InvalidateSpeedResult(proxyId string) error {
	_, err := d.db.Exec(`
		UPDATE browser_proxies SET last_latency_ms=-1, last_test_ok=0, last_tested_at='',
		last_test_engine='', last_test_connector_type='', last_test_fingerprint=''
		WHERE proxy_id=?`, proxyId)
	if err != nil {
		return fmt.Errorf("使代理测速结果失效失败: %w", err)
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
			&p.ProxyId, &p.ProxyName, &p.ProxyConfig, &p.PreferredKernel, &p.DnsServers, &p.GroupName,
			&p.SourceID, &p.SourceURL, &p.SourceNamePrefix, &autoRefreshInt, &p.SourceRefreshIntervalM, &p.SourceLastRefreshAt,
			&p.LastLatencyMs, &okInt, &p.LastTestedAt, &p.LastTestEngine, &p.LastTestConnectorType, &p.LastTestFingerprint,
			&p.LastIPHealthJSON, &p.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("读取代理行失败: %w", err)
		}
		p.LastTestOk = okInt == 1
		p.SourceAutoRefresh = autoRefreshInt == 1
		list = append(list, p)
	}
	return list, rows.Err()
}
