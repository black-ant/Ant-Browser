import clsx from 'clsx'

interface ButtonGroupOption<T = string> {
  value: T
  label: string
  icon?: React.ReactNode
}

interface ButtonGroupProps<T = string> {
  options: ButtonGroupOption<T>[]
  value: T
  onChange: (value: T) => void
  className?: string
  size?: 'sm' | 'md'
}

export function ButtonGroup<T = string>({
  options,
  value,
  onChange,
  className,
  size = 'md'
}: ButtonGroupProps<T>) {
  return (
    <div className={clsx('inline-flex rounded-lg border border-[var(--color-border-default)]', className)}>
      {options.map((option, idx) => {
        const isSelected = option.value === value
        const isFirst = idx === 0
        const isLast = idx === options.length - 1

        return (
          <button
            key={String(option.value)}
            type="button"
            onClick={() => onChange(option.value)}
            className={clsx(
              'flex items-center justify-center gap-1.5 transition-all',
              size === 'sm' && 'px-2 py-1 text-xs',
              size === 'md' && 'px-3 py-1.5 text-sm',
              !isFirst && 'border-l border-[var(--color-border-default)]',
              isFirst && 'rounded-l-lg',
              isLast && 'rounded-r-lg',
              isSelected
                ? 'bg-[var(--color-accent)] text-white font-medium'
                : 'bg-[var(--color-bg-elevated)] text-[var(--color-text-primary)] hover:bg-[var(--color-bg-muted)]'
            )}
          >
            {option.icon}
            <span>{option.label}</span>
          </button>
        )
      })}
    </div>
  )
}
