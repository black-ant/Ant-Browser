import { useEffect, useRef, useState } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { Clock3, Settings2, Upload } from 'lucide-react'

import { Button, Progress, toast } from '../../shared/components'
import { BackupImportModal } from './components/BackupImportModal'
import { ScheduledBackupModal } from './components/ScheduledBackupModal'
import { BackupChannelConfigModal } from './components/BackupChannelConfigModal'
import { OpenListConfigModal } from './channels/openlist/OpenListConfigModal'
import type { BackupExportLogItem, BackupExportProgress } from './progress'
import {
  createBackupPackage,
  fetchLocalBackupSettings,
  importSystemConfig,
} from './api'
import { BackupTypeModal } from './components/BackupTypeModal'
import type { BackupTypeSelection } from './components/BackupTypeModal'
import { BackupScopeModal } from './components/BackupScopeModal'
import { BackupHistoryTable } from './components/BackupHistoryTable'
import { ProfilePackageConflictModal } from './components/ProfilePackageConflictModal'
import { fetchOpenListSettings } from './channels/openlist/api'
import type { OpenListConnection } from './channels/openlist/api'
import { fetchS3Settings } from './channels/s3/api'
import type { S3Connection } from './channels/s3/api'
import { importBrowserProfilePackageWithOptions } from '../browser/api/profiles'
import type { BrowserProfilePackageImportPreview } from '../browser/types'
import { createBackupRouteState } from './flow'
import type { BackupRouteState } from './flow'
import { useBackupProgressEffects } from './hooks/useBackupProgressEffects'

type BackupActionLoading = 'none' | 'export' | 'import-merge'

function formatBackupBytes(value?: number) {
  const bytes = Math.max(0, Number(value) || 0)
  if (bytes < 1024) return `${Math.round(bytes)} B`
  const units = ['KB', 'MB', 'GB', 'TB']
  let amount = bytes
  let unitIndex = -1
  while (amount >= 1024 && unitIndex < units.length - 1) {
    amount /= 1024
    unitIndex += 1
  }
  return `${amount.toFixed(2)} ${units[unitIndex]}`
}

function formatBackupRate(value?: number) {
  const rate = Math.max(0, Number(value) || 0)
  if (rate <= 0) return '计算中'
  return `${formatBackupBytes(rate)}/s`
}

export function BackupPage() {
  const location = useLocation()
  const navigate = useNavigate()
  const [importModalOpen, setImportModalOpen] = useState(false)
  const [backupScopeModalOpen, setBackupScopeModalOpen] = useState(false)
  const [backupTypeModalOpen, setBackupTypeModalOpen] = useState(false)
  const [channelConfigModalOpen, setChannelConfigModalOpen] = useState(false)
  const [openListConfigModalOpen, setOpenListConfigModalOpen] = useState(false)
  const [openListConnection, setOpenListConnection] = useState<OpenListConnection | null>(null)
  const [s3Connection, setS3Connection] = useState<S3Connection | null>(null)
  const [localBackupDirectory, setLocalBackupDirectory] = useState('')
  const [pendingBackupTypes, setPendingBackupTypes] = useState<BackupTypeSelection | null>(null)
  const [pendingProfileIds, setPendingProfileIds] = useState<string[]>([])
  const openListConfigurationCompletedRef = useRef(false)
  const openListConfiguredConnectionRef = useRef<OpenListConnection | null>(null)
  const [historyRefreshToken, setHistoryRefreshToken] = useState(0)
  const [historyBusy, setHistoryBusy] = useState(false)
  const [openListBusy, setOpenListBusy] = useState(false)
  const [scheduledBackupModalOpen, setScheduledBackupModalOpen] = useState(false)
  const [scheduledBackupRefreshToken, setScheduledBackupRefreshToken] = useState(0)
  const [scheduledBackupBusy, setScheduledBackupBusy] = useState(false)
  const [actionLoading, setActionLoading] = useState<BackupActionLoading>('none')
  const [profileImportPreview, setProfileImportPreview] = useState<BrowserProfilePackageImportPreview | null>(null)
  const [profileImportBusy, setProfileImportBusy] = useState(false)
  const [exportProgress, setExportProgress] = useState<BackupExportProgress | null>(null)
  const [importProgress, setImportProgress] = useState<BackupExportProgress | null>(null)
  const [exportLogs, setExportLogs] = useState<BackupExportLogItem[]>([])
  const exportLogsRef = useRef<HTMLDivElement | null>(null)
  const [connectionsLoading, setConnectionsLoading] = useState(true)
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
    setConnectionsLoading(true)
    void Promise.allSettled([fetchOpenListSettings(), fetchS3Settings(), fetchLocalBackupSettings()]).then(([openListResult, s3Result, localResult]) => {
      if (!active) return
      if (openListResult.status === 'fulfilled') {
        const settings = openListResult.value
        if (settings.tokenConfigured && settings.baseURL) {
          setOpenListConnection({ baseURL: settings.baseURL, remotePath: settings.remotePath })
        }
      }
      if (s3Result.status === 'fulfilled') {
        const settings = s3Result.value
        if (settings.credentialsConfigured && settings.bucket.trim()) {
          setS3Connection({
            endpoint: settings.endpoint,
            region: settings.region,
            bucket: settings.bucket,
            prefix: settings.prefix,
            forcePathStyle: settings.forcePathStyle,
          })
        }
      }
      if (localResult.status === 'fulfilled') {
        setLocalBackupDirectory(localResult.value.localDirectory)
      }
    }).finally(() => {
      if (active) setConnectionsLoading(false)
    })
    return () => {
      active = false
    }
  }, [])

  useEffect(() => {
    const routeState = location.state as BackupRouteState | null
    const resume = routeState?.backupResume
    if (!resume?.pendingBackupTypes) return

    setPendingBackupTypes(resume.pendingBackupTypes)
    setPendingProfileIds(Array.isArray(resume.pendingProfileIds) ? resume.pendingProfileIds : [])
    if (resume.openListConnection !== undefined) {
      setOpenListConnection(resume.openListConnection)
    }
    if (resume.s3Connection !== undefined) {
      setS3Connection(resume.s3Connection)
    }
    navigate(location.pathname, { replace: true, state: null })
  }, [location.key, location.pathname, location.state, navigate])

  const handleBackup = async (destinations: BackupTypeSelection, profileIds: string[]) => {
    setActionLoading('export')
    setExportLogs([])
    setExportProgress({ phase: 'starting', progress: 0, message: '准备备份...' })
    try {
      const res = await createBackupPackage(destinations, profileIds)
      if (res.cancelled) {
        setExportProgress(null)
        setExportLogs([])
        toast.info('已取消备份')
        return
      }
      const localSaved = res.localSaved === true || (destinations.local === true && Boolean(res.zipPath))
      const remoteUploaded = res.remoteUploaded === true || ((destinations.openlist === true || destinations.s3 === true) && Boolean(res.remoteName))
      const fileHint = Number.isFinite(res.fileCount) && (res.fileCount || 0) > 0
        ? `，共 ${res.fileCount} 个文件`
        : ''
      const profileHint = res.packageType === 'profile' && Number.isFinite(res.profileCount) && (res.profileCount || 0) > 0
        ? `，共 ${res.profileCount} 个实例`
        : ''
      const resultMessage = `${res.message || '备份完成'}${profileHint}${fileHint}`
      const partial = Boolean(res.partial || res.remoteError)
      setExportProgress({
        phase: partial ? 'error' : 'done',
        progress: 100,
        message: resultMessage,
      })
      if (localSaved && res.zipPath) {
        setHistoryRefreshToken(previous => previous + 1)
      }
      if (remoteUploaded && !localSaved) {
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

  const handleBackupScopeConfirm = (profileIds: string[]) => {
    setPendingProfileIds(profileIds)
    setBackupScopeModalOpen(false)
    setBackupTypeModalOpen(true)
  }

  useEffect(() => {
    if (connectionsLoading || channelConfigModalOpen || openListConfigModalOpen || backupTypeModalOpen || backupScopeModalOpen) return
    const destinations = pendingBackupTypes
    if (!destinations) return
    const configuredOpenListConnection = openListConnection || openListConfiguredConnectionRef.current
    if (destinations.local === true && !localBackupDirectory.trim()) {
      setChannelConfigModalOpen(true)
      toast.info('请先配置本地备份目录，再开始备份')
      return
    }
    if (destinations.openlist === true && !configuredOpenListConnection) {
      openListConfigurationCompletedRef.current = false
      openListConfiguredConnectionRef.current = null
      setOpenListConfigModalOpen(true)
      toast.info('请先配置 OpenList，再开始备份')
      return
    }

    if (destinations.s3 === true && !s3Connection) {
      setBackupTypeModalOpen(false)
      navigate('/system/backup/s3', {
        state: createBackupRouteState(destinations, pendingProfileIds, configuredOpenListConnection, s3Connection),
      })
      toast.info('请先配置 S3，再开始备份')
      return
    }

    const profileIds = pendingProfileIds
    setPendingBackupTypes(null)
    setPendingProfileIds([])
    void handleBackup(destinations, profileIds)
  }, [
    backupScopeModalOpen,
    channelConfigModalOpen,
    backupTypeModalOpen,
    connectionsLoading,
    navigate,
    openListConfigModalOpen,
    openListConnection,
    localBackupDirectory,
    pendingBackupTypes,
    pendingProfileIds,
    s3Connection,
  ])

  const handleBackupTypeConfirm = (destinations: BackupTypeSelection) => {
    if (destinations.local === true && !localBackupDirectory.trim()) {
      setPendingBackupTypes(destinations)
      setBackupTypeModalOpen(false)
      setChannelConfigModalOpen(true)
      toast.info('请先配置本地备份目录，再开始备份')
      return
    }
    if (destinations.openlist === true && !openListConnection) {
      openListConfigurationCompletedRef.current = false
      openListConfiguredConnectionRef.current = null
      setPendingBackupTypes(destinations)
      setBackupTypeModalOpen(false)
      setOpenListConfigModalOpen(true)
      toast.info('请先配置 OpenList，再开始备份')
      return
    }
    if (destinations.s3 === true && !s3Connection) {
      setPendingBackupTypes(destinations)
      setBackupTypeModalOpen(false)
      navigate('/system/backup/s3', {
        state: createBackupRouteState(destinations, pendingProfileIds, openListConnection, s3Connection),
      })
      toast.info('请先配置 S3，再开始备份')
      return
    }
    const profileIds = pendingProfileIds
    setPendingBackupTypes(null)
    setPendingProfileIds([])
    setBackupTypeModalOpen(false)
    void handleBackup(destinations, profileIds)
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
      if (res.requiresProfileImportConfirmation && res.profileImportPreview) {
        setImportModalOpen(false)
        setImportProgress(null)
        setProfileImportPreview(res.profileImportPreview)
        return
      }
      const imported = res.imported ?? 0
      const skipped = res.skipped ?? 0
      const conflicts = res.conflicts ?? 0
      const componentFailed = Number.isFinite(res.componentFailed) ? Math.max(0, Math.round(res.componentFailed || 0)) : 0
      const componentTotal = Number.isFinite(res.componentTotal) ? Math.max(0, Math.round(res.componentTotal || 0)) : 0
      const failedComponents = Array.isArray(res.failedComponents) ? res.failedComponents : []
      const warnings = Array.isArray(res.warnings) ? res.warnings.filter(Boolean) : []

      if (res.packageType === 'profile') {
        const importedProfiles = res.importedCount ?? res.profileCount ?? imported
        const overwrittenCount = res.overwrittenCount ?? 0
        const createdCount = res.createdCount ?? Math.max(0, importedProfiles - overwrittenCount)
        const summary = overwrittenCount > 0
          ? `实例备份导入完成：覆盖 ${overwrittenCount} 个实例，旧实例已移入回收站；新建 ${createdCount} 个实例`
          : `实例备份导入完成：新建 ${createdCount} 个实例`
        if (warnings.length > 0) {
          toast.warning(`${summary}；${warnings[0]}`)
        } else {
          toast.success(summary)
        }
      } else if (res.partial || componentFailed > 0) {
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

  const executeProfileImport = async (preview: BrowserProfilePackageImportPreview, conflictMode: 'new' | 'overwrite') => {
    setProfileImportPreview(null)
    setProfileImportBusy(true)
    setActionLoading('import-merge')
    setImportProgress({ phase: 'importing', progress: 40, message: '正在导入实例备份...' })
    try {
      const result = await importBrowserProfilePackageWithOptions(preview.zipPath, conflictMode, true)
      if (result.cancelled) {
        setImportProgress(null)
        return
      }
      const overwrittenCount = result.overwrittenCount ?? 0
      const createdCount = result.createdCount ?? Math.max(0, result.importedCount - overwrittenCount)
      const summary = overwrittenCount > 0
        ? `实例备份导入完成：覆盖 ${overwrittenCount} 个实例，旧实例已移入回收站；新建 ${createdCount} 个实例`
        : `实例备份导入完成：新建 ${createdCount} 个实例`
      const warnings = result.warnings || []
      if (warnings.length > 0) {
        toast.warning(`${summary}；${warnings[0]}`)
      } else {
        toast.success(summary)
      }
      setImportProgress(null)
      setHistoryRefreshToken(previous => previous + 1)
    } catch (error: any) {
      toast.error(error?.message || '导入实例失败')
      setImportProgress(null)
    } finally {
      setProfileImportBusy(false)
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
          {exportProgress.phase === 'uploading' && exportProgress.totalBytes && exportProgress.totalBytes > 0 && (
            <div className="flex items-center justify-between gap-3 text-xs text-[var(--color-text-muted)]">
              <span>
                已上传 {formatBackupBytes(exportProgress.bytesTransferred)} / {formatBackupBytes(exportProgress.totalBytes)}
              </span>
              <span>速度 {formatBackupRate(exportProgress.bytesPerSecond)}</span>
            </div>
          )}
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
        configuredS3Connection={s3Connection}
        refreshToken={historyRefreshToken}
        onBusyChange={setHistoryBusy}
        onLocalDirectoryChange={setLocalBackupDirectory}
        actions={(
          <div className="flex max-w-full flex-wrap items-center justify-end gap-2" aria-label="备份操作">
            <Button
              size="sm"
              onClick={() => {
                setPendingProfileIds([])
                setBackupScopeModalOpen(true)
              }}
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
              className="!border-[var(--color-border-default)] !bg-[var(--color-bg-muted)] !text-[var(--color-text-primary)] hover:!border-[var(--color-border-strong)] hover:!bg-[var(--color-border-default)]"
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
              className="!border-[var(--color-border-default)] !bg-[var(--color-bg-muted)] !text-[var(--color-text-primary)] hover:!border-[var(--color-border-strong)] hover:!bg-[var(--color-border-default)]"
              onClick={() => {
                openListConfigurationCompletedRef.current = false
                setPendingBackupTypes(null)
                setChannelConfigModalOpen(true)
              }}
              disabled={remoteBackupBusy || scheduledBackupBusy || actionLoading !== 'none'}
              title="配置备份渠道"
            >
              <Settings2 className="h-4 w-4" />
              渠道
            </Button>
            <Button
              variant="secondary"
              size="sm"
              className="!border-[var(--color-border-default)] !bg-[var(--color-bg-muted)] !text-[var(--color-text-primary)] hover:!border-[var(--color-border-strong)] hover:!bg-[var(--color-border-default)]"
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

      <ProfilePackageConflictModal
        preview={profileImportPreview}
        busy={profileImportBusy}
        onClose={() => setProfileImportPreview(null)}
        onConfirm={(mode) => {
          if (profileImportPreview) {
            void executeProfileImport(profileImportPreview, mode)
          }
        }}
      />

      <BackupChannelConfigModal
        open={channelConfigModalOpen}
        localDirectory={localBackupDirectory}
        onLocalConfigured={directory => {
          setLocalBackupDirectory(directory)
          setHistoryRefreshToken(previous => previous + 1)
          setChannelConfigModalOpen(false)
        }}
        onClose={() => {
          setChannelConfigModalOpen(false)
          if (!pendingBackupTypes?.local || !localBackupDirectory.trim()) {
            setPendingBackupTypes(null)
            setPendingProfileIds([])
          }
        }}
        onSelect={channelId => {
          setChannelConfigModalOpen(false)
          if (channelId === 's3') {
            navigate('/system/backup/s3')
            return
          }
          if (channelId === 'openlist') {
            openListConfigurationCompletedRef.current = false
            openListConfiguredConnectionRef.current = null
            setOpenListConfigModalOpen(true)
          }
        }}
      />

      <BackupTypeModal
        open={backupTypeModalOpen}
        onClose={() => {
          setBackupTypeModalOpen(false)
          setPendingBackupTypes(null)
          setPendingProfileIds([])
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
          s3: {
            configured: Boolean(s3Connection),
            summary: s3Connection
              ? `${s3Connection.bucket}${s3Connection.prefix ? `/${s3Connection.prefix}` : ''}`
              : undefined,
          },
          local: {
            configured: Boolean(localBackupDirectory.trim()),
            summary: localBackupDirectory || undefined,
          },
        }}
      />

      <BackupScopeModal
        open={backupScopeModalOpen}
        initialProfileIds={pendingProfileIds}
        onClose={() => {
          setBackupScopeModalOpen(false)
          setPendingProfileIds([])
        }}
        onConfirm={handleBackupScopeConfirm}
      />

      <ScheduledBackupModal
        open={scheduledBackupModalOpen}
        onClose={() => setScheduledBackupModalOpen(false)}
        refreshToken={scheduledBackupRefreshToken}
        onRequestOpenListConfig={() => {
          openListConfigurationCompletedRef.current = false
          openListConfiguredConnectionRef.current = null
          setOpenListConfigModalOpen(true)
        }}
        onBusyChange={setScheduledBackupBusy}
      />

      <OpenListConfigModal
        open={openListConfigModalOpen}
        onClose={() => {
          const configured = openListConfigurationCompletedRef.current
          setOpenListConfigModalOpen(false)
          if (!configured) {
            setPendingBackupTypes(null)
            setPendingProfileIds([])
            openListConfiguredConnectionRef.current = null
          }
          openListConfigurationCompletedRef.current = false
        }}
        onConfigured={(connection) => {
          openListConfigurationCompletedRef.current = true
          openListConfiguredConnectionRef.current = connection
          setOpenListConnection(connection)
          setScheduledBackupRefreshToken(previous => previous + 1)
          setHistoryRefreshToken(previous => previous + 1)
        }}
        onBusyChange={setOpenListBusy}
      />
    </div>
  )
}
