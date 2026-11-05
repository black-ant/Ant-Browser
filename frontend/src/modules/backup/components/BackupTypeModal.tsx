import { useEffect, useState } from 'react'

import { Button, Modal, Switch } from '../../../shared/components'
import {
  backupChannelDefinitions,
  type BackupChannelId,
  type BackupChannelSelection,
  type BackupChannelStatus,
} from '../channels'

export type BackupTypeSelection = BackupChannelSelection

interface BackupTypeModalProps {
  open: boolean
  onClose: () => void
  onConfirm: (selection: BackupTypeSelection) => void
  initialSelection?: BackupTypeSelection
  channelStatus?: Partial<Record<BackupChannelId, BackupChannelStatus>>
}

const defaultSelection: BackupTypeSelection = { local: true, openlist: false }

function normalizeSelection(selection?: BackupTypeSelection): BackupTypeSelection {
  return backupChannelDefinitions.reduce<BackupTypeSelection>((result, option) => {
    result[option.id] = selection?.[option.id] === true
    return result
  }, { ...defaultSelection })
}

export function BackupTypeModal({
  open,
  onClose,
  onConfirm,
  initialSelection,
  channelStatus = {},
}: BackupTypeModalProps) {
  const [selection, setSelection] = useState<BackupTypeSelection>(defaultSelection)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!open) return
    setSelection(normalizeSelection(initialSelection))
    setError('')
  }, [initialSelection, open])

  const updateType = (type: BackupChannelId, checked: boolean) => {
    if (!checked && Object.values(selection).filter(Boolean).length <= 1) {
      setError('至少选择一种备份类型')
      return
    }
    setError('')
    setSelection(current => ({ ...current, [type]: checked }))
  }

  const selectedCount = Object.values(selection).filter(Boolean).length
  const needsConfiguration = backupChannelDefinitions.some(option => {
    const status = channelStatus[option.id]
    return selection[option.id] === true && status && !status.configured
  })

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="选择备份类型"
      width="460px"
      footer={(
        <>
          <Button variant="secondary" onClick={onClose}>取消</Button>
          <Button
            onClick={() => onConfirm(selection)}
            disabled={selectedCount === 0}
          >
            开始备份
          </Button>
        </>
      )}
    >
      <div className="space-y-2">
        {backupChannelDefinitions.map(option => {
          const selected = selection[option.id] === true
          const selectable = option.available
          const Icon = option.icon
          const status = channelStatus[option.id]

          return (
            <div
              key={option.id}
              className={`flex items-center gap-3 rounded-lg border px-3 py-3 transition-colors ${
                selected
                  ? 'border-[var(--color-accent)] bg-[var(--color-accent-muted)]'
                  : 'border-[var(--color-border-default)]'
              } ${!selectable ? 'opacity-60' : ''}`}
            >
              <button
                type="button"
                className="flex min-w-0 flex-1 items-center gap-3 text-left disabled:cursor-not-allowed"
                onClick={() => updateType(option.id, !selected)}
                disabled={!selectable}
                aria-pressed={selected}
                aria-label={`选择${option.label}`}
              >
                <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-[var(--color-bg-muted)] text-[var(--color-text-secondary)]">
                  <Icon className="h-4 w-4" />
                </span>
                <span className="min-w-0">
                  <span className="block text-sm font-medium text-[var(--color-text-primary)]">{option.label}</span>
                  <span className="block text-xs text-[var(--color-text-muted)]">{option.description}</span>
                  {status && (
                    <span className="block truncate text-xs text-[var(--color-text-muted)]">
                      {status.configured ? `已配置${status.summary ? `：${status.summary}` : ''}` : '未配置，选择后需要先配置'}
                    </span>
                  )}
                </span>
              </button>
              {selectable ? (
                <Switch
                  checked={selected}
                  onChange={checked => updateType(option.id, checked)}
                  aria-label={option.label}
                />
              ) : (
                <span className="shrink-0 text-xs text-[var(--color-text-muted)]">即将支持</span>
              )}
            </div>
          )
        })}
        {needsConfiguration && (
          <p role="alert" className="text-sm text-[var(--color-error)]">选中的备份渠道尚未配置，确认后会先进入配置。</p>
        )}
        {error && <p role="alert" className="text-sm text-[var(--color-error)]">{error}</p>}
      </div>
    </Modal>
  )
}
