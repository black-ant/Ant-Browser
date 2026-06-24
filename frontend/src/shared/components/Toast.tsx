import { CheckCircle, XCircle, AlertCircle, Info, X } from 'lucide-react'
import { create } from 'zustand'

type ToastType = 'success' | 'error' | 'warning' | 'info'

interface Toast {
  id: string
  type: ToastType
  message: string
  duration?: number
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

    // 自动移除
    const duration = toast.duration ?? 3000
    if (duration > 0) {
      setTimeout(() => {
        set((state) => ({
          toasts: state.toasts.filter((t) => t.id !== id),
        }))
      }, duration)
    }
  },
  removeToast: (id) =>
    set((state) => ({
      toasts: state.toasts.filter((t) => t.id !== id),
    })),
}))

// Toast 工具函数
export const toast = {
  success: (message: string, duration?: number) =>
    useToastStore.getState().addToast({ type: 'success', message, duration }),
  error: (message: string, duration?: number) =>
    useToastStore.getState().addToast({ type: 'error', message, duration }),
  warning: (message: string, duration?: number) =>
    useToastStore.getState().addToast({ type: 'warning', message, duration }),
  info: (message: string, duration?: number) =>
    useToastStore.getState().addToast({ type: 'info', message, duration }),
}

const icons = {
  success: CheckCircle,
  error: XCircle,
  warning: AlertCircle,
  info: Info,
}

const styles = {
  success: 'bg-[var(--color-bg-elevated)] text-[var(--color-success)] border-[var(--color-success)]/20 shadow-xl',
  error: 'bg-[var(--color-bg-elevated)] text-[var(--color-error)] border-[var(--color-error)]/20 shadow-xl',
  warning: 'bg-[var(--color-bg-elevated)] text-[var(--color-warning)] border-[var(--color-warning)]/20 shadow-xl',
  info: 'bg-[var(--color-bg-elevated)] text-[var(--color-accent)] border-[var(--color-accent)]/20 shadow-xl',
}

const bgStyles = {
  success: 'bg-[var(--color-success)]/10',
  error: 'bg-[var(--color-error)]/10',
  warning: 'bg-[var(--color-warning)]/10',
  info: 'bg-[var(--color-accent)]/10',
}

function ToastItem({ toast: t }: { toast: Toast }) {
  const removeToast = useToastStore((state) => state.removeToast)
  const Icon = icons[t.type]

  return (
    <div
      className={`flex items-start gap-3 px-4 py-3.5 rounded-xl border-2 backdrop-blur-sm animate-slide-in-right ${styles[t.type]}`}
    >
      <div className={`w-8 h-8 rounded-lg flex items-center justify-center flex-shrink-0 ${bgStyles[t.type]}`}>
        <Icon className="w-4.5 h-4.5" />
      </div>
      <div className="flex-1 min-w-0">
        <p className="text-sm font-medium text-[var(--color-text-primary)]">{t.message}</p>
      </div>
      <button
        onClick={() => removeToast(t.id)}
        className="p-1.5 rounded-lg hover:bg-[var(--color-bg-muted)] transition-all duration-200 text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)]"
        aria-label="关闭"
      >
        <X className="w-4 h-4" />
      </button>
    </div>
  )
}

export function ToastContainer() {
  const toasts = useToastStore((state) => state.toasts)

  return (
    <div className="fixed top-4 right-4 z-50 flex flex-col gap-2 max-w-md">
      {toasts.map((t) => (
        <ToastItem key={t.id} toast={t} />
      ))}
    </div>
  )
}
