export const DOC_API_PROFILES_LAUNCH = `# 实例与启动

## 实例接口

| 方法 | 路径 | 用途 |
|------|------|------|
| \`GET\` | \`/api/profiles\` | 列实例 |
| \`POST\` | \`/api/profiles\` | 创建实例 |
| \`GET\` | \`/api/profiles/{profileId}\` | 查单个实例 |
| \`PUT\` | \`/api/profiles/{profileId}\` | 更新实例 |
| \`DELETE\` | \`/api/profiles/{profileId}\` | 删除实例 |
| \`GET\` | \`/api/profiles/{profileId}/status\` | 查实例运行态 |
| \`POST\` | \`/api/profiles/{profileId}/stop\` | 停止实例 |

## 创建实例

\`\`\`bash
curl -X POST http://127.0.0.1:19876/api/profiles \\
  -H "Content-Type: application/json" \\
  -d '{
    "profile": {
      "profileName": "buyer-001",
      "proxyId": "proxy-us",
      "keywords": ["buyer-001"],
      "tags": ["电商"]
    },
    "launchCode": "BUYER_001"
  }'
\`\`\`

## 创建并立即启动

\`\`\`bash
curl -X POST http://127.0.0.1:19876/api/profiles \\
  -H "Content-Type: application/json" \\
  -d '{
    "profile": {
      "profileName": "buyer-002",
      "keywords": ["buyer-002"]
    },
    "autoLaunch": true,
    "start": {
      "skipDefaultStartUrls": true
    }
  }'
\`\`\`

## 创建实例（自定义代理配置）

\`\`\`bash
curl -X POST http://127.0.0.1:19876/api/profiles \\
  -H "Content-Type: application/json" \\
  -d '{
    "profile": {
      "profileName": "buyer-003",
      "proxyConfig": "http://127.0.0.1:18080",
      "keywords": ["buyer-003"]
    }
  }'
\`\`\`

## 查询 / 更新 / 删除

\`\`\`bash
curl http://127.0.0.1:19876/api/profiles
curl http://127.0.0.1:19876/api/profiles/550e8400-e29b-41d4-a716-446655440000
curl -X PUT http://127.0.0.1:19876/api/profiles/550e8400-e29b-41d4-a716-446655440000 -H "Content-Type: application/json" -d '{ ... }'
curl -X DELETE http://127.0.0.1:19876/api/profiles/550e8400-e29b-41d4-a716-446655440000
\`\`\`

## 启动接口

| 方法 | 路径 | 用途 |
|------|------|------|
| \`GET\` | \`/api/launch/{code}\` | 按唯一 Code 启动 |
| \`POST\` | \`/api/launch\` | 按 code / selector 参数化启动 |

### 按 Code 启动

\`\`\`bash
curl http://127.0.0.1:19876/api/launch/A3F9K2
\`\`\`

### 按 selector 启动

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

## 启动成功响应

\`\`\`json
{
  "ok": true,
  "profileId": "550e8400-e29b-41d4-a716-446655440000",
  "launchCode": "BUYER_001",
  "debugReady": true,
  "cdpUrl": "http://127.0.0.1:19876"
}
\`\`\`

## 单实例状态 / 停止

| 方法 | 路径 | 示例用途 |
|------|------|----------|
| \`GET\` | \`/api/profiles/{profileId}/status\` | 查实例是否运行、是否 ready |
| \`POST\` | \`/api/profiles/{profileId}/stop\` | 任务完成后精确停止 |

## 记住这几个规则

\`\`\`text
launchCode 冲突 -> 409
PUT 是整份更新
运行中的实例不能直接 DELETE
matchMode=all 只在 POST /api/launch 可用
proxyId 和 proxyConfig 同时传 -> 优先 proxyId
proxyId 无效 + proxyConfig 非空 -> 使用 proxyConfig
proxyId 无效 + proxyConfig 为空 -> 400
\`\`\`
`

export const DOC_API_RUNTIME = `# 运行态与接管

## 接口

| 方法 | 路径 | 用途 |
|------|------|------|
| \`GET\` | \`/api/runtime/active\` | 查当前活动实例 |
| \`POST\` | \`/api/runtime/session\` | 准备可接管会话 |
| \`POST\` | \`/api/runtime/status\` | 按 selector 查运行态 |
| \`POST\` | \`/api/runtime/stop\` | 按 selector 停止实例 |
| \`GET\` | \`/json/version\` | 统一 CDP 入口 |
| \`GET\` | \`/json/list\` | 统一 CDP 入口 |
| \`WS\` | \`/devtools/...\` | CDP WebSocket 接管 |

## 查询当前活动实例

\`\`\`bash
curl http://127.0.0.1:19876/api/runtime/active
\`\`\`

\`\`\`json
{
  "ok": true,
  "active": true,
  "profileId": "550e8400-e29b-41d4-a716-446655440000",
  "launchCode": "BUYER_001",
  "debugReady": true,
  "cdpUrl": "http://127.0.0.1:19876"
}
\`\`\`

## 准备可接管会话

\`\`\`bash
curl -X POST http://127.0.0.1:19876/api/runtime/session \\
  -H "Content-Type: application/json" \\
  -d '{
    "selector": {
      "code": "BUYER_001"
    },
    "timeoutMs": 45000,
    "skipDefaultStartUrls": true
  }'
\`\`\`

| 返回 | 含义 |
|------|------|
| \`200 + ready=true\` | 可以直接 attach |
| \`202 + ready=false\` | 已处理，但还没 ready |

## 按 selector 查状态

\`\`\`bash
curl -X POST http://127.0.0.1:19876/api/runtime/status \\
  -H "Content-Type: application/json" \\
  -d '{
    "selector": {
      "keyword": "shop",
      "matchMode": "first"
    }
  }'
\`\`\`

## 按 selector 停止

\`\`\`bash
curl -X POST http://127.0.0.1:19876/api/runtime/stop \\
  -H "Content-Type: application/json" \\
  -d '{
    "code": "BUYER_001"
  }'
\`\`\`

## 接管示例

\`\`\`javascript
import { chromium } from "playwright";

const res = await fetch("http://127.0.0.1:19876/api/runtime/session", {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({
    selector: { code: "BUYER_001" },
    skipDefaultStartUrls: true
  })
});

const data = await res.json();
const browser = await chromium.connectOverCDP(data.cdpUrl);
\`\`\`

## 记住这几个规则

\`\`\`text
runtime/status 和 runtime/stop 不支持 matchMode=all
attach 前先看 active / debugReady / cdpUrl
统一入口只指向一个活动实例
\`\`\`
`

export const DOC_API_PROXY_GATEWAY = `# 代理网关与热切换

## 接口

| 方法 | 路径 | 用途 |
|------|------|------|
| \`POST\` | \`/api/proxy-gateway/switch\` | 热切换代理出口 |
| \`GET / POST\` | \`/api/proxy-gateway/status\` | 查询网关和连接状态 |
| \`GET\` | \`/api/proxy-gateway/routing\` | 读取分流配置 |
| \`PUT\` | \`/api/proxy-gateway/routing\` | 保存并热应用分流配置 |

每个运行实例都会使用一个独立的本地 SOCKS5 网关。代理切换后新连接立即使用新路由，旧连接默认自然排空；传 \`force=true\` 才会立即断开旧连接。代理拨号失败时不会回退直连。

## 热切换

\`\`\`bash
curl -X POST http://127.0.0.1:19876/api/proxy-gateway/switch \\
  -H "Content-Type: application/json" \\
  -d '{
    "selector": { "code": "BUYER_001" },
    "proxyId": "proxy-us-new",
    "force": false
  }'
\`\`\`

## 规则模式

\`mode\` 可选 \`proxy / rule / direct\`。规则按数组顺序匹配，\`action\` 可选 \`proxy / direct / block\`，\`matchType\` 可选 \`domain / domain_suffix / domain_keyword / ip_cidr\`。

\`\`\`bash
curl -X PUT http://127.0.0.1:19876/api/proxy-gateway/routing \\
  -H "Content-Type: application/json" \\
  -d '{
    "selector": { "profileId": "550e8400-e29b-41d4-a716-446655440000" },
    "routing": {
      "mode": "rule",
      "rules": [
        { "id": "cn", "enabled": true, "matchType": "domain_suffix", "pattern": ".cn", "action": "direct" },
        { "id": "track", "enabled": true, "matchType": "domain_keyword", "pattern": "tracker", "action": "block" }
      ]
    },
    "force": false
  }'
\`\`\`
`

export const DOC_API_AUTOMATION = `# 脚本自动化

## 接口

| 方法 | 路径 | 用途 |
|------|------|------|
| \`GET\` | \`/api/automation/scripts\` | 查脚本列表 |
| \`GET\` | \`/api/automation/scripts/{scriptId}\` | 查单个脚本详情 |
| \`POST\` | \`/api/automation/scripts/run\` | 执行脚本 |
| \`GET\` | \`/api/automation/scripts/runs\` | 查运行记录 |

## 列脚本

\`\`\`bash
curl http://127.0.0.1:19876/api/automation/scripts
\`\`\`

\`\`\`json
{
  "ok": true,
  "items": [
    {
      "id": "news-query-txt",
      "name": "查询新闻并写 TXT",
      "type": "playwright-cdp",
      "status": "ready"
    }
  ]
}
\`\`\`

## 执行脚本

\`\`\`bash
curl -X POST http://127.0.0.1:19876/api/automation/scripts/run \\
  -H "Content-Type: application/json" \\
  -d '{
    "scriptId": "news-query-txt",
    "selector": { "code": "BUYER_001" },
    "params": { "keyword": "OpenAI", "limit": 10 }
  }'
\`\`\`

\`\`\`json
{
  "ok": true,
  "run": {
    "id": "run-1",
    "status": "success",
    "summary": "已抓取 10 条新闻并写入 TXT"
  }
}
\`\`\`

如果脚本已经在界面里配置成 \`使用已有实例\` 或 \`按模板新建实例\`，也可以只传：

\`\`\`bash
curl -X POST http://127.0.0.1:19876/api/automation/scripts/run \\
  -H "Content-Type: application/json" \\
  -d '{
    "scriptId": "news-query-txt"
  }'
\`\`\`

## 查运行记录

\`\`\`bash
curl http://127.0.0.1:19876/api/automation/scripts/runs?limit=20
\`\`\`

## 记住这几个规则

\`\`\`text
scriptId 必填
推荐优先使用 selector.code，而不是 profileId
selector / params 必须是 JSON object
不传 selector / params 时，默认沿用脚本内配置
\`\`\`
`
