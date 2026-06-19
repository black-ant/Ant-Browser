import { useEffect, useState } from 'react'
import { Monitor, Play, Shield, Cpu, ArrowRight, ExternalLink, Globe, Settings } from 'lucide-react'
import { Link } from 'react-router-dom'
import { Card, Button, toast } from '../../shared/components'
import { fetchDashboardStats, redeemCDKey, redeemGithubStar, reloadConfig } from './api'
import type { DashboardStats } from './types'
import { BrowserOpenURL } from '../../wailsjs/runtime/runtime'
import { PROJECT_GITHUB_URL } from '../../config/links'

interface StatCardProps {
  title: string
  value: string | number
  icon: React.ReactNode
  color: string
}

function StatCard({ title, value, icon, color }: StatCardProps) {
  return (
    <div className="rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-card)] p-5">
      <div className="flex items-center justify-between mb-3">
        <span className="text-sm text-[var(--color-text-muted)]">{title}</span>
        <div className={`w-9 h-9 rounded-lg flex items-center justify-center ${color}`}>
          {icon}
        </div>
      </div>
      <div className="text-2xl font-semibold text-[var(--color-text-primary)]">{value}</div>
    </div>
  )
}

const QUICK_LINKS = [
  { to: '/workbench', icon: <Monitor className="w-5 h-5" />, label: '店铺工作台', desc: '管理本机店铺窗口' },
  { to: '/browser/proxy-pool', icon: <Shield className="w-5 h-5" />, label: '代理池', desc: '配置和测试代理节点' },
  { to: '/browser/cores', icon: <Cpu className="w-5 h-5" />, label: '内核管理', desc: '管理 Chrome 内核版本' },
  { to: '/settings', icon: <Settings className="w-5 h-5" />, label: '系统设置', desc: '全局参数配置' },
]

export function DashboardPage() {
  const [stats, setStats] = useState<DashboardStats>({
    totalInstances: 0,
    runningInstances: 0,
    proxyCount: 0,
    coreCount: 0,
    memUsedMB: 0,
    maxProfileLimit: 20,
    appVersion: 'unknown',
  })
  const [loading, setLoading] = useState(true)
  const [cdKey, setCdKey] = useState('')
  const [redeeming, setRedeeming] = useState(false)

  useEffect(() => {
    load()
  }, [])

  const load = async () => {
    setLoading(true)
    try {
      await reloadConfig() // 强制从本地磁盘刷一次最新配置，解决各种情况下的容量不同步
      setStats(await fetchDashboardStats())
    } finally {
      setLoading(false)
    }
  }

  const handleRedeem = async () => {
    if (!cdKey.trim()) return
    setRedeeming(true)
    const result = await redeemCDKey(cdKey.trim())
    setRedeeming(false)
    if (result.success) {
      toast.success('兑换成功！此名额已到账')
      setCdKey('')
      load() // Refresh stats
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

  return (
    <div className="space-y-6 animate-fade-in">
      <div>
        <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">今日执行台</h1>
        <p className="text-sm text-[var(--color-text-muted)] mt-1">浏览器指纹管理平台概览</p>
      </div>

      {/* 统计卡片 */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard
          title="实例总数"
          value={v(stats.totalInstances)}
          icon={<Monitor className="w-4 h-4 text-blue-500" />}
          color="bg-blue-50 dark:bg-blue-900/20"
        />
        <StatCard
          title="运行中"
          value={v(stats.runningInstances)}
          icon={<Play className="w-4 h-4 text-green-500" />}
          color="bg-green-50 dark:bg-green-900/20"
        />
        <StatCard
          title="代理节点"
          value={v(stats.proxyCount)}
          icon={<Globe className="w-4 h-4 text-purple-500" />}
          color="bg-purple-50 dark:bg-purple-900/20"
        />
        <StatCard
          title="内核版本"
          value={v(stats.coreCount)}
          icon={<Cpu className="w-4 h-4 text-orange-500" />}
          color="bg-orange-50 dark:bg-orange-900/20"
        />
      </div>

      {/* 快捷操作 + 系统信息 */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <Card title="快捷操作">
          <div className="grid grid-cols-2 gap-3">
            {QUICK_LINKS.map(link => (
              <Link
                key={link.to}
                to={link.to}
                className="group flex items-center gap-3 p-4 rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-subtle)]
                         hover:border-[var(--color-border-strong)] hover:bg-[var(--color-bg-muted)] transition-all duration-150"
              >
                <div className="w-10 h-10 rounded-xl bg-[var(--color-accent-muted)] flex items-center justify-center text-[var(--color-text-secondary)]
                              group-hover:bg-[var(--color-accent)] group-hover:text-[var(--color-text-inverse)] transition-colors shrink-0">
                  {link.icon}
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium text-[var(--color-text-primary)]">{link.label}</p>
                  <p className="text-xs text-[var(--color-text-muted)] truncate">{link.desc}</p>
                </div>
                <ArrowRight className="w-4 h-4 text-[var(--color-text-muted)] opacity-0 -translate-x-2 group-hover:opacity-100 group-hover:translate-x-0 transition-all shrink-0" />
              </Link>
            ))}
          </div>
        </Card>

        <Card title="系统信息">
          <div className="space-y-1">
            {[
              { label: '系统版本', value: loading ? '-' : stats.appVersion },
              { label: '运行环境', value: 'Wails v2 + React' },
              { label: '数据存储', value: 'SQLite + YAML' },
              { label: '内存占用', value: loading ? '-' : `${stats.memUsedMB} MB` },
              { label: '实例运行', value: loading ? '-' : `${stats.runningInstances} / ${stats.totalInstances}` },
            ].map(item => (
              <div
                key={item.label}
                className="flex justify-between items-center py-3 border-b border-[var(--color-border-muted)] last:border-0"
              >
                <span className="text-sm text-[var(--color-text-muted)]">{item.label}</span>
                <span className="text-sm font-medium text-[var(--color-text-primary)]">{item.value}</span>
              </div>
            ))}
          </div>

          <div className="mt-6 pt-6 border-t border-[var(--color-border-muted)]">
            <h3 className="text-sm font-medium text-[var(--color-text-primary)] mb-3">扩容系统</h3>
            <div className="flex gap-2">
              <input
                type="text"
                placeholder="输入兑换码 (如 ANT-...)"
                value={cdKey}
                onChange={e => setCdKey(e.target.value)}
                onKeyDown={e => e.key === 'Enter' && handleRedeem()}
                className="flex-1 px-3 py-2 text-sm rounded-lg border border-[var(--color-border-default)]
                           bg-[var(--color-bg-input)] text-[var(--color-text-primary)]
                           focus:outline-none focus:border-[var(--color-primary)] placeholder-[var(--color-text-muted)]"
              />
              <Button onClick={handleRedeem} loading={redeeming} disabled={!cdKey.trim()}>
                兑换
              </Button>
            </div>
            <p className="mt-2 text-xs text-[var(--color-text-muted)] flex items-center justify-between">
              <span>当前容量限制：</span>
              <span className={`font-medium ${stats.totalInstances >= stats.maxProfileLimit ? 'text-red-500' : 'text-[var(--color-success)]'}`}>
                {loading ? '-' : `${stats.totalInstances} / ${stats.maxProfileLimit}`}
              </span>
            </p>

            <div className="mt-4 rounded-lg border border-blue-500/20 bg-blue-500/10 p-4">
              <div className="flex items-center justify-between gap-4">
                <p className="text-sm text-[var(--color-text-primary)]">点亮 GitHub Star 后，可再获赠 50 个永久额度</p>
                <button
                  type="button"
                  className="shrink-0 rounded-full p-2 text-[var(--color-accent)] transition-colors hover:bg-[var(--color-accent)]/10 disabled:opacity-50"
                  onClick={handleOpenGithubStarGift}
                  disabled={redeeming}
                  title="打开 GitHub 并领取赠送"
                  aria-label="打开 GitHub 并领取赠送"
                >
                  <ExternalLink className="w-4 h-4" />
                </button>
              </div>
            </div>
          </div>
        </Card>
      </div>
    </div>
  )
}
