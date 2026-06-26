import { ReactNode, useEffect } from 'react'
import { createPortal } from 'react-dom'
import { X } from 'lucide-react'

interface SideDrawerProps {
  open: boolean
  onClose: () => void
  title?: string
  children: ReactNode
  footer?: ReactNode
  width?: string
  closable?: boolean
}

// 从右侧滑入的抽屉容器（portal + 遮罩 + 头/体/底）。
export function SideDrawer({
  open,
  onClose,
  title,
  children,
  footer,
  width = '760px',
  closable = true,
}: SideDrawerProps) {
  useEffect(() => {
    if (open) {
      document.body.style.overflow = 'hidden'
    } else {
      document.body.style.overflow = ''
    }
    return () => {
      document.body.style.overflow = ''
    }
  }, [open])

  if (!open) return null

  return createPortal(
    <div className="fixed inset-0 z-50 flex justify-end">
      {/* 遮罩 */}
      <div
        className="absolute inset-0 bg-black/40 backdrop-blur-sm animate-fade-in"
        onClick={closable ? onClose : undefined}
      />

      {/* 抽屉主体 */}
      <div
        className="relative h-full bg-[var(--color-bg-elevated)] shadow-2xl flex flex-col animate-slide-in-right"
        style={{ width, maxWidth: '92vw' }}
        onClick={(e) => e.stopPropagation()}
      >
        {(title || closable) && (
          <div className="flex items-center justify-between px-6 py-4 border-b border-[var(--color-border-default)] flex-shrink-0">
            {title && (
              <h3 className="text-lg font-semibold text-[var(--color-text-primary)]">{title}</h3>
            )}
            {closable && (
              <button
                onClick={onClose}
                className="p-2 rounded-xl text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)] hover:bg-[var(--color-bg-muted)] transition-all duration-200 ml-auto"
                aria-label="关闭"
              >
                <X className="w-5 h-5" />
              </button>
            )}
          </div>
        )}

        <div className="px-6 py-5 overflow-y-auto flex-1 min-h-0">{children}</div>

        {footer && (
          <div className="flex items-center justify-end gap-3 px-6 py-4 border-t border-[var(--color-border-default)] bg-[var(--color-bg-surface)] flex-shrink-0">
            {footer}
          </div>
        )}
      </div>
    </div>,
    document.body,
  )
}
