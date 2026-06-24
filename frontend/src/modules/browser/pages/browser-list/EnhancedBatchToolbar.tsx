import { Play, Square, Trash2, CheckSquare, X } from 'lucide-react'
import { Button, Badge } from '../../../../shared/components'

interface EnhancedBatchToolbarProps {
  selectedCount: number
  totalCount: number
  onSelectAll: () => void
  onDeselectAll: () => void
  onBatchStart: () => void
  onBatchStop: () => void
  onBatchDelete: () => void
  batchLoading: boolean
}

export function EnhancedBatchToolbar({
  selectedCount,
  totalCount,
  onSelectAll,
  onDeselectAll,
  onBatchStart,
  onBatchStop,
  onBatchDelete,
  batchLoading,
}: EnhancedBatchToolbarProps) {
  if (selectedCount === 0) return null

  return (
    <div className="flex items-center justify-between px-4 py-3 bg-[var(--color-accent)]/5 border border-[var(--color-accent)]/20 rounded-xl animate-fade-in">
      {/* 左侧：选中信息 */}
      <div className="flex items-center gap-3">
        <Badge variant="info" size="md">
          已选 {selectedCount} 项
        </Badge>
        {selectedCount < totalCount ? (
          <button
            onClick={onSelectAll}
            className="text-sm text-[var(--color-accent)] hover:underline flex items-center gap-1"
          >
            <CheckSquare className="w-3.5 h-3.5" />
            全选
          </button>
        ) : (
          <button
            onClick={onDeselectAll}
            className="text-sm text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)] flex items-center gap-1"
          >
            <X className="w-3.5 h-3.5" />
            取消全选
          </button>
        )}
      </div>

      {/* 右侧：批量操作按钮 */}
      <div className="flex items-center gap-2">
        <Button
          variant="secondary"
          size="sm"
          onClick={onBatchStart}
          disabled={batchLoading}
        >
          <Play className="w-4 h-4" />
          批量启动
        </Button>
        <Button
          variant="secondary"
          size="sm"
          onClick={onBatchStop}
          disabled={batchLoading}
        >
          <Square className="w-4 h-4" />
          批量停止
        </Button>
        <Button
          variant="danger"
          size="sm"
          onClick={onBatchDelete}
          disabled={batchLoading}
        >
          <Trash2 className="w-4 h-4" />
          批量删除
        </Button>
      </div>
    </div>
  )
}
