package backend

import (
	"fmt"
	"strings"
)

// Cloak（CloakBrowser）后端的指纹参数矩阵。
//
// 与 fingerprint-chromium 的关键差异在于"判断相反"，不只是参数多寡不同：
//   - GPU vendor / renderer、设备内存、屏幕宽高在 fingerprint-chromium 上实测无效、会被剔除，
//     但在 Cloak 上是受支持的参数，必须原样下发；
//   - Cloak 没有 --disable-spoofing，也没有独立的 canvas / clientRects 噪声开关，
//     噪声统一由 --fingerprint 种子驱动、用 --fingerprint-noise=false 整体关闭；
//   - 语言和时区走 --fingerprint-locale / --fingerprint-timezone。
//
// P1 的策略是"只增不减"：除了补齐种子，不从运行参数里剔除任何用户配置，
// 对本后端没有对应能力的参数只给出提示。这样即使这里的建模有偏差，
// 也不会静默丢掉用户已经配好的指纹参数。

// cloakSeedMin / cloakSeedMax 是 Cloak 文档描述的默认种子取值区间。
// 为保证种子一定被接受，后端补齐时收敛到这个区间。
const (
	cloakSeedMin = 10000
	cloakSeedMax = 99999
)

// cloakStableArgLabels 是 Cloak 已知支持的指纹参数。
var cloakStableArgLabels = map[string]string{
	"--fingerprint":                      "指纹种子",
	"--fingerprint-platform":             "平台",
	"--fingerprint-platform-version":     "系统版本",
	"--fingerprint-brand":                "浏览器品牌",
	"--fingerprint-brand-version":        "品牌版本",
	"--fingerprint-locale":               "语言",
	"--fingerprint-timezone":             "时区",
	"--fingerprint-hardware-concurrency": "CPU 核心数",
	"--fingerprint-device-memory":        "设备内存",
	"--fingerprint-screen-width":         "屏幕宽度",
	"--fingerprint-screen-height":        "屏幕高度",
	"--fingerprint-gpu-vendor":           "GPU Vendor",
	"--fingerprint-gpu-renderer":         "GPU Renderer",
	"--fingerprint-webrtc-ip":            "WebRTC 出口 IP",
	"--fingerprint-noise":                "噪声总开关",
	"--fingerprint-storage-quota":        "存储配额",
	"--fingerprint-taskbar-height":       "任务栏高度",
	"--fingerprint-windows-font-metrics": "Windows 字体度量",
	"--fingerprint-allow-3p-cookies":     "第三方 Cookie",
	"--fingerprint-sapi-voices":          "语音合成列表",
	"--license-through-proxy":            "License 走代理",
	"--window-size":                      "窗口大小",
	"--lang":                             "语言（Chromium 原生）",
	"--accept-lang":                      "Accept-Language（Chromium 原生）",
	"--disable-non-proxied-udp":          "WebRTC（Chromium 原生）",
	"--webrtc-ip-handling-policy":        "WebRTC（Chromium 原生）",
}

// cloakForeignArgLabels 是 fingerprint-chromium 专属、Cloak 没有对应实现的参数。
// 这些参数会原样传递（Chromium 会忽略未知开关），但需要明确告知用户不会生效。
var cloakForeignArgLabels = map[string]string{
	"--disable-spoofing":                        "排除伪装",
	"--fingerprinting-canvas-image-data-noise":  "Canvas ImageData",
	"--fingerprinting-client-rects-noise":       "ClientRects",
	"--fingerprinting-canvas-measuretext-noise": "Canvas MeasureText",
	"--fingerprint-canvas-noise":                "Canvas ImageData",
	"--fingerprint-client-rects-noise":          "ClientRects",
	"--fingerprint-audio-noise":                 "Audio",
	"--disable-gpu-fingerprint":                 "GPU 伪装开关",
	"--timezone":                                "时区",
	"--fingerprint-color-depth":                 "色深",
	"--fingerprint-touch-points":                "触控点",
	"--fingerprint-do-not-track":                "Do Not Track",
	"--fingerprint-media-devices":               "媒体设备",
	"--fingerprint-font-list":                   "字体",
	"--fingerprint-fonts":                       "字体",
	"--fingerprint-webgl-vendor":                "WebGL Vendor",
	"--fingerprint-webgl-renderer":              "WebGL Renderer",
	"--fingerprint-location":                    "地理位置",
	"--fingerprint-device-scale-factor":         "DPR",
}

// cloakForeignArgReplacement 给出 fingerprint-chromium 参数在 Cloak 下的推荐替代。
var cloakForeignArgReplacement = map[string]string{
	"--timezone":                               "--fingerprint-timezone",
	"--fingerprint-webgl-vendor":               "--fingerprint-gpu-vendor",
	"--fingerprint-webgl-renderer":             "--fingerprint-gpu-renderer",
	"--fingerprinting-canvas-image-data-noise": "--fingerprint 种子（或 --fingerprint-noise=false 整体关闭）",
	"--fingerprinting-client-rects-noise":      "--fingerprint 种子（或 --fingerprint-noise=false 整体关闭）",
	"--fingerprint-canvas-noise":               "--fingerprint 种子（或 --fingerprint-noise=false 整体关闭）",
	"--fingerprint-client-rects-noise":         "--fingerprint 种子（或 --fingerprint-noise=false 整体关闭）",
	"--fingerprint-audio-noise":                "--fingerprint 种子（或 --fingerprint-noise=false 整体关闭）",
	"--disable-spoofing":                       "--fingerprint-noise=false",
	"--disable-gpu-fingerprint":                "--fingerprint-gpu-vendor / --fingerprint-gpu-renderer",
}

// cloakSeedForProfileID 把稳定种子收敛到 Cloak 文档描述的取值区间。
func cloakSeedForProfileID(profileID string) int {
	seed := browserFingerprintSeedForProfileID(profileID)
	span := cloakSeedMax - cloakSeedMin + 1
	return cloakSeedMin + (seed % span)
}

// buildCloakFingerprintLaunchPlan 构建 Cloak 后端的运行参数与能力矩阵。
func buildCloakFingerprintLaunchPlan(profileId string, rawArgs []string, coreVersion string) browserFingerprintLaunchPlan {
	profileId = strings.TrimSpace(profileId)
	args := normalizeNonEmptyStrings(rawArgs)

	plan := browserFingerprintLaunchPlan{
		launchArgs: make([]string, 0, len(args)+1),
		rows:       make([]BrowserFingerprintCapabilityRow, 0, len(args)+1),
		warnings:   make([]string, 0, 2),
	}
	if strings.TrimSpace(coreVersion) == "" {
		plan.warnings = append(plan.warnings, "未识别 Cloak 内核版本，部分参数（如 --fingerprint-windows-font-metrics 需要 148+）无法校验适用性")
	}

	seedArg := browserArgWithKey(args, "--fingerprint")
	seedExplicit := seedArg != ""
	seedIsOff := seedExplicit && strings.EqualFold(strings.TrimSpace(browserArgValue(args, "--fingerprint")), "off")

	if !seedExplicit {
		if profileId != "" {
			injectedSeed := fmt.Sprintf("--fingerprint=%d", cloakSeedForProfileID(profileId))
			plan.launchArgs = append(plan.launchArgs, injectedSeed)
			plan.rows = append(plan.rows, BrowserFingerprintCapabilityRow{
				Capability: "指纹种子",
				Status:     "injected",
				RuntimeArg: injectedSeed,
				Action:     "后端补齐",
				Note:       fmt.Sprintf("配置未设置种子，按实例 ID 生成 %d-%d 区间内的稳定种子（Cloak 默认区间）", cloakSeedMin, cloakSeedMax),
			})
		} else {
			plan.rows = append(plan.rows, BrowserFingerprintCapabilityRow{
				Capability: "指纹种子",
				Status:     "pending",
				Action:     "启动时补齐",
				Note:       "创建态没有实例 ID，保存后启动会生成稳定种子",
			})
		}
	}

	for _, arg := range args {
		key := browserFingerprintArgKey(arg)
		if key == "" {
			plan.launchArgs = append(plan.launchArgs, arg)
			continue
		}

		// Cloak 下所有用户参数都原样下发，只有状态说明不同。
		plan.launchArgs = append(plan.launchArgs, arg)

		if key == "--fingerprint" {
			note := "Canvas / WebGL / Audio / ClientRects 噪声都由该种子派生"
			if seedIsOff {
				note = "--fingerprint=off 为直通调试模式，会暴露真实指纹值，请勿用于生产实例"
			}
			plan.rows = append(plan.rows, BrowserFingerprintCapabilityRow{
				Capability: "指纹种子",
				Status:     "kept",
				InputArg:   arg,
				RuntimeArg: arg,
				Action:     "保留",
				Note:       note,
			})
			continue
		}

		if label, ok := cloakForeignArgLabels[key]; ok {
			note := "该参数属于 fingerprint-chromium，Cloak 没有对应实现；已原样传递但预期不生效"
			if replacement, hasReplacement := cloakForeignArgReplacement[key]; hasReplacement {
				note = fmt.Sprintf("%s。Cloak 下请改用 %s", note, replacement)
			}
			plan.rows = append(plan.rows, BrowserFingerprintCapabilityRow{
				Capability: label,
				Status:     "foreign_backend",
				InputArg:   arg,
				RuntimeArg: arg,
				Action:     "原样传递",
				Note:       note,
			})
			continue
		}

		if label, ok := cloakStableArgLabels[key]; ok {
			plan.rows = append(plan.rows, BrowserFingerprintCapabilityRow{
				Capability: label,
				Status:     "kept",
				InputArg:   arg,
				RuntimeArg: arg,
				Action:     "保留",
				Note:       "Cloak 已知支持的指纹参数",
			})
			continue
		}

		plan.rows = append(plan.rows, BrowserFingerprintCapabilityRow{
			Capability: key,
			Status:     "kept_unknown",
			InputArg:   arg,
			RuntimeArg: arg,
			Action:     "保留",
			Note:       "后端未建模该参数，为避免误删按原样传递",
		})
	}

	if hasForeignBackendRow(plan.rows) {
		plan.warnings = append(plan.warnings, "存在 fingerprint-chromium 专属参数，在 Cloak 内核下预期不生效，建议按提示替换")
	}

	return plan
}

func hasForeignBackendRow(rows []BrowserFingerprintCapabilityRow) bool {
	for _, row := range rows {
		if row.Status == "foreign_backend" {
			return true
		}
	}
	return false
}
