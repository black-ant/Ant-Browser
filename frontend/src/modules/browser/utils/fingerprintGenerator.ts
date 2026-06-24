// 智能指纹生成器
import type { FingerprintConfig } from './fingerprintSerializer'
import { randomFingerprintSeed } from './fingerprintSerializer'

/**
 * 根据地理位置推荐的浏览器品牌分布
 */
const BROWSER_BRAND_BY_REGION: Record<string, { brand: string; weight: number }[]> = {
  CN: [
    { brand: 'Chrome', weight: 60 },
    { brand: 'Edge', weight: 30 },
    { brand: 'Firefox', weight: 8 },
    { brand: 'Safari', weight: 2 },
  ],
  US: [
    { brand: 'Chrome', weight: 65 },
    { brand: 'Safari', weight: 20 },
    { brand: 'Edge', weight: 10 },
    { brand: 'Firefox', weight: 5 },
  ],
  JP: [
    { brand: 'Chrome', weight: 55 },
    { brand: 'Safari', weight: 25 },
    { brand: 'Edge', weight: 15 },
    { brand: 'Firefox', weight: 5 },
  ],
  GB: [
    { brand: 'Chrome', weight: 68 },
    { brand: 'Safari', weight: 18 },
    { brand: 'Edge', weight: 10 },
    { brand: 'Firefox', weight: 4 },
  ],
  default: [
    { brand: 'Chrome', weight: 65 },
    { brand: 'Edge', weight: 20 },
    { brand: 'Firefox', weight: 10 },
    { brand: 'Safari', weight: 5 },
  ],
}

/**
 * 平台分布（全球统计）
 */
const PLATFORM_DISTRIBUTION = [
  { platform: 'windows', weight: 75 },
  { platform: 'mac', weight: 20 },
  { platform: 'linux', weight: 5 },
]

/**
 * 常见分辨率分布
 */
const RESOLUTION_DISTRIBUTION = [
  { resolution: '1920,1080', weight: 45 },
  { resolution: '1366,768', weight: 18 },
  { resolution: '2560,1440', weight: 12 },
  { resolution: '1440,900', weight: 10 },
  { resolution: '1536,864', weight: 8 },
  { resolution: '1600,900', weight: 7 },
]

/**
 * 硬件配置组合（CPU 核心 + 内存）
 */
const HARDWARE_CONFIGS = [
  { cores: '4', memory: '4', weight: 15, type: 'low' },
  { cores: '4', memory: '8', weight: 20, type: 'medium' },
  { cores: '8', memory: '8', weight: 30, type: 'medium' },
  { cores: '8', memory: '16', weight: 20, type: 'high' },
  { cores: '16', memory: '16', weight: 10, type: 'high' },
  { cores: '12', memory: '32', weight: 5, type: 'high' },
]

/**
 * 时区映射（国家代码 -> IANA 时区）
 */
const TIMEZONE_BY_COUNTRY: Record<string, string[]> = {
  CN: ['Asia/Shanghai', 'Asia/Urumqi'],
  US: ['America/New_York', 'America/Chicago', 'America/Denver', 'America/Los_Angeles'],
  JP: ['Asia/Tokyo'],
  GB: ['Europe/London'],
  FR: ['Europe/Paris'],
  DE: ['Europe/Berlin'],
  KR: ['Asia/Seoul'],
  SG: ['Asia/Singapore'],
  AU: ['Australia/Sydney', 'Australia/Melbourne'],
  CA: ['America/Toronto', 'America/Vancouver'],
}

/**
 * 语言映射（国家代码 -> 语言代码）
 */
const LANG_BY_COUNTRY: Record<string, string[]> = {
  CN: ['zh-CN'],
  US: ['en-US'],
  JP: ['ja-JP'],
  GB: ['en-GB'],
  FR: ['fr-FR'],
  DE: ['de-DE'],
  KR: ['ko-KR'],
  SG: ['en-SG', 'zh-CN'],
  AU: ['en-AU'],
  CA: ['en-CA', 'fr-CA'],
}

/**
 * 字体列表（按平台和地区）
 */
const FONTS_BY_PLATFORM_REGION: Record<string, Record<string, string>> = {
  windows: {
    CN: 'Arial,Microsoft YaHei,SimSun,SimHei,Helvetica,Times New Roman,Courier New',
    US: 'Arial,Helvetica,Times New Roman,Courier New,Georgia,Verdana,Tahoma',
    JP: 'Arial,MS Gothic,Meiryo,Yu Gothic,Helvetica,Times New Roman',
    KR: 'Arial,Malgun Gothic,Gulim,Dotum,Helvetica,Times New Roman',
    default: 'Arial,Helvetica,Times New Roman,Courier New,Verdana',
  },
  mac: {
    CN: 'Arial,Helvetica,PingFang SC,Hiragino Sans GB,STHeiti,Times New Roman',
    US: 'Arial,Helvetica,Times New Roman,Courier New,Georgia,Palatino',
    JP: 'Arial,Helvetica,Hiragino Kaku Gothic ProN,Yu Gothic,Times New Roman',
    KR: 'Arial,Helvetica,Apple SD Gothic Neo,NanumGothic,Times New Roman',
    default: 'Arial,Helvetica,Times New Roman,Courier New,Monaco',
  },
  linux: {
    default: 'Arial,Helvetica,Liberation Sans,DejaVu Sans,Times New Roman,Courier New',
  },
}

/**
 * 智能生成选项
 */
export interface SmartGenerateOptions {
  // 代理地理位置（国家代码，如 'CN', 'US'）
  proxyCountry?: string
  // 代理城市（用于更精确的时区推断）
  proxyCity?: string
  // 代理时区（优先级最高，如果提供则直接使用）
  proxyTimezone?: string
  // 使用场景：'office'（办公）, 'home'（家用）, 'gaming'（游戏）, 'random'（随机）
  scenario?: 'office' | 'home' | 'gaming' | 'random'
}

/**
 * 根据权重随机选择
 */
function weightedRandom<T extends { weight: number }>(items: T[]): T {
  const totalWeight = items.reduce((sum, item) => sum + item.weight, 0)
  let random = Math.random() * totalWeight

  for (const item of items) {
    random -= item.weight
    if (random <= 0) {
      return item
    }
  }

  return items[items.length - 1]
}

/**
 * 随机选择数组元素
 */
function randomChoice<T>(arr: T[]): T {
  return arr[Math.floor(Math.random() * arr.length)]
}

/**
 * 根据场景调整硬件配置权重
 */
function adjustHardwareWeights(scenario: string): typeof HARDWARE_CONFIGS {
  if (scenario === 'office') {
    // 办公场景：偏向中低配
    return HARDWARE_CONFIGS.map(cfg => ({
      ...cfg,
      weight: cfg.type === 'low' ? cfg.weight * 2 : cfg.type === 'medium' ? cfg.weight * 1.5 : cfg.weight * 0.5,
    }))
  } else if (scenario === 'gaming') {
    // 游戏场景：偏向高配
    return HARDWARE_CONFIGS.map(cfg => ({
      ...cfg,
      weight: cfg.type === 'high' ? cfg.weight * 2 : cfg.type === 'medium' ? cfg.weight * 1 : cfg.weight * 0.3,
    }))
  }
  return HARDWARE_CONFIGS
}

/**
 * 根据场景调整分辨率权重
 */
function adjustResolutionWeights(scenario: string): typeof RESOLUTION_DISTRIBUTION {
  if (scenario === 'gaming') {
    // 游戏场景：偏向高分辨率
    return RESOLUTION_DISTRIBUTION.map(res => ({
      ...res,
      weight: res.resolution === '2560,1440' ? res.weight * 2 : res.weight,
    }))
  }
  return RESOLUTION_DISTRIBUTION
}

/**
 * 智能生成指纹配置
 */
export function generateSmartFingerprint(options: SmartGenerateOptions = {}): FingerprintConfig {
  const { proxyCountry, proxyTimezone, scenario = 'random' } = options

  // 1. 选择平台
  const platform = weightedRandom(PLATFORM_DISTRIBUTION).platform

  // 2. 根据地区选择浏览器品牌
  const brandDistribution = proxyCountry
    ? (BROWSER_BRAND_BY_REGION[proxyCountry] || BROWSER_BRAND_BY_REGION.default)
    : BROWSER_BRAND_BY_REGION.default
  const brand = weightedRandom(brandDistribution).brand

  // 3. 选择语言（仅在提供代理国家时才生成）
  const langOptions = proxyCountry ? (LANG_BY_COUNTRY[proxyCountry] || undefined) : undefined
  const lang = langOptions ? randomChoice(langOptions) : undefined

  // 4. 选择时区（仅在提供代理信息时才生成）
  let timezone: string | undefined
  if (proxyTimezone) {
    // 优先使用提供的时区
    timezone = proxyTimezone
  } else if (proxyCountry && TIMEZONE_BY_COUNTRY[proxyCountry]) {
    // 根据国家代码选择时区
    timezone = randomChoice(TIMEZONE_BY_COUNTRY[proxyCountry])
  }
  // 否则留空，让后端根据代理 IP 自动推导

  // 5. 选择分辨率
  const resolutionWeights = adjustResolutionWeights(scenario)
  const resolution = weightedRandom(resolutionWeights).resolution

  // 6. 选择硬件配置
  const hardwareWeights = adjustHardwareWeights(scenario)
  const hardware = weightedRandom(hardwareWeights)

  // 7. 色深（大部分是 24 位）
  const colorDepth = Math.random() < 0.9 ? '24' : '30'

  // 8. 选择字体
  const fontsByRegion = FONTS_BY_PLATFORM_REGION[platform] || FONTS_BY_PLATFORM_REGION.windows
  const fonts = proxyCountry
    ? (fontsByRegion[proxyCountry] || fontsByRegion.default)
    : fontsByRegion.default

  // 9. 触摸点数（移动设备才有）
  const touchPoints = '0'

  // 10. Do Not Track（随机）
  const doNotTrack = Math.random() < 0.3 // 30% 概率启用

  // 11. WebRTC 策略（默认禁用非代理 UDP）
  const webrtcPolicy = 'disable_non_proxied_udp'

  // 12. 生成唯一指纹种子
  const seed = randomFingerprintSeed()

  return {
    seed,
    brand,
    platform,
    lang,
    timezone,
    resolution,
    colorDepth,
    hardwareConcurrency: hardware.cores,
    deviceMemory: hardware.memory,
    fonts,
    webrtcPolicy,
    doNotTrack,
    touchPoints,
    // 使用自动混淆策略（不设置 webglVendor/webglRenderer/canvasNoise/audioNoise）
  }
}

/**
 * 批量生成多个指纹配置（确保多样性）
 */
export function generateBatchFingerprints(
  count: number,
  options: SmartGenerateOptions = {},
): FingerprintConfig[] {
  const fingerprints: FingerprintConfig[] = []
  const usedSeeds = new Set<string>()

  for (let i = 0; i < count; i++) {
    let config = generateSmartFingerprint(options)

    // 确保种子唯一
    while (config.seed && usedSeeds.has(config.seed)) {
      config = generateSmartFingerprint(options)
    }

    if (config.seed) {
      usedSeeds.add(config.seed)
    }

    fingerprints.push(config)
  }

  return fingerprints
}

/**
 * 根据现有配置生成相似但不同的配置（用于克隆 profile）
 */
export function generateSimilarFingerprint(base: FingerprintConfig): FingerprintConfig {
  // 保持平台、品牌、地区相关设置，但改变硬件细节和种子
  const hardware = weightedRandom(HARDWARE_CONFIGS)

  return {
    ...base,
    seed: randomFingerprintSeed(), // 新种子
    hardwareConcurrency: hardware.cores, // 改变 CPU
    deviceMemory: hardware.memory, // 改变内存
    // 其他保持不变（平台、品牌、语言、时区等）
  }
}
