# Feature Specification: 每环境指纹自洽引擎 (Per-Environment Fingerprint Self-Consistency Engine)

**Feature Branch**: `feature/fingerprint-self-consistency`

**Created**: 2026-08-15

**Status**: Draft

**Input**: 在 Ant-Browser-Plus(Go + Wails,驱动 fingerprint-chromium 内核的多环境管理器)基础上二次开发,使**每个浏览器环境的指纹参数独立、不重复、且自洽**——尤其时区/语言/地理定位自动与所绑代理 IP 一致,并在启动前做一致性校验。

---

## 背景与目标 (Overview)

当前基座把指纹完全交给 fingerprint-chromium,存储形态是一串 Chromium 命令行 flag(`fingerprint_args`),"生成"只是从 8 个固定预设/5 个 persona 里挑(平台/时区/分辨率写死),且时区/语言与代理的对齐是**手动按钮 + 只覆盖 tz/lang**。这导致两个问题:多个环境会撞相同的平台/时区/分辨率组合(谈不上"不重复 + 真实"),且 IP 与时区/地理不匹配(头号封号点)。

本特性新增一个 **Identity Engine(身份引擎)**,在创建环境 / 绑定代理 / 启动 三个时机,自动产出一个唯一、真实分布、且与代理 IP 自洽的完整身份,并在启动前逻辑校验、启动后经 CDP 注入地理定位。**运行时全离线**(指纹池 + GeoIP 内嵌),契合受限网络环境。

---

## User Scenarios & Testing *(mandatory)*

### User Story 1 - 创建环境即得到唯一且自洽的身份 (Priority: P1)

运营人员点"新建配置",无需手工调指纹,系统自动为该环境生成一套**内部自洽**(平台↔UA↔品牌↔硬件↔屏幕彼此不矛盾)、且**与其他环境不重复**的指纹身份,并可复现。

**Why this priority**: 这是用户第一诉求"每创建一个环境都是独立不重复并且自洽的参数"的核心;没有它其余都无从谈起。

**Independent Test**: 连续创建 N 个环境,校验:每个身份内部一致(用一致性校验器判定)、任意两个环境的 seed 与关键指纹 hash 均不相同;同一环境重复启动,指纹保持不变。

**Acceptance Scenarios**:

1. **Given** 一个新环境,**When** 创建完成,**Then** 系统已为其持久化一套结构化身份(平台/品牌+版本/UA/硬件并发/屏幕+窗口/语言/时区/seed 等),且该身份通过一致性校验。
2. **Given** 已存在若干环境,**When** 再创建一个,**Then** 新环境的 seed 与关键指纹 hash 不与任何现存环境重复。
3. **Given** 一个已创建的环境,**When** 两次启动,**Then** 两次呈现的指纹完全一致(可复现)。

---

### User Story 2 - 时区/语言/地理自动对齐代理出口 (Priority: P1)

给环境绑定或更换代理后,系统自动把该环境的**时区、语言/locale、地理定位坐标**对齐到代理**真实出口 IP** 的地理归属,无需手动点"对齐"。

**Why this priority**: "自洽"最难也最关键的一半;IP 与时区/地理不匹配是最直接的封号触发点。与 US1 并列为 MVP。

**Independent Test**: 绑定一个已知国家/城市的代理(如美国纽约出口),校验身份的 timezone 变为 `America/New_York`、语言含 `en-US`、地理坐标落在该城市;换成德国代理后再次校验对应变化。

**Acceptance Scenarios**:

1. **Given** 一个环境绑定了某国代理,**When** 绑定完成,**Then** 系统探测代理真实出口 IP,经离线 GeoIP 解析出国家/城市/经纬度/时区,并据此更新身份的 timezone / 语言&accept-language / 地理坐标。
2. **Given** 环境已对齐某国代理,**When** 更换为另一国代理,**Then** 系统重新解析并更新时区/语言/地理,并对"国家发生漂移"给出提示。
3. **Given** 环境未绑定代理(直连),**When** 启动,**Then** 使用身份自带的 tz/locale,不做代理对齐,且不报矛盾。

---

### User Story 3 - 启动前一致性校验 + 启动后 CDP 注入地理 (Priority: P2)

启动实例前,系统校验该身份是否自洽(平台↔UA、时区↔国家↔坐标、语言↔国家、屏幕/窗口合理);**不通过则硬拦截并提供一键修复**。启动后通过 CDP 注入地理定位坐标(因 Chrome 144+ 已废弃地理相关启动 flag)。

**Why this priority**: 保证"自洽"在上线一刻真正成立,并补齐地理定位这一 flag 不可控的维度。依赖 US1/US2。

**Independent Test**: 人为构造一个矛盾身份(如平台=windows 但语言=仅日语且代理在美国),启动被拦截并给出可一键修复项;修复后启动成功,且在浏览器内查询 geolocation 返回被注入的坐标。

**Acceptance Scenarios**:

1. **Given** 一个存在矛盾的身份,**When** 点击启动,**Then** 启动被拦截,列出具体矛盾项,并提供一键修复。
2. **Given** 一个自洽身份,**When** 启动,**Then** 实例正常运行,且经 CDP 注入了与代理一致的 geolocation(及 timezone 兜底覆盖)。

---

### User Story 4 - 跨环境唯一性登记 (Priority: P2)

系统维护一份唯一性登记(已用 seed 与关键指纹 hash),生成时若撞车则重采,保证跨环境不重复;但偏向真实分布,不追求"最大唯一"(过度唯一本身是可追踪信号)。

**Why this priority**: 让"不重复"有硬保障且可审计;支撑规模化(成百上千环境)。

**Independent Test**: 批量生成 1000 个身份,登记表中无重复 seed / 指纹 hash;人为耗尽某一小类组合时,系统能重采或回退并记录。

**Acceptance Scenarios**:

1. **Given** 已登记若干身份,**When** 生成新身份且初次采样撞车,**Then** 系统自动重采直至唯一后再落库。

---

### User Story 5 - 手动覆盖与重生成 (Priority: P3)

高级用户可在编辑页查看自动生成的身份摘要,手动覆盖个别字段或"重新生成"整套身份;手动覆盖若引入矛盾,校验器给出警告。

**Why this priority**: 保留逃生口与可控性,兼容既有 `fingerprint_args` 手填习惯。

**Independent Test**: 手动改某字段造成矛盾 → 编辑页出现一致性警告;点"重新生成" → 得到一套新的自洽且唯一的身份。

**Acceptance Scenarios**:

1. **Given** 编辑页,**When** 用户手动覆盖某指纹字段,**Then** 系统保存覆盖值并即时给出一致性状态(通过/警告及原因)。
2. **Given** 编辑页,**When** 用户点"重新生成",**Then** 生成一套全新的、通过校验且不与他人重复的身份。

---

### Edge Cases

- **代理出口 IP 探测失败 / GeoIP 未命中**:保留上一次成功的地理快照;若从无快照,则回退为身份自带 tz/locale,并标记"未对齐",不硬拦截(直连语义)。
- **代理国家漂移**(出口国与上次不同):启动时轻量复核发现漂移 → 重新对齐并提示。
- **克隆环境**:默认为克隆体**重新生成**一套唯一身份(防跨环境关联);提供"保留原身份"选项。
- **既有老环境(仅有 fingerprint_args,无结构化身份)**:首次访问时从 flag 反解并补齐结构化身份,向后兼容,不破坏原有启动。
- **规模化撞车**:唯一性登记重采多次仍撞 → 放宽到次优真实组合并记录告警,不阻塞创建。
- **手动覆盖与代理对齐冲突**:以"最近一次显式操作"为准,并在校验中提示不一致。

---

## Requirements *(mandatory)*

### Functional Requirements

**生成与建模**
- **FR-001**: 系统 MUST 内置一份**离线真机分布指纹池**(构建期由 fingerprint-suite/BrowserForge 生成、内嵌进二进制),运行期无需联网即可采样。
- **FR-002**: 系统 MUST 通过**加权采样**从指纹池整条取出彼此自洽的基础元组(平台 / 品牌+版本 / UA / 硬件并发 / 屏幕 / 设备内存(记录用) / 基础语言),避免出现真机中不存在的矛盾组合。
- **FR-003**: 系统 MUST 为每个环境分配**唯一 seed**,并将 seed 与关键指纹 hash 写入**唯一性登记**;采样撞车 MUST 重采直至唯一。
- **FR-004**: 系统 MUST 以**结构化身份模型**持久化每个环境的完整身份(新增数据表/迁移),并保持与既有 `fingerprint_args` 的**向后兼容**(flag 由身份派生;保留手动覆盖逃生口)。
- **FR-005**: 系统 MUST 支持从既有 `fingerprint_args` 反解补齐结构化身份,不破坏老环境启动。

**地理自洽**
- **FR-006**: 系统 MUST 内置**离线 GeoIP 库**,将 IP 解析为 国家/城市/经纬度/时区,运行期完全离线。
- **FR-007**: 系统 MUST 在绑定/更换代理及启动时,**探测代理真实出口 IP**(经实例代理链路的轻量请求),再用离线 GeoIP 完成 IP→地理映射。
- **FR-008**: 系统 MUST 依据代理地理,自动推导并写入身份的 **timezone、语言/locale(含 `--lang`↔`--accept-lang` 交叉填充)、地理坐标**。
- **FR-009**: 系统 MUST 在代理国家发生漂移时,重新对齐并向用户提示。

**启动与校验**
- **FR-010**: 系统 MUST 在启动前运行**一致性校验**,至少覆盖:平台↔UA↔品牌一致、时区↔国家↔坐标一致、语言↔国家一致、屏幕/窗口合理、seed 存在。
- **FR-011**: 校验不通过时系统 MUST **硬拦截启动**,列出矛盾项,并提供**一键修复**。
- **FR-012**: 系统 MUST 将身份**序列化为内核可识别的启动 flag**(`--fingerprint` seed、`--fingerprint-platform`、`--fingerprint-brand`、`--fingerprint-brand-version`、`--fingerprint-hardware-concurrency`、`--window-size`、`--lang`、`--accept-lang`、`--timezone`、canvas/clientRects 加噪、`--webrtc-ip-handling-policy`、`--disable-non-proxied-udp`),并在启动集成的**唯一拼参点**注入。
- **FR-013**: 系统 MUST 在启动后通过 **CDP `Emulation.setGeolocationOverride`** 注入地理坐标(并以 `Emulation.setTimezoneOverride` 兜底时区),因 Chrome 144+ 已废弃对应启动 flag。
- **FR-014**: 系统 MUST 保证同一环境**跨会话可复现**(持久化 seed + 结构化身份),使目标站每次看到同一台设备;唯一性体现在环境之间。

**前端**
- **FR-015**: 编辑/创建页 MUST 展示自动生成的身份摘要、一致性状态徽章、"重新生成"、以及高级手动覆盖入口;原"手动对齐代理"改为自动(保留手动重对齐按钮)。

**范围排除(MUST NOT,保持内核原样)**
- **FR-016**: 本特性 MUST NOT 处理 WebGPU(保持 fingerprint-chromium 原有行为,不禁用、不伪造、不校验)。
- **FR-017**: 本特性 MUST NOT 触碰网络传输层指纹 TLS/JA3/JA4、HTTP/2(JA4H)、HTTP/3/QUIC、JA4L 握手时序(这些由内核继承的真 Chrome 值决定,非按环境可改)。

### Key Entities *(include if feature involves data)*

- **Fingerprint Identity(结构化身份)**:一个环境的完整身份。属性:os/platform、platformVersion、browserBrand、brandVersion、uaFull、hardwareConcurrency、deviceMemory(记录)、screen{w,h,dpr,colorDepth}、windowSize、languages/locale、timezone、geo{lat,lon,accuracy}、seed、canvasNoise、clientRectsNoise、webrtcPolicy、sourcePoolRecordId(溯源)、fingerprintHash(唯一性)、proxyGeoSnapshot、coherenceStatus。与 Profile/环境 一对一。
- **Fingerprint Pool Record(指纹池记录)**:离线数据集中一条真机指纹,采样来源;含市场份额权重。
- **GeoIP Database(离线地理库)**:IP→国家/城市/经纬度/时区,内嵌只读数据。
- **Uniqueness Registry(唯一性登记)**:已用 seed 与关键指纹 hash 的集合,供撞车检测。
- **Proxy Geo Snapshot(代理地理快照)**:某代理最近一次解析出的出口 IP + 地理结果,缓存复用。

---

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 批量创建 1000 个环境,**零重复**(无重复 seed、无重复关键指纹 hash)。
- **SC-002**: 绑定代理的环境中,**100%** 在启动时其 timezone/语言/地理坐标与代理出口国家(按离线 GeoIP)一致。
- **SC-003**: 一致性校验器对上线身份的内部矛盾**零漏放**;在指纹检测页抽样复核无平台/时区/语言/地理冲突。
- **SC-004**: 同一环境重复启动 10 次,指纹**100% 可复现**(关键指纹 hash 恒定)。
- **SC-005**: 运行期除"经代理探测出口 IP"外**无任何外部网络调用**(指纹池与 GeoIP 均离线)。
- **SC-006**: 单个身份生成(采样+对齐+校验)耗时 **< 50ms**,不显著拖慢创建/启动。
- **SC-007**: 既有老环境(仅 `fingerprint_args`)在升级后**100% 可正常启动**,并被补齐结构化身份。

---

## Assumptions

- 目标内核为 fingerprint-chromium(Chrome 144+):webgl-vendor/renderer、screen、device-memory、location 等细粒度 flag 已废弃,改由 seed 派生;地理定位改走运行时 CDP。
- 离线指纹池与离线 GeoIP 为**构建期产物**(在可联网机器或用户自有海外服务器生成一次),运行期只读、离线;两者带版本号与刷新脚本。
- 探测代理真实出口 IP 需经代理联网;IP→地理为离线。
- 复用既有代理栈(xray/sing-box/mihomo)与既有 CDP 基础设施(统一 CDP 入口、内部 CDP 调用)。
- 数据集许可(DB-IP 为 CC-BY / GeoLite2 为其 EULA)与项目 LGPLv3 兼容,作为独立数据文件随附。
- WebGPU 与网络层(TLS/HTTP2/3/JA4L)保持内核默认,不在本特性范围。
- 本轮不含:改名 zwbrowser、内核内置/自建镜像源分发、Camoufox 第二内核、CreepJS/BrowserLeaks 在线逼真度打分。
