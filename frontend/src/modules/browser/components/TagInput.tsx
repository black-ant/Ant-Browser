import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { X } from 'lucide-react'

interface TagInputProps {
  value: string[]
  onChange: (tags: string[]) => void
  suggestions?: string[]
  placeholder?: string
}

interface DropdownPosition {
  left: number
  top: number
  width: number
}

export function TagInput({ value, onChange, suggestions = [], placeholder = '输入标签后按回车' }: TagInputProps) {
  const [input, setInput] = useState('')
  const [showSuggestions, setShowSuggestions] = useState(false)
  const [position, setPosition] = useState<DropdownPosition | null>(null)
  const wrapRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)

  const filtered = suggestions.filter(
    s => s.toLowerCase().includes(input.toLowerCase()) && !value.includes(s)
  )
  const open = showSuggestions && filtered.length > 0

  const updatePosition = useCallback(() => {
    const el = wrapRef.current
    if (!el) return
    const rect = el.getBoundingClientRect()
    setPosition({
      left: rect.left,
      top: rect.bottom + 4,
      width: rect.width,
    })
  }, [])

  const addTag = (tag: string) => {
    const t = tag.trim()
    if (!t || value.includes(t)) return
    onChange([...value, t])
    setInput('')
    setShowSuggestions(false)
  }

  const removeTag = (tag: string) => {
    onChange(value.filter(t => t !== tag))
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter' || e.key === ',') {
      e.preventDefault()
      addTag(input)
    } else if (e.key === 'Backspace' && !input && value.length > 0) {
      removeTag(value[value.length - 1])
    } else if (e.key === 'Escape') {
      setShowSuggestions(false)
    }
  }

  useLayoutEffect(() => {
    if (!open) {
      setPosition(null)
      return
    }
    updatePosition()
  }, [open, filtered.length, value.length, updatePosition])

  useEffect(() => {
    if (!open) return

    const handleReposition = () => updatePosition()
    window.addEventListener('resize', handleReposition)
    window.addEventListener('scroll', handleReposition, true)
    return () => {
      window.removeEventListener('resize', handleReposition)
      window.removeEventListener('scroll', handleReposition, true)
    }
  }, [open, updatePosition])

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      const target = e.target as Node
      if (wrapRef.current?.contains(target)) return
      if ((target as Element).closest?.('[data-tag-input-dropdown]')) return
      setShowSuggestions(false)
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [])

  return (
    <div ref={wrapRef} className="tag-input-wrap relative">
      <div
        className="min-h-9 flex flex-wrap gap-1.5 items-center px-3 py-1.5 rounded-md border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] cursor-text focus-within:border-[var(--color-accent)] transition-colors"
        onClick={() => inputRef.current?.focus()}
      >
        {value.map(tag => (
          <span
            key={tag}
            className="inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-xs font-medium bg-[var(--color-accent-muted)] text-[var(--color-accent)]"
          >
            {tag}
            <button
              type="button"
              onClick={e => { e.stopPropagation(); removeTag(tag) }}
              className="hover:text-[var(--color-error)] transition-colors"
            >
              <X className="w-3 h-3" />
            </button>
          </span>
        ))}
        <input
          ref={inputRef}
          value={input}
          onChange={e => { setInput(e.target.value); setShowSuggestions(true) }}
          onKeyDown={handleKeyDown}
          onFocus={() => setShowSuggestions(true)}
          placeholder={value.length === 0 ? placeholder : ''}
          className="flex-1 min-w-24 bg-transparent text-sm text-[var(--color-text-primary)] placeholder:text-[var(--color-text-muted)] outline-none"
        />
      </div>

      {open && position && createPortal(
        <div
          data-tag-input-dropdown
          className="fixed z-[9999] max-h-56 overflow-auto rounded-md border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] shadow-lg"
          style={{ left: position.left, top: position.top, width: position.width }}
        >
          {filtered.slice(0, 8).map(s => (
            <button
              key={s}
              type="button"
              onMouseDown={e => { e.preventDefault(); addTag(s) }}
              className="w-full text-left px-3 py-2 text-sm text-[var(--color-text-primary)] hover:bg-[var(--color-bg-muted)] transition-colors"
            >
              {s}
            </button>
          ))}
        </div>,
        document.body,
      )}
    </div>
  )
}
