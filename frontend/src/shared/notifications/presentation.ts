import { AlertCircle, CheckCircle, Info, XCircle } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import type { NotificationSource, NotificationType } from '../../store/notificationProtocol'

export interface NotificationVisual {
  icon: LucideIcon
  iconClass: string
  iconBackground: string
  rail: string
}

export const notificationVisuals: Record<NotificationType, NotificationVisual> = {
  success: {
    icon: CheckCircle,
    iconClass: 'text-[var(--color-success)]',
    iconBackground: 'bg-[var(--color-bg-muted)]',
    rail: 'bg-[var(--color-success)]',
  },
  warning: {
    icon: AlertCircle,
    iconClass: 'text-[var(--color-warning)]',
    iconBackground: 'bg-[var(--color-bg-muted)]',
    rail: 'bg-[var(--color-warning)]',
  },
  error: {
    icon: XCircle,
    iconClass: 'text-[var(--color-error)]',
    iconBackground: 'bg-[var(--color-bg-muted)]',
    rail: 'bg-[var(--color-error)]',
  },
  info: {
    icon: Info,
    iconClass: 'text-[var(--color-info)]',
    iconBackground: 'bg-[var(--color-bg-muted)]',
    rail: 'bg-[var(--color-info)]',
  },
}

export const notificationSourceLabels: Record<NotificationSource, string> = {
  operation: '操作',
  runtime: '运行时',
  frontend: '前端',
  system: '系统',
}
