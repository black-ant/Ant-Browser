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
  coreVersions?: Record<string, string>
  selectedExtensionIds?: string[]
  // 屏幕可用尺寸（通常取 window.screen.availWidth/availHeight），用于把九宫格窗口位置
  // 换算成 --window-position=x,y 绝对坐标。缺省时不生成窗口位置参数。
  screen?: { width: number; height: number }
}

const CREATE_WINDOW_CONFIG_VERSION = 1
const SWITCH_WITH_VALUE_RE = /^(--[^=\s]+)=/
const SWITCH_RE = /^(--[^\s=]+)/

// 受管参数：完全由富表单控件生成的开关。编辑回显时必须从存量 args 中剥离，
// 否则 createWindowFormToProfileInput 的「prefer-first」合并会让旧值压制控件
// 重新生成的值，导致改了控件却不生效。注意 --disable-features / --blink-settings
// 按开关名整体剥离：若旧 profile 在这两个开关里混入了其他特性（如 --disable-features=Foo,WebGPU），
// 重存时只会保留控件对应的部分，其余需在「启动参数」文本框手动补回。
const MANAGED_FINGERPRINT_SWITCHES = new Set([
  '--user-agent',
  '--fingerprint-platform',
  '--lang',
  '--timezone',
  '--ant-timezone-mode',
  '--fingerprint-hardware-concurrency',
  '--fingerprint-device-memory',
  '--fingerprint-do-not-track',
  '--webrtc-ip-handling-policy',
  '--ant-geolocation-permission',
  '--ant-geolocation',
  '--disable-spoofing',
  // Chrome 144+ 已废弃，仍按受管剥离，值已保留在 profileConfig.formState 中用于回显
  '--fingerprint-webgl-vendor',
  '--fingerprint-webgl-renderer',
])

const MANAGED_LAUNCH_SWITCHES = new Set([
  '--accept-language',
  '--start-maximized',
  '--window-size',
  '--window-position',
  '--mute-audio',
  '--blink-settings',
  '--autoplay-policy',
  '--ant-search-engine',
  '--disable-gpu',
  '--no-sandbox',
  '--disable-features',
  '--allow-browser-signin',
])
const FALLBACK_CHROME_VERSION = '144.0.0.0'
const LEGACY_DEFAULT_CHROME_149_UA_RE = /^Mozilla\/5\.0 \((?:Windows NT 10\.0; Win64; x64|Macintosh; Intel Mac OS X 14_6|X11; Linux x86_64)\) AppleWebKit\/537\.36 \(KHTML, like Gecko\) Chrome\/149\.0\.0\.0 Safari\/537\.36$/
const DISABLE_SPOOFING_ORDER = ['font', 'gpu', 'canvas', 'audio', 'clientrects'] as const

const DEFAULT_CREATE_WINDOW_FORM_STATE: Partial<CreateWindowFormState> = {
  system: 'windows',
  systemVersion: 'Windows 11',
  browserCore: 'chrome',
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

function isStartupUrlArg(arg: string): boolean {
  return !arg.startsWith('--') && /^(https?:\/\/|about:|chrome:|edge:)/i.test(arg)
}

// 剥离由富表单控件托管的参数，只保留用户真正手填的非受管参数（如 --fingerprint= 种子、
// 其它自定义启动开关）。用于编辑回显，避免存量受管参数在重新转换时压制控件新值。
function stripManagedFingerprintArgs(args: string[]): string[] {
  return normalizeArgs(args).filter(arg => {
    const key = switchKey(arg)
    return !(key && MANAGED_FINGERPRINT_SWITCHES.has(key))
  })
}

function stripManagedLaunchArgs(args: string[]): string[] {
  return normalizeArgs(args).filter(arg => {
    if (isStartupUrlArg(arg)) return false
    const key = switchKey(arg)
    return !(key && MANAGED_LAUNCH_SWITCHES.has(key))
  })
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

function normalizeChromeVersion(value: string | undefined): string {
  const match = trim(value).match(/\b(\d{2,3})(?:\.(\d+))?(?:\.(\d+))?(?:\.(\d+))?\b/)
  if (!match) return ''
  return [match[1], match[2] || '0', match[3] || '0', match[4] || '0'].join('.')
}

function selectedCore(formState: CreateWindowFormState, cores: BrowserCore[] | undefined): BrowserCore | undefined {
  if (!cores || cores.length === 0) return undefined
  const explicit = trim(formState.coreId)
  if (explicit) {
    const matched = cores.find(core => core.coreId === explicit)
    if (matched) return matched
  }
  const version = trim(formState.browserVersion)
  if (version) {
    const matched = cores.find(core => core.coreName === version)
    if (matched) return matched
  }
  return cores.find(core => core.isDefault) || cores[0]
}

export function coreChromeVersion(
  core: BrowserCore | undefined,
  coreVersions: Record<string, string> | undefined,
): string {
  if (!core) return ''
  return (
    normalizeChromeVersion(coreVersions?.[core.coreId]) ||
    normalizeChromeVersion(core.coreName) ||
    normalizeChromeVersion(core.corePath)
  )
}

export function buildChromeUserAgent(version: string | undefined, platform: string | undefined = 'windows'): string {
  const chromeVersion = normalizeChromeVersion(version) || FALLBACK_CHROME_VERSION
  const normalizedPlatform = trim(platform).toLowerCase()
  const platformToken = normalizedPlatform === 'mac'
    ? 'Macintosh; Intel Mac OS X 14_6'
    : normalizedPlatform === 'linux'
      ? 'X11; Linux x86_64'
      : 'Windows NT 10.0; Win64; x64'
  return `Mozilla/5.0 (${platformToken}) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/${chromeVersion} Safari/537.36`
}

export function defaultUserAgentForCreateWindow(
  formState: CreateWindowFormState,
  cores: BrowserCore[] | undefined,
  coreVersions?: Record<string, string>,
): string {
  return buildChromeUserAgent(coreChromeVersion(selectedCore(formState, cores), coreVersions), formState.system)
}

export function shouldRegenerateUserAgent(userAgent: string | undefined): boolean {
  const normalized = trim(userAgent)
  return !normalized || LEGACY_DEFAULT_CHROME_149_UA_RE.test(normalized)
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

// buildGeolocationValue 在「地理位置=自定义」且经纬度合法时，生成后端 --ant-geolocation=
// 的取值 lat,lon（经纬度超范围或非数字时返回 null，不生成参数，由后端按代理 IP 推导兜底）。
function buildGeolocationValue(formState: CreateWindowFormState): string | null {
  if (trim(formState.geolocation) !== 'custom') return null
  const lat = Number.parseFloat(trim(formState.latitude))
  const lon = Number.parseFloat(trim(formState.longitude))
  if (!Number.isFinite(lat) || !Number.isFinite(lon)) return null
  if (lat < -90 || lat > 90 || lon < -180 || lon > 180) return null
  return `${lat},${lon}`
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
  coreVersions?: Record<string, string>,
): CreateWindowFormState {
  const defaultCoreName = cores?.find(core => core.isDefault)?.coreName
  const normalized = {
    ...DEFAULT_CREATE_WINDOW_FORM_STATE,
    browserVersion: defaultCoreName || 'Chrome',
    ...formState,
  }
  if (shouldRegenerateUserAgent(normalized.userAgent)) {
    normalized.userAgent = defaultUserAgentForCreateWindow(normalized, cores, coreVersions)
  }
  return normalized
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
  const geolocationValue = buildGeolocationValue(formState)
  if (geolocationValue) args.push(`--ant-geolocation=${geolocationValue}`)
  appendDisableSpoofingArg(args, formState)

  // 注意：--fingerprint-webgl-vendor / --fingerprint-webgl-renderer 在 Chrome 144+ 内核已废弃，
  // 设置后会被忽略并导致指纹不一致。WebGL「真实」通过 --disable-spoofing=gpu 表达（见 appendDisableSpoofingArg），
  // 「随机」走内核默认混淆（不加任何参数）。这里不再生成这两个废弃参数。

  return args
}

// resolveWindowPosition 把九宫格位置（top-left/center/bottom-right 等）按屏幕可用尺寸
// 与窗口尺寸换算成 --window-position=x,y 像素坐标。
//   - top-left 等于 OS 默认放置，返回 null（不生成参数，避免每个窗口都多出 0,0 噪声）。
//   - 缺少屏幕尺寸或窗口尺寸无法解析时返回 null（无法可靠定位）。
function resolveWindowPosition(
  position: string,
  windowWidth: string,
  windowHeight: string,
  screen: { width: number; height: number } | undefined,
): string | null {
  if (!screen || !position || position === 'top-left') return null
  const winW = Number.parseInt(trim(windowWidth), 10)
  const winH = Number.parseInt(trim(windowHeight), 10)
  if (!Number.isFinite(winW) || !Number.isFinite(winH) || winW <= 0 || winH <= 0) return null

  const [vRaw, hRaw] = position.includes('-') ? position.split('-') : [position, position]
  const vMap: Record<string, number> = { top: 0, center: 0.5, bottom: 1 }
  const hMap: Record<string, number> = { left: 0, center: 0.5, right: 1 }
  // 单字 token（top/bottom 居中水平；left/right 居中垂直；center 双向居中）
  const vFactor = vMap[vRaw] ?? 0.5
  const hFactor = hMap[hRaw] ?? 0.5

  const x = Math.max(0, Math.round((screen.width - winW) * hFactor))
  const y = Math.max(0, Math.round((screen.height - winH) * vFactor))
  return `--window-position=${x},${y}`
}

function generatedLaunchArgs(
  formState: CreateWindowFormState,
  screen?: { width: number; height: number },
): string[] {
  const args: string[] = []
  appendModeArg(args, '--accept-language=', formState.uiLanguage)

  const resolution = trim(formState.resolution)
  if (resolution === 'fullscreen') {
    args.push('--start-maximized')
  } else {
    const width = trim(formState.windowWidth)
    const height = trim(formState.windowHeight)
    if (width && height) args.push(`--window-size=${width},${height}`)
    const positionArg = resolveWindowPosition(
      trim(formState.windowPosition),
      width,
      height,
      screen,
    )
    if (positionArg) args.push(positionArg)
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
  const persistableFormState = { ...formState }
  delete persistableFormState.webglVendor
  delete persistableFormState.webglRenderer
  return {
    ...persistableFormState,
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
  const normalizedFormState = withCreateWindowDefaults(formState, options.cores, options.coreVersions)
  const manualLaunchArgs = parseLaunchArgsText(options.launchArgsText)
  const generatedFp = generatedFingerprintArgs(normalizedFormState)
  const generatedLaunch = generatedLaunchArgs(normalizedFormState, options.screen)

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

// restoreGeolocation 从 --ant-geolocation=lat,lon[,accuracy] 反解析自定义经纬度，
// 解析成功时回填 geolocation='custom' 与纬度/经度；无该参数则返回空（落回 auto）。
function restoreGeolocation(args: string[]): Pick<CreateWindowFormState, 'geolocation' | 'latitude' | 'longitude'> {
  const value = argValue(args, '--ant-geolocation=')
  if (!value) return {}
  const parts = value.split(',').map(item => item.trim())
  if (parts.length < 2) return {}
  const lat = Number.parseFloat(parts[0])
  const lon = Number.parseFloat(parts[1])
  if (!Number.isFinite(lat) || !Number.isFinite(lon)) return {}
  return { geolocation: 'custom', latitude: parts[0], longitude: parts[1] }
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
  // 只保留非受管的手填参数：受管开关由富表单控件托管，回显时从存量 args 剥离，
  // 这样再次保存时控件能重新生成受管参数，而不会被旧值压制（见 MANAGED_*_SWITCHES）。
  const base: CreateWindowFormState = {
    profileName: profile.profileName,
    userDataDir: profile.userDataDir,
    coreId: profile.coreId,
    fingerprintArgs: stripManagedFingerprintArgs(profile.fingerprintArgs || []),
    proxyId: profile.proxyId,
    proxyConfig: profile.proxyConfig,
    launchArgs: stripManagedLaunchArgs(profile.launchArgs || []),
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
    ...restoreGeolocation(fingerprintArgs),
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
