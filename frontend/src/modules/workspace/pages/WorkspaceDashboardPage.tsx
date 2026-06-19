import { useEffect, useMemo, useState } from 'react'
import { AlertTriangle, ArrowRight, CheckCircle2, HardDrive, Link2, ListChecks, RefreshCw, Store, WifiOff } from 'lucide-react'
import { Link } from 'react-router-dom'
import { Button, Card, toast } from '../../../shared/components'
import {
  deriveWorkspaceDashboardStats,
  fetchWorkspaceAuthorizedShops,
  fetchWorkspaceSummary,
} from '../api'
import {
  deriveExecutionBlockers,
  readyRate,
  workspaceHealthLabel,
  workspaceHealthTone,
} from '../businessPresentation'
import { WorkspaceSummaryCards } from '../components/WorkspaceSummaryCards'
import type { WorkspaceAuthorizedShop, WorkspaceSummary } from '../types'

export function WorkspaceDashboardPage() {
  const [summary, setSummary] = useState<WorkspaceSummary | null>(null)
  const [shops, setShops] = useState<WorkspaceAuthorizedShop[]>([])
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)

  const stats = useMemo(() => deriveWorkspaceDashboardStats(shops), [shops])
  const blockers = useMemo(() => summary ? deriveExecutionBlockers(summary, shops) : [], [summary, shops])
  const readyRateText = readyRate(stats.totalAccounts, stats.readyShopCount)

  async function load(silent = false) {
    if (silent) {
      setRefreshing(true)
    } else {
      setLoading(true)
    }

    try {
      const [nextSummary, nextShops] = await Promise.all([
        fetchWorkspaceSummary(),
        fetchWorkspaceAuthorizedShops(),
      ])
      setSummary(nextSummary)
      setShops(nextShops)
    } catch (error) {
      console.error('load workspace dashboard failed', error)
      toast.error('加载 Workspace 总览失败')
    } finally {
      setLoading(false)
      setRefreshing(false)
    }
  }

  useEffect(() => {
    void load()
  }, [])

  const derivedSummary = summary ?? {
    status: '',
    agentStatus: '',
    sessionReady: false,
    serverReachable: false,
    antRuntimeReachable: false,
    activeRunCount: 0,
    deviceId: '',
    deviceStatus: '',
  }

  return (
    <div className="space-y-6 animate-fade-in">
      <div className="flex items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">今日执行台</h1>
          <p className="mt-1 text-sm text-[var(--color-text-muted)]">先确认环境、凭据和店铺窗口状态，再进入当天的店铺动作。</p>
        </div>
        <Button variant="secondary" size="sm" onClick={() => void load(true)} loading={refreshing}>
          <RefreshCw className="h-4 w-4" />
          刷新
        </Button>
      </div>

      <WorkspaceSummaryCards stats={stats} loading={loading} />

      <div className="grid grid-cols-1 gap-4 lg:grid-cols-[1.4fr_1fr]">
        <Card
          title="设备与连接状态"
          subtitle="展示当前 Workspace Host 与 Maka Runtime 的整体可用性"
          actions={(
            <div className={`rounded-full border px-3 py-1 text-xs font-medium ${workspaceHealthTone(derivedSummary)}`}>
              {loading ? '加载中' : workspaceHealthLabel(derivedSummary)}
            </div>
          )}
        >
          <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
            <div className="rounded-xl border border-[var(--color-border-muted)] bg-[var(--color-bg-subtle)] p-4">
              <div className="mb-3 flex items-center gap-2 text-sm font-medium text-[var(--color-text-primary)]">
                <HardDrive className="h-4 w-4 text-[var(--color-text-secondary)]" />
                设备状态
              </div>
              <div className="space-y-2 text-sm">
                <div className="flex items-center justify-between">
                  <span className="text-[var(--color-text-muted)]">设备 ID</span>
                  <span className="max-w-[60%] truncate text-right text-[var(--color-text-primary)]">
                    {loading ? '-' : (derivedSummary.deviceId || '未上报')}
                  </span>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-[var(--color-text-muted)]">设备状态</span>
                  <span className="text-[var(--color-text-primary)]">{loading ? '-' : (derivedSummary.deviceStatus || 'unknown')}</span>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-[var(--color-text-muted)]">活跃任务</span>
                  <span className="text-[var(--color-text-primary)]">{loading ? '-' : derivedSummary.activeRunCount}</span>
                </div>
              </div>
            </div>

            <div className="rounded-xl border border-[var(--color-border-muted)] bg-[var(--color-bg-subtle)] p-4">
              <div className="mb-3 flex items-center gap-2 text-sm font-medium text-[var(--color-text-primary)]">
                <Link2 className="h-4 w-4 text-[var(--color-text-secondary)]" />
                连接状态
              </div>
              <div className="space-y-3 text-sm">
                <div className="flex items-center justify-between">
                  <span className="text-[var(--color-text-muted)]">Workspace Host</span>
                  <span className={`inline-flex items-center gap-1 ${derivedSummary.serverReachable ? 'text-emerald-600' : 'text-amber-600'}`}>
                    {derivedSummary.serverReachable ? <CheckCircle2 className="h-4 w-4" /> : <WifiOff className="h-4 w-4" />}
                    {loading ? '-' : (derivedSummary.serverReachable ? '已连接' : '不可达')}
                  </span>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-[var(--color-text-muted)]">Maka Runtime</span>
                  <span className={`inline-flex items-center gap-1 ${derivedSummary.antRuntimeReachable ? 'text-emerald-600' : 'text-amber-600'}`}>
                    {derivedSummary.antRuntimeReachable ? <CheckCircle2 className="h-4 w-4" /> : <WifiOff className="h-4 w-4" />}
                    {loading ? '-' : (derivedSummary.antRuntimeReachable ? '已连通' : '未连通')}
                  </span>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-[var(--color-text-muted)]">Session Ready</span>
                  <span className={`inline-flex items-center gap-1 ${derivedSummary.sessionReady ? 'text-emerald-600' : 'text-slate-500'}`}>
                    <CheckCircle2 className="h-4 w-4" />
                    {loading ? '-' : (derivedSummary.sessionReady ? 'ready' : 'not ready')}
                  </span>
                </div>
              </div>
            </div>
          </div>
        </Card>

        <Card title="业务摘要" subtitle="店铺可执行、当前卡点与当天动作入口。">
          <div className="space-y-4">
            <div className="rounded-xl border border-[var(--color-border-muted)] bg-[var(--color-bg-subtle)] p-4">
              <div className="mb-3 flex items-center gap-2 text-sm font-medium text-[var(--color-text-primary)]">
                <Store className="h-4 w-4 text-[var(--color-text-secondary)]" />
                店铺可执行概况
              </div>
              <div className="space-y-2 text-sm">
                <div className="flex items-center justify-between">
                  <span className="text-[var(--color-text-muted)]">授权店铺</span>
                  <span className="text-[var(--color-text-primary)]">{loading ? '-' : stats.totalAccounts}</span>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-[var(--color-text-muted)]">Ready 店铺</span>
                  <span className="text-[var(--color-text-primary)]">{loading ? '-' : stats.readyShopCount}</span>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-[var(--color-text-muted)]">Ready 占比</span>
                  <span className="text-[var(--color-text-primary)]">{loading ? '-' : readyRateText}</span>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-[var(--color-text-muted)]">待处理凭据</span>
                  <span className="text-[var(--color-text-primary)]">{loading ? '-' : stats.manualAttentionCount}</span>
                </div>
              </div>
            </div>

            <div className="rounded-xl border border-[var(--color-border-muted)] bg-[var(--color-bg-card)] p-4">
              <div className="mb-3 flex items-center gap-2 text-sm font-medium text-[var(--color-text-primary)]">
                <AlertTriangle className="h-4 w-4 text-amber-500" />
                当前卡点
              </div>
              {loading ? (
                <div className="text-sm text-[var(--color-text-muted)]">加载中</div>
              ) : blockers.length > 0 ? (
                <div className="space-y-3">
                  {blockers.slice(0, 3).map((blocker) => (
                    <div key={blocker.kind} className="min-w-0">
                      <div className="truncate text-sm font-medium text-[var(--color-text-primary)]">{blocker.title}</div>
                      <div className="mt-0.5 line-clamp-2 text-xs leading-5 text-[var(--color-text-muted)]">{blocker.message}</div>
                    </div>
                  ))}
                </div>
              ) : (
                <div className="text-sm text-emerald-600">暂无执行卡点</div>
              )}
            </div>

            <div className="rounded-xl border border-dashed border-[var(--color-border-default)] bg-[var(--color-bg-card)] p-4">
              <div className="mb-3 flex items-center gap-2 text-sm font-medium text-[var(--color-text-primary)]">
                <ListChecks className="h-4 w-4 text-[var(--color-text-secondary)]" />
                快捷入口
              </div>
              <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-1 xl:grid-cols-2">
                <Link
                  to="/shops"
                  className="group inline-flex min-w-0 items-center justify-between gap-2 rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-3 py-2 text-sm font-medium text-[var(--color-text-primary)] transition-all hover:border-[var(--color-border-strong)] hover:bg-[var(--color-bg-muted)]"
                >
                  <span className="truncate">店铺授权</span>
                  <ArrowRight className="h-4 w-4 shrink-0 transition-transform group-hover:translate-x-0.5" />
                </Link>
                <Link
                  to="/operations"
                  className="group inline-flex min-w-0 items-center justify-between gap-2 rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-3 py-2 text-sm font-medium text-[var(--color-text-primary)] transition-all hover:border-[var(--color-border-strong)] hover:bg-[var(--color-bg-muted)]"
                >
                  <span className="truncate">运营任务</span>
                  <ArrowRight className="h-4 w-4 shrink-0 transition-transform group-hover:translate-x-0.5" />
                </Link>
              </div>
            </div>
          </div>
        </Card>
      </div>
    </div>
  )
}
