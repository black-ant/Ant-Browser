import type { StructuredApiEndpointDoc } from './structuredApiDocs.types'

export const AUTOMATION_API_ENDPOINT_DOCS: StructuredApiEndpointDoc[] = [
  {
    id: 'api-automation-list-detail',
    parentId: 'api-automation',
    label: '脚本列表',
    method: 'GET',
    path: '/api/automation/scripts',
    purpose: '查询可执行脚本清单。',
    description: '返回脚本元数据，用于拿 scriptId、默认 selector / params 和脚本类型。',
    fields: [],
    requestExample: {
      language: 'bash',
      code: ({ launchBaseUrl, authHeader }) => `curl ${launchBaseUrl}/api/automation/scripts \\
  -H "${authHeader}: <your-api-key>"`,
    },
    responseExample: {
      language: 'json',
      code: () => `{
  "ok": true,
  "status": "success",
  "data": {
    "count": 1,
    "items": [
      {
        "id": "news-query-txt",
        "name": "查询新闻并写 TXT",
        "type": "playwright-cdp",
        "status": "ready",
        "entryFile": "index.cjs",
        "selector": { "code": "BUYER_001" },
        "params": { "keyword": "OpenAI", "limit": 10 }
      }
    ]
  }
}`,
    },
    responseCodes: [
      { code: '200', description: '返回脚本列表。' },
      { code: '503', description: '自动化脚本能力未启用。' },
    ],
    notes: [
      '不返回 scriptText。',
    ],
  },
  {
    id: 'api-automation-script-detail',
    parentId: 'api-automation',
    label: '脚本详情',
    method: 'GET',
    path: '/api/automation/scripts/{scriptId}',
    purpose: '按 scriptId 查询单个脚本详情。',
    description: '标准单资源读取接口，用于从脚本列表进入某个脚本时补充其来源和包格式等元数据。',
    fields: [
      { name: 'scriptId', type: 'string', required: true, location: 'Path', description: '脚本唯一 ID。' },
    ],
    requestExample: {
      language: 'bash',
      code: ({ launchBaseUrl, authHeader }) => `curl ${launchBaseUrl}/api/automation/scripts/news-query-txt \\
  -H "${authHeader}: <your-api-key>"`,
    },
    responseExample: {
      language: 'json',
      code: () => `{
  "ok": true,
  "status": "success",
  "data": {
    "item": {
      "id": "news-query-txt",
      "name": "查询新闻并写 TXT",
      "type": "playwright-cdp",
      "status": "ready",
      "entryFile": "index.cjs",
      "selector": { "code": "BUYER_001" },
      "params": { "keyword": "OpenAI", "limit": 10 },
      "packageFormat": "ant-automation-script",
      "manifestVersion": 1,
      "source": {
        "type": "git",
        "uri": "https://example.com/repo.git",
        "ref": "main"
      }
    }
  }
}`,
    },
    responseCodes: [
      { code: '200', description: '返回脚本详情。' },
      { code: '404', description: '脚本不存在。' },
      { code: '503', description: '自动化脚本能力未启用。' },
    ],
    notes: [
      '不返回 scriptText。',
    ],
  },
  {
    id: 'api-automation-run-detail',
    parentId: 'api-automation',
    label: '执行脚本',
    method: 'POST',
    path: '/api/automation/scripts/run',
    purpose: '按 scriptId 执行脚本。',
    description: '外部调用方只需传入脚本 ID 和对象形态的 selector / params。推荐优先传 selector.code；如果脚本已在 UI 中绑定目标实例，也可以只传 scriptId 直接执行。',
    fields: [
      { name: 'scriptId', type: 'string', required: true, location: 'Body', description: '要执行的脚本 ID。' },
      { name: 'selector', type: 'object', required: false, location: 'Body', description: '覆盖脚本默认 selector。' },
      { name: 'params', type: 'object', required: false, location: 'Body', description: '覆盖脚本默认 params。' },
      { name: 'useScriptSelector', type: 'boolean', required: false, location: 'Body', description: '显式指定是否沿用脚本默认 selector。' },
      { name: 'useScriptParams', type: 'boolean', required: false, location: 'Body', description: '显式指定是否沿用脚本默认 params。' },
      { name: 'timeoutMs', type: 'integer', required: false, location: 'Body', description: '本次脚本执行超时时间，范围 1000 到 1800000。' },
    ],
    requestExample: {
      language: 'bash',
      code: ({ launchBaseUrl, authHeader }) => `curl -X POST ${launchBaseUrl}/api/automation/scripts/run \\
  -H "Content-Type: application/json" \\
  -H "${authHeader}: <your-api-key>" \\
  -d '{
    "scriptId": "news-query-txt",
    "selector": { "code": "BUYER_001" },
    "params": { "keyword": "OpenAI", "limit": 10 }
  }'`,
    },
    responseExample: {
      language: 'json',
      code: () => `{
  "ok": true,
  "status": "success",
  "data": {
    "run": {
      "id": "run-1",
      "scriptId": "news-query-txt",
      "scriptName": "查询新闻并写 TXT",
      "scriptType": "playwright-cdp",
      "status": "success",
      "summary": "已抓取 10 条新闻并写入 TXT",
      "durationMs": 12034
    },
    "summary": "已抓取 10 条新闻并写入 TXT"
  }
}`,
    },
    responseCodes: [
      { code: '200', description: '执行成功。' },
      { code: '400', description: 'scriptId 缺失、selector / params 不是对象，或 timeoutMs 超出范围。' },
      { code: '500', description: '脚本执行失败。' },
    ],
    notes: [
      'selector / params 必须是 object。',
      '不传时沿用脚本默认配置。',
    ],
  },
  {
    id: 'api-automation-runs-detail',
    parentId: 'api-automation',
    label: '运行记录',
    method: 'GET',
    path: '/api/automation/scripts/runs?limit=20',
    purpose: '查询最近脚本运行记录。',
    description: '返回最近 N 次脚本执行记录，适合调试、审计和任务结果回看。',
    fields: [
      { name: 'limit', type: 'integer', required: false, location: 'Query', description: '返回记录条数，默认 20，最小 1，最大 200。' },
    ],
    requestExample: {
      language: 'bash',
      code: ({ launchBaseUrl, authHeader }) => `curl "${launchBaseUrl}/api/automation/scripts/runs?limit=20" \\
  -H "${authHeader}: <your-api-key>"`,
    },
    responseExample: {
      language: 'json',
      code: () => `{
  "ok": true,
  "status": "success",
  "data": {
    "count": 1,
    "limit": 20,
    "items": [
      {
        "id": "run-1",
        "scriptId": "news-query-txt",
        "status": "success",
        "summary": "已抓取 10 条新闻并写入 TXT",
        "durationMs": 12034
      }
    ]
  }
}`,
    },
    responseCodes: [
      { code: '200', description: '返回运行记录。' },
      { code: '503', description: '自动化脚本能力未启用。' },
    ],
    notes: [],
  },
]
