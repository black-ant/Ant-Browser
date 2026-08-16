package browser

import (
	"ant-chrome/backend/internal/logger"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Create 创建配置
func (m *Manager) Create(input ProfileInput) (*Profile, error) {
	m.InitData()
	m.Mutex.Lock()
	defer m.Mutex.Unlock()

	profile, err := m.createProfileLocked(input)
	if err != nil {
		return nil, err
	}
	if err := m.SaveProfiles(); err != nil {
		return nil, err
	}
	m.ensureProfileLaunchCode(profile)
	return profile, nil
}

// createProfileLocked 构建并登记一个新环境(含唯一自洽身份),但不做持久化、不分配启动码。
// 调用方必须已持有 m.Mutex,并在合适时机自行调用 SaveProfiles / ensureProfileLaunchCode。
// 供 Create(单个:建后即存)与 CreateBatch(批量:循环建、末尾统一 SaveProfiles)复用,
// 避免批量时对每个环境都全量重写(SaveProfiles 会 upsert 全部 profile,N 次即 O(N²))。
func (m *Manager) createProfileLocked(input ProfileInput) (*Profile, error) {
	log := logger.New("Browser")

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
		fingerprintArgs = append([]string{}, m.Config.Browser.DefaultFingerprintArgs...)
	}
	profile := &Profile{
		ProfileId:            profileId,
		ProfileName:          input.ProfileName,
		UserDataDir:          userDataDir,
		CoreId:               coreId,
		RestoreLastSession:   NormalizeRestoreLastSessionMode(input.RestoreLastSession),
		FingerprintArgs:      fingerprintArgs,
		ProxyId:              resolvedProxy.ProxyId,
		ProxyConfig:          resolvedProxy.ProxyConfig,
		MemoryLimitMB:        normalizeMemoryLimitMB(input.MemoryLimitMB),
		LaunchArgs:           input.LaunchArgs,
		Tags:                 input.Tags,
		Keywords:             append([]string{}, input.Keywords...),
		GroupId:              strings.TrimSpace(input.GroupId),
		LiveKeepAliveEnabled: input.LiveKeepAliveEnabled == nil || *input.LiveKeepAliveEnabled,
		MuteAudio:            input.MuteAudio == nil || *input.MuteAudio,
		Running:              false,
		DebugPort:            0,
		Pid:                  0,
		LastError:            "",
		CreatedAt:            now,
		UpdatedAt:            now,
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
	if m.IdentityService != nil && !hasExplicitFingerprintSeed(input.FingerprintArgs) {
		// 新建环境:只要调用方没有显式指定 --fingerprint=<seed>(GUI 新建仅提交无种子的
		// 静态默认模板参数,如 --fingerprint-brand / --fingerprint-platform),就强制从真机池
		// 采样一套全新的唯一自洽身份,保证“每创建一个环境都是独立不重复且自洽”的核心要求。
		// 不能走反解分支,否则会把静态默认 flag 当作“老环境”反解成 seed=0 的身份。
		// 显式带 seed 的调用(Launch API 指定指纹 / 复现场景)则被尊重,不覆盖。
		if err := m.IdentityService.RegenerateForPlatform(profile, input.IdentityPlatform); err != nil {
			if strings.TrimSpace(input.IdentityPlatform) != "" {
				// 指定了平台却生成失败(如该平台池为空):必须硬失败——不能回退到静态默认指纹,
				// 否则会既不唯一(未入去重库)又平台不符(回退成宿主 windows)。
				return nil, fmt.Errorf("生成 %s 平台唯一身份失败: %w", input.IdentityPlatform, err)
			}
			log.Warn("自动生成自洽指纹身份失败，回退默认指纹", logger.F("profile_id", profileId), logger.F("error", err.Error()))
		} else {
			// 直连(无代理)实例:出口即真机本地 IP,把人设对齐到本地国家(默认 CN),
			// 避免“本地 IP + 境外时区/语言”被平台风控判废(如抖音过一会掉登录)。
			m.alignDirectProfileToLocalGeo(profile)
		}
	}
	m.Profiles[profileId] = profile
	log.Info("浏览器配置创建", logger.F("profile_id", profileId), logger.F("profile_name", input.ProfileName))
	return profile, nil
}

// hasExplicitFingerprintSeed 判断调用方是否显式提供了具体指纹种子(--fingerprint=<非零数字>)。
// 注意与 --fingerprint-brand= / --fingerprint-platform= 等模板 flag 区分:它们前缀是
// "--fingerprint-",不会被 "--fingerprint=" 命中。GUI 新建只提交这类无种子模板,应视为
// “未指定”从而触发唯一身份生成;而显式带 seed 的调用应被尊重。
func hasExplicitFingerprintSeed(args []string) bool {
	for _, a := range args {
		v := strings.TrimSpace(a)
		if !strings.HasPrefix(v, "--fingerprint=") {
			continue
		}
		raw := strings.TrimSpace(strings.TrimPrefix(v, "--fingerprint="))
		if n, err := strconv.ParseInt(raw, 10, 64); err == nil && n != 0 {
			return true
		}
	}
	return false
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
