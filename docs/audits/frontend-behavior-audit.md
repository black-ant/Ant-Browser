# 前端页面行为变更审计

## 审计范围

- 审计类型：只读 Git 和源码审计。
- 共同基线：`61feab7`。
- 当前 `master`：`becd1d5`，相对基线领先 15 个提交，其中 13 个提交触碰 `frontend`。
- 旧备份线：`backup/master-before-origin-sync-20261016`，末端为 `8ac183a`，相对基线领先 28 个提交，其中 25 个提交触碰 `frontend`。
- 两条线从 `61feab7` 分叉，不能把旧备份线的最终行为直接当成当前 `master` 的累计结果。
- 本次未修改代码、未还原提交、未创建分支、未提交 Git。

当前工作区原本已有未提交文件，未纳入页面行为判断：

- `frontend/package-lock.json`
- `frontend/package.json.md5`
- `frontend/src/wailsjs/go/models.ts`
- `go.mod`

## 风险定义

- **高**：已有入口或操作被删除，或全局布局、恢复数据语义发生改变。
- **中**：已有页面的行为、筛选、展示语义发生改变，但主流程仍然存在；或新增页面会改变原有信息结构。
- **低**：仅文案或新增辅助信息变化。
- “非必要前端变化”表示：如果目标是保护基线页面布局和既有操作，这项变化是否应阻断。新增功能单独标记为可保留。

## 页面行为审计表

| 编号 | 线 | 页面/流程 | 已确认的删除、修改或新增行为 | 修改前 → 修改后 | 来源提交 | 风险 | 非必要前端变化 |
|---:|---|---|---|---|---|---|---|
| 1 | 当前 `master` | 代理选择弹窗：实例编辑、实例列表、插件管理 | 删除全部测速、单行测速及结果、编辑代理、删除代理、删除确认、导入代理；相关回调也被删除。 | 选择弹窗兼代理管理 → 只剩搜索、分组筛选、点击行选择、关闭；代理池独立页面仍保留管理能力。证据：`frontend/src/modules/browser/components/ProxyPickerModal.tsx:38`、`frontend/src/modules/browser/components/ProxyPickerModal.rows.tsx:29`。 | `becd1d5` | **高** | **是** |
| 2 | 当前 `master` | 虚拟分组导航 | 提交过程中先新增侧边栏入口，后续又删除；最终将入口迁移到代理池页面。 | 侧边栏直接进入 → 代理池“虚拟节点”按钮、表格“管理”按钮进入；路由仍存在于 `frontend/src/routes/AppRoutes.tsx:105`。 | `3049858`、`cdb48c7`；`frontend/src/modules/browser/pages/proxyPool/ProxyPoolHeader.tsx:86` | **高** | **是** |
| 3 | 两条线 | 实例编辑页代理策略 | 修改已有表单语义；新增动态代理、实体分组和虚拟分组选择；受影响字段改用自定义下拉。 | `代理池 / 本地代理` → `固定代理 / 动态代理 / 本地代理`；动态模式按实体分组或虚拟分组选择。证据：`frontend/src/modules/browser/pages/BrowserEditPage.tsx:650`、`:663`、`:731`。 | 当前：`544c52f`、`3049858`、`b348df9`、`cdb48c7`；旧线：`5b80fcc`、`90aed48`、`0049473` | **高** | **部分**：动态代理可保留；原有模式改名和结构变化属于非必要改动 |
| 4 | 两条线 | 实例列表代理单元 | 动态代理显示分组名；只从测速有效节点中选候选；无有效候选时测速按钮禁用；新增虚拟分组同步。 | 固定代理展示和直接测速 → 显示“动态代理 · 分组名”，测速当前动态候选或在无候选时禁用。证据：`frontend/src/modules/browser/components/BrowserProfilesPanel.tsx:154`、`:290`。 | 当前：`cbe9c4b`、`612e9a7`；旧线：`5b80fcc`、`17dc539`、`6552fed` | **中** | **部分** |
| 5 | 两条线 | 代理池表格和虚拟节点 | 新增虚拟分组行、虚拟分组筛选、成员查看弹窗；虚拟行不能参与批量删除；操作按钮改为“管理”。 | 只有普通代理行 → 普通代理和虚拟分组混合展示；虚拟行的延迟和 IP 健康按成员节点显示。证据：`frontend/src/modules/browser/pages/ProxyPoolPage.tsx:202`、`frontend/src/modules/browser/pages/proxyPool/ProxyPoolTableCard.tsx:269`、`:474`。 | 当前：`3049858`、`cdb48c7`；旧线：`5b80fcc`、`214c757`、`325a526` | **中** | **否，属于新增功能** |
| 6 | 两条线 | 虚拟分组页面 | 新增创建、编辑、删除、节点筛选、全部测速、单节点测速、策略配置。 | 原来没有该流程 → 独立虚拟分组管理页和编辑流程。证据：`frontend/src/modules/browser/pages/ProxyVirtualGroupsPage.tsx:485`、`:512`。 | 当前：`3049858`；旧线：`5b80fcc`、`4f89bf1`、`26713e5`、`0049473`、`90aed48`、`325a526` | **中** | **否，属于新增功能** |
| 7 | 两条线 | 代理测速结果和“只展示可用” | 修改有效性判断；增加结果有效期、测速指纹、上下文变化失效；过期或指纹不匹配不再算可用。 | 有历史测速值即可显示或筛选 → 必须是新鲜且成功，或有有效 IP 健康结果。证据：`frontend/src/modules/browser/pages/proxyPool/useProxyPoolFilter.ts:87`、`frontend/src/modules/browser/proxySpeedFreshness.ts:15`。 | 当前：`5488d1e`、`a34e040`、`b348df9`、`cbe9c4b`、`612e9a7`；旧线：`ce9d9dd`、`0758294`、`17dc539`、`6552fed`、`9afcaf5`、`8ac183a` | **中** | **未定**：代码显示这是检测一致性功能，不是单纯布局变化 |
| 8 | 两条线 | 代理检测设置弹窗 | 新增成功重测间隔、失败重试间隔、手动测速并发上限等字段。 | 只有桥接等待和目标配置 → 增加周期、重试、并发控制。证据：`frontend/src/modules/browser/pages/proxyPool/ProxyPoolCheckSettingsModal.tsx:46`、`:55`、`:64`。 | 当前：`b348df9`、`cbe9c4b`；旧线：`0758294`、`9afcaf5` | **中** | **否，属于新增配置** |
| 9 | 两条线 | 实例备份和恢复 | 新增迁移密码；恢复增加新建、覆盖、重命名；覆盖时旧实例进回收站；失败支持回滚提示。 | 原有直接导入导出 → 密码步骤、冲突处理、覆盖保护、回滚结果提示。证据：`frontend/src/modules/browser/pages/BrowserListPage.tsx:761`、`frontend/src/modules/backup/BackupPage.tsx:342`。 | 当前：`1b59357`、`b348df9`；旧线：`16a2167` | **高** | **否，属于数据安全流程** |
| 10 | 两条线 | OpenList 配置 | Token-only 改为支持 WebDAV 用户名/密码和 OpenList 全局 Token；按认证方式显示字段；认证下拉改为自定义键盘下拉。 | 仅 Token → 两种认证模式和不同字段校验。证据：`frontend/src/modules/backup/channels/openlist/OpenListConfigModal.tsx:292`、`frontend/src/modules/backup/channels/openlist/OpenListAuthModeDropdown.tsx:77`。 | 当前：`2c58df9`；旧线：`df5a2bc` | **中** | **否，属于新增能力** |
| 11 | 两条线 | 全局代理失败通知 | 修改提示语义和通知内容。 | “代理桥接失败，已直连启动” → “代理桥接失败：具体错误”；节点失效通知增加受影响实例和代理。证据：`frontend/src/App.tsx:58`。 | 当前：`5488d1e`、`cbe9c4b`；旧线：`0758294`、`17dc539` | **中** | **部分**：错误详情可保留，但删除“已直连启动”会改变用户理解 |
| 12 | 两条线 | 插件配置保存提示 | 仅修改用户提示文案。 | “下次启动时写入” → “运行中的实例下次启动生效”。证据：`frontend/src/modules/browser/components/ProfileExtensionModal.tsx:52`。 | 当前：`cc595b6`；旧线：`7af6faf` | **低** | **是，纯提示文案** |
| 13 | 两条线 | 关于页面 | 新增当前版本、检查更新、最新版本、下载页入口。 | 没有更新检查 → 增加更新检查卡片和下载入口。证据：`frontend/src/modules/profile/ProfilePage.tsx:229`、`:242`。 | 当前：`1e1a5d5`；旧线：`e644e18` | **低** | **否，属于新增功能** |
| 14 | 旧备份线专有 | 代理导入入口 | 删除 `ProxyImportModal` 及其相关文件，把导入流程迁移到代理池页面。 | 代理选择弹窗可直接导入 → 只能从代理池页面导入。删除文件包括 `frontend/src/modules/browser/components/ProxyImportModal.tsx:1`；新流程在 `frontend/src/modules/browser/pages/proxyPool/useProxyImportFlow.ts:43`。 | `8bb8f0a` | **高** | **是** |
| 15 | 旧备份线专有 | 全局 Button 样式 | 全局 Button 增加边框、阴影、点击位移；`secondary`、`ghost` 颜色和背景改变。 | 基线按钮样式 → 所有按钮统一变重、带阴影和按压动效。旧线证据：`frontend/src/shared/components/Button.tsx:20`。 | `17dc539` | **高** | **是** |
| 16 | 旧备份线专有 | 代理池表格布局 | IP 健康摘要压缩；桌面表格固定布局、最小宽度约 `1600px`、单元格不换行；新增移动端卡片。 | 原表格自适应、IP 健康多行展示 → 固定宽大表格和移动端另一套布局。旧线证据：`frontend/src/modules/browser/pages/proxyPool/ProxyPoolTableCard.tsx:691`。 | `53b14b6`、`17dc539`、`325a526` | **高** | **是** |
| 17 | 旧备份线专有 | 内核管理表格 | 内核路径、Chrome 版本字号放大，路径允许强制断行；Table 增加固定布局相关能力。 | 普通字号、普通路径展示 → `text-lg` + `break-all`。旧线证据：`frontend/src/modules/browser/pages/CoreManagementPage.tsx:177`。 | `53b14b6` | **中** | **是** |

## 重点结论

1. 当前 `master` 最严重的是 `becd1d5`：它不是单纯重构，而是把代理选择弹窗里已有的测速、编辑、删除、导入能力整体删掉。
2. 旧备份线最不应该整体还原的是：
   - `8bb8f0a`：导入入口从选择弹窗迁移到代理池。
   - `17dc539`：全局 Button 样式变化。
   - `53b14b6`：代理池表格布局变化。
3. 旧备份线末态仍保留代理选择弹窗的测速、编辑、删除；它主要移走的是导入。当前 `master` 的选择弹窗收缩比旧备份线更严重。
4. 当前 `master` 的 `frontend/src/shared/components/Table.tsx:30` 只是增加 `tableLayout` API，默认仍是 `auto`，不能把旧备份线的固定表格视觉变化误算到当前 `master`。
5. 不建议整条旧备份线回滚或重新集成，应按编号逐项选择：阻断高风险非必要改动，保留新增功能，谨慎合入中风险行为修复。

## 中间提交说明

本表按两条分支的最终页面行为合并相近提交，没有把中间态重复计数。特别是：

- `26713e5` 曾合并虚拟分组编辑步骤。
- `4f89bf1` 曾调整虚拟分组页面中的测速能力。
- 后续提交又恢复或重构了相关流程，不能把这些中间态删除直接当成旧备份线最终状态。

## 排除项

以下提交或文件没有足够证据表明会改变本次审计范围内的页面布局或业务操作，因此未列入主表：

- `b5d60a9`：前端主要是生成模型变化。
- `0570f64`：依赖、发布文件和后端持久化变化，没有页面行为代码。
- `700af38`：生成前端模型变化。
- `7926b7a`：发布文档、生成绑定和变更日志内容，不属于页面操作流程。
- `88ef762`：发布准备和依赖版本变化。
- `frontend/src/wailsjs/go/models.ts`、`frontend/package-lock.json`、`frontend/package.json.md5`、`go.mod`：本次没有发现独立的页面行为证据。

## 验证边界

- 本文是 Git 和源码静态审计结果。
- 本次没有启动前端、没有运行构建或测试、没有实际展开检查下拉框和弹窗。
- 自定义下拉的键盘、焦点、关闭逻辑在代码中存在，但展开态的实际视觉效果尚未实机确认。
- 因此本文不宣称任何运行时视觉效果或修复结果已经验证。

## 本轮处理决定

- **编号 1：已还原**。恢复代理选择弹窗的全部测速、单行测速结果、编辑代理、删除代理、删除确认、导入代理，以及实例编辑、实例列表、插件管理三个调用方的同步回调。
- **编号 2：按要求对齐**。保留代理池页的“虚拟节点”和虚拟分组表格“管理”入口；未发现侧边栏或其他页面可见入口。路由仍保留，供代理池页面跳转使用。
- **编号 3–17：保持当前实现**。未按本轮要求回退或重构。
- **工作区保护**。未修改原有未提交的 `frontend/package-lock.json`、`frontend/package.json.md5`、`frontend/src/wailsjs/go/models.ts`、`go.mod`。
- **验证状态**。已完成 Git 差异检查、入口静态检索，以及 `frontend` 下的 `npm run build`（TypeScript 与 Vite 生产构建均通过）。未进行实际 UI 展开态检查。
