import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import {
  createNotificationPayload,
  notificationTitles,
  type Notification,
  type NotificationInput,
  type NotificationSource,
  type NotificationType,
} from './notificationProtocol'

export type { Notification } from './notificationProtocol'

interface NotificationState {
  notifications: Notification[]
  addNotification: (notification: NotificationInput) => void
  markAsRead: (id: string) => void
  markAllAsRead: () => void
  clearNotifications: () => void
}

const MAX_NOTIFICATIONS = 100
const DEDUPE_WINDOW_MS = 10_000
const SETTINGS_KEY = 'app_settings'
const NOTIFICATION_TYPES: readonly NotificationType[] = ['info', 'success', 'warning', 'error']
const NOTIFICATION_SOURCES: readonly NotificationSource[] = ['operation', 'runtime', 'frontend', 'system']

export function isNotificationHistoryEnabled() {
  if (typeof localStorage === 'undefined') return true

  try {
    const stored = localStorage.getItem(SETTINGS_KEY)
    if (!stored) return true
    const settings = JSON.parse(stored) as { enableNotifications?: unknown }
    return settings.enableNotifications !== false
  } catch {
    return true
  }
}

function formatNotificationTime(date: Date) {
  const pad = (value: number) => String(value).padStart(2, '0')
  return `${date.getFullYear()}/${pad(date.getMonth() + 1)}/${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

function isNotificationType(value: unknown): value is NotificationType {
  return typeof value === 'string' && NOTIFICATION_TYPES.includes(value as NotificationType)
}

function isNotificationSource(value: unknown): value is NotificationSource {
  return typeof value === 'string' && NOTIFICATION_SOURCES.includes(value as NotificationSource)
}

function parseStoredDate(value: unknown) {
  if (typeof value !== 'string' || !value.trim()) return null
  const timestamp = Date.parse(value)
  return Number.isFinite(timestamp) ? new Date(timestamp) : null
}

function normalizeAction(value: unknown): Notification['action'] {
  if (!value || typeof value !== 'object') return undefined

  const action = value as Record<string, unknown>
  if (action.type !== 'navigate' || typeof action.label !== 'string' || typeof action.path !== 'string') {
    return undefined
  }

  const label = action.label.trim()
  const path = action.path.trim()
  return label && path ? { type: 'navigate', label, path } : undefined
}

function normalizeNotification(value: unknown, index: number): Notification | null {
  if (!value || typeof value !== 'object') return null

  const raw = value as Record<string, unknown>
  const type = isNotificationType(raw.type) ? raw.type : 'info'
  const message = typeof raw.message === 'string' ? raw.message.trim() : ''
  if (!message) return null

  const createdAt = parseStoredDate(raw.createdAt) ?? parseStoredDate(raw.time) ?? new Date(0)
  const time = typeof raw.time === 'string' && raw.time.trim()
    ? raw.time
    : formatNotificationTime(createdAt)

  return {
    type,
    title: typeof raw.title === 'string' && raw.title.trim() ? raw.title.trim() : notificationTitles[type],
    message,
    source: isNotificationSource(raw.source) ? raw.source : 'system',
    dedupeKey: typeof raw.dedupeKey === 'string' ? raw.dedupeKey.trim() || undefined : undefined,
    persistent: typeof raw.persistent === 'boolean' ? raw.persistent : true,
    action: normalizeAction(raw.action),
    id: typeof raw.id === 'string' && raw.id.trim()
      ? raw.id
      : `legacy-${index}-${Math.random().toString(36).slice(2, 9)}`,
    time,
    createdAt: createdAt.toISOString(),
    read: raw.read === true,
  }
}

function normalizeStoredNotifications(value: unknown) {
  if (!Array.isArray(value)) return []
  return value
    .map((notification, index) => normalizeNotification(notification, index))
    .filter((notification): notification is Notification => notification !== null)
    .slice(0, MAX_NOTIFICATIONS)
}

function isRecentDuplicate(notification: Notification, dedupeKey: string, now: number) {
  if (notification.dedupeKey !== dedupeKey) return false
  const createdAt = Date.parse(notification.createdAt)
  return Number.isFinite(createdAt) && now - createdAt < DEDUPE_WINDOW_MS
}

export const useNotificationStore = create<NotificationState>()(persist((set) => ({
  notifications: [],

  addNotification: (data) => {
    const payload = createNotificationPayload(data)
    if (!isNotificationHistoryEnabled() || !payload.persistent) return

    set((state) => {
      const now = Date.now()
      const dedupeKey = payload.dedupeKey
      if (dedupeKey && state.notifications.some((notification) => isRecentDuplicate(notification, dedupeKey, now))) {
        return state
      }

      const createdAt = new Date(now)
      const newNotification: Notification = {
        ...payload,
        id: Math.random().toString(36).substring(2, 9),
        time: formatNotificationTime(createdAt),
        createdAt: createdAt.toISOString(),
        read: false,
      }
      return { notifications: [newNotification, ...state.notifications].slice(0, MAX_NOTIFICATIONS) }
    })
  },

  markAsRead: (id) => set((state) => ({
    notifications: state.notifications.map((notification) =>
      notification.id === id ? { ...notification, read: true } : notification,
    ),
  })),

  markAllAsRead: () => set((state) => ({
    notifications: state.notifications.map((notification) => ({ ...notification, read: true })),
  })),

  clearNotifications: () => set({ notifications: [] }),
}), {
  name: 'ant-browser-notifications',
  version: 1,
  migrate: (persistedState) => {
    const state = persistedState as { notifications?: unknown } | undefined
    return { notifications: normalizeStoredNotifications(state?.notifications) }
  },
  partialize: (state) => ({ notifications: state.notifications }),
}))
