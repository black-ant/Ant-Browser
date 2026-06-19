import type { DashboardStats, DashboardMetrics, ActivityEntry } from './types'

const getBindings = async () => {
  try {
    return await import('../../wailsjs/go/main/App')
  } catch {
    return null
  }
}

// callDash 解析并调用绑定方法，回退到 window.go.main.App（兼容 wailsjs 未重新生成）
async function callDash<T>(method: string, args: any[], fallback: T): Promise<T> {
  const bindings: any = await getBindings()
  if (bindings?.[method]) {
    return ((await bindings[method](...args)) as T) ?? fallback
  }
  const goApp = (window as any).go?.main?.App
  if (goApp?.[method]) {
    return ((await goApp[method](...args)) as T) ?? fallback
  }
  return fallback
}

const EMPTY_METRICS: DashboardMetrics = {
  live: { timestamp: 0, cpuPercent: 0, memUsedMB: 0, memTotalMB: 0, appMemMB: 0, runningInstances: 0 },
  history: [],
}

// 真实资源指标（实时 + 历史，供图表）
export async function fetchDashboardMetrics(): Promise<DashboardMetrics> {
  const m = await callDash<DashboardMetrics>('GetDashboardMetrics', [], EMPTY_METRICS)
  return { live: m?.live ?? EMPTY_METRICS.live, history: m?.history ?? [] }
}

// 真实活动日志（最新在前）
export async function fetchActivityLog(): Promise<ActivityEntry[]> {
  return callDash<ActivityEntry[]>('GetActivityLog', [], [])
}

// 最近错误（最新在前）
export async function fetchRecentErrors(): Promise<ActivityEntry[]> {
  return callDash<ActivityEntry[]>('GetRecentErrors', [], [])
}

export async function fetchDashboardStats(): Promise<DashboardStats> {
  const bindings: any = await getBindings()
  if (bindings?.GetDashboardStats) {
    try {
      const data = await bindings.GetDashboardStats()
      const licenseStatus = bindings.GetLicenseStatus ? await bindings.GetLicenseStatus() : { maxLimit: 100 }
      return {
        totalInstances: data?.totalInstances ?? 0,
        runningInstances: data?.runningInstances ?? 0,
        proxyCount: data?.proxyCount ?? 0,
        proxyAvailable: data?.proxyAvailable ?? 0,
        coreCount: data?.coreCount ?? 0,
        memUsedMB: data?.memUsedMB ?? 0,
        maxProfileLimit: licenseStatus?.maxLimit ?? 100,
        appVersion: data?.appVersion ?? 'unknown',
      }
    } catch (e) {
      console.error('fetchDashboardStats error:', e)
    }
  }
  return { totalInstances: 0, runningInstances: 0, proxyCount: 0, proxyAvailable: 0, coreCount: 0, memUsedMB: 0, maxProfileLimit: 100, appVersion: 'unknown' }
}

export async function redeemCDKey(cdkey: string): Promise<{ success: boolean, message?: string }> {
  const bindings: any = await getBindings()
  if (bindings?.RedeemCDKey) {
    try {
      await bindings.RedeemCDKey(cdkey)
      return { success: true }
    } catch (e: any) {
      return { success: false, message: e.message || '兑换失败' }
    }
  }
  return { success: false, message: '系统 API 未就绪' }
}

export async function redeemGithubStar(): Promise<{ success: boolean, message?: string }> {
  const bindings: any = await getBindings()
  if (bindings?.RedeemGithubStar) {
    try {
      await bindings.RedeemGithubStar()
      return { success: true }
    } catch (e: any) {
      return { success: false, message: e.message || '领取失败' }
    }
  }
  return { success: false, message: '系统 API 未就绪' }
}

export async function reloadConfig(): Promise<void> {
  const bindings: any = await getBindings()
  if (bindings?.ReloadConfig) {
    try {
      await bindings.ReloadConfig()
    } catch (e) {
      console.error('reloadConfig error:', e)
    }
  }
}

export async function generateCDKeys(count: number): Promise<{ success: boolean, keys: string[], message?: string }> {
  const bindings: any = await getBindings()
  if (bindings?.GenerateCDKeys) {
    try {
      const keys = await bindings.GenerateCDKeys(count)
      return { success: true, keys: keys || [] }
    } catch (e: any) {
      return { success: false, keys: [], message: e.message || '生成失败' }
    }
  }
  return { success: false, keys: [], message: '系统 API 未就绪' }
}
