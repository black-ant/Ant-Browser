import { useEffect, useState } from 'react'
import { Plus, Trash2, Check, FolderInput } from 'lucide-react'

import { Button, Modal, toast } from '../../../shared/components'
import type { BrowserGroupWithCount } from '../types'
import { createGroup, deleteGroup, fetchGroups, moveInstancesToGroup, updateGroup } from '../api'

interface GroupOpsModalProps {
  open: boolean
  profileIds: string[]
  onClose: () => void
  onDone: () => void
}

// 分组入口:上半"移动选中环境到某分组/未分组/新建",下半"管理分组(改名/删除/新建)"。
export function GroupOpsModal({ open, profileIds, onClose, onDone }: GroupOpsModalProps) {
  const [groups, setGroups] = useState<BrowserGroupWithCount[]>([])
  const [busy, setBusy] = useState(false)
  const [newName, setNewName] = useState('')
  const [editingId, setEditingId] = useState<string | null>(null)
  const [editName, setEditName] = useState('')

  const load = async () => { setGroups(await fetchGroups()) }
  useEffect(() => { if (open) void load() }, [open])

  const move = async (groupId: string) => {
    if (!profileIds.length) { toast.error('请先勾选环境'); return }
    setBusy(true)
    try {
      await moveInstancesToGroup(profileIds, groupId)
      toast.success(groupId ? '已移动到分组' : '已移出分组')
      onDone()
    } catch (e: any) { toast.error(e?.message || '移动失败') } finally { setBusy(false) }
  }

  const addGroup = async (moveAfter: boolean) => {
    const name = newName.trim()
    if (!name) return
    setBusy(true)
    try {
      const g = await createGroup({ groupName: name, parentId: '', sortOrder: 0 })
      setNewName('')
      await load()
      if (moveAfter && g?.groupId) await move(g.groupId)
      else toast.success('分组已创建')
    } catch (e: any) { toast.error(e?.message || '创建失败') } finally { setBusy(false) }
  }

  const rename = async (groupId: string) => {
    const name = editName.trim()
    if (!name) { setEditingId(null); return }
    setBusy(true)
    try {
      await updateGroup(groupId, { groupName: name, parentId: '', sortOrder: 0 })
      setEditingId(null)
      await load()
      onDone()
    } catch (e: any) { toast.error(e?.message || '重命名失败') } finally { setBusy(false) }
  }

  const remove = async (groupId: string) => {
    const name = groups.find((g) => g.groupId === groupId)?.groupName ?? ''
    if (!window.confirm(`确定删除分组「${name}」?其下环境将移动到未分组/父级。`)) return
    setBusy(true)
    try {
      await deleteGroup(groupId)
      await load()
      onDone()
    } catch (e: any) { toast.error(e?.message || '删除失败') } finally { setBusy(false) }
  }

  return (
    <Modal open={open} onClose={onClose} title={`分组(已选 ${profileIds.length} 个)`} width="480px"
      footer={<Button variant="secondary" onClick={onClose} disabled={busy}>关闭</Button>}>
      <div className="space-y-5">
        <div>
          <div className="mb-2 text-sm font-medium text-[var(--color-text-secondary)]">移动选中环境到</div>
          <div className="flex flex-wrap gap-2">
            <Button size="sm" variant="secondary" onClick={() => move('')} disabled={busy || !profileIds.length}>
              <FolderInput className="w-3.5 h-3.5" />未分组(移出)
            </Button>
            {groups.map((g) => (
              <Button key={g.groupId} size="sm" variant="secondary" onClick={() => move(g.groupId)} disabled={busy || !profileIds.length}>
                {g.groupName} <span className="opacity-60">({g.instanceCount})</span>
              </Button>
            ))}
          </div>
          <div className="mt-2 flex gap-2">
            <input value={newName} onChange={(e) => setNewName(e.target.value)} placeholder="新建分组名"
              className="flex-1 px-2 py-1.5 text-sm rounded border border-[var(--color-border-default)] bg-[var(--color-bg-input)] text-[var(--color-text-primary)] focus:outline-none focus:border-[var(--color-accent)]" />
            <Button size="sm" onClick={() => addGroup(true)} loading={busy} disabled={!newName.trim()}>
              <Plus className="w-3.5 h-3.5" />新建并移入
            </Button>
          </div>
        </div>

        <div className="border-t border-[var(--color-border-default)] pt-3">
          <div className="mb-2 text-sm font-medium text-[var(--color-text-secondary)]">管理分组</div>
          <div className="space-y-1.5 max-h-56 overflow-y-auto">
            {groups.length === 0 && <div className="text-xs text-[var(--color-text-muted)]">暂无分组</div>}
            {groups.map((g) => (
              <div key={g.groupId} className="flex items-center gap-2">
                {editingId === g.groupId ? (
                  <input autoFocus value={editName} onChange={(e) => setEditName(e.target.value)}
                    onKeyDown={(e) => { if (e.key === 'Enter') rename(g.groupId); if (e.key === 'Escape') setEditingId(null) }}
                    className="flex-1 px-2 py-1 text-xs rounded border border-[var(--color-accent)] bg-[var(--color-bg-input)] text-[var(--color-text-primary)] focus:outline-none" />
                ) : (
                  <button className="flex-1 text-left text-sm text-[var(--color-text-primary)]" onClick={() => { setEditingId(g.groupId); setEditName(g.groupName) }}>
                    {g.groupName} <span className="text-xs opacity-60">({g.instanceCount})</span>
                  </button>
                )}
                {editingId === g.groupId ? (
                  <button onClick={() => rename(g.groupId)} className="p-1 text-[var(--color-accent)]"><Check className="w-4 h-4" /></button>
                ) : (
                  <button onClick={() => remove(g.groupId)} disabled={busy} className="p-1 text-[var(--color-text-muted)] hover:text-red-500 disabled:opacity-50"><Trash2 className="w-4 h-4" /></button>
                )}
              </div>
            ))}
          </div>
        </div>
      </div>
    </Modal>
  )
}
