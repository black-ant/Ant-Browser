package browser

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// GroupDAO 分组数据访问接口
type GroupDAO interface {
	List() ([]*Group, error)
	GetById(groupId string) (*Group, error)
	Create(input GroupInput) (*Group, error)
	Update(groupId string, input GroupInput) (*Group, error)
	Delete(groupId string) error
	GetChildren(parentId string) ([]*Group, error)
	MoveChildren(fromGroupId, toGroupId string) error
}

// SQLiteGroupDAO 基于 SQLite 的 GroupDAO 实现
type SQLiteGroupDAO struct {
	db *sql.DB
}

// NewSQLiteGroupDAO 创建 SQLiteGroupDAO
func NewSQLiteGroupDAO(db *sql.DB) *SQLiteGroupDAO {
	return &SQLiteGroupDAO{db: db}
}

// List 查询所有分组
func (d *SQLiteGroupDAO) List() ([]*Group, error) {
	rows, err := d.db.Query(`
		SELECT group_id, group_name, parent_id, sort_order, created_at, updated_at
		FROM browser_groups ORDER BY sort_order ASC, created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("查询分组列表失败: %w", err)
	}
	defer rows.Close()

	var list []*Group
	for rows.Next() {
		g, err := scanGroup(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, g)
	}
	return list, rows.Err()
}

// GetById 根据 groupId 查询单个分组
func (d *SQLiteGroupDAO) GetById(groupId string) (*Group, error) {
	row := d.db.QueryRow(`
		SELECT group_id, group_name, parent_id, sort_order, created_at, updated_at
		FROM browser_groups WHERE group_id = ?`, groupId)
	g, err := scanGroup(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("分组不存在: %s", groupId)
	}
	return g, err
}

// Create 创建分组
func (d *SQLiteGroupDAO) Create(input GroupInput) (*Group, error) {
	if input.GroupName == "" {
		return nil, errors.New("分组名称不能为空")
	}
	// 验证父分组存在性
	if input.ParentId != "" {
		_, err := d.GetById(input.ParentId)
		if err != nil {
			return nil, fmt.Errorf("父分组不存在: %s", input.ParentId)
		}
	}

	now := time.Now().Format(time.RFC3339)
	group := &Group{
		GroupId:   uuid.New().String(),
		GroupName: input.GroupName,
		ParentId:  input.ParentId,
		SortOrder: input.SortOrder,
		CreatedAt: now,
		UpdatedAt: now,
	}

	_, err := d.db.Exec(`
		INSERT INTO browser_groups (group_id, group_name, parent_id, sort_order, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		group.GroupId, group.GroupName, group.ParentId, group.SortOrder, group.CreatedAt, group.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("创建分组失败: %w", err)
	}
	return group, nil
}

// Update 更新分组
func (d *SQLiteGroupDAO) Update(groupId string, input GroupInput) (*Group, error) {
	if input.GroupName == "" {
		return nil, errors.New("分组名称不能为空")
	}
	// 检查分组是否存在
	existing, err := d.GetById(groupId)
	if err != nil {
		return nil, err
	}
	// 验证父分组存在性
	if input.ParentId != "" {
		_, err := d.GetById(input.ParentId)
		if err != nil {
			return nil, fmt.Errorf("父分组不存在: %s", input.ParentId)
		}
		// 检查循环引用
		if err := d.checkCircularReference(groupId, input.ParentId); err != nil {
			return nil, err
		}
	}

	now := time.Now().Format(time.RFC3339)
	_, err = d.db.Exec(`
		UPDATE browser_groups SET group_name = ?, parent_id = ?, sort_order = ?, updated_at = ?
		WHERE group_id = ?`,
		input.GroupName, input.ParentId, input.SortOrder, now, groupId)
	if err != nil {
		return nil, fmt.Errorf("更新分组失败: %w", err)
	}

	existing.GroupName = input.GroupName
	existing.ParentId = input.ParentId
	existing.SortOrder = input.SortOrder
	existing.UpdatedAt = now
	return existing, nil
}

// Delete 删除分组（级联处理：子分组和窗口移动到父分组）
func (d *SQLiteGroupDAO) Delete(groupId string) error {
	group, err := d.GetById(groupId)
	if err != nil {
		return err
	}
	// 将子分组移动到父分组
	if err := d.MoveChildren(groupId, group.ParentId); err != nil {
		return err
	}
	// 将该分组下的窗口移动到父分组
	_, err = d.db.Exec(`UPDATE browser_profiles SET group_id = ? WHERE group_id = ?`, group.ParentId, groupId)
	if err != nil {
		return fmt.Errorf("移动窗口失败: %w", err)
	}
	// 删除分组
	_, err = d.db.Exec(`DELETE FROM browser_groups WHERE group_id = ?`, groupId)
	if err != nil {
		return fmt.Errorf("删除分组失败: %w", err)
	}
	return nil
}

// GetChildren 获取子分组
func (d *SQLiteGroupDAO) GetChildren(parentId string) ([]*Group, error) {
	rows, err := d.db.Query(`
		SELECT group_id, group_name, parent_id, sort_order, created_at, updated_at
		FROM browser_groups WHERE parent_id = ? ORDER BY sort_order ASC`, parentId)
	if err != nil {
		return nil, fmt.Errorf("查询子分组失败: %w", err)
	}
	defer rows.Close()

	var list []*Group
	for rows.Next() {
		g, err := scanGroup(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, g)
	}
	return list, rows.Err()
}

// MoveChildren 将子分组移动到新的父分组
func (d *SQLiteGroupDAO) MoveChildren(fromGroupId, toGroupId string) error {
	_, err := d.db.Exec(`UPDATE browser_groups SET parent_id = ? WHERE parent_id = ?`, toGroupId, fromGroupId)
	if err != nil {
		return fmt.Errorf("移动子分组失败: %w", err)
	}
	return nil
}

// checkCircularReference 检查循环引用
func (d *SQLiteGroupDAO) checkCircularReference(groupId, newParentId string) error {
	if newParentId == groupId {
		return errors.New("不能将分组设为自己的子分组")
	}
	// 遍历祖先链检查是否包含 groupId
	currentId := newParentId
	visited := make(map[string]bool)
	for currentId != "" {
		if visited[currentId] {
			return errors.New("检测到循环引用")
		}
		visited[currentId] = true
		if currentId == groupId {
			return errors.New("不能将分组设为自己的后代分组")
		}
		parent, err := d.GetById(currentId)
		if err != nil {
			break
		}
		currentId = parent.ParentId
	}
	return nil
}

// scanGroup 扫描分组行
func scanGroup(s scanner) (*Group, error) {
	var g Group
	err := s.Scan(&g.GroupId, &g.GroupName, &g.ParentId, &g.SortOrder, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &g, nil
}
