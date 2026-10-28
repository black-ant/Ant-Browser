import { EventsOn } from '../../wailsjs/runtime/runtime'
import { browserProxyBatchTestSpeed, browserProxyTestSpeed } from './api'
import { useProxySpeedStore, type ProxySpeedResult } from './proxySpeedStore'
import type { ProxySpeedTestResult } from './types'

const SPEED_RESULT_EVENT = 'proxy:speed:result'

// 单次测速最长等待：内核启动 15s + 两次探测 3s + 重试余量。
const SPEED_TEST_TIMEOUT_MS = 45_000
// 批量测速按并发分批，给足整体余量。
const SPEED_BATCH_TIMEOUT_MS = 180_000

let listenerReady = false

function withTimeout<T>(promise: Promise<T>, timeoutMs: number, message: string): Promise<T> {
  return new Promise<T>((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error(message)), timeoutMs)
    promise.then(
      value => {
        clearTimeout(timer)
        resolve(value)
      },
      error => {
        clearTimeout(timer)
        reject(error)
      },
    )
  })
}

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
    const result = await withTimeout(browserProxyTestSpeed(id), SPEED_TEST_TIMEOUT_MS, '测速超时：内核启动或探测耗时过长')
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
    const results = await withTimeout(browserProxyBatchTestSpeed(ids, concurrency), SPEED_BATCH_TIMEOUT_MS, '批量测速超时')
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