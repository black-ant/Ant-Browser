package browser

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type SQLiteProxyVirtualGroupDAO struct {
	db *sql.DB
}

func NewSQLiteProxyVirtualGroupDAO(db *sql.DB) *SQLiteProxyVirtualGroupDAO {
	return &SQLiteProxyVirtualGroupDAO{db: db}
}

func (d *SQLiteProxyVirtualGroupDAO) List() ([]ProxyVirtualGroup, error) {
	rows, err := d.db.Query(`
		SELECT COALESCE(group_id, ''), COALESCE(group_name, ''),
		       COALESCE(strategy, 'round_robin'), COALESCE(selected_proxy_id, ''),
		       COALESCE(member_proxy_ids, '[]'), COALESCE(member_group_names, '[]'),
		       COALESCE(sort_order, 0), COALESCE(created_at, ''), COALESCE(updated_at, '')
		FROM browser_proxy_virtual_groups
		ORDER BY sort_order ASC, group_name COLLATE NOCASE ASC`)
	if err != nil {
		return nil, fmt.Errorf("查询虚拟代理分组失败: %w", err)
	}
	defer rows.Close()

	groups := make([]ProxyVirtualGroup, 0)
	for rows.Next() {
		group, err := d.scanProxyVirtualGroup(rows)
		if err != nil {
			return nil, err
		}
		groups = append(groups, group)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return groups, nil
}

func (d *SQLiteProxyVirtualGroupDAO) GetByName(groupName string) (*ProxyVirtualGroup, error) {
	groupName = strings.TrimSpace(groupName)
	if groupName == "" {
		return nil, sql.ErrNoRows
	}
	row := d.db.QueryRow(`
		SELECT COALESCE(group_id, ''), COALESCE(group_name, ''),
		       COALESCE(strategy, 'round_robin'), COALESCE(selected_proxy_id, ''),
		       COALESCE(member_proxy_ids, '[]'), COALESCE(member_group_names, '[]'),
		       COALESCE(sort_order, 0), COALESCE(created_at, ''), COALESCE(updated_at, '')
		FROM browser_proxy_virtual_groups
		WHERE group_name = ? COLLATE NOCASE`, groupName)
	group, err := d.scanProxyVirtualGroup(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, sql.ErrNoRows
	}
	if err != nil {
		return nil, fmt.Errorf("查询虚拟代理分组失败: %w", err)
	}
	return &group, nil
}

func (d *SQLiteProxyVirtualGroupDAO) Upsert(group ProxyVirtualGroup) error {
	group = ResolveProxyVirtualGroupFromDatabase(d.db, group)
	if group.GroupId == "" {
		group.GroupId = uuid.NewString()
	}
	if group.GroupName == "" {
		return fmt.Errorf("虚拟代理分组名称不能为空")
	}
	now := time.Now().Format(time.RFC3339)
	if group.CreatedAt == "" {
		group.CreatedAt = now
	}
	group.UpdatedAt = now
	memberProxyIDs, err := json.Marshal(group.MemberProxyIds)
	if err != nil {
		return fmt.Errorf("序列化虚拟代理分组代理节点失败: %w", err)
	}
	memberGroupNames, err := json.Marshal(group.MemberGroupNames)
	if err != nil {
		return fmt.Errorf("序列化虚拟代理分组历史成员失败: %w", err)
	}
	_, err = d.db.Exec(`
		INSERT INTO browser_proxy_virtual_groups (
			group_id, group_name, strategy, selected_proxy_id, member_proxy_ids, member_group_names, sort_order, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(group_id) DO UPDATE SET
			group_name = excluded.group_name,
			strategy = excluded.strategy,
			selected_proxy_id = excluded.selected_proxy_id,
			member_proxy_ids = excluded.member_proxy_ids,
			member_group_names = excluded.member_group_names,
			sort_order = excluded.sort_order,
			updated_at = excluded.updated_at`,
		group.GroupId, group.GroupName, group.Strategy, group.SelectedProxyId, string(memberProxyIDs), string(memberGroupNames), group.SortOrder, group.CreatedAt, group.UpdatedAt)
	if err != nil {
		return fmt.Errorf("保存虚拟代理分组失败: %w", err)
	}
	return nil
}

func (d *SQLiteProxyVirtualGroupDAO) Delete(groupId string) error {
	groupId = strings.TrimSpace(groupId)
	if groupId == "" {
		return fmt.Errorf("虚拟代理分组 ID 不能为空")
	}
	if _, err := d.db.Exec(`DELETE FROM browser_proxy_virtual_groups WHERE group_id = ?`, groupId); err != nil {
		return fmt.Errorf("删除虚拟代理分组失败: %w", err)
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanProxyVirtualGroup(row rowScanner) (ProxyVirtualGroup, error) {
	var (
		group                ProxyVirtualGroup
		strategy             string
		selectedProxyID      string
		memberProxyIDsJSON   string
		memberGroupNamesJSON string
	)
	if err := row.Scan(&group.GroupId, &group.GroupName, &strategy, &selectedProxyID, &memberProxyIDsJSON, &memberGroupNamesJSON, &group.SortOrder, &group.CreatedAt, &group.UpdatedAt); err != nil {
		return ProxyVirtualGroup{}, err
	}
	group.Strategy = strategy
	group.SelectedProxyId = selectedProxyID
	if err := json.Unmarshal([]byte(memberProxyIDsJSON), &group.MemberProxyIds); err != nil {
		return ProxyVirtualGroup{}, fmt.Errorf("解析虚拟代理分组代理节点失败: %w", err)
	}
	if err := json.Unmarshal([]byte(memberGroupNamesJSON), &group.MemberGroupNames); err != nil {
		return ProxyVirtualGroup{}, fmt.Errorf("解析虚拟代理分组成员失败: %w", err)
	}
	return NormalizeProxyVirtualGroup(group), nil
}

func (d *SQLiteProxyVirtualGroupDAO) scanProxyVirtualGroup(row rowScanner) (ProxyVirtualGroup, error) {
	group, err := scanProxyVirtualGroup(row)
	if err != nil {
		return ProxyVirtualGroup{}, err
	}
	return NormalizeProxyVirtualGroup(group), nil
}

func ResolveProxyVirtualGroupFromDatabase(db *sql.DB, group ProxyVirtualGroup) ProxyVirtualGroup {
	group = NormalizeProxyVirtualGroup(group)
	if db == nil || len(group.MemberProxyIds) > 0 || len(group.MemberGroupNames) == 0 {
		return group
	}
	rows, err := db.Query(`
		SELECT proxy_id, proxy_name, proxy_config, group_name
		FROM browser_proxies ORDER BY sort_order ASC, created_at ASC`)
	if err != nil {
		return group
	}
	defer rows.Close()
	proxies := make([]Proxy, 0)
	for rows.Next() {
		var item Proxy
		if err := rows.Scan(&item.ProxyId, &item.ProxyName, &item.ProxyConfig, &item.GroupName); err != nil {
			return group
		}
		proxies = append(proxies, item)
	}
	return ResolveProxyVirtualGroup(group, proxies)
}
