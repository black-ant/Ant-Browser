package browser

import "strings"

const (
	ProxyModeFixed   = "fixed"
	ProxyModeDynamic = "dynamic"
	ProxyModeLocal   = "local"
)

func NormalizeProxyMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case ProxyModeDynamic:
		return ProxyModeDynamic
	case ProxyModeLocal:
		return ProxyModeLocal
	default:
		return ProxyModeFixed
	}
}

func NormalizeProxyGroupName(value string) string {
	return strings.TrimSpace(value)
}

func IsDynamicProxyMode(value string) bool {
	return NormalizeProxyMode(value) == ProxyModeDynamic
}
