package identity

import "strings"

// Severity 校验问题的严重级别。
const (
	SeverityError   = "error"
	SeverityWarning = "warning"
)

// Issue 一条校验问题。
type Issue struct {
	Field    string `json:"field"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
	Fixable  bool   `json:"fixable"` // 能否一键修复(重新生成/重对齐)
}

// ValidationResult 校验结果。OK 为真表示没有 error 级问题(warning 可放行)。
type ValidationResult struct {
	OK     bool    `json:"ok"`
	Issues []Issue `json:"issues"`
}

// Validate 检查身份的内部自洽与代理对齐,返回问题列表。
// 存在任意 error 级问题时 OK=false(上线前应硬拦截并提供一键修复)。
func Validate(id Identity) ValidationResult {
	var issues []Issue
	add := func(field, msg, sev string, fixable bool) {
		issues = append(issues, Issue{Field: field, Message: msg, Severity: sev, Fixable: fixable})
	}

	// seed 必须存在。
	if id.Seed == 0 {
		add("seed", "缺少指纹 seed", SeverityError, true)
	}

	// 平台必须是受支持值。
	switch id.Platform {
	case "windows", "macos", "linux":
	default:
		add("platform", "平台取值非法(应为 windows/macos/linux)", SeverityError, true)
	}

	// 平台与 UA 必须一致。
	if id.UAFull != "" && !uaMatchesPlatform(id.Platform, id.UAFull) {
		add("ua", "User-Agent 与平台不一致", SeverityError, true)
	}

	// 屏幕尺寸必须合法。
	if id.Screen.Width <= 0 || id.Screen.Height <= 0 {
		add("screen", "屏幕尺寸非法", SeverityError, true)
	}

	// 绑定代理国家时:必须对齐时区,locale 语言应与国家匹配。
	if id.ProxyGeoSnapshot != "" {
		if def, known := countryDefaults[id.ProxyGeoSnapshot]; known {
			if id.Timezone == "" {
				add("timezone", "已绑定代理国家但未对齐时区", SeverityError, true)
			}
			if id.Locale != "" && !sameLanguage(id.Locale, def.Locale) {
				add("locale", "语言/locale 与代理国家不匹配", SeverityWarning, true)
			}
			if (id.Geo == Geo{}) {
				add("geo", "已绑定代理国家但未设置地理坐标", SeverityWarning, true)
			}
		}
	}

	ok := true
	for _, is := range issues {
		if is.Severity == SeverityError {
			ok = false
			break
		}
	}
	return ValidationResult{OK: ok, Issues: issues}
}

// ValidatePoolRecord 校验一条身份池模板的自洽性(比 Validate 更贴合模板:不查 seed/geo,
// 但强制 UA 版本 == brandVersion 版本,防止 UA 与 Client Hints 版本矛盾)。
func ValidatePoolRecord(rec PoolRecord) ValidationResult {
	var issues []Issue
	add := func(field, msg, sev string, fixable bool) {
		issues = append(issues, Issue{Field: field, Message: msg, Severity: sev, Fixable: fixable})
	}

	switch rec.Platform {
	case "windows", "macos", "linux":
	default:
		add("platform", "平台取值非法(应为 windows/macos/linux)", SeverityError, false)
	}

	if strings.TrimSpace(rec.UAFull) == "" {
		add("uaFull", "缺少 User-Agent", SeverityError, false)
	} else {
		if !uaMatchesPlatform(rec.Platform, rec.UAFull) {
			add("ua", "User-Agent 与平台不一致", SeverityError, false)
		}
		// UA 里的 Chrome 主版本必须与 brandVersion(Client Hints)主版本一致。
		uaMajor := chromeMajorFromUA(rec.UAFull)
		brandMajor := majorVersion(rec.BrandVersion)
		if uaMajor != "" && brandMajor != "" && uaMajor != brandMajor {
			add("brandVersion", "UA 版本("+uaMajor+")与品牌版本("+brandMajor+")不一致,会被 UA↔Client Hints 交叉检测", SeverityError, false)
		}
	}

	if strings.TrimSpace(rec.BrandVersion) == "" {
		add("brandVersion", "缺少品牌版本", SeverityError, false)
	}
	if rec.Screen.Width <= 0 || rec.Screen.Height <= 0 {
		add("screen", "屏幕尺寸非法", SeverityError, false)
	}
	if rec.HardwareConcurrency <= 0 {
		add("hardwareConcurrency", "CPU 核心数应大于 0", SeverityError, false)
	}
	if len(rec.Languages) == 0 {
		add("languages", "缺少语言列表", SeverityWarning, false)
	}
	if strings.TrimSpace(rec.Locale) == "" {
		add("locale", "缺少 locale", SeverityWarning, false)
	}
	if rec.Weight <= 0 {
		add("weight", "权重为 0,该记录永远不会被采样", SeverityWarning, false)
	}

	ok := true
	for _, is := range issues {
		if is.Severity == SeverityError {
			ok = false
			break
		}
	}
	return ValidationResult{OK: ok, Issues: issues}
}

// chromeMajorFromUA 从 UA 字符串抽取 Chrome 主版本号(如 "…Chrome/145.0.0.0…" → "145")。
func chromeMajorFromUA(ua string) string {
	i := strings.Index(ua, "Chrome/")
	if i < 0 {
		return ""
	}
	return majorVersion(ua[i+len("Chrome/"):])
}

// majorVersion 取版本串的主版本号("145.0.0.0" → "145")。
func majorVersion(v string) string {
	v = strings.TrimSpace(v)
	if i := strings.IndexByte(v, '.'); i >= 0 {
		return v[:i]
	}
	return v
}

// uaMatchesPlatform 判断 UA 是否包含与平台一致的 OS 标记。
func uaMatchesPlatform(platform, ua string) bool {
	switch platform {
	case "windows":
		return strings.Contains(ua, "Windows NT")
	case "macos":
		return strings.Contains(ua, "Mac OS X") || strings.Contains(ua, "Macintosh")
	case "linux":
		return strings.Contains(ua, "Linux") && !strings.Contains(ua, "Android")
	}
	return false
}

// sameLanguage 比较两个 locale 的主语言子标签是否相同(如 de-DE 与 de-AT → true)。
func sameLanguage(a, b string) bool {
	return primaryLang(a) == primaryLang(b)
}

func primaryLang(locale string) string {
	if i := strings.IndexAny(locale, "-_"); i > 0 {
		return strings.ToLower(locale[:i])
	}
	return strings.ToLower(locale)
}
