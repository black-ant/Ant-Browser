import type { BrowserProxy } from '../types'

// 内置代理（业务数据，从页面组件抽到配置）。不可删除、不可编辑。
export const BUILTIN_PROXY_IDS = new Set(['__direct__', '__local__'])

export const BUILTIN_PROXIES: BrowserProxy[] = [
  { proxyId: '__direct__', proxyName: '直连（不走代理）', proxyConfig: 'direct://' },
  { proxyId: '__local__', proxyName: '本地代理', proxyConfig: 'http://127.0.0.1:7890' },
]
