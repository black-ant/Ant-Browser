import { useEffect, useId, useRef, useState, type KeyboardEvent } from 'react'
import { Check, ChevronDown } from 'lucide-react'

export interface BrowserEditSelectOption {
  value: string
  label: string
  disabled?: boolean
}

interface BrowserEditSelectProps {
  label: string
  value: string
  options: BrowserEditSelectOption[]
  onChange: (value: string) => void
  className?: string
  disabled?: boolean
}

function findEnabledIndex(options: BrowserEditSelectOption[], start: number, direction: 1 | -1) {
  if (options.length === 0) return -1
  const normalizedStart = ((start % options.length) + options.length) % options.length
  for (let offset = 0; offset < options.length; offset += 1) {
    const index = (normalizedStart + offset * direction + options.length * 2) % options.length
    if (!options[index]?.disabled) return index
  }
  return -1
}

export function BrowserEditSelect({ label, value, options, onChange, className = '', disabled = false }: BrowserEditSelectProps) {
  const id = `browser-edit-select-${useId()}`
  const selected = Math.max(0, options.findIndex(option => option.value === value))
  const initialActive = findEnabledIndex(options, selected, 1)
  const [open, setOpen] = useState(false)
  const [active, setActive] = useState(initialActive)
  const root = useRef<HTMLDivElement>(null)
  const trigger = useRef<HTMLButtonElement>(null)
  const listRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) setActive(findEnabledIndex(options, selected, 1))
  }, [open, options, selected])

  useEffect(() => {
    if (!open || active < 0) return
    const item = listRef.current?.children[active]
    if (item instanceof HTMLElement) item.scrollIntoView({ block: 'nearest' })
  }, [open, active])

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
    if (!option || option.disabled || disabled) return
    setActive(index)
    onChange(option.value)
    setOpen(false)
    trigger.current?.focus()
  }

  const onKeyDown = (event: KeyboardEvent<HTMLButtonElement>) => {
    if (disabled || options.length === 0) return
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault()
      const delta = event.key === 'ArrowDown' ? 1 : -1
      setActive(open
        ? findEnabledIndex(options, active + delta, delta)
        : findEnabledIndex(options, selected, 1))
      setOpen(true)
    } else if (event.key === 'Home' || event.key === 'End') {
      event.preventDefault()
      setActive(event.key === 'Home'
        ? findEnabledIndex(options, 0, 1)
        : findEnabledIndex(options, options.length - 1, -1))
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
    <div
      ref={root}
      className={`relative w-full ${className}`}
      onBlur={event => {
        const target = event.relatedTarget
        if (!(target instanceof Node) || !event.currentTarget.contains(target)) setOpen(false)
      }}
    >
      <button
        ref={trigger}
        type="button"
        role="combobox"
        aria-label={label}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={id}
        aria-activedescendant={open && active >= 0 ? `${id}-${active}` : undefined}
        disabled={disabled}
        onClick={() => { setActive(findEnabledIndex(options, selected, 1)); setOpen(current => !current) }}
        onKeyDown={onKeyDown}
        className="flex h-9 w-full items-center justify-between rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-3 text-left text-sm text-[var(--color-text-primary)] hover:border-[var(--color-border-strong)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-border-strong)] disabled:cursor-not-allowed disabled:opacity-60"
      >
        <span className="truncate">{options[selected]?.label || ''}</span>
        <ChevronDown aria-hidden="true" className={`ml-2 h-4 w-4 shrink-0 text-[var(--color-text-secondary)] transition-transform ${open ? 'rotate-180' : ''}`} />
      </button>
      <div
        id={id}
        ref={listRef}
        role="listbox"
        aria-label={label}
        hidden={!open}
        className="absolute left-0 right-0 top-[calc(100%+0.25rem)] z-50 max-h-64 overflow-y-auto rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-elevated)] py-1 shadow-lg"
      >
        {options.map((option, index) => (
          <button
            key={option.value}
            id={`${id}-${index}`}
            type="button"
            role="option"
            aria-selected={option.value === value}
            aria-disabled={option.disabled || undefined}
            disabled={option.disabled}
            tabIndex={-1}
            onMouseEnter={() => { if (!option.disabled) setActive(index) }}
            onMouseDown={event => event.preventDefault()}
            onClick={() => choose(index)}
            className={`flex w-full items-center justify-between px-3 py-2 text-left text-sm text-[var(--color-text-primary)] ${option.disabled ? 'cursor-not-allowed opacity-50' : active === index ? 'bg-[var(--color-bg-muted)]' : 'bg-transparent hover:bg-[var(--color-bg-muted)]'} ${option.value === value ? 'font-medium' : ''}`}
          >
            <span className="truncate">{option.label}</span>
            {option.value === value && <Check aria-hidden="true" className="ml-2 h-4 w-4 shrink-0 text-[var(--color-accent)]" />}
          </button>
        ))}
      </div>
    </div>
  )
}
