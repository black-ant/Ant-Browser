import { useEffect, useMemo, useState } from 'react'

import { Button, FormItem, Input, Modal, Select, Switch, toast } from '../../../shared/components'
import {
  defaultOpenListSettings,
  fetchOpenListSettings,
  listOpenListBackups,
} from '../channels/openlist/api'
import type { OpenListBackupFile, OpenListConnection, OpenListDraft, OpenListSettings } from '../channels/openlist/api'
import {
  defaultS3Settings,
  fetchS3Settings,
  listS3Backups,
} from '../channels/s3/api'
import type { S3BackupFile, S3Connection, S3Draft, S3Settings } from '../channels/s3/api'

export type RemoteScanChannel = 'openlist' | 's3'
type ScanFieldErrors<T> = Partial<Record<keyof T, string>>

export interface BackupRemoteScanResult {
  channel: RemoteScanChannel
  connection: OpenListConnection | S3Connection
  files: OpenListBackupFile[] | S3BackupFile[]
}

interface BackupRemoteScanModalProps {
  open: boolean
  onClose: () => void
  initialOpenListConnection?: OpenListConnection | null
  initialS3Connection?: S3Connection | null
  onScanned: (result: BackupRemoteScanResult) => void
}

const emptyOpenListDraft: OpenListDraft = {
  baseURL: '',
  remotePath: defaultOpenListSettings.remotePath,
  token: '',
  uploadRateLimitMBps: String(defaultOpenListSettings.uploadRateLimitMBps),
}

const emptyS3Draft: S3Draft = {
  endpoint: defaultS3Settings.endpoint,
  region: defaultS3Settings.region,
  bucket: defaultS3Settings.bucket,
  prefix: defaultS3Settings.prefix,
  forcePathStyle: defaultS3Settings.forcePathStyle,
  accessKeyID: '',
  secretAccessKey: '',
  sessionToken: '',
}

function errorMessage(error: unknown, fallback: string) {
  if (error && typeof error === 'object' && 'message' in error && typeof error.message === 'string') {
    return error.message
  }
  return fallback
}

function validateOpenListDraft(draft: OpenListDraft, tokenConfigured: boolean): ScanFieldErrors<OpenListDraft> {
  const errors: ScanFieldErrors<OpenListDraft> = {}
  const baseURL = draft.baseURL.trim()
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
  if (draft.remotePath.trim().split('/').some(segment => segment.trim() === '..')) {
    errors.remotePath = '远程目录不能包含 ..'
  }
  if (!draft.token.trim() && !tokenConfigured) {
    errors.token = '请输入 OpenList Token'
  }
  return errors
}

function validateS3Draft(draft: S3Draft, accessKeyConfigured: boolean, secretAccessKeyConfigured: boolean): ScanFieldErrors<S3Draft> {
  const errors: ScanFieldErrors<S3Draft> = {}
  const endpoint = draft.endpoint.trim()
  if (endpoint) {
    try {
      const parsed = new URL(endpoint)
      if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
        errors.endpoint = '地址必须使用 http 或 https'
      } else if (!parsed.hostname) {
        errors.endpoint = '地址缺少主机名'
      } else if (parsed.username || parsed.password || parsed.search || parsed.hash) {
        errors.endpoint = '地址不能包含用户信息、查询参数或片段'
      }
    } catch {
      errors.endpoint = '请输入有效的 S3 地址'
    }
  }
  if (!draft.region.trim()) {
    errors.region = '请输入区域'
  } else if (/\s/.test(draft.region.trim())) {
    errors.region = '区域不能包含空格'
  }
  const bucket = draft.bucket.trim()
  if (!bucket) {
    errors.bucket = '请输入 Bucket'
  } else if (/[\\/\s]/.test(bucket)) {
    errors.bucket = 'Bucket 不能包含斜杠或空格'
  }
  if (draft.prefix.trim().split('/').some(segment => segment.trim() === '..')) {
    errors.prefix = '对象前缀不能包含 ..'
  }
  if (!draft.accessKeyID.trim() && !accessKeyConfigured) {
    errors.accessKeyID = '请输入 Access Key ID'
  }
  if (!draft.secretAccessKey.trim() && !secretAccessKeyConfigured) {
    errors.secretAccessKey = '请输入 Secret Access Key'
  }
  return errors
}

export function BackupRemoteScanModal({
  open,
  onClose,
  initialOpenListConnection,
  initialS3Connection,
  onScanned,
}: BackupRemoteScanModalProps) {
  const [channel, setChannel] = useState<RemoteScanChannel>('openlist')
  const [openListDraft, setOpenListDraft] = useState<OpenListDraft>(emptyOpenListDraft)
  const [s3Draft, setS3Draft] = useState<S3Draft>(emptyS3Draft)
  const [openListErrors, setOpenListErrors] = useState<ScanFieldErrors<OpenListDraft>>({})
  const [s3Errors, setS3Errors] = useState<ScanFieldErrors<S3Draft>>({})
  const [error, setError] = useState('')
  const [busy, setBusy] = useState<'none' | 'load' | 'scan'>('none')
  const [openListTokenConfigured, setOpenListTokenConfigured] = useState(false)
  const [s3AccessKeyConfigured, setS3AccessKeyConfigured] = useState(false)
  const [s3SecretAccessKeyConfigured, setS3SecretAccessKeyConfigured] = useState(false)

  useEffect(() => {
    if (!open) return
    let active = true
    setBusy('load')
    setOpenListErrors({})
    setS3Errors({})
    setError('')
    void Promise.allSettled([fetchOpenListSettings(), fetchS3Settings()]).then(([openListResult, s3Result]) => {
      if (!active) return
      const savedOpenList: OpenListSettings | null = openListResult.status === 'fulfilled' ? openListResult.value : null
      const savedS3: S3Settings | null = s3Result.status === 'fulfilled' ? s3Result.value : null
      const nextOpenList = initialOpenListConnection
      const nextS3 = initialS3Connection
      const hasOpenList = Boolean(nextOpenList || savedOpenList?.tokenConfigured)
      const hasS3 = Boolean(nextS3 || savedS3?.credentialsConfigured)

      setChannel(hasOpenList || !hasS3 ? 'openlist' : 's3')
      setOpenListTokenConfigured(savedOpenList?.tokenConfigured === true)
      setS3AccessKeyConfigured(savedS3?.accessKeyIDConfigured === true)
      setS3SecretAccessKeyConfigured(savedS3?.secretAccessKeyConfigured === true)
      setOpenListDraft({
        ...emptyOpenListDraft,
        baseURL: nextOpenList?.baseURL ?? savedOpenList?.baseURL ?? '',
        remotePath: nextOpenList?.remotePath ?? savedOpenList?.remotePath ?? defaultOpenListSettings.remotePath,
        token: nextOpenList?.token ?? '',
      })
      setS3Draft({
        ...emptyS3Draft,
        endpoint: nextS3?.endpoint ?? savedS3?.endpoint ?? '',
        region: nextS3?.region ?? savedS3?.region ?? defaultS3Settings.region,
        bucket: nextS3?.bucket ?? savedS3?.bucket ?? '',
        prefix: nextS3?.prefix ?? savedS3?.prefix ?? '',
        forcePathStyle: nextS3?.forcePathStyle ?? savedS3?.forcePathStyle ?? defaultS3Settings.forcePathStyle,
        accessKeyID: nextS3?.accessKeyID ?? '',
        secretAccessKey: nextS3?.secretAccessKey ?? '',
        sessionToken: nextS3?.sessionToken ?? '',
      })
    }).finally(() => {
      if (active) setBusy('none')
    })
    return () => {
      active = false
    }
  }, [open])

  const normalizedOpenListDraft = useMemo(() => ({
    ...openListDraft,
    baseURL: openListDraft.baseURL.trim(),
    remotePath: openListDraft.remotePath.trim().replace(/\\/g, '/'),
    token: openListDraft.token.trim(),
  }), [openListDraft])

  const normalizedS3Draft = useMemo(() => ({
    ...s3Draft,
    endpoint: s3Draft.endpoint.trim(),
    region: s3Draft.region.trim(),
    bucket: s3Draft.bucket.trim(),
    prefix: s3Draft.prefix.trim(),
    accessKeyID: s3Draft.accessKeyID.trim(),
    secretAccessKey: s3Draft.secretAccessKey.trim(),
    sessionToken: s3Draft.sessionToken.trim(),
  }), [s3Draft])

  const updateOpenListDraft = (key: keyof OpenListDraft, value: string) => {
    setOpenListDraft(previous => ({ ...previous, [key]: value }))
    setOpenListErrors(previous => ({ ...previous, [key]: undefined }))
    setError('')
  }

  const updateS3Draft = <K extends keyof S3Draft>(key: K, value: S3Draft[K]) => {
    setS3Draft(previous => ({ ...previous, [key]: value }))
    setS3Errors(previous => ({ ...previous, [key]: undefined }))
    setError('')
  }

  const handleScan = async () => {
    if (channel === 'openlist') {
      const nextErrors = validateOpenListDraft(normalizedOpenListDraft, openListTokenConfigured || Boolean(normalizedOpenListDraft.token))
      setOpenListErrors(nextErrors)
      if (Object.values(nextErrors).some(Boolean)) return
    } else {
      const nextErrors = validateS3Draft(
        normalizedS3Draft,
        s3AccessKeyConfigured || Boolean(normalizedS3Draft.accessKeyID),
        s3SecretAccessKeyConfigured || Boolean(normalizedS3Draft.secretAccessKey),
      )
      setS3Errors(nextErrors)
      if (Object.values(nextErrors).some(Boolean)) return
    }

    setBusy('scan')
    setError('')
    try {
      if (channel === 'openlist') {
        const connection: OpenListConnection = {
          baseURL: normalizedOpenListDraft.baseURL,
          remotePath: normalizedOpenListDraft.remotePath,
          token: normalizedOpenListDraft.token,
        }
        const files = await listOpenListBackups(connection)
        onScanned({ channel, connection, files })
        toast.success(`OpenList 扫描完成，共 ${files.length} 个备份`)
      } else {
        const connection: S3Connection = {
          endpoint: normalizedS3Draft.endpoint,
          region: normalizedS3Draft.region,
          bucket: normalizedS3Draft.bucket,
          prefix: normalizedS3Draft.prefix,
          forcePathStyle: normalizedS3Draft.forcePathStyle,
          accessKeyID: normalizedS3Draft.accessKeyID,
          secretAccessKey: normalizedS3Draft.secretAccessKey,
          sessionToken: normalizedS3Draft.sessionToken,
        }
        const files = await listS3Backups(connection)
        onScanned({ channel, connection, files })
        toast.success(`S3 扫描完成，共 ${files.length} 个备份`)
      }
      onClose()
    } catch (scanError) {
      const message = errorMessage(scanError, `${channel === 'openlist' ? 'OpenList' : 'S3'} 扫描失败`)
      setError(message)
      toast.error(message)
    } finally {
      setBusy('none')
    }
  }

  const disabled = busy !== 'none'
  const openListTokenRequired = !openListTokenConfigured && !openListDraft.token.trim()
  const s3AccessKeyRequired = !s3AccessKeyConfigured && !s3Draft.accessKeyID.trim()
  const s3SecretAccessKeyRequired = !s3SecretAccessKeyConfigured && !s3Draft.secretAccessKey.trim()

  return (
    <Modal
      open={open}
      onClose={() => {
        if (!disabled) onClose()
      }}
      title="扫描远程备份"
      width="620px"
      closable={!disabled}
      footer={(
        <div className="flex w-full flex-wrap justify-end gap-2">
          <Button variant="secondary" onClick={onClose} disabled={disabled}>取消</Button>
          <Button onClick={() => { void handleScan() }} loading={busy === 'scan'} disabled={disabled}>开始扫描</Button>
        </div>
      )}
    >
      <form
        className="space-y-4"
        onSubmit={event => {
          event.preventDefault()
          void handleScan()
        }}
      >
        <FormItem label="渠道">
          <Select
            value={channel}
            onChange={event => {
              setChannel(event.target.value as RemoteScanChannel)
              setError('')
            }}
            options={[
              { value: 'openlist', label: 'OpenList' },
              { value: 's3', label: 'S3' },
            ]}
            disabled={disabled}
          />
        </FormItem>

        {channel === 'openlist' ? (
          <div className="space-y-4">
            <FormItem label="WebDAV 地址" required error={openListErrors.baseURL}>
              <Input
                value={openListDraft.baseURL}
                onChange={event => updateOpenListDraft('baseURL', event.target.value)}
                placeholder="http://127.0.0.1:5244/dav"
                type="url"
                inputMode="url"
                autoComplete="url"
                error={Boolean(openListErrors.baseURL)}
                disabled={disabled}
                autoFocus
              />
            </FormItem>
            <FormItem label="远程目录" error={openListErrors.remotePath}>
              <Input
                value={openListDraft.remotePath}
                onChange={event => updateOpenListDraft('remotePath', event.target.value)}
                placeholder="ant-chrome/backups"
                autoComplete="off"
                error={Boolean(openListErrors.remotePath)}
                disabled={disabled}
              />
            </FormItem>
            <FormItem label="Token" required={openListTokenRequired} hint={openListTokenConfigured ? '留空沿用已保存 Token' : undefined} error={openListErrors.token}>
              <Input
                type="password"
                value={openListDraft.token}
                onChange={event => updateOpenListDraft('token', event.target.value)}
                autoComplete="new-password"
                error={Boolean(openListErrors.token)}
                disabled={disabled}
              />
            </FormItem>
          </div>
        ) : (
          <div className="space-y-4">
            <div className="grid gap-4 md:grid-cols-2">
              <FormItem label="Endpoint" error={s3Errors.endpoint} className="md:col-span-2">
                <Input
                  value={s3Draft.endpoint}
                  onChange={event => updateS3Draft('endpoint', event.target.value)}
                  placeholder="https://s3.example.com"
                  type="url"
                  inputMode="url"
                  autoComplete="url"
                  disabled={disabled}
                  error={Boolean(s3Errors.endpoint)}
                />
              </FormItem>
              <FormItem label="Region" required error={s3Errors.region}>
                <Input
                  value={s3Draft.region}
                  onChange={event => updateS3Draft('region', event.target.value)}
                  placeholder="us-east-1"
                  autoComplete="off"
                  error={Boolean(s3Errors.region)}
                  disabled={disabled}
                />
              </FormItem>
              <FormItem label="Bucket" required error={s3Errors.bucket}>
                <Input
                  value={s3Draft.bucket}
                  onChange={event => updateS3Draft('bucket', event.target.value)}
                  placeholder="ant-chrome-backups"
                  autoComplete="off"
                  error={Boolean(s3Errors.bucket)}
                  disabled={disabled}
                />
              </FormItem>
              <FormItem label="对象前缀" error={s3Errors.prefix} className="md:col-span-2">
                <Input
                  value={s3Draft.prefix}
                  onChange={event => updateS3Draft('prefix', event.target.value)}
                  placeholder="ant-chrome/backups"
                  autoComplete="off"
                  error={Boolean(s3Errors.prefix)}
                  disabled={disabled}
                />
              </FormItem>
              <FormItem label="强制 Path Style" className="md:col-span-2">
                <div className="flex items-center gap-3">
                  <Switch checked={s3Draft.forcePathStyle} onChange={value => updateS3Draft('forcePathStyle', value)} disabled={disabled} aria-label="强制 Path Style" />
                  <span className="text-sm text-[var(--color-text-secondary)]">{s3Draft.forcePathStyle ? '已开启' : '未开启'}</span>
                </div>
              </FormItem>
            </div>
            <div className="grid gap-4 md:grid-cols-2">
              <FormItem label="Access Key ID" required={s3AccessKeyRequired} hint={s3AccessKeyConfigured ? '留空沿用已保存凭据' : undefined} error={s3Errors.accessKeyID}>
                <Input
                  value={s3Draft.accessKeyID}
                  onChange={event => updateS3Draft('accessKeyID', event.target.value)}
                  autoComplete="username"
                  error={Boolean(s3Errors.accessKeyID)}
                  disabled={disabled}
                />
              </FormItem>
              <FormItem label="Secret Access Key" required={s3SecretAccessKeyRequired} hint={s3SecretAccessKeyConfigured ? '留空沿用已保存凭据' : undefined} error={s3Errors.secretAccessKey}>
                <Input
                  type="password"
                  value={s3Draft.secretAccessKey}
                  onChange={event => updateS3Draft('secretAccessKey', event.target.value)}
                  autoComplete="new-password"
                  error={Boolean(s3Errors.secretAccessKey)}
                  disabled={disabled}
                />
              </FormItem>
              <FormItem label="Session Token" error={s3Errors.sessionToken} className="md:col-span-2">
                <Input
                  type="password"
                  value={s3Draft.sessionToken}
                  onChange={event => updateS3Draft('sessionToken', event.target.value)}
                  autoComplete="new-password"
                  disabled={disabled}
                />
              </FormItem>
            </div>
          </div>
        )}
        {error && <p role="alert" className="text-sm text-[var(--color-error)]">{error}</p>}
      </form>
    </Modal>
  )
}
