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
// existingArgs 为该实例当前已拼好的全部启动参数（含用户指纹/启动参数）。
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

// proxyGeoRefreshInflight 去重并发的后台地理缓存刷新（同一代理同一时刻只查一次）。
var proxyGeoRefreshInflight sync.Map

// refreshProxyGeoCacheAsync 在后台经代理链路查询出口 IP 健康信息并持久化，
// 使下次启动即可按代理地区注入时区/语言。非阻塞、失败静默（仅记日志）。
func (a *App) refreshProxyGeoCacheAsync(proxyId string) {
	proxyId = strings.TrimSpace(proxyId)
	if proxyId == "" {
		return
	}
	if _, loaded := proxyGeoRefreshInflight.LoadOrStore(proxyId, struct{}{}); loaded {
		return
	}

	go func() {
		defer proxyGeoRefreshInflight.Delete(proxyId)

		proxies := a.getLatestProxies()
		data, err := proxy.FetchIPPureInfo(proxyId, proxies, a.xrayMgr, a.singboxMgr)
		result := buildProxyIPHealthResult(proxyId, data, err)
		if !result.Ok {
			logger.New("Browser").Debug("后台代理地理缓存刷新失败",
				logger.F("proxy_id", proxyId), logger.F("error", result.Error))
			return
		}
		a.persistProxyIPHealthResult(result)
		logger.New("Browser").Info("已缓存代理出口地理信息",
			logger.F("proxy_id", proxyId),
			logger.F("country", result.Country),
			logger.F("region", result.Region))
	}()
}
