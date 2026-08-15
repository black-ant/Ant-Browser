package backend

import (
	"fmt"
	"strings"

	"ant-chrome/backend/internal/identity"
)

// FingerprintValidationResult 指纹一致性校验结果(前端一致性徽章使用)。
type FingerprintValidationResult = identity.ValidationResult

// BrowserProfileAlignFingerprintToProxy 探测实例代理出口 IP,用离线 GeoIP(含精确时区+坐标)
// 把身份的时区/语言/地理定位对齐到代理,并持久化。
func (a *App) BrowserProfileAlignFingerprintToProxy(profileId string) (*BrowserProfile, error) {
	profileId = strings.TrimSpace(profileId)
	profile, ok := a.browserMgr.Profiles[profileId]
	if !ok {
		return nil, fmt.Errorf("未找到实例配置（ID=%s）", profileId)
	}
	if a.browserMgr.IdentityService == nil {
		return nil, fmt.Errorf("指纹自洽引擎不可用")
	}
	proxyId := strings.TrimSpace(profile.ProxyId)
	if proxyId == "" || strings.EqualFold(proxyId, "__direct__") {
		return nil, fmt.Errorf("该实例为直连，无需按代理对齐")
	}
	health := a.BrowserProxyCheckIPHealth(proxyId)
	if !health.Ok || strings.TrimSpace(health.IP) == "" {
		return nil, fmt.Errorf("无法探测代理出口 IP：%s", strings.TrimSpace(health.Error))
	}
	geo, ok := a.browserMgr.IdentityService.ResolveExitIPGeo(health.IP)
	if !ok {
		return nil, fmt.Errorf("离线 GeoIP 未就绪或解析失败（出口 IP %s）。请确认 data/geoip 下已放置 .mmdb", health.IP)
	}
	return a.browserMgr.AlignFingerprintToProxyGeo(profileId, geo)
}

// BrowserProfileRegenerateFingerprint 为实例重新生成唯一自洽指纹身份。
func (a *App) BrowserProfileRegenerateFingerprint(profileId string) (*BrowserProfile, error) {
	return a.browserMgr.RegenerateFingerprint(profileId)
}

// BrowserProfileValidateFingerprint 校验实例指纹身份的自洽性。
func (a *App) BrowserProfileValidateFingerprint(profileId string) (FingerprintValidationResult, error) {
	return a.browserMgr.ValidateFingerprint(profileId)
}

// BrowserGetMemorySaver 返回省内存模式开关。
func (a *App) BrowserGetMemorySaver() bool {
	if a.config == nil {
		return false
	}
	return a.config.Browser.MemorySaverEnabled
}

// BrowserSetMemorySaver 设置省内存模式开关(下次启动实例生效)。
func (a *App) BrowserSetMemorySaver(enabled bool) {
	if a.config != nil {
		a.config.Browser.MemorySaverEnabled = enabled
	}
}
