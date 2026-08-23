import yaml from 'js-yaml'
import type { BrowserProxy } from '../../types'
import type { ClashProxy, ImportCandidate, ProxyDisplayInfo } from './helpers.types'
import { BUILTIN_PROXIES } from './helpers.types'
import { parseChainProxyConfig, parseChainSocks5Config } from './helpers.chain'

export function ensureBuiltinProxies(proxies: BrowserProxy[]): BrowserProxy[] {
  const result = [...proxies]
  for (const builtin of BUILTIN_PROXIES) {
    if (!result.find((proxy) => proxy.proxyId === builtin.proxyId)) {
      result.unshift(builtin)
    }
  }
  return result
}

export function parseProxyInfo(proxyConfig: string): { type: string; server: string; port: number } {
  const cfg = proxyConfig.trim()
  if (cfg === 'direct://') return { type: 'direct', server: '-', port: 0 }

  const chain = parseChainSocks5Config(cfg)
  if (chain) {
    const firstHop = `${chain.first.server}:${chain.first.port}`
    const secondHop = `${chain.second.server}:${chain.second.port}`
    return { type: '链式', server: `${firstHop} → ${secondHop}`, port: chain.second.port }
  }

  const genericChain = parseChainProxyConfig(cfg)
  if (genericChain) {
    return {
      type: '链式',
      server: `${genericChain.frontProxyId} → ${genericChain.landing.server}:${genericChain.landing.port}`,
      port: genericChain.localPort || 0,
    }
  }

  const urlMatch = cfg.match(/^([a-zA-Z0-9+\-]+):\/\//)
  if (urlMatch) {
    const scheme = urlMatch[1].toLowerCase()
    try {
      const parsed = new URL(cfg)
      return { type: scheme, server: parsed.hostname, port: parseInt(parsed.port, 10) || 0 }
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

export function toDisplayList(proxies: BrowserProxy[]): ProxyDisplayInfo[] {
  return proxies.map((proxy) => {
    const info = parseProxyInfo(proxy.proxyConfig)
    return {
      proxyId: proxy.proxyId,
      proxyName: proxy.proxyName,
      proxyConfig: proxy.proxyConfig,
      preferredKernel: proxy.preferredKernel || 'auto',
      groupName: proxy.groupName || '',
      sourceId: proxy.sourceId || '',
      sourceUrl: proxy.sourceUrl || '',
      sourceAutoRefresh: !!proxy.sourceAutoRefresh,
      sourceRefreshIntervalM: Math.max(0, Number(proxy.sourceRefreshIntervalM || 0)),
      sourceLastRefreshAt: proxy.sourceLastRefreshAt || '',
      ...info,
    }
  })
}

export function proxyToYaml(proxy: ClashProxy): string {
  return yaml.dump([proxy], { flowLevel: -1, lineWidth: -1 }).trim()
}

function quoteYamlScalar(value: string): string {
  const trimmed = value.trim()
  if (!trimmed) return "''"
  return `'${trimmed.replace(/'/g, "''")}'`
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
  const normalizedNewline = raw.replace(/\uFEFF/g, '').replace(/\r\n/g, '\n').trim()
  if (!normalizedNewline) return normalizedNewline

  const fixedLines = normalizedNewline.split('\n').map((line) => {
    const match = line.match(/^(\s*)-\s*([^,{][^,]*?)\s*,\s*(type\s*:.*)$/i)
    if (!match) return line
    const indent = match[1] || ''
    const name = match[2] || ''
    const tail = match[3] || ''
    return `${indent}- { name: ${quoteYamlScalar(name)}, ${tail.trim()} }`
  })

  const hasProxiesRoot = fixedLines.some((line) => /^\s*proxies\s*:/.test(line))
  if (hasProxiesRoot) {
    return fixedLines.join('\n')
  }

  const looksLikeProxyList = fixedLines.some((line) => /^\s*-\s*/.test(line))
  if (!looksLikeProxyList) {
    return fixedLines.join('\n')
  }

  const indented = fixedLines.map((line) => (line.trim() ? `  ${line}` : line))
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
      preferredKernel: 'auto',
      groupName: candidate.groupName || groupName,
      sourceId: '',
      sourceUrl: '',
      sourceAutoRefresh: false,
      sourceRefreshIntervalM: 0,
      sourceLastRefreshAt: '',
      type: info.type || '-',
      server: info.server || '-',
      port: info.port || 0,
    }
  })
}

export function nextProxyID(): string {
  return `proxy-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
}

export function resolveImportedProxyName(proxy: ClashProxy, index: number, prefix: string): string {
  const rawName = (proxy.name || '').trim() || `导入代理 ${index + 1}`
  return prefix ? `${prefix}-${rawName}` : rawName
}
