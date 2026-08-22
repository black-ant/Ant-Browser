import { ReactNode, useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { X } from 'lucide-react'
import { Button } from './Button'

export const MODAL_EXIT_DURATION_MS = 420

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
  const [mounted, setMounted] = useState(open)
  const [closing, setClosing] = useState(false)

  useEffect(() => {
    if (open) {
      setMounted(true)
      setClosing(false)
      return
    }

    setClosing(true)
    const timer = window.setTimeout(() => {
      setMounted(false)
      setClosing(false)
    }, MODAL_EXIT_DURATION_MS)

    return () => window.clearTimeout(timer)
  }, [open])

  useEffect(() => {
    document.body.style.overflow = mounted ? 'hidden' : ''
    return () => {
      document.body.style.overflow = ''
    }
  }, [mounted])

  if (!mounted) return null

  return createPortal(
    <div className="fixed inset-0 z-[9990] flex items-center justify-center">
      <div
        className={`modal-backdrop absolute inset-0 ${closing ? 'animate-modal-backdrop-out' : 'animate-modal-backdrop-in'}`}
        onClick={closable ? onClose : undefined}
      />

      <div
        className={`modal-surface relative flex max-h-[90vh] w-full flex-col rounded-xl bg-[var(--color-bg-elevated)] shadow-2xl ${closing ? 'animate-modal-out' : 'animate-modal-in'}`}
        style={{ width, maxWidth: '90vw' }}
        onClick={(e) => e.stopPropagation()}
      >
        {(title || closable) && (
          <div className="flex items-center justify-between px-6 py-4 border-b border-[var(--color-border)] flex-shrink-0">
            {title && (
              <h3 className="text-lg font-semibold text-[var(--color-text-primary)]">
                {title}
              </h3>
            )}
            {closable && (
              <button
                onClick={onClose}
                className="p-1.5 rounded-lg text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)] hover:bg-[var(--color-bg-muted)] transition-colors ml-auto"
              >
                <X className="w-5 h-5" />
              </button>
            )}
          </div>
        )}

        <div className="px-6 py-4 overflow-y-auto flex-1 min-h-0">
          {children}
        </div>

        {footer && (
          <div className="flex items-center justify-end gap-3 px-6 py-4 border-t border-[var(--color-border)] flex-shrink-0">
            {footer}
          </div>
        )}
      </div>
    </div>,
    document.body,
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
  const [confirming, setConfirming] = useState(false)
  const confirmTimerRef = useRef<number | null>(null)

  useEffect(() => () => {
    if (confirmTimerRef.current !== null) window.clearTimeout(confirmTimerRef.current)
  }, [])

  useEffect(() => {
    if (open) setConfirming(false)
  }, [open])

  const handleConfirm = () => {
    if (confirming) return

    setConfirming(true)
    onClose()
    confirmTimerRef.current = window.setTimeout(() => {
      confirmTimerRef.current = null
      onConfirm()
    }, MODAL_EXIT_DURATION_MS)
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={title}
      width="400px"
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={confirming}>
            {cancelText}
          </Button>
          <Button
            variant={danger ? 'danger' : 'primary'}
            onClick={handleConfirm}
            disabled={confirming}
          >
            {confirmText}
          </Button>
        </>
      }
    >
      <div className="text-[var(--color-text-secondary)]">{content}</div>
    </Modal>
  )
}
