package backend

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"ant-chrome/backend/internal/logger"
)

// BrowserExtension 浏览器扩展
type BrowserExtension struct {
	ExtensionID      string   `json:"extensionId"`
	ExtensionName    string   `json:"extensionName"`
	ExtensionPath    string   `json:"extensionPath"`
	Version          string   `json:"version"`
	Enabled          bool     `json:"enabled"`
	BoundProfileIDs  []string `json:"boundProfileIds"` // 绑定的实例ID
	SourceType       string   `json:"sourceType"`      // local/crx/url/store
	SourceURL        string   `json:"sourceUrl"`
	Description      string   `json:"description"`
	CreatedAt        string   `json:"createdAt"`
	UpdatedAt        string   `json:"updatedAt"`
}

// BrowserExtensionInput 扩展输入
type BrowserExtensionInput struct {
	ExtensionName   string   `json:"extensionName"`
	ExtensionPath   string   `json:"extensionPath"`
	Version         string   `json:"version"`
	Enabled         bool     `json:"enabled"`
	BoundProfileIDs []string `json:"boundProfileIds"`
	SourceType      string   `json:"sourceType"`
	SourceURL       string   `json:"sourceUrl"`
	Description     string   `json:"description"`
}

// BrowserExtensionCreate 创建扩展
func (a *App) BrowserExtensionCreate(input BrowserExtensionInput) (*BrowserExtension, error) {
	if input.ExtensionName == "" {
		return nil, fmt.Errorf("扩展名称不能为空")
	}
	if input.ExtensionPath == "" {
		return nil, fmt.Errorf("扩展路径不能为空")
	}
	// 本地扩展必须是含 manifest.json 的有效目录（store/builtin 仅作引用，不校验本地路径）
	if isLocalExtensionSource(input.SourceType) {
		if res := validateExtensionDir(input.ExtensionPath); !res.Valid {
			return nil, fmt.Errorf("扩展目录无效：%s", res.Message)
		}
	}

	extensionID := fmt.Sprintf("ext-%s", generateUUID()[:8])
	now := time.Now().UTC().Format(time.RFC3339)

	// 序列化绑定实例ID
	boundIDsJSON, err := json.Marshal(input.BoundProfileIDs)
	if err != nil {
		return nil, fmt.Errorf("序列化绑定实例ID失败: %w", err)
	}

	enabled := 0
	if input.Enabled {
		enabled = 1
	}

	// 插入数据库
	_, err = a.db.GetConn().Exec(`
		INSERT INTO browser_extensions (
			extension_id, extension_name, extension_path, version, enabled,
			bound_profile_ids, source_type, source_url, description,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, extensionID, input.ExtensionName, input.ExtensionPath, input.Version, enabled,
		string(boundIDsJSON), input.SourceType, input.SourceURL, input.Description, now, now)

	if err != nil {
		return nil, fmt.Errorf("插入扩展失败: %w", err)
	}

	logger.New("Extension").Info("[Extension] 扩展已创建", logger.F("extension_id", extensionID), logger.F("extension_name", input.ExtensionName))

	return &BrowserExtension{
		ExtensionID:     extensionID,
		ExtensionName:   input.ExtensionName,
		ExtensionPath:   input.ExtensionPath,
		Version:         input.Version,
		Enabled:         input.Enabled,
		BoundProfileIDs: input.BoundProfileIDs,
		SourceType:      input.SourceType,
		SourceURL:       input.SourceURL,
		Description:     input.Description,
		CreatedAt:       now,
		UpdatedAt:       now,
	}, nil
}

// BrowserExtensionList 获取扩展列表
func (a *App) BrowserExtensionList() ([]BrowserExtension, error) {
	rows, err := a.db.GetConn().Query(`
		SELECT extension_id, extension_name, extension_path, version, enabled,
		       bound_profile_ids, source_type, source_url, description,
		       created_at, updated_at
		FROM browser_extensions
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("查询扩展列表失败: %w", err)
	}
	defer rows.Close()

	var extensions []BrowserExtension
	for rows.Next() {
		var ext BrowserExtension
		var enabled int
		var boundIDsJSON string

		err := rows.Scan(
			&ext.ExtensionID, &ext.ExtensionName, &ext.ExtensionPath,
			&ext.Version, &enabled, &boundIDsJSON, &ext.SourceType,
			&ext.SourceURL, &ext.Description, &ext.CreatedAt, &ext.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描扩展数据失败: %w", err)
		}

		ext.Enabled = enabled == 1

		// 反序列化绑定实例ID
		if err := json.Unmarshal([]byte(boundIDsJSON), &ext.BoundProfileIDs); err != nil {
			ext.BoundProfileIDs = []string{}
		}

		extensions = append(extensions, ext)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历扩展列表失败: %w", err)
	}

	logger.New("Extension").Info("[Extension] 扩展列表查询", logger.F("count", len(extensions)))
	return extensions, nil
}

// BrowserExtensionGet 获取单个扩展详情
func (a *App) BrowserExtensionGet(extensionID string) (*BrowserExtension, error) {
	var ext BrowserExtension
	var enabled int
	var boundIDsJSON string

	err := a.db.GetConn().QueryRow(`
		SELECT extension_id, extension_name, extension_path, version, enabled,
		       bound_profile_ids, source_type, source_url, description,
		       created_at, updated_at
		FROM browser_extensions
		WHERE extension_id = ?
	`, extensionID).Scan(
		&ext.ExtensionID, &ext.ExtensionName, &ext.ExtensionPath,
		&ext.Version, &enabled, &boundIDsJSON, &ext.SourceType,
		&ext.SourceURL, &ext.Description, &ext.CreatedAt, &ext.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("扩展不存在: %s", extensionID)
	}
	if err != nil {
		return nil, fmt.Errorf("查询扩展失败: %w", err)
	}

	ext.Enabled = enabled == 1

	// 反序列化绑定实例ID
	if err := json.Unmarshal([]byte(boundIDsJSON), &ext.BoundProfileIDs); err != nil {
		ext.BoundProfileIDs = []string{}
	}

	return &ext, nil
}

// BrowserExtensionUpdate 更新扩展
func (a *App) BrowserExtensionUpdate(extensionID string, input BrowserExtensionInput) (*BrowserExtension, error) {
	// 检查扩展是否存在
	var exists bool
	err := a.db.GetConn().QueryRow("SELECT 1 FROM browser_extensions WHERE extension_id = ?", extensionID).Scan(&exists)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("扩展不存在: %s", extensionID)
	}
	if err != nil {
		return nil, fmt.Errorf("检查扩展失败: %w", err)
	}

	// 本地扩展必须是含 manifest.json 的有效目录
	if isLocalExtensionSource(input.SourceType) {
		if res := validateExtensionDir(input.ExtensionPath); !res.Valid {
			return nil, fmt.Errorf("扩展目录无效：%s", res.Message)
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)

	// 序列化绑定实例ID
	boundIDsJSON, err := json.Marshal(input.BoundProfileIDs)
	if err != nil {
		return nil, fmt.Errorf("序列化绑定实例ID失败: %w", err)
	}

	enabled := 0
	if input.Enabled {
		enabled = 1
	}

	// 更新数据库
	_, err = a.db.GetConn().Exec(`
		UPDATE browser_extensions
		SET extension_name = ?, extension_path = ?, version = ?, enabled = ?,
		    bound_profile_ids = ?, source_type = ?, source_url = ?,
		    description = ?, updated_at = ?
		WHERE extension_id = ?
	`, input.ExtensionName, input.ExtensionPath, input.Version, enabled,
		string(boundIDsJSON), input.SourceType, input.SourceURL,
		input.Description, now, extensionID)

	if err != nil {
		return nil, fmt.Errorf("更新扩展失败: %w", err)
	}

	logger.New("Extension").Info("[Extension] 扩展已更新", logger.F("extension_id", extensionID), logger.F("extension_name", input.ExtensionName))

	return a.BrowserExtensionGet(extensionID)
}

// BrowserExtensionDelete 删除扩展
func (a *App) BrowserExtensionDelete(extensionID string) error {
	result, err := a.db.GetConn().Exec("DELETE FROM browser_extensions WHERE extension_id = ?", extensionID)
	if err != nil {
		return fmt.Errorf("删除扩展失败: %w", err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("扩展不存在: %s", extensionID)
	}

	logger.New("Extension").Info("[Extension] 扩展已删除", logger.F("extension_id", extensionID))
	return nil
}

// BrowserExtensionToggle 切换扩展启用状态
func (a *App) BrowserExtensionToggle(extensionID string, enabled bool) error {
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}

	now := time.Now().UTC().Format(time.RFC3339)

	result, err := a.db.GetConn().Exec(`
		UPDATE browser_extensions
		SET enabled = ?, updated_at = ?
		WHERE extension_id = ?
	`, enabledInt, now, extensionID)

	if err != nil {
		return fmt.Errorf("切换扩展状态失败: %w", err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("扩展不存在: %s", extensionID)
	}

	logger.New("Extension").Info("[Extension] 扩展状态已切换", logger.F("extension_id", extensionID), logger.F("enabled", enabled))
	return nil
}
