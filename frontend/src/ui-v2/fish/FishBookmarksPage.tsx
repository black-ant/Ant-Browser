import { useEffect, useMemo, useState, type DragEvent } from 'react'
import { Bookmark, GripVertical, Plus, RefreshCw, RotateCcw, Save, Trash2 } from 'lucide-react'
import { toast } from '../../shared/components'
import type { BrowserBookmark } from '../../modules/browser/types'
import { fetchBookmarks, resetBookmarks, saveBookmarks, syncBookmarksToProfiles } from '../../modules/browser/api'
import { FishConfirm, FishSwitch } from './FishFormPrimitives'
import './fish-modules.css'

const protectedUrl = 'ant://fingerprint-check'
const isProtected = (item: BrowserBookmark) => item.url.trim().toLowerCase() === protectedUrl

export function FishBookmarksPage() {
  const [items, setItems] = useState<BrowserBookmark[]>([])
  const [saving, setSaving] = useState(false)
  const [syncing, setSyncing] = useState(false)
  const [resetOpen, setResetOpen] = useState(false)
  const [syncOpen, setSyncOpen] = useState(false)
  const [dragIndex, setDragIndex] = useState<number | null>(null)
  useEffect(() => { void fetchBookmarks().then(setItems) }, [])
  const protectedCount = useMemo(() => items.filter(isProtected).length, [items])
  const startCount = useMemo(() => items.filter((item) => item.openOnStart).length, [items])
  const regularCount = items.length - protectedCount
  const update = (index: number, patch: Partial<BrowserBookmark>) => setItems((current) => current.map((item, i) => i === index ? { ...item, ...patch } : item))
  const add = () => setItems((current) => [...current, { name: '', url: '', openOnStart: false }])
  const remove = (index: number) => { if (!isProtected(items[index])) setItems((current) => current.filter((_, i) => i !== index)) }
  const save = async () => {
    if (items.some((item) => !item.name.trim() || !item.url.trim())) { toast.error('存在空的名称或 URL，请填写完整后保存'); return }
    setSaving(true)
    try {
      await saveBookmarks(items)
      const result = await syncBookmarksToProfiles()
      const parts = ['书签已保存']
      if (result.synced) parts.push(`同步 ${result.synced} 个实例`)
      if (result.skipped) parts.push(`跳过运行中 ${result.skipped} 个`)
      if (result.failed) parts.push(`失败 ${result.failed} 个`)
      result.failed || result.skipped ? toast.warning(parts.join('，')) : toast.success(parts.join('，'))
    } finally { setSaving(false) }
  }
  const sync = async () => {
    setSyncing(true)
    try {
      const result = await syncBookmarksToProfiles()
      const text = [`同步 ${result.synced} 个实例`, result.skipped ? `跳过 ${result.skipped}` : '', result.failed ? `失败 ${result.failed}` : ''].filter(Boolean).join('，')
      result.failed || result.skipped ? toast.warning(text) : toast.success(text)
      setSyncOpen(false)
    } catch (error) { toast.error(error instanceof Error ? error.message : '同步失败') } finally { setSyncing(false) }
  }
  const reset = async () => { await resetBookmarks(); setItems(await fetchBookmarks()); setResetOpen(false); toast.success('已恢复默认书签') }
  const dragOver = (event: DragEvent, index: number) => {
    event.preventDefault()
    if (dragIndex === null || dragIndex === index || isProtected(items[dragIndex]) || isProtected(items[index])) return
    setItems((current) => { const next = [...current]; const [moved] = next.splice(dragIndex, 1); next.splice(index, 0, moved); return next })
    setDragIndex(index)
  }
  return <div className="fish-content fish-module-content">
    <section className="fish-module-hero"><div><span className="fish-eyebrow">默认书签</span><h1>默认书签</h1><p>管理新实例书签、排序与启动打开行为。</p></div><button className="fish-btn primary" type="button" disabled={saving} onClick={() => void save()}><Save size={16} strokeWidth={1.8} />{saving ? '保存中' : '保存书签'}</button></section>
    <section className="fish-module-strip"><div><span>全部书签</span><strong>{items.length}</strong></div><div><span>内置保护</span><strong>{protectedCount}</strong></div><div><span>自定义</span><strong>{regularCount}</strong></div><div><span>启动打开</span><strong>{startCount}</strong></div></section>
    <section className="fish-module-toolbar"><button className="fish-btn" type="button" onClick={add}><Plus size={16} strokeWidth={1.8} />添加书签</button><button className="fish-btn" type="button" onClick={() => setSyncOpen(true)}><RefreshCw size={16} strokeWidth={1.8} />手动同步</button><span className="fish-spacer" /><button className="fish-btn" type="button" onClick={() => setResetOpen(true)}><RotateCcw size={16} strokeWidth={1.8} />恢复默认</button></section>
    <section>{items.map((item, index) => { const locked = isProtected(item); return <div className={`fish-bookmark-row${locked ? ' protected' : ''}`} key={`${item.url}-${index}`} draggable={!locked} onDragStart={() => !locked && setDragIndex(index)} onDragOver={(event) => dragOver(event,index)} onDragEnd={() => setDragIndex(null)}><span className="fish-bookmark-grip"><GripVertical size={16} strokeWidth={1.8} /></span><input className="fish-input" value={item.name} readOnly={locked} onChange={(event) => update(index,{name:event.target.value})} placeholder="书签名称" /><input className="fish-input" value={item.url} readOnly={locked} onChange={(event) => update(index,{url:event.target.value})} placeholder="https://..." /><div className="fish-bookmark-start"><div className="fish-inline-actions"><FishSwitch checked={Boolean(item.openOnStart)} onChange={(checked) => update(index,{openOnStart:checked})} /><span className="fish-status-pill">启动打开</span></div></div><button className="fish-mini-icon" type="button" disabled={locked} title={locked?'内置书签不可删除':'删除'} onClick={() => remove(index)}>{locked?<Bookmark size={16} strokeWidth={1.8}/>:<Trash2 size={16} strokeWidth={1.8}/>}</button></div> })}</section>
    <FishConfirm open={resetOpen} title="恢复默认书签" content="将清除当前所有自定义书签并恢复内置默认列表。" confirmText="恢复默认" danger onClose={() => setResetOpen(false)} onConfirm={() => void reset()} />
    <FishConfirm open={syncOpen} title="同步已有实例" content="只会增量追加缺失的默认书签，不删除、改名或移动用户已有书签；运行中的实例会跳过。" confirmText={syncing?'同步中':'开始同步'} busy={syncing} onClose={() => setSyncOpen(false)} onConfirm={() => void sync()} />
  </div>
}
