import { useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { Plus, X } from 'lucide-react'

import { toast } from '../../../shared/components'
import { batchRemoveProfileTags, batchSetProfileTags } from '../api'

interface TagInlineCellProps {
  tags: string[]
  profileId: string
  allTags: string[]
  onChanged: () => void
  maxVisible?: number
}

// 列表内标签单元:等高显示前 maxVisible 个,其余悬停/点击 +N 展开;支持行内加/删。
// 展开面板与新增面板均 portal 到 document.body 并以 fixed 定位,避免被表格的 overflow-auto 容器裁切
// (与 BrowserProfilesPanel.tsx 里 ProfileMoreActions 的“更多”菜单同一套解法)。
export function TagInlineCell({ tags, profileId, allTags, onChanged, maxVisible = 2 }: TagInlineCellProps) {
  const [open, setOpen] = useState(false)
  const [adding, setAdding] = useState(false)
  const [input, setInput] = useState('')
  const [busy, setBusy] = useState(false)
  const [expandPosition, setExpandPosition] = useState({ top: 0, left: 0 })
  const [addPosition, setAddPosition] = useState({ top: 0, left: 0 })

  const expandTriggerRef = useRef<HTMLButtonElement>(null)
  const expandPopupRef = useRef<HTMLDivElement>(null)
  const addTriggerRef = useRef<HTMLButtonElement>(null)
  const addPopupRef = useRef<HTMLDivElement>(null)

  const list = tags ?? []
  const visible = list.slice(0, maxVisible)
  const hiddenCount = Math.max(0, list.length - maxVisible)
  const suggestions = allTags.filter((t) => !list.includes(t) && t.includes(input.trim())).slice(0, 8)

  // 展开面板:定位 + 点击外部/Escape 关闭(mirror ProfileMoreActions 的 useEffect 结构)
  useEffect(() => {
    if (!open) return
    const estimatedPopupHeight = 160
    const popupWidth = 280
    const updatePosition = () => {
      const rect = expandTriggerRef.current?.getBoundingClientRect()
      if (!rect) return
      const gap = 4
      const left = Math.max(8, Math.min(rect.left, window.innerWidth - popupWidth - 8))
      const belowTop = rect.bottom + gap
      const top = belowTop + estimatedPopupHeight > window.innerHeight
        ? Math.max(8, rect.top - estimatedPopupHeight - gap)
        : belowTop
      setExpandPosition({ top, left })
    }
    const handlePointerDown = (event: MouseEvent) => {
      const target = event.target as Node
      if (!expandTriggerRef.current?.contains(target) && !expandPopupRef.current?.contains(target)) {
        setOpen(false)
      }
    }
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setOpen(false)
    }
    updatePosition()
    document.addEventListener('mousedown', handlePointerDown)
    document.addEventListener('keydown', handleKeyDown)
    window.addEventListener('resize', updatePosition)
    window.addEventListener('scroll', updatePosition, true)
    return () => {
      document.removeEventListener('mousedown', handlePointerDown)
      document.removeEventListener('keydown', handleKeyDown)
      window.removeEventListener('resize', updatePosition)
      window.removeEventListener('scroll', updatePosition, true)
    }
  }, [open])

  // 新增面板:定位 + 点击外部/Escape 关闭(mirror ProfileMoreActions 的 useEffect 结构)
  useEffect(() => {
    if (!adding) return
    const estimatedPopupHeight = 120
    const popupWidth = 224
    const updatePosition = () => {
      const rect = addTriggerRef.current?.getBoundingClientRect()
      if (!rect) return
      const gap = 4
      const left = Math.max(8, Math.min(rect.left, window.innerWidth - popupWidth - 8))
      const belowTop = rect.bottom + gap
      const top = belowTop + estimatedPopupHeight > window.innerHeight
        ? Math.max(8, rect.top - estimatedPopupHeight - gap)
        : belowTop
      setAddPosition({ top, left })
    }
    const handlePointerDown = (event: MouseEvent) => {
      const target = event.target as Node
      if (!addTriggerRef.current?.contains(target) && !addPopupRef.current?.contains(target)) {
        setAdding(false)
      }
    }
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setAdding(false)
    }
    updatePosition()
    document.addEventListener('mousedown', handlePointerDown)
    document.addEventListener('keydown', handleKeyDown)
    window.addEventListener('resize', updatePosition)
    window.addEventListener('scroll', updatePosition, true)
    return () => {
      document.removeEventListener('mousedown', handlePointerDown)
      document.removeEventListener('keydown', handleKeyDown)
      window.removeEventListener('resize', updatePosition)
      window.removeEventListener('scroll', updatePosition, true)
    }
  }, [adding])

  // 互斥:展开面板与新增面板同时只能开一个
  const openExpand = () => { setAdding(false); setOpen(true) }
  const toggleExpand = () => setOpen((v) => { const next = !v; if (next) setAdding(false); return next })
  const toggleAdd = () => setAdding((v) => { const next = !v; if (next) setOpen(false); return next })

  const addTag = async (raw: string) => {
    const tag = raw.trim()
    if (!tag || list.includes(tag)) { setInput(''); setAdding(false); return }
    setBusy(true)
    try {
      await batchSetProfileTags([profileId], [tag], false)
      setInput('')
      setAdding(false)
      onChanged()
    } catch (e: any) {
      toast.error(e?.message || '添加标签失败')
    } finally { setBusy(false) }
  }

  const removeTag = async (tag: string) => {
    setBusy(true)
    try {
      await batchRemoveProfileTags([profileId], [tag])
      onChanged()
    } catch (e: any) {
      toast.error(e?.message || '删除标签失败')
    } finally { setBusy(false) }
  }

  return (
    <div className="relative flex items-center gap-1 min-w-0">
      <div className="flex items-center gap-1 overflow-hidden">
        {visible.map((t) => (
          <span key={t} className="inline-flex items-center rounded bg-[var(--color-bg-secondary)] px-1.5 py-0.5 text-xs text-[var(--color-text-secondary)] max-w-[96px] truncate">{t}</span>
        ))}
        {hiddenCount > 0 && (
          <button
            ref={expandTriggerRef}
            type="button"
            onMouseEnter={openExpand}
            onClick={toggleExpand}
            className="rounded bg-[var(--color-bg-secondary)] px-1.5 py-0.5 text-xs text-[var(--color-accent)] shrink-0"
          >+{hiddenCount}</button>
        )}
        {list.length === 0 && <span className="text-xs text-[var(--color-text-muted)]">无标签</span>}
      </div>

      <button
        ref={addTriggerRef}
        type="button"
        title="添加标签"
        onClick={toggleAdd}
        disabled={busy}
        className="shrink-0 p-0.5 rounded text-[var(--color-text-muted)] hover:text-[var(--color-accent)] disabled:opacity-50"
      ><Plus className="w-3.5 h-3.5" /></button>

      {/* 悬停/点击展开:全部标签 + 逐个删除。portal 到 body,避免被表格 overflow-auto 裁切 */}
      {open && list.length > 0 && createPortal(
        <div
          ref={expandPopupRef}
          className="fixed z-[9999] max-w-[280px] rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-elevated)] p-2 shadow-xl"
          style={{ top: expandPosition.top, left: expandPosition.left }}
        >
          <div className="flex flex-wrap gap-1">
            {list.map((t) => (
              <span key={t} className="inline-flex items-center gap-1 rounded bg-[var(--color-bg-secondary)] px-1.5 py-0.5 text-xs text-[var(--color-text-secondary)]">
                {t}
                <button type="button" onClick={() => removeTag(t)} disabled={busy} className="hover:text-red-500 disabled:opacity-50"><X className="w-3 h-3" /></button>
              </span>
            ))}
          </div>
        </div>,
        document.body
      )}

      {/* 行内新增输入。portal 到 body,避免被表格 overflow-auto 裁切 */}
      {adding && createPortal(
        <div
          ref={addPopupRef}
          className="fixed z-[9999] w-56 rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-elevated)] p-2 shadow-xl"
          style={{ top: addPosition.top, left: addPosition.left }}
        >
          <input
            autoFocus
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => { if (e.key === 'Enter') addTag(input); if (e.key === 'Escape') setAdding(false) }}
            placeholder="输入标签,回车添加"
            className="w-full px-2 py-1 text-xs rounded border border-[var(--color-accent)] bg-[var(--color-bg-input)] text-[var(--color-text-primary)] focus:outline-none"
          />
          {suggestions.length > 0 && (
            <div className="mt-1 flex flex-wrap gap-1">
              {suggestions.map((s) => (
                <button key={s} type="button" onClick={() => addTag(s)} className="rounded bg-[var(--color-bg-secondary)] px-1.5 py-0.5 text-xs text-[var(--color-text-secondary)] hover:text-[var(--color-accent)]">{s}</button>
              ))}
            </div>
          )}
        </div>,
        document.body
      )}
    </div>
  )
}
