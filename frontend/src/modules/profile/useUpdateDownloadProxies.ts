import { useEffect, useMemo, useState } from 'react'

import { fetchBrowserProxies } from '../browser/api'
import type { BrowserProxy } from '../browser/types'
import { isProxySpeedResultFresh } from '../browser/pages/proxyPool/storage'
import { selectUsableProxies, useProxySpeedStore } from '../browser/proxySpeedStore'

export interface UpdateDownloadProxyOption {
  proxy: BrowserProxy
  latencyMs: number
}

export interface UpdateDownloadProxyState {
  options: UpdateDownloadProxyOption[]
  // ready 表示节点列表已经拉取完成（无论是否为空），用于避免打开弹窗时用空列表做判断。
  ready: boolean
}

// 只列测速成功（延迟为正）的节点，并带上延迟，避免选中不可用节点后下载必然失败。
export function useUpdateDownloadProxies(enabled: boolean): UpdateDownloadProxyState {
  const [proxies, setProxies] = useState<BrowserProxy[]>([])
  const [ready, setReady] = useState(false)
  const speedResults = useProxySpeedStore(state => state.results)

  useEffect(() => {
    if (!enabled) {
      setReady(false)
      return
    }
    let active = true
    setReady(false)
    void fetchBrowserProxies()
      .then((list) => {
        if (!active) return
        setProxies(list)
        useProxySpeedStore.getState().applyBackendResults(list, testedAt => isProxySpeedResultFresh(testedAt))
        setReady(true)
      })
      .catch(() => {
        if (!active) return
        setProxies([])
        setReady(true)
      })
    return () => {
      active = false
    }
  }, [enabled])

  const options = useMemo(
    () => selectUsableProxies(proxies.filter(proxy => proxy.proxyId && proxy.proxyId !== '__direct__'), speedResults)
      .map(proxy => ({
        proxy,
        latencyMs: speedResults[proxy.proxyId]?.latencyMs ?? proxy.lastLatencyMs ?? 0,
      })),
    [proxies, speedResults],
  )

  return { options, ready }
}
