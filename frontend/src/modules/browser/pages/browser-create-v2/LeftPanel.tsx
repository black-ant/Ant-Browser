import { Card } from '../../../../shared/components'
import type { FingerprintProfileConfig } from './types'
import type { ValidationError } from './validation'
import { getFieldError } from './validation'
import { WindowInfoSection } from './WindowInfoSection'
import { ProxyConfigSection } from './ProxyConfigSection'
import { URLsSection } from './URLsSection'
import { BasicSettingsSection } from './BasicSettingsSection'
import { AdvancedFingerprintSection } from './AdvancedFingerprintSection'
import { AccountSection } from './AccountSection'
import type { BrowserCore, BrowserGroupWithCount, BrowserProxy } from '../../types'
import type { BrowserExtension } from '../../api'

function isLoadableExtension(extension: BrowserExtension): boolean {
  return (
    extension.enabled !== false &&
    (extension.sourceType || 'local').toLowerCase() === 'local' &&
    Boolean(extension.extensionPath?.trim())
  )
}

interface LeftPanelProps {
  config: FingerprintProfileConfig
  updateConfig: (updater: (prev: FingerprintProfileConfig) => FingerprintProfileConfig) => void
  errors: ValidationError[]
  cores: BrowserCore[]
  proxies: BrowserProxy[]  // 代理池列表
  allTags: string[]
  groups: BrowserGroupWithCount[]
  extensions: BrowserExtension[]
  selectedCoreId: string
  selectedGroupId: string
  onCoreChange: (coreId: string) => void
  onGroupChange: (groupId: string) => void
}

export function LeftPanel({
  config,
  updateConfig,
  errors,
  cores,
  proxies,
  allTags,
  groups,
  extensions,
  selectedCoreId,
  selectedGroupId,
  onCoreChange,
  onGroupChange,
}: LeftPanelProps) {
  const launchCodeError = getFieldError(errors, 'preferences.launchCode')
  const loadableExtensions = extensions.filter(isLoadableExtension)
  const selectedExtensionIds = config.preferences.extensionIds || []

  const handleExtensionToggle = (extensionId: string) => {
    updateConfig(prev => {
      const current = prev.preferences.extensionIds || []
      const next = current.includes(extensionId)
        ? current.filter(id => id !== extensionId)
        : [...current, extensionId]
      return {
        ...prev,
        preferences: {
          ...prev.preferences,
          extensionIds: next,
        },
      }
    })
  }

  return (
    <div className="browser-create-form-stack">
      {/* 窗口信息 */}
      <WindowInfoSection
        config={config}
        updateConfig={updateConfig}
        errors={errors}
        cores={cores}
        selectedCoreId={selectedCoreId}
        onCoreChange={onCoreChange}
      />

      {/* 代理 IP */}
      <ProxyConfigSection config={config} updateConfig={updateConfig} errors={errors} proxies={proxies} />

      {/* URLs */}
      <URLsSection config={config} updateConfig={updateConfig} />

      {/* 基础设置 */}
      <BasicSettingsSection config={config} updateConfig={updateConfig} />

      {/* 高级指纹设置 */}
      <AdvancedFingerprintSection config={config} updateConfig={updateConfig} />

      {/* 账号关联 */}
      <AccountSection config={config} updateConfig={updateConfig} />

      <Card>
        <div className="p-6">
          <h3 className="text-base font-semibold text-[var(--color-text-primary)] mb-4">归类</h3>
          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-[var(--color-text-secondary)] mb-2">
                分组
              </label>
              <select
                value={selectedGroupId}
                onChange={(event) => onGroupChange(event.target.value)}
                className="w-full px-3 py-2 bg-[var(--color-bg-input)] border border-[var(--color-border-default)] rounded-lg text-[var(--color-text-primary)] focus:outline-none focus:ring-2 focus:ring-[var(--color-primary)] focus:border-transparent"
              >
                <option value="">不分组</option>
                {groups.map(group => (
                  <option key={group.groupId} value={group.groupId}>
                    {group.groupName}{group.instanceCount ? `（${group.instanceCount}）` : ''}
                  </option>
                ))}
              </select>
            </div>

            <div>
              <label className="block text-sm font-medium text-[var(--color-text-secondary)] mb-2">
                配置标签（用于分类和搜索）
              </label>
              <input
                type="text"
                list="browser-create-v2-tags"
                placeholder="输入标签，用逗号分隔"
                value={config.preferences.tags.join(', ')}
                onChange={(e) => {
                  const tags = e.target.value.split(',').map(t => t.trim()).filter(Boolean)
                  updateConfig(prev => ({
                    ...prev,
                    preferences: { ...prev.preferences, tags }
                  }))
                }}
                className="w-full px-3 py-2 bg-[var(--color-bg-input)] border border-[var(--color-border-default)] rounded-lg text-[var(--color-text-primary)] focus:outline-none focus:ring-2 focus:ring-[var(--color-primary)] focus:border-transparent"
              />
              <datalist id="browser-create-v2-tags">
                {allTags.map(tag => (
                  <option key={tag} value={tag} />
                ))}
              </datalist>
            </div>

            <div>
              <label className="block text-sm font-medium text-[var(--color-text-secondary)] mb-2">
                启动关键字（用于快速启动 API 和列表检索）
              </label>
              <input
                type="text"
                placeholder="例如 buyer-001, amazon, checkout"
                value={config.preferences.keywords.join(', ')}
                onChange={(e) => {
                  const keywords = e.target.value.split(',').map(item => item.trim()).filter(Boolean)
                  updateConfig(prev => ({
                    ...prev,
                    preferences: { ...prev.preferences, keywords }
                  }))
                }}
                className="w-full px-3 py-2 bg-[var(--color-bg-input)] border border-[var(--color-border-default)] rounded-lg text-[var(--color-text-primary)] focus:outline-none focus:ring-2 focus:ring-[var(--color-primary)] focus:border-transparent"
              />
              <p className="mt-2 text-xs text-[var(--color-text-muted)]">
                关键字会随实例保存，可被列表页关键字搜索和自动化启动接口命中。
              </p>
            </div>

            <div>
              <label className="block text-sm font-medium text-[var(--color-text-secondary)] mb-2">
                启动码（用于自动化直接启动）
              </label>
              <input
                type="text"
                placeholder="例如 BUYER_001"
                maxLength={32}
                value={config.preferences.launchCode}
                onChange={(e) => {
                  updateConfig(prev => ({
                    ...prev,
                    preferences: { ...prev.preferences, launchCode: e.target.value.toUpperCase() }
                  }))
                }}
                className={`w-full px-3 py-2 bg-[var(--color-bg-input)] border rounded-lg text-[var(--color-text-primary)] placeholder:text-[var(--color-text-muted)] focus:outline-none focus:ring-2 focus:ring-[var(--color-primary)] focus:border-transparent ${
                  launchCodeError ? 'border-[var(--color-error)]' : 'border-[var(--color-border-default)]'
                }`}
              />
              {launchCodeError && (
                <p className="mt-2 text-xs text-[var(--color-error)]">{launchCodeError}</p>
              )}
              <p className="mt-2 text-xs text-[var(--color-text-muted)]">
                留空时系统自动生成；填写后可通过启动 API 或启动码入口直接打开该实例。
              </p>
            </div>

            <div>
              <div className="flex items-center justify-between gap-3 mb-2">
                <label className="block text-sm font-medium text-[var(--color-text-secondary)]">
                  启动扩展
                </label>
                {loadableExtensions.length > 0 && (
                  <span className="text-xs text-[var(--color-text-muted)]">
                    已选 {selectedExtensionIds.filter(id => loadableExtensions.some(ext => ext.extensionId === id)).length} 个
                  </span>
                )}
              </div>

              {loadableExtensions.length > 0 ? (
                <div className="space-y-2 max-h-64 overflow-y-auto pr-1">
                  {loadableExtensions.map(extension => {
                    const checked = selectedExtensionIds.includes(extension.extensionId)
                    return (
                      <label
                        key={extension.extensionId}
                        className={`flex items-start gap-3 p-3 rounded-lg border cursor-pointer transition-colors ${
                          checked
                            ? 'bg-[rgba(59,130,246,0.12)] border-[#1890ff]'
                            : 'bg-[var(--color-bg-elevated)] border-[var(--color-border-default)] hover:border-[var(--color-border-strong)]'
                        }`}
                      >
                        <input
                          type="checkbox"
                          checked={checked}
                          onChange={() => handleExtensionToggle(extension.extensionId)}
                          className="mt-1 w-4 h-4 accent-[#1890ff]"
                        />
                        <span className="min-w-0 flex-1">
                          <span className="block text-sm font-medium text-[var(--color-text-primary)] truncate">
                            {extension.extensionName || '未命名扩展'}
                          </span>
                          <span className="block text-xs text-[var(--color-text-muted)] truncate">
                            {extension.version ? `v${extension.version} · ` : ''}{extension.extensionPath}
                          </span>
                        </span>
                      </label>
                    )
                  })}
                </div>
              ) : (
                <div className="p-3 rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-elevated)] text-sm text-[var(--color-text-muted)]">
                  暂无可加载的本地启用扩展
                </div>
              )}
              <p className="mt-2 text-xs text-[var(--color-text-muted)]">
                保存实例后会自动绑定到所选本地扩展，启动时由后端加载。
              </p>
            </div>
          </div>
        </div>
      </Card>
    </div>
  )
}
