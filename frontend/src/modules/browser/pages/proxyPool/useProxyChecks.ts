import { useEffect, useRef, useState } from 'react'
import { toast } from '../../../../shared/components'
import { EventsOn } from '../../../../wailsjs/runtime/runtime'
import {
  browserProxyBatchCheckIPHealth,
  browserProxyBatchTestSpeed,
  browserProxyBatchWarmupBridge,
  browserProxyCheckIPHealth,
  browserProxyTestSpeed,
  browserProxyWarmupBridge,
} from '../../api'
import type { BrowserProxy, ProxyIPHealthResult, ProxySpeedTestResult } from '../../types'
import type { ProxyDisplayInfo } from './helpers'
import { toLatencyValue } from './storage'
import {
  readIPHealthCache,
  readLatencyCache,
  readLatencyEngineCache,
  readLatencyFingerprintCache,
  writeIPHealthCache,
  writeLatencyCache,
  writeLatencyEngineCache,
  writeLatencyFingerprintCache,
} from './storage'

interface UseProxyChecksOptions {
  proxies: Array<Pick<BrowserProxy, 'proxyId' | 'proxyConfig' | 'preferredKernel' | 'dnsServers'>>
}

function proxyRouteFingerprint(proxy: Pick<BrowserProxy, 'proxyId' | 'proxyConfig' | 'preferredKernel' | 'dnsServers'>): string {
  return [proxy.proxyConfig || '', proxy.preferredKernel || '', proxy.dnsServers || ''].map(value => value.trim()).join('\u001f')
}

export function useProxyChecks({ proxies }: UseProxyChecksOptions) {
  const [latencyMap, setLatencyMap] = useState<Record<string, number>>(() => readLatencyCache())
  const [latencyEngineMap, setLatencyEngineMap] = useState<Record<string, string>>(() => readLatencyEngineCache())
  const [latencyErrorMap, setLatencyErrorMap] = useState<Record<string, string>>({})
  const [latencyFingerprintMap, setLatencyFingerprintMap] = useState<Record<string, string>>(() => readLatencyFingerprintCache())
  const [testingAll, setTestingAll] = useState(false)
  const [ipHealthMap, setIPHealthMap] = useState<Record<string, ProxyIPHealthResult>>(() => readIPHealthCache())
  const [checkingIPHealthIds, setCheckingIPHealthIds] = useState<Set<string>>(new Set())
  const [checkingAllIPHealth, setCheckingAllIPHealth] = useState(false)
  const [warmingBridgeIds, setWarmingBridgeIds] = useState<Set<string>>(new Set())
  const [warmingAllBridges, setWarmingAllBridges] = useState(false)
  const [ipHealthDetailOpen, setIPHealthDetailOpen] = useState(false)
  const [currentIPHealthDetail, setCurrentIPHealthDetail] = useState<ProxyIPHealthResult | null>(null)
  const routeFingerprintsRef = useRef<Record<string, string>>({})

  useEffect(() => {
    const markBridgeFailure = (proxyIds: string[], message: string) => {
      const uniqueIds = Array.from(new Set(proxyIds.filter(Boolean)))
      if (uniqueIds.length === 0) return
      setLatencyMap(prev => {
        const next = { ...prev }
        uniqueIds.forEach(proxyId => { next[proxyId] = toLatencyValue(false, 0, message) })
        return next
      })
      setLatencyEngineMap(prev => {
        const next = { ...prev }
        uniqueIds.forEach(proxyId => { delete next[proxyId] })
        return next
      })
      setLatencyErrorMap(prev => {
        const next = { ...prev }
        uniqueIds.forEach(proxyId => { next[proxyId] = message })
        return next
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

  useEffect(() => {
    writeLatencyCache(latencyMap)
  }, [latencyMap])

  useEffect(() => {
    writeLatencyEngineCache(latencyEngineMap)
  }, [latencyEngineMap])

  useEffect(() => {
    writeLatencyFingerprintCache(latencyFingerprintMap)
  }, [latencyFingerprintMap])

  useEffect(() => {
    writeIPHealthCache(ipHealthMap)
  }, [ipHealthMap])

  useEffect(() => {
    if (!proxies.length) return
    const validIds = new Set(proxies.map(p => p.proxyId))
    const nextFingerprints: Record<string, string> = {}
    const changedIds = new Set<string>()
    proxies.forEach(proxy => {
      const fingerprint = proxyRouteFingerprint(proxy)
      nextFingerprints[proxy.proxyId] = fingerprint
      if (latencyFingerprintMap[proxy.proxyId] !== fingerprint) changedIds.add(proxy.proxyId)
    })
    routeFingerprintsRef.current = nextFingerprints

    setLatencyMap(prev => {
      let changed = false
      const next: Record<string, number> = {}
      Object.entries(prev).forEach(([proxyId, latency]) => {
        if (validIds.has(proxyId) && !changedIds.has(proxyId)) next[proxyId] = latency
        else changed = true
      })
      return changed ? next : prev
    })

    setLatencyEngineMap(prev => {
      let changed = false
      const next: Record<string, string> = {}
      Object.entries(prev).forEach(([proxyId, engine]) => {
        if (validIds.has(proxyId) && !changedIds.has(proxyId)) next[proxyId] = engine
        else changed = true
      })
      return changed ? next : prev
    })

    setLatencyErrorMap(prev => {
      let changed = false
      const next: Record<string, string> = {}
      Object.entries(prev).forEach(([proxyId, error]) => {
        if (validIds.has(proxyId) && !changedIds.has(proxyId)) next[proxyId] = error
        else changed = true
      })
      return changed ? next : prev
    })

    setIPHealthMap(prev => {
      let changed = false
      const next: Record<string, ProxyIPHealthResult> = {}
      Object.entries(prev).forEach(([proxyId, health]) => {
        if (validIds.has(proxyId) && !changedIds.has(proxyId)) next[proxyId] = health
        else changed = true
      })
      return changed ? next : prev
      })

    setLatencyFingerprintMap(prev => {
      let changed = Object.keys(prev).length !== Object.keys(nextFingerprints).length
      if (!changed) {
        Object.entries(nextFingerprints).some(([proxyId, fingerprint]) => {
          if (prev[proxyId] !== fingerprint) {
            changed = true
            return true
          }
          return false
        })
      }
      return changed ? nextFingerprints : prev
    })
  }, [latencyFingerprintMap, proxies])

  const handleTestOne = async (record: ProxyDisplayInfo) => {
    if (record.proxyConfig === 'direct://') {
      toast.info('直连模式无需测速')
      return
    }
    setLatencyMap(prev => ({ ...prev, [record.proxyId]: -1 }))
    setLatencyEngineMap(prev => {
      const next = { ...prev }
      delete next[record.proxyId]
      return next
    })
    setLatencyErrorMap(prev => {
      const next = { ...prev }
      delete next[record.proxyId]
      return next
    })
    const expectedFingerprint = routeFingerprintsRef.current[record.proxyId]
    try {
      const result = await browserProxyTestSpeed(record.proxyId)
      if (!expectedFingerprint || routeFingerprintsRef.current[record.proxyId] !== expectedFingerprint) return
      const val = toLatencyValue(result.ok, result.latencyMs, result.error)
      setLatencyMap(prev => ({ ...prev, [record.proxyId]: val }))
      if (result.error) setLatencyErrorMap(prev => ({ ...prev, [record.proxyId]: result.error || '' }))
      if (result.engine) setLatencyEngineMap(prev => ({ ...prev, [record.proxyId]: result.engine || '' }))
    } catch (error: any) {
      if (!expectedFingerprint || routeFingerprintsRef.current[record.proxyId] !== expectedFingerprint) return
      const message = error?.message || '测速失败'
      setLatencyMap(prev => ({ ...prev, [record.proxyId]: toLatencyValue(false, 0, message) }))
      setLatencyErrorMap(prev => ({ ...prev, [record.proxyId]: message }))
      toast.error(message)
    }
  }

  const handleTestAll = async (items: ProxyDisplayInfo[]) => {
    const testable = items.filter(p => p.proxyConfig !== 'direct://')
    if (testable.length === 0) return
    setTestingAll(true)
    const init: Record<string, number> = {}
    testable.forEach(p => { init[p.proxyId] = -1 })
    setLatencyMap(prev => ({ ...prev, ...init }))
    setLatencyEngineMap(prev => {
      const next = { ...prev }
      testable.forEach(p => { delete next[p.proxyId] })
      return next
    })
    setLatencyErrorMap(prev => {
      const next = { ...prev }
      testable.forEach(p => { delete next[p.proxyId] })
      return next
    })

    const testableIds = new Set(testable.map(item => item.proxyId))
    const expectedFingerprints = new Map(testable.map(item => [item.proxyId, routeFingerprintsRef.current[item.proxyId]]))
    const isCurrentRoute = (proxyId: string) => {
      const expected = expectedFingerprints.get(proxyId)
      return !!expected && routeFingerprintsRef.current[proxyId] === expected
    }
    const off = EventsOn('proxy:speed:result', (data: ProxySpeedTestResult) => {
      if (!data?.proxyId || !testableIds.has(data.proxyId) || !isCurrentRoute(data.proxyId)) return
      const val = toLatencyValue(data.ok, data.latencyMs, data.error)
      setLatencyMap(prev => ({ ...prev, [data.proxyId]: val }))
      if (data.error) setLatencyErrorMap(prev => ({ ...prev, [data.proxyId]: data.error || '' }))
      if (data.engine) setLatencyEngineMap(prev => ({ ...prev, [data.proxyId]: data.engine || '' }))
    })

    try {
      const proxyIds = testable.map(p => p.proxyId)
      const results = await browserProxyBatchTestSpeed(proxyIds, 0)
      setLatencyMap(prev => {
        const next = { ...prev }
        results.forEach(result => {
          if (isCurrentRoute(result.proxyId)) next[result.proxyId] = toLatencyValue(result.ok, result.latencyMs, result.error)
        })
        return next
      })
      setLatencyEngineMap(prev => {
        const next = { ...prev }
        results.forEach(result => {
          if (result.engine && isCurrentRoute(result.proxyId)) next[result.proxyId] = result.engine
        })
        return next
      })
      setLatencyErrorMap(prev => {
        const next = { ...prev }
        results.forEach(result => {
          if (result.error && isCurrentRoute(result.proxyId)) next[result.proxyId] = result.error
        })
        return next
      })
    } catch (error: any) {
      const message = error?.message || '批量测速失败'
      setLatencyMap(prev => {
        const next = { ...prev }
        testable.forEach(item => {
          if (isCurrentRoute(item.proxyId)) next[item.proxyId] = toLatencyValue(false, 0, message)
        })
        return next
      })
      setLatencyErrorMap(prev => {
        const next = { ...prev }
        testable.forEach(item => {
          if (isCurrentRoute(item.proxyId)) next[item.proxyId] = message
        })
        return next
      })
      toast.error(message)
    } finally {
      off()
      setTestingAll(false)
    }
  }

  const handleWarmupOne = async (record: ProxyDisplayInfo) => {
    if (record.proxyConfig === 'direct://') {
      toast.info('直连模式无需预热')
      return
    }
    if (warmingBridgeIds.has(record.proxyId)) return

    setWarmingBridgeIds(prev => new Set(prev).add(record.proxyId))
    try {
      const result = await browserProxyWarmupBridge(record.proxyId)
      if (result.ok) toast.success(`${record.proxyName} 已预热`)
      else toast.error(result.error || `${record.proxyName} 预热失败`)
    } finally {
      setWarmingBridgeIds(prev => {
        const next = new Set(prev)
        next.delete(record.proxyId)
        return next
      })
    }
  }

  const handleWarmupAll = async (items: ProxyDisplayInfo[]) => {
    const testable = items.filter(p => p.proxyConfig !== 'direct://')
    if (testable.length === 0) return
    setWarmingAllBridges(true)
    const ids = testable.map(p => p.proxyId)
    setWarmingBridgeIds(prev => new Set([...Array.from(prev), ...ids]))
    try {
      const results = await browserProxyBatchWarmupBridge(ids, 5)
      const failed = results.filter(r => !r.ok).length
      if (failed > 0) toast.info(`预热完成：成功 ${results.length - failed}，失败 ${failed}`)
      else toast.success(`预热完成：共 ${results.length} 条`)
    } finally {
      setWarmingBridgeIds(prev => {
        const next = new Set(prev)
        ids.forEach(id => next.delete(id))
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

    setCheckingIPHealthIds(prev => new Set(prev).add(record.proxyId))
    try {
      const result = await browserProxyCheckIPHealth(record.proxyId)
      setIPHealthMap(prev => ({ ...prev, [record.proxyId]: result }))
      if (!result.ok) toast.error(result.error || `${record.proxyName} 检测失败`)
    } finally {
      setCheckingIPHealthIds(prev => {
        const next = new Set(prev)
        next.delete(record.proxyId)
        return next
      })
    }
  }

  const handleCheckAllIPHealth = async (items: ProxyDisplayInfo[]) => {
    const testable = items.filter(p => p.proxyConfig !== 'direct://')
    if (testable.length === 0) return
    setCheckingAllIPHealth(true)

    const ids = testable.map(p => p.proxyId)
    const idSet = new Set(ids)
    setCheckingIPHealthIds(prev => new Set([...Array.from(prev), ...ids]))

    const off = EventsOn('proxy:iphealth:result', (data: ProxyIPHealthResult) => {
      if (!data?.proxyId || !idSet.has(data.proxyId)) return
      setIPHealthMap(prev => ({ ...prev, [data.proxyId]: data }))
      setCheckingIPHealthIds(prev => {
        const next = new Set(prev)
        next.delete(data.proxyId)
        return next
      })
    })

    try {
      const results = await browserProxyBatchCheckIPHealth(ids, 10)
      setIPHealthMap(prev => {
        const next = { ...prev }
        results.forEach(result => {
          if (result?.proxyId && idSet.has(result.proxyId)) next[result.proxyId] = result
        })
        return next
      })
      const failed = results.filter(r => !r.ok).length
      if (failed > 0) toast.info(`IP 健康检测完成：成功 ${results.length - failed}，失败 ${failed}`)
      else toast.success(`IP 健康检测完成：共 ${results.length} 条`)
    } finally {
      off()
      setCheckingIPHealthIds(prev => {
        const next = new Set(prev)
        ids.forEach(id => next.delete(id))
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
    setLatencyMap,
    setLatencyEngineMap,
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
