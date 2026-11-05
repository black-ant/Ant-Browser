// 指纹参数序列化/反序列化工具

import type { FingerprintPersona } from './fingerprintCapabilities'

/**
 * 获取系统当前时区
 * @returns IANA 时区标识符，如 "Asia/Shanghai"
 */
export function getSystemTimezone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone
  } catch {
    return 'UTC'
  }
}

export interface FingerprintConfig {
  // 指纹种子（核心）
  seed?: string            // --fingerprint=<seed>  控制所有随机噪声的根种子

  // 基础身份
  brand?: string           // --fingerprint-brand=
  brandVersion?: string    // --fingerprint-brand-version=
  platform?: string        // --fingerprint-platform=
  platformVersion?: string // --fingerprint-platform-version=
  lang?: string            // --lang=
  acceptLang?: string      // --accept-lang=
  timezone?: string        // --timezone=

  // 屏幕与窗口
  resolution?: string      // --window-size=
  customResolution?: string // 当 resolution === 'custom' 时使用

  // 硬件信息
  hardwareConcurrency?: string  // --fingerprint-hardware-concurrency=
  colorDepth?: string           // 兼容解析：本地 Chrom-144 实测无效，保存时不再输出
  deviceMemory?: string         // 兼容解析：本地 Chrom-144 实测无效，保存时不再输出
  touchPoints?: string          // 兼容解析：本地 Chrom-144 实测无效，保存时不再输出

  // 网络与隐私
  webrtcPolicy?: string         // --disable-non-proxied-udp 或 --webrtc-ip-handling-policy=
  disableSpoofing?: string[]    // --disable-spoofing=font,audio,canvas,clientrects,gpu
  doNotTrack?: string           // 兼容解析：本地 Chrom-144 实测无效，保存时不再输出
  mediaDevices?: string         // 兼容解析：本地 Chrom-144 实测无效，保存时不再输出

  // 兼容细项：已验证噪声项会转换到 Chrom-144 真实开关，其余原样保留
  canvasNoise?: string          // 1 -> --fingerprinting-canvas-image-data-noise
  audioNoise?: string           // 兼容解析：本地 Chrom-144 实测无效，保存时不再输出
  clientRectsNoise?: string     // 1 -> --fingerprinting-client-rects-noise
  fontList?: string             // 兼容解析：本地 Chrom-144 实测无效，保存时不再输出
  webglVendor?: string          // 兼容解析：本地 Chrom-144 实测无效，保存时不再输出
  webglRenderer?: string        // 兼容解析：本地 Chrom-144 实测无效，保存时不再输出

  unknownArgs?: string[]        // 无法识别的原始参数，原样保留
}

export const PRESET_RESOLUTIONS = ['1920,1080', '1440,900', '1366,768', '2560,1440', '1280,800', '1600,900']

const DEFAULT_WINDOW_WIDTH_RATIO = 0.42
const DEFAULT_WINDOW_MAX_WIDTH = 1440
const DEFAULT_WINDOW_MIN_WIDTH = 960
const DEFAULT_WINDOW_MIN_HEIGHT = 540
const DEFAULT_WINDOW_FALLBACK_SCREEN_WIDTH = 1920
const DEFAULT_WINDOW_FALLBACK_SCREEN_HEIGHT = 1080

type WindowSizeScreenLike = Partial<Pick<Screen, 'availWidth' | 'availHeight' | 'width' | 'height'>>

function positiveNumber(value: unknown): number {
  return typeof value === 'number' && Number.isFinite(value) && value > 0 ? value : 0
}

function clampNumber(value: number, min: number, max: number): number {
  return Math.min(Math.max(value, min), max)
}

export function buildAdaptiveDefaultWindowSize(screenInfo?: WindowSizeScreenLike): string {
  const resolvedScreen = screenInfo ?? (typeof window !== 'undefined' ? window.screen : undefined)
  const availableWidth = positiveNumber(resolvedScreen?.availWidth) || positiveNumber(resolvedScreen?.width) || DEFAULT_WINDOW_FALLBACK_SCREEN_WIDTH
  const availableHeight = positiveNumber(resolvedScreen?.availHeight) || positiveNumber(resolvedScreen?.height) || DEFAULT_WINDOW_FALLBACK_SCREEN_HEIGHT
  const width = Math.round(clampNumber(availableWidth * DEFAULT_WINDOW_WIDTH_RATIO, DEFAULT_WINDOW_MIN_WIDTH, DEFAULT_WINDOW_MAX_WIDTH))
  const maxHeight = Math.max(DEFAULT_WINDOW_MIN_HEIGHT, Math.floor(availableHeight * 0.82))
  const height = Math.round(clampNumber(width * 9 / 16, DEFAULT_WINDOW_MIN_HEIGHT, maxHeight))
  return `${width},${height}`
}

export function withAdaptiveDefaultWindowSize(args: string[]): string[] {
  const normalizedArgs = (args || []).map(item => item.trim()).filter(Boolean)
  if (normalizedArgs.some(arg => arg.toLowerCase().startsWith('--window-size='))) return normalizedArgs
  return [...normalizedArgs, `--window-size=${buildAdaptiveDefaultWindowSize()}`]
}

// CLI 参数前缀 → FingerprintConfig 字段映射
export const KEY_MAP: Record<string, keyof FingerprintConfig> = {
  '--fingerprint': 'seed',
  '--fingerprint-brand': 'brand',
  '--fingerprint-brand-version': 'brandVersion',
  '--fingerprint-platform': 'platform',
  '--fingerprint-platform-version': 'platformVersion',
  '--lang': 'lang',
  '--accept-lang': 'acceptLang',
  '--timezone': 'timezone',
  '--window-size': 'resolution',
  '--fingerprint-hardware-concurrency': 'hardwareConcurrency',
  '--fingerprint-color-depth': 'colorDepth',
  '--fingerprint-device-memory': 'deviceMemory',
  '--fingerprint-touch-points': 'touchPoints',
  '--webrtc-ip-handling-policy': 'webrtcPolicy',
  '--disable-spoofing': 'disableSpoofing',
  '--fingerprint-do-not-track': 'doNotTrack',
  '--fingerprint-media-devices': 'mediaDevices',
  '--fingerprint-canvas-noise': 'canvasNoise',
  '--fingerprinting-canvas-image-data-noise': 'canvasNoise',
  '--fingerprint-audio-noise': 'audioNoise',
  '--fingerprint-client-rects-noise': 'clientRectsNoise',
  '--fingerprinting-client-rects-noise': 'clientRectsNoise',
  '--fingerprint-font-list': 'fontList',
  '--fingerprint-fonts': 'fontList',
  '--fingerprint-webgl-vendor': 'webglVendor',
  '--fingerprint-webgl-renderer': 'webglRenderer',
}

export interface FingerprintValidationIssue {
  level: 'error' | 'warning' | 'info'
  message: string
}

export interface FingerprintValidationResult {
  valid: boolean
  issues: FingerprintValidationIssue[]
}

const FLAG_MAP: Record<string, keyof FingerprintConfig> = {
  '--disable-non-proxied-udp': 'webrtcPolicy',
  '--fingerprinting-canvas-image-data-noise': 'canvasNoise',
  '--fingerprinting-client-rects-noise': 'clientRectsNoise',
}

const KERNEL_SPOOFING_KEYS = new Set(['font', 'audio', 'canvas', 'clientrects', 'gpu'])
const KERNEL_BRANDS = new Set(['Chrome', 'Edge', 'Opera', 'Vivaldi'])
const KERNEL_PLATFORMS = new Set(['windows', 'linux', 'macos'])
const KERNEL_WEBRTC_POLICIES = new Set(['disable_non_proxied_udp', 'default_public_interface_only', 'default_public_and_private_interfaces'])
const KERNEL_SUPPORTED_ARG_KEYS = new Set([
  '--fingerprint',
  '--fingerprint-brand',
  '--fingerprint-brand-version',
  '--fingerprint-platform',
  '--fingerprint-platform-version',
  '--fingerprint-hardware-concurrency',
  '--fingerprinting-canvas-image-data-noise',
  '--fingerprinting-client-rects-noise',
  '--disable-non-proxied-udp',
  '--disable-spoofing',
  '--lang',
  '--accept-lang',
  '--timezone',
  '--window-size',
  '--webrtc-ip-handling-policy',
])


const LEGACY_FINGERPRINT_ARG_KEYS = new Set([
  '--fingerprint-gpu-vendor',
  '--fingerprint-gpu-renderer',
  '--disable-gpu-fingerprint',
])

const CONVERTED_FINGERPRINT_ARG_KEYS = new Set([
  '--fingerprint-canvas-noise',
  '--fingerprint-client-rects-noise',
])

const NO_EFFECT_FINGERPRINT_ARG_KEYS = new Set([
  '--fingerprint-color-depth',
  '--fingerprint-device-memory',
  '--fingerprint-touch-points',
  '--fingerprint-do-not-track',
  '--fingerprint-media-devices',
  '--fingerprint-audio-noise',
  '--fingerprint-font-list',
  '--fingerprint-fonts',
  '--fingerprint-webgl-vendor',
  '--fingerprint-webgl-renderer',
  '--fingerprint-screen-width',
  '--fingerprint-screen-height',
  '--fingerprint-device-scale-factor',
  '--fingerprint-location',
  '--fingerprinting-canvas-measuretext-noise',
])

// FingerprintConfig → string[]
export function serialize(config: FingerprintConfig): string[] {
  const args: string[] = []
  if (config.seed) args.push(`--fingerprint=${config.seed}`)
  if (config.brand) args.push(`--fingerprint-brand=${config.brand}`)
  if (config.brandVersion) args.push(`--fingerprint-brand-version=${config.brandVersion}`)
  if (config.platform) args.push(`--fingerprint-platform=${normalizeFingerprintPlatform(config.platform)}`)
  if (config.platformVersion) args.push(`--fingerprint-platform-version=${config.platformVersion}`)
  if (config.lang) {
    args.push(`--lang=${config.lang}`)
    args.push(`--accept-lang=${config.acceptLang || buildAcceptLanguage(config.lang)}`)
  } else if (config.acceptLang) {
    args.push(`--accept-lang=${config.acceptLang}`)
  }
  if (config.timezone) {
    // 如果是 system，替换为实际系统时区
    const tz = config.timezone === 'system' ? getSystemTimezone() : config.timezone
    args.push(`--timezone=${tz}`)
  }

  const res = config.resolution === 'custom' ? config.customResolution : config.resolution
  if (res) args.push(`--window-size=${res}`)

  if (config.hardwareConcurrency) args.push(`--fingerprint-hardware-concurrency=${config.hardwareConcurrency}`)
  if (config.webrtcPolicy === 'disable_non_proxied_udp') {
    args.push('--disable-non-proxied-udp')
  } else if (config.webrtcPolicy) {
    args.push(`--webrtc-ip-handling-policy=${config.webrtcPolicy}`)
  }
  if (isEnabledFlagValue(config.canvasNoise)) {
    args.push('--fingerprinting-canvas-image-data-noise')
  } else if (isDisabledFlagValue(config.canvasNoise)) {
    args.push('--fingerprinting-canvas-image-data-noise=0')
  }
  if (isEnabledFlagValue(config.clientRectsNoise)) {
    args.push('--fingerprinting-client-rects-noise')
  } else if (isDisabledFlagValue(config.clientRectsNoise)) {
    args.push('--fingerprinting-client-rects-noise=0')
  }
  const disableSpoofing = normalizeDisableSpoofing(config.disableSpoofing)
  if (disableSpoofing.length) args.push(`--disable-spoofing=${disableSpoofing.join(',')}`)

  return [...args, ...(config.unknownArgs ?? [])]
}

// string[] → FingerprintConfig
export function deserialize(args: string[]): FingerprintConfig {
  const config: FingerprintConfig = { unknownArgs: [] }

  for (const arg of args) {
    const eqIdx = arg.indexOf('=')
    if (eqIdx === -1) {
      const field = FLAG_MAP[arg]
      if (field === 'webrtcPolicy') {
        config.webrtcPolicy = 'disable_non_proxied_udp'
      } else if (field === 'canvasNoise' || field === 'clientRectsNoise') {
        ;(config as Record<string, unknown>)[field] = '1'
      } else {
        config.unknownArgs!.push(arg)
      }
      continue
    }
    const key = arg.slice(0, eqIdx)
    const val = arg.slice(eqIdx + 1)
    const field = KEY_MAP[key]

    if (!field) {
      config.unknownArgs!.push(arg)
      continue
    }

    if (field === 'resolution') {
      if (PRESET_RESOLUTIONS.includes(val)) {
        config.resolution = val
      } else {
        config.resolution = 'custom'
        config.customResolution = val
      }
    } else if (field === 'platform') {
      config.platform = normalizeFingerprintPlatform(val)
    } else if (field === 'disableSpoofing') {
      config.disableSpoofing = normalizeDisableSpoofing(val.split(','))
    } else {
      (config as Record<string, unknown>)[field] = val
    }
  }

  return config
}

export function normalizeFingerprintPlatform(platform: string): string {
  const normalized = platform.trim().toLowerCase()
  return normalized === 'mac' ? 'macos' : normalized
}

export function normalizeDisableSpoofing(values?: string[]): string[] {
  const result: string[] = []
  for (const value of values || []) {
    const normalized = value.trim().toLowerCase()
    if (KERNEL_SPOOFING_KEYS.has(normalized) && !result.includes(normalized)) result.push(normalized)
  }
  return result
}

export function validateFingerprintArgs(rawArgs: string[]): FingerprintValidationResult {
  const config = deserialize(rawArgs || [])
  const issues: FingerprintValidationIssue[] = []

  for (const arg of rawArgs || []) {
    const trimmed = arg.trim()
    if (!trimmed.startsWith('--')) continue
    const key = argKey(trimmed)
    if (LEGACY_FINGERPRINT_ARG_KEYS.has(key)) {
      issues.push({ level: 'warning', message: `${key} 配置中保留，但当前适配矩阵不作为独立运行参数；对应能力按 --fingerprint 种子或 --disable-spoofing 处理` })
    } else if (CONVERTED_FINGERPRINT_ARG_KEYS.has(key)) {
      issues.push({ level: 'info', message: `${key} 保存时会转换为当前 Chrom-144 已验证的噪声开关` })
    } else if (NO_EFFECT_FINGERPRINT_ARG_KEYS.has(key)) {
      issues.push({ level: 'warning', message: `${key} 本地 Chrom-144 实测无效，保存后不会作为运行参数传递` })
    } else if (!KERNEL_SUPPORTED_ARG_KEYS.has(key)) {
      issues.push({ level: 'info', message: `${key} 未在当前指纹面板建模，会作为原始参数保留` })
    }
  }

  if (config.seed && !isUint32Seed(config.seed)) {
    issues.push({ level: 'error', message: '指纹种子必须是 1 到 2147483647 的整数' })
  }
  if (!config.seed) {
    issues.push({ level: 'info', message: '未显式设置种子时，启动会按实例 ID 自动生成' })
  }
  if (config.brand && !KERNEL_BRANDS.has(config.brand)) {
    issues.push({ level: 'error', message: `浏览器品牌仅支持 ${Array.from(KERNEL_BRANDS).join(' / ')}` })
  }
  if (config.brandVersion && !isDottedVersion(config.brandVersion)) {
    issues.push({ level: 'warning', message: '品牌版本建议使用数字点分格式，例如 144.0.7559.132' })
  }
  if (config.platform && !KERNEL_PLATFORMS.has(normalizeFingerprintPlatform(config.platform))) {
    issues.push({ level: 'error', message: '平台仅支持 windows / linux / macos' })
  }
  if (config.platformVersion && !isSafeVersionText(config.platformVersion)) {
    issues.push({ level: 'warning', message: '系统版本建议只包含字母、数字、点、横线和下划线' })
  }
  if (config.lang && !isLanguageTag(config.lang)) {
    issues.push({ level: 'error', message: '语言格式不正确，例如 zh-CN、en-US、ja-JP' })
  }
  if (config.acceptLang && !isAcceptLanguage(config.acceptLang)) {
    issues.push({ level: 'error', message: 'Accept-Language 格式不正确，例如 zh-CN,zh 或 en-US,en' })
  }
  if (config.timezone && config.timezone !== 'system' && !isKnownTimezone(config.timezone)) {
    issues.push({ level: 'warning', message: '时区未被当前系统 Intl 识别，启动后可能不会按预期生效' })
  }
  if (config.resolution === 'custom' && !isWindowSize(config.customResolution || '')) {
    issues.push({ level: 'error', message: '自定义窗口大小必须是 宽,高，例如 1920,1080' })
  } else if (config.resolution && config.resolution !== 'custom' && !isWindowSize(config.resolution)) {
    issues.push({ level: 'error', message: '窗口大小必须是 宽,高，例如 1920,1080' })
  }
  if (config.hardwareConcurrency && !isPositiveIntInRange(config.hardwareConcurrency, 1, 128)) {
    issues.push({ level: 'error', message: 'CPU 核心数必须是 1 到 128 的整数' })
  }
  if (config.webrtcPolicy && !KERNEL_WEBRTC_POLICIES.has(config.webrtcPolicy)) {
    issues.push({ level: 'warning', message: 'WebRTC 策略不是面板内置值，请确认当前内核是否支持' })
  }
  if (normalizeDisableSpoofing(config.disableSpoofing).length !== (config.disableSpoofing || []).length) {
    issues.push({ level: 'warning', message: '排除伪装项仅支持 font、audio、canvas、clientrects、gpu' })
  }

  return {
    valid: !issues.some(issue => issue.level === 'error'),
    issues,
  }
}

function isEnabledFlagValue(value?: string): boolean {
  const normalized = String(value ?? '').trim().toLowerCase()
  return normalized === '1' || normalized === 'true' || normalized === 'yes' || normalized === 'on'
}

function isDisabledFlagValue(value?: string): boolean {
  const normalized = String(value ?? '').trim().toLowerCase()
  return normalized === '0' || normalized === 'false' || normalized === 'no' || normalized === 'off'
}

function argKey(arg: string): string {
  const eqIndex = arg.indexOf('=')
  return (eqIndex > 0 ? arg.slice(0, eqIndex) : arg).toLowerCase()
}

function isUint32Seed(value: string): boolean {
  if (!/^\d+$/.test(value.trim())) return false
  const parsed = Number(value)
  return Number.isSafeInteger(parsed) && parsed >= 1 && parsed <= 2147483647
}

function isDottedVersion(value: string): boolean {
  return /^\d+(?:\.\d+){0,4}$/.test(value.trim())
}

function isSafeVersionText(value: string): boolean {
  return /^[A-Za-z0-9._-]+$/.test(value.trim())
}

function isLanguageTag(value: string): boolean {
  return /^[A-Za-z]{2,3}(?:[-_][A-Za-z0-9]{2,8}){0,2}$/.test(value.trim())
}

function isAcceptLanguage(value: string): boolean {
  return value.split(',').map(item => item.trim()).filter(Boolean).every(isLanguageTag)
}

function isWindowSize(value: string): boolean {
  const match = value.trim().match(/^(\d{3,5}),(\d{3,5})$/)
  if (!match) return false
  const width = Number(match[1])
  const height = Number(match[2])
  return width >= 200 && height >= 200
}

function isPositiveIntInRange(value: string, min: number, max: number): boolean {
  if (!/^\d+$/.test(value.trim())) return false
  const parsed = Number(value)
  return Number.isSafeInteger(parsed) && parsed >= min && parsed <= max
}

function isKnownTimezone(value: string): boolean {
  try {
    Intl.DateTimeFormat(undefined, { timeZone: value.trim() })
    return true
  } catch {
    return false
  }
}

// 生成随机指纹种子（32位正整数）
export function randomFingerprintSeed(): string {
  return String(Math.floor(Math.random() * 2147483647) + 1)
}

export function buildFingerprintConfigFromPersona(persona: FingerprintPersona): FingerprintConfig {
  return {
    seed: randomFingerprintSeed(),
    brand: persona.brand,
    brandVersion: persona.brandVersion,
    platform: persona.platform,
    platformVersion: persona.platformVersion,
    lang: persona.lang,
    acceptLang: buildAcceptLanguage(persona.lang),
    timezone: persona.timezone,
    resolution: persona.resolution,
    hardwareConcurrency: persona.hardwareConcurrency,
    webrtcPolicy: 'disable_non_proxied_udp',
    canvasNoise: '1',
    clientRectsNoise: '1',
  }
}

const EFFECTIVE_RUNTIME_NOISE_CONFIG: Pick<FingerprintConfig, 'canvasNoise' | 'clientRectsNoise'> = {
  canvasNoise: '1',
  clientRectsNoise: '1',
}

// ─── 预设指纹配置 ────────────────────────────────────────────────────────────

export interface FingerprintPreset {
  id: string
  name: string
  description: string
  config: Partial<FingerprintConfig>
}

export const FINGERPRINT_PRESETS: FingerprintPreset[] = [
  {
    id: 'win-chrome-office',
    name: 'Windows / Chrome / 办公',
    description: '模拟国内办公室 Windows 用户，中文环境，1920x1080',
    config: {
      brand: 'Chrome',
      platform: 'windows',
      lang: 'zh-CN',
      timezone: 'Asia/Shanghai',
      resolution: '1920,1080',
      hardwareConcurrency: '8',
      webrtcPolicy: 'disable_non_proxied_udp',
      ...EFFECTIVE_RUNTIME_NOISE_CONFIG,
    },
  },
  {
    id: 'win-chrome-gaming',
    name: 'Windows / Chrome / 游戏主机',
    description: '模拟高配 Windows 用户，英文环境，2560x1440',
    config: {
      brand: 'Chrome',
      platform: 'windows',
      lang: 'en-US',
      timezone: 'America/New_York',
      resolution: '2560,1440',
      hardwareConcurrency: '16',
      webrtcPolicy: 'disable_non_proxied_udp',
      ...EFFECTIVE_RUNTIME_NOISE_CONFIG,
    },
  },
  {
    id: 'mac-chrome-designer',
    name: 'macOS / Chrome / 设计师',
    description: '模拟 Mac 设计师用户，中文环境，Retina 分辨率',
    config: {
      brand: 'Chrome',
      platform: 'macos',
      lang: 'zh-CN',
      timezone: 'Asia/Shanghai',
      resolution: '2560,1440',
      hardwareConcurrency: '10',
      webrtcPolicy: 'disable_non_proxied_udp',
      ...EFFECTIVE_RUNTIME_NOISE_CONFIG,
    },
  },
  {
    id: 'win-edge-enterprise',
    name: 'Windows / Edge / 企业',
    description: '模拟企业 Windows 用户，Edge 浏览器，标准配置',
    config: {
      brand: 'Edge',
      platform: 'windows',
      lang: 'zh-CN',
      timezone: 'Asia/Shanghai',
      resolution: '1366,768',
      hardwareConcurrency: '4',
      webrtcPolicy: 'default_public_interface_only',
      ...EFFECTIVE_RUNTIME_NOISE_CONFIG,
    },
  },
  {
    id: 'win-chrome-us-user',
    name: 'Windows / Chrome / 美国用户',
    description: '模拟美国普通用户，英文环境，1920x1080',
    config: {
      brand: 'Chrome',
      platform: 'windows',
      lang: 'en-US',
      timezone: 'America/Los_Angeles',
      resolution: '1920,1080',
      hardwareConcurrency: '8',
      webrtcPolicy: 'disable_non_proxied_udp',
      ...EFFECTIVE_RUNTIME_NOISE_CONFIG,
    },
  },
  {
    id: 'mac-chrome-jp',
    name: 'macOS / Chrome / 日本用户',
    description: '模拟日本 Mac 用户，Chrome 浏览器，日语环境',
    config: {
      brand: 'Chrome',
      platform: 'macos',
      lang: 'ja-JP',
      timezone: 'Asia/Tokyo',
      resolution: '1440,900',
      hardwareConcurrency: '8',
      webrtcPolicy: 'disable_non_proxied_udp',
      ...EFFECTIVE_RUNTIME_NOISE_CONFIG,
    },
  },
  {
    id: 'win-chrome-uk-office',
    name: 'Windows / Chrome / 英国-办公',
    description: '模拟英国办公室 Windows 用户，英文环境 (en-GB)',
    config: {
      brand: 'Chrome',
      platform: 'windows',
      lang: 'en-GB',
      timezone: 'Europe/London',
      resolution: '1920,1080',
      hardwareConcurrency: '8',
      webrtcPolicy: 'disable_non_proxied_udp',
      ...EFFECTIVE_RUNTIME_NOISE_CONFIG,
    },
  },
  {
    id: 'mac-chrome-us-edu',
    name: 'macOS / Chrome / 美国-教育',
    description: '模拟美国大学教育网 Mac 用户，英文环境 (en-US)',
    config: {
      brand: 'Chrome',
      platform: 'macos',
      lang: 'en-US',
      timezone: 'America/New_York',
      resolution: '1440,900',
      hardwareConcurrency: '8',
      webrtcPolicy: 'disable_non_proxied_udp',
      ...EFFECTIVE_RUNTIME_NOISE_CONFIG,
    },
  },
]

export function applyLocaleToFingerprintArgs(args: string[], lang: string, timezone: string): string[] {
  const nextConfig = deserialize(args || [])
  if (lang) {
    nextConfig.lang = lang
    nextConfig.acceptLang = buildAcceptLanguage(lang)
  }
  if (timezone) nextConfig.timezone = timezone
  return serialize(nextConfig)
}

export function buildAcceptLanguage(lang: string): string {
  const normalized = lang.trim()
  if (!normalized) return ''
  const base = normalized.split(/[-_]/)[0]
  if (!base || base.toLowerCase() === normalized.toLowerCase()) return normalized
  return `${normalized},${base}`
}
