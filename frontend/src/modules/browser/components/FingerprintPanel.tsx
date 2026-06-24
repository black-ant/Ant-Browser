import { useEffect, useState } from 'react'
import { ChevronDown, ChevronUp, RefreshCw, Wand2, Sparkles } from 'lucide-react'
import { ConfirmModal, FormItem, Input, Select, Textarea } from '../../../shared/components'
import {
  type FingerprintConfig,
  FINGERPRINT_PRESETS,
  PRESET_RESOLUTIONS,
  deserialize,
  getSystemTimezone,
  randomFingerprintSeed,
  serialize,
} from '../utils/fingerprintSerializer'
import { generateSmartFingerprint } from '../utils/fingerprintGenerator'

interface FingerprintPanelProps {
  value: string[]
  onChange: (args: string[]) => void
}

const BRAND_OPTIONS = [
  { value: '', label: '不设置' },
  { value: 'Chrome', label: 'Chrome' },
  { value: 'Edge', label: 'Edge' },
  { value: 'Firefox', label: 'Firefox' },
  { value: 'Safari', label: 'Safari' },
]

const PLATFORM_OPTIONS = [
  { value: '', label: '不设置' },
  { value: 'windows', label: 'Windows' },
  { value: 'mac', label: 'macOS' },
  { value: 'linux', label: 'Linux' },
]

const LANG_OPTIONS = [
  { value: '', label: '不设置' },
  { value: 'zh-CN', label: '中文 (zh-CN)' },
  { value: 'en-US', label: 'English (en-US)' },
  { value: 'en-GB', label: 'English (en-GB)' },
  { value: 'ja-JP', label: '日本語 (ja-JP)' },
  { value: 'ko-KR', label: '한국어 (ko-KR)' },
  { value: 'fr-FR', label: 'Français (fr-FR)' },
  { value: 'de-DE', label: 'Deutsch (de-DE)' },
]

const TIMEZONE_OPTIONS = [
  { value: '', label: '不设置' },
  { value: 'system', label: '跟随系统时区' },
  // 亚洲
  { value: 'Asia/Shanghai', label: 'Asia/Shanghai (UTC+8)' },
  { value: 'Asia/Tokyo', label: 'Asia/Tokyo (UTC+9)' },
  { value: 'Asia/Seoul', label: 'Asia/Seoul (UTC+9)' },
  { value: 'Asia/Singapore', label: 'Asia/Singapore (UTC+8)' },
  { value: 'Asia/Hong_Kong', label: 'Asia/Hong_Kong (UTC+8)' },
  { value: 'Asia/Dubai', label: 'Asia/Dubai (UTC+4)' },
  { value: 'Asia/Kolkata', label: 'Asia/Kolkata (UTC+5:30)' },
  // 美洲
  { value: 'America/New_York', label: 'America/New_York (UTC-5)' },
  { value: 'America/Los_Angeles', label: 'America/Los_Angeles (UTC-8)' },
  { value: 'America/Chicago', label: 'America/Chicago (UTC-6)' },
  { value: 'America/Denver', label: 'America/Denver (UTC-7)' },
  { value: 'America/Toronto', label: 'America/Toronto (UTC-5)' },
  { value: 'America/Sao_Paulo', label: 'America/Sao_Paulo (UTC-3)' },
  // 欧洲
  { value: 'Europe/London', label: 'Europe/London (UTC+0)' },
  { value: 'Europe/Paris', label: 'Europe/Paris (UTC+1)' },
  { value: 'Europe/Berlin', label: 'Europe/Berlin (UTC+1)' },
  { value: 'Europe/Moscow', label: 'Europe/Moscow (UTC+3)' },
  // 大洋洲
  { value: 'Australia/Sydney', label: 'Australia/Sydney (UTC+10)' },
  { value: 'Pacific/Auckland', label: 'Pacific/Auckland (UTC+12)' },
]

const RESOLUTION_OPTIONS = [
  { value: '', label: '不设置' },
  ...PRESET_RESOLUTIONS.map(r => ({ value: r, label: r })),
  { value: 'custom', label: '自定义...' },
]

const WEBGL_VENDOR_OPTIONS = [
  { value: '', label: '不设置' },
  { value: 'Intel', label: 'Intel' },
  { value: 'NVIDIA', label: 'NVIDIA' },
  { value: 'AMD', label: 'AMD' },
  { value: 'Apple', label: 'Apple' },
]

const WEBGL_STRATEGY_OPTIONS = [
  { value: 'auto', label: '自动混淆（推荐）' },
  { value: 'real', label: '使用真实硬件' },
  { value: 'custom', label: '自定义（已废弃）' },
]

const SMART_GENERATE_SCENARIO_OPTIONS = [
  { value: 'random', label: '🎲 随机生成' },
  { value: 'office', label: '💼 办公场景（中低配）' },
  { value: 'home', label: '🏠 家用场景（均衡）' },
  { value: 'gaming', label: '🎮 游戏场景（高配）' },
]

const WEBGL_RENDERER_OPTIONS: Record<string, { value: string; label: string }[]> = {
  Intel: [
    { value: '', label: '不设置' },
    { value: 'Intel(R) UHD Graphics 630', label: 'UHD Graphics 630' },
    { value: 'Intel(R) UHD Graphics 620', label: 'UHD Graphics 620' },
    { value: 'Intel(R) HD Graphics 520', label: 'HD Graphics 520' },
    { value: 'Intel(R) Iris(R) Xe Graphics', label: 'Iris Xe Graphics' },
    { value: 'custom', label: '自定义...' },
  ],
  NVIDIA: [
    { value: '', label: '不设置' },
    { value: 'NVIDIA GeForce RTX 3080', label: 'GeForce RTX 3080' },
    { value: 'NVIDIA GeForce RTX 3060', label: 'GeForce RTX 3060' },
    { value: 'NVIDIA GeForce GTX 1660', label: 'GeForce GTX 1660' },
    { value: 'NVIDIA GeForce GTX 1080 Ti', label: 'GeForce GTX 1080 Ti' },
    { value: 'custom', label: '自定义...' },
  ],
  AMD: [
    { value: '', label: '不设置' },
    { value: 'AMD Radeon RX 6600', label: 'Radeon RX 6600' },
    { value: 'AMD Radeon RX 580', label: 'Radeon RX 580' },
    { value: 'AMD Radeon Vega 8', label: 'Radeon Vega 8' },
    { value: 'custom', label: '自定义...' },
  ],
  Apple: [
    { value: '', label: '不设置' },
    { value: 'Apple M1', label: 'Apple M1' },
    { value: 'Apple M2', label: 'Apple M2' },
    { value: 'Apple M3', label: 'Apple M3' },
    { value: 'custom', label: '自定义...' },
  ],
}

const BOOL_OPTIONS = [
  { value: '', label: '不设置' },
  { value: 'true', label: '启用' },
  { value: 'false', label: '禁用' },
]

const HARDWARE_CONCURRENCY_OPTIONS = [
  { value: '', label: '不设置' },
  { value: '2', label: '2 核' },
  { value: '4', label: '4 核' },
  { value: '6', label: '6 核' },
  { value: '8', label: '8 核' },
  { value: '10', label: '10 核' },
  { value: '12', label: '12 核' },
  { value: '16', label: '16 核' },
]

const DEVICE_MEMORY_OPTIONS = [
  { value: '', label: '不设置' },
  { value: '2', label: '2 GB' },
  { value: '4', label: '4 GB' },
  { value: '8', label: '8 GB' },
  { value: '16', label: '16 GB' },
  { value: '32', label: '32 GB' },
]

const COLOR_DEPTH_OPTIONS = [
  { value: '', label: '不设置' },
  { value: '24', label: '24 位（标准）' },
  { value: '30', label: '30 位（HDR）' },
  { value: '32', label: '32 位' },
]

const WEBRTC_OPTIONS = [
  { value: '', label: '不设置' },
  { value: 'disable_non_proxied_udp', label: '禁用非代理 UDP（推荐）' },
  { value: 'default_public_interface_only', label: '仅公网接口' },
  { value: 'default_public_and_private_interfaces', label: '公网+私网接口' },
]

const TOUCH_POINTS_OPTIONS = [
  { value: '', label: '不设置' },
  { value: '0', label: '0（无触摸）' },
  { value: '1', label: '1 点触摸' },
  { value: '5', label: '5 点触摸' },
  { value: '10', label: '10 点触摸' },
]

const PRESET_OPTIONS = [
  { value: '', label: '选择预设...' },
  ...FINGERPRINT_PRESETS.map(p => ({ value: p.id, label: p.name })),
]

export function FingerprintPanel({ value, onChange }: FingerprintPanelProps) {
  const [config, setConfig] = useState<FingerprintConfig>(() => deserialize(value))
  const [advancedOpen, setAdvancedOpen] = useState(false)
  const [, setCustomRenderer] = useState('')
  const [confirmSeedOpen, setConfirmSeedOpen] = useState(false)

  // 检测当前 WebGL 策略
  const getWebGLStrategy = (): string => {
    if (config.disableSpoofing?.includes('gpu')) {
      return 'real'
    } else if (config.webglVendor || config.webglRenderer) {
      return 'custom'
    }
    return 'auto'
  }

  const [webglStrategy, setWebglStrategy] = useState<string>(() => getWebGLStrategy())
  const [smartGenerateOpen, setSmartGenerateOpen] = useState(false)
  const [smartScenario, setSmartScenario] = useState<'random' | 'office' | 'home' | 'gaming'>('random')

  useEffect(() => {
    setConfig(deserialize(value))
  }, [value.join('\n')])

  const update = (patch: Partial<FingerprintConfig>) => {
    const next = { ...config, ...patch }
    setConfig(next)
    onChange(serialize(next))
  }

  const handlePresetChange = (presetId: string) => {
    if (!presetId) return
    const preset = FINGERPRINT_PRESETS.find(p => p.id === presetId)
    if (!preset) return
    // 应用预设时自动生成新种子，保留未知参数
    const next: FingerprintConfig = {
      ...preset.config,
      seed: randomFingerprintSeed(),
      unknownArgs: config.unknownArgs,
    }
    setConfig(next)
    onChange(serialize(next))
  }

  const handleAdvancedChange = (text: string) => {
    const args = text.split('\n').map(s => s.trim()).filter(Boolean)
    const parsed = deserialize(args)
    setConfig(parsed)
    onChange(serialize(parsed))
  }

  const handleWebGLStrategyChange = (strategy: string) => {
    setWebglStrategy(strategy)

    if (strategy === 'auto') {
      // 自动混淆：移除 GPU 禁用标记和自定义 vendor/renderer
      const newDisableSpoofing = (config.disableSpoofing || []).filter(s => s !== 'gpu')
      update({
        disableSpoofing: newDisableSpoofing.length > 0 ? newDisableSpoofing : undefined,
        webglVendor: undefined,
        webglRenderer: undefined,
      })
    } else if (strategy === 'real') {
      // 使用真实硬件：添加 GPU 到禁用列表
      const newDisableSpoofing = [...new Set([...(config.disableSpoofing || []), 'gpu'])]
      update({
        disableSpoofing: newDisableSpoofing,
        webglVendor: undefined,
        webglRenderer: undefined,
      })
    }
    // strategy === 'custom' 时保持 webglVendor/webglRenderer 不变
  }

  const handleSmartGenerate = () => {
    // TODO: 从代理设置中获取国家代码和时区
    // 这里先使用默认值，后续可以集成代理信息
    const generated = generateSmartFingerprint({
      scenario: smartScenario,
      // proxyCountry: 从代理获取
      // proxyTimezone: 从代理获取
    })

    // 保留未知参数
    const next: FingerprintConfig = {
      ...generated,
      unknownArgs: config.unknownArgs,
    }

    setConfig(next)
    onChange(serialize(next))
    setSmartGenerateOpen(false)
  }

  const rendererOptions = config.webglVendor
    ? (WEBGL_RENDERER_OPTIONS[config.webglVendor] ?? [{ value: '', label: '不设置' }, { value: 'custom', label: '自定义...' }])
    : [{ value: '', label: '不设置' }]

  const isCustomRenderer = config.webglRenderer
    ? !rendererOptions.some(o => o.value === config.webglRenderer && o.value !== 'custom')
    : false

  const advancedText = serialize(config).join('\n')

  return (
    <div className="space-y-4">
      {/* 指纹种子 */}
      <div className="p-3 rounded-lg bg-[var(--color-bg-hover)] border border-[var(--color-border)] space-y-2">
        <div className="flex items-center justify-between">
          <span className="text-xs font-medium text-[var(--color-text-muted)] uppercase tracking-wide">指纹种子（Fingerprint Seed）</span>
          <span className="text-xs text-[var(--color-text-muted)]">决定所有随机噪声的根值，不同种子 = 不同指纹</span>
        </div>
        <div className="flex items-center gap-2">
          <Input
            value={config.seed ?? ''}
            onChange={e => update({ seed: e.target.value || undefined })}
            placeholder="留空则由系统按 ProfileId 自动生成"
            className="flex-1 font-mono text-sm"
          />
          <button
            type="button"
            title="随机生成新种子"
            onClick={() => {
              if (config.seed) {
                setConfirmSeedOpen(true)
              } else {
                update({ seed: randomFingerprintSeed() })
              }
            }}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs bg-[var(--color-primary)] text-white hover:opacity-90 transition-opacity shrink-0"
          >
            <RefreshCw className="w-3.5 h-3.5" />
            随机
          </button>
        </div>
      </div>

      <ConfirmModal
        open={confirmSeedOpen}
        onClose={() => setConfirmSeedOpen(false)}
        onConfirm={() => update({ seed: randomFingerprintSeed() })}
        title="重新生成指纹种子"
        content="重新生成后，当前指纹将完全改变，浏览器的 Canvas、WebGL、Audio 等所有噪声特征都会随之变化。确定继续？"
        confirmText="确定重新生成"
        danger
      />

      {/* 预设选择 */}
      <div className="flex items-center gap-3 p-3 rounded-lg bg-[var(--color-bg-hover)] border border-[var(--color-border)]">
        <Wand2 className="w-4 h-4 text-[var(--color-text-muted)] shrink-0" />
        <div className="flex-1 min-w-0">
          <Select
            value=""
            onChange={e => handlePresetChange(e.target.value)}
            options={PRESET_OPTIONS}
          />
        </div>
        <span className="text-xs text-[var(--color-text-muted)] shrink-0">选择后覆盖当前配置</span>
      </div>

      {/* 智能生成 */}
      <div className="p-3 rounded-lg bg-purple-50 dark:bg-purple-950 border border-purple-200 dark:border-purple-800">
        <div className="flex items-center justify-between mb-2">
          <div className="flex items-center gap-2">
            <Sparkles className="w-4 h-4 text-purple-600 dark:text-purple-400" />
            <span className="text-sm font-medium text-purple-900 dark:text-purple-100">智能生成指纹</span>
          </div>
          <button
            type="button"
            onClick={() => setSmartGenerateOpen(!smartGenerateOpen)}
            className="text-xs text-purple-600 dark:text-purple-400 hover:underline"
          >
            {smartGenerateOpen ? '收起' : '展开'}
          </button>
        </div>

        {smartGenerateOpen && (
          <div className="space-y-3 mt-3">
            <p className="text-xs text-purple-800 dark:text-purple-200">
              根据统计学数据智能生成合理的指纹配置，包括浏览器、平台、分辨率、硬件配置等，确保配置真实可信。
            </p>
            <FormItem label="生成场景">
              <Select
                value={smartScenario}
                onChange={e => setSmartScenario(e.target.value as any)}
                options={SMART_GENERATE_SCENARIO_OPTIONS}
              />
            </FormItem>
            <button
              type="button"
              onClick={handleSmartGenerate}
              className="w-full flex items-center justify-center gap-2 px-4 py-2 rounded-md text-sm bg-purple-600 text-white hover:bg-purple-700 transition-all shadow-md hover:shadow-lg"
            >
              <Sparkles className="w-4 h-4" />
              立即生成
            </button>
          </div>
        )}
      </div>

      {/* 基础身份 */}
      <div>
        <p className="text-xs font-medium text-[var(--color-text-muted)] mb-2 uppercase tracking-wide">基础身份</p>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <FormItem label="浏览器品牌">
            <Select value={config.brand ?? ''} onChange={e => update({ brand: e.target.value || undefined })} options={BRAND_OPTIONS} />
          </FormItem>
          <FormItem label="操作系统">
            <Select value={config.platform ?? ''} onChange={e => update({ platform: e.target.value || undefined })} options={PLATFORM_OPTIONS} />
          </FormItem>
          <FormItem label="语言">
            <Select value={config.lang ?? ''} onChange={e => update({ lang: e.target.value || undefined })} options={LANG_OPTIONS} />
          </FormItem>
          <FormItem label="时区">
            <Select value={config.timezone ?? ''} onChange={e => update({ timezone: e.target.value || undefined })} options={TIMEZONE_OPTIONS.map(opt =>
              opt.value === 'system'
                ? { ...opt, label: `跟随系统时区 (当前: ${getSystemTimezone()})` }
                : opt
            )} />
          </FormItem>
        </div>
        <p className="text-xs text-[var(--color-text-muted)] mt-2">
          语言与时区留空（不设置）时，已有代理出口检测缓存会自动补齐相应语言/时区；无缓存时会后台检测代理出口 IP，下次启动生效。需固定值时再手动选择。
        </p>
      </div>

      {/* 屏幕与硬件 */}
      <div>
        <p className="text-xs font-medium text-[var(--color-text-muted)] mb-2 uppercase tracking-wide">屏幕与硬件</p>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <FormItem label="分辨率">
            <Select
              value={config.resolution ?? ''}
              onChange={e => update({ resolution: e.target.value || undefined, customResolution: undefined })}
              options={RESOLUTION_OPTIONS}
            />
          </FormItem>
          {config.resolution === 'custom' && (
            <FormItem label="自定义分辨率">
              <Input value={config.customResolution ?? ''} onChange={e => update({ customResolution: e.target.value || undefined })} placeholder="1600,900" />
            </FormItem>
          )}
          <FormItem label="色深">
            <Select value={config.colorDepth ?? ''} onChange={e => update({ colorDepth: e.target.value || undefined })} options={COLOR_DEPTH_OPTIONS} />
          </FormItem>
          <FormItem label="CPU 核心数">
            <Select value={config.hardwareConcurrency ?? ''} onChange={e => update({ hardwareConcurrency: e.target.value || undefined })} options={HARDWARE_CONCURRENCY_OPTIONS} />
          </FormItem>
          <FormItem label="设备内存">
            <Select value={config.deviceMemory ?? ''} onChange={e => update({ deviceMemory: e.target.value || undefined })} options={DEVICE_MEMORY_OPTIONS} />
          </FormItem>
          <FormItem label="触摸点数">
            <Select value={config.touchPoints ?? ''} onChange={e => update({ touchPoints: e.target.value || undefined })} options={TOUCH_POINTS_OPTIONS} />
          </FormItem>
        </div>
      </div>

      {/* 渲染指纹 */}
      <div>
        <p className="text-xs font-medium text-[var(--color-text-muted)] mb-2 uppercase tracking-wide">渲染指纹</p>

        {/* WebGL 策略选择 */}
        <div className="mb-4 p-3 rounded-lg bg-blue-50 dark:bg-blue-950 border border-blue-200 dark:border-blue-800">
          <FormItem label="WebGL 指纹策略">
            <Select
              value={webglStrategy}
              onChange={e => handleWebGLStrategyChange(e.target.value)}
              options={WEBGL_STRATEGY_OPTIONS}
            />
          </FormItem>
          <div className="mt-2 text-xs text-[var(--color-text-muted)] space-y-1">
            {webglStrategy === 'auto' && (
              <p>✅ <strong>自动混淆（推荐）</strong>：内核基于指纹种子自动生成一致的 Canvas/Audio/WebGL 指纹，所有指纹特征相互匹配，不会被检测为异常。</p>
            )}
            {webglStrategy === 'real' && (
              <p>✅ <strong>使用真实硬件</strong>：禁用 GPU 指纹混淆，使用本机真实 GPU 信息。适合单机多账号场景，但所有实例将共享相同的 GPU 指纹。</p>
            )}
            {webglStrategy === 'custom' && (
              <p>⚠️ <strong>自定义（已废弃）</strong>：Chrome 144+ 已移除自定义 GPU 参数支持。如果使用此选项，设置的 vendor/renderer 会被内核忽略，可能导致指纹不一致被检测为异常。</p>
            )}
          </div>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {webglStrategy === 'custom' && (
            <>
              <FormItem label="WebGL 供应商（已废弃）">
                <Select
                  value={config.webglVendor ?? ''}
                  onChange={e => update({ webglVendor: e.target.value || undefined, webglRenderer: undefined })}
                  options={WEBGL_VENDOR_OPTIONS}
                />
              </FormItem>
              <FormItem label="WebGL 渲染器（已废弃）">
                {isCustomRenderer ? (
                  <Input
                    value={config.webglRenderer ?? ''}
                    onChange={e => update({ webglRenderer: e.target.value || undefined })}
                    placeholder="自定义渲染器名称"
                  />
                ) : (
                  <Select
                    value={config.webglRenderer ?? ''}
                    onChange={e => {
                      if (e.target.value === 'custom') {
                        setCustomRenderer('')
                        update({ webglRenderer: undefined })
                      } else {
                        update({ webglRenderer: e.target.value || undefined })
                      }
                    }}
                    options={rendererOptions}
                    disabled={!config.webglVendor}
                  />
                )}
              </FormItem>
            </>
          )}
          <FormItem label="Canvas 噪声（已废弃）">
            <Select
              value={config.canvasNoise === undefined ? '' : String(config.canvasNoise)}
              onChange={e => { const v = e.target.value; update({ canvasNoise: v === '' ? undefined : v === 'true' }) }}
              options={BOOL_OPTIONS}
            />
          </FormItem>
          <FormItem label="Audio 噪声（已废弃）">
            <Select
              value={config.audioNoise === undefined ? '' : String(config.audioNoise)}
              onChange={e => { const v = e.target.value; update({ audioNoise: v === '' ? undefined : v === 'true' }) }}
              options={BOOL_OPTIONS}
            />
          </FormItem>
        </div>
        <p className="text-xs text-[var(--color-text-muted)] mt-2">
          Chrome 144+ 内核已内置自动混淆，Canvas/Audio/WebGL 噪声由 --fingerprint 种子统一控制。手动设置的 Canvas/Audio/WebGL 参数已被废弃。
        </p>
      </div>

      {/* 网络与隐私 */}
      <div>
        <p className="text-xs font-medium text-[var(--color-text-muted)] mb-2 uppercase tracking-wide">网络与隐私</p>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <FormItem label="WebRTC 策略">
            <Select value={config.webrtcPolicy ?? ''} onChange={e => update({ webrtcPolicy: e.target.value || undefined })} options={WEBRTC_OPTIONS} />
          </FormItem>
          <FormItem label="Do Not Track">
            <Select
              value={config.doNotTrack === undefined ? '' : String(config.doNotTrack)}
              onChange={e => { const v = e.target.value; update({ doNotTrack: v === '' ? undefined : v === 'true' }) }}
              options={BOOL_OPTIONS}
            />
          </FormItem>
          <FormItem label="媒体设备 (摄像头,麦克风,扬声器)">
            <Input
              value={config.mediaDevices ?? ''}
              onChange={e => update({ mediaDevices: e.target.value || undefined })}
              placeholder="2,1,1"
            />
          </FormItem>
        </div>
      </div>

      {/* 字体 */}
      <div>
        <p className="text-xs font-medium text-[var(--color-text-muted)] mb-2 uppercase tracking-wide">字体</p>
        <FormItem label="字体列表">
          <Input
            value={config.fonts ?? ''}
            onChange={e => update({ fonts: e.target.value || undefined })}
            placeholder="Arial,Helvetica,Times New Roman（逗号分隔）"
          />
        </FormItem>
      </div>

      {/* 高级模式 */}
      <div className="border border-[var(--color-border)] rounded-lg overflow-hidden">
        <button
          type="button"
          className="w-full flex items-center justify-between px-4 py-2.5 text-sm text-[var(--color-text-muted)] hover:bg-[var(--color-bg-hover)] transition-colors"
          onClick={() => setAdvancedOpen(v => !v)}
        >
          <span>高级模式（原始参数）</span>
          {advancedOpen ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
        </button>
        {advancedOpen && (
          <div className="px-4 pb-4 pt-2 border-t border-[var(--color-border)]">
            <p className="text-xs text-[var(--color-text-muted)] mb-2">每行一个参数，修改后自动同步到上方控件</p>
            <Textarea
              value={advancedText}
              onChange={e => handleAdvancedChange(e.target.value)}
              rows={6}
              placeholder="--fingerprint-brand=Chrome"
            />
          </div>
        )}
      </div>
    </div>
  )
}
