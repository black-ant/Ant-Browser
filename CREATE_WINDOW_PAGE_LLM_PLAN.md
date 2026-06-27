# 创建窗口页功能落地计划（给大模型执行）

## 背景

当前创建窗口页 `frontend/src/modules/browser/pages/BrowserCreateWorkstationPage.tsx` 已经提供大量 UI 控件，但保存链路在 `BrowserEditPage.handleSave` 中基本只是把 `formData` 原样传给后端。后端 `backend/internal/browser/types.go` 的 `ProfileInput` 只接收基础字段，导致许多创建页字段无法持久化，也不会转换成实际启动参数。

目标是让创建窗口页的所有控件都有明确落点：能保存、能回显、能启动生效，或在未支持前被禁用/隐藏。

## 总目标

1. 创建页所有可编辑字段都进入明确的数据契约。
2. 核心浏览器/指纹/启动设置转换为后端实际使用的 `fingerprintArgs` 和 `launchArgs`。
3. 需要启动前/启动后执行的功能通过后端动作执行器落地。
4. 编辑页能完整回显创建页配置。
5. 未实现功能不能表现为可点击但无效果。

## 关键文件

- `frontend/src/modules/browser/pages/BrowserCreateWorkstationPage.tsx`
- `frontend/src/modules/browser/pages/BrowserEditPage.tsx`
- `frontend/src/modules/browser/types.ts`
- `frontend/src/modules/browser/api.ts`
- `backend/internal/browser/types.go`
- `backend/internal/browser/profile.go`
- `backend/internal/browser/profile_dao.go`
- `backend/internal/database/sqlite.go`
- `backend/app_instance.go`
- `backend/app_cookie.go`
- `backend/internal/browser/bookmarks.go`

## 实施原则

- 不要继续把所有创建页字段平铺到 `BrowserProfileInput` 顶层。
- 新建清晰的前端表单类型，例如 `CreateWindowFormState`。
- 新增结构化持久化字段，例如 `profileConfig`，保存 UI 原始配置。
- 新建转换器，将 `CreateWindowFormState` 转换为后端可理解的 `BrowserProfileInput`。
- 对不能立即生效的功能，先保存为结构化配置，并在 UI 上明确禁用或隐藏入口。

## 阶段 1：修复主保存链路

### 目标

让核心控件创建后立即生效，先解决“填了但没用”的问题。

### 任务

1. 新建前端转换器：
   - 建议路径：`frontend/src/modules/browser/utils/createWindowConverter.ts`
   - 输入：创建页表单状态、用户手填启动参数、默认配置
   - 输出：`BrowserProfileInput`

2. 转换字段：
   - `userAgent` -> `--user-agent=`
   - `system` -> `--fingerprint-platform=`
   - `browserCore` / `browserVersion` -> `coreId` 或结构化配置
   - `language` -> `--lang=`
   - `uiLanguage` -> `--accept-language=`
   - `timezone` -> `--timezone=` / `--ant-timezone-mode=`
   - `urls` -> 解析为启动 URL，追加到 `launchArgs`
   - `windowWidth` / `windowHeight` / `resolution` -> `--window-size=`
   - `audio` -> `--mute-audio`
   - `image` -> `--blink-settings=imagesEnabled=false`
   - `video` -> `--autoplay-policy=user-gesture-required`
   - `searchEngine` -> `--ant-search-engine=`
   - `hardwareAcceleration` -> `--disable-gpu`
   - `sandbox` -> `--no-sandbox`
   - `webgpu` -> `--disable-features=WebGPU`
   - `hardwareConcurrency` -> `--fingerprint-hardware-concurrency=`
   - `deviceMemory` -> `--fingerprint-device-memory=`
   - `doNotTrack` -> `--fingerprint-do-not-track=`
   - `webrtc` -> 对应已有 WebRTC 指纹参数
   - `webglVendor` / `webglRenderer` -> 对应 WebGL 指纹参数

3. 修改 `BrowserEditPage.handleSave`：
   - `isCreate` 时调用转换器生成 payload。
   - 不再直接 `{ ...formData }` 发给后端。
   - 保留用户手填 `launchArgsText`，并与转换器参数去重合并。

4. 添加单测：
   - UA 转换
   - 启动 URL 转换
   - 窗口大小转换
   - 语言/时区转换
   - 媒体开关转换
   - 搜索引擎转换

## 阶段 2：结构化持久化和回显

### 目标

所有创建页字段保存后可以完整回显。

### 任务

1. 后端新增字段：
   - `Profile.ProfileConfig string` 或结构体
   - `ProfileInput.ProfileConfig string` 或结构体
   - 数据库 `browser_profiles` 新增 `profile_config TEXT NOT NULL DEFAULT '{}'`

2. 更新 DAO：
   - `profile_dao.go` 的 insert/update/select
   - 兼容旧数据，空值返回 `{}`

3. 更新数据库迁移：
   - `backend/internal/database/sqlite.go`

4. 更新备份/恢复：
   - `backend/app_backup_ops.go`

5. 前端类型调整：
   - `BrowserProfile.profileConfig`
   - `BrowserProfileInput.profileConfig`
   - `CreateWindowFormState`

6. 编辑页加载：
   - 如果存在 `profileConfig`，优先用它恢复创建页状态。
   - 如果不存在，尝试从旧的 `fingerprintArgs` / `launchArgs` 反解析基础字段。

## 阶段 3：启动前/启动后动作执行器

### 目标

处理不能单靠启动参数实现的功能。

### 动作模型

建议在 `profileConfig` 中保存：

```ts
postCreateActions: {
  importCookies?: string
  applyDefaultBookmarks?: boolean
  clearCacheBeforeStart?: boolean
  clearCookiesBeforeStart?: boolean
  clearLocalStorageBeforeStart?: boolean
}
```

### 后端任务

1. 启动前动作：
   - 清缓存
   - 清 Cookie
   - 清 Local Storage
   - 写入默认书签

2. 启动后动作：
   - 通过 CDP 导入 Cookie
   - 应用地理位置权限
   - 应用网站黑白名单（如果当前内核/扩展已有支持）

3. 错误处理：
   - 动作失败不应静默。
   - 写入 `LastError` 或返回明确错误。
   - 前端 toast 展示失败原因。

## 阶段 4：逐项落地功能清单

### P0 必须落地

- 窗口名称
- 浏览器内核
- User-Agent
- 启动 URL
- 启动参数
- 分组
- 标签
- 账号绑定
- 扩展绑定
- 代理绑定

### P1 启动参数/指纹类

- 系统平台
- 语言
- UI 语言
- 时区
- 地理位置模式
- 音频/图片/视频
- 窗口大小
- 搜索引擎
- WebRTC
- WebGL
- WebGPU
- Canvas
- AudioContext
- Speech Voices
- Do Not Track
- ClientRects
- 媒体设备
- 设备名
- MAC 地址
- CPU 核心数
- 设备内存
- 硬件加速
- 沙箱

### P2 启动前/启动后动作类

- Cookie 导入
- 默认书签
- 启动前清缓存
- 启动前清 Cookie
- 启动前清 Local Storage
- 启动时随机指纹
- IP 变化提醒
- IP 变化停止打开
- 网站黑名单
- 网站白名单

### P3 UI 体验类

- 一键配置预设
- 随机配置
- 默认项目
- 备注
- 保存为模板
- 从模板创建

## 阶段 5：处理未实现 UI

当前存在看起来可点击但无处理器的入口：

- 一键配置预设按钮
- 底部 `默认项目`
- 底部 `标签`
- 底部 `备注`
- `存为新模板`

处理方式：

1. 如果本轮实现，补齐实际逻辑。
2. 如果本轮不实现，隐藏或禁用按钮。
3. 禁用时使用 tooltip 简短说明原因。

## 验收标准

每个功能必须满足至少一种验收：

1. 保存回显：
   - 创建窗口后再进入编辑页，字段值不丢。

2. 启动生效：
   - 启动窗口后，能通过实际启动参数、CDP、页面行为或文件系统验证。

3. 明确不可用：
   - 暂不支持的功能不能是可点击无反应状态。

## 测试计划

### 前端

- converter 单测：
  - `createWindowConverter.test.ts`
  - 覆盖参数映射、去重、空值、URL 解析。

- 页面冒烟测试：
  - 填写关键字段。
  - 点击创建。
  - 验证调用 API payload。

### 后端

- DAO 测试：
  - `profile_config` 插入、更新、读取。

- migration 测试：
  - 老数据库升级后 `profile_config` 默认 `{}`。

- 启动参数测试：
  - 验证 `fingerprintArgs` 和 `launchArgs` 没有被默认配置覆盖。

- 启动动作测试：
  - 清理动作路径正确。
  - Cookie JSON 解析错误能返回明确错误。

### 手动验收

1. 创建一个窗口，设置 UA、语言、时区、窗口大小、启动 URL。
2. 保存后进入编辑页，确认字段回显。
3. 启动窗口，确认页面打开指定 URL。
4. 通过页面 JS 或 CDP 检查 UA、语言、窗口尺寸。
5. 输入 Cookie JSON，启动后检查 Cookie 是否存在。

## 推荐提交顺序

1. 前端转换器和保存链路。
2. 后端 `profile_config` 持久化。
3. 编辑页回显逻辑。
4. 启动前/启动后动作执行器。
5. 预设、模板、备注等体验功能。
6. 测试和 Wails 类型生成。

## 注意事项

- 不要一次性重构所有浏览器模块。
- 不要删除用户现有配置字段。
- 不要让新增字段破坏旧 profile 的读取。
- Cookie 属于敏感数据，持久化前需要明确是否加密；如果只是创建时导入，优先作为一次性动作处理。
- 启动参数合并时要按“用户显式输入优先”处理冲突。
- 新增数据库列后同步备份/恢复逻辑。
