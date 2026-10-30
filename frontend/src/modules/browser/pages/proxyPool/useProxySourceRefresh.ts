import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { toast } from '../../../../shared/components'
import type { BrowserProxy } from '../../types'
import { refreshProxySource } from '../../api'
import { collectURLImportSources, parseTimestampMs } from './helpers'

interface UseProxySourceRefreshOptions {
  proxies: BrowserProxy[]
  globalAutoRefreshEnabled: boolean
  globalRefreshInterval: number
  /** 刷新成功后重新加载代理池（后端已完成匹配与保存） */
  reloadProxies: () => Promise<void>
}

// 刷新逻辑统一收敛在后端 BrowserProxyRefreshSource：
// 定时调度（app_proxy_source_refresh_scheduler.go）与前端按钮走同一条链路，
// 保证“名称 → 同国家 → 标记异常”的匹配语义只有一份实现。
export function useProxySourceRefresh({
  proxies,
  globalAutoRefreshEnabled,
  globalRefreshInterval,
  reloadProxies,
}: UseProxySourceRefreshOptions) {
  const [refreshingAllSources, setRefreshingAllSources] = useState(false)
  const [refreshingSourceIds, setRefreshingSourceIds] = useState<Set<string>>(new Set())
  const proxiesRef = useRef<BrowserProxy[]>([])
  const refreshingSourceIdsRef = useRef<Set<string>>(new Set())
  const autoRefreshRunningRef = useRef(false)

  useEffect(() => {
    proxiesRef.current = proxies
  }, [proxies])

  useEffect(() => {
    refreshingSourceIdsRef.current = refreshingSourceIds
  }, [refreshingSourceIds])

  const sourceMetas = useMemo(() => collectURLImportSources(proxies), [proxies])
  const hasURLImportSources = sourceMetas.length > 0

  const describeRefreshResult = (result: any): string => {
    const parts = [
      `${result?.nameMatched ?? 0} 个名称匹配`,
      `${result?.countryMatched ?? 0} 个国家匹配`,
      `${result?.added ?? 0} 个新增`,
    ]
    if ((result?.markedMissing ?? 0) > 0) {
      parts.push(`${result.markedMissing} 个已标记异常`)
    }
    return parts.join('，')
  }

  const refreshSingleSource = useCallback(async (sourceId: string, silent: boolean) => {
    const currentList = proxiesRef.current
    const meta = collectURLImportSources(currentList).find(item => item.sourceId === sourceId)
    if (!meta) return false

    if (refreshingSourceIdsRef.current.has(sourceId)) return false
    setRefreshingSourceIds(prev => {
      const next = new Set(prev)
      next.add(sourceId)
      return next
    })

    try {
      const result = await refreshProxySource(sourceId)
      await reloadProxies()
      if (!silent) {
        toast.success(`订阅刷新成功（${describeRefreshResult(result)}）`)
      }
      return true
    } catch (error: any) {
      if (!silent) {
        toast.error(error?.message || '订阅刷新失败')
      }
      return false
    } finally {
      setRefreshingSourceIds(prev => {
        const next = new Set(prev)
        next.delete(sourceId)
        return next
      })
    }
  }, [reloadProxies])

  const handleRefreshAllSources = useCallback(async (silent = false) => {
    const metas = collectURLImportSources(proxiesRef.current)
    if (metas.length === 0) {
      if (!silent) {
        toast.info('当前没有 URL 导入订阅')
      }
      return
    }

    setRefreshingAllSources(true)
    let successCount = 0
    for (const meta of metas) {
      const ok = await refreshSingleSource(meta.sourceId, true)
      if (ok) successCount += 1
    }
    setRefreshingAllSources(false)

    if (!silent) {
      if (successCount === metas.length) {
        toast.success(`订阅刷新完成：${successCount}/${metas.length}`)
      } else {
        toast.warning(`订阅刷新完成：成功 ${successCount}/${metas.length}`)
      }
    }
  }, [refreshSingleSource])

  useEffect(() => {
    const runAutoRefresh = async () => {
      if (autoRefreshRunningRef.current || refreshingAllSources) return
      if (!globalAutoRefreshEnabled) return
      const intervalMs = globalRefreshInterval * 60 * 1000
      const metas = collectURLImportSources(proxiesRef.current).filter(meta => {
        if (!meta.sourceUrl.trim()) return false
        // 与后端调度器同口径：导入时关闭“自动更新订阅”的源不参与自动刷新
        if (!meta.sourceAutoRefresh) return false
        const last = parseTimestampMs(meta.sourceLastRefreshAt)
        return last <= 0 || Date.now() - last >= intervalMs
      })
      if (metas.length === 0) return

      autoRefreshRunningRef.current = true
      try {
        for (const meta of metas) {
          await refreshSingleSource(meta.sourceId, true)
        }
      } finally {
        autoRefreshRunningRef.current = false
      }
    }

    void runAutoRefresh()
    const timer = window.setInterval(() => {
      void runAutoRefresh()
    }, 60 * 1000)

    return () => {
      window.clearInterval(timer)
    }
  }, [globalAutoRefreshEnabled, globalRefreshInterval, refreshingAllSources, refreshSingleSource])

  return {
    sourceMetas,
    hasURLImportSources,
    refreshingAllSources,
    refreshingSourceIds,
    refreshSingleSource,
    handleRefreshAllSources,
  }
}
