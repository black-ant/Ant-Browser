import { useEffect, useState } from 'react'
import { CheckCircle2, Clock3, XCircle } from 'lucide-react'

import { Button, FormItem, Input, Modal, Switch, toast } from '../../../shared/components'
import {
  defaultScheduledBackupSettings,
  fetchScheduledBackupSettings,
  saveScheduledBackupSettings,
} from '../schedule/api'
import type { ScheduledBackupDraft, ScheduledBackupSettings } from '../schedule/api'

interface ScheduledBackupModalProps {
  open: boolean
  onClose: () => void
  onSaved: (settings: ScheduledBackupSettings) => void
  onRequestOpenListConfig?: () => void
  onBusyChange?: (busy: boolean) => void
}

type ScheduledBackupField = 'dailyTime'
type ScheduledBackupErrors = Partial<Record<ScheduledBackupField, string>>

const emptyDraft: ScheduledBackupDraft = {
  enabled: false,
  dailyTime: '02:00',
}

function formatDate(value: string) {
  if (!value) return ''
  const timestamp = Date.parse(value)
  return Number.isFinite(timestamp)
    ? new Date(timestamp).toLocaleString('zh-CN', { hour12: false })
    : value
}

function validateDraft(draft: ScheduledBackupDraft): ScheduledBackupErrors {
  const errors: ScheduledBackupErrors = {}
  if (!/^([01]\d|2[0-3]):[0-5]\d$/.test(draft.dailyTime.trim())) {
    errors.dailyTime = '请输入有效的时间'
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

export function ScheduledBackupModal({ open, onClose, onSaved, onRequestOpenListConfig, onBusyChange }: ScheduledBackupModalProps) {
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
    const nextErrors = validateDraft(draft)
    if (Object.values(nextErrors).some(Boolean)) {
      setErrors(nextErrors)
      return
    }
    if (draft.enabled && !settings.tokenConfigured) {
      setError('请先配置 OpenList Token')
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

          {!settings.tokenConfigured && onRequestOpenListConfig && (
            <div className="flex items-center justify-between gap-3 rounded-lg border border-[var(--color-warning)]/40 bg-[var(--color-warning)]/10 px-3 py-2 text-sm">
              <span className="text-[var(--color-text-secondary)]">OpenList 尚未配置</span>
              <Button variant="secondary" size="sm" onClick={onRequestOpenListConfig} disabled={saving}>去配置</Button>
            </div>
          )}

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


          {error && <p role="alert" className="text-sm text-[var(--color-error)]">{error}</p>}
        </div>
      )}
    </Modal>
  )
}
