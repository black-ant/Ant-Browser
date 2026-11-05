import yaml from 'js-yaml'
import type { BrowserProxy } from '../types'
import { CHAIN_SOCKS5_PREFIX, type ChainImportForm, type ChainHopForm, type ChainSocks5Config, type ChainSocks5HopConfig, type ClashProxy, type DirectImportForm, type ImportCandidate, type ProxyDisplayInfo } from './ProxyImportModal.types'

function parseChainSocks5Config(proxyConfig: string): ChainSocks5Config | null {
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

function proxyToYaml(proxy: ClashProxy): string {
  return yaml.dump([proxy], { flowLevel: -1, lineWidth: -1 }).trim()
}

function quoteYamlScalar(value: string): string {
  const v = value.trim()
  if (!v) return "''"
  return `'${v.replace(/'/g, "''")}'`
}

function normalizeImportedProxyArray(payload: unknown): ClashProxy[] | null {
  const asArray = (input: unknown): ClashProxy[] => {
    if (!Array.isArray(input)) return []
    return input.filter((item): item is ClashProxy => !!item && typeof item === 'object')
  }

  if (Array.isArray(payload)) {
    return asArray(payload)
  }
  if (!payload || typeof payload !== 'object') {
    return null
  }

  const record = payload as Record<string, unknown>
  if (Array.isArray(record.proxies)) {
    return asArray(record.proxies)
  }
  if (Array.isArray(record.proxy)) {
    return asArray(record.proxy)
  }
  if (Array.isArray(record.Proxy)) {
    return asArray(record.Proxy)
  }
  return null
}

function normalizeLooseClashImportText(raw: string): string {
  const normalizedNewline = raw.replace(/﻿/g, '').replace(/\r\n/g, '\n').trim()
  if (!normalizedNewline) return normalizedNewline

  const lines = normalizedNewline.split('\n')
  const fixedLines = lines.map(line => {
    const m = line.match(/^(\s*)-\s*([^,{][^,]*?)\s*,\s*(type\s*:.*)$/i)
    if (!m) return line
    const indent = m[1] || ''
    const name = m[2] || ''
    const tail = m[3] || ''
    return `${indent}- { name: ${quoteYamlScalar(name)}, ${tail.trim()} }`
  })

  const hasProxiesRoot = fixedLines.some(line => /^\s*proxies\s*:/.test(line))
  if (hasProxiesRoot) {
    return fixedLines.join('\n')
  }

  const looksLikeProxyList = fixedLines.some(line => /^\s*-\s*/.test(line))
  if (!looksLikeProxyList) {
    return fixedLines.join('\n')
  }

  const indented = fixedLines.map(line => {
    if (!line.trim()) return line
    return `  ${line}`
  })
  return `proxies:\n${indented.join('\n')}`
}

export function parseClashImportText(raw: string): ClashProxy[] {
  const input = raw.trim()
  if (!input) {
    throw new Error('请输入 YAML 内容')
  }

  const attempts = [input]
  const normalized = normalizeLooseClashImportText(input)
  if (normalized && normalized !== input) {
    attempts.push(normalized)
  }

  let lastError: unknown = null
  for (const text of attempts) {
    try {
      const parsed = yaml.load(text)
      const proxies = normalizeImportedProxyArray(parsed)
      if (proxies) {
        return proxies
      }
    } catch (error) {
      lastError = error
    }
  }

  if (lastError && typeof lastError === 'object' && lastError !== null && 'message' in lastError) {
    throw new Error(String((lastError as { message?: string }).message || '解析失败'))
  }
  throw new Error('无效的 YAML 格式，需要包含 proxies 数组')
}

function normalizeDirectProxyConfig(raw: string): string {
  const trimmed = raw.trim()
  if (!trimmed) return ''
  if (/^socket:\/\//i.test(trimmed)) {
    return trimmed.replace(/^socket:\/\//i, 'socks5://')
  }
  if (/^socks:\/\//i.test(trimmed)) {
    return trimmed.replace(/^socks:\/\//i, 'socks5://')
  }
  return trimmed
}

function resolveDirectProxyName(rawName: string, scheme: string, server: string, port: number, index: number, prefix: string): string {
  const name = rawName.trim()
  const fallbackName = server
    ? `${scheme.toUpperCase()}-${server}${port > 0 ? `:${port}` : ''}`
    : `导入代理 ${index + 1}`
  const finalName = name || fallbackName
  return prefix ? `${prefix}-${finalName}` : finalName
}

function formatDirectProxyHost(raw: string): string {
  const host = raw.trim()
  if (!host) return ''
  if (host.startsWith('[') && host.endsWith(']')) {
    return host
  }
  return host.includes(':') ? `[${host}]` : host
}

export function buildDirectImportCandidate(form: DirectImportForm): ImportCandidate {
  const serverInput = form.server.trim()
  if (!serverInput) {
    throw new Error('请输入代理地址')
  }
  if (/^[a-zA-Z][a-zA-Z0-9+.-]*:\/\//.test(serverInput)) {
    throw new Error('代理地址只需要填写主机名或 IP，不需要协议头')
  }

  const portInput = form.port.trim()
  if (!portInput) {
    throw new Error('请输入代理端口')
  }
  if (!/^\d+$/.test(portInput)) {
    throw new Error('代理端口必须为数字')
  }

  const port = Number(portInput)
  if (port < 1 || port > 65535) {
    throw new Error('代理端口必须在 1-65535 之间')
  }

  const username = form.username.trim()
  const password = form.password
  if (password && !username) {
    throw new Error('填写密码时请同时填写账号')
  }

  const auth = username
    ? `${encodeURIComponent(username)}${password ? `:${encodeURIComponent(password)}` : ''}@`
    : ''
  const rawConfig = `${form.protocol}://${auth}${formatDirectProxyHost(serverInput)}:${port}`

  let parsedURL: URL
  try {
    parsedURL = new URL(rawConfig)
  } catch {
    throw new Error('请输入有效的代理地址')
  }

  if (!parsedURL.hostname) {
    throw new Error('请输入有效的代理地址')
  }

  const normalizedConfig = normalizeDirectProxyConfig(parsedURL.toString()).replace(/\/$/, '')
  const normalizedServer = parsedURL.hostname.replace(/^\[(.*)\]$/, '$1')

  return {
    proxyName: resolveDirectProxyName(form.proxyName, form.protocol, normalizedServer, port, 0, ''),
    proxyConfig: normalizedConfig,
  }
}

export function buildChainImportCandidate(form: ChainImportForm): ImportCandidate {
  const parseHop = (label: string, hop: ChainHopForm): ChainSocks5HopConfig => {
    const protocol = hop.protocol === 'socks5' ? 'socks5' : 'http'
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

  const payload: ChainSocks5Config = {
    first: parseHop('第一层', form.first),
    second: parseHop('第二层', form.second),
    localPort: localPort > 0 ? localPort : undefined,
  }

  const encodedPayload = encodeURIComponent(JSON.stringify(payload))
  const proxyConfig = `${CHAIN_SOCKS5_PREFIX}${encodedPayload}`

  return {
    proxyName: form.proxyName.trim() || `链式代理-${payload.first.server}-${payload.second.server}`,
    proxyConfig,
  }
}

function resolveImportedProxyName(proxy: ClashProxy, index: number, prefix: string): string {
  const rawName = (proxy.name || '').trim() || `导入代理 ${index + 1}`
  return prefix ? `${prefix}-${rawName}` : rawName
}

export function buildImportCandidatesFromClash(parsedProxies: ClashProxy[], prefix: string): ImportCandidate[] {
  return parsedProxies.map((proxy, index) => ({
    proxyName: resolveImportedProxyName(proxy, index, prefix),
    proxyConfig: proxyToYaml(proxy),
  }))
}

export function buildImportPreview(candidates: ImportCandidate[], groupName: string): ProxyDisplayInfo[] {
  return candidates.map((candidate, index) => {
    const info = parseProxyInfo(candidate.proxyConfig)
    return {
      proxyId: `preview-${index}`,
      proxyName: candidate.proxyName,
      proxyConfig: candidate.proxyConfig,
      groupName: candidate.groupName || groupName,
      type: info.type || '-',
      server: info.server || '-',
      port: info.port || 0,
    }
  })
}

export function normalizeRefreshIntervalM(value: number): number {
  if (!Number.isFinite(value)) return 0
  if (value <= 0) return 0
  if (value < 5) return 5
  if (value > 24 * 60) return 24 * 60
  return Math.round(value)
}

function normalizeSourceURL(sourceURL: string): string {
  const raw = (sourceURL || '').trim()
  if (!raw) return ''
  try {
    const parsed = new URL(raw)
    parsed.hash = ''
    return parsed.toString()
  } catch {
    return raw
  }
}

function buildStableSourceID(sourceURL: string, sourceNamePrefix: string): string {
  const key = `${normalizeSourceURL(sourceURL)}|||${sourceNamePrefix.trim()}`
  let hash = 5381
  for (let i = 0; i < key.length; i += 1) {
    hash = ((hash << 5) + hash) ^ key.charCodeAt(i)
  }
  const unsigned = hash >>> 0
  return `src-${unsigned.toString(36)}`
}

export function resolveImportSourceID(list: BrowserProxy[], sourceURL: string, sourceNamePrefix: string): string {
  const normalizedURL = normalizeSourceURL(sourceURL)
  const normalizedPrefix = sourceNamePrefix.trim()
  const existing = list.find(item =>
    normalizeSourceURL(item.sourceUrl || '') === normalizedURL &&
    (item.sourceNamePrefix || '').trim() === normalizedPrefix &&
    (item.sourceId || '').trim() !== ''
  )
  if (existing?.sourceId?.trim()) {
    return existing.sourceId.trim()
  }
  return buildStableSourceID(sourceURL, sourceNamePrefix)
}

export function nextProxyID(): string {
  return `proxy-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
}

export function createExistingProxyPicker(oldSourceProxies: BrowserProxy[]) {
  const exactMap = new Map<string, BrowserProxy[]>()
  const nameMap = new Map<string, BrowserProxy[]>()
  oldSourceProxies.forEach(item => {
    const exactKey = `${item.proxyName}|||${item.proxyConfig}`
    const exactList = exactMap.get(exactKey) || []
    exactList.push(item)
    exactMap.set(exactKey, exactList)

    const nameKey = item.proxyName
    const nameList = nameMap.get(nameKey) || []
    nameList.push(item)
    nameMap.set(nameKey, nameList)
  })

  return (name: string, configText: string): BrowserProxy | null => {
    const exactKey = `${name}|||${configText}`
    const exactList = exactMap.get(exactKey)
    if (exactList && exactList.length > 0) {
      const item = exactList.shift()
      if (item?.proxyId) return item
    }

    const nameList = nameMap.get(name)
    if (nameList && nameList.length > 0) {
      const item = nameList.shift()
      if (item?.proxyId) return item
    }
    return null
  }
}

export function createExistingProxyIDPicker(oldSourceProxies: BrowserProxy[]) {
  const pickExisting = createExistingProxyPicker(oldSourceProxies)
  return (name: string, configText: string): string | null => pickExisting(name, configText)?.proxyId || null
}
