// 指纹参数序列化/反序列化工具

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
  platform?: string        // --fingerprint-platform=
  lang?: string            // --lang=
  timezone?: string        // --timezone=

  // 屏幕与窗口
  resolution?: string      // --window-size=（预设值或 'custom'）
  customResolution?: string // 当 resolution === 'custom' 时使用
  colorDepth?: string      // --fingerprint-color-depth=

  // 硬件信息
  hardwareConcurrency?: string  // --fingerprint-hardware-concurrency=
  deviceMemory?: string         // --fingerprint-device-memory=

  // 渲染指纹（Chrome 144+ 使用新方案）
  canvasNoise?: boolean         // --fingerprint-canvas-noise=（已废弃）
  webglVendor?: string          // --fingerprint-webgl-vendor=（已废弃，Chrome 144+ 不支持）
  webglRenderer?: string        // --fingerprint-webgl-renderer=（已废弃，Chrome 144+ 不支持）
  audioNoise?: boolean          // --fingerprint-audio-noise=（已废弃）

  // Chrome 144+ 新参数：选择性禁用指纹混淆
  disableSpoofing?: string[]    // --disable-spoofing=font,audio,canvas,clientrects,gpu

  // 字体
  fonts?: string                // --fingerprint-fonts=

  // 网络与隐私
  webrtcPolicy?: string         // --webrtc-ip-handling-policy=
  doNotTrack?: boolean          // --fingerprint-do-not-track=

  // 媒体设备
  mediaDevices?: string         // --fingerprint-media-devices= (格式: "2,1,0" 摄像头,麦克风,扬声器)

  // 触摸
  touchPoints?: string          // --fingerprint-touch-points=

  unknownArgs?: string[]        // 无法识别的原始参数，原样保留
}

export const PRESET_RESOLUTIONS = ['1920,1080', '1440,900', '1366,768', '2560,1440', '1280,800', '1600,900']

// CLI 参数前缀 → FingerprintConfig 字段映射
export const KEY_MAP: Record<string, keyof FingerprintConfig> = {
  '--fingerprint': 'seed',
  '--fingerprint-brand': 'brand',
  '--fingerprint-platform': 'platform',
  '--lang': 'lang',
  '--timezone': 'timezone',
  '--window-size': 'resolution',
  '--fingerprint-color-depth': 'colorDepth',
  '--fingerprint-hardware-concurrency': 'hardwareConcurrency',
  '--fingerprint-device-memory': 'deviceMemory',
  '--fingerprint-canvas-noise': 'canvasNoise',
  '--fingerprint-webgl-vendor': 'webglVendor',
  '--fingerprint-webgl-renderer': 'webglRenderer',
  '--fingerprint-audio-noise': 'audioNoise',
  '--fingerprint-fonts': 'fonts',
  '--webrtc-ip-handling-policy': 'webrtcPolicy',
  '--fingerprint-do-not-track': 'doNotTrack',
  '--fingerprint-media-devices': 'mediaDevices',
  '--fingerprint-touch-points': 'touchPoints',
  '--disable-spoofing': 'disableSpoofing',
}

// FingerprintConfig → string[]
export function serialize(config: FingerprintConfig): string[] {
  const args: string[] = []
  if (config.seed) args.push(`--fingerprint=${config.seed}`)
  if (config.brand) args.push(`--fingerprint-brand=${config.brand}`)
  if (config.platform) args.push(`--fingerprint-platform=${config.platform}`)
  if (config.lang) args.push(`--lang=${config.lang}`)
  if (config.timezone) {
    // 如果是 system，替换为实际系统时区
    const tz = config.timezone === 'system' ? getSystemTimezone() : config.timezone
    args.push(`--timezone=${tz}`)
  }

  const res = config.resolution === 'custom' ? config.customResolution : config.resolution
  if (res) args.push(`--window-size=${res}`)

  if (config.colorDepth) args.push(`--fingerprint-color-depth=${config.colorDepth}`)
  if (config.hardwareConcurrency) args.push(`--fingerprint-hardware-concurrency=${config.hardwareConcurrency}`)
  if (config.deviceMemory) args.push(`--fingerprint-device-memory=${config.deviceMemory}`)

  // 废弃参数（Chrome 144+ 不支持，但保留用于向后兼容）
  if (config.canvasNoise !== undefined) args.push(`--fingerprint-canvas-noise=${config.canvasNoise}`)
  if (config.webglVendor) args.push(`--fingerprint-webgl-vendor=${config.webglVendor}`)
  if (config.webglRenderer) args.push(`--fingerprint-webgl-renderer=${config.webglRenderer}`)
  if (config.audioNoise !== undefined) args.push(`--fingerprint-audio-noise=${config.audioNoise}`)

  // Chrome 144+ 新参数：选择性禁用指纹混淆
  if (config.disableSpoofing && config.disableSpoofing.length > 0) {
    args.push(`--disable-spoofing=${config.disableSpoofing.join(',')}`)
  }

  if (config.fonts) args.push(`--fingerprint-fonts=${config.fonts}`)

  if (config.webrtcPolicy) args.push(`--webrtc-ip-handling-policy=${config.webrtcPolicy}`)
  if (config.doNotTrack !== undefined) args.push(`--fingerprint-do-not-track=${config.doNotTrack}`)
  if (config.mediaDevices) args.push(`--fingerprint-media-devices=${config.mediaDevices}`)
  if (config.touchPoints) args.push(`--fingerprint-touch-points=${config.touchPoints}`)

  return [...args, ...(config.unknownArgs ?? [])]
}

// string[] → FingerprintConfig
export function deserialize(args: string[]): FingerprintConfig {
  const config: FingerprintConfig = { unknownArgs: [] }

  for (const arg of args) {
    const eqIdx = arg.indexOf('=')
    if (eqIdx === -1) {
      config.unknownArgs!.push(arg)
      continue
    }
    const key = arg.slice(0, eqIdx)
    const val = arg.slice(eqIdx + 1)
    const field = KEY_MAP[key]

    if (!field) {
      config.unknownArgs!.push(arg)
      continue
    }

    if (field === 'canvasNoise' || field === 'audioNoise' || field === 'doNotTrack') {
      (config as Record<string, unknown>)[field] = val === 'true'
    } else if (field === 'resolution') {
      if (PRESET_RESOLUTIONS.includes(val)) {
        config.resolution = val
      } else {
        config.resolution = 'custom'
        config.customResolution = val
      }
    } else if (field === 'disableSpoofing') {
      // 解析逗号分隔的值：--disable-spoofing=font,audio,canvas,clientrects,gpu
      config.disableSpoofing = val ? val.split(',').map(s => s.trim()).filter(Boolean) : []
    } else {
      (config as Record<string, unknown>)[field] = val
    }
  }

  return config
}

// 生成随机指纹种子（32位正整数）
export function randomFingerprintSeed(): string {
  return String(Math.floor(Math.random() * 2147483647) + 1)
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
      colorDepth: '24',
      hardwareConcurrency: '8',
      deviceMemory: '8',
      fonts: 'Arial,Microsoft YaHei,SimSun,SimHei,Helvetica,Times New Roman',
      webrtcPolicy: 'disable_non_proxied_udp',
      doNotTrack: false,
      touchPoints: '0',
      // 使用自动混淆（不设置 webglVendor/webglRenderer/canvasNoise/audioNoise）
    },
  },
  {
    id: 'win-chrome-gaming',
    name: 'Windows / Chrome / 游戏主机',
    description: '模拟高配游戏 PC，2560x1440',
    config: {
      brand: 'Chrome',
      platform: 'windows',
      lang: 'en-US',
      timezone: 'America/New_York',
      resolution: '2560,1440',
      colorDepth: '24',
      hardwareConcurrency: '16',
      deviceMemory: '16',
      fonts: 'Arial,Helvetica,Times New Roman,Courier New,Verdana',
      webrtcPolicy: 'disable_non_proxied_udp',
      doNotTrack: false,
      touchPoints: '0',
    },
  },
  {
    id: 'mac-chrome-designer',
    name: 'macOS / Chrome / 设计师',
    description: '模拟 Mac 设计师用户，Retina 分辨率',
    config: {
      brand: 'Chrome',
      platform: 'mac',
      lang: 'zh-CN',
      timezone: 'Asia/Shanghai',
      resolution: '2560,1440',
      colorDepth: '30',
      hardwareConcurrency: '10',
      deviceMemory: '16',
      fonts: 'Arial,Helvetica,PingFang SC,Hiragino Sans GB,STHeiti,Times New Roman',
      webrtcPolicy: 'disable_non_proxied_udp',
      doNotTrack: true,
      touchPoints: '0',
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
      colorDepth: '24',
      hardwareConcurrency: '4',
      deviceMemory: '4',
      fonts: 'Arial,Microsoft YaHei,Calibri,Segoe UI,Times New Roman',
      webrtcPolicy: 'default_public_interface_only',
      doNotTrack: false,
      touchPoints: '0',
    },
  },
  {
    id: 'win-chrome-us-user',
    name: 'Windows / Chrome / 美国用户',
    description: '模拟美国普通用户，英文环境',
    config: {
      brand: 'Chrome',
      platform: 'windows',
      lang: 'en-US',
      timezone: 'America/Los_Angeles',
      resolution: '1920,1080',
      colorDepth: '24',
      hardwareConcurrency: '8',
      deviceMemory: '8',
      fonts: 'Arial,Helvetica,Times New Roman,Courier New,Georgia',
      webrtcPolicy: 'disable_non_proxied_udp',
      doNotTrack: false,
      touchPoints: '0',
    },
  },
  {
    id: 'mac-safari-jp',
    name: 'macOS / Safari / 日本用户',
    description: '模拟日本 Mac 用户，Safari 风格，日语环境',
    config: {
      brand: 'Safari',
      platform: 'mac',
      lang: 'ja-JP',
      timezone: 'Asia/Tokyo',
      resolution: '1440,900',
      colorDepth: '24',
      hardwareConcurrency: '8',
      deviceMemory: '8',
      fonts: 'Arial,Helvetica,Hiragino Kaku Gothic ProN,Yu Gothic,Times New Roman',
      webrtcPolicy: 'disable_non_proxied_udp',
      doNotTrack: true,
      touchPoints: '0',
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
      colorDepth: '24',
      hardwareConcurrency: '8',
      deviceMemory: '8',
      fonts: 'Arial,Helvetica,Times New Roman,Courier New,Verdana',
      webrtcPolicy: 'disable_non_proxied_udp',
      doNotTrack: false,
      touchPoints: '0',
    },
  },
  {
    id: 'mac-chrome-us-edu',
    name: 'macOS / Chrome / 美国-教育',
    description: '模拟美国大学教育网 Mac 用户，英文环境 (en-US)',
    config: {
      brand: 'Chrome',
      platform: 'mac',
      lang: 'en-US',
      timezone: 'America/New_York',
      resolution: '1440,900',
      colorDepth: '24',
      hardwareConcurrency: '8',
      deviceMemory: '8',
      fonts: 'Arial,Helvetica,Times New Roman,Courier New,Georgia',
      webrtcPolicy: 'disable_non_proxied_udp',
      doNotTrack: false,
      touchPoints: '0',
    },
  },
]
