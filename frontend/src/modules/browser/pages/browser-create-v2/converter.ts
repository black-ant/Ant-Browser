import { deserialize, randomFingerprintSeed } from '../../utils/fingerprintSerializer'
import type { BrowserProfileInput } from '../../types'
import type {
  BrowserLanguageMode,
  BrowserCore,
  FingerprintProfileConfig,
  FingerprintValueType,
  Platform,
  ProxyType,
  SearchEngineId,
  WebRTCPolicy,
  WebGPUMode,
  WindowMode,
} from './types'

export const DIRECT_PROXY_CONFIG = 'direct://'

interface ConvertOptions {
  defaultFingerprintArgs?: string[]
  defaultLaunchArgs?: string[]
  coreId?: string
  groupId?: string
}

interface NormalizeOptions {
  rejectUnsupportedBrowserCore?: boolean
}

const VALUE_TYPES: FingerprintValueType[] = ['real', 'random', 'custom']
const REAL_RANDOM_TYPES: FingerprintValueType[] = ['real', 'random']
const PLATFORMS: Platform[] = ['windows', 'mac', 'linux', 'android']
const SUPPORTED_BROWSER_CORES: BrowserCore[] = ['chrome']
const PROXY_TYPES: ProxyType[] = ['none', 'http', 'https', 'socks5']
const WEBRTC_POLICIES: WebRTCPolicy[] = ['default', 'disable_non_proxied_udp', 'disable_all']
const GEOLOCATION_PROMPTS = ['allow', 'block', 'prompt'] as const
const LANGUAGE_MODES: BrowserLanguageMode[] = ['auto', 'custom']
const WINDOW_MODES: WindowMode[] = ['custom', 'fullscreen']
const WEBGPU_MODES: WebGPUMode[] = ['match_webgl', 'real', 'disable']
const SEARCH_ENGINES: SearchEngineId[] = ['google', 'bing', 'duckduckgo', 'baidu']
const SEARCH_ENGINE_ARG_PREFIX = '--ant-search-engine='

function isRecord(value: unknown): value is Record<string, any> {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
}

function asOneOf<T extends string>(value: unknown, allowed: readonly T[], fallback: T): T {
  return typeof value === 'string' && allowed.includes(value as T) ? value as T : fallback
}

function setArg(args: string[], prefix: string, value?: string) {
  for (let i = args.length - 1; i >= 0; i--) {
    if (args[i].startsWith(prefix)) {
      args.splice(i, 1)
    }
  }
  if (value !== undefined && value !== '') {
    args.push(`${prefix}${value}`)
  }
}

function setSwitch(args: string[], switchName: string, enabled: boolean) {
  const normalized = switchName.toLowerCase()
  for (let i = args.length - 1; i >= 0; i--) {
    const item = args[i].trim().toLowerCase()
    if (item === normalized || item.startsWith(`${normalized}=`)) {
      args.splice(i, 1)
    }
  }
  if (enabled) {
    args.push(switchName)
  }
}

function uniqueList(items: string[], caseInsensitive = true): string[] {
  const seen = new Set<string>()
  const output: string[] = []
  for (const item of items) {
    const trimmed = item.trim()
    if (!trimmed) continue
    const key = caseInsensitive ? trimmed.toLowerCase() : trimmed
    if (seen.has(key)) continue
    seen.add(key)
    output.push(trimmed)
  }
  return output
}

function isFiniteNumber(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value)
}

function getArgValue(args: unknown, prefix: string): string | undefined {
  if (!Array.isArray(args)) return undefined
  for (let i = args.length - 1; i >= 0; i--) {
    const item = args[i]
    if (typeof item !== 'string') continue
    const trimmed = item.trim()
    if (trimmed.startsWith(prefix)) {
      return trimmed.slice(prefix.length)
    }
  }
  return undefined
}

function hasSwitch(args: unknown, switchName: string): boolean {
  if (!Array.isArray(args)) return false
  const normalized = switchName.toLowerCase()
  return args.some((item) => (
    typeof item === 'string' &&
    item.trim().toLowerCase() === normalized
  ))
}

function parseStartupArgs(value: unknown): string[] {
  if (Array.isArray(value)) {
    return value.filter((item): item is string => typeof item === 'string')
  }
  if (typeof value !== 'string') {
    return []
  }
  return value
    .split(/\r?\n/)
    .map(item => item.trim())
    .filter(Boolean)
}

function launchArgsToText(args: unknown): string {
  return parseStartupArgs(args)
    .map(normalizeLaunchArgForStartupText)
    .filter((item): item is string => Boolean(item))
    .join('\n')
}

function normalizeLaunchArgForStartupText(value: string): string | null {
  const item = value.trim()
  if (!item || /^https?:\/\//i.test(item)) return null

  const lower = item.toLowerCase()
  const structuredPrefixes = [
    '--lang=',
    '--accept-language=',
    '--window-size=',
    '--blink-settings=',
    '--autoplay-policy=',
    SEARCH_ENGINE_ARG_PREFIX,
  ]
  const structuredSwitches = [
    '--start-fullscreen',
    '--mute-audio',
    '--disable-gpu',
    '--no-sandbox',
  ]
  if (structuredPrefixes.some(prefix => lower.startsWith(prefix))) return null
  if (structuredSwitches.includes(lower)) return null

  if (lower.startsWith('--disable-features=')) {
    const features = item
      .slice('--disable-features='.length)
      .split(',')
      .map(part => part.trim())
      .filter(part => part && part.toLowerCase() !== 'webgpu')
    return features.length > 0 ? `--disable-features=${features.join(',')}` : null
  }

  return item
}

function hasDisableFeature(args: unknown, featureName: string): boolean {
  const normalizedFeature = featureName.toLowerCase()
  const featureValue = getArgValue(args, '--disable-features=')
  if (!featureValue) return false
  return featureValue
    .split(',')
    .map(item => item.trim().toLowerCase())
    .includes(normalizedFeature)
}

function setDisableFeature(args: string[], featureName: string, enabled: boolean) {
  const features: string[] = []
  for (let i = args.length - 1; i >= 0; i--) {
    const item = args[i].trim()
    if (!item.toLowerCase().startsWith('--disable-features=')) continue
    args.splice(i, 1)
    features.push(
      ...item
        .slice('--disable-features='.length)
        .split(',')
        .map(part => part.trim())
        .filter(Boolean)
    )
  }

  const next = uniqueList(
    enabled
      ? [...features.filter(item => item.toLowerCase() !== featureName.toLowerCase()), featureName]
      : features.filter(item => item.toLowerCase() !== featureName.toLowerCase())
  )

  if (next.length > 0) {
    args.push(`--disable-features=${next.join(',')}`)
  }
}

function imagesEnabledFromLaunchArgs(args: unknown): boolean {
  const blinkSettings = getArgValue(args, '--blink-settings=')
  if (!blinkSettings) return true
  return !blinkSettings
    .split(',')
    .map(part => part.trim().toLowerCase())
    .includes('imagesenabled=false')
}

function videoEnabledFromLaunchArgs(args: unknown): boolean {
  const autoplayPolicy = getArgValue(args, '--autoplay-policy=')
  return autoplayPolicy !== 'user-gesture-required'
}

function parseProxyConfig(proxyConfig: unknown): FingerprintProfileConfig['proxy'] {
  const base: FingerprintProfileConfig['proxy'] = {
    mode: 'manual',
    proxyId: '',
    type: 'none',
    host: '',
    port: '',
    username: '',
    password: '',
  }

  if (typeof proxyConfig !== 'string' || !proxyConfig.trim() || proxyConfig.trim() === DIRECT_PROXY_CONFIG) {
    return base
  }

  try {
    const url = new URL(proxyConfig)
    const type = asOneOf(url.protocol.replace(':', ''), PROXY_TYPES, 'none')
    if (type === 'none') return base
    return {
      mode: 'manual',
      proxyId: '',
      type,
      host: url.hostname,
      port: url.port,
      username: decodeURIComponent(url.username || ''),
      password: decodeURIComponent(url.password || ''),
    }
  } catch {
    return base
  }
}

function hasUnsupportedFirefoxConfig(input: Record<string, any>): boolean {
  if (typeof input.browserCore === 'string' && input.browserCore.toLowerCase() === 'firefox') {
    return true
  }
  if (Array.isArray(input.fingerprintArgs)) {
    const fp = deserialize(input.fingerprintArgs)
    return fp.brand?.toLowerCase() === 'firefox'
  }
  return false
}

function parseGeolocationArgs(args: unknown): Partial<FingerprintProfileConfig['basic']['geolocation']> {
  if (!Array.isArray(args)) {
    return {}
  }

  const result: Partial<FingerprintProfileConfig['basic']['geolocation']> = {}
  for (const item of args) {
    if (typeof item !== 'string') continue
    if (item.startsWith('--ant-geolocation=')) {
      const [lat, lon, accuracy] = item
        .slice('--ant-geolocation='.length)
        .split(',')
        .map(part => Number(part.trim()))
      if (Number.isFinite(lat) && Number.isFinite(lon)) {
        result.type = 'custom'
        result.latitude = lat
        result.longitude = lon
      }
      if (Number.isFinite(accuracy) && accuracy > 0) {
        result.accuracy = accuracy
      }
    }
    if (item.startsWith('--ant-geolocation-permission=')) {
      result.prompt = asOneOf(item.slice('--ant-geolocation-permission='.length), GEOLOCATION_PROMPTS, 'prompt')
    }
    if (item.startsWith('--ant-geolocation-mode=')) {
      const mode = item.slice('--ant-geolocation-mode='.length).trim()
      if (mode === 'real') result.type = 'real'
    }
  }
  return result
}

function parseTimezoneMode(args: unknown): FingerprintValueType | undefined {
  if (!Array.isArray(args)) {
    return undefined
  }
  for (const item of args) {
    if (typeof item !== 'string') continue
    if (!item.startsWith('--ant-timezone-mode=')) continue
    const mode = item.slice('--ant-timezone-mode='.length).trim()
    if (mode === 'real') return 'real'
  }
  return undefined
}

function migrateBrowserProfile(input: Record<string, any>): Partial<FingerprintProfileConfig> | null {
  if (!Array.isArray(input.fingerprintArgs) && !Array.isArray(input.launchArgs) && !('proxyConfig' in input)) {
    return null
  }

  const fp = deserialize(Array.isArray(input.fingerprintArgs) ? input.fingerprintArgs : [])
  const geolocation = parseGeolocationArgs(input.fingerprintArgs)
  const timezoneMode = parseTimezoneMode(input.fingerprintArgs)
  const resolutionValue = fp.resolution === 'custom' ? fp.customResolution || '' : fp.resolution || ''
  const [width, height] = resolutionValue
    .split(',')
    .map((part) => Number(part.trim()))

  return {
    profileName: typeof input.profileName === 'string' ? input.profileName : '',
    platform: asOneOf(fp.platform, PLATFORMS, 'windows'),
    browserCore: 'chrome',
    coreVersion: '',
    userAgent: {
      type: 'random',
      value: '',
    },
    proxy: parseProxyConfig(input.proxyConfig),
    startupUrls: Array.isArray(input.launchArgs)
      ? input.launchArgs.filter((item: unknown) => typeof item === 'string' && /^https?:\/\//i.test(item.trim()))
      : [],
    basic: {
      language: fp.lang || 'auto',
      uiLanguage: {
        mode: getArgValue(input.launchArgs, '--accept-language=') ? 'custom' : 'auto',
        value: getArgValue(input.launchArgs, '--accept-language=') || fp.lang || 'zh-CN,zh',
      },
      timezone: {
        type: timezoneMode || (fp.timezone ? 'custom' : 'random'),
        value: fp.timezone || '',
      },
      geolocation: {
        prompt: 'prompt',
        type: 'random',
        ...geolocation,
      },
      resolution: {
        type: width && height ? 'custom' : 'random',
        width: width || 1920,
        height: height || 1080,
      },
      windowMode: hasSwitch(input.launchArgs, '--start-fullscreen') ? 'fullscreen' : 'custom',
      content: {
        sound: !hasSwitch(input.launchArgs, '--mute-audio'),
        images: imagesEnabledFromLaunchArgs(input.launchArgs),
        video: videoEnabledFromLaunchArgs(input.launchArgs),
      },
      searchEngine: asOneOf(getArgValue(input.launchArgs, SEARCH_ENGINE_ARG_PREFIX), SEARCH_ENGINES, 'google'),
      colorDepth: {
        type: fp.colorDepth ? 'custom' : 'random',
        value: Number(fp.colorDepth) || 24,
      },
      fonts: {
        type: fp.fonts ? 'custom' : 'random',
        list: fp.fonts ? fp.fonts.split(',').map((item) => item.trim()).filter(Boolean) : [],
      },
    },
    advanced: {
      webrtc: {
        policy: asOneOf(fp.webrtcPolicy, WEBRTC_POLICIES, 'disable_non_proxied_udp'),
        publicIp: '',
        localIp: '',
      },
      webgl: {
        vendor: { type: 'random', value: '' },
        renderer: { type: 'random', value: '' },
        image: { type: 'random' },
      },
      webgpu: {
        mode: hasDisableFeature(input.launchArgs, 'WebGPU') ? 'disable' : 'match_webgl',
      },
      canvas: { type: 'random' },
      audioContext: { type: 'random' },
      clientRects: { type: 'random' },
      speechVoices: { type: 'random' },
      doNotTrack: fp.doNotTrack ?? false,
      hardwareConcurrency: {
        type: fp.hardwareConcurrency ? 'custom' : 'random',
        value: Number(fp.hardwareConcurrency) || 8,
      },
      deviceMemory: {
        type: fp.deviceMemory ? 'custom' : 'random',
        value: Number(fp.deviceMemory) || 8,
      },
      macAddress: { type: 'random', value: '' },
      mediaDevices: {
        type: fp.mediaDevices ? 'custom' : 'random',
        videoInputs: Number(fp.mediaDevices?.split(',')[0]) || 1,
        audioInputs: Number(fp.mediaDevices?.split(',')[1]) || 1,
        audioOutputs: Number(fp.mediaDevices?.split(',')[2]) || 1,
      },
      touchPoints: {
        type: fp.touchPoints ? 'custom' : 'random',
        value: Number(fp.touchPoints) || 0,
      },
      sslFingerprint: false,
      portScanProtection: true,
      hardwareAcceleration: !hasSwitch(input.launchArgs, '--disable-gpu'),
      disableSandbox: hasSwitch(input.launchArgs, '--no-sandbox'),
      startupArgs: launchArgsToText(input.launchArgs),
    },
    preferences: {
      defaultProject: '',
      tags: Array.isArray(input.tags) ? uniqueList(input.tags.filter((item: unknown) => typeof item === 'string')) : [],
      keywords: Array.isArray(input.keywords) ? uniqueList(input.keywords.filter((item: unknown) => typeof item === 'string')) : [],
      launchCode: typeof input.launchCode === 'string' ? input.launchCode : '',
      accountIds: Array.isArray(input.accountIds) ? uniqueList(input.accountIds.filter((item: unknown) => typeof item === 'string')) : [],
      extensionIds: [],
      notes: '',
      autoStart: false,
      closeAction: 'minimize',
    },
  }
}

function assertImportableConfig(value: unknown): Record<string, any> {
  if (!isRecord(value)) {
    throw new Error('配置文件必须是 JSON 对象')
  }
  const isV2Like = 'profileName' in value || 'platform' in value || 'basic' in value || 'advanced' in value || 'startupUrls' in value
  const isLegacyProfile = Array.isArray(value.fingerprintArgs) || Array.isArray(value.launchArgs) || 'proxyConfig' in value
  if (!isV2Like && !isLegacyProfile) {
    throw new Error('无法识别的配置格式')
  }
  return value
}

export function convertToProfileInput(
  config: FingerprintProfileConfig,
  options: ConvertOptions = {}
): BrowserProfileInput {
  const fingerprintArgs: string[] = [...(options.defaultFingerprintArgs || [])]
  const launchArgs: string[] = uniqueList([
    ...(options.defaultLaunchArgs || []),
    ...parseStartupArgs(config.advanced.startupArgs),
  ], false)

  setArg(fingerprintArgs, '--fingerprint=', randomFingerprintSeed())
  setArg(fingerprintArgs, '--fingerprint-platform=', config.platform)
  setArg(fingerprintArgs, '--fingerprint-brand=', 'Chrome')

  if (config.userAgent.type === 'custom') {
    setArg(fingerprintArgs, '--user-agent=', config.userAgent.value.trim())
  } else {
    setArg(fingerprintArgs, '--user-agent=')
  }

  const language = config.basic.language.trim()
  if (language && language !== 'auto') {
    setArg(fingerprintArgs, '--lang=', language)
    setArg(launchArgs, '--lang=', language)
  } else {
    setArg(fingerprintArgs, '--lang=')
    setArg(launchArgs, '--lang=')
  }

  if (config.basic.uiLanguage.mode === 'custom' && config.basic.uiLanguage.value.trim()) {
    setArg(launchArgs, '--accept-language=', config.basic.uiLanguage.value.trim())
  } else {
    setArg(launchArgs, '--accept-language=')
  }

  if (config.basic.timezone.type === 'custom' && config.basic.timezone.value.trim()) {
    setArg(fingerprintArgs, '--timezone=', config.basic.timezone.value.trim())
  } else {
    setArg(fingerprintArgs, '--timezone=')
  }
  setArg(fingerprintArgs, '--ant-timezone-mode=')
  if (config.basic.timezone.type === 'real') {
    setArg(fingerprintArgs, '--ant-timezone-mode=', 'real')
  }

  setSwitch(launchArgs, '--start-fullscreen', config.basic.windowMode === 'fullscreen')
  if (config.basic.windowMode === 'custom' && config.basic.resolution.type === 'custom') {
    setArg(fingerprintArgs, '--window-size=', `${config.basic.resolution.width},${config.basic.resolution.height}`)
    setArg(launchArgs, '--window-size=', `${config.basic.resolution.width},${config.basic.resolution.height}`)
  } else {
    setArg(fingerprintArgs, '--window-size=')
    setArg(launchArgs, '--window-size=')
  }

  setSwitch(launchArgs, '--mute-audio', !config.basic.content.sound)
  setArg(launchArgs, '--blink-settings=', config.basic.content.images ? undefined : 'imagesEnabled=false')
  setArg(launchArgs, '--autoplay-policy=', config.basic.content.video ? undefined : 'user-gesture-required')
  setSwitch(launchArgs, '--disable-gpu', !config.advanced.hardwareAcceleration)
  setSwitch(launchArgs, '--no-sandbox', config.advanced.disableSandbox)
  setDisableFeature(launchArgs, 'WebGPU', config.advanced.webgpu.mode === 'disable')
  setArg(launchArgs, SEARCH_ENGINE_ARG_PREFIX, asOneOf(config.basic.searchEngine, SEARCH_ENGINES, 'google'))

  if (config.basic.colorDepth.type === 'custom') {
    setArg(fingerprintArgs, '--fingerprint-color-depth=', String(config.basic.colorDepth.value))
  } else {
    setArg(fingerprintArgs, '--fingerprint-color-depth=')
  }

  if (config.basic.fonts.type === 'custom' && config.basic.fonts.list.length > 0) {
    setArg(fingerprintArgs, '--fingerprint-fonts=', uniqueList(config.basic.fonts.list).join(','))
  } else {
    setArg(fingerprintArgs, '--fingerprint-fonts=')
  }

  setArg(fingerprintArgs, '--ant-geolocation=')
  setArg(fingerprintArgs, '--ant-geolocation-mode=')
  setArg(fingerprintArgs, '--ant-geolocation-permission=')
  if (config.basic.geolocation.type === 'real') {
    setArg(fingerprintArgs, '--ant-geolocation-mode=', 'real')
  } else if (
    config.basic.geolocation.type === 'custom' &&
    isFiniteNumber(config.basic.geolocation.latitude) &&
    isFiniteNumber(config.basic.geolocation.longitude)
  ) {
    const accuracy = isFiniteNumber(config.basic.geolocation.accuracy)
      ? config.basic.geolocation.accuracy
      : 100
    setArg(
      fingerprintArgs,
      '--ant-geolocation=',
      `${config.basic.geolocation.latitude},${config.basic.geolocation.longitude},${accuracy}`
    )
    if (config.basic.geolocation.prompt !== 'prompt') {
      setArg(fingerprintArgs, '--ant-geolocation-permission=', config.basic.geolocation.prompt)
    }
  }

  if (config.advanced.webrtc.policy && config.advanced.webrtc.policy !== 'default') {
    setArg(fingerprintArgs, '--webrtc-ip-handling-policy=', config.advanced.webrtc.policy)
  } else {
    setArg(fingerprintArgs, '--webrtc-ip-handling-policy=')
  }

  if (config.advanced.doNotTrack) {
    setArg(fingerprintArgs, '--fingerprint-do-not-track=', 'true')
  } else {
    setArg(fingerprintArgs, '--fingerprint-do-not-track=')
  }

  if (config.advanced.hardwareConcurrency.type === 'custom') {
    setArg(fingerprintArgs, '--fingerprint-hardware-concurrency=', String(config.advanced.hardwareConcurrency.value))
  } else {
    setArg(fingerprintArgs, '--fingerprint-hardware-concurrency=')
  }

  if (config.advanced.deviceMemory.type === 'custom') {
    setArg(fingerprintArgs, '--fingerprint-device-memory=', String(config.advanced.deviceMemory.value))
  } else {
    setArg(fingerprintArgs, '--fingerprint-device-memory=')
  }

  if (config.advanced.mediaDevices.type === 'custom') {
    setArg(
      fingerprintArgs,
      '--fingerprint-media-devices=',
      `${config.advanced.mediaDevices.videoInputs},${config.advanced.mediaDevices.audioInputs},${config.advanced.mediaDevices.audioOutputs}`
    )
  } else {
    setArg(fingerprintArgs, '--fingerprint-media-devices=')
  }

  if (config.advanced.touchPoints.type === 'custom') {
    setArg(fingerprintArgs, '--fingerprint-touch-points=', String(config.advanced.touchPoints.value))
  } else {
    setArg(fingerprintArgs, '--fingerprint-touch-points=')
  }

  const disableSpoofing: string[] = []
  if (
    config.advanced.webgl.image.type === 'real' ||
    config.advanced.webgl.vendor.type === 'real' ||
    config.advanced.webgl.renderer.type === 'real'
  ) {
    disableSpoofing.push('gpu')
  }
  if (config.advanced.canvas.type === 'real') disableSpoofing.push('canvas')
  if (config.advanced.audioContext.type === 'real') disableSpoofing.push('audio')
  if (config.advanced.clientRects.type === 'real') disableSpoofing.push('clientrects')
  if (config.basic.fonts.type === 'real') disableSpoofing.push('font')
  setArg(fingerprintArgs, '--disable-spoofing=', disableSpoofing.length ? disableSpoofing.join(',') : undefined)

  let proxyConfig = ''
  let proxyId = ''
  if (config.proxy.mode === 'pool') {
    proxyId = config.proxy.proxyId.trim()
    if (!proxyId) proxyConfig = DIRECT_PROXY_CONFIG
  } else if (config.proxy.type === 'none') {
    proxyConfig = DIRECT_PROXY_CONFIG
  } else if (config.proxy.host.trim() && config.proxy.port.trim()) {
    const auth = config.proxy.username && config.proxy.password
      ? `${encodeURIComponent(config.proxy.username)}:${encodeURIComponent(config.proxy.password)}@`
      : ''
    proxyConfig = `${config.proxy.type}://${auth}${config.proxy.host.trim()}:${config.proxy.port.trim()}`
  }

  const tags = uniqueList(config.preferences.tags || [])
  const keywords = uniqueList([
    config.profileName.toLowerCase(),
    ...tags.map((tag) => tag.toLowerCase()),
    ...(config.preferences.keywords || []),
  ])

  return {
    profileName: config.profileName.trim() || '未命名配置',
    userDataDir: '',
    coreId: options.coreId || '',
    fingerprintArgs,
    proxyId,
    proxyConfig,
    launchArgs: uniqueList([
      ...launchArgs,
      ...(config.startupUrls || []),
    ], false),
    tags,
    keywords,
    groupId: options.groupId || '',
    launchCode: config.preferences.launchCode.trim(),
    accountIds: config.preferences.accountIds || [],
  }
}

export function normalizeConfig(value: unknown, options: NormalizeOptions = {}): FingerprintProfileConfig {
  const raw = assertImportableConfig(value)
  if (options.rejectUnsupportedBrowserCore && hasUnsupportedFirefoxConfig(raw)) {
    throw new Error('当前创建页暂不支持 Firefox 内核，请改用 Chrome 内核后再导入')
  }
  const migrated = migrateBrowserProfile(raw)
  const partial = (migrated || raw) as Partial<FingerprintProfileConfig>

  return {
    profileName: typeof partial.profileName === 'string' ? partial.profileName : '',
    platform: asOneOf(partial.platform, PLATFORMS, 'windows'),
    browserCore: asOneOf(partial.browserCore, SUPPORTED_BROWSER_CORES, 'chrome'),
    coreVersion: typeof partial.coreVersion === 'string' ? partial.coreVersion : '',
    userAgent: {
      type: asOneOf(partial.userAgent?.type, VALUE_TYPES, 'random'),
      value: typeof partial.userAgent?.value === 'string' ? partial.userAgent.value : '',
    },
    proxy: {
      mode: partial.proxy?.mode === 'pool' ? 'pool' : 'manual',
      proxyId: typeof partial.proxy?.proxyId === 'string' ? partial.proxy.proxyId : '',
      type: asOneOf(partial.proxy?.type, PROXY_TYPES, 'none'),
      host: typeof partial.proxy?.host === 'string' ? partial.proxy.host : '',
      port: typeof partial.proxy?.port === 'string' ? partial.proxy.port : '',
      username: typeof partial.proxy?.username === 'string' ? partial.proxy.username : '',
      password: typeof partial.proxy?.password === 'string' ? partial.proxy.password : '',
    },
    account: {
      platform: typeof partial.account?.platform === 'string' ? partial.account.platform : '',
      username: typeof partial.account?.username === 'string' ? partial.account.username : '',
      password: typeof partial.account?.password === 'string' ? partial.account.password : '',
      cookies: typeof partial.account?.cookies === 'string' ? partial.account.cookies : '',
    },
    startupUrls: Array.isArray(partial.startupUrls)
      ? partial.startupUrls.filter((item): item is string => typeof item === 'string')
      : [],
    basic: {
      language: typeof partial.basic?.language === 'string' ? partial.basic.language : 'auto',
      uiLanguage: {
        mode: asOneOf(partial.basic?.uiLanguage?.mode, LANGUAGE_MODES, 'auto'),
        value: typeof partial.basic?.uiLanguage?.value === 'string' ? partial.basic.uiLanguage.value : 'zh-CN,zh',
      },
      timezone: {
        type: asOneOf(partial.basic?.timezone?.type, VALUE_TYPES, 'random'),
        value: typeof partial.basic?.timezone?.value === 'string' ? partial.basic.timezone.value : '',
      },
      geolocation: {
        prompt: asOneOf(partial.basic?.geolocation?.prompt, GEOLOCATION_PROMPTS, 'prompt'),
        type: asOneOf(partial.basic?.geolocation?.type, VALUE_TYPES, 'random'),
        latitude: isFiniteNumber(partial.basic?.geolocation?.latitude) ? partial.basic.geolocation.latitude : undefined,
        longitude: isFiniteNumber(partial.basic?.geolocation?.longitude) ? partial.basic.geolocation.longitude : undefined,
        accuracy: isFiniteNumber(partial.basic?.geolocation?.accuracy) ? partial.basic.geolocation.accuracy : undefined,
      },
      resolution: {
        type: asOneOf(partial.basic?.resolution?.type, VALUE_TYPES, 'random'),
        width: Number(partial.basic?.resolution?.width) || 1920,
        height: Number(partial.basic?.resolution?.height) || 1080,
      },
      windowMode: asOneOf(partial.basic?.windowMode, WINDOW_MODES, 'custom'),
      content: {
        sound: partial.basic?.content?.sound !== false,
        images: partial.basic?.content?.images !== false,
        video: partial.basic?.content?.video !== false,
      },
      searchEngine: asOneOf(partial.basic?.searchEngine, SEARCH_ENGINES, 'google'),
      colorDepth: {
        type: asOneOf(partial.basic?.colorDepth?.type, VALUE_TYPES, 'random'),
        value: Number(partial.basic?.colorDepth?.value) || 24,
      },
      fonts: {
        type: asOneOf(partial.basic?.fonts?.type, VALUE_TYPES, 'random'),
        list: Array.isArray(partial.basic?.fonts?.list)
          ? partial.basic.fonts.list.filter((item): item is string => typeof item === 'string')
          : [],
      },
    },
    advanced: {
      webrtc: {
        policy: asOneOf(partial.advanced?.webrtc?.policy, WEBRTC_POLICIES, 'disable_non_proxied_udp'),
        publicIp: typeof partial.advanced?.webrtc?.publicIp === 'string' ? partial.advanced.webrtc.publicIp : '',
        localIp: typeof partial.advanced?.webrtc?.localIp === 'string' ? partial.advanced.webrtc.localIp : '',
      },
      webgl: {
        vendor: {
          type: asOneOf(partial.advanced?.webgl?.vendor?.type, REAL_RANDOM_TYPES, 'random'),
          value: '',
        },
        renderer: {
          type: asOneOf(partial.advanced?.webgl?.renderer?.type, REAL_RANDOM_TYPES, 'random'),
          value: '',
        },
        image: {
          type: asOneOf(partial.advanced?.webgl?.image?.type, REAL_RANDOM_TYPES, 'random'),
        },
      },
      webgpu: {
        mode: asOneOf(partial.advanced?.webgpu?.mode, WEBGPU_MODES, 'match_webgl'),
      },
      canvas: { type: asOneOf(partial.advanced?.canvas?.type, VALUE_TYPES, 'random') },
      audioContext: { type: asOneOf(partial.advanced?.audioContext?.type, VALUE_TYPES, 'random') },
      clientRects: { type: asOneOf(partial.advanced?.clientRects?.type, VALUE_TYPES, 'random') },
      speechVoices: { type: 'random' },
      doNotTrack: Boolean(partial.advanced?.doNotTrack),
      hardwareConcurrency: {
        type: asOneOf(partial.advanced?.hardwareConcurrency?.type, VALUE_TYPES, 'random'),
        value: Number(partial.advanced?.hardwareConcurrency?.value) || 8,
      },
      deviceMemory: {
        type: asOneOf(partial.advanced?.deviceMemory?.type, VALUE_TYPES, 'random'),
        value: Number(partial.advanced?.deviceMemory?.value) || 8,
      },
      macAddress: {
        type: asOneOf(partial.advanced?.macAddress?.type, VALUE_TYPES, 'random'),
        value: typeof partial.advanced?.macAddress?.value === 'string' ? partial.advanced.macAddress.value : '',
      },
      mediaDevices: {
        type: asOneOf(partial.advanced?.mediaDevices?.type, VALUE_TYPES, 'random'),
        videoInputs: Number(partial.advanced?.mediaDevices?.videoInputs) || 1,
        audioInputs: Number(partial.advanced?.mediaDevices?.audioInputs) || 1,
        audioOutputs: Number(partial.advanced?.mediaDevices?.audioOutputs) || 1,
      },
      touchPoints: {
        type: asOneOf(partial.advanced?.touchPoints?.type, VALUE_TYPES, 'random'),
        value: Number(partial.advanced?.touchPoints?.value) || 0,
      },
      sslFingerprint: Boolean(partial.advanced?.sslFingerprint),
      portScanProtection: partial.advanced?.portScanProtection !== false,
      hardwareAcceleration: partial.advanced?.hardwareAcceleration !== false,
      disableSandbox: Boolean(partial.advanced?.disableSandbox),
      startupArgs: typeof partial.advanced?.startupArgs === 'string' ? partial.advanced.startupArgs : '',
    },
    preferences: {
      defaultProject: typeof partial.preferences?.defaultProject === 'string' ? partial.preferences.defaultProject : '',
      tags: Array.isArray(partial.preferences?.tags)
        ? uniqueList(partial.preferences.tags.filter((item): item is string => typeof item === 'string'))
        : [],
      keywords: Array.isArray(partial.preferences?.keywords)
        ? uniqueList(partial.preferences.keywords.filter((item): item is string => typeof item === 'string'))
        : [],
      launchCode: typeof partial.preferences?.launchCode === 'string' ? partial.preferences.launchCode : '',
      accountIds: Array.isArray(partial.preferences?.accountIds)
        ? uniqueList(partial.preferences.accountIds.filter((item): item is string => typeof item === 'string'))
        : [],
      extensionIds: Array.isArray(partial.preferences?.extensionIds)
        ? uniqueList(partial.preferences.extensionIds.filter((item): item is string => typeof item === 'string'))
        : [],
      notes: typeof partial.preferences?.notes === 'string' ? partial.preferences.notes : '',
      autoStart: Boolean(partial.preferences?.autoStart),
      closeAction: partial.preferences?.closeAction === 'close' ? 'close' : 'minimize',
    },
  }
}
