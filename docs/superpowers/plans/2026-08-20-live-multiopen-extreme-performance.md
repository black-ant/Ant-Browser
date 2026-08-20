# 直播多开极致性能优化 实施方案

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在指纹面、窗口隔离、直播可播性完全不变的前提下,把多开直播场景的每实例 CPU/内存/磁盘/网络消耗压到最低,并让闲置资源自动回收。

**Architecture:** 新增"直播性能模式"(`live_perf_enabled`,默认开)作为一层独立的启动参数/Preferences 叠加层,与现有指纹参数、MemorySaver 参数并存;修复多个 `--disable-features` 互相覆盖的合并 bug 使叠加层可靠生效;应用侧关停 API 日志拦截器、自动测速、降低前端轮询频率;新增已停实例缓存清理。

**Tech Stack:** Go (Wails backend)、fingerprint-chromium 144/148 内核、React/TS 前端。

## Global Constraints(每个任务隐含遵守)

**红线——以下内容任何任务都不得增删改(约束 1:指纹面零变化):**

- 一切 `--fingerprint-*`、`--disable-spoofing`、`--disable-non-proxied-udp`、`--fingerprinting-*` 参数。
- 一切 GPU/图形管线开关:`--disable-gpu`、`--use-gl`、`--use-angle`、`--enable/disable-gpu-rasterization`、`--enable-zero-copy`、`--force-color-profile` 等(动了会改 Canvas/WebGL 渲染输出)。
- `--js-flags`(`performance.memory.jsHeapSizeLimit` JS 可读,改堆上限=改指纹;本方案还要把它从现存 MemorySaverArgs 里**移除**)。
- `--autoplay-policy`(新内核有 `navigator.getAutoplayPolicy()`,JS 可见)。
- Preferences 里任何映射到 JS 权限状态的内容设置(如 notifications → `Notification.permission`)、Privacy Sandbox 类 JS 可见 API(如 Topics → `document.browsingTopics`)。
- UA、语言、时区、字体、WebRTC 相关现有逻辑。

**必须保留(约束 2:直播可播、看播时长有效):**

- `--disable-backgrounding-occluded-windows`、`--disable-renderer-backgrounding`、`--disable-background-timer-throttling`(app_instance_start_prepare.go:329-331,遮挡/后台窗口保持 `visibilityState=visible`,防止平台判定"未观看")。
- `--disable-features=CalculateNativeWinOcclusion`(同上,禁掉原生遮挡检测)。
- CDP 保活循环(app_live_keepalive.go)、`--mute-audio` 按实例静音逻辑。

**工程约束:**

- 新增 feature 名称以 148 内核为准;144 内核不认识的 feature 名 Chromium 会静默忽略,无害。
- 所有新增参数集必须有"违禁前缀"守卫测试(见 Task 2)。
- 每完成一个任务:`go test ./...` 通过后即 `git commit`(分支 `feature/fingerprint-self-consistency`),按 CLAUDE.md 交付流程 push,不打包。

---

## 方案总览(为什么是这些改动)

### 现状盘点(已核实的代码事实)

| 现状 | 位置 | 问题/机会 |
|---|---|---|
| `MemorySaverArgs()` 已存在但默认关,且含 `--js-flags=--max-old-space-size=512` | internal/browser/memory_saver.go | js-flags 改 `jsHeapSizeLimit`,是指纹面泄漏,必须移除 |
| 多个 `--disable-features=` 直接 append | app_instance_start_prepare.go:321 buildBrowserLaunchArgs | Chromium 同名开关**后者覆盖前者**,叠加层互相踩,必须合并 |
| API 日志拦截器全开(参数+结果全序列化) | config.yaml logging.interceptor | 每次 Wails 调用都做 JSON 序列化,纯浪费 |
| 前端实例列表每 2s 全量拉取 | frontend .../useBrowserListData.ts:132 | 已有事件驱动(started/stopped/crashed),轮询只需兜底,可降到 10s |
| 代理测速调度器每 30min 全量测速 | app_startup.go:214,internal/browser/proxy_speed.go | 直播挂机场景会周期性拉起代理内核+发探测流量,应默认关 |
| xray/singbox 桥按节点共享、引用计数、45s 空闲回收 | internal/proxy/xray.go:12 | 已达标,不动 |
| 每实例 Chromium 各自维护 Spare Renderer(备用渲染进程) | Chromium 默认行为 | N 开时每实例白养一个 ~80-150MB 进程,可关 |
| 磁盘缓存无上限 | 无 | 直播流缓存基本无复用价值,100 实例会吃满盘 |
| 已停实例的 Cache/GPUCache 永不清理 | 无 | 磁盘持续增长 |

### 改动分级

**A 级(默认开启,零指纹面、零播放影响)——本方案主体:**

1. **内核参数层** `LivePerfArgs()`:进程合并与上限(`--process-per-site`、`--renderer-process-limit=3`)、关停后台子系统(background networking / component update / breakpad+crashpad / domain reliability / phishing detection / default apps / hang monitor / metrics 仅记录不上报)、关停无用 feature(Translate、MediaRouter、OptimizationHints、OptimizationGuideModelDownloading、InterestFeedContentSuggestions、LiveCaption、GlobalMediaControls、HardwareMediaKeyHandling、SpareRendererForSitePerProcess、AutofillServerCommunication)、缓存上限(`--disk-cache-size=128MB`、`--media-cache-size=32MB`)、`--no-pings`、`--disable-back-forward-cache`(单页直播无前进后退,省整页缓存)。
2. **Preferences 层**(JS 不可见的浏览器服务开关):Safe Browsing 关、网络预测/预连接关、搜索建议关、翻译关、拼写检查关、密码泄漏检测关、备用错误页关。
3. **应用层**:日志拦截器关+级别降 warn、自动测速默认关(手动测速保留)、前端轮询 2s→10s(事件驱动为主)、无扩展实例加 `--disable-extensions`。
4. **闲置回收**:应用启动时自动清理"未运行实例"的 HTTP/GPU/着色器缓存目录(不碰 Cookie/LocalStorage/Service Worker 存储——那是账号状态)。

**B 级(`live_perf_aggressive`,默认关,给极限压榨留的开关):**

- `--disable-site-isolation-trials`:跨站 iframe(广告/埋点)并进同一渲染进程,每实例再省 1~4 个进程。JS 不可见,但牺牲站点隔离安全性;受控挂机墙可接受。
- `--disable-features=AudioServiceOutOfProcess`:音频服务并入浏览器主进程,每实例省一个进程(静音挂机时收益明显)。

**明确不做(评估过、否决):**

- `--in-process-gpu`:每实例可再省一个 GPU 进程,但 fingerprint 内核未验证稳定性,崩溃会带走整实例 → 不值。
- 限制 V8 堆 / 改 renderer 优先级(EcoQoS):前者改指纹,后者可能造成解码掉帧。
- 任何页面内注入(关弹幕/降画质的 JS/CSS 注入):属页面行为面,有被检测风险。改用**运维指引**:弹幕开关和画质选择由平台播放器记住(localStorage 按 profile 持久化),每个号手动设一次最低画质+关弹幕即永久生效,这是单实例 CPU 最大的合规削减手段。

### 预期收益(诚实估计,交付前按 Task 10 实测校准)

- 内存:每实例 A 级省 ~150–250MB(Spare Renderer ~80–150MB + 进程合并/上限 + bfcache 关),B 级再省 ~50–150MB。50 开量级即 7–20GB。
- CPU:后台子系统关停节省的是常量小头;**大头是视频解码与合成,靠三件事**——确认硬解生效(Task 10 核查清单)、静音、运维侧最低画质+关弹幕。
- 磁盘:缓存上限+已停实例清理,把 100 实例的缓存从无界压到 <16GB。
- 网络:去掉 component update / safe browsing / 预测预连接 / 自动测速的持续底噪。

### 容量与部署指引(非代码,写给运维)

- 硬解会话数是多开上限的硬约束:Intel iGPU 按吞吐(1080p30 约 15–20 路),NVIDIA 消费卡 NVDEC 并发有驱动限制;超限后新窗口回落软解,CPU 会陡增。用 Task 10 的核查步骤逐台标定。
- 每实例预算(A 级后):静音+480p 约 350–500MB 内存、0.3–1.0 核;按机器规格反推开数,留 20% 余量。
- exe 为 mac 交叉编译,从未在真实 Windows 跑过;首次在客户机验证时先 5 开观察 chrome://media-internals 与任务管理器,再放量。

---

## File Structure

- 新增 `backend/internal/browser/live_perf.go` + `live_perf_test.go` —— A/B 级参数集,唯一事实来源。
- 新增 `backend/browser_launch_args_features.go` + `_test.go` —— `--disable-features`/`--enable-features` 合并器。
- 新增 `backend/app_profile_cache_cleanup.go` + `_test.go` —— 已停实例缓存清理。
- 修改 `backend/internal/browser/memory_saver.go` —— 移除 `--js-flags`。
- 修改 `backend/internal/config/config.go`、`config_defaults.go` —— 新配置项。
- 修改 `backend/app_instance_start_prepare.go` —— 叠加 LivePerfArgs、feature 合并、无扩展禁用扩展。
- 修改 `backend/browser_cookie_pref.go` —— 性能 Preferences。
- 修改 `backend/app_startup.go` —— 测速调度器门控、启动时缓存清理。
- 修改 `frontend/src/modules/browser/pages/browserList/useBrowserListData.ts` —— 轮询降频。
- 修改 `config.yaml` —— 日志/开关默认值。
- 新增 `scripts/measure-live-perf.ps1` —— Windows 实测脚本。

---

### Task 1: `--disable-features` / `--enable-features` 合并器(前置修复)

Chromium 对重复出现的同名开关取**最后一个**,现在 fingerprint 参数、MemorySaver、用户自定义 LaunchArgs 里若各带一条 `--disable-features=`,前面的会被静默覆盖。本方案要再叠一层,必须先修这个。

**Files:**
- Create: `backend/browser_launch_args_features.go`
- Test: `backend/browser_launch_args_features_test.go`

**Interfaces:**
- Produces: `mergeFeatureSwitches(args []string) []string` —— 输入完整参数列表,把所有 `--disable-features=`/`--enable-features=` 各合并成一条(去重、保序、逗号连接,放在原第一条出现的位置),其余参数原样保留。

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
// Chromium 对同名开关只取最后一条,不合并会导致指纹层/性能层的 feature 列表互相覆盖。
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

修改 `backend/app_instance_start_prepare.go` 的 `buildBrowserLaunchArgs` 最后一行:

```go
	args = append(args, normalizeNonEmptyStrings(fingerprintLaunchArgs)...)
	args = append(args, sanitizedProfileLaunchArgs...)
	args = append(args, sanitizedExtraLaunchArgs...)
	args = mergeFeatureSwitches(args)
	return browser.BuildLaunchArgs(args, launchTargets)
```

- [ ] **Step 6: 全量测试 + 提交**

Run: `go test ./backend/... `
Expected: PASS

```bash
git add backend/browser_launch_args_features.go backend/browser_launch_args_features_test.go backend/app_instance_start_prepare.go
git commit -m "fix: merge duplicate --disable/enable-features switches (last-one-wins bug)"
```

---

### Task 2: `LivePerfArgs()` 直播性能参数集 + 违禁前缀守卫

**Files:**
- Create: `backend/internal/browser/live_perf.go`
- Test: `backend/internal/browser/live_perf_test.go`

**Interfaces:**
- Produces: `browser.LivePerfArgs() []string`(A 级)、`browser.LivePerfAggressiveArgs() []string`(B 级增量)。Task 4 在 prepare 阶段消费。

- [ ] **Step 1: 写失败测试(含守卫)**

```go
package browser

import (
	"strings"
	"testing"
)

func TestLivePerfArgsMustHaves(t *testing.T) {
	args := LivePerfArgs()
	must := []string{
		"--process-per-site",
		"--renderer-process-limit=3",
		"--disable-background-networking",
		"--disable-component-update",
		"--metrics-recording-only",
		"--disk-cache-size=134217728",
		"--media-cache-size=33554432",
	}
	joined := strings.Join(args, "\n")
	for _, m := range must {
		if !strings.Contains(joined, m) {
			t.Errorf("missing %s", m)
		}
	}
	if !strings.Contains(joined, "SpareRendererForSitePerProcess") {
		t.Error("must disable SpareRendererForSitePerProcess")
	}
	if !strings.Contains(joined, "CalculateNativeWinOcclusion") {
		t.Error("must keep CalculateNativeWinOcclusion disabled (visibilityState=visible for watch-time)")
	}
}

// 守卫:直播性能参数永不触碰指纹面/图形管线/播放行为面。
func TestLivePerfArgsForbiddenPrefixes(t *testing.T) {
	forbidden := []string{
		"--fingerprint", "--disable-spoofing", "--js-flags", "--disable-gpu",
		"--use-gl", "--use-angle", "--force-color-profile", "--autoplay-policy",
		"--enable-gpu-rasterization", "--disable-accelerated-video-decode",
		"--single-process", "--disable-javascript", "--mute-audio",
	}
	for _, set := range [][]string{LivePerfArgs(), LivePerfAggressiveArgs()} {
		for _, a := range set {
			for _, f := range forbidden {
				if strings.HasPrefix(a, f) {
					t.Errorf("forbidden arg %q in live perf set", a)
				}
			}
		}
	}
}

func TestLivePerfAggressiveArgs(t *testing.T) {
	joined := strings.Join(LivePerfAggressiveArgs(), "\n")
	if !strings.Contains(joined, "--disable-site-isolation-trials") {
		t.Error("aggressive set must merge cross-site iframes")
	}
	if !strings.Contains(joined, "AudioServiceOutOfProcess") {
		t.Error("aggressive set must fold audio service into browser process")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./backend/internal/browser/ -run TestLivePerf -v`
Expected: FAIL(未定义)

- [ ] **Step 3: 实现**

```go
package browser

// LivePerfArgs 返回"直播多开性能模式"(A 级)的 Chromium 启动参数。
//
// 铁律:此列表只允许出现【JS 不可见、不改 Canvas/WebGL/UA/媒体能力查询、不影响
// 直播播放与 visibilityState】的开关;新增前先过 live_perf_test.go 的违禁前缀守卫。
// feature 名以 148 内核为准,144 不认识的名字会被 Chromium 静默忽略(无害)。
func LivePerfArgs() []string {
	return []string{
		// 进程模型:合并与上限(进程数 JS 不可见)
		"--process-per-site",
		"--renderer-process-limit=3",

		// 后台子系统关停(全部为浏览器服务面,页面无感)
		"--disable-background-networking",
		"--disable-component-update",
		"--disable-breakpad",
		"--disable-crash-reporter",
		"--disable-domain-reliability",
		"--disable-client-side-phishing-detection",
		"--disable-default-apps",
		"--disable-hang-monitor", // 挂机墙无人值守,不要"页面无响应"弹窗
		"--metrics-recording-only",
		"--no-pings",

		// 单页直播:前进/后退整页缓存无价值,禁掉防内存累积
		"--disable-back-forward-cache",

		// 缓存上限:直播流缓存复用价值≈0,防止 N 实例吃满磁盘
		"--disk-cache-size=134217728", // 128MB
		"--media-cache-size=33554432", // 32MB

		// CalculateNativeWinOcclusion 必须保持禁用:遮挡窗口保持 visible,看播时长才有效。
		// SpareRendererForSitePerProcess:每实例的"备用渲染进程"~80-150MB,多开纯浪费。
		// 其余为翻译/投屏/推荐流/字幕/媒体全局控件/自动填充云端交互等无用后台 feature。
		"--disable-features=" +
			"CalculateNativeWinOcclusion," +
			"SpareRendererForSitePerProcess," +
			"Translate," +
			"MediaRouter," +
			"OptimizationHints," +
			"OptimizationGuideModelDownloading," +
			"InterestFeedContentSuggestions," +
			"LiveCaption," +
			"GlobalMediaControls," +
			"HardwareMediaKeyHandling," +
			"AutofillServerCommunication",
	}
}

// LivePerfAggressiveArgs 返回 B 级(激进)增量参数,默认不启用。
// 代价:放弃站点隔离安全性、音频服务并入主进程;受控挂机墙可接受,普通浏览不建议。
func LivePerfAggressiveArgs() []string {
	return []string{
		"--disable-site-isolation-trials", // 跨站 iframe(广告/埋点)并进同进程,每实例省 1-4 个进程
		"--disable-features=AudioServiceOutOfProcess", // 省一个音频服务进程(静音挂机收益明显)
	}
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./backend/internal/browser/ -run TestLivePerf -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/browser/live_perf.go backend/internal/browser/live_perf_test.go
git commit -m "feat: add LivePerfArgs live-streaming performance arg set with fingerprint guardrail tests"
```

---

### Task 3: 从 MemorySaverArgs 移除 `--js-flags`(指纹面修复)

`--js-flags=--max-old-space-size=512` 会把 `performance.memory.jsHeapSizeLimit` 从常规 ~4GB 改成 512MB,任何检测脚本一读便知,违反约束 1。进程内存控制交给 Task 2 的进程上限方案。

**Files:**
- Modify: `backend/internal/browser/memory_saver.go:20`
- Test: `backend/internal/browser/memory_saver_test.go`

- [ ] **Step 1: 改守卫测试(在 memory_saver_test.go 追加)**

```go
func TestMemorySaverArgsNoJSFlags(t *testing.T) {
	for _, a := range MemorySaverArgs() {
		if strings.HasPrefix(a, "--js-flags") {
			t.Fatalf("--js-flags 会改 performance.memory.jsHeapSizeLimit(JS 可见指纹面): %s", a)
		}
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./backend/internal/browser/ -run TestMemorySaverArgsNoJSFlags -v`
Expected: FAIL

- [ ] **Step 3: 删除 memory_saver.go 中整行**

```go
		"--js-flags=--max-old-space-size=512", // 每渲染进程 V8 老生代上限(足够重型 SPA/电商)
```

删除,并在函数 doc 注释末尾追加一行:

```go
// 注意:不得加 --js-flags——它会改 performance.memory.jsHeapSizeLimit,属 JS 可见指纹面。
```

- [ ] **Step 4: 跑测试确认通过(含既有测试)**

Run: `go test ./backend/internal/browser/ -v -run 'TestMemorySaver'`
Expected: PASS(若既有测试断言了 js-flags 存在,同步删除该断言)

- [ ] **Step 5: Commit**

```bash
git add backend/internal/browser/memory_saver.go backend/internal/browser/memory_saver_test.go
git commit -m "fix: drop --js-flags from MemorySaverArgs (JS-visible jsHeapSizeLimit fingerprint leak)"
```

---

### Task 4: 配置项 + 启动装配接线

**Files:**
- Modify: `backend/internal/config/config.go`(BrowserConfig 结构体,`MemorySaverEnabled` 字段旁)
- Modify: `backend/internal/config/config_defaults.go`(默认值)
- Modify: `backend/app_instance_start_prepare.go:174-176`(prepareBrowserStartPlan)
- Modify: `config.yaml`
- Test: `backend/app_instance_launch_args_liveperf_test.go`(新建)

**Interfaces:**
- Consumes: Task 2 的 `browser.LivePerfArgs()` / `browser.LivePerfAggressiveArgs()`。
- Produces: 配置字段 `Browser.LivePerfEnabled *bool`(nil=默认开)、`Browser.LivePerfAggressive bool`(默认关)。

- [ ] **Step 1: config.go 增加字段(加在 MemorySaverEnabled 之后)**

```go
	LivePerfEnabled    *bool `yaml:"live_perf_enabled,omitempty"`    // 直播多开性能模式(nil=默认开):LivePerfArgs 叠加层
	LivePerfAggressive bool  `yaml:"live_perf_aggressive,omitempty"` // B级激进模式(默认关):站点隔离合并+音频服务并入
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
	cfg := &config.Config{}
	args := appendLivePerfArgs(nil, cfg)
	if !strings.Contains(strings.Join(args, "\n"), "--process-per-site") {
		t.Fatal("live perf args should apply when unset (default on)")
	}
	if strings.Contains(strings.Join(args, "\n"), "--disable-site-isolation-trials") {
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

- [ ] **Step 4: 实现 appendLivePerfArgs 并接线**

在 `backend/app_instance_start_prepare.go` 中(muteAudioLaunchArgs 附近)新增:

```go
// appendLivePerfArgs 按配置叠加"直播多开性能模式"参数(nil=默认开)。
// 参数集本体与红线守卫见 internal/browser/live_perf.go。
func appendLivePerfArgs(args []string, cfg *config.Config) []string {
	if cfg != nil {
		if p := cfg.Browser.LivePerfEnabled; p != nil && !*p {
			return args
		}
	}
	args = append(args, browser.LivePerfArgs()...)
	if cfg != nil && cfg.Browser.LivePerfAggressive {
		args = append(args, browser.LivePerfAggressiveArgs()...)
	}
	return args
}
```

并把 `prepareBrowserStartPlan` 中:

```go
	if a.config != nil && a.config.Browser.MemorySaverEnabled {
		sanitizedExtraLaunchArgs = append(sanitizedExtraLaunchArgs, browser.MemorySaverArgs()...)
	}
```

之后追加一行:

```go
	sanitizedExtraLaunchArgs = appendLivePerfArgs(sanitizedExtraLaunchArgs, a.config)
```

(顺序在 MemorySaver 之后、mute-audio 之前;Task 1 的 mergeFeatureSwitches 会在出口统一合并 feature 列表。)

- [ ] **Step 5: config.yaml 显式声明(文档化默认值)**

在 `browser:` 段 `light_start_enabled: true` 下追加:

```yaml
    live_perf_enabled: true
    live_perf_aggressive: false
```

- [ ] **Step 6: 跑测试 + 提交**

Run: `go test ./backend/... `
Expected: PASS

```bash
git add backend/internal/config/config.go backend/internal/config/config_defaults.go backend/app_instance_start_prepare.go backend/app_instance_launch_args_liveperf_test.go config.yaml
git commit -m "feat: wire live_perf_enabled/live_perf_aggressive config into browser launch args"
```

---

### Task 5: 无扩展实例追加 `--disable-extensions`

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
	argsWithExt := buildBrowserLaunchArgs("/tmp/x", 9222, "", []string{"/ext/a"}, nil, nil, nil, nil, false)
	if argsContain(argsWithExt, "--disable-extensions") {
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

- [ ] **Step 3: 实现(改扩展分支为 if/else)**

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

### Task 6: 性能 Preferences(浏览器服务开关,JS 不可见)

**Files:**
- Modify: `backend/browser_cookie_pref.go`(ensureLaunchPreferences)
- Modify: 两处调用点 `backend/app_instance_start_prepare.go:243`、`backend/app_instance_launch_args.go:157`
- Test: `backend/browser_cookie_pref_test.go`(若无则新建)

红线提醒:**不得**写任何 `profile.default_content_setting_values.*`(会改 `Notification.permission` 等 JS 可见权限状态)。

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
	sb, _ := prefs["safebrowsing"].(map[string]any)
	if sb == nil || sb["enabled"] != false {
		t.Error("safebrowsing.enabled should be false")
	}
	net, _ := prefs["net"].(map[string]any)
	if net == nil || net["network_prediction_options"] != float64(2) {
		t.Error("net.network_prediction_options should be 2 (no prefetch/preconnect)")
	}
	if _, hasContentSettings := prefs["default_content_setting_values"]; hasContentSettings {
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

- [ ] **Step 3: 实现——ensureLaunchPreferences 加 livePerf 参数,函数体尾部(marshal 前)追加**

签名改为 `func ensureLaunchPreferences(userDataDir string, livePerf bool)`,并在 `prefs["browser"] = browser` 之后插入:

```go
	if livePerf {
		// 直播性能模式:关掉纯后台的浏览器服务(均为浏览器面,JS 不可见)。
		// 红线:不得写 default_content_setting_values(权限状态 JS 可读,属指纹面)。
		prefs["safebrowsing"] = mergeSubMap(prefs, "safebrowsing", map[string]interface{}{
			"enabled": false,
		})
		prefs["net"] = mergeSubMap(prefs, "net", map[string]interface{}{
			"network_prediction_options": 2, // 永不预取/预连接
		})
		prefs["search"] = mergeSubMap(prefs, "search", map[string]interface{}{
			"suggest_enabled": false,
		})
		prefs["translate"] = mergeSubMap(prefs, "translate", map[string]interface{}{
			"enabled": false,
		})
		prefs["spellcheck"] = mergeSubMap(prefs, "spellcheck", map[string]interface{}{
			"use_spelling_service": false,
		})
		prefs["credentials_enable_service"] = false // 密码保存气泡+泄漏检测网络调用
		prefs["alternate_error_pages"] = mergeSubMap(prefs, "alternate_error_pages", map[string]interface{}{
			"enabled": false,
		})
		profileSub := prefs["profile"].(map[string]interface{})
		profileSub["password_manager_leak_detection"] = false
	}
```

辅助函数(同文件):

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

两处调用点改为:

```go
	ensureLaunchPreferences(userDataDir, livePerfPrefsEnabled(a.config))
```

并新增:

```go
// livePerfPrefsEnabled 与 appendLivePerfArgs 同一开关:nil=默认开。
func livePerfPrefsEnabled(cfg *config.Config) bool {
	if cfg == nil {
		return true
	}
	if p := cfg.Browser.LivePerfEnabled; p != nil {
		return *p
	}
	return true
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./backend/ -run TestEnsureLaunchPreferences -v && go build ./...`
Expected: PASS,编译通过(两处调用点都已更新)

- [ ] **Step 5: Commit**

```bash
git add backend/browser_cookie_pref.go backend/browser_cookie_pref_test.go backend/app_instance_start_prepare.go backend/app_instance_launch_args.go
git commit -m "feat: disable safebrowsing/prefetch/suggest/translate/spellcheck/leak-detection prefs in live perf mode"
```

---

### Task 7: 关停应用侧底噪——日志拦截器、自动测速

**Files:**
- Modify: `config.yaml`(logging 段)
- Modify: `backend/internal/config/config.go`(ProxyCheck 或 Browser 段加 `SpeedAutoTestEnabled`)
- Modify: `backend/app_startup.go:214`(startupInitSpeedScheduler)
- Test: `backend/app_startup_speedsched_test.go`(新建,纯函数测试)

- [ ] **Step 1: config.yaml 日志降噪(纯配置,无代码)**

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

- [ ] **Step 2: 写失败测试(测速门控解析)**

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

config.go BrowserConfig 增加(LivePerfAggressive 之后):

```go
	SpeedAutoTestEnabled *bool `yaml:"speed_autotest_enabled,omitempty"` // 代理后台自动测速(nil=默认关;手动测速不受影响)
```

app_startup.go:

```go
// speedAutoTestEnabled 后台自动测速开关:直播挂机场景默认关(会周期拉起代理内核+发探测流量),
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

`startupInitSpeedScheduler` 尾部 `a.speedScheduler.Start()` 改为:

```go
	if speedAutoTestEnabled(a.config) {
		a.speedScheduler.Start()
	}
```

(调度器仍构造,前端手动 RunOnce 路径保持可用。`app_backup_runtime.go:72` 的重建路径同样加此门控。)

- [ ] **Step 5: 跑测试 + 提交**

Run: `go test ./backend/... `
Expected: PASS

```bash
git add config.yaml backend/internal/config/config.go backend/app_startup.go backend/app_backup_runtime.go backend/app_startup_speedsched_test.go
git commit -m "perf: gate background proxy speed tests behind opt-in; silence API interceptor logging"
```

---

### Task 8: 前端实例列表轮询降频

事件(`browser:instance:started/updated/stopped/crashed`)已是主通道,2s 全量轮询只是兜底 → 降到 10s,且无运行中实例时跳过运行时同步。

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

### Task 9: 已停实例缓存自动清理(闲置磁盘回收)

只清 HTTP/编译/GPU 缓存目录;**绝不**触碰 Cookies、Local Storage、IndexedDB、Service Worker(账号状态与站点可见存储)。

**Files:**
- Create: `backend/app_profile_cache_cleanup.go`
- Test: `backend/app_profile_cache_cleanup_test.go`
- Modify: `backend/app_startup.go`(启动时对未运行实例执行一次)

**Interfaces:**
- Produces: `cleanProfileCaches(userDataDir string) (freedBytes int64, err error)`(纯函数,可单测);`(a *App) CleanStoppedProfileCaches() int64`(Wails 导出,前端可加"清理缓存"按钮,本方案先不做 UI)。

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
	cacheFile := mustWrite("Default/Cache/Cache_Data/f_000001")
	codeCache := mustWrite("Default/Code Cache/js/a_0")
	gpuCache := mustWrite("Default/GPUCache/data_0")
	shader := mustWrite("GrShaderCache/data_0")
	cookies := mustWrite("Default/Cookies")
	localStorage := mustWrite("Default/Local Storage/leveldb/000003.log")
	sw := mustWrite("Default/Service Worker/CacheStorage/x/index")

	freed, err := cleanProfileCaches(dir)
	if err != nil {
		t.Fatal(err)
	}
	if freed <= 0 {
		t.Fatal("expected freed bytes > 0")
	}
	for _, gone := range []string{cacheFile, codeCache, gpuCache, shader} {
		if _, err := os.Stat(gone); !os.IsNotExist(err) {
			t.Errorf("cache path should be removed: %s", gone)
		}
	}
	for _, keep := range []string{cookies, localStorage, sw} {
		if _, err := os.Stat(keep); err != nil {
			t.Errorf("account/site state must be untouched: %s", keep)
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
// 红线:不得加入 Cookies/Local Storage/IndexedDB/Service Worker——那是账号状态与站点可见存储,
// 删了轻则掉登录,重则形成"缓存被清空"的行为异常面。
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

### Task 10: Windows 实测脚本 + 验证清单(交付门槛)

**Files:**
- Create: `scripts/measure-live-perf.ps1`

- [ ] **Step 1: 写测量脚本**

```powershell
# measure-live-perf.ps1 — 统计所有 ZwBrowser 内核进程的内存/CPU 总量,用于 live_perf A/B 对比。
# 用法:开 N 个直播窗口稳定 5 分钟后运行;改 live_perf_enabled 前后各测一次。
$ErrorActionPreference = 'SilentlyContinue'
$procs = Get-CimInstance Win32_Process | Where-Object {
    $_.CommandLine -match '--user-data-dir' -and $_.CommandLine -match 'fingerprint|ant|zw'
}
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
if ($byProfile.Count -gt 0) { "每实例均值: {0:N0} MB" -f ($total / 1MB / $byProfile.Count) }
$byProfile.GetEnumerator() | Sort-Object Value -Descending | Select-Object -First 10 | ForEach-Object {
    "{0:N0} MB  {1}" -f ($_.Value / 1MB), (Split-Path $_.Key -Leaf)
}
```

- [ ] **Step 2: 写验证清单(追加到本文件末尾,交付时逐项打钩)**

交付前必须在真实 Windows 机器逐项确认(exe 系 mac 交叉编译,用户是首个运行者,交付时明示):

1. **指纹零变化**:同一 profile 在 live_perf 开/关两种状态各启动一次,跑内置指纹自检页,逐项比对必须完全一致;另在检测页控制台确认 `performance.memory.jsHeapSizeLimit` 与优化前一致(验证 Task 3)。
2. **硬解生效**:开 5 窗直播,`chrome://media-internals` 每窗的 video decoder 应为 `D3D11VideoDecoder`(或 VDAVideoDecoder),**不得**出现 FFmpegVideoDecoder/VpxVideoDecoder(软解);任务管理器 GPU"Video Decode"占用应随窗口数上升。
3. **播放与看播**:全部窗口出画面、被遮挡窗口 `document.visibilityState === 'visible'`、挂 10 分钟无"长时间无操作"暂停(keepalive 正常)。
4. **量化对比**:`live_perf_enabled: false` vs `true` 各跑 measure-live-perf.ps1,记录每实例均值与进程数;预期每实例 −150MB 以上、进程数明显下降。
5. **功能回归**:代理经 xray 正常、手动测速可用、扩展实例(如有)扩展仍加载、停止实例后启动期缓存清理无报错。

- [ ] **Step 3: Commit**

```bash
git add scripts/measure-live-perf.ps1 docs/superpowers/plans/2026-08-20-live-multiopen-extreme-performance.md
git commit -m "docs: add live perf measurement script and delivery verification checklist"
```

---

## 运维指引(不进代码,随交付说明发给使用者)

1. **单实例 CPU 最大头是画质与弹幕**:每个账号首次登录后,在平台播放器里手动选最低画质、关弹幕——平台把这两项记在该 profile 的 localStorage 里,重启不丢。这是合规(无注入、无行为面)且收益最大的一步,50 开时通常能省 30–50% 总 CPU。
2. **静音默认开**(实例 MuteAudio),需要听声再对单个实例"取消静音并重启"。
3. **开数标定**:按 Task 10 脚本 + chrome://media-internals 逐台机器标定硬解上限,超过上限的窗口会回落软解,CPU 陡增反而拖垮全场。
4. **激进模式**:确认 A 级稳定后,如需再压,`live_perf_aggressive: true` 重启实例生效;它牺牲站点隔离安全性,仅建议在只开直播平台的专用机上用。
