import { useEffect, useState } from 'react'
import { Plus, Trash2, RotateCcw, GripVertical } from 'lucide-react'
import { Button, Card, ConfirmModal, Input, toast } from '../../../shared/components'
import type { BrowserBookmark } from '../types'
import { fetchBookmarks, resetBookmarks, saveBookmarks } from '../api'

export function BookmarkSettingsPage() {
  const [items, setItems] = useState<BrowserBookmark[]>([])
  const [saving, setSaving] = useState(false)
  const [resetOpen, setResetOpen] = useState(false)
  const [dragIndex, setDragIndex] = useState<number | null>(null)

  useEffect(() => {
    fetchBookmarks().then(setItems)
  }, [])

  const handleChange = (index: number, field: keyof BrowserBookmark, value: string) => {
    setItems(prev => prev.map((item, i) => i === index ? { ...item, [field]: value } : item))
  }

  const handleAdd = () => {
    setItems(prev => [...prev, { name: '', url: '' }])
  }

  const handleDelete = (index: number) => {
    setItems(prev => prev.filter((_, i) => i !== index))
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
      toast.success('书签已保存，下次新建窗口时生效')
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

  // 拖拽排序
  const handleDragStart = (index: number) => setDragIndex(index)
  const handleDragOver = (e: React.DragEvent, index: number) => {
    e.preventDefault()
    if (dragIndex === null || dragIndex === index) return
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
    <div className="space-y-5 animate-fade-in">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">默认书签</h1>
          <p className="text-sm text-[var(--color-text-muted)] mt-1">新建窗口首次启动时自动写入书签栏，已有书签不受影响</p>
        </div>
        <div className="flex gap-2">
          <Button variant="secondary" size="sm" onClick={() => setResetOpen(true)}>
            <RotateCcw className="w-4 h-4 mr-1.5" />
            恢复默认
          </Button>
          <Button size="sm" onClick={handleSave} loading={saving}>保存</Button>
        </div>
      </div>

      <Card title={`书签列表（${items.length} 项）`} subtitle="拖拽左侧图标可调整顺序">
        <div className="space-y-2">
          {items.map((item, index) => (
            <div
              key={index}
              draggable
              onDragStart={() => handleDragStart(index)}
              onDragOver={e => handleDragOver(e, index)}
              onDragEnd={handleDragEnd}
              className={`flex items-center gap-2 p-2 rounded-lg border transition-colors ${
                dragIndex === index
                  ? 'border-[var(--color-primary)] bg-[var(--color-bg-hover)]'
                  : 'border-[var(--color-border)] hover:border-[var(--color-border-hover)]'
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
              <button
                type="button"
                onClick={() => handleDelete(index)}
                className="p-1.5 rounded text-[var(--color-text-muted)] hover:text-red-500 hover:bg-red-50 transition-colors shrink-0"
              >
                <Trash2 className="w-4 h-4" />
              </button>
            </div>
          ))}

          {items.length === 0 && (
            <p className="text-sm text-[var(--color-text-muted)] text-center py-6">
              暂无书签，点击下方按钮添加
            </p>
          )}
        </div>

        <button
          type="button"
          onClick={handleAdd}
          className="mt-3 w-full flex items-center justify-center gap-2 py-2 rounded-lg border border-dashed border-[var(--color-border)] text-sm text-[var(--color-text-muted)] hover:border-[var(--color-primary)] hover:text-[var(--color-primary)] transition-colors"
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
    </div>
  )
}
