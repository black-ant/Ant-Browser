import { useEffect, useMemo, useState } from 'react'
import yaml from 'js-yaml'
import { Button, FormItem, Input, Modal, Select, SideDrawer, Table, Textarea, toast } from '../../../shared/components'
import type { TableColumn } from '../../../shared/components/Table'
import type { BrowserProxy, IPDetectResult, IPDetectSource } from '../types'
import { detectIPByConfig, fetchClashImportFromURL, listIPDetectSources, saveBrowserProxies } from '../api'

interface ProxyImportModalProps {
  open: boolean
  onClose: () => void
  existingProxies: BrowserProxy[]
  groups: string[]
  globalAutoRefreshEnabled?: boolean
  globalRefreshIntervalM?: number
  onImported?: (newProxies: BrowserProxy[]) => void | Promise<void>
}

interface ClashProxy {
  name: string
  type: string
  server: string
  port: number
  [key: string]: any
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
  type: string
  server: string
  port: number
}

const CHAIN_SOCKS5_PREFIX = 'chain+socks5://'

interface ChainSocks5HopConfig {
  protocol: 'socks5'
  server: string
  port: number
  username?: string
  password?: string
}

interface ChainSocks5Config {
  localPort?: number
  first: ChainSocks5HopConfig
  second: ChainSocks5HopConfig
}

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

function parseProxyInfo(proxyConfig: string): { type: string; server: string; port: number } {
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

function resolveImportedProxyName(proxy: ClashProxy, index: number, prefix: string): string {
  const rawName = (proxy.name || '').trim() || `导入代理 ${index + 1}`
  return prefix ? `${prefix}-${rawName}` : rawName
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
      type: info.type || '-',
      server: info.server || '-',
      port: info.port || 0,
    }
  })
}

function normalizeRefreshIntervalM(value: number): number {
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

function nextProxyID(): string {
  return `proxy-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
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

export function ProxyImportModal({
  open,
  onClose,
  existingProxies,
  groups,
  globalAutoRefreshEnabled = false,
  globalRefreshIntervalM = 60,
  onImported,
}: ProxyImportModalProps) {
  const [importMode, setImportMode] = useState<ProxyImportMode>('clash')
  const [importUrl, setImportUrl] = useState('')
  const [importResolvedUrl, setImportResolvedUrl] = useState('')
  const [importText, setImportText] = useState('')
  const [importDnsServers, setImportDnsServers] = useState('')
  const [importNamePrefix, setImportNamePrefix] = useState('')
  const [importGroupName, setImportGroupName] = useState('')
  const [directImportForm, setDirectImportForm] = useState<DirectImportForm>(() => ({ ...INITIAL_DIRECT_IMPORT_FORM }))
  const [chainImportForm, setChainImportForm] = useState<ChainImportForm>(() => ({ ...INITIAL_CHAIN_IMPORT_FORM }))
  const [previewModalOpen, setPreviewModalOpen] = useState(false)
  const [previewList, setPreviewList] = useState<ProxyDisplayInfo[]>([])
  const [importing, setImporting] = useState(false)
  const [fetchingImportUrl, setFetchingImportUrl] = useState(false)
  const [directSmartText, setDirectSmartText] = useState('')
  const [directRefreshUrl, setDirectRefreshUrl] = useState('')
  const [ipDetectSources, setIpDetectSources] = useState<IPDetectSource[]>([])
  const [ipDetectSource, setIpDetectSource] = useState('')
  const [ipDetecting, setIpDetecting] = useState(false)
  const [ipDetectResult, setIpDetectResult] = useState<IPDetectResult | null>(null)

  useEffect(() => {
    if (open) return
    setPreviewModalOpen(false)
  }, [open])

  // 加载可用的出口 IP 检测源（用于 HTTP/SOCKS5 模式的 IP 检测下拉框）
  useEffect(() => {
    if (!open) return
    let cancelled = false
    listIPDetectSources()
      .then(list => {
        if (cancelled || list.length === 0) return
        setIpDetectSources(list)
        setIpDetectSource(prev => prev || list[0].key)
      })
      .catch(() => { /* 忽略：下拉为空时按钮禁用 */ })
    return () => { cancelled = true }
  }, [open])

  const resetImportState = () => {
    setImportMode('clash')
    setImportUrl('')
    setImportResolvedUrl('')
    setImportText('')
    setImportDnsServers('')
    setImportNamePrefix('')
    setImportGroupName('')
    setDirectImportForm({ ...INITIAL_DIRECT_IMPORT_FORM })
    setChainImportForm({ ...INITIAL_CHAIN_IMPORT_FORM })
    setPreviewList([])
    setDirectSmartText('')
    setDirectRefreshUrl('')
    setIpDetectResult(null)
  }

  // 智能识别：解析常见代理串，支持以下形态（刷新 URL 与备注为选填）：
  //   host:port:用户名:密码[刷新URL]{备注}
  //   用户名:密码@host:port[刷新URL]{备注}
  //   protocol://用户名:密码@host:port
  const handleDirectSmartParse = (text: string) => {
    setDirectSmartText(text)
    const raw = text.trim()
    if (!raw) return

    const patch: Partial<DirectImportForm> = {}

    // 备注 {note}
    const noteMatch = raw.match(/\{([^}]*)\}/)
    const noteStripped = noteMatch ? raw.replace(noteMatch[0], '').trim() : raw

    // 刷新 URL [url]
    const urlMatch = noteStripped.match(/\[([^\]]*)\]/)
    if (urlMatch && urlMatch[1].trim()) {
      setDirectRefreshUrl(urlMatch[1].trim())
    }
    let core = urlMatch ? noteStripped.replace(urlMatch[0], '').trim() : noteStripped

    // 协议头 protocol://
    const protoMatch = core.match(/^([a-zA-Z][a-zA-Z0-9+.-]*):\/\//)
    if (protoMatch) {
      const proto = protoMatch[1].toLowerCase()
      if (proto === 'http' || proto === 'https' || proto === 'socks5') {
        patch.protocol = proto
      } else if (proto === 'socks' || proto === 'socket') {
        patch.protocol = 'socks5'
      }
      core = core.slice(protoMatch[0].length).trim()
    }

    let host = ''
    let port = ''
    let username = ''
    let password = ''

    if (core.includes('@')) {
      // 用户名:密码@host:port
      const [auth, hostPart] = core.split('@', 2)
      const authSegs = auth.split(':')
      username = (authSegs[0] || '').trim()
      password = (authSegs[1] || '').trim()
      const hostSegs = hostPart.split(':')
      host = (hostSegs[0] || '').trim()
      port = (hostSegs[1] || '').trim()
    } else {
      // host:port:用户名:密码
      const segs = core.split(':')
      host = (segs[0] || '').trim()
      port = (segs[1] || '').trim()
      username = (segs[2] || '').trim()
      password = (segs[3] || '').trim()
    }

    if (host) patch.server = host
    if (port) patch.port = port
    if (username) patch.username = username
    if (password) patch.password = password

    if (Object.keys(patch).length > 0) {
      setDirectImportForm(prev => ({ ...prev, ...patch }))
    }
  }

  // 通过当前 HTTP/SOCKS5 表单构造一段临时代理配置，并用所选检测源测试出口 IP
  const handleDetectIP = async () => {
    let candidate: ImportCandidate
    try {
      candidate = buildDirectImportCandidate(directImportForm)
    } catch (error: any) {
      toast.error(error?.message || '请先填写有效的代理地址与端口')
      return
    }
    setIpDetecting(true)
    setIpDetectResult(null)
    try {
      const result = await detectIPByConfig(ipDetectSource, candidate.proxyConfig)
      setIpDetectResult(result)
      if (result.ok) {
        toast.success(`检测成功：${result.ip}${result.country ? ` · ${result.country}` : ''}`)
      } else {
        toast.error(result.error || '检测失败')
      }
    } catch (error: any) {
      toast.error(error?.message || '检测失败')
    } finally {
      setIpDetecting(false)
    }
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
      setPreviewList(preview)
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
      const sourceID = isURLImport ? resolveImportSourceID(existingProxies, sourceURL, sourceNamePrefix) : ''
      const sourceAutoRefresh = isURLImport ? !!globalAutoRefreshEnabled : false
      const sourceRefreshIntervalM = sourceAutoRefresh
        ? normalizeRefreshIntervalM(Number(globalRefreshIntervalM || 0))
        : 0
      const sourceLastRefreshAt = isURLImport ? new Date().toISOString() : ''
      const oldSourceProxies = isURLImport
        ? existingProxies.filter(item => (item.sourceId || '').trim() === sourceID)
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
        ? existingProxies.filter(item => (item.sourceId || '').trim() !== sourceID).concat(newProxies)
        : [...existingProxies, ...newProxies]

      await saveBrowserProxies(allProxies)
      await onImported?.(newProxies)
      toast.success(`成功导入 ${newProxies.length} 个代理`)
      setPreviewModalOpen(false)
      resetImportState()
      onClose()
    } catch (error: any) {
      toast.error(error?.message || '导入失败')
    } finally {
      setImporting(false)
    }
  }

  const handleRemovePreviewProxy = (proxyId: string) => {
    setPreviewList(prev => prev.filter(item => item.proxyId !== proxyId))
  }

  const canParseImport = importMode === 'clash'
    ? !!importText.trim()
    : importMode === 'direct'
      ? !!directImportForm.server.trim() && !!directImportForm.port.trim()
      : !!chainImportForm.first.server.trim() && !!chainImportForm.first.port.trim() && !!chainImportForm.second.server.trim() && !!chainImportForm.second.port.trim()

  const previewColumns = useMemo<TableColumn<ProxyDisplayInfo>[]>(() => [
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
  ], [])

  return (
    <>
      <SideDrawer
        open={open}
        onClose={onClose}
        title="导入代理配置"
        width="600px"
        footer={
          <>
            <Button variant="secondary" onClick={onClose} disabled={fetchingImportUrl}>取消</Button>
            <Button onClick={handleParseImport} disabled={fetchingImportUrl || !canParseImport}>解析</Button>
          </>
        }
      >
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
                <p className="text-xs text-[var(--color-text-muted)] mt-1">获取成功后会自动回填 YAML 文本，并尝试自动填充 DNS 与建议分组</p>
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
            <div className="space-y-4">
              <FormItem label="智能识别（支持动态 / 静态代理 IP）">
                <Textarea
                  value={directSmartText}
                  onChange={e => handleDirectSmartParse(e.target.value)}
                  rows={3}
                  placeholder={`示例（刷新 URL 与备注为选填）：\n192.168.0.1:8000:用户名:密码[刷新URL]{备注}\n用户名:密码@192.168.0.1:8000[刷新URL]{备注}`}
                />
                <p className="text-xs text-[var(--color-text-muted)] mt-1">粘贴常见格式的代理串，自动拆分到下方协议 / 地址 / 端口 / 账号 / 密码字段</p>
              </FormItem>
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

              <FormItem label="刷新 URL（可选）">
                <Input
                  value={directRefreshUrl}
                  onChange={e => setDirectRefreshUrl(e.target.value)}
                  placeholder="动态代理的刷新链接，留空表示静态代理"
                />
              </FormItem>

              <FormItem label="IP 检测">
                <div className="flex gap-2">
                  <Select
                    className="flex-1"
                    value={ipDetectSource}
                    onChange={e => setIpDetectSource(e.target.value)}
                    options={ipDetectSources.map(s => ({ value: s.key, label: s.label }))}
                  />
                  <Button
                    variant="secondary"
                    onClick={handleDetectIP}
                    loading={ipDetecting}
                    disabled={!ipDetectSource || !directImportForm.server.trim() || !directImportForm.port.trim()}
                  >
                    测试代理 IP
                  </Button>
                </div>
                {ipDetectResult && (
                  ipDetectResult.ok ? (
                    <p className="text-xs text-[var(--color-success)] mt-1.5 break-all">
                      出口 IP：{ipDetectResult.ip}
                      {ipDetectResult.country ? ` · ${ipDetectResult.country}` : ''}
                      {ipDetectResult.city ? ` / ${ipDetectResult.city}` : ''}
                      {ipDetectResult.org ? ` · ${ipDetectResult.org}` : ''}
                      {ipDetectResult.latencyMs > 0 ? ` · ${ipDetectResult.latencyMs}ms` : ''}
                    </p>
                  ) : (
                    <p className="text-xs text-[var(--color-error)] mt-1.5 break-all">检测失败：{ipDetectResult.error}</p>
                  )
                )}
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
      </SideDrawer>

      <Modal
        open={previewModalOpen}
        onClose={() => setPreviewModalOpen(false)}
        title="确认导入以下代理"
        width="700px"
        footer={
          <>
            <Button variant="secondary" onClick={() => setPreviewModalOpen(false)}>返回修改</Button>
            <Button onClick={handleConfirmImport} loading={importing} disabled={previewList.length === 0}>确认导入</Button>
          </>
        }
      >
        <div className="space-y-3">
          {importMode === 'clash' && importDnsServers.trim() && (
            <p className="text-xs text-[var(--color-text-muted)] bg-[var(--color-bg-secondary)] px-3 py-2 rounded">已配置批量 DNS，将应用到以下所有代理</p>
          )}
          <Table columns={previewColumns} data={previewList} rowKey="proxyId" maxHeight="380px" emptyText="无代理数据" />
        </div>
      </Modal>
    </>
  )
}
