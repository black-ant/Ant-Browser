package browser

import (
	"ant-chrome/backend/internal/logger"
	"fmt"
	"strings"
	"time"
)

// Update 更新配置
func (m *Manager) Update(profileId string, input ProfileInput) (*Profile, error) {
	log := logger.New("Browser")
	m.InitData()
	m.Mutex.Lock()
	defer m.Mutex.Unlock()

	profile, exists := m.Profiles[profileId]
	if !exists {
		log.Error("浏览器配置不存在", logger.F("profile_id", profileId))
		return nil, fmt.Errorf("profile not found")
	}
	resolvedProxy, err := m.resolveProfileProxyInput(input.ProxyId, input.ProxyConfig)
	if err != nil {
		log.Error("代理绑定失败", logger.F("profile_id", profileId), logger.F("proxy_id", strings.TrimSpace(input.ProxyId)), logger.F("error", err.Error()))
		return nil, err
	}

	profile.ProfileName = input.ProfileName
	profile.UserDataDir = input.UserDataDir
	profile.CoreId = normalizeProfileCoreID(input.CoreId)
	profile.RestoreLastSession = NormalizeRestoreLastSessionMode(input.RestoreLastSession)
	profile.FingerprintArgs = input.FingerprintArgs
	if resolvedProxy.HasSelectedProxy {
		_ = BindProfileToProxy(profile, resolvedProxy.SelectedProxy, true)
	} else if resolvedProxy.FallbackToDirect {
		_ = m.bindProfileToDirectProxy(profile)
	} else {
		profile.ProxyId = resolvedProxy.ProxyId
		profile.ProxyConfig = resolvedProxy.ProxyConfig
		_ = ClearProfileProxyBinding(profile)
	}
	profile.MemoryLimitMB = normalizeMemoryLimitMB(input.MemoryLimitMB)
	if resolvedProxy.UsedConfigFallback {
		log.Warn("代理ID未命中，已改为使用输入的代理配置",
			logger.F("profile_id", profileId),
			logger.F("proxy_id", strings.TrimSpace(input.ProxyId)),
		)
	}
	profile.LaunchArgs = input.LaunchArgs
	profile.Tags = input.Tags
	profile.Keywords = append([]string{}, input.Keywords...)
	profile.GroupId = buildProfileGroupID(input.GroupId)
	if input.LiveKeepAliveEnabled != nil {
		profile.LiveKeepAliveEnabled = *input.LiveKeepAliveEnabled
	}
	if input.MuteAudio != nil {
		profile.MuteAudio = *input.MuteAudio
	}
	profile.UpdatedAt = time.Now().Format(time.RFC3339)

	log.Info("浏览器配置更新", logger.F("profile_id", profileId), logger.F("profile_name", input.ProfileName))
	if err := m.SaveProfiles(); err != nil {
		return nil, err
	}
	return profile, nil
}

// MoveInstancesToGroup 批量移动实例到分组:先更新数据库,再同步内存 m.Profiles。
// 必须同步内存——List()(以及 GUI 列表/分组筛选)读的是内存 m.Profiles,若只改 DB,
// 同一会话内按分组筛选会查不到数据(内存里的 group_id 仍为旧值),重启后才"恢复"。
func (m *Manager) MoveInstancesToGroup(profileIds []string, groupId string) error {
	dao, ok := m.ProfileDAO.(*SQLiteProfileDAO)
	if !ok {
		return fmt.Errorf("ProfileDAO 不支持批量移动")
	}
	if err := dao.MoveToGroup(profileIds, groupId); err != nil {
		return err
	}
	m.Mutex.Lock()
	for _, id := range profileIds {
		if p, ok := m.Profiles[id]; ok {
			p.GroupId = groupId
		}
	}
	m.Mutex.Unlock()
	return nil
}

// SetKeywords 设置实例关键字（独立接口，不影响其他字段）
func (m *Manager) SetKeywords(profileId string, keywords []string) (*Profile, error) {
	log := logger.New("Browser")
	m.InitData()
	m.Mutex.Lock()
	defer m.Mutex.Unlock()

	profile, exists := m.Profiles[profileId]
	if !exists {
		return nil, fmt.Errorf("profile not found")
	}
	profile.Keywords = append([]string{}, keywords...)
	profile.UpdatedAt = time.Now().Format(time.RFC3339)

	log.Info("关键字更新", logger.F("profile_id", profileId))
	if err := m.SaveProfiles(); err != nil {
		return nil, err
	}
	return profile, nil
}
