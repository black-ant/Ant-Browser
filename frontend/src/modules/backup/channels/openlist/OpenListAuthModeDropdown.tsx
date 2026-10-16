import { useEffect, useId, useRef, useState, type KeyboardEvent } from 'react'
import { Check, ChevronDown } from 'lucide-react'
import type { OpenListAuthMode } from './api'

const options: { value: OpenListAuthMode; label: string }[] = [
  { value: 'basic', label: 'WebDAV 用户名/密码' },
  { value: 'token', label: 'OpenList 服务端全局 Token' },
]

interface Props {
  value: OpenListAuthMode
  onChange: (value: OpenListAuthMode) => void
  disabled?: boolean
}

export function OpenListAuthModeDropdown({ value, onChange, disabled = false }: Props) {
  const id = `openlist-auth-${useId()}`
  const selected = Math.max(0, options.findIndex(option => option.value === value))
  const [open, setOpen] = useState(false)
  const [active, setActive] = useState(selected)
  const root = useRef<HTMLDivElement>(null)
  const trigger = useRef<HTMLButtonElement>(null)

  useEffect(() => {
    if (!open) setActive(selected)
  }, [open, selected])

  useEffect(() => {
    if (!open) return
    const closeOutside = (event: PointerEvent) => {
      if (event.target instanceof Node && !root.current?.contains(event.target)) setOpen(false)
    }
    document.addEventListener('pointerdown', closeOutside)
    return () => document.removeEventListener('pointerdown', closeOutside)
  }, [open])

  const choose = (index: number) => {
    const option = options[index]
    if (!option) return
    setActive(index)
    onChange(option.value)
    setOpen(false)
    trigger.current?.focus()
  }

  const onKeyDown = (event: KeyboardEvent<HTMLButtonElement>) => {
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault()
      const delta = event.key === 'ArrowDown' ? 1 : -1
      setActive(open ? index => (index + delta + options.length) % options.length : selected)
      setOpen(true)
    } else if (event.key === 'Home' || event.key === 'End') {
      event.preventDefault()
      setActive(event.key === 'Home' ? 0 : options.length - 1)
      setOpen(true)
    } else if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault()
      if (open) choose(active)
      else {
        setActive(selected)
        setOpen(true)
      }
    } else if (event.key === 'Escape' && open) {
      event.preventDefault()
      setOpen(false)
      trigger.current?.focus()
    } else if (event.key === 'Tab' && open) {
      setOpen(false)
    }
  }

  return (
    <div ref={root} className='relative w-full' onBlur={event => {
      const target = event.relatedTarget
      if (!(target instanceof Node) || !event.currentTarget.contains(target)) setOpen(false)
    }}>
      <button ref={trigger} type='button' role='combobox' aria-label='认证方式' aria-haspopup='listbox' aria-expanded={open} aria-controls={id} aria-activedescendant={open ? `${id}-${active}` : undefined} disabled={disabled} onClick={() => { setActive(selected); setOpen(current => !current) }} onKeyDown={onKeyDown} className='flex h-9 w-full items-center justify-between rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-3 text-left text-sm text-[var(--color-text-primary)] hover:border-[var(--color-border-strong)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-border-strong)] disabled:cursor-not-allowed disabled:opacity-60'>
        {options[selected].label}
        <ChevronDown aria-hidden='true' className={`h-4 w-4 text-[var(--color-text-secondary)] transition-transform ${open ? 'rotate-180' : ''}`} />
      </button>
      <div id={id} role='listbox' aria-label='认证方式' hidden={!open} className='absolute left-0 right-0 top-[calc(100%+0.25rem)] z-50 overflow-hidden rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-elevated)] py-1 shadow-lg'>
        {options.map((option, index) => <button key={option.value} id={`${id}-${index}`} type='button' role='option' aria-selected={option.value === value} tabIndex={-1} onMouseEnter={() => setActive(index)} onMouseDown={event => event.preventDefault()} onClick={() => choose(index)} className={`flex w-full items-center justify-between px-3 py-2 text-left text-sm text-[var(--color-text-primary)] ${active === index ? 'bg-[var(--color-bg-muted)]' : 'bg-transparent hover:bg-[var(--color-bg-muted)]'} ${option.value === value ? 'font-medium' : ''}`}>
          {option.label}
          {option.value === value && <Check aria-hidden='true' className='h-4 w-4 text-[var(--color-accent)]' />}
        </button>)}
      </div>
    </div>
  )
}
