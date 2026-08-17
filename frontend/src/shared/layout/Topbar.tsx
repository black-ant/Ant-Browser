import { useState, useRef, useEffect } from 'react'
import { Bell, User, Settings, Check, Trash2 } from 'lucide-react'
import { Link } from 'react-router-dom'
import clsx from 'clsx'
import { useNotificationStore, type Notification } from '../../store/notificationStore'
import { GetAppConfig } from '../../wailsjs/go/main/App'
import { notificationVisuals } from '../notifications/presentation'

function NotificationDropdown({
  notifications,
  onMarkAsRead,
  onMarkAllAsRead,
  onClear,
  onClose,
}: {
  notifications: Notification[]
  onMarkAsRead: (id: string) => void
  onMarkAllAsRead: () => void
  onClear: () => void
  onClose: () => void
}) {
  const unreadCount = notifications.filter(n => !n.read).length

  return (
    <div
      role="dialog"
      aria-label="通知中心"
      className="absolute right-0 top-full z-50 mt-2 w-[min(360px,calc(100vw-2rem))] overflow-hidden rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] shadow-xl animate-fade-in"
    >
      {/* Header */}
      <div className="flex items-center justify-between border-b border-[var(--color-border-muted)] px-4 py-3">
        <div className="flex items-center gap-2">
          <span className="text-sm font-semibold text-[var(--color-text-primary)]">通知中心</span>
          {unreadCount > 0 && (
            <span
              aria-label={`${unreadCount} 条未读通知`}
              className="rounded-full bg-[var(--color-accent)] px-1.5 py-0.5 text-xs font-medium text-white"
            >
              {unreadCount}
            </span>
          )}
        </div>
        <div className="flex items-center gap-1">
          {unreadCount > 0 && (
            <button
              type="button"
              onClick={onMarkAllAsRead}
              aria-label="全部标为已读"
              title="全部标为已读"
              className="flex h-7 w-7 items-center justify-center rounded-md border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] text-[var(--color-text-muted)] transition-colors hover:border-[var(--color-border-strong)] hover:bg-[var(--color-bg-muted)] hover:text-[var(--color-accent)]"
            >
              <Check className="h-3.5 w-3.5" />
            </button>
          )}
          {notifications.length > 0 && (
            <button
              type="button"
              onClick={onClear}
              aria-label="清空通知"
              title="清空通知"
              className="flex h-7 w-7 items-center justify-center rounded-md border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] text-[var(--color-text-muted)] transition-colors hover:border-[var(--color-error)] hover:bg-[var(--color-bg-muted)] hover:text-[var(--color-error)]"
            >
              <Trash2 className="h-3.5 w-3.5" />
            </button>
          )}
        </div>
      </div>

      {/* Notification List */}
      <div className="max-h-[22rem] overflow-y-auto">
        {notifications.length === 0 ? (
          <div className="py-8 text-center text-[var(--color-text-muted)]">
            <Bell className="mx-auto mb-2 h-8 w-8 opacity-50" />
            <p className="text-sm">暂无通知</p>
          </div>
        ) : (
          notifications.slice(0, 5).map((notification) => {
            const visual = notificationVisuals[notification.type]
            const Icon = visual.icon

            return (
              <button
                key={notification.id}
                type="button"
                onClick={() => onMarkAsRead(notification.id)}
                aria-label={notification.read ? notification.title : `标记为已读：${notification.title}`}
                className={clsx(
                  'relative flex w-full items-start gap-3 border-b border-[var(--color-border-muted)] px-4 py-3 text-left transition-colors last:border-0 hover:bg-[var(--color-bg-muted)] focus:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-[var(--color-accent)]',
                  !notification.read && 'bg-[var(--color-accent-muted)]'
                )}
              >
                <span aria-hidden="true" className={`absolute inset-y-0 left-0 w-1 ${visual.rail}`} />
                <span className={`mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-lg ${visual.iconBackground}`}>
                  <Icon className={`h-4 w-4 ${visual.iconClass}`} />
                </span>
                <span className="min-w-0 flex-1">
                  <span className="flex items-start justify-between gap-2">
                    <span className={clsx(
                      'min-w-0 break-words text-sm leading-5',
                      notification.read ? 'text-[var(--color-text-secondary)]' : 'font-semibold text-[var(--color-text-primary)]'
                    )}>
                      {notification.title}
                    </span>
                    {!notification.read && (
                      <span aria-label="未读" className="mt-1.5 h-2 w-2 shrink-0 rounded-full bg-[var(--color-accent)]" />
                    )}
                  </span>
                  <span className="mt-0.5 block line-clamp-3 break-words text-xs leading-4 text-[var(--color-text-secondary)]">
                    {notification.message}
                  </span>
                  <span className="mt-1 block text-[11px] leading-4 text-[var(--color-text-muted)]">
                    {notification.time}
                  </span>
                </span>
              </button>
            )
          })
        )}
      </div>
      {notifications.length > 0 && (
        <div className="border-t border-[var(--color-border-muted)] bg-[var(--color-bg-muted)] px-4 py-2">
          <Link
            to="/notifications"
            onClick={onClose}
            className="block w-full text-center text-xs text-[var(--color-accent)] hover:underline"
          >
            查看全部通知
          </Link>
        </div>
      )}
    </div>
  )
}

export function Topbar() {
  const [showNotifications, setShowNotifications] = useState(false)
  const [appVersion, setAppVersion] = useState('')
  const { notifications, markAsRead, markAllAsRead, clearNotifications } = useNotificationStore()
  const dropdownRef = useRef<HTMLDivElement>(null)

  const unreadCount = notifications.filter(n => !n.read).length

  useEffect(() => {
    const app = (window as any).go?.main?.App
    if (!app?.GetAppConfig) return

    GetAppConfig()
      .then((config) => {
        const version = typeof config?.version === 'string' ? config.version.trim() : ''
        setAppVersion(version)
      })
      .catch(() => {
        setAppVersion('')
      })
  }, [])

  // 点击外部关闭
  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setShowNotifications(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  return (
    <header className="h-14 bg-[var(--color-bg-surface)] border-b border-[var(--color-border-default)] px-4 flex items-center justify-between gap-4">
      <div className="flex-1" />

      {appVersion && (
        <span className='text-xs font-medium text-[var(--color-text-muted)]'>
          v{appVersion}
        </span>
      )}

      {/* 右侧操作 */}
      <div className="flex items-center gap-1">
        {/* 通知按钮 */}
        <div className="relative" ref={dropdownRef}>
          <button
            type="button"
            onClick={() => setShowNotifications(!showNotifications)}
            aria-label={`通知${unreadCount > 0 ? `，${unreadCount} 条未读` : ''}`}
            aria-expanded={showNotifications}
            aria-haspopup="dialog"
            className={clsx(
              'relative w-8 h-8 flex items-center justify-center rounded-md transition-colors duration-150',
              showNotifications
                ? 'text-[var(--color-accent)] bg-[var(--color-accent-muted)]'
                : 'text-[var(--color-text-muted)] hover:text-[var(--color-text-secondary)] hover:bg-[var(--color-accent-muted)]'
            )}
            title="通知"
          >
            <Bell className="w-4 h-4" />
            {unreadCount > 0 && (
              <span className="absolute -top-0.5 -right-0.5 w-4 h-4 text-[10px] font-medium bg-[var(--color-error)] text-white rounded-full flex items-center justify-center">
                {unreadCount > 9 ? '9+' : unreadCount}
              </span>
            )}
          </button>

          {showNotifications && (
            <NotificationDropdown
              notifications={notifications}
              onMarkAsRead={markAsRead}
              onMarkAllAsRead={markAllAsRead}
              onClose={() => setShowNotifications(false)}
              onClear={() => {
                clearNotifications()
                setShowNotifications(false)
              }}
            />
          )}
        </div>

        <Link
          to="/settings"
          className="w-8 h-8 flex items-center justify-center text-[var(--color-text-muted)] hover:text-[var(--color-text-secondary)] hover:bg-[var(--color-accent-muted)] rounded-md transition-colors duration-150"
          title="设置"
        >
          <Settings className="w-4 h-4" />
        </Link>

        <div className="w-px h-5 bg-[var(--color-border-default)] mx-1.5" />

        <Link
          to="/profile"
          className="flex items-center gap-2 pl-1 pr-2.5 py-1 rounded-md hover:bg-[var(--color-accent-muted)] transition-colors duration-150"
        >
          <div className="w-7 h-7 bg-[var(--color-accent)] rounded-md flex items-center justify-center">
            <User className="w-3.5 h-3.5 text-[var(--color-text-inverse)]" />
          </div>
          <span className="text-sm font-medium text-[var(--color-text-secondary)]">Admin</span>
        </Link>
      </div>
    </header>
  )
}
