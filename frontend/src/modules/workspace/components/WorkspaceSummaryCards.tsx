import { Activity, AlertTriangle, MonitorUp, ShieldCheck } from 'lucide-react'
import { StatCard } from '../../../shared/components'
import type { WorkspaceDashboardStats } from '../types'

interface WorkspaceSummaryCardsProps {
  stats: WorkspaceDashboardStats
  loading?: boolean
}

function displayValue(loading: boolean | undefined, value: number) {
  return loading ? '-' : String(value)
}

export function WorkspaceSummaryCards({ stats, loading = false }: WorkspaceSummaryCardsProps) {
  return (
    <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
      <StatCard
        title="授权店铺"
        value={displayValue(loading, stats.totalAccounts)}
        icon={<ShieldCheck className="h-5 w-5 text-emerald-500" />}
      />
      <StatCard
        title="Ready 店铺"
        value={displayValue(loading, stats.readyShopCount)}
        icon={<Activity className="h-5 w-5 text-green-500" />}
      />
      <StatCard
        title="待处理凭据"
        value={displayValue(loading, stats.manualAttentionCount)}
        icon={<AlertTriangle className="h-5 w-5 text-amber-500" />}
      />
      <StatCard
        title="运行窗口"
        value={displayValue(loading, stats.runningInstanceCount)}
        icon={<MonitorUp className="h-5 w-5 text-sky-500" />}
      />
    </div>
  )
}
