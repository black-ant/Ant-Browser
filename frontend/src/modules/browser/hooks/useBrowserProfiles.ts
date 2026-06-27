import { useCallback, useEffect, useState } from 'react'
import { fetchBrowserProfiles } from '../api'
import type { BrowserProfile } from '../types'

// useBrowserProfiles 统一的窗口列表加载逻辑（加载 + loading + 重载）。
// 适用于只需简单拉取列表的页面（账号/扩展/快捷启动等）。
// 注：BrowserListPage 有静默刷新/事件同步等特化逻辑，保留其自有实现。
export function useBrowserProfiles(autoLoad = true) {
  const [profiles, setProfiles] = useState<BrowserProfile[]>([])
  const [loading, setLoading] = useState(false)

  const reload = useCallback(async () => {
    setLoading(true)
    try {
      const items = await fetchBrowserProfiles()
      setProfiles(items)
      return items
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    if (autoLoad) void reload()
  }, [autoLoad, reload])

  return { profiles, setProfiles, loading, reload }
}
