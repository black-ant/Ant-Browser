import { Search, RefreshCw, Plus, Filter, X, LayoutGrid, List } from 'lucide-react'
import { Button, Input } from '../../../../shared/components'
import { Link } from 'react-router-dom'

interface EnhancedToolbarProps {
  searchQuery: string
  onSearchChange: (query: string) => void
  onRefresh: () => void
  viewMode: 'card' | 'table'
  onViewModeChange: (mode: 'card' | 'table') => void
  filterActive: boolean
  onToggleFilter: () => void
  refreshing?: boolean
}

export function EnhancedToolbar({
  searchQuery,
  onSearchChange,
  onRefresh,
  viewMode,
  onViewModeChange,
  filterActive,
  onToggleFilter,
  refreshing = false,
}: EnhancedToolbarProps) {
  return (
    <div className="flex items-center gap-3 px-4 py-3 bg-[var(--color-bg-surface)] border-b border-[var(--color-border-default)]">
      {/* 搜索框 */}
      <div className="relative flex-1 max-w-md">
        <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-[var(--color-text-muted)]" />
        <Input
          value={searchQuery}
          onChange={(e) => onSearchChange(e.target.value)}
          placeholder="搜索窗口名称、标签、关键字..."
          className="pl-10 pr-9 h-9"
        />
        {searchQuery && (
          <button
            onClick={() => onSearchChange('')}
            className="absolute right-2 top-1/2 -translate-y-1/2 p-1 hover:bg-[var(--color-bg-muted)] rounded transition-colors"
          >
            <X className="w-4 h-4 text-[var(--color-text-muted)]" />
          </button>
        )}
      </div>

      {/* 工具按钮组 */}
      <div className="flex items-center gap-2">
        {/* 筛选按钮 */}
        <Button
          variant={filterActive ? 'primary' : 'secondary'}
          size="sm"
          onClick={onToggleFilter}
          className="flex-shrink-0"
        >
          <Filter className="w-4 h-4" />
          筛选
        </Button>

        {/* 刷新按钮 */}
        <Button
          variant="secondary"
          size="sm"
          onClick={onRefresh}
          disabled={refreshing}
          className="flex-shrink-0"
        >
          <RefreshCw className={`w-4 h-4 ${refreshing ? 'animate-spin' : ''}`} />
          刷新
        </Button>

        {/* 视图切换 */}
        <div className="flex items-center bg-[var(--color-bg-muted)] rounded-lg border border-[var(--color-border-default)] p-0.5">
          <button
            className={`p-1.5 rounded transition-all ${
              viewMode === 'table'
                ? 'bg-[var(--color-bg-surface)] shadow-sm text-[var(--color-accent)]'
                : 'text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)]'
            }`}
            onClick={() => onViewModeChange('table')}
            title="表格视图"
          >
            <List className="w-4 h-4" />
          </button>
          <button
            className={`p-1.5 rounded transition-all ${
              viewMode === 'card'
                ? 'bg-[var(--color-bg-surface)] shadow-sm text-[var(--color-accent)]'
                : 'text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)]'
            }`}
            onClick={() => onViewModeChange('card')}
            title="卡片视图"
          >
            <LayoutGrid className="w-4 h-4" />
          </button>
        </div>

        {/* 分隔线 */}
        <div className="w-px h-5 bg-[var(--color-border-default)]" />

        {/* 新建按钮 */}
        <Link to="/browser/create">
          <Button size="sm">
            <Plus className="w-4 h-4" />
            新建窗口
          </Button>
        </Link>
      </div>
    </div>
  )
}
