import type { StructuredApiEndpointDoc } from './structuredApiDocs.types'

export const PROXY_GATEWAY_API_ENDPOINT_DOCS: StructuredApiEndpointDoc[] = [
  {
    id: 'api-proxy-gateway-switch-detail',
    parentId: 'api-proxy-gateway',
    label: '热切换代理',
    method: 'POST',
    path: '/api/proxy-gateway/switch',
    purpose: '在实例运行中切换代理出口。',
    description: '按 selector 找到一个实例，更新该实例的代理配置；运行中的实例会立即切换网关当前路由，新建连接使用新代理，旧连接默认继续排空。',
    fields: [
      { name: 'selector', type: 'object', required: true, location: 'Body', description: '目标实例选择条件，推荐使用 profileId 或 code。' },
      { name: 'proxyId', type: 'string', required: false, location: 'Body', description: '代理池节点 ID；与 proxyConfig 同传时优先使用有效的 proxyId。' },
      { name: 'proxyConfig', type: 'string', required: false, location: 'Body', description: '自定义代理 URL，例如 socks5://127.0.0.1:1080；proxyId 无效时作为回退输入。' },
      { name: 'force', type: 'boolean', required: false, location: 'Body', description: '是否立即断开切换前已经建立的旧连接，默认 false。' },
    ],
    requestExample: {
      language: 'bash',
      code: ({ launchBaseUrl, authHeader }) => `curl -X POST ${launchBaseUrl}/api/proxy-gateway/switch \\
  -H "Content-Type: application/json" \\
  -H "${authHeader}: <your-api-key>" \\
  -d '{
    "selector": { "code": "BUYER_001" },
    "proxyId": "proxy-us-new",
    "force": false
  }'`,
    },
    responseExample: {
      language: 'json',
      code: () => `{
  "ok": true,
  "profileId": "550e8400-e29b-41d4-a716-446655440000",
  "appliedLive": true,
  "gateway": {
    "profileId": "550e8400-e29b-41d4-a716-446655440000",
    "proxyUrl": "socks5://127.0.0.1:43127",
    "currentRouteId": "proxy-us-new",
    "activeConnections": 1,
    "drainingConnections": 2
  }
}`,
    },
    responseCodes: [
      { code: '200', description: '代理已保存；运行中的实例已应用到网关。' },
      { code: '400', description: 'selector、请求体或代理配置非法，包括 proxyId 不存在且未提供有效 proxyConfig。' },
      { code: '404', description: 'selector 未命中实例。' },
      { code: '502', description: '代理网关或代理出口不可用，原路由保持不变。' },
      { code: '503', description: '代理网关控制能力未就绪。' },
    ],
    notes: [
      '实例未运行时 appliedLive=false，仅保存下次启动使用的代理。',
      '切换不会迁移已经建立的 TCP 连接；force=true 才会立即关闭旧连接。',
      '代理拨号失败时网关返回失败，不会回退到直连。',
      '每个运行实例拥有独立的 127.0.0.1 SOCKS5 网关端口，端口由程序动态分配。',
    ],
  },
  {
    id: 'api-proxy-gateway-status-detail',
    parentId: 'api-proxy-gateway',
    label: '网关状态',
    method: 'GET',
    path: '/api/proxy-gateway/status?profileId={profileId}',
    purpose: '查询实例代理网关的实时状态。',
    description: '按 profileId 读取实例网关地址、当前路由、活动连接、排空连接和浏览器调试信息。',
    fields: [
      { name: 'profileId', type: 'string', required: true, location: 'Query', description: '实例 ID；GET 查询使用该字段。' },
    ],
    requestExample: {
      language: 'bash',
      code: ({ launchBaseUrl, authHeader }) => `curl "${launchBaseUrl}/api/proxy-gateway/status?profileId=550e8400-e29b-41d4-a716-446655440000" \\
  -H "${authHeader}: <your-api-key>"`,
    },
    responseExample: {
      language: 'json',
      code: () => `{
  "ok": true,
  "profileId": "550e8400-e29b-41d4-a716-446655440000",
  "gateway": {
    "proxyUrl": "socks5://127.0.0.1:43127",
    "currentRouteId": "proxy-us-new",
    "mode": "rule",
    "activeConnections": 2,
    "drainingConnections": 0,
    "browserPid": 12340,
    "browserDebugPort": 9333
  }
}`,
    },
    responseCodes: [
      { code: '200', description: '返回网关状态。' },
      { code: '400', description: '缺少 selector 或 profileId。' },
      { code: '404', description: 'selector 未命中实例或网关实例不存在。' },
      { code: '502', description: '网关进程不可用。' },
      { code: '503', description: '代理网关控制能力未就绪。' },
    ],
    notes: [
      'proxyUrl 是浏览器进程应使用的本地 SOCKS5 地址，不是远端代理地址。',
      'drainingConnections 大于 0 表示仍有旧路由连接，等待其自然结束或用 force=true 切换。',
    ],
  },
  {
    id: 'api-proxy-gateway-status-selector-detail',
    parentId: 'api-proxy-gateway',
    label: '按 selector 查网关',
    method: 'POST',
    path: '/api/proxy-gateway/status',
    purpose: '按 selector 查询实例代理网关状态。',
    description: '适合外部系统只掌握 launchCode、实例名、标签、关键字或分组时查询网关状态；必须唯一命中一个实例。',
    fields: [
      { name: 'selector', type: 'object', required: true, location: 'Body', description: '支持 code、profileId、profileName、keyword/keywords、tag/tags、groupId 和 matchMode。' },
    ],
    requestExample: {
      language: 'bash',
      code: ({ launchBaseUrl, authHeader }) => `curl -X POST ${launchBaseUrl}/api/proxy-gateway/status \\
  -H "Content-Type: application/json" \\
  -H "${authHeader}: <your-api-key>" \\
  -d '{ "selector": { "code": "BUYER_001" } }'`,
    },
    responseExample: {
      language: 'json',
      code: () => `{
  "ok": true,
  "profileId": "550e8400-e29b-41d4-a716-446655440000",
  "gateway": {
    "proxyUrl": "socks5://127.0.0.1:43127",
    "currentRouteId": "proxy-us-new",
    "mode": "proxy",
    "activeConnections": 2,
    "drainingConnections": 0
  }
}`,
    },
    responseCodes: [
      { code: '200', description: '返回网关状态。' },
      { code: '400', description: 'selector 缺失或 matchMode=all。' },
      { code: '404', description: 'selector 未命中实例或实例网关未运行。' },
      { code: '409', description: 'selector 命中多个实例。' },
      { code: '502', description: '网关进程不可用。' },
      { code: '503', description: '代理网关控制能力未就绪。' },
    ],
    notes: [
      '运行态控制不支持 matchMode=all；需要唯一目标时推荐使用 code 或 profileId。',
    ],
  },
  {
    id: 'api-proxy-gateway-routing-get-detail',
    parentId: 'api-proxy-gateway',
    label: '读取分流规则',
    method: 'GET',
    path: '/api/proxy-gateway/routing?profileId={profileId}',
    purpose: '读取实例保存的分流模式和规则。',
    description: '读取实例的持久化分流配置；规则更新后会在运行中的网关立即生效。',
    fields: [
      { name: 'profileId', type: 'string', required: true, location: 'Query', description: '实例 ID。' },
    ],
    requestExample: {
      language: 'bash',
      code: ({ launchBaseUrl, authHeader }) => `curl "${launchBaseUrl}/api/proxy-gateway/routing?profileId=550e8400-e29b-41d4-a716-446655440000" \\
  -H "${authHeader}: <your-api-key>"`,
    },
    responseExample: {
      language: 'json',
      code: () => `{
  "ok": true,
  "profileId": "550e8400-e29b-41d4-a716-446655440000",
  "routing": {
    "mode": "rule",
    "rules": [
      { "id": "local", "name": "本地域名", "enabled": true, "matchType": "domain_suffix", "pattern": ".cn", "action": "direct" },
      { "id": "blocked", "name": "阻断追踪", "enabled": true, "matchType": "domain_keyword", "pattern": "tracker", "action": "block" }
    ]
  }
}`,
    },
    responseCodes: [
      { code: '200', description: '返回规范化后的分流配置。' },
      { code: '400', description: '缺少 profileId。' },
      { code: '404', description: '实例不存在。' },
      { code: '503', description: '代理网关控制能力未就绪。' },
    ],
    notes: [
      'mode 可选 proxy（全部代理）、rule（按规则）、direct（全部直连）。',
      '规则 action 可选 proxy、direct、block；匹配类型可选 domain、domain_suffix、domain_keyword、ip_cidr。',
    ],
  },
  {
    id: 'api-proxy-gateway-routing-save-detail',
    parentId: 'api-proxy-gateway',
    label: '保存分流规则',
    method: 'PUT',
    path: '/api/proxy-gateway/routing',
    purpose: '保存实例分流配置并应用到运行中的网关。',
    description: '按 selector 定位实例，将规范化后的 mode 和 rules 写入实例配置；运行中的网关会立即使用新规则。',
    fields: [
      { name: 'selector', type: 'object', required: true, location: 'Body', description: '目标实例选择条件。' },
      { name: 'routing', type: 'object', required: true, location: 'Body', description: '包含 mode 和 rules 的分流配置。' },
      { name: 'force', type: 'boolean', required: false, location: 'Body', description: '是否立即断开旧连接，默认 false。' },
    ],
    requestExample: {
      language: 'bash',
      code: ({ launchBaseUrl, authHeader }) => `curl -X PUT ${launchBaseUrl}/api/proxy-gateway/routing \\
  -H "Content-Type: application/json" \\
  -H "${authHeader}: <your-api-key>" \\
  -d '{
    "selector": { "profileId": "550e8400-e29b-41d4-a716-446655440000" },
    "routing": {
      "mode": "rule",
      "rules": [
        { "id": "cn", "matchType": "domain_suffix", "pattern": ".cn", "action": "direct", "enabled": true }
      ]
    },
    "force": false
  }'`,
    },
    responseExample: {
      language: 'json',
      code: () => `{
  "ok": true,
  "profileId": "550e8400-e29b-41d4-a716-446655440000",
  "routing": {
    "mode": "rule",
    "rules": [
      { "id": "cn", "matchType": "domain_suffix", "pattern": ".cn", "action": "direct", "enabled": true }
    ]
  },
  "gateway": { "mode": "rule", "drainingConnections": 1 }
}`,
    },
    responseCodes: [
      { code: '200', description: '配置保存并应用成功。' },
      { code: '400', description: 'selector、routing 或规则字段非法。' },
      { code: '404', description: 'selector 未命中实例。' },
      { code: '502', description: '运行中的网关应用失败，持久化配置保持不变。' },
      { code: '503', description: '代理网关控制能力未就绪。' },
    ],
    notes: [
      '无效 mode、matchType、pattern、action 或 IP/CIDR 会返回 400，原配置保持不变。',
      '规则按数组顺序匹配；mode=rule 时未命中的连接默认走代理。',
      '空 pattern 的规则不会生效，不能用来表达默认规则；请用 mode=proxy 或 mode=direct 表达默认动作。',
    ],
  },
]
