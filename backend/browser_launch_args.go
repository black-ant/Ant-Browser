package backend

import (
	"ant-chrome/backend/internal/logger"
	"strings"
)

const searchEngineLaunchArgPrefix = "--ant-search-engine="

type managedLaunchArgSpec struct {
	prefix     string
	takesValue bool
}

var managedLaunchArgSpecs = []managedLaunchArgSpec{
	{prefix: "--user-data-dir", takesValue: true},
	{prefix: "--remote-debugging-port", takesValue: true},
	{prefix: "--remote-debugging-address", takesValue: true},
	{prefix: "--remote-debugging-pipe", takesValue: false},
	{prefix: "--proxy-server", takesValue: true},
	{prefix: "--enable-automation", takesValue: false},
	{prefix: "--disable-blink-features", takesValue: true},

	// 危险参数：会破坏沙箱、加载任意扩展/代码或关闭安全边界。
	// 一律由系统接管并剥离，避免外部自动化接口（profile 写入 / 一次性启动）
	// 注入这些参数后达到代码执行或沙箱逃逸的效果。
	// 注：--disable-features / --headless 等有正当用途的参数不在此列。
	{prefix: "--load-extension", takesValue: true},
	{prefix: "--disable-extensions-except", takesValue: true},
	{prefix: "--disable-web-security", takesValue: false},
	{prefix: "--no-sandbox", takesValue: false},
	{prefix: "--disable-setuid-sandbox", takesValue: false},
	{prefix: "--allow-running-insecure-content", takesValue: false},
	{prefix: "--disable-site-isolation-trials", takesValue: false},
	{prefix: "--remote-allow-origins", takesValue: true},
}

var blockedLaunchArgValues = map[string]map[string]bool{
	"--disable-blink-features": {
		"automationcontrolled": true,
	},
}

func sanitizeManagedLaunchArgs(args []string) ([]string, []string) {
	if len(args) == 0 {
		return nil, nil
	}

	sanitized := make([]string, 0, len(args))
	removed := make([]string, 0, 4)

	for i := 0; i < len(args); i++ {
		arg := strings.TrimSpace(args[i])
		if arg == "" {
			continue
		}

		spec, matched := matchManagedLaunchArg(arg)
		if !matched {
			sanitized = append(sanitized, arg)
			continue
		}

		if hasSelectiveBlockedValues(spec.prefix) {
			if strings.Contains(arg, "=") {
				value := strings.TrimSpace(arg[strings.Index(arg, "=")+1:])
				cleanedValue, changed := filterBlockedManagedLaunchArgValue(spec.prefix, value)
				if changed {
					removed = appendUniqueString(removed, spec.prefix)
					if cleanedValue != "" {
						sanitized = append(sanitized, spec.prefix+"="+cleanedValue)
					}
					continue
				}
				sanitized = append(sanitized, arg)
				continue
			}

			if spec.takesValue && i+1 < len(args) {
				next := strings.TrimSpace(args[i+1])
				if next != "" && !strings.HasPrefix(next, "-") {
					cleanedValue, changed := filterBlockedManagedLaunchArgValue(spec.prefix, next)
					if changed {
						removed = appendUniqueString(removed, spec.prefix)
						if cleanedValue != "" {
							sanitized = append(sanitized, spec.prefix+"="+cleanedValue)
						}
						i++
						continue
					}
					sanitized = append(sanitized, arg, next)
					i++
					continue
				}
			}
			sanitized = append(sanitized, arg)
			continue
		}

		removed = appendUniqueString(removed, spec.prefix)
		if spec.takesValue && !strings.Contains(arg, "=") && i+1 < len(args) {
			next := strings.TrimSpace(args[i+1])
			if next != "" && !strings.HasPrefix(next, "-") {
				i++
			}
		}
	}

	return sanitized, removed
}

func matchManagedLaunchArg(arg string) (managedLaunchArgSpec, bool) {
	for _, spec := range managedLaunchArgSpecs {
		if strings.EqualFold(arg, spec.prefix) || strings.HasPrefix(strings.ToLower(arg), strings.ToLower(spec.prefix)+"=") {
			return spec, true
		}
	}
	return managedLaunchArgSpec{}, false
}

func hasSelectiveBlockedValues(prefix string) bool {
	_, ok := blockedLaunchArgValues[strings.ToLower(prefix)]
	return ok
}

func filterBlockedManagedLaunchArgValue(prefix string, value string) (string, bool) {
	blockedValues, ok := blockedLaunchArgValues[strings.ToLower(prefix)]
	if !ok {
		return strings.TrimSpace(value), false
	}
	parts := strings.Split(value, ",")
	kept := make([]string, 0, len(parts))
	changed := false
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		if blockedValues[strings.ToLower(trimmed)] {
			changed = true
			continue
		}
		kept = append(kept, trimmed)
	}
	return strings.Join(kept, ","), changed
}

func logManagedLaunchArgOverrides(log *logger.Logger, profileId string, source string, managedArgs []string) {
	if log == nil || len(managedArgs) == 0 {
		return
	}
	log.Warn("忽略由系统接管的浏览器启动参数",
		logger.F("profile_id", profileId),
		logger.F("source", source),
		logger.F("managed_args", managedArgs),
	)
}

func extractSearchEngineLaunchArg(args []string) ([]string, string) {
	if len(args) == 0 {
		return nil, ""
	}

	cleaned := make([]string, 0, len(args))
	searchEngine := ""
	for _, arg := range args {
		trimmed := strings.TrimSpace(arg)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(trimmed), searchEngineLaunchArgPrefix) {
			value := strings.TrimSpace(trimmed[len(searchEngineLaunchArgPrefix):])
			if isSupportedSearchEngine(value) {
				searchEngine = strings.ToLower(value)
			}
			continue
		}
		cleaned = append(cleaned, trimmed)
	}
	return cleaned, searchEngine
}

func appendUniqueString(items []string, value string) []string {
	for _, item := range items {
		if strings.EqualFold(item, value) {
			return items
		}
	}
	return append(items, value)
}
