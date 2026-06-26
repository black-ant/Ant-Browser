import type {
  BrowserCore,
  BrowserProfile,
  BrowserProfileInput,
  CreateWindowFormState,
  CreateWindowProfileConfig,
} from '../types'

interface ConvertOptions {
  launchArgsText?: string
  cores?: BrowserCore[]
  selectedExtensionIds?: string[]
}

const CREATE_WINDOW_CONFIG_VERSION = 1
const SWITCH_WITH_VALUE_RE = /^(--[^=\s]+)=/
const SWITCH_RE = /^(--[^\s=]+)/
const DEFAULT_USER_AGENT = 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36'
const DISABLE_SPOOFING_ORDER = ['font', 'gpu', 'canvas', 'audio', 'clientrects'] as const

const DEFAULT_CREATE_WINDOW_FORM_STATE: Partial<CreateWindowFormState> = {
  system: 'windows',
  systemVersion: 'Windows 11',
  browserCore: 'chrome',
  userAgent: DEFAULT_USER_AGENT,
  language: 'auto',
  uiLanguage: 'auto',
  timezone: 'auto',
  geolocationDisplay: 'allow',
  geolocation: 'auto',
  audio: true,
  image: true,
  video: true,
  windowWidth: '1000',
  windowHeight: '1000',
  windowPosition: 'top-left',
  searchEngine: 'google',
  resolution: 'custom',
  screenResolution: 'system',
  fontFingerprint: 'random',
  webrtc: 'disabled',
  webgl: 'random',
  webglInfo: 'random',
  webgpu: 'match',
  canvas: 'random',
  audioContext: 'random',
  speechVoices: 'random',
  doNotTrack: 'on',
  clientRects: 'random',
  mediaDevices: 'random',
  deviceName: 'random',
  macAddress: 'custom',
  hardwareConcurrency: '12',
  deviceMemory: '8',
  sslFingerprint: 'disabled',
  portScanProtection: 'on',
  hardwareAcceleration: 'on',
  sandbox: 'off',
}

function trim(value: string | undefined): string {
  return (value || '').trim()
}

function normalizeArgs(args: string[] | undefined): string[] {
  return (args || []).map(item => item.trim()).filter(Boolean)
}

export function parseLaunchArgsText(text: string | undefined): string[] {
  return (text || '')
    .split(/[\n;]+/)
    .map(item => item.trim())
    .filter(Boolean)
}

function switchKey(arg: string): string | null {
  return arg.match(SWITCH_WITH_VALUE_RE)?.[1] || arg.match(SWITCH_RE)?.[1] || null
}

function mergeArgsPreferFirst(...groups: string[][]): string[] {
  const seenSwitches = new Set<string>()
  const out: string[] = []
  for (const group of groups) {
    for (const raw of group) {
      const arg = raw.trim()
      if (!arg) continue
      const key = switchKey(arg)
      if (key) {
        if (seenSwitches.has(key)) continue
        seenSwitches.add(key)
      } else if (out.includes(arg)) {
        continue
      }
      out.push(arg)
    }
  }
  return out
}

function appendValueArg(args: string[], prefix: string, value: string | undefined) {
  const normalized = trim(value)
  if (normalized) args.push(`${prefix}${normalized}`)
}

function appendModeArg(args: string[], prefix: string, value: string | undefined) {
  const normalized = trim(value)
  if (!normalized || normalized === 'auto' || normalized === 'custom' || normalized === 'random' || normalized === 'system') {
    return
  }
  args.push(`${prefix}${normalized}`)
}

function appendDisableSpoofingArg(args: string[], formState: CreateWindowFormState) {
  const flags = new Set<string>()
  // 字体「跟随系统」= 禁用字体混淆，暴露真实系统字体；「随机」= 内核默认混淆（不加 flag）。
  if (formState.fontFingerprint === 'system') flags.add('font')
  if (formState.webgl === 'real' || formState.webglInfo === 'real') flags.add('gpu')
  if (formState.canvas === 'real') flags.add('canvas')
  if (formState.audioContext === 'real') flags.add('audio')
  if (formState.clientRects === 'real') flags.add('clientrects')

  const ordered = DISABLE_SPOOFING_ORDER.filter(flag => flags.has(flag))
  if (ordered.length > 0) args.push(`--disable-spoofing=${ordered.join(',')}`)
}

function parseStartupUrls(value: string | undefined): string[] {
  return (value || '')
    .split(/[\s,]+/)
    .map(item => item.trim())
    .filter(Boolean)
    .flatMap(item => {
      if (/^(about|chrome|edge):/i.test(item)) return [item]
      const candidate = /^[a-zA-Z][a-zA-Z0-9+.-]*:\/\//.test(item) ? item : `https://${item}`
      try {
        const parsed = new URL(candidate)
        return ['http:', 'https:'].includes(parsed.protocol) ? [parsed.toString()] : []
      } catch {
        return []
      }
    })
}

function mapWebRtcPolicy(value: string | undefined): string | undefined {
  if (value === 'disabled') return 'disable_non_proxied_udp'
  if (value === 'replace') return 'default_public_interface_only'
  if (value === 'real') return ''
  return value
}

function mapGeolocationPermission(value: string | undefined): string | undefined {
  if (value === 'allow') return 'allow'
  if (value === 'deny') return 'block'
  return undefined
}

function resolveCoreId(formState: CreateWindowFormState, cores: BrowserCore[] | undefined): string {
  const explicit = trim(formState.coreId)
  if (explicit) return explicit
  const version = trim(formState.browserVersion)
  return cores?.find(core => core.coreName === version)?.coreId || ''
}

function withCreateWindowDefaults(
  formState: CreateWindowFormState,
  cores: BrowserCore[] | undefined,
): CreateWindowFormState {
  const defaultCoreName = cores?.find(core => core.isDefault)?.coreName
  return {
    ...DEFAULT_CREATE_WINDOW_FORM_STATE,
    browserVersion: defaultCoreName || 'RoxyChrome 149',
    ...formState,
  }
}

function generatedFingerprintArgs(formState: CreateWindowFormState): string[] {
  const args: string[] = []
  appendValueArg(args, '--user-agent=', formState.userAgent)
  appendValueArg(args, '--fingerprint-platform=', formState.system)
  appendModeArg(args, '--lang=', formState.language)

  const timezone = trim(formState.timezone)
  if (timezone === 'auto') {
    args.push('--ant-timezone-mode=auto')
  } else if (timezone && timezone !== 'custom') {
    args.push(`--timezone=${timezone}`)
    args.push('--ant-timezone-mode=real')
  }

  appendValueArg(args, '--fingerprint-hardware-concurrency=', formState.hardwareConcurrency)
  appendValueArg(args, '--fingerprint-device-memory=', formState.deviceMemory)

  if (formState.doNotTrack === 'on') args.push('--fingerprint-do-not-track=true')
  if (formState.doNotTrack === 'off') args.push('--fingerprint-do-not-track=false')

  const webrtcPolicy = mapWebRtcPolicy(formState.webrtc)
  appendValueArg(args, '--webrtc-ip-handling-policy=', webrtcPolicy)
  appendValueArg(args, '--ant-geolocation-permission=', mapGeolocationPermission(formState.geolocationDisplay))
  appendDisableSpoofingArg(args, formState)

  // 注意：--fingerprint-webgl-vendor / --fingerprint-webgl-renderer 在 Chrome 144+ 内核已废弃，
  // 设置后会被忽略并导致指纹不一致。WebGL「真实」通过 --disable-spoofing=gpu 表达（见 appendDisableSpoofingArg），
  // 「随机」走内核默认混淆（不加任何参数）。这里不再生成这两个废弃参数。

  return args
}

function generatedLaunchArgs(formState: CreateWindowFormState): string[] {
  const args: string[] = []
  appendModeArg(args, '--accept-language=', formState.uiLanguage)

  const resolution = trim(formState.resolution)
  if (resolution === 'fullscreen') {
    args.push('--start-maximized')
  } else {
    const width = trim(formState.windowWidth)
    const height = trim(formState.windowHeight)
    if (width && height) args.push(`--window-size=${width},${height}`)
  }

  if (formState.audio === false) args.push('--mute-audio')
  if (formState.image === false) args.push('--blink-settings=imagesEnabled=false')
  if (formState.video === false) args.push('--autoplay-policy=user-gesture-required')
  appendValueArg(args, '--ant-search-engine=', formState.searchEngine)
  if (formState.hardwareAcceleration === 'off') args.push('--disable-gpu')
  if (formState.sandbox === 'on') args.push('--no-sandbox')
  if (formState.webgpu === 'disabled') args.push('--disable-features=WebGPU')
  // 谷歌登录：关闭时禁用浏览器级账号登录（隔离账号、避免 profile sync 串号）。
  if (formState.googleLogin === 'off') args.push('--allow-browser-signin=false')

  return [...args, ...parseStartupUrls(formState.urls)]
}

function sanitizeFormStateForPersistence(formState: CreateWindowFormState): CreateWindowFormState {
  return {
    ...formState,
    cookies: '',
    launchArgs: normalizeArgs(formState.launchArgs),
    fingerprintArgs: normalizeArgs(formState.fingerprintArgs),
    tags: formState.tags || [],
    keywords: formState.keywords || [],
    accountIds: formState.accountIds || [],
  }
}

export function buildCreateWindowProfileConfig(
  formState: CreateWindowFormState,
  selectedExtensionIds: string[] = [],
): string {
  const sanitized = sanitizeFormStateForPersistence(formState)
  const config: CreateWindowProfileConfig = {
    version: CREATE_WINDOW_CONFIG_VERSION,
    formState: sanitized,
    selectedExtensionIds,
    postCreateActions: {
      importCookies: trim(formState.cookies) || undefined,
      applyDefaultBookmarks: sanitized.customBookmarks === true,
      clearCacheBeforeStart: sanitized.clearCacheBeforeStart === true,
      clearCookiesBeforeStart: sanitized.clearCookiesBeforeStart === true,
      clearLocalStorageBeforeStart: sanitized.clearLocalStorageBeforeStart === true,
    },
  }
  return JSON.stringify(config)
}

export function createWindowFormToProfileInput(
  formState: CreateWindowFormState,
  options: ConvertOptions = {},
): BrowserProfileInput {
  const normalizedFormState = withCreateWindowDefaults(formState, options.cores)
  const manualLaunchArgs = parseLaunchArgsText(options.launchArgsText)
  const generatedFp = generatedFingerprintArgs(normalizedFormState)
  const generatedLaunch = generatedLaunchArgs(normalizedFormState)

  return {
    profileName: normalizedFormState.profileName,
    userDataDir: normalizedFormState.userDataDir,
    coreId: resolveCoreId(normalizedFormState, options.cores),
    fingerprintArgs: mergeArgsPreferFirst(normalizeArgs(normalizedFormState.fingerprintArgs), generatedFp),
    proxyId: normalizedFormState.proxyId,
    proxyConfig: normalizedFormState.proxyConfig,
    launchArgs: mergeArgsPreferFirst(manualLaunchArgs, normalizeArgs(normalizedFormState.launchArgs), generatedLaunch),
    tags: normalizedFormState.tags || [],
    keywords: normalizedFormState.keywords || [],
    groupId: normalizedFormState.groupId || '',
    launchCode: normalizedFormState.launchCode || '',
    accountIds: normalizedFormState.accountIds || [],
    profileConfig: buildCreateWindowProfileConfig(normalizedFormState, options.selectedExtensionIds),
  }
}

function argValue(args: string[] | undefined, prefix: string): string | undefined {
  return normalizeArgs(args).find(arg => arg.startsWith(prefix))?.slice(prefix.length)
}

function hasArg(args: string[] | undefined, value: string): boolean {
  return normalizeArgs(args).includes(value)
}

function restoreTimezone(args: string[]): string | undefined {
  const mode = argValue(args, '--ant-timezone-mode=')
  if (mode === 'auto') return 'auto'
  return argValue(args, '--timezone=')
}

function restoreDoNotTrack(args: string[]): string | undefined {
  const value = argValue(args, '--fingerprint-do-not-track=')
  if (value === 'true') return 'on'
  if (value === 'false') return 'off'
  return undefined
}

function restoreWebRtc(args: string[]): string | undefined {
  const value = argValue(args, '--webrtc-ip-handling-policy=')
  if (value === 'disable_non_proxied_udp') return 'disabled'
  if (value === 'default_public_interface_only') return 'replace'
  return value
}

function restoreGeolocationDisplay(args: string[]): string | undefined {
  const value = argValue(args, '--ant-geolocation-permission=')
  if (value === 'allow' || value === 'granted') return 'allow'
  if (value === 'block' || value === 'deny' || value === 'denied') return 'deny'
  return undefined
}

function restoreWindowSize(args: string[]): Pick<CreateWindowFormState, 'resolution' | 'windowWidth' | 'windowHeight'> {
  if (hasArg(args, '--start-maximized')) {
    return { resolution: 'fullscreen' }
  }
  const value = argValue(args, '--window-size=')
  const match = value?.match(/^(\d+),(\d+)$/)
  if (!match) return {}
  return {
    resolution: 'custom',
    windowWidth: match[1],
    windowHeight: match[2],
  }
}

function restoreWebGpu(args: string[]): string | undefined {
  const disabledFeatures = argValue(args, '--disable-features=')
  if (!disabledFeatures) return undefined
  return disabledFeatures.split(',').map(item => item.trim()).includes('WebGPU') ? 'disabled' : undefined
}

function restoreDisableSpoofing(args: string[]): Set<string> {
  const value = argValue(args, '--disable-spoofing=')
  return new Set((value || '').split(',').map(item => item.trim()).filter(Boolean))
}

function restoreStartupUrls(args: string[]): string | undefined {
  const urls = args.filter(arg => (
    !arg.startsWith('--') &&
    /^(https?:\/\/|about:|chrome:|edge:)/i.test(arg)
  ))
  return urls.length > 0 ? urls.join('\n') : undefined
}

function compactFormState(values: Partial<CreateWindowFormState>): Partial<CreateWindowFormState> {
  const out: Record<string, unknown> = {}
  for (const [key, value] of Object.entries(values)) {
    if (value !== undefined) out[key] = value
  }
  return out as Partial<CreateWindowFormState>
}

export function restoreCreateWindowFormState(profile: BrowserProfile): CreateWindowFormState {
  const base: CreateWindowFormState = {
    profileName: profile.profileName,
    userDataDir: profile.userDataDir,
    coreId: profile.coreId,
    fingerprintArgs: normalizeArgs(profile.fingerprintArgs),
    proxyId: profile.proxyId,
    proxyConfig: profile.proxyConfig,
    launchArgs: normalizeArgs(profile.launchArgs),
    tags: profile.tags || [],
    keywords: profile.keywords || [],
    groupId: profile.groupId || '',
    launchCode: profile.launchCode || '',
    accountIds: profile.accountIds || [],
  }

  if (profile.profileConfig) {
    try {
      const parsed = JSON.parse(profile.profileConfig) as Partial<CreateWindowProfileConfig>
      if (parsed?.formState) {
        return {
          ...DEFAULT_CREATE_WINDOW_FORM_STATE,
          ...base,
          ...parsed.formState,
          profileName: base.profileName,
          userDataDir: base.userDataDir,
          coreId: base.coreId,
          fingerprintArgs: base.fingerprintArgs,
          proxyId: base.proxyId,
          proxyConfig: base.proxyConfig,
          launchArgs: base.launchArgs,
          tags: base.tags,
          keywords: base.keywords,
          groupId: base.groupId,
          launchCode: base.launchCode,
          accountIds: base.accountIds,
        }
      }
    } catch {
      // Fall back to launch/fingerprint arguments below.
    }
  }

  const fingerprintArgs = normalizeArgs(profile.fingerprintArgs)
  const launchArgs = normalizeArgs(profile.launchArgs)
  const disableSpoofing = restoreDisableSpoofing(fingerprintArgs)
  const restoredFromArgs = compactFormState({
    userAgent: argValue(fingerprintArgs, '--user-agent='),
    system: argValue(fingerprintArgs, '--fingerprint-platform='),
    language: argValue(fingerprintArgs, '--lang='),
    timezone: restoreTimezone(fingerprintArgs),
    hardwareConcurrency: argValue(fingerprintArgs, '--fingerprint-hardware-concurrency='),
    deviceMemory: argValue(fingerprintArgs, '--fingerprint-device-memory='),
    doNotTrack: restoreDoNotTrack(fingerprintArgs),
    webrtc: restoreWebRtc(fingerprintArgs),
    geolocationDisplay: restoreGeolocationDisplay(fingerprintArgs),
    // 旧 profile 可能残留已废弃的 webgl vendor/renderer，仍读出来回显，避免显示空白。
    webglVendor: argValue(fingerprintArgs, '--fingerprint-webgl-vendor='),
    webglRenderer: argValue(fingerprintArgs, '--fingerprint-webgl-renderer='),
    fontFingerprint: disableSpoofing.has('font') ? 'system' : undefined,
    webgl: disableSpoofing.has('gpu') ? 'real' : undefined,
    webglInfo: disableSpoofing.has('gpu') ? 'real' : undefined,
    canvas: disableSpoofing.has('canvas') ? 'real' : undefined,
    audioContext: disableSpoofing.has('audio') ? 'real' : undefined,
    clientRects: disableSpoofing.has('clientrects') ? 'real' : undefined,
    uiLanguage: argValue(launchArgs, '--accept-language='),
    audio: hasArg(launchArgs, '--mute-audio') ? false : undefined,
    image: hasArg(launchArgs, '--blink-settings=imagesEnabled=false') ? false : undefined,
    video: hasArg(launchArgs, '--autoplay-policy=user-gesture-required') ? false : undefined,
    searchEngine: argValue(launchArgs, '--ant-search-engine='),
    hardwareAcceleration: hasArg(launchArgs, '--disable-gpu') ? 'off' : undefined,
    sandbox: hasArg(launchArgs, '--no-sandbox') ? 'on' : undefined,
    webgpu: restoreWebGpu(launchArgs),
    googleLogin: hasArg(launchArgs, '--allow-browser-signin=false') ? 'off' : undefined,
    urls: restoreStartupUrls(launchArgs),
  })

  return {
    ...DEFAULT_CREATE_WINDOW_FORM_STATE,
    ...base,
    ...restoreWindowSize(launchArgs),
    ...restoredFromArgs,
  }
}

// 身份字段：套用模板时不应覆盖这些与具体窗口实例绑定的值。
const TEMPLATE_IDENTITY_FIELDS: (keyof CreateWindowFormState)[] = [
  'profileName',
  'userDataDir',
  'launchCode',
  'accountIds',
]

// restoreFormStateFromTemplate 把模板保存的 profileConfig JSON 解析为表单补丁，
// 用于「从模板创建」。它只回填指纹/启动/偏好类配置，保留当前窗口的身份字段
// （名称、数据目录、启动码、账号绑定）。解析失败返回空补丁（不改动表单）。
export function restoreFormStateFromTemplate(
  profileConfig: string | undefined,
  current: CreateWindowFormState,
): Partial<CreateWindowFormState> {
  if (!profileConfig) return {}
  let parsed: Partial<CreateWindowProfileConfig>
  try {
    parsed = JSON.parse(profileConfig) as Partial<CreateWindowProfileConfig>
  } catch {
    return {}
  }
  if (!parsed?.formState) return {}

  const patch: Partial<CreateWindowFormState> = { ...parsed.formState }
  // 模板内的运行时一次性数据不应带入新窗口。
  delete patch.cookies
  for (const field of TEMPLATE_IDENTITY_FIELDS) {
    // 保留当前窗口的身份字段。
    ;(patch as Record<string, unknown>)[field] = current[field]
  }
  return patch
}
