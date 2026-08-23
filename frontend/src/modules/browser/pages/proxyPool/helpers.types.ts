import type { BrowserProxy } from '../../types'

export const BUILTIN_PROXY_IDS = new Set(['__direct__'])

export const BUILTIN_PROXIES: BrowserProxy[] = [
  { proxyId: '__direct__', proxyName: '直连（不走代理）', proxyConfig: 'direct://' },
]

export interface ClashProxy {
  name: string
  type: string
  server: string
  port: number
  [key: string]: unknown
}

export type ProxyImportMode = 'clash' | 'direct' | 'chain'

export interface DirectImportForm {
  proxyName: string
  protocol: 'http' | 'https' | 'socks5'
  server: string
  port: string
  username: string
  password: string
}

export interface ChainHopForm {
  protocol: 'http' | 'https' | 'socks5'
  server: string
  port: string
  username: string
  password: string
}

export interface ChainImportForm {
  proxyName: string
  localPort: string
  frontProxyId: string
  landing: ChainHopForm
}

export interface ChainSocks5HopConfig {
  protocol: 'http' | 'https' | 'socks5'
  server: string
  port: number
  username?: string
  password?: string
}

export interface ChainSocks5Config {
  localPort?: number
  first: ChainSocks5HopConfig
  second: ChainSocks5HopConfig
}

export const CHAIN_SOCKS5_PREFIX = 'chain+socks5://'
export const CHAIN_PROXY_PREFIX = 'chain+proxy://'

export interface ChainProxyConfig {
  version: 2
  frontProxyId: string
  landing: ChainSocks5HopConfig
  localPort?: number
}

export const CHAIN_QUICK_IMPORT_TEMPLATE = `{
  "name": "",
  "group": "",
  "localPort": "",
  "frontProxyId": "从代理池选择或填写节点 ID",
  "landing": {
    "protocol": "socks5",
    "server": "",
    "port": "",
    "username": "",
    "password": ""
  }
}`

export const DIRECT_QUICK_IMPORT_TEMPLATE = `{
  "name": "",
  "group": "",
  "protocol": "http",
  "server": "",
  "port": "",
  "username": "",
  "password": ""
}`

export const DIRECT_PROXY_PROTOCOL_OPTIONS = [
  { value: 'http', label: 'HTTP' },
  { value: 'https', label: 'HTTPS' },
  { value: 'socks5', label: 'SOCKS5' },
] as const

export const INITIAL_DIRECT_IMPORT_FORM: DirectImportForm = {
  proxyName: '',
  protocol: 'http',
  server: '',
  port: '',
  username: '',
  password: '',
}

export const INITIAL_CHAIN_IMPORT_FORM: ChainImportForm = {
  proxyName: '',
  localPort: '',
  frontProxyId: '',
  landing: {
    protocol: 'socks5',
    server: '',
    port: '',
    username: '',
    password: '',
  },
}

export function createInitialChainImportForm(): ChainImportForm {
  return {
    ...INITIAL_CHAIN_IMPORT_FORM,
    landing: { ...INITIAL_CHAIN_IMPORT_FORM.landing },
  }
}

export interface ImportCandidate {
  proxyName: string
  proxyConfig: string
  groupName?: string
}

export interface ProxyDisplayInfo {
  proxyId: string
  proxyName: string
  proxyConfig: string
  preferredKernel: string
  groupName: string
  sourceId: string
  sourceUrl: string
  sourceAutoRefresh: boolean
  sourceRefreshIntervalM: number
  sourceLastRefreshAt: string
  type: string
  server: string
  port: number
  latencyMs?: number
}

export interface URLImportSourceMeta {
  sourceId: string
  sourceUrl: string
  sourceNamePrefix: string
  sourceGroupName: string
  sourceDnsServers: string
  sourceAutoRefresh: boolean
  sourceRefreshIntervalM: number
  sourceLastRefreshAt: string
}
