import type { FingerprintProfileConfig } from '../modules/browser/pages/browser-create-v2/types'
import {
  applyDeviceProfile,
  chooseDeviceProfile,
  resolveChromeMajor,
} from './deviceProfiles'

// 一键随机指纹：选择一套完整设备画像，避免 UA、硬件、字体、分辨率互相错配。
export function generateRandomFingerprint(
  config: FingerprintProfileConfig,
  browserCoreName?: string,
): FingerprintProfileConfig {
  const profile = chooseDeviceProfile({ platform: config.platform })

  return applyDeviceProfile(config, profile, {
    chromeMajor: resolveChromeMajor(config, browserCoreName),
    timezone: {
      type: 'random',
      value: config.basic.timezone.value || profile.defaultTimezone,
    },
    geolocation: {
      prompt: 'prompt',
      type: 'random',
      latitude: undefined,
      longitude: undefined,
      accuracy: undefined,
    },
    doNotTrack: false,
  })
}
