import { ReactNode } from 'react'
import clsx from 'clsx'

interface StatCardProps {
  title: string
  value: string | number
  icon?: ReactNode
  trend?: {
    value: number
    label: string
  }
}

export function StatCard({ title, value, icon, trend }: StatCardProps) {
  return (
    <div 
      className={clsx(
        'min-w-0 bg-[var(--color-bg-surface)] rounded-xl overflow-hidden',
        'border border-[var(--color-border-default)]',
        'transition-all duration-200',
        'hover:border-[var(--color-border-strong)]',
        'group'
      )}
    >
      <div className="p-4">
        <div className="flex items-start justify-between gap-3">
          <div className="flex-1 min-w-0">
            <p className="text-xs text-[var(--color-text-muted)] font-medium tracking-wide uppercase">
              {title}
            </p>
            <p className="text-2xl font-semibold text-[var(--color-text-primary)] mt-2 tabular-nums">
              {value}
            </p>
            {trend && (
              <div className="flex items-center gap-1.5 mt-2">
                <span className={clsx(
                  'text-xs font-medium',
                  trend.value >= 0 ? 'text-[var(--color-success)]' : 'text-[var(--color-error)]'
                )}>
                  {trend.value >= 0 ? '↑' : '↓'} {Math.abs(trend.value)}%
                </span>
                <span className="text-xs text-[var(--color-text-muted)]">
                  {trend.label}
                </span>
              </div>
            )}
          </div>
          {icon && (
            <div className="w-11 h-11 rounded-xl bg-[var(--color-bg-muted)] flex items-center justify-center text-[var(--color-text-secondary)] transition-colors group-hover:bg-[var(--color-accent-muted)]">
              {icon}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
