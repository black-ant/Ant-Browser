package backend

import (
	"encoding/json"
	"fmt"
	"strings"

	"ant-chrome/backend/internal/identity"
	"ant-chrome/backend/internal/logger"
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

// autoAlignProfilesToProxyGeo 创建/换绑后,对绑定了代理的实例自动按代理出口 IP 的地理
// 对齐时区/语言/坐标。同一代理只探测一次出口 IP(批量创建通常共用一个代理);直连实例
// 跳过(创建/换绑路径已按本地国家对齐)。任一环节失败仅记日志降级,不影响创建/更新结果,
// 之后换绑代理会再次触发对齐。
func (a *App) autoAlignProfilesToProxyGeo(profiles []*BrowserProfile) {
	if a.browserMgr == nil || a.browserMgr.IdentityService == nil || len(profiles) == 0 {
		return
	}
	log := logger.New("Fingerprint")
	groups := map[string][]string{} // proxyId -> profileIds
	for _, p := range profiles {
		if p == nil {
			continue
		}
		proxyId := strings.TrimSpace(p.ProxyId)
		if proxyId == "" || strings.EqualFold(proxyId, "__direct__") {
			continue
		}
		groups[proxyId] = append(groups[proxyId], p.ProfileId)
	}
	for proxyId, ids := range groups {
		geo, ok := a.proxyExitGeo(proxyId)
		if !ok {
			log.Warn("代理出口地理解析失败，本次跳过自动对齐（重新换绑代理可再次触发）",
				logger.F("proxy_id", proxyId), logger.F("profiles", len(ids)))
			continue
		}
		aligned, err := a.browserMgr.AlignFingerprintsToProxyGeo(ids, geo)
		if err != nil {
			log.Warn("按代理地理对齐持久化失败", logger.F("proxy_id", proxyId), logger.F("error", err.Error()))
			continue
		}
		log.Info("已按代理出口地理自动对齐",
			logger.F("proxy_id", proxyId),
			logger.F("country", geo.CountryCode),
			logger.F("timezone", geo.Timezone),
			logger.F("aligned", aligned),
		)
	}
}

// proxyExitGeo 解析代理出口 IP 的地理:先实测(经代理探测出口 IP),失败回退上次持久化
// 的健康检测结果里的出口 IP(代理出口一般稳定)。两者都拿不到或 GeoIP 未就绪则 ok=false。
func (a *App) proxyExitGeo(proxyId string) (identity.GeoInfo, bool) {
	if a.browserMgr == nil || a.browserMgr.IdentityService == nil {
		return identity.GeoInfo{}, false
	}
	health := a.BrowserProxyCheckIPHealth(proxyId)
	ip := strings.TrimSpace(health.IP)
	if !health.Ok || ip == "" {
		ip = a.lastKnownProxyExitIP(proxyId)
	}
	if ip == "" {
		return identity.GeoInfo{}, false
	}
	return a.browserMgr.IdentityService.ResolveExitIPGeo(ip)
}

// lastKnownProxyExitIP 取代理上次持久化的 IP 健康结果里的出口 IP(实测失败时的兜底)。
func (a *App) lastKnownProxyExitIP(proxyId string) string {
	for _, p := range a.getLatestProxies() {
		if p.ProxyId != proxyId {
			continue
		}
		var last ProxyIPHealthResult
		if strings.TrimSpace(p.LastIPHealthJSON) == "" {
			return ""
		}
		if json.Unmarshal([]byte(p.LastIPHealthJSON), &last) == nil && last.Ok {
			return strings.TrimSpace(last.IP)
		}
		return ""
	}
	return ""
}

// BrowserProfileRegenerateFingerprint 为实例重新生成唯一自洽指纹身份。
// 重生成后地区字段回到池内占位值,因此挂代理的实例随即按代理出口地理自动对齐
// (直连实例已在 Manager 内按本地国家对齐)。
func (a *App) BrowserProfileRegenerateFingerprint(profileId string) (*BrowserProfile, error) {
	profile, err := a.browserMgr.RegenerateFingerprint(profileId)
	if err != nil {
		return nil, err
	}
	a.autoAlignProfilesToProxyGeo([]*BrowserProfile{profile})
	return profile, nil
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
