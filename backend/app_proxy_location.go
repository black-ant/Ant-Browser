package backend

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type ProxyLocationResolveResult struct {
	ProxyId    string                `json:"proxyId"`
	Ok         bool                  `json:"ok"`
	Auto       bool                  `json:"auto"`
	Source     string                `json:"source"`
	Error      string                `json:"error"`
	IP         string                `json:"ip"`
	Country    string                `json:"country"`
	Region     string                `json:"region"`
	City       string                `json:"city"`
	Timezone   string                `json:"timezone"`
	Lang       string                `json:"lang"`
	Health     *ProxyIPHealthResult  `json:"health,omitempty"`
	Alternates []ProxyLocationOption `json:"alternates,omitempty"`
	ResolvedAt string                `json:"resolvedAt"`
}

type ProxyLocationOption struct {
	Label    string `json:"label"`
	Timezone string `json:"timezone"`
	Lang     string `json:"lang"`
}

var countryLocaleDefaults = map[string]ProxyLocationOption{
	"CN": {Label: "中国", Timezone: "Asia/Shanghai", Lang: "zh-CN"},
	"HK": {Label: "中国香港", Timezone: "Asia/Hong_Kong", Lang: "zh-HK"},
	"TW": {Label: "中国台湾", Timezone: "Asia/Taipei", Lang: "zh-TW"},
	"US": {Label: "美国", Timezone: "America/New_York", Lang: "en-US"},
	"GB": {Label: "英国", Timezone: "Europe/London", Lang: "en-GB"},
	"JP": {Label: "日本", Timezone: "Asia/Tokyo", Lang: "ja-JP"},
	"KR": {Label: "韩国", Timezone: "Asia/Seoul", Lang: "ko-KR"},
	"SG": {Label: "新加坡", Timezone: "Asia/Singapore", Lang: "en-SG"},
	"DE": {Label: "德国", Timezone: "Europe/Berlin", Lang: "de-DE"},
	"FR": {Label: "法国", Timezone: "Europe/Paris", Lang: "fr-FR"},
	"NL": {Label: "荷兰", Timezone: "Europe/Amsterdam", Lang: "nl-NL"},
	"CA": {Label: "加拿大", Timezone: "America/Toronto", Lang: "en-CA"},
	"AU": {Label: "澳大利亚", Timezone: "Australia/Sydney", Lang: "en-AU"},
	"RU": {Label: "俄罗斯", Timezone: "Europe/Moscow", Lang: "ru-RU"},
	"BR": {Label: "巴西", Timezone: "America/Sao_Paulo", Lang: "pt-BR"},
	"IN": {Label: "印度", Timezone: "Asia/Kolkata", Lang: "en-IN"},
}

var cityTimezoneDefaults = map[string]string{
	"US|new york":      "America/New_York",
	"US|los angeles":   "America/Los_Angeles",
	"US|san francisco": "America/Los_Angeles",
	"US|chicago":       "America/Chicago",
	"US|denver":        "America/Denver",
	"US|phoenix":       "America/Phoenix",
	"CA|toronto":       "America/Toronto",
	"CA|vancouver":     "America/Vancouver",
	"AU|sydney":        "Australia/Sydney",
	"AU|melbourne":     "Australia/Melbourne",
	"AU|perth":         "Australia/Perth",
}

func (a *App) BrowserProxyResolveLocation(proxyId string) ProxyLocationResolveResult {
	proxyId = strings.TrimSpace(proxyId)
	resolvedAt := time.Now().Format(time.RFC3339)
	if proxyId == "" || strings.EqualFold(proxyId, "__direct__") {
		return ProxyLocationResolveResult{ProxyId: proxyId, Ok: false, Auto: false, Source: "manual", Error: "直连或未选择代理，请手动选择定位", ResolvedAt: resolvedAt}
	}

	if cached, ok := a.cachedProxyIPHealthResult(proxyId); ok && cached.Ok {
		if result := buildProxyLocationResolveResult(proxyId, a.fillMissingGeoFromMMDB(cached), "cache", resolvedAt); result.Ok {
			return result
		}
		// 缓存虽然 Ok 但解不出定位(如旧版检测端点没返回国家字段)——
		// 不能被这种坏缓存永久卡死,忽略缓存重新检测一次。
	}

	health := a.BrowserProxyCheckIPHealth(proxyId)
	if !health.Ok {
		return ProxyLocationResolveResult{ProxyId: proxyId, Ok: false, Auto: false, Source: health.Source, Error: health.Error, Health: &health, ResolvedAt: resolvedAt}
	}
	return buildProxyLocationResolveResult(proxyId, a.fillMissingGeoFromMMDB(health), "ip_health", resolvedAt)
}

// fillMissingGeoFromMMDB 在健康检查拿到了出口 IP 但国家字段缺失/无法识别时,
// 用应用内置的离线 GeoIP 库(data/geoip/dbip-city-lite.mmdb)兜底补出国家码与城市。
func (a *App) fillMissingGeoFromMMDB(health ProxyIPHealthResult) ProxyIPHealthResult {
	if a == nil || a.geoResolver == nil {
		return health
	}
	ip := strings.TrimSpace(health.IP)
	if ip == "" || resolveProxyLocationCountryCode(health) != "" {
		return health
	}
	info, err := a.geoResolver.Resolve(ip)
	if err != nil || strings.TrimSpace(info.CountryCode) == "" {
		return health
	}
	if health.RawData == nil {
		health.RawData = map[string]interface{}{}
	}
	health.RawData["countryCode"] = info.CountryCode
	health.RawData["_geoFallback"] = "local_mmdb"
	if strings.TrimSpace(health.Country) == "" {
		health.Country = info.CountryCode
	}
	if strings.TrimSpace(health.City) == "" {
		health.City = info.City
	}
	return health
}

func (a *App) cachedProxyIPHealthResult(proxyId string) (ProxyIPHealthResult, bool) {
	if a == nil || a.browserMgr == nil || a.browserMgr.ProxyDAO == nil {
		return ProxyIPHealthResult{}, false
	}
	proxies, err := a.browserMgr.ProxyDAO.List()
	if err != nil {
		return ProxyIPHealthResult{}, false
	}
	for _, item := range proxies {
		if !strings.EqualFold(strings.TrimSpace(item.ProxyId), proxyId) || strings.TrimSpace(item.LastIPHealthJSON) == "" {
			continue
		}
		var result ProxyIPHealthResult
		if err := json.Unmarshal([]byte(item.LastIPHealthJSON), &result); err == nil && result.ProxyId != "" {
			return result, true
		}
	}
	return ProxyIPHealthResult{}, false
}

func buildProxyLocationResolveResult(proxyId string, health ProxyIPHealthResult, source string, resolvedAt string) ProxyLocationResolveResult {
	countryCode := resolveProxyLocationCountryCode(health)
	option := resolveProxyLocationOption(countryCode, health.Country, health.City)
	ok := health.Ok && option.Timezone != "" && option.Lang != ""
	result := ProxyLocationResolveResult{
		ProxyId:    proxyId,
		Ok:         ok,
		Auto:       ok,
		Source:     source,
		IP:         health.IP,
		Country:    health.Country,
		Region:     health.Region,
		City:       health.City,
		Timezone:   option.Timezone,
		Lang:       option.Lang,
		Health:     &health,
		ResolvedAt: resolvedAt,
	}
	if !ok {
		detail := strings.TrimSpace(strings.TrimSpace(health.Country) + " " + strings.TrimSpace(health.City))
		if detail == "" {
			detail = fmt.Sprintf("检测目标未返回国家信息（出口 IP %s，来源 %s），请手动选择定位或重新检测", strings.TrimSpace(health.IP), strings.TrimSpace(health.Source))
		}
		result.Error = "无法根据地区自动匹配定位：" + detail
		result.Alternates = defaultProxyLocationOptions()
	}
	return result
}

func resolveProxyLocationOption(countryCode string, country string, city string) ProxyLocationOption {
	countryCode = normalizeCountryCode(countryCode)
	if countryCode == "" {
		countryCode = normalizeCountryCode(country)
	}
	option := countryLocaleDefaults[countryCode]
	if option.Timezone == "" {
		return ProxyLocationOption{}
	}
	cityKey := countryCode + "|" + strings.ToLower(strings.TrimSpace(city))
	if timezone := cityTimezoneDefaults[cityKey]; timezone != "" {
		option.Timezone = timezone
	}
	return option
}

func resolveProxyLocationCountryCode(health ProxyIPHealthResult) string {
	if health.RawData != nil {
		for _, key := range []string{"countryCode", "country_code", "countryISO", "country_iso_code", "cc"} {
			if code := normalizeCountryCode(mapString(health.RawData, key)); code != "" {
				return code
			}
		}
	}
	return normalizeCountryCode(health.Country)
}

func normalizeCountryCode(country string) string {
	value := strings.TrimSpace(country)
	upper := strings.ToUpper(value)
	if len(upper) == 2 {
		return upper
	}
	switch strings.ToLower(value) {
	case "china", "中国", "mainland china":
		return "CN"
	case "hong kong", "香港":
		return "HK"
	case "taiwan", "台湾":
		return "TW"
	case "united states", "usa", "us", "美国":
		return "US"
	case "united kingdom", "uk", "great britain", "英国":
		return "GB"
	case "japan", "日本":
		return "JP"
	case "south korea", "korea", "韩国":
		return "KR"
	case "singapore", "新加坡":
		return "SG"
	case "germany", "德国":
		return "DE"
	case "france", "法国":
		return "FR"
	case "netherlands", "荷兰":
		return "NL"
	case "canada", "加拿大":
		return "CA"
	case "australia", "澳大利亚":
		return "AU"
	case "russia", "俄罗斯":
		return "RU"
	case "brazil", "巴西":
		return "BR"
	case "india", "印度":
		return "IN"
	default:
		return upper
	}
}

func defaultProxyLocationOptions() []ProxyLocationOption {
	return []ProxyLocationOption{
		countryLocaleDefaults["US"],
		countryLocaleDefaults["GB"],
		countryLocaleDefaults["JP"],
		countryLocaleDefaults["SG"],
		countryLocaleDefaults["CN"],
	}
}
