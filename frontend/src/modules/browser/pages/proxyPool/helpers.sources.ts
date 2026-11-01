import type { BrowserProxy } from '../../types'
import type { URLImportSourceMeta } from './helpers.types'

export function parseTimestampMs(value: string): number {
  const trimmed = value.trim()
  if (!trimmed) return 0
  const timestamp = Date.parse(trimmed)
  return Number.isFinite(timestamp) ? timestamp : 0
}

export function normalizeRefreshIntervalM(value: number): number {
  if (!Number.isFinite(value)) return 0
  if (value <= 0) return 0
  if (value < 5) return 5
  if (value > 24 * 60) return 24 * 60
  return Math.round(value)
}

export function sourceHostLabel(sourceURL: string): string {
  const raw = sourceURL.trim()
  if (!raw) return ''
  try {
    const parsed = new URL(raw)
    return parsed.host || raw
  } catch {
    return raw
  }
}

function normalizeSourceURL(sourceURL: string): string {
  const raw = sourceURL.trim()
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
  for (let index = 0; index < key.length; index += 1) {
    hash = ((hash << 5) + hash) ^ key.charCodeAt(index)
  }
  return `src-${(hash >>> 0).toString(36)}`
}

export function resolveImportSourceID(list: BrowserProxy[], sourceURL: string, sourceNamePrefix: string): string {
  const normalizedURL = normalizeSourceURL(sourceURL)
  const normalizedPrefix = sourceNamePrefix.trim()
  const existing = list.find((item) =>
    normalizeSourceURL(item.sourceUrl || '') === normalizedURL &&
    (item.sourceNamePrefix || '').trim() === normalizedPrefix &&
    (item.sourceId || '').trim() !== '',
  )
  if (existing?.sourceId?.trim()) {
    return existing.sourceId.trim()
  }
  return buildStableSourceID(sourceURL, sourceNamePrefix)
}

export function collectURLImportSources(list: BrowserProxy[]): URLImportSourceMeta[] {
  const sourceMap = new Map<string, URLImportSourceMeta>()
  for (const item of list) {
    const sourceId = (item.sourceId || '').trim()
    const sourceUrl = (item.sourceUrl || '').trim()
    if (!sourceId || !sourceUrl) continue

    const existing = sourceMap.get(sourceId)
    const currentLastRefreshAt = item.sourceLastRefreshAt || ''
    if (!existing) {
      sourceMap.set(sourceId, {
        sourceId,
        sourceUrl,
        sourceNamePrefix: (item.sourceNamePrefix || '').trim(),
        sourceGroupName: (item.groupName || '').trim(),
        sourceDnsServers: (item.dnsServers || '').trim(),
        sourceAutoRefresh: !!item.sourceAutoRefresh,
        sourceRefreshIntervalM: normalizeRefreshIntervalM(Number(item.sourceRefreshIntervalM || 0)),
        sourceLastRefreshAt: currentLastRefreshAt,
        sourceFetchProxyId: (item.sourceFetchProxyId || '').trim(),
        sourceLastRefreshError: (item.sourceLastRefreshError || '').trim(),
      })
      continue
    }

    if (
      parseTimestampMs(currentLastRefreshAt) > parseTimestampMs(existing.sourceLastRefreshAt) &&
      currentLastRefreshAt.trim()
    ) {
      existing.sourceLastRefreshAt = currentLastRefreshAt
    }
    if (!existing.sourceFetchProxyId) {
      existing.sourceFetchProxyId = (item.sourceFetchProxyId || '').trim()
    }
    if (!existing.sourceLastRefreshError) {
      existing.sourceLastRefreshError = (item.sourceLastRefreshError || '').trim()
    }
  }
  return Array.from(sourceMap.values())
}
export function createExistingProxyPicker(oldSourceProxies: BrowserProxy[]) {
  const exactMap = new Map<string, BrowserProxy[]>()
  const nameMap = new Map<string, BrowserProxy[]>()

  oldSourceProxies.forEach((item) => {
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

// 与后端订阅刷新语义一致（buildRefreshedProxyPool 第三级规则）：
// 名称匹配不上的旧节点不删除，保留在池内并标记异常（实例绑定不动），
// 订阅里重新出现时自动恢复。代理池导入与 ProxyPicker 导入两条路径共用，
// overrides 用于把该源新配置（开关/间隔/拉取代理）同步到保留行，
// 避免"同一源两套配置"让后端聚合出不确定的值。
export function buildKeptMissingSourceProxies(
  oldSourceProxies: BrowserProxy[],
  refreshedNames: Set<string>,
  overrides: Partial<BrowserProxy> = {},
): BrowserProxy[] {
  return oldSourceProxies
    .filter(item => !refreshedNames.has((item.proxyName || '').trim()))
    .map(item => ({
      ...item,
      sourceMissing: true,
      sourceMissingSince: item.sourceMissingSince || new Date().toISOString(),
      ...overrides,
    }))
}


// ===== 对齐工作台：订阅改名后的人工核对与合并 =====
// 背景：订阅方改节点名时，自动刷新按名字匹配会失败——旧节点标异常、新节点新增，
// 出现"新旧两代并存"。此工作台把上一轮人工清洗动作产品化：
// 展示 之前/最新/已对齐，允许人工把旧节点与新节点配对合并（旧 ID 保留，实例绑定不动）。

export interface AlignSubNode {
  poolName: string   // 按源前缀规则命名后的池内名字
  config: string     // 订阅节点的完整 YAML 配置
  endpoint: string   // server:port:type，用于自动建议配对
}

export interface SourceAlignment {
  aligned: Array<{ row: BrowserProxy; sub: AlignSubNode }>
  remainingOld: BrowserProxy[]
  remainingNew: AlignSubNode[]
  suggestions: Record<string, string>   // 旧 proxyId -> 建议配对的新 poolName（端点相同）
}

export function configEndpoint(config: string): string {
  const host = /server:\s*([^\s]+)/.exec(config || '')?.[1] || ''
  const port = /port:\s*(\d+)/.exec(config || '')?.[1] || ''
  const type = /type:\s*([^\s]+)/.exec(config || '')?.[1] || ''
  return host && port ? `${host}:${port}:${type}` : ''
}

export function buildSourceAlignment(sourceRows: BrowserProxy[], subNodes: AlignSubNode[]): SourceAlignment {
  const subByName = new Map(subNodes.map(node => [node.poolName, node]))
  const used = new Set<string>()
  const aligned: SourceAlignment['aligned'] = []
  const remainingOld: BrowserProxy[] = []
  sourceRows.forEach(row => {
    const hit = subByName.get((row.proxyName || '').trim())
    if (hit && !used.has(hit.poolName)) {
      used.add(hit.poolName)
      aligned.push({ row, sub: hit })
    } else {
      remainingOld.push(row)
    }
  })
  const remainingNew = subNodes.filter(node => !used.has(node.poolName))
  const newByEndpoint = new Map<string, AlignSubNode>()
  remainingNew.forEach(node => {
    if (node.endpoint && !newByEndpoint.has(node.endpoint)) newByEndpoint.set(node.endpoint, node)
  })
  const suggestions: Record<string, string> = {}
  const taken = new Set<string>()
  remainingOld.forEach(row => {
    const endpoint = configEndpoint(row.proxyConfig)
    const hit = endpoint ? newByEndpoint.get(endpoint) : undefined
    if (hit && !taken.has(hit.poolName)) {
      suggestions[row.proxyId] = hit.poolName
      taken.add(hit.poolName)
    }
  })
  return { aligned, remainingOld, remainingNew, suggestions }
}

// 应用对齐结果：配对的旧行吸收新节点（名字/配置更新为新版，旧 ID 保留 → 实例绑定不动），
// 未配对旧行保持异常，订阅中未配对的新节点按正常刷新语义新增。
export function applySourceAlignment(
  proxies: BrowserProxy[],
  source: URLImportSourceMeta,
  alignment: SourceAlignment,
  manualPairs: Record<string, AlignSubNode>,
  nowISO: string,
  generateID: () => string,
): BrowserProxy[] {
  const mergeByName = new Map<string, AlignSubNode>()
  alignment.aligned.forEach(item => mergeByName.set(item.row.proxyId, item.sub))
  Object.entries(manualPairs).forEach(([oldId, sub]) => {
    if (sub) mergeByName.set(oldId, sub)
  })
  const usedNew = new Set(Array.from(mergeByName.values()).map(node => node.poolName))

  const others: BrowserProxy[] = []
  const sourceUpdated: BrowserProxy[] = []
  let firstDns = ''
  proxies.forEach(row => {
    if ((row.sourceId || '').trim() !== source.sourceId) {
      others.push(row)
      return
    }
    if (!firstDns) firstDns = row.dnsServers || ''
    const sub = mergeByName.get(row.proxyId)
    if (sub) {
      sourceUpdated.push({
        ...row,
        proxyName: sub.poolName,
        proxyConfig: sub.config,
        sourceMissing: false,
        sourceMissingSince: '',
        sourceLastRefreshError: '',
        sourceLastRefreshAt: nowISO,
      })
    } else {
      sourceUpdated.push({
        ...row,
        sourceMissing: true,
        sourceMissingSince: row.sourceMissingSince || nowISO,
        sourceLastRefreshAt: nowISO,
      })
    }
  })
  const added: BrowserProxy[] = alignment.remainingNew
    .filter(node => !usedNew.has(node.poolName))
    .map(node => ({
      proxyId: generateID(),
      proxyName: node.poolName,
      proxyConfig: node.config,
      groupName: source.sourceGroupName || '',
      dnsServers: firstDns,
      sourceId: source.sourceId,
      sourceUrl: source.sourceUrl,
      sourceNamePrefix: source.sourceNamePrefix,
      sourceAutoRefresh: source.sourceAutoRefresh,
      sourceRefreshIntervalM: source.sourceRefreshIntervalM,
      sourceFetchProxyId: source.sourceFetchProxyId || undefined,
      sourceLastRefreshAt: nowISO,
    }))
  return [...others, ...sourceUpdated, ...added]
}
