import { useEffect, useState } from 'react'
import { Modal, Button, FormItem, Input, Select, Textarea } from '../../../../shared/components'
import {
  IdentityPoolRecord,
  IdentityValidationResult,
  emptyIdentityRecord,
  validateIdentity,
} from '../../api'

const PLATFORM_OPTIONS = [
  { value: 'windows', label: 'Windows' },
  { value: 'macos', label: 'macOS' },
  { value: 'linux', label: 'Linux' },
]

interface Props {
  open: boolean
  record: IdentityPoolRecord | null // null = 新增
  saving: boolean
  onClose: () => void
  onSubmit: (rec: IdentityPoolRecord) => void
}

export function IdentityEditModal({ open, record, saving, onClose, onSubmit }: Props) {
  const [form, setForm] = useState<IdentityPoolRecord>(emptyIdentityRecord())
  const [validation, setValidation] = useState<IdentityValidationResult | null>(null)

  useEffect(() => {
    if (!open) return
    setForm(record ? { ...record } : emptyIdentityRecord())
    setValidation(null)
  }, [open, record])

  const set = (patch: Partial<IdentityPoolRecord>) => setForm(prev => ({ ...prev, ...patch }))
  const setScreen = (patch: Partial<IdentityPoolRecord['screen']>) =>
    setForm(prev => ({ ...prev, screen: { ...prev.screen, ...patch } }))

  const handleSubmit = async () => {
    // 时区/语言/Locale 不在此编辑:保留记录原值(仅作占位),创建环境时按出口 IP 自动覆盖。
    const rec: IdentityPoolRecord = { ...form }
    const res = await validateIdentity(rec)
    if (res && !res.ok) {
      setValidation(res) // 有 error 级问题 → 阻止保存,展示问题
      return
    }
    onSubmit(rec)
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      width="660px"
      title={record ? '编辑身份模板' : '新增身份模板'}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>取消</Button>
          <Button loading={saving} onClick={handleSubmit}>保存</Button>
        </>
      }
    >
      <div className="grid grid-cols-2 gap-3">
        <FormItem label="平台" required>
          <Select options={PLATFORM_OPTIONS} value={form.platform} onChange={e => set({ platform: e.target.value })} />
        </FormItem>
        <FormItem label="品牌版本" required hint="Client Hints 版本;必须与 UA 里的 Chrome 版本一致">
          <Input value={form.brandVersion} onChange={e => set({ brandVersion: e.target.value })} placeholder="147.0.0.0" />
        </FormItem>
        <FormItem label="User-Agent" required className="col-span-2" hint="Chrome/版本 必须与品牌版本一致,OS 段必须与平台一致">
          <Textarea rows={2} value={form.uaFull} onChange={e => set({ uaFull: e.target.value })} placeholder="Mozilla/5.0 ... Chrome/147.0.0.0 Safari/537.36" />
        </FormItem>
        <FormItem label="CPU 核心数" required>
          <Input type="number" value={form.hardwareConcurrency} onChange={e => set({ hardwareConcurrency: Number(e.target.value) })} />
        </FormItem>
        <FormItem label="设备内存(GB)">
          <Input type="number" value={form.deviceMemory} onChange={e => set({ deviceMemory: Number(e.target.value) })} />
        </FormItem>
        <FormItem label="屏幕宽">
          <Input type="number" value={form.screen.width} onChange={e => setScreen({ width: Number(e.target.value) })} />
        </FormItem>
        <FormItem label="屏幕高">
          <Input type="number" value={form.screen.height} onChange={e => setScreen({ height: Number(e.target.value) })} />
        </FormItem>
        <FormItem label="DPR">
          <Input type="number" step="0.01" value={form.screen.dpr} onChange={e => setScreen({ dpr: Number(e.target.value) })} />
        </FormItem>
        <FormItem label="色深">
          <Input type="number" value={form.screen.colorDepth} onChange={e => setScreen({ colorDepth: Number(e.target.value) })} />
        </FormItem>
        <FormItem label="窗口大小" hint="如 1536,816">
          <Input value={form.windowSize} onChange={e => set({ windowSize: e.target.value })} placeholder="1536,816" />
        </FormItem>
        <FormItem label="权重" hint="加权采样概率;0 = 永不采样">
          <Input type="number" value={form.weight} onChange={e => set({ weight: Number(e.target.value) })} />
        </FormItem>
        <div className="col-span-2 rounded-lg bg-[var(--color-bg-muted)] px-3 py-2 text-xs text-[var(--color-text-muted)]">
          时区 / 语言 / Locale 不在此配置:创建环境时按出口 IP 自动对齐(直连=本地国家,绑代理=代理所在地区)。
        </div>
      </div>
      {validation && !validation.ok && (
        <div className="mt-3 rounded-lg border border-[var(--color-error)]/30 bg-[var(--color-error)]/10 p-2.5 text-xs">
          <div className="mb-1 font-medium text-[var(--color-error)]">不自洽,请修正后再保存:</div>
          <ul className="space-y-0.5">
            {validation.issues.map((is, i) => (
              <li key={i} className="text-[var(--color-text-secondary)]">• [{is.field}] {is.message}</li>
            ))}
          </ul>
        </div>
      )}
    </Modal>
  )
}
