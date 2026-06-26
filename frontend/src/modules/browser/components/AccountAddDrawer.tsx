import { useEffect, useRef, useState } from 'react'
import { Eye, EyeOff } from 'lucide-react'
import { Button, SideDrawer, toast } from '../../../shared/components'
import { createAccount, type BrowserAccount, type BrowserAccountInput } from '../api'
import { PLATFORM_PRESETS } from '../config/platformPresets'

interface AccountAddDrawerProps {
  open: boolean
  onClose: () => void
  onCreated: (account: BrowserAccount, autoSelect: boolean) => void
}

interface AddForm {
  platform: string
  username: string
  password: string
  twoFA: string
}

const EMPTY_FORM: AddForm = {
  platform: '',
  username: '',
  password: '',
  twoFA: '',
}

// 添加平台账号抽屉：从右侧滑出，智能识别 + 平台/账号/密码/2FA。
export function AccountAddDrawer({ open, onClose, onCreated }: AccountAddDrawerProps) {
  const [form, setForm] = useState<AddForm>(EMPTY_FORM)
  const [smartText, setSmartText] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [platformOpen, setPlatformOpen] = useState(false)
  const [saving, setSaving] = useState(false)
  const platformRef = useRef<HTMLDivElement | null>(null)

  useEffect(() => {
    if (open) {
      setForm(EMPTY_FORM)
      setSmartText('')
      setShowPassword(false)
      setPlatformOpen(false)
    }
  }, [open])

  useEffect(() => {
    const onDocClick = (e: MouseEvent) => {
      if (platformRef.current && !platformRef.current.contains(e.target as Node)) {
        setPlatformOpen(false)
      }
    }
    document.addEventListener('mousedown', onDocClick)
    return () => document.removeEventListener('mousedown', onDocClick)
  }, [])

  const update = (patch: Partial<AddForm>) => setForm(prev => ({ ...prev, ...patch }))

  // 智能识别：账号:密码:2FA（支持 : ： | 空格 制表符 分隔）
  const handleSmartChange = (text: string) => {
    setSmartText(text)
    const parts = text.trim().split(/[:：|\t ]+/).map(s => s.trim()).filter(Boolean)
    if (parts.length === 0) return
    setForm(prev => ({
      ...prev,
      username: parts[0] ?? prev.username,
      password: parts[1] ?? prev.password,
      twoFA: parts[2] ?? prev.twoFA,
    }))
  }

  const submit = async (autoSelect: boolean) => {
    if (!form.username.trim()) {
      toast.error('请输入账号')
      return
    }
    setSaving(true)
    try {
      const preset = PLATFORM_PRESETS.find(p => p.url === form.platform || p.value === form.platform)
      const input: BrowserAccountInput = {
        accountName: form.username.trim(),
        platform: preset?.value || form.platform.trim() || 'other',
        username: form.username.trim(),
        email: '',
        password: form.password,
        twoFA: form.twoFA,
        relatedProfileIds: [],
        notes: '',
        cookies: '',
      }
      const created = await createAccount(input)
      if (created) {
        toast.success('账号已添加')
        onCreated(created, autoSelect)
        if (autoSelect) {
          onClose()
        } else {
          // 保存 & 继续添加：重置表单
          setForm(EMPTY_FORM)
          setSmartText('')
          setShowPassword(false)
        }
      }
    } catch (error: any) {
      toast.error(error?.message || '保存失败')
    } finally {
      setSaving(false)
    }
  }

  const labelClass = 'mb-1.5 block text-sm font-medium text-[var(--color-text-primary)]'
  const inputClass =
    'h-9 w-full rounded-md border border-[var(--color-border-default)] bg-[var(--color-bg-input)] px-3 text-sm text-[var(--color-text-primary)] placeholder:text-[var(--color-text-muted)] outline-none focus:border-[var(--color-accent)]'

  return (
    <SideDrawer
      open={open}
      onClose={onClose}
      title="添加平台账号"
      width="460px"
      footer={
        <>
          <Button variant="secondary" onClick={() => submit(false)} loading={saving}>保存 &amp; 继续添加</Button>
          <Button onClick={() => submit(true)} loading={saving}>保存</Button>
        </>
      }
    >
      <div className="space-y-5">
        {/* 智能识别 */}
        <div>
          <label className={labelClass}>智能识别</label>
          <textarea
            value={smartText}
            onChange={e => handleSmartChange(e.target.value)}
            rows={3}
            placeholder="账号:密码:2FA"
            className="w-full resize-none rounded-md border border-[var(--color-border-default)] bg-[var(--color-bg-input)] px-3 py-2 text-sm text-[var(--color-text-primary)] placeholder:text-[var(--color-text-muted)] outline-none focus:border-[var(--color-accent)]"
          />
        </div>

        {/* 平台 */}
        <div ref={platformRef} className="relative">
          <label className={labelClass}>平台</label>
          <input
            value={form.platform}
            onChange={e => { update({ platform: e.target.value }); setPlatformOpen(true) }}
            onFocus={() => setPlatformOpen(true)}
            placeholder="请输入或选择平台网址"
            className={inputClass}
          />
          {platformOpen && (
            <div className="absolute z-10 mt-1 max-h-56 w-full overflow-y-auto rounded-md border border-[var(--color-border-default)] bg-[var(--color-bg-elevated)] py-1 shadow-lg">
              {PLATFORM_PRESETS.filter(p => p.url).map(p => (
                <button
                  key={p.value}
                  type="button"
                  onClick={() => { update({ platform: p.url }); setPlatformOpen(false) }}
                  className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm text-[var(--color-text-primary)] hover:bg-[var(--color-bg-muted)]"
                >
                  <span className="text-base">{p.icon}</span>
                  <span className="flex-1">{p.label}</span>
                  <span className="truncate text-xs text-[var(--color-text-muted)]">{p.url}</span>
                </button>
              ))}
            </div>
          )}
        </div>

        {/* 账号 */}
        <div>
          <label className={labelClass}>账号</label>
          <input
            value={form.username}
            onChange={e => update({ username: e.target.value })}
            placeholder="请输入账号"
            className={inputClass}
          />
        </div>

        {/* 密码 */}
        <div>
          <label className={labelClass}>密码</label>
          <div className="relative">
            <input
              type={showPassword ? 'text' : 'password'}
              value={form.password}
              onChange={e => update({ password: e.target.value })}
              placeholder="请输入密码"
              className={`${inputClass} pr-10`}
            />
            <button
              type="button"
              onClick={() => setShowPassword(v => !v)}
              className="absolute right-2 top-1/2 -translate-y-1/2 text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)]"
              aria-label={showPassword ? '隐藏密码' : '显示密码'}
            >
              {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
            </button>
          </div>
        </div>

        {/* 2FA */}
        <div>
          <label className={labelClass}>2FA</label>
          <input
            value={form.twoFA}
            onChange={e => update({ twoFA: e.target.value })}
            placeholder="输入 2FA 密钥"
            className={inputClass}
          />
        </div>
      </div>
    </SideDrawer>
  )
}
