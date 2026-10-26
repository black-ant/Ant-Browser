import { create } from 'zustand'

import type { ProxySpeedTestResult } from './types'

// 全局唯一的测速结果存放处：所有页面共享同一份，不再各自维护 speedMap / 本地缓存。
export interface ProxySpeedResult extends Omit<ProxySpeedTestResult, 'testedAt'> {
  testedAt: string
}

export interface ProxySpeedSource {
  proxyId: string
  lastTestedAt?: string
  lastLatencyMs?: number
  lastTestOk?: boolean
  lastTestEngine?: string
  speedTestFingerprint?: string
}

interface ProxySpeedStoreState {
  results: Record<string, ProxySpeedResult>
  testingIds: Record<string, true>
  setResult: (result: ProxySpeedResult) => void
  setManyResults: (results: ProxySpeedResult[]) => void
  setTesting: (proxyId: string, testing: boolean) => void
  setManyTesting: (proxyIds: string[], testing: boolean) => void
  applyBackendResults: (proxies: ProxySpeedSource[], isFresh: (testedAt?: string) => boolean) => void
  prune: (validIds: Set<string>) => void
  reset: () => void
}

export const useProxySpeedStore = create<ProxySpeedStoreState>((set) => ({
  results: {},
  testingIds: {},
  setResult: (result) => set((state) => ({
    results: { ...state.results, [result.proxyId]: result },
  })),
  setManyResults: (list) => set((state) => {
    if (list.length === 0) return {}
    const next = { ...state.results }
    list.forEach((result) => { next[result.proxyId] = result })
    return { results: next }
  }),
  setTesting: (proxyId, testing) => set((state) => {
    if (testing) {
      if (state.testingIds[proxyId]) return {}
      return { testingIds: { ...state.testingIds, [proxyId]: true } }
    }
    if (!state.testingIds[proxyId]) return {}
    const next = { ...state.testingIds }
    delete next[proxyId]
    return { testingIds: next }
  }),
  setManyTesting: (proxyIds, testing) => set((state) => {
    const next = { ...state.testingIds }
    let changed = false
    proxyIds.forEach((proxyId) => {
      if (!proxyId) return
      if (testing) {
        if (!next[proxyId]) { next[proxyId] = true; changed = true }
      } else if (next[proxyId]) {
        delete next[proxyId]
        changed = true
      }
    })
    return changed ? { testingIds: next } : {}
  }),
  applyBackendResults: (proxies, isFresh) => set((state) => {
    let changed = false
    const next = { ...state.results }
    proxies.forEach((proxy) => {
      if (next[proxy.proxyId]) return
      if (!proxy.lastTestedAt || typeof proxy.lastLatencyMs !== 'number') return
      if (!isFresh(proxy.lastTestedAt)) return
      next[proxy.proxyId] = {
        proxyId: proxy.proxyId,
        ok: proxy.lastTestOk ?? false,
        latencyMs: proxy.lastLatencyMs,
        engine: proxy.lastTestEngine || undefined,
        error: '',
        testedAt: proxy.lastTestedAt,
        fingerprint: proxy.speedTestFingerprint || undefined,
      }
      changed = true
    })
    return changed ? { results: next } : {}
  }),
  prune: (validIds) => set((state) => {
    if (validIds.size === 0) return {}
    let changed = false
    const next: Record<string, ProxySpeedResult> = {}
    Object.entries(state.results).forEach(([proxyId, result]) => {
      if (validIds.has(proxyId)) next[proxyId] = result
      else changed = true
    })
    return changed ? { results: next } : {}
  }),
  reset: () => set({ results: {}, testingIds: {} }),
}))