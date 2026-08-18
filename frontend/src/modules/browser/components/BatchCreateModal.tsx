import { useMemo, useState } from 'react'

import { Button, FormItem, Input, Modal } from '../../../shared/components'

const MAX_BATCH = 200

type IdentityPlatform = '' | 'windows' | 'macos'
type KernelSelect = 'auto' | 'all148' | 'all144'

interface BatchCreateModalProps {
  open: boolean
  loading: boolean
  onClose: () => void
  onSubmit: (
    prefix: string,
    count: number,
    startIndex: number,
    platform: string,
    kernel: string,
    liveKeepaliveEnabled: boolean,
    muteAudio: boolean,
  ) => void
}

const pad3 = (n: number) => String(n).padStart(3, '0')

export function BatchCreateModal({ open, loading, onClose, onSubmit }: BatchCreateModalProps) {
  const [prefix, setPrefix] = useState('env')
  const [count, setCount] = useState(10)
  const [startIndex, setStartIndex] = useState(1)
  const [platform, setPlatform] = useState<IdentityPlatform>('')
  const [kernel, setKernel] = useState<KernelSelect>('auto')
  const [liveKeepalive, setLiveKeepalive] = useState(true)
  const [muteAudio, setMuteAudio] = useState(true)

  const trimmedPrefix = prefix.trim()
  const safeCount = Math.min(Math.max(1, Math.floor(count) || 0), MAX_BATCH)
  const safeStart = Math.max(1, Math.floor(startIndex) || 1)

  const preview = useMemo(() => {
    if (!trimmedPrefix) return ''
    const first = `${trimmedPrefix}-${pad3(safeStart)}`
    if (safeCount === 1) return first
    return `${first} ~ ${trimmedPrefix}-${pad3(safeStart + safeCount - 1)}`
  }, [trimmedPrefix, safeCount, safeStart])

  const canSubmit = trimmedPrefix.length > 0 && safeCount >= 1 && !loading

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="批量新建配置"
      width="480px"
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={loading}>取消</Button>
          <Button
            onClick={() => onSubmit(trimmedPrefix, safeCount, safeStart, platform, kernel, liveKeepalive, muteAudio)}
            loading={loading}
            disabled={!canSubmit}
          >
            创建 {safeCount} 个
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        <FormItem label="名称前缀" required>
          <Input value={prefix} onChange={(event) => setPrefix(event.target.value)} placeholder="env" />
        </FormItem>
        <div className="grid grid-cols-2 gap-4">
          <FormItem label="数量" hint={`最多 ${MAX_BATCH} 个`}>
            <Input
              type="number"
              min={1}
              max={MAX_BATCH}
              value={count}
              onChange={(event) => setCount(Number(event.target.value))}
            />
          </FormItem>
          <FormItem label="起始编号">
            <Input
              type="number"
              min={1}
              value={startIndex}
              onChange={(event) => setStartIndex(Number(event.target.value))}
            />
          </FormItem>
        </div>

        <FormItem label="身份平台" hint="选定后本批全部生成该平台身份;任何情况都不重复">
          <select
            value={platform}
            onChange={(event) => setPlatform(event.target.value as IdentityPlatform)}
            className="w-full px-3 py-2 text-sm rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-input)] text-[var(--color-text-primary)] focus:outline-none focus:border-[var(--color-accent)]"
          >
            <option value="">全部平台</option>
            <option value="windows">Windows</option>
            <option value="macos">macOS</option>
          </select>
        </FormItem>

        <FormItem label="内核版本" hint="UA 与引擎版本一致;默认按真实人群分布(148 为主、144 少数)">
          <select
            value={kernel}
            onChange={(event) => setKernel(event.target.value as KernelSelect)}
            className="w-full px-3 py-2 text-sm rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-input)] text-[var(--color-text-primary)] focus:outline-none focus:border-[var(--color-accent)]"
          >
            <option value="auto">自动分布(148 为主,推荐)</option>
            <option value="all148">全部 148</option>
            <option value="all144">全部 144</option>
          </select>
        </FormItem>

        <div className="flex items-center gap-6">
          <label className="flex items-center gap-2 text-sm text-[var(--color-text-secondary)] cursor-pointer">
            <input type="checkbox" className="w-4 h-4 accent-[var(--color-accent)]" checked={liveKeepalive} onChange={(e) => setLiveKeepalive(e.target.checked)} />
            开启直播保活
          </label>
          <label className="flex items-center gap-2 text-sm text-[var(--color-text-secondary)] cursor-pointer">
            <input type="checkbox" className="w-4 h-4 accent-[var(--color-accent)]" checked={muteAudio} onChange={(e) => setMuteAudio(e.target.checked)} />
            默认静音
          </label>
        </div>

        <div className="rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-secondary)] px-3 py-2 text-sm">
          <div className="mb-1 text-[var(--color-text-muted)]">名称预览(编号 3 位)</div>
          <div className="font-mono text-[var(--color-text-primary)]">{preview || '请输入前缀'}</div>
          <div className="mt-2 text-xs text-[var(--color-text-muted)]">
            每个环境都会获得独立、唯一、自洽的指纹身份。大批量会占用较多内存,请按需创建。
          </div>
        </div>
      </div>
    </Modal>
  )
}
