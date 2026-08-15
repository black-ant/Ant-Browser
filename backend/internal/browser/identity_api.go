package browser

import (
	"fmt"
	"time"

	"ant-chrome/backend/internal/identity"
)

// RegenerateFingerprint 为实例重新生成一套唯一自洽身份并持久化。
func (m *Manager) RegenerateFingerprint(profileId string) (*Profile, error) {
	m.InitData()
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	p, ok := m.Profiles[profileId]
	if !ok {
		return nil, fmt.Errorf("未找到实例配置（ID=%s）", profileId)
	}
	if m.IdentityService == nil {
		return nil, fmt.Errorf("指纹自洽引擎不可用")
	}
	if err := m.IdentityService.Regenerate(p); err != nil {
		return nil, err
	}
	p.UpdatedAt = time.Now().Format(time.RFC3339)
	if err := m.SaveProfiles(); err != nil {
		return nil, err
	}
	return p, nil
}

// AlignFingerprintToProxyGeo 用给定的代理出口地理对齐实例身份(时区/语言/地理)并持久化。
func (m *Manager) AlignFingerprintToProxyGeo(profileId string, geo identity.GeoInfo) (*Profile, error) {
	m.InitData()
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	p, ok := m.Profiles[profileId]
	if !ok {
		return nil, fmt.Errorf("未找到实例配置（ID=%s）", profileId)
	}
	if m.IdentityService == nil {
		return nil, fmt.Errorf("指纹自洽引擎不可用")
	}
	if err := m.IdentityService.AlignProfileToGeo(p, geo); err != nil {
		return nil, err
	}
	p.UpdatedAt = time.Now().Format(time.RFC3339)
	if err := m.SaveProfiles(); err != nil {
		return nil, err
	}
	return p, nil
}

// ValidateFingerprint 校验实例当前身份的自洽性。
func (m *Manager) ValidateFingerprint(profileId string) (identity.ValidationResult, error) {
	m.InitData()
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	p, ok := m.Profiles[profileId]
	if !ok {
		return identity.ValidationResult{}, fmt.Errorf("未找到实例配置（ID=%s）", profileId)
	}
	if m.IdentityService == nil {
		return identity.ValidationResult{}, fmt.Errorf("指纹自洽引擎不可用")
	}
	id, found := m.IdentityService.IdentityForProfile(profileId, p.FingerprintArgs)
	if !found {
		return identity.ValidationResult{OK: false, Issues: []identity.Issue{{
			Field: "identity", Message: "该实例尚无结构化身份", Severity: identity.SeverityWarning, Fixable: true,
		}}}, nil
	}
	return identity.Validate(id), nil
}
