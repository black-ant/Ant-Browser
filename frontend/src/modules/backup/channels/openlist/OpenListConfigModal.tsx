import { useEffect, useMemo, useState } from 'react'
import { CheckCircle2, Eye, EyeOff, Settings2, XCircle } from 'lucide-react'

import { Button, FormItem, Input, Modal, toast } from '../../../../shared/components'
import {
  defaultOpenListSettings,
  fetchOpenListSettings,
  revealOpenListToken,
  saveOpenListSettings,
  testOpenListConnection,
} from './api'
import type { OpenListConnection, OpenListDraft, OpenListSettings } from './api'

interface OpenListConfigModalProps {
  open: boolean
  onClose: () => void
  onConfigured?: (connection: OpenListConnection) => void
  onBusyChange?: (busy: boolean) => void
}

type OpenListField = keyof OpenListDraft
type OpenListFieldErrors = Partial<Record<OpenListField, string>>
type OpenListTestResult = { status: 'success' | 'error'; message: string }
type OpenListBusy = 'none' | 'load' | 'test' | 'save'

const emptyDraft: OpenListDraft = {
  baseURL: '',
  remotePath: defaultOpenListSettings.remotePath,
  token: '',
  uploadRateLimitMBps: String(defaultOpenListSettings.uploadRateLimitMBps),
}

function validateConnection(draft: OpenListDraft, tokenConfigured: boolean): OpenListFieldErrors {
	const errors: OpenListFieldErrors = {}
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

	const remotePath = draft.remotePath.trim().replace(/\\/g, '/')
  if (remotePath.split('/').some(segment => segment.trim() === '..')) {
    errors.remotePath = '远程目录不能包含 ..'
  }

	if (!draft.token.trim() && !tokenConfigured) {
		errors.token = '请输入 OpenList Token'
  }

  const uploadRateLimit = draft.uploadRateLimitMBps.trim()
  if (!/^\d+$/.test(uploadRateLimit)) {
    errors.uploadRateLimitMBps = '请输入非负整数，0 表示不限速'
  } else if (Number(uploadRateLimit) > 1048576) {
    errors.uploadRateLimitMBps = '上传限速不能超过 1048576 MB/s'
  }

  return errors
}

function errorMessage(error: unknown, fallback: string) {
  if (error && typeof error === 'object' && 'message' in error && typeof error.message === 'string') {
    return error.message
  }
  return fallback
}

export function OpenListConfigModal({ open, onClose, onConfigured, onBusyChange }: OpenListConfigModalProps) {
	const [draft, setDraft] = useState<OpenListDraft>(emptyDraft)
	const [settings, setSettings] = useState<OpenListSettings>(defaultOpenListSettings)
	const [busy, setBusy] = useState<OpenListBusy>('none')
	const [error, setError] = useState('')
  const [fieldErrors, setFieldErrors] = useState<OpenListFieldErrors>({})
	const [testResult, setTestResult] = useState<OpenListTestResult | null>(null)
  const [tokenVisible, setTokenVisible] = useState(false)
  const [revealingToken, setRevealingToken] = useState(false)

	useEffect(() => {
		if (!open) return
		let active = true
		setBusy('load')
		setError('')
		setFieldErrors({})
		setTestResult(null)
    setTokenVisible(false)
    setRevealingToken(false)
		void fetchOpenListSettings()
			.then(next => {
				if (!active) return
				setSettings(next)
				setDraft({
					baseURL: next.baseURL,
					remotePath: next.remotePath,
					token: '',
					uploadRateLimitMBps: String(next.uploadRateLimitMBps),
				})
			})
			.catch(loadError => {
				if (active) setError(errorMessage(loadError, '读取 OpenList 配置失败'))
			})
			.finally(() => {
				if (active) setBusy('none')
			})
		return () => {
			active = false
		}
	}, [open])

  useEffect(() => {
    onBusyChange?.(busy !== 'none')
    return () => onBusyChange?.(false)
  }, [busy, onBusyChange])

	const normalizedDraft = useMemo(() => ({
		...draft,
		baseURL: draft.baseURL.trim(),
		remotePath: draft.remotePath.trim(),
		token: draft.token.trim(),
		uploadRateLimitMBps: draft.uploadRateLimitMBps.trim(),
	}), [draft])

	const updateDraft = (key: OpenListField, value: string) => {
		setDraft(current => ({ ...current, [key]: value }))
    setError('')
    setFieldErrors({})
    setTestResult(null)
  }

	const validate = () => {
		const nextErrors = validateConnection(normalizedDraft, settings.tokenConfigured)
    if (Object.keys(nextErrors).length === 0) return true
    setFieldErrors(nextErrors)
    setError('')
    setTestResult(null)
    return false
	}

  const toggleTokenVisibility = async () => {
    if (tokenVisible) {
      setTokenVisible(false)
      return
    }

    if (draft.token || !settings.tokenConfigured) {
      setTokenVisible(true)
      return
    }

    setRevealingToken(true)
    setError('')
    try {
      const token = await revealOpenListToken()
      setDraft(current => ({ ...current, token }))
      setTokenVisible(true)
    } catch (revealError) {
      const message = errorMessage(revealError, '显示 OpenList Token 失败')
      setError(message)
      toast.error(message)
    } finally {
      setRevealingToken(false)
    }
  }

  const handleTestConnection = async () => {
    if (!validate()) return

    setBusy('test')
    setError('')
    setTestResult(null)
	try {
		await testOpenListConnection(normalizedDraft)
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

  const handleSave = async () => {
    if (!validate()) return

    setBusy('save')
    setError('')
    try {
      const next = await saveOpenListSettings(normalizedDraft)
      setSettings(next)
      onConfigured?.({ baseURL: next.baseURL, remotePath: next.remotePath })
      toast.success('OpenList 配置已保存')
      onClose()
    } catch (saveError) {
      const message = errorMessage(saveError, 'OpenList 配置保存失败')
      setError(message)
      toast.error(message)
    } finally {
      setBusy('none')
    }
  }

  const canClose = busy === 'none' && !revealingToken

  return (
    <Modal
      open={open}
      onClose={() => {
        if (canClose) onClose()
      }}
      title="OpenList 配置"
      width="520px"
      closable={canClose}
      footer={(
        <div className="flex w-full flex-wrap justify-end gap-2">
          <Button variant="secondary" onClick={onClose} disabled={!canClose}>取消</Button>
          <Button variant="secondary" onClick={() => { void handleTestConnection() }} loading={busy === 'test'} disabled={busy !== 'none'}>
            <Settings2 className="h-4 w-4" />
            测试连接
          </Button>
		<Button onClick={() => { void handleSave() }} loading={busy === 'save'} disabled={busy !== 'none'}>
            <Settings2 className="h-4 w-4" />
            保存配置
          </Button>
        </div>
      )}
    >
	  {busy === 'load' ? (
	    <div className="flex h-32 items-center justify-center text-sm text-[var(--color-text-muted)]">读取 OpenList 配置中...</div>
	  ) : (
	    <div
	      className="space-y-4"
	      onKeyDown={event => {
	        if (event.key !== 'Enter' || !canClose) return
	        event.preventDefault()
	        void handleSave()
	      }}
	    >
        <FormItem label="WebDAV 地址" required hint="填写 OpenList 的 WebDAV 地址，例如 http://127.0.0.1:5244/dav" error={fieldErrors.baseURL}>
          <Input
            value={draft.baseURL}
            onChange={event => updateDraft('baseURL', event.target.value)}
            placeholder="http://127.0.0.1:5244/dav"
            type="url"
            inputMode="url"
            autoComplete="url"
            error={Boolean(fieldErrors.baseURL)}
            autoFocus
          />
        </FormItem>
        <FormItem label="远程目录" hint="留空表示使用 WebDAV 根目录" error={fieldErrors.remotePath}>
          <Input
            value={draft.remotePath}
            onChange={event => updateDraft('remotePath', event.target.value)}
            placeholder="ant-chrome/backups"
            autoComplete="off"
            error={Boolean(fieldErrors.remotePath)}
          />
        </FormItem>
        <FormItem label="上传限速（MB/s）" hint="0 表示不限速" error={fieldErrors.uploadRateLimitMBps}>
          <Input
            type="number"
            min="0"
            max="1048576"
            step="1"
            value={draft.uploadRateLimitMBps}
            onChange={event => updateDraft('uploadRateLimitMBps', event.target.value)}
            inputMode="numeric"
            autoComplete="off"
            error={Boolean(fieldErrors.uploadRateLimitMBps)}
          />
        </FormItem>
        <FormItem label="Token" required={!settings.tokenConfigured} hint={settings.tokenConfigured ? '留空沿用已保存 Token' : '用于 OpenList 请求认证'} error={fieldErrors.token}>
          <div className="relative">
            <Input
              type={tokenVisible ? 'text' : 'password'}
              value={draft.token}
              placeholder={settings.tokenConfigured ? '****' : undefined}
              onChange={event => updateDraft('token', event.target.value)}
              autoComplete="new-password"
              error={Boolean(fieldErrors.token)}
              className="pr-10"
              disabled={!canClose}
            />
            <button
              type="button"
              className="absolute inset-y-0 right-0 inline-flex w-10 items-center justify-center text-[var(--color-text-muted)] transition-colors hover:text-[var(--color-text-primary)] disabled:cursor-not-allowed disabled:opacity-50"
              onClick={() => { void toggleTokenVisibility() }}
              disabled={!canClose}
              aria-label={tokenVisible ? '隐藏 OpenList Token' : '显示 OpenList Token'}
              title={tokenVisible ? '隐藏 OpenList Token' : '显示 OpenList Token'}
            >
              {tokenVisible ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
            </button>
          </div>
        </FormItem>
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
	    </div>
	  )}
    </Modal>
  )
}
