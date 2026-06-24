import type { FingerprintProfileConfig } from './types'
import type { BrowserCore } from '../../types'
import { extractChromeMajor } from '../../../../services/deviceProfiles'

export interface ValidationError {
  field: string
  message: string
}

// URL 格式验证
function isValidUrl(url: string): boolean {
  if (!url.trim()) return true // 空 URL 不验证
  try {
    new URL(url)
    return true
  } catch {
    return false
  }
}

function parseStartupArgs(value: string): string[] {
  return value
    .split(/\r?\n/)
    .map(item => item.trim())
    .filter(Boolean)
}

function isValidLaunchCode(value: string): boolean {
  return /^[A-Z0-9_-]{4,32}$/.test(value.trim().toUpperCase())
}

interface ValidateConfigOptions {
  selectedCore?: BrowserCore
}

export function validateConfig(config: FingerprintProfileConfig, options: ValidateConfigOptions = {}): ValidationError[] {
  const errors: ValidationError[] = []

  // 配置名称必填
  if (!config.profileName.trim()) {
    errors.push({
      field: 'profileName',
      message: '配置名称不能为空'
    })
  }

  // 代理配置验证：代理池模式仅校验已选代理 ID，手动模式才校验 host/port。
  if (config.proxy.mode === 'manual' && config.proxy.type !== 'none') {
    if (!config.proxy.host.trim()) {
      errors.push({
        field: 'proxy.host',
        message: '代理地址不能为空'
      })
    }

    if (!config.proxy.port.trim()) {
      errors.push({
        field: 'proxy.port',
        message: '代理端口不能为空'
      })
    } else {
      const portText = config.proxy.port.trim()
      const port = Number(portText)
      if (!/^\d+$/.test(portText) || !Number.isInteger(port) || port < 1 || port > 65535) {
        errors.push({
          field: 'proxy.port',
          message: '代理端口必须是 1-65535 之间的数字'
        })
      }
    }
  }

  if (config.userAgent.type === 'custom' && !config.userAgent.value.trim()) {
    errors.push({
      field: 'userAgent.value',
      message: '自定义 User-Agent 不能为空'
    })
  }

  if (config.userAgent.type === 'custom' && config.userAgent.value.trim()) {
    const uaChromeMajor = extractChromeMajor(config.userAgent.value)
    const coreChromeMajor = extractChromeMajor(options.selectedCore?.coreName, config.coreVersion)
    if (uaChromeMajor && coreChromeMajor && uaChromeMajor !== coreChromeMajor) {
      errors.push({
        field: 'userAgent.value',
        message: `User-Agent 的 Chrome ${uaChromeMajor} 与当前内核 Chrome ${coreChromeMajor} 不一致，请重新一键随机或修改 UA`
      })
    }

    const isMobileUA = /Android|Mobile|iPhone|iPad/i.test(config.userAgent.value)
    if (config.platform === 'android' && !isMobileUA) {
      errors.push({
        field: 'userAgent.value',
        message: 'Android 平台需要使用移动端 User-Agent'
      })
    }
    if (config.platform !== 'android' && isMobileUA) {
      errors.push({
        field: 'userAgent.value',
        message: '桌面平台不能使用移动端 User-Agent'
      })
    }
  }

  // URL 格式验证
  config.startupUrls.forEach((url, index) => {
    if (url && !isValidUrl(url)) {
      errors.push({
        field: `startupUrls.${index}`,
        message: `URL 格式不正确: ${url.substring(0, 50)}${url.length > 50 ? '...' : ''}`
      })
    }
  })

  if (config.basic.uiLanguage.mode === 'custom' && !config.basic.uiLanguage.value.trim()) {
    errors.push({
      field: 'basic.uiLanguage.value',
      message: '自定义界面语言不能为空'
    })
  }

  if (config.basic.geolocation.type === 'custom') {
    const { latitude, longitude, accuracy } = config.basic.geolocation
    if (typeof latitude !== 'number' || !Number.isFinite(latitude) || latitude < -90 || latitude > 90) {
      errors.push({
        field: 'basic.geolocation.latitude',
        message: '纬度必须是 -90 到 90 之间的数字'
      })
    }
    if (typeof longitude !== 'number' || !Number.isFinite(longitude) || longitude < -180 || longitude > 180) {
      errors.push({
        field: 'basic.geolocation.longitude',
        message: '经度必须是 -180 到 180 之间的数字'
      })
    }
    if (accuracy !== undefined && (!Number.isFinite(accuracy) || accuracy <= 0)) {
      errors.push({
        field: 'basic.geolocation.accuracy',
        message: '定位精度必须是大于 0 的数字'
      })
    }
  }

  if (config.basic.windowMode === 'custom' && config.basic.resolution.type === 'custom') {
    const { width, height } = config.basic.resolution
    if (!Number.isInteger(width) || !Number.isInteger(height) || width < 320 || height < 320 || width > 7680 || height > 4320) {
      errors.push({
        field: 'basic.resolution',
        message: '分辨率必须是合理的屏幕尺寸'
      })
    }
    if (config.platform === 'android' && width > height) {
      errors.push({
        field: 'basic.resolution',
        message: 'Android 设备建议使用竖屏分辨率'
      })
    }
    if (config.platform !== 'android' && height > width) {
      errors.push({
        field: 'basic.resolution',
        message: '桌面设备不应使用移动端竖屏分辨率'
      })
    }
  }

  if (config.advanced.touchPoints.type === 'custom') {
    if (config.platform === 'android' && config.advanced.touchPoints.value <= 0) {
      errors.push({
        field: 'advanced.touchPoints',
        message: 'Android 设备触摸点数应大于 0'
      })
    }
    if (config.platform !== 'android' && config.advanced.touchPoints.value > 0) {
      errors.push({
        field: 'advanced.touchPoints',
        message: '桌面设备触摸点数应为 0'
      })
    }
  }

  if (config.advanced.hardwareConcurrency.type === 'custom') {
    const value = config.advanced.hardwareConcurrency.value
    if (!Number.isInteger(value) || value < 1 || value > 128) {
      errors.push({
        field: 'advanced.hardwareConcurrency',
        message: '硬件并发数必须是 1-128 之间的整数'
      })
    }
  }

  if (config.advanced.deviceMemory.type === 'custom') {
    const value = config.advanced.deviceMemory.value
    if (!Number.isInteger(value) || value < 1 || value > 256) {
      errors.push({
        field: 'advanced.deviceMemory',
        message: '设备内存必须是 1-256 GB 之间的整数'
      })
    }
  }

  if (config.advanced.mediaDevices.type === 'custom') {
    const values = [
      config.advanced.mediaDevices.videoInputs,
      config.advanced.mediaDevices.audioInputs,
      config.advanced.mediaDevices.audioOutputs,
    ]
    if (values.some(value => !Number.isInteger(value) || value < 0 || value > 32)) {
      errors.push({
        field: 'advanced.mediaDevices',
        message: '媒体设备数量必须是 0-32 之间的整数'
      })
    }
  }

  if (config.preferences.launchCode.trim() && !isValidLaunchCode(config.preferences.launchCode)) {
    errors.push({
      field: 'preferences.launchCode',
      message: '启动码必须是 4-32 位，仅支持字母、数字、下划线和短横线'
    })
  }

  const startupArgs = parseStartupArgs(config.advanced.startupArgs)
  startupArgs.forEach((arg, index) => {
    if (!arg.startsWith('--')) {
      errors.push({
        field: `advanced.startupArgs.${index}`,
        message: `启动参数必须以 -- 开头: ${arg.substring(0, 50)}`
      })
    }
  })

  return errors
}

export function hasErrors(errors: ValidationError[]): boolean {
  return errors.length > 0
}

export function getFieldError(errors: ValidationError[], field: string): string | undefined {
  return errors.find(e => e.field === field)?.message
}
