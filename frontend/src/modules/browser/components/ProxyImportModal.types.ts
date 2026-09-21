import type { BrowserProxy } from '../types'

export interface ProxyImportModalProps {
  open: boolean
  onClose: () => void
  existingProxies: BrowserProxy[]
  groups: string[]
  globalAutoRefreshEnabled?: boolean
  globalRefreshIntervalM?: number
  onImported?: (newProxies: BrowserProxy[]) => void | Promise<void>
}

export interface ClashProxy {
  name: string
  type: string
  server: string
  port: number
  [key: string]: any
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
  protocol: 'http' | 'socks5'
  server: string
  port: string
  username: string
  password: string
}

export type ChainFirstHopMode = 'standard' | 'node'
export type ChainNodeProtocol = 'vless' | 'vmess' | 'trojan' | 'ss' | 'hysteria2'
export type ChainNodeSource = 'pool' | 'manual'

export interface ChainImportForm {
  proxyName: string
  localPort: string
  firstMode: ChainFirstHopMode
  firstNodeProtocol: ChainNodeProtocol
  firstNodeSource: ChainNodeSource
  firstProxyConfig: string
  first: ChainHopForm
  second: ChainHopForm
}

export const CHAIN_FIRST_PROTOCOL_OPTIONS = [
  { value: 'http', label: 'HTTP' },
  { value: 'socks5', label: 'SOCKS5' },
  { value: 'vless', label: 'VLESS' },
  { value: 'vmess', label: 'VMess' },
  { value: 'trojan', label: 'Trojan' },
  { value: 'ss', label: 'SS' },
  { value: 'hysteria2', label: 'HY2' },
] as const

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
  firstMode: 'standard',
  firstNodeProtocol: 'vless',
  firstNodeSource: 'pool',
  firstProxyConfig: '',
  first: {
    protocol: 'http',
    server: '',
    port: '',
    username: '',
    password: '',
  },
  second: {
    protocol: 'http',
    server: '',
    port: '',
    username: '',
    password: '',
  },
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
  groupName: string
  type: string
  server: string
  port: number
}

export const CHAIN_SOCKS5_PREFIX = 'chain+socks5://'

export interface ChainSocks5HopConfig {
  protocol?: 'http' | 'socks5'
  server?: string
  port?: number
  username?: string
  password?: string
  proxyConfig?: string
}

export interface ChainSocks5Config {
  localPort?: number
  first: ChainSocks5HopConfig
  second: ChainSocks5HopConfig
}
