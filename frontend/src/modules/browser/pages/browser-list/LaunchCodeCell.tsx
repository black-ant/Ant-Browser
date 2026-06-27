import { useState } from 'react'
import { Copy, RefreshCw, Pencil } from 'lucide-react'
import { toast, Modal, Button, Input } from '../../../../shared/components'
import { regenerateBrowserProfileCode, setBrowserProfileCode } from '../../api'

// 快捷配置码单元格（从 BrowserListPage 提取）
export function LaunchCodeCell({ profileId, code, onRefresh }: { profileId: string; code: string; onRefresh: () => void }) {
  const [loading, setLoading] = useState(false)
  const [customOpen, setCustomOpen] = useState(false)
  const [customValue, setCustomValue] = useState('')

  const handleCopy = () => {
    if (!code) return
    navigator.clipboard.writeText(code).then(() => toast.success('已复制快捷码'))
  }

  const handleRegenerate = async () => {
    setLoading(true)
    try {
      await regenerateBrowserProfileCode(profileId)
      onRefresh()
      toast.success('快捷码已重新生成')
    } catch {
      toast.error('重新生成失败')
    } finally {
      setLoading(false)
    }
  }

  const openCustom = () => {
    setCustomValue(code || '')
    setCustomOpen(true)
  }

  const submitCustom = async () => {
    const value = customValue.trim()
    if (!value) {
      toast.error('Code 不能为空')
      return
    }
    setLoading(true)
    try {
      const applied = await setBrowserProfileCode(profileId, value)
      onRefresh()
      toast.success(`Code 已更新为 ${applied}`)
      setCustomOpen(false)
    } catch (error: any) {
      toast.error(error?.message || '设置自定义 Code 失败')
    } finally {
      setLoading(false)
    }
  }

  if (!code) return <span className="text-[var(--color-text-muted)] text-xs">-</span>

  return (
    <div className="flex min-w-0 items-center gap-1 whitespace-nowrap">
      <code className="max-w-[84px] truncate rounded bg-[var(--color-bg-secondary)] px-1.5 py-0.5 font-mono text-xs text-[var(--color-accent)]" title={code}>{code}</code>
      <button onClick={handleCopy} className="flex h-6 w-6 items-center justify-center rounded-md text-[var(--color-text-muted)] transition-colors hover:bg-[var(--color-accent)]/10 hover:text-[var(--color-accent)]" title="复制">
        <Copy className="h-3.5 w-3.5" />
      </button>
      <button onClick={handleRegenerate} disabled={loading} className="flex h-6 w-6 items-center justify-center rounded-md text-[var(--color-text-muted)] transition-colors hover:bg-[var(--color-accent)]/10 hover:text-[var(--color-accent)] disabled:opacity-50" title="重新生成">
        <RefreshCw className="h-3.5 w-3.5" />
      </button>
      <button onClick={openCustom} disabled={loading} className="flex h-6 w-6 items-center justify-center rounded-md text-[var(--color-text-muted)] transition-colors hover:bg-[var(--color-accent)]/10 hover:text-[var(--color-accent)] disabled:opacity-50" title="自定义">
        <Pencil className="h-3.5 w-3.5" />
      </button>

      <Modal
        open={customOpen}
        onClose={() => setCustomOpen(false)}
        title="自定义快捷码"
        width="420px"
        footer={<>
          <Button variant="secondary" onClick={() => setCustomOpen(false)}>取消</Button>
          <Button onClick={submitCustom} loading={loading}>保存</Button>
        </>}
      >
        <div className="space-y-2">
          <Input
            value={customValue}
            onChange={e => setCustomValue(e.target.value)}
            placeholder="4-32 位，仅支持字母 / 数字 / _ / -"
            onKeyDown={e => { if (e.key === 'Enter') void submitCustom() }}
          />
          <p className="text-xs text-[var(--color-text-muted)]">用于通过快捷码快速启动该窗口。</p>
        </div>
      </Modal>
    </div>
  )
}

