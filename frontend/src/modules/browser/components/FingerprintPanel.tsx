import { useEffect, useState, type ReactNode } from 'react'
import { ChevronDown, ChevronUp, HelpCircle, RefreshCw } from 'lucide-react'
import { Button, ConfirmModal, FormItem, Input, Modal, Select, Switch, Textarea } from '../../../shared/components'
import {
  type FingerprintConfig,
  FINGERPRINT_PRESETS,
  PRESET_RESOLUTIONS,
  buildAcceptLanguage,
  buildFingerprintConfigFromPersona,
  deserialize,
  getSystemTimezone,
  randomFingerprintSeed,
  serialize,
  validateFingerprintArgs,
} from '../utils/fingerprintSerializer'
import { FINGERPRINT_CAPABILITIES, FINGERPRINT_PERSONAS, capabilityModeLabel } from '../utils/fingerprintCapabilities'
import { isCloakBackend } from '../utils/coreBackend'

interface FingerprintPanelProps {
  value: string[]
  onChange: (args: string[]) => void
  // 当前实例使用的内核后端。指纹参数在不同后端下的有效性结论可能相反，
  // 面板需要据此调整校验口径，避免把 fingerprint-chromium 的结论套到 Cloak 上。
  coreBackend?: string
}

const BRAND_OPTIONS = [
  { value: '', label: '不设置' },
  { value: 'Chrome', label: 'Chrome' },
  { value: 'Edge', label: 'Edge' },
  { value: 'Opera', label: 'Opera' },
  { value: 'Vivaldi', label: 'Vivaldi' },
]

const PLATFORM_OPTIONS = [
  { value: '', label: '不设置' },
  { value: 'windows', label: 'Windows' },
  { value: 'macos', label: 'macOS' },
  { value: 'linux', label: 'Linux' },
]

const LANG_OPTIONS = [
  { value: '', label: '不设置' },
  { value: 'zh-CN', label: '中文 (zh-CN)' },
  { value: 'zh-HK', label: '繁體中文香港 (zh-HK)' },
  { value: 'zh-TW', label: '繁體中文台灣 (zh-TW)' },
  { value: 'en-US', label: 'English US (en-US)' },
  { value: 'en-GB', label: 'English UK (en-GB)' },
  { value: 'en-CA', label: 'English Canada (en-CA)' },
  { value: 'en-AU', label: 'English Australia (en-AU)' },
  { value: 'en-SG', label: 'English Singapore (en-SG)' },
  { value: 'en-IN', label: 'English India (en-IN)' },
  { value: 'ja-JP', label: '日本語 (ja-JP)' },
  { value: 'ko-KR', label: '한국어 (ko-KR)' },
  { value: 'fr-FR', label: 'Français (fr-FR)' },
  { value: 'de-DE', label: 'Deutsch (de-DE)' },
  { value: 'nl-NL', label: 'Nederlands (nl-NL)' },
  { value: 'ru-RU', label: 'Русский (ru-RU)' },
  { value: 'pt-BR', label: 'Português Brasil (pt-BR)' },
]

const BRAND_VERSION_OPTIONS = [
  { value: '144.0.7559.132', label: '144.0.7559.132' },
  { value: '143.0.7499.10', label: '143.0.7499.10' },
  { value: '148.0.7778.215', label: '148.0.7778.215' },
]

const PLATFORM_VERSION_OPTIONS = [
  { value: '10.0.0', label: 'Windows 10 / 10.0.0' },
  { value: '15.2.0', label: 'macOS 15.2 / 15.2.0' },
]

const ACCEPT_LANG_OPTIONS = LANG_OPTIONS
  .filter(option => option.value)
  .map(option => ({ value: buildAcceptLanguage(option.value), label: buildAcceptLanguage(option.value) }))

const TIMEZONE_OPTIONS = [
  { value: '', label: '不设置' },
  { value: 'system', label: '跟随系统时区' },
  { value: 'Asia/Shanghai', label: 'Asia/Shanghai (UTC+8)' },
  { value: 'Asia/Tokyo', label: 'Asia/Tokyo (UTC+9)' },
  { value: 'Asia/Seoul', label: 'Asia/Seoul (UTC+9)' },
  { value: 'Asia/Singapore', label: 'Asia/Singapore (UTC+8)' },
  { value: 'Asia/Hong_Kong', label: 'Asia/Hong_Kong (UTC+8)' },
  { value: 'Asia/Taipei', label: 'Asia/Taipei (UTC+8)' },
  { value: 'Asia/Dubai', label: 'Asia/Dubai (UTC+4)' },
  { value: 'Asia/Kolkata', label: 'Asia/Kolkata (UTC+5:30)' },
  { value: 'America/New_York', label: 'America/New_York (UTC-5)' },
  { value: 'America/Los_Angeles', label: 'America/Los_Angeles (UTC-8)' },
  { value: 'America/Chicago', label: 'America/Chicago (UTC-6)' },
  { value: 'America/Denver', label: 'America/Denver (UTC-7)' },
  { value: 'America/Toronto', label: 'America/Toronto (UTC-5)' },
  { value: 'America/Vancouver', label: 'America/Vancouver (UTC-8)' },
  { value: 'America/Phoenix', label: 'America/Phoenix (UTC-7)' },
  { value: 'America/Sao_Paulo', label: 'America/Sao_Paulo (UTC-3)' },
  { value: 'Europe/London', label: 'Europe/London (UTC+0)' },
  { value: 'Europe/Paris', label: 'Europe/Paris (UTC+1)' },
  { value: 'Europe/Berlin', label: 'Europe/Berlin (UTC+1)' },
  { value: 'Europe/Moscow', label: 'Europe/Moscow (UTC+3)' },
  { value: 'Australia/Sydney', label: 'Australia/Sydney (UTC+10)' },
  { value: 'Australia/Melbourne', label: 'Australia/Melbourne (UTC+10)' },
  { value: 'Australia/Perth', label: 'Australia/Perth (UTC+8)' },
  { value: 'Pacific/Auckland', label: 'Pacific/Auckland (UTC+12)' },
]

const RESOLUTION_OPTIONS = [
  { value: '', label: '不设置' },
  ...PRESET_RESOLUTIONS.map(r => ({ value: r, label: r })),
  { value: 'custom', label: '自定义...' },
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

const WEBRTC_OPTIONS = [
  { value: '', label: '不设置' },
  { value: 'disable_non_proxied_udp', label: '禁用非代理 UDP' },
  { value: 'default_public_interface_only', label: '仅公网接口' },
  { value: 'default_public_and_private_interfaces', label: '公网+私网接口' },
]

const NOISE_OPTIONS = [
  { value: '', label: '默认（使用全局默认）' },
  { value: '0', label: '显式关闭' },
  { value: '1', label: '显式开启' },
]

const SPOOFING_OPTIONS = [
  { value: 'font', label: '字体' },
  { value: 'audio', label: '音频' },
  { value: 'canvas', label: 'Canvas' },
  { value: 'clientrects', label: 'ClientRects' },
  { value: 'gpu', label: 'GPU' },
]

const PRESET_OPTIONS = [
  { value: '', label: '选择预设...' },
  ...FINGERPRINT_PRESETS.map(p => ({ value: p.id, label: p.name })),
]

const PERSONA_OPTIONS = [
  { value: '', label: '选择高级画像...' },
  ...FINGERPRINT_PERSONAS.map(item => ({ value: item.id, label: item.name })),
]

const ADVANCED_ARG_HELP_ROWS = [
  { arg: '--fingerprint=<seed>', usage: '统一指纹种子；留空时启动按实例 ID 补稳定种子', example: '--fingerprint=123456' },
  { arg: '--fingerprint-brand=<brand>', usage: '浏览器品牌；影响 UA / UA-CH 品牌口径', example: '--fingerprint-brand=Chrome' },
  { arg: '--fingerprint-brand-version=<version>', usage: '浏览器版本；完整版本优先，检测时 UA 降级按主版本口径匹配', example: '--fingerprint-brand-version=144.0.7559.132' },
  { arg: '--fingerprint-platform=<platform>', usage: '系统平台画像；支持 windows、macos、linux', example: '--fingerprint-platform=windows' },
  { arg: '--fingerprint-platform-version=<version>', usage: '系统版本；检测时按 UA / UA-CH 可见版本前缀匹配', example: '--fingerprint-platform-version=10.0.0' },
  { arg: '--lang=<locale>', usage: '主语言；同时会补默认 Accept-Language', example: '--lang=ja-JP' },
  { arg: '--accept-lang=<list>', usage: '语言列表；逗号分隔，按 navigator.languages 前缀比对', example: '--accept-lang=ja-JP,ja' },
  { arg: '--timezone=<iana>', usage: '时区；使用 IANA 时区名', example: '--timezone=Asia/Tokyo' },
  { arg: '--window-size=<w,h>', usage: '启动窗口外框尺寸；检测比对 outerWidth/outerHeight', example: '--window-size=1600,900' },
  { arg: '--fingerprint-hardware-concurrency=<n>', usage: 'CPU 核心数；1 到 128 的整数', example: '--fingerprint-hardware-concurrency=8' },
  { arg: '--disable-non-proxied-udp', usage: 'WebRTC 防泄漏；禁用非代理 UDP', example: '--disable-non-proxied-udp' },
  { arg: '--webrtc-ip-handling-policy=<policy>', usage: 'WebRTC 策略；用于更细的 IP 暴露控制', example: '--webrtc-ip-handling-policy=default_public_interface_only' },
  { arg: '--fingerprinting-canvas-image-data-noise', usage: '启用 Canvas ImageData 噪声；页面只能观察 Canvas Hash', example: '--fingerprinting-canvas-image-data-noise' },
  { arg: '--fingerprinting-client-rects-noise', usage: '启用 ClientRects 噪声；页面只能观察 ClientRects Hash', example: '--fingerprinting-client-rects-noise' },
  { arg: '--disable-spoofing=<items>', usage: '禁用指定伪装项；可选 font、audio、canvas、clientrects、gpu', example: '--disable-spoofing=font,audio' },
]

interface EditableOptionInputProps {
  value: string
  onChange: (value: string) => void
  options: { value: string; label: string }[]
  placeholder?: string
}

interface FingerprintSectionProps {
  title: string
  children: ReactNode
}

function FingerprintSection({ title, children }: FingerprintSectionProps) {
  return (
    <section className="rounded-lg border border-[var(--color-border-muted)] bg-[var(--color-bg-subtle)] p-4 shadow-[var(--shadow-xs)] space-y-4">
      <h4 className="text-xs font-medium text-[var(--color-text-muted)] uppercase tracking-wide">{title}</h4>
      {children}
    </section>
  )
}

function EditableOptionInput({ value, onChange, options, placeholder }: EditableOptionInputProps) {
  const [open, setOpen] = useState(false)

  return (
    <div className="relative">
      <Input
        value={value}
        onChange={e => onChange(e.target.value)}
        onFocus={() => setOpen(true)}
        onBlur={() => window.setTimeout(() => setOpen(false), 120)}
        placeholder={placeholder}
        className="pr-9"
      />
      <button
        type="button"
        className="absolute right-2 top-1/2 -translate-y-1/2 p-1 text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)]"
        onMouseDown={e => e.preventDefault()}
        onClick={() => setOpen(current => !current)}
      >
        <ChevronDown className="h-4 w-4" />
      </button>
      {open && (
        <div className="absolute z-20 mt-1 max-h-56 w-full overflow-auto rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] shadow-lg">
          {options.map(option => (
            <button
              key={option.value}
              type="button"
              className="block w-full px-3 py-2 text-left text-sm text-[var(--color-text-primary)] hover:bg-[var(--color-bg-hover)]"
              onMouseDown={e => e.preventDefault()}
              onClick={() => {
                onChange(option.value)
                setOpen(false)
              }}
            >
              {option.label}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}

export function FingerprintPanel({ value, onChange, coreBackend }: FingerprintPanelProps) {
  const [config, setConfig] = useState<FingerprintConfig>(() => deserialize(value))
  const [advancedOpen, setAdvancedOpen] = useState(false)
  const [advancedHelpOpen, setAdvancedHelpOpen] = useState(false)
  const [confirmSeedOpen, setConfirmSeedOpen] = useState(false)
  const [capabilitiesOpen, setCapabilitiesOpen] = useState(false)

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

  const handlePersonaChange = (personaId: string) => {
    if (!personaId) return
    const persona = FINGERPRINT_PERSONAS.find(item => item.id === personaId)
    if (!persona) return
    const next: FingerprintConfig = {
      ...buildFingerprintConfigFromPersona(persona),
      unknownArgs: config.unknownArgs,
    }
    setConfig(next)
    onChange(serialize(next))
  }

  const toggleDisableSpoofing = (key: string, checked: boolean) => {
    const current = config.disableSpoofing ?? []
    const next = checked ? [...current, key] : current.filter(item => item !== key)
    update({ disableSpoofing: next.length ? next : undefined })
  }

  const advancedText = serialize(config).join('\n')
  const cloakBackend = isCloakBackend(coreBackend)
  // Cloak 的参数集与 fingerprint-chromium 差异较大（GPU / 内存 / 屏幕在 Cloak 下有效，
  // 噪声开关和 --disable-spoofing 在 Cloak 下无效），当前面板按 fingerprint-chromium 建模，
  // 因此在 Cloak 内核下不展示这套校验结论，避免给出相反的判断。
  const validation = validateFingerprintArgs(value)
  const validationTone = validation.issues.some(issue => issue.level === 'error')
    ? 'border-red-200 bg-red-50 text-red-700'
    : validation.issues.some(issue => issue.level === 'warning')
      ? 'border-amber-200 bg-amber-50 text-amber-700'
      : 'border-emerald-200 bg-emerald-50 text-emerald-700'
  const validationTitle = validation.valid
    ? validation.issues.some(issue => issue.level === 'warning') ? '配置可用，有提示' : '配置有效'
    : '配置需要修正'

  return (
    <div className="space-y-4">
      {cloakBackend ? (
        <div className="rounded-lg border border-sky-200 bg-sky-50 px-3 py-2 text-sm text-sky-800">
          <div className="font-medium">当前实例使用 Cloak 内核</div>
          <ul className="mt-1 space-y-1 text-xs leading-5">
            <li>指纹面板的可用性提示按 fingerprint-chromium 建模，对 Cloak 不适用，已暂时隐藏。</li>
            <li>Cloak 下 GPU Vendor / Renderer、设备内存、屏幕宽高是受支持参数；Canvas / ClientRects 噪声开关和排除伪装不生效，噪声由指纹种子驱动。</li>
            <li>保存后请在“指纹能力矩阵”查看按 Cloak 后端解析的实际运行参数。</li>
          </ul>
        </div>
      ) : (
        <div className={`rounded-lg border px-3 py-2 text-sm ${validationTone}`}>
        <div className="font-medium">{validationTitle}</div>
        {validation.issues.length > 0 && (
          <ul className="mt-1 space-y-1">
            {validation.issues.slice(0, 4).map((issue, index) => (
              <li key={`${issue.level}-${index}`} className="text-xs">{issue.message}</li>
            ))}
            {validation.issues.length > 4 && <li className="text-xs">还有 {validation.issues.length - 4} 项，请在高级模式中检查</li>}
          </ul>
        )}
        </div>
      )}

      <div className="p-3 rounded-lg bg-[var(--color-bg-hover)] border border-[var(--color-border)] space-y-2">
        <div className="flex items-center justify-between gap-3">
          <span className="text-xs font-medium text-[var(--color-text-muted)] uppercase tracking-wide">指纹种子</span>
          <button
            type="button"
            className="inline-flex items-center gap-1 text-xs text-[var(--color-primary)] hover:underline"
            onClick={() => setConfirmSeedOpen(true)}
          >
            <RefreshCw className="w-3 h-3" />
            重新生成
          </button>
        </div>
        <Input value={config.seed ?? ''} onChange={e => update({ seed: e.target.value || undefined })} placeholder="留空则启动时按实例 ID 自动生成" />
      </div>

      <ConfirmModal
        open={confirmSeedOpen}
        onClose={() => setConfirmSeedOpen(false)}
        onConfirm={() => update({ seed: randomFingerprintSeed() })}
        title="重新生成指纹种子"
        content="重新生成后，会影响当前内核支持的随机指纹项；具体生效范围以检测结果为准。确定继续？"
        confirmText="确定重新生成"
      />

      <Modal
        open={capabilitiesOpen}
        onClose={() => setCapabilitiesOpen(false)}
        title="指纹能力覆盖"
        width="860px"
        footer={<Button variant="secondary" onClick={() => setCapabilitiesOpen(false)}>关闭</Button>}
      >
        <div className="rounded-lg border border-[var(--color-border)] overflow-hidden">
          <div className="grid grid-cols-[120px_96px_minmax(0,1fr)] gap-3 px-3 py-2 text-xs font-medium text-[var(--color-text-muted)] border-b border-[var(--color-border)] bg-[var(--color-bg-hover)]">
            <div>能力</div>
            <div>模式</div>
            <div>覆盖</div>
          </div>
          {FINGERPRINT_CAPABILITIES.map(item => (
            <div key={item.id} className="grid grid-cols-[120px_96px_minmax(0,1fr)] gap-3 px-3 py-2 text-xs border-b last:border-b-0 border-[var(--color-border)]">
              <div className="font-medium text-[var(--color-text-primary)]">{item.name}</div>
              <div className="text-[var(--color-text-secondary)]">{capabilityModeLabel(item.mode)}</div>
              <div className="text-[var(--color-text-muted)]">{item.coverage}</div>
            </div>
          ))}
        </div>
      </Modal>

      <Modal
        open={advancedHelpOpen}
        onClose={() => setAdvancedHelpOpen(false)}
        title="原始参数使用方式"
        width="980px"
        footer={<Button variant="secondary" onClick={() => setAdvancedHelpOpen(false)}>关闭</Button>}
      >
        <div className="rounded-lg border border-[var(--color-border)] overflow-hidden">
          <div className="grid grid-cols-[220px_minmax(0,1fr)_minmax(220px,0.8fr)] gap-3 px-3 py-2 text-xs font-medium text-[var(--color-text-muted)] border-b border-[var(--color-border)] bg-[var(--color-bg-hover)]">
            <div>参数</div>
            <div>用途</div>
            <div>示例</div>
          </div>
          {ADVANCED_ARG_HELP_ROWS.map(row => (
            <div key={row.arg} className="grid grid-cols-[220px_minmax(0,1fr)_minmax(220px,0.8fr)] gap-3 px-3 py-2 text-xs border-b last:border-b-0 border-[var(--color-border)]">
              <code className="break-all text-[var(--color-text-primary)]">{row.arg}</code>
              <div className="text-[var(--color-text-secondary)]">{row.usage}</div>
              <code className="break-all text-[var(--color-text-muted)]">{row.example}</code>
            </div>
          ))}
        </div>
      </Modal>

      <FingerprintSection title="快速生成">
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <FormItem label="快速预设">
            <Select value="" onChange={e => handlePresetChange(e.target.value)} options={PRESET_OPTIONS} />
          </FormItem>

          <FormItem label="高级画像">
            <Select value="" onChange={e => handlePersonaChange(e.target.value)} options={PERSONA_OPTIONS} />
          </FormItem>
        </div>
        <div className="flex justify-end">
          <Button type="button" variant="secondary" size="sm" onClick={() => setCapabilitiesOpen(true)}>
            查看能力覆盖
          </Button>
        </div>
      </FingerprintSection>

      <FingerprintSection title="身份与定位">
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <FormItem label="浏览器品牌">
            <Select value={config.brand ?? ''} onChange={e => update({ brand: e.target.value || undefined })} options={BRAND_OPTIONS} />
          </FormItem>
          <FormItem label="品牌版本">
            <EditableOptionInput value={config.brandVersion ?? ''} onChange={nextValue => update({ brandVersion: nextValue || undefined })} options={BRAND_VERSION_OPTIONS} placeholder="默认跟随内核" />
          </FormItem>
          <FormItem label="平台">
            <Select value={config.platform ?? ''} onChange={e => update({ platform: e.target.value || undefined })} options={PLATFORM_OPTIONS} />
          </FormItem>
          <FormItem label="系统版本">
            <EditableOptionInput value={config.platformVersion ?? ''} onChange={nextValue => update({ platformVersion: nextValue || undefined })} options={PLATFORM_VERSION_OPTIONS} placeholder="如 15.2.0" />
          </FormItem>
          <FormItem label="语言">
            <Select value={config.lang ?? ''} onChange={e => update({ lang: e.target.value || undefined, acceptLang: undefined })} options={LANG_OPTIONS} />
          </FormItem>
          <FormItem label="语言列表">
            <EditableOptionInput value={config.acceptLang ?? ''} onChange={nextValue => update({ acceptLang: nextValue || undefined })} options={ACCEPT_LANG_OPTIONS} placeholder={config.lang ? `默认 ${config.lang.split(/[-_]/)[0] === config.lang ? config.lang : `${config.lang},${config.lang.split(/[-_]/)[0]}`}` : '如 ja-JP,ja'} />
          </FormItem>
          <FormItem label="时区">
            <Select
              value={config.timezone ?? ''}
              onChange={e => update({ timezone: e.target.value || undefined })}
              options={TIMEZONE_OPTIONS.map(opt => opt.value === 'system' ? { ...opt, label: `跟随系统时区 (当前: ${getSystemTimezone()})` } : opt)}
            />
          </FormItem>
        </div>
      </FingerprintSection>

      <FingerprintSection title="设备与网络">
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <FormItem label="窗口大小">
            <Select value={config.resolution ?? ''} onChange={e => update({ resolution: e.target.value || undefined })} options={RESOLUTION_OPTIONS} />
          </FormItem>
          {config.resolution === 'custom' && (
            <FormItem label="自定义分辨率">
              <Input value={config.customResolution ?? ''} onChange={e => update({ customResolution: e.target.value || undefined })} placeholder="1920,1080" />
            </FormItem>
          )}
          <FormItem label="CPU 核心数">
            <Select value={config.hardwareConcurrency ?? ''} onChange={e => update({ hardwareConcurrency: e.target.value || undefined })} options={HARDWARE_CONCURRENCY_OPTIONS} />
          </FormItem>
          <FormItem label="WebRTC 策略">
            <Select value={config.webrtcPolicy ?? ''} onChange={e => update({ webrtcPolicy: e.target.value || undefined })} options={WEBRTC_OPTIONS} />
          </FormItem>
        </div>
      </FingerprintSection>

      <FingerprintSection title="兼容伪装">
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <FormItem label="Canvas 噪声">
            <Select value={config.canvasNoise ?? ''} onChange={e => update({ canvasNoise: e.target.value || undefined })} options={NOISE_OPTIONS} />
          </FormItem>
          <FormItem label="ClientRects 噪声">
            <Select value={config.clientRectsNoise ?? ''} onChange={e => update({ clientRectsNoise: e.target.value || undefined })} options={NOISE_OPTIONS} />
          </FormItem>
        </div>

        <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-surface)] p-3 space-y-3">
          <div className="flex items-center justify-between gap-3">
            <div className="text-sm font-medium text-[var(--color-text-primary)]">禁用某项伪装</div>
            <div className="text-xs text-[var(--color-text-muted)]">关闭开关 = 保持伪装</div>
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-2">
            {SPOOFING_OPTIONS.map(option => (
              <label key={option.value} className="flex items-center justify-between gap-3 rounded-lg border border-[var(--color-border-muted)] bg-[var(--color-bg-subtle)] px-3 py-2 text-sm text-[var(--color-text-primary)]">
                <span>禁用 {option.label}</span>
                <Switch
                  checked={(config.disableSpoofing ?? []).includes(option.value)}
                  onChange={checked => toggleDisableSpoofing(option.value, checked)}
                />
              </label>
            ))}
          </div>
        </div>
      </FingerprintSection>

      <div className="border border-[var(--color-border)] rounded-lg overflow-hidden">
        <button
          type="button"
          className="w-full flex items-center justify-between px-4 py-2.5 text-sm text-[var(--color-text-muted)] hover:bg-[var(--color-bg-hover)] transition-colors"
          onClick={() => setAdvancedOpen(v => !v)}
        >
          <span className="inline-flex items-center gap-2">
            <span>高级模式（原始参数）</span>
            <span
              role="button"
              tabIndex={0}
              className="inline-flex h-5 w-5 items-center justify-center rounded-full text-[var(--color-text-muted)] hover:bg-[var(--color-bg-muted)] hover:text-[var(--color-text-primary)]"
              onClick={event => {
                event.stopPropagation()
                setAdvancedHelpOpen(true)
              }}
              onKeyDown={event => {
                if (event.key === 'Enter' || event.key === ' ') {
                  event.preventDefault()
                  event.stopPropagation()
                  setAdvancedHelpOpen(true)
                }
              }}
              aria-label="查看原始参数使用方式"
            >
              <HelpCircle className="h-4 w-4" />
            </span>
          </span>
          {advancedOpen ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
        </button>
        {advancedOpen && (
          <div className="px-4 pb-4 pt-2 border-t border-[var(--color-border)]">
            <p className="text-xs text-[var(--color-text-muted)] mb-2">未建模参数会保留；本地实测无效的旧版细项保存时移除。</p>
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
