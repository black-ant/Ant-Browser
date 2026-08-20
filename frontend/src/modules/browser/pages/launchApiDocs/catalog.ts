import type { AutomationDemoActionKey } from '../../demoSession'
import { DEFAULT_API_AUTH, DEFAULT_LAUNCH_BASE_URL } from '../../launchContext'
import {
  DOC_API_OVERVIEW,
  DOC_CORE_INTRO,
  DOC_EXTENSION_INTRO,
  DOC_OPERATION_FLOW,
  DOC_PROXY_INTRO,
  DOC_SKILL_USAGE,
  DOC_TUTORIAL,
} from './contentIntro'
import { DOC_CHANGELOG } from './contentChangelog'
import {
  DOC_API_AUTOMATION,
  DOC_API_PROXY_GATEWAY,
  DOC_API_PROFILES_LAUNCH,
  DOC_API_RUNTIME,
} from './contentApi'
import { DOC_API_SUPPORT } from './contentReference'
import { getStructuredApiHiddenDocItems } from './structuredApiDocs'

export interface LaunchDocDemoConfig {
  title: string
  subtitle: string
  primaryDocLabel: string
  actionKeys: AutomationDemoActionKey[]
}

export interface LaunchDocItem {
  id: string
  label: string
  summary: string
  content: string
  demo?: LaunchDocDemoConfig
}

export interface LaunchDocGroup {
  id: string
  label: string
  items: LaunchDocItem[]
}

export const DOC_GROUPS: LaunchDocGroup[] = [
  {
    id: 'tutorial',
    label: '使用教程',
    items: [
      {
        id: 'tutorial-basic',
        label: '基础使用',
        summary: '应用上手流程、菜单入口和最小接口链路。',
        content: DOC_TUTORIAL,
      },
      {
        id: 'tutorial-skill',
        label: 'SKILL 使用',
        summary: 'OpenClaw 接入 ant-chrome-openclaw 的安装、提问模板和稳定使用规则。',
        content: DOC_SKILL_USAGE,
      },
      {
        id: 'tutorial-flow',
        label: '操作流程',
        summary: '按步骤串起内核、代理、实例和接口调用。',
        content: DOC_OPERATION_FLOW,
      },
    ],
  },
  {
    id: 'changelog',
    label: '更新日志',
    items: [
      {
        id: 'changelog-versions',
        label: '版本更新',
        summary: '按大版本查看主要变化。',
        content: DOC_CHANGELOG,
      },
    ],
  },
  {
    id: 'core',
    label: '内核介绍',
    items: [
      {
        id: 'core-management',
        label: '内核渠道与用法',
        summary: '内核下载来源、目录结构和应用内使用方式。',
        content: DOC_CORE_INTRO,
      },
    ],
  },
  {
    id: 'proxy',
    label: '代理介绍',
    items: [
      {
        id: 'proxy-usage',
        label: '代理池使用',
        summary: '代理类型、导入方式和实例绑定建议。',
        content: DOC_PROXY_INTRO,
      },
    ],
  },
  {
    id: 'extension',
    label: '插件介绍',
    items: [
      {
        id: 'extension-usage',
        label: '插件包管理',
        summary: '插件安装、实例限制、分组选择和单实例配置。',
        content: DOC_EXTENSION_INTRO,
      },
    ],
  },
  {
    id: 'api',
    label: '接口介绍',
    items: [
      {
        id: 'api-overview',
        label: '接口总览',
        summary: '认证、选择规则和全部对外接口索引。',
        content: DOC_API_OVERVIEW,
      },
      {
        id: 'api-profiles-launch',
        label: '实例与启动',
        summary: '实例配置、按 Code 启动和参数化启动。',
        content: DOC_API_PROFILES_LAUNCH,
      },
      {
        id: 'api-runtime',
        label: '运行态与接管',
        summary: 'runtime session、活动实例和统一 CDP 入口。',
        content: DOC_API_RUNTIME,
      },
      {
        id: 'api-proxy-gateway',
        label: '代理网关与热切换',
        summary: '运行中切换代理、连接排空、失败关闭和域名/IP 分流。',
        content: DOC_API_PROXY_GATEWAY,
      },
      {
        id: 'api-automation',
        label: '脚本自动化',
        summary: '自动化接口总览、字段规则和调用顺序。',
        content: DOC_API_AUTOMATION,
      },
      {
        id: 'api-support',
        label: '排障与日志',
        summary: '调用日志、常见错误码和排查顺序。',
        content: DOC_API_SUPPORT,
      },
    ],
  },
]

const DOC_ITEMS = DOC_GROUPS.flatMap((group) => group.items)
const HIDDEN_DOC_ITEMS: LaunchDocItem[] = getStructuredApiHiddenDocItems()

export function getDefaultDoc(): LaunchDocItem {
  return DOC_GROUPS[0].items[0]
}

export function findDocById(targetId: string): LaunchDocItem | null {
  return DOC_ITEMS.find((item) => item.id === targetId)
    || HIDDEN_DOC_ITEMS.find((item) => item.id === targetId)
    || null
}

export function findGroupByDocId(targetId: string): LaunchDocGroup | null {
  return DOC_GROUPS.find((group) => group.items.some((item) => item.id === targetId)) || null
}

export function getAdjacentDocs(targetId: string): {
  previous: LaunchDocItem | null
  next: LaunchDocItem | null
} {
  const index = DOC_ITEMS.findIndex((item) => item.id === targetId)
  if (index < 0) {
    return { previous: null, next: null }
  }

  return {
    previous: index > 0 ? DOC_ITEMS[index - 1] : null,
    next: index < DOC_ITEMS.length - 1 ? DOC_ITEMS[index + 1] : null,
  }
}

export function renderDocWithLaunchContext(raw: string, baseUrl: string, authHeader: string): string {
  if (!raw) return raw
  const safeBase = baseUrl.trim() || DEFAULT_LAUNCH_BASE_URL
  const safeAuthHeader = authHeader.trim() || DEFAULT_API_AUTH.header
  const hostPort = safeBase.replace(/^https?:\/\//, '')
  return raw
    .split('http://127.0.0.1:19876').join(safeBase)
    .split('127.0.0.1:19876').join(hostPort)
    .split('X-Ant-Api-Key').join(safeAuthHeader)
}
