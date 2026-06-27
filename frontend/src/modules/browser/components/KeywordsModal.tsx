import { useEffect, useState } from 'react'
import { Plus, Trash2 } from 'lucide-react'
import { Button, Input, Modal, toast } from '../../../shared/components'
import { setProfileKeywords } from '../api'

interface Props {
  profileId: string
  profileName: string
  initialKeywords: string[]
  open: boolean
  onClose: () => void
  onSaved: (keywords: string[]) => void
}

export function KeywordsModal({ profileId, profileName, initialKeywords, open, onClose, onSaved }: Props) {
  const [items, setItems] = useState<string[]>([])
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (!open) return
    const init = (initialKeywords || []).filter(Boolean)
    setItems(init.length > 0 ? [...init] : [''])
  }, [open, initialKeywords])

  const setItem = (i: number, val: string) =>
    setItems(prev => prev.map((v, idx) => idx === i ? val : v))

  const addItem = () => setItems(prev => [...prev, ''])

  const removeItem = (i: number) =>
    setItems(prev => prev.length === 1 ? [''] : prev.filter((_, idx) => idx !== i))

  const handleSave = async () => {
    const keywords = items.map(s => s.trim()).filter(Boolean)
    setSaving(true)
    try {
      await setProfileKeywords(profileId, keywords)
      toast.success('关键字已保存')
      onSaved(keywords)
      onClose()
    } catch {
      toast.error('保存失败')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={`关键字 — ${profileName}`}
      width="420px"
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>取消</Button>
          <Button onClick={handleSave} loading={saving}>保存</Button>
        </>
      }
    >
      <div className="space-y-2">
        <p className="text-xs text-[var(--color-text-muted)] mb-3">
          关键字附加在窗口上，例如账号、备注等，支持在列表页搜索。
        </p>

        {items.map((item, i) => (
          <div key={i} className="flex items-center gap-2">
            <Input
              value={item}
              onChange={e => setItem(i, e.target.value)}
              placeholder="输入关键字"
              className="flex-1"
            />
            <button
              onClick={() => removeItem(i)}
              className="p-1.5 text-[var(--color-text-muted)] hover:text-[var(--color-error)] hover:bg-[var(--color-bg-muted)] rounded transition-colors shrink-0"
            >
              <Trash2 className="w-4 h-4" />
            </button>
          </div>
        ))}

        <Button variant="ghost" size="sm" onClick={addItem} className="mt-1">
          <Plus className="w-4 h-4" />
          添加关键字
        </Button>
      </div>
    </Modal>
  )
}
