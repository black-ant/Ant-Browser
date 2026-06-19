import { ReactNode } from 'react'
import clsx from 'clsx'

interface CardProps {
  title?: string
  subtitle?: string
  children: ReactNode
  className?: string
  padding?: 'none' | 'sm' | 'md' | 'lg'
  actions?: ReactNode
  hover?: boolean
}

export function Card({ 
  title, 
  subtitle, 
  children, 
  className,
  padding = 'md',
  actions,
  hover = false
}: CardProps) {
  const paddings = {
    none: '',
    sm: 'p-4',
    md: 'p-5',
    lg: 'p-6',
  }

  return (
    <div
      className={clsx(
        'bg-[var(--color-bg-surface)] rounded-2xl overflow-hidden',
        'border border-[var(--color-border-default)]',
        'shadow-[var(--shadow-sm)]',
        'transition-all duration-300',
        hover && 'hover:shadow-[var(--shadow-lg)] hover:border-[var(--color-border-strong)] hover:-translate-y-1',
        className
      )}
    >
      {(title || actions) && (
        <div className="flex items-center justify-between px-5 py-4 border-b border-[var(--color-border-muted)] bg-gradient-to-r from-[var(--color-bg-surface)] to-[var(--color-bg-subtle)]">
          <div>
            {title && (
              <h3 className="text-sm font-semibold text-[var(--color-text-primary)]">
                {title}
              </h3>
            )}
            {subtitle && (
              <p className="text-xs text-[var(--color-text-muted)] mt-0.5">
                {subtitle}
              </p>
            )}
          </div>
          {actions && <div className="flex items-center gap-2">{actions}</div>}
        </div>
      )}
      <div className={paddings[padding]}>{children}</div>
    </div>
  )
}
