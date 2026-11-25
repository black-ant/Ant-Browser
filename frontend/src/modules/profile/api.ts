import profilePageConfig from '../../config/profile.config'
import type { AppDownloadedUpdate, AppUpdateCheckResult, AppUpdateInfo, AuthorProfile, IconKey, ProfileChannel, ProfilePageData, ProfileProject, UpdatePendingInfo } from './types'
import { loadUpdateChannel, loadUpdateProxySelection, type UpdateChannel } from './updatePrefs'

const PROFILE_ICON_KEYS: IconKey[] = [
  'book-open',
  'globe',
  'message-square',
  'github',
  'mail',
  'external-link',
]

const CHANNEL_ICON_BY_NAME: Record<string, IconKey> = {
  掘金: 'book-open',
  个人博客: 'globe',
  博客: 'globe',
  公众号: 'message-square',
  微信公众号: 'message-square',
  github: 'github',
  邮件: 'mail',
}

const getBindings = async () => {
  try {
    return await import('../../wailsjs/go/main/App')
  } catch {
    return null
  }
}

export function createDefaultProfilePageData(): ProfilePageData {
  return {
    author: cloneAuthor(profilePageConfig.defaultAuthor),
    project: cloneProject(profilePageConfig.project),
    meta: {
      source: 'default',
    },
  }
}

export async function loadProfilePageData(): Promise<ProfilePageData> {
  const defaultData = createDefaultProfilePageData()
  const authorURL = profilePageConfig.remoteAuthor.authorURL.trim()
  const timeoutMs = profilePageConfig.remoteAuthor.timeoutMs

  if (!authorURL) {
    return defaultData
  }

  try {
    const payload = await fetchRemoteAuthorPayload(authorURL, timeoutMs)
    return {
      author: normalizeAuthorProfile(payload, defaultData.author),
      project: defaultData.project,
      meta: {
        source: 'remote',
      },
    }
  } catch (error: any) {
    return {
      ...defaultData,
      meta: {
        source: 'default',
        message: error?.message || '远程作者配置不可用，已切换为默认资料。',
      },
    }
  }
}

export async function loadCurrentAppVersion(): Promise<string> {
  const bindings: any = await getBindings()
  const goApp = (window as any).go?.main?.App
  if (bindings?.GetAppConfig) {
    const config = await bindings.GetAppConfig()
    return typeof config?.version === 'string' ? config.version.trim() : ''
  }
  if (goApp?.GetAppConfig) {
    const config = await goApp.GetAppConfig()
    return typeof config?.version === 'string' ? config.version.trim() : ''
  }
  return ''
}

function normalizeUpdateInfo(raw: any): AppUpdateInfo {
  return {
    ok: raw?.ok === true,
    currentVersion: typeof raw?.currentVersion === 'string' ? raw.currentVersion.trim() : '',
    latestVersion: typeof raw?.latestVersion === 'string' ? raw.latestVersion.trim() : '',
    updateAvailable: raw?.updateAvailable === true,
    releaseUrl: typeof raw?.releaseUrl === 'string' ? raw.releaseUrl.trim() : '',
    notesUrl: typeof raw?.notesUrl === 'string' ? raw.notesUrl.trim() : '',
    force: raw?.force === true,
    assets: raw?.assets && typeof raw.assets === 'object' ? raw.assets : {},
    versions: Array.isArray(raw?.versions) ? raw.versions.filter((item: any) => typeof item === 'string' && item) : [],
    error: typeof raw?.error === 'string' ? raw.error.trim() : '',
  }
}

function unsupportedUpdateInfo(error: string): AppUpdateInfo {
  return {
    ok: false,
    currentVersion: '',
    latestVersion: '',
    updateAvailable: false,
    releaseUrl: '',
    notesUrl: '',
    force: false,
    assets: {},
    error,
  }
}

export async function checkAppUpdate(): Promise<AppUpdateInfo> {
  const bindings: any = await getBindings()
  const goApp = (window as any).go?.main?.App
  let raw: any
  if (bindings?.CheckAppUpdate) {
    raw = await bindings.CheckAppUpdate()
  } else if (goApp?.CheckAppUpdate) {
    raw = await goApp.CheckAppUpdate()
  } else {
    return unsupportedUpdateInfo('当前环境不支持更新检查')
  }
  return normalizeUpdateInfo(raw)
}

// 后台检查/旧入口使用上次成功用过的代理与通道：更新清单在 github.com 上，直连被阻断时只能走节点。
export async function checkAppUpdateWithSavedProxy(): Promise<AppUpdateInfo> {
  const saved = loadUpdateProxySelection()
  const channel = loadUpdateChannel()
  if (saved?.mode === 'pool' && saved.proxyId) {
    return checkAppUpdateWithChannel('', saved.proxyId, channel)
  }
  if (saved?.mode === 'custom' && saved.customProxy.trim()) {
    return checkAppUpdateWithChannel(saved.customProxy.trim(), '', channel)
  }
  return checkAppUpdateWithChannel('', '', channel)
}

// 检查指定通道（正式 / 预览）；旧构建没有通道方法时退回正式版接口。
export async function checkAppUpdateWithChannel(proxyConfig: string, proxyId: string, channel: UpdateChannel): Promise<AppUpdateInfo> {
  const bindings: any = await getBindings()
  const goApp = (window as any).go?.main?.App
  let raw: any
  if (bindings?.CheckAppUpdateWithChannel) {
    raw = await bindings.CheckAppUpdateWithChannel(proxyConfig, proxyId, channel)
  } else if (goApp?.CheckAppUpdateWithChannel) {
    raw = await goApp.CheckAppUpdateWithChannel(proxyConfig, proxyId, channel)
  } else {
    return checkAppUpdateWithProxySelection(proxyConfig, proxyId)
  }
  return normalizeUpdateInfo(raw)
}

// 查看指定版本的更新信息；version 为空表示当前通道的最新版本。
export async function checkAppUpdateVersion(
  proxyConfig: string,
  proxyId: string,
  channel: UpdateChannel,
  version: string,
): Promise<AppUpdateInfo> {
  const bindings: any = await getBindings()
  const goApp = (window as any).go?.main?.App
  let raw: any
  if (bindings?.CheckAppUpdateVersion) {
    raw = await bindings.CheckAppUpdateVersion(proxyConfig, proxyId, channel, version)
  } else if (goApp?.CheckAppUpdateVersion) {
    raw = await goApp.CheckAppUpdateVersion(proxyConfig, proxyId, channel, version)
  } else {
    return checkAppUpdateWithChannel(proxyConfig, proxyId, channel)
  }
  return normalizeUpdateInfo(raw)
}

// 检查更新也支持临时指定代理：更新清单在 github.com 上，直连被阻断时只能走节点。
export async function checkAppUpdateWithProxySelection(proxyConfig: string, proxyId: string): Promise<AppUpdateInfo> {
  const bindings: any = await getBindings()
  const goApp = (window as any).go?.main?.App
  let raw: any
  if (bindings?.CheckAppUpdateWithProxySelection) {
    raw = await bindings.CheckAppUpdateWithProxySelection(proxyConfig, proxyId)
  } else if (goApp?.CheckAppUpdateWithProxySelection) {
    raw = await goApp.CheckAppUpdateWithProxySelection(proxyConfig, proxyId)
  } else {
    return checkAppUpdate()
  }
  return normalizeUpdateInfo(raw)
}

async function invokeUpdateMethod(name: string, ...args: any[]): Promise<boolean> {
  const bindings: any = await getBindings()
  const goApp = (window as any).go?.main?.App
  if (bindings?.[name]) {
    await bindings[name](...args)
    return true
  }
  if (goApp?.[name]) {
    await goApp[name](...args)
    return true
  }
  return false
}

export async function downloadAppUpdate(version: string, assetKey: string): Promise<boolean> {
  return invokeUpdateMethod('DownloadAppUpdate', version, assetKey)
}

export async function applyAppUpdate(version: string, assetKey: string): Promise<boolean> {
  return invokeUpdateMethod('ApplyAppUpdate', version, assetKey)
}

export async function downloadAppUpdateWithProxy(version: string, assetKey: string, proxyConfig: string): Promise<boolean> {
  return invokeUpdateMethod('DownloadAppUpdateWithProxy', version, assetKey, proxyConfig)
}

export async function downloadAppUpdateWithProxySelection(version: string, assetKey: string, proxyConfig: string, proxyId: string): Promise<boolean> {
  return invokeUpdateMethod('DownloadAppUpdateWithProxySelection', version, assetKey, proxyConfig, proxyId)
}

// 下载指定通道的更新包；旧构建没有通道方法时退回正式版下载。
export async function downloadAppUpdateWithChannel(
  version: string,
  assetKey: string,
  proxyConfig: string,
  proxyId: string,
  channel: UpdateChannel,
): Promise<boolean> {
  const started = await invokeUpdateMethod('DownloadAppUpdateWithChannel', version, assetKey, proxyConfig, proxyId, channel)
  if (started) return true
  return downloadAppUpdateWithProxySelection(version, assetKey, proxyConfig, proxyId)
}

export async function applyPendingAppUpdate(): Promise<boolean> {
  return invokeUpdateMethod('ApplyPendingAppUpdate')
}

export async function getPendingAppUpdate(): Promise<UpdatePendingInfo | null> {
  const bindings: any = await getBindings()
  const goApp = (window as any).go?.main?.App
  let raw: any
  if (bindings?.GetPendingAppUpdate) {
    raw = await bindings.GetPendingAppUpdate()
  } else if (goApp?.GetPendingAppUpdate) {
    raw = await goApp.GetPendingAppUpdate()
  }
  return raw?.installerPath ? raw as UpdatePendingInfo : null
}

export async function clearPendingAppUpdate(): Promise<boolean> {
  return invokeUpdateMethod('ClearPendingAppUpdate')
}

export async function getDownloadedAppUpdate(): Promise<AppDownloadedUpdate | null> {
  const bindings: any = await getBindings()
  const goApp = (window as any).go?.main?.App
  let raw: any
  if (bindings?.GetDownloadedAppUpdate) {
    raw = await bindings.GetDownloadedAppUpdate()
  } else if (goApp?.GetDownloadedAppUpdate) {
    raw = await goApp.GetDownloadedAppUpdate()
  }
  return raw?.installerPath ? raw as AppDownloadedUpdate : null
}

export async function clearDownloadedAppUpdate(): Promise<boolean> {
  return invokeUpdateMethod('ClearDownloadedAppUpdate')
}

export async function openUpdatePackageLocation(): Promise<boolean> {
  return invokeUpdateMethod('OpenUpdatePackageLocation')
}

export async function checkForUpdates(): Promise<AppUpdateCheckResult> {
  const fallback: AppUpdateCheckResult = {
    ok: false,
    currentVersion: '',
    latestVersion: '',
    updateAvailable: false,
    releaseUrl: '',
    error: '当前环境不支持更新检查',
  }
  const bindings: any = await getBindings()
  const goApp = (window as any).go?.main?.App
  let rawResult: any
  if (bindings?.CheckForUpdates) {
    rawResult = await bindings.CheckForUpdates()
  } else if (goApp?.CheckForUpdates) {
    rawResult = await goApp.CheckForUpdates()
  } else {
    return fallback
  }

  return {
    ok: rawResult?.ok === true,
    currentVersion: typeof rawResult?.currentVersion === 'string' ? rawResult.currentVersion.trim() : '',
    latestVersion: typeof rawResult?.latestVersion === 'string' ? rawResult.latestVersion.trim() : '',
    updateAvailable: rawResult?.updateAvailable === true,
    releaseUrl: typeof rawResult?.releaseUrl === 'string' ? rawResult.releaseUrl.trim() : '',
    error: typeof rawResult?.error === 'string' ? rawResult.error.trim() : '',
  }
}

async function fetchRemoteAuthorPayload(authorURL: string, timeoutMs: number): Promise<Record<string, any>> {
  const bindings: any = await getBindings()
  if (bindings?.FetchRemoteAuthorProfile) {
    return (await bindings.FetchRemoteAuthorProfile(authorURL, timeoutMs)) || {}
  }

  const goApp = (window as any).go?.main?.App
  if (goApp?.FetchRemoteAuthorProfile) {
    return (await goApp.FetchRemoteAuthorProfile(authorURL, timeoutMs)) || {}
  }

  return await fetchRemoteAuthorPayloadViaBrowser(authorURL, timeoutMs)
}

async function fetchRemoteAuthorPayloadViaBrowser(authorURL: string, timeoutMs: number): Promise<Record<string, any>> {
  const controller = new AbortController()
  const timer = window.setTimeout(() => controller.abort(), clampTimeout(timeoutMs))

  try {
    const response = await fetch(authorURL, {
      method: 'GET',
      headers: {
        Accept: 'application/json',
      },
      signal: controller.signal,
    })

    if (!response.ok) {
      throw new Error(`远程作者配置返回异常状态码: ${response.status}`)
    }

    const payload = await response.json()
    if (!payload || typeof payload !== 'object' || Array.isArray(payload)) {
      throw new Error('远程作者配置格式无效')
    }

    return payload as Record<string, any>
  } catch (error: any) {
    if (error?.name === 'AbortError') {
      throw new Error('远程作者配置请求超时')
    }
    throw error
  } finally {
    window.clearTimeout(timer)
  }
}

function normalizeAuthorProfile(payload: Record<string, any>, fallback: AuthorProfile): AuthorProfile {
  const source = extractAuthorPayload(payload)
  const name = normalizeString(source.name, fallback.name)
  const initial = normalizeString(source.initial, name.charAt(0) || fallback.initial).charAt(0) || fallback.initial

  return {
    name,
    initial,
    title: normalizeString(source.title, fallback.title),
    bio: normalizeString(source.bio, fallback.bio),
    location: normalizeString(source.location, fallback.location),
    joinDate: normalizeString(source.joinDate, fallback.joinDate),
    email: normalizeString(source.email, fallback.email),
    website: normalizeString(source.website, fallback.website),
    github: normalizeString(source.github, fallback.github),
    skills: normalizeStringArray(source.skills, fallback.skills),
    channels: normalizeChannels(source.channels, fallback.channels),
  }
}

function normalizeChannels(value: unknown, fallback: ProfileChannel[]): ProfileChannel[] {
  if (!Array.isArray(value)) {
    return cloneChannels(fallback)
  }

  const channels = value
    .map((item) => normalizeChannel(item))
    .filter((item): item is ProfileChannel => !!item)

  return channels.length > 0 ? channels : cloneChannels(fallback)
}

function normalizeChannel(value: any): ProfileChannel | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return null
  }

  const name = normalizeString(value.name)
  if (!name) {
    return null
  }

  const href = normalizeOptionalString(value.href ?? value.url)
  const detail = normalizeString(value.detail ?? value.value, href ? stripProtocol(href) : '')

  return {
    name,
    description: normalizeString(value.description),
    detail,
    href,
    icon: normalizeIconKey(value.icon, name),
  }
}

function normalizeIconKey(value: unknown, name: string): IconKey | undefined {
  if (typeof value === 'string') {
    const normalized = value.trim().toLowerCase() as IconKey
    if (PROFILE_ICON_KEYS.includes(normalized)) {
      return normalized
    }
  }

  return CHANNEL_ICON_BY_NAME[name] || CHANNEL_ICON_BY_NAME[name.toLowerCase()]
}

function extractAuthorPayload(payload: Record<string, any>): Record<string, any> {
  if (payload.author && typeof payload.author === 'object' && !Array.isArray(payload.author)) {
    return payload.author
  }
  return payload
}

function normalizeString(value: unknown, fallback = ''): string {
  if (typeof value !== 'string') {
    return fallback
  }

  const trimmed = value.trim()
  return trimmed || fallback
}

function normalizeOptionalString(value: unknown): string | undefined {
  if (typeof value !== 'string') {
    return undefined
  }

  const trimmed = value.trim()
  return trimmed || undefined
}

function normalizeStringArray(value: unknown, fallback: string[]): string[] {
  if (!Array.isArray(value)) {
    return [...fallback]
  }

  const items = value
    .map((item) => normalizeString(item))
    .filter(Boolean)

  return items.length > 0 ? items : [...fallback]
}

function stripProtocol(value: string): string {
  return value.replace(/^https?:\/\//, '').replace(/\/$/, '')
}

function clampTimeout(timeoutMs: number): number {
  if (!Number.isFinite(timeoutMs) || timeoutMs <= 0) {
    return 3000
  }
  return Math.min(timeoutMs, 15000)
}

function cloneAuthor(author: AuthorProfile): AuthorProfile {
  return {
    ...author,
    skills: [...author.skills],
    channels: cloneChannels(author.channels),
  }
}

function cloneChannels(channels: ProfileChannel[]): ProfileChannel[] {
  return channels.map((channel) => ({ ...channel }))
}

function cloneProject(project: ProfileProject): ProfileProject {
  return {
    ...project,
    techStack: [...project.techStack],
    actions: project.actions.map((action) => ({ ...action })),
  }
}
