import { useEffect, useState } from 'react'

import { isProxySpeedResultFresh, PROXY_SPEED_RESULT_MAX_AGE_MS } from './pages/proxyPool/storage'

export const PROXY_SPEED_SETTINGS_CHANGED_EVENT = 'proxy:speed:settings-changed'

export function notifyProxySpeedResultSettingsChanged() {
  if (typeof window === 'undefined') return
  window.dispatchEvent(new Event(PROXY_SPEED_SETTINGS_CHANGED_EVENT))
}

// 选择弹窗、实例列表、插件管理页共用的测速结果显示窗口（36 小时）。
export function useProxySpeedFreshness() {
  const [now, setNow] = useState(() => Date.now())

  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 30_000)
    return () => window.clearInterval(timer)
  }, [])

  return { now, speedResultMaxAgeMs: PROXY_SPEED_RESULT_MAX_AGE_MS }
}

export { isProxySpeedResultFresh }
