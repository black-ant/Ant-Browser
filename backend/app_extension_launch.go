package backend

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ant-chrome/backend/internal/logger"
)

// ExtensionValidateResult 扩展目录校验结果
type ExtensionValidateResult struct {
	Valid   bool   `json:"valid"`
	Message string `json:"message"`
	Name    string `json:"name"`
	Version string `json:"version"`
}

// validateExtensionDir 校验路径是否为有效的解压版扩展目录（含 manifest.json），
// 并尽力从 manifest 解析 name/version。
func validateExtensionDir(path string) ExtensionValidateResult {
	p := strings.TrimSpace(path)
	if p == "" {
		return ExtensionValidateResult{Valid: false, Message: "路径为空"}
	}
	info, err := os.Stat(p)
	if err != nil {
		return ExtensionValidateResult{Valid: false, Message: "路径不存在或无法访问"}
	}
	if !info.IsDir() {
		return ExtensionValidateResult{Valid: false, Message: "路径不是目录（需指向解压后的扩展文件夹）"}
	}
	data, err := os.ReadFile(filepath.Join(p, "manifest.json"))
	if err != nil {
		return ExtensionValidateResult{Valid: false, Message: "目录下未找到 manifest.json"}
	}
	res := ExtensionValidateResult{Valid: true, Message: "有效扩展目录"}
	var manifest struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if json.Unmarshal(data, &manifest) == nil {
		res.Name = strings.TrimSpace(manifest.Name)
		res.Version = strings.TrimSpace(manifest.Version)
	}
	return res
}

// BrowserExtensionValidatePath 供前端实时校验扩展目录并回填 name/version
func (a *App) BrowserExtensionValidatePath(path string) ExtensionValidateResult {
	return validateExtensionDir(path)
}

// isLocalExtensionSource 仅本地（解压）扩展可经 --load-extension 加载；store/builtin 仅作引用。
func isLocalExtensionSource(sourceType string) bool {
	s := strings.ToLower(strings.TrimSpace(sourceType))
	return s == "" || s == "local"
}

// extensionLoadPathsForProfile 返回某窗口启动时应加载的扩展目录：
// 启用 + 本地来源 + 已绑定该窗口 + 目录含 manifest.json。
func (a *App) extensionLoadPathsForProfile(profileID string) []string {
	if a.db == nil || strings.TrimSpace(profileID) == "" {
		return nil
	}
	exts, err := a.BrowserExtensionList()
	if err != nil {
		return nil
	}
	log := logger.New("Extension")
	var paths []string
	for _, ext := range exts {
		if !ext.Enabled || !isLocalExtensionSource(ext.SourceType) {
			continue
		}
		bound := false
		for _, id := range ext.BoundProfileIDs {
			if id == profileID {
				bound = true
				break
			}
		}
		if !bound {
			continue
		}
		if res := validateExtensionDir(ext.ExtensionPath); !res.Valid {
			log.Warn("启动时跳过无效扩展目录",
				logger.F("extension_id", ext.ExtensionID),
				logger.F("path", ext.ExtensionPath),
				logger.F("reason", res.Message))
			continue
		}
		paths = append(paths, ext.ExtensionPath)
	}
	return paths
}

// BrowserExtensionSetProfiles 设置扩展绑定的窗口集合（过滤无效/重复 id）
func (a *App) BrowserExtensionSetProfiles(extensionID string, profileIDs []string) error {
	valid := map[string]bool{}
	a.browserMgr.Mutex.Lock()
	for id := range a.browserMgr.Profiles {
		valid[id] = true
	}
	a.browserMgr.Mutex.Unlock()

	filtered := make([]string, 0, len(profileIDs))
	seen := map[string]bool{}
	for _, id := range profileIDs {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] || !valid[id] {
			continue
		}
		seen[id] = true
		filtered = append(filtered, id)
	}
	data, _ := json.Marshal(filtered)
	res, err := a.db.GetConn().Exec(
		`UPDATE browser_extensions SET bound_profile_ids=?, updated_at=? WHERE extension_id=?`,
		string(data), time.Now().UTC().Format(time.RFC3339), extensionID)
	if err != nil {
		return fmt.Errorf("更新扩展绑定失败: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("扩展不存在: %s", extensionID)
	}
	return nil
}
