package backend

import (
	"ant-chrome/backend/internal/logger"
	"strings"
)

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
	{prefix: "--load-extension", takesValue: true},
	{prefix: "--disable-extensions-except", takesValue: true},
	{prefix: "--restore-last-session", takesValue: false},
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

func appendUniqueString(items []string, value string) []string {
	for _, item := range items {
		if strings.EqualFold(item, value) {
			return items
		}
	}
	return append(items, value)
}
