import { useEffect, useState } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { CheckCircle, ChevronRight, Copy, FileText } from 'lucide-react'
import { toast } from '../../../shared/components'
import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime'
import { fetchLaunchServerInfo, type LaunchServerInfo } from '../api'

// ============================================================================
// 文档内容（自动化优先重构版）
// ============================================================================

const DOC_OVERVIEW = `# 自动化接口文档（重构版）

## 文档目标

本页聚焦 **外部脚本 / 调度器通过 HTTP 创建并唤起实例** 的场景，重点回答 5 个问题：

1. 如何通过 HTTP 创建实例配置，并写入代理 / 标签 / 关键字 / 分组等信息
2. 如何通过 Code 或关键字直接唤起实例
3. 如何通过 \`profileId / profileName / keyword / tags / groupId\` 选择实例
4. 如何带参数启动，并拿到固定 \`cdpUrl\` 接入 CDP
5. 如何通过日志排查选择器命中和启动失败问题

## 协议定位

这是一组本地 \`HTTP + JSON\` 唤起接口，协议层只约定：

- HTTP 方法、路径、查询参数
- \`Content-Type: application/json\`
- JSON 请求体与 JSON 响应体
- 服务仅监听本机 \`127.0.0.1\`
- 如启用 \`launch_server.auth\`，所有 \`/api/*\` 请求还需附带 API Key Header

因此它和调用语言无关：

- Python / Node.js / Go / Java / C# / PowerShell / curl 都可以按同一协议调用
- 文档里的多语言代码，只是同一协议的不同客户端写法

## 当前支持能力

- 支持通过 \`/api/profiles\` 进行实例配置的创建、查询、更新、删除，并写入代理 / 标签 / 关键字 / 分组 / 启动参数等信息
- 兼容旧版：\`GET /api/launch/{code}\`
- 推荐主入口：\`POST /api/launch\`
- \`POST /api/launch\` 中的 \`code\` 字段支持“LaunchCode 优先，关键字兜底”
- 支持复杂选择器：\`code / profileId / profileName / key / keyword / keywords / tag / tags / groupId / matchMode\`
- \`selector\` 与顶层选择字段可混用，服务端会做归一化合并
- \`key\` 会优先精确命中 \`keywords[]\`；精确没命中时再参与模糊匹配
- 多命中时支持三种行为：\`unique\` / \`first\` / \`all\`
- 启动后返回：\`profileId / profileName / launchCode / pid / debugPort / debugReady / runtimeWarning / cdpPort / cdpUrl\`
- 外部统一使用 LaunchServer 固定端口接入 CDP，\`debugPort\` 仅表示内部实际调试端口
- 当 \`debugReady=false\` 时，表示浏览器窗口已拉起，但 CDP 仍在后台附着；此时 \`runtimeWarning\` 会说明当前限制
- 保留最近调用日志：\`GET /api/launch/logs\`，其中 \`selector\` 为归一化后的结构
- 可选 API Key 认证：仅保护 \`/api/*\`，不改变 CDP 统一入口的 localhost 访问模型

## 运行前提

- Ant Browser 应用已启动
- Launch 服务监听本机（地址见本页顶部）
- 如启用了 API 认证，请准备好请求头 \`X-Ant-Api-Key: <your-api-key>\`
- 如果你要用 \`key / keyword / tags\` 选择实例，需要先在实例配置里维护这些字段
- 如果你要用 \`groupId\`，请保证脚本拿到的是分组 ID，不是分组展示名

## 自动化链路

\`\`\`
任意语言客户端 / 调度器
  -> POST /api/profiles（可选：先创建实例配置）
  -> POST /api/launch
  -> 选择器解析实例
  -> 启动浏览器
  -> 返回 cdpUrl / debugReady
  -> Playwright / Selenium / 自研 CDP 客户端接管
\`\`\`
`

const DOC_QUICKSTART = `# 快速接入（3 分钟）

建议先用 \`curl\` 把协议跑通，再换成你自己的语言封装；请求路径、Header、JSON 结构保持不变。

## 第一步：准备实例标识

推荐至少准备一种稳定标识：

- \`launchCode\`：最稳，适合生产脚本（🔒 v1.2.0+ 长度为 12 位）
- \`profileId\`：适合系统内部编排
- \`key / keywords / tags\`：适合”按业务语义找实例”的场景

> 🔒 **安全提示**（v1.2.0+）：Launch Code 长度已从 6 位增加到 12 位，旧的 6 位 Code 将失效，需重新生成。

如果你准备使用关键字或标签：

1. 打开实例编辑页
2. 给目标实例填好 \`keywords\`
3. 视情况补上 \`tags\`、\`groupId\`

如果你的外部脚本手里只有”账号 / 业务关键字”：

- 推荐直接走 \`POST /api/launch\`
- 可以把账号或关键字直接放进 \`code\`
- 后端会先按真实 LaunchCode 查；查不到再按关键字匹配，并在多命中时默认取第一个
- 如果需要把所有命中实例都启动，显式传 \`matchMode=all\`

## 如果启用 API 认证

所有 \`/api/*\` 请求都需要追加认证头：

\`\`\`bash
curl -H "X-Ant-Api-Key: <your-api-key>" http://127.0.0.1:19876/api/health
\`\`\`

## 第二步：选择创建触发方式

推荐按你的编排方式选择下面三种触发模式：

1. 仅创建配置：\`POST /api/profiles\`，只写实例资料，不启动浏览器
2. 创建并立即启动：\`POST /api/profiles\` + \`autoLaunch=true\`
3. 先创建后再启动：先 \`POST /api/profiles\`，再 \`POST /api/launch\`

稳定性优先时，推荐第 3 种。这样创建和启动是两个独立动作，失败时更容易做幂等、补偿和重试。

## 第三步：健康检查

\`\`\`bash
curl http://127.0.0.1:19876/api/health
# {"ok":true}
\`\`\`

## 第四步：最简按 Code 启动

\`\`\`bash
curl http://127.0.0.1:19876/api/launch/A3F9K2
\`\`\`

成功后会返回 \`cdpUrl\`；如果同时返回 \`debugReady=false\`，说明浏览器已启动，但统一 CDP 入口还在等待附着完成。

## 第五步：推荐改为 POST 主入口

\`\`\`bash
curl -X POST http://127.0.0.1:19876/api/launch \\
  -H "Content-Type: application/json" \\
  -d '{
    "selector": {
      "code": "A3F9K2"
    },
    "launchArgs": ["--window-size=1280,800"],
    "startUrls": ["https://example.com"],
    "skipDefaultStartUrls": true
  }'
\`\`\`

如果你只有“账号 / 关键字”，也可以直接这样写：

\`\`\`bash
curl -X POST http://127.0.0.1:19876/api/launch \\
  -H "Content-Type: application/json" \\
  -d '{
    "code": "buyer-001",
    "launchArgs": ["--window-size=1280,800"],
    "startUrls": ["https://example.com"],
    "skipDefaultStartUrls": true
  }'
\`\`\`

## 第六步：复杂场景改用选择器

\`\`\`bash
curl -X POST http://127.0.0.1:19876/api/launch \\
  -H "Content-Type: application/json" \\
  -d '{
    "selector": {
      "keyword": "checkout",
      "tags": ["电商", "北美"],
      "groupId": "group-sales-us",
      "matchMode": "unique"
    },
    "skipDefaultStartUrls": true
  }'
\`\`\`
`

const DOC_SELECTOR = `# 目标实例选择器

## 设计目标

\`POST /api/launch\` 不再只接受 \`code\`，而是支持一组 **可组合** 的选择条件。

- 单一明确标识：用 \`code\` 或 \`profileId\`
- 人类可读标识：用 \`profileName\`
- 业务语义匹配：用 \`keyword / keywords / tag / tags / groupId\`

补充说明：

- 在 \`POST /api/launch\` 下，\`code\` 既可以传真实 LaunchCode，也可以传账号 / 关键字
- 如果 \`code\` 不是有效 LaunchCode，后端会自动按关键字继续匹配
- 这种“关键字兜底”只作用于 \`POST /api/launch\`
- 旧版 \`GET /api/launch/{code}\` 仍然只接受真实 Code

## 推荐写法

推荐把所有选择条件放进 \`selector\` 对象：

\`\`\`json
{
  "selector": {
    "keyword": "checkout",
    "tags": ["电商"],
    "groupId": "group-sales-us",
    "matchMode": "unique"
  },
  "skipDefaultStartUrls": true
}
\`\`\`

兼容旧版写法，也允许把字段直接放在顶层：

\`\`\`json
{
  "keyword": "checkout",
  "tags": ["电商"],
  "groupId": "group-sales-us"
}
\`\`\`

顶层字段和 \`selector\` 也可以同时出现，合并规则如下：

- 同名单值字段以 \`selector\` 内为准：\`code / key / profileId / profileName / groupId / matchMode\`
- \`keyword / keywords\` 会合并、去重，并统一归一化到 \`keywords\`
- \`tag / tags\` 会合并、去重，并统一归一化到 \`tags\`

## 选择字段

| 字段 | 类型 | 匹配方式 | 说明 |
|------|------|----------|------|
| \`code\` | string | 精确匹配 / 关键字兜底 | 会自动 trim 并转成大写；先按 LaunchCode 查；仅在 POST 请求里，查不到时再按关键字匹配 |
| \`profileId\` | string | 精确匹配 | 实例唯一 ID |
| \`profileName\` | string | 精确匹配 | 名称忽略大小写，适合名称唯一的实例 |
| \`key\` | string | 先精确后模糊 | 先在实例 \`keywords[]\` 中做精确相等；若没有精确命中，再按 contains 模糊匹配 |
| \`keyword\` | string | 模糊匹配 | 在实例 \`keywords[]\` 中做 contains；日志里会归一化到 \`keywords[]\` |
| \`keywords\` | string[] | 多词 AND | 每个查询词都必须命中某个关键字 |
| \`tag\` | string | 精确匹配 | 标签完全相等，忽略大小写；日志里会归一化到 \`tags[]\` |
| \`tags\` | string[] | 多标签 AND | 实例必须包含全部标签 |
| \`groupId\` | string | 精确匹配 | 只匹配指定分组 ID |
| \`matchMode\` | string | 行为控制 | \`unique\` / \`first\` / \`all\`；\`code / key / keyword / keywords\` 默认 \`first\`，其他默认 \`unique\` |

## 组合规则

- 所有已提供条件按 **AND** 组合
- \`keywords\` 是 AND 关系
- \`tags\` 也是 AND 关系
- \`keyword\` 会归一化到 \`keywords\`
- \`key\` 保持独立语义：优先精确匹配，未命中时再参与模糊匹配
- \`tag\` 会并入 \`tags\`

## matchMode 规则

| 值 | 含义 |
|----|------|
| \`unique\` | 显式要求唯一。命中 0 个返回 404，命中多个返回 409 |
| \`first\` | 当命中多个实例时，按后端稳定顺序取第一个；\`key / keyword / keywords\` 默认就是这个行为 |
| \`all\` | 当命中多个实例时，按后端稳定顺序依次启动全部实例，并返回 \`items[]\` |

## 什么时候该用哪个字段

- 稳定生产脚本：优先 \`code\`
- 系统内部编排：优先 \`profileId\`
- 名称严格唯一：可用 \`profileName\`
- 一类实例共享规则：用 \`keyword + tags + groupId\`
- 外部脚本只有账号 / 关键字：可直接把值传到 \`POST.code\`
- 容许“取第一个命中实例”：加 \`matchMode=first\`
- 需要“把所有命中实例都启动”：加 \`matchMode=all\`
`

const DOC_API_INDEX = `# 接口总览

| 能力 | 方法 | 路径 | 用途 |
|------|------|------|------|
| 健康检查 | GET | \`/api/health\` | 检查 Launch 服务是否可用 |
| 实例配置管理 | GET / POST | \`/api/profiles\` | 查询实例列表，或创建包含代理/标签/关键字/分组的实例配置 |
| 单实例配置管理 | GET / PUT / DELETE | \`/api/profiles/{profileId}\` | 查询、更新、删除指定实例配置 |
| 按 Code 启动 | GET | \`/api/launch/{code}\` | 兼容旧版、最快捷的唤起方式 |
| 选择器启动 | POST | \`/api/launch\` | 支持 code / profileId / 名称 / 关键字 / 标签 / 分组 |
| CDP 统一入口 | GET / WS | \`/json/version\`、\`/json/list\`、\`/devtools/...\` | 将非 \`/api\` 请求代理到当前活动实例 |
| 调用记录 | GET | \`/api/launch/logs?limit=50\` | 查看最近接口调用与错误 |

说明：

- 如已启用 API 认证，所有 \`/api/*\` 请求都需要追加 \`X-Ant-Api-Key: <your-api-key>\`
- CDP 统一入口仍只受 localhost 限制，不额外读取 API Key
`

const DOC_API_HEALTH = `# 接口：健康检查

\`\`\`
GET /api/health
\`\`\`

## 请求示例

\`\`\`bash
curl http://127.0.0.1:19876/api/health
\`\`\`

## 成功响应

\`\`\`json
{
  "ok": true
}
\`\`\`
`

const DOC_API_PROFILES = `# 接口：实例配置管理

\`\`\`
GET    /api/profiles
POST   /api/profiles
GET    /api/profiles/{profileId}
PUT    /api/profiles/{profileId}
DELETE /api/profiles/{profileId}
\`\`\`

## 说明

- 用于外部脚本完整管理实例配置：先查、再创建、后续更新，最后按需删除
- \`profile\` 内是持久化配置，会保存到实例资料里
- \`start\` 内是本次自动启动的临时参数；只有 \`autoLaunch=true\` 时才会使用
- 如果传 \`launchCode\`，会尝试设置为自定义启动码；重复时返回 \`409\`
- \`DELETE /api/profiles/{profileId}\` 会拒绝删除运行中的实例，避免留下未托管进程

## 触发创建的 3 种方式

### 方式 1：仅创建配置

- 适合先建档、后续再由别的任务决定何时启动
- 请求里只传 \`profile\`，可选传 \`launchCode\`
- 成功后返回 \`created=true\`、\`launched=false\`

### 方式 2：创建并立即启动

- 在 \`POST /api/profiles\` 里同时传 \`autoLaunch=true\`
- 如需给这次启动加临时参数，放进 \`start\`
- 成功后返回 \`created=true\`、\`launched=true\`，并附带 \`pid / debugReady / cdpUrl\`

### 方式 3：先创建，再单独触发启动

- 第一步：\`POST /api/profiles\`
- 第二步：读取响应里的 \`launchCode\` 或 \`profileId\`
- 第三步：再调用 \`POST /api/launch\` 或 \`GET /api/launch/{code}\`

推荐这个模式作为默认编排方式，因为它把“配置持久化成功”和“浏览器实际拉起成功”分离开了，更适合脚本做重试和补偿。

## 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| \`profile\` | object | 是 | 实例配置对象 |
| \`profile.profileName\` | string | 是 | 实例名称 |
| \`profile.userDataDir\` | string | 否 | 用户数据目录；为空时自动生成 |
| \`profile.coreId\` | string | 否 | 指定浏览器内核 |
| \`profile.fingerprintArgs\` | string[] | 否 | 持久化指纹参数 |
| \`profile.proxyId\` | string | 否 | 代理池中的代理 ID；传它时会自动回填 \`proxyConfig\` |
| \`profile.proxyConfig\` | string | 否 | 直接写死的代理配置，如 \`http://user:pass@host:port\` |
| \`profile.launchArgs\` | string[] | 否 | 实例默认启动参数，会持久化 |
| \`profile.tags\` | string[] | 否 | 实例标签 |
| \`profile.keywords\` | string[] | 否 | 实例关键字，供 \`/api/launch\` 检索 |
| \`profile.groupId\` | string | 否 | 所属分组 ID |
| \`launchCode\` | string | 否 | 自定义启动码，4-32 位，字符集 \`A-Z 0-9 _ -\` |
| \`autoLaunch\` | boolean | 否 | 创建后是否立即启动 |
| \`start.launchArgs\` | string[] | 否 | 本次自动启动附加参数，不持久化 |
| \`start.startUrls\` | string[] | 否 | 本次自动启动打开的 URL |
| \`start.skipDefaultStartUrls\` | boolean | 否 | 本次自动启动时是否跳过系统默认起始页 |

## 示例 1：创建一个绑定代理池节点的实例

\`\`\`bash
curl -X POST http://127.0.0.1:19876/api/profiles \\
  -H "Content-Type: application/json" \\
  -d '{
    "profile": {
      "profileName": "buyer-001",
      "userDataDir": "buyers/buyer-001",
      "proxyId": "proxy-us",
      "launchArgs": ["--lang=en-US"],
      "tags": ["电商", "北美"],
      "keywords": ["buyer-001", "amazon"],
      "groupId": "group-sales-us"
    },
    "launchCode": "BUYER_001"
  }'
\`\`\`

## 示例 2：创建后立刻启动，并带一次性打开页面参数

\`\`\`bash
curl -X POST http://127.0.0.1:19876/api/profiles \\
  -H "Content-Type: application/json" \\
  -d '{
    "profile": {
      "profileName": "buyer-002",
      "proxyConfig": "http://user:pass@127.0.0.1:8080",
      "launchArgs": ["--disable-sync"],
      "keywords": ["buyer-002"]
    },
    "autoLaunch": true,
    "start": {
      "launchArgs": ["--window-size=1280,800"],
      "startUrls": ["https://example.com/order"],
      "skipDefaultStartUrls": true
    }
  }'
\`\`\`

## 成功响应：仅创建

\`\`\`json
{
  "ok": true,
  "created": true,
  "launched": false,
  "profileId": "550e8400-e29b-41d4-a716-446655440000",
  "profileName": "buyer-001",
  "launchCode": "BUYER_001",
  "profile": {
    "profileId": "550e8400-e29b-41d4-a716-446655440000",
    "profileName": "buyer-001",
    "proxyId": "proxy-us",
    "proxyConfig": "socks5://127.0.0.1:1080",
    "tags": ["电商", "北美"],
    "keywords": ["buyer-001", "amazon"],
    "groupId": "group-sales-us"
  }
}
\`\`\`

## 成功响应：创建并启动

\`\`\`json
{
  "ok": true,
  "created": true,
  "launched": true,
  "profileId": "550e8400-e29b-41d4-a716-446655440000",
  "profileName": "buyer-002",
  "launchCode": "A3F9K2",
  "pid": 12345,
  "debugPort": 9222,
  "debugReady": true,
  "runtimeWarning": "",
  "cdpPort": 9222,
  "cdpUrl": "http://127.0.0.1:9222",
  "profile": {
    "profileId": "550e8400-e29b-41d4-a716-446655440000",
    "profileName": "buyer-002",
    "proxyConfig": "http://user:pass@127.0.0.1:8080",
    "running": true
  }
}
\`\`\`

## 示例 3：先创建，再按返回的 launchCode 单独触发启动

\`\`\`bash
# 第一步：创建实例配置
curl -X POST http://127.0.0.1:19876/api/profiles \\
  -H "Content-Type: application/json" \\
  -d '{
    "profile": {
      "profileName": "buyer-003",
      "keywords": ["buyer-003"]
    }
  }'

# 假设响应里返回 launchCode=A8KQ21

# 第二步：后续任务再单独触发启动
curl -X POST http://127.0.0.1:19876/api/launch \\
  -H "Content-Type: application/json" \\
  -d '{
    "code": "A8KQ21",
    "skipDefaultStartUrls": true
  }'
\`\`\`

## 列表查询

\`\`\`bash
curl http://127.0.0.1:19876/api/profiles
\`\`\`

成功响应示例：

\`\`\`json
{
  "ok": true,
  "count": 2,
  "items": [
    {
      "profileId": "profile-a",
      "profileName": "buyer-001",
      "launchCode": "BUYER_001",
      "proxyId": "proxy-us",
      "tags": ["电商", "北美"]
    },
    {
      "profileId": "profile-b",
      "profileName": "buyer-002",
      "launchCode": "A3F9K2",
      "proxyConfig": "http://user:pass@127.0.0.1:8080"
    }
  ]
}
\`\`\`

## 单实例查询

\`\`\`bash
curl http://127.0.0.1:19876/api/profiles/550e8400-e29b-41d4-a716-446655440000
\`\`\`

## 更新实例配置

\`\`\`bash
curl -X PUT http://127.0.0.1:19876/api/profiles/550e8400-e29b-41d4-a716-446655440000 \\
  -H "Content-Type: application/json" \\
  -d '{
    "profile": {
      "profileName": "buyer-001-updated",
      "userDataDir": "buyers/buyer-001",
      "proxyId": "proxy-us",
      "launchArgs": ["--lang=en-US"],
      "tags": ["电商", "北美"],
      "keywords": ["buyer-001", "amazon"],
      "groupId": "group-sales-us"
    },
    "launchCode": "BUYER_001",
    "autoLaunch": true,
    "start": {
      "startUrls": ["https://example.com/order"],
      "skipDefaultStartUrls": true
    }
  }'
\`\`\`

说明：

- \`PUT\` 采用“整份配置更新”语义，建议先 \`GET /api/profiles/{profileId}\` 再修改后回写
- 如果 \`launchCode\` 冲突，后端会回滚本次更新，避免配置部分落库

## 删除实例配置

\`\`\`bash
curl -X DELETE http://127.0.0.1:19876/api/profiles/550e8400-e29b-41d4-a716-446655440000
\`\`\`

成功响应示例：

\`\`\`json
{
  "ok": true,
  "deleted": true,
  "profileId": "550e8400-e29b-41d4-a716-446655440000",
  "profileName": "buyer-001",
  "launchCode": "BUYER_001"
}
\`\`\`
`

const DOC_API_LAUNCH_GET = `# 接口：按 Code 启动

\`\`\`
GET /api/launch/{code}
\`\`\`

## 说明

- 适合“我已经知道唯一 Code”的场景
- 只支持 \`code\`，不支持复杂选择器
- 不支持关键字兜底；如果你想传账号 / 关键字，请改用 \`POST /api/launch\`
- 实例已运行时返回当前运行信息（幂等）

## 请求示例

\`\`\`bash
curl http://127.0.0.1:19876/api/launch/A3F9K2
\`\`\`

## 成功响应

\`\`\`json
{
  "ok": true,
  "profileId": "550e8400-e29b-41d4-a716-446655440000",
  "profileName": "账号 A",
  "launchCode": "A3F9K2",
  "pid": 12345,
  "debugPort": 9222,
  "debugReady": true,
  "runtimeWarning": "",
  "cdpPort": 19876,
  "cdpUrl": "http://127.0.0.1:19876"
}
\`\`\`
`

const DOC_API_LAUNCH_POST = `# 接口：选择器启动（自动化主入口）

\`\`\`
POST /api/launch
\`\`\`

## 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| \`selector\` | object | 否 | 推荐写法，放复杂选择条件 |
| \`code\` | string | 否 | 兼容旧版 flat 写法；POST 下支持“Code 优先，关键字兜底” |
| \`profileId\` | string | 否 | 顶层选择字段 |
| \`profileName\` | string | 否 | 顶层选择字段 |
| \`key / keyword / keywords\` | string / string[] | 否 | 顶层关键字条件 |
| \`tag / tags\` | string / string[] | 否 | 顶层标签条件 |
| \`groupId\` | string | 否 | 顶层分组条件 |
| \`matchMode\` | string | 否 | \`unique\` / \`first\` / \`all\` |
| \`launchArgs\` | string[] | 否 | 本次附加启动参数 |
| \`startUrls\` | string[] | 否 | 本次启动打开 URL 列表 |
| \`skipDefaultStartUrls\` | boolean | 否 | 跳过系统默认起始页 |

> 注意：\`selector\` 与顶层选择字段可以混用，不是互斥关系。重名单值字段以 \`selector\` 为准；\`keyword / keywords\`、\`tag / tags\` 会合并并去重。至少要提供一个选择字段。

## 示例 1：旧版 code 写法

\`\`\`bash
curl -X POST http://127.0.0.1:19876/api/launch \\
  -H "Content-Type: application/json" \\
  -d '{
    "code":"A3F9K2",
    "skipDefaultStartUrls":true
  }'
\`\`\`

## 示例 1B：\`code\` 直接传账号 / 关键字

\`\`\`bash
curl -X POST http://127.0.0.1:19876/api/launch \\
  -H "Content-Type: application/json" \\
  -d '{
    "code":"buyer-001",
    "launchArgs":["--window-size=1280,800","--lang=en-US"],
    "startUrls":["https://example.com"],
    "skipDefaultStartUrls":true
  }'
\`\`\`

说明：

- 先按真实 LaunchCode 查
- 查不到就把 \`buyer-001\` 当关键字去匹配实例
- 如果命中多个实例，默认取第一个

## 示例 1C：\`code\` 传关键字，并把所有命中实例都启动

\`\`\`bash
curl -X POST http://127.0.0.1:19876/api/launch \\
  -H "Content-Type: application/json" \\
  -d '{
    "code":"shop",
    "matchMode":"all",
    "skipDefaultStartUrls":true
  }'
\`\`\`

## 示例 2：按 profileId 启动

\`\`\`bash
curl -X POST http://127.0.0.1:19876/api/launch \\
  -H "Content-Type: application/json" \\
  -d '{
    "selector": {
      "profileId":"550e8400-e29b-41d4-a716-446655440000"
    },
    "launchArgs":["--lang=en-US"]
  }'
\`\`\`

## 示例 3：按唯一名称启动

\`\`\`bash
curl -X POST http://127.0.0.1:19876/api/launch \\
  -H "Content-Type: application/json" \\
  -d '{
    "selector": {
      "profileName":"账号 A"
    }
  }'
\`\`\`

## 示例 4：按关键字 + 标签 + 分组定位实例

\`\`\`bash
curl -X POST http://127.0.0.1:19876/api/launch \\
  -H "Content-Type: application/json" \\
  -d '{
    "selector": {
      "keyword":"checkout",
      "tags":["电商","北美"],
      "groupId":"group-sales-us",
      "matchMode":"unique"
    },
    "startUrls":["https://example.com/order"],
    "skipDefaultStartUrls":true
  }'
\`\`\`

## 示例 5：多关键字 AND 匹配

\`\`\`bash
curl -X POST http://127.0.0.1:19876/api/launch \\
  -H "Content-Type: application/json" \\
  -d '{
    "selector": {
      "keywords":["amazon","buyer","checkout"],
      "tags":["电商"],
      "matchMode":"unique"
    }
  }'
\`\`\`

## 示例 6：允许多命中时取第一个

\`\`\`bash
curl -X POST http://127.0.0.1:19876/api/launch \\
  -H "Content-Type: application/json" \\
  -d '{
    "selector": {
      "keyword":"shop",
      "matchMode":"first"
    }
  }'
\`\`\`

## 示例 7：允许多命中时全部启动

\`\`\`bash
curl -X POST http://127.0.0.1:19876/api/launch \\
  -H "Content-Type: application/json" \\
  -d '{
    "selector": {
      "keyword":"shop",
      "matchMode":"all"
    }
  }'
\`\`\`

## 成功响应：单实例

\`\`\`json
{
  "ok": true,
  "profileId": "550e8400-e29b-41d4-a716-446655440000",
  "profileName": "账号 A",
  "launchCode": "A3F9K2",
  "pid": 12345,
  "debugPort": 9222,
  "debugReady": true,
  "runtimeWarning": "",
  "cdpPort": 19876,
  "cdpUrl": "http://127.0.0.1:19876"
}
\`\`\`

## 成功响应：多实例（\`matchMode=all\`）

\`\`\`json
{
  "ok": true,
  "matchMode": "all",
  "count": 2,
  "activeProfileId": "profile-b",
  "activeProfileName": "账号 B",
  "cdpPort": 19876,
  "cdpUrl": "http://127.0.0.1:19876",
  "items": [
    {
      "profileId": "profile-a",
      "profileName": "账号 A",
      "launchCode": "A3F9K2",
      "pid": 12345,
      "debugPort": 9222,
      "debugReady": true,
      "runtimeWarning": "",
      "isActive": false
    },
    {
      "profileId": "profile-b",
      "profileName": "账号 B",
      "launchCode": "B7Q2W9",
      "pid": 12346,
      "debugPort": 9333,
      "debugReady": true,
      "runtimeWarning": "",
      "isActive": true
    }
  ]
}
\`\`\`

补充说明：

- \`activeProfileId\` 是当前统一 CDP 入口实际指向的实例
- \`matchMode=all\` 时，会按后端稳定顺序依次启动；最后一个成功启动的实例会成为活动实例
`

const DOC_API_CDP = `# 接口：CDP 统一入口

\`\`\`
GET /json/version
GET /json/list
GET /devtools/...
WS  /devtools/...
\`\`\`

## 说明

- LaunchServer 会把所有非 \`/api\` 请求代理到当前活动实例的内部调试端口
- 当前活动实例等于最近一次成功启动的实例
- 如果使用 \`matchMode=all\`，则最后一个启动成功的实例会成为当前活动实例
- 如果启动响应里 \`debugReady=false\`，说明当前实例暂时还不会成为活动实例，直到后台附着完成
- 如果当前没有活动实例，请求会返回 \`503\` 和 \`no active browser debug target\`

## 请求示例

\`\`\`bash
curl http://127.0.0.1:19876/json/version
\`\`\`

\`\`\`bash
curl http://127.0.0.1:19876/json/list
\`\`\`

## 使用建议

- 推荐直接把启动响应里的 \`cdpUrl\` 交给 Playwright / Selenium / 自研 CDP 客户端
- 如果你手动请求 \`/json/version\` 或 \`/json/list\`，返回内容由目标 Chrome/CDP 直接提供，字段会随 Chrome 版本变化
`

const DOC_API_LOGS = `# 接口：调用记录

\`\`\`
GET /api/launch/logs?limit=50
\`\`\`

## 说明

- 默认返回最近 50 条
- 最大支持 200 条
- 返回顺序：按时间倒序（最新在前）
- 不管是按 Code 启动，还是按关键字/标签启动，都会记录当次使用的 selector
- 日志里的 \`selector\` 是归一化结果：例如 \`keyword\` 会显示为 \`keywords[]\`，\`tag\` 会显示为 \`tags[]\`

## 请求示例

\`\`\`bash
curl http://127.0.0.1:19876/api/launch/logs?limit=20
\`\`\`

## 成功响应

\`\`\`json
{
  "ok": true,
  "items": [
    {
      "timestamp": "2026-03-01T12:00:00+08:00",
      "method": "POST",
      "path": "/api/launch",
      "clientIp": "127.0.0.1",
      "code": "A3F9K2",
      "selector": {
        "keywords": ["checkout"],
        "tags": ["电商", "北美"],
        "groupId": "group-sales-us",
        "matchMode": "unique"
      },
      "profileId": "550e8400-e29b-41d4-a716-446655440000",
      "profileName": "账号 A",
      "params": {
        "launchArgs": ["--window-size=1280,800"],
        "startUrls": ["https://example.com/order"],
        "skipDefaultStartUrls": true
      },
      "ok": true,
      "status": 200,
      "error": "",
      "durationMs": 156
    }
  ]
}
\`\`\`
`

const DOC_SCENARIOS = `# 场景示例

## 场景 1：生产环境固定实例

用 \`code\` 或 \`profileId\`。

原因：

- 最稳定
- 不怕名称变更
- 不怕关键字维护失误

## 场景 1B：外部脚本只有账号 / 业务关键字

直接用 \`POST /api/launch\` 的 \`code\` 字段。

示例：

\`\`\`json
{
  "code": "buyer-001",
  "skipDefaultStartUrls": true
}
\`\`\`

说明：

- 这是给外部脚本最省事的写法
- 后端会先按真实 Code 查，再按关键字兜底
- 如果一类实例会命中多个，默认取第一个
- 如果你就是要全起，显式加 \`matchMode=all\`

## 场景 2：一个业务线下有多组实例

用 \`keyword + tags + groupId\`。

示例：

\`\`\`json
{
  "selector": {
    "keyword": "checkout",
    "tags": ["电商", "北美"],
    "groupId": "group-sales-us",
    "matchMode": "unique"
  }
}
\`\`\`

## 场景 3：批量模板实例，需要把命中的实例全部启动

用 \`keyword + matchMode=all\`。

示例：

\`\`\`json
{
  "selector": {
    "keyword": "template",
    "matchMode": "all"
  }
}
\`\`\`

## 场景 4：想按“业务语义”启动，但又担心误命中

用多条件收窄：

\`\`\`json
{
  "selector": {
    "keywords": ["amazon", "buyer", "checkout"],
    "tags": ["电商", "北美"],
    "groupId": "group-sales-us",
    "matchMode": "unique"
  }
}
\`\`\`

## 场景 5：脚本第一次靠关键字命中，后续改用 Code

返回体里会附带 \`launchCode\`，你可以把它缓存下来，下一次直接用 Code 启动。
`

const DOC_ERRORS = `# 错误码与重试策略

| 状态码 | 场景 | 建议处理 |
|--------|------|----------|
| 400 | 请求体非法 / 含未知字段 / selector 缺失 / matchMode 非法 / launchCode 格式错误 | 修复参数后重试 |
| 401 | 已启用 API 认证，但缺少或写错 API Key | 补上正确的 \`X-Ant-Api-Key\` 请求头后重试 |
| 403 | 非 localhost 访问 | 改为本机请求 |
| 404 | GET 的 Code 不存在 / POST 的 code 关键字兜底后仍未命中 / selector 没命中实例 | 检查 code、keywords、tags、groupId |
| 405 | 方法错误 | 使用正确 HTTP 方法 |
| 409 | selector 命中多个实例 / 创建或更新时 launchCode 冲突 / 达到实例上限 / 删除运行中实例 | 收窄条件、换一个 launchCode，或先停掉实例后重试 |
| 429 | 🔒 速率限制：Launch Code 解析超过 10 次/秒 | 降低请求频率，实现客户端速率控制 |
| 500 | 启动失败 | 查 \`/api/launch/logs\` + 应用日志 |
| 503 | 访问 CDP 统一入口时还没有活动实例，或启动响应仍处于 \`debugReady=false\` | 先确认启动接口成功，再等待 \`debugReady=true\` 后访问 \`cdpUrl\` |

## 🔒 安全增强（v1.2.0）

**错误消息保护**：为防止信息泄漏，错误消息不再包含敏感信息（如具体的 code、profileId、sessionId 值）。建议使用 HTTP 状态码判断错误类型，而非依赖错误消息文本。

**响应头增强**：所有 API 响应现在包含安全响应头：
- \`X-Content-Type-Options: nosniff\`
- \`X-Frame-Options: DENY\`
- \`Content-Security-Policy: default-src 'none'\`
- \`X-XSS-Protection: 1; mode=block\`

**速率限制**：
- Launch Code 解析限制为 10 次/秒（令牌桶算法）
- 超限返回 429 状态码和 \`{"ok":false,"error":"rate limit exceeded"}\`
- 建议客户端实现请求队列和速率控制

**审计日志**：认证失败会被记录到应用日志，便于检测异常行为和暴力破解尝试。

## 自动化建议

- 设置请求超时（3-10 秒）
- 对 \`500\` 可短暂重试（指数退避）
- 对 \`400/404/409\` 不建议盲目重试
- 🔒 对 \`429\` 必须实现退避策略，建议等待 1-2 秒后重试
- 对复杂 selector，先在低风险环境验证日志是否命中正确实例
- 使用 HTTP 状态码而非错误消息文本判断错误类型
`

const DOC_EXAMPLES = `# 多语言调用示例（同一协议）

下面这些示例调用的是同一组 HTTP 接口，只是客户端语法不同。你可以直接替换成自己的语言或框架实现。

如果你启用了 API 认证，请记得在各语言客户端里补上 \`X-Ant-Api-Key\` 请求头。

## Python：触发创建并立即启动

\`\`\`python
import requests

BASE = "http://127.0.0.1:19876"

def create_profile(auto_launch: bool = False) -> dict:
    res = requests.post(
        f"{BASE}/api/profiles",
        json={
            "profile": {
                "profileName": "buyer-100",
                "proxyId": "proxy-us",
                "keywords": ["buyer-100"],
            },
            "autoLaunch": auto_launch,
            "start": {
                "startUrls": ["https://example.com/order"],
                "skipDefaultStartUrls": True,
            } if auto_launch else None,
        },
        timeout=10,
    )
    data = res.json()
    if not res.ok or not data.get("ok"):
        raise RuntimeError(data.get("error", f"HTTP {res.status_code}"))
    return data
\`\`\`

## Python：按关键字启动并连接 CDP

\`\`\`python
import requests
from playwright.sync_api import sync_playwright

BASE = "http://127.0.0.1:19876"

def launch(selector: dict) -> dict:
    res = requests.post(
        f"{BASE}/api/launch",
        json={
            "selector": selector,
            "skipDefaultStartUrls": True,
        },
        timeout=10,
    )
    data = res.json()
    if not res.ok or not data.get("ok"):
        raise RuntimeError(data.get("error", f"HTTP {res.status_code}"))
    return data

with sync_playwright() as p:
    data = launch({
        "keyword": "checkout",
        "tags": ["电商", "北美"],
        "groupId": "group-sales-us",
        "matchMode": "unique",
    })
    browser = p.chromium.connect_over_cdp(data["cdpUrl"])
    page = browser.contexts[0].new_page()
    page.goto("https://example.com")
\`\`\`

## Node.js：封装通用 launch(selector)

\`\`\`javascript
const BASE = 'http://127.0.0.1:19876'

async function launch(selector, extra = {}) {
  const res = await fetch(\`\${BASE}/api/launch\`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      selector,
      skipDefaultStartUrls: true,
      ...extra,
    }),
  })

  const data = await res.json()
  if (!res.ok || !data.ok) {
    throw new Error(data.error || \`HTTP \${res.status}\`)
  }
  return data
}

await launch({
  keywords: ['amazon', 'buyer', 'checkout'],
  tags: ['电商'],
  matchMode: 'unique',
})
\`\`\`

## PowerShell：按名称或关键字启动

\`\`\`powershell
$body = @{
  code = "buyer-001"
  skipDefaultStartUrls = $true
} | ConvertTo-Json -Depth 6

Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:19876/api/launch" -ContentType "application/json" -Body $body
\`\`\`

## cURL：\`code\` 传关键字

\`\`\`bash
curl -X POST http://127.0.0.1:19876/api/launch \\
  -H "Content-Type: application/json" \\
  -d '{
    "code":"buyer-001",
    "skipDefaultStartUrls":true
  }'
\`\`\`
`

const DOC_PRACTICES = `# 最佳实践

## 1) 稳定标识优先级

推荐优先级：

1. \`code\`
2. \`profileId\`
3. \`profileName\`
4. \`keyword / tags / groupId\`

补充：

- 如果外部脚本只有账号 / 关键字，又不想额外构造 selector，优先用 \`POST.code\`

## 2) 关键字维护规范

- \`keywords\` 里尽量放稳定业务词，不要放容易漂移的描述
- 同一类实例的关键字保持风格统一
- 如果脚本要靠关键字精确命中，至少再加一个 \`tag\` 或 \`groupId\`

## 3) 标签与分组策略

- \`tags\` 用来表达能力或属性，例如：\`电商\`、\`北美\`、\`付款\`
- \`groupId\` 用来表达组织归属，例如：\`group-sales-us\`
- 不建议只靠单个宽泛标签做生产启动条件

## 4) 启动参数策略

- 把通用参数放在实例默认配置
- 把任务临时参数放在 \`launchArgs\`
- 只在当前任务需要时才传 \`startUrls\`

## 5) 创建触发策略

- 批量导入或配置预热：用“仅创建配置”
- 单任务一次性跑通：用“创建并立即启动”
- 生产调度和高可用编排：优先“先创建，再单独触发启动”

## 6) 排障流程

1. 先调 \`/api/health\`
2. 再调 \`POST /api/launch\`
3. 如果报 \`409\`，先收窄 selector；如果业务允许多命中，再改 \`matchMode=first\` 或 \`matchMode=all\`
4. 如果报 \`500\`，再查 \`/api/launch/logs\` 和应用日志
`

const DOC_TROUBLESHOOT = `# 常见问题

## Q1：返回 \`selector is required\`

- 你没有提供 \`selector\`
- 也没有提供顶层的 \`code / profileId / keyword / tags\` 等字段

## Q2：返回 \`launch code not found\`

- Code 拼写错误
- 目标实例没有这个 Code
- 你把自定义 Code 改过，但脚本没同步
- 这通常是 \`GET /api/launch/{code}\` 或“你明确想按真实 Code 启动”的报错
- 如果你传的是账号 / 关键字，请改用 \`POST /api/launch\`

## Q3：返回 \`profile selector matched no instance\`

- 关键字没有配置到实例的 \`keywords\`
- \`tags\` 或 \`groupId\` 条件写错
- 组合条件过严，导致 0 命中
- 或者你在 \`POST\` 里把 \`code\` 当关键字传了，但该关键字没有命中任何实例

## Q4：返回 \`selector matched multiple profiles\`

- 说明关键字过宽或标签过少
- 优先补 \`groupId\` / 更多 \`tags\`
- 如果业务允许，显式加 \`matchMode=first\`
- 如果你要把这些命中实例全部启动，改用 \`matchMode=all\`

## Q5：返回 \`unauthorized: invalid api key\`

- 说明当前已经启用 API 认证
- 你没有传认证头，或传错了 API Key
- 请检查请求头 \`X-Ant-Api-Key: <your-api-key>\` 是否正确

## Q6：返回 \`forbidden: only localhost is allowed\`

- 当前服务只允许本机访问
- 请在同一台机器发起请求

## Q7：返回 \`500\` 启动失败

- 先看 \`/api/launch/logs\` 里的 \`error\`
- 再检查内核路径、代理配置、启动参数是否合法
- 如果是复杂 selector，先确认命中的实例就是你预期那一个

## Q8：访问 \`cdpUrl\` 返回 \`no active browser debug target\`

- 说明当前还没有活动实例
- 先调用一次 \`GET /api/launch/{code}\` 或 \`POST /api/launch\`
- 如果启动响应里 \`debugReady=false\`，说明实例正在后台附着，稍后再访问 \`cdpUrl\`
- 如果刚启动完实例仍然出现这个问题，再检查启动接口是否真的返回了 \`200\`

## Q9：🔒 返回 \`429 rate limit exceeded\`（v1.2.0+）

- Launch Code 解析超过速率限制（10 次/秒）
- 这是安全保护机制，防止暴力破解
- 请在客户端实现请求队列或速率控制
- 建议：批量启动时加入延迟（每次请求间隔 100-200ms）
- 如果是正常业务需求，考虑优化为批量接口调用

## Q10：🔒 错误消息不再显示具体的 code 或 profileId（v1.2.0+）

- 这是安全增强，防止信息泄漏和枚举攻击
- 错误消息现在是通用格式，例如：
  - \`"launch code not found"\` 而非 \`"launch code not found: ABC123"\`
  - \`"profile not found"\` 而非 \`"profile not found: profile_xyz"\`
- 建议使用 HTTP 状态码判断错误类型，而非解析错误消息文本
- 详细调试信息可查看应用日志（\`data/logs/\`）

## Q11：🔒 为什么错误响应包含额外的安全响应头？（v1.2.0+）

- 所有 API 响应现在包含安全响应头（X-Frame-Options、CSP 等）
- 这是 Web 安全最佳实践，防止 XSS、点击劫持等攻击
- 不影响 API 功能，客户端无需特殊处理
- 浏览器会自动应用这些安全策略

## Q12：🔒 Launch Code 长度变化（v1.2.0+）

- Launch Code 长度从 6 位增加到 12 位
- 旧的 6 位 Code 在升级后将失效
- 需要重新生成 12 位 Launch Code
- 这是安全增强，大幅提高暴力破解难度
`

// ============================================================================
// 文档树结构
// ============================================================================

interface DocNode {
  id: string
  label: string
  children?: DocNode[]
  content?: string
}

const DOC_TREE: DocNode[] = [
  {
    id: 'overview',
    label: '文档说明',
    content: DOC_OVERVIEW,
  },
  {
    id: 'quickstart',
    label: '快速接入',
    content: DOC_QUICKSTART,
  },
  {
    id: 'selector',
    label: '选择器规则',
    content: DOC_SELECTOR,
  },
  {
    id: 'api-index',
    label: '接口总览',
    content: DOC_API_INDEX,
  },
  {
    id: 'api',
    label: '核心接口',
    children: [
      { id: 'api-health', label: '健康检查', content: DOC_API_HEALTH },
      { id: 'api-profiles', label: '实例管理', content: DOC_API_PROFILES },
      { id: 'api-launch-get', label: '按 Code 启动', content: DOC_API_LAUNCH_GET },
      { id: 'api-launch-post', label: '参数化启动', content: DOC_API_LAUNCH_POST },
      { id: 'api-cdp', label: 'CDP 统一入口', content: DOC_API_CDP },
      { id: 'api-logs', label: '调用记录', content: DOC_API_LOGS },
    ],
  },
  {
    id: 'scenarios',
    label: '场景示例',
    content: DOC_SCENARIOS,
  },
  {
    id: 'errors',
    label: '错误与重试',
    content: DOC_ERRORS,
  },
  {
    id: 'examples',
    label: '多语言示例',
    content: DOC_EXAMPLES,
  },
  {
    id: 'practices',
    label: '最佳实践',
    content: DOC_PRACTICES,
  },
  {
    id: 'troubleshoot',
    label: '常见问题',
    content: DOC_TROUBLESHOOT,
  },
]

const DEFAULT_LAUNCH_BASE_URL = 'http://127.0.0.1:19876'
const DEFAULT_API_AUTH: LaunchServerInfo['apiAuth'] = {
  requested: false,
  configured: false,
  enabled: false,
  header: 'X-Ant-Api-Key',
}

function renderDocWithLaunchContext(raw: string, baseUrl: string, authHeader: string): string {
  if (!raw) return raw
  const safeBase = baseUrl.trim() || DEFAULT_LAUNCH_BASE_URL
  const safeAuthHeader = authHeader.trim() || DEFAULT_API_AUTH.header
  const hostPort = safeBase.replace(/^https?:\/\//, '')
  return raw
    .split('http://127.0.0.1:19876').join(safeBase)
    .split('127.0.0.1:19876').join(hostPort)
    .split('X-Ant-Api-Key').join(safeAuthHeader)
}

// ============================================================================
// 组件
// ============================================================================

function DocTreeItem({
  node,
  depth,
  activeId,
  onSelect,
  expandedIds,
  onToggle,
}: {
  node: DocNode
  depth: number
  activeId: string
  onSelect: (id: string, content: string) => void
  expandedIds: Set<string>
  onToggle: (id: string) => void
}) {
  const hasChildren = !!node.children?.length
  const isExpanded = expandedIds.has(node.id)
  const isActive = activeId === node.id

  const handleClick = () => {
    if (hasChildren) {
      onToggle(node.id)
    } else if (node.content) {
      onSelect(node.id, node.content)
    }
  }

  return (
    <div>
      <button
        onClick={handleClick}
        className={[
          'w-full flex items-center gap-1.5 px-3 py-1.5 rounded-md text-sm transition-colors text-left',
          isActive && !hasChildren
            ? 'bg-[var(--color-accent)] text-[var(--color-text-inverse)]'
            : 'text-[var(--color-text-secondary)] hover:bg-[var(--color-accent-muted)] hover:text-[var(--color-text-primary)]',
        ].join(' ')}
        style={{ paddingLeft: `${12 + depth * 14}px` }}
      >
        {hasChildren ? (
          <ChevronRight
            className={`w-3.5 h-3.5 shrink-0 transition-transform ${isExpanded ? 'rotate-90' : ''}`}
          />
        ) : (
          <FileText className="w-3.5 h-3.5 shrink-0 opacity-60" />
        )}
        <span className="truncate">{node.label}</span>
      </button>

      {hasChildren && isExpanded && (
        <div>
          {node.children!.map(child => (
            <DocTreeItem
              key={child.id}
              node={child}
              depth={depth + 1}
              activeId={activeId}
              onSelect={onSelect}
              expandedIds={expandedIds}
              onToggle={onToggle}
            />
          ))}
        </div>
      )}
    </div>
  )
}

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false)
  return (
    <button
      onClick={() => {
        navigator.clipboard.writeText(text).then(() => {
          setCopied(true)
          toast.success('已复制')
          setTimeout(() => setCopied(false), 2000)
        })
      }}
      className="flex items-center gap-1 text-xs text-[var(--color-text-muted)] hover:text-[var(--color-accent)] transition-colors"
    >
      {copied ? <CheckCircle className="w-3.5 h-3.5 text-green-500" /> : <Copy className="w-3.5 h-3.5" />}
      {copied ? '已复制' : '复制'}
    </button>
  )
}

function MarkdownContent({ content }: { content: string }) {
  return (
    <ReactMarkdown
      remarkPlugins={[remarkGfm]}
      components={{
        h1: ({ children }) => (
          <h1 className="text-2xl font-bold text-[var(--color-text-primary)] mb-6 pb-3 border-b border-[var(--color-border-default)]">
            {children}
          </h1>
        ),
        h2: ({ children }) => (
          <h2 className="text-lg font-semibold text-[var(--color-text-primary)] mt-8 mb-3 flex items-center gap-2">
            <span className="w-1 h-5 bg-[var(--color-accent)] rounded-full inline-block shrink-0" />
            {children}
          </h2>
        ),
        h3: ({ children }) => (
          <h3 className="text-base font-semibold text-[var(--color-text-primary)] mt-6 mb-2">
            {children}
          </h3>
        ),
        p: ({ children }) => (
          <p className="text-sm text-[var(--color-text-secondary)] leading-relaxed mb-3">
            {children}
          </p>
        ),
        ul: ({ children }) => (
          <ul className="space-y-1 mb-4 pl-5 list-disc marker:text-[var(--color-accent)]">{children}</ul>
        ),
        ol: ({ children }) => (
          <ol className="space-y-1 mb-4 pl-5 list-decimal marker:text-[var(--color-accent)]">{children}</ol>
        ),
        li: ({ children }) => (
          <li className="text-sm text-[var(--color-text-secondary)] leading-relaxed">
            {children}
          </li>
        ),
        code: ({ children, className }) => {
          const isBlock = className?.includes('language-')
          if (isBlock) return null
          return (
            <code className="text-xs font-mono bg-[var(--color-bg-secondary)] text-[var(--color-accent)] px-1.5 py-0.5 rounded border border-[var(--color-border-muted)]">
              {children}
            </code>
          )
        },
        pre: ({ children }) => {
          const codeEl = (children as any)?.props
          const lang = codeEl?.className?.replace('language-', '') || ''
          const codeText = codeEl?.children || ''
          return (
            <div className="my-4 rounded-lg overflow-hidden border border-[var(--color-border-default)]">
              <div className="flex items-center justify-between px-4 py-2 bg-[var(--color-bg-surface)] border-b border-[var(--color-border-muted)]">
                <span className="text-xs font-mono text-[var(--color-text-muted)]">{lang || 'code'}</span>
                <CopyButton text={String(codeText).replace(/\n$/, '')} />
              </div>
              <pre className="p-4 bg-[var(--color-bg-secondary)] overflow-x-auto text-sm font-mono text-[var(--color-text-primary)] leading-relaxed">
                {children}
              </pre>
            </div>
          )
        },
        table: ({ children }) => (
          <div className="my-4 overflow-x-auto rounded-lg border border-[var(--color-border-default)]">
            <table className="w-full text-sm">{children}</table>
          </div>
        ),
        thead: ({ children }) => (
          <thead className="bg-[var(--color-bg-surface)] border-b border-[var(--color-border-default)]">
            {children}
          </thead>
        ),
        th: ({ children }) => (
          <th className="px-4 py-2.5 text-left text-xs font-semibold text-[var(--color-text-muted)] uppercase tracking-wide">
            {children}
          </th>
        ),
        td: ({ children }) => (
          <td className="px-4 py-2.5 text-[var(--color-text-secondary)] border-t border-[var(--color-border-muted)]">
            {children}
          </td>
        ),
        blockquote: ({ children }) => (
          <blockquote className="my-3 pl-4 border-l-2 border-[var(--color-accent)] text-[var(--color-text-muted)] italic">
            {children}
          </blockquote>
        ),
        strong: ({ children }) => (
          <strong className="font-semibold text-[var(--color-text-primary)]">{children}</strong>
        ),
        hr: () => <hr className="my-6 border-[var(--color-border-default)]" />,
        a: ({ href, children }) => (
          <a
            href={href}
            onClick={(e) => {
              e.preventDefault()
              if (href) {
                BrowserOpenURL(href)
              }
            }}
            className="text-[var(--color-accent)] hover:underline cursor-pointer"
            title={href}
          >
            {children}
          </a>
        ),
      }}
    >
      {content}
    </ReactMarkdown>
  )
}

// ============================================================================
// 主页面
// ============================================================================

function findFirstLeaf(nodes: DocNode[]): DocNode | null {
  for (const n of nodes) {
    if (!n.children) return n
    const found = findFirstLeaf(n.children)
    if (found) return found
  }
  return null
}

function collectParentIds(nodes: DocNode[], targetId: string, path: string[] = []): string[] {
  for (const n of nodes) {
    if (n.id === targetId) return path
    if (n.children) {
      const found = collectParentIds(n.children, targetId, [...path, n.id])
      if (found.length) return found
    }
  }
  return []
}

export function LaunchApiDocsPage() {
  const firstLeaf = findFirstLeaf(DOC_TREE)!
  const [activeId, setActiveId] = useState(firstLeaf.id)
  const [activeContent, setActiveContent] = useState(firstLeaf.content || '')
  const [launchBaseUrl, setLaunchBaseUrl] = useState(DEFAULT_LAUNCH_BASE_URL)
  const [launchServerReady, setLaunchServerReady] = useState(false)
  const [apiAuth, setApiAuth] = useState<LaunchServerInfo['apiAuth']>(DEFAULT_API_AUTH)

  const [expandedIds, setExpandedIds] = useState<Set<string>>(() => {
    const parents = collectParentIds(DOC_TREE, firstLeaf.id)
    return new Set(parents)
  })

  useEffect(() => {
    let disposed = false

    void fetchLaunchServerInfo()
      .then((info) => {
        if (disposed) return
        if (info.baseUrl) {
          setLaunchBaseUrl(info.baseUrl)
        }
        setLaunchServerReady(info.ready)
        setApiAuth(info.apiAuth)
      })
      .catch(() => {})

    return () => {
      disposed = true
    }
  }, [])

  const handleSelect = (id: string, content: string) => {
    setActiveId(id)
    setActiveContent(content)
  }

  const handleToggle = (id: string) => {
    setExpandedIds(prev => {
      const next = new Set(prev)
      next.has(id) ? next.delete(id) : next.add(id)
      return next
    })
  }

  const renderedContent = renderDocWithLaunchContext(activeContent, launchBaseUrl, apiAuth.header)

  return (
    <div className="flex h-full -m-5 overflow-hidden">
      <aside className="w-52 shrink-0 border-r border-[var(--color-border-default)] bg-[var(--color-bg-surface)] flex flex-col overflow-hidden">
        <div className="px-4 py-3 border-b border-[var(--color-border-muted)]">
          <p className="text-xs font-semibold text-[var(--color-text-muted)] uppercase tracking-widest">文档</p>
        </div>
        <nav className="flex-1 overflow-y-auto py-2 px-2 space-y-0.5">
          {DOC_TREE.map(node => (
            <DocTreeItem
              key={node.id}
              node={node}
              depth={0}
              activeId={activeId}
              onSelect={handleSelect}
              expandedIds={expandedIds}
              onToggle={handleToggle}
            />
          ))}
        </nav>
      </aside>

      <main className="flex-1 overflow-y-auto">
        <div className="max-w-3xl mx-auto px-10 py-8">
          <div className="mb-4 px-3 py-2 text-xs rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] text-[var(--color-text-secondary)]">
            当前 Launch 地址：<code>{launchBaseUrl}</code>
            {!launchServerReady ? '（服务启动后会自动刷新）' : ''}
          </div>
          <div className="mb-4 px-3 py-2 text-xs rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] text-[var(--color-text-secondary)]">
            {apiAuth.enabled
              ? <>当前 API 认证已启用，请为所有 <code>/api/*</code> 请求追加 <code>{apiAuth.header}: &lt;your-api-key&gt;</code>。</>
              : apiAuth.requested && !apiAuth.configured
                ? <>当前配置要求启用 API 认证，但 <code>api_key</code> 为空，认证尚未生效。</>
                : <>当前 API 认证未启用；如需开启，可在 <code>config.yaml</code> 的 <code>launch_server.auth</code> 下配置。</>}
          </div>
          <MarkdownContent content={renderedContent} />
        </div>
      </main>
    </div>
  )
}
