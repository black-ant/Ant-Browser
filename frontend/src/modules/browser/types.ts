export type BrowserProfileStatus =
  | 'stopped'
  | 'starting'
  | 'debug_pending'
  | 'running'
  | 'stopping'
  | 'crashed'

export interface BrowserProfile {
  profileId: string
  profileName: string
  userDataDir: string
  coreId: string
  fingerprintArgs: string[]
  proxyId: string
  proxyConfig: string
  proxyBindSourceId?: string
  proxyBindSourceUrl?: string
  proxyBindName?: string
  proxyBindUpdatedAt?: string
  launchArgs: string[]
  tags: string[]
  keywords: string[]
  groupId?: string
  profileConfig?: string
  accountIds?: string[]
  running: boolean
  status?: BrowserProfileStatus
  debugPort: number
  debugReady: boolean
  pid: number
  runtimeWarning: string
  lastError: string
  createdAt: string
  updatedAt: string
  lastStartAt?: string
  lastStopAt?: string
  launchCode?: string
}

export interface BrowserProfileInput {
  profileName: string
  userDataDir: string
  coreId: string
  fingerprintArgs: string[]
  proxyId: string
  proxyConfig: string
  launchArgs: string[]
  tags: string[]
  keywords: string[]
  groupId?: string
  profileConfig?: string
  launchCode?: string
  accountIds?: string[]
}

export interface CreateWindowFormState {
  profileName: string
  userDataDir: string
  coreId: string
  fingerprintArgs: string[]
  proxyId: string
  proxyConfig: string
  launchArgs: string[]
  tags: string[]
  keywords: string[]
  groupId?: string
  launchCode?: string
  accountIds?: string[]
  system?: string
  systemVersion?: string
  browserCore?: string
  browserVersion?: string
  userAgent?: string
  cookies?: string
  urls?: string
  language?: string
  uiLanguage?: string
  timezone?: string
  geolocationDisplay?: string
  geolocation?: string
  latitude?: string
  longitude?: string
  audio?: boolean
  image?: boolean
  video?: boolean
  windowWidth?: string
  windowHeight?: string
  windowPosition?: string
  searchEngine?: string
  resolution?: string
  screenResolution?: string
  fontFingerprint?: string
  webrtc?: string
  webgl?: string
  webglInfo?: string
  webglVendor?: string
  webglRenderer?: string
  webgpu?: string
  canvas?: string
  audioContext?: string
  speechVoices?: string
  doNotTrack?: string
  clientRects?: string
  mediaDevices?: string
  deviceName?: string
  deviceNameValue?: string
  macAddress?: string
  macAddressValue?: string
  hardwareConcurrency?: string
  deviceMemory?: string
  sslFingerprint?: string
  portScanProtection?: string
  portScanAllowList?: string
  hardwareAcceleration?: string
  sandbox?: string
  customBookmarks?: boolean
  syncBookmarks?: boolean
  syncHistory?: boolean
  syncTabs?: boolean
  syncCookies?: boolean
  syncPasswords?: boolean
  syncExtensions?: boolean
  syncIndexedDB?: boolean
  syncLocalStorage?: boolean
  syncSessionStorage?: boolean
  clearCacheBeforeStart?: boolean
  clearCookiesBeforeStart?: boolean
  clearLocalStorageBeforeStart?: boolean
  randomFingerprintOnStart?: boolean
  passwordPrompt?: boolean
  keepNetworkOn?: boolean
  stopOnIpChange?: boolean
  stopOnIpRegionChange?: boolean
  openWorkbench?: string
  ipChangeReminder?: string
  googleLogin?: string
  websiteAccessBlacklist?: string
  websiteAccessWhitelist?: string
  networkDetection?: boolean
  ipChangeAlert?: boolean
  autoCloseOnIpChange?: boolean
  notes?: string
}

export interface CreateWindowProfileConfig {
  version: 1
  formState: CreateWindowFormState
  selectedExtensionIds?: string[]
  postCreateActions?: {
    importCookies?: string
    applyDefaultBookmarks?: boolean
    clearCacheBeforeStart?: boolean
    clearCookiesBeforeStart?: boolean
    clearLocalStorageBeforeStart?: boolean
  }
}

export interface BrowserTab {
  tabId: string
  title: string
  url: string
  active: boolean
}

export interface BrowserSettings {
  userDataRoot: string
  defaultFingerprintArgs: string[]
  defaultLaunchArgs: string[]
  defaultProxy: string
  // 全局前置代理：让走 xray 桥接的带账密代理流量先经过本地前置代理出口，
  // 绕过上游网关按本地区 IP 的准入限制。
  frontProxyEnabled: boolean
  frontProxyAuto: boolean
  frontProxyAddr: string
  startReadyTimeoutMs: number
  startStableWindowMs: number
}

// LocalProxyCandidate 本地代理扫描命中的单个候选。
export interface LocalProxyCandidate {
  addr: string      // 规范化地址，如 socks5://127.0.0.1:7891
  protocol: string  // "socks5" | "http"
  port: number
}

// LocalProxyScanResult 本地代理扫描结果（对应后端 proxy.LocalProxyScanResult）。
export interface LocalProxyScanResult {
  found: boolean
  best: string
  candidates: LocalProxyCandidate[]
  error: string
}

export interface BrowserCore {
  coreId: string
  coreName: string
  corePath: string
  isDefault: boolean
}

export interface BrowserCoreInput {
  coreId: string
  coreName: string
  corePath: string
  isDefault: boolean
}

export interface BrowserCoreValidateResult {
  valid: boolean
  message: string
}

export interface BrowserProxy {
  proxyId: string
  proxyName: string
  proxyConfig: string
  dnsServers?: string
  groupName?: string
  sourceId?: string
  sourceUrl?: string
  sourceNamePrefix?: string
  sourceAutoRefresh?: boolean
  sourceRefreshIntervalM?: number
  sourceLastRefreshAt?: string
  lastLatencyMs?: number
  lastTestOk?: boolean
  lastTestedAt?: string
  lastIPHealthJson?: string
}

// ProxySource 代理订阅源（后端独立建模）
export interface ProxySource {
  sourceId: string
  sourceUrl: string
  sourceName?: string
  groupName?: string
  namePrefix?: string
  dnsServers?: string
  autoRefresh?: boolean
  refreshIntervalM?: number
  importStrategy?: string // merge | replace
  lastRefreshAt?: string
  lastRefreshError?: string
  createdAt?: string
}

// ProxySourceOverride 订阅源节点的忽略/重命名记录
export interface ProxySourceOverride {
  sourceId: string
  nodeKey: string
  action: string // ignore | rename
  customName?: string
}

export interface ProxyIPHealthResult {
  proxyId: string
  ok: boolean
  source: string
  error: string
  ip: string
  fraudScore: number
  isResidential: boolean
  isBroadcast: boolean
  country: string
  region: string
  city: string
  asOrganization: string
  rawData: Record<string, any>
  updatedAt: string
}

// 多源出口 IP 检测结果（添加代理时通过代理链路实测）
export interface IPDetectResult {
  source: string
  ok: boolean
  error: string
  ip: string
  country: string
  countryCode: string
  region: string
  city: string
  isp: string
  org: string
  latencyMs: number
  updatedAt: string
  rawData: Record<string, any>
}

// IP 检测源元数据（供下拉展示）
export interface IPDetectSource {
  key: string
  label: string
}

// 代理协议探测结果（导入裸格式 host:port[:user:pass] 时自动判定 SOCKS5 / HTTP）
export interface ProxyProbeResult {
  protocol: string        // 'socks5' | 'http' | ''（无法判定）
  reachable: boolean      // server:port 可建立 TCP 连接
  usable: boolean         // 握手 / CONNECT 完整成功，可真正转发
  needAuth: boolean       // 需要鉴权但凭据缺失或被拒
  gatewayStatus: number   // HTTP CONNECT 返回的状态码（403 / 407 等）
  gatewayMessage: string  // 网关自身响应文本（如 403 china IP is not allow）
  latencyMs: number
  error: string
}

export interface BrowserCoreExtended {
  coreId: string
  chromeVersion: string
  instanceCount: number
}

export interface CookieInfo {
  name: string
  value: string
  domain: string
  path: string
  expires: number
  httpOnly: boolean
  secure: boolean
  sameSite: string
}

export interface SnapshotInfo {
  snapshotId: string
  profileId: string
  name: string
  sizeMB: number
  createdAt: string
}

export interface BrowserBookmark {
  name: string
  url: string
}


// 分组相关类型
export interface BrowserGroup {
  groupId: string
  groupName: string
  parentId: string
  sortOrder: number
  createdAt: string
  updatedAt: string
}

export interface BrowserGroupInput {
  groupName: string
  parentId: string
  sortOrder: number
}

export interface BrowserGroupWithCount extends BrowserGroup {
  instanceCount: number
}

// 窗口创建模板：保存创建页的完整结构化配置（profileConfig JSON），供复用
export interface BrowserTemplate {
  templateId: string
  templateName: string
  profileConfig: string
  createdAt: string
  updatedAt: string
}

export interface BrowserTemplateInput {
  templateName: string
  profileConfig: string
}
