import { useState } from 'react'
import { ChevronDown, ChevronUp } from 'lucide-react'
import { Card } from '../../../../shared/components'
import type {
  BrowserLanguageMode,
  FingerprintProfileConfig,
  FingerprintValueType,
  GeolocationPrompt,
  WindowMode,
} from './types'

interface BasicSettingsSectionProps {
  config: FingerprintProfileConfig
  updateConfig: (updater: (prev: FingerprintProfileConfig) => FingerprintProfileConfig) => void
}

type Option<T extends string> = {
  value: T
  label: string
}

const languageOptions = [
  { value: 'zh-CN', label: '简体中文' },
  { value: 'en-US', label: 'English (US)' },
  { value: 'en-GB', label: 'English (UK)' },
  { value: 'ja-JP', label: '日本語' },
  { value: 'ko-KR', label: '한국어' },
]

const acceptLanguageOptions = [
  { value: 'zh-CN,zh', label: '简体中文' },
  { value: 'en-US,en', label: 'English (US)' },
  { value: 'en-GB,en', label: 'English (UK)' },
  { value: 'ja-JP,ja', label: '日本語' },
  { value: 'ko-KR,ko', label: '한국어' },
]

const timezoneOptions = [
  { value: 'Asia/Shanghai', label: '北京时间 (Asia/Shanghai)' },
  { value: 'America/New_York', label: '纽约时间 (America/New_York)' },
  { value: 'America/Los_Angeles', label: '洛杉矶时间 (America/Los_Angeles)' },
  { value: 'Europe/London', label: '伦敦时间 (Europe/London)' },
  { value: 'Asia/Tokyo', label: '东京时间 (Asia/Tokyo)' },
]

const resolutionOptions = [
  { width: 1920, height: 1080, label: '1920 x 1080' },
  { width: 1366, height: 768, label: '1366 x 768' },
  { width: 1440, height: 900, label: '1440 x 900' },
  { width: 2560, height: 1440, label: '2560 x 1440' },
  { width: 3840, height: 2160, label: '3840 x 2160' },
]

const ipCustomOptions: Option<BrowserLanguageMode>[] = [
  { value: 'auto', label: '基于 IP 匹配' },
  { value: 'custom', label: '自定义' },
]

const geolocationPromptOptions: Option<GeolocationPrompt>[] = [
  { value: 'prompt', label: '询问' },
  { value: 'allow', label: '允许' },
  { value: 'block', label: '禁止' },
]

const windowModeOptions: Option<WindowMode>[] = [
  { value: 'custom', label: '自定义' },
  { value: 'fullscreen', label: '全屏' },
]

const booleanOptions = [
  { value: 'on', label: '开启' },
  { value: 'off', label: '关闭' },
]

export function BasicSettingsSection({ config, updateConfig }: BasicSettingsSectionProps) {
  const [expanded, setExpanded] = useState(false)

  const Segment = <T extends string>({
    options,
    value,
    onChange,
  }: {
    options: Option<T>[]
    value: T
    onChange: (value: T) => void
  }) => (
    <div className="browser-basic-segment inline-flex min-w-0 rounded-lg border border-[var(--color-border-strong)] overflow-hidden">
      {options.map((option, index) => (
        <button
          key={option.value}
          type="button"
          onClick={() => onChange(option.value)}
          className={`
            px-4 py-1.5 text-sm font-medium transition-all whitespace-nowrap
            ${value === option.value
              ? 'bg-[#1890ff] text-white'
              : 'bg-[var(--color-bg-elevated)] text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-muted)]'
            }
            ${index > 0 ? 'border-l border-[var(--color-border-strong)]' : ''}
          `}
        >
          {option.label}
        </button>
      ))}
    </div>
  )

  const SettingRow = ({
    label,
    children,
  }: {
    label: string
    children: React.ReactNode
  }) => (
    <div className="browser-basic-row grid grid-cols-1 lg:grid-cols-[150px_minmax(0,1fr)] gap-3 lg:gap-5 py-4 border-b border-[var(--color-border-muted)] last:border-b-0">
      <div className="text-sm font-medium text-[var(--color-text-secondary)] pt-1">{label}</div>
      <div className="min-w-0 space-y-3">{children}</div>
    </div>
  )

  const languageMode = config.basic.language === 'auto' ? 'auto' : 'custom'
  const geolocationMode: BrowserLanguageMode = config.basic.geolocation.type === 'custom' ? 'custom' : 'auto'
  const timezoneMode: BrowserLanguageMode = config.basic.timezone.type === 'custom' ? 'custom' : 'auto'

  return (
    <Card className="browser-create-accordion-card" padding="none">
      <button
        onClick={() => setExpanded(!expanded)}
        className="browser-create-accordion-toggle w-full p-6 flex items-center justify-between hover:bg-[var(--color-bg-muted)] transition-colors"
      >
        <h3 className="text-base font-semibold text-[var(--color-text-primary)]">基础设置</h3>
        {expanded ? (
          <ChevronUp className="w-5 h-5 text-[var(--color-text-muted)]" />
        ) : (
          <ChevronDown className="w-5 h-5 text-[var(--color-text-muted)]" />
        )}
      </button>

      {expanded && (
        <div className="px-6 pb-6 border-t border-[var(--color-border-muted)]">
          <div className="browser-basic-panel mt-5 rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-elevated)] px-5">
            <SettingRow label="语言">
              <Segment
                options={ipCustomOptions}
                value={languageMode}
                onChange={(value) => {
                  updateConfig(prev => ({
                    ...prev,
                    basic: {
                      ...prev.basic,
                      language: value === 'auto' ? 'auto' : (prev.basic.language === 'auto' ? 'en-US' : prev.basic.language),
                    },
                  }))
                }}
              />
              {languageMode === 'custom' && (
                <select
                  value={config.basic.language}
                  onChange={(event) => {
                    updateConfig(prev => ({
                      ...prev,
                      basic: { ...prev.basic, language: event.target.value },
                    }))
                  }}
                  className="w-full max-w-sm px-3 py-2 bg-[var(--color-bg-input)] border border-[var(--color-border-strong)] rounded-lg text-[var(--color-text-primary)] focus:outline-none focus:ring-2 focus:ring-[#1890ff] focus:border-transparent"
                >
                  {languageOptions.map(option => (
                    <option key={option.value} value={option.value}>{option.label}</option>
                  ))}
                </select>
              )}
            </SettingRow>

            <SettingRow label="界面语言">
              <Segment
                options={ipCustomOptions}
                value={config.basic.uiLanguage.mode}
                onChange={(value) => {
                  updateConfig(prev => ({
                    ...prev,
                    basic: {
                      ...prev.basic,
                      uiLanguage: {
                        ...prev.basic.uiLanguage,
                        mode: value,
                      },
                    },
                  }))
                }}
              />
              {config.basic.uiLanguage.mode === 'custom' && (
                <select
                  value={config.basic.uiLanguage.value}
                  onChange={(event) => {
                    updateConfig(prev => ({
                      ...prev,
                      basic: {
                        ...prev.basic,
                        uiLanguage: {
                          ...prev.basic.uiLanguage,
                          value: event.target.value,
                        },
                      },
                    }))
                  }}
                  className="w-full max-w-sm px-3 py-2 bg-[var(--color-bg-input)] border border-[var(--color-border-strong)] rounded-lg text-[var(--color-text-primary)] focus:outline-none focus:ring-2 focus:ring-[#1890ff] focus:border-transparent"
                >
                  {acceptLanguageOptions.map(option => (
                    <option key={option.value} value={option.value}>{option.label}</option>
                  ))}
                </select>
              )}
            </SettingRow>

            <SettingRow label="时区">
              <Segment
                options={ipCustomOptions}
                value={timezoneMode}
                onChange={(value) => {
                  updateConfig(prev => ({
                    ...prev,
                    basic: {
                      ...prev.basic,
                      timezone: {
                        ...prev.basic.timezone,
                        type: value === 'auto' ? 'random' : 'custom',
                      },
                    },
                  }))
                }}
              />
              {config.basic.timezone.type === 'custom' && (
                <select
                  value={config.basic.timezone.value}
                  onChange={(event) => {
                    updateConfig(prev => ({
                      ...prev,
                      basic: {
                        ...prev.basic,
                        timezone: { ...prev.basic.timezone, value: event.target.value },
                      },
                    }))
                  }}
                  className="w-full max-w-sm px-3 py-2 bg-[var(--color-bg-input)] border border-[var(--color-border-strong)] rounded-lg text-[var(--color-text-primary)] focus:outline-none focus:ring-2 focus:ring-[#1890ff] focus:border-transparent"
                >
                  {timezoneOptions.map(option => (
                    <option key={option.value} value={option.value}>{option.label}</option>
                  ))}
                </select>
              )}
            </SettingRow>

            <SettingRow label="地理位置提示">
              <Segment
                options={geolocationPromptOptions}
                value={config.basic.geolocation.prompt}
                onChange={(value) => {
                  updateConfig(prev => ({
                    ...prev,
                    basic: {
                      ...prev.basic,
                      geolocation: { ...prev.basic.geolocation, prompt: value },
                    },
                  }))
                }}
              />
            </SettingRow>

            <SettingRow label="地理位置">
              <Segment
                options={ipCustomOptions}
                value={geolocationMode}
                onChange={(value) => {
                  updateConfig(prev => ({
                    ...prev,
                    basic: {
                      ...prev.basic,
                      geolocation: {
                        ...prev.basic.geolocation,
                        type: (value === 'custom' ? 'custom' : 'random') as FingerprintValueType,
                      },
                    },
                  }))
                }}
              />
              {config.basic.geolocation.type === 'custom' && (
                <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
                  <label className="block">
                    <span className="block text-xs text-[var(--color-text-muted)] mb-1">纬度</span>
                    <input
                      type="number"
                      step="0.000001"
                      value={config.basic.geolocation.latitude ?? ''}
                      onChange={(event) => {
                        updateConfig(prev => ({
                          ...prev,
                          basic: {
                            ...prev.basic,
                            geolocation: {
                              ...prev.basic.geolocation,
                              latitude: event.target.value ? Number(event.target.value) : undefined,
                            },
                          },
                        }))
                      }}
                      placeholder="39.904200"
                      className="w-full px-3 py-2 bg-[var(--color-bg-input)] border border-[var(--color-border-strong)] rounded-lg text-sm text-[var(--color-text-primary)] placeholder:text-[var(--color-text-muted)] focus:outline-none focus:ring-2 focus:ring-[#1890ff] focus:border-transparent"
                    />
                  </label>
                  <label className="block">
                    <span className="block text-xs text-[var(--color-text-muted)] mb-1">经度</span>
                    <input
                      type="number"
                      step="0.000001"
                      value={config.basic.geolocation.longitude ?? ''}
                      onChange={(event) => {
                        updateConfig(prev => ({
                          ...prev,
                          basic: {
                            ...prev.basic,
                            geolocation: {
                              ...prev.basic.geolocation,
                              longitude: event.target.value ? Number(event.target.value) : undefined,
                            },
                          },
                        }))
                      }}
                      placeholder="116.407396"
                      className="w-full px-3 py-2 bg-[var(--color-bg-input)] border border-[var(--color-border-strong)] rounded-lg text-sm text-[var(--color-text-primary)] placeholder:text-[var(--color-text-muted)] focus:outline-none focus:ring-2 focus:ring-[#1890ff] focus:border-transparent"
                    />
                  </label>
                  <label className="block">
                    <span className="block text-xs text-[var(--color-text-muted)] mb-1">精度</span>
                    <input
                      type="number"
                      min="1"
                      step="1"
                      value={config.basic.geolocation.accuracy ?? ''}
                      onChange={(event) => {
                        updateConfig(prev => ({
                          ...prev,
                          basic: {
                            ...prev.basic,
                            geolocation: {
                              ...prev.basic.geolocation,
                              accuracy: event.target.value ? Number(event.target.value) : undefined,
                            },
                          },
                        }))
                      }}
                      placeholder="100"
                      className="w-full px-3 py-2 bg-[var(--color-bg-input)] border border-[var(--color-border-strong)] rounded-lg text-sm text-[var(--color-text-primary)] placeholder:text-[var(--color-text-muted)] focus:outline-none focus:ring-2 focus:ring-[#1890ff] focus:border-transparent"
                    />
                  </label>
                </div>
              )}
            </SettingRow>

            <SettingRow label="声音">
              <Segment
                options={booleanOptions}
                value={config.basic.content.sound ? 'on' : 'off'}
                onChange={(value) => {
                  updateConfig(prev => ({
                    ...prev,
                    basic: {
                      ...prev.basic,
                      content: { ...prev.basic.content, sound: value === 'on' },
                    },
                  }))
                }}
              />
            </SettingRow>

            <SettingRow label="图片">
              <Segment
                options={booleanOptions}
                value={config.basic.content.images ? 'on' : 'off'}
                onChange={(value) => {
                  updateConfig(prev => ({
                    ...prev,
                    basic: {
                      ...prev.basic,
                      content: { ...prev.basic.content, images: value === 'on' },
                    },
                  }))
                }}
              />
            </SettingRow>

            <SettingRow label="视频">
              <Segment
                options={booleanOptions}
                value={config.basic.content.video ? 'on' : 'off'}
                onChange={(value) => {
                  updateConfig(prev => ({
                    ...prev,
                    basic: {
                      ...prev.basic,
                      content: { ...prev.basic.content, video: value === 'on' },
                    },
                  }))
                }}
              />
              <div className="text-xs text-[var(--color-text-muted)]">
                关闭时阻止自动播放，已加载的视频资源仍可能由网站手动触发。
              </div>
            </SettingRow>

            <SettingRow label="窗口大小">
              <Segment
                options={windowModeOptions}
                value={config.basic.windowMode}
                onChange={(value) => {
                  updateConfig(prev => ({
                    ...prev,
                    basic: {
                      ...prev.basic,
                      windowMode: value,
                      resolution: {
                        ...prev.basic.resolution,
                        type: value === 'custom' ? 'custom' : prev.basic.resolution.type,
                      },
                    },
                  }))
                }}
              />
              {config.basic.windowMode === 'custom' && (
                <select
                  value={`${config.basic.resolution.width}x${config.basic.resolution.height}`}
                  onChange={(event) => {
                    const [width, height] = event.target.value.split('x').map(Number)
                    updateConfig(prev => ({
                      ...prev,
                      basic: {
                        ...prev.basic,
                        resolution: {
                          ...prev.basic.resolution,
                          type: 'custom',
                          width,
                          height,
                        },
                      },
                    }))
                  }}
                  className="w-full max-w-sm px-3 py-2 bg-[var(--color-bg-input)] border border-[var(--color-border-strong)] rounded-lg text-[var(--color-text-primary)] focus:outline-none focus:ring-2 focus:ring-[#1890ff] focus:border-transparent"
                >
                  {resolutionOptions.map(option => (
                    <option key={`${option.width}x${option.height}`} value={`${option.width}x${option.height}`}>
                      {option.label}
                    </option>
                  ))}
                </select>
              )}
            </SettingRow>

            {config.basic.windowMode === 'custom' && (
              <SettingRow label="Width / Height">
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 max-w-sm">
                  <label className="block">
                    <span className="block text-xs text-[var(--color-text-muted)] mb-1">Width</span>
                    <input
                      type="number"
                      min="320"
                      max="7680"
                      value={config.basic.resolution.width}
                      onChange={(event) => {
                        updateConfig(prev => ({
                          ...prev,
                          basic: {
                            ...prev.basic,
                            resolution: {
                              ...prev.basic.resolution,
                              type: 'custom',
                              width: Number(event.target.value) || 0,
                            },
                          },
                        }))
                      }}
                      className="w-full px-3 py-2 bg-[var(--color-bg-input)] border border-[var(--color-border-strong)] rounded-lg text-sm text-[var(--color-text-primary)] focus:outline-none focus:ring-2 focus:ring-[#1890ff] focus:border-transparent"
                    />
                  </label>
                  <label className="block">
                    <span className="block text-xs text-[var(--color-text-muted)] mb-1">Height</span>
                    <input
                      type="number"
                      min="320"
                      max="4320"
                      value={config.basic.resolution.height}
                      onChange={(event) => {
                        updateConfig(prev => ({
                          ...prev,
                          basic: {
                            ...prev.basic,
                            resolution: {
                              ...prev.basic.resolution,
                              type: 'custom',
                              height: Number(event.target.value) || 0,
                            },
                          },
                        }))
                      }}
                      className="w-full px-3 py-2 bg-[var(--color-bg-input)] border border-[var(--color-border-strong)] rounded-lg text-sm text-[var(--color-text-primary)] focus:outline-none focus:ring-2 focus:ring-[#1890ff] focus:border-transparent"
                    />
                  </label>
                </div>
              </SettingRow>
            )}
          </div>
        </div>
      )}
    </Card>
  )
}
