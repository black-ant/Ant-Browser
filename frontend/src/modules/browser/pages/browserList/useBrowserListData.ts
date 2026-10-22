import { useEffect, useRef, useState } from 'react'
import { toast } from '../../../../shared/components'
import type { BrowserGroupWithCount, BrowserProfile, BrowserProxy, BrowserProxyVirtualGroup } from '../../types'
import { fetchBrowserProfiles, fetchBrowserProxies, fetchBrowserProxyVirtualGroups, fetchGroups } from '../../api'
import { EventsOn } from '../../../../wailsjs/runtime/runtime'

const PROFILE_LIST_FALLBACK_REFRESH_MS = 15_000
const APP_READY_EVENT = 'app:ready'

interface UseBrowserListDataOptions {
  loadCores: () => Promise<void>
}

export function useBrowserListData({ loadCores }: UseBrowserListDataOptions) {
  const [profiles, setProfiles] = useState<BrowserProfile[]>([])
  const [loading, setLoading] = useState(true)
  const [proxies, setProxies] = useState<BrowserProxy[]>([])
  const [proxyVirtualGroups, setProxyVirtualGroups] = useState<BrowserProxyVirtualGroup[]>([])
  const [groups, setGroups] = useState<BrowserGroupWithCount[]>([])
  const [startingIds, setStartingIds] = useState<Set<string>>(new Set())
  const [stoppingIds, setStoppingIds] = useState<Set<string>>(new Set())
  const profilesRef = useRef<BrowserProfile[]>([])
  const silentRefreshInFlightRef = useRef(false)
  const initialLoadInFlightRef = useRef(false)
  const initialLoadQueuedRef = useRef(false)

  const updatePendingIds = (
    setter: React.Dispatch<React.SetStateAction<Set<string>>>,
    profileId: string,
    active: boolean
  ) => {
    setter(prev => {
      const next = new Set(prev)
      if (active) {
        next.add(profileId)
      } else {
        next.delete(profileId)
      }
      return next
    })
  }

  const replaceProfilesState = (items: BrowserProfile[]) => {
    profilesRef.current = items
    setProfiles(items)
  }

  const updateProfilesState = (updater: (items: BrowserProfile[]) => BrowserProfile[]) => {
    const next = updater(profilesRef.current)
    profilesRef.current = next
    setProfiles(next)
  }

  const mergeProfileState = (profile: BrowserProfile | null | undefined) => {
    if (!profile) return
    updateProfilesState(prev => prev.map(item => (
      item.profileId === profile.profileId ? { ...item, ...profile } : item
    )))
  }

  const syncProfiles = (items: BrowserProfile[], syncRuntimeState: boolean) => {
    if (syncRuntimeState) {
      const previousById = new Map(profilesRef.current.map(item => [item.profileId, item]))
      const newlyRunning = items.find(item => item.running && !previousById.get(item.profileId)?.running)
      if (newlyRunning) {
        updatePendingIds(setStartingIds, newlyRunning.profileId, false)
        updatePendingIds(setStoppingIds, newlyRunning.profileId, false)
      }
      items.forEach(item => {
        if (!item.running && previousById.get(item.profileId)?.running) {
          updatePendingIds(setStartingIds, item.profileId, false)
          updatePendingIds(setStoppingIds, item.profileId, false)
        }
      })
    }
    replaceProfilesState(items)
  }

  const loadProfiles = async ({ silent = false, syncRuntimeState = false }: { silent?: boolean; syncRuntimeState?: boolean } = {}) => {
    if (silent && silentRefreshInFlightRef.current) {
      return profilesRef.current
    }
    if (!silent) {
      setLoading(true)
    } else {
      silentRefreshInFlightRef.current = true
    }
    try {
      const items = await fetchBrowserProfiles()
      syncProfiles(items, syncRuntimeState)
      return items
    } finally {
      if (silent) {
        silentRefreshInFlightRef.current = false
      } else {
        setLoading(false)
      }
    }
  }

  const loadGroups = async () => {
    setGroups(await fetchGroups())
  }

  const updateProxiesState = (items: BrowserProxy[]) => {
    setProxies(items)
  }

  const loadProxyCatalog = async () => {
    const [proxyResult, virtualGroupResult] = await Promise.allSettled([
      fetchBrowserProxies(),
      fetchBrowserProxyVirtualGroups(),
    ])
    if (proxyResult.status === 'fulfilled') {
      setProxies(proxyResult.value)
    } else {
      setProxies([])
      toast.error(proxyResult.reason instanceof Error ? proxyResult.reason.message : '加载代理目录失败')
    }
    if (virtualGroupResult.status === 'fulfilled') {
      setProxyVirtualGroups(virtualGroupResult.value)
    } else {
      setProxyVirtualGroups([])
    }
  }

  useEffect(() => {
    let disposed = false
    let runtimeRetryTimer: number | null = null
    let runtimeSubscriptionFailed = false
    const runtimeUnsubscribers: Array<() => void> = []

    const loadInitialData = () => {
      if (disposed) return
      if (initialLoadInFlightRef.current) {
        initialLoadQueuedRef.current = true
        return
      }

      initialLoadQueuedRef.current = false
      initialLoadInFlightRef.current = true
      void Promise.allSettled([
        loadProfiles(),
        loadGroups(),
        loadProxyCatalog(),
        loadCores(),
      ]).finally(() => {
        initialLoadInFlightRef.current = false
        if (!disposed && initialLoadQueuedRef.current) {
          initialLoadQueuedRef.current = false
          loadInitialData()
        }
      })
    }

    const clearPending = (payload: any) => {
      const profileId = typeof payload === 'string' ? payload : payload?.profileId
      if (profileId) {
        updatePendingIds(setStartingIds, profileId, false)
        updatePendingIds(setStoppingIds, profileId, false)
      }
    }

    loadInitialData()

    const subscribeRuntimeEvents = () => {
      if (disposed) return
      const subscriptions: Array<() => void> = []
      try {
        const subscribe = (eventName: string, handler: (...data: any[]) => void) => {
          const unsubscribe = EventsOn(eventName, handler)
          if (typeof unsubscribe !== 'function') {
            throw new Error('runtime event subscription unavailable')
          }
          subscriptions.push(unsubscribe)
        }

        subscribe('browser:instance:started', (payload: any) => {
          clearPending(payload)
          void loadProfiles({ silent: true, syncRuntimeState: true })
        })
        subscribe('browser:instance:updated', (payload: any) => {
          clearPending(payload)
          void loadProfiles({ silent: true, syncRuntimeState: true })
        })
        subscribe('browser:instance:stopped', (payload: any) => {
          clearPending(payload)
          void loadProfiles({ silent: true, syncRuntimeState: true })
        })
        subscribe('browser:instance:crashed', (payload: any) => {
          clearPending(payload)
          void loadProfiles({ silent: true, syncRuntimeState: true })
        })
        subscribe(APP_READY_EVENT, () => {
          loadInitialData()
        })

        runtimeUnsubscribers.push(...subscriptions)
        if (runtimeSubscriptionFailed) {
          runtimeSubscriptionFailed = false
          loadInitialData()
        }
      } catch {
        subscriptions.forEach(unsubscribe => unsubscribe())
        runtimeSubscriptionFailed = true
        if (runtimeRetryTimer === null) {
          runtimeRetryTimer = window.setTimeout(() => {
            runtimeRetryTimer = null
            subscribeRuntimeEvents()
          }, 250)
        }
      }
    }

    subscribeRuntimeEvents()

    const timer = window.setInterval(() => {
      if (document.visibilityState !== 'visible') return
      void loadProfiles({ silent: true, syncRuntimeState: true })
    }, PROFILE_LIST_FALLBACK_REFRESH_MS)

    return () => {
      disposed = true
      window.clearInterval(timer)
      if (runtimeRetryTimer !== null) {
        window.clearTimeout(runtimeRetryTimer)
      }
      runtimeUnsubscribers.forEach(unsubscribe => unsubscribe())
    }
  }, [])

  return {
    profiles,
    loading,
    proxies,
    proxyVirtualGroups,
    groups,
    startingIds,
    stoppingIds,
    setStartingIds,
    setStoppingIds,
    updatePendingIds,
    updateProfilesState,
    mergeProfileState,
    updateProxiesState,
    loadProfiles,
  }
}
