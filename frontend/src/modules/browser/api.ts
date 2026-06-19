import type { BrowserProfile, BrowserProfileInput, BrowserTab, BrowserSettings, BrowserCore, BrowserCoreInput, BrowserCoreValidateResult, BrowserProxy, BrowserCoreExtended, CookieInfo, SnapshotInfo, BrowserBookmark, BrowserGroup, BrowserGroupInput, BrowserGroupWithCount, ProxyIPHealthResult, ProxySource, ProxySourceOverride } from './types'

const getBindings = async () => {
  try {
    return await import('../../wailsjs/go/main/App')
  } catch {
    return null
  }
}

// callBinding 解析并调用一个 Wails 绑定方法：优先用生成模块，回落到 window.go.main.App，
// 以便在 wailsjs 尚未重新生成时仍能调用新增的后端方法。
async function callBinding<T>(method: string, args: any[], fallback: T): Promise<T> {
  const bindings: any = await getBindings()
  if (bindings?.[method]) {
    return ((await bindings[method](...args)) as T) ?? fallback
  }
  const goApp = (window as any).go?.main?.App
  if (goApp?.[method]) {
    return ((await goApp[method](...args)) as T) ?? fallback
  }
  return fallback
}

let mockProfiles: BrowserProfile[] = [
  {
    profileId: 'mock-1',
    profileName: '默认指纹配置',
    userDataDir: 'data/default',
    coreId: 'default',
    fingerprintArgs: ['--fingerprint-brand=Chrome', '--fingerprint-platform=windows'],
    proxyId: '',
    proxyConfig: '',
    launchArgs: ['--disable-features=Translate'],
    tags: ['默认'],
    keywords: [],
    running: false,
    debugPort: 0,
    debugReady: false,
    pid: 0,
    runtimeWarning: '',
    lastError: '',
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
  },
]

let mockCores: BrowserCore[] = []

let mockProxies: BrowserProxy[] = []

// ============================================================================
// Profile API
// ============================================================================

export async function fetchBrowserProfiles(): Promise<BrowserProfile[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProfileList) {
    return (await bindings.BrowserProfileList()) || []
  }
  return mockProfiles
}

export async function fetchBrowserProfilesByTag(tag: string): Promise<BrowserProfile[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProfileListByTag) {
    return (await bindings.BrowserProfileListByTag(tag)) || []
  }
  return mockProfiles.filter(p => p.tags?.includes(tag))
}

export async function fetchAllTags(): Promise<string[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserGetAllTags) {
    return (await bindings.BrowserGetAllTags()) || []
  }
  const set = new Set<string>()
  mockProfiles.forEach(p => p.tags?.forEach(t => set.add(t)))
  return Array.from(set).sort()
}

export async function createBrowserProfile(input: BrowserProfileInput): Promise<BrowserProfile | null> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProfileCreate) {
    return (await bindings.BrowserProfileCreate(input)) || null
  }
  const profile: BrowserProfile = {
    profileId: `mock-${Date.now()}`,
    ...input,
    keywords: input.keywords || [],
    running: false,
    debugPort: 0,
    debugReady: false,
    pid: 0,
    runtimeWarning: '',
    lastError: '',
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
  }
  mockProfiles = [profile, ...mockProfiles]
  return profile
}

export async function updateBrowserProfile(profileId: string, input: BrowserProfileInput): Promise<BrowserProfile | null> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProfileUpdate) {
    return (await bindings.BrowserProfileUpdate(profileId, input)) || null
  }
  const index = mockProfiles.findIndex(item => item.profileId === profileId)
  if (index === -1) return null
  mockProfiles[index] = { ...mockProfiles[index], ...input, updatedAt: new Date().toISOString() }
  return mockProfiles[index]
}

export async function deleteBrowserProfile(profileId: string): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProfileDelete) {
    await bindings.BrowserProfileDelete(profileId)
    return true
  }
  mockProfiles = mockProfiles.filter(item => item.profileId !== profileId)
  return true
}

export async function copyBrowserProfile(profileId: string, newName: string): Promise<BrowserProfile | null> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProfileCopy) {
    return (await bindings.BrowserProfileCopy(profileId, newName)) || null
  }
  // mock
  const src = mockProfiles.find(p => p.profileId === profileId)
  if (!src) return null
  const copy: BrowserProfile = {
    ...src,
    profileId: `mock-${Date.now()}`,
    profileName: newName || src.profileName + ' (副本)',
    userDataDir: `mock-${Date.now()}`,
    running: false,
    debugReady: false,
    runtimeWarning: '',
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
  }
  mockProfiles = [copy, ...mockProfiles]
  return copy
}

// ============================================================================
// Instance API
// ============================================================================

export async function startBrowserInstance(profileId: string): Promise<BrowserProfile | null> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserInstanceStart) {
    return (await bindings.BrowserInstanceStart(profileId)) || null
  }
  mockProfiles = mockProfiles.map(item =>
    item.profileId === profileId ? { ...item, running: true, debugPort: 9222, debugReady: true, pid: Math.floor(Math.random() * 100000), runtimeWarning: '', lastStartAt: new Date().toISOString() } : item
  )
  return mockProfiles.find(item => item.profileId === profileId) || null
}

export async function startBrowserInstanceByCode(code: string): Promise<BrowserProfile | null> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserInstanceStartByCode) {
    return (await bindings.BrowserInstanceStartByCode(code)) || null
  }
  const normalized = code.trim().toUpperCase()
  const profile = mockProfiles.find(item => (item.launchCode || '').toUpperCase() === normalized)
  if (!profile) {
    throw new Error('launch code not found')
  }
  return await startBrowserInstance(profile.profileId)
}

export async function stopBrowserInstance(profileId: string): Promise<BrowserProfile | null> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserInstanceStop) {
    return (await bindings.BrowserInstanceStop(profileId)) || null
  }
  mockProfiles = mockProfiles.map(item =>
    item.profileId === profileId ? { ...item, running: false, debugReady: false, debugPort: 0, pid: 0, runtimeWarning: '', lastStopAt: new Date().toISOString() } : item
  )
  return mockProfiles.find(item => item.profileId === profileId) || null
}

export async function restartBrowserInstance(profileId: string): Promise<BrowserProfile | null> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserInstanceRestart) {
    return (await bindings.BrowserInstanceRestart(profileId)) || null
  }
  await stopBrowserInstance(profileId)
  return await startBrowserInstance(profileId)
}

export async function openBrowserUrl(profileId: string, targetUrl: string): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserInstanceOpenUrl) {
    return (await bindings.BrowserInstanceOpenUrl(profileId, targetUrl)) === true
  }
  return true
}

export async function fetchBrowserTabs(profileId: string): Promise<BrowserTab[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserInstanceGetTabs) {
    return (await bindings.BrowserInstanceGetTabs(profileId)) || []
  }
  return [
    { tabId: 'tab-1', title: '新标签页', url: 'about:blank', active: true },
    { tabId: 'tab-2', title: '示例站点', url: 'https://example.com', active: false },
  ]
}

// ============================================================================
// Settings API
// ============================================================================

export async function fetchBrowserSettings(): Promise<BrowserSettings> {
  const bindings: any = await getBindings()
  if (bindings?.GetBrowserSettings) {
    return (await bindings.GetBrowserSettings()) || { userDataRoot: 'data', defaultFingerprintArgs: [], defaultLaunchArgs: [], defaultProxy: '', startReadyTimeoutMs: 3000, startStableWindowMs: 1200 }
  }
  return { userDataRoot: 'data', defaultFingerprintArgs: [], defaultLaunchArgs: [], defaultProxy: '', startReadyTimeoutMs: 3000, startStableWindowMs: 1200 }
}

export async function saveBrowserSettings(settings: BrowserSettings): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.SaveBrowserSettings) {
    await bindings.SaveBrowserSettings(settings)
    return true
  }
  return true
}

// ============================================================================
// Core API
// ============================================================================

export async function fetchBrowserCores(): Promise<BrowserCore[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserCoreList) {
    return (await bindings.BrowserCoreList()) || []
  }
  return mockCores
}

export async function saveBrowserCore(input: BrowserCoreInput): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserCoreSave) {
    await bindings.BrowserCoreSave(input)
    return true
  }
  const index = mockCores.findIndex(c => c.coreId === input.coreId)
  if (index >= 0) {
    mockCores[index] = input
  } else {
    mockCores.push({ ...input, coreId: input.coreId || `core-${Date.now()}` })
  }
  return true
}

export async function deleteBrowserCore(coreId: string): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserCoreDelete) {
    await bindings.BrowserCoreDelete(coreId)
    return true
  }
  mockCores = mockCores.filter(c => c.coreId !== coreId)
  return true
}

export async function setDefaultBrowserCore(coreId: string): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserCoreSetDefault) {
    await bindings.BrowserCoreSetDefault(coreId)
    return true
  }
  mockCores = mockCores.map(c => ({ ...c, isDefault: c.coreId === coreId }))
  return true
}

export async function validateBrowserCorePath(corePath: string): Promise<BrowserCoreValidateResult> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserCoreValidate) {
    return (await bindings.BrowserCoreValidate(corePath)) || { valid: false, message: '验证失败' }
  }
  return { valid: true, message: '路径有效（模拟）' }
}

export async function fetchCoreExtendedInfo(): Promise<BrowserCoreExtended[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserCoreExtendedInfo) {
    return (await bindings.BrowserCoreExtendedInfo()) || []
  }
  return []
}

export async function scanBrowserCores(): Promise<BrowserCore[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserCoreScan) {
    return (await bindings.BrowserCoreScan()) || []
  }
  return mockCores
}

export async function BrowserCoreDownload(coreName: string, url: string, proxyConfig?: string): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserCoreDownload) {
    await bindings.BrowserCoreDownload(coreName, url, proxyConfig || '')
    return true
  }
  return true
}

// ============================================================================
// Proxy API
// ============================================================================

export async function fetchBrowserProxies(): Promise<BrowserProxy[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProxyList) {
    return (await bindings.BrowserProxyList()) || []
  }
  return mockProxies
}

export async function fetchBrowserProxyGroups(): Promise<string[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProxyListGroups) {
    return (await bindings.BrowserProxyListGroups()) || []
  }
  return []
}

export async function fetchBrowserProxiesByGroup(groupName: string): Promise<BrowserProxy[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProxyListByGroup) {
    return (await bindings.BrowserProxyListByGroup(groupName)) || []
  }
  return mockProxies.filter(p => p.groupName === groupName)
}

export interface ClashImportURLResult {
  url: string
  content: string
  proxyCount: number
  dnsServers?: string
  suggestedGroup?: string
}

export async function fetchClashImportFromURL(targetURL: string): Promise<ClashImportURLResult> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProxyFetchClashByURL) {
    return (await bindings.BrowserProxyFetchClashByURL(targetURL)) || {
      url: targetURL,
      content: '',
      proxyCount: 0,
    }
  }

  // 兜底：wailsjs 尚未刷新时，直接通过 window.go 调用后端绑定
  const goApp = (window as any).go?.main?.App
  if (goApp?.BrowserProxyFetchClashByURL) {
    return (await goApp.BrowserProxyFetchClashByURL(targetURL)) || {
      url: targetURL,
      content: '',
      proxyCount: 0,
    }
  }

  throw new Error('当前环境不支持 URL 导入 Clash 配置')
}

// ============================================================================
// Proxy Source（订阅源后端建模）API
// ============================================================================

export async function fetchProxySources(): Promise<ProxySource[]> {
  return callBinding<ProxySource[]>('BrowserProxySourceList', [], [])
}

export async function fetchProxySourceOverrides(sourceId: string): Promise<ProxySourceOverride[]> {
  return callBinding<ProxySourceOverride[]>('BrowserProxySourceListOverrides', [sourceId], [])
}

export async function upsertProxySource(source: ProxySource): Promise<ProxySource | null> {
  return callBinding<ProxySource | null>('BrowserProxySourceUpsert', [source], null)
}

export async function deleteProxySource(sourceId: string, deleteProxies: boolean): Promise<void> {
  await callBinding<void>('BrowserProxySourceDelete', [sourceId, deleteProxies], undefined as any)
}

export async function refreshProxySource(sourceId: string): Promise<void> {
  await callBinding<void>('BrowserProxySourceRefresh', [sourceId], undefined as any)
}

export async function refreshAllProxySources(): Promise<void> {
  await callBinding<void>('BrowserProxySourceRefreshAll', [], undefined as any)
}

export async function setProxySourceOverride(sourceId: string, nodeKey: string, action: 'ignore' | 'rename', customName = ''): Promise<void> {
  await callBinding<void>('BrowserProxySourceSetOverride', [sourceId, nodeKey, action, customName], undefined as any)
}

export async function removeProxySourceOverride(sourceId: string, nodeKey: string): Promise<void> {
  await callBinding<void>('BrowserProxySourceRemoveOverride', [sourceId, nodeKey], undefined as any)
}

// cancelProxyBatch 取消正在进行的批量测速 / IP 健康检测
export async function cancelProxyBatch(): Promise<void> {
  await callBinding<void>('BrowserProxyCancelBatch', [], undefined as any)
}

export async function saveBrowserProxies(proxies: BrowserProxy[]): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.SaveBrowserProxies) {
    await bindings.SaveBrowserProxies(proxies)
    return true
  }
  mockProxies = proxies
  return true
}

export async function validateProxyConfig(proxyConfig: string, proxyId: string): Promise<{ supported: boolean; errorMsg: string }> {
  const bindings: any = await getBindings()
  if (bindings?.ValidateProxyConfig) {
    return (await bindings.ValidateProxyConfig(proxyConfig, proxyId)) || { supported: true, errorMsg: '' }
  }
  return { supported: true, errorMsg: '' }
}

export async function testProxyConnectivity(proxyId: string, proxyConfig: string): Promise<{ proxyId: string; ok: boolean; latencyMs: number; error: string }> {
  const bindings: any = await getBindings()
  if (bindings?.TestProxyConnectivity) {
    return (await bindings.TestProxyConnectivity(proxyId, proxyConfig)) || { proxyId, ok: false, latencyMs: 0, error: '调用失败' }
  }
  // mock: simulate latency
  await new Promise(r => setTimeout(r, 300 + Math.random() * 500))
  return { proxyId, ok: true, latencyMs: Math.floor(100 + Math.random() * 200), error: '' }
}

export async function testProxyRealConnectivity(proxyId: string): Promise<{ proxyId: string; ok: boolean; latencyMs: number; error: string }> {
  const bindings: any = await getBindings()
  if (bindings?.TestProxyRealConnectivity) {
    return (await bindings.TestProxyRealConnectivity(proxyId)) || { proxyId, ok: false, latencyMs: 0, error: '调用失败' }
  }
  // mock: simulate latency 300-800ms
  await new Promise(r => setTimeout(r, 300 + Math.random() * 500))
  return { proxyId, ok: true, latencyMs: Math.floor(100 + Math.random() * 400), error: '' }
}

export async function browserProxyTestSpeed(proxyId: string): Promise<{ proxyId: string; ok: boolean; latencyMs: number; error: string }> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProxyTestSpeed) {
    return (await bindings.BrowserProxyTestSpeed(proxyId)) || { proxyId, ok: false, latencyMs: 0, error: '调用失败' }
  }
  await new Promise(r => setTimeout(r, 300 + Math.random() * 500))
  return { proxyId, ok: true, latencyMs: Math.floor(100 + Math.random() * 400), error: '' }
}

export async function browserProxyBatchTestSpeed(proxyIds: string[], concurrency: number = 20): Promise<{ proxyId: string; ok: boolean; latencyMs: number; error: string }[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProxyBatchTestSpeed) {
    return (await bindings.BrowserProxyBatchTestSpeed(proxyIds, concurrency)) || []
  }
  // mock
  await new Promise(r => setTimeout(r, 1000))
  return proxyIds.map(id => ({ proxyId: id, ok: true, latencyMs: Math.floor(100 + Math.random() * 400), error: '' }))
}

export async function browserProxyCheckIPHealth(proxyId: string): Promise<ProxyIPHealthResult> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProxyCheckIPHealth) {
    return (await bindings.BrowserProxyCheckIPHealth(proxyId)) || {
      proxyId,
      ok: false,
      source: 'ippure',
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
      updatedAt: new Date().toISOString(),
    }
  }
  await new Promise(r => setTimeout(r, 600))
  return {
    proxyId,
    ok: true,
    source: 'ippure',
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
    updatedAt: new Date().toISOString(),
  }
}

export async function browserProxyBatchCheckIPHealth(proxyIds: string[], concurrency: number = 10): Promise<ProxyIPHealthResult[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProxyBatchCheckIPHealth) {
    return (await bindings.BrowserProxyBatchCheckIPHealth(proxyIds, concurrency)) || []
  }
  await new Promise(r => setTimeout(r, 1200))
  return proxyIds.map(proxyId => ({
    proxyId,
    ok: true,
    source: 'ippure',
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
    updatedAt: new Date().toISOString(),
  }))
}

export async function openUserDataDir(userDataDir: string): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.OpenUserDataDir) {
    await bindings.OpenUserDataDir(userDataDir)
    return true
  }
  return false
}

export async function openCorePath(corePath: string): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.OpenCorePath) {
    await bindings.OpenCorePath(corePath)
    return true
  }
  return false
}

// ============================================================================
// Cookie API
// ============================================================================

export async function fetchBrowserCookies(profileId: string): Promise<CookieInfo[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserGetCookies) {
    return (await bindings.BrowserGetCookies(profileId)) || []
  }
  // mock data
  return [
    { name: 'session', value: 'abc123', domain: '.example.com', path: '/', expires: Date.now() / 1000 + 3600, httpOnly: true, secure: true, sameSite: 'Lax' },
    { name: 'pref', value: 'dark', domain: 'example.com', path: '/', expires: -1, httpOnly: false, secure: false, sameSite: 'None' },
  ]
}

export async function clearBrowserCookies(profileId: string): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserClearCookies) {
    await bindings.BrowserClearCookies(profileId)
    return true
  }
  return true
}

export async function exportBrowserCookies(profileId: string): Promise<string> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserExportCookies) {
    return (await bindings.BrowserExportCookies(profileId)) || ''
  }
  return '# Netscape HTTP Cookie File\n# Generated by BrowserManager\n\n.example.com\tTRUE\t/\tTRUE\t0\tsession\tabc123\n'
}

// ============================================================================
// Snapshot API
// ============================================================================

export async function listSnapshots(profileId: string): Promise<SnapshotInfo[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserSnapshotList) {
    return (await bindings.BrowserSnapshotList(profileId)) || []
  }
  return []
}

export async function createSnapshot(profileId: string, name: string): Promise<SnapshotInfo | null> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserSnapshotCreate) {
    return (await bindings.BrowserSnapshotCreate(profileId, name)) || null
  }
  // mock
  return {
    snapshotId: `snap-${Date.now()}`,
    profileId,
    name,
    sizeMB: 12.5,
    createdAt: new Date().toISOString(),
  }
}

export async function restoreSnapshot(profileId: string, snapshotId: string): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserSnapshotRestore) {
    await bindings.BrowserSnapshotRestore(profileId, snapshotId)
    return true
  }
  return true
}

export async function deleteSnapshot(profileId: string, snapshotId: string): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserSnapshotDelete) {
    await bindings.BrowserSnapshotDelete(profileId, snapshotId)
    return true
  }
  return true
}

// ============================================================================
// Bookmark API
// ============================================================================

export async function fetchBookmarks(): Promise<BrowserBookmark[]> {
  const bindings: any = await getBindings()
  if (bindings?.BookmarkList) {
    return (await bindings.BookmarkList()) || []
  }
  return [
    { name: 'Google', url: 'https://www.google.com/' },
    { name: 'Gmail', url: 'https://mail.google.com/' },
    { name: 'Claude', url: 'https://claude.ai/' },
    { name: 'ChatGPT', url: 'https://chatgpt.com/' },
    { name: 'YouTube', url: 'https://www.youtube.com/' },
  ]
}

export async function saveBookmarks(items: BrowserBookmark[]): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.BookmarkSave) {
    await bindings.BookmarkSave(items)
    return true
  }
  return true
}

export async function resetBookmarks(): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.BookmarkReset) {
    await bindings.BookmarkReset()
    return true
  }
  return true
}

// ============================================================================
// Keywords API
// ============================================================================

export async function setProfileKeywords(profileId: string, keywords: string[]): Promise<BrowserProfile | null> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProfileSetKeywords) {
    return (await bindings.BrowserProfileSetKeywords(profileId, keywords)) || null
  }
  mockProfiles = mockProfiles.map(p =>
    p.profileId === profileId ? { ...p, keywords, updatedAt: new Date().toISOString() } : p
  )
  return mockProfiles.find(p => p.profileId === profileId) || null
}

// ============================================================================
// LaunchCode API
// ============================================================================

export interface LaunchServerInfo {
  host: string
  port: number
  preferredPort: number
  baseUrl: string
  cdpUrl: string
  activeDebugPort: number
  ready: boolean
  apiAuth: {
    requested: boolean
    configured: boolean
    enabled: boolean
    header: string
  }
}

function normalizeLaunchServerInfo(payload: any): LaunchServerInfo {
  const host = String(payload?.host || '127.0.0.1')
  const port = Number(payload?.port) || 0
  const preferredPort = Number(payload?.preferredPort) || 0
  const fallbackPort = preferredPort > 0 ? preferredPort : 19876
  const effectivePort = port > 0 ? port : fallbackPort
  const baseUrl = String(payload?.baseUrl || (effectivePort > 0 ? `http://${host}:${effectivePort}` : ''))
  const cdpUrl = String(payload?.cdpUrl || baseUrl)
  const activeDebugPort = Number(payload?.activeDebugPort) || 0
  const apiAuthPayload = payload?.apiAuth || {}
  const apiAuth = {
    requested: !!apiAuthPayload?.requested,
    configured: !!apiAuthPayload?.configured,
    enabled: !!apiAuthPayload?.enabled,
    header: String(apiAuthPayload?.header || 'X-Ant-Api-Key'),
  }

  return {
    host,
    port: effectivePort,
    preferredPort,
    baseUrl,
    cdpUrl,
    activeDebugPort,
    ready: !!payload?.ready && port > 0,
    apiAuth,
  }
}

export async function fetchLaunchServerInfo(): Promise<LaunchServerInfo> {
  const bindings: any = await getBindings()
  if (bindings?.GetLaunchServerInfo) {
    return normalizeLaunchServerInfo(await bindings.GetLaunchServerInfo())
  }

  const goApp = (window as any).go?.main?.App
  if (goApp?.GetLaunchServerInfo) {
    return normalizeLaunchServerInfo(await goApp.GetLaunchServerInfo())
  }

  return {
    host: '127.0.0.1',
    port: 19876,
    preferredPort: 19876,
    baseUrl: 'http://127.0.0.1:19876',
    cdpUrl: 'http://127.0.0.1:19876',
    activeDebugPort: 0,
    ready: false,
    apiAuth: {
      requested: false,
      configured: false,
      enabled: false,
      header: 'X-Ant-Api-Key',
    },
  }
}

export async function getBrowserProfileCode(profileId: string): Promise<string> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProfileGetCode) {
    return (await bindings.BrowserProfileGetCode(profileId)) || ''
  }
  return ''
}

export async function regenerateBrowserProfileCode(profileId: string): Promise<string> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProfileRegenerateCode) {
    return (await bindings.BrowserProfileRegenerateCode(profileId)) || ''
  }
  return ''
}

export async function setBrowserProfileCode(profileId: string, code: string): Promise<string> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProfileSetCode) {
    return (await bindings.BrowserProfileSetCode(profileId, code)) || ''
  }
  return code.trim().toUpperCase()
}


export async function batchSetProfileTags(profileIds: string[], tags: string[], replace: boolean): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProfileBatchSetTags) {
    await bindings.BrowserProfileBatchSetTags(profileIds, tags, replace)
    return true
  }
  return true
}

export async function batchRemoveProfileTags(profileIds: string[], tags: string[]): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProfileBatchRemoveTags) {
    await bindings.BrowserProfileBatchRemoveTags(profileIds, tags)
    return true
  }
  return true
}

export async function renameBrowserTag(oldName: string, newName: string): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserRenameTag) {
    await bindings.BrowserRenameTag(oldName, newName)
    return true
  }
  return true
}

// ============================================================================
// Group API
// ============================================================================

export async function fetchGroups(): Promise<BrowserGroupWithCount[]> {
  const bindings: any = await getBindings()
  if (bindings?.ListGroups) {
    return (await bindings.ListGroups()) || []
  }
  return []
}

export async function createGroup(input: BrowserGroupInput): Promise<BrowserGroup | null> {
  const bindings: any = await getBindings()
  if (bindings?.CreateGroup) {
    return (await bindings.CreateGroup(input)) || null
  }
  return null
}

export async function updateGroup(groupId: string, input: BrowserGroupInput): Promise<BrowserGroup | null> {
  const bindings: any = await getBindings()
  if (bindings?.UpdateGroup) {
    return (await bindings.UpdateGroup(groupId, input)) || null
  }
  return null
}

export async function deleteGroup(groupId: string): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.DeleteGroup) {
    await bindings.DeleteGroup(groupId)
    return true
  }
  return false
}

export async function moveInstancesToGroup(profileIds: string[], groupId: string): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.MoveInstancesToGroup) {
    await bindings.MoveInstancesToGroup(profileIds, groupId)
    return true
  }
  return false
}

// ============================================================================
// Account API
// ============================================================================

export interface BrowserAccount {
  accountId: string
  accountName: string
  platform: string
  username: string
  email: string
  password: string
  relatedProfileIds: string[]
  notes: string
  cookies: string
  createdAt: string
  updatedAt: string
  cookieCount?: number
  cookieEarliestExpiry?: number
}

export interface BrowserAccountInput {
  accountName: string
  platform: string
  username: string
  email: string
  password: string
  relatedProfileIds: string[]
  notes: string
  cookies: string
}

export async function fetchAccounts(): Promise<BrowserAccount[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserAccountList) {
    return (await bindings.BrowserAccountList()) || []
  }
  return []
}

export async function fetchAccount(accountId: string): Promise<BrowserAccount | null> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserAccountGet) {
    return (await bindings.BrowserAccountGet(accountId)) || null
  }
  return null
}

export async function createAccount(input: BrowserAccountInput): Promise<BrowserAccount | null> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserAccountCreate) {
    return (await bindings.BrowserAccountCreate(input)) || null
  }
  return null
}

export async function updateAccount(accountId: string, input: BrowserAccountInput): Promise<BrowserAccount | null> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserAccountUpdate) {
    return (await bindings.BrowserAccountUpdate(accountId, input)) || null
  }
  return null
}

export async function deleteAccount(accountId: string): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserAccountDelete) {
    await bindings.BrowserAccountDelete(accountId)
    return true
  }
  return false
}

// 设置账号关联的实例集合（一对多）
export async function setAccountProfiles(accountId: string, profileIds: string[]): Promise<void> {
  await callBinding<void>('BrowserAccountSetProfiles', [accountId, profileIds], undefined as any)
}

// 从运行中实例读取 Cookie 并保存到账号；返回保存条数
export async function saveAccountCookiesFromProfile(accountId: string, profileId: string): Promise<number> {
  return callBinding<number>('BrowserAccountSaveCookiesFromProfile', [accountId, profileId], 0)
}

// 把账号已存 Cookie 回写到运行中实例；返回回写条数
export async function restoreAccountCookiesToProfile(accountId: string, profileId: string, clearFirst: boolean): Promise<number> {
  return callBinding<number>('BrowserAccountRestoreCookiesToProfile', [accountId, profileId, clearFirst], 0)
}

// 导出账号 JSON；includeSecrets=false 时不含密码/Cookie
export async function exportAccounts(accountIds: string[], includeSecrets: boolean): Promise<string> {
  return callBinding<string>('BrowserAccountExport', [accountIds, includeSecrets], '')
}

// 批量导入账号 JSON；返回导入条数
export async function importAccounts(payload: string): Promise<number> {
  return callBinding<number>('BrowserAccountImport', [payload], 0)
}

// ============================================================================
// Extension API
// ============================================================================

export interface BrowserExtension {
  extensionId: string
  extensionName: string
  extensionPath: string
  version: string
  enabled: boolean
  boundProfileIds: string[]
  sourceType: string
  sourceUrl: string
  description: string
  createdAt: string
  updatedAt: string
}

export interface BrowserExtensionInput {
  extensionName: string
  extensionPath: string
  version: string
  enabled: boolean
  boundProfileIds: string[]
  sourceType: string
  sourceUrl: string
  description: string
}

export async function fetchExtensions(): Promise<BrowserExtension[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserExtensionList) {
    return (await bindings.BrowserExtensionList()) || []
  }
  return []
}

export async function fetchExtension(extensionId: string): Promise<BrowserExtension | null> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserExtensionGet) {
    return (await bindings.BrowserExtensionGet(extensionId)) || null
  }
  return null
}

export async function createExtension(input: BrowserExtensionInput): Promise<BrowserExtension | null> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserExtensionCreate) {
    return (await bindings.BrowserExtensionCreate(input)) || null
  }
  return null
}

export async function updateExtension(extensionId: string, input: BrowserExtensionInput): Promise<BrowserExtension | null> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserExtensionUpdate) {
    return (await bindings.BrowserExtensionUpdate(extensionId, input)) || null
  }
  return null
}

export async function deleteExtension(extensionId: string): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserExtensionDelete) {
    await bindings.BrowserExtensionDelete(extensionId)
    return true
  }
  return false
}

export async function toggleExtension(extensionId: string, enabled: boolean): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserExtensionToggle) {
    await bindings.BrowserExtensionToggle(extensionId, enabled)
    return true
  }
  return false
}

export interface ExtensionValidateResult {
  valid: boolean
  message: string
  name: string
  version: string
}

// 校验扩展目录（manifest.json）并回填 name/version
export async function validateExtensionPath(path: string): Promise<ExtensionValidateResult> {
  return callBinding<ExtensionValidateResult>('BrowserExtensionValidatePath', [path], { valid: false, message: '后端不可用', name: '', version: '' })
}

// 设置扩展绑定的实例集合
export async function setExtensionProfiles(extensionId: string, profileIds: string[]): Promise<void> {
  await callBinding<void>('BrowserExtensionSetProfiles', [extensionId, profileIds], undefined as any)
}

// ============================================================================
// CDP API
// ============================================================================

export interface CDPNetworkRequest {
  requestId: string
  url: string
  method: string
  statusCode: number
  statusText: string
  type: string
  timestamp: number
  requestHeaders: Record<string, string>
  responseHeaders: Record<string, string>
  requestBody: string
  responseBody: string
  duration: number
  size: number
  mimeType: string
  startTime: number
  endTime: number
}

export interface CDPConsoleLog {
  id: string
  type: string // log/warn/error/info
  message: string
  timestamp: number
  stackTrace?: string
}

export async function CDPSessionCreate(profileId: string, targetType: string): Promise<string> {
  const bindings: any = await getBindings()
  if (bindings?.CDPSessionCreate) {
    return await bindings.CDPSessionCreate(profileId, targetType)
  }
  throw new Error('CDP功能不可用')
}

export async function CDPSessionClose(sessionId: string): Promise<void> {
  const bindings: any = await getBindings()
  if (bindings?.CDPSessionClose) {
    await bindings.CDPSessionClose(sessionId)
  }
}

export async function CDPGetNetworkRequests(sessionId: string): Promise<CDPNetworkRequest[]> {
  const bindings: any = await getBindings()
  if (bindings?.CDPGetNetworkRequests) {
    return (await bindings.CDPGetNetworkRequests(sessionId)) || []
  }
  return []
}

export async function CDPGetConsoleLogs(sessionId: string): Promise<CDPConsoleLog[]> {
  const bindings: any = await getBindings()
  if (bindings?.CDPGetConsoleLogs) {
    return (await bindings.CDPGetConsoleLogs(sessionId)) || []
  }
  return []
}

export async function CDPClearNetworkRequests(sessionId: string): Promise<void> {
  const bindings: any = await getBindings()
  if (bindings?.CDPClearNetworkRequests) {
    await bindings.CDPClearNetworkRequests(sessionId)
  }
}

export async function CDPClearConsoleLogs(sessionId: string): Promise<void> {
  const bindings: any = await getBindings()
  if (bindings?.CDPClearConsoleLogs) {
    await bindings.CDPClearConsoleLogs(sessionId)
  }
}

export async function CDPExecuteJavaScript(sessionId: string, code: string): Promise<string> {
  const bindings: any = await getBindings()
  if (bindings?.CDPExecuteJavaScript) {
    return await bindings.CDPExecuteJavaScript(sessionId, code)
  }
  throw new Error('JavaScript执行失败')
}

export async function CDPCaptureScreenshot(sessionId: string): Promise<string> {
  const bindings: any = await getBindings()
  if (bindings?.CDPCaptureScreenshot) {
    return await bindings.CDPCaptureScreenshot(sessionId)
  }
  throw new Error('截图失败')
}

export async function CDPExportHAR(sessionId: string): Promise<string> {
  const bindings: any = await getBindings()
  if (bindings?.CDPExportHAR) {
    return await bindings.CDPExportHAR(sessionId)
  }
  throw new Error('导出HAR失败')
}

export async function CDPGetStatistics(sessionId: string): Promise<any> {
  const bindings: any = await getBindings()
  if (bindings?.CDPGetStatistics) {
    return await bindings.CDPGetStatistics(sessionId)
  }
  return null
}

// CDPGetStorage 获取 localStorage / sessionStorage（用 callBinding 兼容未重生成的绑定）
export async function CDPGetStorage(sessionId: string, storageType: 'localStorage' | 'sessionStorage'): Promise<Record<string, string>> {
  return callBinding<Record<string, string>>('CDPGetStorage', [sessionId, storageType], {})
}
