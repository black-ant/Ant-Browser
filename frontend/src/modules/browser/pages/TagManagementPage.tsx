import { useEffect, useMemo, useRef, useState } from 'react'
import { Plus, Tag, Trash2, X } from 'lucide-react'
import { Badge, Button, Card, toast } from '../../../shared/components'
import type { BrowserProfile } from '../types'
import { batchRemoveProfileTags, batchSetProfileTags, fetchBrowserProfiles, renameBrowserTag } from '../api'

// ─── 左侧标签面板 ────────────────────────────────────────────────────────────

interface TagPanelProps {
  tags: string[]
  selected: string | null
  profilesByTag: Record<string, number>
  totalCount: number
  onSelect: (tag: string | null) => void
  onCreateTag: (tag: string) => void
  onRenameTag: (oldName: string, newName: string) => void
}

function TagPanel({ tags, selected, profilesByTag, totalCount, onSelect, onCreateTag, onRenameTag }: TagPanelProps) {
  const [creating, setCreating] = useState(false)
  const [newTag, setNewTag] = useState('')
  const inputRef = useRef<HTMLInputElement>(null)

  const [editingTag, setEditingTag] = useState<string | null>(null)
  const [editValue, setEditValue] = useState('')

  const commit = () => {
    const t = newTag.trim()
    if (t && !tags.includes(t)) {
      onCreateTag(t)
      onSelect(t)
    }
    setNewTag('')
    setCreating(false)
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') commit()
    if (e.key === 'Escape') { setNewTag(''); setCreating(false) }
  }

  const startEdit = (tag: string) => {
    setEditingTag(tag)
    setEditValue(tag)
  }

  const commitEdit = () => {
    const newVal = editValue.trim()
    if (newVal && editingTag && newVal !== editingTag) {
      onRenameTag(editingTag, newVal)
    }
    setEditingTag(null)
  }

  return (
    <div className="w-52 shrink-0 border-r border-[var(--color-border)] flex flex-col bg-[var(--color-bg-surface)]">
      <div className="px-4 py-3 border-b border-[var(--color-border)] flex items-center justify-between">
        <span className="text-xs font-semibold text-[var(--color-text-muted)] uppercase tracking-wider">标签列表</span>
        <button
          onClick={() => { setCreating(true); setTimeout(() => inputRef.current?.focus(), 50) }}
          title="新建标签"
          className="p-0.5 rounded text-[var(--color-text-muted)] hover:text-[var(--color-primary)] hover:bg-[var(--color-primary)]/10 transition-colors"
        >
          <Plus className="w-3.5 h-3.5" />
        </button>
      </div>
      <div className="flex-1 overflow-y-auto py-2">
        <button
          onClick={() => onSelect(null)}
          className={`w-full text-left px-4 py-2 text-sm flex items-center justify-between transition-colors ${selected === null
              ? 'bg-[var(--color-primary)]/10 text-[var(--color-primary)] font-medium'
              : 'text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-hover)]'
            }`}
        >
          <span>全部窗口</span>
          <span className="text-xs opacity-60">{totalCount}</span>
        </button>
        {tags.map(tag => (
          <div
            key={tag}
            onContextMenu={e => { e.preventDefault(); startEdit(tag) }}
            onClick={() => onSelect(tag)}
            className={`w-full text-left px-4 py-2 text-sm flex items-center justify-between gap-2 transition-colors cursor-pointer group ${selected === tag
                ? 'bg-[var(--color-primary)]/10 text-[var(--color-primary)] font-medium'
                : 'text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-hover)]'
              }`}
            title="右键可以重命名"
          >
            {editingTag === tag ? (
              <input
                autoFocus
                value={editValue}
                onChange={e => setEditValue(e.target.value)}
                onBlur={commitEdit}
                onKeyDown={e => {
                  if (e.key === 'Enter') commitEdit()
                  if (e.key === 'Escape') setEditingTag(null)
                }}
                onClick={e => e.stopPropagation()}
                className="flex-1 min-w-0 px-1.5 py-0.5 text-xs rounded border border-[var(--color-primary)] bg-[var(--color-bg-input)] text-[var(--color-text-primary)] focus:outline-none"
              />
            ) : (
              <span className="flex items-center gap-1.5 truncate">
                <Tag className="w-3.5 h-3.5 shrink-0 opacity-60" />
                <span className="truncate">{tag}</span>
              </span>
            )}

            {editingTag !== tag && (
              <span className="text-xs opacity-60 shrink-0">{profilesByTag[tag] ?? 0}</span>
            )}
          </div>
        ))}
        {tags.length === 0 && !creating && (
          <p className="px-4 py-3 text-xs text-[var(--color-text-muted)]">暂无标签，点击 + 创建</p>
        )}

        {/* 内联新建输入框 */}
        {creating && (
          <div className="px-3 py-2 flex items-center gap-1">
            <input
              ref={inputRef}
              value={newTag}
              onChange={e => setNewTag(e.target.value)}
              onKeyDown={handleKeyDown}
              onBlur={commit}
              placeholder="标签名称"
              className="flex-1 min-w-0 px-2 py-1 text-xs rounded border border-[var(--color-primary)] bg-[var(--color-bg-input)] text-[var(--color-text-primary)] placeholder-[var(--color-text-muted)] focus:outline-none"
            />
          </div>
        )}
      </div>
    </div>
  )
}

// ─── 批量操作工具栏 ───────────────────────────────────────────────────────────

interface ActionBarProps {
  selectedCount: number
  allTags: string[]
  onAddTags: (tags: string[]) => void
  onRemoveTags: (tags: string[]) => void
  onClear: () => void
}

function ActionBar({ selectedCount, allTags, onAddTags, onRemoveTags, onClear }: ActionBarProps) {
  const [addInput, setAddInput] = useState('')
  const [removeTag, setRemoveTag] = useState('')

  if (selectedCount === 0) return null

  const handleAdd = () => {
    const tags = addInput.split(/[,，\s]+/).map(t => t.trim()).filter(Boolean)
    if (!tags.length) return
    onAddTags(tags)
    setAddInput('')
  }

  return (
    <div className="flex items-center gap-3 px-4 py-2.5 bg-[var(--color-primary)]/5 border border-[var(--color-primary)]/20 rounded-lg text-sm">
      <span className="text-[var(--color-primary)] font-medium shrink-0">已选 {selectedCount} 个</span>
      <div className="flex items-center gap-1.5 flex-1 flex-wrap">
        {/* 添加标签 */}
        <div className="flex items-center gap-1">
          <input
            value={addInput}
            onChange={e => setAddInput(e.target.value)}
            onKeyDown={e => e.key === 'Enter' && handleAdd()}
            placeholder="输入标签，逗号分隔"
            className="px-2 py-1 text-xs rounded border border-[var(--color-border)] bg-[var(--color-bg-input)] text-[var(--color-text-primary)] placeholder-[var(--color-text-muted)] focus:outline-none focus:border-[var(--color-primary)] w-40"
          />
          <Button size="sm" onClick={handleAdd} disabled={!addInput.trim()}>
            <Plus className="w-3.5 h-3.5" />添加标签
          </Button>
        </div>
        {/* 移除标签 */}
        {allTags.length > 0 && (
          <div className="flex items-center gap-1">
            <select
              value={removeTag}
              onChange={e => setRemoveTag(e.target.value)}
              className="px-2 py-1 text-xs rounded border border-[var(--color-border)] bg-[var(--color-bg-input)] text-[var(--color-text-primary)] focus:outline-none focus:border-[var(--color-primary)]"
            >
              <option value="">选择要移除的标签</option>
              {allTags.map(t => <option key={t} value={t}>{t}</option>)}
            </select>
            <Button size="sm" variant="secondary" onClick={() => { if (removeTag) { onRemoveTags([removeTag]); setRemoveTag('') } }} disabled={!removeTag}>
              <Trash2 className="w-3.5 h-3.5" />移除
            </Button>
          </div>
        )}
      </div>
      <button onClick={onClear} className="shrink-0 text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)]">
        <X className="w-4 h-4" />
      </button>
    </div>
  )
}

// ─── 主页面 ───────────────────────────────────────────────────────────────────

export function TagManagementPage() {
  const [profiles, setProfiles] = useState<BrowserProfile[]>([])
  const [loading, setLoading] = useState(true)
  const [selectedTag, setSelectedTag] = useState<string | null>(null)
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [saving, setSaving] = useState(false)
  // 用户新建但尚未分配给任何窗口的标签（纯前端暂存）
  const [pendingTags, setPendingTags] = useState<string[]>([])

  // 合并：窗口已有标签 + 用户新建的待分配标签
  const allTagsWithPending = useMemo(() => {
    const set = new Set<string>()
    profiles.forEach(p => p.tags?.forEach(t => set.add(t)))
    pendingTags.forEach(t => set.add(t))
    return Array.from(set).sort()
  }, [profiles, pendingTags])

  const handleCreateTag = (tag: string) => {
    if (!allTagsWithPending.includes(tag)) {
      setPendingTags(prev => [...prev, tag])
    }
  }

  const load = async () => {
    setLoading(true)
    try {
      const data = await fetchBrowserProfiles()
      setProfiles(data)
      // 清理已被窗口使用的 pendingTags
      const usedTags = new Set<string>()
      data.forEach(p => p.tags?.forEach(t => usedTags.add(t)))
      setPendingTags(prev => prev.filter(t => !usedTags.has(t)))
    } finally { setLoading(false) }
  }

  useEffect(() => { load() }, [])

  // 重置勾选当切换标签时
  useEffect(() => { setSelectedIds(new Set()) }, [selectedTag])

  const allTags = allTagsWithPending

  const profilesByTag = useMemo(() => {
    const map: Record<string, number> = {}
    profiles.forEach(p => p.tags?.forEach(t => { map[t] = (map[t] || 0) + 1 }))
    return map
  }, [profiles])

  const displayProfiles = useMemo(() => {
    if (selectedTag === null) return profiles
    return profiles.filter(p => p.tags?.includes(selectedTag))
  }, [profiles, selectedTag])

  // 勾选逻辑
  const isAllSelected = displayProfiles.length > 0 && displayProfiles.every(p => selectedIds.has(p.profileId))
  const isIndeterminate = !isAllSelected && displayProfiles.some(p => selectedIds.has(p.profileId))
  const toggleAll = () => {
    if (isAllSelected) setSelectedIds(new Set())
    else setSelectedIds(new Set(displayProfiles.map(p => p.profileId)))
  }
  const toggleOne = (id: string) => setSelectedIds(prev => {
    const next = new Set(prev); next.has(id) ? next.delete(id) : next.add(id); return next
  })

  // 批量添加标签
  const handleAddTags = async (tags: string[]) => {
    const ids = Array.from(selectedIds)
    setSaving(true)
    try {
      await batchSetProfileTags(ids, tags, false)
      toast.success(`已为 ${ids.length} 个窗口添加标签`)
      await load()
    } catch (e: any) {
      toast.error(e?.message || '操作失败')
    } finally { setSaving(false) }
  }

  // 批量移除标签
  const handleRemoveTags = async (tags: string[]) => {
    const ids = Array.from(selectedIds)
    setSaving(true)
    try {
      await batchRemoveProfileTags(ids, tags)
      toast.success(`已从 ${ids.length} 个窗口移除标签`)
      await load()
    } catch (e: any) {
      toast.error(e?.message || '操作失败')
    } finally { setSaving(false) }
  }

  // 重命名标签
  const handleRenameTag = async (oldName: string, newName: string) => {
    if (oldName === newName || !newName.trim()) return
    if (allTags.includes(newName.trim())) {
      toast.error('标签名称已存在')
      return
    }
    setSaving(true)
    try {
      await renameBrowserTag(oldName, newName.trim())
      toast.success('标签重命名成功')
      if (pendingTags.includes(oldName)) {
        setPendingTags(prev => prev.map(t => t === oldName ? newName.trim() : t))
      }
      if (selectedTag === oldName) {
        setSelectedTag(newName.trim())
      }
      await load()
    } catch (e: any) {
      toast.error(e?.message || '重命名失败')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="flex h-full animate-fade-in">
      {/* 左侧标签面板 */}
      <TagPanel
        tags={allTags}
        selected={selectedTag}
        profilesByTag={profilesByTag}
        totalCount={profiles.length}
        onSelect={setSelectedTag}
        onCreateTag={handleCreateTag}
        onRenameTag={handleRenameTag}
      />

      {/* 右侧内容区 */}
      <div className="flex-1 flex flex-col overflow-hidden p-5 gap-4">
        {/* 页头 */}
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">标签管理</h1>
            <p className="text-sm text-[var(--color-text-muted)] mt-0.5">
              {selectedTag ? (
                <>标签 <span className="text-[var(--color-primary)]">「{selectedTag}」</span> 下共 {displayProfiles.length} 个窗口</>
              ) : (
                <>全部窗口，共 {profiles.length} 个</>
              )}
            </p>
          </div>
        </div>

        {/* 批量操作栏 */}
        <ActionBar
          selectedCount={selectedIds.size}
          allTags={allTags}
          onAddTags={handleAddTags}
          onRemoveTags={handleRemoveTags}
          onClear={() => setSelectedIds(new Set())}
        />

        {/* 窗口表格 */}
        <Card padding="none" className="flex-1 overflow-hidden">
          <div className="overflow-auto h-full">
            <table className="min-w-full">
              <thead className="sticky top-0 z-10">
                <tr>
                  <th className="px-4 py-3 bg-[var(--color-bg-muted)] w-10">
                    <input
                      type="checkbox"
                      className="w-4 h-4 rounded cursor-pointer accent-[var(--color-accent)]"
                      checked={isAllSelected}
                      ref={el => { if (el) el.indeterminate = isIndeterminate }}
                      onChange={toggleAll}
                    />
                  </th>
                  <th className="px-4 py-3 text-xs font-semibold text-[var(--color-text-muted)] uppercase tracking-wider bg-[var(--color-bg-muted)] text-left">窗口名称</th>
                  <th className="px-4 py-3 text-xs font-semibold text-[var(--color-text-muted)] uppercase tracking-wider bg-[var(--color-bg-muted)] text-left">当前标签</th>
                  <th className="px-4 py-3 text-xs font-semibold text-[var(--color-text-muted)] uppercase tracking-wider bg-[var(--color-bg-muted)] text-left">状态</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-[var(--color-border-muted)] bg-[var(--color-bg-surface)]">
                {loading ? (
                  <tr><td colSpan={4} className="px-4 py-16 text-center text-sm text-[var(--color-text-muted)]">加载中...</td></tr>
                ) : displayProfiles.length === 0 ? (
                  <tr><td colSpan={4} className="px-4 py-16 text-center text-sm text-[var(--color-text-muted)]">暂无窗口</td></tr>
                ) : displayProfiles.map(p => (
                  <tr
                    key={p.profileId}
                    className={`transition-colors cursor-pointer ${selectedIds.has(p.profileId) ? 'bg-[var(--color-primary)]/5' : 'hover:bg-[var(--color-bg-muted)]/50'}`}
                    onClick={() => toggleOne(p.profileId)}
                  >
                    <td className="px-4 py-3" onClick={e => e.stopPropagation()}>
                      <input
                        type="checkbox"
                        className="w-4 h-4 rounded cursor-pointer accent-[var(--color-accent)]"
                        checked={selectedIds.has(p.profileId)}
                        onChange={() => toggleOne(p.profileId)}
                      />
                    </td>
                    <td className="px-4 py-3 text-sm font-medium text-[var(--color-text-primary)]">{p.profileName}</td>
                    <td className="px-4 py-3">
                      <div className="flex flex-wrap gap-1">
                        {p.tags?.length ? p.tags.map(t => (
                          <Badge key={t} variant={t === selectedTag ? 'info' : 'default'}>{t}</Badge>
                        )) : <span className="text-xs text-[var(--color-text-muted)]">无标签</span>}
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <Badge variant={p.running ? 'success' : 'warning'} dot>{p.running ? '运行中' : '已停止'}</Badge>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Card>

        {saving && (
          <div className="fixed inset-0 bg-black/20 z-50 flex items-center justify-center">
            <div className="bg-[var(--color-bg-elevated)] rounded-lg px-6 py-4 text-sm text-[var(--color-text-primary)] shadow-xl">
              保存中...
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
