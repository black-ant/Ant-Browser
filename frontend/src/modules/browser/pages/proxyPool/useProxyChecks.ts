import { useCallback, useEffect, useRef, useState } from 'react'
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
import { isFreshProxySpeedTestAt, toLatencyValue } from './storage'
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
  proxies: Array<Pick<BrowserProxy, 'proxyId' | 'proxyConfig' | 'speedTestFingerprint' | 'lastTestedAt'>>
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
  const speedFingerprintsRef = useRef<Record<string, string>>({})
  const speedTestedAtRef = useRef<Record<string, string>>({})

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
    const currentFingerprints: Record<string, string> = {}
    const currentTestedAt: Record<string, string> = {}
    const changedIds = new Set<string>()
    const now = Date.now()
    proxies.forEach(proxy => {
      const fingerprint = (proxy.speedTestFingerprint || '').trim()
      currentFingerprints[proxy.proxyId] = fingerprint
      const backendTestedAt = (proxy.lastTestedAt || '').trim()
      const sessionTestedAt = speedTestedAtRef.current[proxy.proxyId] || ''
      const effectiveTestedAt = isFreshProxySpeedTestAt(sessionTestedAt, now)
        && (!isFreshProxySpeedTestAt(backendTestedAt, now) || Date.parse(sessionTestedAt) > Date.parse(backendTestedAt))
        ? sessionTestedAt
        : backendTestedAt
      if (isFreshProxySpeedTestAt(effectiveTestedAt, now)) {
        currentTestedAt[proxy.proxyId] = effectiveTestedAt
        speedTestedAtRef.current[proxy.proxyId] = effectiveTestedAt
      } else {
        delete speedTestedAtRef.current[proxy.proxyId]
      }
      if (!fingerprint || latencyFingerprintMap[proxy.proxyId] !== fingerprint || !currentTestedAt[proxy.proxyId]) changedIds.add(proxy.proxyId)
    })
    speedFingerprintsRef.current = currentFingerprints

    setLatencyMap(prev => {
      let changed = false
      const next: Record<string, number> = {}
      Object.entries(prev).forEach(([proxyId, latency]) => {
        if (validIds.has(proxyId) && !changedIds.has(proxyId) && currentTestedAt[proxyId]) {
          next[proxyId] = latency
        } else {
          changed = true
          delete speedTestedAtRef.current[proxyId]
        }
      })
      return changed ? next : prev
    })

    setLatencyEngineMap(prev => {
      let changed = false
      const next: Record<string, string> = {}
      Object.entries(prev).forEach(([proxyId, engine]) => {
        if (validIds.has(proxyId) && !changedIds.has(proxyId) && currentTestedAt[proxyId]) next[proxyId] = engine
        else changed = true
      })
      return changed ? next : prev
    })

    setLatencyErrorMap(prev => {
      let changed = false
      const next: Record<string, string> = {}
      Object.entries(prev).forEach(([proxyId, error]) => {
        if (validIds.has(proxyId) && !changedIds.has(proxyId) && currentTestedAt[proxyId]) next[proxyId] = error
        else changed = true
      })
      return changed ? next : prev
    })

    setIPHealthMap(prev => {
      let changed = false
      const next: Record<string, ProxyIPHealthResult> = {}
      Object.entries(prev).forEach(([proxyId, health]) => {
        if (validIds.has(proxyId)) next[proxyId] = health
        else changed = true
      })
      return changed ? next : prev
    })

    setLatencyFingerprintMap(prev => {
      const next: Record<string, string> = {}
      Object.entries(prev).forEach(([proxyId, fingerprint]) => {
        if (validIds.has(proxyId) && currentTestedAt[proxyId] && currentFingerprints[proxyId] && currentFingerprints[proxyId] === fingerprint) {
          next[proxyId] = fingerprint
        }
      })
      const changed = Object.keys(prev).length !== Object.keys(next).length
        || Object.entries(next).some(([proxyId, fingerprint]) => prev[proxyId] !== fingerprint)
      return changed ? next : prev
    })
  }, [latencyFingerprintMap, proxies])

  useEffect(() => {
    const timer = window.setInterval(() => {
      const now = Date.now()
      const expiredIds = Object.entries(speedTestedAtRef.current)
        .filter(([, testedAt]) => !isFreshProxySpeedTestAt(testedAt, now))
        .map(([proxyId]) => proxyId)
      if (expiredIds.length === 0) return
      expiredIds.forEach(proxyId => delete speedTestedAtRef.current[proxyId])
      const expired = new Set(expiredIds)
      setLatencyMap(prev => {
        const next = { ...prev }
        expired.forEach(proxyId => delete next[proxyId])
        return next
      })
      setLatencyEngineMap(prev => {
        const next = { ...prev }
        expired.forEach(proxyId => delete next[proxyId])
        return next
      })
      setLatencyErrorMap(prev => {
        const next = { ...prev }
        expired.forEach(proxyId => delete next[proxyId])
        return next
      })
      setLatencyFingerprintMap(prev => {
        const next = { ...prev }
        expired.forEach(proxyId => delete next[proxyId])
        return next
      })
    }, 60 * 1000)
    return () => window.clearInterval(timer)
  }, [])

  const isCurrentSpeedContext = (proxyId: string, expectedFingerprint: string) => {
    return expectedFingerprint !== '' && speedFingerprintsRef.current[proxyId] === expectedFingerprint
  }

  const isCurrentSpeedResult = (result: ProxySpeedTestResult, expectedFingerprints: Record<string, string>) => {
    const expectedFingerprint = expectedFingerprints[result.proxyId] || ''
    if (!isCurrentSpeedContext(result.proxyId, expectedFingerprint)) return false
    return result.fingerprint === expectedFingerprint
  }

  const applySpeedResult = (result: ProxySpeedTestResult, expectedFingerprints: Record<string, string>) => {
    if (!isCurrentSpeedResult(result, expectedFingerprints)) return false
    const val = toLatencyValue(result.ok, result.latencyMs, result.error)
    setLatencyMap(prev => ({ ...prev, [result.proxyId]: val }))
    setLatencyErrorMap(prev => {
      const next = { ...prev }
      if (result.error) next[result.proxyId] = result.error
      else delete next[result.proxyId]
      return next
    })
    setLatencyEngineMap(prev => {
      const next = { ...prev }
      if (result.engine) next[result.proxyId] = result.engine
      else delete next[result.proxyId]
      return next
    })
    setLatencyFingerprintMap(prev => {
      const next = { ...prev }
      if (result.fingerprint) next[result.proxyId] = result.fingerprint
      else delete next[result.proxyId]
      return next
    })
    speedTestedAtRef.current[result.proxyId] = new Date().toISOString()
    return true
  }

  const clearSpeedResults = useCallback(() => {
    setLatencyMap({})
    setLatencyEngineMap({})
    setLatencyErrorMap({})
    setLatencyFingerprintMap({})
    speedTestedAtRef.current = {}
  }, [])

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
    const expectedFingerprints = { [record.proxyId]: speedFingerprintsRef.current[record.proxyId] || '' }
    try {
      const result = await browserProxyTestSpeed(record.proxyId)
      applySpeedResult(result, expectedFingerprints)
    } catch (error: any) {
      if (!isCurrentSpeedContext(record.proxyId, expectedFingerprints[record.proxyId])) return
      const message = error?.message || '测速失败'
      speedTestedAtRef.current[record.proxyId] = new Date().toISOString()
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
    const expectedFingerprints: Record<string, string> = {}
    testable.forEach(item => { expectedFingerprints[item.proxyId] = speedFingerprintsRef.current[item.proxyId] || '' })
    const off = EventsOn('proxy:speed:result', (data: ProxySpeedTestResult) => {
      if (!data?.proxyId || !testableIds.has(data.proxyId)) return
      applySpeedResult(data, expectedFingerprints)
    })

    try {
      const proxyIds = testable.map(p => p.proxyId)
      const results = await browserProxyBatchTestSpeed(proxyIds, 0)
      results.forEach(result => applySpeedResult(result, expectedFingerprints))
    } catch (error: any) {
      const message = error?.message || '批量测速失败'
      const testedAt = new Date().toISOString()
      setLatencyMap(prev => {
        const next = { ...prev }
        testable.forEach(item => {
          if (isCurrentSpeedContext(item.proxyId, expectedFingerprints[item.proxyId])) {
            speedTestedAtRef.current[item.proxyId] = testedAt
            next[item.proxyId] = toLatencyValue(false, 0, message)
          }
        })
        return next
      })
      setLatencyErrorMap(prev => {
        const next = { ...prev }
        testable.forEach(item => {
          if (isCurrentSpeedContext(item.proxyId, expectedFingerprints[item.proxyId])) next[item.proxyId] = message
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
    clearSpeedResults,
    handleTestOne,
    handleTestAll,
    handleWarmupOne,
    handleWarmupAll,
    handleCheckOneIPHealth,
    handleCheckAllIPHealth,
    openIPHealthDetail,
  }
}
