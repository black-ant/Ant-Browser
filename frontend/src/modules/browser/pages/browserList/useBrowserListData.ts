import { useEffect, useRef, useState } from 'react'
import type { BrowserGroupWithCount, BrowserProfile, BrowserProxy } from '../../types'
import { fetchBrowserProfiles, fetchBrowserProxies, fetchGroups } from '../../api'
import { EventsOn } from '../../../../wailsjs/runtime/runtime'

interface UseBrowserListDataOptions {
  loadCores: () => void
}

export function useBrowserListData({ loadCores }: UseBrowserListDataOptions) {
  const [profiles, setProfiles] = useState<BrowserProfile[]>([])
  const [loading, setLoading] = useState(true)
  const [proxies, setProxies] = useState<BrowserProxy[]>([])
  const [groups, setGroups] = useState<BrowserGroupWithCount[]>([])
  const [startingIds, setStartingIds] = useState<Set<string>>(new Set())
  const [stoppingIds, setStoppingIds] = useState<Set<string>>(new Set())
  const profilesRef = useRef<BrowserProfile[]>([])
  const silentRefreshInFlightRef = useRef(false)

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

  useEffect(() => {
    void loadProfiles()
    loadGroups()
    fetchBrowserProxies().then(setProxies)
    loadCores()

    const clearPending = (payload: any) => {
      const profileId = typeof payload === 'string' ? payload : payload?.profileId
      if (profileId) {
        updatePendingIds(setStartingIds, profileId, false)
        updatePendingIds(setStoppingIds, profileId, false)
      }
    }

    const offStarted = EventsOn('browser:instance:started', (payload: any) => {
      clearPending(payload)
      void loadProfiles({ silent: true, syncRuntimeState: true })
    })
    const offUpdated = EventsOn('browser:instance:updated', () => {
      void loadProfiles({ silent: true, syncRuntimeState: true })
    })
    const offStopped = EventsOn('browser:instance:stopped', (payload: any) => {
      clearPending(payload)
      void loadProfiles({ silent: true, syncRuntimeState: true })
    })
    const offCrashed = EventsOn('browser:instance:crashed', (payload: any) => {
      clearPending(payload)
      void loadProfiles({ silent: true, syncRuntimeState: true })
    })

    const timer = window.setInterval(() => {
      if (document.visibilityState !== 'visible') return
      void loadProfiles({ silent: true, syncRuntimeState: true })
    }, 2000)

    return () => {
      window.clearInterval(timer)
      offStarted?.()
      offUpdated?.()
      offStopped?.()
      offCrashed?.()
    }
  }, [])

  return {
    profiles,
    loading,
    proxies,
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
    loadGroups,
  }
}
