import type { BrowserProfile } from '../types'

export interface ProfileTagSection {
  tag: string
  items: BrowserProfile[]
}

export interface GroupFilterOption {
  id: string
  name: string
  count: number
}

export const UNTAGGED_LABEL = '未打标签'
export const GROUP_ALL = '__all__'
export const GROUP_UNGROUPED = '__ungrouped__'

export function normalizeText(v?: string): string {
  return (v || '').trim().toLowerCase()
}

export function normalizeCode(v?: string): string {
  return normalizeText(v).toUpperCase()
}

export function buildSearchText(profile: BrowserProfile): string {
  return [
    profile.profileName,
    profile.launchCode || '',
    ...(profile.tags || []),
    ...(profile.keywords || []),
  ]
    .join(' ')
    .toLowerCase()
}

export function sortProfiles(a: BrowserProfile, b: BrowserProfile): number {
  if (a.running !== b.running) {
    return a.running ? -1 : 1
  }
  return a.profileName.localeCompare(b.profileName, 'zh-CN')
}

export function pickPrimaryTag(profile: BrowserProfile): string {
  const tags = (profile.tags || []).map(t => t.trim()).filter(Boolean)
  return tags.length > 0 ? tags[0] : UNTAGGED_LABEL
}
