import yaml from 'js-yaml'

// 代理配置解析工具（从 ProxyPoolPage 提取的纯函数 + 类型，无副作用、可复用）。

export interface ClashProxy {
  name: string
  type: string
  server: string
  port: number
  [key: string]: any
}

export const CHAIN_SOCKS5_PREFIX = 'chain+socks5://'

export interface ChainSocks5HopConfig {
  protocol: 'socks5'
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

export function parseChainSocks5Config(proxyConfig: string): ChainSocks5Config | null {
  const cfg = proxyConfig.trim()
  if (!cfg.toLowerCase().startsWith(CHAIN_SOCKS5_PREFIX)) {
    return null
  }
  const encoded = cfg.slice(CHAIN_SOCKS5_PREFIX.length)
  if (!encoded) {
    return null
  }

  const normalizeHop = (raw: unknown): ChainSocks5HopConfig | null => {
    if (!raw || typeof raw !== 'object') return null
    const hop = raw as Record<string, unknown>
    const protocol = String(hop.protocol || '').trim().toLowerCase()
    if (protocol && protocol !== 'socks5') return null

    const server = String(hop.server || '').trim()
    if (!server) return null

    const portVal = Number(hop.port || 0)
    if (!Number.isInteger(portVal) || portVal < 1 || portVal > 65535) return null

    const username = String(hop.username || '').trim()
    const password = hop.password === undefined || hop.password === null ? '' : String(hop.password)
    if (password && !username) return null

    return {
      protocol: 'socks5',
      server,
      port: portVal,
      username: username || undefined,
      password: password || undefined,
    }
  }

  try {
    const decoded = decodeURIComponent(encoded)
    const parsed = JSON.parse(decoded) as Record<string, unknown>
    const first = normalizeHop(parsed.first)
    const second = normalizeHop(parsed.second)
    if (!first || !second) return null

    const localPortRaw = parsed.localPort
    const localPortNum = localPortRaw === undefined || localPortRaw === null || localPortRaw === ''
      ? 0
      : Number(localPortRaw)
    if (!Number.isInteger(localPortNum) || localPortNum < 0 || localPortNum > 65535) return null

    return {
      first,
      second,
      localPort: localPortNum > 0 ? localPortNum : undefined,
    }
  } catch {
    return null
  }
}

export function parseProxyInfo(proxyConfig: string): { type: string; server: string; port: number } {
  const cfg = proxyConfig.trim()
  if (cfg === 'direct://') return { type: 'direct', server: '-', port: 0 }

  const chain = parseChainSocks5Config(cfg)
  if (chain) {
    return { type: 'chain-socks5', server: '127.0.0.1', port: chain.localPort || 0 }
  }

  const urlMatch = cfg.match(/^([a-zA-Z0-9+\-]+):\/\//)
  if (urlMatch) {
    const scheme = urlMatch[1].toLowerCase()
    try {
      const u = new URL(cfg)
      return { type: scheme, server: u.hostname, port: parseInt(u.port) || 0 }
    } catch {
      return { type: scheme, server: '-', port: 0 }
    }
  }
  try {
    const parsed = yaml.load(cfg) as ClashProxy[] | ClashProxy
    const proxy = Array.isArray(parsed) ? parsed[0] : parsed
    return { type: proxy?.type || '-', server: proxy?.server || '-', port: proxy?.port || 0 }
  } catch {
    return { type: '-', server: '-', port: 0 }
  }
}
