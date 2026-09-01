import { useEffect, useRef, useState, type ReactNode } from 'react'
import { Cloud, Database, Download, ExternalLink, RefreshCw, Search } from 'lucide-react'

import { Button, Card, ConfirmModal, Table, toast } from '../../../shared/components'
import type { TableColumn } from '../../../shared/components'
import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime'
import { getBackupFileInfo, openBackupPath, restoreLocalSystemConfig } from '../api'
import {
  downloadOpenListBackup,
  listOpenListBackups,
  restoreOpenListBackup,
} from '../channels/openlist/api'
import type { OpenListBackupFile, OpenListConnection } from '../channels/openlist/api'
import {
  downloadS3Backup,
  listS3Backups,
  restoreS3Backup,
} from '../channels/s3/api'
import type { S3BackupFile, S3Connection } from '../channels/s3/api'
import { BackupRemoteScanModal } from './BackupRemoteScanModal'
import type { BackupRemoteScanResult } from './BackupRemoteScanModal'

type BackupHistorySource = 'local' | 'openlist' | 's3'
type BackupHistoryFilter = BackupHistorySource
type RemoteBackupFile = OpenListBackupFile | S3BackupFile

interface BackupHistoryItem {
  id: string
  name: string
  source: BackupHistorySource
  size: number
  modifiedAt: string
  location: string
  localPath?: string
  remoteFile?: RemoteBackupFile
}

interface StoredLocalBackup {
  name: string
  path: string
  size: number
  modifiedAt: string
}

interface StoredOpenListHistory {
  location: string
  files: OpenListBackupFile[]
}

interface StoredS3History {
  location: string
  files: S3BackupFile[]
}

interface BackupHistoryTableProps {
  actions?: ReactNode
  configuredConnection?: OpenListConnection | null
  configuredS3Connection?: S3Connection | null
  refreshToken?: number
  onBusyChange?: (busy: boolean) => void
}

const LOCAL_HISTORY_KEY = 'ant_chrome_local_backup_history'
const OPENLIST_HISTORY_CACHE_KEY = 'ant_chrome_openlist_backup_history_cache'
const S3_HISTORY_CACHE_KEY = 'ant_chrome_s3_backup_history_cache'

function readLocalBackups(): StoredLocalBackup[] {
  if (typeof window === 'undefined') return []
  try {
    const raw = localStorage.getItem(LOCAL_HISTORY_KEY)
    const parsed = raw ? JSON.parse(raw) : []
    if (!Array.isArray(parsed)) return []
    const entries = parsed as Array<Record<string, unknown>>
    return entries
      .map(item => ({
        name: typeof item.name === 'string' ? item.name : '',
        path: typeof item.path === 'string' ? item.path : '',
        size: Number.isFinite(item.size) ? Math.max(0, Number(item.size)) : 0,
        modifiedAt: typeof item.modifiedAt === 'string' ? item.modifiedAt : '',
      }))
      .filter(item => item.name || item.path)
  } catch {
    return []
  }
}

function saveLocalBackups(items: StoredLocalBackup[]) {
  if (typeof window === 'undefined') return
  try {
    localStorage.setItem(LOCAL_HISTORY_KEY, JSON.stringify(items))
  } catch {
    return
  }
}

function readCachedOpenListHistory(): StoredOpenListHistory | null {
  if (typeof window === 'undefined') return null
  try {
    const raw = localStorage.getItem(OPENLIST_HISTORY_CACHE_KEY)
    const parsed = raw ? JSON.parse(raw) : null
    if (!parsed || typeof parsed !== 'object' || !Array.isArray(parsed.files)) return null
    const entries = parsed.files as Array<Record<string, unknown>>
    return {
      location: typeof parsed.location === 'string' ? parsed.location : 'OpenList',
      files: entries
        .map(item => ({
          name: typeof item.name === 'string' ? item.name : '',
          size: Number.isFinite(item.size) ? Math.max(0, Number(item.size)) : 0,
          modifiedAt: typeof item.modifiedAt === 'string' ? item.modifiedAt : '',
        }))
        .filter(item => item.name),
    }
  } catch {
    return null
  }
}

function saveCachedOpenListHistory(files: OpenListBackupFile[], connection: OpenListConnection) {
  if (typeof window === 'undefined') return
  try {
    localStorage.setItem(OPENLIST_HISTORY_CACHE_KEY, JSON.stringify({
      location: buildOpenListLocation(connection),
      files,
    }))
  } catch {
    // 缓存不可用时不阻断当前历史读取。
  }
}

function readCachedS3History(): StoredS3History | null {
  if (typeof window === 'undefined') return null
  try {
    const raw = localStorage.getItem(S3_HISTORY_CACHE_KEY)
    const parsed = raw ? JSON.parse(raw) : null
    if (!parsed || typeof parsed !== 'object' || !Array.isArray(parsed.files)) return null
    const entries = parsed.files as Array<Record<string, unknown>>
    return {
      location: typeof parsed.location === 'string' ? parsed.location : 'S3',
      files: entries
        .map(item => ({
          name: typeof item.name === 'string' ? item.name : '',
          size: Number.isFinite(item.size) ? Math.max(0, Number(item.size)) : 0,
          modifiedAt: typeof item.modifiedAt === 'string' ? item.modifiedAt : '',
        }))
        .filter(item => item.name),
    }
  } catch {
    return null
  }
}

function saveCachedS3History(files: S3BackupFile[], connection: S3Connection) {
  if (typeof window === 'undefined') return
  try {
    localStorage.setItem(S3_HISTORY_CACHE_KEY, JSON.stringify({
      location: buildS3Location(connection),
      files,
    }))
  } catch {
    return
  }
}

export async function recordLocalBackupHistory(path: string) {
  const trimmedPath = path.trim()
  if (!trimmedPath || typeof window === 'undefined') return

  const name = trimmedPath.split(/[\\/]/).pop() || trimmedPath
  let fileInfo = { size: 0, modifiedAt: '' }
  try {
    fileInfo = await getBackupFileInfo(trimmedPath)
  } catch {
    fileInfo = { size: 0, modifiedAt: '' }
  }
  const nextItem: StoredLocalBackup = {
    name,
    path: trimmedPath,
    size: fileInfo.size,
    modifiedAt: fileInfo.modifiedAt || new Date().toISOString(),
  }
  const previous = readLocalBackups()
  const next = [nextItem, ...previous.filter(item => item.path !== trimmedPath)].slice(0, 50)
  saveLocalBackups(next)
}

async function refreshMissingLocalBackupInfo() {
  const previous = readLocalBackups()
  const pending = previous.filter(item => item.path && item.size <= 0)
  if (!pending.length) return new Map<string, StoredLocalBackup>()

  const resolved = await Promise.all(pending.map(async item => {
    try {
      const fileInfo = await getBackupFileInfo(item.path)
      return {
        ...item,
        size: fileInfo.size,
        modifiedAt: fileInfo.modifiedAt || item.modifiedAt,
      }
    } catch { return item }
  }))
  const updates = new Map(resolved.map(item => [item.path, item]))
  const changed = resolved.some((item, index) => {
    const original = pending[index]
    return item.size !== original.size || item.modifiedAt !== original.modifiedAt
  })
  if (changed) {
    saveLocalBackups(previous.map(item => updates.get(item.path) || item))
  }
  return updates
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

function buildInitialItems(): BackupHistoryItem[] {
  const localItems = readLocalBackups().map(item => ({
    id: `local:${item.path || item.name}`,
    name: item.name || item.path,
    source: 'local' as const,
    size: item.size,
    modifiedAt: item.modifiedAt,
    location: item.path || '本地文件',
    localPath: item.path,
  }))
  const cached = readCachedOpenListHistory()
  const openListItems = cached?.files.map(file => ({
    id: `openlist:${file.name}`,
    name: file.name,
    source: 'openlist' as const,
    size: file.size,
    modifiedAt: file.modifiedAt,
    location: cached.location,
    remoteFile: file,
  })) || []
  const s3Cached = readCachedS3History()
  const s3Items = s3Cached?.files.map(file => ({
    id: `s3:${file.name}`,
    name: file.name,
    source: 's3' as const,
    size: file.size,
    modifiedAt: file.modifiedAt,
    location: s3Cached.location,
    remoteFile: file,
  })) || []

  return sortHistoryItems([...localItems, ...openListItems, ...s3Items])
}

function sortHistoryItems(items: BackupHistoryItem[]) {
  return [...items].sort((left, right) => {
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

function remoteFilesFromItems<T extends RemoteBackupFile>(items: BackupHistoryItem[], source: BackupHistorySource) {
  return items
    .filter(item => item.source === source && item.remoteFile)
    .map(item => item.remoteFile as T)
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

function sourceLabel(source: BackupHistorySource) {
  if (source === 'local') return '本地'
  if (source === 'openlist') return 'OpenList'
  return 'S3'
}

export function BackupHistoryTable({ actions, configuredConnection, configuredS3Connection, refreshToken = 0, onBusyChange }: BackupHistoryTableProps) {
  const [items, setItems] = useState<BackupHistoryItem[]>(buildInitialItems)
  const [filter, setFilter] = useState<BackupHistoryFilter>('local')
  const [busy, setBusy] = useState<'none' | 'list' | 'restore-merge' | 'download'>('none')
  const [restoringItemId, setRestoringItemId] = useState('')
  const [downloadingItemId, setDownloadingItemId] = useState('')
  const [pendingRestoreItem, setPendingRestoreItem] = useState<BackupHistoryItem | null>(null)
  const [restoreConfirming, setRestoreConfirming] = useState(false)
  const [openingLocationId, setOpeningLocationId] = useState('')
  const [scanModalOpen, setScanModalOpen] = useState(false)
  const [error, setError] = useState('')
  const [activeConnection, setActiveConnection] = useState<OpenListConnection | null>(configuredConnection || null)
  const [activeS3Connection, setActiveS3Connection] = useState<S3Connection | null>(configuredS3Connection || null)
  const configuredOpenListKeyRef = useRef<string | null>(null)
  const configuredS3KeyRef = useRef<string | null>(null)
  const remoteLoadVersionRef = useRef(0)

  useEffect(() => {
    onBusyChange?.(busy === 'restore-merge' || busy === 'download' || pendingRestoreItem !== null || restoreConfirming)
  }, [busy, onBusyChange, pendingRestoreItem, restoreConfirming])

  useEffect(() => {
    let disposed = false
    void refreshMissingLocalBackupInfo().then(updates => {
      if (disposed || updates.size === 0) return
      setItems(previous => sortHistoryItems(previous.map(item => {
        if (item.source !== 'local') return item
        const updated = updates.get(item.localPath || '')
        return updated
          ? { ...item, size: updated.size, modifiedAt: updated.modifiedAt }
          : item
      })))
    })
    return () => {
      disposed = true
    }
  }, [refreshToken])

  const mergeHistoryItems = (
    openListFiles: OpenListBackupFile[],
    openListConnection: OpenListConnection | null,
    s3Files: S3BackupFile[],
    s3Connection: S3Connection | null,
  ) => {
    const localItems = readLocalBackups().map(item => ({
      id: `local:${item.path || item.name}`,
      name: item.name || item.path,
      source: 'local' as const,
      size: item.size,
      modifiedAt: item.modifiedAt,
      location: item.path || '本地文件',
      localPath: item.path,
    }))
    const openListItems = openListConnection
      ? openListFiles.map(file => ({
        id: `openlist:${file.name}`,
        name: file.name,
        source: 'openlist' as const,
        size: file.size,
        modifiedAt: file.modifiedAt,
        location: buildOpenListLocation(openListConnection),
        remoteFile: file,
      }))
      : []
    const s3Items = s3Connection
      ? s3Files.map(file => ({
        id: `s3:${file.name}`,
        name: file.name,
        source: 's3' as const,
        size: file.size,
        modifiedAt: file.modifiedAt,
        location: buildS3Location(s3Connection),
        remoteFile: file,
      }))
      : []
    setItems(sortHistoryItems([...localItems, ...openListItems, ...s3Items]))
  }

  const loadRemoteHistory = async (
    openListConnection: OpenListConnection | null,
    s3Connection: S3Connection | null,
    showToast: boolean,
  ) => {
    const loadVersion = ++remoteLoadVersionRef.current
    setBusy('list')
    setError('')
    const openListPromise = openListConnection
      ? listOpenListBackups(openListConnection)
      : Promise.resolve<OpenListBackupFile[] | null>(null)
    const s3Promise = s3Connection
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
        if (openListConnection) saveCachedOpenListHistory(openListFiles, openListConnection)
      } else {
        errors.push(errorMessage(openListResult.reason, 'OpenList 历史读取失败'))
      }
      if (s3Result.status === 'fulfilled') {
        s3Files = s3Result.value || []
        if (s3Connection) saveCachedS3History(s3Files, s3Connection)
      } else {
        errors.push(errorMessage(s3Result.reason, 'S3 历史读取失败'))
      }
      mergeHistoryItems(openListFiles, openListConnection, s3Files, s3Connection)
      if (errors.length > 0) {
        const message = errors.join('; ')
        setError(message)
        if (showToast) toast.error(message)
      } else if (showToast) {
        toast.success(`已刷新远程历史，共 ${openListFiles.length + s3Files.length} 个备份`)
      }
    } finally {
      if (loadVersion === remoteLoadVersionRef.current) {
        setBusy('none')
      }
    }
  }

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
    if (!nextOpenListConnection && !nextS3Connection) {
      setItems(buildInitialItems())
      return
    }
    void loadRemoteHistory(nextOpenListConnection, nextS3Connection, false)
  }, [configuredConnection, configuredS3Connection, refreshToken])

  const handleRefresh = () => {
    setItems(buildInitialItems())
    if (!activeConnection && !activeS3Connection) {
      setError('请先配置 OpenList 或 S3')
      return
    }
    void loadRemoteHistory(activeConnection, activeS3Connection, true)
  }

  const handleRemoteScanned = (result: BackupRemoteScanResult) => {
    remoteLoadVersionRef.current += 1
    setBusy('none')
    setError('')
    if (result.channel === 'openlist') {
      const connection = result.connection as OpenListConnection
      const files = result.files as OpenListBackupFile[]
      setActiveConnection(connection)
      setFilter('openlist')
      saveCachedOpenListHistory(files, connection)
      mergeHistoryItems(files, connection, remoteFilesFromItems<S3BackupFile>(items, 's3'), activeS3Connection)
      return
    }

    const connection = result.connection as S3Connection
    const files = result.files as S3BackupFile[]
    setActiveS3Connection(connection)
    setFilter('s3')
    saveCachedS3History(files, connection)
    mergeHistoryItems(remoteFilesFromItems<OpenListBackupFile>(items, 'openlist'), activeConnection, files, connection)
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
      await recordLocalBackupHistory(result.zipPath)
      setItems(buildInitialItems())
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

  const handleRestore = async (item: BackupHistoryItem) => {
    setBusy('restore-merge')
    setRestoringItemId(item.id)
    setError('')
    try {
      let result
      if (item.source === 'local') {
        const localPath = item.localPath?.trim()
        if (!localPath) {
          throw new Error('本地备份路径不可用，请重新导入该 ZIP 文件')
        }
        result = await restoreLocalSystemConfig(localPath)
      } else if (item.source === 'openlist') {
        if (!item.remoteFile) {
          throw new Error('OpenList 备份文件信息不可用，请先刷新历史')
        }
        if (!activeConnection) {
          throw new Error('请先点击“配置”连接 OpenList')
        }
        result = await restoreOpenListBackup(item.remoteFile.name, activeConnection)
      } else if (item.source === 's3') {
        if (!item.remoteFile) {
          throw new Error('S3 备份文件信息不可用，请先刷新历史')
        }
        if (!activeS3Connection) {
          throw new Error('请先配置 S3')
        }
        result = await restoreS3Backup(item.remoteFile.name, activeS3Connection)
      } else {
        throw new Error('未知备份来源')
      }
      if (result.partial || Number(result.componentFailed || 0) > 0) {
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
      render: value => <span className="min-w-0 break-all text-[var(--color-text-primary)]">{value}</span>,
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
      render: (value, item) => {
        const location = String(value || '').trim()
        if (!location) return '—'
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
              disabled={busy !== 'none' || pendingRestoreItem !== null || restoreConfirming}
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
            disabled={busy !== 'none' || pendingRestoreItem !== null || restoreConfirming || downloadingItemId !== ''}
          >
            合并恢复
          </Button>
        </div>
      ),
    },
  ]

  return (
    <Card padding="none">
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-[var(--color-border-muted)] px-5 py-3">
        <div className="flex min-w-0 items-end gap-0" role="tablist" aria-label="备份来源">
          {filterItems.map(filterItem => (
            <button
              key={filterItem.key}
              type="button"
              role="tab"
              aria-selected={filter === filterItem.key}
              onClick={() => setFilter(filterItem.key)}
              className={`border-b-2 px-4 py-2 text-sm transition-colors ${
                filter === filterItem.key
                  ? 'border-[var(--color-accent)] font-medium text-[var(--color-text-primary)]'
                  : 'border-transparent text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)]'
              }`}
            >
              {filterItem.label}
            </button>
          ))}
        </div>
        <div className="flex max-w-full flex-wrap items-center justify-end gap-2">
          {actions}
          <Button
            size="sm"
            variant="secondary"
            onClick={() => setScanModalOpen(true)}
            disabled={busy === 'restore-merge' || busy === 'download' || pendingRestoreItem !== null || restoreConfirming || openingLocationId !== ''}
            title="输入远程连接并扫描备份"
          >
            <Search className="h-4 w-4" />
            扫描远程
          </Button>
          <Button size="sm" variant="secondary" onClick={handleRefresh} loading={busy === 'list'} disabled={busy !== 'none' || pendingRestoreItem !== null || restoreConfirming || openingLocationId !== ''}>
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
          loading={busy === 'list' && items.length === 0}
          emptyText={filter === 'local'
            ? '暂无本地备份'
            : filter === 'openlist'
              ? '暂无 OpenList 备份'
              : '暂无 S3 备份'}
           className="min-w-[1000px]"
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
      <BackupRemoteScanModal
        open={scanModalOpen}
        onClose={() => setScanModalOpen(false)}
        initialOpenListConnection={activeConnection}
        initialS3Connection={activeS3Connection}
        onScanned={handleRemoteScanned}
      />
    </Card>
  )
}
