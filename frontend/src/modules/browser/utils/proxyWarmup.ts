import { browserProxyWarmupBridgeWithConfig } from '../api'
import type { BrowserProfile } from '../types'

export async function warmupProfileProxyBeforeStart(profile: BrowserProfile | null | undefined): Promise<void> {
  if (!profile || profile.running || (!profile.proxyId && !profile.proxyConfig)) {
    return
  }
  try {
    await browserProxyWarmupBridgeWithConfig(profile.proxyId || '', profile.proxyConfig || '')
  } catch {
  }
}
