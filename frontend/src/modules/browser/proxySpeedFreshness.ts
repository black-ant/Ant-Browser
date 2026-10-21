import { useEffect, useState } from 'react'

import { fetchProxyCheckSettings } from './api/proxyCheck'
import { isProxySpeedResultFresh, PROXY_SPEED_RESULT_MAX_AGE_MS, resolveProxySpeedResultMaxAgeMs } from './pages/proxyPool/storage'

export const PROXY_SPEED_SETTINGS_CHANGED_EVENT = 'proxy:speed:settings-changed'
const PROXY_SPEED_FRESHNESS_REFRESH_INTERVAL_MS = 30_000

export function notifyProxySpeedResultSettingsChanged() {
  if (typeof window === 'undefined') return
  window.dispatchEvent(new Event(PROXY_SPEED_SETTINGS_CHANGED_EVENT))
}

export function useProxySpeedFreshness() {
  const [speedResultMaxAgeMs, setSpeedResultMaxAgeMs] = useState(PROXY_SPEED_RESULT_MAX_AGE_MS)
  const [now, setNow] = useState(() => Date.now())

  useEffect(() => {
    let active = true

    const refreshSettings = async () => {
      try {
        const settings = await fetchProxyCheckSettings()
        if (active) setSpeedResultMaxAgeMs(resolveProxySpeedResultMaxAgeMs(settings.speedIntervalMinutes))
      } catch {
        return
      }
    }

    const handleSettingsChanged = () => {
      setNow(Date.now())
      void refreshSettings()
    }

    void refreshSettings()
    window.addEventListener(PROXY_SPEED_SETTINGS_CHANGED_EVENT, handleSettingsChanged)
    const timer = window.setInterval(() => setNow(Date.now()), PROXY_SPEED_FRESHNESS_REFRESH_INTERVAL_MS)
    return () => {
      active = false
      window.removeEventListener(PROXY_SPEED_SETTINGS_CHANGED_EVENT, handleSettingsChanged)
      window.clearInterval(timer)
    }
  }, [])

  return { now, speedResultMaxAgeMs }
}

export { isProxySpeedResultFresh }
