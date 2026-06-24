import { ReactNode, useEffect } from 'react'
import { X } from 'lucide-react'
import { Button } from './Button'

interface ModalProps {
  open: boolean
  onClose: () => void
  title?: string
  children: ReactNode
  footer?: ReactNode
  width?: string
  closable?: boolean
}

export function Modal({
  open,
  onClose,
  title,
  children,
  footer,
  width = '500px',
  closable = true,
}: ModalProps) {
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

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      {/* 遮罩层 - 增强模糊效果 */}
      <div
        className="absolute inset-0 bg-black/40 backdrop-blur-sm animate-fade-in"
        onClick={closable ? onClose : undefined}
      />

      {/* 弹窗内容 - 增强阴影和圆角 */}
      <div
        className="relative bg-[var(--color-bg-elevated)] rounded-2xl shadow-2xl animate-scale-in max-h-[90vh] w-full flex flex-col overflow-hidden"
        style={{ width, maxWidth: '90vw' }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* 标题栏 - 添加渐变背景 */}
        {(title || closable) && (
          <div className="flex items-center justify-between px-6 py-4 border-b border-[var(--color-border-default)] bg-gradient-to-r from-[var(--color-bg-elevated)] to-[var(--color-bg-surface)] flex-shrink-0">
            {title && (
              <h3 className="text-lg font-semibold text-[var(--color-text-primary)]">
                {title}
              </h3>
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

        {/* 内容区 */}
        <div className="px-6 py-5 overflow-y-auto flex-1 min-h-0">
          {children}
        </div>

        {/* 底部按钮 */}
        {footer && (
          <div className="flex items-center justify-end gap-3 px-6 py-4 border-t border-[var(--color-border-default)] bg-[var(--color-bg-surface)] flex-shrink-0">
            {footer}
          </div>
        )}
      </div>
    </div>
  )
}

// 确认对话框
interface ConfirmModalProps {
  open: boolean
  onClose: () => void
  onConfirm: () => void
  title?: string
  content: ReactNode
  confirmText?: string
  cancelText?: string
  danger?: boolean
}

export function ConfirmModal({
  open,
  onClose,
  onConfirm,
  title = '确认',
  content,
  confirmText = '确定',
  cancelText = '取消',
  danger = false,
}: ConfirmModalProps) {
  return (
    <Modal
      open={open}
      onClose={onClose}
      title={title}
      width="420px"
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            {cancelText}
          </Button>
          <Button
            variant={danger ? 'danger' : 'primary'}
            onClick={() => {
              onConfirm()
              onClose()
            }}
          >
            {confirmText}
          </Button>
        </>
      }
    >
      <div className="text-[var(--color-text-secondary)] text-sm leading-relaxed">
        {content}
      </div>
    </Modal>
  )
}
