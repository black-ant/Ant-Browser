package browser

import (
	"database/sql"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"ant-chrome/backend/internal/identity"
)

// IdentityService 把指纹自洽引擎(池采样 + 唯一性登记 + 序列化)封装成实例管理器可用的服务。
// 在创建环境时为其生成唯一自洽的结构化身份并落库,同时把身份序列化为 fingerprint_args。
type IdentityService struct {
	poolStore *identity.PoolStore // 运行时可编辑的身份池(overlay + 内嵌回退)
	store     *identity.SQLiteStore
	resolver  identity.GeoResolver // 可选;接入离线 GeoIP mmdb 后用于代理地理对齐
	mu        sync.Mutex
	rng       *rand.Rand
}

// NewIdentityService 用数据库连接创建服务;poolOverlayPath 为可写身份池文件路径
// (空字符串=纯内存内嵌池,不落盘,用于测试)。
func NewIdentityService(db *sql.DB, poolOverlayPath string) (*IdentityService, error) {
	ps, err := identity.NewPoolStore(poolOverlayPath)
	if err != nil {
		return nil, err
	}
	return &IdentityService{
		poolStore: ps,
		store:     identity.NewSQLiteStore(db),
		rng:       rand.New(rand.NewSource(time.Now().UnixNano())),
	}, nil
}

// SetGeoResolver 注入离线地理解析器,启用代理地理对齐(mmdb 就绪后调用)。
func (s *IdentityService) SetGeoResolver(r identity.GeoResolver) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resolver = r
}

// IdentityForProfile 返回该 profile 的结构化身份:优先读库,其次从 fingerprint_args 反解。
func (s *IdentityService) IdentityForProfile(profileID string, args []string) (identity.Identity, bool) {
	if id, ok, err := s.store.Load(profileID); err == nil && ok {
		return id, true
	}
	if len(args) > 0 {
		derived := identity.FromLaunchArgs(args)
		if derived.FingerprintHash() != "" {
			return derived, true
		}
	}
	return identity.Identity{}, false
}

// GenerateUnique 采样并重采,产出一套跨环境唯一的基础身份(尚未对齐代理地理)。
func (s *IdentityService) GenerateUnique() (identity.Identity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	pool := s.poolStore.Pool() // 采样当前(可能已被编辑的)池快照
	return identity.GenerateUnique(s.store, func() identity.Identity {
		return pool.NewIdentity(s.rng)
	}, 100)
}

// GenerateUniqueForPlatform 与 GenerateUnique 相同,但仅从指定平台的真机模板采样。
// platform 为空则等价 GenerateUnique(全平台)。目标平台无模板时返回错误。
func (s *IdentityService) GenerateUniqueForPlatform(platform string) (identity.Identity, error) {
	platform = strings.TrimSpace(platform)
	if platform == "" {
		return s.GenerateUnique()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	pool := s.poolStore.Pool().Filter(func(r identity.PoolRecord) bool { return r.Platform == platform })
	if pool.Len() == 0 {
		return identity.Identity{}, fmt.Errorf("身份池中没有平台 %q 的模板", platform)
	}
	return identity.GenerateUnique(s.store, func() identity.Identity {
		return pool.NewIdentity(s.rng)
	}, 100)
}

// —— 身份池(模板)管理:编辑只影响之后新建的环境,不改已有环境 ——

// PoolRecords 返回身份池全部记录(副本)。
func (s *IdentityService) PoolRecords() []identity.PoolRecord { return s.poolStore.Records() }

// PoolCount 返回身份池记录数。
func (s *IdentityService) PoolCount() int { return s.poolStore.Count() }

// AddPoolRecord 新增一条身份池模板(自动赋 ID)。
func (s *IdentityService) AddPoolRecord(rec identity.PoolRecord) (identity.PoolRecord, error) {
	return s.poolStore.Add(rec)
}

// UpdatePoolRecord 按 ID 更新一条身份池模板。
func (s *IdentityService) UpdatePoolRecord(id string, rec identity.PoolRecord) (identity.PoolRecord, error) {
	return s.poolStore.Update(id, rec)
}

// DeletePoolRecord 按 ID 删除一条身份池模板。
func (s *IdentityService) DeletePoolRecord(id string) error { return s.poolStore.Delete(id) }

// RestorePoolDefaults 把身份池恢复为内嵌默认。
func (s *IdentityService) RestorePoolDefaults() error { return s.poolStore.RestoreDefaults() }

// ValidatePoolRecord 校验一条身份池模板的自洽性。
func (s *IdentityService) ValidatePoolRecord(rec identity.PoolRecord) identity.ValidationResult {
	return identity.ValidatePoolRecord(rec)
}

// Regenerate 全平台重生成(等价 RegenerateForPlatform(profile, ""))。用于前端“重新生成指纹”。
func (s *IdentityService) Regenerate(profile *Profile) error {
	return s.RegenerateForPlatform(profile, "")
}

// RegenerateForPlatform 强制为 profile 生成一套唯一身份(可限定平台),存库并刷新 FingerprintArgs。
func (s *IdentityService) RegenerateForPlatform(profile *Profile, platform string) error {
	if profile == nil {
		return nil
	}
	id, err := s.GenerateUniqueForPlatform(platform)
	if err != nil {
		return err
	}
	s.mu.Lock()
	err = s.store.Save(profile.ProfileId, id)
	s.mu.Unlock()
	if err != nil {
		return err
	}
	profile.FingerprintArgs = id.LaunchArgs()
	return nil
}

// ResolveExitIPGeo 用注入的离线 GeoIP 解析器解析代理出口 IP 的地理;
// 未注入解析器(无 mmdb)或解析失败时返回 ok=false,调用方据此优雅降级为不对齐。
func (s *IdentityService) ResolveExitIPGeo(exitIP string) (identity.GeoInfo, bool) {
	s.mu.Lock()
	r := s.resolver
	s.mu.Unlock()
	if r == nil {
		return identity.GeoInfo{}, false
	}
	geo, err := r.Resolve(exitIP)
	if err != nil {
		return identity.GeoInfo{}, false
	}
	return geo, true
}

// AlignProfileToGeo 用给定地理信息对齐 profile 的身份(时区/语言/locale/坐标),
// 存库并刷新 FingerprintArgs。profile 尚无身份时先生成一套再对齐。
func (s *IdentityService) AlignProfileToGeo(profile *Profile, geo identity.GeoInfo) error {
	if profile == nil {
		return nil
	}
	id, ok := s.IdentityForProfile(profile.ProfileId, profile.FingerprintArgs)
	if !ok {
		var err error
		if id, err = s.GenerateUnique(); err != nil {
			return err
		}
	}
	aligned := identity.AlignToProxyGeo(id, geo)
	s.mu.Lock()
	err := s.store.Save(profile.ProfileId, aligned)
	s.mu.Unlock()
	if err != nil {
		return err
	}
	profile.FingerprintArgs = aligned.LaunchArgs()
	return nil
}

// AlignProfileToCountry 用指定国家默认地区设置对齐 profile 身份(时区/语言/locale),
// 保留 seed 等设备指纹字段;存库并刷新 FingerprintArgs。用于直连(无代理)场景:
// 出口即真机本地 IP,应把人设对齐到本地国家(如 CN),避免地理与 IP 矛盾被平台风控判废。
// profile 尚无身份时先生成一套再对齐。countryCode 未收录时不改动地区字段。
func (s *IdentityService) AlignProfileToCountry(profile *Profile, countryCode string) error {
	if profile == nil || strings.TrimSpace(countryCode) == "" {
		return nil
	}
	id, ok := s.IdentityForProfile(profile.ProfileId, profile.FingerprintArgs)
	if !ok {
		var err error
		if id, err = s.GenerateUnique(); err != nil {
			return err
		}
	}
	aligned := identity.AlignToCountry(id, countryCode)
	s.mu.Lock()
	err := s.store.Save(profile.ProfileId, aligned)
	s.mu.Unlock()
	if err != nil {
		return err
	}
	profile.FingerprintArgs = aligned.LaunchArgs()
	return nil
}

// AssignToProfile 为 profile 分配结构化身份并写入 FingerprintArgs:
//   - 已有结构化身份:直接用其序列化结果;
//   - 老环境(仅有 fingerprint_args):反解补齐并落库;
//   - 全新环境:采样生成唯一身份并落库。
func (s *IdentityService) AssignToProfile(profile *Profile) error {
	if profile == nil {
		return nil
	}
	// 已有结构化身份 → 直接序列化。
	if existing, ok, err := s.store.Load(profile.ProfileId); err == nil && ok {
		profile.FingerprintArgs = existing.LaunchArgs()
		return nil
	}
	// 老环境:从既有 flag 反解补齐结构化身份。
	if len(profile.FingerprintArgs) > 0 {
		derived := identity.FromLaunchArgs(profile.FingerprintArgs)
		if derived.FingerprintHash() != "" {
			s.mu.Lock()
			_ = s.store.Save(profile.ProfileId, derived) // 迁移尽力而为,冲突忽略
			s.mu.Unlock()
			profile.FingerprintArgs = derived.LaunchArgs()
			return nil
		}
	}
	// 全新环境:采样生成唯一身份。
	id, err := s.GenerateUnique()
	if err != nil {
		return err
	}
	s.mu.Lock()
	err = s.store.Save(profile.ProfileId, id)
	s.mu.Unlock()
	if err != nil {
		return err
	}
	profile.FingerprintArgs = id.LaunchArgs()
	return nil
}
