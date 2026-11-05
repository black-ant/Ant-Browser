import type { LaunchServerInfo } from '../../../api'
import { Card } from '../../../../../shared/components'
import { LaunchServerStatusBlock } from '../../../components/LaunchServerStatusBlock'

interface LaunchStatusPanelProps {
  currentGroupLabel: string
  currentDocLabel: string
  launchBaseUrl: string
  launchServerReady: boolean
  apiAuth: LaunchServerInfo['apiAuth']
}

export function LaunchStatusPanel({
  currentGroupLabel,
  currentDocLabel,
  launchBaseUrl,
  launchServerReady,
  apiAuth,
}: LaunchStatusPanelProps) {
  return (
    <Card
      title="文档上下文"
      className="bg-[var(--color-bg-elevated)] shadow-[var(--shadow-sm)]"
    >
      <div className="space-y-3">
        <div className="flex flex-wrap items-center gap-2 text-sm text-[var(--color-text-secondary)]">
          <span>当前分组：<code>{currentGroupLabel}</code></span>
          <span>当前章节：<code>{currentDocLabel}</code></span>
        </div>

        <LaunchServerStatusBlock
          launchBaseUrl={launchBaseUrl}
          launchServerReady={launchServerReady}
          apiAuth={apiAuth}
        />
      </div>
    </Card>
  )
}
