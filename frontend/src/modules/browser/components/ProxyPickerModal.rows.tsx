import { Check } from 'lucide-react'
import type { BrowserProxy } from '../types'

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
  displayConfig: string
  onSelect: () => void
}

export function ProxyRow({ proxy, selected, displayConfig, onSelect }: ProxyRowProps) {
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
      {selected && <Check className="w-4 h-4 text-[var(--color-primary)] shrink-0" />}
    </div>
  )
}
