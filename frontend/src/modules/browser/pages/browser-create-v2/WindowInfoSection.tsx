import { Apple, Chrome, Monitor, Smartphone, Terminal } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { Card, Input } from '../../../../shared/components'
import type { FingerprintProfileConfig, Platform, BrowserCore } from './types'
import type { ValidationError } from './validation'
import { getFieldError } from './validation'
import type { BrowserCore as BrowserCoreInfo } from '../../types'

interface WindowInfoSectionProps {
  config: FingerprintProfileConfig
  updateConfig: (updater: (prev: FingerprintProfileConfig) => FingerprintProfileConfig) => void
  errors: ValidationError[]
  cores: BrowserCoreInfo[]
  selectedCoreId: string
  onCoreChange: (coreId: string) => void
}

export function WindowInfoSection({
  config,
  updateConfig,
  errors,
  cores,
  selectedCoreId,
  onCoreChange,
}: WindowInfoSectionProps) {
  const platforms: { value: Platform; label: string; Icon: LucideIcon }[] = [
    { value: 'windows', label: 'Windows', Icon: Monitor },
    { value: 'mac', label: 'macOS', Icon: Apple },
    { value: 'linux', label: 'Linux', Icon: Terminal },
    { value: 'android', label: 'Android', Icon: Smartphone },
  ]

  const browsers: { value: BrowserCore; label: string; disabled?: boolean }[] = [
    { value: 'chrome', label: 'Chrome' },
    { value: 'firefox', label: 'Firefox', disabled: true },
  ]

  const profileNameError = getFieldError(errors, 'profileName')
  const userAgentError = getFieldError(errors, 'userAgent.value')
  const defaultCore = cores.find(core => core.isDefault)

  return (
    <Card>
      <div className="p-6">
        <h3 className="text-base font-semibold text-[var(--color-text-primary)] mb-4">窗口信息</h3>

        <div className="space-y-4">
          {/* 配置名称 */}
          <div>
            <label className="block text-sm font-medium text-[var(--color-text-secondary)] mb-2">
              配置名称 <span className="text-[var(--color-error)]">*</span>
            </label>
            <Input
              value={config.profileName}
              onChange={(e) => {
                updateConfig(prev => ({ ...prev, profileName: e.target.value }))
              }}
              placeholder="请输入配置名称"
              className={`w-full ${profileNameError ? 'border-[var(--color-error)]' : ''}`}
            />
            {profileNameError && (
              <p className="mt-1 text-sm text-[var(--color-error)]">{profileNameError}</p>
            )}
          </div>

          {/* 系统平台 */}
          <div>
            <label className="block text-sm font-medium text-[var(--color-text-secondary)] mb-2">
              系统
            </label>
            <div className="grid grid-cols-4 gap-3">
              {platforms.map(platform => {
                const Icon = platform.Icon
                return (
                  <button
                    key={platform.value}
                    type="button"
                    onClick={() => {
                      updateConfig(prev => ({ ...prev, platform: platform.value }))
                    }}
                    className={`
                      flex flex-col items-center gap-2 p-3 rounded-lg border-2 transition-all
                      ${config.platform === platform.value
                        ? 'border-[#1890ff] bg-[rgba(59,130,246,0.12)]'
                        : 'border-[var(--color-border-default)] hover:border-[var(--color-border-strong)] hover:bg-[var(--color-bg-muted)]'
                      }
                    `}
                  >
                    <Icon className={`w-5 h-5 ${
                      config.platform === platform.value ? 'text-[#1890ff]' : 'text-[var(--color-text-muted)]'
                    }`} />
                    <span className={`text-xs font-medium ${
                      config.platform === platform.value ? 'text-[#1890ff]' : 'text-[var(--color-text-secondary)]'
                    }`}>
                      {platform.label}
                    </span>
                  </button>
                )
              })}
            </div>
          </div>

          {/* 浏览器内核 */}
          <div>
            <label className="block text-sm font-medium text-[var(--color-text-secondary)] mb-2">
              浏览器品牌
            </label>
            <div className="flex gap-3">
              {browsers.map(browser => (
                <button
                  key={browser.value}
                  type="button"
                  disabled={browser.disabled}
                  onClick={() => {
                    if (!browser.disabled) {
                      updateConfig(prev => ({ ...prev, browserCore: browser.value }))
                    }
                  }}
                  className={`
                    flex items-center gap-2 px-4 py-2 rounded-lg border-2 transition-all
                    ${browser.disabled
                      ? 'opacity-50 cursor-not-allowed border-[var(--color-border-default)] bg-[var(--color-bg-muted)]'
                      : config.browserCore === browser.value
                        ? 'border-[#1890ff] bg-[rgba(59,130,246,0.12)]'
                        : 'border-[var(--color-border-default)] hover:border-[var(--color-border-strong)]'
                    }
                  `}
                >
                  <Chrome className={`w-4 h-4 ${
                    config.browserCore === browser.value ? 'text-[#1890ff]' : 'text-[var(--color-text-muted)]'
                  }`} />
                  <span className={`text-sm font-medium ${
                    config.browserCore === browser.value ? 'text-[#1890ff]' : 'text-[var(--color-text-secondary)]'
                  }`}>
                    {browser.label}
                    {browser.disabled && ' (待支持)'}
                  </span>
                </button>
              ))}
            </div>
          </div>

          {/* 实际浏览器内核 */}
          <div>
            <label className="block text-sm font-medium text-[var(--color-text-secondary)] mb-2">
              启动内核
            </label>
            <select
              value={selectedCoreId}
              onChange={(event) => onCoreChange(event.target.value)}
              className="w-full px-3 py-2 bg-[var(--color-bg-input)] border border-[var(--color-border-default)] rounded-lg text-[var(--color-text-primary)] focus:outline-none focus:ring-2 focus:ring-[var(--color-primary)] focus:border-transparent"
            >
              <option value="">
                {defaultCore ? `使用默认内核（${defaultCore.coreName}）` : '使用默认内核'}
              </option>
              {cores.map(core => (
                <option key={core.coreId} value={core.coreId}>
                  {core.coreName}{core.isDefault ? '（默认）' : ''}
                </option>
              ))}
            </select>
          </div>

          {/* User-Agent */}
          <div>
            <label className="block text-sm font-medium text-[var(--color-text-secondary)] mb-2">
              User-Agent
            </label>
            <div className="space-y-2">
              <div className="flex gap-2">
                {['real', 'random', 'custom'].map(type => (
                  <button
                    key={type}
                    type="button"
                    onClick={() => {
                      updateConfig(prev => ({
                        ...prev,
                        userAgent: { ...prev.userAgent, type: type as any }
                      }))
                    }}
                    className={`
                      px-4 py-1.5 text-sm font-medium rounded-lg border-2 transition-all
                      ${config.userAgent.type === type
                        ? 'border-[#1890ff] bg-[rgba(59,130,246,0.12)] text-[#1890ff]'
                        : 'border-[var(--color-border-default)] text-[var(--color-text-secondary)] hover:border-[var(--color-border-strong)]'
                      }
                    `}
                  >
                    {type === 'real' ? '真实' : type === 'random' ? '随机' : '自定义'}
                  </button>
                ))}
              </div>
              {config.userAgent.type === 'custom' && (
                <textarea
                  value={config.userAgent.value}
                  onChange={(e) => {
                    updateConfig(prev => ({
                      ...prev,
                      userAgent: { ...prev.userAgent, value: e.target.value }
                    }))
                  }}
                  placeholder="例如: Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36..."
                  rows={3}
                  className={`w-full px-3 py-2 bg-[var(--color-bg-input)] border rounded-lg text-[var(--color-text-primary)] placeholder:text-[var(--color-text-muted)] focus:outline-none focus:ring-2 focus:ring-[#1890ff] focus:border-transparent resize-none font-mono text-xs ${
                    userAgentError ? 'border-[var(--color-error)]' : 'border-[var(--color-border-strong)]'
                  }`}
                />
              )}
              {userAgentError && (
                <p className="text-sm text-[var(--color-error)]">{userAgentError}</p>
              )}
            </div>
          </div>
        </div>
      </div>
    </Card>
  )
}
