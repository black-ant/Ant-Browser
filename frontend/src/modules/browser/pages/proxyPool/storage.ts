import type { ProxyIPHealthResult } from '../../types'

import type { ClashProxy } from './helpers'
import { normalizeRefreshIntervalM, resolveImportedProxyName } from './helpers'


const PROXY_IP_HEALTH_CACHE_KEY = 'browser:proxyPool:ipHealthMap:v1'
const PROXY_SOURCE_IGNORED_NAMES_KEY = 'browser:proxyPool:sourceIgnoredProxyNames:v1'
const PROXY_GLOBAL_AUTO_REFRESH_KEY = 'browser:proxyPool:globalAutoRefreshEnabled:v1'
const PROXY_GLOBAL_REFRESH_INTERVAL_KEY = 'browser:proxyPool:globalRefreshIntervalM:v1'
// 选择弹窗、实例列表、插件管理页的历史测速结果显示窗口
export const PROXY_SPEED_RESULT_MAX_AGE_MS = 72 * 60 * 60 * 1000
export const PROXY_IP_HEALTH_RESULT_MAX_AGE_MS = 12 * 60 * 60 * 1000


export function isProxySpeedResultFresh(testedAt?: string, now = Date.now(), maxAgeMs = PROXY_SPEED_RESULT_MAX_AGE_MS): boolean {
  if (!testedAt) return false
  const timestamp = Date.parse(testedAt)
  if (!Number.isFinite(timestamp)) return false
  const age = now - timestamp
  const effectiveMaxAgeMs = maxAgeMs > 0 ? maxAgeMs : PROXY_SPEED_RESULT_MAX_AGE_MS
  return age >= 0 && age <= effectiveMaxAgeMs
}

export function readSourceIgnoredProxyNames(): Record<string, string[]> {
  try {
    const raw = localStorage.getItem(PROXY_SOURCE_IGNORED_NAMES_KEY)
    if (!raw) return {}
    const parsed = JSON.parse(raw)
    if (!parsed || typeof parsed !== 'object') return {}

    const cleaned: Record<string, string[]> = {}
    Object.entries(parsed as Record<string, unknown>).forEach(([sourceId, value]) => {
      if (!sourceId.trim() || !Array.isArray(value)) return
      const names = value
        .map((item) => (typeof item === 'string' ? item.trim() : ''))
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
      const validNames = names.map((name) => (name || '').trim()).filter(Boolean)
      if (validNames.length > 0) {
        cleaned[key] = validNames
      }
    })
    localStorage.setItem(PROXY_SOURCE_IGNORED_NAMES_KEY, JSON.stringify(cleaned))
  } catch {
    // ignore write failures
  }
}

export function appendSourceIgnoredProxyNames(sourceId: string, names: string[]) {
  const sourceKey = sourceId.trim()
  if (!sourceKey || names.length === 0) return
  const cleaned = names.map((name) => name.trim()).filter(Boolean)
  if (cleaned.length === 0) return

  const existing = readSourceIgnoredProxyNames()
  existing[sourceKey] = [...(existing[sourceKey] || []), ...cleaned]
  writeSourceIgnoredProxyNames(existing)
}

export function applyIgnoredProxyNamesForSource(
  parsedProxies: ClashProxy[],
  sourceNamePrefix: string,
  ignoredProxyNames: string[],
): ClashProxy[] {
  if (ignoredProxyNames.length === 0) return parsedProxies

  const ignoredCounter = new Map<string, number>()
  ignoredProxyNames.forEach((name) => {
    const key = name.trim()
    if (!key) return
    ignoredCounter.set(key, (ignoredCounter.get(key) || 0) + 1)
  })
  if (ignoredCounter.size === 0) return parsedProxies

  return parsedProxies.filter((proxy, index) => {
    const proxyName = resolveImportedProxyName(proxy, index, sourceNamePrefix)
    const count = ignoredCounter.get(proxyName) || 0
    if (count <= 0) return true
    if (count === 1) {
      ignoredCounter.delete(proxyName)
    } else {
      ignoredCounter.set(proxyName, count - 1)
    }
    return false
  })
}

export function readGlobalRefreshConfig(): { enabled: boolean; intervalM: number } {
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

export function writeGlobalRefreshConfig(enabled: boolean, intervalM: number) {
  try {
    localStorage.setItem(PROXY_GLOBAL_AUTO_REFRESH_KEY, enabled ? '1' : '0')
    localStorage.setItem(PROXY_GLOBAL_REFRESH_INTERVAL_KEY, String(intervalM))
  } catch {
    // ignore write failures
  }
}

export function toLatencyValue(ok: boolean, latencyMs: number, error?: string): number {
  if (ok) return latencyMs
  const message = (error || '').toLowerCase()
  if (message.includes('不支持')) return -3
  if (message.includes('timeout') || message.includes('超时') || message.includes('deadline exceeded') || message.includes('i/o timeout')) return -2
  return -4
}


export function readIPHealthCache(): Record<string, ProxyIPHealthResult> {
  try {
    const raw = localStorage.getItem(PROXY_IP_HEALTH_CACHE_KEY)
    if (!raw) return {}
    const parsed = JSON.parse(raw) as { timestamp?: number; data?: Record<string, ProxyIPHealthResult> }
    if (!parsed?.timestamp || !parsed?.data) return {}
    if (Date.now() - parsed.timestamp > PROXY_IP_HEALTH_RESULT_MAX_AGE_MS) return {}

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

export function writeIPHealthCache(data: Record<string, ProxyIPHealthResult>) {
  try {
    localStorage.setItem(PROXY_IP_HEALTH_CACHE_KEY, JSON.stringify({
      timestamp: Date.now(),
      data,
    }))
  } catch {
    // ignore write failures
  }
}
