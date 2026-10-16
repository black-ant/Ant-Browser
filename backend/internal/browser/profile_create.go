package browser

import (
	"ant-chrome/backend/internal/logger"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Create 创建配置
func (m *Manager) Create(input ProfileInput) (*Profile, error) {
	log := logger.New("Browser")
	m.InitData()
	m.Mutex.Lock()
	defer m.Mutex.Unlock()

	now := time.Now().Format(time.RFC3339)
	profileId := uuid.NewString()
	userDataDir := strings.TrimSpace(input.UserDataDir)
	if userDataDir == "" {
		userDataDir = profileId
	}
	resolvedProxy, err := m.resolveProfileProxyInput(input.ProxyId, input.ProxyConfig)
	if err != nil {
		log.Error("代理绑定失败", logger.F("profile_id", profileId), logger.F("proxy_id", strings.TrimSpace(input.ProxyId)), logger.F("error", err.Error()))
		return nil, err
	}
	coreId := normalizeProfileCoreID(input.CoreId)
	if coreId == "" {
		if defaultCore, ok := m.GetDefaultCore(); ok {
			coreId = defaultCore.CoreId
		}
	}
	fingerprintArgs := append([]string{}, input.FingerprintArgs...)
	if len(fingerprintArgs) == 0 && m.Config != nil {
		fingerprintArgs = m.defaultFingerprintArgsForCore(coreId)
	}
	profile := &Profile{
		ProfileId:          profileId,
		ProfileName:        input.ProfileName,
		UserDataDir:        userDataDir,
		CoreId:             coreId,
		RestoreLastSession: NormalizeRestoreLastSessionMode(input.RestoreLastSession),
		FingerprintArgs:    fingerprintArgs,
		ProxyId:            resolvedProxy.ProxyId,
		ProxyConfig:        resolvedProxy.ProxyConfig,
		MemoryLimitMB:      normalizeMemoryLimitMB(input.MemoryLimitMB),
		LaunchArgs:         input.LaunchArgs,
		Tags:               input.Tags,
		Keywords:           append([]string{}, input.Keywords...),
		GroupId:            strings.TrimSpace(input.GroupId),
		Running:            false,
		DebugPort:          0,
		Pid:                0,
		LastError:          "",
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if resolvedProxy.HasSelectedProxy {
		_ = BindProfileToProxy(profile, resolvedProxy.SelectedProxy, true)
	} else if resolvedProxy.FallbackToDirect {
		_ = m.bindProfileToDirectProxy(profile)
	}
	if resolvedProxy.UsedConfigFallback {
		log.Warn("代理ID未命中，已改为使用输入的代理配置",
			logger.F("profile_id", profileId),
			logger.F("proxy_id", strings.TrimSpace(input.ProxyId)),
		)
	}
	m.Profiles[profileId] = profile
	log.Info("浏览器配置创建", logger.F("profile_id", profileId), logger.F("profile_name", input.ProfileName))
	if err := m.SaveProfiles(); err != nil {
		return nil, err
	}
	m.ensureProfileLaunchCode(profile)
	return profile, nil
}

func (m *Manager) ensureProfileLaunchCode(profile *Profile) {
	if m.CodeProvider == nil || profile == nil {
		return
	}
	if code, err := m.CodeProvider.EnsureCode(profile.ProfileId); err == nil {
		profile.LaunchCode = code
	}
}

func buildProfileGroupID(value string) string {
	return strings.TrimSpace(value)
}
