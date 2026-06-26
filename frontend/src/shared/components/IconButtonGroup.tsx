import clsx from 'clsx'

interface IconButtonOption<T = string> {
  value: T
  icon: React.ReactNode
  label: string
}

interface IconButtonGroupProps<T = string> {
  options: IconButtonOption<T>[]
  value: T
  onChange: (value: T) => void
  className?: string
}

export function IconButtonGroup<T = string>({
  options,
  value,
  onChange,
  className
}: IconButtonGroupProps<T>) {
  return (
    <div className={clsx('flex flex-wrap gap-2', className)}>
      {options.map((option) => {
        const isSelected = option.value === value

        return (
          <button
            key={String(option.value)}
            type="button"
            onClick={() => onChange(option.value)}
            title={option.label}
            className={clsx(
              'flex items-center justify-center w-12 h-12 rounded-lg border-2 transition-all',
              isSelected
                ? 'border-[var(--color-accent)] bg-[var(--color-accent-muted)] shadow-sm'
                : 'border-[var(--color-border-default)] bg-[var(--color-bg-elevated)] hover:border-[var(--color-border-strong)]'
            )}
          >
            {option.icon}
          </button>
        )
      })}
    </div>
  )
}
