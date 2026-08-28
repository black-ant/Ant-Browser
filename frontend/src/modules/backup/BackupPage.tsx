import { useEffect, useRef, useState } from 'react'
import { Clock3, Settings2, Upload } from 'lucide-react'

import { Button, Progress, toast } from '../../shared/components'
import { BackupImportModal } from './components/BackupImportModal'
import { ScheduledBackupModal } from './components/ScheduledBackupModal'
import { OpenListConfigModal } from './channels/openlist/OpenListConfigModal'
import type { BackupExportLogItem, BackupExportProgress } from './progress'
import {
  createBackupPackage,
  importSystemConfig,
} from './api'
import { BackupTypeModal } from './components/BackupTypeModal'
import type { BackupTypeSelection } from './components/BackupTypeModal'
import { BackupHistoryTable, recordLocalBackupHistory } from './components/BackupHistoryTable'
import { fetchOpenListSettings } from './channels/openlist/api'
import type { OpenListConnection } from './channels/openlist/api'
import { useBackupProgressEffects } from './hooks/useBackupProgressEffects'

type BackupActionLoading = 'none' | 'export' | 'import-merge'

export function BackupPage() {
  const [importModalOpen, setImportModalOpen] = useState(false)
  const [backupTypeModalOpen, setBackupTypeModalOpen] = useState(false)
  const [openListConfigModalOpen, setOpenListConfigModalOpen] = useState(false)
  const [openListConnection, setOpenListConnection] = useState<OpenListConnection | null>(null)
  const [pendingBackupTypes, setPendingBackupTypes] = useState<BackupTypeSelection | null>(null)
  const openListConfigurationCompletedRef = useRef(false)
  const [historyRefreshToken, setHistoryRefreshToken] = useState(0)
  const [historyBusy, setHistoryBusy] = useState(false)
  const [openListBusy, setOpenListBusy] = useState(false)
  const [scheduledBackupModalOpen, setScheduledBackupModalOpen] = useState(false)
  const [scheduledBackupBusy, setScheduledBackupBusy] = useState(false)
  const [actionLoading, setActionLoading] = useState<BackupActionLoading>('none')
  const [exportProgress, setExportProgress] = useState<BackupExportProgress | null>(null)
  const [importProgress, setImportProgress] = useState<BackupExportProgress | null>(null)
  const [exportLogs, setExportLogs] = useState<BackupExportLogItem[]>([])
  const exportLogsRef = useRef<HTMLDivElement | null>(null)
  const remoteBackupBusy = historyBusy || openListBusy

  useBackupProgressEffects({
    actionLoading,
    exportLogs,
    exportLogsRef,
    importProgress,
    setExportLogs,
    setExportProgress,
    setImportProgress,
  })

  useEffect(() => {
    let active = true
    void fetchOpenListSettings().then(settings => {
      if (active && settings.tokenConfigured && settings.baseURL) {
        setOpenListConnection({ baseURL: settings.baseURL, remotePath: settings.remotePath })
      }
    }).catch(() => {})
    return () => {
      active = false
    }
  }, [])

  useEffect(() => {
    if (openListConfigModalOpen || !openListConnection || !pendingBackupTypes) return
    setBackupTypeModalOpen(true)
  }, [openListConfigModalOpen, openListConnection, pendingBackupTypes])

  const handleBackup = async (destinations: BackupTypeSelection) => {
    setActionLoading('export')
    setExportLogs([])
    setExportProgress({ phase: 'starting', progress: 0, message: '准备备份...' })
    try {
      const res = await createBackupPackage(destinations)
      if (res.cancelled) {
        setExportProgress(null)
        setExportLogs([])
        toast.info('已取消备份')
        return
      }
      const localSaved = res.localSaved === true || (destinations.local === true && Boolean(res.zipPath))
      const remoteUploaded = res.remoteUploaded === true || (destinations.openlist === true && Boolean(res.remoteName))
      const fileHint = Number.isFinite(res.fileCount) && (res.fileCount || 0) > 0
        ? `，共 ${res.fileCount} 个文件`
        : ''
      const resultMessage = `${res.message || '备份完成'}${fileHint}`
      const partial = Boolean(res.partial || res.remoteError)
      setExportProgress({
        phase: partial ? 'error' : 'done',
        progress: 100,
        message: resultMessage,
      })
      if (localSaved && res.zipPath) {
        await recordLocalBackupHistory(res.zipPath)
      }
      if (localSaved || remoteUploaded) {
        setHistoryRefreshToken(previous => previous + 1)
      }
      if (partial) {
        toast.warning(resultMessage)
      } else {
        toast.success(resultMessage)
      }
    } catch (error: any) {
      setExportProgress(prev => ({
        phase: 'error',
        progress: prev?.progress ?? 0,
        message: error?.message || '备份失败',
      }))
      setExportLogs(prev => {
        const timestamp = new Date().toLocaleTimeString('zh-CN', { hour12: false })
        const text = error?.message || '备份失败'
        const next = [
          ...prev,
          { id: Date.now() + Math.floor(Math.random() * 1000), phase: 'error', time: timestamp, text },
        ]
        return next.length > 120 ? next.slice(next.length - 120) : next
      })
      toast.error(error?.message || '备份失败')
    } finally {
      setActionLoading('none')
    }
  }

  const handleBackupTypeConfirm = (destinations: BackupTypeSelection) => {
    if (destinations.openlist === true && !openListConnection) {
      openListConfigurationCompletedRef.current = false
      setPendingBackupTypes(destinations)
      setBackupTypeModalOpen(false)
      setOpenListConfigModalOpen(true)
      toast.info('请先配置 OpenList，再开始备份')
      return
    }
    setPendingBackupTypes(null)
    setBackupTypeModalOpen(false)
    void handleBackup(destinations)
  }

  const handleImportSystem = async () => {
    setActionLoading('import-merge')
    setImportProgress({
      phase: 'starting',
      progress: 0,
      message: '等待选择 ZIP 备份（合并导入）...',
    })
    try {
      const res = await importSystemConfig()
      if (res.cancelled) {
        setImportProgress(null)
        toast.info('已取消导入')
        return
      }
      const imported = res.imported ?? 0
      const skipped = res.skipped ?? 0
      const conflicts = res.conflicts ?? 0
      const componentFailed = Number.isFinite(res.componentFailed) ? Math.max(0, Math.round(res.componentFailed || 0)) : 0
      const componentTotal = Number.isFinite(res.componentTotal) ? Math.max(0, Math.round(res.componentTotal || 0)) : 0
      const failedComponents = Array.isArray(res.failedComponents) ? res.failedComponents : []

      if (res.partial || componentFailed > 0) {
        const moduleNames = failedComponents
          .map(item => (item?.componentName || item?.componentId || '').trim())
          .filter(Boolean)
        const moduleHint = moduleNames.length > 0
          ? `：${moduleNames.slice(0, 3).join('、')}${moduleNames.length > 3 ? ` 等 ${moduleNames.length} 个模块` : ''}`
          : ''
        if (componentTotal > 0) {
          const componentSuccess = Math.max(0, componentTotal - componentFailed)
          toast.warning(`导入完成（部分成功）：模块成功 ${componentSuccess}/${componentTotal}，异常 ${componentFailed}${moduleHint}`)
        } else {
          toast.warning(`导入完成（部分成功）：异常模块 ${componentFailed}${moduleHint}`)
        }
      } else {
        toast.success(`导入完成：导入 ${imported}，跳过 ${skipped}，冲突 ${conflicts}`)
      }
      if (res.zipPath) {
        await recordLocalBackupHistory(res.zipPath)
        setHistoryRefreshToken(previous => previous + 1)
      }
      setImportModalOpen(false)
      setImportProgress(null)
    } catch (error: any) {
      setImportProgress(prev => ({
        phase: 'error',
        progress: prev?.progress ?? 0,
        message: error?.message || '导入失败',
      }))
      toast.error(error?.message || '导入失败')
    } finally {
      setActionLoading('none')
    }
  }

  return (
    <div className="w-full space-y-5 animate-fade-in">
      {exportProgress && (
        <div className="space-y-2 rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-4 py-3" role="status" aria-live="polite">
          <div className="flex items-center justify-between gap-3 text-xs">
            <span className="min-w-0 break-words text-[var(--color-text-secondary)]">{exportProgress.message}</span>
            <span className={exportProgress.phase === 'error' ? 'text-[var(--color-error)]' : exportProgress.phase === 'done' ? 'text-[var(--color-success)]' : 'text-[var(--color-text-muted)]'}>
              {exportProgress.phase === 'error' ? '失败' : exportProgress.phase === 'done' ? '完成' : `${exportProgress.progress}%`}
            </span>
          </div>
          <Progress
            percent={exportProgress.progress}
            size="sm"
            status={exportProgress.phase === 'error' ? 'error' : exportProgress.phase === 'done' ? 'success' : 'normal'}
            showInfo={false}
          />
          {exportLogs.length > 0 && (
            <div ref={exportLogsRef} className="max-h-28 overflow-y-auto border-t border-[var(--color-border-muted)] pt-2 font-mono text-xs leading-5 text-[var(--color-text-muted)]">
              {exportLogs.map(item => (
                <div key={item.id}>
                  <span className="mr-2">{item.time}</span>
                  <span className={item.phase === 'error' ? 'text-[var(--color-error)]' : item.phase === 'done' ? 'text-[var(--color-success)]' : 'text-[var(--color-text-secondary)]'}>
                    {item.text}
                  </span>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      <BackupHistoryTable
        configuredConnection={openListConnection}
        refreshToken={historyRefreshToken}
        onBusyChange={setHistoryBusy}
        actions={(
          <div className="flex max-w-full flex-wrap items-center justify-end gap-2" aria-label="备份操作">
            <Button
              size="sm"
              onClick={() => setBackupTypeModalOpen(true)}
              loading={actionLoading === 'export'}
              disabled={remoteBackupBusy || scheduledBackupBusy || actionLoading !== 'none'}
              title="选择备份类型并开始备份"
            >
              <Upload className="h-4 w-4" />
              备份
            </Button>
            <Button
              variant="secondary"
              size="sm"
              onClick={() => {
                setImportProgress(null)
                setImportModalOpen(true)
              }}
              disabled={remoteBackupBusy || scheduledBackupBusy || actionLoading !== 'none'}
              title="导入本地 ZIP 备份"
            >
              <Upload className="h-4 w-4" />
              导入
            </Button>
            <Button
              variant="secondary"
              size="sm"
              onClick={() => {
                openListConfigurationCompletedRef.current = false
                setPendingBackupTypes(null)
                setOpenListConfigModalOpen(true)
              }}
              disabled={remoteBackupBusy || scheduledBackupBusy || actionLoading !== 'none'}
              title="配置 OpenList 连接"
            >
              <Settings2 className="h-4 w-4" />
              配置
            </Button>
            <Button
              variant="secondary"
              size="sm"
              onClick={() => setScheduledBackupModalOpen(true)}
              disabled={remoteBackupBusy || actionLoading !== 'none'}
              title="设置 OpenList 定时备份"
            >
              <Clock3 className="h-4 w-4" />
              定时
            </Button>
          </div>
        )}
      />

      <BackupImportModal
        open={importModalOpen}
        actionLoading={actionLoading}
        importProgress={importProgress}
        onClose={() => {
          setImportModalOpen(false)
          setImportProgress(null)
        }}
        onImport={() => { void handleImportSystem() }}
      />

      <BackupTypeModal
        open={backupTypeModalOpen}
        onClose={() => {
          setBackupTypeModalOpen(false)
          setPendingBackupTypes(null)
        }}
        onConfirm={handleBackupTypeConfirm}
        initialSelection={pendingBackupTypes || undefined}
        channelStatus={{
          openlist: {
            configured: Boolean(openListConnection),
            summary: openListConnection
              ? `${openListConnection.baseURL}${openListConnection.remotePath ? `/${openListConnection.remotePath}` : ''}`
              : undefined,
          },
        }}
      />

      <OpenListConfigModal
        open={openListConfigModalOpen}
        onClose={() => {
          setOpenListConfigModalOpen(false)
          if (!openListConfigurationCompletedRef.current) {
            setPendingBackupTypes(null)
          }
          openListConfigurationCompletedRef.current = false
        }}
        onConfigured={(connection) => {
          openListConfigurationCompletedRef.current = true
          setOpenListConnection(connection)
          setHistoryRefreshToken(previous => previous + 1)
        }}
        onBusyChange={setOpenListBusy}
      />

      <ScheduledBackupModal
        open={scheduledBackupModalOpen}
        onClose={() => setScheduledBackupModalOpen(false)}
        onSaved={() => {}}
        onRequestOpenListConfig={() => {
          setScheduledBackupModalOpen(false)
          setOpenListConfigModalOpen(true)
        }}
        onBusyChange={setScheduledBackupBusy}
      />
    </div>
  )
}
