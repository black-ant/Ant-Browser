import { AlertCircle, RefreshCw, ShieldCheck, Wrench } from 'lucide-react'
import { Alert, Badge, Button, Card } from '../../../shared/components'
import type { EnvironmentStatus } from '../types'

interface EnvironmentStatusCardProps {
  status: EnvironmentStatus
  checking: boolean
  repairing: boolean
  exporting: boolean
  diagnosticsPath: string
  diagnosticsError: string
  updateBlocking: boolean
  updateError: string
  onRetry: () => void
  onRepair: () => void
  onExport: () => void
}

function stateMeta(status: EnvironmentStatus, updateBlocking: boolean) {
  const hasWarnings = status.items.some((item) => item.severity === 'warning' || item.severity === 'error')

  if (updateBlocking) {
    return {
      title: '需要完成更新后才能进入应用',
      subtitle: '本地环境检查已经通过，但当前版本要求先完成运行时升级。',
      badgeVariant: 'warning' as const,
      badgeText: '需要更新',
      icon: ShieldCheck,
      panelClassName: 'border-amber-200/70 bg-amber-50/70',
    }
  }

  switch (status.state) {
    case 'pass':
      if (hasWarnings) {
        return {
          title: '运行环境已就绪，但还有附带提醒',
          subtitle: '登录和主界面可以继续进入，但建议顺手处理下面这些环境提醒，避免后续排障信息缺失。',
          badgeVariant: 'warning' as const,
          badgeText: '通过（有提醒）',
          icon: ShieldCheck,
          panelClassName: 'border-amber-200/70 bg-amber-50/70',
        }
      }
      return {
        title: '运行环境已就绪',
        subtitle: '可以继续进入 Maka Browser，后续如遇异常仍可在设置里重新检查。',
        badgeVariant: 'success' as const,
        badgeText: '已通过',
        icon: ShieldCheck,
        panelClassName: 'border-emerald-200/70 bg-emerald-50/70',
      }
    case 'repairable':
      return {
        title: '发现可自动修复的问题',
        subtitle: '当前环境还不能稳定工作，但可以先尝试自动修复，再重新校验。',
        badgeVariant: 'warning' as const,
        badgeText: '可修复',
        icon: Wrench,
        panelClassName: 'border-amber-200/70 bg-amber-50/70',
      }
    case 'blocked':
      return {
        title: '环境异常，暂时无法自动修复',
        subtitle: '建议先导出诊断包，再结合错误码排查网络、权限或目录问题。',
        badgeVariant: 'error' as const,
        badgeText: '已阻塞',
        icon: AlertCircle,
        panelClassName: 'border-rose-200/70 bg-rose-50/70',
      }
    default:
      return {
        title: '正在检查运行环境',
        subtitle: 'Maka Browser 正在核对运行时、目录可写性和当前版本状态。',
        badgeVariant: 'info' as const,
        badgeText: '检查中',
        icon: RefreshCw,
        panelClassName: 'border-sky-200/70 bg-sky-50/70',
      }
  }
}

export function EnvironmentStatusCard({
  status,
  checking,
  repairing,
  exporting,
  diagnosticsPath,
  diagnosticsError,
  updateBlocking,
  updateError,
  onRetry,
  onRepair,
  onExport,
}: EnvironmentStatusCardProps) {
  const meta = stateMeta(status, updateBlocking)
  const Icon = meta.icon
  const actionable = status.state === 'repairable' || status.state === 'blocked'

  return (
    <Card
      className={`overflow-visible border ${meta.panelClassName} shadow-[0_24px_60px_rgba(15,23,42,0.08)]`}
      padding="lg"
    >
      <div className="flex flex-col gap-6">
        <div className="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
          <div className="flex items-start gap-4">
            <div className="flex h-14 w-14 items-center justify-center rounded-2xl bg-white/80 text-[var(--color-text-primary)] shadow-sm">
              <Icon className={`h-6 w-6 ${checking || repairing ? 'animate-spin' : ''}`} />
            </div>
            <div className="space-y-2">
              <div className="flex flex-wrap items-center gap-2">
                <Badge variant={meta.badgeVariant} size="lg" dot>
                  {meta.badgeText}
                </Badge>
                {repairing ? <Badge variant="warning">修复中</Badge> : null}
                {checking ? <Badge variant="info">检查中</Badge> : null}
              </div>
              <div>
                <h2 className="text-2xl font-semibold tracking-tight text-[var(--color-text-primary)]">{meta.title}</h2>
                <p className="mt-2 max-w-2xl text-sm leading-6 text-[var(--color-text-secondary)]">{meta.subtitle}</p>
              </div>
            </div>
          </div>

          <div className="grid grid-cols-1 gap-2 sm:grid-cols-3">
            <Button onClick={onRetry} variant="secondary" disabled={checking || repairing}>
              重新检查
            </Button>
            <Button onClick={onRepair} disabled={checking || repairing || status.state === 'blocked' || updateBlocking}>
              自动修复
            </Button>
            <Button onClick={onExport} variant="ghost" loading={exporting}>
              导出诊断
            </Button>
          </div>
        </div>

        {updateError ? (
          <Alert
            type="warning"
            title="更新处理未完成"
            message={updateError}
            className="border-white/80 bg-white/80"
          />
        ) : null}

        {diagnosticsPath ? (
          <Alert
            type="info"
            title="诊断包已生成"
            message={`已导出到 ${diagnosticsPath}`}
            className="border-white/80 bg-white/80"
          />
        ) : null}

        {diagnosticsError ? (
          <Alert
            type="warning"
            title="诊断导出未完成"
            message={diagnosticsError}
            className="border-white/80 bg-white/80"
          />
        ) : null}

        <div className="grid gap-3">
          {status.items.length > 0 ? (
            status.items.map((item) => (
              <div
                key={`${item.code}-${item.message}`}
                className="rounded-2xl border border-white/80 bg-white/90 px-4 py-3 shadow-sm"
              >
                <div className="flex flex-wrap items-center gap-2">
                  <Badge variant={item.severity === 'error' ? 'error' : item.severity === 'warning' ? 'warning' : 'info'}>
                    {item.code || 'UNKNOWN'}
                  </Badge>
                  {item.repairable ? <Badge variant="success">可自动修复</Badge> : <Badge variant="default">需人工处理</Badge>}
                </div>
                <p className="mt-2 text-sm leading-6 text-[var(--color-text-secondary)]">{item.message}</p>
                {item.recommendedAction ? (
                  <p className="mt-2 text-xs leading-6 text-slate-500">
                    建议处理：{item.recommendedAction}
                  </p>
                ) : null}
                {Object.keys(item.details).length > 0 ? (
                  <div className="mt-3 rounded-xl border border-slate-200/80 bg-slate-50/80 px-3 py-2 text-xs text-slate-600">
                    {Object.entries(item.details).map(([key, value]) => (
                      <div key={`${item.code}-${key}`} className="flex flex-wrap gap-2 leading-6">
                        <span className="font-medium text-slate-700">{key}:</span>
                        <span className="break-all">{value}</span>
                      </div>
                    ))}
                  </div>
                ) : null}
              </div>
            ))
          ) : (
            <div className="rounded-2xl border border-white/80 bg-white/90 px-4 py-3 text-sm text-[var(--color-text-secondary)] shadow-sm">
              {actionable ? '当前没有可展示的细分项目。' : '当前环境检查没有返回额外错误项。'}
            </div>
          )}
        </div>
      </div>
    </Card>
  )
}
