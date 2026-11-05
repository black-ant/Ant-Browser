import { Edit2, Settings } from 'lucide-react'
import { Button, Card } from '../../../../shared/components'
import type { BrowserSettings } from '../../types'

interface CoreSettingsCardProps {
  settings: BrowserSettings
  onEdit: () => void
}

const settingsValueClass = 'min-h-8 rounded-lg border border-[var(--color-border-muted)] bg-[var(--color-bg-subtle)] px-2.5 py-1.5 text-sm leading-5 text-[var(--color-text-primary)]'
const settingsListClass = 'min-h-9 max-h-32 overflow-auto whitespace-pre-wrap rounded-lg border border-[var(--color-border-muted)] bg-[var(--color-bg-subtle)] px-3 py-2 text-sm leading-5 text-[var(--color-text-primary)]'
const settingsCompactListClass = `${settingsValueClass} max-h-16 overflow-auto whitespace-pre-wrap`

export function CoreSettingsCard({ settings, onEdit }: CoreSettingsCardProps) {
  return (
    <Card>
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2">
          <Settings className="w-5 h-5 text-[var(--color-text-muted)]" />
          <h3 className="text-base font-medium text-[var(--color-text-primary)]">全局设置</h3>
        </div>
        <Button size="sm" variant="ghost" onClick={onEdit}>
          <Edit2 className="w-4 h-4 mr-1" />
          编辑
        </Button>
      </div>
      <div className="space-y-4">
        <div className="grid grid-cols-2 gap-3 lg:grid-cols-12">
          <SettingsValue className="col-span-2 lg:col-span-3" label="用户数据根目录" value={settings.userDataRoot || '-'} />
          <SettingsValue className="lg:col-span-1" label="恢复历史标签" value={settings.restoreLastSession ? '开启' : '关闭'} />
          <SettingsValue className="lg:col-span-1" label="轻启动模式" value={settings.lightStartEnabled ? '开启' : '关闭'} />
          <SettingsValue className="lg:col-span-2" label="启动就绪超时" value={`${settings.startReadyTimeoutMs} ms`} />
          <SettingsValue className="lg:col-span-2" label="启动稳定窗口" value={`${settings.startStableWindowMs} ms`} />
          <SettingsList className="col-span-2 lg:col-span-3" label="默认启动页面" values={settings.defaultStartUrls} compact />
        </div>
        <div className="grid grid-cols-1 gap-3">
          <SettingsList label="默认指纹参数" values={settings.defaultFingerprintArgs} />
          <SettingsList label="默认启动参数" values={settings.defaultLaunchArgs} />
        </div>
      </div>
    </Card>
  )
}

function SettingsValue({ label, value, className = '' }: { label: string; value: string; className?: string }) {
  return (
    <div className={className}>
      <p className="text-xs text-[var(--color-text-muted)] mb-1">{label}</p>
      <div className={`${settingsValueClass} break-all`}>
        {value}
      </div>
    </div>
  )
}

function SettingsList({ label, values, className = '', compact = false }: { label: string; values: string[]; className?: string; compact?: boolean }) {
  return (
    <div className={className}>
      <p className="text-xs text-[var(--color-text-muted)] mb-1">{label}</p>
      {values.length > 0 ? (
        <pre className={compact ? settingsCompactListClass : settingsListClass}>
          {values.join('\n')}
        </pre>
      ) : (
        <div className={settingsValueClass}>
          -
        </div>
      )}
    </div>
  )
}
