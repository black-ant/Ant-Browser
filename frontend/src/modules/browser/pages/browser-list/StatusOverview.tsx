import { Activity, AlertCircle, Loader2, Server, WifiOff } from 'lucide-react'

interface StatusOverviewProps {
  total: number
  running: number
  starting: number
  error: number
  proxyError: number
}

export function StatusOverview({ total, running, starting, error, proxyError }: StatusOverviewProps) {
  const stats = [
    {
      label: '总窗口',
      value: total,
      icon: Server,
      color: 'text-[var(--color-text-secondary)]',
      bgColor: 'bg-[var(--color-bg-muted)]',
    },
    {
      label: '运行中',
      value: running,
      icon: Activity,
      color: 'text-[var(--color-success)]',
      bgColor: 'bg-[var(--color-success)]/10',
    },
    {
      label: '启动中',
      value: starting,
      icon: Loader2,
      color: 'text-[var(--color-info)]',
      bgColor: 'bg-[var(--color-info)]/10',
      spin: true,
    },
    {
      label: '异常',
      value: error,
      icon: AlertCircle,
      color: 'text-[var(--color-error)]',
      bgColor: 'bg-[var(--color-error)]/10',
    },
    {
      label: '代理异常',
      value: proxyError,
      icon: WifiOff,
      color: 'text-[var(--color-warning)]',
      bgColor: 'bg-[var(--color-warning)]/10',
    },
  ]

  return (
    <div className="-mx-1 overflow-x-auto px-1 pb-1">
      <div className="grid min-w-[780px] grid-cols-5 gap-3">
        {stats.map((stat) => {
          const Icon = stat.icon
          return (
            <div
              key={stat.label}
              className="flex h-11 items-center justify-center gap-2 rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-3 whitespace-nowrap transition-colors hover:border-[var(--color-border-strong)]"
            >
              <span className={`flex h-7 w-7 flex-shrink-0 items-center justify-center rounded-lg ${stat.bgColor}`}>
                <Icon className={`h-4 w-4 ${stat.color} ${stat.spin ? 'animate-spin' : ''}`} />
              </span>
              <span className="text-xs text-[var(--color-text-muted)]">{stat.label}</span>
              <span className="text-lg font-semibold leading-none text-[var(--color-text-primary)]">{stat.value}</span>
            </div>
          )
        })}
      </div>
    </div>
  )
}
