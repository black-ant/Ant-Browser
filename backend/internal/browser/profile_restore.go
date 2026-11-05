package browser

import "strings"

const (
	RestoreLastSessionFollow   = ""
	RestoreLastSessionEnabled  = "enabled"
	RestoreLastSessionDisabled = "disabled"
)

func NormalizeRestoreLastSessionMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "follow", "default", "inherit":
		return RestoreLastSessionFollow
	case "on", "true", "enabled", "enable":
		return RestoreLastSessionEnabled
	case "off", "false", "disabled", "disable":
		return RestoreLastSessionDisabled
	default:
		return RestoreLastSessionFollow
	}
}
