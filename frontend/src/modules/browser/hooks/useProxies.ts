import { useCallback, useEffect, useState } from 'react'
import { fetchBrowserProxies, fetchBrowserProxyGroups } from '../api'
import type { BrowserProxy } from '../types'

// useProxies 统一的代理列表 + 分组加载逻辑（供绑定选择、列表等复用）。
export function useProxies(autoLoad = true) {
  const [proxies, setProxies] = useState<BrowserProxy[]>([])
  const [groups, setGroups] = useState<string[]>([])
  const [loading, setLoading] = useState(false)

  const reload = useCallback(async () => {
    setLoading(true)
    try {
      const [list, grps] = await Promise.all([fetchBrowserProxies(), fetchBrowserProxyGroups()])
      setProxies(list)
      setGroups(grps)
      return list
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    if (autoLoad) void reload()
  }, [autoLoad, reload])

  return { proxies, setProxies, groups, setGroups, loading, reload }
}
