import { useEffect } from 'react'
import type { Dispatch, RefObject, SetStateAction } from 'react'

import { EventsOn, EventsOff } from '../../../wailsjs/runtime/runtime'
import { useBackupStore } from '../../../store/backupStore'

import { fetchAutomationState } from '../api'
import type { AutomationState } from '../api'
import type { AutomationRuntimeProgress, BackupExportLogItem, BackupExportProgress } from '../progress'

type SettingsActionLoading = 'none' | 'init' | 'export' | 'import-reset' | 'import-merge'

interface UseSettingsProgressEffectsOptions {
  actionLoading: SettingsActionLoading
  exportLogs: BackupExportLogItem[]
  exportLogsRef: RefObject<HTMLDivElement>
  importProgress: BackupExportProgress | null
  setAutomationProgress: Dispatch<SetStateAction<AutomationRuntimeProgress | null>>
  setAutomationState: Dispatch<SetStateAction<AutomationState>>
  setExportLogs: Dispatch<SetStateAction<BackupExportLogItem[]>>
  setExportProgress: Dispatch<SetStateAction<BackupExportProgress | null>>
  setImportProgress: Dispatch<SetStateAction<BackupExportProgress | null>>
}

function normalizeBackupProgress(payload: BackupExportProgress | null | undefined, fallbackPhase: string, fallbackMessage: string) {
  if (!payload || typeof payload !== 'object') {
    return null
  }

  const phase = typeof payload.phase === 'string' ? payload.phase : fallbackPhase
  const progress = Number.isFinite(payload.progress) ? Math.max(0, Math.min(100, Math.round(payload.progress))) : 0
  const message = typeof payload.message === 'string' && payload.message.trim() ? payload.message.trim() : fallbackMessage
  const componentId = typeof payload.componentId === 'string' ? payload.componentId.trim() : ''
  const componentName = typeof payload.componentName === 'string' ? payload.componentName.trim() : ''
  const entryIndex = Number.isFinite(payload.entryIndex) ? Math.max(0, Math.round(payload.entryIndex || 0)) : 0
  const entryTotal = Number.isFinite(payload.entryTotal) ? Math.max(0, Math.round(payload.entryTotal || 0)) : 0
  const timestamp = typeof payload.timestamp === 'string' && payload.timestamp.trim()
    ? payload.timestamp.trim()
    : new Date().toLocaleTimeString('zh-CN', { hour12: false })

  return {
    phase,
    progress,
    message,
    componentId: componentId || undefined,
    componentName: componentName || undefined,
    entryIndex: entryIndex || undefined,
    entryTotal: entryTotal || undefined,
    timestamp,
  }
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
  actionLoading,
  exportLogs,
  exportLogsRef,
  importProgress,
  setAutomationProgress,
  setAutomationState,
  setExportLogs,
  setExportProgress,
  setImportProgress,
}: UseSettingsProgressEffectsOptions) {
  const setImportState = useBackupStore((state) => state.setImportState)
  const clearImportState = useBackupStore((state) => state.clearImportState)

  useEffect(() => {
    const onExportProgress = (payload: BackupExportProgress) => {
      const next = normalizeBackupProgress(payload, 'writing', '正在导出...')
      if (!next) {
        return
      }
      if (next.phase === 'cancelled') {
        setExportProgress(null)
        setExportLogs([])
        return
      }

      setExportProgress(next)

      const prefix = next.componentName ? `[${next.componentName}] ` : next.componentId ? `[${next.componentId}] ` : ''
      const text = `${prefix}${next.message}`
      setExportLogs(prev => {
        const last = prev[prev.length - 1]
        if (last && last.text === text && last.phase === next.phase) {
          return prev
        }
        const nextLogs = [...prev, { id: Date.now() + Math.floor(Math.random() * 1000), phase: next.phase, time: next.timestamp || '', text }]
        return nextLogs.length > 120 ? nextLogs.slice(nextLogs.length - 120) : nextLogs
      })
    }

    EventsOn('backup:export:progress', onExportProgress)
    return () => {
      EventsOff('backup:export:progress')
    }
  }, [setExportLogs, setExportProgress])

  useEffect(() => {
    const onImportProgress = (payload: BackupExportProgress) => {
      const next = normalizeBackupProgress(payload, 'importing', '正在导入备份...')
      if (!next) {
        return
      }
      if (next.phase === 'cancelled') {
        setImportProgress(null)
        return
      }

      setImportProgress(next)
    }

    EventsOn('backup:import:progress', onImportProgress)
    return () => {
      EventsOff('backup:import:progress')
    }
  }, [setImportProgress])

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

  useEffect(() => {
    const isImporting = actionLoading === 'import-reset' || actionLoading === 'import-merge'
    if (isImporting) {
      setImportState({
        inProgress: true,
        progress: importProgress?.progress ?? 0,
        message: importProgress?.message || '正在导入备份...',
      })
      return
    }
    clearImportState()
  }, [actionLoading, clearImportState, importProgress?.message, importProgress?.progress, setImportState])

  useEffect(() => {
    return () => {
      clearImportState()
    }
  }, [clearImportState])

  useEffect(() => {
    if (!exportLogsRef.current) {
      return
    }
    exportLogsRef.current.scrollTop = exportLogsRef.current.scrollHeight
  }, [exportLogs, exportLogsRef])
}
