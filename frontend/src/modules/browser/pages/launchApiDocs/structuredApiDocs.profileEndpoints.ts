import type { StructuredApiEndpointDoc } from './structuredApiDocs.types'

export const PROFILE_API_ENDPOINT_DOCS: StructuredApiEndpointDoc[] = [
  {
    id: 'api-profiles-list-detail',
    parentId: 'api-profiles-launch',
    label: '实例列表',
    method: 'GET',
    path: '/api/profiles',
    purpose: '列出当前全部实例。',
    description: '读取当前实例目录中的全部实例配置，适合做实例选择器或管理后台列表。',
    fields: [],
    requestExample: {
      language: 'bash',
      code: ({ launchBaseUrl, authHeader }) => `curl ${launchBaseUrl}/api/profiles \\
  -H "${authHeader}: <your-api-key>"`,
    },
    responseExample: {
      language: 'json',
      code: () => `{
  "ok": true,
  "count": 1,
  "items": [
    {
      "profileId": "550e8400-e29b-41d4-a716-446655440000",
      "profileName": "buyer-001",
      "launchCode": "BUYER_001",
      "keywords": ["buyer-001"],
      "tags": ["电商"],
      "proxyId": "proxy-us",
      "running": false,
      "debugReady": false
    }
  ]
}`,
    },
    responseCodes: [
      { code: '200', description: '返回实例列表。' },
      { code: '503', description: '实例目录当前不可用。' },
    ],
    notes: [],
  },
  {
    id: 'api-profiles-create-detail',
    parentId: 'api-profiles-launch',
    label: '创建实例',
    method: 'POST',
    path: '/api/profiles',
    purpose: '创建一个新实例，可选创建后立即启动。',
    description: '写入实例配置，必要时同时申请 launchCode，并支持通过 autoLaunch 在创建后直接启动浏览器。',
    fields: [
      { name: 'profile', type: 'object', required: true, location: 'Body', description: '实例配置主体。' },
      { name: 'launchCode', type: 'string', required: false, location: 'Body', description: '指定实例 launchCode。' },
      { name: 'autoLaunch', type: 'boolean', required: false, location: 'Body', description: '是否在创建后立即启动。' },
      { name: 'start', type: 'object', required: false, location: 'Body', description: '仅本次自动启动时附加的启动参数。' },
    ],
    requestExample: {
      language: 'bash',
      code: ({ launchBaseUrl, authHeader }) => `curl -X POST ${launchBaseUrl}/api/profiles \\
  -H "Content-Type: application/json" \\
  -H "${authHeader}: <your-api-key>" \\
  -d '{
    "profile": {
      "profileName": "buyer-001",
      "proxyId": "proxy-us",
      "keywords": ["buyer-001"],
      "tags": ["电商"]
    },
    "launchCode": "BUYER_001"
  }'`,
    },
    responseExample: {
      language: 'json',
      code: () => `{
  "ok": true,
  "created": true,
  "updated": false,
  "launched": false,
  "profileId": "550e8400-e29b-41d4-a716-446655440000",
  "profileName": "buyer-001",
  "launchCode": "BUYER_001",
  "profile": {
    "profileId": "550e8400-e29b-41d4-a716-446655440000",
    "profileName": "buyer-001",
    "keywords": ["buyer-001"],
    "proxyId": "proxy-us"
  }
}`,
    },
    responseCodes: [
      { code: '201', description: '实例创建成功。' },
      { code: '400', description: '请求体非法或 profile 缺失。' },
      { code: '409', description: 'launchCode 冲突或实例数超限。' },
    ],
    notes: [
      'autoLaunch=true 时，响应会附带启动结果字段。',
      'profile.proxyId 与 profile.proxyConfig 同时传时，优先使用 proxyId 对应的代理池节点。',
      '若 proxyId 无效：提供 proxyConfig 则按自定义代理保存；未提供 proxyConfig 则返回 400。',
    ],
  },
  {
    id: 'api-profiles-get-detail',
    parentId: 'api-profiles-launch',
    label: '单个实例',
    method: 'GET',
    path: '/api/profiles/{profileId}',
    purpose: '查询单个实例配置。',
    description: '读取指定实例的完整配置快照，适合进入实例详情页或编辑页前预加载数据。',
    fields: [
      { name: 'profileId', type: 'string', required: true, location: 'Path', description: '实例 ID。' },
    ],
    requestExample: {
      language: 'bash',
      code: ({ launchBaseUrl, authHeader }) => `curl ${launchBaseUrl}/api/profiles/550e8400-e29b-41d4-a716-446655440000 \\
  -H "${authHeader}: <your-api-key>"`,
    },
    responseExample: {
      language: 'json',
      code: () => `{
  "ok": true,
  "profileId": "550e8400-e29b-41d4-a716-446655440000",
  "profileName": "buyer-001",
  "launchCode": "BUYER_001",
  "profile": {
    "profileId": "550e8400-e29b-41d4-a716-446655440000",
    "profileName": "buyer-001",
    "keywords": ["buyer-001"],
    "tags": ["电商"],
    "proxyId": "proxy-us"
  }
}`,
    },
    responseCodes: [
      { code: '200', description: '返回实例详情。' },
      { code: '404', description: '实例不存在。' },
    ],
    notes: [],
  },
  {
    id: 'api-profiles-update-detail',
    parentId: 'api-profiles-launch',
    label: '更新实例',
    method: 'PUT',
    path: '/api/profiles/{profileId}',
    purpose: '更新指定实例配置。',
    description: '用整份 profile 配置覆盖更新实例，可选顺带更新 launchCode，并支持更新后立即启动。',
    fields: [
      { name: 'profileId', type: 'string', required: true, location: 'Path', description: '实例 ID。' },
      { name: 'profile', type: 'object', required: true, location: 'Body', description: '更新后的实例配置。' },
      { name: 'launchCode', type: 'string', required: false, location: 'Body', description: '需要覆盖时传新的 launchCode。' },
      { name: 'autoLaunch', type: 'boolean', required: false, location: 'Body', description: '更新后是否直接启动。' },
    ],
    requestExample: {
      language: 'bash',
      code: ({ launchBaseUrl, authHeader }) => `curl -X PUT ${launchBaseUrl}/api/profiles/550e8400-e29b-41d4-a716-446655440000 \\
  -H "Content-Type: application/json" \\
  -H "${authHeader}: <your-api-key>" \\
  -d '{
    "profile": {
      "profileName": "buyer-001-updated",
      "proxyId": "proxy-us",
      "keywords": ["buyer-001", "checkout"]
    }
  }'`,
    },
    responseExample: {
      language: 'json',
      code: () => `{
  "ok": true,
  "created": false,
  "updated": true,
  "launched": false,
  "profileId": "550e8400-e29b-41d4-a716-446655440000",
  "profileName": "buyer-001-updated",
  "launchCode": "BUYER_001"
}`,
    },
    responseCodes: [
      { code: '200', description: '更新成功。' },
      { code: '400', description: '请求体非法。' },
      { code: '404', description: '实例不存在。' },
    ],
    notes: [
      '整份更新，不是 patch。',
      'profile.proxyId 与 profile.proxyConfig 同时传时，优先使用 proxyId 对应的代理池节点。',
      '若 proxyId 无效：提供 proxyConfig 则按自定义代理保存；未提供 proxyConfig 则返回 400。',
    ],
  },
  {
    id: 'api-profiles-delete-detail',
    parentId: 'api-profiles-launch',
    label: '删除实例',
    method: 'DELETE',
    path: '/api/profiles/{profileId}',
    purpose: '删除一个未运行中的实例。',
    description: '删除实例配置并移除关联 launchCode；运行中的实例会被直接拒绝删除。',
    fields: [
      { name: 'profileId', type: 'string', required: true, location: 'Path', description: '实例 ID。' },
    ],
    requestExample: {
      language: 'bash',
      code: ({ launchBaseUrl, authHeader }) => `curl -X DELETE ${launchBaseUrl}/api/profiles/550e8400-e29b-41d4-a716-446655440000 \\
  -H "${authHeader}: <your-api-key>"`,
    },
    responseExample: {
      language: 'json',
      code: () => `{
  "ok": true,
  "deleted": true,
  "profileId": "550e8400-e29b-41d4-a716-446655440000",
  "profileName": "buyer-001",
  "launchCode": "BUYER_001"
}`,
    },
    responseCodes: [
      { code: '200', description: '删除成功。' },
      { code: '404', description: '实例不存在。' },
      { code: '409', description: '实例仍在运行，不能直接删除。' },
    ],
    notes: [
      '运行中的实例先 stop，再 delete。',
    ],
  },
  {
    id: 'api-profiles-status-detail',
    parentId: 'api-profiles-launch',
    label: '实例状态',
    method: 'GET',
    path: '/api/profiles/{profileId}/status',
    purpose: '查询单个实例的实时运行态。',
    description: '返回运行中、debugReady、cdpUrl 等运行态字段，适合精确观察单个实例当前状态。',
    fields: [
      { name: 'profileId', type: 'string', required: true, location: 'Path', description: '实例 ID。' },
    ],
    requestExample: {
      language: 'bash',
      code: ({ launchBaseUrl, authHeader }) => `curl ${launchBaseUrl}/api/profiles/550e8400-e29b-41d4-a716-446655440000/status \\
  -H "${authHeader}: <your-api-key>"`,
    },
    responseExample: {
      language: 'json',
      code: () => `{
  "ok": true,
  "profileId": "550e8400-e29b-41d4-a716-446655440000",
  "profileName": "buyer-001",
  "launchCode": "BUYER_001",
  "running": true,
  "debugPort": 9333,
  "debugReady": true,
  "active": true,
  "cdpUrl": "http://127.0.0.1:19876",
  "directDebugUrl": "http://127.0.0.1:9333"
}`,
    },
    responseCodes: [
      { code: '200', description: '返回实例运行态。' },
      { code: '404', description: '实例不存在。' },
    ],
    notes: [],
  },
  {
    id: 'api-profiles-stop-detail',
    parentId: 'api-profiles-launch',
    label: '停止实例',
    method: 'POST',
    path: '/api/profiles/{profileId}/stop',
    purpose: '精确停止一个指定实例。',
    description: '按 profileId 停止实例，适合任务完成后的精确回收。',
    fields: [
      { name: 'profileId', type: 'string', required: true, location: 'Path', description: '实例 ID。' },
    ],
    requestExample: {
      language: 'bash',
      code: ({ launchBaseUrl, authHeader }) => `curl -X POST ${launchBaseUrl}/api/profiles/550e8400-e29b-41d4-a716-446655440000/stop \\
  -H "${authHeader}: <your-api-key>"`,
    },
    responseExample: {
      language: 'json',
      code: () => `{
  "ok": true,
  "stopped": true,
  "profileId": "550e8400-e29b-41d4-a716-446655440000",
  "running": false,
  "debugReady": false,
  "active": false
}`,
    },
    responseCodes: [
      { code: '200', description: '停止成功。' },
      { code: '404', description: '实例不存在。' },
      { code: '503', description: '当前环境不支持运行态控制。' },
    ],
    notes: [],
  },
  {
    id: 'api-launch-code-detail',
    parentId: 'api-profiles-launch',
    label: '按 Code 启动',
    method: 'GET',
    path: '/api/launch/{code}',
    purpose: '按唯一 launchCode 启动实例。',
    description: '最短路径的启动接口，适合外部系统已经拿到唯一 launchCode 的场景。',
    fields: [
      { name: 'code', type: 'string', required: true, location: 'Path', description: '实例 launchCode。' },
    ],
    requestExample: {
      language: 'bash',
      code: ({ launchBaseUrl, authHeader }) => `curl ${launchBaseUrl}/api/launch/BUYER_001 \\
  -H "${authHeader}: <your-api-key>"`,
    },
    responseExample: {
      language: 'json',
      code: () => `{
  "ok": true,
  "profileId": "550e8400-e29b-41d4-a716-446655440000",
  "profileName": "buyer-001",
  "launchCode": "BUYER_001",
  "pid": 10240,
  "debugPort": 9333,
  "debugReady": true,
  "cdpPort": 19876,
  "cdpUrl": "http://127.0.0.1:19876"
}`,
    },
    responseCodes: [
      { code: '200', description: '启动成功。' },
      { code: '404', description: 'launchCode 不存在。' },
    ],
    notes: [],
  },
  {
    id: 'api-launch-body-detail',
    parentId: 'api-profiles-launch',
    label: '按 selector 启动',
    method: 'POST',
    path: '/api/launch',
    purpose: '按 selector 或兼容顶层字段启动实例。',
    description: '更灵活的启动入口，支持 selector、兼容顶层选择字段、launchArgs、startUrls 和 skipDefaultStartUrls 等临时参数。',
    fields: [
      { name: 'selector', type: 'object', required: false, location: 'Body', description: '目标实例选择条件；新接入推荐使用。' },
      { name: 'code', type: 'string', required: false, location: 'Body', description: '兼容写法：等价于 selector.code。' },
      { name: 'profileId', type: 'string', required: false, location: 'Body', description: '兼容写法：等价于 selector.profileId。' },
      { name: 'profileName', type: 'string', required: false, location: 'Body', description: '兼容写法：等价于 selector.profileName。' },
      { name: 'keyword / keywords', type: 'string / string[]', required: false, location: 'Body', description: '兼容写法：等价于 selector.keyword / selector.keywords。' },
      { name: 'tag / tags', type: 'string / string[]', required: false, location: 'Body', description: '兼容写法：等价于 selector.tag / selector.tags。' },
      { name: 'groupId', type: 'string', required: false, location: 'Body', description: '兼容写法：等价于 selector.groupId。' },
      { name: 'matchMode', type: 'unique | first | all', required: false, location: 'Body', description: '兼容写法：等价于 selector.matchMode。' },
      { name: 'launchArgs', type: 'string[]', required: false, location: 'Body', description: '本次启动的临时附加参数。' },
      { name: 'startUrls', type: 'string[]', required: false, location: 'Body', description: '本次启动后额外打开的网址。' },
      { name: 'skipDefaultStartUrls', type: 'boolean', required: false, location: 'Body', description: '是否跳过实例默认启动 URL。' },
    ],
    requestExample: {
      language: 'bash',
      code: ({ launchBaseUrl, authHeader }) => `curl -X POST ${launchBaseUrl}/api/launch \\
  -H "Content-Type: application/json" \\
  -H "${authHeader}: <your-api-key>" \\
  -d '{
    "selector": {
      "keyword": "checkout",
      "tags": ["电商", "北美"],
      "matchMode": "unique"
    },
    "skipDefaultStartUrls": true
  }'`,
    },
    responseExample: {
      language: 'json',
      code: () => `{
  "ok": true,
  "profileId": "550e8400-e29b-41d4-a716-446655440000",
  "profileName": "buyer-001",
  "launchCode": "BUYER_001",
  "debugReady": true,
  "cdpUrl": "http://127.0.0.1:19876"
}`,
    },
    responseCodes: [
      { code: '200', description: '启动成功。' },
      { code: '400', description: 'selector 缺失或请求体非法。' },
      { code: '409', description: 'selector 命中多个实例。' },
    ],
    notes: [
      'selector 为空且没有任何兼容顶层选择字段时返回 400。',
      'matchMode=all 只在这个接口可用。',
      'proxyId / proxyConfig 只影响本次启动，不覆盖实例原代理。',
    ],
  },
]
