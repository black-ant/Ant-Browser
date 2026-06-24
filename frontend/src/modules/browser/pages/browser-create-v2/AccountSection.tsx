import { Card } from '../../../../shared/components'
import { AccountSelector } from './AccountSelector'
import type { FingerprintProfileConfig } from './types'

interface AccountSectionProps {
  config: FingerprintProfileConfig
  updateConfig: (updater: (prev: FingerprintProfileConfig) => FingerprintProfileConfig) => void
}

export function AccountSection({ config, updateConfig }: AccountSectionProps) {
  const handleAccountChange = (accountIds: string[]) => {
    updateConfig(prev => ({
      ...prev,
      preferences: {
        ...prev.preferences,
        accountIds,
      },
    }))
  }

  return (
    <Card title="账号关联" subtitle="关联平台账号以同步 Cookie">
      <div>
        <label className="block text-sm font-medium text-[var(--color-text-secondary)] mb-2">
          关联的平台账号
        </label>
        <AccountSelector
          selectedIds={config.preferences.accountIds}
          onChange={handleAccountChange}
        />
        <p className="mt-2 text-xs text-[var(--color-text-muted)]">
          💡 提示：关联账号后，可在实例启动时自动导入账号的 Cookie
        </p>
      </div>
    </Card>
  )
}
