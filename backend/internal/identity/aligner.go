package identity

// GeoInfo 是代理出口 IP 的地理归属(由离线 GeoIP 解析得到)。
type GeoInfo struct {
	CountryCode string  // ISO-3166 alpha-2,例如 "US"
	City        string  // 城市名(可空)
	Latitude    float64 // 纬度
	Longitude   float64 // 经度
	Timezone    string  // IANA 时区,例如 "America/New_York"(部分库可能为空)
	Accuracy    float64 // 定位精度(米)
}

// GeoResolver 把 IP 解析为地理信息;离线实现读取内嵌 mmdb。
// 定义在此便于地理对齐与代理绑定接入,真实实现见 geoip.go(构建期产物)。
type GeoResolver interface {
	Resolve(ip string) (GeoInfo, error)
}

// countryDefault 是国家默认地区设置,用于 GeoIP 缺时区时兜底,以及推导语言/locale。
type countryDefault struct {
	Timezone  string
	Locale    string
	Languages []string
}

// countryDefaults 覆盖常见国家/地区。时区取代表性时区(跨时区国家如 US 优先用 GeoIP 的实际时区)。
var countryDefaults = map[string]countryDefault{
	"US": {"America/New_York", "en-US", []string{"en-US", "en"}},
	"CA": {"America/Toronto", "en-CA", []string{"en-CA", "en", "fr-CA"}},
	"GB": {"Europe/London", "en-GB", []string{"en-GB", "en"}},
	"IE": {"Europe/Dublin", "en-IE", []string{"en-IE", "en"}},
	"AU": {"Australia/Sydney", "en-AU", []string{"en-AU", "en"}},
	"DE": {"Europe/Berlin", "de-DE", []string{"de-DE", "de", "en"}},
	"FR": {"Europe/Paris", "fr-FR", []string{"fr-FR", "fr", "en"}},
	"NL": {"Europe/Amsterdam", "nl-NL", []string{"nl-NL", "nl", "en"}},
	"ES": {"Europe/Madrid", "es-ES", []string{"es-ES", "es", "en"}},
	"IT": {"Europe/Rome", "it-IT", []string{"it-IT", "it", "en"}},
	"RU": {"Europe/Moscow", "ru-RU", []string{"ru-RU", "ru", "en"}},
	"JP": {"Asia/Tokyo", "ja-JP", []string{"ja-JP", "ja", "en"}},
	"KR": {"Asia/Seoul", "ko-KR", []string{"ko-KR", "ko", "en"}},
	"CN": {"Asia/Shanghai", "zh-CN", []string{"zh-CN", "zh", "en"}},
	"HK": {"Asia/Hong_Kong", "zh-HK", []string{"zh-HK", "zh", "en"}},
	"TW": {"Asia/Taipei", "zh-TW", []string{"zh-TW", "zh", "en"}},
	"SG": {"Asia/Singapore", "en-SG", []string{"en-SG", "en", "zh"}},
	"IN": {"Asia/Kolkata", "en-IN", []string{"en-IN", "en", "hi"}},
	"BR": {"America/Sao_Paulo", "pt-BR", []string{"pt-BR", "pt", "en"}},
	"VN": {"Asia/Ho_Chi_Minh", "vi-VN", []string{"vi-VN", "vi", "en"}},
	"TH": {"Asia/Bangkok", "th-TH", []string{"th-TH", "th", "en"}},
	"ID": {"Asia/Jakarta", "id-ID", []string{"id-ID", "id", "en"}},
}

// AlignToProxyGeo 返回一份把 时区/语言/locale/地理坐标 对齐到代理出口地理后的身份副本。
// 非地理字段(平台/UA/硬件/屏幕/seed 等)保持不变。
func AlignToProxyGeo(id Identity, geo GeoInfo) Identity {
	out := id
	def, known := countryDefaults[geo.CountryCode]

	// 时区:优先用 GeoIP 的实际时区(跨时区国家更准),否则回退国家兜底表。
	switch {
	case geo.Timezone != "":
		out.Timezone = geo.Timezone
	case known:
		out.Timezone = def.Timezone
	}

	// 语言/locale:按国家推导(已知国家),未知国家保留原值以免破坏自洽。
	if known {
		out.Locale = def.Locale
		out.Languages = append([]string(nil), def.Languages...)
	}

	// 地理坐标:直接采用代理出口坐标。
	out.Geo = Geo{Latitude: geo.Latitude, Longitude: geo.Longitude, Accuracy: geo.Accuracy}

	// 记录代理地理快照,便于漂移检测与审计。
	out.ProxyGeoSnapshot = geo.CountryCode
	return out
}
