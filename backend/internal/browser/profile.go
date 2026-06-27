package browser

import (
	"ant-chrome/backend/internal/logger"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

func normalizeProfileConfigJSON(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "{}"
	}
	if !json.Valid([]byte(raw)) {
		return "{}"
	}
	return raw
}

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
	if m.StartingProfiles == nil {
		m.StartingProfiles = make(map[string]bool)
	}
	// 执行配置迁移
	m.MigrateConfig()
	if len(m.Profiles) > 0 {
		return
	}
	m.loadProfiles()
}

func (m *Manager) loadProfiles() {
	log := logger.New("Browser")

	// 优先从 DAO（SQLite）加载
	if m.ProfileDAO != nil {
		profiles, err := m.ProfileDAO.List()
		if err != nil {
			log.Error("从数据库加载窗口配置失败", logger.F("error", err))
		} else {
			// SQLite 模式：无论是否为空都直接使用，不自动创建默认窗口
			for _, p := range profiles {
				p.CoreId = normalizeProfileCoreID(p.CoreId)
				m.Profiles[p.ProfileId] = p
			}
			if len(profiles) > 0 {
				log.Info("窗口配置从数据库加载完成", logger.F("count", len(profiles)))
			} else {
				log.Info("窗口表为空，用户可手动创建新窗口")
			}
			return
		}
	}

	// 降级：从 config.yaml 加载（仅在无 SQLite 时使用）
	if len(m.Config.Browser.Profiles) == 0 {
		// 不自动创建默认窗口，保持空列表
		log.Info("窗口配置为空，用户可手动创建新窗口")
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
		m.Profiles[profileId] = &Profile{
			ProfileId:          profileId,
			ProfileName:        item.ProfileName,
			UserDataDir:        item.UserDataDir,
			CoreId:             normalizeProfileCoreID(item.CoreId),
			FingerprintArgs:    append([]string{}, item.FingerprintArgs...),
			ProxyId:            item.ProxyId,
			ProxyConfig:        item.ProxyConfig,
			ProxyBindSourceID:  item.ProxyBindSourceID,
			ProxyBindSourceURL: item.ProxyBindSourceURL,
			ProxyBindName:      item.ProxyBindName,
			ProxyBindUpdatedAt: item.ProxyBindUpdatedAt,
			LaunchArgs:         append([]string{}, item.LaunchArgs...),
			ProfileConfig:      normalizeProfileConfigJSON(item.ProfileConfig),
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

// SaveProfiles 保存所有窗口配置（DAO 模式：逐条 upsert）
func (m *Manager) SaveProfiles() error {
	log := logger.New("Browser")
	if m.ProfileDAO != nil {
		for _, profile := range m.Profiles {
			profile.CoreId = normalizeProfileCoreID(profile.CoreId)
			if err := m.ProfileDAO.Upsert(profile); err != nil {
				log.Error("窗口配置持久化失败", logger.F("profile_id", profile.ProfileId), logger.F("error", err))
				return err
			}
		}
		log.Info("窗口配置持久化成功", logger.F("count", len(m.Profiles)))
		return nil
	}

	// 降级：写回 config.yaml
	profiles := make([]ProfileConfig, 0, len(m.Profiles))
	for _, profile := range m.Profiles {
		profiles = append(profiles, ProfileConfig{
			ProfileId:          profile.ProfileId,
			ProfileName:        profile.ProfileName,
			UserDataDir:        profile.UserDataDir,
			CoreId:             normalizeProfileCoreID(profile.CoreId),
			FingerprintArgs:    append([]string{}, profile.FingerprintArgs...),
			ProxyId:            profile.ProxyId,
			ProxyConfig:        profile.ProxyConfig,
			ProxyBindSourceID:  profile.ProxyBindSourceID,
			ProxyBindSourceURL: profile.ProxyBindSourceURL,
			ProxyBindName:      profile.ProxyBindName,
			ProxyBindUpdatedAt: profile.ProxyBindUpdatedAt,
			LaunchArgs:         append([]string{}, profile.LaunchArgs...),
			ProfileConfig:      normalizeProfileConfigJSON(profile.ProfileConfig),
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

// List 获取配置列表
func (m *Manager) List() []Profile {
	log := logger.New("Browser")
	m.InitData()
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	list := make([]Profile, 0, len(m.Profiles))
	for _, profile := range m.Profiles {
		p := *profile
		if m.CodeProvider != nil {
			if code, err := m.CodeProvider.EnsureCode(p.ProfileId); err == nil {
				p.LaunchCode = code
			}
		}
		list = append(list, p)
	}
	// 按 ProfileId 排序，保持稳定顺序
	sort.Slice(list, func(i, j int) bool {
		return list[i].ProfileId < list[j].ProfileId
	})
	log.Info("浏览器配置列表查询", logger.F("count", len(list)))
	return list
}

// ListByTag 按标签筛选配置列表
func (m *Manager) ListByTag(tag string) []Profile {
	tag = strings.TrimSpace(tag)
	all := m.List()
	if tag == "" {
		return all
	}
	result := make([]Profile, 0)
	for _, p := range all {
		for _, t := range p.Tags {
			if strings.EqualFold(t, tag) {
				result = append(result, p)
				break
			}
		}
	}
	return result
}

// GetAllTags 获取所有已使用的标签（去重排序）
func (m *Manager) GetAllTags() []string {
	m.InitData()
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	seen := make(map[string]struct{})
	for _, p := range m.Profiles {
		for _, t := range p.Tags {
			t = strings.TrimSpace(t)
			if t != "" {
				seen[t] = struct{}{}
			}
		}
	}
	tags := make([]string, 0, len(seen))
	for t := range seen {
		tags = append(tags, t)
	}
	sort.Strings(tags)
	return tags
}

// Create 创建配置
func (m *Manager) Create(input ProfileInput) (*Profile, error) {
	log := logger.New("Browser")
	m.InitData()
	m.Mutex.Lock()
	defer m.Mutex.Unlock()

	// Check Profile Limit
	if m.Config.App.MaxProfileLimit > 0 && len(m.Profiles) >= m.Config.App.MaxProfileLimit {
		return nil, fmt.Errorf("窗口数量已达上限 (%d个)，无法创建新的窗口。请兑换额度后重试！", m.Config.App.MaxProfileLimit)
	}

	now := time.Now().Format(time.RFC3339)
	profileId := uuid.NewString()
	userDataDir := strings.TrimSpace(input.UserDataDir)
	if userDataDir == "" {
		userDataDir = profileId
	}
	proxyConfig := strings.TrimSpace(input.ProxyConfig)
	proxyId := strings.TrimSpace(input.ProxyId)
	selectedProxy := Proxy{}
	hasSelectedProxy := false
	if proxyId != "" {
		if proxyItem, ok := m.GetProxyByID(proxyId); ok {
			proxyConfig = strings.TrimSpace(proxyItem.ProxyConfig)
			selectedProxy = proxyItem
			hasSelectedProxy = true
		} else {
			log.Error("代理绑定失败", logger.F("profile_id", profileId), logger.F("proxy_id", proxyId))
		}
	}
	coreId := normalizeProfileCoreID(input.CoreId)
	if coreId == "" {
		if defaultCore, ok := m.GetDefaultCore(); ok {
			coreId = defaultCore.CoreId
		}
	}
	if proxyConfig == "" && m.Config.Browser.DefaultProxy != "" {
		proxyConfig = m.Config.Browser.DefaultProxy
	}
	profile := &Profile{
		ProfileId:       profileId,
		ProfileName:     input.ProfileName,
		UserDataDir:     userDataDir,
		CoreId:          coreId,
		FingerprintArgs: input.FingerprintArgs,
		ProxyId:         proxyId,
		ProxyConfig:     proxyConfig,
		LaunchArgs:      input.LaunchArgs,
		ProfileConfig:   normalizeProfileConfigJSON(input.ProfileConfig),
		Tags:            input.Tags,
		Keywords:        append([]string{}, input.Keywords...),
		GroupId:         strings.TrimSpace(input.GroupId),
		Running:         false,
		DebugPort:       0,
		Pid:             0,
		LastError:       "",
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if hasSelectedProxy {
		_ = BindProfileToProxy(profile, selectedProxy, true)
	}
	m.Profiles[profileId] = profile
	log.Info("浏览器配置创建", logger.F("profile_id", profileId), logger.F("profile_name", input.ProfileName))
	if err := m.SaveProfiles(); err != nil {
		return nil, err
	}
	if m.CodeProvider != nil {
		if code, err := m.CodeProvider.EnsureCode(profile.ProfileId); err == nil {
			profile.LaunchCode = code
		}
	}
	return profile, nil
}

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
	profile.ProfileName = input.ProfileName
	profile.UserDataDir = input.UserDataDir
	profile.CoreId = normalizeProfileCoreID(input.CoreId)
	profile.FingerprintArgs = input.FingerprintArgs
	profile.ProxyId = strings.TrimSpace(input.ProxyId)
	if profile.ProxyId != "" {
		if proxyItem, ok := m.GetProxyByID(profile.ProxyId); ok {
			_ = BindProfileToProxy(profile, proxyItem, true)
		} else {
			log.Error("代理绑定失败", logger.F("profile_id", profileId), logger.F("proxy_id", profile.ProxyId))
		}
	} else {
		profile.ProxyConfig = input.ProxyConfig
		_ = ClearProfileProxyBinding(profile)
	}
	profile.LaunchArgs = input.LaunchArgs
	profile.ProfileConfig = normalizeProfileConfigJSON(input.ProfileConfig)
	profile.Tags = input.Tags
	profile.Keywords = append([]string{}, input.Keywords...)
	profile.GroupId = strings.TrimSpace(input.GroupId)
	profile.UpdatedAt = time.Now().Format(time.RFC3339)
	log.Info("浏览器配置更新", logger.F("profile_id", profileId), logger.F("profile_name", input.ProfileName))
	if err := m.SaveProfiles(); err != nil {
		return nil, err
	}
	return profile, nil
}

// Delete 删除配置
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

	// 防止删除运行中的窗口：保护用户数据和会话。
	// API 层 (profile_api.go:250) 有此检查，但 Wails 绑定层 (app.go:565) 直接调用本方法，
	// 因此必须在底层也加检查，形成纵深防御。
	if profile.Running {
		log.Error("运行中的窗口不可删除", logger.F("profile_id", profileId))
		return fmt.Errorf("cannot delete running profile")
	}

	delete(m.Profiles, profileId)
	log.Info("浏览器配置删除", logger.F("profile_id", profileId))

	// DAO 删除
	if m.ProfileDAO != nil {
		if err := m.ProfileDAO.Delete(profileId); err != nil {
			log.Error("数据库删除窗口失败", logger.F("profile_id", profileId), logger.F("error", err))
			return err
		}
	} else {
		if err := m.SaveProfiles(); err != nil {
			return err
		}
	}

	if m.CodeProvider != nil {
		_ = m.CodeProvider.Remove(profileId)
	}
	return nil
}

// ApplyDefaults 应用默认配置
func (m *Manager) ApplyDefaults(profile *Profile) bool {
	log := logger.New("Browser")
	if profile.FingerprintArgs == nil || len(profile.FingerprintArgs) == 0 {
		profile.FingerprintArgs = append([]string{}, m.Config.Browser.DefaultFingerprintArgs...)
	}
	if profile.LaunchArgs == nil || len(profile.LaunchArgs) == 0 {
		profile.LaunchArgs = append([]string{}, m.Config.Browser.DefaultLaunchArgs...)
	}
	profile.ProfileConfig = normalizeProfileConfigJSON(profile.ProfileConfig)
	if strings.TrimSpace(profile.UserDataDir) == "" {
		profile.UserDataDir = profile.ProfileId
	}
	profile.CoreId = normalizeProfileCoreID(profile.CoreId)
	if profile.CoreId == "" {
		if defaultCore, ok := m.GetDefaultCore(); ok {
			profile.CoreId = defaultCore.CoreId
		}
	}
	proxyChanged := false
	bindChanged, boundInPool, bindMode := m.ResolveProfileProxyBinding(profile)
	if bindChanged {
		proxyChanged = true
	}
	if bindMode != "" && bindMode != "proxy_id" {
		log.Info("窗口代理自动重关联",
			logger.F("profile_id", profile.ProfileId),
			logger.F("proxy_id", profile.ProxyId),
			logger.F("mode", bindMode),
		)
	}
	if profile.ProxyId != "" && !boundInPool {
		if strings.TrimSpace(profile.ProxyConfig) == "" {
			log.Error("窗口代理未找到", logger.F("profile_id", profile.ProfileId), logger.F("proxy_id", profile.ProxyId))
		} else {
			log.Warn("窗口代理未找到，回退使用历史代理配置", logger.F("profile_id", profile.ProfileId), logger.F("proxy_id", profile.ProxyId))
		}
	}
	if profile.ProxyConfig == "" && m.Config.Browser.DefaultProxy != "" {
		profile.ProxyConfig = m.Config.Browser.DefaultProxy
		proxyChanged = true
	}
	return proxyChanged
}

// Copy 复制窗口配置（除指纹参数外全部复制，指纹使用默认值生成新种子）
func (m *Manager) Copy(profileId string, newName string) (*Profile, error) {
	log := logger.New("Browser")
	m.InitData()
	m.Mutex.Lock()
	defer m.Mutex.Unlock()

	// Check Profile Limit
	if m.Config.App.MaxProfileLimit > 0 && len(m.Profiles) >= m.Config.App.MaxProfileLimit {
		log.Error("复制窗口失败: 达到数量上限", logger.F("limit", m.Config.App.MaxProfileLimit))
		return nil, fmt.Errorf("窗口数量已达上限 (%d个)，无法复制窗口。请兑换额度后重试！", m.Config.App.MaxProfileLimit)
	}

	src, exists := m.Profiles[profileId]
	if !exists {
		log.Error("源窗口不存在", logger.F("profile_id", profileId))
		return nil, fmt.Errorf("profile not found")
	}

	now := time.Now().Format(time.RFC3339)
	newId := uuid.NewString()

	// 处理名称
	profileName := strings.TrimSpace(newName)
	if profileName == "" {
		profileName = src.ProfileName + " (副本)"
	}

	// 复制配置，指纹参数使用默认值（新种子）
	profile := &Profile{
		ProfileId:          newId,
		ProfileName:        profileName,
		UserDataDir:        newId, // 新的用户数据目录
		CoreId:             normalizeProfileCoreID(src.CoreId),
		FingerprintArgs:    append([]string{}, m.Config.Browser.DefaultFingerprintArgs...), // 使用默认指纹（新种子）
		ProxyId:            src.ProxyId,
		ProxyConfig:        src.ProxyConfig,
		ProxyBindSourceID:  src.ProxyBindSourceID,
		ProxyBindSourceURL: src.ProxyBindSourceURL,
		ProxyBindName:      src.ProxyBindName,
		ProxyBindUpdatedAt: src.ProxyBindUpdatedAt,
		LaunchArgs:         append([]string{}, src.LaunchArgs...),
		ProfileConfig:      normalizeProfileConfigJSON(src.ProfileConfig),
		Tags:               append([]string{}, src.Tags...),
		Keywords:           append([]string{}, src.Keywords...),
		GroupId:            src.GroupId, // 复制分组
		Running:            false,
		DebugPort:          0,
		Pid:                0,
		LastError:          "",
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	m.Profiles[newId] = profile
	log.Info("窗口复制成功", logger.F("src_id", profileId), logger.F("new_id", newId), logger.F("new_name", profileName))

	if err := m.SaveProfiles(); err != nil {
		return nil, err
	}

	if m.CodeProvider != nil {
		if code, err := m.CodeProvider.EnsureCode(profile.ProfileId); err == nil {
			profile.LaunchCode = code
		}
	}

	return profile, nil
}

// SetKeywords 设置窗口关键字（独立接口，不影响其他字段）
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

// copyKeywords 深拷贝 keywords map
func copyKeywords(src map[string]string) map[string]string {
	if src == nil {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
