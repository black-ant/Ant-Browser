import { Link } from 'react-router-dom'
import { Square, Play, RotateCcw, Key, Settings, Copy, Trash2 } from 'lucide-react'
import { Badge, Button } from '../../../../shared/components'
import type { BrowserProfile } from '../../types'
import { LaunchCodeCell } from './LaunchCodeCell'
import { KeywordInlineRow } from './KeywordInlineRow'
import { formatTime, type StatusVariant } from './helpers'

interface ProfileCardProps {
  record: BrowserProfile
  selected: boolean
  coreLabel: string
  proxyName: string
  status: { variant: StatusVariant; label: string }
  isStarting: boolean
  isStopping: boolean
  isBusy: boolean
  onToggleSelect: (id: string) => void
  onStart: (id: string) => void
  onStop: (id: string) => void
  onRestart: (id: string) => void
  onKeywords: (record: BrowserProfile) => void
  onCopy: (record: BrowserProfile) => void
  onDelete: (id: string) => void
  onRefreshCode: () => void
}

// 实例卡片视图（从 BrowserListPage 提取）。所有交互通过 props 传入，保持零行为变更。
export function ProfileCard({
  record, selected, coreLabel, proxyName, status, isStarting, isStopping, isBusy,
  onToggleSelect, onStart, onStop, onRestart, onKeywords, onCopy, onDelete, onRefreshCode,
}: ProfileCardProps) {
  return (
    <div
      className={`flex flex-col border rounded-xl bg-[var(--color-bg-surface)] p-3 shadow-[0_1px_4px_rgba(0,0,0,0.08)] transition-all duration-200 h-[320px] overflow-hidden
          ${selected ? 'border-[var(--color-accent)] ring-1 ring-[var(--color-accent)]/20' : 'border-[var(--color-border-default)] hover:border-[var(--color-accent)]'}
        `}
    >
      {/* Header Row: Title, Status, Checkbox, Actions */}
      <div className="flex flex-col gap-3 pb-3 border-b border-[var(--color-border-muted)]/50 shrink-0">
        <div className="flex justify-between items-start gap-2">
          <div className="flex items-center gap-2 flex-wrap">
            <input
              type="checkbox"
              className="w-4 h-4 rounded cursor-pointer accent-[var(--color-accent)] mt-0.5 shrink-0"
              checked={selected}
              onChange={() => onToggleSelect(record.profileId)}
            />
            <Link className="text-[var(--color-accent)] font-medium text-sm hover:text-[var(--color-accent)] transition-colors truncate max-w-[200px]" to={`/browser/detail/${record.profileId}`}>
              {record.profileName}
            </Link>
            {record.tags && record.tags.length > 0 && (
              <div className="flex gap-1 ml-1">
                {record.tags.map(tag => <Badge variant="default" key={tag}>{tag}</Badge>)}
              </div>
            )}
          </div>
          <Badge variant={status.variant} dot dotClassName="w-2 h-2 shrink-0">{status.label}</Badge>
        </div>

        <div className="flex items-center gap-1 flex-wrap">
          {record.running ? (
            <Button size="sm" variant="secondary" onClick={() => onStop(record.profileId)} title={isStopping ? '停止中' : '停止'} loading={isStopping}>
              {!isStopping && <Square className="w-4 h-4 mr-1.5" />}
              {isStopping ? '停止中' : '停止'}
            </Button>
          ) : (
            <Button size="sm" onClick={() => onStart(record.profileId)} title={isStarting ? '启动中' : '启动'} loading={isStarting}>
              {!isStarting && <Play className="w-4 h-4 fill-current mr-1.5" />}
              {isStarting ? '启动中' : '启动'}
            </Button>
          )}
          <span className="w-px h-4 bg-[var(--color-border-muted)] mx-1"></span>
          <Button size="sm" variant="ghost" onClick={() => onRestart(record.profileId)} title="重启" className="px-3" disabled={isBusy}><RotateCcw className="w-4 h-4 mr-1.5" />重启</Button>
          <Button size="sm" variant="ghost" onClick={() => onKeywords(record)} title="关键字管理" className="px-3" disabled={isBusy}><Key className="w-4 h-4 mr-1.5" />关键字</Button>
          <Link to={`/browser/edit/${record.profileId}`}><Button size="sm" variant="ghost" title="配置" className="px-3" disabled={isBusy}><Settings className="w-4 h-4 mr-1.5" />配置</Button></Link>
          <Button size="sm" variant="ghost" onClick={() => onCopy(record)} title="克隆" className="px-3" disabled={isBusy}><Copy className="w-4 h-4 mr-1.5" />克隆</Button>
          <Button size="sm" variant="ghost" onClick={() => onDelete(record.profileId)} title="删除" className="px-3 text-red-500 hover:text-red-600 hover:bg-red-50" disabled={isBusy}><Trash2 className="w-4 h-4 mr-1.5" />删除</Button>
        </div>
      </div>

      {/* Body Grid: Key-Value Pairs */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 py-2 shrink-0">
        <div className="flex flex-col gap-0.5">
          <span className="text-xs text-[var(--color-text-muted)] font-medium">内核版本</span>
          <span className="text-xs text-[var(--color-text-primary)]">{coreLabel}</span>
        </div>
        <div className="flex flex-col gap-0.5">
          <span className="text-xs text-[var(--color-text-muted)] font-medium">代理配置</span>
          <span className="text-xs text-[var(--color-text-primary)]">{proxyName}</span>
        </div>
        <div className="flex flex-col gap-0.5">
          <span className="text-xs text-[var(--color-text-muted)] font-medium">快捷配置码</span>
          <div className="mt-0.5"><LaunchCodeCell profileId={record.profileId} code={record.launchCode || ''} onRefresh={onRefreshCode} /></div>
        </div>
        <div className="flex flex-col gap-0.5">
          <span className="text-xs text-[var(--color-text-muted)] font-medium">上次更新时间</span>
          <span className="text-xs text-[var(--color-text-primary)]">{formatTime(record.updatedAt)}</span>
        </div>
      </div>

      {/* Footer: Keywords */}
      <div className="border-t border-[var(--color-border-muted)]/50 pt-2 flex items-start gap-2 flex-1 min-h-0">
        <span className="text-xs font-medium text-[var(--color-text-primary)] shrink-0 pt-0.5">系统关键字</span>
        <div className="flex-1 min-h-0 overflow-y-auto pr-1">
          <KeywordInlineRow keywords={record.keywords || []} />
        </div>
      </div>
    </div>
  )
}
