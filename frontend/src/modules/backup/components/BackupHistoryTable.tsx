import { useEffect, useRef, useState, type ReactNode } from 'react'
import { Cloud, Database, Download, ExternalLink, RefreshCw } from 'lucide-react'

import { Button, Card, ConfirmModal, Table, toast } from '../../../shared/components'
import type { TableColumn } from '../../../shared/components'
import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime'
import {
  fetchLocalBackupSettings,
  listLocalBackups,
  openBackupPath,
  restoreLocalSystemConfig,
  saveLocalBackupDirectory,
  type BackupLocalHistoryItem,
} from '../api'
import {
  downloadOpenListBackup,
  listOpenListBackups,
} from '../channels/openlist/api'
import type { OpenListBackupFile, OpenListConnection } from '../channels/openlist/api'
import {
  downloadS3Backup,
  listS3Backups,
} from '../channels/s3/api'
import type { S3BackupFile, S3Connection } from '../channels/s3/api'
import { normalizeBackupPackageInfo, type BackupPackageInfo } from '../packageInfo'
import { ProfilePackageConflictModal } from './ProfilePackageConflictModal'
import { importBrowserProfilePackageWithOptions } from '../../browser/api/profiles'
import type { BrowserProfilePackageImportAction, BrowserProfilePackageImportPreview } from '../../browser/types'

type BackupHistorySource = 'local' | 'openlist' | 's3'
type BackupHistoryFilter = BackupHistorySource
type RemoteBackupSource = Exclude<BackupHistorySource, 'local'>
type RemoteBackupFile = OpenListBackupFile | S3BackupFile

interface BackupHistoryItem extends BackupPackageInfo {
  id: string
  name: string
  source: BackupHistorySource
  size: number
  modifiedAt: string
  location: string
  localPath?: string
  remoteFile?: RemoteBackupFile
  metadataAvailable?: boolean
  metadataError?: string
  metadataOrphan?: boolean
  appName?: string
  appVersion?: string
}

interface BackupHistoryTableProps {
  actions?: ReactNode
  configuredConnection?: OpenListConnection | null
  configuredS3Connection?: S3Connection | null
  refreshToken?: number
  onBusyChange?: (busy: boolean) => void
  onLocalDirectoryChange?: (directory: string) => void
}

function buildOpenListLocation(connection: OpenListConnection) {
  const baseURL = connection.baseURL.trim().replace(/\/$/, '')
  const remotePath = connection.remotePath.trim().replace(/^\/+/, '')
  return remotePath ? `${baseURL}/${remotePath}` : baseURL
}

function buildS3Location(connection: S3Connection) {
  const endpoint = connection.endpoint.trim().replace(/\/$/, '')
  const bucket = connection.bucket.trim()
  const prefix = connection.prefix.trim().replace(/^\/+|\/+$/g, '')
  const root = endpoint ? `${endpoint}/${bucket}` : `s3://${bucket}`
  return prefix ? `${root}/${prefix}` : root
}

function localItemsFromEntries(entries: BackupLocalHistoryItem[]): BackupHistoryItem[] {
  return sortHistoryItems(entries.map(item => ({
    id: `local:${item.path || item.name}`,
    name: item.name || item.path,
    source: 'local' as const,
    size: item.size,
    modifiedAt: item.modifiedAt,
    location: item.path || '本地文件',
    localPath: item.path,
    metadataAvailable: item.metadataAvailable,
    metadataError: item.metadataError,
    metadataOrphan: item.metadataOrphan,
    appName: item.appName,
    appVersion: item.appVersion,
    ...(item.metadataAvailable
      ? {
        packageType: item.packageType,
        profileCount: item.profileCount,
        profileNames: item.profileNames,
      }
      : {}),
  })))
}

function historyItemIdentity(item: BackupHistoryItem) {
  const rawName = item.source === 'local'
    ? (item.name || item.localPath || item.location)
    : item.name
  const normalizedName = rawName.trim().replace(/\.json$/i, '.zip').toLocaleLowerCase()
  return `${item.source}:${normalizedName}`
}

function historyItemPriority(item: BackupHistoryItem) {
  let priority = 0
  if (item.source === 'local' && /\.zip$/i.test(item.name.trim())) priority += 4
  if (item.metadataAvailable) priority += 2
  if (!item.metadataOrphan) priority += 1
  return priority
}

function sortHistoryItems(items: BackupHistoryItem[]) {
  const unique = new Map<string, BackupHistoryItem>()
  for (const item of items) {
    const key = historyItemIdentity(item)
    const current = unique.get(key)
    if (!current || historyItemPriority(item) > historyItemPriority(current)) {
      unique.set(key, item)
    }
  }
  return [...unique.values()].sort((left, right) => {
    const leftTime = Date.parse(left.modifiedAt)
    const rightTime = Date.parse(right.modifiedAt)
    if (Number.isFinite(leftTime) && Number.isFinite(rightTime)) return rightTime - leftTime
    if (Number.isFinite(rightTime)) return 1
    if (Number.isFinite(leftTime)) return -1
    return left.name.localeCompare(right.name)
  })
}

function connectionKey(connection?: OpenListConnection | S3Connection | null) {
  if (!connection) return ''
  return JSON.stringify(connection)
}

function formatSize(size: number) {
  if (size <= 0) return '—'
  if (size < 1024) return `${size} B`
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`
  if (size < 1024 * 1024 * 1024) return `${(size / 1024 / 1024).toFixed(1)} MB`
  return `${(size / 1024 / 1024 / 1024).toFixed(2)} GB`
}

function formatModifiedAt(value: string) {
  if (!value) return '—'
  const timestamp = Date.parse(value)
  if (!Number.isFinite(timestamp)) return value
  return new Date(timestamp).toLocaleString('zh-CN', { hour12: false })
}

function errorMessage(error: unknown, fallback: string) {
  if (error && typeof error === 'object' && 'message' in error && typeof error.message === 'string') {
    return error.message
  }
  return fallback
}

const LEGACY_LOCAL_HISTORY_KEY = 'ant_chrome_local_backup_history'

function legacyLocalBackupDirectories() {
  if (typeof window === 'undefined') return []
  try {
    const raw = window.localStorage.getItem(LEGACY_LOCAL_HISTORY_KEY)
    const parsed = raw ? JSON.parse(raw) : null
    if (!Array.isArray(parsed)) return []

    const directories = new Map<string, number>()
    for (const entry of parsed) {
      if (!entry || typeof entry !== 'object') continue
      const record = entry as Record<string, unknown>
      const rawPath = record.path
      const path = typeof rawPath === 'string'
        ? rawPath.trim()
        : ''
      if (!path || !/\.zip$/i.test(path)) continue
      const separatorIndex = Math.max(path.lastIndexOf('\\'), path.lastIndexOf('/'))
      if (separatorIndex < 0) continue
      const directory = path.slice(0, separatorIndex) || path.slice(0, separatorIndex + 1)
      if (!directory) continue
      directories.set(directory, (directories.get(directory) || 0) + 1)
    }

    return [...directories.entries()]
      .sort((left, right) => right[1] - left[1] || left[0].localeCompare(right[0]))
      .map(([directory]) => directory)
  } catch {
    return []
  }
}

function clearLegacyLocalBackupHistory() {
  if (typeof window === 'undefined') return
  try {
    window.localStorage.removeItem(LEGACY_LOCAL_HISTORY_KEY)
  } catch {
    return
  }
}

function sourceLabel(source: BackupHistorySource) {
  if (source === 'local') return '本地'
  if (source === 'openlist') return 'OpenList'
  return 'S3'
}

function backupPackageTypeLabel(item: BackupPackageInfo) {
  if (item.packageType === 'full') return '全量备份'
  if (item.packageType === 'profile') {
    const count = item.profileCount || item.profileNames?.length || 0
    if (count === 1) return '单实例备份'
    if (count > 1) return '多实例备份'
    return '实例备份'
  }
  return '备份类型未识别'
}

function renderBackupPackageType(item: BackupHistoryItem) {
  if (item.source === 'local' && item.metadataOrphan) {
    return (
      <span
        className="text-[var(--color-error)]"
        title={item.metadataError || '同名 JSON 存在，但 ZIP 文件不存在'}
      >
        备份文件缺失
      </span>
    )
  }
  if (item.source === 'local' && item.metadataAvailable !== true) {
    return (
      <span
        className="text-[var(--color-text-muted)]"
        title={item.metadataError || '同名 JSON 元数据不存在或不可用'}
      >
        信息不可用
      </span>
    )
  }
  return <span className="text-[var(--color-text-secondary)]">{backupPackageTypeLabel(item)}</span>
}

function renderBackupPackageContent(item: BackupHistoryItem) {
  if (item.source === 'local' && (item.metadataOrphan || item.metadataAvailable !== true)) {
    return <span className="text-[var(--color-text-muted)]">—</span>
  }
  if (item.packageType !== 'profile') {
    return <span className="text-[var(--color-text-muted)]">—</span>
  }

  const names = item.profileNames || []
  if (names.length === 0) {
    const count = item.profileCount || 0
    return (
      <span className="text-[var(--color-text-muted)]">
        {count > 0 ? `共 ${count} 个实例，名称未记录` : '名称未记录'}
      </span>
    )
  }
  return (
    <span className="block max-w-[240px] truncate text-[var(--color-text-primary)]" title={names.join('、')}>
      {names.join('、')}
    </span>
  )
}

export function BackupHistoryTable({
  actions,
  configuredConnection,
  configuredS3Connection,
  refreshToken = 0,
  onBusyChange,
  onLocalDirectoryChange,
}: BackupHistoryTableProps) {
  const [items, setItems] = useState<BackupHistoryItem[]>([])
  const [filter, setFilter] = useState<BackupHistoryFilter>('local')
  const [busy, setBusy] = useState<'none' | 'list' | 'restore-merge' | 'download'>('none')
  const [restoringItemId, setRestoringItemId] = useState('')
  const [downloadingItemId, setDownloadingItemId] = useState('')
  const [pendingRestoreItem, setPendingRestoreItem] = useState<BackupHistoryItem | null>(null)
  const [restoreConfirming, setRestoreConfirming] = useState(false)
  const [profileImportPreview, setProfileImportPreview] = useState<BrowserProfilePackageImportPreview | null>(null)
  const [profileImportItem, setProfileImportItem] = useState<BackupHistoryItem | null>(null)
  const [profileImportBusy, setProfileImportBusy] = useState(false)
  const [openingLocationId, setOpeningLocationId] = useState('')
  const [error, setError] = useState('')
  const [localDirectory, setLocalDirectory] = useState('')
  const [activeConnection, setActiveConnection] = useState<OpenListConnection | null>(configuredConnection || null)
  const [activeS3Connection, setActiveS3Connection] = useState<S3Connection | null>(configuredS3Connection || null)
  const configuredOpenListKeyRef = useRef<string | null>(null)
  const configuredS3KeyRef = useRef<string | null>(null)
  const remoteLoadVersionRef = useRef(0)
  const localLoadVersionRef = useRef(0)
  const localItemsRef = useRef<BackupHistoryItem[]>([])

  useEffect(() => {
    onBusyChange?.(busy === 'restore-merge' || busy === 'download' || pendingRestoreItem !== null || restoreConfirming || profileImportPreview !== null || profileImportBusy)
  }, [busy, onBusyChange, pendingRestoreItem, profileImportBusy, profileImportPreview, restoreConfirming])

  const mergeHistoryItems = (
    openListFiles: OpenListBackupFile[],
    openListConnection: OpenListConnection | null,
    s3Files: S3BackupFile[],
    s3Connection: S3Connection | null,
    replaceSources: RemoteBackupSource[] = ['openlist', 's3'],
  ) => {
    const localItems = localItemsRef.current
    setItems(previous => {
      const preservedItems = previous.filter(item => {
        if (item.source === 'local') return false
        return !replaceSources.includes(item.source)
      })
      const openListItems = replaceSources.includes('openlist') && openListConnection
        ? openListFiles.map(file => ({
          id: `openlist:${file.name}`,
          name: file.name,
          source: 'openlist' as const,
          size: file.size,
          modifiedAt: file.modifiedAt,
          location: buildOpenListLocation(openListConnection),
          remoteFile: file,
          ...normalizeBackupPackageInfo(file),
        }))
        : []
      const s3Items = replaceSources.includes('s3') && s3Connection
        ? s3Files.map(file => ({
          id: `s3:${file.name}`,
          name: file.name,
          source: 's3' as const,
          size: file.size,
          modifiedAt: file.modifiedAt,
          location: buildS3Location(s3Connection),
          remoteFile: file,
          ...normalizeBackupPackageInfo(file),
        }))
        : []
      return sortHistoryItems([...localItems, ...preservedItems, ...openListItems, ...s3Items])
    })
  }

  const loadLocalHistory = async (
    directory: string,
    showToast = false,
    manageBusy = false,
  ) => {
    const loadVersion = ++localLoadVersionRef.current
    if (manageBusy) setBusy('list')
    try {
      const entries = await listLocalBackups(directory)
      if (loadVersion !== localLoadVersionRef.current) return
      const nextLocalItems = localItemsFromEntries(entries)
      localItemsRef.current = nextLocalItems
      setItems(previous => sortHistoryItems([
        ...nextLocalItems,
        ...previous.filter(item => item.source !== 'local'),
      ]))
      if (showToast) {
        toast.success(`已刷新本地历史，共 ${nextLocalItems.length} 个备份`)
      }
    } catch (localError) {
      if (loadVersion !== localLoadVersionRef.current) return
      localItemsRef.current = []
      setItems(previous => previous.filter(item => item.source !== 'local'))
      const message = errorMessage(localError, '本地备份目录扫描失败')
      setError(message)
      if (showToast) toast.error(message)
    } finally {
      if (manageBusy && loadVersion === localLoadVersionRef.current) {
        setBusy('none')
      }
    }
  }

  const loadRemoteHistory = async (
    openListConnection: OpenListConnection | null,
    s3Connection: S3Connection | null,
    showToast: boolean,
    manageBusy = true,
    source: RemoteBackupSource | 'all' = 'all',
  ) => {
    const loadVersion = ++remoteLoadVersionRef.current
    if (manageBusy) setBusy('list')
    setError('')
    const openListPromise = source !== 's3' && openListConnection
      ? listOpenListBackups(openListConnection)
      : Promise.resolve<OpenListBackupFile[] | null>(null)
    const s3Promise = source !== 'openlist' && s3Connection
      ? listS3Backups(s3Connection)
      : Promise.resolve<S3BackupFile[] | null>(null)
    try {
      const [openListResult, s3Result] = await Promise.allSettled([openListPromise, s3Promise])
      if (loadVersion !== remoteLoadVersionRef.current) return
      const errors: string[] = []
      let openListFiles: OpenListBackupFile[] = []
      let s3Files: S3BackupFile[] = []
      if (openListResult.status === 'fulfilled') {
        openListFiles = openListResult.value || []
      } else {
        errors.push(errorMessage(openListResult.reason, 'OpenList 历史读取失败'))
      }
      if (s3Result.status === 'fulfilled') {
        s3Files = s3Result.value || []
      } else {
        errors.push(errorMessage(s3Result.reason, 'S3 历史读取失败'))
      }
      const replaceSources: RemoteBackupSource[] = source === 'all' ? ['openlist', 's3'] : [source]
      mergeHistoryItems(
        openListFiles,
        source === 's3' ? null : openListConnection,
        s3Files,
        source === 'openlist' ? null : s3Connection,
        replaceSources,
      )
      if (errors.length > 0) {
        const message = errors.join('; ')
        setError(message)
        if (showToast) toast.error(message)
      } else if (showToast) {
        toast.success(`已刷新远程历史，共 ${openListFiles.length + s3Files.length} 个备份`)
      }
    } finally {
      if (manageBusy && loadVersion === remoteLoadVersionRef.current) {
        setBusy('none')
      }
    }
  }

  const recoverLegacyLocalDirectory = async () => {
    for (const directory of legacyLocalBackupDirectories()) {
      try {
        const entries = await listLocalBackups(directory)
        if (entries.length === 0) continue
        const settings = await saveLocalBackupDirectory(directory)
        if (!settings.localDirectory) continue
        clearLegacyLocalBackupHistory()
        return settings.localDirectory
      } catch {
        continue
      }
    }
    return ''
  }

  useEffect(() => {
    let active = true
    const loadLocalBackups = async () => {
      try {
        const settings = await fetchLocalBackupSettings()
        if (!active) return
        let directory = settings.localDirectory
        if (!directory) {
          directory = await recoverLegacyLocalDirectory()
        } else {
          clearLegacyLocalBackupHistory()
        }
        if (!active) return
        setLocalDirectory(directory)
        onLocalDirectoryChange?.(directory)
        await loadLocalHistory(directory)
      } catch (localError) {
        if (!active) return
        setError(errorMessage(localError, '读取本地备份目录设置失败'))
      }
    }
    void loadLocalBackups()
    return () => {
      active = false
    }
  }, [onLocalDirectoryChange, refreshToken])

  useEffect(() => {
    const nextOpenListKey = connectionKey(configuredConnection)
    const nextS3Key = connectionKey(configuredS3Connection)
    const configurationChanged = configuredOpenListKeyRef.current !== nextOpenListKey
      || configuredS3KeyRef.current !== nextS3Key
    configuredOpenListKeyRef.current = nextOpenListKey
    configuredS3KeyRef.current = nextS3Key

    const nextOpenListConnection = configurationChanged
      ? configuredConnection || null
      : activeConnection
    const nextS3Connection = configurationChanged
      ? configuredS3Connection || null
      : activeS3Connection
    if (configurationChanged) {
      setActiveConnection(nextOpenListConnection)
      setActiveS3Connection(nextS3Connection)
    }
    if (filter === 'local') {
      setItems(sortHistoryItems([
        ...localItemsRef.current,
      ]))
      return
    }
    if (filter === 'openlist') {
      if (!nextOpenListConnection) {
        setItems(previous => sortHistoryItems([
          ...localItemsRef.current,
          ...previous.filter(item => item.source !== 'openlist'),
        ]))
        return
      }
      void loadRemoteHistory(nextOpenListConnection, nextS3Connection, false, true, 'openlist')
      return
    }
    if (!nextS3Connection) {
      setItems(previous => sortHistoryItems([
        ...localItemsRef.current,
        ...previous.filter(item => item.source !== 's3'),
      ]))
      return
    }
    void loadRemoteHistory(nextOpenListConnection, nextS3Connection, false, true, 's3')
  }, [configuredConnection, configuredS3Connection, refreshToken])

  const handleFilterChange = (nextFilter: BackupHistoryFilter) => {
    if (busy !== 'none' || pendingRestoreItem !== null || restoreConfirming || profileImportPreview !== null || profileImportBusy || openingLocationId !== '') return
    setFilter(nextFilter)
    setError('')
    if (nextFilter === 'local') {
      void loadLocalHistory(localDirectory, false, true)
      return
    }

    if (nextFilter === 'openlist' && !activeConnection) {
      setItems(previous => sortHistoryItems([
        ...localItemsRef.current,
        ...previous.filter(item => item.source !== nextFilter),
      ]))
      return
    }
    if (nextFilter === 's3' && !activeS3Connection) {
      setItems(previous => sortHistoryItems([
        ...localItemsRef.current,
        ...previous.filter(item => item.source !== nextFilter),
      ]))
      return
    }

    void loadRemoteHistory(
      activeConnection,
      activeS3Connection,
      false,
      true,
      nextFilter,
    )
  }

  const handleRefresh = async () => {
    setError('')
    let directory = localDirectory
    if (!directory) {
      directory = await recoverLegacyLocalDirectory()
      if (directory) {
        setLocalDirectory(directory)
        onLocalDirectoryChange?.(directory)
      }
    }
    if (filter === 'local') {
      await loadLocalHistory(directory, false, true)
      toast.success(`已刷新本地历史，共 ${localItemsRef.current.length} 个备份`)
      return
    }

    if (filter === 'openlist' && !activeConnection) {
      setItems(previous => sortHistoryItems([
        ...localItemsRef.current,
        ...previous.filter(item => item.source !== filter),
      ]))
      setError(`请先配置 ${sourceLabel(filter)}`)
      return
    }
    if (filter === 's3' && !activeS3Connection) {
      setItems(previous => sortHistoryItems([
        ...localItemsRef.current,
        ...previous.filter(item => item.source !== filter),
      ]))
      setError(`请先配置 ${sourceLabel(filter)}`)
      return
    }
    await loadRemoteHistory(
      activeConnection,
      activeS3Connection,
      false,
      true,
      filter,
    )
    toast.success(`已刷新${sourceLabel(filter)}历史`)
  }

  const handleDownload = async (item: BackupHistoryItem) => {
    if (!item.remoteFile || item.source === 'local') return
    setBusy('download')
    setDownloadingItemId(item.id)
    setError('')
    try {
      const result = item.source === 'openlist'
        ? await downloadOpenListBackup(item.remoteFile.name, activeConnection || undefined)
        : await downloadS3Backup(item.remoteFile.name, activeS3Connection || undefined)
      if (result.cancelled) {
        toast.info('已取消下载')
        return
      }
      if (!result.zipPath) {
        throw new Error('下载完成但未返回本地文件路径')
      }
      const downloadedDirectory = result.localDirectory?.trim() || localDirectory
      if (downloadedDirectory && downloadedDirectory !== localDirectory) {
        setLocalDirectory(downloadedDirectory)
        onLocalDirectoryChange?.(downloadedDirectory)
      }
      await loadLocalHistory(downloadedDirectory)
      toast.success(`${result.message || `已从${sourceLabel(item.source)}下载备份`}：${result.zipPath}`)
    } catch (downloadError) {
      const message = errorMessage(downloadError, `${sourceLabel(item.source)}备份下载失败`)
      setError(message)
      toast.error(message)
    } finally {
      setBusy('none')
      setDownloadingItemId('')
    }
  }

  const requestDownload = (item: BackupHistoryItem) => {
    if (item.source === 'local') return
    if (!item.remoteFile) {
      setError(`${sourceLabel(item.source)} 备份文件信息不可用，请先刷新历史`)
      return
    }
    if (item.source === 'openlist' && !activeConnection) {
      setError('请先扫描或配置 OpenList')
      return
    }
    if (item.source === 's3' && !activeS3Connection) {
      setError('请先扫描或配置 S3')
      return
    }
    void handleDownload(item)
  }

  const showProfileImportResult = (item: BackupHistoryItem, result: {
    imported?: number
    importedCount?: number
    profileCount?: number
    createdCount?: number
    overwrittenCount?: number
    renamedCount?: number
    warnings?: string[]
  }) => {
    const importedCount = Number(result.importedCount ?? result.profileCount ?? result.imported ?? 0)
    const overwrittenCount = Number(result.overwrittenCount ?? 0)
    const createdCount = Number(result.createdCount ?? Math.max(0, importedCount - overwrittenCount))
    const renamedCount = Number(result.renamedCount ?? 0)
    const summary = `已从${sourceLabel(item.source)}恢复：新建 ${createdCount} 个，覆盖 ${overwrittenCount} 个，重命名 ${renamedCount} 个`
    const warnings = result.warnings || []
    if (warnings.length > 0) {
      toast.warning(`${summary}；${warnings[0]}`)
    } else {
      toast.success(summary)
    }
  }

  const executeProfileImport = async (
    item: BackupHistoryItem,
    preview: BrowserProfilePackageImportPreview,
    actions: BrowserProfilePackageImportAction[],
  ) => {
    setProfileImportPreview(null)
    setProfileImportItem(null)
    setProfileImportBusy(true)
    setBusy('restore-merge')
    setRestoringItemId(item.id)
    setError('')
    try {
      const result = await importBrowserProfilePackageWithOptions(preview.zipPath, 'new', true, actions)
      if (!result.cancelled) {
        showProfileImportResult(item, result)
      }
    } catch (importError) {
      const message = errorMessage(importError, `${sourceLabel(item.source)}实例恢复失败`)
      setError(message)
      toast.error(message)
    } finally {
      setProfileImportBusy(false)
      setBusy('none')
      setRestoringItemId('')
    }
  }

  const handleRestore = async (item: BackupHistoryItem) => {
    setBusy('restore-merge')
    setRestoringItemId(item.id)
    setError('')
    try {
      let result
      let restorePath = ''

      if (item.source === 'local') {
        if (item.metadataOrphan) {
          throw new Error(item.metadataError || '同名 JSON 存在，但 ZIP 文件不存在')
        }
        const localPath = item.localPath?.trim()
        if (!localPath) {
          throw new Error('本地备份路径不可用，请重新导入该 ZIP 文件')
        }
        restorePath = localPath
      } else if (item.source === 'openlist') {
        if (!item.remoteFile) {
          throw new Error('OpenList 备份文件信息不可用，请先刷新历史')
        }
        if (!activeConnection) {
          throw new Error('请先点击“配置”连接 OpenList')
        }
        const downloaded = await downloadOpenListBackup(item.remoteFile.name, activeConnection)
        if (downloaded.cancelled) return
        restorePath = downloaded.zipPath?.trim() || ''
        if (!restorePath) {
          throw new Error('下载备份完成但未返回本地文件路径')
        }
        const downloadedDirectory = downloaded.localDirectory?.trim() || localDirectory
        if (downloadedDirectory && downloadedDirectory !== localDirectory) {
          setLocalDirectory(downloadedDirectory)
          onLocalDirectoryChange?.(downloadedDirectory)
        }
        await loadLocalHistory(downloadedDirectory)
      } else if (item.source === 's3') {
        if (!item.remoteFile) {
          throw new Error('S3 备份文件信息不可用，请先刷新历史')
        }
        if (!activeS3Connection) {
          throw new Error('请先配置 S3')
        }
        const downloaded = await downloadS3Backup(item.remoteFile.name, activeS3Connection)
        if (downloaded.cancelled) return
        restorePath = downloaded.zipPath?.trim() || ''
        if (!restorePath) {
          throw new Error('下载备份完成但未返回本地文件路径')
        }
        const downloadedDirectory = downloaded.localDirectory?.trim() || localDirectory
        if (downloadedDirectory && downloadedDirectory !== localDirectory) {
          setLocalDirectory(downloadedDirectory)
          onLocalDirectoryChange?.(downloadedDirectory)
        }
        await loadLocalHistory(downloadedDirectory)
      } else {
        throw new Error('未知备份来源')
      }

      result = await restoreLocalSystemConfig(restorePath)
      if (result.requiresProfileImportConfirmation && result.profileImportPreview) {
        setProfileImportItem(item)
        setProfileImportPreview(result.profileImportPreview)
        return
      }
      if (result.packageType === 'profile') {
        showProfileImportResult(item, result)
      } else if (result.partial || Number(result.componentFailed || 0) > 0) {
        toast.warning('备份已恢复，但有部分模块失败，请查看导入结果')
      } else {
        toast.success(`已从${sourceLabel(item.source)}合并恢复`)
      }
    } catch (restoreError) {
      const message = errorMessage(restoreError, `${sourceLabel(item.source)}备份恢复失败`)
      setError(message)
      toast.error(message)
    } finally {
      setBusy('none')
      setRestoringItemId('')
      setRestoreConfirming(false)
    }
  }

  const requestRestore = (item: BackupHistoryItem) => {
    if (item.source === 'local' && item.metadataOrphan) {
      setError(item.metadataError || '同名 JSON 存在，但 ZIP 文件不存在')
      return
    }
    if (item.source === 'local' && !item.localPath?.trim()) {
      setError('本地备份路径不可用，请重新导入该 ZIP 文件')
      return
    }
    if (item.source === 'openlist' && !item.remoteFile) {
      setError('OpenList 备份文件信息不可用，请先刷新历史')
      return
    }
    if (item.source === 'openlist' && !activeConnection) {
      setError('请先点击“配置”连接 OpenList')
      return
    }
    if (item.source === 's3') {
      if (!item.remoteFile) {
        setError('S3 备份文件信息不可用，请先刷新历史')
        return
      }
      if (!activeS3Connection) {
        setError('请先配置 S3')
        return
      }
    }
    setError('')
    setPendingRestoreItem(item)
  }

  const handleOpenLocation = async (item: BackupHistoryItem) => {
    setOpeningLocationId(item.id)
    setError('')
    try {
      if (item.source === 'local') {
        if (item.metadataOrphan) {
          throw new Error(item.metadataError || '同名 JSON 存在，但 ZIP 文件不存在')
        }
        const localPath = item.localPath?.trim()
        if (!localPath) {
          throw new Error('本地备份路径不可用')
        }
        await openBackupPath(localPath)
        return
      }

      if (item.source === 's3') {
        throw new Error('S3 备份地址需要使用带凭据的客户端访问')
      }
      const location = item.location.trim()
      if (!location) {
        throw new Error('OpenList 备份地址不可用')
      }
      const parsed = new URL(location)
      if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
        throw new Error('OpenList 备份地址必须使用 http 或 https')
      }
      BrowserOpenURL(location)
    } catch (openError) {
      const message = errorMessage(openError, '打开备份地址失败')
      setError(message)
      toast.error(message)
    } finally {
      setOpeningLocationId('')
    }
  }

  const filteredItems = items.filter(item => item.source === filter)
  const filterItems: Array<{ key: BackupHistoryFilter; label: string }> = [
    { key: 'local', label: '本地' },
    { key: 'openlist', label: 'OpenList' },
    { key: 's3', label: 'S3' },
  ]
  const tableColumns: TableColumn<BackupHistoryItem>[] = [
    {
      key: 'name',
      title: '备份文件',
      width: 280,
      render: value => {
        const name = String(value || '').trim()
        return (
          <span
            className="block max-w-[280px] truncate text-[var(--color-text-primary)]"
            title={name || undefined}
          >
            {name || '-'}
          </span>
        )
      },
    },
    {
      key: 'packageType',
      title: '备份类型',
      width: 180,
      render: (_value, item) => renderBackupPackageType(item),
    },
    {
      key: 'profileNames',
      title: '备份内容',
      width: 240,
      render: (_value, item) => renderBackupPackageContent(item),
    },
    {
      key: 'source',
      title: '来源',
      width: 120,
      render: value => (
        <span className="inline-flex items-center gap-1.5 text-[var(--color-text-secondary)]">
          {value === 'openlist'
            ? <Cloud className="h-4 w-4" />
            : value === 's3'
              ? <Database className="h-4 w-4" />
              : <span className="h-2 w-2 rounded-full bg-[var(--color-text-muted)]" />}
          {value === 'openlist' ? 'OpenList' : value === 's3' ? 'S3' : '本地'}
        </span>
      ),
    },
    {
      key: 'modifiedAt',
      title: '备份时间',
      width: 190,
      render: value => formatModifiedAt(String(value || '')),
    },
    {
      key: 'size',
      title: '大小',
      width: 110,
      align: 'right',
      render: value => formatSize(Number(value) || 0),
    },
    {
      key: 'location',
      title: '位置',
      width: 280,
      render: (value, item) => {
        const location = String(value || '').trim()
        if (!location) return '—'
        if (item.source === 'local' && item.metadataOrphan) {
          return <span className="text-[var(--color-error)]" title={item.metadataError}>同名 JSON，ZIP 缺失</span>
        }
        if (item.source === 's3') {
          return <span className="block max-w-[320px] truncate text-[var(--color-text-secondary)]" title={location}>{location}</span>
        }
        return (
          <button
            type="button"
            className="group inline-flex max-w-full min-w-0 items-center gap-1 text-left text-[var(--color-accent)] hover:underline disabled:cursor-wait disabled:opacity-60"
            title={`打开备份地址：${location}`}
             aria-label={`打开${sourceLabel(item.source)}备份地址`}
            onClick={() => { void handleOpenLocation(item) }}
            disabled={openingLocationId === item.id}
          >
            <span className="block max-w-[290px] truncate">{location}</span>
            <ExternalLink className="h-3.5 w-3.5 shrink-0 opacity-70 group-hover:opacity-100" />
          </button>
        )
      },
    },
    {
      key: 'actions',
      title: '操作',
      width: 220,
      align: 'right',
      render: (_value, item) => (
        <div className="flex flex-wrap justify-end gap-2">
          {item.source !== 'local' && (
            <Button
              size="sm"
              variant="primary"
              className="!border-[var(--color-accent)] !bg-[var(--color-accent)] !text-[var(--color-text-inverse)] hover:!bg-[var(--color-accent-hover)] disabled:!opacity-60"
              onClick={() => requestDownload(item)}
              loading={downloadingItemId === item.id}
              disabled={busy !== 'none' || pendingRestoreItem !== null || restoreConfirming || profileImportPreview !== null || profileImportBusy}
              title="下载到本地"
            >
              <Download className="h-3.5 w-3.5" />
              下载
            </Button>
          )}
          <Button
            size="sm"
            variant="secondary"
            className="!border-[var(--color-border-strong)] !bg-[var(--color-bg-muted)] !text-[var(--color-text-primary)] hover:!bg-[var(--color-border-default)] disabled:!opacity-60"
            onClick={() => requestRestore(item)}
            loading={restoringItemId === item.id}
              disabled={busy !== 'none' || pendingRestoreItem !== null || restoreConfirming || profileImportPreview !== null || profileImportBusy || downloadingItemId !== '' || item.metadataOrphan === true}
          >
            合并恢复
          </Button>
        </div>
      ),
    },
  ]

  return (
    <Card padding="none">
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-[var(--color-border-default)] px-5 py-3">
        <div
          className="min-w-[240px] flex-1"
          role="tablist"
          aria-label="备份来源"
        >
          <div className="grid grid-cols-3 gap-0.5 rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-muted)] p-0.5">
          {filterItems.map(filterItem => (
            <button
              key={filterItem.key}
              type="button"
              role="tab"
              aria-selected={filter === filterItem.key}
              onClick={() => handleFilterChange(filterItem.key)}
              disabled={busy !== 'none' || pendingRestoreItem !== null || restoreConfirming || profileImportPreview !== null || profileImportBusy || openingLocationId !== ''}
              className={`flex h-8 min-w-0 items-center justify-center whitespace-nowrap rounded-lg px-3 text-sm font-medium transition-colors duration-200 disabled:cursor-wait disabled:opacity-60 ${
                filter === filterItem.key
                  ? '!bg-[#0f172a] !text-white shadow-sm [-webkit-text-fill-color:#ffffff]'
                  : 'bg-transparent text-[var(--color-text-primary)] hover:bg-[var(--color-bg-surface)] hover:text-[var(--color-text-primary)]'
              }`}
            >
              {filterItem.label}
            </button>
          ))}
          </div>
        </div>
        <div className="flex max-w-full flex-wrap items-center justify-end gap-2">
          {actions}
          {filter === 'local' && localDirectory && (
            <span
              className="max-w-[260px] truncate text-xs text-[var(--color-text-muted)]"
              title={localDirectory}
            >
              {localDirectory}
            </span>
          )}
          <Button size="sm" variant="secondary" className="!border-[var(--color-border-default)] !bg-[var(--color-bg-muted)] !text-[var(--color-text-primary)] hover:!border-[var(--color-border-strong)] hover:!bg-[var(--color-border-default)]" onClick={handleRefresh} loading={busy === 'list'} disabled={busy !== 'none' || pendingRestoreItem !== null || restoreConfirming || profileImportPreview !== null || profileImportBusy || openingLocationId !== ''}>
            <RefreshCw className="h-4 w-4" />
            刷新
          </Button>
        </div>
      </div>
      <div className="space-y-3 p-5">
        {error && <p role="alert" className="text-sm text-[var(--color-error)]">{error}</p>}
        <Table
          columns={tableColumns}
          data={filteredItems}
          rowKey="id"
          loading={busy === 'list' && filteredItems.length === 0}
          emptyText={filter === 'local'
            ? (localDirectory ? '暂无本地备份' : '请先配置本地备份目录')
            : filter === 'openlist'
              ? (activeConnection ? '暂无 OpenList 备份' : '请先配置 OpenList')
              : (activeS3Connection ? '暂无 S3 备份' : '请先配置 S3')}
          className="w-full"
          tableMinWidth={1620}
          maxHeight="none"
         />
      </div>
      <ConfirmModal
        open={pendingRestoreItem !== null}
        onClose={() => setPendingRestoreItem(null)}
        onConfirmStart={() => setRestoreConfirming(true)}
        onConfirm={() => {
          if (pendingRestoreItem) {
            void handleRestore(pendingRestoreItem)
          }
        }}
        title="确认合并恢复"
        content={(
          <div className="space-y-2 text-sm leading-5">
            <p>
              将恢复「{pendingRestoreItem?.name || '备份文件'}」。
            </p>
            <p>
              恢复前会停止运行中的浏览器和代理；当前数据不会清空，只合并新增数据，已存在冲突不覆盖。
            </p>
          </div>
        )}
        confirmText="开始恢复"
      />
      <ProfilePackageConflictModal
        preview={profileImportPreview}
        busy={profileImportBusy}
        onClose={() => {
          setProfileImportPreview(null)
          setProfileImportItem(null)
        }}
        onConfirm={(actions) => {
          if (profileImportItem && profileImportPreview) {
            void executeProfileImport(profileImportItem, profileImportPreview, actions)
          }
        }}
      />
    </Card>
  )
}
