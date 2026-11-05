import { ButtonHTMLAttributes, ReactNode } from 'react'
import clsx from 'clsx'

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'secondary' | 'danger' | 'ghost'
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
  const baseStyles = 'inline-flex items-center justify-center font-medium rounded-[6px] transition-all duration-150 focus:outline-none focus-visible:ring-2 focus-visible:ring-offset-2'
  
  const variants = {
    primary: 'bg-[var(--color-accent)] text-[var(--color-text-inverse)] hover:opacity-90 focus-visible:ring-[var(--color-accent)]',
    secondary: 'bg-[var(--color-bg-muted)] text-[var(--color-text-secondary)] border border-[var(--color-border-default)] hover:border-[var(--color-border-strong)] hover:text-[var(--color-text-primary)] focus-visible:ring-[var(--color-border-strong)]',
    danger: 'bg-[var(--color-error)] text-white hover:opacity-90 focus-visible:ring-[var(--color-error)]',
    ghost: 'text-[var(--color-text-secondary)] hover:bg-[var(--color-accent-muted)] hover:text-[var(--color-text-primary)]',
  }

  const sizes = {
    sm: 'h-7 px-2.5 text-xs gap-1.5',
    md: 'h-8 px-3.5 text-sm gap-2',
    lg: 'h-9 px-4 text-sm gap-2',
  }

  return (
    <button
      className={clsx(
        baseStyles,
        variants[variant],
        sizes[size],
        (disabled || loading) && 'opacity-50 cursor-not-allowed',
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
