import clsx from 'clsx'

type LoadingSize = 'sm' | 'md' | 'lg'

interface LoadingProps {
  size?: LoadingSize
  text?: string
  fullscreen?: boolean
  className?: string
}

const sizeStyles = {
  sm: 'w-4 h-4 border-2',
  md: 'w-6 h-6 border-2',
  lg: 'w-8 h-8 border-[3px]',
}

export function Loading({
  size = 'md',
  text,
  fullscreen = false,
  className
}: LoadingProps) {
  const spinner = (
    <div className={clsx('flex flex-col items-center gap-3', className)}>
      <div className="relative">
        <div
          className={clsx(
            'border-[var(--color-border-default)] border-t-[var(--color-accent)] rounded-full animate-spin',
            sizeStyles[size]
          )}
        />
        {size === 'lg' && (
          <div className="absolute inset-0 border-[3px] border-[var(--color-accent)]/20 rounded-full animate-ping" />
        )}
      </div>
      {text && (
        <span className="text-sm font-medium text-[var(--color-text-secondary)]">{text}</span>
      )}
    </div>
  )

  if (fullscreen) {
    return (
      <div className="fixed inset-0 z-50 flex items-center justify-center bg-[var(--color-bg-base)]/80 backdrop-blur-md animate-fade-in">
        {spinner}
      </div>
    )
  }

  return spinner
}

// 骨架屏组件
interface SkeletonProps {
  width?: string
  height?: string
  circle?: boolean
  className?: string
}

export function Skeleton({
  width = '100%',
  height = '20px',
  circle = false,
  className
}: SkeletonProps) {
  return (
    <div
      className={clsx(
        'bg-gradient-to-r from-[var(--color-bg-muted)] via-[var(--color-bg-muted)]/50 to-[var(--color-bg-muted)] bg-[length:200%_100%] animate-pulse',
        circle ? 'rounded-full' : 'rounded-lg',
        className
      )}
      style={{ width, height, animation: 'pulse 2s cubic-bezier(0.4, 0, 0.6, 1) infinite' }}
    />
  )
}

// 点状加载组件
interface DotsLoadingProps {
  size?: 'sm' | 'md' | 'lg'
  className?: string
}

const dotSizes = {
  sm: 'w-1.5 h-1.5',
  md: 'w-2 h-2',
  lg: 'w-2.5 h-2.5',
}

export function DotsLoading({ size = 'md', className }: DotsLoadingProps) {
  return (
    <div className={clsx('flex items-center gap-1.5', className)}>
      <div className={clsx(dotSizes[size], 'rounded-full bg-[var(--color-accent)] animate-bounce [animation-delay:-0.3s]')} />
      <div className={clsx(dotSizes[size], 'rounded-full bg-[var(--color-accent)] animate-bounce [animation-delay:-0.15s]')} />
      <div className={clsx(dotSizes[size], 'rounded-full bg-[var(--color-accent)] animate-bounce')} />
    </div>
  )
}
