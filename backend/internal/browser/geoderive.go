package browser

import (
	"encoding/json"
	"strconv"
	"strings"
)

// ProxyGeo 为从代理出口 IP 健康信息推导出的、用于与代理地区保持一致的指纹要素。
// 反检测的常见穿帮点是「时区 / 语言 ≠ 代理 IP 地理位置」，本结构用于在启动时按代理
// 出口地区自动补齐这些要素。无法可靠推导的字段保持零值。
type ProxyGeo struct {
	Timezone  string  // IANA 时区，如 "America/New_York"
	Language  string  // 主语言标签（BCP-47），如 "en-US"
	Lat       float64 // 纬度（用于后续 CDP 地理定位覆盖，可选）
	Lon       float64 // 经度
	HasLatLon bool
}

// DeriveProxyGeo 解析缓存的 IPPure 健康 JSON（ProxyIPHealthResult 序列化结果），
// 推导时区 / 语言 / 经纬度。优先采用第三方接口原始响应里的 timezone / 经纬度，
// 缺失时回退到「国家 → 时区 / 语言」映射。ok=false 表示完全无可用信息。
func DeriveProxyGeo(healthJSON string) (ProxyGeo, bool) {
	healthJSON = strings.TrimSpace(healthJSON)
	if healthJSON == "" {
		return ProxyGeo{}, false
	}

	var parsed struct {
		Ok      bool                   `json:"ok"`
		Country string                 `json:"country"`
		Region  string                 `json:"region"`
		RawData map[string]interface{} `json:"rawData"`
	}
	if err := json.Unmarshal([]byte(healthJSON), &parsed); err != nil {
		return ProxyGeo{}, false
	}

	geo := ProxyGeo{}

	// 1) 时区：优先用原始响应里的 timezone 字段，再回退国家（必要时省/州）映射
	tz := firstNonEmpty(
		rawString(parsed.RawData, "timezone"),
		rawString(parsed.RawData, "time_zone"),
		rawString(parsed.RawData, "tz"),
	)
	if !looksLikeIANATimezone(tz) {
		tz = countryToTimezone(parsed.Country, parsed.Region)
	}
	geo.Timezone = tz

	// 2) 语言：按国家映射主语言
	geo.Language = countryToLanguage(parsed.Country)

	// 3) 经纬度（可选，供后续 CDP 地理定位覆盖使用）
	if lat, lon, ok := rawLatLon(parsed.RawData); ok {
		geo.Lat, geo.Lon, geo.HasLatLon = lat, lon, true
	}

	if geo.Timezone == "" && geo.Language == "" && !geo.HasLatLon {
		return geo, false
	}
	return geo, true
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func rawString(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

// looksLikeIANATimezone 粗判是否像 IANA 时区标识（如 "Asia/Shanghai"），
// 用于决定能否直接采用第三方接口给出的 timezone 字段。
func looksLikeIANATimezone(tz string) bool {
	tz = strings.TrimSpace(tz)
	if tz == "" || !strings.Contains(tz, "/") || strings.Contains(tz, " ") {
		return false
	}
	return true
}

// rawLatLon 从原始响应中尽量提取经纬度，兼容 latitude/longitude、lat/lon(lng)、
// 以及 "lat,lon" 形式的 loc 字段。
func rawLatLon(m map[string]interface{}) (float64, float64, bool) {
	if m == nil {
		return 0, 0, false
	}
	lat, latOK := rawFloat(m, "latitude")
	if !latOK {
		lat, latOK = rawFloat(m, "lat")
	}
	lon, lonOK := rawFloat(m, "longitude")
	if !lonOK {
		lon, lonOK = rawFloat(m, "lon")
	}
	if !lonOK {
		lon, lonOK = rawFloat(m, "lng")
	}
	if latOK && lonOK {
		return lat, lon, true
	}

	// loc: "37.751,-97.822"
	if loc := rawString(m, "loc"); loc != "" {
		parts := strings.Split(loc, ",")
		if len(parts) == 2 {
			a, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
			b, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
			if err1 == nil && err2 == nil {
				return a, b, true
			}
		}
	}
	return 0, 0, false
}

func rawFloat(m map[string]interface{}, key string) (float64, bool) {
	v, ok := m[key]
	if !ok || v == nil {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(n), 64)
		if err == nil {
			return f, true
		}
	}
	return 0, false
}

// normalizeCountry 把国家字段归一化为 ISO 3166-1 alpha-2 大写代码。
// 兼容接口直接返回两位代码，或返回常见英文国家名的情况。
func normalizeCountry(country string) string {
	c := strings.ToUpper(strings.TrimSpace(country))
	if c == "" {
		return ""
	}
	if len(c) == 2 {
		if c == "UK" { // 常见非标准写法
			return "GB"
		}
		return c
	}
	if code, ok := countryNameToCode[strings.ToLower(strings.TrimSpace(country))]; ok {
		return code
	}
	return ""
}

// countryToTimezone 返回国家（美国按省/州细分）对应的代表性 IANA 时区。
// 未覆盖的国家返回空串——宁可不注入，也不要注入错误时区造成新的穿帮。
func countryToTimezone(country, region string) string {
	code := normalizeCountry(country)
	if code == "" {
		return ""
	}
	if code == "US" {
		if tz := usRegionToTimezone(region); tz != "" {
			return tz
		}
		return "America/New_York"
	}
	return countryCodeToTimezone[code]
}

// countryToLanguage 返回国家对应的主语言标签（BCP-47）。未覆盖返回空串。
func countryToLanguage(country string) string {
	code := normalizeCountry(country)
	if code == "" {
		return ""
	}
	return countryCodeToLanguage[code]
}

// usRegionToTimezone 按美国州名/缩写粗略映射到四大时区，覆盖常见州，
// 其余回退到东部时区（由调用方处理）。
func usRegionToTimezone(region string) string {
	r := strings.ToLower(strings.TrimSpace(region))
	if r == "" {
		return ""
	}
	if tz, ok := usRegionTZ[r]; ok {
		return tz
	}
	return ""
}

var countryCodeToTimezone = map[string]string{
	"CA": "America/Toronto",
	"GB": "Europe/London",
	"DE": "Europe/Berlin",
	"FR": "Europe/Paris",
	"NL": "Europe/Amsterdam",
	"ES": "Europe/Madrid",
	"IT": "Europe/Rome",
	"RU": "Europe/Moscow",
	"JP": "Asia/Tokyo",
	"KR": "Asia/Seoul",
	"CN": "Asia/Shanghai",
	"HK": "Asia/Hong_Kong",
	"TW": "Asia/Taipei",
	"SG": "Asia/Singapore",
	"IN": "Asia/Kolkata",
	"AU": "Australia/Sydney",
	"NZ": "Pacific/Auckland",
	"BR": "America/Sao_Paulo",
	"MX": "America/Mexico_City",
	"AR": "America/Argentina/Buenos_Aires",
	"ID": "Asia/Jakarta",
	"TH": "Asia/Bangkok",
	"VN": "Asia/Ho_Chi_Minh",
	"MY": "Asia/Kuala_Lumpur",
	"PH": "Asia/Manila",
	"TR": "Europe/Istanbul",
	"PL": "Europe/Warsaw",
	"SE": "Europe/Stockholm",
	"CH": "Europe/Zurich",
	"AT": "Europe/Vienna",
	"BE": "Europe/Brussels",
	"PT": "Europe/Lisbon",
	"IE": "Europe/Dublin",
	"NO": "Europe/Oslo",
	"DK": "Europe/Copenhagen",
	"FI": "Europe/Helsinki",
	"CZ": "Europe/Prague",
	"UA": "Europe/Kyiv",
	"RO": "Europe/Bucharest",
	"GR": "Europe/Athens",
	"AE": "Asia/Dubai",
	"SA": "Asia/Riyadh",
	"IL": "Asia/Jerusalem",
	"ZA": "Africa/Johannesburg",
	"EG": "Africa/Cairo",
}

var countryCodeToLanguage = map[string]string{
	"US": "en-US",
	"GB": "en-GB",
	"CA": "en-CA",
	"AU": "en-AU",
	"NZ": "en-NZ",
	"IE": "en-IE",
	"DE": "de-DE",
	"AT": "de-AT",
	"CH": "de-CH",
	"FR": "fr-FR",
	"BE": "fr-BE",
	"ES": "es-ES",
	"MX": "es-MX",
	"AR": "es-AR",
	"IT": "it-IT",
	"NL": "nl-NL",
	"RU": "ru-RU",
	"UA": "uk-UA",
	"JP": "ja-JP",
	"KR": "ko-KR",
	"CN": "zh-CN",
	"HK": "zh-HK",
	"TW": "zh-TW",
	"SG": "en-SG",
	"IN": "en-IN",
	"BR": "pt-BR",
	"PT": "pt-PT",
	"ID": "id-ID",
	"TH": "th-TH",
	"VN": "vi-VN",
	"MY": "ms-MY",
	"PH": "en-PH",
	"TR": "tr-TR",
	"PL": "pl-PL",
	"SE": "sv-SE",
	"NO": "nb-NO",
	"DK": "da-DK",
	"FI": "fi-FI",
	"CZ": "cs-CZ",
	"RO": "ro-RO",
	"GR": "el-GR",
	"AE": "ar-AE",
	"SA": "ar-SA",
	"IL": "he-IL",
	"ZA": "en-ZA",
	"EG": "ar-EG",
}

var countryNameToCode = map[string]string{
	"united states":            "US",
	"united states of america": "US",
	"usa":                      "US",
	"united kingdom":           "GB",
	"great britain":            "GB",
	"england":                  "GB",
	"canada":                   "CA",
	"germany":                  "DE",
	"france":                   "FR",
	"netherlands":              "NL",
	"spain":                    "ES",
	"italy":                    "IT",
	"russia":                   "RU",
	"russian federation":       "RU",
	"japan":                    "JP",
	"south korea":              "KR",
	"korea":                    "KR",
	"republic of korea":        "KR",
	"china":                    "CN",
	"hong kong":                "HK",
	"taiwan":                   "TW",
	"singapore":                "SG",
	"india":                    "IN",
	"australia":                "AU",
	"new zealand":              "NZ",
	"brazil":                   "BR",
	"mexico":                   "MX",
	"argentina":                "AR",
	"indonesia":                "ID",
	"thailand":                 "TH",
	"vietnam":                  "VN",
	"malaysia":                 "MY",
	"philippines":              "PH",
	"turkey":                   "TR",
	"poland":                   "PL",
	"sweden":                   "SE",
	"switzerland":              "CH",
	"austria":                  "AT",
	"belgium":                  "BE",
	"portugal":                 "PT",
	"ireland":                  "IE",
	"norway":                   "NO",
	"denmark":                  "DK",
	"finland":                  "FI",
	"czechia":                  "CZ",
	"czech republic":           "CZ",
	"ukraine":                  "UA",
	"romania":                  "RO",
	"greece":                   "GR",
	"united arab emirates":     "AE",
	"saudi arabia":             "SA",
	"israel":                   "IL",
	"south africa":             "ZA",
	"egypt":                    "EG",
}

var usRegionTZ = map[string]string{
	"california":     "America/Los_Angeles",
	"ca":             "America/Los_Angeles",
	"washington":     "America/Los_Angeles",
	"wa":             "America/Los_Angeles",
	"oregon":         "America/Los_Angeles",
	"or":             "America/Los_Angeles",
	"nevada":         "America/Los_Angeles",
	"nv":             "America/Los_Angeles",
	"arizona":        "America/Phoenix",
	"az":             "America/Phoenix",
	"colorado":       "America/Denver",
	"co":             "America/Denver",
	"utah":           "America/Denver",
	"ut":             "America/Denver",
	"new mexico":     "America/Denver",
	"nm":             "America/Denver",
	"montana":        "America/Denver",
	"mt":             "America/Denver",
	"texas":          "America/Chicago",
	"tx":             "America/Chicago",
	"illinois":       "America/Chicago",
	"il":             "America/Chicago",
	"missouri":       "America/Chicago",
	"mo":             "America/Chicago",
	"minnesota":      "America/Chicago",
	"mn":             "America/Chicago",
	"wisconsin":      "America/Chicago",
	"wi":             "America/Chicago",
	"louisiana":      "America/Chicago",
	"la":             "America/Chicago",
	"new york":       "America/New_York",
	"ny":             "America/New_York",
	"florida":        "America/New_York",
	"fl":             "America/New_York",
	"georgia":        "America/New_York",
	"ga":             "America/New_York",
	"virginia":       "America/New_York",
	"va":             "America/New_York",
	"pennsylvania":   "America/New_York",
	"pa":             "America/New_York",
	"massachusetts":  "America/New_York",
	"ma":             "America/New_York",
	"new jersey":     "America/New_York",
	"nj":             "America/New_York",
	"ohio":           "America/New_York",
	"oh":             "America/New_York",
	"michigan":       "America/New_York",
	"mi":             "America/New_York",
	"north carolina": "America/New_York",
	"nc":             "America/New_York",
}
