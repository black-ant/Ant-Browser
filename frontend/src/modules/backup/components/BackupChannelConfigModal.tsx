import { Modal, Button } from '../../../shared/components'
import { backupChannelDefinitions, type BackupChannelId } from '../channels'

interface BackupChannelConfigModalProps {
  open: boolean
  onClose: () => void
  onSelect: (channelId: BackupChannelId) => void
}

export function BackupChannelConfigModal({ open, onClose, onSelect }: BackupChannelConfigModalProps) {
  const options = backupChannelDefinitions.filter(option => option.configurable)

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="配置备份渠道"
      width="440px"
      footer={<Button variant="secondary" onClick={onClose}>取消</Button>}
    >
      <div className="space-y-2">
        {options.map(option => {
          const Icon = option.icon
          return (
            <button
              key={option.id}
              type="button"
              className="flex w-full items-center gap-3 rounded-lg border border-[var(--color-border-default)] px-3 py-3 text-left transition-colors hover:border-[var(--color-border-strong)] hover:bg-[var(--color-bg-muted)]"
              onClick={() => onSelect(option.id)}
            >
              <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-[var(--color-bg-muted)] text-[var(--color-text-secondary)]">
                <Icon className="h-4 w-4" />
              </span>
              <span className="min-w-0 flex-1">
                <span className="block text-sm font-medium text-[var(--color-text-primary)]">{option.label}</span>
                <span className="block text-xs text-[var(--color-text-muted)]">{option.description}</span>
              </span>
              <span className="shrink-0 text-xs text-[var(--color-text-muted)]">配置</span>
            </button>
          )
        })}
      </div>
    </Modal>
  )
}
