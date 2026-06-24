import { ReactNode, InputHTMLAttributes, SelectHTMLAttributes, TextareaHTMLAttributes } from 'react'
import clsx from 'clsx'

interface FormItemProps {
  label?: string
  required?: boolean
  hint?: string
  error?: string
  children: ReactNode
  className?: string
}

export function FormItem({ label, required, hint, error, children, className }: FormItemProps) {
  return (
    <div className={clsx('space-y-2', className)}>
      {label && (
        <label className="block text-sm font-medium text-[var(--color-text-secondary)]">
          {label}
          {required && <span className="text-[var(--color-error)] ml-0.5">*</span>}
          {hint && <span className="text-xs font-normal text-[var(--color-text-muted)] ml-1.5">({hint})</span>}
        </label>
      )}
      {children}
      {error && (
        <p className="text-xs text-[var(--color-error)] flex items-center gap-1">
          <svg className="w-3.5 h-3.5" fill="currentColor" viewBox="0 0 20 20">
            <path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clipRule="evenodd" />
          </svg>
          {error}
        </p>
      )}
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
        'block h-10 px-4 text-sm',
        'bg-[var(--color-bg-input)] text-[var(--color-text-primary)]',
        'border-2 border-[var(--color-border-default)] rounded-lg',
        'placeholder:text-[var(--color-text-muted)]',
        'focus:outline-none focus:border-[var(--color-accent)] focus:ring-4 focus:ring-[var(--color-accent)]/10',
        'disabled:bg-[var(--color-bg-muted)] disabled:text-[var(--color-text-muted)] disabled:cursor-not-allowed',
        'transition-all duration-200',
        error && 'border-[var(--color-error)] focus:border-[var(--color-error)] focus:ring-[var(--color-error)]/10',
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

export function Select({ error, options, className, ...props }: SelectProps) {
  return (
    <select
      className={clsx(
        'block h-10 px-4 text-sm',
        'bg-[var(--color-bg-input)] text-[var(--color-text-primary)]',
        'border-2 border-[var(--color-border-default)] rounded-lg',
        'focus:outline-none focus:border-[var(--color-accent)] focus:ring-4 focus:ring-[var(--color-accent)]/10',
        'disabled:bg-[var(--color-bg-muted)] disabled:text-[var(--color-text-muted)] disabled:cursor-not-allowed',
        'transition-all duration-200',
        'cursor-pointer',
        error && 'border-[var(--color-error)] focus:border-[var(--color-error)] focus:ring-[var(--color-error)]/10',
        !className?.includes('w-') && 'w-full',
        className
      )}
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
        'block w-full px-4 py-3 text-sm',
        'bg-[var(--color-bg-input)] text-[var(--color-text-primary)]',
        'border-2 border-[var(--color-border-default)] rounded-lg',
        'placeholder:text-[var(--color-text-muted)]',
        'focus:outline-none focus:border-[var(--color-accent)] focus:ring-4 focus:ring-[var(--color-accent)]/10',
        'disabled:bg-[var(--color-bg-muted)] disabled:text-[var(--color-text-muted)] disabled:cursor-not-allowed',
        'transition-all duration-200 resize-none',
        error && 'border-[var(--color-error)] focus:border-[var(--color-error)] focus:ring-[var(--color-error)]/10',
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
        'relative inline-flex h-6 w-11 items-center rounded-full transition-all duration-200',
        'focus:outline-none focus-visible:ring-4 focus-visible:ring-[var(--color-accent)]/20 focus-visible:ring-offset-2',
        checked ? 'bg-[var(--color-accent)] shadow-md' : 'bg-[var(--color-border-strong)]',
        disabled && 'opacity-50 cursor-not-allowed'
      )}
    >
      <span
        className={clsx(
          'inline-block h-5 w-5 transform rounded-full bg-white shadow-lg transition-transform duration-200',
          checked ? 'translate-x-5' : 'translate-x-0.5'
        )}
      />
    </button>
  )
}
