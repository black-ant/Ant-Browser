import { useEffect, useMemo, useState } from 'react'
import { ChevronDown, ChevronUp, HelpCircle, RefreshCw } from 'lucide-react'
import {
  FINGERPRINT_PRESETS,
  PRESET_RESOLUTIONS,
  buildAcceptLanguage,
  buildFingerprintConfigFromPersona,
  deserialize,
  getSystemTimezone,
  randomFingerprintSeed,
  serialize,
  validateFingerprintArgs,
  type FingerprintConfig,
} from '../../modules/browser/utils/fingerprintSerializer'
import { FINGERPRINT_CAPABILITIES, FINGERPRINT_PERSONAS, capabilityModeLabel } from '../../modules/browser/utils/fingerprintCapabilities'
import { FishConfirm, FishModal, FishSwitch } from './FishFormPrimitives'

interface FishFingerprintEditorProps {
  value: string[]
  onChange: (args: string[]) => void
}

const BRAND_OPTIONS = ['', 'Chrome', 'Edge', 'Opera', 'Vivaldi']
const PLATFORM_OPTIONS = ['', 'windows', 'macos', 'linux']
const LANG_OPTIONS = ['', 'zh-CN', 'zh-HK', 'zh-TW', 'en-US', 'en-GB', 'en-CA', 'en-AU', 'en-SG', 'en-IN', 'ja-JP', 'ko-KR', 'fr-FR', 'de-DE', 'nl-NL', 'ru-RU', 'pt-BR']
const BRAND_VERSION_OPTIONS = ['', '144.0.7559.132', '143.0.7499.10', '148.0.7778.215']
const PLATFORM_VERSION_OPTIONS = ['', '10.0.0', '15.2.0']
const TIMEZONE_OPTIONS = ['', 'system', 'Asia/Shanghai', 'Asia/Tokyo', 'Asia/Seoul', 'Asia/Singapore', 'Asia/Hong_Kong', 'Asia/Taipei', 'Asia/Dubai', 'Asia/Kolkata', 'America/New_York', 'America/Los_Angeles', 'America/Chicago', 'America/Denver', 'America/Toronto', 'America/Vancouver', 'America/Phoenix', 'America/Sao_Paulo', 'Europe/London', 'Europe/Paris', 'Europe/Berlin', 'Europe/Moscow', 'Australia/Sydney', 'Australia/Melbourne', 'Australia/Perth', 'Pacific/Auckland']
const CPU_OPTIONS = ['', '2', '4', '6', '8', '10', '12', '16']
const WEBRTC_OPTIONS = ['', 'disable_non_proxied_udp', 'default_public_interface_only', 'default_public_and_private_interfaces']
const NOISE_OPTIONS = ['', '0', '1']
const SPOOFING_OPTIONS = [
  { value: 'font', label: '字体' },
  { value: 'audio', label: '音频' },
  { value: 'canvas', label: 'Canvas' },
  { value: 'clientrects', label: 'ClientRects' },
  { value: 'gpu', label: 'GPU' },
]
const ADVANCED_ARG_HELP_ROWS = [
  ['--fingerprint=<seed>', '统一指纹种子；留空时启动按实例 ID 补稳定种子', '--fingerprint=123456'],
  ['--fingerprint-brand=<brand>', '浏览器品牌；影响 UA / UA-CH 品牌口径', '--fingerprint-brand=Chrome'],
  ['--fingerprint-brand-version=<version>', '浏览器版本；完整版本优先', '--fingerprint-brand-version=144.0.7559.132'],
  ['--fingerprint-platform=<platform>', '系统平台画像；windows / macos / linux', '--fingerprint-platform=windows'],
  ['--fingerprint-platform-version=<version>', '系统版本', '--fingerprint-platform-version=10.0.0'],
  ['--lang=<locale>', '主语言，并补默认 Accept-Language', '--lang=ja-JP'],
  ['--accept-lang=<list>', '语言列表，逗号分隔', '--accept-lang=ja-JP,ja'],
  ['--timezone=<iana>', 'IANA 时区名', '--timezone=Asia/Tokyo'],
  ['--window-size=<w,h>', '启动窗口外框尺寸', '--window-size=1600,900'],
  ['--fingerprint-hardware-concurrency=<n>', 'CPU 核心数', '--fingerprint-hardware-concurrency=8'],
  ['--disable-non-proxied-udp', 'WebRTC 防泄漏', '--disable-non-proxied-udp'],
  ['--webrtc-ip-handling-policy=<policy>', '更细的 WebRTC IP 暴露控制', '--webrtc-ip-handling-policy=default_public_interface_only'],
  ['--fingerprinting-canvas-image-data-noise', '启用 Canvas ImageData 噪声', '--fingerprinting-canvas-image-data-noise'],
  ['--fingerprinting-client-rects-noise', '启用 ClientRects 噪声', '--fingerprinting-client-rects-noise'],
  ['--disable-spoofing=<items>', '禁用指定伪装项', '--disable-spoofing=font,audio'],
]

function optionLabel(value: string, type: 'brand' | 'platform' | 'timezone' | 'cpu' | 'webrtc' | 'noise' | 'lang') {
  if (!value) return '不设置'
  if (type === 'platform') return value === 'windows' ? 'Windows' : value === 'macos' ? 'macOS' : value === 'linux' ? 'Linux' : value
  if (type === 'timezone' && value === 'system') return `跟随系统时区 (${getSystemTimezone()})`
  if (type === 'cpu') return `${value} 核`
  if (type === 'webrtc') return value === 'disable_non_proxied_udp' ? '禁用非代理 UDP' : value === 'default_public_interface_only' ? '仅公网接口' : value === 'default_public_and_private_interfaces' ? '公网 + 私网接口' : value
  if (type === 'noise') return value === '1' ? '显式开启' : value === '0' ? '显式关闭' : '默认（使用全局默认）'
  return value
}

function FishField({ label, children, hint }: { label: string; children: React.ReactNode; hint?: string }) {
  return <div className="fish-field"><label>{label}</label>{children}{hint ? <small>{hint}</small> : null}</div>
}

export function FishFingerprintEditor({ value, onChange }: FishFingerprintEditorProps) {
  const [config, setConfig] = useState<FingerprintConfig>(() => deserialize(value))
  const [advancedOpen, setAdvancedOpen] = useState(false)
  const [helpOpen, setHelpOpen] = useState(false)
  const [capabilitiesOpen, setCapabilitiesOpen] = useState(false)
  const [seedConfirmOpen, setSeedConfirmOpen] = useState(false)

  useEffect(() => { setConfig(deserialize(value)) }, [value.join('\n')])

  const validation = useMemo(() => validateFingerprintArgs(value), [value])
  const update = (patch: Partial<FingerprintConfig>) => {
    const next = { ...config, ...patch }
    setConfig(next)
    onChange(serialize(next))
  }
  const applyPreset = (presetId: string) => {
    const preset = FINGERPRINT_PRESETS.find((item) => item.id === presetId)
    if (!preset) return
    const next: FingerprintConfig = { ...preset.config, seed: randomFingerprintSeed(), unknownArgs: config.unknownArgs }
    setConfig(next)
    onChange(serialize(next))
  }
  const applyPersona = (personaId: string) => {
    const persona = FINGERPRINT_PERSONAS.find((item) => item.id === personaId)
    if (!persona) return
    const next: FingerprintConfig = { ...buildFingerprintConfigFromPersona(persona), unknownArgs: config.unknownArgs }
    setConfig(next)
    onChange(serialize(next))
  }
  const toggleSpoofing = (key: string, enabled: boolean) => {
    const current = config.disableSpoofing ?? []
    const next = enabled ? [...current, key] : current.filter((item) => item !== key)
    update({ disableSpoofing: next.length ? next : undefined })
  }
  const advancedText = serialize(config).join('\n')
  const validationTone = validation.valid ? (validation.issues.some((issue) => issue.level === 'warning') ? 'warn' : 'ok') : 'error'

  return (
    <div className="fish-fingerprint-editor">
      <div className={`fish-validation ${validationTone}`}>
        <strong>{validation.valid ? (validation.issues.some((issue) => issue.level === 'warning') ? '配置可用，有提示' : '配置有效') : '配置需要修正'}</strong>
        {validation.issues.length > 0 ? <div>{validation.issues.slice(0, 4).map((issue, index) => <span key={`${issue.level}-${index}`}>{issue.message}</span>)}{validation.issues.length > 4 ? <span>还有 {validation.issues.length - 4} 项，请在高级模式中检查。</span> : null}</div> : null}
      </div>

      <section className="fish-subpanel fish-seed-panel">
        <div className="fish-subpanel-head"><strong>指纹种子</strong><button className="fish-link-btn" type="button" onClick={() => setSeedConfirmOpen(true)}><RefreshCw size={16} strokeWidth={1.8} />重新生成</button></div>
        <input className="fish-input" value={config.seed ?? ''} onChange={(event) => update({ seed: event.target.value || undefined })} placeholder="留空则启动时按实例 ID 自动生成" />
      </section>

      <section className="fish-subpanel">
        <div className="fish-subpanel-head"><strong>快速生成</strong><button className="fish-link-btn" type="button" onClick={() => setCapabilitiesOpen(true)}>查看能力覆盖</button></div>
        <div className="fish-form-grid two">
          <FishField label="快速预设"><select className="fish-select" value="" onChange={(event) => applyPreset(event.target.value)}><option value="">选择预设…</option>{FINGERPRINT_PRESETS.map((preset) => <option key={preset.id} value={preset.id}>{preset.name}</option>)}</select></FishField>
          <FishField label="高级画像"><select className="fish-select" value="" onChange={(event) => applyPersona(event.target.value)}><option value="">选择高级画像…</option>{FINGERPRINT_PERSONAS.map((persona) => <option key={persona.id} value={persona.id}>{persona.name}</option>)}</select></FishField>
        </div>
      </section>

      <section className="fish-subpanel">
        <div className="fish-subpanel-head"><strong>身份与定位</strong></div>
        <div className="fish-form-grid three">
          <FishField label="浏览器品牌"><select className="fish-select" value={config.brand ?? ''} onChange={(event) => update({ brand: event.target.value || undefined })}>{BRAND_OPTIONS.map((item) => <option key={item || 'none'} value={item}>{optionLabel(item, 'brand')}</option>)}</select></FishField>
          <FishField label="品牌版本"><input className="fish-input" list="fish-brand-version-options" value={config.brandVersion ?? ''} onChange={(event) => update({ brandVersion: event.target.value || undefined })} placeholder="默认跟随内核" /><datalist id="fish-brand-version-options">{BRAND_VERSION_OPTIONS.filter(Boolean).map((item) => <option key={item} value={item} />)}</datalist></FishField>
          <FishField label="平台"><select className="fish-select" value={config.platform ?? ''} onChange={(event) => update({ platform: event.target.value || undefined })}>{PLATFORM_OPTIONS.map((item) => <option key={item || 'none'} value={item}>{optionLabel(item, 'platform')}</option>)}</select></FishField>
          <FishField label="系统版本"><input className="fish-input" list="fish-platform-version-options" value={config.platformVersion ?? ''} onChange={(event) => update({ platformVersion: event.target.value || undefined })} placeholder="如 15.2.0" /><datalist id="fish-platform-version-options">{PLATFORM_VERSION_OPTIONS.filter(Boolean).map((item) => <option key={item} value={item} />)}</datalist></FishField>
          <FishField label="语言"><select className="fish-select" value={config.lang ?? ''} onChange={(event) => update({ lang: event.target.value || undefined, acceptLang: undefined })}>{LANG_OPTIONS.map((item) => <option key={item || 'none'} value={item}>{optionLabel(item, 'lang')}</option>)}</select></FishField>
          <FishField label="语言列表"><input className="fish-input" value={config.acceptLang ?? ''} onChange={(event) => update({ acceptLang: event.target.value || undefined })} placeholder={config.lang ? buildAcceptLanguage(config.lang) : '如 ja-JP,ja'} /></FishField>
          <FishField label="时区"><select className="fish-select" value={config.timezone ?? ''} onChange={(event) => update({ timezone: event.target.value || undefined })}>{TIMEZONE_OPTIONS.map((item) => <option key={item || 'none'} value={item}>{optionLabel(item, 'timezone')}</option>)}</select></FishField>
        </div>
      </section>

      <section className="fish-subpanel">
        <div className="fish-subpanel-head"><strong>设备与网络</strong></div>
        <div className="fish-form-grid three">
          <FishField label="窗口大小"><select className="fish-select" value={config.resolution ?? ''} onChange={(event) => update({ resolution: event.target.value || undefined })}><option value="">不设置</option>{PRESET_RESOLUTIONS.map((item) => <option value={item} key={item}>{item.replace(',', ' × ')}</option>)}<option value="custom">自定义…</option></select></FishField>
          {config.resolution === 'custom' ? <FishField label="自定义分辨率"><input className="fish-input" value={config.customResolution ?? ''} onChange={(event) => update({ customResolution: event.target.value || undefined })} placeholder="1920,1080" /></FishField> : null}
          <FishField label="CPU 核心数"><select className="fish-select" value={config.hardwareConcurrency ?? ''} onChange={(event) => update({ hardwareConcurrency: event.target.value || undefined })}>{CPU_OPTIONS.map((item) => <option key={item || 'none'} value={item}>{optionLabel(item, 'cpu')}</option>)}</select></FishField>
          <FishField label="WebRTC 策略"><select className="fish-select" value={config.webrtcPolicy ?? ''} onChange={(event) => update({ webrtcPolicy: event.target.value || undefined })}>{WEBRTC_OPTIONS.map((item) => <option key={item || 'none'} value={item}>{optionLabel(item, 'webrtc')}</option>)}</select></FishField>
        </div>
      </section>

      <section className="fish-subpanel">
        <div className="fish-subpanel-head"><strong>兼容伪装</strong><span>按当前内核实际支持范围保存</span></div>
        <div className="fish-form-grid two">
          <FishField label="Canvas 噪声"><select className="fish-select" value={config.canvasNoise ?? ''} onChange={(event) => update({ canvasNoise: event.target.value || undefined })}>{NOISE_OPTIONS.map((item) => <option key={item || 'none'} value={item}>{optionLabel(item, 'noise')}</option>)}</select></FishField>
          <FishField label="ClientRects 噪声"><select className="fish-select" value={config.clientRectsNoise ?? ''} onChange={(event) => update({ clientRectsNoise: event.target.value || undefined })}>{NOISE_OPTIONS.map((item) => <option key={item || 'none'} value={item}>{optionLabel(item, 'noise')}</option>)}</select></FishField>
        </div>
        <div className="fish-setting-list compact">
          {SPOOFING_OPTIONS.map((item) => <div className="fish-setting-row" key={item.value}><div><strong>禁用 {item.label} 伪装</strong><span>关闭开关时保持该项伪装。</span></div><FishSwitch checked={(config.disableSpoofing ?? []).includes(item.value)} onChange={(checked) => toggleSpoofing(item.value, checked)} label={`禁用 ${item.label} 伪装`} /></div>)}
        </div>
      </section>

      <section className="fish-collapsible">
        <div className="fish-collapsible-trigger"><button className="fish-collapsible-main" type="button" onClick={() => setAdvancedOpen((current) => !current)}><span>高级模式（原始参数）</span>{advancedOpen ? <ChevronUp size={16} strokeWidth={1.8} /> : <ChevronDown size={16} strokeWidth={1.8} />}</button><button className="fish-help-inline" type="button" aria-label="查看原始参数说明" onClick={() => setHelpOpen(true)}><HelpCircle size={16} strokeWidth={1.8} /></button></div>
        {advancedOpen ? <div className="fish-collapsible-body"><p>未建模参数会保留；本地实测无效的旧版细项保存时移除。</p><textarea className="fish-textarea" rows={7} value={advancedText} onChange={(event) => { const args = event.target.value.split('\n').map((item) => item.trim()).filter(Boolean); const parsed = deserialize(args); setConfig(parsed); onChange(serialize(parsed)) }} /></div> : null}
      </section>

      <FishConfirm open={seedConfirmOpen} title="重新生成指纹种子" content="重新生成后，会影响当前内核支持的随机指纹项；具体生效范围以检测结果为准。" confirmText="重新生成" onClose={() => setSeedConfirmOpen(false)} onConfirm={() => { update({ seed: randomFingerprintSeed() }); setSeedConfirmOpen(false) }} />

      <FishModal open={capabilitiesOpen} title="指纹能力覆盖" onClose={() => setCapabilitiesOpen(false)} wide footer={<button className="fish-btn" type="button" onClick={() => setCapabilitiesOpen(false)}>关闭</button>}>
        <div className="fish-report-table capability"><div className="fish-report-head"><span>能力</span><span>模式</span><span>覆盖</span></div>{FINGERPRINT_CAPABILITIES.map((item) => <div className="fish-report-row" key={item.id}><strong>{item.name}</strong><span>{capabilityModeLabel(item.mode)}</span><span>{item.coverage}</span></div>)}</div>
      </FishModal>

      <FishModal open={helpOpen} title="原始参数使用方式" onClose={() => setHelpOpen(false)} extraWide footer={<button className="fish-btn" type="button" onClick={() => setHelpOpen(false)}>关闭</button>}>
        <div className="fish-report-table help"><div className="fish-report-head"><span>参数</span><span>用途</span><span>示例</span></div>{ADVANCED_ARG_HELP_ROWS.map(([arg, usage, example]) => <div className="fish-report-row" key={arg}><code>{arg}</code><span>{usage}</span><code>{example}</code></div>)}</div>
      </FishModal>
    </div>
  )
}
