import { useCallback, useRef, useState, type ReactNode } from 'react'
import { ConfirmModal } from './Modal'

export interface ConfirmOptions {
  title?: string
  content: ReactNode
  confirmText?: string
  cancelText?: string
  danger?: boolean
}

// useConfirm 统一的确认弹窗：以 Promise 形式替换原生 confirm()。
// 用法：const { confirm, dialog } = useConfirm(); 在 JSX 渲染 {dialog}；
//   if (await confirm({ content: '确定删除？', danger: true })) { ... }
export function useConfirm() {
  const [open, setOpen] = useState(false)
  const [opts, setOpts] = useState<ConfirmOptions>({ content: '' })
  const resolver = useRef<((v: boolean) => void) | null>(null)

  const confirm = useCallback((o: ConfirmOptions) => {
    setOpts(o)
    setOpen(true)
    return new Promise<boolean>(resolve => {
      resolver.current = resolve
    })
  }, [])

  const settle = (value: boolean) => {
    setOpen(false)
    resolver.current?.(value)
    resolver.current = null
  }

  const dialog = (
    <ConfirmModal
      open={open}
      onClose={() => settle(false)}
      onConfirm={() => settle(true)}
      title={opts.title}
      content={opts.content}
      confirmText={opts.confirmText}
      cancelText={opts.cancelText}
      danger={opts.danger}
    />
  )

  return { confirm, dialog }
}
