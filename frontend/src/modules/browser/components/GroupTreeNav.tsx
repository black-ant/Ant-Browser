import { useState, useMemo } from 'react'
import { ChevronRight, ChevronDown, Folder, FolderOpen, Plus, Pencil, Trash2, FolderInput } from 'lucide-react'
import type { BrowserGroupWithCount, BrowserGroupInput } from '../types'
import { createGroup, updateGroup, deleteGroup } from '../api'
import { useConfirm } from '../../../shared/components'

interface GroupTreeNavProps {
  groups: BrowserGroupWithCount[]
  selectedGroupId: string | null
  onSelectGroup: (groupId: string | null) => void
  onRefresh: () => void
}

interface TreeNode extends BrowserGroupWithCount {
  children: TreeNode[]
  level: number
}

// 构建树形结构
function buildTree(groups: BrowserGroupWithCount[]): TreeNode[] {
  const map = new Map<string, TreeNode>()
  const roots: TreeNode[] = []

  // 初始化所有节点
  groups.forEach(g => {
    map.set(g.groupId, { ...g, children: [], level: 0 })
  })

  // 构建父子关系
  groups.forEach(g => {
    const node = map.get(g.groupId)!
    if (g.parentId && map.has(g.parentId)) {
      const parent = map.get(g.parentId)!
      node.level = parent.level + 1
      parent.children.push(node)
    } else {
      roots.push(node)
    }
  })

  // 按 sortOrder 排序
  const sortNodes = (nodes: TreeNode[]) => {
    nodes.sort((a, b) => a.sortOrder - b.sortOrder)
    nodes.forEach(n => sortNodes(n.children))
  }
  sortNodes(roots)

  return roots
}

export function GroupTreeNav({ groups, selectedGroupId, onSelectGroup, onRefresh }: GroupTreeNavProps) {
  const [expanded, setExpanded] = useState<Set<string>>(new Set())
  const [showCreateModal, setShowCreateModal] = useState(false)
  const [createParentId, setCreateParentId] = useState<string>('')
  const [newGroupName, setNewGroupName] = useState('')
  const [editingGroup, setEditingGroup] = useState<BrowserGroupWithCount | null>(null)
  const [contextMenu, setContextMenu] = useState<{ x: number; y: number; group: BrowserGroupWithCount } | null>(null)

  const tree = useMemo(() => buildTree(groups), [groups])

  const toggleExpand = (groupId: string) => {
    setExpanded(prev => {
      const next = new Set(prev)
      if (next.has(groupId)) {
        next.delete(groupId)
      } else {
        next.add(groupId)
      }
      return next
    })
  }

  const handleCreate = async () => {
    if (!newGroupName.trim()) return
    const input: BrowserGroupInput = {
      groupName: newGroupName.trim(),
      parentId: createParentId,
      sortOrder: 0,
    }
    await createGroup(input)
    setShowCreateModal(false)
    setNewGroupName('')
    setCreateParentId('')
    onRefresh()
  }

  const handleRename = async () => {
    if (!editingGroup || !newGroupName.trim()) return
    const input: BrowserGroupInput = {
      groupName: newGroupName.trim(),
      parentId: editingGroup.parentId,
      sortOrder: editingGroup.sortOrder,
    }
    await updateGroup(editingGroup.groupId, input)
    setEditingGroup(null)
    setNewGroupName('')
    onRefresh()
  }

  const { confirm, dialog: confirmDialog } = useConfirm()

  const handleDelete = async (groupId: string) => {
    if (!(await confirm({ content: '确定删除此分组？子分组和实例将移动到父分组。', danger: true }))) return
    await deleteGroup(groupId)
    if (selectedGroupId === groupId) {
      onSelectGroup(null)
    }
    onRefresh()
  }

  const handleContextMenu = (e: React.MouseEvent, group: BrowserGroupWithCount) => {
    e.preventDefault()
    setContextMenu({ x: e.clientX, y: e.clientY, group })
  }

  const renderNode = (node: TreeNode) => {
    const isExpanded = expanded.has(node.groupId)
    const isSelected = selectedGroupId === node.groupId
    const hasChildren = node.children.length > 0

    return (
      <div key={node.groupId}>
        <div
          className={`flex items-center gap-2 px-3 py-1.5 cursor-pointer rounded hover:bg-gray-100 dark:hover:bg-gray-700 ${
            isSelected ? 'bg-blue-50 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400' : ''
          }`}
          style={{ paddingLeft: `${node.level * 16 + 12}px` }}
          onClick={() => onSelectGroup(node.groupId)}
          onContextMenu={(e) => handleContextMenu(e, node)}
        >
          {hasChildren ? (
            <button
              className="p-0 hover:bg-gray-200 dark:hover:bg-gray-600 rounded shrink-0"
              onClick={(e) => { e.stopPropagation(); toggleExpand(node.groupId) }}
            >
              {isExpanded ? <ChevronDown className="w-3.5 h-3.5" /> : <ChevronRight className="w-3.5 h-3.5" />}
            </button>
          ) : null}
          {isExpanded && hasChildren ? (
            <FolderOpen className="w-4 h-4 text-yellow-500 shrink-0" />
          ) : (
            <Folder className="w-4 h-4 text-yellow-500 shrink-0" />
          )}
          <span className="flex-1 truncate text-sm">{node.groupName}</span>
          <span className="text-xs text-gray-400">{node.instanceCount}</span>
        </div>
        {isExpanded && node.children.map(child => renderNode(child))}
      </div>
    )
  }

  return (
    <div className="w-48 border-r border-gray-200 dark:border-gray-700 flex flex-col h-full">
      {confirmDialog}
      <div className="p-2 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
        <span className="text-sm font-medium">分组</span>
        <button
          className="p-1 hover:bg-gray-100 dark:hover:bg-gray-700 rounded"
          onClick={() => { setCreateParentId(''); setShowCreateModal(true) }}
          title="新建分组"
        >
          <Plus className="w-4 h-4" />
        </button>
      </div>

      <div className="flex-1 overflow-y-auto py-1">
        {/* 全部 */}
        <div
          className={`flex items-center gap-2 px-3 py-1.5 cursor-pointer rounded mx-1 hover:bg-gray-100 dark:hover:bg-gray-700 ${
            selectedGroupId === null ? 'bg-blue-50 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400' : ''
          }`}
          onClick={() => onSelectGroup(null)}
        >
          <Folder className="w-4 h-4 text-gray-400" />
          <span className="flex-1 text-sm">全部</span>
        </div>

        {/* 未分组 */}
        <div
          className={`flex items-center gap-2 px-3 py-1.5 cursor-pointer rounded mx-1 hover:bg-gray-100 dark:hover:bg-gray-700 ${
            selectedGroupId === '__ungrouped__' ? 'bg-blue-50 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400' : ''
          }`}
          onClick={() => onSelectGroup('__ungrouped__')}
        >
          <FolderInput className="w-4 h-4 text-gray-400" />
          <span className="flex-1 text-sm">未分组</span>
        </div>

        {/* 分组树 */}
        {tree.length > 0 && (
          <div className="mt-2 mx-1">
            <div className="px-2 py-1 text-xs font-medium text-gray-400 uppercase tracking-wider">我的分组</div>
            {tree.map(node => renderNode(node))}
          </div>
        )}
      </div>

      {/* 创建分组弹窗 */}
      {showCreateModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={() => setShowCreateModal(false)}>
          <div className="bg-white dark:bg-gray-800 rounded-lg p-4 w-80" onClick={e => e.stopPropagation()}>
            <h3 className="text-lg font-medium mb-3">新建分组</h3>
            <input
              type="text"
              className="w-full px-3 py-2 border rounded dark:bg-gray-700 dark:border-gray-600"
              placeholder="分组名称"
              value={newGroupName}
              onChange={e => setNewGroupName(e.target.value)}
              autoFocus
            />
            {groups.length > 0 && (
              <select
                className="w-full mt-2 px-3 py-2 border rounded dark:bg-gray-700 dark:border-gray-600"
                value={createParentId}
                onChange={e => setCreateParentId(e.target.value)}
              >
                <option value="">根级分组</option>
                {groups.map(g => (
                  <option key={g.groupId} value={g.groupId}>{g.groupName}</option>
                ))}
              </select>
            )}
            <div className="flex justify-end gap-2 mt-4">
              <button className="px-3 py-1.5 text-sm rounded hover:bg-gray-100 dark:hover:bg-gray-700" onClick={() => setShowCreateModal(false)}>
                取消
              </button>
              <button className="px-3 py-1.5 text-sm bg-blue-500 text-white rounded hover:bg-blue-600" onClick={handleCreate}>
                创建
              </button>
            </div>
          </div>
        </div>
      )}

      {/* 重命名弹窗 */}
      {editingGroup && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={() => setEditingGroup(null)}>
          <div className="bg-white dark:bg-gray-800 rounded-lg p-4 w-80" onClick={e => e.stopPropagation()}>
            <h3 className="text-lg font-medium mb-3">重命名分组</h3>
            <input
              type="text"
              className="w-full px-3 py-2 border rounded dark:bg-gray-700 dark:border-gray-600"
              placeholder="分组名称"
              value={newGroupName}
              onChange={e => setNewGroupName(e.target.value)}
              autoFocus
            />
            <div className="flex justify-end gap-2 mt-4">
              <button className="px-3 py-1.5 text-sm rounded hover:bg-gray-100 dark:hover:bg-gray-700" onClick={() => setEditingGroup(null)}>
                取消
              </button>
              <button className="px-3 py-1.5 text-sm bg-blue-500 text-white rounded hover:bg-blue-600" onClick={handleRename}>
                保存
              </button>
            </div>
          </div>
        </div>
      )}

      {/* 右键菜单 */}
      {contextMenu && (
        <div
          className="fixed bg-white dark:bg-gray-800 border dark:border-gray-700 rounded shadow-lg py-1 z-50"
          style={{ left: contextMenu.x, top: contextMenu.y }}
          onClick={() => setContextMenu(null)}
        >
          <button
            className="w-full px-4 py-1.5 text-sm text-left hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-2"
            onClick={() => { setCreateParentId(contextMenu.group.groupId); setShowCreateModal(true) }}
          >
            <Plus className="w-4 h-4" /> 新建子分组
          </button>
          <button
            className="w-full px-4 py-1.5 text-sm text-left hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-2"
            onClick={() => { setNewGroupName(contextMenu.group.groupName); setEditingGroup(contextMenu.group) }}
          >
            <Pencil className="w-4 h-4" /> 重命名
          </button>
          <button
            className="w-full px-4 py-1.5 text-sm text-left hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-2 text-red-500"
            onClick={() => handleDelete(contextMenu.group.groupId)}
          >
            <Trash2 className="w-4 h-4" /> 删除
          </button>
        </div>
      )}

      {/* 点击其他地方关闭右键菜单 */}
      {contextMenu && (
        <div className="fixed inset-0 z-40" onClick={() => setContextMenu(null)} />
      )}
    </div>
  )
}
