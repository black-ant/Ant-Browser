import { Button, FormItem, Input, Modal, Textarea } from '../../../../shared/components'
import type { ProxyCheckSettings } from '../../types'

interface ProxyPoolCheckSettingsModalProps {
  open: boolean
  checkSettings: ProxyCheckSettings
  checkTargetsText: string
  saving: boolean
  onClose: () => void
  onSave: () => void
  onCheckSettingsChange: (updater: (current: ProxyCheckSettings) => ProxyCheckSettings) => void
  onCheckTargetsTextChange: (value: string) => void
}

export function ProxyPoolCheckSettingsModal({
  open,
  checkSettings,
  checkTargetsText,
  saving,
  onClose,
  onSave,
  onCheckSettingsChange,
  onCheckTargetsTextChange,
}: ProxyPoolCheckSettingsModalProps) {
  return (
    <Modal
      open={open}
      onClose={onClose}
      title="检测设置"
      width="760px"
      footer={(
        <>
          <Button variant="secondary" onClick={onClose}>取消</Button>
          <Button onClick={onSave} loading={saving}>保存</Button>
        </>
      )}
    >
      <div className="space-y-4">
        <FormItem label="桥接启动等待" hint="毫秒" >
          <Input
            type="number"
            value={checkSettings.bridgeStartTimeoutMs}
            onChange={(e) => onCheckSettingsChange(prev => ({ ...prev, bridgeStartTimeoutMs: Number(e.target.value) || 15000 }))}
          />
        </FormItem>
        <FormItem label="成功节点重测间隔" hint="分钟">
          <Input
            type="number"
            min={1}
            max={1440}
            value={checkSettings.speedIntervalMinutes}
            onChange={(e) => onCheckSettingsChange(prev => ({ ...prev, speedIntervalMinutes: Number(e.target.value) || 30 }))}
          />
        </FormItem>
        <FormItem label="失败重试间隔" hint="分钟">
          <Input
            type="number"
            min={1}
            max={1440}
            value={checkSettings.failureRetryMinutes}
            onChange={(e) => onCheckSettingsChange(prev => ({ ...prev, failureRetryMinutes: Number(e.target.value) || 5 }))}
          />
        </FormItem>
        <FormItem label="测速并发上限" hint="手动测速最多同时运行的代理数">
          <Input
            type="number"
            min={1}
            max={10}
            value={checkSettings.speedConcurrency}
            onChange={(e) => onCheckSettingsChange(prev => ({ ...prev, speedConcurrency: Number(e.target.value) || 5 }))}
          />
        </FormItem>
        <FormItem label="测速目标 ID">
          <Input
            value={checkSettings.speedTargetId}
            onChange={(e) => onCheckSettingsChange(prev => ({ ...prev, speedTargetId: e.target.value }))}
          />
        </FormItem>
        <FormItem label="IP 健康目标 ID">
          <Input
            value={checkSettings.ipHealthTargetId}
            onChange={(e) => onCheckSettingsChange(prev => ({ ...prev, ipHealthTargetId: e.target.value }))}
          />
        </FormItem>
        <FormItem label="检测目标列表（JSON，每项一个）" hint="可直接编辑 URL、超时、期望状态码">
          <Textarea
            value={checkTargetsText}
            onChange={(e) => onCheckTargetsTextChange(e.target.value)}
            rows={14}
          />
        </FormItem>
      </div>
    </Modal>
  )
}
