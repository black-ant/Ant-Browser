package browser

import "strings"

// livePerfDisabledFeatures 是直播性能模式关闭的 Chromium feature。
//
// 铁律:新增前必须过 AssertNoForbiddenArgs —— 任何"关掉会让 JS API 消失"或
// "改 codec 能力查询结果"的 feature 都不许进(见 forbidden_args.go)。
// feature 名以 148 内核为准;144 不认识的名字会被 Chromium 静默忽略(无害)。
var livePerfDisabledFeatures = []string{
	// 看播时长命脉:禁用原生遮挡检测,遮挡窗口保持 visibilityState=visible。
	// 这一条是"必须保持禁用",不是性能削减 —— 删了会导致多开时看播时长不计。
	"CalculateNativeWinOcclusion",

	// 每实例的"备用渲染进程",约 80-150MB,多开时纯浪费
	"SpareRendererForSitePerProcess",
	// 音频服务独立进程;挂机墙默认静音,并入主进程省一个进程
	"AudioServiceOutOfProcess",
	// 投屏 / Cast:服务线程 + 局域网 mDNS 组播底噪
	"MediaRouter", "DialMediaRouteProvider",

	// 后台预测 / 云端交互
	"OptimizationHints", "OptimizationGuideModelDownloading",
	"AutofillServerCommunication", "NetworkTimeServiceQuerying", "Prerender2",

	// 纯 UI 外围
	"Translate", "LiveCaption", "GlobalMediaControls", "HardwareMediaKeyHandling",
	"InterestFeedContentSuggestions", "ReadLater", "Journeys",
	"TabHoverCardImages", "ChromeWhatsNewUI",
}

// LivePerfArgs 返回"直播多开性能模式"(A 级)的 Chromium 启动参数。
//
// 设计边界:此列表只允许出现【JS 不可见、不改 Canvas/WebGL/UA/codec 能力查询、
// 不影响直播播放与 visibilityState、不打断 CDP】的开关。
// 新增参数前先跑 live_perf_test.go 的红线守卫。
//
// 不在此处做的事(有意为之):
//   - 不改 V8 堆上限(--js-flags 会改 performance.memory.jsHeapSizeLimit)
//   - 不动 GPU / 解码管线(改 Canvas/WebGL 指纹面)
//   - 不关任何 JS 可见 API(Notification / WebUSB / BackgroundSync 等,零收益纯风险)
func LivePerfArgs() []string {
	return []string{
		// ── 进程模型(进程数 JS 不可见)──
		"--process-per-site",         // 同站合并渲染进程
		"--renderer-process-limit=3", // 限制单实例渲染进程数

		// ── 整块砍掉的服务 / 常驻组件 ──
		"--disable-component-extensions-with-background-pages",
		"--disable-breakpad",
		"--disable-crash-reporter",
		"--noerrdialogs",
		"--disable-print-preview",
		"--no-service-autorun",
		"--disable-background-mode", // 关窗后不留后台驻留进程
		"--disable-default-apps",

		// ── 后台网络底噪(--disable-background-networking 是总闸)──
		"--disable-background-networking",
		"--disable-component-update",
		"--disable-domain-reliability",
		"--disable-client-side-phishing-detection",
		"--disable-search-engine-choice-screen",
		"--no-pings",
		"--metrics-recording-only",

		// ── 磁盘 / 日志 ──
		"--disable-logging",
		"--log-level=3",
		"--disable-back-forward-cache", // 单页直播无前进后退,不缓存整页
		"--disk-cache-size=134217728",  // 128MB
		"--media-cache-size=33554432",  // 32MB:直播分片复用价值≈0

		// ── UI 打扰 ──
		"--disable-hang-monitor", // 无人值守,不要"页面无响应"弹窗
		"--disable-prompt-on-repost",
		"--password-store=basic",

		// ── 为 CDP 稳定性而加(非削减)──
		// Chromium 会节流高频发 IPC 的渲染进程,playwright 批量操作时可能被限速。
		"--disable-ipc-flooding-protection",

		"--disable-features=" + strings.Join(livePerfDisabledFeatures, ","),
	}
}

// LivePerfAggressiveArgs 返回 B 级(激进)增量参数,默认不启用。
//
// --disable-site-isolation-trials:跨站 iframe(广告 / 埋点)并进同一渲染进程,
// 每实例再省 1-4 个进程。它【不改任何 JS 可见指纹面】(crossOriginIsolated 由
// COOP/COEP 决定,与站点隔离无关),真实代价是:
//
//	① 放弃站点隔离安全性(Spectre 类跨站内存读取)
//	② 同进程内广告 iframe 崩溃会带走整页
//
// 结论:只开固定直播平台的专用挂机墙可接受;什么站都开的通用环境不要开。
func LivePerfAggressiveArgs() []string {
	return []string{
		"--disable-site-isolation-trials",
	}
}
