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
          return
        }
      }

      await warmupProfileProxyBeforeStart(profile)
      const startedProfile = await startBrowserInstance(profileId)
      mergeProfileState(startedProfile)
      if (startedProfile?.runtimeWarning) {
        toast.warning(startedProfile.runtimeWarning)
      } else {
        toast.success(`实例已启动${startedProfile?.profileName ? `：${startedProfile.profileName}` : ''}`)
      }
      await loadProfiles({ silent: true, syncRuntimeState: true })
    } catch (error: any) {
      const feedback = resolveActionFeedback(error, '实例启动失败')
      if (feedback.tone === 'warning') {
        toast.warning(feedback.message)
      } else {
        toast.error(feedback.message)
      }
      await loadProfiles({ silent: true, syncRuntimeState: true })
    } finally {
      updatePendingIds(setStartingIds, profileId, false)
    }
  }

  const handleStartDirect = async (profileId: string) => {
    updatePendingIds(setStartingIds, profileId, true)
    try {
      const startedProfile = await startBrowserInstanceDirect(profileId)
      mergeProfileState(startedProfile)
      setProxyErrorModal(false)
      setPendingStartId(null)
      if (startedProfile?.runtimeWarning) {
        toast.warning(startedProfile.runtimeWarning)
      } else {
        toast.success(`实例已直连启动${startedProfile?.profileName ? `：${startedProfile.profileName}` : ''}`)
      }
      await loadProfiles({ silent: true, syncRuntimeState: true })
    } catch (error: any) {
      setProxyErrorModal(false)
      setPendingStartId(null)
      const feedback = resolveActionFeedback(error, '实例直连启动失败')
      if (feedback.tone === 'warning') {
        toast.warning(feedback.message)
      } else {
        toast.error(feedback.message)
      }
      await loadProfiles({ silent: true, syncRuntimeState: true })
    } finally {
      updatePendingIds(setStartingIds, profileId, false)
    }
  }

  const handleStop = async (profileId: string) => {
    updatePendingIds(setStoppingIds, profileId, true)
    try {
      const stoppedProfile = await stopBrowserInstance(profileId)
      mergeProfileState(stoppedProfile)
      toast.success('实例已停止')
      await loadProfiles({ silent: true, syncRuntimeState: true })
    } catch (error: any) {
      toast.error(resolveActionErrorMessage(error, '实例停止失败'))
      await loadProfiles({ silent: true, syncRuntimeState: true })
    } finally {
      updatePendingIds(setStoppingIds, profileId, false)
    }
  }

  const handleRestart = async (profileId: string) => {
    const profile = profiles.find(p => p.profileId === profileId)
    updatePendingIds(setStoppingIds, profileId, true)
    try {
      await warmupProfileProxyBeforeStart(profile)
      const restartedProfile = await restartBrowserInstance(profileId)
      mergeProfileState(restartedProfile)
      if (restartedProfile?.runtimeWarning || (restartedProfile?.running && !restartedProfile.debugReady)) {
        toast.warning(restartedProfile.runtimeWarning || '浏览器窗口已启动，调试接口仍在后台接管。')
      } else {
        toast.success(`实例已重启${restartedProfile?.profileName ? `：${restartedProfile.profileName}` : ''}`)
      }
      await loadProfiles({ silent: true, syncRuntimeState: true })
    } catch (error: any) {
      const feedback = resolveActionFeedback(error, '实例重启失败')
      if (feedback.tone === 'warning') {
        toast.warning(feedback.message)
      } else {
        setOpError(feedback.message)
      }
      await loadProfiles({ silent: true, syncRuntimeState: true })
    } finally {
      updatePendingIds(setStoppingIds, profileId, false)
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
