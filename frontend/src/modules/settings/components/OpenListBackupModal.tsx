import { useEffect, useMemo, useState } from 'react'
import { AlertTriangle, CheckCircle2, RefreshCw, Settings2, Upload, XCircle } from 'lucide-react'

import { Button, FormItem, Input, Modal, Progress, Table, toast } from '../../../shared/components'
import type { TableColumn } from '../../../shared/components'
import { EventsOn } from '../../../wailsjs/runtime/runtime'
import {
  listOpenListBackups,
  restoreOpenListBackup,
  testOpenListConnection,
  uploadOpenListBackup,
} from '../openListApi'
import type { OpenListBackupFile, OpenListConnection } from '../openListApi'

interface OpenListBackupModalProps {
  open: boolean
  mode: OpenListMode
  onClose: () => void
  onBusyChange?: (busy: boolean) => void
}

type OpenListMode = 'backup' | 'history'
type OpenListStage = 'connect' | 'prepare' | 'list'
type OpenListBusy = 'none' | 'test' | 'connect' | 'list' | 'upload' | 'restore-reset' | 'restore-merge'
type OpenListField = keyof OpenListConnection
type OpenListFieldErrors = Partial<Record<OpenListField, string>>
type OpenListTestResult = { status: 'success' | 'error'; message: string }

interface OpenListOperationProgress {
  phase: string
  progress: number
  message: string
}

const STORAGE_KEY = 'ant_chrome_openlist_backup_connection'

const emptyConnection: OpenListConnection = {
  baseURL: '',
  remotePath: 'ant-chrome/backups',
  username: '',
  password: '',
}

function loadSavedConnection(): Partial<OpenListConnection> {
  try {
    const value = localStorage.getItem(STORAGE_KEY)
    if (!value) return {}
    const parsed = JSON.parse(value)
    return {
      baseURL: typeof parsed?.baseURL === 'string' ? parsed.baseURL : '',
      remotePath: typeof parsed?.remotePath === 'string' ? parsed.remotePath : emptyConnection.remotePath,
      username: typeof parsed?.username === 'string' ? parsed.username : '',
    }
  } catch {
    return {}
  }
}

function saveConnection(connection: OpenListConnection) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify({
      baseURL: connection.baseURL.trim(),
      remotePath: connection.remotePath.trim(),
      username: connection.username.trim(),
    }))
  } catch {
    // 本地存储不可用时不阻断当前备份操作。
  }
}

function formatSize(size: number) {
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

function validateConnection(connection: OpenListConnection): OpenListFieldErrors {
  const errors: OpenListFieldErrors = {}
  const baseURL = connection.baseURL.trim()

  if (!baseURL) {
    errors.baseURL = '请输入 WebDAV 地址'
  } else {
    try {
      const parsed = new URL(baseURL)
      if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
        errors.baseURL = '地址必须使用 http 或 https'
      } else if (!parsed.hostname) {
        errors.baseURL = '地址缺少主机名'
      } else if (parsed.search || parsed.hash) {
        errors.baseURL = '地址不能包含查询参数或片段'
      }
    } catch {
      errors.baseURL = '请输入有效的 WebDAV 地址'
    }
  }

  const remotePath = connection.remotePath.trim().replace(/\\/g, '/')
  if (remotePath.split('/').some(segment => segment.trim() === '..')) {
    errors.remotePath = '远程目录不能包含 ..'
  }

  const username = connection.username.trim()
  if (username && !connection.password) {
    errors.password = '请输入密码，或清空用户名以使用匿名 WebDAV'
  } else if (!username && connection.password) {
    errors.username = '请输入用户名，或清空密码以使用匿名 WebDAV'
  }

  return errors
}

function normalizeOperationProgress(value: unknown): OpenListOperationProgress | null {
  if (!value || typeof value !== 'object') return null
  const payload = value as Record<string, unknown>
  const progress = Number(payload.progress)
  const message = typeof payload.message === 'string' ? payload.message.trim() : ''
  if (!message || !Number.isFinite(progress)) return null
  return {
    phase: typeof payload.phase === 'string' ? payload.phase : 'working',
    progress: Math.max(0, Math.min(100, Math.round(progress))),
    message,
  }
}

export function OpenListBackupModal({ open, mode, onClose, onBusyChange }: OpenListBackupModalProps) {
  const [stage, setStage] = useState<OpenListStage>('connect')
  const [connection, setConnection] = useState<OpenListConnection>(emptyConnection)
  const [files, setFiles] = useState<OpenListBackupFile[]>([])
  const [busy, setBusy] = useState<OpenListBusy>('none')
  const [error, setError] = useState('')
  const [fieldErrors, setFieldErrors] = useState<OpenListFieldErrors>({})
  const [testResult, setTestResult] = useState<OpenListTestResult | null>(null)
  const [operationProgress, setOperationProgress] = useState<OpenListOperationProgress | null>(null)

  useEffect(() => {
    if (!open) return
    setStage('connect')
    setFiles([])
    setBusy('none')
    setError('')
    setFieldErrors({})
    setTestResult(null)
    setOperationProgress(null)
    setConnection({ ...emptyConnection, ...loadSavedConnection(), password: '' })
  }, [open, mode])

  useEffect(() => {
    if (!open) return

    const onExportProgress = (payload: unknown) => {
      const next = normalizeOperationProgress(payload)
      if (next) setOperationProgress(next)
    }
    const onImportProgress = (payload: unknown) => {
      const next = normalizeOperationProgress(payload)
      if (next) setOperationProgress(next)
    }

    const stopExportProgress = EventsOn('backup:export:progress', onExportProgress)
    const stopImportProgress = EventsOn('backup:import:progress', onImportProgress)
    return () => {
      stopExportProgress()
      stopImportProgress()
    }
  }, [open])

  useEffect(() => {
    onBusyChange?.(busy !== 'none')
  }, [busy, onBusyChange])

  const connected = useMemo(() => ({
    ...connection,
    baseURL: connection.baseURL.trim(),
    remotePath: connection.remotePath.trim(),
    username: connection.username.trim(),
  }), [connection])

  const updateConnection = (key: keyof OpenListConnection, value: string) => {
    setConnection(current => ({ ...current, [key]: value }))
    setError('')
    setFieldErrors({})
    setTestResult(null)
  }

  const handleProceed = async () => {
    const nextErrors = validateConnection(connected)
    if (Object.keys(nextErrors).length > 0) {
      setFieldErrors(nextErrors)
      setError('')
      setTestResult(null)
      return
    }

    setTestResult(null)
    setBusy('connect')
    setError('')
    try {
      await testOpenListConnection(connected)
      saveConnection(connected)
      if (mode === 'backup') {
        setStage('prepare')
        toast.success('OpenList 连接成功，请开始备份')
        return
      }

      const nextFiles = await listOpenListBackups(connected)
      setFiles(nextFiles)
      setStage('list')
      toast.success(`已读取 ${nextFiles.length} 个备份`)
    } catch (proceedError) {
      const message = errorMessage(proceedError, mode === 'backup' ? 'OpenList 连接失败' : 'OpenList 历史读取失败')
      setError(message)
      toast.error(message)
    } finally {
      setBusy('none')
    }
  }

  const loadHistory = async (nextConnection: OpenListConnection) => {
    setBusy('list')
    setError('')
    try {
      await testOpenListConnection(nextConnection)
      const nextFiles = await listOpenListBackups(nextConnection)
      setFiles(nextFiles)
      saveConnection(nextConnection)
    } catch (loadError) {
      const message = errorMessage(loadError, 'OpenList 历史刷新失败')
      setError(message)
      toast.error(message)
    } finally {
      setBusy('none')
    }
  }

  const handleTestConnection = async () => {
    const nextErrors = validateConnection(connected)
    if (Object.keys(nextErrors).length > 0) {
      setFieldErrors(nextErrors)
      setError('')
      setTestResult(null)
      return
    }

    setBusy('test')
    setError('')
    setTestResult(null)
    try {
      await testOpenListConnection(connected)
      saveConnection(connected)
      setTestResult({ status: 'success', message: '连接测试成功，远程目录可访问' })
      toast.success('OpenList 连接测试成功')
    } catch (testError) {
      const message = errorMessage(testError, 'OpenList 连接测试失败')
      setTestResult({ status: 'error', message })
      toast.error(message)
    } finally {
      setBusy('none')
    }
  }

  const handleRefresh = () => {
    void loadHistory(connected)
  }

  const handleUpload = async () => {
    setBusy('upload')
    setError('')
    setOperationProgress(null)
    try {
      await uploadOpenListBackup(connected)
      saveConnection(connected)
      setStage('list')
      try {
        const nextFiles = await listOpenListBackups(connected)
        setFiles(nextFiles)
        toast.success('全量备份已上传到 OpenList')
      } catch (refreshError) {
        const message = errorMessage(refreshError, '备份列表刷新失败')
        setError(`备份已上传，但列表刷新失败：${message}`)
        toast.warning('备份已上传，列表刷新失败，请点击刷新')
      }
    } catch (uploadError) {
      const message = errorMessage(uploadError, 'OpenList 备份上传失败')
      setError(message)
      toast.error(message)
    } finally {
      setBusy('none')
      setOperationProgress(null)
    }
  }

  async function handleRestore(file: OpenListBackupFile, resetFirst: boolean) {
    if (resetFirst && !window.confirm('清空后恢复会删除当前业务数据，确认继续吗？')) return
    setBusy(resetFirst ? 'restore-reset' : 'restore-merge')
    setError('')
    setOperationProgress(null)
    try {
      const result = await restoreOpenListBackup(connected, file.name, resetFirst)
      if (result.partial || Number(result.componentFailed || 0) > 0) {
        toast.warning('备份已恢复，但有部分模块失败，请查看导入结果')
      } else {
        toast.success(resetFirst ? '已从 OpenList 清空后恢复' : '已从 OpenList 合并恢复')
      }
      onClose()
    } catch (restoreError) {
      const message = errorMessage(restoreError, 'OpenList 备份恢复失败')
      setError(message)
      toast.error(message)
    } finally {
      setBusy('none')
      setOperationProgress(null)
    }
  }

  const handleEditConnection = () => {
    setError('')
    setTestResult(null)
    setStage('connect')
  }

  const handlePrepareBackup = () => {
    setError('')
    setOperationProgress(null)
    setStage('prepare')
  }

  const restoreBusy = busy === 'restore-reset' || busy === 'restore-merge'
  const canClose = busy === 'none'
  const tableColumns: TableColumn<OpenListBackupFile>[] = [
    {
      key: 'name',
      title: '备份文件',
      render: value => <span className='min-w-0 break-all text-[var(--color-text-primary)]'>{value}</span>,
    },
    {
      key: 'size',
      title: '大小',
      width: 100,
      render: value => formatSize(Number(value) || 0),
    },
    {
      key: 'modifiedAt',
      title: '时间',
      width: 180,
      render: value => formatModifiedAt(String(value || '')),
    },
    {
      key: 'actions',
      title: '恢复',
      width: 190,
      align: 'right',
      render: (_, file) => (
        <div className='flex justify-end gap-1.5 whitespace-nowrap'>
          <Button size='sm' variant='secondary' onClick={() => { void handleRestore(file, false) }} disabled={busy !== 'none'}>
            合并
          </Button>
          <Button size='sm' variant='danger' onClick={() => { void handleRestore(file, true) }} disabled={busy !== 'none'}>
            清空恢复
          </Button>
        </div>
      ),
    },
  ]

  const modalTitle = stage === 'list'
    ? 'OpenList 备份历史'
    : mode === 'backup' ? '备份到 OpenList' : 'OpenList 备份历史'
  const modalWidth = stage === 'list' ? '820px' : '520px'

  return (
    <Modal
      open={open}
      onClose={() => {
        if (canClose) onClose()
      }}
      title={modalTitle}
      width={modalWidth}
      closable={canClose}
      footer={stage === 'connect' ? (
        <div className='flex w-full flex-wrap justify-end gap-2'>
          <Button variant='secondary' onClick={onClose} disabled={!canClose}>取消</Button>
          <Button variant='secondary' onClick={() => { void handleTestConnection() }} loading={busy === 'test'} disabled={busy !== 'none'}>
            <Settings2 className='h-4 w-4' />
            测试连接
          </Button>
          <Button onClick={() => { void handleProceed() }} loading={busy === 'connect'} disabled={busy !== 'none'}>
            <Settings2 className='h-4 w-4' />
            {mode === 'backup' ? '下一步' : '读取历史'}
          </Button>
        </div>
      ) : stage === 'prepare' ? (
        <div className='flex w-full flex-wrap justify-end gap-2'>
          <Button variant='secondary' onClick={handleEditConnection} disabled={!canClose}>
            修改连接
          </Button>
          <Button onClick={() => { void handleUpload() }} loading={busy === 'upload'} disabled={busy !== 'none'}>
            <Upload className='h-4 w-4' />
            开始备份
          </Button>
        </div>
      ) : (
        <div className='flex w-full flex-wrap justify-end gap-2'>
          <Button variant='secondary' onClick={handleEditConnection} disabled={!canClose}>
            修改连接
          </Button>
          {mode === 'backup' && (
            <Button onClick={handlePrepareBackup} disabled={busy !== 'none'}>
              <Upload className='h-4 w-4' />
              再次备份
            </Button>
          )}
        </div>
      )}
    >
      {stage === 'connect' ? (
        <div
          className='space-y-4'
          onKeyDown={event => {
            if (event.key !== 'Enter' || busy !== 'none') return
            event.preventDefault()
            void handleProceed()
          }}
        >
          <FormItem label='WebDAV 地址' required hint='填写 OpenList 的 WebDAV 地址，例如 http://127.0.0.1:5244/dav' error={fieldErrors.baseURL}>
            <Input
              value={connection.baseURL}
              onChange={event => updateConnection('baseURL', event.target.value)}
              placeholder='http://127.0.0.1:5244/dav'
              type='url'
              inputMode='url'
              autoComplete='url'
              error={Boolean(fieldErrors.baseURL)}
              autoFocus
            />
          </FormItem>
          <FormItem label='远程目录' hint='留空表示使用 WebDAV 根目录' error={fieldErrors.remotePath}>
            <Input
              value={connection.remotePath}
              onChange={event => updateConnection('remotePath', event.target.value)}
              placeholder='ant-chrome/backups'
              autoComplete='off'
              error={Boolean(fieldErrors.remotePath)}
            />
          </FormItem>
          <FormItem label='用户名' error={fieldErrors.username}>
            <Input
              value={connection.username}
              onChange={event => updateConnection('username', event.target.value)}
              autoComplete='username'
              error={Boolean(fieldErrors.username)}
            />
          </FormItem>
          <FormItem label='密码' required={Boolean(connected.username)} hint='用户名和密码都留空时使用匿名 WebDAV；密码只在本次应用运行期间使用，不保存到本地配置' error={fieldErrors.password}>
            <Input
              type='password'
              value={connection.password}
              onChange={event => updateConnection('password', event.target.value)}
            autoComplete='current-password'
              error={Boolean(fieldErrors.password)}
            />
          </FormItem>
          {testResult && (
            <div
              role={testResult.status === 'error' ? 'alert' : 'status'}
              className={testResult.status === 'error'
                ? 'flex items-start gap-2 text-sm text-[var(--color-error)]'
                : 'flex items-start gap-2 text-sm text-[var(--color-success)]'}
            >
              {testResult.status === 'error'
                ? <XCircle className='mt-0.5 h-4 w-4 shrink-0' />
                : <CheckCircle2 className='mt-0.5 h-4 w-4 shrink-0' />}
              <span className='min-w-0 break-words'>{testResult.message}</span>
            </div>
          )}
          {error && <p role='alert' className='text-sm text-[var(--color-error)]'>{error}</p>}
        </div>
      ) : stage === 'prepare' ? (
        <div className='space-y-4'>
          <div className='rounded-md border border-[var(--color-border-default)] bg-[var(--color-bg-secondary)] px-3 py-2'>
            <div className='text-xs text-[var(--color-text-muted)]'>备份目标</div>
            <div className='mt-1 break-all text-sm text-[var(--color-text-primary)]'>
              {connected.baseURL}{connected.remotePath ? `/${connected.remotePath}` : ''}
            </div>
          </div>
          <div className='flex gap-2 rounded-md border border-[var(--color-warning)]/40 bg-[var(--color-warning)]/10 px-3 py-2 text-xs text-[var(--color-warning)]'>
            <AlertTriangle className='mt-0.5 h-4 w-4 shrink-0' />
            <span className='leading-5'>开始备份前请停止所有实例，运行中的实例会被后端拒绝导出。</span>
          </div>
          {operationProgress && busy === 'upload' && (
            <div className='space-y-1.5' role='status' aria-live='polite'>
              <div className='flex items-center justify-between gap-3 text-xs text-[var(--color-text-secondary)]'>
                <span className='min-w-0 break-words'>{operationProgress.message}</span>
                <span className='shrink-0'>{operationProgress.progress}%</span>
              </div>
              <Progress
                percent={operationProgress.progress}
                size='sm'
                status={operationProgress.phase === 'error' ? 'error' : operationProgress.phase === 'done' ? 'success' : 'normal'}
                showInfo={false}
              />
            </div>
          )}
          {error && <p role='alert' className='text-sm text-[var(--color-error)]'>{error}</p>}
        </div>
      ) : (
        <div className='space-y-3'>
          <div className='flex items-center justify-between gap-3 text-xs text-[var(--color-text-muted)]'>
            <span className='min-w-0 truncate' title={`${connected.baseURL}${connected.remotePath ? `/${connected.remotePath}` : ''}`}>
              {connected.baseURL}{connected.remotePath ? `/${connected.remotePath}` : ''}
            </span>
            <Button size='sm' variant='secondary' onClick={handleRefresh} loading={busy === 'list'} disabled={busy !== 'none'}>
              <RefreshCw className='h-4 w-4' />
              刷新
            </Button>
          </div>
          {error && <p role='alert' className='text-sm text-[var(--color-error)]'>{error}</p>}
          {restoreBusy && <p className='text-sm text-[var(--color-warning)]'>正在下载并校验备份，当前弹窗不可关闭。</p>}
          {busy === 'upload' && <p className='text-sm text-[var(--color-warning)]'>正在生成并上传全量备份，实例必须保持停止。</p>}
          {operationProgress && (busy === 'upload' || restoreBusy) && (
            <div className='space-y-1.5' role='status' aria-live='polite'>
              <div className='flex items-center justify-between gap-3 text-xs text-[var(--color-text-secondary)]'>
                <span className='min-w-0 break-words'>{operationProgress.message}</span>
                <span className='shrink-0'>{operationProgress.progress}%</span>
              </div>
              <Progress
                percent={operationProgress.progress}
                size='sm'
                status={operationProgress.phase === 'error' ? 'error' : operationProgress.phase === 'done' ? 'success' : 'normal'}
                showInfo={false}
              />
            </div>
          )}
          <Table
            columns={tableColumns}
            data={files}
            rowKey='name'
            loading={busy === 'list' || busy === 'upload'}
            emptyText='暂无 OpenList ZIP 备份'
            className='min-w-[680px]'
            maxHeight='none'
          />
        </div>
      )}
    </Modal>
  )
}
