import { useState } from 'react'
import { Check, ChevronDown, Copy } from 'lucide-react'
import clsx from 'clsx'
import { getNotificationMessageView, type NotificationMessageContext } from './message'
interface NotificationMessageProps {
  message: string
  context?: NotificationMessageContext
  className?: string
  compact?: boolean
}

export function NotificationMessage({ message, context = 'generic', className, compact = false }: NotificationMessageProps) {
  const view = getNotificationMessageView(message, context)
  const [copied, setCopied] = useState(false)

  const copyDetails = async () => {
    if (!view.copyText || !navigator.clipboard?.writeText) return
    try {
      await navigator.clipboard.writeText(view.copyText)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 1600)
    } catch {
      setCopied(false)
    }
  }
  return (
    <div className={clsx('min-w-0', className)}>
      <p className={clsx('whitespace-pre-wrap break-words', compact ? 'text-xs leading-5' : 'text-[13px] leading-5')}>
        {view.summary}
      </p>
      {view.details && (
        <details className='mt-2 min-w-0'>
          <summary className='inline-flex cursor-pointer list-none items-center gap-1 rounded-md px-1 py-0.5 text-[11px] font-medium text-[var(--color-text-muted)] hover:bg-[var(--color-bg-muted)] hover:text-[var(--color-text-primary)]'>
            <ChevronDown className='h-3.5 w-3.5' />
            查看技术详情
          </summary>
          <div className='mt-2 overflow-hidden rounded-lg border border-[var(--color-border-muted)] bg-[var(--color-bg-muted)]'>
            <div className='max-h-56 overflow-auto whitespace-pre-wrap break-words px-3 py-2 font-mono text-[11px] leading-5 text-[var(--color-text-secondary)]'>
              {view.details}
            </div>
            <div className='flex justify-end border-t border-[var(--color-border-muted)] px-2 py-1.5'>
              <button type='button' onClick={() => { void copyDetails() }} className='inline-flex cursor-pointer items-center gap-1 rounded-md px-2 py-1 text-[11px] font-medium text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-surface)] hover:text-[var(--color-text-primary)]'>
                {copied ? <Check className='h-3.5 w-3.5 text-[var(--color-success)]' /> : <Copy className='h-3.5 w-3.5' />}
                {copied ? '已复制' : '复制详情'}
              </button>
            </div>
          </div>
        </details>
      )}
    </div>
  )
}
