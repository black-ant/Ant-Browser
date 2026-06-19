import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Badge, Button, Card, ConfirmModal, FormItem, Input, Modal, Select, Switch, Table, Textarea, toast } from '../../../shared/components'
import type { SortOrder, TableColumn } from '../../../shared/components/Table'
import type { BrowserProxy, ProxyIPHealthResult, ProxySourceOverride } from '../types'
import { fetchBrowserProxies, fetchBrowserProxyGroups, saveBrowserProxies, browserProxyTestSpeed, browserProxyBatchTestSpeed, browserProxyCheckIPHealth, browserProxyBatchCheckIPHealth, fetchClashImportFromURL, fetchProxySources, fetchProxySourceOverrides, upsertProxySource, refreshProxySource, refreshAllProxySources, setProxySourceOverride, cancelProxyBatch } from '../api'
import { computeProxyScore, proxyScoreVariant } from '../proxyScore'
import { BUILTIN_PROXIES, BUILTIN_PROXY_IDS } from '../config/builtinProxies'
import { parseChainSocks5Config, parseProxyInfo, CHAIN_SOCKS5_PREFIX, type ClashProxy, type ChainSocks5Config, type ChainSocks5HopConfig } from '../utils/proxyParse'
import { EventsOn } from '../../../wailsjs/runtime/runtime'
import yaml from 'js-yaml'

// 内置代理 ID，不可删除、不可编辑
const PROXY_LATENCY_CACHE_KEY = 'browser:proxyPool:latencyMap:v1'
const PROXY_IP_HEALTH_CACHE_KEY = 'browser:proxyPool:ipHealthMap:v1'
const PROXY_SOURCE_IGNORED_NAMES_KEY = 'browser:proxyPool:sourceIgnoredProxyNames:v1'
const PROXY_GLOBAL_AUTO_REFRESH_KEY = 'browser:proxyPool:globalAutoRefreshEnabled:v1'
const PROXY_GLOBAL_REFRESH_INTERVAL_KEY = 'browser:proxyPool:globalRefreshIntervalM:v1'
const PROXY_LATENCY_CACHE_TTL_MS = 12 * 60 * 60 * 1000
const PROXY_IP_HEALTH_CACHE_TTL_MS = 12 * 60 * 60 * 1000
const SPEED_RESULT_EVENT = 'proxy:speed:result'
const IP_HEALTH_RESULT_EVENT = 'proxy:iphealth:result'
const PROXY_SPEED_TEST_CONCURRENCY = 20
const PROXY_IP_HEALTH_TEST_CONCURRENCY = 10

function ensureBuiltinProxies(proxies: BrowserProxy[]): BrowserProxy[] {
  const result = [...proxies]
  for (const builtin of BUILTIN_PROXIES) {
    if (!result.find(p => p.proxyId === builtin.proxyId)) {
      result.unshift(builtin)
    }
  }
  return result
}

type ProxyImportMode = 'clash' | 'direct' | 'chain'

interface DirectImportForm {
  proxyName: string
  protocol: 'http' | 'https' | 'socks5'
  server: string
  port: string
  username: string
  password: string
}

interface ChainHopForm {
  server: string
  port: string
  username: string
  password: string
}

interface ChainImportForm {
  proxyName: string
  localPort: string
  first: ChainHopForm
  second: ChainHopForm
}

const DIRECT_PROXY_PROTOCOL_OPTIONS = [
  { value: 'http', label: 'HTTP' },
  { value: 'https', label: 'HTTPS' },
  { value: 'socks5', label: 'SOCKS5' },
] as const

const INITIAL_DIRECT_IMPORT_FORM: DirectImportForm = {
  proxyName: '',
  protocol: 'http',
  server: '',
  port: '',
  username: '',
  password: '',
}

const INITIAL_CHAIN_IMPORT_FORM: ChainImportForm = {
  proxyName: '',
  localPort: '',
  first: {
    server: '',
    port: '',
    username: '',
    password: '',
  },
  second: {
    server: '',
    port: '',
    username: '',
    password: '',
  },
}

interface ImportCandidate {
  proxyName: string
  proxyConfig: string
}

interface ProxyDisplayInfo {
  proxyId: string
  proxyName: string
  proxyConfig: string
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

interface URLImportSourceMeta {
  sourceId: string
  sourceUrl: string
  sourceNamePrefix: string
  sourceGroupName: string
  sourceDnsServers: string
  sourceAutoRefresh: boolean
  sourceRefreshIntervalM: number
  sourceLastRefreshAt: string
}

function toChainImportForm(proxyName: string, cfg: ChainSocks5Config): ChainImportForm {
  return {
    proxyName,
    localPort: cfg.localPort ? String(cfg.localPort) : '',
    first: {
      server: cfg.first.server,
      port: String(cfg.first.port),
      username: cfg.first.username || '',
      password: cfg.first.password || '',
    },
    second: {
      server: cfg.second.server,
      port: String(cfg.second.port),
      username: cfg.second.username || '',
      password: cfg.second.password || '',
    },
  }
}

function toDisplayList(proxies: BrowserProxy[]): ProxyDisplayInfo[] {
  return proxies.map(p => {
    const info = parseProxyInfo(p.proxyConfig)
    return {
      proxyId: p.proxyId,
      proxyName: p.proxyName,
      proxyConfig: p.proxyConfig,
      groupName: p.groupName || '',
      sourceId: p.sourceId || '',
      sourceUrl: p.sourceUrl || '',
      sourceAutoRefresh: !!p.sourceAutoRefresh,
      sourceRefreshIntervalM: Math.max(0, Number(p.sourceRefreshIntervalM || 0)),
      sourceLastRefreshAt: p.sourceLastRefreshAt || '',
      ...info,
    }
  })
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
  const normalizedNewline = raw.replace(/\uFEFF/g, '').replace(/\r\n/g, '\n').trim()
  if (!normalizedNewline) return normalizedNewline

  const lines = normalizedNewline.split('\n')
  const fixedLines = lines.map(line => {
    // 容错: -节点名, type: vless, server: ... => - { name: '节点名', type: vless, server: ... }
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

function parseClashImportText(raw: string): ClashProxy[] {
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

function buildDirectImportCandidate(form: DirectImportForm): ImportCandidate {
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

function buildChainImportCandidate(form: ChainImportForm): ImportCandidate {
  const parseHop = (label: string, hop: ChainHopForm): ChainSocks5HopConfig => {
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
      protocol: 'socks5',
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


function buildImportCandidatesFromClash(parsedProxies: ClashProxy[], prefix: string): ImportCandidate[] {
  return parsedProxies.map((proxy, index) => ({
    proxyName: resolveImportedProxyName(proxy, index, prefix),
    proxyConfig: proxyToYaml(proxy),
  }))
}

function buildImportPreview(candidates: ImportCandidate[], groupName: string): ProxyDisplayInfo[] {
  return candidates.map((candidate, index) => {
    const info = parseProxyInfo(candidate.proxyConfig)
    return {
      proxyId: `preview-${index}`,
      proxyName: candidate.proxyName,
      proxyConfig: candidate.proxyConfig,
      groupName,
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

function parseTimestampMs(value: string): number {
  const v = (value || '').trim()
  if (!v) return 0
  const t = Date.parse(v)
  return Number.isFinite(t) ? t : 0
}

function normalizeRefreshIntervalM(value: number): number {
  if (!Number.isFinite(value)) return 0
  if (value <= 0) return 0
  if (value < 5) return 5
  if (value > 24 * 60) return 24 * 60
  return Math.round(value)
}

function sourceHostLabel(sourceURL: string): string {
  const raw = (sourceURL || '').trim()
  if (!raw) return ''
  try {
    const u = new URL(raw)
    return u.host || raw
  } catch {
    return raw
  }
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
  // djb2 变体，输出稳定且实现简单。
  let hash = 5381
  for (let i = 0; i < key.length; i += 1) {
    hash = ((hash << 5) + hash) ^ key.charCodeAt(i)
  }
  const unsigned = hash >>> 0
  return `src-${unsigned.toString(36)}`
}

function resolveImportSourceID(list: BrowserProxy[], sourceURL: string, sourceNamePrefix: string): string {
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

function collectURLImportSources(list: BrowserProxy[]): URLImportSourceMeta[] {
  const sourceMap = new Map<string, URLImportSourceMeta>()
  for (const item of list) {
    const sourceId = (item.sourceId || '').trim()
    const sourceUrl = (item.sourceUrl || '').trim()
    if (!sourceId || !sourceUrl) continue

    const last = sourceMap.get(sourceId)
    const currentLastRefreshAt = item.sourceLastRefreshAt || ''
    if (!last) {
      sourceMap.set(sourceId, {
        sourceId,
        sourceUrl,
        sourceNamePrefix: (item.sourceNamePrefix || '').trim(),
        sourceGroupName: (item.groupName || '').trim(),
        sourceDnsServers: (item.dnsServers || '').trim(),
        sourceAutoRefresh: !!item.sourceAutoRefresh,
        sourceRefreshIntervalM: normalizeRefreshIntervalM(Number(item.sourceRefreshIntervalM || 0)),
        sourceLastRefreshAt: currentLastRefreshAt,
      })
      continue
    }

    if (
      parseTimestampMs(currentLastRefreshAt) > parseTimestampMs(last.sourceLastRefreshAt) &&
      currentLastRefreshAt.trim()
    ) {
      last.sourceLastRefreshAt = currentLastRefreshAt
    }
  }
  return Array.from(sourceMap.values())
}

function nextProxyID(): string {
  return `proxy-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
}

function resolveImportedProxyName(proxy: ClashProxy, index: number, prefix: string): string {
  const rawName = (proxy.name || '').trim() || `导入代理 ${index + 1}`
  return prefix ? `${prefix}-${rawName}` : rawName
}

function createExistingProxyIDPicker(oldSourceProxies: BrowserProxy[]) {
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

  return (name: string, configText: string): string | null => {
    const exactKey = `${name}|||${configText}`
    const exactList = exactMap.get(exactKey)
    if (exactList && exactList.length > 0) {
      const item = exactList.shift()
      if (item?.proxyId) return item.proxyId
    }

    const nameList = nameMap.get(name)
    if (nameList && nameList.length > 0) {
      const item = nameList.shift()
      if (item?.proxyId) return item.proxyId
    }
    return null
  }
}

function readSourceIgnoredProxyNames(): Record<string, string[]> {
  try {
    const raw = localStorage.getItem(PROXY_SOURCE_IGNORED_NAMES_KEY)
    if (!raw) return {}
    const parsed = JSON.parse(raw)
    if (!parsed || typeof parsed !== 'object') return {}
    const cleaned: Record<string, string[]> = {}
    Object.entries(parsed as Record<string, unknown>).forEach(([sourceId, value]) => {
      if (!sourceId.trim() || !Array.isArray(value)) return
      const names = value
        .map(item => (typeof item === 'string' ? item.trim() : ''))
        .filter(Boolean)
      if (names.length > 0) {
        cleaned[sourceId] = names
      }
    })
    return cleaned
  } catch {
    return {}
  }
}

function writeSourceIgnoredProxyNames(data: Record<string, string[]>) {
  try {
    const cleaned: Record<string, string[]> = {}
    Object.entries(data).forEach(([sourceId, names]) => {
      const key = sourceId.trim()
      if (!key || !Array.isArray(names)) return
      const validNames = names.map(name => (name || '').trim()).filter(Boolean)
      if (validNames.length > 0) {
        cleaned[key] = validNames
      }
    })
    localStorage.setItem(PROXY_SOURCE_IGNORED_NAMES_KEY, JSON.stringify(cleaned))
  } catch {
    // ignore write failures
  }
}

function appendSourceIgnoredProxyNames(sourceId: string, names: string[]) {
  const sourceKey = sourceId.trim()
  if (!sourceKey || names.length === 0) return
  const cleaned = names.map(name => name.trim()).filter(Boolean)
  if (cleaned.length === 0) return

  const existing = readSourceIgnoredProxyNames()
  existing[sourceKey] = [...(existing[sourceKey] || []), ...cleaned]
  writeSourceIgnoredProxyNames(existing)
}

function readGlobalRefreshConfig(): { enabled: boolean; intervalM: number } {
  try {
    const rawEnabled = localStorage.getItem(PROXY_GLOBAL_AUTO_REFRESH_KEY)
    const rawInterval = localStorage.getItem(PROXY_GLOBAL_REFRESH_INTERVAL_KEY)
    const enabled = rawEnabled === '1'
    const interval = normalizeRefreshIntervalM(Number(rawInterval || 0))
    return {
      enabled,
      intervalM: interval > 0 ? interval : 60,
    }
  } catch {
    return { enabled: false, intervalM: 60 }
  }
}

function writeGlobalRefreshConfig(enabled: boolean, intervalM: number) {
  try {
    localStorage.setItem(PROXY_GLOBAL_AUTO_REFRESH_KEY, enabled ? '1' : '0')
    localStorage.setItem(PROXY_GLOBAL_REFRESH_INTERVAL_KEY, String(intervalM))
  } catch {
    // ignore write failures
  }
}

function toLatencyValue(ok: boolean, latencyMs: number, error?: string): number {
  if (ok) return latencyMs
  return error?.includes('不支持') ? -3 : -2
}

function readLatencyCache(): Record<string, number> {
  try {
    const raw = localStorage.getItem(PROXY_LATENCY_CACHE_KEY)
    if (!raw) return {}
    const parsed = JSON.parse(raw) as { timestamp?: number; data?: Record<string, number> }
    if (!parsed?.timestamp || !parsed?.data) return {}
    if (Date.now() - parsed.timestamp > PROXY_LATENCY_CACHE_TTL_MS) return {}
    const cleaned: Record<string, number> = {}
    Object.entries(parsed.data).forEach(([proxyId, latency]) => {
      if (typeof latency === 'number' && Number.isFinite(latency) && latency !== -1) {
        cleaned[proxyId] = latency
      }
    })
    return cleaned
  } catch {
    return {}
  }
}

function writeLatencyCache(data: Record<string, number>) {
  try {
    const cleaned: Record<string, number> = {}
    Object.entries(data).forEach(([proxyId, latency]) => {
      if (typeof latency === 'number' && Number.isFinite(latency) && latency !== -1) {
        cleaned[proxyId] = latency
      }
    })
    localStorage.setItem(PROXY_LATENCY_CACHE_KEY, JSON.stringify({
      timestamp: Date.now(),
      data: cleaned,
    }))
  } catch {
    // ignore write failures
  }
}

function readIPHealthCache(): Record<string, ProxyIPHealthResult> {
  try {
    const raw = localStorage.getItem(PROXY_IP_HEALTH_CACHE_KEY)
    if (!raw) return {}
    const parsed = JSON.parse(raw) as { timestamp?: number; data?: Record<string, ProxyIPHealthResult> }
    if (!parsed?.timestamp || !parsed?.data) return {}
    if (Date.now() - parsed.timestamp > PROXY_IP_HEALTH_CACHE_TTL_MS) return {}
    const cleaned: Record<string, ProxyIPHealthResult> = {}
    Object.entries(parsed.data).forEach(([proxyId, item]) => {
      if (item && typeof item === 'object') {
        cleaned[proxyId] = item
      }
    })
    return cleaned
  } catch {
    return {}
  }
}

function writeIPHealthCache(data: Record<string, ProxyIPHealthResult>) {
  try {
    localStorage.setItem(PROXY_IP_HEALTH_CACHE_KEY, JSON.stringify({
      timestamp: Date.now(),
      data,
    }))
  } catch {
    // ignore write failures
  }
}

// deriveSourceNodeKey 推导某代理在订阅源中的稳定 nodeKey（= 订阅原始节点名）。
// 优先复用已有 rename 覆盖（customName 命中当前名）的 nodeKey；否则从当前名剥离前缀。
function deriveSourceNodeKey(proxy: BrowserProxy, overrides: ProxySourceOverride[]): string {
  const byCustom = overrides.find(o => o.action === 'rename' && o.customName === proxy.proxyName)
  if (byCustom) return byCustom.nodeKey
  const prefix = (proxy.sourceNamePrefix || '').trim()
  if (prefix && proxy.proxyName.startsWith(`${prefix}-`)) {
    return proxy.proxyName.slice(prefix.length + 1)
  }
  return proxy.proxyName
}

export function ProxyPoolPage() {
  const [proxies, setProxies] = useState<BrowserProxy[]>([])
  const [displayList, setDisplayList] = useState<ProxyDisplayInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [groups, setGroups] = useState<string[]>([])

  const [filterProtocol, setFilterProtocol] = useState<string>('all')
  const [filterKeyword, setFilterKeyword] = useState('')
  const [filterGroup, setFilterGroup] = useState<string>('all')
  const [sortColumn, setSortColumn] = useState<string>('') // 默认不排序
  const [sortOrder, setSortOrder] = useState<SortOrder>(undefined)

  const [latencyMap, setLatencyMap] = useState<Record<string, number>>({})
  const [testingAll, setTestingAll] = useState(false)
  const [ipHealthMap, setIPHealthMap] = useState<Record<string, ProxyIPHealthResult>>({})
  const [checkingIPHealthIds, setCheckingIPHealthIds] = useState<Set<string>>(new Set())
  const [checkingAllIPHealth, setCheckingAllIPHealth] = useState(false)

  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [batchDeleteConfirmOpen, setBatchDeleteConfirmOpen] = useState(false)

  const [importModalOpen, setImportModalOpen] = useState(false)
  const [importMode, setImportMode] = useState<ProxyImportMode>('clash')
  const [importUrl, setImportUrl] = useState('')
  const [importResolvedUrl, setImportResolvedUrl] = useState('')
  const [importText, setImportText] = useState('')
  const [importDnsServers, setImportDnsServers] = useState('')
  const [importNamePrefix, setImportNamePrefix] = useState('')
  const [importGroupName, setImportGroupName] = useState('')
  const [directImportForm, setDirectImportForm] = useState<DirectImportForm>(() => ({ ...INITIAL_DIRECT_IMPORT_FORM }))
  const [chainImportForm, setChainImportForm] = useState<ChainImportForm>(() => ({ ...INITIAL_CHAIN_IMPORT_FORM }))
  const [chainEditMode, setChainEditMode] = useState(false)
  const [previewModalOpen, setPreviewModalOpen] = useState(false)
  const [previewList, setPreviewList] = useState<ProxyDisplayInfo[]>([])
  const [removedPreviewProxyNames, setRemovedPreviewProxyNames] = useState<string[]>([])
  const [importing, setImporting] = useState(false)
  const [fetchingImportUrl, setFetchingImportUrl] = useState(false)
  const [refreshingAllSources, setRefreshingAllSources] = useState(false)
  const [refreshingSourceIds, setRefreshingSourceIds] = useState<Set<string>>(new Set())
  const [globalAutoRefreshEnabled, setGlobalAutoRefreshEnabled] = useState(false)
  const [globalRefreshIntervalM, setGlobalRefreshIntervalM] = useState('60')

  const [editModalOpen, setEditModalOpen] = useState(false)
  const [editingProxy, setEditingProxy] = useState<BrowserProxy | null>(null)
  const [editForm, setEditForm] = useState({ proxyName: '', proxyConfig: '', dnsServers: '', groupName: '' })
  const [saving, setSaving] = useState(false)

  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false)
  const [deletingId, setDeletingId] = useState<string | null>(null)
  const [ipHealthDetailOpen, setIPHealthDetailOpen] = useState(false)
  const [currentIPHealthDetail, setCurrentIPHealthDetail] = useState<ProxyIPHealthResult | null>(null)
  const proxiesRef = useRef<BrowserProxy[]>([])
  const refreshingSourceIdsRef = useRef<Set<string>>(new Set())
  const globalRefreshInterval = useMemo(() => {
    const interval = normalizeRefreshIntervalM(Number(globalRefreshIntervalM || 0))
    return interval > 0 ? interval : 60
  }, [globalRefreshIntervalM])

  useEffect(() => {
    const cfg = readGlobalRefreshConfig()
    setGlobalAutoRefreshEnabled(cfg.enabled)
    setGlobalRefreshIntervalM(String(cfg.intervalM))
    setLatencyMap(readLatencyCache())
    setIPHealthMap(readIPHealthCache())
    loadProxies()
  }, [])

  useEffect(() => {
    writeLatencyCache(latencyMap)
  }, [latencyMap])

  useEffect(() => {
    writeIPHealthCache(ipHealthMap)
  }, [ipHealthMap])

  useEffect(() => {
    writeGlobalRefreshConfig(globalAutoRefreshEnabled, globalRefreshInterval)
  }, [globalAutoRefreshEnabled, globalRefreshInterval])

  // syncGlobalRefreshToBackend 把全局自动刷新设置推给所有后端订阅源（仅在用户显式变更时调用，
  // 不在挂载/恢复时运行，避免用默认值覆盖 seed 出来的 per-source 设置）。
  const syncGlobalRefreshToBackend = useCallback(async (enabled: boolean, intervalM: number) => {
    try {
      const sources = await fetchProxySources()
      await Promise.all(sources.map(s => upsertProxySource({
        ...s,
        autoRefresh: enabled,
        refreshIntervalM: intervalM,
      })))
    } catch {
      // ignore：后端不可用时仅保留本地设置
    }
  }, [])

  useEffect(() => {
    proxiesRef.current = proxies
  }, [proxies])

  useEffect(() => {
    refreshingSourceIdsRef.current = refreshingSourceIds
  }, [refreshingSourceIds])

  useEffect(() => {
    if (!proxies.length) return
    const validIds = new Set(proxies.map(p => p.proxyId))
    setLatencyMap(prev => {
      let changed = false
      const next: Record<string, number> = {}
      Object.entries(prev).forEach(([proxyId, latency]) => {
        if (validIds.has(proxyId)) {
          next[proxyId] = latency
        } else {
          changed = true
        }
      })
      return changed ? next : prev
    })

    setIPHealthMap(prev => {
      let changed = false
      const next: Record<string, ProxyIPHealthResult> = {}
      Object.entries(prev).forEach(([proxyId, health]) => {
        if (validIds.has(proxyId)) {
          next[proxyId] = health
        } else {
          changed = true
        }
      })
      return changed ? next : prev
    })
  }, [proxies])

  const loadProxies = async () => {
    setLoading(true)
    try {
      const raw = await fetchBrowserProxies()
      const proxyList = ensureBuiltinProxies(raw)
      const persistedLatency: Record<string, number> = {}
      const persistedIPHealth: Record<string, ProxyIPHealthResult> = {}
      proxyList.forEach(proxy => {
        if (proxy.lastTestedAt) {
          persistedLatency[proxy.proxyId] = (proxy.lastTestOk ?? false)
            ? (proxy.lastLatencyMs ?? -2)
            : -2
        }
        if (proxy.lastIPHealthJson) {
          try {
            const parsed = JSON.parse(proxy.lastIPHealthJson) as ProxyIPHealthResult
            if (parsed && typeof parsed === 'object' && parsed.proxyId) {
              persistedIPHealth[proxy.proxyId] = parsed
            }
          } catch {
            // ignore bad historical json
          }
        }
      })

      setProxies(proxyList)
      setDisplayList(toDisplayList(proxyList))
      setLatencyMap(prev => ({ ...persistedLatency, ...prev }))
      setIPHealthMap(prev => ({ ...persistedIPHealth, ...prev }))
      const grps = await fetchBrowserProxyGroups()
      setGroups(grps)
    } finally {
      setLoading(false)
    }
  }

  // 直接保存完整列表，内置代理保护由后端负责
  const saveProxies = useCallback(async (list: BrowserProxy[]) => {
    await saveBrowserProxies(list)
    setProxies(list)
    setDisplayList(toDisplayList(list))
    // 刷新分组列表（可能有新分组加入）
    const grps = await fetchBrowserProxyGroups()
    setGroups(grps)
  }, [])

  const sourceMetas = useMemo(() => collectURLImportSources(proxies), [proxies])
  const proxyById = useMemo(() => {
    const m = new Map<string, BrowserProxy>()
    proxies.forEach(p => m.set(p.proxyId, p))
    return m
  }, [proxies])
  const hasURLImportSources = sourceMetas.length > 0

  const refreshSingleSource = useCallback(async (sourceId: string, silent: boolean) => {
    if (!sourceId.trim()) return false
    if (refreshingSourceIdsRef.current.has(sourceId)) return false
    setRefreshingSourceIds(prev => {
      const next = new Set(prev)
      next.add(sourceId)
      return next
    })
    try {
      // 后端刷新：套用忽略/重命名覆盖、保留 proxyId（实例绑定）与测速/IP 健康结果
      await refreshProxySource(sourceId)
      await loadProxies()
      if (!silent) {
        toast.success('订阅刷新成功')
      }
      return true
    } catch (error: any) {
      if (!silent) {
        toast.error(error?.message || '订阅刷新失败')
      }
      return false
    } finally {
      setRefreshingSourceIds(prev => {
        const next = new Set(prev)
        next.delete(sourceId)
        return next
      })
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const handleRefreshAllSources = useCallback(async (silent = false) => {
    setRefreshingAllSources(true)
    try {
      await refreshAllProxySources()
      await loadProxies()
      if (!silent) {
        toast.success('订阅刷新完成')
      }
    } catch (error: any) {
      if (!silent) {
        toast.error(error?.message || '订阅刷新失败')
      }
    } finally {
      setRefreshingAllSources(false)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // 订阅自动刷新由后端调度器负责（界面关闭也生效）。这里仅监听刷新事件后重载列表。
  useEffect(() => {
    const off = EventsOn('proxy:source:refreshed', () => {
      void loadProxies()
    })
    return () => {
      off?.()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const protocolOptions = useMemo(
    () => ['all', ...Array.from(new Set(displayList.map(p => p.type).filter(t => t !== '-')))],
    [displayList]
  )

  const getLatencySortTuple = (proxyId: string): [number, number] => {
    const v = latencyMap[proxyId]
    if (v === undefined) return [4, Number.MAX_SAFE_INTEGER]
    if (v === -1) return [1, Number.MAX_SAFE_INTEGER] // 测试中
    if (v === -2) return [2, Number.MAX_SAFE_INTEGER] // 超时
    if (v === -3) return [3, Number.MAX_SAFE_INTEGER] // 不支持
    return [0, v] // 正常延迟
  }

  const compareText = (a: string, b: string) => a.localeCompare(b, 'zh-CN')

  const compareByColumn = (a: ProxyDisplayInfo, b: ProxyDisplayInfo, column: string) => {
    switch (column) {
      case 'proxyName':
        return compareText(a.proxyName || '', b.proxyName || '')
      case 'groupName':
        return compareText(a.groupName || '', b.groupName || '')
      case 'type':
        return compareText(a.type || '', b.type || '')
      case 'server':
        return compareText(a.server || '', b.server || '')
      case 'port':
        return (a.port || 0) - (b.port || 0)
      case 'latency': {
        const [rankA, valA] = getLatencySortTuple(a.proxyId)
        const [rankB, valB] = getLatencySortTuple(b.proxyId)
        if (rankA !== rankB) return rankA - rankB
        if (valA !== valB) return valA - valB
        return compareText(a.proxyName || '', b.proxyName || '')
      }
      default:
        return 0
    }
  }

  const filteredList = useMemo(() => {
    const filtered = displayList.filter(p => {
      const matchProtocol = filterProtocol === 'all' || p.type === filterProtocol
      const matchKeyword = !filterKeyword || p.proxyName.toLowerCase().includes(filterKeyword.toLowerCase()) || p.server.toLowerCase().includes(filterKeyword.toLowerCase())
      const matchGroup = filterGroup === 'all' || p.groupName === filterGroup
      return matchProtocol && matchKeyword && matchGroup
    })

    if (!sortColumn || !sortOrder) return filtered

    return [...filtered].sort((a, b) => {
      const cmp = compareByColumn(a, b, sortColumn)
      return sortOrder === 'asc' ? cmp : -cmp
    })
  }, [displayList, filterProtocol, filterKeyword, filterGroup, sortColumn, sortOrder, latencyMap])

  // 计算代理池健康度统计
  const healthStats = useMemo(() => {
    const total = filteredList.length
    const available = filteredList.filter(p => {
      const latency = latencyMap[p.proxyId]
      return latency !== undefined && latency > 0 && latency < 10000
    }).length
    const avgLatency = (() => {
      const validLatencies = filteredList
        .map(p => latencyMap[p.proxyId])
        .filter(l => l !== undefined && l > 0 && l < 10000)
      return validLatencies.length > 0
        ? Math.round(validLatencies.reduce((sum, l) => sum + l, 0) / validLatencies.length)
        : 0
    })()
    const recentFailed = filteredList.filter(p => {
      const latency = latencyMap[p.proxyId]
      return latency !== undefined && latency <= 0
    }).length
    const successRate = total > 0 ? Math.round((available / total) * 100) : 0

    return { total, available, avgLatency, recentFailed, successRate }
  }, [filteredList, latencyMap])

  const allFilteredSelected = filteredList.length > 0 && filteredList.every(p => selectedIds.has(p.proxyId))
  const someFilteredSelected = filteredList.some(p => selectedIds.has(p.proxyId))

  const handleToggleAll = () => {
    if (allFilteredSelected) {
      setSelectedIds(prev => {
        const next = new Set(prev)
        filteredList.forEach(p => next.delete(p.proxyId))
        return next
      })
    } else {
      setSelectedIds(prev => {
        const next = new Set(prev)
        filteredList.filter(p => !BUILTIN_PROXY_IDS.has(p.proxyId)).forEach(p => next.add(p.proxyId))
        return next
      })
    }
  }

  const handleToggleOne = (proxyId: string) => {
    if (BUILTIN_PROXY_IDS.has(proxyId)) return
    setSelectedIds(prev => {
      const next = new Set(prev)
      next.has(proxyId) ? next.delete(proxyId) : next.add(proxyId)
      return next
    })
  }

  const handleBatchDeleteConfirm = async () => {
    try {
      const newProxies = proxies.filter(p => !selectedIds.has(p.proxyId))
      await saveProxies(newProxies)
      toast.success(`已删除 ${selectedIds.size} 个代理`)
      setSelectedIds(new Set())
    } catch (error: any) {
      toast.error(error?.message || '删除失败')
    }
  }

  const handleTestOne = async (record: ProxyDisplayInfo) => {
    if (record.proxyConfig === 'direct://') {
      toast.info('直连模式无需测速')
      return
    }
    setLatencyMap(prev => ({ ...prev, [record.proxyId]: -1 }))
    const result = await browserProxyTestSpeed(record.proxyId)
    const val = toLatencyValue(result.ok, result.latencyMs, result.error)
    setLatencyMap(prev => ({ ...prev, [record.proxyId]: val }))
  }

  const handleTestAll = async () => {
    const testable = filteredList.filter(p => p.proxyConfig !== 'direct://')
    if (testable.length === 0) return
    setTestingAll(true)
    const init: Record<string, number> = {}
    testable.forEach(p => { init[p.proxyId] = -1 })
    setLatencyMap(prev => ({ ...prev, ...init }))

    // 监听后端实时推送的单个测速结果
    const off = EventsOn(SPEED_RESULT_EVENT, (data: { proxyId: string; ok: boolean; latencyMs: number; error: string }) => {
      const val = toLatencyValue(data.ok, data.latencyMs, data.error)
      setLatencyMap(prev => ({ ...prev, [data.proxyId]: val }))
    })

    try {
      const proxyIds = testable.map(p => p.proxyId)
      const results = await browserProxyBatchTestSpeed(proxyIds, PROXY_SPEED_TEST_CONCURRENCY)
      setLatencyMap(prev => {
        const next = { ...prev }
        results.forEach(result => {
          next[result.proxyId] = toLatencyValue(result.ok, result.latencyMs, result.error)
        })
        return next
      })
    } finally {
      off()
      setTestingAll(false)
      // 清理被取消/未返回的 -1（测试中）状态，避免残留 spinner
      setLatencyMap(prev => {
        const next = { ...prev }
        testable.forEach(p => { if (next[p.proxyId] === -1) delete next[p.proxyId] })
        return next
      })
    }
  }

  const handleCheckOneIPHealth = async (record: ProxyDisplayInfo) => {
    if (record.proxyConfig === 'direct://') {
      toast.info('直连模式无需检测')
      return
    }
    if (checkingIPHealthIds.has(record.proxyId)) return

    setCheckingIPHealthIds(prev => new Set(prev).add(record.proxyId))
    try {
      const result = await browserProxyCheckIPHealth(record.proxyId)
      setIPHealthMap(prev => ({ ...prev, [record.proxyId]: result }))
      if (!result.ok) {
        toast.error(result.error || `${record.proxyName} 检测失败`)
      }
    } finally {
      setCheckingIPHealthIds(prev => {
        const next = new Set(prev)
        next.delete(record.proxyId)
        return next
      })
    }
  }

  const handleCheckAllIPHealth = async () => {
    const testable = filteredList.filter(p => p.proxyConfig !== 'direct://')
    if (testable.length === 0) return
    setCheckingAllIPHealth(true)

    const ids = testable.map(p => p.proxyId)
    const idSet = new Set(ids)
    setCheckingIPHealthIds(prev => new Set([...Array.from(prev), ...ids]))

    const off = EventsOn(IP_HEALTH_RESULT_EVENT, (data: ProxyIPHealthResult) => {
      if (!data?.proxyId || !idSet.has(data.proxyId)) return
      setIPHealthMap(prev => ({ ...prev, [data.proxyId]: data }))
      setCheckingIPHealthIds(prev => {
        const next = new Set(prev)
        next.delete(data.proxyId)
        return next
      })
    })

    try {
      const results = await browserProxyBatchCheckIPHealth(ids, PROXY_IP_HEALTH_TEST_CONCURRENCY)
      setIPHealthMap(prev => {
        const next = { ...prev }
        results.forEach(result => {
          if (result?.proxyId && idSet.has(result.proxyId)) {
            next[result.proxyId] = result
          }
        })
        return next
      })
      const failed = results.filter(r => !r.ok).length
      if (failed > 0) {
        toast.info(`IP 健康检测完成：成功 ${results.length - failed}，失败 ${failed}`)
      } else {
        toast.success(`IP 健康检测完成：共 ${results.length} 条`)
      }
    } finally {
      off()
      setCheckingIPHealthIds(prev => {
        const next = new Set(prev)
        ids.forEach(id => next.delete(id))
        return next
      })
      setCheckingAllIPHealth(false)
    }
  }

  const renderLatency = (record: ProxyDisplayInfo) => {
    if (record.proxyConfig === 'direct://') {
      return <span className="text-[var(--color-text-muted)] text-xs">不适用</span>
    }
    const val = latencyMap[record.proxyId]
    if (val === undefined) return <span className="text-[var(--color-text-muted)] text-xs">-</span>
    if (val === -1) return <span className="text-[var(--color-text-muted)] text-xs animate-pulse">测试中...</span>
    if (val === -2) return <span className="text-red-500 text-xs">超时</span>
    if (val === -3) return <span className="text-gray-400 text-xs">不支持</span>
    const color = val < 200 ? 'text-green-500' : val < 500 ? 'text-yellow-500' : 'text-red-500'
    return <span className={`text-xs font-medium ${color}`}>{val} ms</span>
  }

  const openIPHealthDetail = (proxyId: string) => {
    const result = ipHealthMap[proxyId]
    if (!result) return
    setCurrentIPHealthDetail(result)
    setIPHealthDetailOpen(true)
  }

  const renderIPHealth = (record: ProxyDisplayInfo) => {
    if (record.proxyConfig === 'direct://') {
      return <span className="text-[var(--color-text-muted)] text-xs">不适用</span>
    }
    if (checkingIPHealthIds.has(record.proxyId)) {
      return <span className="text-[var(--color-text-muted)] text-xs animate-pulse">检测中...</span>
    }

    const result = ipHealthMap[record.proxyId]
    if (!result) return <span className="text-[var(--color-text-muted)] text-xs">-</span>
    if (!result.ok) {
      return (
        <div className="flex items-center gap-2">
          <span className="text-xs text-red-500 truncate max-w-[120px]" title={result.error || '检测失败'}>失败</span>
          <Button size="sm" variant="ghost" onClick={(e) => { e.stopPropagation(); openIPHealthDetail(record.proxyId) }}>原始</Button>
        </div>
      )
    }

    const location = [result.country, result.region, result.city].filter(Boolean).join(' / ')
    return (
      <div className="flex items-center gap-2 min-w-0">
        <div className="min-w-0">
          <div className="text-xs text-[var(--color-text-primary)] truncate">{result.ip || '-'}</div>
          <div className="text-[11px] text-[var(--color-text-muted)] truncate">
            {`fraud ${result.fraudScore} | ${result.isResidential ? '住宅' : '机房'}${location ? ` | ${location}` : ''}`}
          </div>
        </div>
        <Button size="sm" variant="ghost" onClick={(e) => { e.stopPropagation(); openIPHealthDetail(record.proxyId) }}>原始</Button>
      </div>
    )
  }

  const renderScore = (record: ProxyDisplayInfo) => {
    const proxy = proxyById.get(record.proxyId)
    const base: BrowserProxy = proxy || { proxyId: record.proxyId, proxyName: record.proxyName, proxyConfig: record.proxyConfig }
    const result = computeProxyScore(base, latencyMap[record.proxyId], ipHealthMap[record.proxyId])
    if (result.score < 0) {
      return <span className="text-[var(--color-text-muted)]">-</span>
    }
    return <Badge variant={proxyScoreVariant(result.grade)}>{`${result.grade} ${result.score}`}</Badge>
  }

  const columns: TableColumn<ProxyDisplayInfo>[] = [
    {
      key: 'checkbox',
      title: '',
      width: '40px',
      render: (_, record) => (
        <input
          type="checkbox"
          checked={selectedIds.has(record.proxyId)}
          disabled={BUILTIN_PROXY_IDS.has(record.proxyId)}
          onChange={() => handleToggleOne(record.proxyId)}
          onClick={e => e.stopPropagation()}
          className="w-4 h-4 rounded border-[var(--color-border)] accent-[var(--color-primary)] cursor-pointer disabled:opacity-30 disabled:cursor-not-allowed"
        />
      ),
    },
    { key: 'proxyName', title: '代理名称', width: '180px', sortable: true },
    { key: 'groupName', title: '分组', width: '100px', sortable: true, render: (val) => val ? <span className="px-1.5 py-0.5 text-xs rounded bg-[var(--color-accent)]/10 text-[var(--color-accent)]">{val}</span> : '-' },
    {
      key: 'source',
      title: '来源',
      width: '180px',
      render: (_, record) => {
        if (!record.sourceUrl) return '-'
        const host = sourceHostLabel(record.sourceUrl)
        return (
          <div className="text-xs leading-5">
            <div className="text-[var(--color-text-primary)] truncate" title={record.sourceUrl}>{host}</div>
            <div className="text-[var(--color-text-muted)]">
              {globalAutoRefreshEnabled ? `自动刷新 ${globalRefreshInterval} 分钟（全局）` : '手动刷新'}
            </div>
          </div>
        )
      },
    },
    { key: 'type', title: '类型', width: '90px', sortable: true },
    { key: 'server', title: '服务器', width: '180px', sortable: true },
    { key: 'port', title: '端口', width: '80px', sortable: true, render: (val) => val || '-' },
    {
      key: 'latency',
      title: '延迟',
      width: '90px',
      sortable: true,
      render: (_, record) => renderLatency(record),
    },
    {
      key: 'score',
      title: '评分',
      width: '90px',
      render: (_, record) => renderScore(record),
    },
    {
      key: 'ipHealth',
      title: 'IP健康',
      width: '280px',
      render: (_, record) => renderIPHealth(record),
    },
    {
      key: 'actions',
      title: '操作',
      width: '320px',
      render: (_, record) => {
        const isBuiltin = BUILTIN_PROXY_IDS.has(record.proxyId)
        const isEditLocked = record.proxyId === '__direct__'
        const hasSource = !!record.sourceId && !!record.sourceUrl
        return (
          <div className="flex gap-2">
            {hasSource && (
              <Button
                size="sm"
                variant="secondary"
                onClick={(e) => { e.stopPropagation(); void refreshSingleSource(record.sourceId, false) }}
                loading={refreshingSourceIds.has(record.sourceId)}
              >
                刷新订阅
              </Button>
            )}
            <Button
              size="sm" variant="ghost"
              onClick={(e) => { e.stopPropagation(); handleTestOne(record) }}
              loading={latencyMap[record.proxyId] === -1}
              disabled={record.proxyConfig === 'direct://'}
            >测速</Button>
            <Button
              size="sm" variant="ghost"
              onClick={(e) => { e.stopPropagation(); handleCheckOneIPHealth(record) }}
              loading={checkingIPHealthIds.has(record.proxyId)}
              disabled={record.proxyConfig === 'direct://'}
            >IP健康</Button>
            <Button
              size="sm" variant="ghost"
              disabled={isEditLocked}
              title={isEditLocked ? '直连代理不可编辑' : undefined}
              onClick={(e) => { e.stopPropagation(); if (!isEditLocked) handleEdit(record) }}
            >编辑</Button>
            <Button
              size="sm" variant="danger"
              disabled={isBuiltin}
              title={isBuiltin ? '内置代理不可删除' : undefined}
              onClick={(e) => { e.stopPropagation(); if (!isBuiltin) handleDeleteClick(record.proxyId) }}
            >删除</Button>
          </div>
        )
      },
    },
  ]

  const handleRemovePreviewProxy = (proxyId: string) => {
    const target = previewList.find(item => item.proxyId === proxyId)
    if (!target) return
    setPreviewList(prev => prev.filter(item => item.proxyId !== proxyId))
    setRemovedPreviewProxyNames(prev => [...prev, target.proxyName])
  }

  const previewColumns: TableColumn<ProxyDisplayInfo>[] = [
    { key: 'proxyName', title: '代理名称', width: '200px' },
    { key: 'type', title: '类型', width: '100px' },
    { key: 'server', title: '服务器', width: '200px' },
    { key: 'port', title: '端口', width: '100px', render: (val) => val || '-' },
    {
      key: 'actions',
      title: '操作',
      width: '96px',
      render: (_, record) => (
        <Button
          size="sm"
          variant="danger"
          onClick={() => handleRemovePreviewProxy(record.proxyId)}
        >
          删除
        </Button>
      ),
    },
  ]

  const handleEdit = (record: ProxyDisplayInfo) => {
    const proxy = proxies.find(p => p.proxyId === record.proxyId)
    if (proxy) {
      setEditingProxy(proxy)
      const chainCfg = parseChainSocks5Config(proxy.proxyConfig)
      if (chainCfg) {
        setChainImportForm(toChainImportForm(proxy.proxyName, chainCfg))
        setChainEditMode(true)
        setEditForm({ proxyName: proxy.proxyName, proxyConfig: proxy.proxyConfig, dnsServers: proxy.dnsServers || '', groupName: proxy.groupName || '' })
      } else {
        setChainEditMode(false)
        setEditForm({ proxyName: proxy.proxyName, proxyConfig: proxy.proxyConfig, dnsServers: proxy.dnsServers || '', groupName: proxy.groupName || '' })
      }
      setEditModalOpen(true)
    }
  }

  const handleSaveProxy = async () => {
    const isChainEditing = chainEditMode
    const nextProxyName = isChainEditing ? chainImportForm.proxyName.trim() : editForm.proxyName.trim()
    if (!nextProxyName) { toast.error('请输入代理名称'); return }
    if (!editingProxy) return
    setSaving(true)
    try {
      const nextProxyConfig = isChainEditing
        ? buildChainImportCandidate(chainImportForm).proxyConfig
        : editForm.proxyConfig
      const newProxies = proxies.map(p =>
        p.proxyId === editingProxy.proxyId
          ? { ...p, proxyName: nextProxyName, proxyConfig: nextProxyConfig, dnsServers: editForm.dnsServers, groupName: editForm.groupName }
          : p
      )
      await saveProxies(newProxies)
      // 订阅源代理改名 → 写入后端 rename 覆盖，使刷新后保留用户自定义名称
      const srcId = (editingProxy.sourceId || '').trim()
      if (srcId && nextProxyName !== editingProxy.proxyName) {
        try {
          const overrides = await fetchProxySourceOverrides(srcId)
          const nodeKey = deriveSourceNodeKey(editingProxy, overrides)
          await setProxySourceOverride(srcId, nodeKey, 'rename', nextProxyName)
        } catch {
          // ignore：后端不可用时仅本地改名
        }
      }
      setEditModalOpen(false)
      setChainEditMode(false)
      toast.success('代理已更新')
    } catch (error: any) {
      toast.error(error?.message || '保存失败')
    } finally {
      setSaving(false)
    }
  }

  const handleDeleteClick = (proxyId: string) => {
    setDeletingId(proxyId)
    setDeleteConfirmOpen(true)
  }

  const handleDeleteConfirm = async () => {
    if (!deletingId) return
    try {
      const target = proxies.find(p => p.proxyId === deletingId)
      const newProxies = proxies.filter(p => p.proxyId !== deletingId)
      await saveProxies(newProxies)
      // 删除订阅源代理 → 写入后端 ignore 覆盖，使刷新后不再被重新加入
      const srcId = (target?.sourceId || '').trim()
      if (target && srcId) {
        try {
          const overrides = await fetchProxySourceOverrides(srcId)
          const nodeKey = deriveSourceNodeKey(target, overrides)
          await setProxySourceOverride(srcId, nodeKey, 'ignore')
        } catch {
          // ignore：后端不可用时仅本地删除
        }
      }
      setSelectedIds(prev => { const next = new Set(prev); next.delete(deletingId); return next })
      toast.success('代理已删除')
    } catch (error: any) {
      toast.error(error?.message || '删除失败')
    }
    setDeletingId(null)
  }

  const handleImportModeChange = (nextMode: ProxyImportMode) => {
    setImportMode(nextMode)
    setImportResolvedUrl('')
    if (nextMode !== 'clash') {
      setImportUrl('')
      setImportDnsServers('')
    }
  }

  const updateChainHop = (hop: 'first' | 'second', field: keyof ChainHopForm, value: string) => {
    setChainImportForm(prev => ({
      ...prev,
      [hop]: {
        ...prev[hop],
        [field]: value,
      },
    }))
  }

  const handleFetchImportURL = async () => {
    const targetURL = importUrl.trim()
    if (!targetURL) {
      toast.error('请输入订阅 URL')
      return
    }

    setFetchingImportUrl(true)
    try {
      const result = await fetchClashImportFromURL(targetURL)
      const content = (result?.content || '').trim()
      if (!content) {
        throw new Error('订阅内容为空')
      }

      setImportResolvedUrl((result?.url || targetURL).trim())
      setImportText(content)

      if (!importDnsServers.trim() && typeof result?.dnsServers === 'string' && result.dnsServers.trim()) {
        setImportDnsServers(result.dnsServers.trim())
      }
      if (!importGroupName.trim() && typeof result?.suggestedGroup === 'string' && result.suggestedGroup.trim()) {
        setImportGroupName(result.suggestedGroup.trim())
      }

      toast.success(`URL 获取成功，检测到 ${Math.max(0, Number(result?.proxyCount || 0))} 个代理`)
    } catch (error: any) {
      setImportResolvedUrl('')
      toast.error(error?.message || 'URL 获取失败')
    } finally {
      setFetchingImportUrl(false)
    }
  }

  const handleParseImport = () => {
    try {
      const prefix = importNamePrefix.trim()
      const candidates = importMode === 'clash'
        ? buildImportCandidatesFromClash(parseClashImportText(importText), prefix)
        : importMode === 'direct'
          ? [buildDirectImportCandidate(directImportForm)]
          : [buildChainImportCandidate(chainImportForm)]
      if (!candidates.length) {
        toast.error('未解析到可导入代理')
        return
      }
      const preview = buildImportPreview(candidates, importGroupName.trim())
      setRemovedPreviewProxyNames([])
      setPreviewList(preview)
      setImportModalOpen(false)
      setPreviewModalOpen(true)
    } catch (error: any) {
      toast.error(`解析失败: ${error?.message || '未知错误'}`)
    }
  }

  const handleConfirmImport = async () => {
    if (previewList.length === 0) {
      toast.error('请至少保留 1 个代理后再导入')
      return
    }
    setImporting(true)
    try {
      const sourceURL = importMode === 'clash' ? importResolvedUrl.trim() : ''
      const isURLImport = !!sourceURL
      const sourceNamePrefix = importMode === 'clash' ? importNamePrefix.trim() : ''
      const sourceID = isURLImport ? resolveImportSourceID(proxies, sourceURL, sourceNamePrefix) : ''
      const sourceAutoRefresh = isURLImport ? globalAutoRefreshEnabled : false
      const sourceRefreshIntervalM = sourceAutoRefresh ? globalRefreshInterval : 0
      const sourceLastRefreshAt = isURLImport ? new Date().toISOString() : ''
      const oldSourceProxies = isURLImport
        ? proxies.filter(item => (item.sourceId || '').trim() === sourceID)
        : []
      const pickExistingID = createExistingProxyIDPicker(oldSourceProxies)

      const newProxies: BrowserProxy[] = previewList.map((p) => ({
        proxyId: pickExistingID(p.proxyName, p.proxyConfig) || nextProxyID(),
        proxyName: p.proxyName,
        proxyConfig: p.proxyConfig,
        dnsServers: importMode === 'clash' ? importDnsServers.trim() || undefined : undefined,
        groupName: importGroupName.trim() || undefined,
        sourceId: sourceID || undefined,
        sourceUrl: sourceURL || undefined,
        sourceNamePrefix: sourceNamePrefix || undefined,
        sourceAutoRefresh,
        sourceRefreshIntervalM,
        sourceLastRefreshAt: sourceLastRefreshAt || undefined,
      }))
      const allProxies = isURLImport
        ? proxies.filter(item => (item.sourceId || '').trim() !== sourceID).concat(newProxies)
        : [...proxies, ...newProxies]
      await saveProxies(allProxies)
      if (isURLImport) {
        // 注册/更新后端订阅源，使后端调度刷新与覆盖记录立即可用（无需重启）
        try {
          await upsertProxySource({
            sourceId: sourceID,
            sourceUrl: sourceURL,
            groupName: importGroupName.trim(),
            namePrefix: sourceNamePrefix,
            dnsServers: importMode === 'clash' ? importDnsServers.trim() : '',
            autoRefresh: sourceAutoRefresh,
            refreshIntervalM: sourceRefreshIntervalM,
            importStrategy: 'merge',
            lastRefreshAt: sourceLastRefreshAt,
          })
          // 预览中被删除的节点 → 后端忽略覆盖（刷新时不再加入）
          if (removedPreviewProxyNames.length > 0) {
            await Promise.all(removedPreviewProxyNames.map(displayName => {
              const nodeKey = sourceNamePrefix && displayName.startsWith(`${sourceNamePrefix}-`)
                ? displayName.slice(sourceNamePrefix.length + 1)
                : displayName
              return setProxySourceOverride(sourceID, nodeKey, 'ignore')
            }))
          }
        } catch {
          // ignore：后端不可用时退化为仅本地记录
        }
        if (removedPreviewProxyNames.length > 0) {
          appendSourceIgnoredProxyNames(sourceID, removedPreviewProxyNames)
        }
      }
      setPreviewModalOpen(false)
      setImportUrl('')
      setImportResolvedUrl('')
      setImportText('')
      setImportDnsServers('')
      setImportNamePrefix('')
      setImportGroupName('')
      setDirectImportForm({ ...INITIAL_DIRECT_IMPORT_FORM })
      setChainImportForm({ ...INITIAL_CHAIN_IMPORT_FORM })
      setPreviewList([])
      setRemovedPreviewProxyNames([])
      toast.success(`成功导入 ${newProxies.length} 个代理`)
    } catch (error: any) {
      toast.error(error?.message || '导入失败')
    } finally {
      setImporting(false)
    }
  }

  const selectedCount = selectedIds.size
  const canParseImport = importMode === 'clash'
    ? !!importText.trim()
    : importMode === 'direct'
      ? !!directImportForm.server.trim() && !!directImportForm.port.trim()
      : !!chainImportForm.first.server.trim() && !!chainImportForm.first.port.trim() && !!chainImportForm.second.server.trim() && !!chainImportForm.second.port.trim()

  return (
    <div className="space-y-5 animate-fade-in">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">代理池配置</h1>
          <p className="text-sm text-[var(--color-text-muted)] mt-1">管理代理配置，支持 Clash 订阅、HTTP、HTTPS、SOCKS5</p>
        </div>
        <div className="flex gap-2">
          <Button
            size="sm"
            variant="secondary"
            onClick={() => void handleRefreshAllSources(false)}
            loading={refreshingAllSources}
            disabled={!hasURLImportSources}
          >
            刷新订阅
          </Button>
          <Button size="sm" variant="secondary" onClick={handleCheckAllIPHealth} loading={checkingAllIPHealth} disabled={filteredList.length === 0}>检测IP健康</Button>
          <Button size="sm" variant="secondary" onClick={handleTestAll} loading={testingAll} disabled={filteredList.length === 0}>测试全部</Button>
          {(testingAll || checkingAllIPHealth) && (
            <Button size="sm" variant="secondary" onClick={() => { void cancelProxyBatch() }} title="停止派发后续任务（进行中的检测会自然结束）">取消</Button>
          )}
          <Button size="sm" onClick={() => setImportModalOpen(true)}>导入代理</Button>
        </div>
      </div>

      {/* 代理池健康度监控面板 */}
      <div className="grid grid-cols-2 lg:grid-cols-5 gap-4">
        {/* 可用代理数 */}
        <Card>
          <div className="text-center">
            <div className="text-sm text-[var(--color-text-muted)] mb-2">可用代理</div>
            <div className="text-3xl font-bold text-green-600 mb-1">
              {healthStats.available}
            </div>
            <div className="text-xs text-[var(--color-text-muted)]">
              总数: {healthStats.total}
            </div>
          </div>
        </Card>

        {/* 成功率 */}
        <Card>
          <div className="text-center">
            <div className="text-sm text-[var(--color-text-muted)] mb-2">测试通过率</div>
            <div className={`text-3xl font-bold mb-1 ${
              healthStats.successRate >= 90 ? 'text-green-600' :
              healthStats.successRate >= 70 ? 'text-yellow-600' :
              'text-red-600'
            }`}>
              {healthStats.successRate}%
            </div>
            <div className="text-xs text-[var(--color-text-muted)]">
              {healthStats.available}/{healthStats.total}
            </div>
          </div>
        </Card>

        {/* 平均延迟 */}
        <Card>
          <div className="text-center">
            <div className="text-sm text-[var(--color-text-muted)] mb-2">平均延迟</div>
            <div className={`text-3xl font-bold mb-1 ${
              healthStats.avgLatency === 0 ? 'text-gray-400' :
              healthStats.avgLatency < 200 ? 'text-green-600' :
              healthStats.avgLatency < 500 ? 'text-yellow-600' :
              'text-red-600'
            }`}>
              {healthStats.avgLatency === 0 ? '-' : healthStats.avgLatency}
            </div>
            <div className="text-xs text-[var(--color-text-muted)]">
              毫秒
            </div>
          </div>
        </Card>

        {/* 失效代理 */}
        <Card>
          <div className="text-center">
            <div className="text-sm text-[var(--color-text-muted)] mb-2">最近失效</div>
            <div className={`text-3xl font-bold mb-1 ${
              healthStats.recentFailed === 0 ? 'text-green-600' : 'text-red-600'
            }`}>
              {healthStats.recentFailed}
            </div>
            <div className="text-xs text-[var(--color-text-muted)]">
              个代理
            </div>
          </div>
        </Card>

        {/* 健康度评分 */}
        <Card>
          <div className="text-center">
            <div className="text-sm text-[var(--color-text-muted)] mb-2">健康度</div>
            <div className={`text-3xl font-bold mb-1 ${
              healthStats.successRate >= 90 && healthStats.avgLatency < 300 ? 'text-green-600' :
              healthStats.successRate >= 70 && healthStats.avgLatency < 500 ? 'text-yellow-600' :
              'text-red-600'
            }`}>
              {healthStats.successRate >= 90 && healthStats.avgLatency < 300 ? '优秀' :
               healthStats.successRate >= 70 && healthStats.avgLatency < 500 ? '良好' :
               healthStats.successRate >= 50 ? '一般' : '较差'}
            </div>
            <div className="text-xs text-[var(--color-text-muted)]">
              综合评价
            </div>
          </div>
        </Card>
      </div>

      <Card>
        <div className="flex items-center gap-3 mb-4">
          <Input
            value={filterKeyword}
            onChange={e => setFilterKeyword(e.target.value)}
            placeholder="搜索名称或服务器..."
            style={{ width: '220px' }}
          />
          <select
            value={filterProtocol}
            onChange={e => setFilterProtocol(e.target.value)}
            className="px-3 py-1.5 text-sm rounded-md border border-[var(--color-border)] bg-[var(--color-bg-secondary)] text-[var(--color-text-primary)] focus:outline-none focus:ring-1 focus:ring-[var(--color-primary)]"
          >
            {protocolOptions.map(p => (
              <option key={p} value={p}>{p === 'all' ? '全部协议' : p.toUpperCase()}</option>
            ))}
          </select>
          <select
            value={filterGroup}
            onChange={e => setFilterGroup(e.target.value)}
            className="px-3 py-1.5 text-sm rounded-md border border-[var(--color-border)] bg-[var(--color-bg-secondary)] text-[var(--color-text-primary)] focus:outline-none focus:ring-1 focus:ring-[var(--color-primary)]"
          >
            <option value="all">全部分组</option>
            {groups.map(g => <option key={g} value={g}>{g}</option>)}
          </select>
          {(filterProtocol !== 'all' || filterKeyword || filterGroup !== 'all') && (
            <Button size="sm" variant="ghost" onClick={() => { setFilterProtocol('all'); setFilterKeyword(''); setFilterGroup('all') }}>清除筛选</Button>
          )}
          <div className="flex items-center gap-2 rounded-md border border-[var(--color-border)] bg-[var(--color-bg-secondary)] px-2 py-1.5">
            <span className="text-xs text-[var(--color-text-muted)]">全局自动刷新</span>
            <Switch
              checked={globalAutoRefreshEnabled}
              onChange={(checked) => { setGlobalAutoRefreshEnabled(checked); void syncGlobalRefreshToBackend(checked, globalRefreshInterval) }}
            />
            <Input
              type="number"
              min={5}
              max={1440}
              value={globalRefreshIntervalM}
              onChange={e => setGlobalRefreshIntervalM(e.target.value)}
              onBlur={() => { if (globalAutoRefreshEnabled) void syncGlobalRefreshToBackend(true, globalRefreshInterval) }}
              className="w-24"
              disabled={!globalAutoRefreshEnabled}
            />
            <span className="text-xs text-[var(--color-text-muted)]">分钟</span>
          </div>
          <div className="flex-1" />
          {filteredList.length > 0 && (
            <label className="flex items-center gap-1.5 text-sm text-[var(--color-text-muted)] cursor-pointer select-none">
              <input
                type="checkbox"
                checked={allFilteredSelected}
                ref={el => { if (el) el.indeterminate = someFilteredSelected && !allFilteredSelected }}
                onChange={handleToggleAll}
                className="w-4 h-4 rounded border-[var(--color-border)] accent-[var(--color-primary)] cursor-pointer"
              />
              全选
            </label>
          )}
          {selectedCount > 0 && (
            <Button size="sm" variant="danger" onClick={() => setBatchDeleteConfirmOpen(true)}>
              删除所选 ({selectedCount})
            </Button>
          )}
        </div>
        <Table
          columns={columns}
          data={filteredList}
          rowKey="proxyId"
          loading={loading}
          emptyText="暂无代理配置，点击上方按钮添加或导入"
          sortColumn={sortColumn}
          sortOrder={sortOrder}
          onSort={({ column, order }) => {
            setSortColumn(column)
            setSortOrder(order)
          }}
        />
      </Card>

      <Modal open={importModalOpen} onClose={() => setImportModalOpen(false)} title="导入代理配置" width="600px"
        footer={
          <>
            <Button variant="secondary" onClick={() => setImportModalOpen(false)} disabled={fetchingImportUrl}>取消</Button>
            <Button onClick={handleParseImport} disabled={fetchingImportUrl || !canParseImport}>解析</Button>
          </>
        }>
        <div className="space-y-4">
          <div className="grid grid-cols-3 gap-2">
            <Button
              variant={importMode === 'clash' ? undefined : 'secondary'}
              onClick={() => handleImportModeChange('clash')}
            >
              Clash 订阅 / YAML
            </Button>
            <Button
              variant={importMode === 'direct' ? undefined : 'secondary'}
              onClick={() => handleImportModeChange('direct')}
            >
              HTTP / SOCKS5（测试中）
            </Button>
            <Button
              variant={importMode === 'chain' ? undefined : 'secondary'}
              onClick={() => handleImportModeChange('chain')}
            >
              链式代理
            </Button>
          </div>
          <p className="text-sm text-[var(--color-text-muted)]">
            {importMode === 'clash'
              ? '支持粘贴 Clash YAML，或通过订阅 URL 自动拉取并解析（含 proxies、dns、proxy-groups）'
              : importMode === 'direct'
                ? '支持单条录入 HTTP / HTTPS / SOCKS5 代理，账号和密码均可留空，导入后直接生效，不走 Clash 桥接'
                : '支持两层 SOCKS5 链式代理，导入后将由本地桥接生成 127.0.0.1 SOCKS5 供 Chromium 使用'}
          </p>
          {importMode === 'clash' && (
            <>
              <FormItem label="订阅 URL（可选）">
                <div className="flex gap-2">
                  <Input
                    value={importUrl}
                    onChange={e => {
                      const next = e.target.value
                      setImportUrl(next)
                      if (importResolvedUrl.trim() && next.trim() !== importResolvedUrl.trim()) {
                        setImportResolvedUrl('')
                      }
                    }}
                    placeholder="https://example.com/clash/subscription"
                    className="flex-1"
                  />
                  <Button
                    variant="secondary"
                    onClick={handleFetchImportURL}
                    loading={fetchingImportUrl}
                    disabled={!importUrl.trim()}
                  >
                    从 URL 获取
                  </Button>
                </div>
                {importResolvedUrl.trim() && (
                  <p className="text-xs text-[var(--color-success)] mt-1 break-all">
                    已绑定订阅：{importResolvedUrl}
                  </p>
                )}
                <p className="text-xs text-[var(--color-text-muted)] mt-1">获取成功后会自动回填 YAML 文本，并尝试自动填充 DNS 与建议分组；自动刷新时间请在列表顶部统一配置</p>
              </FormItem>
              <Textarea
                value={importText}
                onChange={e => setImportText(e.target.value)}
                rows={12}
                placeholder={`proxies:\n  - name: vless-v6\n    type: vless\n    server: example.com\n    port: 443\n    uuid: your-uuid\n    ...`}
              />
            </>
          )}
          {importMode === 'direct' && (
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <FormItem label="代理协议" required>
                <Select
                  options={[...DIRECT_PROXY_PROTOCOL_OPTIONS]}
                  value={directImportForm.protocol}
                  onChange={e => setDirectImportForm(prev => ({ ...prev, protocol: e.target.value as DirectImportForm['protocol'] }))}
                />
              </FormItem>
              <FormItem label="代理名称（可选）">
                <Input
                  value={directImportForm.proxyName}
                  onChange={e => setDirectImportForm(prev => ({ ...prev, proxyName: e.target.value }))}
                  placeholder="例如：香港节点"
                />
              </FormItem>
              <FormItem label="代理地址" required>
                <Input
                  value={directImportForm.server}
                  onChange={e => setDirectImportForm(prev => ({ ...prev, server: e.target.value }))}
                  placeholder="例如：127.0.0.1 或 hk.example.com"
                />
              </FormItem>
              <FormItem label="代理端口" required>
                <Input
                  type="number"
                  min={1}
                  max={65535}
                  value={directImportForm.port}
                  onChange={e => setDirectImportForm(prev => ({ ...prev, port: e.target.value }))}
                  placeholder="例如：1080"
                />
              </FormItem>
              <FormItem label="账号（可选）">
                <Input
                  value={directImportForm.username}
                  onChange={e => setDirectImportForm(prev => ({ ...prev, username: e.target.value }))}
                  placeholder="留空则不使用认证"
                />
              </FormItem>
              <FormItem label="密码（可选）">
                <Input
                  type="password"
                  value={directImportForm.password}
                  onChange={e => setDirectImportForm(prev => ({ ...prev, password: e.target.value }))}
                  placeholder="留空则不使用密码"
                />
              </FormItem>
            </div>
          )}
          {importMode === 'chain' && (
            <div className="space-y-4">
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <FormItem label="代理名称（可选）">
                  <Input
                    value={chainImportForm.proxyName}
                    onChange={e => setChainImportForm(prev => ({ ...prev, proxyName: e.target.value }))}
                    placeholder="例如：双层香港链路"
                  />
                </FormItem>
                <FormItem label="本地监听端口（可选）">
                  <Input
                    type="number"
                    min={1}
                    max={65535}
                    value={chainImportForm.localPort}
                    onChange={e => setChainImportForm(prev => ({ ...prev, localPort: e.target.value }))}
                    placeholder="留空自动分配"
                  />
                </FormItem>
              </div>

              <div className="rounded-md border border-[var(--color-border)] p-3 space-y-3">
                <h4 className="text-sm font-medium text-[var(--color-text-primary)]">第一层 SOCKS5</h4>
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                  <FormItem label="代理地址" required>
                    <Input
                      value={chainImportForm.first.server}
                      onChange={e => updateChainHop('first', 'server', e.target.value)}
                      placeholder="例如：s1.example.com"
                    />
                  </FormItem>
                  <FormItem label="代理端口" required>
                    <Input
                      type="number"
                      min={1}
                      max={65535}
                      value={chainImportForm.first.port}
                      onChange={e => updateChainHop('first', 'port', e.target.value)}
                      placeholder="例如：1080"
                    />
                  </FormItem>
                  <FormItem label="账号（可选）">
                    <Input
                      value={chainImportForm.first.username}
                      onChange={e => updateChainHop('first', 'username', e.target.value)}
                      placeholder="留空则不使用认证"
                    />
                  </FormItem>
                  <FormItem label="密码（可选）">
                    <Input
                      type="password"
                      value={chainImportForm.first.password}
                      onChange={e => updateChainHop('first', 'password', e.target.value)}
                      placeholder="留空则不使用密码"
                    />
                  </FormItem>
                </div>
              </div>

              <div className="rounded-md border border-[var(--color-border)] p-3 space-y-3">
                <h4 className="text-sm font-medium text-[var(--color-text-primary)]">第二层 SOCKS5</h4>
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                  <FormItem label="代理地址" required>
                    <Input
                      value={chainImportForm.second.server}
                      onChange={e => updateChainHop('second', 'server', e.target.value)}
                      placeholder="例如：s2.example.com"
                    />
                  </FormItem>
                  <FormItem label="代理端口" required>
                    <Input
                      type="number"
                      min={1}
                      max={65535}
                      value={chainImportForm.second.port}
                      onChange={e => updateChainHop('second', 'port', e.target.value)}
                      placeholder="例如：1081"
                    />
                  </FormItem>
                  <FormItem label="账号（可选）">
                    <Input
                      value={chainImportForm.second.username}
                      onChange={e => updateChainHop('second', 'username', e.target.value)}
                      placeholder="留空则不使用认证"
                    />
                  </FormItem>
                  <FormItem label="密码（可选）">
                    <Input
                      type="password"
                      value={chainImportForm.second.password}
                      onChange={e => updateChainHop('second', 'password', e.target.value)}
                      placeholder="留空则不使用密码"
                    />
                  </FormItem>
                </div>
              </div>
            </div>
          )}

          <FormItem label="分组名称（可选）">
            <Input
              value={importGroupName}
              onChange={e => setImportGroupName(e.target.value)}
              placeholder="例如：香港、美国、机场A"
              list="proxy-groups-datalist"
            />
            {groups.length > 0 && (
              <datalist id="proxy-groups-datalist">
                {groups.map(g => <option key={g} value={g} />)}
              </datalist>
            )}
            <p className="text-xs text-[var(--color-text-muted)] mt-1">填写后本次导入的代理将归入该分组，可按分组筛选</p>
          </FormItem>
          {importMode === 'clash' && (
            <FormItem label="名称前缀（可选）">
              <Input
                value={importNamePrefix}
                onChange={e => setImportNamePrefix(e.target.value)}
                placeholder="例如：HK、US、机场A"
              />
              <p className="text-xs text-[var(--color-text-muted)] mt-1">
                填写后代理名称将变为 <code className="px-1 bg-[var(--color-bg-secondary)] rounded">前缀-原名称</code>，留空则保持原名
              </p>
            </FormItem>
          )}
          {importMode === 'clash' && (
            <FormItem label="批量 DNS 配置（可选）">
              <Textarea value={importDnsServers} onChange={e => setImportDnsServers(e.target.value)} rows={5}
                placeholder={`dns:\n  enable: true\n  nameserver:\n    - 119.29.29.29\n    - 223.5.5.5`} />
              <p className="text-xs text-[var(--color-text-muted)] mt-1">留空则不配置 DNS，填写后将应用到本次导入的所有代理</p>
            </FormItem>
          )}
        </div>
      </Modal>

      <Modal open={previewModalOpen} onClose={() => setPreviewModalOpen(false)} title="确认导入以下代理" width="700px"
        footer={<><Button variant="secondary" onClick={() => { setPreviewModalOpen(false); setImportModalOpen(true) }}>返回修改</Button><Button onClick={handleConfirmImport} loading={importing} disabled={previewList.length === 0}>确认导入</Button></>}>
        <div className="space-y-3">
          {importMode === 'clash' && importDnsServers.trim() && (
            <p className="text-xs text-[var(--color-text-muted)] bg-[var(--color-bg-secondary)] px-3 py-2 rounded">已配置批量 DNS，将应用到以下所有代理</p>
          )}
          <p className="text-xs text-[var(--color-text-muted)]">
            保留 {previewList.length} 条，删除 {removedPreviewProxyNames.length} 条。删除项不会进入后续比较环节。
          </p>
          <Table columns={previewColumns} data={previewList} rowKey="proxyId" maxHeight="380px" emptyText="无代理数据" />
        </div>
      </Modal>

      <Modal open={editModalOpen} onClose={() => setEditModalOpen(false)} title="编辑代理" width="500px"
        footer={<><Button variant="secondary" onClick={() => setEditModalOpen(false)}>取消</Button><Button onClick={handleSaveProxy} loading={saving}>保存</Button></>}>
        <div className="space-y-4">
          <FormItem label="代理名称" required>
            <Input
              value={chainEditMode ? chainImportForm.proxyName : editForm.proxyName}
              onChange={e => {
                if (chainEditMode) {
                  setChainImportForm(prev => ({ ...prev, proxyName: e.target.value }))
                } else {
                  setEditForm(prev => ({ ...prev, proxyName: e.target.value }))
                }
              }}
              placeholder="例如：香港节点"
            />
          </FormItem>
          <FormItem label="分组名称（可选）">
            <Input value={editForm.groupName} onChange={e => setEditForm(prev => ({ ...prev, groupName: e.target.value }))} placeholder="例如：香港、美国" list="edit-proxy-groups-datalist" />
            <datalist id="edit-proxy-groups-datalist">
              {groups.map(g => <option key={g} value={g} />)}
            </datalist>
          </FormItem>
          {chainEditMode ? (
            <div className="space-y-3 rounded-md border border-[var(--color-border)] p-3">
              <p className="text-xs text-[var(--color-text-muted)]">
                链式代理会在启动实例时自动桥接为本地 SOCKS5，并以 <code className="px-1 bg-[var(--color-bg-secondary)] rounded">socks5://127.0.0.1:&lt;port&gt;</code> 传给 Chromium。
              </p>
              <FormItem label="本地监听端口（可选）">
                <Input
                  type="number"
                  min={1}
                  max={65535}
                  value={chainImportForm.localPort}
                  onChange={e => setChainImportForm(prev => ({ ...prev, localPort: e.target.value }))}
                  placeholder="留空自动分配"
                />
              </FormItem>
              <div className="rounded-md border border-[var(--color-border)] p-3 space-y-3">
                <h4 className="text-sm font-medium text-[var(--color-text-primary)]">第一层 SOCKS5</h4>
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                  <FormItem label="代理地址" required>
                    <Input value={chainImportForm.first.server} onChange={e => updateChainHop('first', 'server', e.target.value)} />
                  </FormItem>
                  <FormItem label="代理端口" required>
                    <Input type="number" min={1} max={65535} value={chainImportForm.first.port} onChange={e => updateChainHop('first', 'port', e.target.value)} />
                  </FormItem>
                  <FormItem label="账号（可选）">
                    <Input value={chainImportForm.first.username} onChange={e => updateChainHop('first', 'username', e.target.value)} />
                  </FormItem>
                  <FormItem label="密码（可选）">
                    <Input type="password" value={chainImportForm.first.password} onChange={e => updateChainHop('first', 'password', e.target.value)} />
                  </FormItem>
                </div>
              </div>
              <div className="rounded-md border border-[var(--color-border)] p-3 space-y-3">
                <h4 className="text-sm font-medium text-[var(--color-text-primary)]">第二层 SOCKS5</h4>
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                  <FormItem label="代理地址" required>
                    <Input value={chainImportForm.second.server} onChange={e => updateChainHop('second', 'server', e.target.value)} />
                  </FormItem>
                  <FormItem label="代理端口" required>
                    <Input type="number" min={1} max={65535} value={chainImportForm.second.port} onChange={e => updateChainHop('second', 'port', e.target.value)} />
                  </FormItem>
                  <FormItem label="账号（可选）">
                    <Input value={chainImportForm.second.username} onChange={e => updateChainHop('second', 'username', e.target.value)} />
                  </FormItem>
                  <FormItem label="密码（可选）">
                    <Input type="password" value={chainImportForm.second.password} onChange={e => updateChainHop('second', 'password', e.target.value)} />
                  </FormItem>
                </div>
              </div>
            </div>
          ) : (
            <FormItem label="代理配置">
              <Textarea value={editForm.proxyConfig} onChange={e => setEditForm(prev => ({ ...prev, proxyConfig: e.target.value }))} rows={10} placeholder="支持 Clash YAML、http://、https://、socks5://、chain+socks5:// 代理配置" />
            </FormItem>
          )}
          <FormItem label="DNS 服务器（可选）">
            <Textarea value={editForm.dnsServers} onChange={e => setEditForm(prev => ({ ...prev, dnsServers: e.target.value }))} rows={6}
              placeholder={`dns:\n  enable: true\n  nameserver:\n    - 119.29.29.29\n    - 223.5.5.5`} />
            <p className="text-xs text-[var(--color-text-muted)] mt-1">支持 Clash dns: YAML 格式，主要用于 Clash / 桥接代理；直连 HTTP/SOCKS5 通常不会使用这里的 DNS 配置</p>
          </FormItem>
        </div>
      </Modal>

      <Modal
        open={ipHealthDetailOpen}
        onClose={() => setIPHealthDetailOpen(false)}
        title="IP健康原始返回"
        width="760px"
        footer={<Button variant="secondary" onClick={() => setIPHealthDetailOpen(false)}>关闭</Button>}
      >
        <div className="space-y-3">
          {currentIPHealthDetail && (
            <>
              <div className="text-xs text-[var(--color-text-muted)]">
                代理ID：{currentIPHealthDetail.proxyId} | 来源：{currentIPHealthDetail.source} | 时间：{currentIPHealthDetail.updatedAt}
              </div>
              {!currentIPHealthDetail.ok && (
                <div className="text-sm text-red-500">{currentIPHealthDetail.error || '检测失败'}</div>
              )}
              <pre className="max-h-[420px] overflow-auto text-xs leading-5 rounded-lg bg-[var(--color-bg-secondary)] border border-[var(--color-border)] p-3">
                {JSON.stringify(currentIPHealthDetail.rawData || {}, null, 2)}
              </pre>
            </>
          )}
        </div>
      </Modal>

      <ConfirmModal open={deleteConfirmOpen} onClose={() => setDeleteConfirmOpen(false)} onConfirm={handleDeleteConfirm}
        title="确认删除" content="确定要删除这个代理吗？此操作不可恢复。" confirmText="删除" danger />

      <ConfirmModal open={batchDeleteConfirmOpen} onClose={() => setBatchDeleteConfirmOpen(false)} onConfirm={handleBatchDeleteConfirm}
        title="批量删除" content={`确定要删除选中的 ${selectedCount} 个代理吗？此操作不可恢复。`} confirmText="删除" danger />
    </div>
  )
}
