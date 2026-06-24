import { useEffect, useRef, useState } from 'react'
import { Monitor, Play, Shield, Cpu, ExternalLink, Globe, FileText, Plus, Activity, CheckCircle, XCircle, AlertCircle, Settings, Zap, Download } from 'lucide-react'
import { Link, useNavigate } from 'react-router-dom'
import { AreaChart, Area, ResponsiveContainer, XAxis, YAxis, Tooltip, CartesianGrid } from 'recharts'
import { Card, Button, toast } from '../../shared/components'
import { fetchDashboardStats, redeemCDKey, redeemGithubStar, reloadConfig, fetchDashboardMetrics, fetchActivityLog, fetchRecentErrors } from './api'
import type { DashboardStats, DashboardMetrics, ActivityEntry } from './types'
import { BrowserOpenURL, EventsOn } from '../../wailsjs/runtime/runtime'
import { PROJECT_GITHUB_URL } from '../../config/links'

interface StatCardProps {
  title: string
  value: string | number
  icon: React.ReactNode
  color: string
  to?: string
}

function StatCard({ title, value, icon, color, to }: StatCardProps) {
  const content = (
    <>
      <div className="flex items-center justify-between mb-4">
        <span className="text-sm font-medium text-[var(--color-text-muted)]">{title}</span>
        <div className={`w-11 h-11 rounded-xl flex items-center justify-center ${color} shadow-lg group-hover:scale-110 transition-transform duration-300`}>{icon}</div>
      </div>
      <div className="text-3xl font-bold text-[var(--color-text-primary)] tracking-tight">{value}</div>
    </>
  )
  const className = "rounded-2xl border border-[var(--color-border-default)] bg-[var(--color-bg-card)] p-6 shadow-[var(--shadow-sm)] hover:shadow-[var(--shadow-md)] transition-all duration-300 hover:-translate-y-1 group"
  return to ? <Link to={to} className={`${className} cursor-pointer block`}>{content}</Link> : <div className={className}>{content}</div>
}

const MAX_ACTIVITY = 30
const MAX_ERRORS = 15

const ACTIVITY_ICON: Record<string, React.ReactNode> = {
  start: <Play className="w-3.5 h-3.5 text-green-600" />,
  stop: <XCircle className="w-3.5 h-3.5 text-gray-500" />,
  crash: <AlertCircle className="w-3.5 h-3.5 text-red-600" />,
  speedtest: <Zap className="w-3.5 h-3.5 text-yellow-600" />,
  import: <Globe className="w-3.5 h-3.5 text-blue-600" />,
  export: <Download className="w-3.5 h-3.5 text-blue-500" />,
  config: <Settings className="w-3.5 h-3.5 text-purple-600" />,
}

function timeAgo(unixSec: number) {
  const seconds = Math.floor(Date.now() / 1000 - unixSec)
  if (seconds < 60) return `${Math.max(seconds, 0)}秒前`
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}分钟前`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}小时前`
  return `${Math.floor(hours / 24)}天前`
}

export function DashboardPage() {
  const navigate = useNavigate()
  const [stats, setStats] = useState<DashboardStats>({
    totalInstances: 0, runningInstances: 0, proxyCount: 0, proxyAvailable: 0, coreCount: 0, memUsedMB: 0, maxProfileLimit: 100, appVersion: 'unknown',
  })
  const [loading, setLoading] = useState(true)
  const [cdKey, setCdKey] = useState('')
  const [redeeming, setRedeeming] = useState(false)
  const [metrics, setMetrics] = useState<DashboardMetrics>({ live: { timestamp: 0, cpuPercent: 0, memUsedMB: 0, memTotalMB: 0, appMemMB: 0, runningInstances: 0 }, history: [] })
  const [activity, setActivity] = useState<ActivityEntry[]>([])
  const [errors, setErrors] = useState<ActivityEntry[]>([])
  const mountedRef = useRef(true)

  useEffect(() => {
    mountedRef.current = true
    void load()
    void refreshDynamic()

    // 实时活动事件：增量更新活动与错误面板
    const off = EventsOn('dashboard:activity', (entry: ActivityEntry) => {
      if (!entry) return
      setActivity(prev => [entry, ...prev].slice(0, MAX_ACTIVITY))
      if (entry.level === 'error') setErrors(prev => [entry, ...prev].slice(0, MAX_ERRORS))
    })

    // 指标每 5s 刷新（后端真实采样）
    const timer = window.setInterval(() => {
      if (document.visibilityState !== 'visible') return
      void fetchDashboardMetrics().then(m => { if (mountedRef.current) setMetrics(m) })
    }, 5000)

    return () => {
      mountedRef.current = false
      window.clearInterval(timer)
      off?.()
    }
  }, [])

  const load = async () => {
    setLoading(true)
    try {
      await reloadConfig()
      const newStats = await fetchDashboardStats()
      if (mountedRef.current) setStats(newStats)
    } finally {
      if (mountedRef.current) setLoading(false)
    }
  }

  const refreshDynamic = async () => {
    const [m, a, e] = await Promise.all([fetchDashboardMetrics(), fetchActivityLog(), fetchRecentErrors()])
    if (!mountedRef.current) return
    setMetrics(m)
    setActivity(a.slice(0, MAX_ACTIVITY))
    setErrors(e.slice(0, MAX_ERRORS))
  }

  const handleRedeem = async () => {
    if (!cdKey.trim()) return
    setRedeeming(true)
    const result = await redeemCDKey(cdKey.trim())
    setRedeeming(false)
    if (result.success) {
      toast.success('兑换成功！此名额已到账')
      setCdKey('')
      load()
    } else {
      toast.error(result.message || '兑换失败')
    }
  }

  const handleClaimStarGift = async () => {
    setRedeeming(true)
    const starRes = await redeemGithubStar()
    setRedeeming(false)
    if (starRes.success) {
      toast.success('感谢您的支持！已额外赠送 50 个永久额度！')
      setCdKey('')
      load()
    } else {
      toast.error(starRes.message || '领取失败')
    }
  }

  const handleOpenGithubStarGift = async () => {
    BrowserOpenURL(PROJECT_GITHUB_URL)
    await handleClaimStarGift()
  }

  const v = (n: number) => loading ? '-' : n.toString()
  const live = metrics.live
  const memPct = live.memTotalMB > 0 ? (live.memUsedMB / live.memTotalMB) * 100 : 0

  // 图表数据（真实历史采样）
  const chartData = metrics.history.map(s => ({
    t: new Date(s.timestamp * 1000).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' }),
    cpu: s.cpuPercent,
    mem: s.memTotalMB > 0 ? Math.round((s.memUsedMB / s.memTotalMB) * 1000) / 10 : 0,
    running: s.runningInstances,
  }))

  const QUICK_ACTIONS = [
    { label: '批量启动', desc: '前往实例列表批量启动', icon: <Play className="w-5 h-5" />, onClick: () => navigate('/browser/list') },
    { label: '代理测速', desc: '测试代理可用性与延迟', icon: <Shield className="w-5 h-5" />, onClick: () => navigate('/browser/proxy-pool') },
    { label: '打开日志', desc: '查看运行日志排障', icon: <FileText className="w-5 h-5" />, onClick: () => navigate('/browser/logs') },
    { label: '创建实例', desc: '新建指纹浏览器实例', icon: <Plus className="w-5 h-5" />, onClick: () => navigate('/browser/list') },
  ]

  return (
    <div className="space-y-6 animate-fade-in">
      <div>
        <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">控制台</h1>
        <p className="text-sm text-[var(--color-text-muted)] mt-1">浏览器指纹管理平台概览（实时数据）</p>
      </div>

      {/* 统计卡片 */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard title="实例总数" value={v(stats.totalInstances)} icon={<Monitor className="w-5 h-5 text-blue-600" />} color="bg-blue-50 dark:bg-blue-900/20" to="/browser/list" />
        <StatCard title="运行中" value={v(stats.runningInstances)} icon={<Play className="w-5 h-5 text-green-600" />} color="bg-green-50 dark:bg-green-900/20" to="/browser/list" />
        <StatCard title="代理节点（可用/总）" value={loading ? '-' : `${stats.proxyAvailable}/${stats.proxyCount}`} icon={<Globe className="w-5 h-5 text-purple-600" />} color="bg-purple-50 dark:bg-purple-900/20" to="/browser/proxy-pool" />
        <StatCard title="内核版本" value={v(stats.coreCount)} icon={<Cpu className="w-5 h-5 text-orange-600" />} color="bg-orange-50 dark:bg-orange-900/20" to="/browser/cores" />
      </div>

      {/* 资源监控（真实采样 + 历史图表） */}
      <Card title="资源监控（实时）">
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          {/* 当前读数 */}
          <div className="space-y-4">
            <div>
              <div className="flex justify-between items-center mb-1.5">
                <span className="text-sm text-[var(--color-text-muted)]">CPU 使用率</span>
                <span className="text-sm font-medium text-[var(--color-text-primary)]">{live.cpuPercent.toFixed(1)}%</span>
              </div>
              <div className="w-full h-2 bg-[var(--color-bg-muted)] rounded-full overflow-hidden">
                <div className={`h-full rounded-full transition-all duration-500 ${live.cpuPercent > 80 ? 'bg-red-500' : live.cpuPercent > 60 ? 'bg-yellow-500' : 'bg-green-500'}`} style={{ width: `${Math.min(live.cpuPercent, 100)}%` }} />
              </div>
            </div>
            <div>
              <div className="flex justify-between items-center mb-1.5">
                <span className="text-sm text-[var(--color-text-muted)]">系统内存</span>
                <span className="text-sm font-medium text-[var(--color-text-primary)]">{live.memUsedMB} / {live.memTotalMB} MB</span>
              </div>
              <div className="w-full h-2 bg-[var(--color-bg-muted)] rounded-full overflow-hidden">
                <div className={`h-full rounded-full transition-all duration-500 ${memPct > 80 ? 'bg-red-500' : memPct > 60 ? 'bg-yellow-500' : 'bg-blue-500'}`} style={{ width: `${Math.min(memPct, 100)}%` }} />
              </div>
            </div>
            <div className="flex justify-between items-center pt-2 border-t border-[var(--color-border-muted)]">
              <span className="text-sm text-[var(--color-text-muted)]">运行中实例</span>
              <span className="text-sm font-medium text-[var(--color-text-primary)]">{live.runningInstances} / {stats.maxProfileLimit}</span>
            </div>
            <div className="flex justify-between items-center">
              <span className="text-sm text-[var(--color-text-muted)]">应用内存占用</span>
              <span className="text-sm font-medium text-[var(--color-text-primary)]">{live.appMemMB} MB</span>
            </div>
          </div>

          {/* 历史图表 */}
          <div className="lg:col-span-2">
            {chartData.length < 2 ? (
              <div className="h-[200px] flex items-center justify-center text-sm text-[var(--color-text-muted)]">正在采集历史数据…（约 5 秒一个采样点）</div>
            ) : (
              <ResponsiveContainer width="100%" height={200}>
                <AreaChart data={chartData} margin={{ top: 5, right: 10, left: -20, bottom: 0 }}>
                  <defs>
                    <linearGradient id="cpuGrad" x1="0" y1="0" x2="0" y2="1"><stop offset="5%" stopColor="#22c55e" stopOpacity={0.4} /><stop offset="95%" stopColor="#22c55e" stopOpacity={0} /></linearGradient>
                    <linearGradient id="memGrad" x1="0" y1="0" x2="0" y2="1"><stop offset="5%" stopColor="#3b82f6" stopOpacity={0.4} /><stop offset="95%" stopColor="#3b82f6" stopOpacity={0} /></linearGradient>
                  </defs>
                  <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border-muted)" />
                  <XAxis dataKey="t" tick={{ fontSize: 10 }} minTickGap={40} />
                  <YAxis domain={[0, 100]} tick={{ fontSize: 10 }} unit="%" />
                  <Tooltip />
                  <Area type="monotone" dataKey="cpu" name="CPU %" stroke="#22c55e" fill="url(#cpuGrad)" strokeWidth={2} isAnimationActive={false} />
                  <Area type="monotone" dataKey="mem" name="内存 %" stroke="#3b82f6" fill="url(#memGrad)" strokeWidth={2} isAnimationActive={false} />
                </AreaChart>
              </ResponsiveContainer>
            )}
          </div>
        </div>
      </Card>

      {/* 最近活动 + 最近错误 */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <Card title="最近活动">
          <div className="space-y-2 max-h-72 overflow-y-auto">
            {activity.length === 0 ? (
              <div className="text-center py-8 text-sm text-[var(--color-text-muted)]">暂无活动记录</div>
            ) : activity.map((log, i) => (
              <div key={`${log.timestamp}-${i}`} className="flex items-start gap-3 p-3 rounded-lg bg-[var(--color-bg-subtle)] hover:bg-[var(--color-bg-muted)] transition-colors">
                <div className="mt-0.5">{ACTIVITY_ICON[log.type] || <Activity className="w-3.5 h-3.5 text-gray-500" />}</div>
                <div className="flex-1 min-w-0">
                  <p className={`text-sm ${log.level === 'error' ? 'text-red-600' : 'text-[var(--color-text-primary)]'}`}>{log.message}</p>
                  {log.profileName && <p className="text-xs text-[var(--color-text-muted)] mt-0.5">实例: {log.profileName}</p>}
                </div>
                <span className="text-xs text-[var(--color-text-muted)] whitespace-nowrap">{timeAgo(log.timestamp)}</span>
              </div>
            ))}
          </div>
        </Card>

        <Card title="最近错误">
          <div className="space-y-2 max-h-72 overflow-y-auto">
            {errors.length === 0 ? (
              <div className="text-center py-8 text-sm text-[var(--color-text-muted)] flex flex-col items-center gap-2">
                <CheckCircle className="w-6 h-6 text-green-500" />
                <span>暂无错误，一切正常</span>
              </div>
            ) : errors.map((log, i) => (
              <div key={`${log.timestamp}-${i}`} className="flex items-start gap-3 p-3 rounded-lg bg-red-50 dark:bg-red-900/15 border border-red-200/60 dark:border-red-800/40">
                <AlertCircle className="w-3.5 h-3.5 text-red-600 mt-0.5 shrink-0" />
                <div className="flex-1 min-w-0">
                  <p className="text-sm text-red-700 dark:text-red-400 break-words">{log.message}</p>
                  {log.profileName && <p className="text-xs text-[var(--color-text-muted)] mt-0.5">实例: {log.profileName}</p>}
                </div>
                <span className="text-xs text-[var(--color-text-muted)] whitespace-nowrap">{timeAgo(log.timestamp)}</span>
              </div>
            ))}
          </div>
        </Card>
      </div>

      {/* 快捷入口 + 系统信息 */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <Card title="快捷入口">
          <div className="grid grid-cols-2 gap-3">
            {QUICK_ACTIONS.map(action => (
              <button key={action.label} onClick={action.onClick}
                className="group flex items-center gap-3 p-5 rounded-2xl border border-[var(--color-border-default)] bg-[var(--color-bg-card)] hover:border-[var(--color-border-strong)] hover:shadow-[var(--shadow-md)] transition-all duration-300 hover:-translate-y-1 text-left">
                <div className="w-12 h-12 rounded-xl bg-[var(--color-bg-muted)] flex items-center justify-center text-[var(--color-text-secondary)] group-hover:bg-[var(--color-accent)] group-hover:text-[var(--color-text-inverse)] group-hover:scale-110 transition-all duration-300 shadow-md shrink-0">
                  {action.icon}
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-semibold text-[var(--color-text-primary)]">{action.label}</p>
                  <p className="text-xs text-[var(--color-text-muted)] truncate mt-0.5">{action.desc}</p>
                </div>
              </button>
            ))}
          </div>
        </Card>

        <Card title="系统信息">
          <div className="space-y-1">
            {[
              { label: '系统版本', value: loading ? '-' : stats.appVersion },
              { label: '运行环境', value: 'Wails v2 + React' },
              { label: '数据存储', value: 'SQLite + YAML' },
              { label: '应用内存', value: `${live.appMemMB} MB` },
              { label: '实例运行', value: loading ? '-' : `${stats.runningInstances} / ${stats.totalInstances}` },
            ].map(item => (
              <div key={item.label} className="flex justify-between items-center py-3 border-b border-[var(--color-border-muted)] last:border-0">
                <span className="text-sm text-[var(--color-text-muted)]">{item.label}</span>
                <span className="text-sm font-medium text-[var(--color-text-primary)]">{item.value}</span>
              </div>
            ))}
          </div>

          <div className="mt-6 pt-6 border-t border-[var(--color-border-muted)]">
            <h3 className="text-sm font-medium text-[var(--color-text-primary)] mb-3">扩容系统</h3>
            <div className="flex gap-2">
              <input type="text" placeholder="输入兑换码 (如 ANT-...)" value={cdKey} onChange={e => setCdKey(e.target.value)} onKeyDown={e => e.key === 'Enter' && handleRedeem()}
                className="flex-1 px-3 py-2 text-sm rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-input)] text-[var(--color-text-primary)] focus:outline-none focus:border-[var(--color-primary)] placeholder-[var(--color-text-muted)]" />
              <Button onClick={handleRedeem} loading={redeeming} disabled={!cdKey.trim()}>兑换</Button>
            </div>
            <p className="mt-2 text-xs text-[var(--color-text-muted)] flex items-center justify-between">
              <span>当前容量限制：</span>
              <span className={`font-medium ${stats.totalInstances >= stats.maxProfileLimit ? 'text-red-500' : 'text-[var(--color-success)]'}`}>{loading ? '-' : `${stats.totalInstances} / ${stats.maxProfileLimit}`}</span>
            </p>
            <div className="mt-4 rounded-xl border border-blue-500/30 bg-blue-500/10 p-4 shadow-glow-blue">
              <div className="flex items-center justify-between gap-4">
                <p className="text-sm font-medium text-[var(--color-text-primary)]">点亮 GitHub Star 后，可再获赠 50 个永久额度</p>
                <button type="button" className="shrink-0 rounded-xl p-2.5 text-blue-600 dark:text-blue-400 transition-all duration-300 hover:bg-blue-500/20 hover:scale-110 disabled:opacity-50 shadow-sm" onClick={handleOpenGithubStarGift} disabled={redeeming} title="打开 GitHub 并领取赠送" aria-label="打开 GitHub 并领取赠送">
                  <ExternalLink className="w-5 h-5" />
                </button>
              </div>
            </div>
          </div>
        </Card>
      </div>
    </div>
  )
}
