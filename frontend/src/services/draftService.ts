import type { FingerprintProfileConfig } from '../modules/browser/pages/browser-create-v2/types'

const DRAFT_KEY = 'browser_profile_draft'
const DRAFT_TIMESTAMP_KEY = 'browser_profile_draft_timestamp'

export interface BrowserProfileDraft {
  config: FingerprintProfileConfig
  selectedCoreId?: string
  selectedGroupId?: string
}

interface DraftOptions {
  selectedCoreId?: string
  selectedGroupId?: string
}

function isDraftEnvelope(value: unknown): value is BrowserProfileDraft {
  return typeof value === 'object' && value !== null && 'config' in value
}

// 保存草稿
export function saveDraft(config: FingerprintProfileConfig, options: DraftOptions = {}): void {
  try {
    localStorage.setItem(DRAFT_KEY, JSON.stringify({
      config,
      selectedCoreId: options.selectedCoreId,
      selectedGroupId: options.selectedGroupId,
    }))
    localStorage.setItem(DRAFT_TIMESTAMP_KEY, Date.now().toString())
  } catch (error) {
    console.error('保存草稿失败:', error)
  }
}

// 加载草稿
export function loadDraft(): BrowserProfileDraft | null {
  try {
    const draft = localStorage.getItem(DRAFT_KEY)
    if (!draft) return null
    const parsed = JSON.parse(draft) as unknown

    if (isDraftEnvelope(parsed)) {
      return parsed
    }

    return {
      config: parsed as FingerprintProfileConfig,
    }
  } catch (error) {
    console.error('加载草稿失败:', error)
    return null
  }
}

// 清除草稿
export function clearDraft(): void {
  try {
    localStorage.removeItem(DRAFT_KEY)
    localStorage.removeItem(DRAFT_TIMESTAMP_KEY)
  } catch (error) {
    console.error('清除草稿失败:', error)
  }
}

// 检查是否有草稿
export function hasDraft(): boolean {
  return localStorage.getItem(DRAFT_KEY) !== null
}

// 获取草稿保存时间
export function getDraftTimestamp(): Date | null {
  try {
    const timestamp = localStorage.getItem(DRAFT_TIMESTAMP_KEY)
    if (!timestamp) return null
    return new Date(parseInt(timestamp))
  } catch (error) {
    console.error('获取草稿时间失败:', error)
    return null
  }
}

// 格式化时间差
export function formatTimeSince(date: Date): string {
  const seconds = Math.floor((Date.now() - date.getTime()) / 1000)

  if (seconds < 60) return '刚刚'
  if (seconds < 3600) return `${Math.floor(seconds / 60)} 分钟前`
  if (seconds < 86400) return `${Math.floor(seconds / 3600)} 小时前`
  return `${Math.floor(seconds / 86400)} 天前`
}
