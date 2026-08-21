import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { Copy, Globe, Play, RefreshCw, RotateCcw, Square } from 'lucide-react'
import { Badge, Button, Card, Input, Table, toast } from '../../../shared/components'
import type { TableColumn } from '../../../shared/components/Table'
import type { BrowserProfile, BrowserTab } from '../types'
import { EventsOn } from '../../../wailsjs/runtime/runtime'
import {
  fetchBrowserProfiles,
  fetchBrowserTabs,
  openBrowserUrl,
  regenerateBrowserProfileCode,
  restartBrowserInstance,
  startBrowserInstance,
  stopBrowserInstance,
} from '../api'
import { CookieManagerCard } from '../components/CookieManagerCard'
import { SnapshotTab } from '../components/SnapshotTab'
import { resolveActionErrorMessage, resolveActionFeedback } from '../utils/actionErrors'
import { warmupProfileProxyBeforeStart } from '../utils/proxyWarmup'

const resolveRuntimeStatus = (running: boolean, debugReady: boolean, lastError = '') => {
  if (!running && lastError.trim()) return { variant: 'error' as const, label: '异常' }
  if (!running) return { variant: 'default' as const, label: '已停止' }
  if (!debugReady) return { variant: 'info' as const, label: '运行中（待就绪）' }
  return { variant: 'success' as const, label: '运行中' }
}

const formatTime = (value?: string) => {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleString('zh-CN')
}

type TabKey = 'overview' | 'snapshot'

const TABS: { key: TabKey; label: string }[] = [
  { key: 'overview', label: '概览' },
  { key: 'snapshot', label: '快照管理' },
]

export function BrowserDetailPage() {
  const { id } = useParams()
  const [profile, setProfile] = useState<BrowserProfile | null>(null)
  const [tabs, setTabs] = useState<BrowserTab[]>([])
  const [targetUrl, setTargetUrl] = useState('https://example.com')
  const [activeTab, setActiveTab] = useState<TabKey>('overview')
  const [pendingAction, setPendingAction] = useState<'starting' | 'stopping' | 'restarting' | null>(null)

  const loadProfile = async () => {
    const list = await fetchBrowserProfiles()
    const current = list.find(item => item.profileId === id) || null
    setProfile(current)
    return current
  }

  const loadTabs = async () => {
    if (!id) return
    const list = await fetchBrowserTabs(id)
    setTabs(list)
  }

  useEffect(() => { void loadProfile() }, [id])
  useEffect(() => { void loadTabs() }, [id])

  useEffect(() => {
    if (!id) return

    const handleRuntimeChange = (payload: any) => {
      const profileId = typeof payload === 'string' ? payload : payload?.profileId
      if (profileId !== id) return

      setPendingAction(null)
      void loadProfile()

      if (typeof payload === 'string' || payload?.error) {
        setTabs([])
        return
      }

      void loadTabs()
    }

    const offStarted = EventsOn('browser:instance:started', handleRuntimeChange)
    const offUpdated = EventsOn('browser:instance:updated', handleRuntimeChange)
    const offStopped = EventsOn('browser:instance:stopped', handleRuntimeChange)
    const offCrashed = EventsOn('browser:instance:crashed', handleRuntimeChange)

    return () => {
      offStarted?.()
      offUpdated?.()
      offStopped?.()
      offCrashed?.()
    }
  }, [id])

  if (!profile) {
    return (
      <div className="flex items-center justify-center h-64 text-sm text-[var(--color-text-muted)]">
        暂无实例信息
      </div>
    )
  }

  const handleOpenUrl = async () => {
    const normalizedTargetUrl = targetUrl.trim()
    if (!normalizedTargetUrl) {
      toast.warning('请输入目标地址')
      return
    }

    try {
      const opened = await openBrowserUrl(profile.profileId, normalizedTargetUrl)
      if (!opened) {
        toast.warning('打开指令未执行')
        return
      }
      toast.success('已在运行中实例打开地址')
    } catch (error: any) {
      const feedback = resolveActionFeedback(error, '打开地址失败')
      if (feedback.tone === 'warning') {
        toast.warning(feedback.message)
      } else {
        toast.error(feedback.message)
      }
    }
  }

  const handleStart = async () => {
    setPendingAction('starting')
    try {
      await warmupProfileProxyBeforeStart(profile)
      const startedProfile = await startBrowserInstance(profile.profileId)
      if (startedProfile) {
        setProfile(startedProfile)
      }
      if (startedProfile?.runtimeWarning || (startedProfile?.running && !startedProfile.debugReady)) {
        toast.warning(startedProfile.runtimeWarning || '浏览器窗口已启动，调试接口仍在后台接管。')
      } else {
        toast.success('实例已启动')
      }
    } catch (error: any) {
      const feedback = resolveActionFeedback(error, '实例启动失败')
      if (feedback.tone === 'warning') {
        toast.warning(feedback.message)
      } else {
        toast.error(feedback.message)
      }
    } finally {
      await loadProfile()
      setPendingAction(null)
    }
  }

  const handleStop = async () => {
    setPendingAction('stopping')
    try {
      const stoppedProfile = await stopBrowserInstance(profile.profileId)
      if (stoppedProfile) {
        setProfile(stoppedProfile)
      }
      toast.success('实例已停止')
    } catch (error: any) {
      toast.error(resolveActionErrorMessage(error, '实例停止失败'))
    } finally {
      await loadProfile()
      setPendingAction(null)
    }
  }

  const handleRestart = async () => {
    setPendingAction('restarting')
    try {
      await warmupProfileProxyBeforeStart(profile)
      const restartedProfile = await restartBrowserInstance(profile.profileId)
      if (restartedProfile) {
        setProfile(restartedProfile)
      }
      if (restartedProfile?.runtimeWarning || (restartedProfile?.running && !restartedProfile.debugReady)) {
        toast.warning(restartedProfile.runtimeWarning || '浏览器窗口已启动，调试接口仍在后台接管。')
      } else {
        toast.success('实例已重启')
      }
    } catch (error: any) {
      const feedback = resolveActionFeedback(error, '实例重启失败')
      if (feedback.tone === 'warning') {
        toast.warning(feedback.message)
      } else {
        toast.error(feedback.message)
      }
    } finally {
      await loadProfile()
      setPendingAction(null)
    }
  }

  const tabsColumns: TableColumn<BrowserTab>[] = [
    { key: 'title', title: '标题' },
    { key: 'url', title: '地址' },
    {
      key: 'active',
      title: '状态',
      render: value => (
        <Badge variant={value ? 'success' : 'default'}>{value ? '当前' : '后台'}</Badge>
      ),
    },
  ]

  const isStarting = pendingAction === 'starting'
  const isStopping = pendingAction === 'stopping'
  const isRestarting = pendingAction === 'restarting'
  const isBusy = pendingAction !== null
  const runtimeStatus = resolveRuntimeStatus(profile.running, profile.debugReady, profile.lastError)

  return (
    <div className="space-y-5 animate-fade-in">
      {/* 页头 */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">实例详情</h1>
          <p className="text-sm text-[var(--color-text-muted)] mt-1">{profile.profileName}</p>
        </div>
        <div className="flex gap-2">
          <Link to={`/browser/edit/${profile.profileId}`}>
            <Button variant="secondary" size="sm">编辑配置</Button>
          </Link>
          <Link to="/browser/list">
            <Button variant="ghost" size="sm">返回列表</Button>
          </Link>
        </div>
      </div>

      {/* Tab 导航 */}
      <div className="flex border-b border-[var(--color-border)]">
        {TABS.map(tab => (
          <button
            key={tab.key}
            onClick={() => setActiveTab(tab.key)}
            className={[
              'px-4 py-2 text-sm font-medium transition-colors',
              activeTab === tab.key
                ? 'border-b-2 border-[var(--color-primary)] text-[var(--color-primary)]'
                : 'text-[var(--color-text-muted)] hover:text-[var(--color-text-secondary)]',
            ].join(' ')}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {/* 概览 Tab */}
      {activeTab === 'overview' && (
        <div className="space-y-4">
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
            <Card title="运行信息" subtitle="实例运行状态与端口信息">
              <div className="space-y-3 text-sm text-[var(--color-text-secondary)]">
                <div className="flex justify-between">
                  <span>状态</span>
                  <Badge variant={runtimeStatus.variant} dot>{runtimeStatus.label}</Badge>
                </div>
                <div className="flex justify-between">
                  <span>进程 PID</span>
                  <span>{profile.pid || '-'}</span>
                </div>
                <div className="flex justify-between">
                  <span>调试端口</span>
                  <span>{profile.debugPort || '-'}</span>
                </div>
                <div className="flex justify-between">
                  <span>调试状态</span>
                  <span>{profile.debugReady ? '已就绪' : (profile.running ? '等待就绪' : '-')}</span>
                </div>
                <div className="flex justify-between">
                  <span>最近启动</span>
                  <span>{formatTime(profile.lastStartAt)}</span>
                </div>
                <div className="flex justify-between">
                  <span>最近停止</span>
                  <span>{formatTime(profile.lastStopAt)}</span>
                </div>
              </div>
            </Card>

            <Card title="配置摘要" subtitle="指纹与启动参数">
              <div className="space-y-3 text-sm text-[var(--color-text-secondary)]">
                <div className="flex justify-between">
                  <span>用户数据目录</span>
                  <span>{profile.userDataDir}</span>
                </div>
                <div className="flex justify-between">
                  <span>内核</span>
                  <span>{profile.coreId || '默认'}</span>
                </div>
                <div className="flex justify-between">
                  <span>代理配置</span>
                  <span>{profile.proxyConfig || '-'}</span>
                </div>
                <div className="flex justify-between">
                  <span>指纹参数</span>
                  <span>{profile.fingerprintArgs?.length || 0} 项</span>
                </div>
                <div className="flex justify-between">
                  <span>启动参数</span>
                  <span>{profile.launchArgs?.length || 0} 项</span>
                </div>
                <div className="flex justify-between">
                  <span>标签</span>
                  <span>{profile.tags?.join(', ') || '-'}</span>
                </div>
                <div className="flex justify-between items-center">
                  <span>快捷码</span>
                  <div className="flex items-center gap-1">
                    {profile.launchCode ? (
                      <>
                        <code className="text-xs font-mono bg-[var(--color-bg-secondary)] px-1.5 py-0.5 rounded text-[var(--color-accent)]">{profile.launchCode}</code>
                        <button
                          onClick={() => navigator.clipboard.writeText(profile.launchCode!).then(() => toast.success('已复制快捷码'))}
                          className="p-0.5 hover:text-[var(--color-accent)] text-[var(--color-text-muted)] transition-colors"
                          title="复制"
                        >
                          <Copy className="w-3 h-3" />
                        </button>
                        <button
                          onClick={async () => {
                            await regenerateBrowserProfileCode(profile.profileId)
                            loadProfile()
                            toast.success('快捷码已重新生成')
                          }}
                          className="p-0.5 hover:text-[var(--color-accent)] text-[var(--color-text-muted)] transition-colors"
                          title="重新生成"
                        >
                          <RefreshCw className="w-3 h-3" />
                        </button>
                      </>
                    ) : (
                      <span className="text-[var(--color-text-muted)]">-</span>
                    )}
                  </div>
                </div>
              </div>
            </Card>
          </div>

          <Card title="快捷操作" subtitle="快速控制实例">
            <div className="flex flex-wrap items-center gap-2">
              {profile.running ? (
                <Button size="sm" variant="secondary" onClick={handleStop} loading={isStopping} disabled={isBusy && !isStopping}>
                  {!isStopping && <Square className="w-4 h-4" />}
                  {isStopping ? '停止中' : '停止'}
                </Button>
              ) : (
                <Button size="sm" onClick={handleStart} loading={isStarting} disabled={isBusy && !isStarting}>
                  {!isStarting && <Play className="w-4 h-4" />}
                  {isStarting ? '启动中' : '启动'}
                </Button>
              )}
              <Button size="sm" variant="ghost" onClick={handleRestart} loading={isRestarting} disabled={isBusy && !isRestarting}>
                {!isRestarting && <RotateCcw className="w-4 h-4" />}
                {isRestarting ? '重启中' : '重启'}
              </Button>
            </div>
          </Card>

          {profile.lastError && (
            <Card title="最近错误" subtitle="最近一次启动或运行失败原因">
              <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 whitespace-pre-line">
                {profile.lastError}
              </div>
            </Card>
          )}

          {profile.runtimeWarning && (
            <Card title="运行提示" subtitle="当前实例处于部分可用状态">
              <div className="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-700 whitespace-pre-line">
                {profile.runtimeWarning}
              </div>
            </Card>
          )}

          <Card title="打开地址" subtitle="向实例发送打开 URL 指令">
            <div className="flex flex-col md:flex-row gap-3">
              <Input value={targetUrl} onChange={e => setTargetUrl(e.target.value)} placeholder="请输入目标地址" />
              <Button onClick={handleOpenUrl}>
                <Globe className="w-4 h-4" />
                打开
              </Button>
            </div>
          </Card>

          <Card title="标签页列表" subtitle="当前实例标签页信息">
            <Table columns={tabsColumns} data={tabs} rowKey="tabId" />
          </Card>

          <CookieManagerCard
            profileId={profile.profileId}
            profileName={profile.profileName}
            running={profile.running}
            ready={profile.running && profile.debugReady}
          />
        </div>
      )}

      {/* 快照管理 Tab */}
      {activeTab === 'snapshot' && (
        <SnapshotTab profileId={profile.profileId} running={profile.running} />
      )}
    </div>
  )
}
