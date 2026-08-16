package browser

import (
	"fmt"
	"strings"
	"time"

	"ant-chrome/backend/internal/identity"
	"ant-chrome/backend/internal/logger"
)

// isDirectProfile 判断实例是否为“直连”(无代理):出口即真机本地 IP。
func isDirectProfile(p *Profile) bool {
	if p == nil {
		return false
	}
	pid := strings.TrimSpace(p.ProxyId)
	if pid != "" && pid != directProxyID {
		return false
	}
	pc := strings.TrimSpace(p.ProxyConfig)
	return pc == "" || strings.EqualFold(pc, "direct://")
}

// alignDirectProfileToLocalGeo 若实例为直连(无代理),把身份对齐到本地国家
// (config.local_country,默认 CN),保留 seed。直连出口即真机本地 IP,人设须与之一致,
// 否则平台风控会因“地理(时区/语言)与 IP 国别矛盾”周期性判废登录会话(表现为“过一会掉登录”)。
func (m *Manager) alignDirectProfileToLocalGeo(p *Profile) {
	if m.IdentityService == nil || !isDirectProfile(p) {
		return
	}
	country := strings.TrimSpace(m.Config.Browser.LocalCountry)
	if country == "" {
		country = "CN"
	}
	if err := m.IdentityService.AlignProfileToCountry(p, country); err != nil {
		logger.New("Browser").Warn("直连本地地理对齐失败",
			logger.F("profile_id", p.ProfileId),
			logger.F("country", country),
			logger.F("error", err.Error()),
		)
	}
}

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
	// 直连实例:重生成后按本地国家对齐人设,保持与真机 IP 地理一致。
	m.alignDirectProfileToLocalGeo(p)
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
