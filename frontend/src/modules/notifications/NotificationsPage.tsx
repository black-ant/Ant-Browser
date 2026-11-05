import { useEffect, useMemo, useState } from 'react'
import { ArrowRight, Bell, Check, Trash2 } from 'lucide-react'
import clsx from 'clsx'
import { Link } from 'react-router-dom'
import { Button } from '../../shared/components'
import { notificationSourceLabels, notificationVisuals } from '../../shared/notifications/presentation'
import { NotificationMessage } from '../../shared/notifications/NotificationMessage'
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
  useEffect(() => {
    markAllAsRead()
  }, [markAllAsRead])

  const unreadCount = notifications.filter((notification) => !notification.read).length
  const filterCounts = useMemo(() => ({
    all: notifications.length,
    unread: unreadCount,
    error: notifications.filter((notification) => notification.type === 'error').length,
    warning: notifications.filter((notification) => notification.type === 'warning').length,
  }), [notifications, unreadCount])
  const filteredNotifications = useMemo(
    () => notifications.filter((notification) => matchesFilter(notification, filter)),
    [filter, notifications],
  )

  return (
    <div className="mx-auto max-w-6xl space-y-3 animate-fade-in">
      <section className="overflow-hidden rounded-2xl border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] shadow-[var(--shadow-xs)]">
        <div className="flex flex-col gap-3 px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex min-w-0 items-center gap-3">
            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-[var(--color-bg-muted)] text-[var(--color-text-secondary)]">
              <Bell className="h-4 w-4" />
            </div>
            <div className="flex min-w-0 items-center gap-2">
              <h1 className="truncate text-lg font-semibold text-[var(--color-text-primary)]">通知中心</h1>
              {unreadCount > 0 && (
                <span className="shrink-0 rounded-full bg-[var(--color-error)]/10 px-2 py-0.5 text-[11px] font-medium text-[var(--color-error)]">
                  {unreadCount} 条未读
                </span>
              )}
            </div>
          </div>
          <div className="flex flex-wrap gap-2 sm:justify-end">
            {unreadCount > 0 && (
              <Button type="button" size="sm" onClick={markAllAsRead} className="cursor-pointer">
                <Check className="w-4 h-4" />
                全部标为已读
              </Button>
            )}
            {notifications.length > 0 && (
              <Button
                type="button"
                variant="secondary"
                size="sm"
                onClick={clearNotifications}
                className="cursor-pointer text-[var(--color-error)] hover:border-[var(--color-error)]/40 hover:text-[var(--color-error)]"
              >
                <Trash2 className="w-4 h-4" />
                清空通知
              </Button>
            )}
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-1 border-t border-[var(--color-border-muted)] px-3 py-2" role="tablist" aria-label="通知筛选">
          {filters.map((item) => {
            const isActive = filter === item.value

            return (
              <button
                key={item.value}
                type="button"
                role="tab"
                aria-selected={isActive}
                aria-controls="notifications-list"
                onClick={() => setFilter(item.value)}
                className={clsx(
                  'inline-flex h-8 cursor-pointer items-center rounded-lg px-3 text-xs font-medium transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent)] focus-visible:ring-offset-1',
                  isActive
                    ? 'bg-[var(--color-accent)] text-[var(--color-text-inverse)]'
                    : 'text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-muted)] hover:text-[var(--color-text-primary)]',
                )}
              >
                {item.label}
                <span className={clsx('ml-1.5 text-[10px]', isActive ? 'opacity-70' : 'text-[var(--color-text-muted)]')}>
                  {filterCounts[item.value]}
                </span>
              </button>
            )
          })}
        </div>
      </section>

      <section
        id="notifications-list"
        aria-live="polite"
        className="rounded-2xl border border-[var(--color-border-default)] bg-[var(--color-bg-muted)] p-2 shadow-[var(--shadow-xs)]"
      >
        {filteredNotifications.length === 0 ? (
          <div className="flex flex-col items-center justify-center rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-6 py-12 text-center text-[var(--color-text-muted)] shadow-[var(--shadow-xs)]">
            <div className="mb-3 flex h-11 w-11 items-center justify-center rounded-2xl bg-[var(--color-bg-muted)]">
              <Bell className="h-5 w-5 opacity-60" />
            </div>
            <p className="text-sm">{notifications.length === 0 ? '暂无通知' : '当前筛选没有通知'}</p>
          </div>
        ) : (
          <div className="space-y-1.5">
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
                    'relative grid grid-cols-[2.25rem_minmax(0,1fr)] gap-3 overflow-hidden rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-4 py-3 shadow-[var(--shadow-xs)] transition-colors duration-200 hover:bg-[var(--color-bg-elevated)] sm:grid-cols-[2.25rem_minmax(0,1fr)_auto]',
                  )}
                >
                  <span aria-hidden="true" className={clsx('absolute inset-y-0 left-0 w-0.5', visual.rail, notification.read && 'opacity-30')} />
                  <span className={`flex h-9 w-9 items-center justify-center rounded-xl ${visual.iconBackground}`}>
                    <Icon className={`h-5 w-5 ${visual.iconClass}`} />
                  </span>
                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
                      {!notification.read && <span aria-label="未读" className="h-1.5 w-1.5 shrink-0 rounded-full bg-[var(--color-error)]" />}
                      <h2 className={clsx(
                        'break-words text-sm leading-5',
                        notification.read ? 'font-medium text-[var(--color-text-secondary)]' : 'font-semibold text-[var(--color-text-primary)]',
                      )}>
                        {notification.title}
                      </h2>
                    </div>
                    <div className="mt-1 flex flex-wrap items-center gap-2 text-[11px] leading-4 text-[var(--color-text-muted)]">
                      <span>{sourceLabel}</span>
                      <span aria-hidden="true" className="text-[var(--color-border-strong)]">·</span>
                      <time dateTime={notification.createdAt}>{notification.time}</time>
                    </div>
                    <NotificationMessage
                      message={notification.message}
                      context={notification.message.includes('备份') || notification.message.includes('OpenList') || notification.message.includes('S3') ? 'backup' : 'generic'}
                      className='mt-2 max-w-4xl text-[var(--color-text-secondary)]'
                    />
                    {notification.action?.type === 'navigate' && (
                      <Link
                        to={notification.action.path}
                        onClick={() => markAsRead(notification.id)}
                        className="mt-2 inline-flex cursor-pointer items-center gap-1 rounded-md border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-2.5 py-1.5 text-xs font-medium text-[var(--color-text-primary)] transition-colors hover:border-[var(--color-border-strong)] hover:bg-[var(--color-bg-muted)] focus:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent)] focus-visible:ring-offset-1"
                      >
                        {notification.action.label}
                        <ArrowRight className="h-3.5 w-3.5" />
                      </Link>
                    )}
                  </div>
                  {!notification.read && (
                    <div className="col-start-2 sm:col-auto sm:pt-0.5">
                      <Button
                        type="button"
                        variant="secondary"
                        size="sm"
                        onClick={() => markAsRead(notification.id)}
                        aria-label={`标记为已读：${notification.title}`}
                        className="h-8 cursor-pointer whitespace-nowrap px-2.5"
                      >
                        <Check className="h-3.5 w-3.5" />
                        标记已读
                      </Button>
                    </div>
                  )}
                </article>
              )
            })}
          </div>
        )}
      </section>
    </div>
  )
}
