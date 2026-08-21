package browser

import "strings"

// 红线守卫:把"永不允许出现在托管参数集里的开关"编码成可执行检查。
//
// 分四类:
//   ① 改 JS 可见指纹面(检测脚本一读便知)
//   ② 改渲染 / 播放行为面(破坏直播可播性或产生行为异常特征)
//   ③ 打断 CDP(项目依赖 CDP 做保活、窗口平铺、Cookie 管理、地理对齐、自动化)
//   ④ 安全性归零
//
// 完整推导见 docs/superpowers/plans/2026-08-20-live-multiopen-extreme-performance.md §1。

// ForbiddenArgPrefixes 返回禁止出现的启动参数前缀。
func ForbiddenArgPrefixes() []string {
	return []string{
		// ① 指纹面:指纹参数本体与 V8 堆上限(改 performance.memory.jsHeapSizeLimit)
		"--fingerprint", "--disable-spoofing", "--js-flags",

		// ① GPU / 渲染管线:改 Canvas / WebGL 输出
		"--disable-gpu", "--use-gl", "--use-angle", "--force-color-profile",
		"--disable-accelerated", "--disable-webgl", "--disable-3d-apis",
		"--disable-reading-from-canvas", "--enable-gpu-rasterization",
		"--disable-gpu-compositing", "--blink-settings",

		// ① JS 可见 API 存在性:关掉会让对应 navigator.* / API 直接消失
		"--disable-notifications", "--disable-plugins", "--disable-speech-api",
		"--disable-file-system", "--disable-databases", "--disable-local-storage",
		"--disable-shared-workers", "--disable-permissions-api",
		"--disable-presentation-api", "--disable-remote-fonts",

		// ① 网络指纹:TLS / HTTP 指纹面
		"--disable-quic", "--disable-http2",

		// ② 播放行为面
		"--autoplay-policy", "--disable-javascript",

		// ③ 打断 CDP / 稳定性
		"--no-startup-window", "--remote-debugging-pipe", "--single-process",
		"--in-process-gpu", "--enable-automation",

		// ④ 安全
		"--no-sandbox", "--disable-web-security",
	}
}

// ForbiddenFeatureNames 返回禁止出现在 --disable-features 里的 feature 名。
// 关掉它们会让对应 JS API 消失或改变 codec 能力查询结果;而这些服务闲置时本就不占资源,
// 属于"零收益、纯风险"。
func ForbiddenFeatureNames() []string {
	return []string{
		// 设备类 API 存在性
		"WebBluetooth", "WebUSB", "WebSerial", "WebNFC",
		// 推送 / 后台同步类 API 存在性
		"PushMessaging", "BackgroundFetch", "BackgroundSync", "PeriodicBackgroundSync",
		"Notifications",
		// 直播站依赖的媒体能力
		"WebAudio", "MediaSource", "EncryptedMedia",
		// codec 支持面:JS 可通过 mediaCapabilities / isTypeSupported / canPlayType 读到
		"PlatformHEVCDecoderSupport", "VaapiVideoDecoder", "Av1Decoder",
		// Privacy Sandbox / Client Hints:JS 可见
		"PrivacySandboxSettings4", "BrowsingTopics", "UserAgentClientHint",
	}
}

// AssertNoForbiddenArgs 返回 args 中命中红线的项;返回空切片表示通过。
// 同一条参数即使命中多个红线也只报一次。
func AssertNoForbiddenArgs(args []string) []string {
	prefixes := ForbiddenArgPrefixes()
	features := ForbiddenFeatureNames()
	var bad []string

	for _, arg := range args {
		trimmed := strings.TrimSpace(arg)
		if matchesAnyPrefix(trimmed, prefixes) {
			bad = append(bad, arg)
			continue
		}
		if disablesForbiddenFeature(trimmed, features) {
			bad = append(bad, arg)
		}
	}
	return bad
}

func matchesAnyPrefix(arg string, prefixes []string) bool {
	lower := strings.ToLower(arg)
	for _, p := range prefixes {
		if strings.HasPrefix(lower, strings.ToLower(p)) {
			return true
		}
	}
	return false
}

func disablesForbiddenFeature(arg string, features []string) bool {
	const prefix = "--disable-features="
	if !strings.HasPrefix(strings.ToLower(arg), prefix) {
		return false
	}
	for _, value := range strings.Split(arg[len(prefix):], ",") {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		for _, f := range features {
			if strings.EqualFold(value, f) {
				return true
			}
		}
	}
	return false
}
