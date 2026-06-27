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

// 窗口卡片视图（从 BrowserListPage 提取）。所有交互通过 props 传入，保持零行为变更。
export function ProfileCard({
  record, selected, coreLabel, proxyName, status, isStarting, isStopping, isBusy,
  onToggleSelect, onStart, onStop, onRestart, onKeywords, onCopy, onDelete, onRefreshCode,
}: ProfileCardProps) {
  return (
    <div
      className={`flex h-[184px] flex-col overflow-hidden rounded-xl border bg-[var(--color-bg-surface)] p-2.5 shadow-[0_1px_4px_rgba(0,0,0,0.08)] transition-all duration-200
          ${selected ? 'border-[var(--color-accent)] ring-1 ring-[var(--color-accent)]/20' : 'border-[var(--color-border-default)] hover:border-[var(--color-accent)]'}
        `}
    >
      {/* Header Row: Title, Status, Checkbox, Actions */}
      <div className="flex shrink-0 flex-col gap-2 border-b border-[var(--color-border-muted)]/50 pb-2">
        <div className="flex justify-between items-start gap-2">
          <div className="flex min-w-0 items-center gap-2">
            <input
              type="checkbox"
              className="h-4 w-4 shrink-0 cursor-pointer rounded accent-[var(--color-accent)]"
              checked={selected}
              onChange={() => onToggleSelect(record.profileId)}
            />
            <Link className="min-w-0 max-w-[160px] truncate text-sm font-medium text-[var(--color-accent)] transition-colors hover:text-[var(--color-accent)]" to={`/browser/detail/${record.profileId}`}>
              {record.profileName}
            </Link>
            {record.tags && record.tags.length > 0 && (
              <div className="ml-1 hidden max-w-[120px] gap-1 overflow-hidden sm:flex">
                {record.tags.map(tag => <Badge variant="default" key={tag}>{tag}</Badge>)}
              </div>
            )}
          </div>
          <Badge variant={status.variant} dot dotClassName="w-2 h-2 shrink-0">{status.label}</Badge>
        </div>

        <div className="flex items-center gap-1 overflow-x-auto whitespace-nowrap pb-0.5">
          {record.running ? (
            <Button size="sm" variant="secondary" className="h-7 rounded-lg px-2" onClick={() => onStop(record.profileId)} title={isStopping ? '停止中' : '停止'} loading={isStopping}>
              {!isStopping && <Square className="mr-1 h-3.5 w-3.5" />}
              {isStopping ? '停止中' : '停止'}
            </Button>
          ) : (
            <Button size="sm" className="h-7 rounded-lg px-2" onClick={() => onStart(record.profileId)} title={isStarting ? '启动中' : '启动'} loading={isStarting}>
              {!isStarting && <Play className="mr-1 h-3.5 w-3.5 fill-current" />}
              {isStarting ? '启动中' : '启动'}
            </Button>
          )}
          <span className="mx-0.5 h-4 w-px shrink-0 bg-[var(--color-border-muted)]"></span>
          <Button size="sm" variant="ghost" onClick={() => onRestart(record.profileId)} title="重启" className="h-7 rounded-lg px-2" disabled={isBusy}><RotateCcw className="mr-1 h-3.5 w-3.5" />重启</Button>
          <Button size="sm" variant="ghost" onClick={() => onKeywords(record)} title="关键字管理" className="h-7 rounded-lg px-2" disabled={isBusy}><Key className="mr-1 h-3.5 w-3.5" />关键字</Button>
          <Link to={`/browser/edit/${record.profileId}`}><Button size="sm" variant="ghost" title="配置" className="h-7 rounded-lg px-2" disabled={isBusy}><Settings className="mr-1 h-3.5 w-3.5" />配置</Button></Link>
          <Button size="sm" variant="ghost" onClick={() => onCopy(record)} title="克隆" className="h-7 rounded-lg px-2" disabled={isBusy}><Copy className="mr-1 h-3.5 w-3.5" />克隆</Button>
          <Button size="sm" variant="ghost" onClick={() => onDelete(record.profileId)} title="删除" className="h-7 rounded-lg px-2 text-red-500 hover:bg-red-50 hover:text-red-600" disabled={isBusy}><Trash2 className="mr-1 h-3.5 w-3.5" />删除</Button>
        </div>
      </div>

      {/* Body Grid: Key-Value Pairs */}
      <div className="grid shrink-0 grid-cols-2 gap-x-3 gap-y-1.5 border-b border-[var(--color-border-muted)]/50 py-2">
        <div className="min-w-0">
          <span className="text-xs text-[var(--color-text-muted)] font-medium">内核版本</span>
          <span className="ml-1 inline-block max-w-[130px] truncate align-bottom text-xs text-[var(--color-text-primary)]">{coreLabel}</span>
        </div>
        <div className="min-w-0">
          <span className="text-xs text-[var(--color-text-muted)] font-medium">代理配置</span>
          <span className="ml-1 inline-block max-w-[130px] truncate align-bottom text-xs text-[var(--color-text-primary)]">{proxyName}</span>
        </div>
        <div className="min-w-0">
          <span className="text-xs text-[var(--color-text-muted)] font-medium">快捷配置码</span>
          <span className="ml-1 inline-flex align-middle"><LaunchCodeCell profileId={record.profileId} code={record.launchCode || ''} onRefresh={onRefreshCode} /></span>
        </div>
        <div className="min-w-0">
          <span className="text-xs text-[var(--color-text-muted)] font-medium">上次更新时间</span>
          <span className="ml-1 inline-block max-w-[130px] truncate align-bottom text-xs text-[var(--color-text-primary)]">{formatTime(record.updatedAt)}</span>
        </div>
      </div>

      {/* Footer: Keywords */}
      <div className="flex min-h-0 flex-1 items-start gap-2 overflow-hidden pt-2">
        <span className="shrink-0 pt-0.5 text-xs font-medium text-[var(--color-text-primary)]">系统关键字</span>
        <div className="min-h-0 flex-1 overflow-hidden pr-1">
          <KeywordInlineRow keywords={record.keywords || []} />
        </div>
      </div>
    </div>
  )
}
