package identity

import (
	"strconv"
	"strings"
)

// FromLaunchArgs 从既有的 fingerprint_args(Chromium 命令行 flag 数组)反解出结构化身份。
// 用于把只存了 flag 的老环境平滑迁移到结构化身份模型。未识别的 flag 忽略。
//
// 注意:screen/deviceMemory/geo 无法从 flag 反解(它们由 seed 派生或经 CDP 注入),
// 因此这些字段保持零值,后续可由采样/对齐补齐。
func FromLaunchArgs(args []string) Identity {
	var id Identity
	for _, a := range args {
		key, val := splitFlag(a)
		switch key {
		case "--fingerprint":
			if n, err := strconv.ParseInt(val, 10, 64); err == nil {
				id.Seed = n
			}
		case "--fingerprint-platform":
			id.Platform = val
		case "--fingerprint-platform-version":
			id.PlatformVersion = val
		case "--fingerprint-brand":
			id.BrowserBrand = val
		case "--fingerprint-brand-version":
			id.BrandVersion = val
		case "--user-agent":
			id.UAFull = val
		case "--fingerprint-hardware-concurrency":
			if n, err := strconv.Atoi(val); err == nil {
				id.HardwareConcurrency = n
			}
		case "--window-size":
			id.WindowSize = val
		case "--lang":
			id.Locale = val
		case "--accept-lang":
			if val != "" {
				id.Languages = strings.Split(val, ",")
			}
		case "--timezone":
			id.Timezone = val
		case "--fingerprinting-canvas-image-data-noise":
			id.CanvasNoise = true
		case "--fingerprinting-client-rects-noise":
			id.ClientRectsNoise = true
		case "--disable-non-proxied-udp":
			id.WebRTCPolicy = "disable_non_proxied_udp"
		}
	}
	return id
}

// splitFlag 把 "--key=value" 拆成 (key, value);无 "=" 时 value 为空。
func splitFlag(arg string) (string, string) {
	if i := strings.IndexByte(arg, '='); i >= 0 {
		return arg[:i], arg[i+1:]
	}
	return arg, ""
}
