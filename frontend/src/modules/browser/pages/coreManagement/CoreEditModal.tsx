import type { Dispatch, SetStateAction } from 'react'
import { Button, FormItem, Input, Modal } from '../../../../shared/components'
import type { BrowserCoreValidateResult } from '../../types'
import type { CoreEditForm } from '../coreManagement.types'

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
  return (
    <Modal
      open={open}
      onClose={onClose}
      title={isEditing ? '编辑内核' : '新增内核'}
      width="500px"
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
      </div>
    </Modal>
  )
}
