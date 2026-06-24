import { ReactNode } from 'react'
import clsx from 'clsx'

type BadgeVariant = 'default' | 'success' | 'error' | 'warning' | 'info'
type BadgeSize = 'sm' | 'md' | 'lg'

interface BadgeProps {
  children: ReactNode
  variant?: BadgeVariant
  size?: BadgeSize
  dot?: boolean
  pulse?: boolean
  outline?: boolean
  dotClassName?: string
  className?: string
}

const variantStyles = {
  default: 'bg-[var(--color-bg-muted)] text-[var(--color-text-secondary)]',
  success: 'bg-[var(--color-success)]/10 text-[var(--color-success)]',
  error: 'bg-[var(--color-error)]/10 text-[var(--color-error)]',
  warning: 'bg-[var(--color-warning)]/10 text-[var(--color-warning)]',
  info: 'bg-[var(--color-accent)]/10 text-[var(--color-accent)]',
}

const outlineStyles = {
  default: 'bg-transparent text-[var(--color-text-secondary)] border-2 border-[var(--color-border-default)]',
  success: 'bg-transparent text-[var(--color-success)] border-2 border-[var(--color-success)]/30',
  error: 'bg-transparent text-[var(--color-error)] border-2 border-[var(--color-error)]/30',
  warning: 'bg-transparent text-[var(--color-warning)] border-2 border-[var(--color-warning)]/30',
  info: 'bg-transparent text-[var(--color-accent)] border-2 border-[var(--color-accent)]/30',
}

const sizeStyles = {
  sm: 'px-2 py-0.5 text-xs',
  md: 'px-2.5 py-1 text-xs',
  lg: 'px-3 py-1.5 text-sm',
}

const dotStyles = {
  default: 'bg-[var(--color-text-muted)]',
  success: 'bg-[var(--color-success)]',
  error: 'bg-[var(--color-error)]',
  warning: 'bg-[var(--color-warning)]',
  info: 'bg-[var(--color-accent)]',
}

export function Badge({
  children,
  variant = 'default',
  size = 'md',
  dot = false,
  pulse = false,
  outline = false,
  dotClassName = 'w-1.5 h-1.5',
  className,
}: BadgeProps) {
  return (
    <span
      className={clsx(
        'inline-flex items-center gap-1.5 rounded-full font-medium transition-all duration-200',
        outline ? outlineStyles[variant] : variantStyles[variant],
        sizeStyles[size],
        className
      )}
    >
      {dot && (
        <span className="relative inline-flex">
          <span className={clsx('rounded-full', dotClassName, dotStyles[variant])} />
          {pulse && (
            <span className={clsx(
              'absolute inset-0 rounded-full animate-ping',
              dotClassName,
              dotStyles[variant],
              'opacity-75'
            )} />
          )}
        </span>
      )}
      {children}
    </span>
  )
}
