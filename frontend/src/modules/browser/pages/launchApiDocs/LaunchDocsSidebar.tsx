import { FileText } from 'lucide-react'
import type { LaunchDocGroup } from './catalog'

interface LaunchDocsSidebarProps {
  groups: LaunchDocGroup[]
  activeId: string
  onSelect: (id: string) => void
}

export function LaunchDocsSidebar({
  groups,
  activeId,
  onSelect,
}: LaunchDocsSidebarProps) {
  return (
    <div className="space-y-4">
      <div className="px-1">
        <p className="text-xs font-semibold uppercase tracking-widest text-[var(--color-text-muted)]">
          文档目录
        </p>
      </div>

      <nav className="space-y-4">
        {groups.map((group) => (
          <section key={group.id} className="space-y-1">
            <p className="px-3 pb-1 text-[11px] font-semibold uppercase tracking-widest text-[var(--color-text-muted)]">
              {group.label}
            </p>
            {group.items.map((item) => {
              const isActive = activeId === item.id
              return (
                <button
                  key={item.id}
                  onClick={() => onSelect(item.id)}
                  className={[
                    'w-full rounded-lg border px-3 py-2 text-left transition-colors',
                    isActive
                      ? 'border-[var(--color-accent)] bg-[var(--color-accent-muted)]'
                      : 'border-transparent hover:border-[var(--color-border-muted)] hover:bg-[var(--color-bg-muted)]',
                  ].join(' ')}
                >
                  <div className="flex items-start gap-2">
                    <FileText className={`mt-0.5 h-3.5 w-3.5 shrink-0 ${isActive ? 'text-[var(--color-accent)]' : 'text-[var(--color-text-muted)]'}`} />
                    <div className={`min-w-0 text-sm font-medium ${isActive ? 'text-[var(--color-text-primary)]' : 'text-[var(--color-text-secondary)]'}`}>
                      {item.label}
                    </div>
                  </div>
                </button>
              )
            })}
          </section>
        ))}
      </nav>
    </div>
  )
}
