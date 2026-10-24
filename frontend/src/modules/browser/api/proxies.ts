import type { BrowserProxy, ProxyBridgeWarmupResult, ProxyCoreDownloadInfoResult, ProxyCoreStatusResult, ProxyIPHealthResult, ProxyLocationResolveResult, ProxySpeedTestResult } from '../types'
import { getBindings, getGoApp, getMockProxies, nowISOString, setMockProxies } from './runtime'

import type { BrowserProxyVirtualGroup } from '../types'

let mockProxyVirtualGroups: BrowserProxyVirtualGroup[] = []

function normalizeBrowserProxyVirtualGroup(value: unknown): BrowserProxyVirtualGroup | null {
  if (!value || typeof value !== 'object') return null
  const source = value as Record<string, unknown>
  return {
    groupId: typeof source.groupId === 'string' ? source.groupId.trim() : '',
    groupName: typeof source.groupName === 'string' ? source.groupName.trim() : '',
    strategy: source.strategy === 'sticky' ? 'sticky' : 'round_robin',
    selectedProxyId: typeof source.selectedProxyId === 'string' ? source.selectedProxyId.trim() : '',
    memberProxyIds: Array.isArray(source.memberProxyIds)
      ? source.memberProxyIds.filter((item): item is string => typeof item === 'string').map(item => item.trim()).filter(Boolean)
      : [],
    memberGroupNames: Array.isArray(source.memberGroupNames)
      ? source.memberGroupNames.filter((item): item is string => typeof item === 'string').map(item => item.trim()).filter(Boolean)
      : [],
    sortOrder: typeof source.sortOrder === 'number' ? source.sortOrder : 0,
    createdAt: typeof source.createdAt === 'string' ? source.createdAt : undefined,
    updatedAt: typeof source.updatedAt === 'string' ? source.updatedAt : undefined,
  }
}

function normalizeBrowserProxyVirtualGroups(value: unknown): BrowserProxyVirtualGroup[] {
  if (!Array.isArray(value)) return []
  return value.map(normalizeBrowserProxyVirtualGroup).filter((item): item is BrowserProxyVirtualGroup => item !== null)
}

export interface ClashImportURLResult {
  url: string
  content: string
  proxyCount: number
  dnsServers?: string
  suggestedGroup?: string
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

const BACKEND_STARTUP_RETRY_DELAYS_MS = [100, 250, 500, 1000, 2000]

function errorMessage(value: unknown): string {
  if (value instanceof Error) return value.message
  if (typeof value === 'string') return value
  if (!value || typeof value !== 'object') return String(value)

  const source = value as Record<string, unknown>
  for (const key of ['message', 'error', 'detail']) {
    if (typeof source[key] === 'string') return source[key] as string
  }
  return String(value)
}

function isBackendStartupError(value: unknown): boolean {
  const message = errorMessage(value)
  return message.includes('代理数据加载上下文不可用')
    || message.includes('应用未初始化')
    || message.includes('应用上下文未初始化')
}

async function withBackendStartupRetry<T>(operation: () => Promise<T>): Promise<T> {
  for (let attempt = 0; ; attempt += 1) {
    try {
      return await operation()
    } catch (error) {
      const delay = BACKEND_STARTUP_RETRY_DELAYS_MS[attempt]
      if (!isBackendStartupError(error) || delay === undefined) {
        throw error
      }
      await sleep(delay)
    }
  }
}

export async function fetchBrowserProxies(): Promise<BrowserProxy[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProxyList) {
    return (await withBackendStartupRetry(() => bindings.BrowserProxyList())) || []
  }
  return getMockProxies()
}

export async function fetchBrowserProxyGroups(): Promise<string[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProxyListGroups) {
    return (await withBackendStartupRetry(() => bindings.BrowserProxyListGroups())) || []
  }
  return []
}

export async function fetchBrowserProxyVirtualGroups(): Promise<BrowserProxyVirtualGroup[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProxyVirtualGroupList) {
    return normalizeBrowserProxyVirtualGroups(
      await withBackendStartupRetry(() => bindings.BrowserProxyVirtualGroupList()),
    )
  }
  const goApp = getGoApp()
  if (goApp?.BrowserProxyVirtualGroupList) {
    return normalizeBrowserProxyVirtualGroups(
      await withBackendStartupRetry(() => goApp.BrowserProxyVirtualGroupList()),
    )
  }
  return normalizeBrowserProxyVirtualGroups(mockProxyVirtualGroups)
}

export async function saveBrowserProxyVirtualGroup(group: BrowserProxyVirtualGroup): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.SaveBrowserProxyVirtualGroup) {
    await bindings.SaveBrowserProxyVirtualGroup(group)
    return true
  }
  const goApp = getGoApp()
  if (goApp?.SaveBrowserProxyVirtualGroup) {
    await goApp.SaveBrowserProxyVirtualGroup(group)
    return true
  }
  const nextGroup = {
    ...group,
    groupId: group.groupId || `virtual-${Date.now()}`,
    strategy: group.strategy === 'sticky' ? 'sticky' : 'round_robin',
    selectedProxyId: group.strategy === 'sticky' ? (group.selectedProxyId || '').trim() : '',
    memberProxyIds: group.memberProxyIds.map(proxyId => proxyId.trim()).filter(Boolean),
  }
  const existingIndex = mockProxyVirtualGroups.findIndex(item => item.groupId === nextGroup.groupId)
  mockProxyVirtualGroups = existingIndex < 0
    ? [...mockProxyVirtualGroups, nextGroup]
    : mockProxyVirtualGroups.map((item, index) => index === existingIndex ? nextGroup : item)
  return true
}

export async function deleteBrowserProxyVirtualGroup(groupId: string): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.DeleteBrowserProxyVirtualGroup) {
    await bindings.DeleteBrowserProxyVirtualGroup(groupId)
    return true
  }
  const goApp = getGoApp()
  if (goApp?.DeleteBrowserProxyVirtualGroup) {
    await goApp.DeleteBrowserProxyVirtualGroup(groupId)
    return true
  }
  mockProxyVirtualGroups = mockProxyVirtualGroups.filter(group => group.groupId !== groupId)
  return true
}

export async function fetchBrowserProxiesByGroup(groupName: string): Promise<BrowserProxy[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProxyListByGroup) {
    return (await bindings.BrowserProxyListByGroup(groupName)) || []
  }
  return getMockProxies().filter((proxy) => proxy.groupName === groupName)
}

export async function fetchClashImportFromURL(targetURL: string, proxyId = ''): Promise<ClashImportURLResult> {
  const bindings: any = await getBindings()
  const trimmedProxyId = proxyId.trim()
  if (trimmedProxyId && bindings?.BrowserProxyFetchClashByURLWithProxy) {
    return (
      (await bindings.BrowserProxyFetchClashByURLWithProxy(targetURL, trimmedProxyId)) || {
        url: targetURL,
        content: '',
        proxyCount: 0,
      }
    )
  }
  if (bindings?.BrowserProxyFetchClashByURL) {
    return (
      (await bindings.BrowserProxyFetchClashByURL(targetURL)) || {
        url: targetURL,
        content: '',
        proxyCount: 0,
      }
    )
  }

  const goApp = getGoApp()
  if (trimmedProxyId && goApp?.BrowserProxyFetchClashByURLWithProxy) {
    return (
      (await goApp.BrowserProxyFetchClashByURLWithProxy(targetURL, trimmedProxyId)) || {
        url: targetURL,
        content: '',
        proxyCount: 0,
      }
    )
  }
  if (goApp?.BrowserProxyFetchClashByURL) {
    return (
      (await goApp.BrowserProxyFetchClashByURL(targetURL)) || {
        url: targetURL,
        content: '',
        proxyCount: 0,
      }
    )
  }

  throw new Error('当前环境不支持 URL 导入 Clash 配置')
}

export async function saveBrowserProxies(proxies: BrowserProxy[]): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.SaveBrowserProxies) {
    await bindings.SaveBrowserProxies(proxies)
    return true
  }
  setMockProxies(proxies)
  return true
}

export async function validateProxyConfig(proxyConfig: string, proxyId: string): Promise<{ supported: boolean; errorMsg: string }> {
  const bindings: any = await getBindings()
  if (bindings?.ValidateProxyConfig) {
    return (await bindings.ValidateProxyConfig(proxyConfig, proxyId)) || { supported: true, errorMsg: '' }
  }
  return { supported: true, errorMsg: '' }
}

export async function testProxyConnectivity(proxyId: string, proxyConfig: string): Promise<ProxySpeedTestResult> {
  const bindings: any = await getBindings()
  if (bindings?.TestProxyConnectivity) {
    return (await bindings.TestProxyConnectivity(proxyId, proxyConfig)) || { proxyId, ok: false, latencyMs: 0, engine: 'unknown', error: '调用失败' }
  }
  await sleep(300 + Math.random() * 500)
  return { proxyId, ok: true, latencyMs: Math.floor(100 + Math.random() * 200), engine: 'mock', error: '' }
}

export async function testProxyRealConnectivity(proxyId: string): Promise<ProxySpeedTestResult> {
  const bindings: any = await getBindings()
  if (bindings?.TestProxyRealConnectivity) {
    return (await bindings.TestProxyRealConnectivity(proxyId)) || { proxyId, ok: false, latencyMs: 0, engine: 'unknown', error: '调用失败' }
  }
  await sleep(300 + Math.random() * 500)
  return { proxyId, ok: true, latencyMs: Math.floor(100 + Math.random() * 400), engine: 'mock', error: '' }
}

export async function browserProxyTestSpeed(proxyId: string): Promise<ProxySpeedTestResult> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProxyTestSpeed) {
    return (await bindings.BrowserProxyTestSpeed(proxyId)) || { proxyId, ok: false, latencyMs: 0, engine: 'unknown', error: '调用失败' }
  }
  await sleep(300 + Math.random() * 500)
  return { proxyId, ok: true, latencyMs: Math.floor(100 + Math.random() * 400), engine: 'mock', error: '' }
}

export async function browserProxyBatchTestSpeed(proxyIds: string[], concurrency: number = 20): Promise<ProxySpeedTestResult[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProxyBatchTestSpeed) {
    return (await bindings.BrowserProxyBatchTestSpeed(proxyIds, concurrency)) || []
  }
  await sleep(1000)
  return proxyIds.map((proxyId) => ({ proxyId, ok: true, latencyMs: Math.floor(100 + Math.random() * 400), engine: 'mock', error: '' }))
}

export async function browserProxyWarmupBridge(proxyId: string): Promise<ProxyBridgeWarmupResult> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProxyWarmupBridge) {
    return (await bindings.BrowserProxyWarmupBridge(proxyId)) || {
      proxyId,
      ok: false,
      engine: '',
      socksUrl: '',
      latencyMs: 0,
      error: '调用失败',
    }
  }
  await sleep(200)
  return { proxyId, ok: true, engine: 'mock', socksUrl: '', latencyMs: 0, error: '' }
}

export async function browserProxyWarmupBridgeWithConfig(proxyId: string, proxyConfig: string): Promise<ProxyBridgeWarmupResult> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProxyWarmupBridgeWithConfig) {
    return (await bindings.BrowserProxyWarmupBridgeWithConfig(proxyId, proxyConfig)) || {
      proxyId,
      ok: false,
      engine: '',
      socksUrl: '',
      latencyMs: 0,
      error: '调用失败',
    }
  }
  return browserProxyWarmupBridge(proxyId)
}

export async function browserProxyBatchWarmupBridge(proxyIds: string[], concurrency: number = 5): Promise<ProxyBridgeWarmupResult[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProxyBatchWarmupBridge) {
    return (await bindings.BrowserProxyBatchWarmupBridge(proxyIds, concurrency)) || []
  }
  await sleep(400)
  return proxyIds.map((proxyId) => ({ proxyId, ok: true, engine: 'mock', socksUrl: '', latencyMs: 0, error: '' }))
}

export async function browserProxyCheckIPHealth(proxyId: string): Promise<ProxyIPHealthResult> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProxyCheckIPHealth) {
    return (
      (await bindings.BrowserProxyCheckIPHealth(proxyId)) || {
        proxyId,
        ok: false,
        source: 'ip_health',
        error: '调用失败',
        ip: '',
        fraudScore: 0,
        isResidential: false,
        isBroadcast: false,
        country: '',
        region: '',
        city: '',
        asOrganization: '',
        rawData: {},
        updatedAt: nowISOString(),
      }
    )
  }

  await sleep(600)
  return {
    proxyId,
    ok: true,
    source: 'ip_health',
    error: '',
    ip: '127.0.0.1',
    fraudScore: Math.floor(Math.random() * 100),
    isResidential: Math.random() > 0.5,
    isBroadcast: false,
    country: 'Mock',
    region: 'Mock',
    city: 'Mock',
    asOrganization: 'Mock ISP',
    rawData: {},
    updatedAt: nowISOString(),
  }
}

export async function browserProxyResolveLocation(proxyId: string): Promise<ProxyLocationResolveResult> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProxyResolveLocation) {
    return (await bindings.BrowserProxyResolveLocation(proxyId)) || {
      proxyId,
      ok: false,
      auto: false,
      source: 'location',
      error: '调用失败',
      ip: '',
      country: '',
      region: '',
      city: '',
      timezone: '',
      lang: '',
      resolvedAt: nowISOString(),
    }
  }

  await sleep(400)
  return {
    proxyId,
    ok: true,
    auto: true,
    source: 'mock',
    error: '',
    ip: '127.0.0.1',
    country: 'US',
    region: 'New York',
    city: 'New York',
    timezone: 'America/New_York',
    lang: 'en-US',
    resolvedAt: nowISOString(),
  }
}

export async function browserProxyBatchCheckIPHealth(proxyIds: string[], concurrency: number = 10): Promise<ProxyIPHealthResult[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProxyBatchCheckIPHealth) {
    return (await bindings.BrowserProxyBatchCheckIPHealth(proxyIds, concurrency)) || []
  }

  await sleep(1200)
  return proxyIds.map((proxyId) => ({
    proxyId,
    ok: true,
    source: 'ip_health',
    error: '',
    ip: '127.0.0.1',
    fraudScore: Math.floor(Math.random() * 100),
    isResidential: Math.random() > 0.5,
    isBroadcast: false,
    country: 'Mock',
    region: 'Mock',
    city: 'Mock',
    asOrganization: 'Mock ISP',
    rawData: {},
    updatedAt: nowISOString(),
  }))
}

export async function browserProxyCoreDownload(core: string, goos: string, goarch: string, proxyConfig = ''): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProxyCoreDownload) {
    await bindings.BrowserProxyCoreDownload({ core, goos, goarch, proxyConfig })
    return true
  }
  return false
}

export async function browserProxyCoreStatus(core: string, goos: string, goarch: string): Promise<ProxyCoreStatusResult> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProxyCoreStatus) {
    return (await bindings.BrowserProxyCoreStatus({ core, goos, goarch })) || {
      core, goos, goarch, installed: false, configured: false, active: false, binaryPath: '', source: '', message: '状态查询失败',
    }
  }
  return { core, goos, goarch, installed: false, configured: false, active: false, binaryPath: '', source: '', message: '未连接后端' }
}

export async function browserProxyCoreDownloadInfo(core: string, goos: string, goarch: string, proxyConfig = ''): Promise<ProxyCoreDownloadInfoResult> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProxyCoreDownloadInfo) {
    return (await bindings.BrowserProxyCoreDownloadInfo({ core, goos, goarch, proxyConfig })) || {
      core, goos, goarch, version: '', repo: '', releaseUrl: '', downloadUrl: '', assetName: '', installDir: '', binaryName: '', message: '下载信息查询失败',
    }
  }
  return { core, goos, goarch, version: '', repo: '', releaseUrl: '', downloadUrl: '', assetName: '', installDir: '', binaryName: '', message: '未连接后端' }
}

export async function browserProxyCoreOpenLocal(core: string, goos: string, goarch: string): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProxyCoreOpenLocal) {
    await bindings.BrowserProxyCoreOpenLocal({ core, goos, goarch })
    return true
  }
  return false
}
