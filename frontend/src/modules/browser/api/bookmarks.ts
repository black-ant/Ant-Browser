import type { BookmarkSyncResult, BrowserBookmark } from '../types'
import { getBindings } from './runtime'

export async function fetchBookmarks(): Promise<BrowserBookmark[]> {
  const bindings: any = await getBindings()
  if (bindings?.BookmarkList) {
    return (await bindings.BookmarkList()) || []
  }
  return [
    { name: '指纹检测', url: 'ant://fingerprint-check', openOnStart: false },
    { name: 'Google', url: 'https://www.google.com/', openOnStart: false },
    { name: 'Gmail', url: 'https://mail.google.com/', openOnStart: false },
    { name: 'Claude', url: 'https://claude.ai/', openOnStart: false },
    { name: 'ChatGPT', url: 'https://chatgpt.com/', openOnStart: false },
    { name: 'YouTube', url: 'https://www.youtube.com/', openOnStart: false },
    { name: 'IPPure', url: 'https://ippure.com/', openOnStart: false },
    { name: 'IPLark', url: 'https://iplark.com/', openOnStart: false },
    { name: 'Ping0', url: 'https://ping0.cc/', openOnStart: false },
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

export async function syncBookmarksToProfiles(): Promise<BookmarkSyncResult> {
  const bindings: any = await getBindings()
  if (bindings?.BookmarkSyncToProfiles) {
    return await bindings.BookmarkSyncToProfiles()
  }
  return {
    total: 0,
    synced: 0,
    skipped: 0,
    failed: 0,
    skippedList: [],
    failedList: [],
  }
}
