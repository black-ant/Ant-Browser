import { useEffect, useState } from 'react'
import { Plus, Trash2, RotateCcw, GripVertical, RefreshCw } from 'lucide-react'
import { Button, Card, ConfirmModal, Input, toast } from '../../../shared/components'
import type { BrowserBookmark } from '../types'
import { fetchBookmarks, resetBookmarks, saveBookmarks, syncBookmarksToProfiles } from '../api'

const fingerprintCheckBookmarkUrl = 'ant://fingerprint-check'

const isProtectedBookmark = (item: BrowserBookmark) =>
  item.url.trim().toLowerCase() === fingerprintCheckBookmarkUrl

export function BookmarkSettingsPage() {
  const [items, setItems] = useState<BrowserBookmark[]>([])
  const [saving, setSaving] = useState(false)
  const [syncing, setSyncing] = useState(false)
  const [resetOpen, setResetOpen] = useState(false)
  const [syncOpen, setSyncOpen] = useState(false)
  const [dragIndex, setDragIndex] = useState<number | null>(null)

  useEffect(() => {
    fetchBookmarks().then(setItems)
  }, [])

  const protectedItems = items
    .map((item, index) => ({ item, index }))
    .filter(({ item }) => isProtectedBookmark(item))
  const regularItems = items
    .map((item, index) => ({ item, index }))
    .filter(({ item }) => !isProtectedBookmark(item))

  const handleChange = (index: number, field: keyof BrowserBookmark, value: string) => {
    if (field !== 'openOnStart' && isProtectedBookmark(items[index])) return
    setItems(prev => prev.map((item, i) => i === index ? { ...item, [field]: value } : item))
  }

  const handleAdd = () => {
    setItems(prev => [...prev, { name: '', url: '', openOnStart: false }])
  }

  const handleDelete = (index: number) => {
    if (isProtectedBookmark(items[index])) return
    setItems(prev => prev.filter((_, i) => i !== index))
  }

  const handleOpenOnStartChange = (index: number, checked: boolean) => {
    setItems(prev => prev.map((item, i) => i === index ? { ...item, openOnStart: checked } : item))
  }

  const handleSave = async () => {
    const valid = items.filter(i => i.name.trim() && i.url.trim())
    if (valid.length !== items.length) {
      toast.error('存在空的名称或 URL，请填写完整后保存')
      return
    }
    setSaving(true)
    try {
      await saveBookmarks(items)
      const result = await syncBookmarksToProfiles()
      const parts = ['书签已保存']
      if (result.synced > 0) parts.push(`已同步 ${result.synced} 个已有实例`)
      if (result.skipped > 0) parts.push(`跳过运行中 ${result.skipped} 个，停止后再同步`)
      if (result.failed > 0) parts.push(`失败 ${result.failed} 个`)
      const message = parts.join('，')
      if (result.failed > 0 || result.skipped > 0) {
        toast.warning(message)
      } else {
        toast.success(message)
      }
    } finally {
      setSaving(false)
    }
  }

  const handleReset = async () => {
    await resetBookmarks()
    const fresh = await fetchBookmarks()
    setItems(fresh)
    toast.success('已恢复默认书签')
  }

  const handleSync = async () => {
    setSyncing(true)
    try {
      const result = await syncBookmarksToProfiles()
      const parts = [`已同步 ${result.synced} 个实例`]
      if (result.skipped > 0) parts.push(`跳过运行中 ${result.skipped} 个，停止后再同步`)
      if (result.failed > 0) parts.push(`失败 ${result.failed} 个`)
      const message = parts.join('，')
      if (result.failed > 0 || result.skipped > 0) {
        toast.warning(message)
      } else {
        toast.success(message)
      }
      setSyncOpen(false)
    } catch (error: any) {
      toast.error(error?.message || '同步失败')
    } finally {
      setSyncing(false)
    }
  }

  // 拖拽排序
  const handleDragStart = (index: number) => {
    if (isProtectedBookmark(items[index])) return
    setDragIndex(index)
  }
  const handleDragOver = (e: React.DragEvent, index: number) => {
    e.preventDefault()
    if (dragIndex === null || dragIndex === index) return
    if (isProtectedBookmark(items[dragIndex]) || isProtectedBookmark(items[index])) return
    setItems(prev => {
      const next = [...prev]
      const [moved] = next.splice(dragIndex, 1)
      next.splice(index, 0, moved)
      return next
    })
    setDragIndex(index)
  }
  const handleDragEnd = () => setDragIndex(null)

  return (
    <div className="space-y-4 animate-fade-in">
      <Card padding="none" className="shadow-[var(--shadow-sm)]">
        <div className="flex flex-col gap-3 px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
          <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">默认书签</h1>
          <div className="flex flex-wrap gap-2 sm:justify-end">
            <Button variant="secondary" size="sm" onClick={() => setSyncOpen(true)} loading={syncing}>
              <RefreshCw className="w-4 h-4 mr-1.5" />
              手动同步
            </Button>
            <Button variant="secondary" size="sm" onClick={() => setResetOpen(true)}>
              <RotateCcw className="w-4 h-4 mr-1.5" />
              恢复默认
            </Button>
            <Button size="sm" onClick={handleSave} loading={saving}>保存</Button>
          </div>
        </div>
      </Card>

      <Card title={`内置检测（${protectedItems.length} 项）`}>
        <div className="space-y-2">
          {protectedItems.map(({ item, index }) => (
            <div
              key={`${item.url}-${index}`}
              className="flex items-center gap-2 p-2.5 rounded-xl bg-[var(--color-bg-muted)] shadow-[var(--shadow-sm)]"
            >
              <GripVertical className="w-4 h-4 text-[var(--color-text-muted)] opacity-40 shrink-0" />
              <Input
                value={item.name}
                readOnly
                className="w-36 shrink-0 bg-[var(--color-bg-subtle)]"
              />
              <Input
                value={item.url}
                readOnly
                className="flex-1 bg-[var(--color-bg-subtle)]"
              />
              <label className="flex items-center gap-1.5 px-2 text-xs text-[var(--color-text-secondary)] whitespace-nowrap select-none">
                <input
                  type="checkbox"
                  checked={Boolean(item.openOnStart)}
                  onChange={e => handleOpenOnStartChange(index, e.target.checked)}
                  className="h-4 w-4 rounded border-[var(--color-border-default)] accent-[var(--color-accent)]"
                />
                启动打开
              </label>
              <span className="px-2 py-1 rounded-lg bg-[var(--color-bg-subtle)] text-xs text-[var(--color-text-muted)] shrink-0">
                内置
              </span>
            </div>
          ))}
        </div>
      </Card>

      <Card title={`书签列表（${regularItems.length} 项）`}>
        <div className="space-y-2">
          {regularItems.map(({ item, index }) => (
            <div
              key={`${item.url}-${index}`}
              draggable
              onDragStart={() => handleDragStart(index)}
              onDragOver={e => handleDragOver(e, index)}
              onDragEnd={handleDragEnd}
              className={`flex items-center gap-2 p-2.5 rounded-xl shadow-[var(--shadow-sm)] transition-all duration-150 ${
                dragIndex === index
                  ? 'bg-[var(--color-accent-muted)] ring-1 ring-[var(--color-border-strong)]'
                  : 'bg-[var(--color-bg-muted)] hover:bg-[var(--color-bg-subtle)]'
              }`}
            >
              <GripVertical className="w-4 h-4 text-[var(--color-text-muted)] cursor-grab shrink-0" />
              <Input
                value={item.name}
                onChange={e => handleChange(index, 'name', e.target.value)}
                placeholder="名称，如 Google"
                className="w-36 shrink-0"
              />
              <Input
                value={item.url}
                onChange={e => handleChange(index, 'url', e.target.value)}
                placeholder="https://..."
                className="flex-1"
              />
              <label className="flex items-center gap-1.5 px-2 text-xs text-[var(--color-text-secondary)] whitespace-nowrap select-none">
                <input
                  type="checkbox"
                  checked={Boolean(item.openOnStart)}
                  onChange={e => handleOpenOnStartChange(index, e.target.checked)}
                  className="h-4 w-4 rounded border-[var(--color-border-default)] accent-[var(--color-accent)]"
                />
                启动打开
              </label>
              <button
                type="button"
                onClick={() => handleDelete(index)}
                className="p-1.5 rounded text-[var(--color-text-muted)] hover:text-red-500 hover:bg-red-50 transition-colors shrink-0"
              >
                <Trash2 className="w-4 h-4" />
              </button>
            </div>
          ))}

          {regularItems.length === 0 && (
            <p className="text-sm text-[var(--color-text-muted)] text-center py-6">
              暂无书签，点击下方按钮添加
            </p>
          )}
        </div>

        <button
          type="button"
          onClick={handleAdd}
          className="mt-3 w-full flex items-center justify-center gap-2 py-2.5 rounded-xl bg-[var(--color-bg-muted)] text-sm text-[var(--color-text-primary)] shadow-[var(--shadow-sm)] hover:bg-[var(--color-bg-subtle)] transition-colors"
        >
          <Plus className="w-4 h-4" />
          添加书签
        </button>
      </Card>

      <ConfirmModal
        open={resetOpen}
        onClose={() => setResetOpen(false)}
        onConfirm={handleReset}
        title="恢复默认书签"
        content="将清除当前所有自定义书签，恢复为内置默认列表。确定继续？"
        confirmText="确定恢复"
        danger
      />

      <ConfirmModal
        open={syncOpen}
        onClose={() => setSyncOpen(false)}
        onConfirm={handleSync}
        title="手动同步已有实例"
        content="只会增量追加缺失的默认书签，不会删除、改名或移动用户已有书签。运行中的实例会跳过。"
        confirmText="开始同步"
      />
    </div>
  )
}
