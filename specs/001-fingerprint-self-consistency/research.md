# Research & Phase 0 决策 — 每环境指纹自洽引擎

**Date**: 2026-08-15 | **Branch**: `feature/fingerprint-self-consistency`

## Phase 0 关键技术决策(已定)

1. **内核**:fingerprint-chromium(Chrome 144+)。细粒度 flag(webgl-vendor/renderer、screen、device-memory、location)已废弃,改由 `--fingerprint` seed 内部派生。→ 引擎只序列化仍被接受的 flag(见 `identity/serializer.go`),其余交给 seed。
2. **地理定位**:Chrome 144+ 无对应启动 flag → 启动后经 CDP `Emulation.setGeolocationOverride`(+`setTimezoneOverride`/`setLocaleOverride` 兜底)注入。载荷构建已实现并测试(`identity/cdp.go` → `Identity.CDPOverrides()`)。**实网验证需内核跑起来一次。**
3. **指纹数据源**:离线内嵌真机分布指纹池,Go 端加权采样(`identity/pool.go` + `data/pool.json`)。引导池已内置;完整池由 `tools/identity/gen-pool.mjs` 用 fingerprint-suite 离线生成后同格式替换。
4. **地理数据源**:离线 GeoIP。推荐 **DB-IP City Lite**(CC-BY-4.0,免费可商用可再分发,带经纬度+时区)或 GeoLite2(需 MaxMind license key)。运行期只读离线。
5. **建模与唯一性**:结构化身份入库 + `fingerprint_hash`/`seed` 部分唯一索引(DB v15)+ 采样重采(`GenerateUnique`)。已实现并测试。
6. **范围排除**(保持内核原样,不处理):WebGPU、TLS/JA3/JA4、HTTP/2(JA4H)、HTTP/3/QUIC、JA4L。

## 已完成(已合入分支,全程 TDD)

引擎包 `backend/internal/identity/`:model(+FingerprintHash)、registry(GenerateUnique)、store_sqlite(Seen/Save/Load)、serializer(LaunchArgs)、pool(采样+BuildIdentity)、aligner(AlignToProxyGeo + GeoResolver 接口 + 国家兜底表)、validator(Validate)、cdp(CDPOverrides)、compat(FromLaunchArgs)。
DB:迁移 v15(browser_identities + 唯一索引)。
接线:`browser.IdentityService` + startup 注入 + `Manager.Create` 自动为新环境生成唯一自洽身份并写入 fingerprint_args(nil 安全,回退静态默认)。

## 剩余工作 + 外部依赖(精确接入点)

> 以下三类需要**外部产物**或**可运行的 app+内核**才能验证,故留待具备条件时完成。

### R1. 离线 GeoIP 解析器(激活代理地理对齐)—— 需 .mmdb 文件
- 加依赖 `github.com/oschwald/maxminddb-golang`。
- 新增 `backend/internal/identity/geoip.go` 实现 `identity.GeoResolver`:读 `.mmdb`,`Resolve(ip)` → `GeoInfo{CountryCode,City,Lat,Lon,Timezone,Accuracy}`。
- startup:若 `data/geoip/dbip-city-lite.mmdb` 存在则 `IdentityService.SetGeoResolver(...)`(接口/hook 已就位)。
- 外部依赖:下载 DB-IP City Lite `.mmdb` 放入 `data/geoip/`(一次,需联网/走代理)。

### R2. 代理绑定/换代理时自动重对齐 —— 接入点已定位
- 在 `backend/internal/browser/proxy_binding.go`(BindProfileToProxy)或 `app_proxy_location.go` 绑定后:探测代理出口 IP(复用 `backend/internal/proxy/iphealth.go` 取出口 IP)→ `resolver.Resolve(ip)` → `identity.AlignToProxyGeo(id, geo)` → 存库 + 刷新 fingerprint_args。
- `AlignToProxyGeo` 已实现并测试;仅差 R1 的 resolver 与调用点接线。

### R3. 启动前校验 + 启动后 CDP 注入地理 —— 需内核实网验证
- `backend/app_instance_start_prepare.go`:拼参前 `id := 加载/反解身份;res := identity.Validate(id)`;`res.OK==false` 则硬拦截并返回可修复项(一键修复=重新生成)。
- 实例 debug 端口就绪后:用既有 CDP 调用(`cdpBrowserCall` / launchcode 统一 CDP 入口)依次下发 `id.CDPOverrides()`。
- 载荷已测;**实网生效需下载内核 148 跑一次验证**(你的环境 GitHub 受限,内核下载可能走代理/海外服务器)。

### R4. 前端(React)—— 需可运行 app 验证
- `frontend/src/modules/browser/components/FingerprintPanel.tsx` / `pages/BrowserEditPage.tsx`:展示自动生成的身份摘要、一致性徽章(调用校验 API)、"重新生成"、"手动重对齐代理";原手动对齐按钮改为自动。
- 需配套后端 Wails API:`BrowserProfileRegenerateFingerprint(profileId)`、`BrowserProfileValidateFingerprint(profileId)`(薄封装 IdentityService/Validate)。

### R5. 完整指纹池 —— 需联网一次
- 运行 `node tools/identity/gen-pool.mjs`(需 `npm i fingerprint-generator`)生成上万条真机分布记录,替换 `backend/internal/identity/data/pool.json`(schema 相同,`go:embed` 自动生效)。

## 验证矩阵(对应 spec 的 SC)
- SC-001 唯一性:`TestPoolGeneratesUniqueIdentitiesAtScale`(已过)。
- SC-002 代理对齐:待 R1+R2 后加集成测试 + 指纹检测页复核。
- SC-003 一致性零漏放:`validator` 单测(已过)+ 上线前 Validate 拦截(R3)。
- SC-004 可复现:同一 profile 身份持久化(store 已过);启动复现待 R3 实网。
- SC-006 生成 <50ms:纯内存采样,当前规模远低于阈值。
