import { ReactNode } from 'react'
import clsx from 'clsx'
import { Inbox, Search, FileQuestion, AlertCircle } from 'lucide-react'

type EmptyType = 'default' | 'search' | 'error' | 'custom'

interface EmptyProps {
  type?: EmptyType
  icon?: ReactNode
  title?: string
  description?: string
  action?: ReactNode
  className?: string
}

const defaultIcons = {
  default: Inbox,
  search: Search,
  error: AlertCircle,
  custom: FileQuestion,
}

const defaultTitles = {
  default: '暂无数据',
  search: '未找到结果',
  error: '加载失败',
  custom: '暂无内容',
}

const defaultDescriptions = {
  default: '还没有任何数据，快来添加第一条吧',
  search: '尝试使用其他关键词或筛选条件',
  error: '数据加载时出现了问题，请稍后重试',
  custom: '',
}

export function Empty({
  type = 'default',
  icon,
  title,
  description,
  action,
  className,
}: EmptyProps) {
  const Icon = icon ? null : defaultIcons[type]
  const displayTitle = title ?? defaultTitles[type]
  const displayDescription = description ?? defaultDescriptions[type]

  return (
    <div className={clsx('flex flex-col items-center justify-center py-16 px-4', className)}>
      {/* 图标容器 */}
      <div className="w-20 h-20 rounded-2xl bg-[var(--color-bg-muted)] flex items-center justify-center mb-4 transition-all duration-200 hover:scale-105">
        {icon ? (
          <div className="text-[var(--color-text-muted)]">{icon}</div>
        ) : Icon ? (
          <Icon className="w-10 h-10 text-[var(--color-text-muted)]" strokeWidth={1.5} />
        ) : null}
      </div>

      {/* 标题 */}
      <h3 className="text-base font-semibold text-[var(--color-text-primary)] mb-2">
        {displayTitle}
      </h3>

      {/* 描述 */}
      {displayDescription && (
        <p className="text-sm text-[var(--color-text-muted)] text-center max-w-sm mb-5">
          {displayDescription}
        </p>
      )}

      {/* 操作按钮 */}
      {action && <div>{action}</div>}
    </div>
  )
}

// 列表空状态组件
interface EmptyListProps {
  icon?: ReactNode
  title?: string
  description?: string
  action?: ReactNode
}

export function EmptyList({ icon, title, description, action }: EmptyListProps) {
  return (
    <Empty
      type="default"
      icon={icon}
      title={title}
      description={description}
      action={action}
      className="py-12"
    />
  )
}

// 搜索空状态组件
interface EmptySearchProps {
  keyword?: string
  onReset?: () => void
}

export function EmptySearch({ keyword, onReset }: EmptySearchProps) {
  return (
    <Empty
      type="search"
      title="未找到匹配结果"
      description={keyword ? `没有找到与"${keyword}"相关的内容` : '请尝试其他搜索条件'}
      action={
        onReset && (
          <button
            onClick={onReset}
            className="text-sm text-[var(--color-accent)] hover:text-[var(--color-accent)]/80 font-medium transition-colors"
          >
            清除筛选条件
          </button>
        )
      }
    />
  )
}

// 错误空状态组件
interface EmptyErrorProps {
  onRetry?: () => void
}

export function EmptyError({ onRetry }: EmptyErrorProps) {
  return (
    <Empty
      type="error"
      title="加载失败"
      description="数据加载时出现了问题，请稍后重试"
      action={
        onRetry && (
          <button
            onClick={onRetry}
            className="px-5 py-2.5 bg-[var(--color-accent)] text-[var(--color-text-inverse)] rounded-xl font-medium hover:opacity-90 transition-all duration-200 hover:-translate-y-0.5 shadow-md hover:shadow-lg"
          >
            重新加载
          </button>
        )
      }
    />
  )
}
