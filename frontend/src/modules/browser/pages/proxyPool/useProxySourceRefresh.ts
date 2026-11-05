import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { toast } from '../../../../shared/components'
import type { BrowserProxy } from '../../types'
import { refreshProxySource } from '../../api'
import { EventsOn } from '../../../../wailsjs/runtime/runtime'
import { collectURLImportSources, parseTimestampMs } from './helpers'

// 与后端调度器 proxySourceRefreshFailureRetry 对齐的失败冷却窗口。
const SOURCE_REFRESH_FAILURE_COOLDOWN_MS = 5 * 60 * 1000
// 后端刷新事件触发的 reload 节流窗口，避免与手动刷新的 reload 重复拉全量池。
const SOURCE_EVENT_RELOAD_THROTTLE_MS = 2000

interface UseProxySourceRefreshOptions {
  proxies: BrowserProxy[]
  /** 刷新成功后重新加载代理池（后端已完成匹配与保存） */
  reloadProxies: () => Promise<void>
}

// 刷新逻辑统一收敛在后端 BrowserProxyRefreshSource：
// 定时调度（app_proxy_source_refresh_scheduler.go）与前端按钮走同一条链路，
// 保证“名称 → 同国家 → 标记异常”的匹配语义只有一份实现。
// 前端定时器只是页面打开期间的兜底巡检，判定口径与后端调度器一致（行级开关 + 行级间隔）。
export function useProxySourceRefresh({
  proxies,
  reloadProxies,
}: UseProxySourceRefreshOptions) {
  const [refreshingAllSources, setRefreshingAllSources] = useState(false)
  const [refreshingSourceIds, setRefreshingSourceIds] = useState<Set<string>>(new Set())
  const proxiesRef = useRef<BrowserProxy[]>([])
  const refreshingSourceIdsRef = useRef<Set<string>>(new Set())
  const autoRefreshRunningRef = useRef(false)
  // 前端自动刷新的失败冷却：失败后 sourceLastRefreshAt 不变，没有冷却的话
  // 每个巡检周期都会再撞一次失效订阅（后端调度器有同款冷却，前端也要对齐）。
  const failureCooldownRef = useRef<Map<string, number>>(new Map())

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
      `${result?.endpointMatched ?? 0} 个端点对齐`,
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
      failureCooldownRef.current.delete(sourceId)
      await reloadProxies()
      if (!silent) {
        toast.success(`订阅刷新成功（${describeRefreshResult(result)}）`)
      }
      return true
    } catch (error: any) {
      failureCooldownRef.current.set(sourceId, Date.now() + SOURCE_REFRESH_FAILURE_COOLDOWN_MS)
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
      const metas = collectURLImportSources(proxiesRef.current).filter(meta => {
        if (!meta.sourceUrl.trim()) return false
        // 与后端调度器同口径：导入时关闭“定时刷新”的源不参与自动刷新；
        // 间隔按每个源自己的设置（未设置时回落 60 分钟），到期即刷，应用没开错过的时间点下次启动补拉
        if (!meta.sourceAutoRefresh) return false
        const cooldownUntil = failureCooldownRef.current.get(meta.sourceId)
        if (cooldownUntil && Date.now() < cooldownUntil) return false
        const interval = (meta.sourceRefreshIntervalM > 0 ? meta.sourceRefreshIntervalM : 60) * 60 * 1000
        const last = parseTimestampMs(meta.sourceLastRefreshAt)
        return last <= 0 || Date.now() - last >= interval
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
  }, [refreshingAllSources, refreshSingleSource])

  // 后端定时调度刷新（成功/失败）都会发事件；前端不在现场时也要把最新
  // 更新时间/失败记录同步到界面（订阅更新管理弹窗、表格）。
  // 手动刷新链路自己会 reload，这里用 trailing 合并避免重复拉全量池；
  // 计时窗口内的后续事件被忽略但窗口结束后必有一次 reload（拉的是最新池），
  // 不会出现"最后一个源的结果丢在界面上"。
  useEffect(() => {
    let reloadTimer: number | null = null
    const scheduleReload = () => {
      if (reloadTimer !== null) return
      reloadTimer = window.setTimeout(() => {
        reloadTimer = null
        void reloadProxies()
      }, SOURCE_EVENT_RELOAD_THROTTLE_MS)
    }
    // 失败通知：只提示"非本次手动触发"的失败（手动失败已由 toast 反馈），
    // 且同一源 5 分钟冷却内不重复弹（后端冷却期内也不会再撞，事件不会连发）。
    const onRefreshFailed = (payload: { sourceId?: string; error?: string }) => {
      scheduleReload()
      const sourceId = (payload?.sourceId || '').trim()
      if (!sourceId || refreshingSourceIdsRef.current.has(sourceId)) return
      toast.warning(`订阅刷新失败：${payload?.error || '未知错误'}`)
    }
    const offRefreshed = EventsOn('proxy:sources:refreshed', scheduleReload)
    const offFailed = EventsOn('proxy:sources:refresh:failed', onRefreshFailed)
    return () => {
      offRefreshed?.()
      offFailed?.()
      if (reloadTimer !== null) window.clearTimeout(reloadTimer)
    }
  }, [reloadProxies])

  return {
    sourceMetas,
    hasURLImportSources,
    refreshingAllSources,
    refreshingSourceIds,
    refreshSingleSource,
    handleRefreshAllSources,
  }
}
