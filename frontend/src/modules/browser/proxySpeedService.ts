import { EventsOn } from '../../wailsjs/runtime/runtime'
import { browserProxyBatchTestSpeed, browserProxyTestSpeed } from './api'
import { useProxySpeedStore, type ProxySpeedResult } from './proxySpeedStore'
import type { ProxySpeedTestResult } from './types'

const SPEED_RESULT_EVENT = 'proxy:speed:result'


let listenerReady = false

function normalizeResult(proxyId: string, result: Partial<ProxySpeedTestResult>): ProxySpeedResult {
  return {
    proxyId,
    ok: !!result.ok,
    latencyMs: typeof result.latencyMs === 'number' ? result.latencyMs : -1,
    engine: result.engine,
    error: result.error || '',
    testedAt: result.testedAt || new Date().toISOString(),
    fingerprint: result.fingerprint,
  }
}

// 所有页面共用的底层测速结果监听：后端推来的任何测速结果都写入唯一 store。
export function ensureProxySpeedListener() {
  if (listenerReady) return
  listenerReady = true
  EventsOn(SPEED_RESULT_EVENT, (data: ProxySpeedTestResult) => {
    if (!data?.proxyId) return
    const store = useProxySpeedStore.getState()
    store.setResult(normalizeResult(data.proxyId, data))
    store.setTesting(data.proxyId, false)
  })
}

export async function testProxySpeedOnce(proxyId: string): Promise<ProxySpeedResult> {
  const id = proxyId.trim()
  if (!id) {
    return normalizeResult(proxyId, { ok: false, latencyMs: -1, error: '缺少代理 ID' })
  }
  ensureProxySpeedListener()
  const store = useProxySpeedStore.getState()
  store.setTesting(id, true)
  try {
    const result = await browserProxyTestSpeed(id)
    const normalized = normalizeResult(id, result)
    useProxySpeedStore.getState().setResult(normalized)
    return normalized
  } catch (error: any) {
    const normalized = normalizeResult(id, { ok: false, latencyMs: -1, error: error?.message || '测速失败' })
    useProxySpeedStore.getState().setResult(normalized)
    return normalized
  } finally {
    useProxySpeedStore.getState().setTesting(id, false)
  }
}

export async function testProxySpeeds(proxyIds: string[], concurrency = 0): Promise<ProxySpeedResult[]> {
  const ids = Array.from(new Set(proxyIds.map((id) => id.trim()).filter(Boolean)))
  if (ids.length === 0) return []
  ensureProxySpeedListener()
  const store = useProxySpeedStore.getState()
  store.setManyTesting(ids, true)
  try {
    const results = await browserProxyBatchTestSpeed(ids, concurrency)
    const normalized = results
      .filter((result) => !!result?.proxyId)
      .map((result) => normalizeResult(result.proxyId, result))
    useProxySpeedStore.getState().setManyResults(normalized)
    return normalized
  } catch (error: any) {
    const message = error?.message || '批量测速失败'
    const failed = ids.map((id) => normalizeResult(id, { ok: false, latencyMs: -1, error: message }))
    useProxySpeedStore.getState().setManyResults(failed)
    return failed
  } finally {
    useProxySpeedStore.getState().setManyTesting(ids, false)
  }
}

// 供实例“真实连通性”等场景把结果写回唯一 store。
export function recordProxySpeedResult(result: Partial<ProxySpeedTestResult> & { proxyId: string }) {
  if (!result?.proxyId) return
  useProxySpeedStore.getState().setResult(normalizeResult(result.proxyId, result))
}