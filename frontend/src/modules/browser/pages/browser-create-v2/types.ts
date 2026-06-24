// 指纹配置的值类型：真实 | 随机 | 自定义
export type FingerprintValueType = 'real' | 'random' | 'custom'

// 代理类型
export type ProxyType = 'none' | 'http' | 'https' | 'socks5'

// 系统平台
export type Platform = 'windows' | 'mac' | 'linux' | 'android'

// 浏览器内核
export type BrowserCore = 'chrome' | 'firefox'

// WebRTC 策略
export type WebRTCPolicy = 'default' | 'disable_non_proxied_udp' | 'disable_all'

// 地理位置提示
export type GeolocationPrompt = 'allow' | 'block' | 'prompt'

export type BrowserLanguageMode = 'auto' | 'custom'
export type WindowMode = 'custom' | 'fullscreen'
export type WebGPUMode = 'match_webgl' | 'real' | 'disable'
export type SearchEngineId = 'google' | 'bing' | 'duckduckgo' | 'baidu'

// 完整的指纹配置接口
export interface FingerprintProfileConfig {
  // ========== 基础信息 ==========
  profileName: string                    // 配置名称
  platform: Platform                     // 操作系统平台
  browserCore: BrowserCore               // 浏览器内核
  coreVersion: string                    // 内核版本（如 "Chrome 149"）
  userAgent: {
    type: FingerprintValueType           // 'real' | 'random' | 'custom'
    value: string                        // 实际的 UA 字符串
  }

  // ========== 代理配置 ==========
  proxy: {
    mode: 'pool' | 'manual'              // 代理模式：代理池 | 手动配置
    proxyId: string                      // 代理池节点 ID（mode=pool 时使用）
    type: ProxyType                      // 代理类型（mode=manual 时使用）
    host: string                         // 代理 IP（mode=manual 时使用）
    port: string                         // 代理端口（mode=manual 时使用）
    username: string                     // 代理用户名（mode=manual 时使用）
    password: string                     // 代理密码（mode=manual 时使用）
  }

  // ========== 平台账号 ==========
  account: {
    platform: string                     // 平台名称（如 "Facebook", "Google"）
    username: string                     // 账号
    password: string                     // 密码
    cookies: string                      // Cookies 数据
  }

  // ========== URLs ==========
  startupUrls: string[]                  // 启动时打开的网址列表

  // ========== 基础设置 ==========
  basic: {
    language: string                     // 浏览器语言（"auto" 表示跟随代理/后端自动推导）
    uiLanguage: {
      mode: BrowserLanguageMode          // 界面语言模式：基于 IP 匹配 | 自定义
      value: string                      // Accept-Language 值，如 "en-US,en"
    }
    timezone: {
      type: FingerprintValueType         // 'real' | 'random' | 'custom'
      value: string                      // 时区字符串（如 "Asia/Shanghai"）
    }
    geolocation: {
      prompt: GeolocationPrompt          // 地理位置提示策略
      type: FingerprintValueType         // 'real' | 'random' | 'custom'
      latitude?: number                  // 纬度
      longitude?: number                 // 经度
      accuracy?: number                  // 精度
    }
    resolution: {
      type: FingerprintValueType         // 'real' | 'random' | 'custom'
      width: number                      // 宽度
      height: number                     // 高度
    }
    windowMode: WindowMode               // 窗口大小模式：自定义 | 全屏
    content: {
      sound: boolean                     // 声音
      images: boolean                    // 图片加载
      video: boolean                     // 视频/媒体播放
    }
    searchEngine: SearchEngineId         // 默认搜索引擎
    colorDepth: {
      type: FingerprintValueType         // 'real' | 'random' | 'custom'
      value: number                      // 色深（24/30/32）
    }
    fonts: {
      type: FingerprintValueType         // 'real' | 'random' | 'custom'
      list: string[]                     // 字体列表
    }
  }

  // ========== 高级指纹设置 ==========
  advanced: {
    webrtc: {
      policy: WebRTCPolicy               // WebRTC 策略
      publicIp: string                   // 公网 IP（如果需要伪造）
      localIp: string                    // 本地 IP（如果需要伪造）
    }
    webgl: {
      vendor: {
        type: FingerprintValueType       // 'real' | 'random' | 'custom'
        value: string                    // 如 "Intel Inc."
      }
      renderer: {
        type: FingerprintValueType       // 'real' | 'random' | 'custom'
        value: string                    // 如 "Intel Iris OpenGL Engine"
      }
      image: {
        type: FingerprintValueType       // WebGL 图像噪声
      }
    }
    webgpu: {
      mode: WebGPUMode                   // WebGPU 策略
    }
    canvas: {
      type: FingerprintValueType         // Canvas 指纹噪声
    }
    audioContext: {
      type: FingerprintValueType         // AudioContext 指纹噪声
    }
    clientRects: {
      type: FingerprintValueType         // ClientRects 噪声
    }
    speechVoices: {
      type: FingerprintValueType         // Speech Voices
    }
    doNotTrack: boolean                  // Do Not Track 开关
    hardwareConcurrency: {
      type: FingerprintValueType         // 'real' | 'random' | 'custom'
      value: number                      // CPU 核心数
    }
    deviceMemory: {
      type: FingerprintValueType         // 'real' | 'random' | 'custom'
      value: number                      // 设备内存（GB）
    }
    macAddress: {
      type: FingerprintValueType         // 'real' | 'random' | 'custom'
      value: string                      // MAC 地址
    }
    mediaDevices: {
      type: FingerprintValueType         // 'real' | 'random' | 'custom'
      videoInputs: number                // 摄像头数量
      audioInputs: number                // 麦克风数量
      audioOutputs: number               // 扬声器数量
    }
    touchPoints: {
      type: FingerprintValueType         // 'real' | 'random' | 'custom'
      value: number                      // 触摸点数量
    }
    sslFingerprint: boolean              // SSL 指纹开关
    portScanProtection: boolean          // 端口扫描保护
    hardwareAcceleration: boolean        // 硬件加速
    disableSandbox: boolean              // 禁用沙盒
    startupArgs: string                  // 额外启动参数，每行一个
  }

  // ========== 偏好设置 ==========
  preferences: {
    defaultProject: string               // 默认项目
    tags: string[]                       // 标签
    keywords: string[]                   // 关键字
    launchCode: string                   // 自动化启动码
    accountIds: string[]                 // 创建后关联的平台账号
    extensionIds: string[]               // 创建后绑定的本地扩展
    notes: string                        // 备注
    autoStart: boolean                   // 自动启动
    closeAction: 'minimize' | 'close'    // 关闭行为
  }
}

// 默认配置
export const DEFAULT_CONFIG: FingerprintProfileConfig = {
  // 基础信息
  profileName: '',
  platform: 'windows',
  browserCore: 'chrome',
  coreVersion: 'Chrome 149',
  userAgent: {
    type: 'random',
    value: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36',
  },

  // 代理配置
  proxy: {
    mode: 'manual',
    proxyId: '',
    type: 'none',
    host: '',
    port: '',
    username: '',
    password: '',
  },

  // 平台账号
  account: {
    platform: '',
    username: '',
    password: '',
    cookies: '',
  },

  // URLs
  startupUrls: [],

  // 基础设置
  basic: {
    language: 'auto',
    uiLanguage: {
      mode: 'auto',
      value: 'zh-CN,zh',
    },
    timezone: {
      type: 'random',
      value: 'Asia/Shanghai',
    },
    geolocation: {
      prompt: 'prompt',
      type: 'random',
      latitude: undefined,
      longitude: undefined,
      accuracy: undefined,
    },
    resolution: {
      type: 'random',
      width: 1920,
      height: 1080,
    },
    windowMode: 'custom',
    content: {
      sound: true,
      images: true,
      video: true,
    },
    searchEngine: 'google',
    colorDepth: {
      type: 'random',
      value: 24,
    },
    fonts: {
      type: 'random',
      list: [],
    },
  },

  // 高级指纹设置
  advanced: {
    webrtc: {
      policy: 'disable_non_proxied_udp',
      publicIp: '',
      localIp: '',
    },
    webgl: {
      vendor: {
        type: 'random',
        value: '',
      },
      renderer: {
        type: 'random',
        value: '',
      },
      image: {
        type: 'random',
      },
    },
    webgpu: {
      mode: 'match_webgl',
    },
    canvas: {
      type: 'random',
    },
    audioContext: {
      type: 'random',
    },
    clientRects: {
      type: 'random',
    },
    speechVoices: {
      type: 'random',
    },
    doNotTrack: false,
    hardwareConcurrency: {
      type: 'random',
      value: 8,
    },
    deviceMemory: {
      type: 'random',
      value: 8,
    },
    macAddress: {
      type: 'random',
      value: '',
    },
    mediaDevices: {
      type: 'random',
      videoInputs: 1,
      audioInputs: 1,
      audioOutputs: 1,
    },
    touchPoints: {
      type: 'random',
      value: 0,
    },
    sslFingerprint: false,
    portScanProtection: true,
    hardwareAcceleration: true,
    disableSandbox: false,
    startupArgs: '',
  },

  // 偏好设置
  preferences: {
    defaultProject: '',
    tags: [],
    keywords: [],
    launchCode: '',
    accountIds: [],
    extensionIds: [],
    notes: '',
    autoStart: false,
    closeAction: 'minimize',
  },
}
