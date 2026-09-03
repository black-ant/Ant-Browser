import { useEffect, useMemo, useState } from 'react'
import { ArrowLeft, CheckCircle2, Database, Eye, EyeOff, Plug, Save, XCircle } from 'lucide-react'
import { useLocation, useNavigate } from 'react-router-dom'

import { Button, Card, FormItem, Input, Switch, toast } from '../../../../shared/components'
import {
  defaultS3Settings,
  fetchS3Settings,
  revealS3Credential,
  saveS3Settings,
  testS3Connection,
} from './api'
import type { S3CredentialField, S3Draft, S3Settings } from './api'
import type { BackupRouteState } from '../../flow'

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

function isCredentialConfigured(settings: S3Settings, field: S3CredentialField) {
  if (field === 'accessKeyID') return settings.accessKeyIDConfigured
  if (field === 'secretAccessKey') return settings.secretAccessKeyConfigured
  return settings.sessionTokenConfigured
}

export function S3ConfigPage() {
  const location = useLocation()
  const navigate = useNavigate()
  const [draft, setDraft] = useState<S3Draft>(emptyDraft)
  const [settings, setSettings] = useState<S3Settings>(defaultS3Settings)
  const [busy, setBusy] = useState<S3Busy>('load')
  const [fieldErrors, setFieldErrors] = useState<S3FieldErrors>({})
  const [error, setError] = useState('')
  const [testResult, setTestResult] = useState<S3TestResult | null>(null)
  const [visibleCredentials, setVisibleCredentials] = useState<Partial<Record<S3CredentialField, boolean>>>({})
  const [revealingCredential, setRevealingCredential] = useState<S3CredentialField | null>(null)

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

  const toggleCredentialVisibility = async (field: S3CredentialField) => {
    if (visibleCredentials[field]) {
      setVisibleCredentials(previous => ({ ...previous, [field]: false }))
      return
    }

    if (draft[field] || !isCredentialConfigured(settings, field)) {
      setVisibleCredentials(previous => ({ ...previous, [field]: true }))
      return
    }

    setRevealingCredential(field)
    setError('')
    try {
      const value = await revealS3Credential(field)
      setDraft(previous => ({ ...previous, [field]: value }))
      setVisibleCredentials(previous => ({ ...previous, [field]: true }))
    } catch (revealError) {
      const message = errorMessage(revealError, '显示 S3 凭据失败')
      setError(message)
      toast.error(message)
    } finally {
      setRevealingCredential(null)
    }
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
      const routeState = location.state as BackupRouteState | null
      const resume = routeState?.backupResume
      if (resume) {
        navigate('/system/backup', {
          state: {
            backupResume: {
              ...resume,
              s3Connection: {
                endpoint: next.endpoint,
                region: next.region,
                bucket: next.bucket,
                prefix: next.prefix,
                forcePathStyle: next.forcePathStyle,
              },
            },
          },
        })
      } else {
        navigate('/system/backup')
      }
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

  const credentialInput = (field: S3CredentialField, label: string, autoComplete: string, required: boolean, hint?: string) => (
    <FormItem label={label} required={required} hint={hint} error={fieldErrors[field]}>
      <div className='relative'>
        <Input
          type={visibleCredentials[field] ? 'text' : 'password'}
          value={draft[field]}
          placeholder={isCredentialConfigured(settings, field) ? '****' : undefined}
          onChange={event => updateDraft(field, event.target.value)}
          autoComplete={autoComplete}
          error={Boolean(fieldErrors[field])}
          className='pr-10'
          disabled={submitting}
        />
        <button
          type='button'
          className='absolute inset-y-0 right-0 inline-flex w-10 items-center justify-center text-[var(--color-text-muted)] transition-colors hover:text-[var(--color-text-primary)] disabled:cursor-not-allowed disabled:opacity-50'
          onClick={() => { void toggleCredentialVisibility(field) }}
          disabled={submitting || revealingCredential !== null}
          aria-label={visibleCredentials[field] ? '隐藏' + label : '显示' + label}
          title={visibleCredentials[field] ? '隐藏' + label : '显示' + label}
        >
          {visibleCredentials[field] ? <EyeOff className='h-4 w-4' /> : <Eye className='h-4 w-4' />}
        </button>
      </div>
    </FormItem>
  )

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
              {credentialInput('accessKeyID', 'Access Key ID', 'username', !settings.accessKeyIDConfigured, settings.accessKeyIDConfigured ? '留空沿用已保存凭据' : undefined)}
              {credentialInput('secretAccessKey', 'Secret Access Key', 'new-password', !settings.secretAccessKeyConfigured, settings.secretAccessKeyConfigured ? '留空沿用已保存凭据' : undefined)}
              <div className="md:col-span-2">
                {credentialInput('sessionToken', 'Session Token', 'new-password', false, settings.sessionTokenConfigured ? '留空清除临时会话 Token' : '可选，临时凭据需要填写')}
              </div>
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
