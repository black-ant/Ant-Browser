package backend

import "ant-chrome/backend/internal/logger"

// applyIdentityCDPOverrides 在实例调试就绪后下发身份的 CDP 覆盖。
//
// 时区/语言已由启动 flag(--timezone/--lang/--accept-lang)进程级生效,这里的 CDP
// 覆盖主要用于**地理定位**(Chrome 144+ 已无对应启动 flag),时区/语言 CDP 覆盖为兜底。
// best-effort:失败只告警,不影响实例运行。
//
// 说明:CDP 地理覆盖按页面会话生效,当前对主页面下发;多标签的持久化注入
// (Target.setAutoAttach)留待后续增强(见 specs/.../research.md R3)。
func (a *App) applyIdentityCDPOverrides(profileID string, profile *BrowserProfile, debugPort int) {
	if a.browserMgr == nil || a.browserMgr.IdentityService == nil || profile == nil {
		return
	}
	id, ok := a.browserMgr.IdentityService.IdentityForProfile(profileID, profile.FingerprintArgs)
	if !ok {
		return
	}
	overrides := id.CDPOverrides()
	if len(overrides) == 0 {
		return
	}
	log := logger.New("Browser")
	for _, ov := range overrides {
		if _, err := cdpCall(debugPort, ov.Method, ov.Params); err != nil {
			log.Warn("身份 CDP 覆盖下发失败",
				logger.F("profile_id", profileID),
				logger.F("method", ov.Method),
				logger.F("error", err.Error()),
			)
		}
	}
}
