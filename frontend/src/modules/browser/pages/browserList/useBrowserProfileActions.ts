import type { Dispatch, SetStateAction } from 'react'
import { toast } from '../../../../shared/components'
import {
  deleteBrowserProfile,
  restartBrowserInstance,
  startBrowserInstance,
  startBrowserInstanceDirect,
  stopBrowserInstance,
  validateProxyConfig,
} from '../../api'
import type { BrowserProfile } from '../../types'
import { resolveActionErrorMessage, resolveActionFeedback } from '../../utils/actionErrors'
import { warmupProfileProxyBeforeStart } from '../../utils/proxyWarmup'

interface UseBrowserProfileActionsOptions {
  profiles: BrowserProfile[]
  setProxyErrorModal: (open: boolean) => void
  setProxyErrorMsg: (message: string) => void
  setPendingStartId: (profileId: string | null) => void
  setOpError: (message: string) => void
  setStartingIds: Dispatch<SetStateAction<Set<string>>>
  setStoppingIds: Dispatch<SetStateAction<Set<string>>>
  updatePendingIds: (
    setter: Dispatch<SetStateAction<Set<string>>>,
    profileId: string,
    active: boolean,
  ) => void
  mergeProfileState: (profile: BrowserProfile | null | undefined) => void
  loadProfiles: (options?: { silent?: boolean; syncRuntimeState?: boolean }) => Promise<BrowserProfile[] | void>
}

export function useBrowserProfileActions({
  profiles,
  setProxyErrorModal,
  setProxyErrorMsg,
  setPendingStartId,
  setOpError,
  setStartingIds,
  setStoppingIds,
  updatePendingIds,
  mergeProfileState,
  loadProfiles,
}: UseBrowserProfileActionsOptions) {
  const refreshProfilesInBackground = () => {
    void loadProfiles({ silent: true, syncRuntimeState: true }).catch(() => undefined)
  }

  const handleStart = async (profileId: string) => {
    const profile = profiles.find(p => p.profileId === profileId)
    updatePendingIds(setStartingIds, profileId, true)
    try {
      if (profile) {
        const result = await validateProxyConfig(profile.proxyConfig || '', profile.proxyId || '')
        if (!result.supported) {
          setProxyErrorMsg(result.errorMsg)
          setPendingStartId(profileId)
          setProxyErrorModal(true)
          updatePendingIds(setStartingIds, profileId, false)
          return
        }
      }

      await warmupProfileProxyBeforeStart(profile)
      const startedProfile = await startBrowserInstance(profileId)
      mergeProfileState(startedProfile)
      updatePendingIds(setStartingIds, profileId, false)
      if (startedProfile?.runtimeWarning) {
        toast.warning(startedProfile.runtimeWarning)
      }
      refreshProfilesInBackground()
    } catch (error: any) {
      updatePendingIds(setStartingIds, profileId, false)
      const feedback = resolveActionFeedback(error, '实例启动失败')
      if (feedback.tone === 'warning') {
        toast.warning(feedback.message)
      } else {
        toast.error(feedback.message)
      }
      refreshProfilesInBackground()
    }
  }

  const handleStartDirect = async (profileId: string) => {
    updatePendingIds(setStartingIds, profileId, true)
    try {
      const startedProfile = await startBrowserInstanceDirect(profileId)
      mergeProfileState(startedProfile)
      updatePendingIds(setStartingIds, profileId, false)
      setProxyErrorModal(false)
      setPendingStartId(null)
      if (startedProfile?.runtimeWarning) {
        toast.warning(startedProfile.runtimeWarning)
      }
      refreshProfilesInBackground()
    } catch (error: any) {
      updatePendingIds(setStartingIds, profileId, false)
      setProxyErrorModal(false)
      setPendingStartId(null)
      const feedback = resolveActionFeedback(error, '实例直连启动失败')
      if (feedback.tone === 'warning') {
        toast.warning(feedback.message)
      } else {
        toast.error(feedback.message)
      }
      refreshProfilesInBackground()
    }
  }

  const handleStop = async (profileId: string) => {
    updatePendingIds(setStoppingIds, profileId, true)
    try {
      const stoppedProfile = await stopBrowserInstance(profileId)
      mergeProfileState(stoppedProfile)
      updatePendingIds(setStoppingIds, profileId, false)
      refreshProfilesInBackground()
    } catch (error: any) {
      updatePendingIds(setStoppingIds, profileId, false)
      toast.error(resolveActionErrorMessage(error, '实例停止失败'))
      refreshProfilesInBackground()
    }
  }

  const handleRestart = async (profileId: string) => {
    const profile = profiles.find(p => p.profileId === profileId)
    updatePendingIds(setStoppingIds, profileId, true)
    try {
      await warmupProfileProxyBeforeStart(profile)
      const restartedProfile = await restartBrowserInstance(profileId)
      mergeProfileState(restartedProfile)
      updatePendingIds(setStoppingIds, profileId, false)
      if (restartedProfile?.runtimeWarning || (restartedProfile?.running && !restartedProfile.debugReady)) {
        toast.warning(restartedProfile.runtimeWarning || '浏览器窗口已启动，调试接口仍在后台接管。')
      }
      refreshProfilesInBackground()
    } catch (error: any) {
      updatePendingIds(setStoppingIds, profileId, false)
      const feedback = resolveActionFeedback(error, '实例重启失败')
      if (feedback.tone === 'warning') {
        toast.warning(feedback.message)
      } else {
        setOpError(feedback.message)
      }
      refreshProfilesInBackground()
    }
  }

  const handleDelete = async (profileId: string) => {
    await deleteBrowserProfile(profileId)
    toast.success('配置已删除')
    void loadProfiles()
  }

  return {
    handleStart,
    handleStartDirect,
    handleStop,
    handleRestart,
    handleDelete,
  }
}
