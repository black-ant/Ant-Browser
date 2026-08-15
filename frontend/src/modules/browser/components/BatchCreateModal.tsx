import { useMemo, useState } from 'react'

import { Button, FormItem, Input, Modal } from '../../../shared/components'

const MAX_BATCH = 200

interface BatchCreateModalProps {
  open: boolean
  loading: boolean
  onClose: () => void
  onSubmit: (prefix: string, count: number, startIndex: number) => void
}

const pad3 = (n: number) => String(n).padStart(3, '0')

export function BatchCreateModal({ open, loading, onClose, onSubmit }: BatchCreateModalProps) {
  const [prefix, setPrefix] = useState('env')
  const [count, setCount] = useState(10)
  const [startIndex, setStartIndex] = useState(1)

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
          <Button onClick={() => onSubmit(trimmedPrefix, safeCount, safeStart)} loading={loading} disabled={!canSubmit}>
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
