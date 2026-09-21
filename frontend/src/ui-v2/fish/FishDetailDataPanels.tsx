import { useEffect, useMemo, useState } from 'react'
import { Archive, Download, RefreshCw, RotateCcw, Search, Trash2 } from 'lucide-react'
import type { CookieInfo, SnapshotInfo } from '../../modules/browser/types'
import { clearBrowserCookies, createSnapshot, deleteSnapshot, exportBrowserCookies, fetchBrowserCookies, listSnapshots, restoreSnapshot } from '../../modules/browser/api'
import { toast } from '../../shared/components'
import { FishConfirm } from './FishFormPrimitives'

function formatExpires(expires: number) {
  if (expires <= 0) return 'Session'
  return new Date(expires * 1000).toLocaleString('zh-CN')
}

function formatSize(mb: number) {
  return mb < 1 ? `${(mb * 1024).toFixed(0)} KB` : `${mb.toFixed(1)} MB`
}

function formatTime(value: string) {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '—' : date.toLocaleString('zh-CN')
}

function defaultSnapshotName() {
  const now = new Date()
  const pad = (value: number) => String(value).padStart(2, '0')
  return `快照_${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}_${pad(now.getHours())}:${pad(now.getMinutes())}`
}

export function FishCookiePanel({ profileId, profileName, running, ready }: { profileId: string; profileName: string; running: boolean; ready: boolean }) {
  const [cookies, setCookies] = useState<CookieInfo[]>([])
  const [filter, setFilter] = useState('')
  const [loading, setLoading] = useState(false)
  const [clearing, setClearing] = useState(false)
  const [clearConfirm, setClearConfirm] = useState(false)

  const load = async () => {
    if (!ready) return
    setLoading(true)
    try { setCookies(await fetchBrowserCookies(profileId)) }
    catch { toast.error('获取 Cookie 失败') }
    finally { setLoading(false) }
  }

  useEffect(() => {
    if (ready) void load()
    else setCookies([])
  }, [profileId, ready])

  const filtered = useMemo(() => {
    const query = filter.trim().toLowerCase()
    if (!query) return cookies
    return cookies.filter((cookie) => `${cookie.domain} ${cookie.name}`.toLowerCase().includes(query))
  }, [cookies, filter])

  const clearAll = async () => {
    setClearing(true)
    try { await clearBrowserCookies(profileId); setCookies([]); toast.success('Cookie 已清除') }
    catch { toast.error('清除 Cookie 失败') }
    finally { setClearing(false); setClearConfirm(false) }
  }

  const exportCookies = async () => {
    try {
      const content = await exportBrowserCookies(profileId)
      const date = new Date().toISOString().slice(0, 10)
      const blob = new Blob([content], { type: 'text/plain' })
      const url = URL.createObjectURL(blob)
      const anchor = document.createElement('a')
      anchor.href = url
      anchor.download = `cookies_${profileName}_${date}.txt`
      anchor.click()
      URL.revokeObjectURL(url)
      toast.success('Cookie 已导出')
    } catch { toast.error('导出 Cookie 失败') }
  }

  if (!running) return <div className="fish-data-empty"><strong>实例未运行</strong><span>启动实例后才能读取 Cookie。</span></div>
  if (!ready) return <div className="fish-data-empty"><strong>等待调试接口</strong><span>浏览器已经启动，调试接口就绪后会自动读取 Cookie。</span></div>

  return <section className="fish-panel">
    <div className="fish-panel-head"><div><strong>Cookie</strong><span>{cookies.length} 条{filter ? ` · 当前显示 ${filtered.length} 条` : ''}</span></div><div className="fish-panel-actions"><button className="fish-btn" type="button" disabled={loading} onClick={() => void load()}><RefreshCw size={16} strokeWidth={1.8} />刷新</button><button className="fish-btn" type="button" onClick={() => void exportCookies()}><Download size={16} strokeWidth={1.8} />导出 Netscape</button><button className="fish-btn danger" type="button" disabled={clearing} onClick={() => setClearConfirm(true)}><Trash2 size={16} strokeWidth={1.8} />清除全部</button></div></div>
    <div className="fish-panel-body">
      <label className="fish-search fish-data-search"><Search size={16} strokeWidth={1.8} /><input value={filter} onChange={(event) => setFilter(event.target.value)} placeholder="搜索域名或 Cookie 名称" /></label>
      <div className="fish-data-table cookie">
        <div className="fish-data-head"><span>域名</span><span>名称</span><span>值</span><span>过期</span><span>标记</span></div>
        {filtered.length ? filtered.map((cookie, index) => <div className="fish-data-row" key={`${cookie.domain}-${cookie.name}-${index}`}><code>{cookie.domain}</code><code>{cookie.name}</code><code title={cookie.value}>{cookie.value}</code><span>{formatExpires(cookie.expires)}</span><div className="fish-tag-row">{cookie.httpOnly ? <span className="fish-pill green">HttpOnly</span> : null}{cookie.secure ? <span className="fish-pill blue">Secure</span> : null}{!cookie.httpOnly && !cookie.secure ? <span className="fish-pill">普通</span> : null}</div></div>) : <div className="fish-empty compact"><strong>没有 Cookie</strong><span>{filter ? '当前过滤条件没有匹配项。' : '此实例暂时没有可读取的 Cookie。'}</span></div>}
      </div>
    </div>
    <FishConfirm open={clearConfirm} title="清除全部 Cookie？" content="此操作会清除当前实例的所有 Cookie，且无法撤销。" confirmText="确认清除" danger busy={clearing} onClose={() => setClearConfirm(false)} onConfirm={() => void clearAll()} />
  </section>
}

export function FishSnapshotPanel({ profileId, running }: { profileId: string; running: boolean }) {
  const [snapshots, setSnapshots] = useState<SnapshotInfo[]>([])
  const [loading, setLoading] = useState(false)
  const [creating, setCreating] = useState(false)
  const [name, setName] = useState(defaultSnapshotName)
  const [restoreId, setRestoreId] = useState<string | null>(null)
  const [deleteId, setDeleteId] = useState<string | null>(null)
  const [actionId, setActionId] = useState<string | null>(null)

  const load = async () => {
    setLoading(true)
    try { setSnapshots(await listSnapshots(profileId)) }
    finally { setLoading(false) }
  }

  useEffect(() => { void load() }, [profileId])

  const create = async () => {
    if (!name.trim() || running) return
    setCreating(true)
    try { await createSnapshot(profileId, name.trim()); toast.success('快照创建成功'); setName(defaultSnapshotName()); await load() }
    catch { toast.error('快照创建失败') }
    finally { setCreating(false) }
  }

  const restore = async () => {
    if (!restoreId || running) return
    setActionId(restoreId)
    try { await restoreSnapshot(profileId, restoreId); toast.success('快照恢复成功') }
    catch { toast.error('快照恢复失败') }
    finally { setActionId(null); setRestoreId(null) }
  }

  const remove = async () => {
    if (!deleteId) return
    setActionId(deleteId)
    try { await deleteSnapshot(profileId, deleteId); toast.success('快照已删除'); await load() }
    catch { toast.error('快照删除失败') }
    finally { setActionId(null); setDeleteId(null) }
  }

  return <div className="fish-detail-stack">
    <section className="fish-panel">
      <div className="fish-panel-head"><div><strong>创建快照</strong><span>{running ? '实例运行中，请先停止后再创建。' : '将当前用户数据目录压缩为可恢复快照。'}</span></div></div>
      <div className="fish-panel-body fish-snapshot-create"><input className="fish-input" value={name} disabled={running || creating} onChange={(event) => setName(event.target.value)} placeholder="快照名称" /><button className="fish-btn primary" type="button" disabled={running || creating || !name.trim()} onClick={() => void create()}><Archive size={16} strokeWidth={1.8} />{creating ? '创建中' : '创建快照'}</button></div>
    </section>
    <section className="fish-panel">
      <div className="fish-panel-head"><div><strong>快照列表</strong><span>{loading ? '加载中…' : `${snapshots.length} 个快照`}</span></div><div className="fish-panel-actions"><button className="fish-btn" type="button" disabled={loading} onClick={() => void load()}><RefreshCw size={16} strokeWidth={1.8} />刷新</button></div></div>
      <div className="fish-panel-body no-pad">
        <div className="fish-snapshot-list">{snapshots.length ? snapshots.map((snapshot) => <div className="fish-snapshot-row" key={snapshot.snapshotId}><div><span className="fish-square-icon"><Archive size={16} strokeWidth={1.8} /></span><span><strong>{snapshot.name}</strong><small>{formatTime(snapshot.createdAt)}</small></span></div><span>{formatSize(snapshot.sizeMB)}</span><div className="fish-row-actions"><button className="fish-mini-btn" type="button" disabled={running || actionId === snapshot.snapshotId} title={running ? '请先停止实例' : '恢复快照'} onClick={() => setRestoreId(snapshot.snapshotId)}><RotateCcw size={16} strokeWidth={1.8} />恢复</button><button className="fish-mini-icon" type="button" disabled={actionId === snapshot.snapshotId} title="删除快照" onClick={() => setDeleteId(snapshot.snapshotId)}><Trash2 size={16} strokeWidth={1.8} /></button></div></div>) : <div className="fish-empty compact"><strong>暂无快照</strong><span>停止实例后可以创建第一份快照。</span></div>}</div>
      </div>
    </section>
    <FishConfirm open={Boolean(restoreId)} title="恢复此快照？" content="恢复会覆盖当前用户数据目录。请确认实例已经停止。" confirmText="确认恢复" busy={Boolean(actionId)} onClose={() => setRestoreId(null)} onConfirm={() => void restore()} />
    <FishConfirm open={Boolean(deleteId)} title="删除此快照？" content="删除后无法恢复该快照文件。" confirmText="确认删除" danger busy={Boolean(actionId)} onClose={() => setDeleteId(null)} onConfirm={() => void remove()} />
  </div>
}
