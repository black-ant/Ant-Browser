import { Play, Trash2 } from 'lucide-react'
import { Badge, Button, Card } from '../../../../../shared/components'
import type { AutomationDemoActionKey, AutomationDemoSession } from '../../../demoSession'
import type { LaunchDocDemoConfig } from '../catalog'

interface LaunchDemoPanelProps {
  config: LaunchDocDemoConfig
  launchServerReady: boolean
  demoSession: AutomationDemoSession
  demoBusyAction: AutomationDemoActionKey
  demoBusy: boolean
  onHealth: () => void
  onCreate: () => void
  onLaunch: () => void
  onDelete: () => void
}

export function LaunchDemoPanel({
  config,
  launchServerReady,
  demoSession,
  demoBusyAction,
  demoBusy,
  onHealth,
  onCreate,
  onLaunch,
  onDelete,
}: LaunchDemoPanelProps) {
  return (
    <Card
      title={config.title}
      className="bg-[var(--color-bg-elevated)] shadow-[var(--shadow-sm)]"
    >
      <div className="space-y-4">
        <div className="flex flex-wrap items-center gap-2">
          <Badge variant={launchServerReady ? 'success' : 'warning'} size="sm" dot>
            Launch API {launchServerReady ? '已就绪' : '待就绪'}
          </Badge>
          <Badge variant={demoSession.profileId ? 'success' : 'default'} size="sm">
            Profile {demoSession.profileId || '-'}
          </Badge>
          <Badge variant={demoSession.launchCode ? 'info' : 'default'} size="sm">
            Launch Code {demoSession.launchCode || '-'}
          </Badge>
        </div>

        <div className="flex flex-wrap gap-2">
          {config.actionKeys.includes('health') && (
            <Button
              size="sm"
              onClick={onHealth}
              loading={demoBusyAction === 'health'}
              disabled={demoBusy}
            >
              <Play className="w-4 h-4" />
              运行健康检查
            </Button>
          )}
          {config.actionKeys.includes('create') && (
            <Button
              size="sm"
              onClick={onCreate}
              loading={demoBusyAction === 'create'}
              disabled={demoBusy}
            >
              <Play className="w-4 h-4" />
              创建演示实例
            </Button>
          )}
          {config.actionKeys.includes('launch') && (
            <Button
              size="sm"
              onClick={onLaunch}
              loading={demoBusyAction === 'launch'}
              disabled={demoBusy || !demoSession.launchCode}
            >
              <Play className="w-4 h-4" />
              按 Code 唤起
            </Button>
          )}
          <Button
            size="sm"
            variant="danger"
            onClick={onDelete}
            loading={demoBusyAction === 'delete'}
            disabled={demoBusy || !demoSession.profileId}
          >
            <Trash2 className="w-4 h-4" />
            清理演示实例
          </Button>
        </div>

        <div className="rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-subtle)] px-4 py-3 space-y-2 text-sm text-[var(--color-text-secondary)]">
          <div className="text-[var(--color-text-primary)] font-medium">当前文档联动状态</div>
          <div>文档节点：<code>{config.primaryDocLabel}</code></div>
          <div>最近动作：<code>{demoSession.lastAction || '-'}</code></div>
          <div>Profile ID：<code>{demoSession.profileId || '-'}</code></div>
          <div>Launch Code：<code>{demoSession.launchCode || '-'}</code></div>
          <div className="break-all">CDP URL：<code>{demoSession.cdpUrl || '-'}</code></div>
        </div>
      </div>
    </Card>
  )
}
