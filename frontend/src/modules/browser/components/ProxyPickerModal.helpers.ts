export type SpeedResult = { ok: boolean; latencyMs: number; engine?: string; error: string }

export type ChainSocksHop = {
  protocol?: 'http' | 'socks5'
  server?: string
  port?: number
  username?: string
  password?: string
  proxyConfig?: string
}

export type ChainSocksConfig = {
  localPort?: number
  first?: ChainSocksHop
  second?: ChainSocksHop
}

export interface ChainHopForm {
  protocol: 'http' | 'socks5'
  server: string
  port: string
  username: string
  password: string
}

export type ChainNodeProtocol = 'vless' | 'vmess' | 'trojan' | 'ss' | 'hysteria2'
export type ChainNodeSource = 'pool' | 'manual'

export interface ChainEditForm {
  proxyName: string
  localPort: string
  firstMode: 'standard' | 'node'
  firstNodeProtocol: ChainNodeProtocol
  firstNodeSource: ChainNodeSource
  firstProxyConfig: string
  first: ChainHopForm
  second: ChainHopForm
}

export const INITIAL_CHAIN_EDIT_FORM: ChainEditForm = {
  proxyName: '',
  localPort: '',
  firstMode: 'standard',
  firstNodeProtocol: 'vless',
  firstNodeSource: 'pool',
  firstProxyConfig: '',
  first: { protocol: 'http', server: '', port: '', username: '', password: '' },
  second: { protocol: 'http', server: '', port: '', username: '', password: '' },
}

export const ALL_GROUP = '__all__'
export const DIRECT_PROXY_ID = '__direct__'
export const SPEED_RESULT_EVENT = 'proxy:speed:result'
export const BATCH_TEST_CONCURRENCY = 20
export const CHAIN_SOCKS5_PREFIX = 'chain+socks5://'

function detectChainNodeProtocol(proxyConfig: string): ChainNodeProtocol {
  const scheme = proxyConfig.trim().match(/^([a-zA-Z0-9+.-]+):\/\//)?.[1]?.toLowerCase()
  if (scheme === 'hysteria2' || scheme === 'hy2') return 'hysteria2'
  return scheme === 'vmess' || scheme === 'trojan' || scheme === 'ss' ? scheme : 'vless'
}
function normalizeChainHop(raw: unknown, allowProxyConfig: boolean): ChainSocksHop | null {
  if (!raw || typeof raw !== 'object') return null
  const hop = raw as Record<string, unknown>
  const proxyConfig = String(hop.proxyConfig || '').trim()
  if (proxyConfig) return allowProxyConfig ? { proxyConfig } : null

  const protocol = String(hop.protocol || '').trim().toLowerCase()
  if (protocol && protocol !== 'socks5' && protocol !== 'http') return null
  const server = String(hop.server || '').trim()
  if (!server) return null
  const portVal = Number(hop.port || 0)
  if (!Number.isInteger(portVal) || portVal < 1 || portVal > 65535) return null
  const username = String(hop.username || '').trim()
  const password = hop.password === undefined || hop.password === null ? '' : String(hop.password)
  if (password && !username) return null

  return {
    protocol: protocol === 'http' ? 'http' : 'socks5',
    server,
    port: portVal,
    username: username || undefined,
    password: password || undefined,
  }
}

export function parseChainSocks5Config(proxyConfig: string): ChainSocksConfig | null {
  const cfg = proxyConfig.trim()
  if (!cfg.toLowerCase().startsWith(CHAIN_SOCKS5_PREFIX)) return null
  const encoded = cfg.slice(CHAIN_SOCKS5_PREFIX.length)
  if (!encoded) return null

  try {
    const decoded = decodeURIComponent(encoded)
    const parsed = JSON.parse(decoded) as Record<string, unknown>
    const first = normalizeChainHop(parsed.first, true)
    const second = normalizeChainHop(parsed.second, false)
    if (!first || !second) return null

    const localPortRaw = parsed.localPort
    const localPortNum = localPortRaw === undefined || localPortRaw === null || localPortRaw === '' ? 0 : Number(localPortRaw)
    if (!Number.isInteger(localPortNum) || localPortNum < 0 || localPortNum > 65535) return null
    return { first, second, localPort: localPortNum > 0 ? localPortNum : undefined }
  } catch {
    return null
  }
}

export function toChainEditForm(proxyName: string, cfg: ChainSocksConfig): ChainEditForm {
  return {
    proxyName,
    localPort: cfg.localPort ? String(cfg.localPort) : '',
    firstMode: cfg.first?.proxyConfig ? 'node' : 'standard',
    firstNodeProtocol: detectChainNodeProtocol(cfg.first?.proxyConfig || ''),
    firstNodeSource: 'manual',
    firstProxyConfig: cfg.first?.proxyConfig || '',
    first: {
      protocol: cfg.first?.protocol || 'socks5',
      server: cfg.first?.server || '',
      port: cfg.first?.port ? String(cfg.first.port) : '',
      username: cfg.first?.username || '',
      password: cfg.first?.password || '',
    },
    second: {
      protocol: cfg.second?.protocol || 'socks5',
      server: cfg.second?.server || '',
      port: cfg.second?.port ? String(cfg.second.port) : '',
      username: cfg.second?.username || '',
      password: cfg.second?.password || '',
    },
  }
}

function parseStandardHop(label: string, hop: ChainHopForm): ChainSocksHop {
  const protocol = hop.protocol === 'socks5' ? 'socks5' : 'http'
  const server = hop.server.trim()
  if (!server) throw new Error(`请输入${label}代理地址`)
  if (/^[a-zA-Z][a-zA-Z0-9+.-]*:\/\//.test(server)) {
    throw new Error(`${label}代理地址只需要填写主机名或 IP，不需要协议头`)
  }
  const portInput = hop.port.trim()
  if (!portInput) throw new Error(`请输入${label}代理端口`)
  if (!/^\d+$/.test(portInput)) throw new Error(`${label}代理端口必须为数字`)
  const port = Number(portInput)
  if (!Number.isInteger(port) || port < 1 || port > 65535) throw new Error(`${label}代理端口必须在 1-65535 之间`)
  const username = hop.username.trim()
  const password = hop.password
  if (password && !username) throw new Error(`${label}填写密码时请同时填写账号`)
  return { protocol, server, port, username: username || undefined, password: password || undefined }
}

export function buildChainProxyConfig(form: ChainEditForm): string {
  const localPortInput = form.localPort.trim()
  if (localPortInput && !/^\d+$/.test(localPortInput)) throw new Error('本地监听端口必须为数字')
  const localPort = localPortInput ? Number(localPortInput) : 0
  if (localPortInput && (!Number.isInteger(localPort) || localPort < 1 || localPort > 65535)) {
    throw new Error('本地监听端口必须在 1-65535 之间')
  }

  const first: ChainSocksHop = form.firstMode === 'node'
    ? (() => {
        const proxyConfig = form.firstProxyConfig.trim()
        if (!proxyConfig) throw new Error('请输入第一层机场节点配置')
        if (/^chain\+socks5:\/\//i.test(proxyConfig)) throw new Error('第一层不支持嵌套链式代理')
        return { proxyConfig }
      })()
    : parseStandardHop('第一层', form.first)

  const payload: ChainSocksConfig = {
    first,
    second: parseStandardHop('第二层', form.second),
    localPort: localPort > 0 ? localPort : undefined,
  }
  return `${CHAIN_SOCKS5_PREFIX}${encodeURIComponent(JSON.stringify(payload))}`
}

export function formatProxyConfigForDisplay(proxyConfig: string): string {
  const raw = (proxyConfig || '').trim()
  if (!raw || !raw.toLowerCase().startsWith(CHAIN_SOCKS5_PREFIX)) return raw
  const encoded = raw.slice(CHAIN_SOCKS5_PREFIX.length)
  if (!encoded) return raw

  try {
    const decoded = decodeURIComponent(encoded)
    const parsed = JSON.parse(decoded) as ChainSocksConfig
    const firstLabel = parsed.first?.proxyConfig
      ? (parsed.first.proxyConfig.match(/^([a-zA-Z0-9+.-]+):\/\//)?.[1]?.toUpperCase() || 'NODE')
      : (parsed.first?.server || '').trim()
    const secondServer = (parsed.second?.server || '').trim()
    if (!firstLabel || !secondServer) return raw
    return `${firstLabel} -> ${secondServer}`
  } catch {
    return raw
  }
}
