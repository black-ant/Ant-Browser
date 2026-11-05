package browser

import "strings"

var effectiveRuntimeFingerprintArgs = []string{
	"--disable-non-proxied-udp",
	"--fingerprinting-canvas-image-data-noise",
	"--fingerprinting-client-rects-noise",
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
