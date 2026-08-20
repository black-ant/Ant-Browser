import { ReactNode, InputHTMLAttributes, SelectHTMLAttributes, TextareaHTMLAttributes } from 'react'
import clsx from 'clsx'

const SELECT_CHEVRON_DATA_URI =
  `url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 20 20' fill='none' stroke='%2364758b' stroke-width='1.75' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpath d='m5.5 7.5 4.5 4.5 4.5-4.5'/%3E%3C/svg%3E")`

interface FormItemProps {
  label?: ReactNode
  required?: boolean
  hint?: string
  error?: string
  children: ReactNode
  className?: string
}

export function FormItem({ label, required, hint, error, children, className }: FormItemProps) {
  return (
    <div className={clsx('space-y-1.5', className)}>
      {label && (
        <div className="flex items-center gap-1 text-sm font-medium text-[var(--color-text-secondary)]">
          <span>
            {label}
            {required && <span className="text-[var(--color-error)] ml-0.5">*</span>}
          </span>
          {hint && (
            <span className="group relative inline-flex" tabIndex={0} aria-label={hint} title={hint}>
              <span className="flex h-4 w-4 cursor-help items-center justify-center rounded-full border border-[var(--color-border-muted)] text-[10px] font-semibold leading-none text-[var(--color-text-muted)]">?</span>
              <span className="pointer-events-none absolute left-0 top-5 z-50 hidden w-64 rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-3 py-2 text-xs font-normal leading-5 text-[var(--color-text-secondary)] shadow-lg group-hover:block group-focus:block">
                {hint}
              </span>
            </span>
          )}
        </div>
      )}
      {children}
      {error && <p className="text-xs text-[var(--color-error)]">{error}</p>}
    </div>
  )
}

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  error?: boolean
}

export function Input({ error, className, ...props }: InputProps) {
  return (
    <input
      className={clsx(
        'block h-9 px-3 text-sm',
        'bg-[var(--color-bg-surface)] text-[var(--color-text-primary)]',
        'border border-[var(--color-border-default)] rounded-lg',
        'placeholder:text-[var(--color-text-muted)]',
        'focus:outline-none focus:border-[var(--color-border-strong)] focus:ring-1 focus:ring-[var(--color-border-strong)]',
        'disabled:bg-[var(--color-bg-muted)] disabled:text-[var(--color-text-muted)] disabled:cursor-not-allowed',
        'transition-colors duration-150',
        error && 'border-[var(--color-error)] focus:border-[var(--color-error)] focus:ring-[var(--color-error)]',
        // 默认宽度自适应，可通过 className 覆盖
        !className?.includes('w-') && 'w-full',
        className
      )}
      {...props}
    />
  )
}

interface SelectProps extends SelectHTMLAttributes<HTMLSelectElement> {
  error?: boolean
  options: { value: string; label: string }[]
}

export function Select({ error, options, className, style, ...props }: SelectProps) {
  return (
    <select
      className={clsx(
        'block h-9 appearance-none px-3 pr-10 text-sm',
        'bg-[var(--color-bg-surface)] text-[var(--color-text-primary)]',
        'border border-[var(--color-border-default)] rounded-lg',
        'shadow-[var(--shadow-xs)]',
        'hover:border-[var(--color-border-strong)]',
        'focus:outline-none focus:border-[var(--color-border-strong)] focus:ring-1 focus:ring-[var(--color-border-strong)]',
        'disabled:bg-[var(--color-bg-muted)] disabled:text-[var(--color-text-muted)] disabled:cursor-not-allowed',
        'transition-colors duration-150',
        'cursor-pointer',
        error && 'border-[var(--color-error)] focus:border-[var(--color-error)] focus:ring-[var(--color-error)]',
        // 默认宽度自适应，可通过 className 覆盖
        !className?.includes('w-') && 'w-full',
        className
      )}
      style={{
        backgroundImage: SELECT_CHEVRON_DATA_URI,
        backgroundRepeat: 'no-repeat',
        backgroundPosition: 'right 0.8rem center',
        backgroundSize: '0.95rem 0.95rem',
        ...style,
      }}
      {...props}
    >
      {options.map((opt) => (
        <option key={opt.value} value={opt.value}>
          {opt.label}
        </option>
      ))}
    </select>
  )
}

interface TextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  error?: boolean
}

export function Textarea({ error, className, ...props }: TextareaProps) {
  return (
    <textarea
      className={clsx(
        'block w-full px-3 py-2 text-sm',
        'bg-[var(--color-bg-surface)] text-[var(--color-text-primary)]',
        'border border-[var(--color-border-default)] rounded-lg',
        'placeholder:text-[var(--color-text-muted)]',
        'focus:outline-none focus:border-[var(--color-border-strong)] focus:ring-1 focus:ring-[var(--color-border-strong)]',
        'disabled:bg-[var(--color-bg-muted)] disabled:text-[var(--color-text-muted)] disabled:cursor-not-allowed',
        'transition-colors duration-150 resize-none',
        error && 'border-[var(--color-error)] focus:border-[var(--color-error)] focus:ring-[var(--color-error)]',
        className
      )}
      {...props}
    />
  )
}

interface SwitchProps {
  checked: boolean
  onChange: (checked: boolean) => void
  disabled?: boolean
}

export function Switch({ checked, onChange, disabled }: SwitchProps) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      disabled={disabled}
      onClick={() => onChange(!checked)}
      className={clsx(
        'relative inline-flex h-5 w-9 shrink-0 items-center rounded-full transition-colors duration-150',
        'focus:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent)] focus-visible:ring-offset-2',
        checked ? 'bg-[var(--color-accent)]' : 'bg-[var(--color-border-strong)]',
        disabled && 'opacity-50 cursor-not-allowed'
      )}
    >
      <span
        className={clsx(
          'inline-block h-4 w-4 transform rounded-full bg-white shadow-sm transition-transform duration-150',
          checked ? 'translate-x-4' : 'translate-x-0.5'
        )}
      />
    </button>
  )
}
