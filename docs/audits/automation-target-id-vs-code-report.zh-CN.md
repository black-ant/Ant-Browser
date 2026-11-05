# 自动化脚本目标标识方案报告

## 主题

在自动化脚本、Launch API 和实例管理场景中，是否应当用 `code` 替代 `profileId` 作为主要标识。

## 结论

`code 更适合做人看到、记住、手动输入和外部调用的主标识；profileId 更适合做系统内部稳定绑定。`

如果只允许保留一个，我不建议直接把内部绑定从 `profileId` 全量切到 `code`。

更合理的方案是：

- `对外`：`code-first`
- `对内`：`profileId-first`
- `展示层`：优先显示 `code`，把 `profileId` 下沉到高级信息
- `脚本持久化`：同时保存 `profileId` 和 `code` 快照，但解析以 `profileId` 为准

一句话判断：

`ID 不适合当用户主视角标识，但仍然适合当系统主键。code 适合成为产品层主标识，不适合单独承担全部内部绑定责任。`

## 当前实现事实

### 1. `profileId` 是实例主键

浏览器实例结构里，`profileId` 是实例记录的唯一标识，`launchCode` 是附加的人类友好字段。

相关位置：

- `backend/internal/browser/types.go`
- `backend/internal/launchcode/dao.go`

### 2. `code` 本质上是 `profileId -> code` 的映射层

当前 `LaunchCodeService` 维护的是：

- `profileToCode`
- `codeToProfile`

也就是说，`code` 不是主键本体，而是主键的可读别名。

相关位置：

- `backend/internal/launchcode/service.go`

### 3. `code` 是唯一的，但不是不可变的

当前实现支持：

- 自动生成 code
- 手动设置 code
- 重新生成 code

这意味着 `code` 虽然唯一，但它是可变的；一旦改码，原码就会释放。

相关位置：

- `backend/internal/launchcode/service.go`

### 4. `/api/launch` 已支持 `code` 和 `profileId`

Launch Selector 同时支持：

- `code`
- `profileId`
- `profileName`
- `groupId`
- `tags`
- `keywords`

而且当前匹配逻辑里，`code` 会先被解析成 `profileId`，再继续走后续筛选。

相关位置：

- `backend/internal/launchcode/selector_types.go`
- `backend/internal/launchcode/selector_match.go`

### 5. 自动化脚本的“使用已有实例”当前偏向 `profileId`

在脚本详情页里：

- `existing` 模式直接用 `profileId` 作为 select value
- `rotate` 模式同时支持 `code` 和 `profileId`
- 手动 JSON selector 例子也偏向 `code`

说明当前产品层语义没有完全统一。

相关位置：

- `frontend/src/modules/browser/pages/AutomationScriptDetailPage.tsx`

## 为什么你会觉得 `profileId` 不合理

这个判断在产品视角上是成立的。

`profileId` 的问题不是“技术上错”，而是“人机交互上错位”：

- 太长，记不住
- 没有业务语义
- 不适合手输
- 不适合口头沟通
- 不适合写文档和教程
- 不适合作为外部 API 的主要示例字段

如果用户看到的是：

- `05caae0a-58b8-4707-b8a9-2d81dc9df42c`

这对操作没有帮助。

如果用户看到的是：

- `BUYER_001`
- `SHOP_US_A`
- `WARM_TIKTOK_03`

这才是可操作、可沟通、可排障的标识。

所以：

`你说 code 更合适，这个在产品层是对的。`

## 为什么我不建议直接用 `code` 完全替代 `profileId`

因为 `code` 在当前系统里是“好用的别名”，不是“稳定的主键”。

### 1. `code` 可变

当前支持 `SetCode` 和 `RegenerateCode`。

如果脚本只存 `code`：

- 今天绑的是 `BUYER_001`
- 明天用户把它改成 `BUYER_A`
- 原脚本就失效

更糟的是，旧 code 之后还可能被别的实例占用。

### 2. `code` 可能漂移到另一实例

因为旧 code 被释放后，可以重新分配给别的 profile。

这会导致“脚本没有报错，但命中了错误实例”，这是比“直接失败”更危险的结果。

### 3. 随机生成的 code 不一定有业务语义

当前自动生成 code 是随机 6 位大写字母数字。

这比 UUID 好很多，但不一定天然有业务可读性。

只有当用户主动维护 code 命名规范时，`code` 才真正成为稳定业务标识。

### 4. 内部数据关联仍然更适合不可变 ID

数据库、缓存、脚本绑定、运行记录、导入导出、恢复数据，这些都更适合基于稳定主键工作。

如果内部主关联层改成可变 code，后续会出现更多迁移、冲突、历史兼容问题。

## 产品层判断

### 适合给用户看的主标识

应该是：

- `code`

不应该是：

- `profileId`

### 适合系统内部绑定的主标识

应该是：

- `profileId`

不应该是：

- 仅 `code`

### 适合外部 API 文档默认示例的字段

应该优先：

- `selector.code`

只在高级场景里提：

- `selector.profileId`

## 推荐方案

## 方案 A：纯 `code` 替代 `profileId`

优点：

- 用户理解成本最低
- 文档和外部调用更直观
- UI 展示更统一

缺点：

- code 可变，绑定不稳定
- 改码后脚本可能失效或漂移
- 数据恢复和内部关联会更脆弱

结论：

`不推荐直接采用。`

## 方案 B：继续维持 `profileId-first`

优点：

- 内部逻辑最稳定
- 历史兼容成本最低
- 不怕改名、改 code

缺点：

- 用户感知很差
- 外部接入不友好
- UI 和文档会持续让人困惑

结论：

`只适合内部实现，不适合作为产品层最终形态。`

## 方案 C：对外 code-first，对内 profileId-first

建议设计：

- UI 主展示：`code`
- UI 次展示：实例名
- 高级信息：`profileId`
- 脚本持久化：保存 `profileId + code`
- 运行解析：优先 `profileId`
- 若 `profileId` 失效，再尝试 `code`
- 若两者不一致，提示“目标实例已变更，请确认”

优点：

- 用户视角清晰
- 外部 API 友好
- 内部绑定稳定
- 能兼容 code 变更场景

缺点：

- 实现比单字段方案复杂一点
- 需要一层“绑定校验/修复”逻辑

结论：

`这是当前项目最合理的方向。`

## 对当前页面和脚本管理的具体建议

### 1. 列表页不要只写“使用已有实例”

应该直接显示：

- 目标实例：`实例名称`
- 目标 Code：`BUYER_001`
- 高级 ID：折叠显示 `profileId`

### 2. 脚本详情页 Existing 模式不要只存“看不见语义的 profileId”

建议改成：

- 下拉项主文案：`实例名 · Code`
- 持久化时同时保存：
  - `profileId`
  - `code`

### 3. 文档与 API 示例优先用 `code`

例如脚本执行示例应优先写：

```json
{
  "scriptId": "news-query-txt",
  "useScriptSelector": false,
  "selector": {
    "code": "BUYER_001"
  }
}
```

而不是默认展示 `profileId`。

### 4. 把 `profileId` 下沉到“高级/调试信息”

适合出现 `profileId` 的地方：

- 调试信息
- 导入导出原始数据
- 错误排查
- 高级编辑器

不适合出现 `profileId` 的地方：

- 普通列表主卡片
- 新手文档
- 操作按钮附近
- 外部调用示例首页

## 迁移建议

如果要往 `code-first` 方向收敛，建议分三步：

### 第 1 步：先改展示，不改底层绑定

- 列表页、详情页、运行弹窗都优先显示 `code`
- `profileId` 只留在高级信息

这是最低风险改法。

### 第 2 步：脚本配置持久化改为同时保存 `profileId + code`

- 兼容历史脚本
- 新保存脚本带上 code 快照
- 页面上明确提示当前绑定实例

### 第 3 步：增加绑定修复机制

当出现以下情况时提示用户：

- `profileId` 不存在
- `code` 已指向别的实例
- `profileId` 与 `code` 对不上

这样可以把“静默跑错实例”的风险降下来。

## 最终判断

如果问题是：

`profileId 适不适合继续当用户主视角字段？`

答案是：

`不适合。`

如果问题是：

`内部实现要不要彻底放弃 profileId，全部换成 code？`

答案是：

`也不建议。`

最合适的落地结论是：

`产品层改成 code-first，系统层继续保留 profileId-first。`

这既符合你的直觉，也符合当前代码库的稳定性要求。
