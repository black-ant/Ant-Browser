import { useEffect } from 'react'
import type { Dispatch, SetStateAction } from 'react'

import { EventsOn, EventsOff } from '../../../wailsjs/runtime/runtime'

import { fetchAutomationState } from '../api'
import type { AutomationState } from '../api'
import type { AutomationRuntimeProgress } from '../progress'

interface UseSettingsProgressEffectsOptions {
  setAutomationProgress: Dispatch<SetStateAction<AutomationRuntimeProgress | null>>
  setAutomationState: Dispatch<SetStateAction<AutomationState>>
}

function normalizeAutomationProgress(payload: AutomationRuntimeProgress | null | undefined) {
  if (!payload || typeof payload !== 'object') {
    return null
  }

  const phase = typeof payload.phase === 'string' ? payload.phase : 'checking'
  const progress = Number.isFinite(payload.progress) ? Math.max(0, Math.min(100, Math.round(payload.progress))) : 0
  const message = typeof payload.message === 'string' && payload.message.trim() ? payload.message.trim() : '正在准备自动化运行时...'
  const component = typeof payload.component === 'string' ? payload.component.trim() : ''

  return {
    phase,
    progress,
    message,
    component: component || undefined,
  }
}

export function useSettingsProgressEffects({
  setAutomationProgress,
  setAutomationState,
}: UseSettingsProgressEffectsOptions) {
  useEffect(() => {
    const onAutomationProgress = (payload: AutomationRuntimeProgress) => {
      const next = normalizeAutomationProgress(payload)
      if (!next) {
        return
      }

      setAutomationProgress(next)

      if (next.phase === 'done' || next.phase === 'error') {
        fetchAutomationState()
          .then(setAutomationState)
          .catch(() => {})
      }
    }

    EventsOn('automation:runtime:progress', onAutomationProgress)
    return () => {
      EventsOff('automation:runtime:progress')
    }
  }, [setAutomationProgress, setAutomationState])

}
