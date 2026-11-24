// 更新检查的本地记忆：上次成功的检查结果、更新通道、上次使用的检查/下载代理。
// 直连访问 github.com 被阻断时，后台检查必然失败；靠这些记忆保证红点不会丢、弹窗不用每次重选节点。

const CHECK_SNAPSHOT_KEY = 'ant.update.checkSnapshot'
const PROXY_SELECTION_KEY = 'ant.update.proxySelection'
const CHANNEL_KEY = 'ant.update.channel'

export type UpdateProxyMode = 'direct' | 'pool' | 'custom'
export type UpdateChannel = 'stable' | 'preview'

export interface UpdateCheckSnapshot {
  updateAvailable: boolean
  latestVersion: string
  checkedAt: string
  channel: UpdateChannel
}

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

export function loadUpdateCheckSnapshot(): UpdateCheckSnapshot | null {
  const snapshot = readJSON<UpdateCheckSnapshot>(CHECK_SNAPSHOT_KEY)
  if (!snapshot || typeof snapshot.updateAvailable !== 'boolean' || typeof snapshot.latestVersion !== 'string') {
    return null
  }
  return {
    updateAvailable: snapshot.updateAvailable,
    latestVersion: snapshot.latestVersion,
    checkedAt: typeof snapshot.checkedAt === 'string' ? snapshot.checkedAt : '',
    // 旧版本快照没有通道字段，按正式版处理。
    channel: snapshot.channel === 'preview' ? 'preview' : 'stable',
  }
}

export function saveUpdateCheckSnapshot(snapshot: UpdateCheckSnapshot): void {
  writeJSON(CHECK_SNAPSHOT_KEY, snapshot)
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
