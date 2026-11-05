import { useEffect, useId, useMemo, useRef, useState, type KeyboardEvent } from 'react'
import { Check, ChevronDown, ChevronRight } from 'lucide-react'

export interface BrowserEditSelectOption {
  value: string
  label: string
  disabled?: boolean
  /** 只读子项：默认折叠，展开后仅作展示，不可选中（如虚拟分组成员） */
  children?: BrowserEditSelectOption[]
}

interface BrowserEditSelectProps {
  label: string
  value: string
  options: BrowserEditSelectOption[]
  onChange: (value: string) => void
  className?: string
  disabled?: boolean
}

interface FlatOption {
  option: BrowserEditSelectOption
  depth: number
  parentValue?: string
}

function flattenOptions(options: BrowserEditSelectOption[], expanded: Set<string>): FlatOption[] {
  const flat: FlatOption[] = []
  options.forEach(option => {
    flat.push({ option, depth: 0 })
    if (option.children?.length && expanded.has(option.value)) {
      option.children.forEach(child => {
        flat.push({ option: child, depth: 1, parentValue: option.value })
      })
    }
  })
  return flat
}

function findEnabledIndex(entries: FlatOption[], start: number, direction: 1 | -1) {
  if (entries.length === 0) return -1
  const normalizedStart = ((start % entries.length) + entries.length) % entries.length
  for (let offset = 0; offset < entries.length; offset += 1) {
    const index = (normalizedStart + offset * direction + entries.length * 2) % entries.length
    if (!entries[index]?.option.disabled) return index
  }
  return -1
}

export function BrowserEditSelect({ label, value, options, onChange, className = '', disabled = false }: BrowserEditSelectProps) {
  const id = `browser-edit-select-${useId()}`
  const [open, setOpen] = useState(false)
  const [expanded, setExpanded] = useState<Set<string>>(() => new Set())
  const entries = useMemo(() => flattenOptions(options, expanded), [options, expanded])
  const selectedIndex = Math.max(0, entries.findIndex(entry => entry.option.value === value))
  const [active, setActive] = useState(() => findEnabledIndex(entries, selectedIndex, 1))
  const root = useRef<HTMLDivElement>(null)
  const trigger = useRef<HTMLButtonElement>(null)
  const listRef = useRef<HTMLDivElement>(null)
  const selectedLabel = options.find(option => option.value === value)?.label || ''

  useEffect(() => {
    if (!open) setActive(findEnabledIndex(entries, selectedIndex, 1))
  }, [open, entries, selectedIndex])

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

  const toggleExpand = (optionValue: string) => {
    setExpanded(current => {
      const next = new Set(current)
      if (next.has(optionValue)) next.delete(optionValue)
      else next.add(optionValue)
      return next
    })
  }

  const choose = (index: number) => {
    const entry = entries[index]
    if (!entry || entry.option.disabled || disabled) return
    setActive(index)
    onChange(entry.option.value)
    setOpen(false)
    trigger.current?.focus()
  }

  const onKeyDown = (event: KeyboardEvent<HTMLButtonElement>) => {
    if (disabled || entries.length === 0) return
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault()
      const delta = event.key === 'ArrowDown' ? 1 : -1
      setActive(open
        ? findEnabledIndex(entries, active + delta, delta)
        : findEnabledIndex(entries, selectedIndex, 1))
      setOpen(true)
    } else if (event.key === 'Home' || event.key === 'End') {
      event.preventDefault()
      setActive(event.key === 'Home'
        ? findEnabledIndex(entries, 0, 1)
        : findEnabledIndex(entries, entries.length - 1, -1))
      setOpen(true)
    } else if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault()
      if (open) choose(active)
      else {
        setActive(selectedIndex)
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
        onClick={() => { setActive(findEnabledIndex(entries, selectedIndex, 1)); setOpen(current => !current) }}
        onKeyDown={onKeyDown}
        className="flex h-9 w-full items-center justify-between rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-3 text-left text-sm text-[var(--color-text-primary)] hover:border-[var(--color-border-strong)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-border-strong)] disabled:cursor-not-allowed disabled:opacity-60"
      >
        <span className="truncate">{selectedLabel}</span>
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
        {entries.map((entry, index) => {
          const { option, depth } = entry
          const hasChildren = Boolean(option.children?.length)
          const isExpanded = hasChildren && expanded.has(option.value)
          const isSelected = option.value === value
          return (
            <div key={`${entry.parentValue || 'root'}-${option.value}-${index}`} className="flex w-full items-center">
              <button
                id={`${id}-${index}`}
                type="button"
                role="option"
                aria-selected={isSelected}
                aria-disabled={option.disabled || undefined}
                disabled={option.disabled}
                tabIndex={-1}
                onMouseEnter={() => { if (!option.disabled) setActive(index) }}
                onMouseDown={event => event.preventDefault()}
                onClick={() => choose(index)}
                className={`flex min-w-0 flex-1 items-center justify-between px-3 py-2 text-left text-sm ${option.disabled ? 'cursor-not-allowed opacity-60' : active === index ? 'bg-[var(--color-bg-muted)]' : 'bg-transparent hover:bg-[var(--color-bg-muted)]'} ${isSelected ? 'font-medium' : ''} ${depth > 0 ? 'pl-8 text-xs' : ''}`}
              >
                <span className={`truncate ${depth > 0 ? 'text-[var(--color-text-muted)]' : 'text-[var(--color-text-primary)]'}`}>{option.label}</span>
                {isSelected && <Check aria-hidden="true" className="ml-2 h-4 w-4 shrink-0 text-[var(--color-accent)]" />}
              </button>
              {hasChildren && (
                <button
                  type="button"
                  aria-label={isExpanded ? '收起成员' : '展开成员'}
                  title={isExpanded ? '收起成员' : '展开成员'}
                  onMouseDown={event => event.preventDefault()}
                  onClick={event => { event.stopPropagation(); event.preventDefault(); toggleExpand(option.value) }}
                  className="mr-2 shrink-0 rounded p-1 text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-muted)]"
                >
                  <ChevronRight aria-hidden="true" className={`h-3.5 w-3.5 transition-transform ${isExpanded ? 'rotate-90' : ''}`} />
                </button>
              )}
            </div>
          )
        })}
      </div>
    </div>
  )
}