import { useMemo, useState } from 'react'
import { Bell, Check, Trash2 } from 'lucide-react'
import clsx from 'clsx'
import { Link } from 'react-router-dom'
import { Button } from '../../shared/components'
import { notificationSourceLabels, notificationVisuals } from '../../shared/notifications/presentation'
import { useNotificationStore, type Notification } from '../../store/notificationStore'

type NotificationFilter = 'all' | 'unread' | 'error' | 'warning'

const filters: Array<{ value: NotificationFilter; label: string }> = [
  { value: 'all', label: '全部' },
  { value: 'unread', label: '未读' },
  { value: 'error', label: '错误' },
  { value: 'warning', label: '警告' },
]

function matchesFilter(notification: Notification, filter: NotificationFilter) {
  if (filter === 'unread') return !notification.read
  if (filter === 'error' || filter === 'warning') return notification.type === filter
  return true
}

export function NotificationsPage() {
  const { notifications, markAsRead, markAllAsRead, clearNotifications } = useNotificationStore()
  const [filter, setFilter] = useState<NotificationFilter>('all')
  const unreadCount = notifications.filter((notification) => !notification.read).length
  const filteredNotifications = useMemo(
    () => notifications.filter((notification) => matchesFilter(notification, filter)),
    [filter, notifications],
  )

  return (
    <div className="space-y-5 animate-fade-in">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">通知中心</h1>
        <div className="flex items-center gap-2">
          {unreadCount > 0 && (
            <Button variant="secondary" size="sm" onClick={markAllAsRead}>
              <Check className="h-3.5 w-3.5" />
              全部标为已读
            </Button>
          )}
          {notifications.length > 0 && (
            <Button variant="secondary" size="sm" onClick={clearNotifications}>
              <Trash2 className="h-3.5 w-3.5" />
              清空通知
            </Button>
          )}
        </div>
      </div>

      <div className="flex flex-wrap gap-2" role="tablist" aria-label="通知筛选">
        {filters.map((item) => (
          <Button
            key={item.value}
            type="button"
            size="sm"
            variant={filter === item.value ? 'primary' : 'secondary'}
            role="tab"
            aria-selected={filter === item.value}
            onClick={() => setFilter(item.value)}
          >
            {item.label}
          </Button>
        ))}
      </div>

      <section className="overflow-hidden rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] shadow-[var(--shadow-xs)]">
        {filteredNotifications.length === 0 ? (
          <div className="flex min-h-64 flex-col items-center justify-center px-6 text-center text-[var(--color-text-muted)]">
            <Bell className="mb-3 h-9 w-9 opacity-50" />
            <p className="text-sm">{notifications.length === 0 ? '暂无通知' : '当前筛选没有通知'}</p>
          </div>
        ) : (
          <div>
            {filteredNotifications.map((notification) => {
              const visual = notificationVisuals[notification.type]
              const Icon = visual.icon
              const sourceLabel = notification.source
                ? notificationSourceLabels[notification.source]
                : '系统'

              return (
                <article
                  key={notification.id}
                  className={clsx(
                    'relative flex gap-4 border-b border-[var(--color-border-muted)] px-5 py-4 last:border-0',
                    !notification.read && 'bg-[var(--color-accent-muted)]',
                  )}
                >
                  <span aria-hidden="true" className={`absolute inset-y-0 left-0 w-1 ${visual.rail}`} />
                  <span className={`mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-lg ${visual.iconBackground}`}>
                    <Icon className={`h-5 w-5 ${visual.iconClass}`} />
                  </span>
                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-start justify-between gap-3">
                      <div className="min-w-0">
                        <h2 className={clsx(
                          'break-words text-sm leading-5',
                          notification.read ? 'font-medium text-[var(--color-text-secondary)]' : 'font-semibold text-[var(--color-text-primary)]',
                        )}>
                          {notification.title}
                        </h2>
                        <div className="mt-1 flex flex-wrap items-center gap-2 text-[11px] leading-4 text-[var(--color-text-muted)]">
                          <span>{sourceLabel}</span>
                          <span aria-hidden="true">·</span>
                          <time>{notification.time}</time>
                        </div>
                      </div>
                      {!notification.read && (
                        <Button
                          variant="secondary"
                          size="sm"
                          onClick={() => markAsRead(notification.id)}
                          aria-label={`标记为已读：${notification.title}`}
                        >
                          标记已读
                        </Button>
                      )}
                    </div>
                    <p className="mt-2 whitespace-pre-wrap break-words text-sm leading-6 text-[var(--color-text-secondary)]">
                      {notification.message}
                    </p>
                    {notification.action?.type === 'navigate' && (
                      <Link
                        to={notification.action.path}
                        onClick={() => markAsRead(notification.id)}
                        className="mt-3 inline-flex cursor-pointer items-center rounded-md border border-[var(--color-border-strong)] bg-[var(--color-bg-surface)] px-3 py-1.5 text-xs font-medium text-[var(--color-text-primary)] transition-colors hover:bg-[var(--color-bg-muted)] focus:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent)] focus-visible:ring-offset-1"
                      >
                        {notification.action.label}
                      </Link>
                    )}
                  </div>
                </article>
              )
            })}
          </div>
        )}
      </section>
    </div>
  )
}
