package browser

// MemorySaverArgs 返回“省内存模式”的 Chromium 启动参数。
//
// 设计目标:在**不破坏复杂电商与视频站**的前提下削减开销——保留图片/JS/媒体/GPU,
// 只做进程合并、关闭后台子系统、限制单实例渲染进程数与 V8 老生代上限。
//
// 实测(内核 148,经代理加载 amazon.com 与 youtube 视频页,均正常渲染):
// 对“单个重页面”的节省有限(页面内容本身占内存大头,单实例边际约 350–550MB);
// 主要收益在**长时导航、多标签与后台实例**。真正的规模杠杆是并发上限、后台实例挂起与轮换。
//
// 注意:不得加 --js-flags —— 它会改 performance.memory.jsHeapSizeLimit,属 JS 可见指纹面。
// 渲染进程内存控制改用进程数上限(--renderer-process-limit),见 live_perf.go。
func MemorySaverArgs() []string {
	return []string{
		"--process-per-site",              // 同站合并渲染进程(多标签省内存)
		"--renderer-process-limit=4",      // 限制单实例渲染进程数
		"--disable-back-forward-cache",    // 不缓存整页,长时导航不累积内存
		"--disable-background-networking", // 关闭后台网络
		"--disable-component-update",      // 关闭后台组件更新
		"--disable-breakpad",              // 关闭崩溃上报
		"--disable-features=Translate,MediaRouter,OptimizationHints,CalculateNativeWinOcclusion",
	}
}
