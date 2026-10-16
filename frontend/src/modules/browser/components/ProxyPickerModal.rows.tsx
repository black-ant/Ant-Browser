import { Check, Loader2, Pencil, Trash2, Wifi } from 'lucide-react'
import type { BrowserProxy } from '../types'
import { DIRECT_PROXY_ID, type SpeedResult } from './ProxyPickerModal.helpers'

export function GroupItem({ label, active, count, onClick }: { label: string; active: boolean; count: number; onClick: () => void }) {
  return (
    <button
      type="button"
      aria-pressed={active}
      onClick={onClick}
      className={`w-full text-left px-3 py-2 text-sm flex items-center justify-between gap-2 transition-colors ${
        active
          ? 'bg-[var(--color-primary)]/10 text-[var(--color-primary)] font-medium'
          : 'text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-hover)]'
      }`}
    >
      <span className="truncate">{label}</span>
      <span className="text-xs opacity-60 shrink-0">{count}</span>
    </button>
  )
}

interface ProxyRowProps {
  proxy: BrowserProxy
  selected: boolean
  testing: boolean
  speedResult?: SpeedResult
  displayConfig: string
  onSelect: () => void
  onTest: (e: React.MouseEvent) => void
  onEdit: (e: React.MouseEvent) => void
  onDelete: (e: React.MouseEvent) => void
}

function SpeedBadge({ testing, result }: { testing: boolean; result?: SpeedResult }) {
  if (testing) return <Loader2 className="w-3.5 h-3.5 animate-spin text-[var(--color-text-muted)] shrink-0" />
  if (!result) return null
  const suffix = result.engine ? ` (${result.engine})` : ''
  if (!result.ok) return <span className="text-xs text-red-500 shrink-0 whitespace-nowrap">失败{suffix}</span>
  const color = result.latencyMs < 200 ? 'text-green-500' : result.latencyMs < 500 ? 'text-yellow-500' : 'text-red-500'
  return <span className={`text-xs font-medium shrink-0 whitespace-nowrap ${color}`}>{result.latencyMs}ms{suffix}</span>
}

export function ProxyRow({ proxy, selected, testing, speedResult, displayConfig, onSelect, onTest, onEdit, onDelete }: ProxyRowProps) {
  const isDirect = proxy.proxyId === DIRECT_PROXY_ID
  const disableDelete = isDirect

  return (
    <div
      onClick={onSelect}
      className={`w-full px-4 py-2.5 flex items-center gap-3 cursor-pointer transition-colors border-b border-[var(--color-border)]/40 last:border-0 overflow-hidden ${
        selected ? 'bg-[var(--color-primary)]/10' : 'hover:bg-[var(--color-bg-hover)]'
      }`}
    >
      <div className="flex-1 min-w-0 overflow-hidden">
        <div className="text-sm font-medium text-[var(--color-text-primary)] truncate">
          {proxy.proxyName || proxy.proxyId}
          {proxy.groupName && <span className="ml-2 text-xs text-[var(--color-primary)]/70 font-normal">[{proxy.groupName}]</span>}
        </div>
        <div className="text-xs text-[var(--color-text-muted)] truncate mt-0.5 w-0 min-w-full">
          {displayConfig}
        </div>
      </div>
      <SpeedBadge testing={testing} result={speedResult} />
      <button
        onClick={onTest}
        disabled={testing}
        title="测速"
        className="shrink-0 p-1 rounded text-[var(--color-text-muted)] hover:text-[var(--color-primary)] hover:bg-[var(--color-primary)]/10 disabled:opacity-40 transition-colors"
      >
        <Wifi className="w-3.5 h-3.5" />
      </button>
      <button
        onClick={onEdit}
        disabled={isDirect}
        title={isDirect ? '直连不可编辑' : '编辑代理'}
        className="shrink-0 p-1 rounded text-[var(--color-text-muted)] hover:text-[var(--color-primary)] hover:bg-[var(--color-primary)]/10 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
      >
        <Pencil className="w-3.5 h-3.5" />
      </button>
      <button
        onClick={onDelete}
        disabled={disableDelete}
        title={isDirect ? '直连不可删除' : '删除代理'}
        className="shrink-0 p-1 rounded text-[var(--color-text-muted)] hover:text-red-500 hover:bg-red-500/10 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
      >
        <Trash2 className="w-3.5 h-3.5" />
      </button>
      {selected && <Check className="w-4 h-4 text-[var(--color-primary)] shrink-0" />}
    </div>
  )
}
