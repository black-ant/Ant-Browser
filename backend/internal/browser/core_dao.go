package browser

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"ant-chrome/backend/internal/config"
)

// CoreDAO 内核配置持久化接口
type CoreDAO interface {
	List() ([]Core, error)
	Upsert(core Core) error
	Delete(coreId string) error
	SetDefault(coreId string) error
}

// SQLiteCoreDAO 基于 SQLite 的 CoreDAO 实现
type SQLiteCoreDAO struct {
	db *sql.DB
}

// NewSQLiteCoreDAO 创建 SQLiteCoreDAO
func NewSQLiteCoreDAO(db *sql.DB) *SQLiteCoreDAO {
	return &SQLiteCoreDAO{db: db}
}

// marshalCoreEnv 将环境变量列表序列化为 JSON 文本，空列表统一存 "[]"。
func marshalCoreEnv(env []string) string {
	normalized := normalizeCoreEnvEntries(env)
	if len(normalized) == 0 {
		return "[]"
	}
	data, err := json.Marshal(normalized)
	if err != nil {
		return "[]"
	}
	return string(data)
}

// unmarshalCoreEnv 解析环境变量 JSON 文本；非法内容按空列表处理，避免坏数据阻塞内核读取。
func unmarshalCoreEnv(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" {
		return nil
	}
	var env []string
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		return nil
	}
	return normalizeCoreEnvEntries(env)
}

func normalizeCoreEnvEntries(env []string) []string {
	if len(env) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(env))
	for _, item := range env {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			normalized = append(normalized, trimmed)
		}
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

// List 查询所有内核，按 sort_order 升序
func (d *SQLiteCoreDAO) List() ([]Core, error) {
	rows, err := d.db.Query(`
		SELECT core_id, core_name, core_path, is_default,
		       COALESCE(core_backend, ''), COALESCE(core_env, '[]')
		FROM browser_cores ORDER BY sort_order ASC, created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("查询内核列表失败: %w", err)
	}
	defer rows.Close()

	var list []Core
	for rows.Next() {
		var c Core
		var isDefault int
		var coreBackend string
		var coreEnv string
		if err := rows.Scan(&c.CoreId, &c.CoreName, &c.CorePath, &isDefault, &coreBackend, &coreEnv); err != nil {
			return nil, fmt.Errorf("读取内核行失败: %w", err)
		}
		c.IsDefault = isDefault == 1
		c.CoreBackend = config.NormalizeCoreBackend(coreBackend)
		c.CoreEnv = unmarshalCoreEnv(coreEnv)
		list = append(list, c)
	}
	return list, rows.Err()
}

// Upsert 新增或更新内核配置
func (d *SQLiteCoreDAO) Upsert(core Core) error {
	now := time.Now().Format(time.RFC3339)
	isDefault := 0
	if core.IsDefault {
		isDefault = 1
	}
	_, err := d.db.Exec(`
		INSERT INTO browser_cores (core_id, core_name, core_path, is_default, core_backend, core_env, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(core_id) DO UPDATE SET
		  core_name    = excluded.core_name,
		  core_path    = excluded.core_path,
		  is_default   = excluded.is_default,
		  core_backend = excluded.core_backend,
		  core_env     = excluded.core_env`,
		core.CoreId, core.CoreName, core.CorePath, isDefault,
		config.NormalizeCoreBackend(core.CoreBackend), marshalCoreEnv(core.CoreEnv), now,
	)
	if err != nil {
		return fmt.Errorf("保存内核配置失败: %w", err)
	}
	return nil
}

// Delete 删除内核配置
func (d *SQLiteCoreDAO) Delete(coreId string) error {
	_, err := d.db.Exec(`DELETE FROM browser_cores WHERE core_id = ?`, coreId)
	if err != nil {
		return fmt.Errorf("删除内核配置失败: %w", err)
	}
	return nil
}

// SetDefault 设置默认内核（先清除所有默认标记，再设置指定内核）
func (d *SQLiteCoreDAO) SetDefault(coreId string) error {
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`UPDATE browser_cores SET is_default = 0`); err != nil {
		return fmt.Errorf("清除默认内核失败: %w", err)
	}
	if _, err := tx.Exec(`UPDATE browser_cores SET is_default = 1 WHERE core_id = ?`, coreId); err != nil {
		return fmt.Errorf("设置默认内核失败: %w", err)
	}
	return tx.Commit()
}
