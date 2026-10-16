import type { Dispatch, SetStateAction } from 'react'
import { Button, FormItem, Input, Modal, Select, Textarea } from '../../../../shared/components'
import type { BrowserCoreValidateResult } from '../../types'
import type { CoreEditForm } from '../coreManagement.types'
import { CORE_BACKEND_OPTIONS, isCloakBackend, normalizeCoreBackend } from '../../utils/coreBackend'

interface CoreEditModalProps {
  open: boolean
  isEditing: boolean
  form: CoreEditForm
  saving: boolean
  pathValidating: boolean
  pathValidResult: BrowserCoreValidateResult | null
  setForm: Dispatch<SetStateAction<CoreEditForm>>
  onClose: () => void
  onSave: () => void
}

export function CoreEditModal({
  open,
  isEditing,
  form,
  saving,
  pathValidating,
  pathValidResult,
  setForm,
  onClose,
  onSave,
}: CoreEditModalProps) {
  const backend = normalizeCoreBackend(form.coreBackend)
  const backendOption = CORE_BACKEND_OPTIONS.find(option => option.value === backend)
  const cloakSelected = isCloakBackend(backend)

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={isEditing ? '编辑内核' : '新增内核'}
      width="560px"
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>取消</Button>
          <Button onClick={onSave} loading={saving}>保存</Button>
        </>
      }
    >
      <div className="space-y-4">
        <FormItem label="内核名称" required>
          <Input
            value={form.coreName}
            onChange={e => setForm(prev => ({ ...prev, coreName: e.target.value }))}
            placeholder="例如：Chrome 142"
          />
        </FormItem>
        <FormItem label="内核后端" required>
          <Select
            value={backend}
            onChange={e => setForm(prev => ({ ...prev, coreBackend: e.target.value }))}
            options={CORE_BACKEND_OPTIONS.map(option => ({ value: option.value, label: option.label }))}
          />
          {backendOption && (
            <p className="text-xs text-[var(--color-text-muted)] mt-1 leading-5">{backendOption.description}</p>
          )}
        </FormItem>
        <FormItem label="内核路径" required>
          <Input
            value={form.corePath}
            onChange={e => setForm(prev => ({ ...prev, corePath: e.target.value }))}
            placeholder="相对路径（如 chrome）或绝对路径"
          />
          {pathValidating && (
            <p className="text-xs text-[var(--color-text-muted)] mt-1">验证中...</p>
          )}
          {!pathValidating && pathValidResult && (
            <p className={`text-xs mt-1 ${pathValidResult.valid ? 'text-green-600' : 'text-red-500'}`}>
              {pathValidResult.message}
            </p>
          )}
        </FormItem>
        {cloakSelected && (
          <FormItem label="内核环境变量">
            <Textarea
              value={form.coreEnv}
              onChange={e => setForm(prev => ({ ...prev, coreEnv: e.target.value }))}
              placeholder={'每行一条 KEY=VALUE，例如：\nCLOAKBROWSER_LICENSE_KEY=your-key\nCLOAKBROWSER_CACHE_DIR=D:/cloak-cache'}
              rows={4}
            />
            <p className="text-xs text-[var(--color-text-muted)] mt-1 leading-5">
              仅接受 <code>CLOAKBROWSER_</code> 前缀的变量，其他键会在启动时被忽略。启动实例时按此内核注入，不影响其他内核。
            </p>
          </FormItem>
        )}
      </div>
    </Modal>
  )
}
