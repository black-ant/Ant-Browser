import { useEffect, useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { ArrowLeft, Copy, ExternalLink, Globe2, Layers3, Pencil, Play, RefreshCw, RotateCcw, Square, TerminalSquare } from 'lucide-react'
import type { BrowserCore, BrowserGroup, BrowserProfile, BrowserProxy, BrowserTab } from '../../modules/browser/types'
import {
  fetchBrowserCores,
  fetchBrowserProfiles,
  fetchBrowserProxies,
  fetchBrowserTabs,
  fetchGroups,
  openBrowserUrl,
  regenerateBrowserProfileCode,
  restartBrowserInstance,
  startBrowserInstance,
  stopBrowserInstance,
} from '../../modules/browser/api'
import { resolveActionErrorMessage, resolveActionFeedback } from '../../modules/browser/utils/actionErrors'
import { warmupProfileProxyBeforeStart } from '../../modules/browser/utils/proxyWarmup'
import { toast } from '../../shared/components'
import { FishCookiePanel, FishSnapshotPanel } from './FishDetailDataPanels'

type DetailTab = 'overview' | 'cookies' | 'snapshots'
type PendingAction = 'starting' | 'stopping' | 'restarting' | null

function formatTime(value?: string) {
  if (!value) return '—'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '—' : date.toLocaleString('zh-CN')
}

function statusLabel(profile: BrowserProfile) {
  if (!profile.running) return { label: '已停止', tone: 'off' }
  if (!profile.debugReady) return { label: '运行中 · 待就绪', tone: 'warn' }
  return { label: '运行中', tone: 'on' }
}

function SummaryPanel({ title, description, children }: { title: string; description?: string; children: React.ReactNode }) {
  return <section className="fish-panel"><div className="fish-panel-head"><div><strong>{title}</strong>{description ? <span>{description}</span> : null}</div></div><div className="fish-panel-body fish-summary-list">{children}</div></section>
}

function SummaryRow({ label, children }: { label: string; children: React.ReactNode }) {
  return <div><span>{label}</span><strong>{children}</strong></div>
}

export function FishBrowserDetailPage() {
  const { id } = useParams()
  const [profile, setProfile] = useState<BrowserProfile | null>(null)
  const [tabs, setTabs] = useState<BrowserTab[]>([])
  const [cores, setCores] = useState<BrowserCore[]>([])
  const [proxies, setProxies] = useState<BrowserProxy[]>([])
  const [groups, setGroups] = useState<BrowserGroup[]>([])
  const [targetUrl, setTargetUrl] = useState('https://example.com')
  const [activeTab, setActiveTab] = useState<DetailTab>('overview')
  const [pendingAction, setPendingAction] = useState<PendingAction>(null)
  const [loading, setLoading] = useState(true)

  const loadProfile = async () => {
    const list = await fetchBrowserProfiles()
    const current = list.find((item) => item.profileId === id) || null
    setProfile(current)
    return current
  }

  const loadTabs = async () => {
    if (!id) return
    try { setTabs(await fetchBrowserTabs(id)) }
    catch { setTabs([]) }
  }

  useEffect(() => {
    let cancelled = false
    const load = async () => {
      setLoading(true)
      try {
        const [profileList, coreList, proxyList, groupList] = await Promise.all([fetchBrowserProfiles(), fetchBrowserCores(), fetchBrowserProxies(), fetchGroups()])
        if (cancelled) return
        setProfile(profileList.find((item) => item.profileId === id) || null)
        setCores(coreList); setProxies(proxyList); setGroups(groupList)
        if (id) {
          try { setTabs(await fetchBrowserTabs(id)) } catch { setTabs([]) }
        }
      } finally { if (!cancelled) setLoading(false) }
    }
    void load()
    return () => { cancelled = true }
  }, [id])

  useEffect(() => {
    if (!id) return
    const runtime = (window as Window & { runtime?: { EventsOn?: (event: string, callback: (payload: unknown) => void) => (() => void) | void } }).runtime
    if (!runtime?.EventsOn) return
    const handleRuntimeChange = (payload: unknown) => {
      const value = payload as { profileId?: string; error?: string } | string
      const profileId = typeof value === 'string' ? value : value?.profileId
      if (profileId !== id) return
      setPendingAction(null)
      void loadProfile()
      if (typeof value === 'string' || value?.error) setTabs([])
      else void loadTabs()
    }
    const offs = ['browser:instance:started', 'browser:instance:updated', 'browser:instance:stopped', 'browser:instance:crashed'].map((event) => runtime.EventsOn?.(event, handleRuntimeChange))
    return () => offs.forEach((off) => off?.())
  }, [id])

  const coreName = useMemo(() => {
    if (!profile?.coreId || profile.coreId.toLowerCase() === 'default') return cores.find((core) => core.isDefault)?.coreName || '默认内核'
    return cores.find((core) => core.coreId === profile.coreId)?.coreName || profile.coreId
  }, [cores, profile])
  const proxyName = useMemo(() => {
    if (!profile) return '—'
    if (profile.proxyId) return proxies.find((proxy) => proxy.proxyId === profile.proxyId)?.proxyName || profile.proxyId
    return profile.proxyConfig || 'DIRECT'
  }, [proxies, profile])
  const groupName = useMemo(() => profile?.groupId ? groups.find((group) => group.groupId === profile.groupId)?.groupName || profile.groupId : '未分组', [groups, profile])

  if (loading) return <div className="fish-content"><div className="fish-empty"><strong>正在读取实例详情</strong><span>同步运行状态、标签页和配置摘要。</span></div></div>
  if (!profile) return <div className="fish-content"><div className="fish-empty"><strong>没有找到该实例</strong><span>它可能已被删除或移入回收站。</span><Link className="fish-btn" to="/browser/list" style={{ marginTop: 14 }}>返回实例列表</Link></div></div>

  const runtimeStatus = statusLabel(profile)
  const isStarting = pendingAction === 'starting'
  const isStopping = pendingAction === 'stopping'
  const isRestarting = pendingAction === 'restarting'
  const isBusy = pendingAction !== null

  const start = async () => {
    setPendingAction('starting')
    try {
      await warmupProfileProxyBeforeStart(profile)
      const updated = await startBrowserInstance(profile.profileId)
      if (updated) setProfile(updated)
      if (updated?.running && !updated.debugReady && updated.runtimeWarning) toast.warning(updated.runtimeWarning)
      else toast.success('实例已启动')
    } catch (error) {
      const feedback = resolveActionFeedback(error, '实例启动失败')
      feedback.tone === 'warning' ? toast.warning(feedback.message) : toast.error(feedback.message)
    } finally { await loadProfile(); setPendingAction(null) }
  }

  const stop = async () => {
    setPendingAction('stopping')
    try { const updated = await stopBrowserInstance(profile.profileId); if (updated) setProfile(updated); toast.success('实例已停止') }
    catch (error) { toast.error(resolveActionErrorMessage(error, '实例停止失败')) }
    finally { await loadProfile(); setPendingAction(null) }
  }

  const restart = async () => {
    setPendingAction('restarting')
    try { await warmupProfileProxyBeforeStart(profile); const updated = await restartBrowserInstance(profile.profileId); if (updated) setProfile(updated); toast.success('实例已重启') }
    catch (error) {
      const feedback = resolveActionFeedback(error, '实例重启失败')
      feedback.tone === 'warning' ? toast.warning(feedback.message) : toast.error(feedback.message)
    } finally { await loadProfile(); setPendingAction(null) }
  }

  const openUrl = async () => {
    const url = targetUrl.trim()
    if (!url) { toast.warning('请输入目标地址'); return }
    try { const opened = await openBrowserUrl(profile.profileId, url); opened ? toast.success('已在实例中打开地址') : toast.warning('打开指令未执行') }
    catch (error) {
      const feedback = resolveActionFeedback(error, '打开地址失败')
      feedback.tone === 'warning' ? toast.warning(feedback.message) : toast.error(feedback.message)
    }
  }

  const copyCode = async () => {
    if (!profile.launchCode) return
    try { await navigator.clipboard.writeText(profile.launchCode); toast.success('已复制快捷码') }
    catch { toast.error('复制快捷码失败') }
  }

  const regenerateCode = async () => {
    try { await regenerateBrowserProfileCode(profile.profileId); await loadProfile(); toast.success('快捷码已重新生成') }
    catch (error) { toast.error(error instanceof Error ? error.message : '重新生成快捷码失败') }
  }

  return <div className="fish-content fish-detail-content">
    <div className="fish-detail-hero">
      <div className="fish-detail-identity"><span className="fish-profile-icon detail"><TerminalSquare size={20} strokeWidth={1.8} /></span><div><span className="fish-eyebrow">浏览器环境</span><h1>{profile.profileName}</h1><p>{profile.launchCode || profile.profileId} · {groupName}</p><div className="fish-tag-row"><span className={`fish-pill ${runtimeStatus.tone === 'on' ? 'green' : runtimeStatus.tone === 'warn' ? '' : ''}`}>{runtimeStatus.label}</span>{profile.tags.slice(0, 4).map((tag) => <span className="fish-pill blue" key={tag}>{tag}</span>)}</div></div></div>
      <div className="fish-detail-actions"><Link className="fish-btn" to="/browser/list"><ArrowLeft size={16} strokeWidth={1.8} />返回</Link><Link className="fish-btn" to={`/browser/edit/${profile.profileId}`}><Pencil size={16} strokeWidth={1.8} />编辑</Link>{profile.running ? <button className="fish-btn" type="button" disabled={isBusy && !isStopping} onClick={() => void stop()}><Square size={16} strokeWidth={1.8} />{isStopping ? '停止中' : '停止'}</button> : <button className="fish-btn primary" type="button" disabled={isBusy && !isStarting} onClick={() => void start()}><Play size={16} strokeWidth={1.8} />{isStarting ? '启动中' : '启动'}</button>}<button className="fish-icon-btn" type="button" disabled={isBusy && !isRestarting} title="重启实例" onClick={() => void restart()}><RotateCcw size={16} strokeWidth={1.8} /></button></div>
    </div>

    <div className="fish-detail-tabs"><button className={activeTab === 'overview' ? 'active' : ''} type="button" onClick={() => setActiveTab('overview')}>概览</button><button className={activeTab === 'cookies' ? 'active' : ''} type="button" onClick={() => setActiveTab('cookies')}>Cookie</button><button className={activeTab === 'snapshots' ? 'active' : ''} type="button" onClick={() => setActiveTab('snapshots')}>快照</button></div>

    {activeTab === 'overview' ? <div className="fish-detail-stack">
      <div className="fish-detail-summary-grid">
        <SummaryPanel title="运行信息" description="进程、调试状态和最近运行时间。"><SummaryRow label="状态"><span className={`fish-status ${profile.running ? profile.debugReady ? '' : 'warn' : 'off'}`}>{runtimeStatus.label}</span></SummaryRow><SummaryRow label="进程 PID">{profile.pid || '—'}</SummaryRow><SummaryRow label="调试端口">{profile.debugPort || '—'}</SummaryRow><SummaryRow label="调试状态">{profile.debugReady ? '已就绪' : profile.running ? '等待就绪' : '—'}</SummaryRow><SummaryRow label="最近启动">{formatTime(profile.lastStartAt)}</SummaryRow><SummaryRow label="最近停止">{formatTime(profile.lastStopAt)}</SummaryRow></SummaryPanel>
        <SummaryPanel title="配置摘要" description="当前实例身份与运行配置。"><SummaryRow label="内核">{coreName}</SummaryRow><SummaryRow label="代理">{proxyName}</SummaryRow><SummaryRow label="分组">{groupName}</SummaryRow><SummaryRow label="指纹参数">{profile.fingerprintArgs.length} 项</SummaryRow><SummaryRow label="启动参数">{profile.launchArgs.length} 项</SummaryRow><SummaryRow label="内存限制">{profile.memoryLimitMb > 0 ? `${profile.memoryLimitMb} MB` : '不限制'}</SummaryRow></SummaryPanel>
        <SummaryPanel title="数据与快捷码" description="本地目录和快速唤起标识。"><SummaryRow label="数据目录"><span className="fish-breakable">{profile.userDataDir || '—'}</span></SummaryRow><SummaryRow label="快捷码">{profile.launchCode ? <span className="fish-code-actions"><code>{profile.launchCode}</code><button className="fish-mini-icon" type="button" title="复制" onClick={() => void copyCode()}><Copy size={16} strokeWidth={1.8} /></button><button className="fish-mini-icon" type="button" title="重新生成" onClick={() => void regenerateCode()}><RefreshCw size={16} strokeWidth={1.8} /></button></span> : '—'}</SummaryRow><SummaryRow label="创建时间">{formatTime(profile.createdAt)}</SummaryRow><SummaryRow label="最近更新">{formatTime(profile.updatedAt)}</SummaryRow></SummaryPanel>
      </div>

      {profile.lastError ? <div className="fish-alert error"><strong>最近错误</strong><span>{profile.lastError}</span></div> : null}
      {profile.runtimeWarning ? <div className="fish-alert warn"><strong>运行提示</strong><span>{profile.runtimeWarning}</span></div> : null}

      <section className="fish-panel">
        <div className="fish-panel-head"><div><strong>打开地址</strong><span>向当前运行实例发送打开 URL 指令。</span></div></div>
        <div className="fish-panel-body fish-open-url"><div className="fish-inline-control"><input className="fish-input" value={targetUrl} onChange={(event) => setTargetUrl(event.target.value)} placeholder="https://example.com" onKeyDown={(event) => { if (event.key === 'Enter') void openUrl() }} /><button className="fish-btn primary" type="button" disabled={!profile.running} onClick={() => void openUrl()}><Globe2 size={16} strokeWidth={1.8} />打开</button></div></div>
      </section>

      <section className="fish-panel">
        <div className="fish-panel-head"><div><strong>标签页</strong><span>{profile.running ? `${tabs.length} 个标签页` : '实例未运行'}</span></div><div className="fish-panel-actions"><button className="fish-btn" type="button" disabled={!profile.running} onClick={() => void loadTabs()}><RefreshCw size={16} strokeWidth={1.8} />刷新</button></div></div>
        <div className="fish-panel-body no-pad"><div className="fish-tab-list">{tabs.length ? tabs.map((tab) => <div className="fish-tab-row" key={tab.tabId}><Layers3 size={16} strokeWidth={1.8} /><div><strong>{tab.title || '无标题页面'}</strong><small>{tab.url}</small></div>{tab.active ? <span className="fish-pill green">当前</span> : <span className="fish-pill">后台</span>}<ExternalLink size={16} strokeWidth={1.8} /></div>) : <div className="fish-empty compact"><strong>{profile.running ? '暂无标签页信息' : '实例未运行'}</strong><span>{profile.running ? '调试接口就绪后刷新查看。' : '启动实例后会显示当前标签页。'}</span></div>}</div></div>
      </section>
    </div> : null}

    {activeTab === 'cookies' ? <FishCookiePanel profileId={profile.profileId} profileName={profile.profileName} running={profile.running} ready={profile.running && profile.debugReady} /> : null}
    {activeTab === 'snapshots' ? <FishSnapshotPanel profileId={profile.profileId} running={profile.running} /> : null}
  </div>
}
