import type { ChainHopForm, ChainImportForm, ChainSocks5Config, ChainSocks5HopConfig, ImportCandidate } from './helpers.types'
import { CHAIN_SOCKS5_PREFIX } from './helpers.types'

function detectChainNodeProtocol(proxyConfig: string): ChainImportForm['firstNodeProtocol'] {
  const scheme = proxyConfig.trim().match(/^([a-zA-Z0-9+.-]+):\/\//)?.[1]?.toLowerCase()
  if (scheme === 'hysteria2' || scheme === 'hy2') return 'hysteria2'
  return scheme === 'vmess' || scheme === 'trojan' || scheme === 'ss' ? scheme : 'vless'
}
function normalizeChainHop(raw: unknown, allowProxyConfig: boolean): ChainSocks5HopConfig | null {
  if (!raw || typeof raw !== 'object') return null
  const hop = raw as Record<string, unknown>
  const proxyConfig = String(hop.proxyConfig || '').trim()
  if (proxyConfig) {
    return allowProxyConfig ? { proxyConfig } : null
  }

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

export function parseChainSocks5Config(proxyConfig: string): ChainSocks5Config | null {
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
  const cfg = parseChainSocks5Config(proxyConfig)
  if (!cfg) return null

  return {
    proxyName,
    localPort: cfg.localPort ? String(cfg.localPort) : '',
    firstMode: cfg.first.proxyConfig ? 'node' : 'standard',
    firstNodeProtocol: detectChainNodeProtocol(cfg.first.proxyConfig || ''),
    firstNodeSource: 'manual',
    firstProxyConfig: cfg.first.proxyConfig || '',
    first: {
      protocol: cfg.first.protocol || 'http',
      server: cfg.first.server || '',
      port: cfg.first.port ? String(cfg.first.port) : '',
      username: cfg.first.username || '',
      password: cfg.first.password || '',
    },
    second: {
      protocol: cfg.second.protocol || 'socks5',
      server: cfg.second.server || '',
      port: cfg.second.port ? String(cfg.second.port) : '',
      username: cfg.second.username || '',
      password: cfg.second.password || '',
    },
  }
}

function parseStandardHop(label: string, hop: ChainHopForm): ChainSocks5HopConfig {
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
  if (!Number.isInteger(port) || port < 1 || port > 65535) {
    throw new Error(`${label}代理端口必须在 1-65535 之间`)
  }

  const username = hop.username.trim()
  const password = hop.password
  if (password && !username) throw new Error(`${label}填写密码时请同时填写账号`)

  return {
    protocol,
    server,
    port,
    username: username || undefined,
    password: password || undefined,
  }
}

export function buildChainImportCandidate(form: ChainImportForm): ImportCandidate {
  const localPortInput = form.localPort.trim()
  if (localPortInput && !/^\d+$/.test(localPortInput)) {
    throw new Error('本地监听端口必须为数字')
  }
  const localPort = localPortInput ? Number(localPortInput) : 0
  if (localPortInput && (!Number.isInteger(localPort) || localPort < 1 || localPort > 65535)) {
    throw new Error('本地监听端口必须在 1-65535 之间')
  }

  const first: ChainSocks5HopConfig = form.firstMode === 'node'
    ? (() => {
        const proxyConfig = form.firstProxyConfig.trim()
        if (!proxyConfig) throw new Error('请输入第一层机场节点配置')
        if (/^chain\+socks5:\/\//i.test(proxyConfig)) throw new Error('第一层不支持嵌套链式代理')
        return { proxyConfig }
      })()
    : parseStandardHop('第一层', form.first)

  const second = parseStandardHop('第二层', form.second)
  const payload: ChainSocks5Config = {
    first,
    second,
    localPort: localPort > 0 ? localPort : undefined,
  }
  const firstLabel = first.proxyConfig
    ? (first.proxyConfig.match(/^([a-zA-Z0-9+.-]+):\/\//)?.[1]?.toUpperCase() || 'NODE')
    : first.server

  return {
    proxyName: form.proxyName.trim() || `链式代理-${firstLabel}-${second.server}`,
    proxyConfig: `${CHAIN_SOCKS5_PREFIX}${encodeURIComponent(JSON.stringify(payload))}`,
  }
}

function parseOptionalChainPort(raw: unknown, label: string): number | undefined {
  if (raw === undefined || raw === null || String(raw).trim() === '') return undefined
  const value = Number(raw)
  if (!Number.isInteger(value) || value < 1 || value > 65535) {
    throw new Error(`${label}必须在 1-65535 之间`)
  }
  return value
}

function parseChainQuickImportHop(raw: unknown, label: string, allowProxyConfig = false): ChainSocks5HopConfig {
  if (!raw || typeof raw !== 'object') throw new Error(`${label}缺少配置`)

  const hop = raw as Record<string, unknown>
  const proxyConfig = String(hop.proxyConfig || '').trim()
  if (proxyConfig) {
    if (!allowProxyConfig) throw new Error(`${label}不支持高级节点配置`)
    if (/^chain\+socks5:\/\//i.test(proxyConfig)) throw new Error(`${label}不支持嵌套链式代理`)
    return { proxyConfig }
  }

  const protocol = String(hop.protocol || 'socks5').trim().toLowerCase()
  if (protocol !== 'socks5' && protocol !== 'http') {
    throw new Error(`${label}仅支持 http / socks5`)
  }

  const server = String(hop.server || '').trim()
  if (!server) throw new Error(`${label}缺少 server`)

  const portValue = Number(hop.port)
  if (!Number.isInteger(portValue) || portValue < 1 || portValue > 65535) {
    throw new Error(`${label}缺少有效 port`)
  }

  const username = String(hop.username || '').trim()
  const password = hop.password === undefined || hop.password === null ? '' : String(hop.password)
  if (password && !username) throw new Error(`${label}填写 password 时请同时填写 username`)

  return {
    protocol: protocol === 'http' ? 'http' : 'socks5',
    server,
    port: portValue,
    username: username || undefined,
    password: password || undefined,
  }
}

export function parseChainImportJSON(raw: string): { form: ChainImportForm; groupName: string } {
  const text = raw.trim()
  if (!text) throw new Error('请输入链式代理 JSON')

  let payload: Record<string, unknown>
  try {
    payload = JSON.parse(text) as Record<string, unknown>
  } catch {
    throw new Error('JSON 格式无效')
  }

  if (!payload || typeof payload !== 'object' || Array.isArray(payload)) {
    throw new Error('JSON 根节点必须是对象')
  }

  const first = parseChainQuickImportHop(payload.first, '第一层', true)
  const second = parseChainQuickImportHop(payload.second, '第二层')
  const localPort = parseOptionalChainPort(payload.localPort, 'localPort')
  const proxyName = String(payload.name ?? payload.proxyName ?? '').trim()
  const groupName = String(payload.group ?? payload.groupName ?? '').trim()

  return {
    form: {
      proxyName,
      localPort: localPort ? String(localPort) : '',
      firstMode: first.proxyConfig ? 'node' : 'standard',
      firstNodeProtocol: detectChainNodeProtocol(first.proxyConfig || ''),
      firstNodeSource: 'manual',
      firstProxyConfig: first.proxyConfig || '',
      first: {
        protocol: first.protocol || 'http',
        server: first.server || '',
        port: first.port ? String(first.port) : '',
        username: first.username || '',
        password: first.password || '',
      },
      second: {
        protocol: second.protocol || 'socks5',
        server: second.server || '',
        port: second.port ? String(second.port) : '',
        username: second.username || '',
        password: second.password || '',
      },
    },
    groupName,
  }
}
