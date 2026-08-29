import { useEffect, useMemo, useState } from 'react'
import { ArrowLeft, CheckCircle2, Database, Plug, Save, XCircle } from 'lucide-react'
import { useNavigate } from 'react-router-dom'

import { Button, Card, FormItem, Input, Switch, toast } from '../../../../shared/components'
import {
  defaultS3Settings,
  fetchS3Settings,
  saveS3Settings,
  testS3Connection,
} from './api'
import type { S3Draft, S3Settings } from './api'

type S3Field = keyof S3Draft
type S3FieldErrors = Partial<Record<S3Field, string>>
type S3TestResult = { status: 'success' | 'error'; message: string }
type S3Busy = 'none' | 'load' | 'test' | 'save'

const emptyDraft: S3Draft = {
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

function validateDraft(draft: S3Draft, settings: S3Settings): S3FieldErrors {
  const errors: S3FieldErrors = {}
  const endpoint = draft.endpoint.trim()
  if (endpoint) {
    try {
      const parsed = new URL(endpoint)
      if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
        errors.endpoint = '地址必须使用 http 或 https'
      } else if (!parsed.hostname) {
        errors.endpoint = '地址缺少主机名'
      } else if (parsed.username || parsed.password) {
        errors.endpoint = '地址不能包含用户信息'
      } else if (parsed.search || parsed.hash) {
        errors.endpoint = '地址不能包含查询参数或片段'
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

  if (!draft.accessKeyID.trim() && !settings.accessKeyIDConfigured) {
    errors.accessKeyID = '请输入 Access Key ID'
  }
  if (!draft.secretAccessKey.trim() && !settings.secretAccessKeyConfigured) {
    errors.secretAccessKey = '请输入 Secret Access Key'
  }

  return errors
}

export function S3ConfigPage() {
  const navigate = useNavigate()
  const [draft, setDraft] = useState<S3Draft>(emptyDraft)
  const [settings, setSettings] = useState<S3Settings>(defaultS3Settings)
  const [busy, setBusy] = useState<S3Busy>('load')
  const [fieldErrors, setFieldErrors] = useState<S3FieldErrors>({})
  const [error, setError] = useState('')
  const [testResult, setTestResult] = useState<S3TestResult | null>(null)

  useEffect(() => {
    let active = true
    void fetchS3Settings()
      .then(next => {
        if (!active) return
        setSettings(next)
        setDraft({
          endpoint: next.endpoint,
          region: next.region,
          bucket: next.bucket,
          prefix: next.prefix,
          forcePathStyle: next.forcePathStyle,
          accessKeyID: '',
          secretAccessKey: '',
          sessionToken: '',
        })
      })
      .catch(loadError => {
        if (active) setError(errorMessage(loadError, '读取 S3 配置失败'))
      })
      .finally(() => {
        if (active) setBusy('none')
      })
    return () => {
      active = false
    }
  }, [])

  const normalizedDraft = useMemo(() => ({
    ...draft,
    endpoint: draft.endpoint.trim(),
    region: draft.region.trim(),
    bucket: draft.bucket.trim(),
    prefix: draft.prefix.trim(),
    accessKeyID: draft.accessKeyID.trim(),
    secretAccessKey: draft.secretAccessKey.trim(),
    sessionToken: draft.sessionToken.trim(),
  }), [draft])

  const updateDraft = <K extends S3Field>(key: K, value: S3Draft[K]) => {
    setDraft(previous => ({ ...previous, [key]: value }))
    setFieldErrors(previous => ({ ...previous, [key]: undefined }))
    setError('')
    setTestResult(null)
  }

  const validate = () => {
    const nextErrors = validateDraft(normalizedDraft, settings)
    setFieldErrors(nextErrors)
    return !Object.values(nextErrors).some(Boolean)
  }

  const handleTest = async () => {
    if (!validate()) return
    setBusy('test')
    setError('')
    setTestResult(null)
    try {
      await testS3Connection(normalizedDraft)
      setTestResult({ status: 'success', message: 'S3 连接测试成功，Bucket 可访问' })
      toast.success('S3 连接测试成功')
    } catch (testError) {
      const message = errorMessage(testError, 'S3 连接测试失败')
      setTestResult({ status: 'error', message })
      toast.error(message)
    } finally {
      setBusy('none')
    }
  }

  const handleSave = async () => {
    if (!validate()) return
    setBusy('save')
    setError('')
    try {
      const next = await saveS3Settings(normalizedDraft)
      setSettings(next)
      toast.success('S3 配置已保存')
      navigate('/system/backup')
    } catch (saveError) {
      const message = errorMessage(saveError, 'S3 配置保存失败')
      setError(message)
      toast.error(message)
    } finally {
      setBusy('none')
    }
  }

  const loading = busy === 'load'
  const submitting = busy === 'test' || busy === 'save'

  return (
    <div className="mx-auto w-full max-w-3xl space-y-4 animate-fade-in">
      <div className="flex items-center justify-between gap-3">
        <Button variant="ghost" onClick={() => navigate('/system/backup')} disabled={submitting}>
          <ArrowLeft className="h-4 w-4" />
          返回备份
        </Button>
        <div className="flex items-center gap-2 text-sm font-medium text-[var(--color-text-primary)]">
          <Database className="h-4 w-4" />
          S3 配置
        </div>
      </div>

      <div className="rounded-lg border border-[var(--color-warning)]/40 bg-[var(--color-warning)]/10 px-3 py-2 text-sm text-[var(--color-text-secondary)]">
        当前仅保存 S3 配置；S3 备份上传与恢复尚未接入。
      </div>

      {loading ? (
        <Card>
          <div className="flex h-40 items-center justify-center text-sm text-[var(--color-text-muted)]">读取 S3 配置中...</div>
        </Card>
      ) : (
        <form
          className="space-y-4"
          onSubmit={event => {
            event.preventDefault()
            void handleSave()
          }}
        >
          <Card title="连接">
            <div className="grid gap-4 md:grid-cols-2">
              <FormItem label="Endpoint" hint="留空使用 AWS S3 默认地址" className="md:col-span-2" error={fieldErrors.endpoint}>
                <Input
                  value={draft.endpoint}
                  onChange={event => updateDraft('endpoint', event.target.value)}
                  placeholder="https://s3.example.com"
                  type="url"
                  inputMode="url"
                  autoComplete="url"
                  error={Boolean(fieldErrors.endpoint)}
                  autoFocus
                  disabled={submitting}
                />
              </FormItem>
              <FormItem label="Region" required error={fieldErrors.region}>
                <Input
                  value={draft.region}
                  onChange={event => updateDraft('region', event.target.value)}
                  placeholder="us-east-1"
                  autoComplete="off"
                  error={Boolean(fieldErrors.region)}
                  disabled={submitting}
                />
              </FormItem>
              <FormItem label="Bucket" required error={fieldErrors.bucket}>
                <Input
                  value={draft.bucket}
                  onChange={event => updateDraft('bucket', event.target.value)}
                  placeholder="ant-chrome-backups"
                  autoComplete="off"
                  error={Boolean(fieldErrors.bucket)}
                  disabled={submitting}
                />
              </FormItem>
              <FormItem label="对象前缀" hint="留空表示直接使用 Bucket 根路径" error={fieldErrors.prefix} className="md:col-span-2">
                <Input
                  value={draft.prefix}
                  onChange={event => updateDraft('prefix', event.target.value)}
                  placeholder="ant-chrome/backups"
                  autoComplete="off"
                  error={Boolean(fieldErrors.prefix)}
                  disabled={submitting}
                />
              </FormItem>
              <FormItem label="强制 Path Style" hint="MinIO 等兼容服务通常需要开启" className="md:col-span-2">
                <div className="flex items-center gap-3">
                  <Switch checked={draft.forcePathStyle} onChange={value => updateDraft('forcePathStyle', value)} disabled={submitting} aria-label="强制 Path Style" />
                  <span className="text-sm text-[var(--color-text-secondary)]">{draft.forcePathStyle ? '已开启' : '未开启'}</span>
                </div>
              </FormItem>
            </div>
          </Card>

          <Card title="访问凭据">
            <div className="grid gap-4 md:grid-cols-2">
              <FormItem label="Access Key ID" required={!settings.accessKeyIDConfigured} hint={settings.accessKeyIDConfigured ? '留空沿用已保存凭据' : undefined} error={fieldErrors.accessKeyID}>
                <Input
                  value={draft.accessKeyID}
                  onChange={event => updateDraft('accessKeyID', event.target.value)}
                  autoComplete="username"
                  error={Boolean(fieldErrors.accessKeyID)}
                  disabled={submitting}
                />
              </FormItem>
              <FormItem label="Secret Access Key" required={!settings.secretAccessKeyConfigured} hint={settings.secretAccessKeyConfigured ? '留空沿用已保存凭据' : undefined} error={fieldErrors.secretAccessKey}>
                <Input
                  type="password"
                  value={draft.secretAccessKey}
                  onChange={event => updateDraft('secretAccessKey', event.target.value)}
                  autoComplete="new-password"
                  error={Boolean(fieldErrors.secretAccessKey)}
                  disabled={submitting}
                />
              </FormItem>
              <FormItem label="Session Token" hint={settings.sessionTokenConfigured ? '留空清除临时会话 Token' : '可选，临时凭据需要填写'} className="md:col-span-2">
                <Input
                  type="password"
                  value={draft.sessionToken}
                  onChange={event => updateDraft('sessionToken', event.target.value)}
                  autoComplete="new-password"
                  disabled={submitting}
                />
              </FormItem>
            </div>
          </Card>

          {testResult && (
            <div
              role={testResult.status === 'error' ? 'alert' : 'status'}
              className={testResult.status === 'error'
                ? 'flex items-start gap-2 text-sm text-[var(--color-error)]'
                : 'flex items-start gap-2 text-sm text-[var(--color-success)]'}
            >
              {testResult.status === 'error'
                ? <XCircle className="mt-0.5 h-4 w-4 shrink-0" />
                : <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0" />}
              <span className="min-w-0 break-words">{testResult.message}</span>
            </div>
          )}
          {error && <p role="alert" className="text-sm text-[var(--color-error)]">{error}</p>}

          <div className="flex flex-wrap justify-end gap-2">
            <Button type="button" variant="secondary" onClick={() => { void handleTest() }} loading={busy === 'test'} disabled={submitting}>
              <Plug className="h-4 w-4" />
              测试连接
            </Button>
            <Button type="submit" loading={busy === 'save'} disabled={submitting}>
              <Save className="h-4 w-4" />
              保存配置
            </Button>
          </div>
        </form>
      )}
    </div>
  )
}
