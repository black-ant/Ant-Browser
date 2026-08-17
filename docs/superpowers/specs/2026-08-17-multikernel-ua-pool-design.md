# 多内核 + UA 随真实内核 + 身份池数据规范化 — 设计

日期:2026-08-17
分支:`feature/fingerprint-self-consistency`

## 背景与动机

ZwBrowser 所有实例目前都运行同一个 fingerprint-chromium **148** 内核,但身份池 `pool.json`
里 95.8% 的 UA 版本是 145/146/147。之前刻意做"内核版本多样性"(UA 显示不同版本号),
但因为只有一个引擎,这属于"145 UA + 148 引擎"的**版本撒谎**:普通网站看 UA 察觉不到,
高级风控(抖音/TikTok、Datadome、FingerprintJS Pro、CreepJS)会交叉验证 UA 声称的版本
与引擎实际暴露的 JS/CSS/V8 能力,判出 UA 与引擎不符,静默降低账号信任分。

### 关键事实(经代码/数据核实)

- **序列化器只下发** `--user-agent` / `--fingerprint-brand-version` / `--fingerprint-hardware-concurrency`
  / `--window-size` / `--lang` / `--timezone` 等;**不下发** `deviceMemory` 与 `screen/dpr`
  —— Chrome 144+ 这些由 `--fingerprint=<seed>` 内部派生
  (`backend/internal/identity/serializer.go:10`)。
  → 池里 `deviceMemory=16/32`、畸形 DPR 是**死数据**,不会暴露给页面 JS。
  → **唯一真实下发且会暴露的硬件字段是 `hardwareConcurrency`**。
- fingerprint-chromium 真实可下载版本:**148 / 144 / 142 / 139 / 138 …**,不存在 145/146/147。
  代码对 **≥144** 有完整支持路径(`chrome144Plus`),完整支持的是 **148 与 144**。
- 多内核基础设施已存在:`browser_cores` 表 + `scanChromeDir` 按子目录自动注册内核
  (`backend/app_utils.go:179`);实例通过 `profile.CoreId` 绑定内核;内核版本由
  `GetChromeVersion()` 读 `manifest.json` 得到(`backend/internal/browser/core_info.go:11`)。
- 存量 100 个实例由用户手动删除后重建,**因此不需要"就地自愈"逻辑**。

## 目标

1. 内置 **两个真实内核 148 + 144**,实例 UA = 其实际运行内核的真实版本(诚实的版本多样性)。
2. 新建实例按 **148:144 ≈ 70:30** 加权分配内核(贴近真实 Chrome 人群:绝大多数最新版 + 少数未更新尾部)。
3. UA/UA-CH 由内核真实版本驱动,换内核自动跟随,永不再撒谎。
4. 规范化身份池:`hardwareConcurrency` 只用真实偶数值;顺带清洁死数据(dm≤8、干净 dpr)、剔除垃圾记录。
5. 加校验,防止上述异常再次进入池或下发。

## 非目标

- 不为存量实例做就地自愈(用户手动删除重建)。
- 不引入 142 及更低版本内核(走 legacy 路径,风险/收益不划算)。
- 不改变现有 seed→canvas/webgl/字体 的派生机制。

## 架构与组件

### 1. 内核/核心层(多内核内置)

- **安装包打包结构**改为版本化子目录:
  ```
  chrome/
    148/   ← fingerprint-chromium-148.0.7778.215(chrome.exe + manifest.json …)
    144/   ← fingerprint-chromium-144.0.7559.132
  ```
  `scanChromeDir` 已能把每个子目录识别为 `core-148` / `core-144` 并自动注册(无需新机制)。
- **默认核心选择改为"最高版本优先"**:`scanChromeDir` 现在用 `len(cores)==0`(即 ReadDir
  的字典序首个)当默认,会把 `144` 选成默认。改为解析子目录版本,取**最高版本**为默认(148)。
- NSIS(`build/windows/installer/project.nsi`)与打包脚本:把两个内核分别 staged 到
  `chrome/148`、`chrome/144`;构建服务器缓存新增 `fingerprint-chromium-144`。

### 2. 版本分配(新建实例)

- 每个 profile 的 `CoreId` 在新建时按加权随机分配:`core-148`(权重 7)/ `core-144`(权重 3),
  比例由 `config.yaml` 的 `browser.core_distribution`(默认 `{"148":70,"144":30}`)控制。
- 批量新建弹窗(`BatchCreateModal.tsx`)新增**"内核版本"选择**,与现有平台选择并列:
  - `自动分布(148为主,推荐)` — 默认,按权重逐个分配。
  - `全部 148` / `全部 144` — 全部指定。
- 加权分配用**确定性**方式(按实例序号 + 权重),保证一批内可复现、可测(~70/30)。
- 单个新建默认走"自动分布"。

### 3. UA ↔ 真实内核绑定(两层,互为保险)

- 新增 helper `identity.BuildReducedUA(platform, major)`:返回平台对应的 Chrome UA Reduction
  形式(`Chrome/<major>.0.0.0`),Windows→`Windows NT 10.0; Win64; x64`,macOS→
  `Macintosh; Intel Mac OS X 10_15_7`。同时提供 `BuildBrandVersion(major)`(`<major>.0.0.0`)。
- **分配身份时**(`RegenerateForPlatform` 路径):拿到 profile 的 core 大版本,把生成身份的
  `uaFull`/`brandVersion` 用上述 helper 重写 → DB/UI 立刻与内核一致。
- **启动时**(`buildBrowserFingerprintLaunchPlan`,`backend/app_browser_fingerprint_matrix.go:176`,
  已持有 `chromeVersion`):把 `--user-agent` 与 `--fingerprint-brand-version` 覆盖成**内核真实
  大版本**。这是权威来源:换内核(148→149)UA 自动跟随,身份无需重生成;池里 UA 版本从此非权威。

### 4. 身份池数据规范化(`tools/identity/gen-pool.mjs` + 重生成 `pool.json`)

- `hardwareConcurrency`(**唯一真下发**):只取**偶数**,加权真实集合——
  主 `{4,6,8,12,16}`、尾 `{2,10,14,20,24}`;剔除全部奇数与 36/40/44/64/**640** 等异常。
  生成器对采样值做"就近映射到策略集合"处理。
- `deviceMemory`(死数据,清洁化):钳到 `≤8`,取 `{4,8}`(与 hc 档位弱相关)。
- `dpr`(死数据):归一到干净集合 `{1,1.25,1.5,1.75,2,2.5}`。
- 剔除 UA 含 `CCleaner`、非 Reduction 全版本(如 `Chrome/147.0.7727.56`)等垃圾记录。
- UA 版本字段仍写规范 Reduction 形式(虽启动会覆盖,保证 UI/DB 干净);平台仍限 windows/macos。

### 5. 校验(`backend/internal/identity/validator.go`)

`ValidatePoolRecord` 新增(违反即 error):
- `hardwareConcurrency` 必须为偶数且 ∈ `[2,32]`。
- `deviceMemory` ∈ `{1,2,4,8}`(若为 0 视为未设,跳过)。
- UA 大版本 ∈ 已内置内核集合 `{148,144}`(集合由常量定义,随内置内核演进)。

启动计划断言:覆盖后 `UA 大版本 == 内核大版本`(不一致记 error 日志并以内核版本为准)。

### 6. 存量实例

用户手动删除现有 100 个,再用修好后的批量流程重建。无代码改动。

### 7. 交付

- 构建服务器下载 `fingerprint-chromium-144`(Windows 版,tag `144.0.7559.132`)入内核缓存。
- 重打 Windows 安装包(内置 148+144,体积约 500–600MB);**顺带把已提交未发布的保活多标签
  修复(`8fd7a74`)一并打进去**,一次重建覆盖两件事。
- commit + push 走既定流程。

## 数据流

```
新建实例
  → 分配 CoreId(加权 148/144 或用户指定)
  → 生成平台+内核一致的身份(偶数 hc、清洁值,UA/brand=core 大版本)
  → 逐个去重(browser_identities UNIQUE)→ 保存

启动实例
  → ResolveChromeBinary(按 CoreId 选内核二进制)
  → GetChromeVersion(读该内核 manifest)→ chromeVersion
  → buildBrowserFingerprintLaunchPlan:覆盖 --user-agent / --fingerprint-brand-version = 内核大版本
  → 组装完整 argv 启动
```

## 测试策略

- `BuildReducedUA` / `BuildBrandVersion`:各平台 × {148,144} 输出精确匹配。
- 加权分配:N 个实例里 core-148 ≈ 70%、core-144 ≈ 30%(容差内),确定性可复现。
- 池策略:重生成后 `pool.json` 用断言校验 **0 条**违规(偶数 hc、hc∈[2,32]、dm≤8、无垃圾 UA、UA 版本∈{148,144})。
- validator 新规则:构造奇数 hc / dm=16 / UA=145 的记录 → 期望 error。
- 启动覆盖:给定 profile(池 UA=旧版本)+ core 148 → 计划里 `--user-agent` 大版本==148。
- scanChromeDir:构造 `chrome/148`+`chrome/144` → 注册两核心且 **148 为默认**。
- 回归:保活多标签测试仍通过。

## 风险与权衡

- **安装包体积**:内置两内核使安装包约翻倍(用户已确认接受"全部内置")。
- **144 legacy 边界**:144 恰在 `chrome144Plus` 支持线上,零 legacy 风险;不下探 142。
- **版本分布真实性**:70/30 贴近真实人群;比例可配,后续可按需微调。
- **UA 覆盖优先级**:启动覆盖是权威值,即使身份/池数据陈旧也能保证 UA==引擎,天然抗漂移。
