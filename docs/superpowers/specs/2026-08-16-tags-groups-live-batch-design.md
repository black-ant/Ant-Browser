# 设计:标签/分组/直播养号/批量创建 六项功能

- 日期:2026-08-16
- 分支:`feature/fingerprint-self-consistency`
- 状态:已与用户确认,进入实施计划

## 背景与目标

ZwBrowser 是内部专业指纹浏览器(Go + Wails + React),核心红线:每个环境的指纹**独立、不重复、自洽**。本次针对多开养号(上百个环境挂抖音等直播)的实际使用,补齐六项功能:

1. 标签管理:环境列表标签"显示不全"、行内加删不便 → 优化显示与操作。
2. 保活:改为每环境一个开关(默认开),批量创建可设"全部是否保活"。
3. 静音:默认全部静音(硬静音),可手动取消(需重启该实例)。
4. 分组:补齐"在哪里加分组 / 编辑分组 / 批量加入分组 / 移出分组"——顶部筛选保留,**所有分组操作放批量工具条**,不做左侧常驻分组树。
5. 批量创建:增加身份池平台选择(默认全部;选定后只生成该平台身份),任何情况都不重复。
6. 修复「标签管理」页:列表无法上下滚动、实例条数显示不全。

关键语义:改身份池/平台选择只影响**之后新建/批量创建**的环境,不改已有环境的既有身份(已存 `browser_identities` 表)。

## 现状勘定(探索结论)

所有路径相对 `Ant-Browser-Plus/`。

- **标签**:是 `Profile.Tags []string`(`backend/internal/browser/types.go:27`),以 JSON 数组存 `browser_profiles.tags` 列。后端已具备并已绑定:`BrowserProfileBatchSetTags` / `BatchRemoveTags`(`backend/app_profile_tags.go`)、`BrowserRenameTag`、`BrowserGetAllTags`。**无**单实例 `SetTags`——单实例改标签用 `BatchSetTags([id],…)` 即可,后端不改。列表截断在 `frontend/src/modules/browser/components/BrowserProfilesPanel.tsx:476`(表格 `tags.slice(0,2)+N`)与 `:335`(卡片)。行内无加删,主列表批量工具条 `components/BrowserListWidgets.tsx` 无标签动作。
- **分组**:`Profile.GroupId`(单列 FK,非连接表)。后端**已完整**且已绑定:`CreateGroup/UpdateGroup/DeleteGroup/ListGroups`(`backend/app_group.go`)、`MoveInstancesToGroup(ids, groupId)`(`groupId=""` 即移出)。前端 `components/GroupTreeNav.tsx`(含增删改的分组树)**未被挂载**;`GroupSelector.tsx` 仅用于编辑页选择(不能新建)。顶部分组筛选下拉已接线(`components/InstanceFilterBar.tsx:104`)。多选 `selectedIds` 与批量工具条已存在(`pages/BrowserListPage.tsx`、`components/BrowserListWidgets.tsx` 的 `BatchToolbar`);注意 `components/InstanceTableCells.tsx` 内有个**废弃重复**的 BatchToolbar,别改错。
- **批量创建**:`App.BrowserProfileCreateBatch(prefix,count,startIndex,template)`(`backend/app_browser_profile_api.go:47`)→ `Manager.CreateBatch`(`backend/internal/browser/profile_create_batch.go:22`,`MaxBatchCreateCount=200`)。每个环境经 `createProfileLocked → IdentityService.Regenerate → GenerateUnique → pool.NewIdentity` 采样唯一身份。前端弹窗 `components/BatchCreateModal.tsx` 目前只有 prefix/count/startIndex。
- **身份去重(红线,已具备)**:全局、库级。唯一键为 `seed`(1..2³¹−1 随机)与 `fingerprint_hash`(seed 参与哈希),`browser_identities` 表有两条部分唯一索引(`backend/internal/database/sqlite.go:237-238`),运行时再查 `Seen`。`identity.GenerateUnique` 重试上限 100 次(`identity_service.go` 附近)。
- **身份池 platform**:`PoolRecord.Platform`(`backend/internal/identity/pool.go`,JSON 键 `platform`),取值仅 `windows`(1470 条)/ `macos`(822 条),**无 linux**,共 2292 条。`Pool.Sample` 是全量加权随机,**无任何平台过滤**。生成身份的 `platform` 经 `serializer.go` 输出为 `--fingerprint-platform=<platform>`。运行时覆盖池路径 `data/identity_pool.json`(`PoolStore`,`app_startup.go:136`)。
- **per-profile 启动配置**:`browser_profiles` **无通用 JSON 配置列**,均为显式列(`backend/internal/database/sqlite.go` 迁移)。可参考三态列 `restore_last_session`(v13)、标量 `memory_limit_mb`(v14),v15 为 `browser_identities`。启动参数在 `backend/app_instance_start_prepare.go`:`buildBrowserLaunchArgs`(`:309`,不持有 `*Profile`)由上层 `prepareBrowserStartPlan`(持有 profile)喂入;`MemorySaverEnabled` 在 `:166` 处注入,是每实例 flag 注入的范例。保活循环 `backend/app_live_keepalive.go` 现读全局 `a.config.Browser.LiveKeepAliveEnabled`;`runKeepAliveDue` 遍历 `a.browserMgr.List()` 拿到完整 `Profile`(可直接读新列)。

## 决策(已确认)

- 静音:**硬静音**(`--mute-audio`);取消某号声音 = 关其静音开关并**重启该实例**。
- 分组:**不做左侧树**;顶部筛选保留;新建/编辑/删除/批量移动/移出**全部集中在批量工具条的「分组」入口弹窗**。
- 标签显示:行**紧凑等高**,显示前若干个 + 悬停/点 `+N` 浮层展开全部(可逐个删);行内可加删;批量加删进主工具条。
- 静音默认对**已有环境也生效**(下次启动静音),符合"默认都关闭声音"。
- 保活默认**开**;全局配置降级为总闸。

## 各功能设计

### ① 标签(纯前端;单实例改标签复用 `BatchSetTags([id],…)`)
- 修 `BrowserProfilesPanel.tsx:476` 表格标签列:等高,显示前 N(约 2–3)+ `+N`,悬停/点击弹浮层显示**全部**标签,每个带 `×` 直接移除(调 `batchRemoveProfileTags([id],[tag])`)。卡片视图 `:335` 同样收敛。
- 行内新增:标签格加 `＋` → 浮层(已有标签建议 + 自由输入)→ `batchSetProfileTags([id], tags, false)`,免整存。
- 主列表批量工具条 `BrowserListWidgets.tsx` 增「加标签 / 删标签」,作用于 `selectedIds`。
- 保留 `/browser/tags` 标签管理页。

### ② 保活:每环境开关(后端加列 + 前端)
- v16 迁移新增列 `live_keepalive_enabled`(默认开)。同步:`Profile`、`ProfileInput`(`types.go`)、`profile_dao.go` 全部 SELECT/UPSERT/scan 站点、`profile_create.go`/`profile_update.go` 映射。为避免部分更新误清,`ProfileInput` 用可空(`*bool`,nil=不改)。
- `app_live_keepalive.go` `runKeepAliveDue` 循环体加每实例判断:该实例关则跳过;全局 `LiveKeepAliveEnabled` 作总闸(显式关=全关)。随机 60–90s 逻辑不变。
- 前端:编辑页 `BrowserEditPage` 加「直播养号」区块含保活开关;批量创建弹窗加「全部开启保活」;批量工具条加「批量保活 开/关」(便捷项,可裁)。

### ③ 静音:默认硬静音(后端加列 + 前端)
- v16 迁移新增列 `mute_audio`(默认开=静音),迁移同 ②。
- 注入:仿 `app_instance_start_prepare.go:166` MemorySaver 块,`profile.MuteAudio` 为真则追加 `--mute-audio`(`--mute-audio` 不在被管控/剥离参数集内)。
- 取消静音:关该实例静音开关 → 重启生效;提供「取消静音并重启」快捷动作。
- 前端:编辑页「直播养号」区块含静音开关;批量创建弹窗默认静音(可关)。

### ④ 分组:操作集中批量工具条(几乎纯前端;后端已全有)
- 顶部分组筛选下拉保持不变。
- 批量工具条 `BrowserListWidgets.tsx` 加「分组」入口 → 弹窗两块:
  - **移动选中环境到**:`未分组(=移出)` / 各分组 / `＋新建分组`(建后直接移入)→ `moveInstancesToGroup(ids, groupId)`。
  - **管理分组**:列出分组,行内 重命名 / 删除,底部 新建 —— 即"分组在哪里加 + 编辑分组功能"。
- 复用 `GroupSelector` 做移动选择;删除沿用后端级联(成员升父级)。后端不改。

### ⑤ 批量创建:平台选择 + 永不重复 + 自洽守卫
- 前端 `BatchCreateModal.tsx` 加平台下拉 `全部平台(默认)/ Windows / macOS`;并入 ②③ 的「全部保活 / 全部静音」。
- 后端:`CreateBatch` 增 `platform string` 形参 → 透传 `Regenerate/GenerateUnique`;身份池新增按 `Platform==target` 的过滤采样(`SampleWhere` 或子池),`""`=全平台(现状)。
- 永不重复:沿用现有库级双唯一索引 + 运行时 `Seen`;平台过滤只缩小真机模板集,**保证不变、不会耗尽**(每模板 ×21 亿 seed)。
- 🔒 自洽守卫:生成 macOS 身份时,确保身份自带 `--fingerprint-platform=macos` **优先级压过** `config.yaml` 全局 `--fingerprint-platform=windows`(核对参数拼接顺序/去重),使 UA / 平台 / 品牌三者一致。实现后 CDP 抽验。

### ⑥ 修复「标签管理」页滚动 + 条数(纯前端)
- 根因:`TagManagementPage.tsx:343` 表格 `<Card className="flex-1 overflow-hidden">` 缺 `min-h-0`,flex 项默认 `min-height:auto` 被内容撑高 → 外层裁切、内层 `overflow-auto` 不触发 → 滚不动、行被截。
- 修:给右侧内容列(`:332`)与表格 Card(`:343`)加 `min-h-0`,内层 `overflow-auto h-full` 恢复滚动(表头已 `sticky top-0`);左侧 TagPanel 滚动列表(`:67`)同样补 `min-h-0`。
- 表格上方加计数行「共 N 个实例 · 已选 M」,显式显示总条数。

## 影响面

- **一次 v16 迁移**加两列:`live_keepalive_enabled`、`mute_audio`;`ProfileInput` 加对应可空字段。
- **需 `wails generate module` 重生成绑定**:`BrowserProfileCreateBatch` 签名新增 `platform`;`ProfileInput` 新字段进 `models.ts`。生成物一并提交(dev 用 `-skipbindings`,不自动生成)。
- 后端改动集中在:identity 过滤采样、两处 per-profile flag/开关注入、CreateBatch 透传;分组/标签/⑥ 几乎纯前端。
- 交付:实现后按既定流程 `git commit + push` → 重打含内核 Windows 安装包(服务器 Docker + 内核缓存)。

## 验证

1. 后端:`go build ./...`、win/linux 交叉编译、`go vet`;新增单测——平台过滤采样只出目标平台、去重仍全局;v16 迁移往返;保活每实例开关解析。
2. 绑定:`wails generate module` 产出新签名/字段。
3. 前端:`npm run build`(tsc + vite)。
4. 端到端(重启 app):
   - 标签:列表紧凑显示 + `+N` 展开全部 + 行内加删 + 批量加删;标签管理页可上下滚动、显示"共 N 个实例"。
   - 保活:编辑页开关生效;批量创建"全部保活"生效;关某实例后不再注入。
   - 静音:默认静音;取消并重启后出声。
   - 分组:工具条新建/改名/删除分组、批量移动/移出;顶部筛选联动。
   - 批量创建:选 Windows/macOS 只生成该平台身份;CDP 抽验 UA/平台/品牌自洽;已有环境不变;身份不重复。

## 风险 / 备注

- v16 静音默认对已有 101 环境生效(下次启动静音)——符合"默认都关闭声音";如需仅新建生效,改为旧行不设默认(待用户另行确认)。
- 硬静音是启动参数,取消静音必须重启该实例(设计已含快捷动作)。
- macOS 平台自洽守卫涉及既有全局 `--fingerprint-platform=windows`;若历史 macOS 身份此前被全局覆盖,此修复顺带纠正,实现时以 CDP 实测为准。
- 绑定重生成为手动步骤,易漏,务必执行并提交生成物。
