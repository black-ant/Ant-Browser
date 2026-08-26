import { useEffect, useState } from 'react'
import { CheckCircle2, Clock3, XCircle } from 'lucide-react'

import { Button, FormItem, Input, Modal, Switch, toast } from '../../../shared/components'
import {
  defaultScheduledBackupSettings,
  fetchScheduledBackupSettings,
  saveScheduledBackupSettings,
} from '../openListApi'
import type { ScheduledBackupDraft, ScheduledBackupSettings } from '../openListApi'

interface ScheduledBackupModalProps {
  open: boolean
  onClose: () => void
  onSaved: (settings: ScheduledBackupSettings) => void
  onBusyChange?: (busy: boolean) => void
}

type ScheduledBackupField = 'dailyTime' | 'baseURL' | 'remotePath' | 'username' | 'password'
type ScheduledBackupErrors = Partial<Record<ScheduledBackupField, string>>

const emptyDraft: ScheduledBackupDraft = {
  enabled: false,
  dailyTime: '02:00',
  baseURL: '',
  remotePath: 'ant-chrome/backups',
  username: '',
  password: '',
}

function formatDate(value: string) {
  if (!value) return ''
  const timestamp = Date.parse(value)
  return Number.isFinite(timestamp)
    ? new Date(timestamp).toLocaleString('zh-CN', { hour12: false })
    : value
}

function validateDraft(draft: ScheduledBackupDraft, passwordConfigured: boolean): ScheduledBackupErrors {
  const errors: ScheduledBackupErrors = {}
  if (!/^([01]\d|2[0-3]):[0-5]\d$/.test(draft.dailyTime.trim())) {
    errors.dailyTime = '请输入有效的时间'
  }
  if (!draft.enabled) return errors

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

  if (draft.remotePath.trim().replace(/\\/g, '/').split('/').some(segment => segment.trim() === '..')) {
    errors.remotePath = '远程目录不能包含 ..'
  }

  const username = draft.username.trim()
  if (username && !draft.password && !passwordConfigured) {
    errors.password = '请输入密码；应用重启后需要重新输入'
  } else if (!username && draft.password) {
    errors.username = '请输入用户名，或清空密码'
  }
  return errors
}

function statusLabel(settings: ScheduledBackupSettings) {
  switch (settings.status) {
    case 'running':
      return '执行中'
    case 'success':
      return settings.lastSuccessAt ? `上次成功：${formatDate(settings.lastSuccessAt)}` : '最近一次成功'
    case 'skipped':
      return '上次跳过：实例仍在运行'
    case 'failed':
      return settings.lastError ? `上次失败：${settings.lastError}` : '上次失败'
    default:
      return '尚未执行'
  }
}

export function ScheduledBackupModal({ open, onClose, onSaved, onBusyChange }: ScheduledBackupModalProps) {
  const [draft, setDraft] = useState<ScheduledBackupDraft>(emptyDraft)
  const [settings, setSettings] = useState<ScheduledBackupSettings>(defaultScheduledBackupSettings)
  const [errors, setErrors] = useState<ScheduledBackupErrors>({})
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (!open) return
    let active = true
    setLoading(true)
    setError('')
    setErrors({})
    void fetchScheduledBackupSettings()
      .then(next => {
        if (!active) return
        setSettings(next)
        setDraft({
          enabled: next.enabled,
          dailyTime: next.dailyTime,
          baseURL: next.baseURL,
          remotePath: next.remotePath,
          username: next.username,
          password: '',
        })
      })
      .catch(loadError => {
        if (active) setError(loadError?.message || '读取定时备份设置失败')
      })
      .finally(() => {
        if (active) setLoading(false)
      })
    return () => {
      active = false
    }
  }, [open])

  useEffect(() => {
    onBusyChange?.(loading || saving)
    return () => onBusyChange?.(false)
  }, [loading, saving, onBusyChange])

  const updateDraft = <K extends keyof ScheduledBackupDraft>(key: K, value: ScheduledBackupDraft[K]) => {
    setDraft(previous => ({ ...previous, [key]: value }))
    setErrors(previous => ({ ...previous, [key]: undefined }))
    setError('')
  }

  const handleSave = async () => {
    const nextErrors = validateDraft(draft, settings.passwordConfigured)
    if (Object.values(nextErrors).some(Boolean)) {
      setErrors(nextErrors)
      return
    }

    setSaving(true)
    setError('')
    try {
      const next = await saveScheduledBackupSettings(draft)
      setSettings(next)
      onSaved(next)
      toast.success(draft.enabled ? `已设置每日 ${next.dailyTime} 自动备份` : '已关闭定时备份')
      onClose()
    } catch (saveError: any) {
      setError(saveError?.message || '保存定时备份设置失败')
    } finally {
      setSaving(false)
    }
  }

  const busy = loading || saving

  return (
    <Modal
      open={open}
      onClose={() => {
        if (!busy) onClose()
      }}
      title="定时备份"
      width="560px"
      closable={!busy}
      footer={(
        <>
          {!busy && <Button variant="secondary" onClick={onClose}>取消</Button>}
          <Button onClick={() => { void handleSave() }} loading={saving} disabled={busy && !saving}>
            保存设置
          </Button>
        </>
      )}
    >
      {loading ? (
        <div className="flex h-32 items-center justify-center text-sm text-[var(--color-text-muted)]">读取设置中...</div>
      ) : (
        <div className="space-y-4">
          <FormItem label="自动备份">
            <div className="flex items-center gap-3">
              <Switch checked={draft.enabled} onChange={value => updateDraft('enabled', value)} disabled={saving} />
              <span className="text-sm text-[var(--color-text-secondary)]">{draft.enabled ? '已启用' : '未启用'}</span>
            </div>
          </FormItem>

          <div className="grid grid-cols-2 gap-3">
            <FormItem label="每日执行时间" required error={errors.dailyTime}>
              <Input
                type="time"
                value={draft.dailyTime}
                onChange={event => updateDraft('dailyTime', event.target.value)}
                error={Boolean(errors.dailyTime)}
                disabled={saving}
              />
            </FormItem>
            <FormItem label="最近状态">
              <div className="flex h-9 items-center gap-2 rounded-lg border border-[var(--color-border-default)] px-3 text-sm text-[var(--color-text-secondary)]">
                {settings.status === 'success' && <CheckCircle2 className="h-4 w-4 text-[var(--color-success)]" />}
                {settings.status === 'failed' && <XCircle className="h-4 w-4 text-[var(--color-error)]" />}
                {settings.status !== 'success' && settings.status !== 'failed' && <Clock3 className="h-4 w-4 text-[var(--color-text-muted)]" />}
                <span className="min-w-0 truncate" title={settings.lastError || statusLabel(settings)}>{statusLabel(settings)}</span>
              </div>
            </FormItem>
          </div>

          <FormItem label="WebDAV 地址" required={draft.enabled} error={errors.baseURL}>
            <Input
              value={draft.baseURL}
              onChange={event => updateDraft('baseURL', event.target.value)}
              placeholder="http://127.0.0.1:5244/dav"
              error={Boolean(errors.baseURL)}
              disabled={saving}
            />
          </FormItem>
          <FormItem label="远程目录" error={errors.remotePath}>
            <Input
              value={draft.remotePath}
              onChange={event => updateDraft('remotePath', event.target.value)}
              placeholder="ant-chrome/backups"
              error={Boolean(errors.remotePath)}
              disabled={saving}
            />
          </FormItem>
          <div className="grid grid-cols-2 gap-3">
            <FormItem label="用户名" error={errors.username}>
              <Input
                value={draft.username}
                onChange={event => updateDraft('username', event.target.value)}
                autoComplete="username"
                error={Boolean(errors.username)}
                disabled={saving}
              />
            </FormItem>
            <FormItem label="密码" hint="密码不会写入配置文件，只保存在本次运行期间。应用重启后需要重新输入。" error={errors.password}>
              <Input
                type="password"
                value={draft.password}
                onChange={event => updateDraft('password', event.target.value)}
                placeholder={settings.passwordConfigured ? '留空沿用当前运行密码' : ''}
                autoComplete="current-password"
                error={Boolean(errors.password)}
                disabled={saving}
              />
            </FormItem>
          </div>

          {error && <p role="alert" className="text-sm text-[var(--color-error)]">{error}</p>}
          {draft.enabled && !settings.passwordConfigured && draft.username.trim() && (
            <p className="text-xs text-[var(--color-warning)]">当前运行实例尚未保存 OpenList 密码；启用账号认证时请在密码框输入。</p>
          )}
        </div>
      )}
    </Modal>
  )
}
