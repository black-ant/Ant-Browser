import { useEffect, useId, useRef, useState, type KeyboardEvent } from 'react'
import { Check, ChevronDown } from 'lucide-react'

export interface ProxyNodeOption {
  value: string
  label: string
  latencyMs?: number
}

interface ProxyNodeSelectProps {
  value: string
  options: ProxyNodeOption[]
  onChange: (value: string) => void
  label?: string
  className?: string
  disabled?: boolean
  placeholder?: string
}

// 延迟按区间着色：<100ms 绿、<200ms 黄、其余红；未知延迟用弱化色。
function latencyTone(latencyMs?: number): string {
  if (!latencyMs || latencyMs <= 0) return 'text-[var(--color-text-muted)]'
  if (latencyMs < 100) return 'text-[var(--color-success)]'
  if (latencyMs < 200) return 'text-[var(--color-warning)]'
  return 'text-[var(--color-error)]'
}

function clampStart(index: number) {
  return index < 0 ? 0 : index
}

// 节点下拉：原生 select 无法把延迟右对齐并着色，这里用 listbox 自绘。
export function ProxyNodeSelect({
  value,
  options,
  onChange,
  label = '代理节点',
  className = '',
  disabled = false,
  placeholder = '请选择节点',
}: ProxyNodeSelectProps) {
  const id = `proxy-node-select-${useId()}`
  const [open, setOpen] = useState(false)
  const [active, setActive] = useState(() => clampStart(options.findIndex(item => item.value === value)))
  const root = useRef<HTMLDivElement>(null)
  const trigger = useRef<HTMLButtonElement>(null)
  const listRef = useRef<HTMLDivElement>(null)

  const selected = options.find(item => item.value === value)
  const selectedIndex = options.findIndex(item => item.value === value)

  useEffect(() => {
    if (!open) setActive(clampStart(selectedIndex))
  }, [open, selectedIndex, options])

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
    if (!option || disabled) return
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
      const base = open ? active : clampStart(selectedIndex)
      const next = (base + delta + options.length) % options.length
      setActive(next)
      setOpen(true)
    } else if (event.key === 'Home' || event.key === 'End') {
      event.preventDefault()
      setActive(event.key === 'Home' ? 0 : options.length - 1)
      setOpen(true)
    } else if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault()
      if (open) choose(active)
      else {
        setActive(clampStart(selectedIndex))
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
        onClick={() => {
          setActive(clampStart(selectedIndex))
          setOpen(current => !current)
        }}
        onKeyDown={onKeyDown}
        className="flex h-9 w-full items-center justify-between gap-2 rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-3 text-left text-sm text-[var(--color-text-primary)] shadow-[var(--shadow-xs)] transition-colors duration-150 hover:border-[var(--color-border-strong)] focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-[var(--color-border-strong)] disabled:cursor-not-allowed disabled:bg-[var(--color-bg-muted)] disabled:text-[var(--color-text-muted)]"
      >
        <span className="truncate">{selected?.label ?? placeholder}</span>
        <span className="flex shrink-0 items-center gap-2">
          {selected?.latencyMs ? (
            <span className={`text-xs tabular-nums ${latencyTone(selected.latencyMs)}`}>{selected.latencyMs}ms</span>
          ) : null}
          <ChevronDown
            aria-hidden="true"
            className={`h-4 w-4 text-[var(--color-text-secondary)] transition-transform ${open ? 'rotate-180' : ''}`}
          />
        </span>
      </button>

      <div
        id={id}
        ref={listRef}
        role="listbox"
        aria-label={label}
        hidden={!open}
        className="absolute left-0 right-0 top-[calc(100%+0.25rem)] z-50 max-h-64 overflow-y-auto rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-elevated)] py-1 shadow-lg"
      >
        {options.map((option, index) => {
          const isSelected = option.value === value
          return (
            <button
              key={option.value}
              id={`${id}-${index}`}
              type="button"
              role="option"
              aria-selected={isSelected}
              tabIndex={-1}
              onMouseEnter={() => setActive(index)}
              onMouseDown={event => event.preventDefault()}
              onClick={() => choose(index)}
              className={`flex w-full items-center gap-2 px-3 py-2 text-left text-sm ${active === index ? 'bg-[var(--color-bg-muted)]' : 'bg-transparent hover:bg-[var(--color-bg-muted)]'} ${isSelected ? 'font-medium' : ''}`}
            >
              <span className="min-w-0 flex-1 truncate text-[var(--color-text-primary)]">{option.label}</span>
              <span className={`shrink-0 text-xs tabular-nums ${latencyTone(option.latencyMs)}`}>
                {option.latencyMs ? `${option.latencyMs}ms` : '—'}
              </span>
              {isSelected ? (
                <Check aria-hidden="true" className="h-4 w-4 shrink-0 text-[var(--color-accent)]" />
              ) : (
                <span aria-hidden="true" className="h-4 w-4 shrink-0" />
              )}
            </button>
          )
        })}
      </div>
    </div>
  )
}
