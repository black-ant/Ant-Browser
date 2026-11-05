import { useEffect, useState } from 'react'

import { Button, FormItem, Input, Modal } from '../../../shared/components'

type ProfilePackagePasswordMode = 'export' | 'import'

interface ProfilePackagePasswordModalProps {
  open: boolean
  mode: ProfilePackagePasswordMode
  busy?: boolean
  error?: string
  onClose: () => void
  onConfirm: (password: string) => void
  onSkip?: () => void
}

const minimumPasswordLength = 8

export function ProfilePackagePasswordModal({
  open,
  mode,
  busy = false,
  error = '',
  onClose,
  onConfirm,
  onSkip,
}: ProfilePackagePasswordModalProps) {
  const [password, setPassword] = useState('')
  const [confirmation, setConfirmation] = useState('')
  const [validationError, setValidationError] = useState('')

  useEffect(() => {
    if (!open) {
      setPassword('')
      setConfirmation('')
      setValidationError('')
    }
  }, [open])

  const handleConfirm = () => {
    if (busy) return
    if (password.length < minimumPasswordLength) {
      setValidationError(`密码至少需要 ${minimumPasswordLength} 个字符`)
      return
    }
    if (mode === 'export' && password !== confirmation) {
      setValidationError('两次输入的密码不一致')
      return
    }
    setValidationError('')
    onConfirm(password)
  }

  const activeError = validationError || error
  const isExport = mode === 'export'

  return (
    <Modal
      open={open}
      onClose={busy ? () => undefined : onClose}
      title={isExport ? '保护跨电脑登录态' : '恢复跨电脑登录态'}
      width="440px"
      footer={(
        <>
          {isExport && onSkip && (
            <Button variant="ghost" onClick={onSkip} disabled={busy}>跳过，不迁移登录态</Button>
          )}
          <Button variant="secondary" onClick={onClose} disabled={busy}>取消</Button>
          <Button onClick={handleConfirm} loading={busy}>{isExport ? '开始导出' : '继续导入'}</Button>
        </>
      )}
    >
      <div className="space-y-4">
        <p className="text-sm leading-6 text-[var(--color-text-secondary)]">
          {isExport
            ? '应用会读取当前实例中已解密的 Cookie，并使用此密码加密后写入实例包。密码不会写入备份。'
            : '请输入导出时设置的密码，恢复实例 Cookie。密码错误或实例包损坏时不会继续导入。'}
        </p>
        <FormItem label="迁移密码" required hint="至少 8 个字符，导入时必须使用同一密码。">
          <Input
            type="password"
            value={password}
            onChange={event => {
              setPassword(event.target.value)
              setValidationError('')
            }}
            autoComplete={isExport ? 'new-password' : 'current-password'}
            placeholder="输入迁移密码"
            disabled={busy}
            autoFocus
          />
        </FormItem>
        {isExport && (
          <FormItem label="确认密码" required>
            <Input
              type="password"
              value={confirmation}
              onChange={event => {
                setConfirmation(event.target.value)
                setValidationError('')
              }}
              autoComplete="new-password"
              placeholder="再次输入迁移密码"
              disabled={busy}
            />
          </FormItem>
        )}
        <p className="text-xs leading-5 text-[var(--color-text-muted)]">
          {isExport
            ? 'Cookie 可跨电脑迁移；Local Storage、IndexedDB、WebAuthn/Passkey 等数据仍可能需要重新登录。不需要迁移登录态时可选择跳过。'
            : 'Cookie 可跨电脑迁移；Local Storage、IndexedDB、WebAuthn/Passkey 等数据仍可能需要重新登录。'}
        </p>
        {activeError && <p className="text-sm text-[var(--color-error)]" role="alert">{activeError}</p>}
      </div>
    </Modal>
  )
}
