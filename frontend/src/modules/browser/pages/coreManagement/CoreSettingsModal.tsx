import type { Dispatch, SetStateAction } from 'react'
import { Button, FormItem, Input, Modal, Switch, Textarea } from '../../../../shared/components'
import type { CoreSettingsForm } from '../coreManagement.types'

interface CoreSettingsModalProps {
  open: boolean
  form: CoreSettingsForm
  saving: boolean
  setForm: Dispatch<SetStateAction<CoreSettingsForm>>
  onClose: () => void
  onSave: () => void
}

export function CoreSettingsModal({ open, form, saving, setForm, onClose, onSave }: CoreSettingsModalProps) {
  return (
    <Modal
      open={open}
      onClose={onClose}
      title="编辑全局设置"
      width="550px"
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>取消</Button>
          <Button onClick={onSave} loading={saving}>保存</Button>
        </>
      }
    >
      <div className="space-y-4">
        <FormItem label="用户数据根目录">
          <Input
            value={form.userDataRoot}
            onChange={e => setForm(prev => ({ ...prev, userDataRoot: e.target.value }))}
            placeholder="例如：data"
          />
        </FormItem>
        <FormItem label="默认指纹参数">
          <Textarea
            value={form.defaultFingerprintArgs}
            onChange={e => setForm(prev => ({ ...prev, defaultFingerprintArgs: e.target.value }))}
            rows={4}
            placeholder="每行一个参数，如 --fingerprint-brand=Chrome"
          />
        </FormItem>
        <FormItem label="默认启动参数">
          <Textarea
            value={form.defaultLaunchArgs}
            onChange={e => setForm(prev => ({ ...prev, defaultLaunchArgs: e.target.value }))}
            rows={4}
            placeholder="每行一个参数，如 --disable-sync"
          />
        </FormItem>
        <FormItem label="默认启动页面" hint="每行一个 URL，留空则启动时不自动打开页面">
          <Textarea
            value={form.defaultStartUrls}
            onChange={e => setForm(prev => ({ ...prev, defaultStartUrls: e.target.value }))}
            rows={4}
            placeholder="启动 URL"
          />
        </FormItem>
        <FormItem label="轻启动模式" hint="先起空白页，实例就绪后再打开默认页面">
          <div className="flex items-center justify-between rounded-lg border border-[var(--color-border-default)] px-3 py-2">
            <span className="text-sm text-[var(--color-text-primary)]">延后打开启动页</span>
            <Switch
              checked={form.lightStartEnabled}
              onChange={checked => setForm(prev => ({ ...prev, lightStartEnabled: checked }))}
            />
          </div>
        </FormItem>
        <FormItem label="默认恢复历史标签" hint="实例选择跟随内核时使用；不影响启动页和启动书签。实例未覆盖时，下次启动恢复之前的标签页和窗口。">
          <div className="flex items-center justify-between rounded-lg border border-[var(--color-border-default)] px-3 py-2">
            <div>
              <p className="text-sm text-[var(--color-text-primary)]">内核默认</p>
            </div>
            <Switch
              checked={form.restoreLastSession}
              onChange={checked => setForm(prev => ({ ...prev, restoreLastSession: checked }))}
            />
          </div>
        </FormItem>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4 items-end">
          <FormItem label="启动就绪超时（毫秒）">
            <Input
              type="number"
              min={1000}
              step={500}
              value={form.startReadyTimeoutMs}
              onChange={e => setForm(prev => ({ ...prev, startReadyTimeoutMs: Math.max(1000, Number(e.target.value) || 3000) }))}
              placeholder="3000"
            />
          </FormItem>
          <FormItem label="启动稳定窗口（毫秒）">
            <Input
              type="number"
              min={0}
              step={100}
              value={form.startStableWindowMs}
              onChange={e => setForm(prev => ({ ...prev, startStableWindowMs: Math.max(0, Number(e.target.value) || 1200) }))}
              placeholder="1200"
            />
          </FormItem>
        </div>
      </div>
    </Modal>
  )
}
