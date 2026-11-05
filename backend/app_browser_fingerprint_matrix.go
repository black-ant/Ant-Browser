package backend

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"unicode"
)

const fingerprintChrome144Major = 144

type BrowserFingerprintCapabilityReport struct {
	ProfileId     string                            `json:"profileId"`
	CoreId        string                            `json:"coreId"`
	CoreName      string                            `json:"coreName"`
	ChromeVersion string                            `json:"chromeVersion"`
	ChromeMajor   int                               `json:"chromeMajor"`
	VersionStatus string                            `json:"versionStatus"`
	RawArgs       []string                          `json:"rawArgs"`
	LaunchArgs    []string                          `json:"launchArgs"`
	Rows          []BrowserFingerprintCapabilityRow `json:"rows"`
	Warnings      []string                          `json:"warnings"`
}

type BrowserFingerprintCapabilityRow struct {
	Capability string `json:"capability"`
	Status     string `json:"status"`
	InputArg   string `json:"inputArg"`
	RuntimeArg string `json:"runtimeArg"`
	Action     string `json:"action"`
	Note       string `json:"note"`
}

type browserFingerprintLaunchPlan struct {
	launchArgs []string
	rows       []BrowserFingerprintCapabilityRow
	warnings   []string
}

var fingerprintStableArgLabels = map[string]string{
	"--fingerprint":                            "指纹种子",
	"--fingerprint-brand":                      "浏览器品牌",
	"--fingerprint-brand-version":              "品牌版本",
	"--fingerprint-platform":                   "平台",
	"--fingerprint-platform-version":           "系统版本",
	"--fingerprint-hardware-concurrency":       "CPU 核心数",
	"--fingerprinting-canvas-image-data-noise": "Canvas ImageData",
	"--fingerprinting-client-rects-noise":      "ClientRects",
	"--lang":                                   "语言",
	"--accept-lang":                            "Accept-Language",
	"--timezone":                               "时区",
	"--window-size":                            "窗口大小",
	"--disable-non-proxied-udp":                "WebRTC",
	"--webrtc-ip-handling-policy":              "WebRTC",
	"--disable-spoofing":                       "排除伪装",
}

var fingerprintNoEffectArgLabels = map[string]string{
	"--fingerprint-color-depth":                 "色深",
	"--fingerprint-device-memory":               "设备内存",
	"--fingerprint-touch-points":                "触控点",
	"--fingerprint-do-not-track":                "Do Not Track",
	"--fingerprint-media-devices":               "媒体设备",
	"--fingerprint-audio-noise":                 "Audio",
	"--fingerprint-font-list":                   "字体",
	"--fingerprint-fonts":                       "字体",
	"--fingerprint-webgl-vendor":                "WebGL Vendor",
	"--fingerprint-webgl-renderer":              "WebGL Renderer",
	"--fingerprint-screen-width":                "屏幕宽度",
	"--fingerprint-screen-height":               "屏幕高度",
	"--fingerprint-device-scale-factor":         "DPR",
	"--fingerprint-location":                    "地理位置",
	"--fingerprinting-canvas-measuretext-noise": "Canvas MeasureText",
}

var fingerprintChrome144ConvertedArgLabels = map[string]struct {
	label      string
	runtimeArg string
}{
	"--fingerprint-canvas-noise":       {label: "Canvas ImageData", runtimeArg: "--fingerprinting-canvas-image-data-noise"},
	"--fingerprint-client-rects-noise": {label: "ClientRects", runtimeArg: "--fingerprinting-client-rects-noise"},
}

var fingerprintEffectiveNoiseArgLabels = map[string]string{
	"--fingerprinting-canvas-image-data-noise": "Canvas ImageData",
	"--fingerprinting-client-rects-noise":      "ClientRects",
}

func fingerprintNoiseRuntimeArgForKey(key string) (string, string, bool) {
	key = strings.ToLower(strings.TrimSpace(key))
	if converted, ok := fingerprintChrome144ConvertedArgLabels[key]; ok {
		return converted.runtimeArg, converted.label, true
	}
	if label, ok := fingerprintEffectiveNoiseArgLabels[key]; ok {
		return key, label, true
	}
	return "", "", false
}

func latestFingerprintNoiseArgIndexes(args []string) map[string]int {
	indexes := make(map[string]int)
	for index, arg := range args {
		key := browserFingerprintArgKey(arg)
		if runtimeArg, _, ok := fingerprintNoiseRuntimeArgForKey(key); ok {
			indexes[runtimeArg] = index
		}
	}
	return indexes
}

var fingerprintChrome144RemovedArgLabels = map[string]string{
	"--fingerprint-gpu-vendor":   "GPU Vendor",
	"--fingerprint-gpu-renderer": "GPU Renderer",
}

func (a *App) BrowserProfileFingerprintMatrix(profileId string, coreId string, fingerprintArgs []string) BrowserFingerprintCapabilityReport {
	return a.buildBrowserFingerprintCapabilityReport(profileId, coreId, fingerprintArgs)
}

func (a *App) buildBrowserFingerprintCapabilityReport(profileId string, coreId string, fingerprintArgs []string) BrowserFingerprintCapabilityReport {
	profileId = strings.TrimSpace(profileId)
	coreId = strings.TrimSpace(coreId)
	if coreId == "" && profileId != "" && a != nil && a.browserMgr != nil {
		if profile, ok := a.browserMgr.Profiles[profileId]; ok && profile != nil {
			coreId = strings.TrimSpace(profile.CoreId)
		}
	}

	var core BrowserCore
	var coreFound bool
	if a != nil && a.browserMgr != nil {
		core, coreFound = a.resolveBrowserCoreForFingerprintReport(coreId)
	}

	chromeVersion := ""
	if coreFound {
		chromeVersion = a.browserMgr.GetChromeVersion(core.CorePath)
	}

	plan := buildBrowserFingerprintLaunchPlan(profileId, fingerprintArgs, chromeVersion)
	report := BrowserFingerprintCapabilityReport{
		ProfileId:     profileId,
		ChromeVersion: chromeVersion,
		ChromeMajor:   parseChromeMajor(chromeVersion),
		VersionStatus: fingerprintVersionStatus(chromeVersion),
		RawArgs:       normalizeNonEmptyStrings(fingerprintArgs),
		LaunchArgs:    append([]string{}, plan.launchArgs...),
		Rows:          append([]BrowserFingerprintCapabilityRow{}, plan.rows...),
		Warnings:      append([]string{}, plan.warnings...),
	}
	if coreFound {
		report.CoreId = core.CoreId
		report.CoreName = core.CoreName
	} else {
		report.Warnings = appendUniqueString(report.Warnings, "未找到可用内核，矩阵按未知版本保守展示")
	}
	return report
}

func (a *App) resolveBrowserCoreForFingerprintReport(coreId string) (BrowserCore, bool) {
	if a == nil || a.browserMgr == nil || a.browserMgr.Config == nil {
		return BrowserCore{}, false
	}
	coreId = strings.TrimSpace(coreId)
	if strings.EqualFold(coreId, "default") {
		coreId = ""
	}
	if coreId != "" {
		if core, ok := a.browserMgr.GetCore(coreId); ok {
			return core, true
		}
	}
	return a.browserMgr.GetDefaultCore()
}

func buildBrowserFingerprintLaunchPlan(profileId string, rawArgs []string, chromeVersion string) browserFingerprintLaunchPlan {
	return buildBrowserFingerprintLaunchPlanForHost(profileId, rawArgs, chromeVersion, runtime.GOOS)
}

func buildBrowserFingerprintLaunchPlanForHost(profileId string, rawArgs []string, chromeVersion string, hostPlatform string) browserFingerprintLaunchPlan {
	profileId = strings.TrimSpace(profileId)
	originalArgs := normalizeNonEmptyStrings(rawArgs)
	args := normalizeBrowserLanguageArgs(originalArgs)
	hostPlatform = normalizeFingerprintPlatform(hostPlatform)
	major := parseChromeMajor(chromeVersion)
	versionStatus := fingerprintVersionStatus(chromeVersion)
	chrome144Plus := major >= fingerprintChrome144Major
	unknownVersion := versionStatus == "unknown"

	plan := browserFingerprintLaunchPlan{
		launchArgs: make([]string, 0, len(args)+2),
		rows:       make([]BrowserFingerprintCapabilityRow, 0, len(args)+2),
		warnings:   make([]string, 0, 2),
	}
	if unknownVersion {
		plan.warnings = append(plan.warnings, "未识别内核版本，旧版指纹参数按保守模式保留")
	}

	if !browserArgsHaveFingerprintSeed(args) {
		if profileId != "" {
			seedArg := fmt.Sprintf("--fingerprint=%d", browserFingerprintSeedForProfileID(profileId))
			plan.launchArgs = append(plan.launchArgs, seedArg)
			plan.rows = append(plan.rows, BrowserFingerprintCapabilityRow{
				Capability: "指纹种子",
				Status:     "injected",
				RuntimeArg: seedArg,
				Action:     "后端补齐",
				Note:       "配置未显式设置种子，启动时按实例 ID 生成稳定种子",
			})
		} else {
			plan.rows = append(plan.rows, BrowserFingerprintCapabilityRow{
				Capability: "指纹种子",
				Status:     "pending",
				Action:     "启动时补齐",
				Note:       "创建态没有实例 ID，保存后启动会生成稳定种子",
			})
		}
	} else {
		seedArg := browserArgWithKey(args, "--fingerprint")
		plan.rows = append(plan.rows, BrowserFingerprintCapabilityRow{
			Capability: "指纹种子",
			Status:     "kept",
			InputArg:   seedArg,
			RuntimeArg: seedArg,
			Action:     "保留",
			Note:       "作为 144+ 随机指纹能力的统一入口",
		})
	}

	appendLanguageCompletionRows(&plan, originalArgs, args)

	disableSpoofingIndex := -1
	disableSpoofingValues := make([]string, 0, 5)
	convertedDisableGPU := false
	latestNoiseArgIndexes := latestFingerprintNoiseArgIndexes(args)

	for index, arg := range args {
		key := browserFingerprintArgKey(arg)
		if key == "" {
			plan.launchArgs = append(plan.launchArgs, arg)
			continue
		}
		if chrome144Plus {
			if runtimeArg, label, ok := fingerprintNoiseRuntimeArgForKey(key); ok && latestNoiseArgIndexes[runtimeArg] != index {
				plan.rows = append(plan.rows, BrowserFingerprintCapabilityRow{
					Capability: label,
					Status:     "overridden",
					InputArg:   arg,
					Action:     "被后续配置覆盖",
					Note:       "同一噪声能力存在后续参数，本项不进入运行参数",
				})
				continue
			}
		}
		if key == "--disable-gpu-fingerprint" {
			if chrome144Plus {
				convertedDisableGPU = true
				plan.rows = append(plan.rows, BrowserFingerprintCapabilityRow{
					Capability: "GPU 伪装开关",
					Status:     "converted",
					InputArg:   arg,
					RuntimeArg: "--disable-spoofing=gpu",
					Action:     "转换",
					Note:       "当前适配矩阵不再把旧开关作为独立运行参数，运行时合并到 --disable-spoofing",
				})
				continue
			}
			plan.launchArgs = append(plan.launchArgs, arg)
			plan.rows = append(plan.rows, BrowserFingerprintCapabilityRow{
				Capability: "GPU 伪装开关",
				Status:     legacyKeepStatus(unknownVersion),
				InputArg:   arg,
				RuntimeArg: arg,
				Action:     "保留",
				Note:       legacyKeepNote(unknownVersion),
			})
			continue
		}

		if key == "--disable-spoofing" {
			disableSpoofingValues = appendUniqueStringValues(disableSpoofingValues, splitCSV(browserArgValue([]string{arg}, key))...)
			runtimeArg := arg
			if len(disableSpoofingValues) > 0 {
				runtimeArg = "--disable-spoofing=" + strings.Join(disableSpoofingValues, ",")
			}
			if disableSpoofingIndex < 0 {
				disableSpoofingIndex = len(plan.launchArgs)
				plan.launchArgs = append(plan.launchArgs, runtimeArg)
			} else {
				plan.launchArgs[disableSpoofingIndex] = runtimeArg
			}
			if !convertedDisableGPU {
				plan.rows = append(plan.rows, BrowserFingerprintCapabilityRow{
					Capability: "排除伪装",
					Status:     "kept",
					InputArg:   arg,
					RuntimeArg: runtimeArg,
					Action:     "保留",
					Note:       "由内核按指定能力关闭伪装",
				})
			}
			continue
		}

		if converted, ok := fingerprintChrome144ConvertedArgLabels[key]; ok {
			if chrome144Plus && browserFingerprintArgEnabled(arg) {
				plan.launchArgs = append(plan.launchArgs, converted.runtimeArg)
				plan.rows = append(plan.rows, BrowserFingerprintCapabilityRow{
					Capability: converted.label,
					Status:     "converted",
					InputArg:   arg,
					RuntimeArg: converted.runtimeArg,
					Action:     "转换",
					Note:       "Chrome 144+ 实测使用该噪声开关，运行时转换旧面板参数",
				})
				continue
			}
			if chrome144Plus {
				plan.rows = append(plan.rows, BrowserFingerprintCapabilityRow{
					Capability: converted.label,
					Status:     "disabled",
					InputArg:   arg,
					Action:     "运行时移除",
					Note:       "旧面板参数显式关闭，当前适配矩阵不把关闭值作为运行参数传递",
				})
				continue
			}
			plan.launchArgs = append(plan.launchArgs, arg)
			note := legacyKeepNote(unknownVersion)
			plan.rows = append(plan.rows, BrowserFingerprintCapabilityRow{
				Capability: converted.label,
				Status:     legacyKeepStatus(unknownVersion || chrome144Plus),
				InputArg:   arg,
				RuntimeArg: arg,
				Action:     "保守保留",
				Note:       note,
			})
			continue
		}

		if label, ok := fingerprintChrome144RemovedArgLabels[key]; ok {
			if chrome144Plus {
				plan.rows = append(plan.rows, BrowserFingerprintCapabilityRow{
					Capability: label,
					Status:     "removed",
					InputArg:   arg,
					Action:     "运行时清理",
					Note:       "当前适配矩阵不再把该旧 GPU 参数作为独立运行参数，GPU 指纹按 --fingerprint 种子处理",
				})
				continue
			}
			plan.launchArgs = append(plan.launchArgs, arg)
			plan.rows = append(plan.rows, BrowserFingerprintCapabilityRow{
				Capability: label,
				Status:     legacyKeepStatus(unknownVersion),
				InputArg:   arg,
				RuntimeArg: arg,
				Action:     "保留",
				Note:       legacyKeepNote(unknownVersion),
			})
			continue
		}

		if label, ok := fingerprintNoEffectArgLabels[key]; ok {
			if chrome144Plus {
				plan.rows = append(plan.rows, BrowserFingerprintCapabilityRow{
					Capability: label,
					Status:     "not_effective",
					InputArg:   arg,
					Action:     "运行时移除",
					Note:       "本地 Chrom-144 实测该参数未改变对应 JS 指纹值；不再作为运行参数或期望值传递",
				})
				continue
			}
			plan.launchArgs = append(plan.launchArgs, arg)
			plan.rows = append(plan.rows, BrowserFingerprintCapabilityRow{
				Capability: label,
				Status:     legacyKeepStatus(unknownVersion),
				InputArg:   arg,
				RuntimeArg: arg,
				Action:     "保守保留",
				Note:       legacyKeepNote(unknownVersion),
			})
			continue
		}

		if label, ok := fingerprintStableArgLabels[key]; ok {
			if noiseLabel, isNoiseArg := fingerprintEffectiveNoiseArgLabels[key]; chrome144Plus && isNoiseArg && !browserFingerprintArgEnabled(arg) {
				plan.rows = append(plan.rows, BrowserFingerprintCapabilityRow{
					Capability: noiseLabel,
					Status:     "disabled",
					InputArg:   arg,
					Action:     "运行时移除",
					Note:       "新版噪声参数显式关闭，当前适配矩阵不把关闭值作为运行参数传递",
				})
				continue
			}
			plan.launchArgs = append(plan.launchArgs, arg)
			if key != "--fingerprint" && key != "--accept-lang" {
				plan.rows = append(plan.rows, BrowserFingerprintCapabilityRow{
					Capability: label,
					Status:     "kept",
					InputArg:   arg,
					RuntimeArg: arg,
					Action:     "保留",
					Note:       "当前版本策略下作为稳定参数传递给内核",
				})
			}
		} else if _, modeled := fingerprintStableArgLabels[key]; !modeled {
			plan.launchArgs = append(plan.launchArgs, arg)
			plan.rows = append(plan.rows, BrowserFingerprintCapabilityRow{
				Capability: key,
				Status:     "kept_unknown",
				InputArg:   arg,
				RuntimeArg: arg,
				Action:     "保留",
				Note:       "后端未建模该参数，为避免误删按原样传递",
			})
		}
	}

	if convertedDisableGPU {
		disableSpoofingValues = appendUniqueStringValues(disableSpoofingValues, "gpu")
		runtimeArg := "--disable-spoofing=" + strings.Join(disableSpoofingValues, ",")
		if disableSpoofingIndex >= 0 {
			plan.launchArgs[disableSpoofingIndex] = runtimeArg
		} else {
			disableSpoofingIndex = len(plan.launchArgs)
			plan.launchArgs = append(plan.launchArgs, runtimeArg)
		}
	}

	targetPlatform, targetPlatformArg := fingerprintPlatformFromArgs(args)
	if targetPlatform != "" && hostPlatform != "" && targetPlatform != hostPlatform && !hasStringValue(disableSpoofingValues, "font") {
		disableSpoofingValues = appendUniqueStringValues(disableSpoofingValues, "font")
		runtimeArg := "--disable-spoofing=" + strings.Join(disableSpoofingValues, ",")
		if disableSpoofingIndex >= 0 {
			plan.launchArgs[disableSpoofingIndex] = runtimeArg
		} else {
			disableSpoofingIndex = len(plan.launchArgs)
			plan.launchArgs = append(plan.launchArgs, runtimeArg)
		}
		plan.rows = append(plan.rows, BrowserFingerprintCapabilityRow{
			Capability: "跨平台字体渲染",
			Status:     "injected",
			InputArg:   targetPlatformArg,
			RuntimeArg: runtimeArg,
			Action:     "后端补齐",
			Note:       fmt.Sprintf("目标平台 %s 与宿主平台 %s 不同；关闭字体伪装并使用宿主真实字体，避免日文、中文、韩文缺字", targetPlatform, hostPlatform),
		})
	}

	if disableSpoofingIndex >= 0 {
		updateDisableSpoofingRows(&plan, plan.launchArgs[disableSpoofingIndex])
	}

	return plan
}

func fingerprintPlatformFromArgs(args []string) (string, string) {
	platformArg := browserArgWithKey(args, "--fingerprint-platform")
	if platformArg == "" {
		return "", ""
	}
	separator := strings.Index(platformArg, "=")
	if separator < 0 {
		return "", platformArg
	}
	return normalizeFingerprintPlatform(platformArg[separator+1:]), platformArg
}

func normalizeFingerprintPlatform(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "windows", "win":
		return "windows"
	case "darwin", "mac", "macos":
		return "macos"
	case "linux":
		return "linux"
	default:
		return ""
	}
}

func hasStringValue(values []string, want string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(want)) {
			return true
		}
	}
	return false
}

func updateDisableSpoofingRows(plan *browserFingerprintLaunchPlan, runtimeArg string) {
	if plan == nil {
		return
	}
	for index := range plan.rows {
		if plan.rows[index].Capability == "排除伪装" {
			plan.rows[index].RuntimeArg = runtimeArg
		}
	}
}

func appendLanguageCompletionRows(plan *browserFingerprintLaunchPlan, originalArgs []string, normalizedArgs []string) {
	if plan == nil {
		return
	}
	originalLang := browserArgValue(originalArgs, browserLangArg)
	originalAcceptLang := browserArgValue(originalArgs, browserAcceptLangArg)
	normalizedLang := browserArgValue(normalizedArgs, browserLangArg)
	normalizedAcceptLang := browserArgValue(normalizedArgs, browserAcceptLangArg)
	if originalLang != "" && originalAcceptLang == "" && normalizedAcceptLang != "" {
		runtimeArg := fmt.Sprintf("%s=%s", browserAcceptLangArg, normalizedAcceptLang)
		plan.rows = append(plan.rows, BrowserFingerprintCapabilityRow{
			Capability: "Accept-Language",
			Status:     "inferred",
			InputArg:   fmt.Sprintf("%s=%s", browserLangArg, originalLang),
			RuntimeArg: runtimeArg,
			Action:     "后端补齐",
			Note:       "避免 navigator.language 与请求语言只配置一侧",
		})
	}
	if originalLang == "" && originalAcceptLang != "" && normalizedLang != "" {
		runtimeArg := fmt.Sprintf("%s=%s", browserLangArg, normalizedLang)
		plan.rows = append(plan.rows, BrowserFingerprintCapabilityRow{
			Capability: "语言",
			Status:     "inferred",
			InputArg:   fmt.Sprintf("%s=%s", browserAcceptLangArg, originalAcceptLang),
			RuntimeArg: runtimeArg,
			Action:     "后端补齐",
			Note:       "避免只设置 Accept-Language 造成语言指纹不完整",
		})
	}
}

func parseChromeMajor(version string) int {
	version = strings.TrimSpace(version)
	if version == "" {
		return 0
	}
	parts := strings.FieldsFunc(version, func(r rune) bool { return !unicode.IsDigit(r) })
	if len(parts) == 0 || parts[0] == "" {
		return 0
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil || major < 0 {
		return 0
	}
	return major
}

func fingerprintVersionStatus(version string) string {
	major := parseChromeMajor(version)
	if major <= 0 {
		return "unknown"
	}
	if major >= fingerprintChrome144Major {
		return "chrome144_plus"
	}
	return "legacy"
}

func browserFingerprintArgKey(arg string) string {
	arg = strings.TrimSpace(arg)
	if arg == "" || !strings.HasPrefix(arg, "--") {
		return ""
	}
	if index := strings.Index(arg, "="); index > 0 {
		return strings.ToLower(strings.TrimSpace(arg[:index]))
	}
	return strings.ToLower(arg)
}

func browserArgWithKey(args []string, key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	for index := len(args) - 1; index >= 0; index-- {
		arg := strings.TrimSpace(args[index])
		if browserFingerprintArgKey(arg) == key {
			return arg
		}
	}
	return ""
}

func browserFingerprintArgEnabled(arg string) bool {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return false
	}
	if index := strings.Index(arg, "="); index >= 0 {
		value := strings.ToLower(strings.TrimSpace(arg[index+1:]))
		return value == "1" || value == "true" || value == "yes" || value == "on"
	}
	return true
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func appendUniqueStringValues(values []string, additions ...string) []string {
	for _, addition := range additions {
		addition = strings.TrimSpace(addition)
		if addition == "" {
			continue
		}
		seen := false
		for _, value := range values {
			if strings.EqualFold(value, addition) {
				seen = true
				break
			}
		}
		if !seen {
			values = append(values, addition)
		}
	}
	return values
}

func legacyKeepStatus(unknownVersion bool) string {
	if unknownVersion {
		return "kept_unconfirmed"
	}
	return "kept_legacy"
}

func legacyKeepNote(unknownVersion bool) string {
	if unknownVersion {
		return "未识别内核版本，无法确认支持范围；为避免误删按原样传递"
	}
	return "144 之前的内核按旧参数兼容传递"
}
