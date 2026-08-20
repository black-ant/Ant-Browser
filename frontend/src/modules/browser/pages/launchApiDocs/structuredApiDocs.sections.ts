import type { StructuredApiSectionDoc } from './structuredApiDocs.types'

export const STRUCTURED_API_SECTION_DOCS: StructuredApiSectionDoc[] = [
  {
    id: 'api-profiles-launch',
    title: '实例与启动',
    intro: '这页只做实例管理和启动能力的总览。先通过实例接口完成配置，再按 launchCode 或 selector 启动实例。',
    highlights: [
      '/api/profiles 负责实例增删改查。',
      '/api/launch 负责启动。',
      '详情只从表格里的“查看详情”进入。',
    ],
  },
  {
    id: 'api-runtime',
    title: '运行态与接管',
    intro: '这页只列运行态控制和统一 CDP 入口。外部编排侧先确认 active / session，再决定是否 attach 或 stop。',
    highlights: [
      'runtime/session 用于接管前准备。',
      'runtime/status / runtime/stop 都按 selector 工作。',
      'CDP 详情只从表格进入。',
    ],
  },
  {
    id: 'api-automation',
    title: '脚本自动化',
    intro: '这页只列自动化脚本相关公共入口。先查脚本列表，再按需查详情、执行脚本、回看运行记录。',
    highlights: [
      '先查列表，再按需查详情。',
      '执行接口只接受 object 形态的 selector / params。',
      '详情只从表格进入。',
    ],
  },
  {
    id: 'api-proxy-gateway',
    title: '代理网关与热切换',
    intro: '每个运行实例有独立的本地 SOCKS5 网关。通过这些接口可以热切换出口、观察连接排空状态，并配置按域名或 IP 分流。',
    highlights: [
      '新连接使用新代理，旧连接默认自然排空；force=true 才会立即断开旧连接。',
      '代理拨号失败时不会回退直连，避免流量逃逸。',
      'routing 支持全部代理、规则分流、全部直连，以及 proxy / direct / block 动作。',
      '网关端口由程序按实例动态分配，浏览器只需使用返回的本地 proxyUrl。',
    ],
  },
]
