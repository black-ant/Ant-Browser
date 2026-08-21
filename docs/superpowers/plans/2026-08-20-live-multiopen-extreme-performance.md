# 直播多开极致性能优化 最终方案 (v2)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在指纹面、窗口隔离、直播可播性完全不变的前提下,把多开直播场景的每实例 CPU/内存/磁盘/网络消耗压到最低,并让闲置资源自动回收。

**Architecture:** 新增"直播性能模式"(`live_perf_enabled`,默认开)作为一层独立的启动参数 / Preferences 叠加层,与现有指纹参数、MemorySaver 参数并存;先修复多个 `--disable-features` 互相覆盖的合并 bug 使叠加层可靠生效;用一组"红线守卫测试"把本次讨论确认的全部禁用项编码进 CI;再补三项运行时监控(硬解巡检、主动内存回收、缓存回收)。

**Tech Stack:** Go (Wails backend)、fingerprint-chromium 144/148 内核、React/TS 前端。

**v2 变更(相对 v1):** 参数集从 8 条扩到 40+ 条并分三梯队;红线清单补入 codec 面 / plugins / API 存在性 / 网络指纹四类新发现的陷阱,并编码为守卫测试;新增 GPU 硬解巡检、CDP 主动内存回收、CDP 冒烟工具三个任务;新增三项前置调研(它们的收益远大于参数层);新增三项后置长期项。

---

## 一、全局约束(红线,每个任务隐含遵守)

### 1.1 ⛔ 指纹面:任何任务不得增删改

**原有红线:**
- 一切 `--fingerprint-*`、`--disable-spoofing`、`--disable-non-proxied-udp`、`--fingerprinting-*`。
- 一切 GPU / 图形管线开关:`--disable-gpu`、`--use-gl`、`--use-angle`、`--enable/disable-gpu-rasterization`、`--disable-gpu-compositing`、`--disable-accelerated-2d-canvas`、`--force-color-profile`、`--blink-settings=*`。
- `--js-flags`(改 `performance.memory.jsHeapSizeLimit`)。
- `--autoplay-policy`(新内核有 `navigator.getAutoplayPolicy()`)。
- Preferences 中映射到 JS 可见状态的项(`default_content_setting_values.*`、Privacy Sandbox)。
- UA、语言、时区、字体、屏幕尺寸、DPR、`hardwareConcurrency`、`deviceMemory`、媒体设备枚举、WebRTC 现有逻辑。

**v2 新增红线(本轮讨论新发现,均为 JS 可读):**

| 类别 | 禁止项 | 泄漏点 |
|---|---|---|
| **codec 支持面** | 任何 codec feature 开关(`PlatformHEVCDecoderSupport`、AV1/VP9/VaapiVideoDecoder 相关) | `MediaSource.isTypeSupported`、`navigator.mediaCapabilities.decodingInfo`、`canPlayType` |
| **插件面** | `--disable-plugins`、编译期 `enable_pdf=false` | `navigator.plugins` / `navigator.mimeTypes`(最老牌检测点) |
| **API 存在性** | 关 Notification / Push / BackgroundSync / BackgroundFetch / PeriodicSync / WebBluetooth / WebUSB / WebSerial / WebNFC / SpeechAPI / FileSystem / WebSQL / SharedWorkers / Permissions / Presentation | 对应 `navigator.*` / `ServiceWorkerRegistration.*` 属性直接消失 |
| **网络指纹** | `--disable-quic`、关 HTTP/2、关 Brotli、动 TLS session cache | TLS/HTTP 指纹面 |
| **字体渲染** | `--disable-remote-fonts` | 文本度量 → ClientRects / 字体指纹 |
| **行为面** | 让页面在 `visibilityState==='visible'` 时被 rAF 限频 | 真实用户不会出现这种组合,本身即自动化信号 |
| **自动化标识** | `--enable-automation`(当前代码未使用,须永不加入) | `navigator.webdriver` |

> 注:上述 API 在不使用时本来就是惰性的,**关掉几乎零收益、纯风险**。

### 1.2 ✅ 必须保留(直播可播 + 看播时长有效)

- `--disable-backgrounding-occluded-windows`、`--disable-renderer-backgrounding`、`--disable-background-timer-throttling`(app_instance_start_prepare.go:329-331)。
- `--disable-features=CalculateNativeWinOcclusion`(遮挡窗口保持 `visibilityState=visible`)。
- CDP 保活循环(app_live_keepalive.go)、`--mute-audio` 按实例静音。
- JavaScript、MSE、EME、WebAudio、WebRTC、Service Worker 全部保留。

### 1.3 ✅ CDP 面:不得打断(已核实项目实际用量)

| 类型 | 使用中的能力 |
|---|---|
| HTTP 端点 | `/json`、`/json/version`、`/json/list` |
| Browser 域 | `close`、`getWindowForTarget`、`setWindowBounds` |
| 其他域 | `Target.createTarget`、`Page.navigate`、`Runtime.evaluate`、`Input.dispatchMouseEvent`、`Network.getAllCookies` / `clearBrowserCookies`、`Emulation.setGeolocationOverride` / `setLocaleOverride` / `setTimezoneOverride` |
| 自动化 | playwright-core(用得更宽:DOM / Fetch / Frame) |

因此禁用:`--no-startup-window`、`--remote-debugging-pipe`(已在 `managedLaunchArgSpecs` 拦截)、`--single-process`、`--in-process-gpu`。

### 1.4 ⛔ 安全/稳定性

`--no-sandbox`、`--disable-web-security` 绝对禁止(前者安全性归零,后者 CORS 行为可被页面检测)。

### 1.5 工程约束

- feature 名以 148 内核为准;144 不认识的名字 Chromium 静默忽略,无害。
- 所有参数集必须通过 §Task 2 的红线守卫测试。
- 每完成一个任务:`go test ./...` 通过后即 `git commit` + `git push`(分支 `feature/fingerprint-self-consistency`),按 CLAUDE.md 交付流程,不打包。

---

## 二、收益预期与执行顺序(先读这段)

**核心结论:参数层是小头。真正的大头是三项前置调研,它们的收益可能超过本方案全部参数之和。**

| 来源 | 内存 | CPU | 性质 |
|---|---|---|---|
| **W2 硬解未生效 → 生效** | — | **−50~80%** | 调研,可能已经是对的,但必须验证 |
| **W1 内核非官方构建 → official+PGO/LTO** | 小 | **−10~25%** | 调研,零指纹变化 |
| **W3 画质 1080p → 480p + 关弹幕** | −20~30% | **−30~50%** | 运维,一次性手动,零注入 |
| 显示器 144Hz → 60Hz | — | 合成开销减半 | 运维,零成本 |
| A 级参数(Task 3) | **−30~40%** | −5~15% | 本方案主体 |
| B 级参数(site isolation) | 再 −10% | 小 | 可选,牺牲隔离安全性 |
| 主动内存回收(Task 12) | 长时运行防累积 | — | 新功能 |

**执行顺序建议:**

1. **先做 W1 / W2 / W3 三项调研**(不写代码,1~2 天)。如果发现跑的是软解,后面所有参数优化都可以先放一放。
2. Task 1 → 2 → 3 → 4 → 5(参数主体,必须按序,Task 1 是前置修复,Task 2 是守卫)。
3. Task 6~10(应用侧削减,彼此独立,可并行)。
4. Task 11~13(监控与验证工具)。
5. 后置长期项 D1~D3 按需排期。

---

## 三、前置调研(不写代码,先做)

### W1:内核构建配置审计 —— 最高价值,零指纹风险

**背景:** 非官方构建的 Chromium 比 `is_official_build=true` + PGO + ThinLTO 构建慢 10~25%,而这个差距**不改任何指纹面**。本地 `chrome/` 目录只有 macOS 开发占位,Windows 内核是外部下载管理的,无法离线判断,必须查。

**步骤:**
1. 查 fingerprint-chromium 发布仓库的构建 workflow / `args.gn`,确认是否含:`is_official_build=true`、`chrome_pgo_phase=2`、`use_thin_lto=true`、`symbol_level=0`、`is_component_build=false`。
2. 若查不到,做**基准对照**:同一台机器上,用当前内核与同版本官方 Chrome 各跑一次 Speedometer / JetStream,差距 >10% 基本可判定为非官方构建。
3. 若确认非官方构建 → 自行编译一版 official+PGO 内核,或推动上游。这是投入产出比最高的一项。

参考:[Chromium PGO 文档](https://chromium.googlesource.com/chromium/src.git/+/refs/heads/main/docs/pgo.md)、[GN build configuration](https://www.chromium.org/developers/gn-build-configuration/)。注意 `is_official_build=true` 需要 `.gclient` 里加 `"checkout_pgo_profiles": True`,否则 `gn gen` 直接失败。

### W2:硬解基线 —— 决定 CPU 大盘

exe 是 mac 交叉编译,首次在真实 Windows / 云主机运行;GPU 被 Chromium 驱动黑名单拉黑非常常见,一旦拉黑就是**全程软解,CPU 翻 2~5 倍**。

**步骤:**
1. 开 1 个实例,访问 `chrome://gpu`,看 "Video Decode" 是否为 `Hardware accelerated`。
2. 开直播,访问 `chrome://media-internals`,确认 video decoder 是 `D3D11VideoDecoder`(或 VDAVideoDecoder);出现 `FFmpegVideoDecoder` / `VpxVideoDecoder` 即为软解。
3. 任务管理器看 GPU 的 "Video Decode" 占用是否随窗口数上升。
4. 若被黑名单拉黑:评估 `--ignore-gpu-blocklist`(逐机测稳定性;它不改渲染输出,但会启用被判定为有问题的驱动路径)。
5. 逐台机器标定**硬解并发上限**(Intel iGPU 1080p30 约 15~20 路;NVIDIA 消费卡 NVDEC 有驱动级并发限制)。超限的窗口会静默回落软解,反而拖垮全场。

### W3:画质 / 弹幕基线 —— 单实例 CPU 最大杠杆

弹幕(持续 DOM + 动画 + 合成)与画质(解码像素数,1080p→480p 解码量降到约 1/4)是单实例 CPU 的两个最大头。**两者都不是浏览器功能,是各直播平台播放器网页自己的 UI,存在该 profile 的 localStorage 里,内核层没有开关。**

- **合规做法(采纳):** 每个账号首次进直播间时手动选一次最低画质、关一次弹幕。因每环境 `user-data-dir` 独立且持久,该设置重启后保持,只需设一次。
- **⛔ 否决:** 注入 JS/CSS 自动点按钮 / 隐藏弹幕层。属页面行为面,有被识别为非真人环境的风险,与约束 1 冲突。
- **步骤:** 取一个实例,分别在 1080p+弹幕开 / 480p+弹幕关两种状态下测 30 分钟 CPU 均值,量化本场景的实际差值,写进运维手册。

---

## 四、File Structure

- 新增 `backend/internal/browser/live_perf.go` + `live_perf_test.go` —— A/B 级参数集,唯一事实来源。
- 新增 `backend/internal/browser/forbidden_args.go` + `forbidden_args_test.go` —— 红线守卫(禁止前缀 + 禁止 feature 名)。
- 新增 `backend/browser_launch_args_features.go` + `_test.go` —— feature 开关合并器。
- 新增 `backend/app_gpu_probe.go` + `_test.go` —— GPU 硬解巡检。
- 新增 `backend/app_memory_reclaim.go` + `_test.go` —— CDP 主动内存回收。
- 新增 `backend/app_profile_cache_cleanup.go` + `_test.go` —— 已停实例缓存清理。
- 新增 `backend/cmd/cdp-smoke/main.go` —— CDP 冒烟工具。
- 新增 `scripts/measure-live-perf.ps1` —— Windows 实测脚本。
- 修改 `backend/internal/browser/memory_saver.go`、`backend/internal/config/config.go`、`config_defaults.go`、`backend/app_instance_start_prepare.go`、`backend/browser_cookie_pref.go`、`backend/app_startup.go`、`backend/app_backup_runtime.go`、`frontend/src/modules/browser/pages/browserList/useBrowserListData.ts`、`config.yaml`。

---

### Task 1: `--disable-features` / `--enable-features` 合并器(前置修复)

Chromium 对重复出现的同名开关取**最后一个**。现在指纹层、MemorySaver、用户自定义 LaunchArgs 若各带一条 `--disable-features=`,前面的会被静默覆盖。本方案要再叠一层 20+ 个 feature,必须先修这个,否则后面全部不可靠。

**Files:**
- Create: `backend/browser_launch_args_features.go`
- Test: `backend/browser_launch_args_features_test.go`

**Interfaces:**
- Produces: `mergeFeatureSwitches(args []string) []string` —— 把所有 `--disable-features=` / `--enable-features=` 各合并成一条(去重、保序、逗号连接,放在原第一条出现的位置),其余参数原样保留。

- [ ] **Step 1: 写失败测试**

```go
package backend

import (
	"reflect"
	"testing"
)

func TestMergeFeatureSwitchesCombinesDuplicates(t *testing.T) {
	in := []string{
		"--user-data-dir=/x",
		"--disable-features=Translate,MediaRouter",
		"--fingerprint=7",
		"--disable-features=MediaRouter,LiveCaption",
		"--enable-features=Foo",
		"--enable-features=Bar",
	}
	got := mergeFeatureSwitches(in)
	want := []string{
		"--user-data-dir=/x",
		"--disable-features=Translate,MediaRouter,LiveCaption",
		"--fingerprint=7",
		"--enable-features=Foo,Bar",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestMergeFeatureSwitchesNoOpWithoutDuplicates(t *testing.T) {
	in := []string{"--disable-features=Translate", "--mute-audio"}
	if got := mergeFeatureSwitches(in); !reflect.DeepEqual(got, in) {
		t.Fatalf("got %v want %v", got, in)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd Ant-Browser-Plus && go test ./backend/ -run TestMergeFeatureSwitches -v`
Expected: FAIL(`mergeFeatureSwitches` 未定义)

- [ ] **Step 3: 实现**

```go
package backend

import "strings"

// mergeFeatureSwitches 把重复的 --disable-features/--enable-features 合并成各一条。
// Chromium 对同名开关只取最后一条,不合并会导致指纹层与性能层的 feature 列表互相覆盖。
func mergeFeatureSwitches(args []string) []string {
	return mergeValueSwitch(mergeValueSwitch(args, "--disable-features="), "--enable-features=")
}

func mergeValueSwitch(args []string, prefix string) []string {
	values := make([]string, 0, 8)
	seen := map[string]struct{}{}
	firstIdx := -1
	out := make([]string, 0, len(args))
	for _, arg := range args {
		if !strings.HasPrefix(arg, prefix) {
			out = append(out, arg)
			continue
		}
		if firstIdx == -1 {
			firstIdx = len(out)
			out = append(out, "") // 占位,最后回填合并结果
		}
		for _, v := range strings.Split(strings.TrimPrefix(arg, prefix), ",") {
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			if _, ok := seen[v]; ok {
				continue
			}
			seen[v] = struct{}{}
			values = append(values, v)
		}
	}
	if firstIdx == -1 {
		return args
	}
	if len(values) == 0 {
		return append(out[:firstIdx], out[firstIdx+1:]...)
	}
	out[firstIdx] = prefix + strings.Join(values, ",")
	return out
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./backend/ -run TestMergeFeatureSwitches -v`
Expected: PASS

- [ ] **Step 5: 在 buildBrowserLaunchArgs 出口接入**

修改 `backend/app_instance_start_prepare.go` 的 `buildBrowserLaunchArgs` 结尾:

```go
	args = append(args, normalizeNonEmptyStrings(fingerprintLaunchArgs)...)
	args = append(args, sanitizedProfileLaunchArgs...)
	args = append(args, sanitizedExtraLaunchArgs...)
	args = mergeFeatureSwitches(args)
	return browser.BuildLaunchArgs(args, launchTargets)
```

- [ ] **Step 6: 全量测试 + 提交**

Run: `go test ./backend/...`
Expected: PASS

```bash
git add backend/browser_launch_args_features.go backend/browser_launch_args_features_test.go backend/app_instance_start_prepare.go
git commit -m "fix: merge duplicate --disable/enable-features switches (last-one-wins bug)"
```

---

### Task 2: 红线守卫(先建守卫,再加参数)

把 §1.1~1.4 的全部禁用项编码成可执行检查。以后任何人往参数集里加违禁项,CI 直接拦下。

**Files:**
- Create: `backend/internal/browser/forbidden_args.go`
- Test: `backend/internal/browser/forbidden_args_test.go`

**Interfaces:**
- Produces: `browser.ForbiddenArgPrefixes() []string`、`browser.ForbiddenFeatureNames() []string`、`browser.AssertNoForbiddenArgs(args []string) []string`(返回违规项列表,空表示通过)。Task 3 与后续所有参数集测试消费。

- [ ] **Step 1: 写失败测试**

```go
package browser

import "testing"

func TestAssertNoForbiddenArgsCatchesPrefixes(t *testing.T) {
	bad := AssertNoForbiddenArgs([]string{"--process-per-site", "--js-flags=--max-old-space-size=512"})
	if len(bad) != 1 || bad[0] != "--js-flags=--max-old-space-size=512" {
		t.Fatalf("expected js-flags flagged, got %v", bad)
	}
}

func TestAssertNoForbiddenArgsCatchesFeatureNames(t *testing.T) {
	bad := AssertNoForbiddenArgs([]string{"--disable-features=Translate,WebBluetooth"})
	if len(bad) != 1 {
		t.Fatalf("expected WebBluetooth flagged, got %v", bad)
	}
}

func TestAssertNoForbiddenArgsAllowsCleanSet(t *testing.T) {
	if bad := AssertNoForbiddenArgs([]string{"--process-per-site", "--disable-features=Translate,MediaRouter"}); len(bad) != 0 {
		t.Fatalf("clean set should pass, got %v", bad)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./backend/internal/browser/ -run TestAssertNoForbiddenArgs -v`
Expected: FAIL

- [ ] **Step 3: 实现**

```go
package browser

import "strings"

// ForbiddenArgPrefixes 是永不允许出现在任何托管参数集里的启动参数前缀。
// 分四类:①改 JS 可见指纹面 ②改渲染/播放行为面 ③打断 CDP ④安全归零。
// 详见 docs/superpowers/plans/2026-08-20-live-multiopen-extreme-performance.md §1。
func ForbiddenArgPrefixes() []string {
	return []string{
		// ① 指纹面
		"--fingerprint", "--disable-spoofing", "--js-flags",
		// ① GPU / 渲染管线(改 Canvas/WebGL 输出)
		"--disable-gpu", "--use-gl", "--use-angle", "--force-color-profile",
		"--disable-accelerated", "--disable-webgl", "--disable-3d-apis",
		"--disable-reading-from-canvas", "--enable-gpu-rasterization",
		"--disable-gpu-compositing", "--blink-settings",
		// ① JS 可见 API 存在性
		"--disable-notifications", "--disable-plugins", "--disable-speech-api",
		"--disable-file-system", "--disable-databases", "--disable-local-storage",
		"--disable-shared-workers", "--disable-permissions-api",
		"--disable-presentation-api", "--disable-remote-fonts",
		// ① 网络指纹
		"--disable-quic", "--disable-http2",
		// ② 播放行为面
		"--autoplay-policy", "--disable-accelerated-video-decode", "--disable-javascript",
		// ③ 打断 CDP
		"--no-startup-window", "--remote-debugging-pipe", "--single-process",
		"--in-process-gpu", "--enable-automation",
		// ④ 安全
		"--no-sandbox", "--disable-web-security",
	}
}

// ForbiddenFeatureNames 是永不允许出现在 --disable-features 里的 feature 名。
// 关掉它们会让对应 JS API 直接消失(可读的指纹面),且这些服务闲置时本就不占资源。
func ForbiddenFeatureNames() []string {
	return []string{
		"WebBluetooth", "WebUSB", "WebSerial", "WebNFC",
		"PushMessaging", "BackgroundFetch", "BackgroundSync", "PeriodicBackgroundSync",
		"Notifications", "WebAudio", "MediaSource", "EncryptedMedia",
		"PlatformHEVCDecoderSupport", "VaapiVideoDecoder", "Av1Decoder",
		"PrivacySandboxSettings4", "BrowsingTopics", "UserAgentClientHint",
	}
}

// AssertNoForbiddenArgs 返回 args 中命中红线的项;空切片表示通过。
func AssertNoForbiddenArgs(args []string) []string {
	var bad []string
	prefixes := ForbiddenArgPrefixes()
	features := ForbiddenFeatureNames()
	for _, arg := range args {
		trimmed := strings.TrimSpace(arg)
		for _, p := range prefixes {
			if strings.HasPrefix(trimmed, p) {
				bad = append(bad, arg)
				break
			}
		}
		if !strings.HasPrefix(trimmed, "--disable-features=") {
			continue
		}
		for _, v := range strings.Split(strings.TrimPrefix(trimmed, "--disable-features="), ",") {
			v = strings.TrimSpace(v)
			for _, f := range features {
				if strings.EqualFold(v, f) {
					bad = append(bad, arg)
					break
				}
			}
		}
	}
	return bad
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./backend/internal/browser/ -run TestAssertNoForbiddenArgs -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/browser/forbidden_args.go backend/internal/browser/forbidden_args_test.go
git commit -m "feat: encode fingerprint/CDP/security red lines as executable arg guard"
```

---

### Task 3: `LivePerfArgs()` 全量参数集(A/B 级)

**Files:**
- Create: `backend/internal/browser/live_perf.go`
- Test: `backend/internal/browser/live_perf_test.go`

**Interfaces:**
- Consumes: Task 2 的 `AssertNoForbiddenArgs`。
- Produces: `browser.LivePerfArgs() []string`(A 级)、`browser.LivePerfAggressiveArgs() []string`(B 级增量)。Task 5 在 prepare 阶段消费。

- [ ] **Step 1: 写失败测试**

```go
package browser

import (
	"strings"
	"testing"
)

func TestLivePerfArgsMustHaves(t *testing.T) {
	joined := strings.Join(LivePerfArgs(), "\n")
	must := []string{
		"--process-per-site",
		"--renderer-process-limit=3",
		"--disable-background-networking",
		"--disable-component-update",
		"--disable-component-extensions-with-background-pages",
		"--metrics-recording-only",
		"--disable-ipc-flooding-protection",
		"--disk-cache-size=134217728",
		"--media-cache-size=33554432",
	}
	for _, m := range must {
		if !strings.Contains(joined, m) {
			t.Errorf("missing %s", m)
		}
	}
	for _, f := range []string{"SpareRendererForSitePerProcess", "AudioServiceOutOfProcess", "MediaRouter"} {
		if !strings.Contains(joined, f) {
			t.Errorf("missing feature %s", f)
		}
	}
	// 看播时长命脉:遮挡窗口必须保持 visible
	if !strings.Contains(joined, "CalculateNativeWinOcclusion") {
		t.Error("must keep CalculateNativeWinOcclusion disabled")
	}
}

// 红线守卫:A/B 两级参数集都不得碰指纹面 / 播放面 / CDP / 安全。
func TestLivePerfArgsRespectRedLines(t *testing.T) {
	for name, set := range map[string][]string{
		"A": LivePerfArgs(),
		"B": LivePerfAggressiveArgs(),
	} {
		if bad := AssertNoForbiddenArgs(set); len(bad) != 0 {
			t.Errorf("tier %s violates red lines: %v", name, bad)
		}
	}
}

func TestLivePerfAggressiveArgs(t *testing.T) {
	if !strings.Contains(strings.Join(LivePerfAggressiveArgs(), "\n"), "--disable-site-isolation-trials") {
		t.Error("aggressive set must merge cross-site iframes")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./backend/internal/browser/ -run TestLivePerf -v`
Expected: FAIL(未定义)

- [ ] **Step 3: 实现**

```go
package browser

import "strings"

// livePerfDisabledFeatures 是直播性能模式关闭的 Chromium feature。
// 铁律:新增前必须过 AssertNoForbiddenArgs —— 任何"关掉会让 JS API 消失"的 feature 都不许进。
var livePerfDisabledFeatures = []string{
	// 看播时长命脉:禁用原生遮挡检测,遮挡窗口保持 visibilityState=visible
	"CalculateNativeWinOcclusion",
	// 每实例的"备用渲染进程",~80-150MB,多开时纯浪费
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
// 铁律:此列表只允许出现【JS 不可见、不改 Canvas/WebGL/UA/codec 能力查询、不影响直播播放
// 与 visibilityState、不打断 CDP】的开关。新增前先过 live_perf_test.go 的红线守卫。
// feature 名以 148 内核为准;144 不认识的名字会被 Chromium 静默忽略(无害)。
func LivePerfArgs() []string {
	return []string{
		// ── 进程模型(进程数 JS 不可见)──
		"--process-per-site",
		"--renderer-process-limit=3",

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
		"--disable-back-forward-cache",  // 单页直播无前进后退,不缓存整页
		"--disk-cache-size=134217728",   // 128MB
		"--media-cache-size=33554432",   // 32MB:直播分片复用价值≈0

		// ── UI 打扰 ──
		"--disable-hang-monitor", // 无人值守,不要"页面无响应"弹窗
		"--disable-prompt-on-repost",
		"--password-store=basic",

		// ── 为 CDP 稳定性而加(非削减):
		// Chromium 会节流高频发 IPC 的渲染进程,playwright 批量操作时可能被限速。
		"--disable-ipc-flooding-protection",

		"--disable-features=" + strings.Join(livePerfDisabledFeatures, ","),
	}
}

// LivePerfAggressiveArgs 返回 B 级(激进)增量参数,默认不启用。
//
// --disable-site-isolation-trials:跨站 iframe(广告/埋点)并进同一渲染进程,每实例再省 1-4 个进程。
// 它【不改任何 JS 可见指纹面】(crossOriginIsolated 由 COOP/COEP 决定,与之无关),
// 真实代价是:① 放弃站点隔离安全性(Spectre 类)② 同进程内广告 iframe 崩溃会带走整页。
// 结论:只开固定直播平台的专用挂机墙可接受;什么站都开的通用环境不要开。
func LivePerfAggressiveArgs() []string {
	return []string{
		"--disable-site-isolation-trials",
	}
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./backend/internal/browser/ -run TestLivePerf -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/browser/live_perf.go backend/internal/browser/live_perf_test.go
git commit -m "feat: full LivePerfArgs set (process/service/network/disk/UI tiers) under red-line guard"
```

---

### Task 4: 从 MemorySaverArgs 移除 `--js-flags`(指纹泄漏修复)

`--js-flags=--max-old-space-size=512` 会把 JS 可读的 `performance.memory.jsHeapSizeLimit` 从常规 ~4GB 改成 512MB,检测脚本一读便知。进程内存控制交给 Task 3 的进程上限方案。

**Files:**
- Modify: `backend/internal/browser/memory_saver.go:20`
- Test: `backend/internal/browser/memory_saver_test.go`

- [ ] **Step 1: 追加守卫测试**

```go
func TestMemorySaverArgsRespectRedLines(t *testing.T) {
	if bad := AssertNoForbiddenArgs(MemorySaverArgs()); len(bad) != 0 {
		t.Fatalf("MemorySaverArgs violates red lines: %v", bad)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./backend/internal/browser/ -run TestMemorySaverArgsRespectRedLines -v`
Expected: FAIL(命中 `--js-flags`)

- [ ] **Step 3: 删除该行并补注释**

删除 `memory_saver.go` 中整行:

```go
		"--js-flags=--max-old-space-size=512", // 每渲染进程 V8 老生代上限(足够重型 SPA/电商)
```

在函数 doc 注释末尾追加:

```go
// 注意:不得加 --js-flags —— 它会改 performance.memory.jsHeapSizeLimit,属 JS 可见指纹面。
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./backend/internal/browser/ -run TestMemorySaver -v`
Expected: PASS(若既有测试断言了 js-flags 存在,同步删除该断言)

- [ ] **Step 5: Commit**

```bash
git add backend/internal/browser/memory_saver.go backend/internal/browser/memory_saver_test.go
git commit -m "fix: drop --js-flags from MemorySaverArgs (JS-visible jsHeapSizeLimit leak)"
```

---

### Task 5: 配置项 + 启动装配接线

**Files:**
- Modify: `backend/internal/config/config.go`(BrowserConfig,`MemorySaverEnabled` 字段旁)
- Modify: `backend/internal/config/config_defaults.go`
- Modify: `backend/app_instance_start_prepare.go:174-176`
- Modify: `config.yaml`
- Test: `backend/app_instance_launch_args_liveperf_test.go`(新建)

**Interfaces:**
- Consumes: `browser.LivePerfArgs()` / `browser.LivePerfAggressiveArgs()`。
- Produces: `Browser.LivePerfEnabled *bool`(nil=默认开)、`Browser.LivePerfAggressive bool`(默认关);`appendLivePerfArgs(args []string, cfg *config.Config) []string`;`livePerfPrefsEnabled(cfg *config.Config) bool`(Task 7 消费)。

- [ ] **Step 1: config.go 增加字段(加在 MemorySaverEnabled 之后)**

```go
	LivePerfEnabled    *bool `yaml:"live_perf_enabled,omitempty"`    // 直播多开性能模式(nil=默认开)
	LivePerfAggressive bool  `yaml:"live_perf_aggressive,omitempty"` // B级激进(默认关):合并站点隔离
```

- [ ] **Step 2: 写失败测试**

```go
package backend

import (
	"strings"
	"testing"

	"ant-chrome/backend/internal/config"
)

func TestLivePerfArgsAppliedByDefault(t *testing.T) {
	joined := strings.Join(appendLivePerfArgs(nil, &config.Config{}), "\n")
	if !strings.Contains(joined, "--process-per-site") {
		t.Fatal("live perf args should apply when unset (default on)")
	}
	if strings.Contains(joined, "--disable-site-isolation-trials") {
		t.Fatal("aggressive args must stay off by default")
	}
}

func TestLivePerfArgsDisabled(t *testing.T) {
	off := false
	cfg := &config.Config{}
	cfg.Browser.LivePerfEnabled = &off
	if got := appendLivePerfArgs(nil, cfg); len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

func TestLivePerfAggressiveOptIn(t *testing.T) {
	cfg := &config.Config{}
	cfg.Browser.LivePerfAggressive = true
	if !strings.Contains(strings.Join(appendLivePerfArgs(nil, cfg), "\n"), "--disable-site-isolation-trials") {
		t.Fatal("aggressive opt-in should append aggressive args")
	}
}
```

- [ ] **Step 3: 跑测试确认失败**

Run: `go test ./backend/ -run TestLivePerf -v`
Expected: FAIL(`appendLivePerfArgs` 未定义)

- [ ] **Step 4: 实现并接线**

在 `backend/app_instance_start_prepare.go`(`muteAudioLaunchArgs` 附近)新增:

```go
// livePerfEnabled 直播性能模式开关:nil=默认开。参数集与红线守卫见 internal/browser/live_perf.go。
func livePerfEnabled(cfg *config.Config) bool {
	if cfg == nil {
		return true
	}
	if p := cfg.Browser.LivePerfEnabled; p != nil {
		return *p
	}
	return true
}

// livePerfPrefsEnabled 与 livePerfEnabled 同一开关(Preferences 层用,见 Task 7)。
func livePerfPrefsEnabled(cfg *config.Config) bool { return livePerfEnabled(cfg) }

// appendLivePerfArgs 按配置叠加直播性能模式参数。
func appendLivePerfArgs(args []string, cfg *config.Config) []string {
	if !livePerfEnabled(cfg) {
		return args
	}
	args = append(args, browser.LivePerfArgs()...)
	if cfg != nil && cfg.Browser.LivePerfAggressive {
		args = append(args, browser.LivePerfAggressiveArgs()...)
	}
	return args
}
```

在 `prepareBrowserStartPlan` 的 MemorySaver 分支之后追加一行:

```go
	sanitizedExtraLaunchArgs = appendLivePerfArgs(sanitizedExtraLaunchArgs, a.config)
```

(顺序:MemorySaver 之后、mute-audio 之前;Task 1 的合并器在出口统一合并 feature 列表。)

- [ ] **Step 5: config.yaml 显式声明**

在 `browser:` 段 `light_start_enabled: true` 下追加:

```yaml
    live_perf_enabled: true
    live_perf_aggressive: false
```

- [ ] **Step 6: 跑测试 + 提交**

Run: `go test ./backend/...`
Expected: PASS

```bash
git add backend/internal/config/config.go backend/internal/config/config_defaults.go backend/app_instance_start_prepare.go backend/app_instance_launch_args_liveperf_test.go config.yaml
git commit -m "feat: wire live_perf_enabled/live_perf_aggressive into browser launch args"
```

---

### Task 6: 无扩展实例追加 `--disable-extensions`

**Files:**
- Modify: `backend/app_instance_start_prepare.go`(buildBrowserLaunchArgs 扩展分支,~343 行)
- Test: 追加到 `backend/app_instance_launch_args_liveperf_test.go`

- [ ] **Step 1: 写失败测试**

```go
func TestBuildArgsDisablesExtensionsWhenNone(t *testing.T) {
	args := buildBrowserLaunchArgs("/tmp/x", 9222, "", nil, nil, nil, nil, nil, false)
	if !argsContain(args, "--disable-extensions") {
		t.Fatal("expected --disable-extensions when no extension dirs")
	}
	withExt := buildBrowserLaunchArgs("/tmp/x", 9222, "", []string{"/ext/a"}, nil, nil, nil, nil, false)
	if argsContain(withExt, "--disable-extensions") {
		t.Fatal("must not disable extensions when extension dirs exist")
	}
}

func argsContain(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./backend/ -run TestBuildArgsDisablesExtensions -v`
Expected: FAIL

- [ ] **Step 3: 实现(扩展分支改 if/else)**

```go
	if extensionArg := strings.Join(normalizeNonEmptyStrings(extensionDirs), ","); extensionArg != "" {
		args = append(args, fmt.Sprintf("--disable-extensions-except=%s", extensionArg))
		args = append(args, fmt.Sprintf("--load-extension=%s", extensionArg))
	} else {
		args = append(args, "--disable-extensions") // 无扩展实例:跳过整个扩展子系统
	}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./backend/ -run TestBuildArgs -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/app_instance_start_prepare.go backend/app_instance_launch_args_liveperf_test.go
git commit -m "feat: pass --disable-extensions for profiles with no enabled extensions"
```

---

### Task 7: 性能 Preferences(浏览器服务开关,JS 不可见)

红线提醒:**不得**写任何 `profile.default_content_setting_values.*`(会改 `Notification.permission` 等 JS 可见权限状态)。

**Files:**
- Modify: `backend/browser_cookie_pref.go`(ensureLaunchPreferences)
- Modify: 两处调用点 `backend/app_instance_start_prepare.go:243`、`backend/app_instance_launch_args.go:157`
- Test: `backend/browser_cookie_pref_test.go`(若无则新建)

- [ ] **Step 1: 写失败测试**

```go
package backend

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureLaunchPreferencesLivePerf(t *testing.T) {
	dir := t.TempDir()
	ensureLaunchPreferences(dir, true)
	data, err := os.ReadFile(filepath.Join(dir, "Default", "Preferences"))
	if err != nil {
		t.Fatal(err)
	}
	var prefs map[string]any
	if err := json.Unmarshal(data, &prefs); err != nil {
		t.Fatal(err)
	}
	if sb, _ := prefs["safebrowsing"].(map[string]any); sb == nil || sb["enabled"] != false {
		t.Error("safebrowsing.enabled should be false")
	}
	if net, _ := prefs["net"].(map[string]any); net == nil || net["network_prediction_options"] != float64(2) {
		t.Error("net.network_prediction_options should be 2 (no prefetch/preconnect)")
	}
	if _, has := prefs["default_content_setting_values"]; has {
		t.Error("must never write content settings (JS-visible permission states)")
	}
}

func TestEnsureLaunchPreferencesLivePerfOff(t *testing.T) {
	dir := t.TempDir()
	ensureLaunchPreferences(dir, false)
	data, _ := os.ReadFile(filepath.Join(dir, "Default", "Preferences"))
	var prefs map[string]any
	_ = json.Unmarshal(data, &prefs)
	if _, ok := prefs["safebrowsing"]; ok {
		t.Error("perf prefs must not be written when live perf off")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./backend/ -run TestEnsureLaunchPreferences -v`
Expected: FAIL(签名不匹配)

- [ ] **Step 3: 实现**

签名改为 `func ensureLaunchPreferences(userDataDir string, livePerf bool)`,在 `prefs["browser"] = browser` 之后插入:

```go
	if livePerf {
		// 直播性能模式:关掉纯后台的浏览器服务(均为浏览器面,JS 不可见)。
		// 红线:不得写 default_content_setting_values(权限状态 JS 可读,属指纹面)。
		prefs["safebrowsing"] = mergeSubMap(prefs, "safebrowsing", map[string]interface{}{"enabled": false})
		prefs["net"] = mergeSubMap(prefs, "net", map[string]interface{}{"network_prediction_options": 2})
		prefs["search"] = mergeSubMap(prefs, "search", map[string]interface{}{"suggest_enabled": false})
		prefs["translate"] = mergeSubMap(prefs, "translate", map[string]interface{}{"enabled": false})
		prefs["spellcheck"] = mergeSubMap(prefs, "spellcheck", map[string]interface{}{"use_spelling_service": false})
		prefs["alternate_error_pages"] = mergeSubMap(prefs, "alternate_error_pages", map[string]interface{}{"enabled": false})
		prefs["credentials_enable_service"] = false
		profile["password_manager_leak_detection"] = false
		prefs["profile"] = profile
	}
```

同文件新增辅助函数:

```go
// mergeSubMap 取 prefs[key] 的既有子对象(无则新建),覆盖写入 kv 后返回。
func mergeSubMap(prefs map[string]interface{}, key string, kv map[string]interface{}) map[string]interface{} {
	sub, ok := prefs[key].(map[string]interface{})
	if !ok {
		sub = map[string]interface{}{}
	}
	for k, v := range kv {
		sub[k] = v
	}
	return sub
}
```

两处调用点改为 `ensureLaunchPreferences(userDataDir, livePerfPrefsEnabled(a.config))`。

- [ ] **Step 4: 跑测试 + 编译**

Run: `go test ./backend/ -run TestEnsureLaunchPreferences -v && go build ./...`
Expected: PASS 且编译通过(两处调用点都已更新)

- [ ] **Step 5: Commit**

```bash
git add backend/browser_cookie_pref.go backend/browser_cookie_pref_test.go backend/app_instance_start_prepare.go backend/app_instance_launch_args.go
git commit -m "feat: disable safebrowsing/prefetch/suggest/translate/spellcheck prefs in live perf mode"
```

---

### Task 8: 关停应用侧底噪(日志拦截器、自动测速)

**Files:**
- Modify: `config.yaml`(logging 段)
- Modify: `backend/internal/config/config.go`
- Modify: `backend/app_startup.go:214`、`backend/app_backup_runtime.go:72`
- Test: `backend/app_startup_speedsched_test.go`(新建)

- [ ] **Step 1: config.yaml 日志降噪**

```yaml
logging:
    level: warn
    file_enabled: false
    interceptor:
        enabled: false
        log_parameters: false
        log_results: false
```

(替换原 `level: info` / `interceptor.enabled: true` / `log_parameters: true` / `log_results: true`,其余字段保持。)

- [ ] **Step 2: 写失败测试**

```go
package backend

import (
	"testing"

	"ant-chrome/backend/internal/config"
)

func TestSpeedAutoTestDefaultOff(t *testing.T) {
	if speedAutoTestEnabled(&config.Config{}) {
		t.Fatal("auto speed test should default OFF for live farm")
	}
	on := true
	cfg := &config.Config{}
	cfg.Browser.SpeedAutoTestEnabled = &on
	if !speedAutoTestEnabled(cfg) {
		t.Fatal("explicit true should enable")
	}
}
```

- [ ] **Step 3: 跑测试确认失败**

Run: `go test ./backend/ -run TestSpeedAutoTest -v`
Expected: FAIL

- [ ] **Step 4: 实现**

config.go BrowserConfig 增加:

```go
	SpeedAutoTestEnabled *bool `yaml:"speed_autotest_enabled,omitempty"` // 后台自动测速(nil=默认关;手动测速不受影响)
```

app_startup.go 新增:

```go
// speedAutoTestEnabled 后台自动测速开关:直播挂机场景默认关(会周期拉起代理内核 + 发探测流量),
// 手动"立即测速"(RunOnce)不受影响。
func speedAutoTestEnabled(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	if p := cfg.Browser.SpeedAutoTestEnabled; p != nil {
		return *p
	}
	return false
}
```

`startupInitSpeedScheduler` 尾部与 `app_backup_runtime.go` 重建路径的 `Start()` 均改为:

```go
	if speedAutoTestEnabled(a.config) {
		a.speedScheduler.Start()
	}
```

- [ ] **Step 5: 跑测试 + 提交**

Run: `go test ./backend/...`
Expected: PASS

```bash
git add config.yaml backend/internal/config/config.go backend/app_startup.go backend/app_backup_runtime.go backend/app_startup_speedsched_test.go
git commit -m "perf: gate background proxy speed tests behind opt-in; silence API interceptor logging"
```

---

### Task 9: 前端实例列表轮询降频

事件(`browser:instance:started/updated/stopped/crashed`)已是主通道,2s 全量轮询只是兜底。

**Files:**
- Modify: `frontend/src/modules/browser/pages/browserList/useBrowserListData.ts:132`

- [ ] **Step 1: 修改轮询**

```ts
    // 事件驱动为主(started/updated/stopped/crashed 已实时刷新);
    // 轮询仅兜底进程意外消亡等无事件场景,10s 足够,降低多开时后端全量序列化压力。
    const timer = window.setInterval(() => {
      if (document.visibilityState !== 'visible') return
      void loadProfiles({ silent: true, syncRuntimeState: true })
    }, 10000)
```

- [ ] **Step 2: 构建验证**

Run: `cd frontend && npm run build`
Expected: 构建成功

- [ ] **Step 3: Commit**

```bash
git add frontend/src/modules/browser/pages/browserList/useBrowserListData.ts
git commit -m "perf: relax browser list fallback polling 2s -> 10s (events remain primary)"
```

---

### Task 10: 已停实例缓存自动清理

只清 HTTP / 编译 / GPU 缓存目录;**绝不**触碰 Cookies、Local Storage、IndexedDB、Service Worker(账号状态与站点可见存储)。

**Files:**
- Create: `backend/app_profile_cache_cleanup.go`
- Test: `backend/app_profile_cache_cleanup_test.go`
- Modify: `backend/app_startup.go`

**Interfaces:**
- Produces: `cleanProfileCaches(userDataDir string) (int64, error)`;`(a *App) CleanStoppedProfileCaches() int64`(Wails 导出)。

- [ ] **Step 1: 写失败测试**

```go
package backend

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanProfileCaches(t *testing.T) {
	dir := t.TempDir()
	mustWrite := func(rel string) string {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("xxxx"), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	gone := []string{
		mustWrite("Default/Cache/Cache_Data/f_000001"),
		mustWrite("Default/Code Cache/js/a_0"),
		mustWrite("Default/GPUCache/data_0"),
		mustWrite("GrShaderCache/data_0"),
	}
	keep := []string{
		mustWrite("Default/Cookies"),
		mustWrite("Default/Local Storage/leveldb/000003.log"),
		mustWrite("Default/Service Worker/CacheStorage/x/index"),
	}

	freed, err := cleanProfileCaches(dir)
	if err != nil {
		t.Fatal(err)
	}
	if freed <= 0 {
		t.Fatal("expected freed bytes > 0")
	}
	for _, p := range gone {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("cache path should be removed: %s", p)
		}
	}
	for _, p := range keep {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("account/site state must be untouched: %s", p)
		}
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./backend/ -run TestCleanProfileCaches -v`
Expected: FAIL

- [ ] **Step 3: 实现**

```go
package backend

import (
	"os"
	"path/filepath"

	"ant-chrome/backend/internal/logger"
)

// profileCacheDirs 可安全删除的纯缓存目录(相对 userDataDir)。
// 红线:不得加入 Cookies / Local Storage / IndexedDB / Service Worker —— 那是账号状态与
// 站点可见存储,删了轻则掉登录,重则形成"缓存被清空"的行为异常面。
var profileCacheDirs = []string{
	"Default/Cache",
	"Default/Code Cache",
	"Default/GPUCache",
	"Default/DawnGraphiteCache",
	"Default/DawnWebGPUCache",
	"GrShaderCache",
	"ShaderCache",
	"GraphiteDawnCache",
}

// cleanProfileCaches 删除单个实例数据目录下的纯缓存目录,返回释放字节数。
func cleanProfileCaches(userDataDir string) (int64, error) {
	var freed int64
	var firstErr error
	for _, rel := range profileCacheDirs {
		dir := filepath.Join(userDataDir, filepath.FromSlash(rel))
		size := dirSize(dir)
		if size == 0 {
			continue
		}
		if err := os.RemoveAll(dir); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		freed += size
	}
	return freed, firstErr
}

func dirSize(dir string) int64 {
	var total int64
	_ = filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err == nil && info.Mode().IsRegular() {
			total += info.Size()
		}
		return nil
	})
	return total
}

// CleanStoppedProfileCaches 对所有未运行实例执行缓存清理,返回释放总字节数(Wails 导出)。
func (a *App) CleanStoppedProfileCaches() int64 {
	log := logger.New("CacheClean")
	var total int64
	for _, p := range a.browserMgr.List() {
		if p.Running {
			continue
		}
		freed, err := cleanProfileCaches(a.browserMgr.ResolveUserDataDir(p))
		if err != nil {
			log.Warn("实例缓存清理部分失败", logger.F("profile_id", p.ProfileId), logger.F("error", err.Error()))
		}
		total += freed
	}
	if total > 0 {
		log.Info("已回收未运行实例缓存", logger.F("freed_mb", total/1024/1024))
	}
	return total
}
```

- [ ] **Step 4: 启动时自动执行一次**

`app_startup.go` 启动序列(`a.startupInitSpeedScheduler()` 之后)追加:

```go
	go a.CleanStoppedProfileCaches() // 启动即回收上次会话留下的已停实例缓存
```

- [ ] **Step 5: 跑测试 + 提交**

Run: `go test ./backend/ -run TestCleanProfileCaches -v && go build ./...`
Expected: PASS

```bash
git add backend/app_profile_cache_cleanup.go backend/app_profile_cache_cleanup_test.go backend/app_startup.go
git commit -m "feat: reclaim stopped-profile HTTP/code/GPU caches on startup (account state untouched)"
```

---

### Task 11: GPU 硬解巡检

W2 说明了硬解是 CPU 大盘的决定因素,且交叉编译的 exe 在客户机上被驱动黑名单拉黑很常见。本任务把"一次性人工检查"变成常驻能力:每次 app 会话对第一个就绪实例探测一次机器级 GPU 能力,软解则告警。

**Files:**
- Create: `backend/app_gpu_probe.go`
- Test: `backend/app_gpu_probe_test.go`

**Interfaces:**
- Consumes: 现有 `cdpBrowserCallResult`、`cdpGetEndpointBody`、`cdpDialWebSocket`、`cdpMessage`、`cdpResponse`、`cdpTarget`。
- Produces: `parseGPUFeatureStatus(text string) map[string]string`(纯函数)、`gpuHardwareDecodeOK(status map[string]string) bool`、`(a *App) ProbeGPUCapability(debugPort int) (map[string]string, error)`(Wails 导出)。

- [ ] **Step 1: 写失败测试(只测纯解析,CDP 部分靠 Task 13 冒烟工具验证)**

```go
package backend

import "testing"

const sampleGPUText = `Graphics Feature Status
Canvas: Hardware accelerated
Video Decode: Hardware accelerated
Video Encode: Software only. Hardware acceleration disabled
WebGL: Hardware accelerated
`

func TestParseGPUFeatureStatus(t *testing.T) {
	status := parseGPUFeatureStatus(sampleGPUText)
	if status["Video Decode"] != "Hardware accelerated" {
		t.Fatalf("got %q", status["Video Decode"])
	}
	if status["Video Encode"] != "Software only. Hardware acceleration disabled" {
		t.Fatalf("got %q", status["Video Encode"])
	}
}

func TestGPUHardwareDecodeOK(t *testing.T) {
	if !gpuHardwareDecodeOK(parseGPUFeatureStatus(sampleGPUText)) {
		t.Error("hardware accelerated video decode should pass")
	}
	soft := parseGPUFeatureStatus("Video Decode: Software only. Hardware acceleration disabled\n")
	if gpuHardwareDecodeOK(soft) {
		t.Error("software-only decode must fail the check")
	}
	if gpuHardwareDecodeOK(map[string]string{}) {
		t.Error("missing entry must fail (fail-closed)")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./backend/ -run 'TestParseGPUFeatureStatus|TestGPUHardwareDecodeOK' -v`
Expected: FAIL

- [ ] **Step 3: 实现**

```go
package backend

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"ant-chrome/backend/internal/logger"
)

// gpuProbeExpression 取 chrome://gpu 的全部可见文本。该页部分版本用 shadow DOM 渲染,
// 因此在 innerText 之外再递归收集各 shadowRoot 的文本。
const gpuProbeExpression = `(() => {
  const parts = [document.body ? document.body.innerText : ''];
  const walk = (root) => {
    root.querySelectorAll('*').forEach((el) => {
      if (el.shadowRoot) { parts.push(el.shadowRoot.textContent || ''); walk(el.shadowRoot); }
    });
  };
  walk(document);
  return parts.join('\n');
})()`

// parseGPUFeatureStatus 把 chrome://gpu 的 "Graphics Feature Status" 文本解析成 特性→状态 映射。
func parseGPUFeatureStatus(text string) map[string]string {
	status := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		idx := strings.Index(line, ":")
		if idx <= 0 || idx == len(line)-1 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])
		if key == "" || value == "" {
			continue
		}
		if _, exists := status[key]; !exists {
			status[key] = value
		}
	}
	return status
}

// gpuHardwareDecodeOK 判定视频硬解是否可用。缺失条目按失败处理(fail-closed),
// 因为"读不到"和"没有硬解"对运维的处置是一样的:必须人工核查。
func gpuHardwareDecodeOK(status map[string]string) bool {
	value, ok := status["Video Decode"]
	if !ok {
		return false
	}
	lower := strings.ToLower(value)
	return strings.Contains(lower, "hardware accelerated") && !strings.Contains(lower, "software only")
}

// ProbeGPUCapability 在指定实例里开一个 chrome://gpu 标签,读取图形特性状态后立即关闭。
// 机器级信息,每个 app 会话只需跑一次(见 startupProbeGPUOnce)。
func (a *App) ProbeGPUCapability(debugPort int) (map[string]string, error) {
	created, err := cdpBrowserCallResult(debugPort, "Target.createTarget", map[string]any{"url": "chrome://gpu"})
	if err != nil {
		return nil, fmt.Errorf("创建 GPU 探测标签失败: %w", err)
	}
	targetID, _ := created["targetId"].(string)
	if targetID == "" {
		return nil, fmt.Errorf("GPU 探测标签未返回 targetId")
	}
	defer func() {
		_, _ = cdpBrowserCallResult(debugPort, "Target.closeTarget", map[string]any{"targetId": targetID})
	}()

	wsURL, err := waitGPUProbeTargetWS(debugPort, targetID, 5*time.Second)
	if err != nil {
		return nil, err
	}
	result, err := cdpCallOnTarget(wsURL, "Runtime.evaluate", map[string]any{
		"expression":    gpuProbeExpression,
		"returnByValue": true,
	})
	if err != nil {
		return nil, err
	}
	inner, _ := result["result"].(map[string]any)
	text, _ := inner["value"].(string)
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("chrome://gpu 文本为空")
	}
	return parseGPUFeatureStatus(text), nil
}

// waitGPUProbeTargetWS 轮询 /json 直到出现该 targetId 的 WebSocket 地址(页面需要一点时间初始化)。
func waitGPUProbeTargetWS(debugPort int, targetID string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		body, err := cdpGetEndpointBody(debugPort, "/json")
		if err == nil {
			var targets []cdpTarget
			if json.Unmarshal(body, &targets) == nil {
				for _, t := range targets {
					if t.Id == targetID && t.WebSocketDebuggerUrl != "" {
						return t.WebSocketDebuggerUrl, nil
					}
				}
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return "", fmt.Errorf("等待 GPU 探测标签的调试地址超时")
}

// cdpCallOnTarget 在指定 target 的 WebSocket 上发一次命令并读回 result。
// 与 cdpCall 的区别:后者固定连"第一个 page",这里需要连指定标签。
func cdpCallOnTarget(wsURL string, method string, params map[string]any) (map[string]any, error) {
	conn, err := cdpDialWebSocket(wsURL)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(cdpWebSocketReadTimeout))
	if err := conn.WriteJSON(cdpMessage{Id: 1, Method: method, Params: params}); err != nil {
		return nil, err
	}
	var resp cdpResponse
	if err := conn.ReadJSON(&resp); err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("CDP 错误: %s", resp.Error.Message)
	}
	return resp.Result, nil
}

// LogGPUCapability 探测并记录结果;软解时打 Error 级日志,便于运维立刻发现。
func (a *App) LogGPUCapability(debugPort int) {
	log := logger.New("GPUProbe")
	status, err := a.ProbeGPUCapability(debugPort)
	if err != nil {
		log.Warn("GPU 能力探测失败", logger.F("port", debugPort), logger.F("error", err.Error()))
		return
	}
	if gpuHardwareDecodeOK(status) {
		log.Info("视频硬解已启用", logger.F("video_decode", status["Video Decode"]))
		return
	}
	log.Error("视频硬解未启用,多开 CPU 将显著升高",
		logger.F("video_decode", status["Video Decode"]),
		logger.F("hint", "检查 GPU 驱动黑名单;必要时评估 --ignore-gpu-blocklist"),
	)
}
```

> 注:`cdpTarget` 若无 `Id` 字段需在其结构体上补 `Id string \`json:"id"\``。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./backend/ -run 'TestParseGPUFeatureStatus|TestGPUHardwareDecodeOK' -v && go build ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/app_gpu_probe.go backend/app_gpu_probe_test.go
git commit -m "feat: probe chrome://gpu for hardware video decode and alert on software fallback"
```

---

### Task 12: CDP 主动内存回收

长时运行的直播实例会缓慢累积图片缓存、脚本缓存、已解码资源。Chromium 自身的内存压力回收链路**本来就会优先丢这些、保留正在播放的媒体**,所以不必自己实现优先级,主动触发它即可。

**Files:**
- Create: `backend/app_memory_reclaim.go`
- Test: `backend/app_memory_reclaim_test.go`
- Modify: `backend/internal/config/config.go`、`backend/app_startup.go`、`config.yaml`

**Interfaces:**
- Consumes: 现有 `cdpBrowserCall`、`a.browserMgr.List()`;复用 app_live_keepalive.go 的错峰循环模式。
- Produces: `Browser.MemoryReclaimEnabled *bool`(nil=默认开)、`Browser.MemoryReclaimIntervalMs int`(默认 600000)、`Browser.MemoryReclaimLevel string`(默认 `moderate`);`(a *App) startMemoryReclaim()`。

- [ ] **Step 1: 写失败测试**

```go
package backend

import (
	"testing"
	"time"

	"ant-chrome/backend/internal/config"
)

func TestMemoryReclaimDefaults(t *testing.T) {
	a := &App{config: &config.Config{}}
	if !a.memoryReclaimEnabled() {
		t.Error("should default on")
	}
	if got := a.memoryReclaimInterval(); got != 10*time.Minute {
		t.Errorf("default interval got %v", got)
	}
	if got := a.memoryReclaimLevel(); got != "moderate" {
		t.Errorf("default level got %q", got)
	}
}

func TestMemoryReclaimIntervalFloor(t *testing.T) {
	a := &App{config: &config.Config{}}
	a.config.Browser.MemoryReclaimIntervalMs = 1000 // 过小,应被抬到下限
	if got := a.memoryReclaimInterval(); got < time.Minute {
		t.Errorf("interval must be floored to >=1m, got %v", got)
	}
}

func TestMemoryReclaimLevelWhitelist(t *testing.T) {
	a := &App{config: &config.Config{}}
	a.config.Browser.MemoryReclaimLevel = "nuke"
	if got := a.memoryReclaimLevel(); got != "moderate" {
		t.Errorf("invalid level must fall back to moderate, got %q", got)
	}
	a.config.Browser.MemoryReclaimLevel = "critical"
	if got := a.memoryReclaimLevel(); got != "critical" {
		t.Errorf("critical should be accepted, got %q", got)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./backend/ -run TestMemoryReclaim -v`
Expected: FAIL

- [ ] **Step 3: 实现**

config.go BrowserConfig 增加:

```go
	MemoryReclaimEnabled    *bool  `yaml:"memory_reclaim_enabled,omitempty"`     // 定时主动内存回收(nil=默认开)
	MemoryReclaimIntervalMs int    `yaml:"memory_reclaim_interval_ms,omitempty"` // 回收间隔ms(默认 600000)
	MemoryReclaimLevel      string `yaml:"memory_reclaim_level,omitempty"`       // moderate(默认) | critical
```

新建 `backend/app_memory_reclaim.go`:

```go
package backend

import (
	"math/rand"
	"time"

	"ant-chrome/backend/internal/logger"
)

// 直播实例长时运行会缓慢累积图片/脚本/已解码资源缓存。这里定时向每个运行中实例发送一次
// CDP 内存压力通知,触发 Chromium 自身的回收链路 —— 它本就会优先丢缓存类内存、
// 保留正在播放的媒体缓冲,不需要我们自己实现优先级。
//
// 错峰:与保活同理,各实例独立随机首触发,避免上百实例同一刻一起回收造成 CPU 尖峰。
// 级别:默认 moderate。critical 回收更狠但可能让播放器出现短暂卡顿,需实测后再开。

const (
	memoryReclaimTickResolution = 30 * time.Second
	memoryReclaimFloor          = time.Minute
)

func (a *App) memoryReclaimEnabled() bool {
	if a == nil || a.config == nil {
		return true
	}
	if p := a.config.Browser.MemoryReclaimEnabled; p != nil {
		return *p
	}
	return true
}

func (a *App) memoryReclaimInterval() time.Duration {
	interval := 10 * time.Minute
	if a != nil && a.config != nil && a.config.Browser.MemoryReclaimIntervalMs > 0 {
		interval = time.Duration(a.config.Browser.MemoryReclaimIntervalMs) * time.Millisecond
	}
	if interval < memoryReclaimFloor {
		interval = memoryReclaimFloor
	}
	return interval
}

func (a *App) memoryReclaimLevel() string {
	if a != nil && a.config != nil && a.config.Browser.MemoryReclaimLevel == "critical" {
		return "critical"
	}
	return "moderate"
}

// startMemoryReclaim 启动后台回收循环(每次 app 启动调用一次),随 app 上下文取消而退出。
func (a *App) startMemoryReclaim() {
	if a == nil {
		return
	}
	log := logger.New("MemReclaim")
	var done <-chan struct{}
	if a.ctx != nil {
		done = a.ctx.Done()
	}
	next := map[string]time.Time{}
	go func() {
		ticker := time.NewTicker(memoryReclaimTickResolution)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case now := <-ticker.C:
				if !a.memoryReclaimEnabled() {
					continue
				}
				a.runMemoryReclaimDue(log, next, now)
			}
		}
	}()
}

func (a *App) runMemoryReclaimDue(log *logger.Logger, next map[string]time.Time, now time.Time) {
	if a.browserMgr == nil {
		return
	}
	interval := a.memoryReclaimInterval()
	level := a.memoryReclaimLevel()
	live := map[string]struct{}{}
	for _, p := range a.browserMgr.List() {
		if !p.Running || !p.DebugReady || p.DebugPort <= 0 {
			continue
		}
		live[p.ProfileId] = struct{}{}
		t, ok := next[p.ProfileId]
		if !ok { // 首次出现:在一个完整周期内随机首触发 → 天然错峰
			next[p.ProfileId] = now.Add(time.Duration(rand.Int63n(int64(interval) + 1)))
			continue
		}
		if now.Before(t) {
			continue
		}
		if err := cdpBrowserCall(p.DebugPort, "Memory.simulatePressureNotification", map[string]any{"level": level}); err != nil {
			log.Debug("内存回收通知发送失败", logger.F("profile", p.ProfileId), logger.F("error", err.Error()))
		}
		next[p.ProfileId] = now.Add(interval)
	}
	for id := range next { // 清理已停实例,避免 map 泄漏
		if _, ok := live[id]; !ok {
			delete(next, id)
		}
	}
}
```

`app_startup.go` 在 `a.startLiveKeepAlive()` 附近追加 `a.startMemoryReclaim()`。

`config.yaml` 的 `browser:` 段追加:

```yaml
    memory_reclaim_enabled: true
    memory_reclaim_interval_ms: 600000
    memory_reclaim_level: moderate
```

- [ ] **Step 4: 跑测试 + 编译**

Run: `go test ./backend/ -run TestMemoryReclaim -v && go build ./...`
Expected: PASS

- [ ] **Step 5: 真机验证(必做)**

`Memory.simulatePressureNotification` 是否在浏览器级 target 可用需实测。若返回 `'Memory.simulatePressureNotification' wasn't found`,改用逐标签 `HeapProfiler.collectGarbage`(经 `cdpCall`)。**验证时必须同时观察直播是否卡顿**,卡顿则把 level 保持 moderate 并拉长间隔。

- [ ] **Step 6: Commit**

```bash
git add backend/app_memory_reclaim.go backend/app_memory_reclaim_test.go backend/internal/config/config.go backend/app_startup.go config.yaml
git commit -m "feat: periodic staggered CDP memory pressure reclaim for long-running instances"
```

---

### Task 13: CDP 冒烟工具 + Windows 实测脚本

**Files:**
- Create: `backend/cmd/cdp-smoke/main.go`
- Create: `scripts/measure-live-perf.ps1`

- [ ] **Step 1: CDP 冒烟工具**

对一个已运行实例的调试端口跑通全部在用的 CDP 能力,任何一步失败即非零退出。改完启动参数后必跑。

```go
// Command cdp-smoke 对指定调试端口跑通项目实际使用的全部 CDP 能力。
// 用法: go run ./backend/cmd/cdp-smoke -port 9222
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

func main() {
	port := flag.Int("port", 0, "实例的 remote-debugging-port")
	flag.Parse()
	if *port <= 0 {
		fmt.Fprintln(os.Stderr, "必须指定 -port")
		os.Exit(2)
	}

	failed := 0
	check := func(name string, fn func() error) {
		if err := fn(); err != nil {
			fmt.Printf("FAIL  %-34s %v\n", name, err)
			failed++
			return
		}
		fmt.Printf("OK    %s\n", name)
	}

	for _, endpoint := range []string{"/json/version", "/json", "/json/list"} {
		ep := endpoint
		check("HTTP "+ep, func() error { _, err := httpGet(*port, ep); return err })
	}

	fmt.Println("\n下列域需在真实实例上人工确认(本工具只验 HTTP 面):")
	for _, m := range []string{
		"Browser.getWindowForTarget / setWindowBounds (窗口平铺)",
		"Target.createTarget (新标签)",
		"Page.navigate / Runtime.evaluate (窗口信息、GPU 探测)",
		"Input.dispatchMouseEvent (保活)",
		"Network.getAllCookies / clearBrowserCookies (Cookie 管理)",
		"Emulation.setGeolocationOverride / setLocaleOverride / setTimezoneOverride (地理对齐)",
		"Memory.simulatePressureNotification (内存回收, Task 12)",
	} {
		fmt.Println("  -", m)
	}

	if failed > 0 {
		fmt.Printf("\n%d 项失败\n", failed)
		os.Exit(1)
	}
	fmt.Println("\nCDP HTTP 面全部通过")
}

func httpGet(port int, endpoint string) ([]byte, error) {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d%s", port, endpoint))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if !json.Valid(body) {
		return nil, fmt.Errorf("响应非合法 JSON")
	}
	return body, nil
}
```

- [ ] **Step 2: Windows 实测脚本**

```powershell
# measure-live-perf.ps1 — 统计所有内核进程的内存/进程数,用于 live_perf 开/关对比。
# 用法:开 N 个直播窗口稳定 5 分钟后运行;改 live_perf_enabled 前后各测一次。
$ErrorActionPreference = 'SilentlyContinue'
$procs = Get-CimInstance Win32_Process | Where-Object { $_.CommandLine -match '--user-data-dir' }
$total = 0; $count = 0
$byProfile = @{}
foreach ($p in $procs) {
    $ws = (Get-Process -Id $p.ProcessId).WorkingSet64
    $total += $ws; $count++
    if ($p.CommandLine -match '--user-data-dir=([^\s"]+)') {
        $key = $Matches[1]
        $byProfile[$key] = [long]($byProfile[$key]) + $ws
    }
}
"进程数: $count"
"总内存: {0:N0} MB" -f ($total / 1MB)
"实例数: $($byProfile.Count)"
if ($byProfile.Count -gt 0) {
    "每实例均值: {0:N0} MB" -f ($total / 1MB / $byProfile.Count)
    "每实例进程均值: {0:N1}" -f ($count / $byProfile.Count)
}
$byProfile.GetEnumerator() | Sort-Object Value -Descending | Select-Object -First 10 | ForEach-Object {
    "{0:N0} MB  {1}" -f ($_.Value / 1MB), (Split-Path $_.Key -Leaf)
}
```

- [ ] **Step 3: Commit**

```bash
git add backend/cmd/cdp-smoke/main.go scripts/measure-live-perf.ps1
git commit -m "test: add CDP smoke tool and Windows live-perf measurement script"
```

---

## 五、交付验证清单(逐项打钩)

exe 系 mac 交叉编译,从未在真实 Windows 上运行过 —— **交付时须明确告知用户他们是首个运行者**。

1. **指纹零变化**:同一 profile 在 `live_perf_enabled` 开 / 关两种状态各启动一次,内置指纹自检页逐项比对**必须完全一致**;另在控制台确认 `performance.memory.jsHeapSizeLimit` 与优化前一致(验证 Task 4)、`navigator.plugins.length` 不变、`Notification.permission` 不变、`navigator.mediaCapabilities` 查询结果不变。
2. **CDP 全通**:`go run ./backend/cmd/cdp-smoke -port <端口>` 通过;人工确认新标签、窗口平铺、保活、Cookie 管理、地理对齐六项功能正常。
3. **硬解生效**:`chrome://gpu` 的 Video Decode 为 Hardware accelerated;开 5 窗直播后 `chrome://media-internals` 每窗解码器为 `D3D11VideoDecoder`,**不得**出现 FFmpeg/Vpx 软解。
4. **播放与看播**:全部窗口出画面;被遮挡窗口 `document.visibilityState === 'visible'`;挂 10 分钟无"长时间无操作"暂停。
5. **量化对比**:`live_perf_enabled` false / true 各跑 `measure-live-perf.ps1`,记录每实例内存均值与进程均值。预期每实例内存 −30~40%、进程数明显下降。
6. **内存回收无副作用**:Task 12 生效后连续观察 30 分钟,直播无卡顿、无播放器报错。
7. **功能回归**:代理经 xray 正常、手动测速可用、有扩展的实例扩展仍加载、已停实例缓存清理无报错。

---

## 六、后续可选 / 长期项

### D1:遮挡窗口跳过上屏(内核 patch)—— 潜在最大的一笔

**思路:** 保持 rAF 与视频解码正常运行,只跳过被遮挡窗口的 GPU 上屏提交。JS 无法直接观测像素是否到达屏幕,**不改指纹面**。

**必须先解决的坑:** `requestVideoFrameCallback` 与 `video.getVideoPlaybackQuality().droppedVideoFrames` 与"帧是否被呈现"挂钩,而平台 QoE 上报会读这些。粗暴跳过呈现会让 dropped frames 飙升 → 被平台看见。patch 必须保证**呈现记账正常、只省实际 GPU 工作**,分寸比听起来细。

**收益边界(诚实):** 省得掉合成与 overlay 开销,**省不掉解码**(停解码会让 `currentTime` 停,直接可检测)。属中等收益、高实现难度,建议在 W1/W2 做完、A 级参数上线后再评估。

### D2:异常实例 CPU 熔断(Windows Job Object)

给每个实例的进程树设 CPU 配额上限,防止单个异常直播页拖垮全场。**作为熔断合理,作为常态限速会掉帧**,故只在监控发现某实例持续超阈值时施加,并同时告警。

### D3:GN 编译期裁剪(需自行编译内核,依赖 W1)

可安全裁:`enable_print_preview=false`、`enable_service_discovery=false`、`safe_browsing_mode=0`、`enable_reporting=false`、`enable_supervised_users=false`、`enable_nacl=false`、`symbol_level=0`、`blink_symbol_level=0`。

**⛔ 绝对不能裁**:`enable_pdf`(动 `navigator.plugins` / `mimeTypes`)、`enable_widevine`(动 `requestMediaKeySystemAccess`,还可能直接播不了)、`enable_extensions`(动 `window.chrome` 面)。

---

## 七、运维指引(随交付说明发给使用者,不进代码)

1. **画质与弹幕是单实例 CPU 最大头**:每个账号首次进直播间时,手动选一次最低画质、关一次弹幕。平台把这两项记在该 profile 的 localStorage 里,重启不丢。零注入、零风控面,50 开时通常省 30~50% 总 CPU。
2. **显示器设 60Hz,不要 144Hz**:Chromium 按显示器刷新率合成,144Hz × N 窗口的合成开销是纯浪费。**不要设到 30Hz** —— rAF 频率跟随刷新率,30Hz 是异常信号。
3. **静音默认开**,需要听声再对单个实例"取消静音并重启"。
4. **开数标定**:用 `measure-live-perf.ps1` + `chrome://media-internals` 逐台机器标定硬解并发上限。超限的窗口会静默回落软解,CPU 陡增反而拖垮全场。每实例预算(A 级后):静音 + 480p 约 250~350MB 内存,按机器规格反推开数并留 20% 余量。
5. **激进模式**:确认 A 级稳定后,如需再压,`live_perf_aggressive: true` 重启实例生效。它牺牲站点隔离安全性、广告 iframe 崩溃会带走整页,仅建议在只开固定直播平台的专用机上用。
