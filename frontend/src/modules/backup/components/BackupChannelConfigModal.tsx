import { useState } from 'react'
import { Plug } from 'lucide-react'

import { Button, Modal, toast } from '../../../shared/components'
import { backupChannelDefinitions, type BackupChannelId } from '../channels'
import { selectLocalBackupDirectory } from '../api'
import { fetchOpenListSettings, testOpenListConnection } from '../channels/openlist/api'
import { fetchS3Settings, testS3Connection } from '../channels/s3/api'

interface BackupChannelConfigModalProps {
  open: boolean
  onClose: () => void
  onSelect: (channelId: BackupChannelId) => void
  localDirectory?: string
  onLocalConfigured?: (directory: string) => void
}

function errorMessage(error: unknown, fallback: string) {
  if (error && typeof error === 'object' && 'message' in error && typeof error.message === 'string') {
    return error.message
  }
  return fallback
}

export function BackupChannelConfigModal({
  open,
  onClose,
  onSelect,
  localDirectory = '',
  onLocalConfigured,
}: BackupChannelConfigModalProps) {
  const options = backupChannelDefinitions.filter(option => option.configurable)
  const [busyChannel, setBusyChannel] = useState<BackupChannelId | null>(null)

  const handleTest = async (channelId: BackupChannelId) => {
    if (busyChannel || channelId === 'local') return

    setBusyChannel(channelId)
    const label = channelId === 'openlist' ? 'OpenList' : 'S3'
    try {
      if (channelId === 'openlist') {
        const settings = await fetchOpenListSettings()
        if (!settings.baseURL.trim() || !settings.tokenConfigured) {
          throw new Error('请先保存完整 OpenList 配置')
        }
        await testOpenListConnection({
          baseURL: settings.baseURL,
          remotePath: settings.remotePath,
          token: '',
          uploadRateLimitMBps: String(settings.uploadRateLimitMBps),
        })
      } else if (channelId === 's3') {
        const settings = await fetchS3Settings()
        if (!settings.region.trim() || !settings.bucket.trim() || !settings.credentialsConfigured) {
          throw new Error('请先保存完整 S3 配置')
        }
        await testS3Connection({
          endpoint: settings.endpoint,
          region: settings.region,
          bucket: settings.bucket,
          prefix: settings.prefix,
          forcePathStyle: settings.forcePathStyle,
          accessKeyID: '',
          secretAccessKey: '',
          sessionToken: '',
        })
      }
      toast.success(`${label} 连接测试成功`)
    } catch (testError) {
      toast.error(errorMessage(testError, `${label} 连接测试失败`))
    } finally {
      setBusyChannel(null)
    }
  }

  const handleLocalConfigure = async () => {
    if (busyChannel) return

    setBusyChannel('local')
    try {
      const result = await selectLocalBackupDirectory()
      if (result.cancelled) return
      if (!result.localDirectory) {
        throw new Error('未返回本地备份目录')
      }
      onLocalConfigured?.(result.localDirectory)
      toast.success('本地备份目录已保存')
    } catch (localError) {
      toast.error(errorMessage(localError, '选择本地备份目录失败'))
    } finally {
      setBusyChannel(null)
    }
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="配置备份渠道"
      width="440px"
      closable={busyChannel === null}
      footer={<Button variant="secondary" onClick={onClose} disabled={busyChannel !== null}>取消</Button>}
    >
      <div className="space-y-2">
        {options.map(option => {
          const Icon = option.icon
          const isLocal = option.id === 'local'
          return (
            <div
              key={option.id}
              className="flex w-full items-center gap-3 rounded-lg border border-[var(--color-border-default)] px-3 py-3 text-left transition-colors hover:border-[var(--color-border-strong)] hover:bg-[var(--color-bg-muted)]"
            >
              <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-[var(--color-bg-muted)] text-[var(--color-text-secondary)]">
                <Icon className="h-4 w-4" />
              </span>
              <span className="min-w-0 flex-1">
                <span className="block text-sm font-medium text-[var(--color-text-primary)]">{option.label}</span>
                <span className="block text-xs text-[var(--color-text-muted)]">{option.description}</span>
                {isLocal && (
                  <span
                    className="block truncate text-xs text-[var(--color-text-muted)]"
                    title={localDirectory || undefined}
                  >
                    {localDirectory || '未配置本地备份目录'}
                  </span>
                )}
              </span>
              <div className="flex shrink-0 items-center gap-2">
                {!isLocal && (
                  <Button
                    type="button"
                    size="sm"
                    variant="secondary"
                    className="!border-[var(--color-border-strong)] !bg-[var(--color-bg-muted)] !text-[var(--color-text-primary)] hover:!bg-[var(--color-border-default)]"
                    onClick={() => { void handleTest(option.id) }}
                    loading={busyChannel === option.id}
                    disabled={busyChannel !== null}
                    title={`测试${option.label}连接`}
                  >
                    <Plug className="h-3.5 w-3.5" />
                    测试
                  </Button>
                )}
                <Button
                  type="button"
                  size="sm"
                  variant="primary"
                  className="!border-black !bg-black !text-white hover:!bg-black/80"
                  onClick={() => {
                    if (isLocal) {
                      void handleLocalConfigure()
                    } else {
                      onSelect(option.id)
                    }
                  }}
                  loading={isLocal && busyChannel === option.id}
                  disabled={busyChannel !== null}
                >
                  配置
                </Button>
              </div>
            </div>
          )
        })}
      </div>
    </Modal>
  )
}
