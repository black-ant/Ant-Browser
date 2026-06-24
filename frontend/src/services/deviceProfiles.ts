import type {
  FingerprintProfileConfig,
  Platform,
  WebRTCPolicy,
} from '../modules/browser/pages/browser-create-v2/types'

export type DeviceProfilePreset = 'general' | 'social' | 'ecommerce' | 'ai'

export interface DeviceProfile {
  id: string
  name: string
  platform: Platform
  osLabel: string
  uaPlatform: string
  defaultLanguage: string
  defaultTimezone: string
  resolution: {
    width: number
    height: number
  }
  colorDepth: number
  fonts: string[]
  hardwareConcurrency: number
  deviceMemory: number
  mediaDevices: {
    videoInputs: number
    audioInputs: number
    audioOutputs: number
  }
  touchPoints: number
  gpuVendor: string
  gpuRenderer: string
  weight: number
  presetWeights: Partial<Record<DeviceProfilePreset, number>>
}

interface ChooseDeviceProfileOptions {
  platform?: Platform
  preset?: DeviceProfilePreset
}

interface ApplyDeviceProfileOptions {
  chromeMajor?: number
  language?: string
  timezone?: FingerprintProfileConfig['basic']['timezone']
  geolocation?: FingerprintProfileConfig['basic']['geolocation']
  webrtcPolicy?: WebRTCPolicy
  doNotTrack?: boolean
}

export const DEVICE_PROFILES: DeviceProfile[] = [
  {
    id: 'win11-amd-rx560',
    name: 'Windows 11 / AMD Radeon RX 560',
    platform: 'windows',
    osLabel: 'Windows 11',
    uaPlatform: 'Windows NT 10.0; Win64; x64',
    defaultLanguage: 'en-US',
    defaultTimezone: 'America/New_York',
    resolution: { width: 1920, height: 1080 },
    colorDepth: 24,
    fonts: [
      'Arial',
      'Segoe UI',
      'Microsoft YaHei',
      'SimSun',
      'Calibri',
      'Times New Roman',
      'Courier New',
      'Verdana',
    ],
    hardwareConcurrency: 6,
    deviceMemory: 64,
    mediaDevices: { videoInputs: 1, audioInputs: 1, audioOutputs: 1 },
    touchPoints: 0,
    gpuVendor: 'Google Inc. (AMD)',
    gpuRenderer: 'ANGLE (AMD, Radeon RX 560 Series Direct3D11 vs_5_0 ps_5_0, D3D11)',
    weight: 8,
    presetWeights: { general: 6, social: 8, ecommerce: 8 },
  },
  {
    id: 'win11-intel-iris-xe',
    name: 'Windows 11 / Intel Iris Xe',
    platform: 'windows',
    osLabel: 'Windows 11',
    uaPlatform: 'Windows NT 10.0; Win64; x64',
    defaultLanguage: 'en-US',
    defaultTimezone: 'America/Los_Angeles',
    resolution: { width: 1920, height: 1080 },
    colorDepth: 24,
    fonts: [
      'Arial',
      'Segoe UI',
      'Calibri',
      'Microsoft YaHei',
      'Times New Roman',
      'Courier New',
      'Verdana',
    ],
    hardwareConcurrency: 8,
    deviceMemory: 16,
    mediaDevices: { videoInputs: 1, audioInputs: 1, audioOutputs: 1 },
    touchPoints: 0,
    gpuVendor: 'Google Inc. (Intel)',
    gpuRenderer: 'ANGLE (Intel, Intel(R) Iris(R) Xe Graphics Direct3D11 vs_5_0 ps_5_0, D3D11)',
    weight: 18,
    presetWeights: { general: 16, social: 16, ecommerce: 14, ai: 8 },
  },
  {
    id: 'win10-nvidia-gtx1650',
    name: 'Windows 10 / NVIDIA GTX 1650',
    platform: 'windows',
    osLabel: 'Windows 10',
    uaPlatform: 'Windows NT 10.0; Win64; x64',
    defaultLanguage: 'en-US',
    defaultTimezone: 'America/Chicago',
    resolution: { width: 1920, height: 1080 },
    colorDepth: 24,
    fonts: [
      'Arial',
      'Segoe UI',
      'Calibri',
      'Times New Roman',
      'Courier New',
      'Georgia',
      'Verdana',
    ],
    hardwareConcurrency: 8,
    deviceMemory: 16,
    mediaDevices: { videoInputs: 1, audioInputs: 1, audioOutputs: 1 },
    touchPoints: 0,
    gpuVendor: 'Google Inc. (NVIDIA)',
    gpuRenderer: 'ANGLE (NVIDIA, NVIDIA GeForce GTX 1650 Direct3D11 vs_5_0 ps_5_0, D3D11)',
    weight: 12,
    presetWeights: { general: 10, social: 10, ecommerce: 12, ai: 10 },
  },
  {
    id: 'win11-nvidia-rtx3060',
    name: 'Windows 11 / NVIDIA RTX 3060',
    platform: 'windows',
    osLabel: 'Windows 11',
    uaPlatform: 'Windows NT 10.0; Win64; x64',
    defaultLanguage: 'en-US',
    defaultTimezone: 'America/New_York',
    resolution: { width: 2560, height: 1440 },
    colorDepth: 24,
    fonts: [
      'Arial',
      'Segoe UI',
      'Calibri',
      'Times New Roman',
      'Courier New',
      'Georgia',
      'Verdana',
    ],
    hardwareConcurrency: 16,
    deviceMemory: 32,
    mediaDevices: { videoInputs: 1, audioInputs: 1, audioOutputs: 1 },
    touchPoints: 0,
    gpuVendor: 'Google Inc. (NVIDIA)',
    gpuRenderer: 'ANGLE (NVIDIA, NVIDIA GeForce RTX 3060 Direct3D11 vs_5_0 ps_5_0, D3D11)',
    weight: 6,
    presetWeights: { general: 4, social: 5, ecommerce: 6, ai: 20 },
  },
  {
    id: 'macbook-air-m2',
    name: 'macOS / MacBook Air M2',
    platform: 'mac',
    osLabel: 'macOS',
    uaPlatform: 'Macintosh; Intel Mac OS X 10_15_7',
    defaultLanguage: 'en-US',
    defaultTimezone: 'America/Los_Angeles',
    resolution: { width: 1440, height: 900 },
    colorDepth: 30,
    fonts: [
      'Arial',
      'Helvetica',
      'San Francisco',
      'PingFang SC',
      'Hiragino Sans',
      'Times New Roman',
      'Courier New',
    ],
    hardwareConcurrency: 8,
    deviceMemory: 8,
    mediaDevices: { videoInputs: 1, audioInputs: 1, audioOutputs: 1 },
    touchPoints: 0,
    gpuVendor: 'Google Inc. (Apple)',
    gpuRenderer: 'ANGLE (Apple, Apple M2, OpenGL 4.1)',
    weight: 8,
    presetWeights: { general: 8, social: 12, ecommerce: 8, ai: 10 },
  },
  {
    id: 'macbook-pro-m3',
    name: 'macOS / MacBook Pro M3',
    platform: 'mac',
    osLabel: 'macOS',
    uaPlatform: 'Macintosh; Intel Mac OS X 10_15_7',
    defaultLanguage: 'en-US',
    defaultTimezone: 'America/New_York',
    resolution: { width: 1512, height: 982 },
    colorDepth: 30,
    fonts: [
      'Arial',
      'Helvetica',
      'San Francisco',
      'PingFang SC',
      'Hiragino Sans',
      'Times New Roman',
      'Courier New',
    ],
    hardwareConcurrency: 12,
    deviceMemory: 16,
    mediaDevices: { videoInputs: 1, audioInputs: 1, audioOutputs: 1 },
    touchPoints: 0,
    gpuVendor: 'Google Inc. (Apple)',
    gpuRenderer: 'ANGLE (Apple, Apple M3, OpenGL 4.1)',
    weight: 5,
    presetWeights: { general: 4, social: 6, ecommerce: 5, ai: 14 },
  },
  {
    id: 'linux-intel-uhd',
    name: 'Linux / Intel UHD',
    platform: 'linux',
    osLabel: 'Linux',
    uaPlatform: 'X11; Linux x86_64',
    defaultLanguage: 'en-US',
    defaultTimezone: 'Europe/London',
    resolution: { width: 1920, height: 1080 },
    colorDepth: 24,
    fonts: [
      'Arial',
      'Helvetica',
      'Liberation Sans',
      'DejaVu Sans',
      'Times New Roman',
      'Courier New',
    ],
    hardwareConcurrency: 8,
    deviceMemory: 16,
    mediaDevices: { videoInputs: 1, audioInputs: 1, audioOutputs: 1 },
    touchPoints: 0,
    gpuVendor: 'Google Inc. (Intel)',
    gpuRenderer: 'ANGLE (Intel, Mesa Intel(R) UHD Graphics, OpenGL 4.6)',
    weight: 3,
    presetWeights: { general: 3, social: 2, ecommerce: 2, ai: 4 },
  },
  {
    id: 'android-pixel-7',
    name: 'Android / Pixel 7',
    platform: 'android',
    osLabel: 'Android 13',
    uaPlatform: 'Linux; Android 13; Pixel 7',
    defaultLanguage: 'en-US',
    defaultTimezone: 'America/Los_Angeles',
    resolution: { width: 412, height: 915 },
    colorDepth: 24,
    fonts: [
      'Roboto',
      'Noto Sans',
      'Arial',
      'sans-serif',
    ],
    hardwareConcurrency: 8,
    deviceMemory: 8,
    mediaDevices: { videoInputs: 2, audioInputs: 1, audioOutputs: 1 },
    touchPoints: 5,
    gpuVendor: 'Google Inc. (Qualcomm)',
    gpuRenderer: 'ANGLE (Qualcomm, Adreno 730, OpenGL ES 3.2)',
    weight: 2,
    presetWeights: { general: 2, social: 4 },
  },
]

function weightedRandom<T>(items: T[], getWeight: (item: T) => number): T {
  const totalWeight = items.reduce((sum, item) => sum + Math.max(0, getWeight(item)), 0)
  if (totalWeight <= 0) {
    return items[Math.floor(Math.random() * items.length)]
  }

  let random = Math.random() * totalWeight
  for (const item of items) {
    random -= Math.max(0, getWeight(item))
    if (random <= 0) {
      return item
    }
  }
  return items[items.length - 1]
}

function normalizeChromeMajor(value: number | undefined): number {
  if (!value || !Number.isFinite(value)) return 149
  return Math.max(120, Math.min(199, Math.round(value)))
}

export function extractChromeMajor(...values: Array<string | undefined>): number | undefined {
  for (const value of values) {
    if (!value) continue
    const chromeMatch = value.match(/\b(?:Chrome|Chromium)\D{0,16}(\d{2,3})\b/i)
    if (chromeMatch) return Number(chromeMatch[1])

    const genericMatch = value.match(/\b(1[2-9]\d)\b/)
    if (genericMatch) return Number(genericMatch[1])
  }
  return undefined
}

export function resolveChromeMajor(
  config: FingerprintProfileConfig,
  browserCoreName?: string,
): number {
  return normalizeChromeMajor(extractChromeMajor(
    browserCoreName,
    config.coreVersion,
    config.userAgent.value,
  ))
}

export function chooseDeviceProfile(options: ChooseDeviceProfileOptions = {}): DeviceProfile {
  const platformMatches = options.platform
    ? DEVICE_PROFILES.filter(profile => profile.platform === options.platform)
    : DEVICE_PROFILES
  const candidates = platformMatches.length > 0 ? platformMatches : DEVICE_PROFILES

  return weightedRandom(candidates, profile => {
    if (!options.preset) return profile.weight
    return profile.presetWeights[options.preset] ?? 0
  })
}

export function findMatchingDeviceProfile(config: FingerprintProfileConfig): DeviceProfile | undefined {
  return DEVICE_PROFILES.find(profile => (
    profile.platform === config.platform &&
    profile.resolution.width === config.basic.resolution.width &&
    profile.resolution.height === config.basic.resolution.height &&
    profile.hardwareConcurrency === config.advanced.hardwareConcurrency.value &&
    profile.deviceMemory === config.advanced.deviceMemory.value
  ))
}

export function buildChromeUserAgent(profile: DeviceProfile, chromeMajor: number): string {
  const version = `${normalizeChromeMajor(chromeMajor)}.0.0.0`
  const mobileToken = profile.platform === 'android' ? ' Mobile' : ''
  return `Mozilla/5.0 (${profile.uaPlatform}) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/${version}${mobileToken} Safari/537.36`
}

export function applyDeviceProfile(
  config: FingerprintProfileConfig,
  profile: DeviceProfile,
  options: ApplyDeviceProfileOptions = {},
): FingerprintProfileConfig {
  const chromeMajor = normalizeChromeMajor(options.chromeMajor ?? resolveChromeMajor(config))

  return {
    ...config,
    platform: profile.platform,
    browserCore: 'chrome',
    coreVersion: `Chrome ${chromeMajor}`,
    userAgent: {
      type: 'custom',
      value: buildChromeUserAgent(profile, chromeMajor),
    },
    basic: {
      ...config.basic,
      language: options.language ?? profile.defaultLanguage,
      timezone: options.timezone ?? {
        type: 'custom',
        value: profile.defaultTimezone,
      },
      geolocation: options.geolocation ?? {
        prompt: 'prompt',
        type: 'random',
        latitude: undefined,
        longitude: undefined,
        accuracy: undefined,
      },
      resolution: {
        type: 'custom',
        width: profile.resolution.width,
        height: profile.resolution.height,
      },
      colorDepth: {
        type: 'custom',
        value: profile.colorDepth,
      },
      fonts: {
        type: 'custom',
        list: [...profile.fonts],
      },
    },
    advanced: {
      ...config.advanced,
      webrtc: {
        ...config.advanced.webrtc,
        policy: options.webrtcPolicy ?? 'disable_non_proxied_udp',
        publicIp: '',
        localIp: '',
      },
      webgl: {
        image: { type: 'random' },
        vendor: { type: 'random', value: '' },
        renderer: { type: 'random', value: '' },
      },
      canvas: { type: 'random' },
      audioContext: { type: 'random' },
      clientRects: { type: 'random' },
      speechVoices: { type: 'random' },
      doNotTrack: options.doNotTrack ?? false,
      hardwareConcurrency: {
        type: 'custom',
        value: profile.hardwareConcurrency,
      },
      deviceMemory: {
        type: 'custom',
        value: profile.deviceMemory,
      },
      macAddress: {
        type: 'random',
        value: '',
      },
      mediaDevices: {
        type: 'custom',
        videoInputs: profile.mediaDevices.videoInputs,
        audioInputs: profile.mediaDevices.audioInputs,
        audioOutputs: profile.mediaDevices.audioOutputs,
      },
      touchPoints: {
        type: 'custom',
        value: profile.touchPoints,
      },
    },
  }
}
