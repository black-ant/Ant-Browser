package browser

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// SQLiteProxyVirtualGroupMemberStateDAO 基于 SQLite 的虚拟分组成员健康状态实现。
type SQLiteProxyVirtualGroupMemberStateDAO struct {
	db *sql.DB
}

func NewSQLiteProxyVirtualGroupMemberStateDAO(db *sql.DB) *SQLiteProxyVirtualGroupMemberStateDAO {
	return &SQLiteProxyVirtualGroupMemberStateDAO{db: db}
}

const proxyVirtualGroupMemberStateColumns = `
	COALESCE(group_id, ''), COALESCE(proxy_id, ''), COALESCE(state, 'unknown'),
	COALESCE(consecutive_failures, 0), COALESCE(consecutive_successes, 0),
	COALESCE(last_probe_at, ''), COALESCE(last_success_at, ''), COALESCE(last_failure_at, ''),
	COALESCE(next_probe_at, ''), COALESCE(last_error, ''), COALESCE(last_engine, ''),
	COALESCE(last_connector_type, ''), COALESCE(updated_at, '')`

func (d *SQLiteProxyVirtualGroupMemberStateDAO) List() ([]ProxyVirtualGroupMemberState, error) {
	rows, err := d.db.Query(`SELECT ` + proxyVirtualGroupMemberStateColumns + `
		FROM browser_proxy_virtual_group_member_state
		ORDER BY group_id ASC, proxy_id ASC`)
	if err != nil {
		return nil, fmt.Errorf("查询虚拟分组成员健康状态失败: %w", err)
	}
	defer rows.Close()
	return scanProxyVirtualGroupMemberStates(rows)
}

func (d *SQLiteProxyVirtualGroupMemberStateDAO) ListByGroup(groupId string) ([]ProxyVirtualGroupMemberState, error) {
	groupId = strings.TrimSpace(groupId)
	if groupId == "" {
		return []ProxyVirtualGroupMemberState{}, nil
	}
	rows, err := d.db.Query(`SELECT `+proxyVirtualGroupMemberStateColumns+`
		FROM browser_proxy_virtual_group_member_state
		WHERE group_id = ?
		ORDER BY proxy_id ASC`, groupId)
	if err != nil {
		return nil, fmt.Errorf("查询虚拟分组成员健康状态失败: %w", err)
	}
	defer rows.Close()
	return scanProxyVirtualGroupMemberStates(rows)
}

func (d *SQLiteProxyVirtualGroupMemberStateDAO) Upsert(state ProxyVirtualGroupMemberState) error {
	state.GroupId = strings.TrimSpace(state.GroupId)
	state.ProxyId = strings.TrimSpace(state.ProxyId)
	if state.GroupId == "" || state.ProxyId == "" {
		return fmt.Errorf("虚拟分组成员健康状态缺少分组或代理 ID")
	}
	state.State = NormalizeMemberHealthState(string(state.State))
	if state.UpdatedAt == "" {
		state.UpdatedAt = time.Now().Format(time.RFC3339)
	}
	_, err := d.db.Exec(`
		INSERT INTO browser_proxy_virtual_group_member_state (
			group_id, proxy_id, state, consecutive_failures, consecutive_successes,
			last_probe_at, last_success_at, last_failure_at, next_probe_at,
			last_error, last_engine, last_connector_type, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(group_id, proxy_id) DO UPDATE SET
			state = excluded.state,
			consecutive_failures = excluded.consecutive_failures,
			consecutive_successes = excluded.consecutive_successes,
			last_probe_at = excluded.last_probe_at,
			last_success_at = excluded.last_success_at,
			last_failure_at = excluded.last_failure_at,
			next_probe_at = excluded.next_probe_at,
			last_error = excluded.last_error,
			last_engine = excluded.last_engine,
			last_connector_type = excluded.last_connector_type,
			updated_at = excluded.updated_at`,
		state.GroupId, state.ProxyId, string(state.State),
		state.ConsecutiveFailures, state.ConsecutiveSuccesses,
		state.LastProbeAt, state.LastSuccessAt, state.LastFailureAt, state.NextProbeAt,
		state.LastError, state.LastEngine, state.LastConnectorType, state.UpdatedAt)
	if err != nil {
		return fmt.Errorf("保存虚拟分组成员健康状态失败: %w", err)
	}
	return nil
}

func (d *SQLiteProxyVirtualGroupMemberStateDAO) DeleteByGroup(groupId string) error {
	groupId = strings.TrimSpace(groupId)
	if groupId == "" {
		return nil
	}
	if _, err := d.db.Exec(`DELETE FROM browser_proxy_virtual_group_member_state WHERE group_id = ?`, groupId); err != nil {
		return fmt.Errorf("删除虚拟分组成员健康状态失败: %w", err)
	}
	return nil
}

func (d *SQLiteProxyVirtualGroupMemberStateDAO) ResetByGroup(groupId string) error {
	groupId = strings.TrimSpace(groupId)
	if groupId == "" {
		return nil
	}
	_, err := d.db.Exec(`
		UPDATE browser_proxy_virtual_group_member_state
		SET state = 'unknown', consecutive_failures = 0, consecutive_successes = 0,
		    next_probe_at = '', last_error = '', updated_at = ?
		WHERE group_id = ?`, time.Now().Format(time.RFC3339), groupId)
	if err != nil {
		return fmt.Errorf("重置虚拟分组成员健康状态失败: %w", err)
	}
	return nil
}

func scanProxyVirtualGroupMemberStates(rows *sql.Rows) ([]ProxyVirtualGroupMemberState, error) {
	states := make([]ProxyVirtualGroupMemberState, 0)
	for rows.Next() {
		var item ProxyVirtualGroupMemberState
		var state string
		if err := rows.Scan(
			&item.GroupId, &item.ProxyId, &state,
			&item.ConsecutiveFailures, &item.ConsecutiveSuccesses,
			&item.LastProbeAt, &item.LastSuccessAt, &item.LastFailureAt, &item.NextProbeAt,
			&item.LastError, &item.LastEngine, &item.LastConnectorType, &item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("读取虚拟分组成员健康状态失败: %w", err)
		}
		item.State = NormalizeMemberHealthState(state)
		states = append(states, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return states, nil
}
