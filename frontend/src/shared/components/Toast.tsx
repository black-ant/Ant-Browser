import { useEffect, useRef, useState } from 'react'
import type { FocusEvent } from 'react'
import { X } from 'lucide-react'
import { create } from 'zustand'
import { Link } from 'react-router-dom'
import { useNotificationStore } from '../../store/notificationStore'
import {
  createNotificationPayload,
  notificationDurations,
  type NotificationInput,
  type NotificationPayload,
  type NotificationType,
} from '../../store/notificationProtocol'
import { notificationVisuals } from '../notifications/presentation'

interface Toast extends NotificationPayload {
  id: string
  duration?: number
}

type ToastOptions = Omit<NotificationInput, 'type' | 'message'>

const TOAST_DEDUPE_WINDOW_MS = 10_000
const toastDedupeTimestamps = new Map<string, number>()

interface ToastStore {
  toasts: Toast[]
  addToast: (toast: Omit<Toast, 'id'>) => void
  removeToast: (id: string) => void
}

export const useToastStore = create<ToastStore>((set) => ({
  toasts: [],
  addToast: (toast) => {
    const id = Math.random().toString(36).substring(7)
    set((state) => ({
      toasts: [...state.toasts, { ...toast, id }],
    }))
  },
  removeToast: (id) =>
    set((state) => ({
      toasts: state.toasts.filter((t) => t.id !== id),
    })),
}))

export const toast = {
  success: (message: string, duration?: number, options?: ToastOptions) =>
    pushToast('success', message, duration, options),
  error: (message: string, duration?: number, options?: ToastOptions) =>
    pushToast('error', message, duration, options),
  warning: (message: string, duration?: number, options?: ToastOptions) =>
    pushToast('warning', message, duration, options),
  info: (message: string, duration?: number, options?: ToastOptions) =>
    pushToast('info', message, duration, options),
}

function pushToast(type: NotificationType, message: string, duration?: number, options?: ToastOptions) {
  const payload = createNotificationPayload({ type, message, ...options })
  if (shouldShowToast(payload.dedupeKey)) {
    useToastStore.getState().addToast({ ...payload, duration })
  }
  useNotificationStore.getState().addNotification(payload)
}

function shouldShowToast(dedupeKey?: string) {
  if (!dedupeKey) return true

  const now = Date.now()
  const previous = toastDedupeTimestamps.get(dedupeKey)
  if (previous !== undefined && now - previous < TOAST_DEDUPE_WINDOW_MS) return false

  toastDedupeTimestamps.set(dedupeKey, now)
  globalThis.setTimeout(() => {
    if (toastDedupeTimestamps.get(dedupeKey) === now) toastDedupeTimestamps.delete(dedupeKey)
  }, TOAST_DEDUPE_WINDOW_MS)
  return true
}

function ToastItem({ toast: t }: { toast: Toast }) {
  const removeToast = useToastStore((state) => state.removeToast)
  const visual = notificationVisuals[t.type]
  const Icon = visual.icon
  const duration = t.duration ?? notificationDurations[t.type]
  const remainingMs = useRef(duration)
  const startedAt = useRef<number | null>(null)
  const [paused, setPaused] = useState(false)
  const [leaving, setLeaving] = useState(false)

  const startExit = () => {
    if (!leaving) setLeaving(true)
  }

  useEffect(() => {
    if (duration <= 0 || paused) return

    startedAt.current = Date.now()
    const timer = window.setTimeout(startExit, remainingMs.current)

    return () => {
      if (startedAt.current !== null) {
        remainingMs.current = Math.max(0, remainingMs.current - (Date.now() - startedAt.current))
        startedAt.current = null
      }
      window.clearTimeout(timer)
    }
  }, [duration, paused, t.id])

  useEffect(() => {
    if (!leaving) return
    const timer = window.setTimeout(() => removeToast(t.id), 220)
    return () => window.clearTimeout(timer)
  }, [leaving, removeToast, t.id])

  const pause = () => setPaused(true)
  const resume = () => setPaused(false)
  const handleBlur = (event: FocusEvent<HTMLDivElement>) => {
    if (event.currentTarget.contains(event.relatedTarget as Node | null)) return
    resume()
  }
  const isCritical = t.type === 'error' || t.type === 'warning'
  const isError = t.type === 'error'
  const toneClass = isError ? 'toast-error' : t.type === 'warning' ? 'toast-warning' : ''
  const titleClass = isError
    ? 'font-bold text-[var(--color-error)]'
    : 'font-semibold text-[var(--color-text-primary)]'
  const iconSizeClass = isCritical ? 'h-9 w-9' : 'h-8 w-8'
  const railWidthClass = isError ? 'w-1.5' : 'w-1'

  return (
    <div
      role={isCritical ? 'alert' : 'status'}
      aria-live={isCritical ? 'assertive' : 'polite'}
      aria-atomic="true"
      onMouseEnter={pause}
      onMouseLeave={resume}
      onFocus={pause}
      onBlur={handleBlur}
      className={`pointer-events-auto relative flex w-full items-start gap-3 overflow-hidden rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-4 py-3.5 shadow-xl ${toneClass} ${leaving ? 'animate-toast-out' : 'animate-toast-in'}`}
    >
      <span aria-hidden="true" className={`absolute inset-y-0 left-0 ${railWidthClass} ${visual.rail}`} />
      <span className={`mt-0.5 flex shrink-0 items-center justify-center rounded-lg ${iconSizeClass} ${visual.iconBackground}`}>
        <Icon className={`h-5 w-5 ${visual.iconClass}`} />
      </span>
      <div className="min-w-0 flex-1">
        <p className={`text-sm leading-5 ${titleClass}`}>{t.title}</p>
        <p className="mt-0.5 break-words text-sm leading-5 text-[var(--color-text-secondary)]">{t.message}</p>
        {t.action?.type === 'navigate' && (
          <Link
            to={t.action.path}
            onClick={startExit}
            className="mt-3 inline-flex cursor-pointer items-center rounded-md border border-[var(--color-border-strong)] bg-[var(--color-bg-surface)] px-2.5 py-1.5 text-xs font-medium text-[var(--color-text-primary)] transition-colors hover:bg-[var(--color-bg-muted)] focus:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent)] focus-visible:ring-offset-1"
          >
            {t.action.label}
          </Link>
        )}
      </div>
      <button
        type="button"
        onClick={startExit}
        aria-label="关闭通知"
        title="关闭通知"
        className="-mr-1 -mt-1 flex h-7 w-7 shrink-0 items-center justify-center rounded-md text-[var(--color-text-muted)] transition-colors hover:bg-[var(--color-bg-muted)] hover:text-[var(--color-text-primary)]"
      >
        <X className="h-4 w-4" />
      </button>
    </div>
  )
}

export function ToastContainer() {
  const toasts = useToastStore((state) => state.toasts)
  const [flashKey, setFlashKey] = useState<string | null>(null)
  const visibleToastIds = useRef(new Set<string>())

  useEffect(() => {
    const currentToastIds = new Set(toasts.map((toast) => toast.id))
    const addedToast = [...toasts].reverse().find((toast) => !visibleToastIds.current.has(toast.id))

    if (addedToast) {
      setFlashKey(addedToast.id)
    }

    visibleToastIds.current = currentToastIds
  }, [toasts])

  return (
    <>
      {flashKey ? (
        <div
          key={flashKey}
          aria-hidden="true"
          onAnimationEnd={() => setFlashKey((current) => (current === flashKey ? null : current))}
          className="toast-flash pointer-events-none fixed inset-0 z-[9995]"
        />
      ) : null}
      <div className="pointer-events-none fixed left-4 right-4 top-[4.5rem] z-[10000] flex max-h-[calc(100vh-6rem)] w-auto flex-col gap-3 overflow-y-auto overscroll-contain sm:left-auto sm:right-4 sm:w-[min(480px,calc(100vw-2rem))]">
        {[...toasts].reverse().map((t) => (
          <ToastItem key={t.id} toast={t} />
        ))}
      </div>
    </>
  )
}
