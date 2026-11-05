package browser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"ant-chrome/backend/internal/logger"
)

const profileDeleteAuditPath = "data/logs/profile-delete-audit.jsonl"

type profileDeleteAuditEntry struct {
	At                   string `json:"at"`
	Action               string `json:"action"`
	ProfileID            string `json:"profileId"`
	ProfileName          string `json:"profileName"`
	UserDataDir          string `json:"userDataDir"`
	ResolvedDir          string `json:"resolvedDir"`
	DeletedAt            string `json:"deletedAt,omitempty"`
	DataDirExistedBefore bool   `json:"dataDirExistedBefore"`
	DataDirExistsAfter   bool   `json:"dataDirExistsAfter"`
	Success              bool   `json:"success"`
	Error                string `json:"error,omitempty"`
}

func (m *Manager) writeProfileDeleteAudit(log *logger.Logger, entry profileDeleteAuditEntry) {
	if entry.At == "" {
		entry.At = time.Now().Format(time.RFC3339)
	}
	if entry.ResolvedDir != "" && !entry.DataDirExistedBefore {
		entry.DataDirExistedBefore = pathExists(entry.ResolvedDir)
	}
	data, err := json.Marshal(entry)
	if err != nil {
		if log != nil {
			log.Error("生成实例删除审计日志失败", logger.F("profile_id", entry.ProfileID), logger.F("error", err))
		}
		return
	}
	auditPath := m.ResolveRelativePath(profileDeleteAuditPath)
	if err := os.MkdirAll(filepath.Dir(auditPath), 0755); err != nil {
		if log != nil {
			log.Error("创建实例删除审计日志目录失败", logger.F("path", auditPath), logger.F("error", err))
		}
		return
	}
	file, err := os.OpenFile(auditPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		if log != nil {
			log.Error("打开实例删除审计日志失败", logger.F("path", auditPath), logger.F("error", err))
		}
		return
	}
	defer file.Close()
	if _, err := file.Write(append(data, '\n')); err != nil && log != nil {
		log.Error("写入实例删除审计日志失败", logger.F("path", auditPath), logger.F("error", err))
	}
}

func pathExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}
