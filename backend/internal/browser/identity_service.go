package browser

import (
	"database/sql"
	"math/rand"
	"sync"
	"time"

	"ant-chrome/backend/internal/identity"
)

// IdentityService 把指纹自洽引擎(池采样 + 唯一性登记 + 序列化)封装成实例管理器可用的服务。
// 在创建环境时为其生成唯一自洽的结构化身份并落库,同时把身份序列化为 fingerprint_args。
type IdentityService struct {
	pool     *identity.Pool
	store    *identity.SQLiteStore
	resolver identity.GeoResolver // 可选;接入离线 GeoIP mmdb 后用于代理地理对齐
	mu       sync.Mutex
	rng      *rand.Rand
}

// NewIdentityService 用数据库连接创建服务,载入内嵌指纹池。
func NewIdentityService(db *sql.DB) (*IdentityService, error) {
	pool, err := identity.LoadEmbeddedPool()
	if err != nil {
		return nil, err
	}
	return &IdentityService{
		pool:  pool,
		store: identity.NewSQLiteStore(db),
		rng:   rand.New(rand.NewSource(time.Now().UnixNano())),
	}, nil
}

// SetGeoResolver 注入离线地理解析器,启用代理地理对齐(mmdb 就绪后调用)。
func (s *IdentityService) SetGeoResolver(r identity.GeoResolver) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resolver = r
}

// GenerateUnique 采样并重采,产出一套跨环境唯一的基础身份(尚未对齐代理地理)。
func (s *IdentityService) GenerateUnique() (identity.Identity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return identity.GenerateUnique(s.store, func() identity.Identity {
		return s.pool.NewIdentity(s.rng)
	}, 100)
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
