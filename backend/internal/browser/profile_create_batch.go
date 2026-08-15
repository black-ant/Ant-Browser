package browser

import (
	"fmt"
	"strings"

	"ant-chrome/backend/internal/logger"
)

// MaxBatchCreateCount 单次批量创建的数量上限,防止一次性建太多把资源打满。
const MaxBatchCreateCount = 200

// CreateBatch 批量创建 count 个环境,名称为 prefix-编号(3 位,从 startIndex 起,startIndex<=0 视为 1)。
// 每个环境都会生成一套独立、唯一、自洽的指纹身份(与单个"新建配置"完全一致的生成路径)。
//
// 关键点:
//   - 在单次 m.Mutex 锁内顺序创建全部环境,末尾只 SaveProfiles() 一次,避免 O(N²) 写放大
//     (SaveProfiles 会 upsert 全部 profile;若每个都存一次,N 个即 ~N²/2 次 upsert)。
//   - 每个环境强制清空模板里的指纹参数,确保都走"唯一自洽身份"生成分支,而非共用同一套。
//   - 唯一性由 IdentityService(GenerateUnique + browser_identities 的 UNIQUE 索引)逐个即时
//     登记保证,因此 N 次顺序创建得到 N 套互不相同的身份。
func (m *Manager) CreateBatch(prefix string, count, startIndex int, template ProfileInput) ([]*Profile, error) {
	log := logger.New("Browser")

	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return nil, fmt.Errorf("前缀不能为空")
	}
	if count <= 0 {
		return nil, fmt.Errorf("数量必须大于 0")
	}
	if count > MaxBatchCreateCount {
		return nil, fmt.Errorf("单次批量创建数量不能超过 %d 个", MaxBatchCreateCount)
	}
	if startIndex <= 0 {
		startIndex = 1
	}

	m.InitData()
	m.Mutex.Lock()
	defer m.Mutex.Unlock()

	created := make([]*Profile, 0, count)
	for i := 0; i < count; i++ {
		item := template
		item.ProfileName = fmt.Sprintf("%s-%03d", prefix, startIndex+i)
		item.FingerprintArgs = nil // 强制每个环境重新采样唯一自洽身份
		item.UserDataDir = ""      // 由 createProfileLocked 以各自新 profileId 命名,避免共用目录

		profile, err := m.createProfileLocked(item)
		if err != nil {
			// 中途失败:持久化已建部分并回填启动码,返回部分结果 + 错误,便于前端提示"已创建 N 个"。
			if saveErr := m.SaveProfiles(); saveErr != nil {
				log.Error("批量创建中途保存失败", logger.F("error", saveErr.Error()))
			}
			for _, p := range created {
				m.ensureProfileLaunchCode(p)
			}
			return created, fmt.Errorf("批量创建第 %d 个(%s)失败: %w", i+1, item.ProfileName, err)
		}
		created = append(created, profile)
	}

	if err := m.SaveProfiles(); err != nil {
		return nil, fmt.Errorf("批量创建保存失败: %w", err)
	}
	for _, p := range created {
		m.ensureProfileLaunchCode(p)
	}

	log.Info("批量创建完成", logger.F("prefix", prefix), logger.F("count", len(created)))
	return created, nil
}
