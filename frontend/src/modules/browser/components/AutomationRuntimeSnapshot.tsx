import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { RefreshCw, Settings2 } from 'lucide-react'
import { Badge, Button, Card, Progress, toast } from '../../../shared/components'
import { EventsOn } from '../../../wailsjs/runtime/runtime'
import { defaultAutomationState, fetchAutomationState, type AutomationState } from '../../settings/api'
import {
  getAutomationNodeSource,
  getAutomationNodeSourceLabel,
  getAutomationNodeVersion,
  getAutomationPlaywrightVersion,
  getAutomationRuntimeBadgeText,
  getAutomationRuntimeBadgeVariant,
  getAutomationSystemNodePath,
} from '../automationRuntime'

interface AutomationRuntimeProgress {
  phase: string
  progress: number
  message: string
  component?: string
}

interface AutomationRuntimeSnapshotProps {
  title?: string
  className?: string
  showSettingsAction?: boolean
}

function normalizeRuntimeProgress(payload: unknown): AutomationRuntimeProgress | null {
  if (!payload || typeof payload !== 'object') {
    return null
  }

  const data = payload as Partial<AutomationRuntimeProgress>
  return {
    phase: typeof data.phase === 'string' ? data.phase : 'checking',
    progress: Number.isFinite(data.progress) ? Math.max(0, Math.min(100, Math.round(Number(data.progress)))) : 0,
    message: typeof data.message === 'string' && data.message.trim()
      ? data.message.trim()
      : '正在准备自动化运行时...',
    component: typeof data.component === 'string' && data.component.trim()
      ? data.component.trim()
      : undefined,
  }
}

export function AutomationRuntimeSnapshot({
  title = '自动化运行时',
  className,
  showSettingsAction = true,
}: AutomationRuntimeSnapshotProps) {
  const navigate = useNavigate()
  const [automationState, setAutomationState] = useState<AutomationState>(defaultAutomationState)
  const [runtimeProgress, setRuntimeProgress] = useState<AutomationRuntimeProgress | null>(null)
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)

  useEffect(() => {
    let disposed = false

    const loadState = async (showError: boolean) => {
      try {
        const nextState = await fetchAutomationState()
        if (!disposed) {
          setAutomationState(nextState)
        }
      } catch (error: any) {
        if (showError) {
          toast.error(error?.message || '自动化状态刷新失败')
        }
      } finally {
        if (!disposed) {
          setLoading(false)
          setRefreshing(false)
        }
      }
    }

    void loadState(false)

    const offRuntimeProgress = EventsOn('automation:runtime:progress', (payload: unknown) => {
      const nextProgress = normalizeRuntimeProgress(payload)
      if (!nextProgress) {
        return
      }

      setRuntimeProgress(nextProgress)

      if (nextProgress.phase === 'done' || nextProgress.phase === 'error') {
        void loadState(false)
      }
    })

    return () => {
      disposed = true
      offRuntimeProgress()
    }
  }, [])

  const handleRefresh = async () => {
    setRefreshing(true)
    try {
      const nextState = await fetchAutomationState()
      setAutomationState(nextState)
    } catch (error: any) {
      toast.error(error?.message || '自动化状态刷新失败')
    } finally {
      setLoading(false)
      setRefreshing(false)
    }
  }

  const handleGoSettings = () => {
    navigate('/settings')
  }

  const nodeSource = getAutomationNodeSource(automationState)
  const nodeSourceLabel = getAutomationNodeSourceLabel(nodeSource)
  const nodeVersion = getAutomationNodeVersion(automationState)
  const playwrightVersion = getAutomationPlaywrightVersion(automationState)
  const systemNodePath = getAutomationSystemNodePath(automationState)

  return (
    <Card
      title={title}
      className={className}
      actions={(
        <>
          <Button size="sm" variant="secondary" onClick={() => void handleRefresh()} loading={refreshing}>
            <RefreshCw className="w-3.5 h-3.5" />
            刷新
          </Button>
          {showSettingsAction && (
            <Button size="sm" variant="secondary" onClick={handleGoSettings}>
              <Settings2 className="w-3.5 h-3.5" />
              运行时设置
            </Button>
          )}
        </>
      )}
    >
      <div className="space-y-3">
        <div className="flex flex-wrap items-center gap-2">
          <Badge variant={getAutomationRuntimeBadgeVariant(automationState)} size="sm" dot>
            自动化支持 · {getAutomationRuntimeBadgeText(automationState)}
          </Badge>
          <Badge variant={automationState.status.ready ? 'success' : 'default'} size="sm">
            Node 来源 {nodeSourceLabel}
          </Badge>
          <Badge variant={automationState.status.ready ? 'success' : 'default'} size="sm">
            Node {nodeVersion}
          </Badge>
          <Badge variant={automationState.status.ready ? 'success' : 'default'} size="sm">
            playwright-core {playwrightVersion}
          </Badge>
          {loading && (
            <Badge variant="default" size="sm">
              正在同步
            </Badge>
          )}
        </div>

        {runtimeProgress && (
          <div className="rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-subtle)] px-4 py-3 space-y-2">
            <div className="flex items-center justify-between gap-4 text-xs">
              <span className="text-[var(--color-text-secondary)]">{runtimeProgress.message}</span>
              <span className="text-[var(--color-text-muted)]">
                {runtimeProgress.component ? `${runtimeProgress.component} · ` : ''}
                {runtimeProgress.phase}
              </span>
            </div>
            <Progress
              percent={runtimeProgress.progress}
              size="sm"
              status={runtimeProgress.phase === 'error' ? 'error' : runtimeProgress.phase === 'done' ? 'success' : 'normal'}
            />
          </div>
        )}

        <div className="rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-subtle)] px-4 py-3 space-y-2 text-xs">
          <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-[var(--color-text-secondary)]">
            <span>Runtime：<code>{automationState.settings.runtimeVersion}</code></span>
            <span>Node 来源：<code>{nodeSourceLabel}</code></span>
            <span>Node / playwright-core：<code>{nodeVersion}</code> / <code>{playwrightVersion}</code></span>
          </div>
          {automationState.status.nodePath && (
            <div className="text-[var(--color-text-muted)] break-all">
              Node 路径：<code>{automationState.status.nodePath}</code>
            </div>
          )}
          {systemNodePath && (
            <div className="text-[var(--color-text-muted)] break-all">
              系统 Node 路径：<code>{systemNodePath}</code>
            </div>
          )}
          {automationState.status.nodeResolution && (
            <div className="text-[var(--color-text-muted)] break-all">
              解析说明：{automationState.status.nodeResolution}
            </div>
          )}
          {automationState.status.runtimeDir && (
            <div className="text-[var(--color-text-muted)] break-all">
              运行时目录：<code>{automationState.status.runtimeDir}</code>
            </div>
          )}
          {automationState.status.systemNodeError && (
            <div className="text-[var(--color-warning)] break-all">
              系统 Node 异常：{automationState.status.systemNodeError}
            </div>
          )}
          {automationState.status.lastError && (
            <div className="text-[var(--color-error)] break-all">
              最近错误：{automationState.status.lastError}
            </div>
          )}
          {!automationState.settings.enabled && (
            <div className="text-[var(--color-text-muted)]">
              自动化尚未启用。打开开关后，首次真实使用时才会准备运行时。
            </div>
          )}
        </div>
      </div>
    </Card>
  )
}
