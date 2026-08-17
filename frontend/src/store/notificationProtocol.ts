export type NotificationType = 'info' | 'success' | 'warning' | 'error'
export type NotificationSource = 'operation' | 'runtime' | 'frontend' | 'system'

export interface NotificationAction {
  type: 'navigate'
  label: string
  path: string
}

export interface NotificationInput {
  type: NotificationType
  title?: string
  message: string
  source?: NotificationSource
  dedupeKey?: string
  persistent?: boolean
  action?: NotificationAction
}

export interface NotificationPayload {
  type: NotificationType
  title: string
  message: string
  source: NotificationSource
  dedupeKey?: string
  persistent: boolean
  action?: NotificationAction
}

export interface Notification extends NotificationPayload {
  id: string
  time: string
  createdAt: string
  read: boolean
}

export const notificationTitles: Record<NotificationType, string> = {
  success: '操作成功',
  error: '操作失败',
  warning: '注意',
  info: '提示',
}

export const notificationDurations: Record<NotificationType, number> = {
  success: 4000,
  error: 8000,
  warning: 6000,
  info: 4000,
}

export function createNotificationPayload(input: NotificationInput): NotificationPayload {
  const source = input.source ?? 'operation'
  const message = input.message.trim() || '未提供详细信息'
  const action = input.action?.label.trim() && input.action.path.trim()
    ? {
        ...input.action,
        label: input.action.label.trim(),
        path: input.action.path.trim(),
      }
    : undefined

  return {
    type: input.type,
    title: input.title?.trim() || notificationTitles[input.type],
    message,
    source,
    dedupeKey: input.dedupeKey?.trim() || undefined,
    persistent: input.persistent ?? (input.type === 'error' || source === 'runtime' || source === 'system'),
    action,
  }
}
