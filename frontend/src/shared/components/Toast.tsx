import { useCallback, useEffect, useRef, useState } from 'react'
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
const TOAST_SUCCESS_BACKDROP_DURATION = 1_200
const TOAST_BACKDROP_EXIT_DURATION = 460
const TOAST_BACKDROP_EXIT_DURATIONS: Record<NotificationType, number> = {
  success: TOAST_BACKDROP_EXIT_DURATION,
  info: 360,
  warning: 360,
  error: 360,
}
const TOAST_FOCUS_PRIORITY: Record<NotificationType, number> = {
  success: 1,
  info: 2,
  warning: 3,
  error: 4,
}
const TOAST_EXIT_DURATIONS: Record<NotificationType, number> = {
  success: 560,
  info: 360,
  warning: 360,
  error: 360,
}
const toastDedupeTimestamps = new Map<string, number>()

interface ToastFocus {
  id: string
  type: NotificationType
  phase: 'enter' | 'leave'
  duration: number
  exitDuration: number
}

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

function selectToastFocus(toasts: Toast[]) {
  return [...toasts].sort((left, right) => TOAST_FOCUS_PRIORITY[right.type] - TOAST_FOCUS_PRIORITY[left.type])[0] ?? null
}

function getToastDuration(toast: Pick<Toast, 'type' | 'duration'>) {
  if (toast.duration === undefined) return notificationDurations[toast.type]
  if (toast.duration <= 0 || toast.type === 'success') return toast.duration
  return Math.round(toast.duration * 0.8)
}

function getToastBackdropDuration(toast: Toast) {
  if (toast.type === 'success') return TOAST_SUCCESS_BACKDROP_DURATION
  return getToastDuration(toast)
}

function ToastItem({ toast: t, onManualDismiss }: { toast: Toast; onManualDismiss: (dismissedId: string) => void }) {
  const removeToast = useToastStore((state) => state.removeToast)
  const visual = notificationVisuals[t.type]
  const Icon = visual.icon
  const duration = getToastDuration(t)
  const exitDuration = TOAST_EXIT_DURATIONS[t.type]
  const exitAnimation = t.type === 'success' ? 'animate-toast-success-out' : 'animate-toast-out'
  const [leaving, setLeaving] = useState(false)

  const startExit = useCallback(
    (manual = false) => {
      if (manual) onManualDismiss(t.id)
      setLeaving(true)
    },
    [onManualDismiss, t.id],
  )

  useEffect(() => {
    if (duration <= 0) return

    const timer = window.setTimeout(startExit, duration)
    return () => window.clearTimeout(timer)
  }, [duration, startExit])

  useEffect(() => {
    if (!leaving) return
    const timer = window.setTimeout(() => removeToast(t.id), exitDuration)
    return () => window.clearTimeout(timer)
  }, [exitDuration, leaving, removeToast, t.id])

  const isCritical = t.type === 'error' || t.type === 'warning'
  const isError = t.type === 'error'
  const toneClass = `toast-${t.type}`
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
      className={`toast-card pointer-events-auto relative flex w-full items-start gap-3 overflow-hidden rounded-xl border px-4 py-3.5 ${toneClass} ${leaving ? exitAnimation : 'animate-toast-in'}`}
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
            onClick={() => startExit(true)}
            className="mt-3 inline-flex cursor-pointer items-center rounded-md border border-[var(--color-border-strong)] bg-[var(--color-bg-surface)] px-2.5 py-1.5 text-xs font-medium text-[var(--color-text-primary)] transition-colors hover:bg-[var(--color-bg-muted)] focus:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent)] focus-visible:ring-offset-1"
          >
            {t.action.label}
          </Link>
        )}
      </div>
      <button
        type="button"
        onClick={() => startExit(true)}
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
  const [focus, setFocus] = useState<ToastFocus | null>(null)
  const focusTimer = useRef<number | null>(null)
  const focusExitTimer = useRef<number | null>(null)
  const focusRef = useRef<ToastFocus | null>(null)
  const visibleToastIds = useRef(new Set<string>())

  useEffect(() => {
    const currentToastIds = new Set(toasts.map((toast) => toast.id))
    const addedToast = [...toasts].reverse().find((toast) => !visibleToastIds.current.has(toast.id))
    visibleToastIds.current = currentToastIds
    if (!addedToast) return

    const focusToast = selectToastFocus(toasts)
    if (!focusToast) return

    const nextFocus: ToastFocus = {
      id: focusToast.id,
      type: focusToast.type,
      phase: 'enter',
      duration: getToastBackdropDuration(focusToast),
      exitDuration: TOAST_BACKDROP_EXIT_DURATIONS[focusToast.type],
    }
    setFocus((current) => {
      if (!current) return nextFocus
      if (current.id === nextFocus.id && current.type === nextFocus.type) {
        return current
      }
      if (TOAST_FOCUS_PRIORITY[nextFocus.type] < TOAST_FOCUS_PRIORITY[current.type]) return current
      return nextFocus
    })
  }, [toasts])

  useEffect(() => {
    focusRef.current = focus
    if (!focus) return

    if (focus.phase === 'leave') {
      if (focusTimer.current !== null) window.clearTimeout(focusTimer.current)
      focusTimer.current = null
      focusExitTimer.current = window.setTimeout(() => {
        setFocus((current) => (current?.id === focus.id ? null : current))
        focusExitTimer.current = null
      }, focus.exitDuration)

      return () => {
        if (focusExitTimer.current !== null) window.clearTimeout(focusExitTimer.current)
        focusExitTimer.current = null
      }
    }

    if (focusExitTimer.current !== null) window.clearTimeout(focusExitTimer.current)
    focusExitTimer.current = null
    if (focusTimer.current !== null) window.clearTimeout(focusTimer.current)
    if (focus.duration <= 0) return

    focusTimer.current = window.setTimeout(() => {
      setFocus((current) => (
        current?.id === focus.id ? { ...current, phase: 'leave' } : current
      ))
      focusTimer.current = null
    }, focus.duration)

    return () => {
      if (focusTimer.current !== null) window.clearTimeout(focusTimer.current)
      focusTimer.current = null
    }
  }, [focus])

  useEffect(() => () => {
    if (focusTimer.current !== null) window.clearTimeout(focusTimer.current)
    if (focusExitTimer.current !== null) window.clearTimeout(focusExitTimer.current)
  }, [])

  const dismissFocus = useCallback((dismissedId: string) => {
    if (!focusRef.current || focusRef.current.id !== dismissedId) return
    if (focusTimer.current !== null) {
      window.clearTimeout(focusTimer.current)
      focusTimer.current = null
    }
    setFocus((current) => (
      current?.id === dismissedId ? { ...current, phase: 'leave' } : current
    ))
  }, [])

  return (
    <>
      {focus ? (
        <div
          key={`${focus.id}-${focus.type}`}
          aria-hidden="true"
          className={`toast-focus-backdrop toast-focus-${focus.type} pointer-events-none fixed inset-0 z-[9995] ${focus.phase === 'leave' ? 'toast-focus-leaving' : ''}`}
        />
      ) : null}
      <div className="pointer-events-none fixed right-3 top-3 z-[10000] flex max-h-[calc(100vh-1.5rem)] w-[min(480px,calc(100vw-1.5rem))] flex-col gap-3 overflow-y-auto overscroll-contain sm:right-4 sm:top-4 sm:w-[min(480px,calc(100vw-2rem))]">
        {[...toasts].reverse().map((t) => (
          <ToastItem key={t.id} toast={t} onManualDismiss={dismissFocus} />
        ))}
      </div>
    </>
  )
}
