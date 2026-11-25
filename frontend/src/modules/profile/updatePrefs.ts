// 更新检查的本地偏好：更新通道、上次使用的检查/下载代理。
// 红点只由实际检查结果决定，这里不缓存“上次检查到的新版本”。

const PROXY_SELECTION_KEY = 'ant.update.proxySelection'
const CHANNEL_KEY = 'ant.update.channel'

export type UpdateProxyMode = 'direct' | 'pool' | 'custom'
export type UpdateChannel = 'stable' | 'preview'

export interface UpdateProxySelection {
  mode: UpdateProxyMode
  proxyId: string
  customProxy: string
}

export function normalizeUpdateVersion(value: string): string {
  const version = (value || '').trim()
  return version.replace(/^[vV]/, '')
}

function readJSON<T>(key: string): T | null {
  try {
    const raw = window.localStorage.getItem(key)
    if (!raw) return null
    return JSON.parse(raw) as T
  } catch {
    return null
  }
}

function writeJSON(key: string, value: unknown): void {
  try {
    window.localStorage.setItem(key, JSON.stringify(value))
  } catch {
    // 本地存储不可用时忽略：只影响记忆能力，不影响更新流程。
  }
}

export function loadUpdateChannel(): UpdateChannel {
  try {
    return window.localStorage.getItem(CHANNEL_KEY) === 'preview' ? 'preview' : 'stable'
  } catch {
    return 'stable'
  }
}

export function saveUpdateChannel(channel: UpdateChannel): void {
  try {
    window.localStorage.setItem(CHANNEL_KEY, channel === 'preview' ? 'preview' : 'stable')
  } catch {
    // 忽略
  }
}

export function loadUpdateProxySelection(): UpdateProxySelection | null {
  const selection = readJSON<UpdateProxySelection>(PROXY_SELECTION_KEY)
  if (!selection || (selection.mode !== 'direct' && selection.mode !== 'pool' && selection.mode !== 'custom')) {
    return null
  }
  return {
    mode: selection.mode,
    proxyId: typeof selection.proxyId === 'string' ? selection.proxyId : '',
    customProxy: typeof selection.customProxy === 'string' ? selection.customProxy : '',
  }
}

export function saveUpdateProxySelection(selection: UpdateProxySelection): void {
  writeJSON(PROXY_SELECTION_KEY, selection)
}
