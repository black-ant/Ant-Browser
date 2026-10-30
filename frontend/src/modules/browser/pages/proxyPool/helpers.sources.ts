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
      })
      continue
    }

    if (
      parseTimestampMs(currentLastRefreshAt) > parseTimestampMs(existing.sourceLastRefreshAt) &&
      currentLastRefreshAt.trim()
    ) {
      existing.sourceLastRefreshAt = currentLastRefreshAt
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

