import type { StructuredApiEndpointDoc } from './structuredApiDocs.types'

export const RUNTIME_API_ENDPOINT_DOCS: StructuredApiEndpointDoc[] = [
  {
    id: 'api-runtime-active-detail',
    parentId: 'api-runtime',
    label: '当前活动实例',
    method: 'GET',
    path: '/api/runtime/active',
    purpose: '查看当前统一 CDP 入口挂着哪个实例。',
    description: '当外部系统只知道 LaunchServer 端口、不知道当前 active target 时，先查这个接口最直接。',
    fields: [],
    requestExample: {
      language: 'bash',
      code: ({ launchBaseUrl, authHeader }) => `curl ${launchBaseUrl}/api/runtime/active \\
  -H "${authHeader}: <your-api-key>"`,
    },
    responseExample: {
      language: 'json',
      code: () => `{
  "ok": true,
  "active": true,
  "profileId": "550e8400-e29b-41d4-a716-446655440000",
  "profileName": "buyer-001",
  "launchCode": "BUYER_001",
  "running": true,
  "debugReady": true,
  "cdpUrl": "http://127.0.0.1:19876",
  "directDebugUrl": "http://127.0.0.1:9333"
}`,
    },
    responseCodes: [
      { code: '200', description: '返回当前 active target 状态。' },
    ],
    notes: [
      'active=false 表示当前没有活动实例。',
    ],
  },
  {
    id: 'api-runtime-session-detail',
    parentId: 'api-runtime',
    label: '准备可接管会话',
    method: 'POST',
    path: '/api/runtime/session',
    purpose: '准备一个可 attach 的运行时会话。',
    description: '按 selector 命中实例，必要时自动启动，并在给定超时时间内等待 debugReady=true。',
    fields: [
      { name: 'selector', type: 'object', required: false, location: 'Body', description: '目标实例选择条件；新接入推荐使用。' },
      { name: 'code', type: 'string', required: false, location: 'Body', description: '兼容写法：等价于 selector.code。' },
      { name: 'profileId', type: 'string', required: false, location: 'Body', description: '兼容写法：等价于 selector.profileId。' },
      { name: 'profileName', type: 'string', required: false, location: 'Body', description: '兼容写法：等价于 selector.profileName。' },
      { name: 'keyword / keywords', type: 'string / string[]', required: false, location: 'Body', description: '兼容写法：等价于 selector.keyword / selector.keywords。' },
      { name: 'tag / tags', type: 'string / string[]', required: false, location: 'Body', description: '兼容写法：等价于 selector.tag / selector.tags。' },
      { name: 'groupId', type: 'string', required: false, location: 'Body', description: '兼容写法：等价于 selector.groupId。' },
      { name: 'matchMode', type: 'unique | first', required: false, location: 'Body', description: '兼容写法：等价于 selector.matchMode。' },
      { name: 'timeoutMs', type: 'integer', required: false, location: 'Body', description: '等待 debugReady 的超时时间。' },
      { name: 'startUrls', type: 'string[]', required: false, location: 'Body', description: '本次启动时额外打开的网址。' },
      { name: 'skipDefaultStartUrls', type: 'boolean', required: false, location: 'Body', description: '是否跳过实例默认启动 URL。' },
      { name: 'launchArgs', type: 'string[]', required: false, location: 'Body', description: '本次启动时临时附加的启动参数。' },
    ],
    requestExample: {
      language: 'bash',
      code: ({ launchBaseUrl, authHeader }) => `curl -X POST ${launchBaseUrl}/api/runtime/session \\
  -H "Content-Type: application/json" \\
  -H "${authHeader}: <your-api-key>" \\
  -d '{
    "selector": { "code": "BUYER_001" },
    "timeoutMs": 45000,
    "skipDefaultStartUrls": true
  }'`,
    },
    responseExample: {
      language: 'json',
      code: () => `{
  "ok": true,
  "ready": true,
  "waitTimedOut": false,
  "retryable": false,
  "profileId": "550e8400-e29b-41d4-a716-446655440000",
  "launchCode": "BUYER_001",
  "running": true,
  "debugReady": true,
  "active": true,
  "cdpUrl": "http://127.0.0.1:19876",
  "directDebugUrl": "http://127.0.0.1:9333",
  "timeoutMs": 45000
}`,
    },
    responseCodes: [
      { code: '200', description: '实例已 ready，可直接 attach。' },
      { code: '202', description: '实例已处理但暂未 ready，可稍后重试。' },
      { code: '400', description: 'selector 缺失或 matchMode 非法。' },
      { code: '404', description: '目标实例不存在。' },
    ],
    notes: [
      'selector 为空且没有任何兼容顶层选择字段时返回 400。',
      '200 表示 ready，可直接接管。',
      '202 表示未 ready，需要重试。',
    ],
  },
  {
    id: 'api-runtime-status-detail',
    parentId: 'api-runtime',
    label: '按 selector 查状态',
    method: 'POST',
    path: '/api/runtime/status',
    purpose: '按 selector 查询实例当前运行态。',
    description: '不启动新实例，不等待 ready，只看当前 selector 命中的实例状态。',
    fields: [
      { name: 'selector', type: 'object', required: false, location: 'Body', description: '目标实例选择条件；新接入推荐使用。' },
      { name: 'code / profileId / profileName', type: 'string', required: false, location: 'Body', description: '兼容顶层选择字段。' },
      { name: 'keyword / keywords', type: 'string / string[]', required: false, location: 'Body', description: '兼容顶层选择字段。' },
      { name: 'tag / tags / groupId', type: 'string / string[]', required: false, location: 'Body', description: '兼容顶层选择字段。' },
      { name: 'matchMode', type: 'unique | first', required: false, location: 'Body', description: '运行态控制不支持 all。' },
    ],
    requestExample: {
      language: 'bash',
      code: ({ launchBaseUrl, authHeader }) => `curl -X POST ${launchBaseUrl}/api/runtime/status \\
  -H "Content-Type: application/json" \\
  -H "${authHeader}: <your-api-key>" \\
  -d '{
    "selector": { "keyword": "checkout", "matchMode": "first" }
  }'`,
    },
    responseExample: {
      language: 'json',
      code: () => `{
  "ok": true,
  "profileId": "550e8400-e29b-41d4-a716-446655440000",
  "launchCode": "BUYER_001",
  "running": true,
  "debugReady": false,
  "active": false,
  "cdpUrl": ""
}`,
    },
    responseCodes: [
      { code: '200', description: '返回运行态。' },
      { code: '400', description: 'selector 缺失或 matchMode 非法。' },
      { code: '404', description: '目标实例不存在。' },
    ],
    notes: [
      '不会启动实例。',
      'selector 为空且没有任何兼容顶层选择字段时返回 400。',
    ],
  },
  {
    id: 'api-runtime-stop-detail',
    parentId: 'api-runtime',
    label: '按 selector 停止',
    method: 'POST',
    path: '/api/runtime/stop',
    purpose: '按 selector 停止实例。',
    description: '和 runtime/status 一样使用 selector，但动作改为停止实例，适合编排侧做统一回收。',
    fields: [
      { name: 'selector', type: 'object', required: false, location: 'Body', description: '目标实例选择条件；新接入推荐使用。' },
      { name: 'code / profileId / profileName', type: 'string', required: false, location: 'Body', description: '兼容顶层选择字段。' },
      { name: 'keyword / keywords', type: 'string / string[]', required: false, location: 'Body', description: '兼容顶层选择字段。' },
      { name: 'tag / tags / groupId', type: 'string / string[]', required: false, location: 'Body', description: '兼容顶层选择字段。' },
      { name: 'matchMode', type: 'unique | first', required: false, location: 'Body', description: '运行态控制不支持 all。' },
    ],
    requestExample: {
      language: 'bash',
      code: ({ launchBaseUrl, authHeader }) => `curl -X POST ${launchBaseUrl}/api/runtime/stop \\
  -H "Content-Type: application/json" \\
  -H "${authHeader}: <your-api-key>" \\
  -d '{
    "selector": { "code": "BUYER_001" }
  }'`,
    },
    responseExample: {
      language: 'json',
      code: () => `{
  "ok": true,
  "stopped": true,
  "profileId": "550e8400-e29b-41d4-a716-446655440000",
  "launchCode": "BUYER_001",
  "running": false,
  "debugReady": false,
  "active": false
}`,
    },
    responseCodes: [
      { code: '200', description: '停止成功。' },
      { code: '400', description: 'selector 缺失或 matchMode 非法。' },
      { code: '404', description: '目标实例不存在。' },
    ],
    notes: [
      'selector 为空且没有任何兼容顶层选择字段时返回 400。',
      '不支持 matchMode=all。',
    ],
  },
  {
    id: 'api-cdp-version-detail',
    parentId: 'api-runtime',
    label: 'CDP 版本信息',
    method: 'GET',
    path: '/json/version',
    purpose: '读取统一 CDP 入口的版本信息。',
    description: '这个接口透传当前 active target 的 CDP 版本信息，适合 attach 前探测调试入口是否可用。',
    fields: [],
    requestExample: {
      language: 'bash',
      code: ({ launchBaseUrl, authHeader }) => `curl ${launchBaseUrl}/json/version \\
  -H "${authHeader}: <your-api-key>"`,
    },
    responseExample: {
      language: 'json',
      code: () => `{
  "Browser": "Chrome/<version>",
  "Protocol-Version": "1.3",
  "User-Agent": "Mozilla/5.0",
  "webSocketDebuggerUrl": "ws://127.0.0.1:19876/devtools/browser/active"
}`,
    },
    responseCodes: [
      { code: '200', description: '返回当前 active target 的版本信息。' },
      { code: '503', description: '当前没有可透传的 active target。' },
    ],
    notes: [
      '无 active target 时返回 503。',
    ],
  },
  {
    id: 'api-cdp-list-detail',
    parentId: 'api-runtime',
    label: 'CDP Target 列表',
    method: 'GET',
    path: '/json/list',
    purpose: '读取统一 CDP 入口当前暴露的 target 列表。',
    description: '给 Playwright、Puppeteer 或诊断工具查看当前活动 target 时使用。',
    fields: [],
    requestExample: {
      language: 'bash',
      code: ({ launchBaseUrl, authHeader }) => `curl ${launchBaseUrl}/json/list \\
  -H "${authHeader}: <your-api-key>"`,
    },
    responseExample: {
      language: 'json',
      code: () => `[
  {
    "id": "page-1",
    "type": "page",
    "title": "Checkout",
    "url": "https://example.com/checkout",
    "webSocketDebuggerUrl": "ws://127.0.0.1:19876/devtools/page/page-1"
  }
]`,
    },
    responseCodes: [
      { code: '200', description: '返回当前活动实例的 target 列表。' },
      { code: '503', description: '当前没有可透传的 active target。' },
    ],
    notes: [
      '无 active target 时返回 503。',
    ],
  },
  {
    id: 'api-cdp-ws-detail',
    parentId: 'api-runtime',
    label: 'CDP WebSocket',
    method: 'WS',
    path: '/devtools/...',
    purpose: '通过统一 WebSocket 入口接管当前活动实例。',
    description: '实际 attach 时使用的就是这个 WebSocket 入口。外部工具通常先拿 /json/version 或 /json/list，再连对应 websocketDebuggerUrl。',
    fields: [],
    requestExample: {
      language: 'javascript',
      code: ({ launchBaseUrl }) => {
        const wsBase = launchBaseUrl.replace(/^http/i, 'ws')
        return `const browser = await chromium.connectOverCDP("${wsBase}");
// 或按 /json/list 返回的 webSocketDebuggerUrl 连接具体 page target`
      },
    },
    responseExample: {
      language: 'text',
      code: () => `WebSocket 握手成功后进入标准 Chrome DevTools Protocol 消息流。`,
    },
    responseCodes: [
      { code: '101', description: 'WebSocket 升级成功。' },
      { code: '503', description: '当前没有可透传的 active target。' },
    ],
    notes: [
      '先调 runtime/session，再连 WS。',
    ],
  },
]
