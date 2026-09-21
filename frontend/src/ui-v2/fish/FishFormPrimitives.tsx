import { useEffect, useRef, useState, type ReactNode } from 'react'
import { X } from 'lucide-react'

interface FishModalProps {
  open: boolean
  title: string
  children: ReactNode
  onClose: () => void
  footer?: ReactNode
  wide?: boolean
  extraWide?: boolean
}

export function FishModal({ open, title, children, onClose, footer, wide = false, extraWide = false }: FishModalProps) {
  useEffect(() => {
    if (!open) return
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', onKeyDown)
    return () => document.removeEventListener('keydown', onKeyDown)
  }, [open, onClose])

  if (!open) return null
  return (
    <div className="fish-modal-backdrop" onMouseDown={(event) => { if (event.currentTarget === event.target) onClose() }}>
      <div className={`fish-modal${wide ? ' wide' : ''}${extraWide ? ' xwide' : ''}`}>
        <div className="fish-modal-head">
          <strong>{title}</strong><span className="fish-spacer" />
          <button className="fish-mini-icon" type="button" onClick={onClose} aria-label="关闭"><X size={16} strokeWidth={1.8} /></button>
        </div>
        <div className="fish-modal-body">{children}</div>
        {footer ? <div className="fish-modal-foot">{footer}</div> : null}
      </div>
    </div>
  )
}

interface FishConfirmProps {
  open: boolean
  title: string
  content: ReactNode
  confirmText?: string
  cancelText?: string
  danger?: boolean
  busy?: boolean
  onClose: () => void
  onConfirm: () => void
}

export function FishConfirm({ open, title, content, confirmText = '确认', cancelText = '取消', danger = false, busy = false, onClose, onConfirm }: FishConfirmProps) {
  return (
    <FishModal
      open={open}
      title={title}
      onClose={onClose}
      footer={<><button className="fish-btn" type="button" disabled={busy} onClick={onClose}>{cancelText}</button><button className={`fish-btn ${danger ? 'danger' : 'primary'}`} type="button" disabled={busy} onClick={onConfirm}>{busy ? '处理中' : confirmText}</button></>}
    >
      <div className="fish-modal-copy">{content}</div>
    </FishModal>
  )
}

interface FishSwitchProps {
  checked: boolean
  onChange: (checked: boolean) => void
  label?: string
}

export function FishSwitch({ checked, onChange, label }: FishSwitchProps) {
  return <button className={`fish-switch${checked ? ' on' : ''}`} type="button" role="switch" aria-checked={checked} aria-label={label} onClick={() => onChange(!checked)}><span /></button>
}

interface FishTagInputProps {
  value: string[]
  onChange: (tags: string[]) => void
  suggestions?: string[]
  placeholder?: string
}

export function FishTagInput({ value, onChange, suggestions = [], placeholder = '输入标签后按回车' }: FishTagInputProps) {
  const [input, setInput] = useState('')
  const [open, setOpen] = useState(false)
  const wrapRef = useRef<HTMLDivElement>(null)
  const filtered = suggestions.filter((item) => item.toLowerCase().includes(input.toLowerCase()) && !value.includes(item)).slice(0, 8)

  const addTag = (raw: string) => {
    const tag = raw.trim()
    if (!tag || value.includes(tag)) return
    onChange([...value, tag])
    setInput('')
    setOpen(false)
  }

  useEffect(() => {
    const onPointerDown = (event: MouseEvent) => {
      if (wrapRef.current && !wrapRef.current.contains(event.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', onPointerDown)
    return () => document.removeEventListener('mousedown', onPointerDown)
  }, [])

  return (
    <div className="fish-tag-input" ref={wrapRef}>
      <div className="fish-tag-control">
        {value.map((tag) => <span className="fish-tag-token" key={tag}>{tag}<button type="button" aria-label={`移除 ${tag}`} onClick={() => onChange(value.filter((item) => item !== tag))}><X size={12} strokeWidth={1.8} /></button></span>)}
        <input
          value={input}
          onChange={(event) => { setInput(event.target.value); setOpen(true) }}
          onFocus={() => setOpen(true)}
          onKeyDown={(event) => {
            if (event.key === 'Enter' || event.key === ',') { event.preventDefault(); addTag(input) }
            if (event.key === 'Backspace' && !input && value.length > 0) onChange(value.slice(0, -1))
            if (event.key === 'Escape') setOpen(false)
          }}
          placeholder={value.length === 0 ? placeholder : ''}
        />
      </div>
      {open && filtered.length > 0 ? <div className="fish-tag-menu">{filtered.map((tag) => <button type="button" key={tag} onMouseDown={(event) => event.preventDefault()} onClick={() => addTag(tag)}>{tag}</button>)}</div> : null}
    </div>
  )
}
