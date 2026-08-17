import { Card, FormItem, Input, Select, Switch } from '../../../shared/components'
import type { AppSettings } from '../types'

type SettingsChangeHandler = <K extends keyof AppSettings>(key: K, value: AppSettings[K]) => void

interface SettingsCardsProps {
  settings: AppSettings
  onChange: SettingsChangeHandler
}

export function SettingsBasicFeatureCards({ settings, onChange }: SettingsCardsProps) {
  return (
    <>
      <Card title="基础设置" subtitle="应用的基本信息配置">
        <div className="space-y-4">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <FormItem label="应用名称" required>
              <Input value={settings.appName} onChange={e => onChange('appName', e.target.value)} placeholder="请输入应用名称" />
            </FormItem>
            <FormItem label="语言">
              <Select
                value={settings.language}
                onChange={e => onChange('language', e.target.value)}
                options={[{ value: 'zh-CN', label: '简体中文' }, { value: 'en-US', label: 'English' }]}
              />
            </FormItem>
          </div>
          <FormItem label="应用描述">
            <Input value={settings.appDescription} onChange={e => onChange('appDescription', e.target.value)} placeholder="请输入应用描述" />
          </FormItem>
        </div>
      </Card>

      <Card title="功能设置" subtitle="启用或禁用特定功能">
        <div className="space-y-5">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium text-[var(--color-text-primary)]">保存通知历史</p>
              <p className="text-xs text-[var(--color-text-muted)] mt-0.5">仅控制是否写入通知中心，不影响即时提示</p>
            </div>
            <Switch checked={settings.enableNotifications} onChange={v => onChange('enableNotifications', v)} />
          </div>
          <div className="h-px bg-[var(--color-border-muted)]" />
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium text-[var(--color-text-primary)]">自动保存</p>
              <p className="text-xs text-[var(--color-text-muted)] mt-0.5">自动保存编辑中的内容</p>
            </div>
            <Switch checked={settings.enableAutoSave} onChange={v => onChange('enableAutoSave', v)} />
          </div>
          {settings.enableAutoSave && (
            <div className="pl-4 border-l-2 border-[var(--color-border-muted)]">
              <FormItem label="自动保存间隔（秒）">
                <Input
                  type="number"
                  value={settings.autoSaveInterval}
                  onChange={e => onChange('autoSaveInterval', parseInt(e.target.value) || 30)}
                  min={5}
                  max={300}
                  className="max-w-[120px]"
                />
              </FormItem>
            </div>
          )}
          <div className="h-px bg-[var(--color-border-muted)]" />
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium text-[var(--color-text-primary)]">启用缓存</p>
              <p className="text-xs text-[var(--color-text-muted)] mt-0.5">缓存数据以提高性能</p>
            </div>
            <Switch checked={settings.cacheEnabled} onChange={v => onChange('cacheEnabled', v)} />
          </div>
        </div>
      </Card>
    </>
  )
}

export function SettingsAdvancedCard({ settings, onChange }: SettingsCardsProps) {
  return (
    <Card title="高级设置" subtitle="高级配置选项">
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
        <FormItem label="最大上传大小（MB）">
          <Input type="number" value={settings.maxUploadSize} onChange={e => onChange('maxUploadSize', parseInt(e.target.value) || 10)} min={1} max={100} />
        </FormItem>
        <FormItem label="会话超时（分钟）">
          <Input type="number" value={settings.sessionTimeout} onChange={e => onChange('sessionTimeout', parseInt(e.target.value) || 30)} min={5} max={120} />
        </FormItem>
        <FormItem label="日志级别">
          <Select
            value={settings.logLevel}
            onChange={e => onChange('logLevel', e.target.value as AppSettings['logLevel'])}
            options={[
              { value: 'debug', label: 'Debug' },
              { value: 'info', label: 'Info' },
              { value: 'warn', label: 'Warning' },
              { value: 'error', label: 'Error' },
            ]}
          />
        </FormItem>
      </div>
    </Card>
  )
}
