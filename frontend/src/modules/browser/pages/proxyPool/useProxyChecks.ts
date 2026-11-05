import { useEffect, useMemo, useState } from 'react'
import { toast } from '../../../../shared/components'
import { EventsOn } from '../../../../wailsjs/runtime/runtime'
import {
  browserProxyBatchCheckIPHealth,
  browserProxyBatchWarmupBridge,
  browserProxyCheckIPHealth,
  browserProxyWarmupBridge,
} from '../../api'
import type { ProxyIPHealthResult } from '../../types'
import { useProxySpeedStore, type ProxySpeedSource } from '../../proxySpeedStore'
import { ensureProxySpeedListener, recordProxySpeedResult, testProxySpeeds, testProxySpeedOnce } from '../../proxySpeedService'
import type { ProxyDisplayInfo } from './helpers'
import { readIPHealthCache, writeIPHealthCache, isProxySpeedResultFresh, toLatencyValue } from './storage'

interface UseProxyChecksOptions {
  proxies: ProxySpeedSource[]
}

function resolveErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof Error && error.message.trim()) return error.message
  if (typeof error === 'string' && error.trim()) return error
  return fallback
}

// 测速结果统一存放在 useProxySpeedStore（全局唯一），本 hook 只负责展示派生与批量操作编排。
export function useProxyChecks({ proxies }: UseProxyChecksOptions) {
  const [testingAll, setTestingAll] = useState(false)
  const [ipHealthMap, setIPHealthMap] = useState<Record<string, ProxyIPHealthResult>>({})
  const [checkingIPHealthIds, setCheckingIPHealthIds] = useState<Set<string>>(new Set())
  const [checkingAllIPHealth, setCheckingAllIPHealth] = useState(false)
  const [warmingBridgeIds, setWarmingBridgeIds] = useState<Set<string>>(new Set())
  const [warmingAllBridges, setWarmingAllBridges] = useState(false)
  const [ipHealthDetailOpen, setIPHealthDetailOpen] = useState(false)
  const [currentIPHealthDetail, setCurrentIPHealthDetail] = useState<ProxyIPHealthResult | null>(null)

  const speedResults = useProxySpeedStore((state) => state.results)
  const testingIds = useProxySpeedStore((state) => state.testingIds)

  const { latencyMap, latencyEngineMap, latencyErrorMap } = useMemo(() => {
    const latency: Record<string, number> = {}
    const engine: Record<string, string> = {}
    const error: Record<string, string> = {}
    Object.entries(speedResults).forEach(([proxyId, result]) => {
      latency[proxyId] = toLatencyValue(result.ok, result.latencyMs, result.error)
      if (result.engine) engine[proxyId] = result.engine
      if (result.error) error[proxyId] = result.error
    })
    Object.keys(testingIds).forEach((proxyId) => { latency[proxyId] = -1 })
    return { latencyMap: latency, latencyEngineMap: engine, latencyErrorMap: error }
  }, [speedResults, testingIds])

  useEffect(() => {
    const store = useProxySpeedStore.getState()
    store.applyBackendResults(proxies, (testedAt) => isProxySpeedResultFresh(testedAt))
    if (proxies.length > 0) {
      store.prune(new Set(proxies.map((proxy) => proxy.proxyId)))
    }
  }, [proxies])

  useEffect(() => {
    ensureProxySpeedListener()
  }, [])

  useEffect(() => {
    setIPHealthMap(readIPHealthCache())
  }, [])

  useEffect(() => {
    writeIPHealthCache(ipHealthMap)
  }, [ipHealthMap])

  useEffect(() => {
    const markBridgeFailure = (proxyIds: string[], message: string) => {
      Array.from(new Set(proxyIds.filter(Boolean))).forEach((proxyId) => {
        recordProxySpeedResult({ proxyId, ok: false, latencyMs: -1, error: message })
      })
    }
    const offFailed = EventsOn('proxy:bridge:failed', (data: { proxyId?: string; error?: string }) => {
      if (data.proxyId) markBridgeFailure([data.proxyId], data.error || '代理桥接启动失败')
    })
    const offDied = EventsOn('proxy:bridge:died', (data: { proxyIds?: string[]; error?: string }) => {
      markBridgeFailure(data.proxyIds || [], data.error || '代理桥接进程已退出')
    })
    return () => {
      offFailed()
      offDied()
    }
  }, [])

  const handleTestOne = async (record: ProxyDisplayInfo) => {
    if (record.proxyConfig === 'direct://') {
      toast.info('直连模式无需测速')
      return
    }
    const result = await testProxySpeedOnce(record.proxyId)
    if (!result.ok && result.error) toast.error(result.error)
  }

  const handleTestAll = async (items: ProxyDisplayInfo[]) => {
    const testable = items.filter((proxy) => proxy.proxyConfig !== 'direct://')
    if (testable.length === 0) return
    setTestingAll(true)
    try {
      const results = await testProxySpeeds(testable.map((proxy) => proxy.proxyId))
      const failed = results.filter((result) => !result.ok).length
      if (failed > 0) {
        const reasonCounts = new Map<string, number>()
        results.forEach((result) => {
          if (result.ok || !result.error) return
          reasonCounts.set(result.error, (reasonCounts.get(result.error) || 0) + 1)
        })
        const topReason = Array.from(reasonCounts.entries()).sort((a, b) => b[1] - a[1])[0]
        const suffix = topReason ? `；主要错误：${topReason[0]}（${topReason[1]} 条）` : ''
        toast.error(`批量测速完成：${failed} 条失败，共 ${results.length} 条${suffix}`)
      }
    } finally {
      setTestingAll(false)
    }
  }

  const handleWarmupOne = async (record: ProxyDisplayInfo) => {
    if (record.proxyConfig === 'direct://') {
      toast.info('直连模式无需预热')
      return
    }
    if (warmingBridgeIds.has(record.proxyId)) return

    setWarmingBridgeIds((prev) => new Set(prev).add(record.proxyId))
    try {
      const result = await browserProxyWarmupBridge(record.proxyId)
      if (result.ok) toast.success(`${record.proxyName} 已预热`)
      else toast.error(result.error || `${record.proxyName} 预热失败`)
    } catch (error) {
      toast.error(resolveErrorMessage(error, `${record.proxyName} 预热失败`))
    } finally {
      setWarmingBridgeIds((prev) => {
        const next = new Set(prev)
        next.delete(record.proxyId)
        return next
      })
    }
  }

  const handleWarmupAll = async (items: ProxyDisplayInfo[]) => {
    const testable = items.filter((p) => p.proxyConfig !== 'direct://')
    if (testable.length === 0) return
    setWarmingAllBridges(true)
    const ids = testable.map((p) => p.proxyId)
    setWarmingBridgeIds((prev) => new Set([...Array.from(prev), ...ids]))
    try {
      const results = await browserProxyBatchWarmupBridge(ids, 5)
      const failed = results.filter((r) => !r.ok).length
      if (failed > 0) toast.info(`预热完成：成功 ${results.length - failed}，失败 ${failed}`)
      else toast.success(`预热完成：共 ${results.length} 条`)
    } catch (error) {
      toast.error(resolveErrorMessage(error, '批量预热失败'))
    } finally {
      setWarmingBridgeIds((prev) => {
        const next = new Set(prev)
        ids.forEach((id) => next.delete(id))
        return next
      })
      setWarmingAllBridges(false)
    }
  }

  const handleCheckOneIPHealth = async (record: ProxyDisplayInfo) => {
    if (record.proxyConfig === 'direct://') {
      toast.info('直连模式无需检测')
      return
    }
    if (checkingIPHealthIds.has(record.proxyId)) return

    setCheckingIPHealthIds((prev) => new Set(prev).add(record.proxyId))
    try {
      const result = await browserProxyCheckIPHealth(record.proxyId)
      setIPHealthMap((prev) => ({ ...prev, [record.proxyId]: result }))
      if (!result.ok) toast.error(result.error || `${record.proxyName} 检测失败`)
    } catch (error) {
      toast.error(resolveErrorMessage(error, `${record.proxyName} IP 健康检测失败`))
    } finally {
      setCheckingIPHealthIds((prev) => {
        const next = new Set(prev)
        next.delete(record.proxyId)
        return next
      })
    }
  }

  const handleCheckAllIPHealth = async (items: ProxyDisplayInfo[]) => {
    const testable = items.filter((p) => p.proxyConfig !== 'direct://')
    if (testable.length === 0) return
    setCheckingAllIPHealth(true)

    const ids = testable.map((p) => p.proxyId)
    const idSet = new Set(ids)
    setCheckingIPHealthIds((prev) => new Set([...Array.from(prev), ...ids]))

    const off = EventsOn('proxy:iphealth:result', (data: ProxyIPHealthResult) => {
      if (!data?.proxyId || !idSet.has(data.proxyId)) return
      setIPHealthMap((prev) => ({ ...prev, [data.proxyId]: data }))
      setCheckingIPHealthIds((prev) => {
        const next = new Set(prev)
        next.delete(data.proxyId)
        return next
      })
    })

    try {
      const results = await browserProxyBatchCheckIPHealth(ids, 10)
      setIPHealthMap((prev) => {
        const next = { ...prev }
        results.forEach((result) => {
          if (result?.proxyId && idSet.has(result.proxyId)) next[result.proxyId] = result
        })
        return next
      })
      const failed = results.filter((r) => !r.ok).length
      if (failed > 0) toast.info(`IP 健康检测完成：成功 ${results.length - failed}，失败 ${failed}`)
      else toast.success(`IP 健康检测完成：共 ${results.length} 条`)
    } catch (error) {
      toast.error(resolveErrorMessage(error, '批量 IP 健康检测失败'))
    } finally {
      off()
      setCheckingIPHealthIds((prev) => {
        const next = new Set(prev)
        ids.forEach((id) => next.delete(id))
        return next
      })
      setCheckingAllIPHealth(false)
    }
  }

  const openIPHealthDetail = (proxyId: string) => {
    const result = ipHealthMap[proxyId]
    if (!result) return
    setCurrentIPHealthDetail(result)
    setIPHealthDetailOpen(true)
  }

  return {
    latencyMap,
    latencyEngineMap,
    latencyErrorMap,
    testingAll,
    ipHealthMap,
    checkingIPHealthIds,
    checkingAllIPHealth,
    warmingBridgeIds,
    warmingAllBridges,
    ipHealthDetailOpen,
    setIPHealthDetailOpen,
    currentIPHealthDetail,
    setIPHealthMap,
    handleTestOne,
    handleTestAll,
    handleWarmupOne,
    handleWarmupAll,
    handleCheckOneIPHealth,
    handleCheckAllIPHealth,
    openIPHealthDetail,
  }
}