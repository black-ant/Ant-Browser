import { useState } from 'react'
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

// 列表内标签单元:等高显示前 maxVisible 个,其余悬停 +N 展开;支持行内加/删。
export function TagInlineCell({ tags, profileId, allTags, onChanged, maxVisible = 2 }: TagInlineCellProps) {
  const [open, setOpen] = useState(false)
  const [adding, setAdding] = useState(false)
  const [input, setInput] = useState('')
  const [busy, setBusy] = useState(false)

  const list = tags ?? []
  const visible = list.slice(0, maxVisible)
  const hiddenCount = Math.max(0, list.length - maxVisible)
  const suggestions = allTags.filter((t) => !list.includes(t) && t.includes(input.trim())).slice(0, 8)

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
    <div className="relative flex items-center gap-1 min-w-0" onMouseLeave={() => setOpen(false)}>
      <div className="flex items-center gap-1 overflow-hidden">
        {visible.map((t) => (
          <span key={t} className="inline-flex items-center rounded bg-[var(--color-bg-secondary)] px-1.5 py-0.5 text-xs text-[var(--color-text-secondary)] max-w-[96px] truncate">{t}</span>
        ))}
        {hiddenCount > 0 && (
          <button
            type="button"
            onMouseEnter={() => setOpen(true)}
            onClick={() => setOpen((v) => !v)}
            className="rounded bg-[var(--color-bg-secondary)] px-1.5 py-0.5 text-xs text-[var(--color-accent)] shrink-0"
          >+{hiddenCount}</button>
        )}
        {list.length === 0 && <span className="text-xs text-[var(--color-text-muted)]">无标签</span>}
      </div>

      <button
        type="button"
        title="添加标签"
        onClick={() => setAdding((v) => !v)}
        disabled={busy}
        className="shrink-0 p-0.5 rounded text-[var(--color-text-muted)] hover:text-[var(--color-accent)] disabled:opacity-50"
      ><Plus className="w-3.5 h-3.5" /></button>

      {/* 悬停/点击展开:全部标签 + 逐个删除 */}
      {open && list.length > 0 && (
        <div className="absolute z-20 top-full left-0 mt-1 max-w-[280px] rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-elevated)] p-2 shadow-xl">
          <div className="flex flex-wrap gap-1">
            {list.map((t) => (
              <span key={t} className="inline-flex items-center gap-1 rounded bg-[var(--color-bg-secondary)] px-1.5 py-0.5 text-xs text-[var(--color-text-secondary)]">
                {t}
                <button type="button" onClick={() => removeTag(t)} disabled={busy} className="hover:text-red-500 disabled:opacity-50"><X className="w-3 h-3" /></button>
              </span>
            ))}
          </div>
        </div>
      )}

      {/* 行内新增输入 */}
      {adding && (
        <div className="absolute z-20 top-full left-0 mt-1 w-56 rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-elevated)] p-2 shadow-xl">
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
        </div>
      )}
    </div>
  )
}
