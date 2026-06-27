package browser

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// TemplateDAO 窗口创建模板数据访问接口
type TemplateDAO interface {
	List() ([]*Template, error)
	GetById(templateId string) (*Template, error)
	Create(input TemplateInput) (*Template, error)
	Update(templateId string, input TemplateInput) (*Template, error)
	Delete(templateId string) error
}

// SQLiteTemplateDAO 基于 SQLite 的 TemplateDAO 实现
type SQLiteTemplateDAO struct {
	db *sql.DB
}

// NewSQLiteTemplateDAO 创建 SQLiteTemplateDAO
func NewSQLiteTemplateDAO(db *sql.DB) *SQLiteTemplateDAO {
	return &SQLiteTemplateDAO{db: db}
}

// List 查询所有模板，按创建时间升序
func (d *SQLiteTemplateDAO) List() ([]*Template, error) {
	rows, err := d.db.Query(`
		SELECT template_id, template_name, COALESCE(profile_config, '{}'), created_at, updated_at
		FROM browser_templates ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("查询模板列表失败: %w", err)
	}
	defer rows.Close()

	var list []*Template
	for rows.Next() {
		t, err := scanTemplate(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, t)
	}
	return list, rows.Err()
}

// GetById 根据 templateId 查询单个模板
func (d *SQLiteTemplateDAO) GetById(templateId string) (*Template, error) {
	row := d.db.QueryRow(`
		SELECT template_id, template_name, COALESCE(profile_config, '{}'), created_at, updated_at
		FROM browser_templates WHERE template_id = ?`, templateId)
	t, err := scanTemplate(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("模板不存在: %s", templateId)
	}
	return t, err
}

// Create 创建模板
func (d *SQLiteTemplateDAO) Create(input TemplateInput) (*Template, error) {
	name := strings.TrimSpace(input.TemplateName)
	if name == "" {
		return nil, errors.New("模板名称不能为空")
	}

	now := time.Now().Format(time.RFC3339)
	tpl := &Template{
		TemplateId:    uuid.New().String(),
		TemplateName:  name,
		ProfileConfig: normalizeTemplateConfig(input.ProfileConfig),
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	_, err := d.db.Exec(`
		INSERT INTO browser_templates (template_id, template_name, profile_config, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)`,
		tpl.TemplateId, tpl.TemplateName, tpl.ProfileConfig, tpl.CreatedAt, tpl.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("创建模板失败: %w", err)
	}
	return tpl, nil
}

// Update 更新模板
func (d *SQLiteTemplateDAO) Update(templateId string, input TemplateInput) (*Template, error) {
	name := strings.TrimSpace(input.TemplateName)
	if name == "" {
		return nil, errors.New("模板名称不能为空")
	}
	existing, err := d.GetById(templateId)
	if err != nil {
		return nil, err
	}

	now := time.Now().Format(time.RFC3339)
	config := normalizeTemplateConfig(input.ProfileConfig)
	_, err = d.db.Exec(`
		UPDATE browser_templates SET template_name = ?, profile_config = ?, updated_at = ?
		WHERE template_id = ?`,
		name, config, now, templateId)
	if err != nil {
		return nil, fmt.Errorf("更新模板失败: %w", err)
	}

	existing.TemplateName = name
	existing.ProfileConfig = config
	existing.UpdatedAt = now
	return existing, nil
}

// Delete 删除模板
func (d *SQLiteTemplateDAO) Delete(templateId string) error {
	_, err := d.db.Exec(`DELETE FROM browser_templates WHERE template_id = ?`, templateId)
	if err != nil {
		return fmt.Errorf("删除模板失败: %w", err)
	}
	return nil
}

func normalizeTemplateConfig(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "{}"
	}
	return s
}

func scanTemplate(s scanner) (*Template, error) {
	var t Template
	if err := s.Scan(&t.TemplateId, &t.TemplateName, &t.ProfileConfig, &t.CreatedAt, &t.UpdatedAt); err != nil {
		return nil, err
	}
	if strings.TrimSpace(t.ProfileConfig) == "" {
		t.ProfileConfig = "{}"
	}
	return &t, nil
}
