package launchcode

import (
	"crypto/rand"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
const codeLen = 12
const maxRetries = 10
const customCodeMinLen = 4
const customCodeMaxLen = 32

var customCodePattern = regexp.MustCompile(`^[A-Z0-9_-]+$`)

// LaunchCodeService 负责 Launch Code 的生成、缓存与管理
type LaunchCodeService struct {
	dao           LaunchCodeDAO
	codeToProfile map[string]string
	profileToCode map[string]string
	mu            sync.RWMutex
	rateLimiter   *rateLimiter // 速率限制器，防止暴力破解
}

// NewLaunchCodeService 创建 LaunchCodeService
func NewLaunchCodeService(dao LaunchCodeDAO) *LaunchCodeService {
	return &LaunchCodeService{
		dao:           dao,
		codeToProfile: make(map[string]string),
		profileToCode: make(map[string]string),
		rateLimiter:   newRateLimiter(10, 20), // 每秒10次尝试，突发20次
	}
}

// EnsureCode 为 profile 生成并持久化 code（幂等：已有则直接返回）
func (s *LaunchCodeService) EnsureCode(profileId string) (string, error) {
	// 使用写锁从一开始就锁定，避免TOCTOU竞态
	s.mu.Lock()
	defer s.mu.Unlock()

	if code, ok := s.profileToCode[profileId]; ok {
		return code, nil
	}

	code, err := s.generateUniqueCodeLocked()
	if err != nil {
		return "", err
	}

	if err := s.dao.Upsert(profileId, code); err != nil {
		return "", err
	}

	s.profileToCode[profileId] = code
	s.codeToProfile[code] = profileId

	return code, nil
}

// SetCode 为指定 profile 设置自定义 launch code。
// code 会自动 trim 并转为大写；格式限制为 4-32 位，字符集 [A-Z0-9_-]。
func (s *LaunchCodeService) SetCode(profileId, code string) (string, error) {
	code = normalizeCode(code)
	if err := validateCustomCode(code); err != nil {
		return "", err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if old, ok := s.profileToCode[profileId]; ok && old == code {
		return code, nil
	}

	if ownerProfile, exists := s.codeToProfile[code]; exists && ownerProfile != profileId {
		return "", fmt.Errorf("launch code already exists")
	}

	if err := s.dao.Upsert(profileId, code); err != nil {
		return "", err
	}

	if old, ok := s.profileToCode[profileId]; ok {
		delete(s.codeToProfile, old)
	}
	s.profileToCode[profileId] = code
	s.codeToProfile[code] = profileId
	return code, nil
}

// RegenerateCode 重新生成 code（废弃旧 code）
func (s *LaunchCodeService) RegenerateCode(profileId string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if oldCode, ok := s.profileToCode[profileId]; ok {
		delete(s.codeToProfile, oldCode)
		delete(s.profileToCode, profileId)
	}

	code, err := s.generateUniqueCodeLocked()
	if err != nil {
		return "", err
	}

	if err := s.dao.Upsert(profileId, code); err != nil {
		return "", err
	}

	s.profileToCode[profileId] = code
	s.codeToProfile[code] = profileId

	return code, nil
}

// Resolve 根据 code 查找 profileId（仅查内存缓存）
// 带速率限制防止暴力破解
func (s *LaunchCodeService) Resolve(code string) (string, error) {
	code = normalizeCode(code)

	// 速率限制检查（使用code作为key，防止单个code被暴力测试）
	if !s.rateLimiter.allow(code) {
		return "", fmt.Errorf("too many attempts, rate limit exceeded")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	profileId, ok := s.codeToProfile[code]
	if !ok {
		return "", fmt.Errorf("launch code not found: %s", code)
	}
	return profileId, nil
}

// Remove 删除 profile 对应的 code（同时清理内存缓存和数据库）
func (s *LaunchCodeService) Remove(profileId string) error {
	s.mu.Lock()
	if code, ok := s.profileToCode[profileId]; ok {
		delete(s.codeToProfile, code)
		delete(s.profileToCode, profileId)
	}
	s.mu.Unlock()

	return s.dao.Delete(profileId)
}

// LoadAll 启动时从数据库加载所有映射到内存
func (s *LaunchCodeService) LoadAll() error {
	profileToCode, err := s.dao.LoadAll()
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.profileToCode = make(map[string]string, len(profileToCode))
	s.codeToProfile = make(map[string]string, len(profileToCode))

	for profileId, code := range profileToCode {
		s.profileToCode[profileId] = code
		s.codeToProfile[code] = profileId
	}
	return nil
}

// generateUniqueCode 生成一个在内存缓存中唯一的 code
func (s *LaunchCodeService) generateUniqueCode() (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.generateUniqueCodeLocked()
}

// generateUniqueCodeLocked 生成唯一code（调用方必须已持有锁）
func (s *LaunchCodeService) generateUniqueCodeLocked() (string, error) {
	for i := 0; i < maxRetries; i++ {
		code, err := randomCode()
		if err != nil {
			return "", fmt.Errorf("生成 launch code 失败: %w", err)
		}

		_, exists := s.codeToProfile[code]
		if !exists {
			return code, nil
		}
	}
	return "", fmt.Errorf("无法在 %d 次重试内生成唯一 launch code", maxRetries)
}

// randomCode 使用 crypto/rand 生成一个随机 12 位字符串
func randomCode() (string, error) {
	buf := make([]byte, codeLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	result := make([]byte, codeLen)
	for i, b := range buf {
		result[i] = charset[int(b)%len(charset)]
	}
	return string(result), nil
}

func normalizeCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func validateCustomCode(code string) error {
	if len(code) < customCodeMinLen || len(code) > customCodeMaxLen {
		return fmt.Errorf("launch code must be %d-%d characters", customCodeMinLen, customCodeMaxLen)
	}
	if !customCodePattern.MatchString(code) {
		return fmt.Errorf("launch code format invalid: only A-Z, 0-9, _ and - are allowed")
	}
	return nil
}
