import type { FingerprintProfileConfig } from '../modules/browser/pages/browser-create-v2/types'
import {
  applyDeviceProfile,
  chooseDeviceProfile,
  resolveChromeMajor,
  type DeviceProfilePreset,
} from './deviceProfiles'

type PresetId = DeviceProfilePreset

interface PresetOptions {
  language?: string
  timezone: FingerprintProfileConfig['basic']['timezone']
  geolocation: FingerprintProfileConfig['basic']['geolocation']
  webrtcPolicy: FingerprintProfileConfig['advanced']['webrtc']['policy']
  doNotTrack: boolean
}

const PRESET_OPTIONS: Record<PresetId, PresetOptions> = {
  general: {
    language: 'zh-CN',
    timezone: { type: 'random', value: 'Asia/Shanghai' },
    geolocation: {
      prompt: 'prompt',
      type: 'random',
      latitude: undefined,
      longitude: undefined,
      accuracy: undefined,
    },
    webrtcPolicy: 'disable_non_proxied_udp',
    doNotTrack: false,
  },
  social: {
    language: 'en-US',
    timezone: { type: 'custom', value: 'America/New_York' },
    geolocation: {
      prompt: 'prompt',
      type: 'custom',
      latitude: 40.7128,
      longitude: -74.0060,
      accuracy: 100,
    },
    webrtcPolicy: 'disable_all',
    doNotTrack: false,
  },
  ecommerce: {
    language: 'en-US',
    timezone: { type: 'custom', value: 'America/Los_Angeles' },
    geolocation: {
      prompt: 'prompt',
      type: 'custom',
      latitude: 34.0522,
      longitude: -118.2437,
      accuracy: 100,
    },
    webrtcPolicy: 'disable_non_proxied_udp',
    doNotTrack: false,
  },
  ai: {
    language: 'en-US',
    timezone: { type: 'custom', value: 'America/New_York' },
    geolocation: {
      prompt: 'prompt',
      type: 'random',
      latitude: undefined,
      longitude: undefined,
      accuracy: undefined,
    },
    webrtcPolicy: 'disable_all',
    doNotTrack: false,
  },
}

// 应用预设配置
export function applyPreset(
  presetId: PresetId,
  currentConfig: FingerprintProfileConfig,
  browserCoreName?: string,
): FingerprintProfileConfig {
  const options = PRESET_OPTIONS[presetId]
  if (!options) {
    return currentConfig
  }

  const profile = chooseDeviceProfile({
    platform: currentConfig.platform,
    preset: presetId,
  })

  return applyDeviceProfile(currentConfig, profile, {
    chromeMajor: resolveChromeMajor(currentConfig, browserCoreName),
    language: options.language,
    timezone: options.timezone,
    geolocation: options.geolocation,
    webrtcPolicy: options.webrtcPolicy,
    doNotTrack: options.doNotTrack,
  })
}
