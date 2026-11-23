package browser

import (
	"strings"

	"ant-chrome/backend/internal/config"
)

var effectiveRuntimeFingerprintArgs = []string{
	"--disable-non-proxied-udp",
	"--fingerprinting-canvas-image-data-noise",
	"--fingerprinting-client-rects-noise",
}

// cloakIncompatibleDefaultArgKeys 是全局默认指纹参数里 Cloak 不支持的键。
// 只用于给新实例挑默认值，不影响用户已保存的配置。
var cloakIncompatibleDefaultArgKeys = map[string]struct{}{
	"--fingerprinting-canvas-image-data-noise":  {},
	"--fingerprinting-client-rects-noise":       {},
	"--fingerprinting-canvas-measuretext-noise": {},
	"--fingerprint-canvas-noise":                {},
	"--fingerprint-client-rects-noise":          {},
	"--fingerprint-audio-noise":                 {},
	"--disable-spoofing":                        {},
	"--disable-gpu-fingerprint":                 {},
	"--timezone":                                {},
}

// defaultFingerprintArgsForCore 返回适用于指定内核的默认指纹参数。
func (m *Manager) defaultFingerprintArgsForCore(coreId string) []string {
	if m == nil || m.Config == nil {
		return nil
	}
	defaults := m.Config.Browser.DefaultFingerprintArgs
	if m.profileCoreBackend(coreId) != config.CoreBackendCloak {
		return append([]string{}, defaults...)
	}
	filtered := make([]string, 0, len(defaults))
	for _, arg := range defaults {
		if _, incompatible := cloakIncompatibleDefaultArgKeys[fingerprintDefaultArgKey(arg)]; incompatible {
			continue
		}
		filtered = append(filtered, arg)
	}
	return filtered
}

func fingerprintDefaultArgKey(arg string) string {
	arg = strings.TrimSpace(arg)
	if index := strings.Index(arg, "="); index > 0 {
		return strings.ToLower(arg[:index])
	}
	return strings.ToLower(arg)
}

// upgradeLegacyMinimalFingerprintArgsForProfile 只对 fingerprint-chromium 后端补齐历史默认参数。
// Cloak 没有这些独立噪声开关（噪声由 --fingerprint 种子驱动），补进去只会产生无效参数。
func (m *Manager) upgradeLegacyMinimalFingerprintArgsForProfile(coreId string, args []string) []string {
	if m.profileCoreBackend(coreId) == config.CoreBackendCloak {
		return append([]string{}, args...)
	}
	return upgradeLegacyMinimalFingerprintArgs(args)
}

// profileCoreBackend 返回实例实际使用内核的后端标记。
func (m *Manager) profileCoreBackend(coreId string) string {
	if m == nil {
		return config.CoreBackendFingerprintChromium
	}
	if coreId := normalizeProfileCoreID(coreId); coreId != "" {
		if core, ok := m.GetCore(coreId); ok {
			return config.NormalizeCoreBackend(core.CoreBackend)
		}
	}
	if core, ok := m.GetDefaultCore(); ok {
		return config.NormalizeCoreBackend(core.CoreBackend)
	}
	return config.CoreBackendFingerprintChromium
}

func upgradeLegacyMinimalFingerprintArgs(args []string) []string {
	if !isLegacyMinimalFingerprintArgs(args) {
		return append([]string{}, args...)
	}
	out := append([]string{}, args...)
	for _, defaultArg := range effectiveRuntimeFingerprintArgs {
		if !fingerprintArgContains(out, defaultArg) {
			out = append(out, defaultArg)
		}
	}
	return out
}

func isLegacyMinimalFingerprintArgs(args []string) bool {
	if len(args) != 2 {
		return false
	}
	hasBrand := false
	hasPlatform := false
	for _, arg := range args {
		trimmed := strings.TrimSpace(arg)
		if strings.HasPrefix(trimmed, "--fingerprint-brand=") {
			hasBrand = true
		}
		if strings.HasPrefix(trimmed, "--fingerprint-platform=") {
			hasPlatform = true
		}
	}
	return hasBrand && hasPlatform
}

func fingerprintArgContains(args []string, expected string) bool {
	for _, arg := range args {
		if strings.TrimSpace(arg) == expected {
			return true
		}
	}
	return false
}
