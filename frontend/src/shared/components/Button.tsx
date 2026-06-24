import { ButtonHTMLAttributes, ReactNode } from 'react'
import clsx from 'clsx'

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'secondary' | 'danger' | 'ghost' | 'outline'
  size?: 'sm' | 'md' | 'lg'
  loading?: boolean
  children: ReactNode
}

export function Button({
  variant = 'primary',
  size = 'md',
  loading = false,
  disabled,
  className,
  children,
  ...props
}: ButtonProps) {
  const baseStyles = 'inline-flex items-center justify-center font-medium rounded-xl transition-all duration-200 focus:outline-none focus-visible:ring-4 focus-visible:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed'

  const variants = {
    primary: 'bg-[var(--color-accent)] text-[var(--color-text-inverse)] shadow-md hover:shadow-lg hover:-translate-y-0.5 active:translate-y-0 focus-visible:ring-[var(--color-accent)]/30',
    secondary: 'bg-[var(--color-bg-surface)] text-[var(--color-text-secondary)] border-2 border-[var(--color-border-default)] hover:bg-[var(--color-bg-muted)] hover:border-[var(--color-border-strong)] hover:-translate-y-0.5 active:translate-y-0 focus-visible:ring-[var(--color-border-strong)]/30',
    outline: 'bg-transparent text-[var(--color-accent)] border-2 border-[var(--color-accent)] hover:bg-[var(--color-accent)]/10 hover:-translate-y-0.5 active:translate-y-0 focus-visible:ring-[var(--color-accent)]/30',
    danger: 'bg-[var(--color-error)] text-white shadow-md hover:shadow-lg hover:-translate-y-0.5 active:translate-y-0 focus-visible:ring-[var(--color-error)]/30',
    ghost: 'text-[var(--color-text-secondary)] hover:bg-[var(--color-accent)]/10 hover:text-[var(--color-accent)]',
  }

  const sizes = {
    sm: 'h-8 px-3 text-xs gap-1.5',
    md: 'h-10 px-5 text-sm gap-2',
    lg: 'h-11 px-6 text-base gap-2.5',
  }

  return (
    <button
      className={clsx(
        baseStyles,
        variants[variant],
        sizes[size],
        className
      )}
      disabled={disabled || loading}
      {...props}
    >
      {loading && (
        <svg className="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
          <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
          <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
        </svg>
      )}
      {children}
    </button>
  )
}
