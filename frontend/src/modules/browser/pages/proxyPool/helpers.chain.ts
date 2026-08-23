import type { ChainHopForm, ChainImportForm, ChainProxyConfig, ChainSocks5Config, ChainSocks5HopConfig, ImportCandidate } from './helpers.types'
import { CHAIN_PROXY_PREFIX, CHAIN_SOCKS5_PREFIX } from './helpers.types'

export function parseChainProxyConfig(proxyConfig: string): ChainProxyConfig | null {
  const raw = proxyConfig.trim()
  if (!raw.toLowerCase().startsWith(CHAIN_PROXY_PREFIX)) return null
  try {
    const cfg = JSON.parse(decodeURIComponent(raw.slice(CHAIN_PROXY_PREFIX.length))) as ChainProxyConfig
    if (!cfg.frontProxyId || !cfg.landing?.server || !cfg.landing?.port) return null
    return cfg
  } catch { return null }
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

export function toChainImportForm(proxyName: string, proxyConfig: string): ChainImportForm | null {
  const cfg = parseChainProxyConfig(proxyConfig)
  if (!cfg) return null
  try {
    return {
      proxyName,
      localPort: cfg.localPort ? String(cfg.localPort) : '',
      frontProxyId: cfg.frontProxyId,
      landing: {
        protocol: cfg.landing.protocol,
        server: cfg.landing.server,
        port: String(cfg.landing.port),
        username: cfg.landing.username || '',
        password: cfg.landing.password || '',
      },
    }
  } catch { return null }
}


export function buildChainImportCandidate(form: ChainImportForm): ImportCandidate {
  const parseHop = (label: string, hop: ChainHopForm): ChainSocks5HopConfig => {
    const protocol = hop.protocol
    const server = hop.server.trim()
    if (!server) {
      throw new Error(`请输入${label}代理地址`)
    }
    if (/^[a-zA-Z][a-zA-Z0-9+.-]*:\/\//.test(server)) {
      throw new Error(`${label}代理地址只需要填写主机名或 IP，不需要协议头`)
    }

    const portInput = hop.port.trim()
    if (!portInput) {
      throw new Error(`请输入${label}代理端口`)
    }
    if (!/^\d+$/.test(portInput)) {
      throw new Error(`${label}代理端口必须为数字`)
    }

    const port = Number(portInput)
    if (!Number.isInteger(port) || port < 1 || port > 65535) {
      throw new Error(`${label}代理端口必须在 1-65535 之间`)
    }

    const username = hop.username.trim()
    const password = hop.password
    if (password && !username) {
      throw new Error(`${label}填写密码时请同时填写账号`)
    }

    return {
      protocol,
      server,
      port,
      username: username || undefined,
      password: password || undefined,
    }
  }

  const localPortInput = form.localPort.trim()
  if (localPortInput && !/^\d+$/.test(localPortInput)) {
    throw new Error('本地监听端口必须为数字')
  }
  const localPort = localPortInput ? Number(localPortInput) : 0
  if (localPortInput && (!Number.isInteger(localPort) || localPort < 1 || localPort > 65535)) {
    throw new Error('本地监听端口必须在 1-65535 之间')
  }

  const frontProxyId = form.frontProxyId.trim()
  if (!frontProxyId) throw new Error('请选择前置代理节点')
  const payload: ChainProxyConfig = {
    version: 2,
    frontProxyId,
    landing: parseHop('落地', form.landing),
    localPort: localPort > 0 ? localPort : undefined,
  }

  return {
    proxyName: form.proxyName.trim() || `链式代理-${payload.landing.server}`,
    proxyConfig: `${CHAIN_PROXY_PREFIX}${encodeURIComponent(JSON.stringify(payload))}`,
  }
}

function parseOptionalChainPort(raw: unknown, label: string): number | undefined {
  if (raw === undefined || raw === null || String(raw).trim() === '') {
    return undefined
  }
  const value = Number(raw)
  if (!Number.isInteger(value) || value < 1 || value > 65535) {
    throw new Error(`${label}必须在 1-65535 之间`)
  }
  return value
}

function parseChainQuickImportHop(raw: unknown, label: string): ChainSocks5HopConfig {
  if (!raw || typeof raw !== 'object') {
    throw new Error(`${label}缺少配置`)
  }

  const hop = raw as Record<string, unknown>
  const protocol = String(hop.protocol || 'socks5').trim().toLowerCase()
  if (protocol !== 'socks5' && protocol !== 'http' && protocol !== 'https') {
    throw new Error(`${label}仅支持 http / https / socks5`)
  }

  const server = String(hop.server || '').trim()
  if (!server) {
    throw new Error(`${label}缺少 server`)
  }

  const portValue = Number(hop.port)
  if (!Number.isInteger(portValue) || portValue < 1 || portValue > 65535) {
    throw new Error(`${label}缺少有效 port`)
  }

  const username = String(hop.username || '').trim()
  const password = hop.password === undefined || hop.password === null ? '' : String(hop.password)
  if (password && !username) {
    throw new Error(`${label}填写 password 时请同时填写 username`)
  }

  return {
    protocol: protocol as ChainSocks5HopConfig['protocol'],
    server,
    port: portValue,
    username: username || undefined,
    password: password || undefined,
  }
}

export function parseChainImportJSON(raw: string): { form: ChainImportForm; groupName: string } {
  const text = raw.trim()
  if (!text) {
    throw new Error('请输入链式代理 JSON')
  }

  let payload: Record<string, unknown>
  try {
    payload = JSON.parse(text) as Record<string, unknown>
  } catch {
    throw new Error('JSON 格式无效')
  }

  if (!payload || typeof payload !== 'object' || Array.isArray(payload)) {
    throw new Error('JSON 根节点必须是对象')
  }

  const frontProxyId = String(payload.frontProxyId || '').trim()
  if (!frontProxyId) throw new Error('缺少 frontProxyId')
  const landing = parseChainQuickImportHop(payload.landing, '落地')
  const localPort = parseOptionalChainPort(payload.localPort, 'localPort')
  const proxyName = String(payload.name ?? payload.proxyName ?? '').trim()
  const groupName = String(payload.group ?? payload.groupName ?? '').trim()

  return {
    form: {
      proxyName,
      localPort: localPort ? String(localPort) : '',
      frontProxyId,
      landing: {
        protocol: landing.protocol,
        server: landing.server,
        port: String(landing.port),
        username: landing.username || '',
        password: landing.password || '',
      },
    },
    groupName,
  }
}
