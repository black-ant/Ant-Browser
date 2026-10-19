import profilePageConfig from '../../config/profile.config'
import type { AppUpdateCheckResult, AuthorProfile, IconKey, ProfileChannel, ProfilePageData, ProfileProject } from './types'

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

export async function checkForUpdates(): Promise<AppUpdateCheckResult> {
  const fallback: AppUpdateCheckResult = {
    ok: false,
    currentVersion: '',
    latestVersion: '',
    updateAvailable: false,
    releaseUrl: '',
    releaseName: '',
    publishedAt: '',
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
    releaseName: typeof rawResult?.releaseName === 'string' ? rawResult.releaseName.trim() : '',
    publishedAt: typeof rawResult?.publishedAt === 'string' ? rawResult.publishedAt.trim() : '',
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
