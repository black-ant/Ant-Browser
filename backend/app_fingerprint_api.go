package backend

import "ant-chrome/backend/internal/identity"

// FingerprintValidationResult 指纹一致性校验结果(前端一致性徽章使用)。
type FingerprintValidationResult = identity.ValidationResult

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
