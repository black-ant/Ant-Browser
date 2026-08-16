import { useState } from 'react'

import { Button, Modal, toast } from '../../../shared/components'
import { batchRemoveProfileTags, batchSetProfileTags } from '../api'

interface BatchTagModalProps {
  open: boolean
  profileIds: string[]
  allTags: string[]
  onClose: () => void
  onDone: () => void
}

export function BatchTagModal({ open, profileIds, allTags, onClose, onDone }: BatchTagModalProps) {
  const [addInput, setAddInput] = useState('')
  const [removeTag, setRemoveTag] = useState('')
  const [busy, setBusy] = useState(false)

  const doAdd = async () => {
    const tags = addInput.split(/[,，\s]+/).map((t) => t.trim()).filter(Boolean)
    if (!tags.length) return
    setBusy(true)
    try {
      await batchSetProfileTags(profileIds, tags, false)
      toast.success(`已为 ${profileIds.length} 个实例添加标签`)
      setAddInput('')
      onDone()
    } catch (e: any) { toast.error(e?.message || '添加失败') } finally { setBusy(false) }
  }

  const doRemove = async () => {
    if (!removeTag) return
    setBusy(true)
    try {
      await batchRemoveProfileTags(profileIds, [removeTag])
      toast.success(`已从 ${profileIds.length} 个实例移除标签`)
      setRemoveTag('')
      onDone()
    } catch (e: any) { toast.error(e?.message || '移除失败') } finally { setBusy(false) }
  }

  return (
    <Modal open={open} onClose={onClose} title={`批量标签(已选 ${profileIds.length} 个)`} width="420px"
      footer={<Button variant="secondary" onClick={onClose} disabled={busy}>关闭</Button>}>
      <div className="space-y-4">
        <div>
          <div className="mb-1 text-sm text-[var(--color-text-secondary)]">添加标签(逗号分隔)</div>
          <div className="flex gap-2">
            <input value={addInput} onChange={(e) => setAddInput(e.target.value)} onKeyDown={(e) => e.key === 'Enter' && doAdd()}
              placeholder="如:养号,抖音" className="flex-1 px-2 py-1.5 text-sm rounded border border-[var(--color-border-default)] bg-[var(--color-bg-input)] text-[var(--color-text-primary)] focus:outline-none focus:border-[var(--color-accent)]" />
            <Button size="sm" onClick={doAdd} loading={busy} disabled={!addInput.trim()}>添加</Button>
          </div>
        </div>
        {allTags.length > 0 && (
          <div>
            <div className="mb-1 text-sm text-[var(--color-text-secondary)]">移除标签</div>
            <div className="flex gap-2">
              <select value={removeTag} onChange={(e) => setRemoveTag(e.target.value)}
                className="flex-1 px-2 py-1.5 text-sm rounded border border-[var(--color-border-default)] bg-[var(--color-bg-input)] text-[var(--color-text-primary)] focus:outline-none focus:border-[var(--color-accent)]">
                <option value="">选择要移除的标签</option>
                {allTags.map((t) => <option key={t} value={t}>{t}</option>)}
              </select>
              <Button size="sm" variant="secondary" onClick={doRemove} loading={busy} disabled={!removeTag}>移除</Button>
            </div>
          </div>
        )}
      </div>
    </Modal>
  )
}
