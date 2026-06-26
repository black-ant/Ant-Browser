import clsx from 'clsx'

interface WindowPositionPickerProps {
  value: string
  onChange: (position: string) => void
  className?: string
}

const positions = [
  'top-left', 'top-center', 'top-right',
  'middle-left', 'middle-center', 'middle-right',
  'bottom-left', 'bottom-center', 'bottom-right'
]

const positionLabels: Record<string, string> = {
  'top-left': '左上',
  'top-center': '顶部居中',
  'top-right': '右上',
  'middle-left': '左侧居中',
  'middle-center': '屏幕居中',
  'middle-right': '右侧居中',
  'bottom-left': '左下',
  'bottom-center': '底部居中',
  'bottom-right': '右下'
}

export function WindowPositionPicker({ value, onChange, className }: WindowPositionPickerProps) {
  return (
    <div className={clsx('inline-grid grid-cols-3 gap-1.5 p-2 bg-[var(--color-bg-elevated)] rounded-lg border border-[var(--color-border-default)]', className)}>
      {positions.map((position) => {
        const isSelected = value === position

        return (
          <button
            key={position}
            type="button"
            onClick={() => onChange(position)}
            title={positionLabels[position]}
            className={clsx(
              'w-10 h-10 rounded border-2 transition-all',
              isSelected
                ? 'border-[var(--color-accent)] bg-[var(--color-accent-muted)]'
                : 'border-[var(--color-border-default)] bg-[var(--color-bg-base)] hover:border-[var(--color-border-strong)]'
            )}
          >
            <span className="sr-only">{positionLabels[position]}</span>
          </button>
        )
      })}
    </div>
  )
}
