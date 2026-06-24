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
      label: '总实例',
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
    <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-5 gap-3">
      {stats.map((stat) => {
        const Icon = stat.icon
        return (
          <div
            key={stat.label}
            className="flex items-center gap-3 px-4 py-3 rounded-xl bg-[var(--color-bg-surface)] border border-[var(--color-border-default)] hover:border-[var(--color-border-strong)] transition-colors"
          >
            <div className={`w-10 h-10 rounded-lg ${stat.bgColor} flex items-center justify-center flex-shrink-0`}>
              <Icon className={`w-5 h-5 ${stat.color} ${stat.spin ? 'animate-spin' : ''}`} />
            </div>
            <div className="flex flex-col min-w-0">
              <span className="text-xs text-[var(--color-text-muted)] truncate">{stat.label}</span>
              <span className="text-xl font-semibold text-[var(--color-text-primary)]">{stat.value}</span>
            </div>
          </div>
        )
      })}
    </div>
  )
}
