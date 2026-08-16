package browser

import (
	"ant-chrome/backend/internal/logger"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const profileTrashRetention = 72 * time.Hour

// Delete 将配置移入回收站
func (m *Manager) Delete(profileId string) error {
	log := logger.New("Browser")
	m.InitData()
	m.Mutex.Lock()
	defer m.Mutex.Unlock()

	profile, exists := m.Profiles[profileId]
	if !exists {
		log.Error("浏览器配置不存在", logger.F("profile_id", profileId))
		return fmt.Errorf("profile not found")
	}
	deletedAt := time.Now().Format(time.RFC3339)
	if m.ProfileDAO != nil {
		if err := m.ProfileDAO.SoftDelete(profileId, deletedAt); err != nil {
			log.Error("数据库移入回收站失败", logger.F("profile_id", profileId), logger.F("error", err))
			return err
		}
	} else {
		profile.DeletedAt = deletedAt
		profile.UpdatedAt = deletedAt
		delete(m.Profiles, profileId)
		if err := m.SaveProfiles(); err != nil {
			return err
		}
	}
	profile.DeletedAt = deletedAt
	profile.UpdatedAt = deletedAt
	delete(m.Profiles, profileId)
	resolvedDir := m.ResolveUserDataDir(profile)
	dataDirExists := pathExists(resolvedDir)
	m.writeProfileDeleteAudit(log, profileDeleteAuditEntry{
		Action:               "soft_delete",
		ProfileID:            profile.ProfileId,
		ProfileName:          profile.ProfileName,
		UserDataDir:          profile.UserDataDir,
		ResolvedDir:          resolvedDir,
		DeletedAt:            deletedAt,
		DataDirExistedBefore: dataDirExists,
		DataDirExistsAfter:   dataDirExists,
		Success:              true,
	})
	log.Info("浏览器配置移入回收站", logger.F("profile_id", profileId))

	return nil
}

// ListDeleted 获取回收站实例
func (m *Manager) ListDeleted() []Profile {
	log := logger.New("Browser")
	m.InitData()
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	m.cleanupExpiredTrashLocked(log)
	if m.ProfileDAO == nil {
		return []Profile{}
	}
	profiles, err := m.ProfileDAO.ListDeleted()
	if err != nil {
		log.Error("查询回收站实例失败", logger.F("error", err))
		return []Profile{}
	}
	list := make([]Profile, 0, len(profiles))
	for _, profile := range profiles {
		p := *profile
		if m.CodeProvider != nil {
			if code, err := m.CodeProvider.EnsureCode(p.ProfileId); err == nil {
				p.LaunchCode = code
			}
		}
		list = append(list, p)
	}
	return list
}

// Restore 从回收站恢复实例
func (m *Manager) Restore(profileId string) (*Profile, error) {
	log := logger.New("Browser")
	m.InitData()
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	if m.ProfileDAO == nil {
		return nil, fmt.Errorf("当前环境不支持回收站恢复")
	}
	profile, err := m.ProfileDAO.GetById(profileId)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(profile.DeletedAt) == "" {
		return nil, fmt.Errorf("实例不在回收站")
	}
	if err := m.ProfileDAO.Restore(profileId); err != nil {
		return nil, err
	}
	resolvedDir := m.ResolveUserDataDir(profile)
	dataDirExists := pathExists(resolvedDir)
	m.writeProfileDeleteAudit(log, profileDeleteAuditEntry{
		Action:               "restore",
		ProfileID:            profile.ProfileId,
		ProfileName:          profile.ProfileName,
		UserDataDir:          profile.UserDataDir,
		ResolvedDir:          resolvedDir,
		DeletedAt:            profile.DeletedAt,
		DataDirExistedBefore: dataDirExists,
		DataDirExistsAfter:   dataDirExists,
		Success:              true,
	})
	profile.DeletedAt = ""
	profile.UpdatedAt = time.Now().Format(time.RFC3339)
	profile.CoreId = normalizeProfileCoreID(profile.CoreId)
	m.Profiles[profile.ProfileId] = profile
	log.Info("实例已从回收站恢复", logger.F("profile_id", profileId))
	return profile, nil
}

// PermanentlyDeleteAll 清空回收站:彻底删除全部已软删除的实例。返回成功删除数量与首个错误。
func (m *Manager) PermanentlyDeleteAll() (int, error) {
	deleted := m.ListDeleted()
	count := 0
	var firstErr error
	for _, p := range deleted {
		if err := m.PermanentlyDelete(p.ProfileId); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		count++
	}
	return count, firstErr
}

// PermanentlyDelete 从回收站彻底删除实例及其关联数据
func (m *Manager) PermanentlyDelete(profileId string) error {
	log := logger.New("Browser")
	m.InitData()
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	if m.ProfileDAO == nil {
		return fmt.Errorf("当前环境不支持回收站物理删除")
	}
	profile, err := m.ProfileDAO.GetById(profileId)
	if err != nil {
		return err
	}
	if strings.TrimSpace(profile.DeletedAt) == "" {
		return fmt.Errorf("只能彻底删除回收站内的实例")
	}
	resolvedDir := m.ResolveUserDataDir(profile)
	dataDirExistedBefore := pathExists(resolvedDir)
	if err := m.deleteProfileRelatedDataLocked(log, profile); err != nil {
		m.writeProfileDeleteAudit(log, profileDeleteAuditEntry{
			Action:               "permanent_delete",
			ProfileID:            profile.ProfileId,
			ProfileName:          profile.ProfileName,
			UserDataDir:          profile.UserDataDir,
			ResolvedDir:          resolvedDir,
			DeletedAt:            profile.DeletedAt,
			DataDirExistedBefore: dataDirExistedBefore,
			DataDirExistsAfter:   pathExists(resolvedDir),
			Success:              false,
			Error:                err.Error(),
		})
		return err
	}
	if err := m.ProfileDAO.Delete(profileId); err != nil {
		m.writeProfileDeleteAudit(log, profileDeleteAuditEntry{
			Action:               "permanent_delete",
			ProfileID:            profile.ProfileId,
			ProfileName:          profile.ProfileName,
			UserDataDir:          profile.UserDataDir,
			ResolvedDir:          resolvedDir,
			DeletedAt:            profile.DeletedAt,
			DataDirExistedBefore: dataDirExistedBefore,
			DataDirExistsAfter:   pathExists(resolvedDir),
			Success:              false,
			Error:                err.Error(),
		})
		return err
	}
	m.writeProfileDeleteAudit(log, profileDeleteAuditEntry{
		Action:               "permanent_delete",
		ProfileID:            profile.ProfileId,
		ProfileName:          profile.ProfileName,
		UserDataDir:          profile.UserDataDir,
		ResolvedDir:          resolvedDir,
		DeletedAt:            profile.DeletedAt,
		DataDirExistedBefore: dataDirExistedBefore,
		DataDirExistsAfter:   pathExists(resolvedDir),
		Success:              true,
	})
	log.Info("回收站实例已彻底删除", logger.F("profile_id", profileId))
	return nil
}

// CleanupExpiredTrash 清理超过保留期的回收站实例
func (m *Manager) CleanupExpiredTrash() error {
	log := logger.New("Browser")
	m.InitData()
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	return m.cleanupExpiredTrashLocked(log)
}

func (m *Manager) cleanupExpiredTrashLocked(log *logger.Logger) error {
	if m.ProfileDAO == nil {
		return nil
	}
	expiredBefore := time.Now().Add(-profileTrashRetention).Format(time.RFC3339)
	expired, err := m.ProfileDAO.ListExpiredDeleted(expiredBefore)
	if err != nil {
		log.Error("清理过期回收站实例失败", logger.F("error", err))
		return err
	}
	cleaned := 0
	for _, profile := range expired {
		resolvedDir := m.ResolveUserDataDir(profile)
		dataDirExistedBefore := pathExists(resolvedDir)
		if err := m.deleteProfileRelatedDataLocked(log, profile); err != nil {
			log.Error("清理过期回收站实例关联数据失败", logger.F("profile_id", profile.ProfileId), logger.F("error", err))
			m.writeProfileDeleteAudit(log, profileDeleteAuditEntry{
				Action:               "expired_cleanup",
				ProfileID:            profile.ProfileId,
				ProfileName:          profile.ProfileName,
				UserDataDir:          profile.UserDataDir,
				ResolvedDir:          resolvedDir,
				DeletedAt:            profile.DeletedAt,
				DataDirExistedBefore: dataDirExistedBefore,
				DataDirExistsAfter:   pathExists(resolvedDir),
				Success:              false,
				Error:                err.Error(),
			})
			continue
		}
		if err := m.ProfileDAO.Delete(profile.ProfileId); err != nil {
			log.Error("删除过期回收站实例记录失败", logger.F("profile_id", profile.ProfileId), logger.F("error", err))
			m.writeProfileDeleteAudit(log, profileDeleteAuditEntry{
				Action:               "expired_cleanup",
				ProfileID:            profile.ProfileId,
				ProfileName:          profile.ProfileName,
				UserDataDir:          profile.UserDataDir,
				ResolvedDir:          resolvedDir,
				DeletedAt:            profile.DeletedAt,
				DataDirExistedBefore: dataDirExistedBefore,
				DataDirExistsAfter:   pathExists(resolvedDir),
				Success:              false,
				Error:                err.Error(),
			})
			continue
		}
		m.writeProfileDeleteAudit(log, profileDeleteAuditEntry{
			Action:               "expired_cleanup",
			ProfileID:            profile.ProfileId,
			ProfileName:          profile.ProfileName,
			UserDataDir:          profile.UserDataDir,
			ResolvedDir:          resolvedDir,
			DeletedAt:            profile.DeletedAt,
			DataDirExistedBefore: dataDirExistedBefore,
			DataDirExistsAfter:   pathExists(resolvedDir),
			Success:              true,
		})
		cleaned++
	}
	if cleaned > 0 {
		log.Info("过期回收站实例已清理", logger.F("count", cleaned))
	}
	return nil
}

func (m *Manager) deleteProfileRelatedDataLocked(log *logger.Logger, profile *Profile) error {
	if profile == nil {
		return nil
	}
	var firstErr error
	if m.CodeProvider != nil {
		if err := m.CodeProvider.Remove(profile.ProfileId); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if m.ExtensionDAO != nil {
		if err := m.ExtensionDAO.DeleteProfileSettings(profile.ProfileId); err != nil {
			log.Error("删除实例插件配置失败", logger.F("profile_id", profile.ProfileId), logger.F("error", err))
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	userDataDir := m.ResolveUserDataDir(profile)
	if err := m.deleteProfileUserDataDir(userDataDir); err != nil {
		log.Error("删除实例数据目录失败", logger.F("profile_id", profile.ProfileId), logger.F("dir", userDataDir), logger.F("error", err))
		if firstErr == nil {
			firstErr = err
		}
	}
	if err := m.deleteProfileSnapshotDir(profile.ProfileId); err != nil {
		log.Error("删除实例快照目录失败", logger.F("profile_id", profile.ProfileId), logger.F("error", err))
		if firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (m *Manager) deleteProfileSnapshotDir(profileId string) error {
	profileId = strings.TrimSpace(profileId)
	if profileId == "" {
		return nil
	}
	dataRoot, err := filepath.Abs(m.ResolveRelativePath("data"))
	if err != nil {
		return fmt.Errorf("解析数据根目录失败: %w", err)
	}
	snapshotRoot := filepath.Join(dataRoot, "snapshots")
	target, err := filepath.Abs(filepath.Join(snapshotRoot, profileId))
	if err != nil {
		return fmt.Errorf("解析快照目录失败: %w", err)
	}
	dataRoot = filepath.Clean(dataRoot)
	snapshotRoot = filepath.Clean(snapshotRoot)
	target = filepath.Clean(target)
	if samePath(target, snapshotRoot) || samePath(target, dataRoot) || !isPathInside(target, snapshotRoot) {
		return nil
	}
	if err := os.RemoveAll(target); err != nil {
		return fmt.Errorf("删除快照目录失败: %w", err)
	}
	return nil
}

func (m *Manager) deleteProfileUserDataDir(userDataDir string) error {
	userDataDir = strings.TrimSpace(userDataDir)
	if userDataDir == "" {
		return nil
	}
	target, err := filepath.Abs(userDataDir)
	if err != nil {
		return fmt.Errorf("解析实例数据目录失败: %w", err)
	}
	root := strings.TrimSpace(m.Config.Browser.UserDataRoot)
	if root == "" {
		root = "data"
	}
	rootAbs, err := filepath.Abs(m.ResolveRelativePath(root))
	if err != nil {
		return fmt.Errorf("解析用户数据根目录失败: %w", err)
	}
	target = filepath.Clean(target)
	rootAbs = filepath.Clean(rootAbs)
	if samePath(target, rootAbs) || !isPathInside(target, rootAbs) {
		return nil
	}
	if err := os.RemoveAll(target); err != nil {
		return fmt.Errorf("删除实例数据目录失败: %w", err)
	}
	return nil
}

func samePath(a string, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

func isPathInside(path string, parent string) bool {
	rel, err := filepath.Rel(parent, path)
	if err != nil || rel == "." || rel == "" {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
