import { ReactNode, useState } from 'react'
import { ChevronDown } from 'lucide-react'
import clsx from 'clsx'

interface AccordionItemProps {
  title: string
  children: ReactNode
  defaultOpen?: boolean
  className?: string
}

export function AccordionItem({ title, children, defaultOpen = false, className }: AccordionItemProps) {
  const [isOpen, setIsOpen] = useState(defaultOpen)

  return (
    <div className={clsx('border border-[var(--color-border-default)] rounded-lg overflow-hidden', className)}>
      <button
        type="button"
        onClick={() => setIsOpen(!isOpen)}
        className="w-full flex items-center justify-between px-3 py-2 bg-[var(--color-bg-elevated)] hover:bg-[var(--color-bg-muted)] transition-colors text-left"
      >
        <span className="text-sm font-medium text-[var(--color-text-primary)]">{title}</span>
        <ChevronDown
          className={clsx(
            'w-4 h-4 text-[var(--color-text-muted)] transition-transform',
            isOpen && 'rotate-180'
          )}
        />
      </button>
      {isOpen && (
        <div className="px-3 py-3 bg-[var(--color-bg-base)] border-t border-[var(--color-border-default)]">
          {children}
        </div>
      )}
    </div>
  )
}
