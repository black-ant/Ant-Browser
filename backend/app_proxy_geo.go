package backend

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/logger"
	"ant-chrome/backend/internal/proxy"
)

const antTimezoneModeArgPrefix = "--ant-timezone-mode="
const proxyGeoLaunchFetchTimeout = 8 * time.Second

type proxyConsistencyControls struct {
	SkipTimezone bool
}

func extractProxyConsistencyControlArgs(args []string) ([]string, proxyConsistencyControls) {
	if len(args) == 0 {
		return nil, proxyConsistencyControls{}
	}

	filtered := make([]string, 0, len(args))
	controls := proxyConsistencyControls{}
	for _, raw := range args {
		arg := strings.TrimSpace(raw)
		if arg == "" {
			continue
		}
		if strings.HasPrefix(arg, antTimezoneModeArgPrefix) {
			if strings.EqualFold(strings.TrimSpace(strings.TrimPrefix(arg, antTimezoneModeArgPrefix)), "real") {
				controls.SkipTimezone = true
			}
			continue
		}
		filtered = append(filtered, arg)
	}
	return filtered, controls
}

// buildProxyConsistencyArgs 依据代理出口 IP 的地理信息，补齐与代理地区一致的指纹要素
// （时区 / 语言 / WebRTC 防泄露策略）。
//
// 反检测原则：
//   - 只补「缺失」项——用户在 FingerprintArgs / 启动参数里显式设置的值始终优先，绝不覆盖
//     （Chromium 同名参数后者生效，因此必须靠前缀判重来保证用户值生效）。
//   - 时区 / 语言依赖代理地理信息，只有解析出可用 geo 时才注入。
//   - WebRTC 策略与代理与否无关，缺失即补默认防泄露值，保护历史 profile。
//
// existingArgs 为该窗口当前已拼好的全部启动参数（含用户指纹/启动参数）。
func buildProxyConsistencyArgs(existingArgs []string, healthJSON string, controls ...proxyConsistencyControls) []string {
	control := proxyConsistencyControls{}
	if len(controls) > 0 {
		control = controls[0]
	}

	has := func(prefix string) bool {
		for _, a := range existingArgs {
			if strings.HasPrefix(strings.TrimSpace(a), prefix) {
				return true
			}
		}
		return false
	}

	out := make([]string, 0, 4)

	if geo, ok := browser.DeriveProxyGeo(healthJSON); ok {
		if geo.Timezone != "" && !control.SkipTimezone && !has("--timezone=") {
			out = append(out, "--timezone="+geo.Timezone)
		}
		// 仅当用户未显式设置 --lang 时，才基于代理 geo 推导语言。
		// 若用户已设 --lang=ja-JP，此处不应再因「代理在美国」而注入 --lang=en-US，
		// 那正是指纹一致性要避免的泄漏（navigator.language 与 Accept-Language 不一致）。
		if geo.Language != "" && !has("--lang=") {
			out = append(out, "--lang="+geo.Language)
			// Accept-Language 跟随 --lang 派生，确保与 navigator.language 一致。
			if !has("--accept-language=") {
				out = append(out, "--accept-language="+acceptLanguageForPrimaryTag(geo.Language))
			}
		}
	}

	if !has("--webrtc-ip-handling-policy=") {
		out = append(out, "--webrtc-ip-handling-policy=disable_non_proxied_udp")
	}

	return out
}

func acceptLanguageForPrimaryTag(language string) string {
	language = strings.TrimSpace(language)
	if language == "" {
		return ""
	}
	primary := language
	if idx := strings.Index(primary, "-"); idx > 0 {
		primary = primary[:idx]
	}
	primary = strings.TrimSpace(primary)
	if primary == "" || strings.EqualFold(primary, language) {
		return language
	}
	return language + "," + primary
}

func (a *App) ensureProxyGeoCacheForLaunch(proxyId string, existingHealthJSON string) string {
	existingHealthJSON = strings.TrimSpace(existingHealthJSON)
	if existingHealthJSON != "" {
		return existingHealthJSON
	}

	proxyId = strings.TrimSpace(proxyId)
	if proxyId == "" {
		return ""
	}
	if _, loaded := proxyGeoRefreshInflight.LoadOrStore(proxyId, struct{}{}); loaded {
		return ""
	}

	type fetchResult struct {
		healthJSON string
		err        error
	}
	ch := make(chan fetchResult, 1)
	go func() {
		defer proxyGeoRefreshInflight.Delete(proxyId)
		proxies := a.getLatestProxies()
		data, err := proxy.FetchIPPureInfo(proxyId, proxies, a.xrayMgr, a.singboxMgr)
		result := buildProxyIPHealthResult(proxyId, data, err)
		if result.Ok {
			a.persistProxyIPHealthResult(result)
			payload, marshalErr := json.Marshal(result)
			if marshalErr == nil {
				ch <- fetchResult{healthJSON: string(payload)}
				return
			}
			ch <- fetchResult{err: marshalErr}
			return
		}
		ch <- fetchResult{err: err}
	}()

	select {
	case result := <-ch:
		if strings.TrimSpace(result.healthJSON) != "" {
			logger.New("Browser").Info("启动前已获取代理出口地理信息",
				logger.F("proxy_id", proxyId))
			return result.healthJSON
		}
		if result.err != nil {
			logger.New("Browser").Debug("启动前代理出口地理信息获取失败",
				logger.F("proxy_id", proxyId),
				logger.F("error", result.err.Error()))
		}
	case <-time.After(proxyGeoLaunchFetchTimeout):
		logger.New("Browser").Debug("启动前代理出口地理信息获取超时",
			logger.F("proxy_id", proxyId),
			logger.F("timeout_ms", proxyGeoLaunchFetchTimeout.Milliseconds()))
	}

	return ""
}

func (a *App) ensureManualProxyGeoForLaunch(proxyConfig string) string {
	proxyConfig = strings.TrimSpace(proxyConfig)
	if proxyConfig == "" || strings.EqualFold(proxyConfig, "direct://") {
		return ""
	}

	sum := sha256.Sum256([]byte(proxyConfig))
	inflightKey := fmt.Sprintf("manual:%x", sum[:])
	if _, loaded := proxyGeoRefreshInflight.LoadOrStore(inflightKey, struct{}{}); loaded {
		return ""
	}

	type fetchResult struct {
		healthJSON string
		err        error
	}
	ch := make(chan fetchResult, 1)
	go func() {
		defer proxyGeoRefreshInflight.Delete(inflightKey)
		proxies := a.getLatestProxies()
		data, err := proxy.FetchIPPureInfoByConfig(proxyConfig, proxies, a.xrayMgr, a.singboxMgr)
		result := buildProxyIPHealthResult("", data, err)
		if result.Ok {
			payload, marshalErr := json.Marshal(result)
			if marshalErr == nil {
				ch <- fetchResult{healthJSON: string(payload)}
				return
			}
			ch <- fetchResult{err: marshalErr}
			return
		}
		ch <- fetchResult{err: err}
	}()

	select {
	case result := <-ch:
		if strings.TrimSpace(result.healthJSON) != "" {
			logger.New("Browser").Info("启动前已获取手动代理出口地理信息",
				logger.F("proxy", sanitizeProxyConfigField(proxyConfig)))
			return result.healthJSON
		}
		if result.err != nil {
			logger.New("Browser").Debug("启动前手动代理出口地理信息获取失败",
				logger.F("proxy", sanitizeProxyConfigField(proxyConfig)),
				logger.F("error", result.err.Error()))
		}
	case <-time.After(proxyGeoLaunchFetchTimeout):
		logger.New("Browser").Debug("启动前手动代理出口地理信息获取超时",
			logger.F("proxy", sanitizeProxyConfigField(proxyConfig)),
			logger.F("timeout_ms", proxyGeoLaunchFetchTimeout.Milliseconds()))
	}

	return ""
}

// proxyGeoRefreshInflight 去重并发的后台代理出口地理查询。键空间按用途加前缀区分：
// 裸 proxyId（启动前同步暖缓存）、"manual:<hash>"（手动代理配置）、"reapply:<profileId>"
// （首启后自愈式重应用，每个窗口一次）。同一键同一时刻只查一次。
var proxyGeoRefreshInflight sync.Map

// scheduleProxyGeoRuntimeReapply 处理「首启时尚无代理出口 geo」的盲区：窗口已经起来，
// 但启动那一刻没拿到 geo（新窗口/新代理无缓存，且同步拉取超时或被并发去重命中），
// 导致时区/地理定位回落到宿主机——检测站即爆「时区不同」。这里在后台经代理补查一次出口
// IP geo，成功后：
//  1. 持久化缓存（供下次启动直接注入，等价于旧的 refreshProxyGeoCacheAsync）；
//  2. 立即把时区（必要时连同地理定位）重新应用到当前运行中的窗口——无需重启。
//
// 仅应在「首启未注入 geo」时调度。geoAppliedAtLaunch 表示启动时是否已应用过地理定位
// （显式设置或已注入代理 geo），为 true 时本次不再重复应用地理定位，避免与启动期已起的
// 地理定位监听重复。非阻塞、失败静默（仅记日志）。
func (a *App) scheduleProxyGeoRuntimeReapply(profileId string, debugPort int, proxyId, proxyConfig string, fromPool bool, controls proxyConsistencyControls, geoAppliedAtLaunch bool) {
	profileId = strings.TrimSpace(profileId)
	proxyId = strings.TrimSpace(proxyId)
	proxyConfig = strings.TrimSpace(proxyConfig)
	if profileId == "" {
		return
	}
	// 每个窗口一次自愈，使用独立去重键，不与按 proxyId 的缓存暖键冲突。
	inflightKey := "reapply:" + profileId
	if _, loaded := proxyGeoRefreshInflight.LoadOrStore(inflightKey, struct{}{}); loaded {
		return
	}

	go func() {
		defer proxyGeoRefreshInflight.Delete(inflightKey)

		log := logger.New("Browser")
		proxies := a.getLatestProxies()

		var result ProxyIPHealthResult
		switch {
		case proxyId != "":
			data, err := proxy.FetchIPPureInfo(proxyId, proxies, a.xrayMgr, a.singboxMgr)
			result = buildProxyIPHealthResult(proxyId, data, err)
		case !fromPool && proxyConfig != "":
			data, err := proxy.FetchIPPureInfoByConfig(proxyConfig, proxies, a.xrayMgr, a.singboxMgr)
			result = buildProxyIPHealthResult("", data, err)
		default:
			return
		}
		if !result.Ok {
			log.Debug("首启后代理地理补查失败",
				logger.F("profile_id", profileId),
				logger.F("proxy_id", proxyId),
				logger.F("error", result.Error))
			return
		}

		// 暖缓存：供下次启动直接注入（等价旧 refreshProxyGeoCacheAsync）。
		if proxyId != "" {
			a.persistProxyIPHealthResult(result)
		}

		payload, err := json.Marshal(result)
		if err != nil {
			return
		}
		healthJSON := string(payload)

		// 窗口可能在补查期间被关闭 / 换了端口：仅当仍在原端口存活时才应用，避免误伤复用端口的新窗口。
		if !a.profileDebugPortActive(profileId, debugPort) {
			log.Debug("首启后代理地理补查完成，但窗口已不在原调试端口，跳过重应用",
				logger.F("profile_id", profileId),
				logger.F("debug_port", debugPort))
			return
		}

		overrides := browserRuntimeOverrides{}
		if geo, ok := browser.DeriveProxyGeo(healthJSON); ok {
			if geo.Timezone != "" && !controls.SkipTimezone {
				overrides.Timezone = profileTimezoneOverride{TimezoneID: geo.Timezone, Source: "proxy"}
			}
		}
		// 启动时未应用地理定位才补——否则会与启动期已起的地理定位监听重复。
		if !geoAppliedAtLaunch {
			if geo, ok := proxyGeolocationOverride(healthJSON); ok {
				overrides.Geolocation = geo
			}
		}
		if !overrides.Timezone.shouldApply() && !overrides.Geolocation.shouldApply() {
			return
		}

		if err := a.applyAndWatchBrowserRuntimeOverrides(profileId, debugPort, overrides); err != nil {
			log.Warn("首启后补充应用代理时区/地理定位失败",
				logger.F("profile_id", profileId),
				logger.F("debug_port", debugPort),
				logger.F("timezone", overrides.Timezone.TimezoneID),
				logger.F("error", err.Error()))
			return
		}
		log.Info("首启后补充应用代理时区/地理定位",
			logger.F("profile_id", profileId),
			logger.F("debug_port", debugPort),
			logger.F("timezone", overrides.Timezone.TimezoneID),
			logger.F("country", result.Country),
			logger.F("region", result.Region),
			logger.F("geolocation_applied", overrides.Geolocation.shouldApply()))
	}()
}
