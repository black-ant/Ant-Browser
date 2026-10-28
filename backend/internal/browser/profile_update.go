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
	resolvedProxy, err := m.resolveProfileProxyInput(input)
	if err != nil {
		log.Error("代理绑定失败", logger.F("profile_id", profileId), logger.F("proxy_id", strings.TrimSpace(input.ProxyId)), logger.F("error", err.Error()))
		return nil, err
	}

	profile.ProfileName = input.ProfileName
	profile.UserDataDir = input.UserDataDir
	profile.CoreId = normalizeProfileCoreID(input.CoreId)
	profile.RestoreLastSession = NormalizeRestoreLastSessionMode(input.RestoreLastSession)
	profile.FingerprintArgs = upgradeLegacyMinimalFingerprintArgs(input.FingerprintArgs)
	profile.ProxyMode = resolvedProxy.ProxyMode
	profile.ProxyGroupName = resolvedProxy.ProxyGroupName
	if resolvedProxy.HasSelectedProxy {
		_ = BindProfileToProxy(profile, resolvedProxy.SelectedProxy, true)
	} else if resolvedProxy.FallbackToDirect {
		// 空代理输入既不保留（用户以为改了）也不自动改直连（暴露真实 IP）：直接报错。
		// 调用方必须显式传入代理池节点 ID，或显式传入直连 ID。
		log.Error("更新实例时未提供代理绑定",
			logger.F("profile_id", profileId),
			logger.F("current_proxy_id", profile.ProxyId),
		)
		return nil, fmt.Errorf("实例未提供代理绑定：请显式选择代理池节点或直连后再保存（不支持清空代理）")
	} else {
		profile.ProxyId = resolvedProxy.ProxyId
		profile.ProxyConfig = resolvedProxy.ProxyConfig
		_ = ClearProfileProxyBinding(profile)
	}
	profile.MemoryLimitMB = normalizeMemoryLimitMB(input.MemoryLimitMB)
	profile.LaunchArgs = input.LaunchArgs
	profile.Tags = input.Tags
	profile.Keywords = append([]string{}, input.Keywords...)
	profile.GroupId = buildProfileGroupID(input.GroupId)
	profile.UpdatedAt = time.Now().Format(time.RFC3339)

	log.Info("浏览器配置更新", logger.F("profile_id", profileId), logger.F("profile_name", input.ProfileName))
	if err := m.SaveProfiles(); err != nil {
		return nil, err
	}
	return profile, nil
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
