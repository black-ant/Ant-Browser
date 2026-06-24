import { useState, type ReactNode } from 'react'
import { ChevronDown, ChevronUp } from 'lucide-react'
import { Card } from '../../../../shared/components'
import { findMatchingDeviceProfile } from '../../../../services/deviceProfiles'
import type {
  FingerprintProfileConfig,
  FingerprintValueType,
  SearchEngineId,
  WebGPUMode,
  WebRTCPolicy,
} from './types'

interface AdvancedFingerprintSectionProps {
  config: FingerprintProfileConfig
  updateConfig: (updater: (prev: FingerprintProfileConfig) => FingerprintProfileConfig) => void
}

type Option<T extends string> = {
  value: T
  label: string
}

const realRandomOptions: Option<Extract<FingerprintValueType, 'real' | 'random'>>[] = [
  { value: 'random', label: '随机' },
  { value: 'real', label: '真实' },
]

const valueTypeOptions: Option<FingerprintValueType>[] = [
  { value: 'random', label: '随机' },
  { value: 'real', label: '真实' },
  { value: 'custom', label: '自定义' },
]

const booleanOptions: Option<'on' | 'off'>[] = [
  { value: 'on', label: '开启' },
  { value: 'off', label: '关闭' },
]

const webrtcOptions: Option<WebRTCPolicy>[] = [
  { value: 'disable_non_proxied_udp', label: '替换' },
  { value: 'default', label: '真实' },
  { value: 'disable_all', label: '禁止' },
]

const webgpuOptions: Option<WebGPUMode>[] = [
  { value: 'match_webgl', label: '跟随 WebGL' },
  { value: 'real', label: '真实' },
  { value: 'disable', label: '禁用' },
]

const searchEngineOptions: Option<SearchEngineId>[] = [
  { value: 'google', label: 'Google' },
  { value: 'bing', label: 'Bing' },
  { value: 'duckduckgo', label: 'DuckDuckGo' },
  { value: 'baidu', label: '百度' },
]

const concurrencyOptions = [2, 4, 6, 8, 10, 12, 16, 24, 32]
const memoryOptions = [2, 4, 8, 16, 32, 64]
const touchPointOptions = [0, 1, 2, 5, 10]

export function AdvancedFingerprintSection({ config, updateConfig }: AdvancedFingerprintSectionProps) {
  const [expanded, setExpanded] = useState(false)
  const matchedDeviceProfile = findMatchingDeviceProfile(config)

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

  const SettingGroup = ({
    title,
    children,
  }: {
    title: string
    children: ReactNode
  }) => (
    <section className="rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-elevated)] overflow-hidden">
      <div className="px-5 py-3 border-b border-[var(--color-border-muted)] bg-[var(--color-bg-muted)]/55">
        <h4 className="text-sm font-semibold text-[var(--color-text-primary)]">{title}</h4>
      </div>
      <div className="px-5">{children}</div>
    </section>
  )

  const SettingRow = ({
    label,
    note,
    children,
  }: {
    label: string
    note?: string
    children: ReactNode
  }) => (
    <div className="browser-basic-row grid grid-cols-1 lg:grid-cols-[150px_minmax(0,1fr)] gap-3 lg:gap-5 py-4 border-b border-[var(--color-border-muted)] last:border-b-0">
      <div className="text-sm font-medium text-[var(--color-text-secondary)] pt-1">{label}</div>
      <div className="min-w-0 space-y-3">
        {children}
        {note && <p className="text-xs text-[var(--color-text-muted)] leading-relaxed">{note}</p>}
      </div>
    </div>
  )

  const FieldLabel = ({ label, children }: { label: string; children: ReactNode }) => (
    <label className="block min-w-0">
      <span className="block text-xs text-[var(--color-text-muted)] mb-1">{label}</span>
      {children}
    </label>
  )

  const fieldClass = 'w-full px-3 py-2 bg-[var(--color-bg-input)] border border-[var(--color-border-strong)] rounded-lg text-sm text-[var(--color-text-primary)] placeholder:text-[var(--color-text-muted)] focus:outline-none focus:ring-2 focus:ring-[#1890ff] focus:border-transparent'

  return (
    <Card className="browser-create-accordion-card" padding="none">
      <button
        onClick={() => setExpanded(!expanded)}
        className="browser-create-accordion-toggle w-full p-6 flex items-center justify-between hover:bg-[var(--color-bg-muted)] transition-colors"
      >
        <h3 className="text-base font-semibold text-[var(--color-text-primary)]">高级指纹设置</h3>
        {expanded ? (
          <ChevronUp className="w-5 h-5 text-[var(--color-text-muted)]" />
        ) : (
          <ChevronDown className="w-5 h-5 text-[var(--color-text-muted)]" />
        )}
      </button>

      {expanded && (
        <div className="px-6 pb-6 border-t border-[var(--color-border-muted)]">
          <div className="mt-5 space-y-4">
            <SettingGroup title="网络与渲染">
              <SettingRow
                label="WebRTC"
                note="替换会写入禁止非代理 UDP 策略；禁止会完全关闭 WebRTC IP 通道。"
              >
                <Segment
                  options={webrtcOptions}
                  value={config.advanced.webrtc.policy}
                  onChange={(value) => {
                    updateConfig(prev => ({
                      ...prev,
                      advanced: {
                        ...prev.advanced,
                        webrtc: { ...prev.advanced.webrtc, policy: value },
                      },
                    }))
                  }}
                />
              </SettingRow>

              <SettingRow label="WebGL 图像">
                <Segment
                  options={realRandomOptions}
                  value={config.advanced.webgl.image.type as 'real' | 'random'}
                  onChange={(value) => {
                    updateConfig(prev => ({
                      ...prev,
                      advanced: {
                        ...prev.advanced,
                        webgl: {
                          ...prev.advanced.webgl,
                          image: { type: value },
                        },
                      },
                    }))
                  }}
                />
              </SettingRow>

              <SettingRow
                label="WebGL 信息"
                note="当前内核使用统一 GPU 混淆开关，Chrome 144+ 的自定义 Vendor / Renderer 参数已不再可靠。"
              >
                <Segment
                  options={realRandomOptions}
                  value={config.advanced.webgl.vendor.type as 'real' | 'random'}
                  onChange={(value) => {
                    updateConfig(prev => ({
                      ...prev,
                      advanced: {
                        ...prev.advanced,
                        webgl: {
                          ...prev.advanced.webgl,
                          vendor: { ...prev.advanced.webgl.vendor, type: value },
                          renderer: { ...prev.advanced.webgl.renderer, type: value },
                        },
                      },
                    }))
                  }}
                />
                <div className="grid grid-cols-1 gap-3">
                  <FieldLabel label="Vendor">
                    <input
                      value={matchedDeviceProfile?.gpuVendor || '自动匹配设备画像'}
                      readOnly
                      className={`${fieldClass} opacity-80`}
                    />
                  </FieldLabel>
                  <FieldLabel label="Renderer">
                    <textarea
                      value={matchedDeviceProfile?.gpuRenderer || '自动匹配设备画像'}
                      readOnly
                      rows={2}
                      className={`${fieldClass} resize-none opacity-80`}
                    />
                  </FieldLabel>
                </div>
              </SettingRow>

              <SettingRow label="WebGPU">
                <Segment
                  options={webgpuOptions}
                  value={config.advanced.webgpu.mode}
                  onChange={(value) => {
                    updateConfig(prev => ({
                      ...prev,
                      advanced: {
                        ...prev.advanced,
                        webgpu: { mode: value },
                      },
                    }))
                  }}
                />
              </SettingRow>

              <SettingRow label="Canvas">
                <Segment
                  options={realRandomOptions}
                  value={config.advanced.canvas.type as 'real' | 'random'}
                  onChange={(value) => {
                    updateConfig(prev => ({
                      ...prev,
                      advanced: { ...prev.advanced, canvas: { type: value } },
                    }))
                  }}
                />
              </SettingRow>

              <SettingRow label="AudioContext">
                <Segment
                  options={realRandomOptions}
                  value={config.advanced.audioContext.type as 'real' | 'random'}
                  onChange={(value) => {
                    updateConfig(prev => ({
                      ...prev,
                      advanced: { ...prev.advanced, audioContext: { type: value } },
                    }))
                  }}
                />
              </SettingRow>

              <SettingRow label="Client Rects">
                <Segment
                  options={realRandomOptions}
                  value={config.advanced.clientRects.type as 'real' | 'random'}
                  onChange={(value) => {
                    updateConfig(prev => ({
                      ...prev,
                      advanced: { ...prev.advanced, clientRects: { type: value } },
                    }))
                  }}
                />
              </SettingRow>

              <SettingRow label="Do Not Track">
                <Segment
                  options={booleanOptions}
                  value={config.advanced.doNotTrack ? 'on' : 'off'}
                  onChange={(value) => {
                    updateConfig(prev => ({
                      ...prev,
                      advanced: { ...prev.advanced, doNotTrack: value === 'on' },
                    }))
                  }}
                />
              </SettingRow>
            </SettingGroup>

            <SettingGroup title="硬件能力">
              <SettingRow label="字体指纹">
                <Segment
                  options={realRandomOptions}
                  value={config.basic.fonts.type === 'real' ? 'real' : 'random'}
                  onChange={(value) => {
                    updateConfig(prev => ({
                      ...prev,
                      basic: {
                        ...prev.basic,
                        fonts: { ...prev.basic.fonts, type: value },
                      },
                    }))
                  }}
                />
              </SettingRow>

              <SettingRow label="媒体设备">
                <Segment
                  options={valueTypeOptions}
                  value={config.advanced.mediaDevices.type}
                  onChange={(value) => {
                    updateConfig(prev => ({
                      ...prev,
                      advanced: {
                        ...prev.advanced,
                        mediaDevices: { ...prev.advanced.mediaDevices, type: value },
                      },
                    }))
                  }}
                />
                {config.advanced.mediaDevices.type === 'custom' && (
                  <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
                    <FieldLabel label="摄像头">
                      <input
                        type="number"
                        min="0"
                        value={config.advanced.mediaDevices.videoInputs}
                        onChange={(event) => {
                          updateConfig(prev => ({
                            ...prev,
                            advanced: {
                              ...prev.advanced,
                              mediaDevices: {
                                ...prev.advanced.mediaDevices,
                                videoInputs: Number(event.target.value) || 0,
                              },
                            },
                          }))
                        }}
                        className={fieldClass}
                      />
                    </FieldLabel>
                    <FieldLabel label="麦克风">
                      <input
                        type="number"
                        min="0"
                        value={config.advanced.mediaDevices.audioInputs}
                        onChange={(event) => {
                          updateConfig(prev => ({
                            ...prev,
                            advanced: {
                              ...prev.advanced,
                              mediaDevices: {
                                ...prev.advanced.mediaDevices,
                                audioInputs: Number(event.target.value) || 0,
                              },
                            },
                          }))
                        }}
                        className={fieldClass}
                      />
                    </FieldLabel>
                    <FieldLabel label="扬声器">
                      <input
                        type="number"
                        min="0"
                        value={config.advanced.mediaDevices.audioOutputs}
                        onChange={(event) => {
                          updateConfig(prev => ({
                            ...prev,
                            advanced: {
                              ...prev.advanced,
                              mediaDevices: {
                                ...prev.advanced.mediaDevices,
                                audioOutputs: Number(event.target.value) || 0,
                              },
                            },
                          }))
                        }}
                        className={fieldClass}
                      />
                    </FieldLabel>
                  </div>
                )}
              </SettingRow>

              <SettingRow label="硬件并发数">
                <Segment
                  options={valueTypeOptions}
                  value={config.advanced.hardwareConcurrency.type}
                  onChange={(value) => {
                    updateConfig(prev => ({
                      ...prev,
                      advanced: {
                        ...prev.advanced,
                        hardwareConcurrency: { ...prev.advanced.hardwareConcurrency, type: value },
                      },
                    }))
                  }}
                />
                {config.advanced.hardwareConcurrency.type === 'custom' && (
                  <select
                    value={String(config.advanced.hardwareConcurrency.value)}
                    onChange={(event) => {
                      updateConfig(prev => ({
                        ...prev,
                        advanced: {
                          ...prev.advanced,
                          hardwareConcurrency: {
                            ...prev.advanced.hardwareConcurrency,
                            value: Number(event.target.value),
                          },
                        },
                      }))
                    }}
                    className={`${fieldClass} max-w-sm`}
                  >
                    {concurrencyOptions.map(value => (
                      <option key={value} value={value}>{value} 核</option>
                    ))}
                  </select>
                )}
              </SettingRow>

              <SettingRow label="设备内存">
                <Segment
                  options={valueTypeOptions}
                  value={config.advanced.deviceMemory.type}
                  onChange={(value) => {
                    updateConfig(prev => ({
                      ...prev,
                      advanced: {
                        ...prev.advanced,
                        deviceMemory: { ...prev.advanced.deviceMemory, type: value },
                      },
                    }))
                  }}
                />
                {config.advanced.deviceMemory.type === 'custom' && (
                  <select
                    value={String(config.advanced.deviceMemory.value)}
                    onChange={(event) => {
                      updateConfig(prev => ({
                        ...prev,
                        advanced: {
                          ...prev.advanced,
                          deviceMemory: {
                            ...prev.advanced.deviceMemory,
                            value: Number(event.target.value),
                          },
                        },
                      }))
                    }}
                    className={`${fieldClass} max-w-sm`}
                  >
                    {memoryOptions.map(value => (
                      <option key={value} value={value}>{value} GB</option>
                    ))}
                  </select>
                )}
              </SettingRow>

              <SettingRow label="触摸点数">
                <Segment
                  options={valueTypeOptions}
                  value={config.advanced.touchPoints.type}
                  onChange={(value) => {
                    updateConfig(prev => ({
                      ...prev,
                      advanced: {
                        ...prev.advanced,
                        touchPoints: { ...prev.advanced.touchPoints, type: value },
                      },
                    }))
                  }}
                />
                {config.advanced.touchPoints.type === 'custom' && (
                  <select
                    value={String(config.advanced.touchPoints.value)}
                    onChange={(event) => {
                      updateConfig(prev => ({
                        ...prev,
                        advanced: {
                          ...prev.advanced,
                          touchPoints: {
                            ...prev.advanced.touchPoints,
                            value: Number(event.target.value),
                          },
                        },
                      }))
                    }}
                    className={`${fieldClass} max-w-sm`}
                  >
                    {touchPointOptions.map(value => (
                      <option key={value} value={value}>
                        {value === 0 ? '0（桌面）' : `${value} 点`}
                      </option>
                    ))}
                  </select>
                )}
              </SettingRow>
            </SettingGroup>

            <SettingGroup title="浏览器启动">
              <SettingRow label="搜索引擎" note="写入 Chrome 默认搜索引擎配置，影响地址栏关键词搜索。">
                <Segment
                  options={searchEngineOptions}
                  value={config.basic.searchEngine}
                  onChange={(value) => {
                    updateConfig(prev => ({
                      ...prev,
                      basic: { ...prev.basic, searchEngine: value },
                    }))
                  }}
                />
              </SettingRow>

              <SettingRow label="硬件加速">
                <Segment
                  options={booleanOptions}
                  value={config.advanced.hardwareAcceleration ? 'on' : 'off'}
                  onChange={(value) => {
                    updateConfig(prev => ({
                      ...prev,
                      advanced: { ...prev.advanced, hardwareAcceleration: value === 'on' },
                    }))
                  }}
                />
              </SettingRow>

              <SettingRow label="禁用沙盒" note="关闭沙盒会写入 --no-sandbox，仅在你明确需要兼容特殊环境时使用。">
                <Segment
                  options={booleanOptions}
                  value={config.advanced.disableSandbox ? 'on' : 'off'}
                  onChange={(value) => {
                    updateConfig(prev => ({
                      ...prev,
                      advanced: { ...prev.advanced, disableSandbox: value === 'on' },
                    }))
                  }}
                />
              </SettingRow>

              <SettingRow label="启动参数" note="每行一个参数；系统接管的调试端口、代理和用户数据目录仍会由后端过滤。">
                <textarea
                  value={config.advanced.startupArgs}
                  onChange={(event) => {
                    updateConfig(prev => ({
                      ...prev,
                      advanced: {
                        ...prev.advanced,
                        startupArgs: event.target.value,
                      },
                    }))
                  }}
                  rows={4}
                  placeholder="--disable-background-networking"
                  className={`${fieldClass} resize-y`}
                />
              </SettingRow>
            </SettingGroup>
          </div>
        </div>
      )}
    </Card>
  )
}
