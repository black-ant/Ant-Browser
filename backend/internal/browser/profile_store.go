package browser

import (
	"ant-chrome/backend/internal/logger"
	"os/exec"
	"strings"
	"time"
)

// InitData 初始化浏览器数据
func (m *Manager) InitData() {
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	if m.Profiles == nil {
		m.Profiles = make(map[string]*Profile)
	}
	if m.BrowserProcesses == nil {
		m.BrowserProcesses = make(map[string]*exec.Cmd)
	}
	if m.XrayBridges == nil {
		m.XrayBridges = make(map[string]*XrayBridge)
	}
	m.MigrateConfig()
	if len(m.Profiles) > 0 {
		return
	}
	m.loadProfiles()
}

func (m *Manager) loadProfiles() {
	log := logger.New("Browser")

	if m.ProfileDAO != nil {
		profiles, err := m.ProfileDAO.List()
		if err != nil {
			log.Error("从数据库加载实例配置失败", logger.F("error", err))
		} else {
			for _, p := range profiles {
				p.CoreId = normalizeProfileCoreID(p.CoreId)
				p.FingerprintArgs = m.upgradeLegacyMinimalFingerprintArgsForProfile(p.CoreId, p.FingerprintArgs)
				m.Profiles[p.ProfileId] = p
			}
			if len(profiles) > 0 {
				log.Info("实例配置从数据库加载完成", logger.F("count", len(profiles)))
			} else {
				log.Info("实例表为空，用户可手动创建新实例")
			}
			return
		}
	}

	if len(m.Config.Browser.Profiles) == 0 {
		log.Info("实例配置为空，用户可手动创建新实例")
		return
	}
	now := time.Now().Format(time.RFC3339)
	for _, item := range m.Config.Browser.Profiles {
		profileId := strings.TrimSpace(item.ProfileId)
		if profileId == "" {
			continue
		}
		createdAt := strings.TrimSpace(item.CreatedAt)
		if createdAt == "" {
			createdAt = now
		}
		updatedAt := strings.TrimSpace(item.UpdatedAt)
		if updatedAt == "" {
			updatedAt = createdAt
		}
		profileCoreId := normalizeProfileCoreID(item.CoreId)
		m.Profiles[profileId] = &Profile{
			ProfileId:          profileId,
			ProfileName:        item.ProfileName,
			UserDataDir:        item.UserDataDir,
			CoreId:             profileCoreId,
			RestoreLastSession: NormalizeRestoreLastSessionMode(item.RestoreLastSession),
			FingerprintArgs:    m.upgradeLegacyMinimalFingerprintArgsForProfile(profileCoreId, item.FingerprintArgs),
			ProxyId:            item.ProxyId,
			ProxyConfig:        item.ProxyConfig,
			ProxyBindSourceID:  item.ProxyBindSourceID,
			ProxyBindSourceURL: item.ProxyBindSourceURL,
			ProxyBindName:      item.ProxyBindName,
			ProxyBindUpdatedAt: item.ProxyBindUpdatedAt,
			MemoryLimitMB:      normalizeMemoryLimitMB(item.MemoryLimitMB),
			LaunchArgs:         append([]string{}, item.LaunchArgs...),
			Tags:               append([]string{}, item.Tags...),
			Keywords:           append([]string{}, item.Keywords...),
			Running:            false,
			DebugPort:          0,
			Pid:                0,
			LastError:          "",
			CreatedAt:          createdAt,
			UpdatedAt:          updatedAt,
		}
	}
	log.Info("浏览器配置从文件加载完成", logger.F("count", len(m.Profiles)))
}

// SaveProfiles 保存所有实例配置（DAO 模式：逐条 upsert）
func (m *Manager) SaveProfiles() error {
	log := logger.New("Browser")
	if m.ProfileDAO != nil {
		for _, profile := range m.Profiles {
			profile.CoreId = normalizeProfileCoreID(profile.CoreId)
			if err := m.ProfileDAO.Upsert(profile); err != nil {
				log.Error("实例配置持久化失败", logger.F("profile_id", profile.ProfileId), logger.F("error", err))
				return err
			}
		}
		log.Info("实例配置持久化成功", logger.F("count", len(m.Profiles)))
		return nil
	}

	profiles := make([]ProfileConfig, 0, len(m.Profiles))
	for _, profile := range m.Profiles {
		profiles = append(profiles, ProfileConfig{
			ProfileId:          profile.ProfileId,
			ProfileName:        profile.ProfileName,
			UserDataDir:        profile.UserDataDir,
			CoreId:             normalizeProfileCoreID(profile.CoreId),
			RestoreLastSession: NormalizeRestoreLastSessionMode(profile.RestoreLastSession),
			FingerprintArgs:    append([]string{}, profile.FingerprintArgs...),
			ProxyId:            profile.ProxyId,
			ProxyConfig:        profile.ProxyConfig,
			ProxyBindSourceID:  profile.ProxyBindSourceID,
			ProxyBindSourceURL: profile.ProxyBindSourceURL,
			ProxyBindName:      profile.ProxyBindName,
			ProxyBindUpdatedAt: profile.ProxyBindUpdatedAt,
			MemoryLimitMB:      normalizeMemoryLimitMB(profile.MemoryLimitMB),
			LaunchArgs:         append([]string{}, profile.LaunchArgs...),
			Tags:               append([]string{}, profile.Tags...),
			Keywords:           append([]string{}, profile.Keywords...),
			CreatedAt:          profile.CreatedAt,
			UpdatedAt:          profile.UpdatedAt,
		})
	}
	m.Config.Browser.Profiles = profiles
	if err := m.Config.Save(m.ResolveRelativePath("config.yaml")); err != nil {
		log.Error("浏览器配置持久化失败", logger.F("error", err))
		return err
	}
	log.Info("浏览器配置持久化成功（文件）", logger.F("count", len(profiles)))
	return nil
}
