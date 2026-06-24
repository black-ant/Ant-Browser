import { Apple, Bot, Globe2, Monitor, ShoppingCart, Smartphone, Sparkles, Terminal, Zap } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { Card } from '../../../../shared/components'
import type { FingerprintProfileConfig } from './types'
import type { BrowserCore as BrowserCoreInfo } from '../../types'
import { generateRandomFingerprint } from '../../../../services/fingerprintGenerator'
import { applyPreset } from '../../../../services/presetConfigs'
import { findMatchingDeviceProfile } from '../../../../services/deviceProfiles'

interface RightPanelProps {
  config: FingerprintProfileConfig
  updateConfig: (updater: (prev: FingerprintProfileConfig) => FingerprintProfileConfig) => void
  cores: BrowserCoreInfo[]
  selectedCoreId: string
}

export function RightPanel({ config, updateConfig, cores, selectedCoreId }: RightPanelProps) {
  const presets = [
    { id: 'general' as const, name: '通用', icon: Zap },
    { id: 'social' as const, name: '海外社媒', icon: Globe2 },
    { id: 'ecommerce' as const, name: '跨境电商', icon: ShoppingCart },
    { id: 'ai' as const, name: 'AI 专用', icon: Bot },
  ]

  // 格式化显示值
  const formatValue = (type: string, value: string | number | undefined): string => {
    if (type === 'random') return '随机'
    if (type === 'real') return '真实'
    return value?.toString() || '-'
  }

  const formatUserAgent = (): string => {
    if (config.userAgent.type === 'random') return '随机'
    if (config.userAgent.type === 'real') return '真实'
    return config.userAgent.value.length > 60
      ? config.userAgent.value.substring(0, 60) + '...'
      : config.userAgent.value || '-'
  }

  const formatGeolocation = (): string => {
    const { type, latitude, longitude } = config.basic.geolocation
    if (type === 'real') return '真实'
    if (type === 'custom' && typeof latitude === 'number' && typeof longitude === 'number') {
      return `${latitude}, ${longitude}`
    }
    return '自动'
  }

  const resolutionSummary = config.basic.resolution.type === 'custom'
    ? `${config.basic.resolution.width} × ${config.basic.resolution.height}`
    : formatValue(config.basic.resolution.type, undefined)
  const windowSummary = config.basic.windowMode === 'fullscreen'
    ? '全屏'
    : resolutionSummary

  const colorDepthSummary = config.basic.colorDepth.type === 'custom'
    ? `${config.basic.colorDepth.value} 位`
    : formatValue(config.basic.colorDepth.type, undefined)
  const selectedCore = selectedCoreId
    ? cores.find(core => core.coreId === selectedCoreId)
    : cores.find(core => core.isDefault)
  const browserSummary = selectedCore?.coreName || config.coreVersion || config.browserCore
  const matchedDeviceProfile = findMatchingDeviceProfile(config)

  // 获取平台显示名称和图标
  const getPlatformInfo = (): { name: string; Icon: LucideIcon } => {
    switch (config.platform) {
      case 'windows':
        return { name: matchedDeviceProfile?.osLabel || 'Windows', Icon: Monitor }
      case 'mac':
        return { name: matchedDeviceProfile?.osLabel || 'macOS', Icon: Apple }
      case 'linux':
        return { name: matchedDeviceProfile?.osLabel || 'Linux', Icon: Terminal }
      case 'android':
        return { name: matchedDeviceProfile?.osLabel || 'Android', Icon: Smartphone }
      default:
        return { name: 'Windows', Icon: Monitor }
    }
  }

  const platformInfo = getPlatformInfo()
  const PlatformIcon = platformInfo.Icon
  const webglSummary = (
    config.advanced.webgl.image.type === 'real' ||
    config.advanced.webgl.vendor.type === 'real' ||
    config.advanced.webgl.renderer.type === 'real'
  )
    ? '真实 GPU'
    : '指纹种子自动混淆'
  const webgpuSummary = config.advanced.webgpu.mode === 'disable'
    ? '禁用'
    : config.advanced.webgpu.mode === 'real'
    ? '真实'
    : '跟随 WebGL'
  const startupArgCount = config.advanced.startupArgs
    .split(/\r?\n/)
    .map(item => item.trim())
    .filter(Boolean)
    .length
  const keywordCount = config.preferences.keywords
    .map(item => item.trim())
    .filter(Boolean)
    .length
  const extensionCount = (config.preferences.extensionIds || [])
    .map(item => item.trim())
    .filter(Boolean)
    .length
  const searchEngineName = {
    google: 'Google',
    bing: 'Bing',
    duckduckgo: 'DuckDuckGo',
    baidu: '百度',
  }[config.basic.searchEngine] || 'Google'
  const launchCodeSummary = config.preferences.launchCode.trim()
    ? config.preferences.launchCode.trim().toUpperCase()
    : '自动生成'

  // 根据配置生成概要信息
  const summaryItems = [
    { label: '操作系统', value: platformInfo.name },
    { label: '设备画像', value: matchedDeviceProfile?.name || '自定义组合' },
    { label: '浏览器', value: browserSummary },
    { label: 'User-Agent', value: formatUserAgent() },
    { label: '语言', value: config.basic.language === 'auto' ? '自动匹配' : config.basic.language },
    { label: '界面语言', value: config.basic.uiLanguage.mode === 'auto' ? '自动匹配' : config.basic.uiLanguage.value },
    { label: '时区', value: formatValue(config.basic.timezone.type, config.basic.timezone.value) },
    { label: '地理位置', value: formatGeolocation() },
    { label: '窗口大小', value: windowSummary },
    {
      label: '网页内容',
      value: `声音${config.basic.content.sound ? '开' : '关'} 图片${config.basic.content.images ? '开' : '关'} 视频${config.basic.content.video ? '开' : '关'}`
    },
    { label: '搜索引擎', value: searchEngineName },
    { label: '色深', value: colorDepthSummary },
    {
      label: 'WebRTC',
      value: config.advanced.webrtc.policy === 'disable_non_proxied_udp'
        ? '禁止非代理UDP'
        : config.advanced.webrtc.policy === 'disable_all'
        ? '完全禁止'
        : '默认'
    },
    { label: 'WebGL', value: webglSummary },
    { label: 'WebGPU', value: webgpuSummary },
    { label: 'GPU 画像', value: matchedDeviceProfile?.gpuRenderer || '自动匹配' },
    { label: 'Canvas', value: formatValue(config.advanced.canvas.type, '自动混淆') },
    { label: 'AudioContext', value: formatValue(config.advanced.audioContext.type, '自动混淆') },
    { label: 'Client Rects', value: formatValue(config.advanced.clientRects.type, '自动混淆') },
    { label: '字体指纹', value: formatValue(config.basic.fonts.type, '自动混淆') },
    { label: 'Do Not Track', value: config.advanced.doNotTrack ? '开启' : '关闭' },
    { label: '硬件并发数', value: formatValue(config.advanced.hardwareConcurrency.type, `${config.advanced.hardwareConcurrency.value} 核`) },
    { label: '设备内存', value: formatValue(config.advanced.deviceMemory.type, `${config.advanced.deviceMemory.value} GB`) },
    {
      label: '媒体设备',
      value: config.advanced.mediaDevices.type === 'custom'
        ? `摄像头${config.advanced.mediaDevices.videoInputs} 麦克风${config.advanced.mediaDevices.audioInputs} 扬声器${config.advanced.mediaDevices.audioOutputs}`
        : formatValue(config.advanced.mediaDevices.type, '随机')
    },
    { label: '触摸点数', value: formatValue(config.advanced.touchPoints.type, `${config.advanced.touchPoints.value} 点`) },
    { label: '硬件加速', value: config.advanced.hardwareAcceleration ? '开启' : '关闭' },
    { label: '沙盒', value: config.advanced.disableSandbox ? '已禁用' : '开启' },
    { label: '启动关键字', value: keywordCount > 0 ? `${keywordCount} 个` : '无' },
    { label: '启动码', value: launchCodeSummary },
    { label: '启动扩展', value: extensionCount > 0 ? `${extensionCount} 个` : '无' },
    { label: '额外启动参数', value: startupArgCount > 0 ? `${startupArgCount} 条` : '无' },
  ]

  // 一键随机指纹
  const handleRandomFingerprint = () => {
    updateConfig(prev => generateRandomFingerprint(prev, selectedCore?.coreName))
  }

  // 应用预设配置
  const handleApplyPreset = (presetId: 'general' | 'social' | 'ecommerce' | 'ai') => {
    updateConfig(prev => applyPreset(presetId, prev, selectedCore?.coreName))
  }

  return (
    <div className="browser-create-side-panel space-y-5">
      {/* 一键配置 */}
      <Card className="browser-create-preset-card">
        <div className="p-6">
          <div className="flex items-center justify-between mb-4">
            <h3 className="text-base font-semibold text-[var(--color-text-primary)]">一键配置</h3>
            <Sparkles className="w-5 h-5 text-[#1890ff]" />
          </div>

          <div className="browser-create-preset-grid grid grid-cols-2 gap-3">
            {presets.map(preset => {
              const Icon = preset.icon
              return (
                <button
                  key={preset.id}
                  onClick={() => handleApplyPreset(preset.id)}
                  className="browser-create-preset-button flex flex-col items-center gap-2 p-4 rounded-lg border-2 border-[var(--color-border-default)] hover:border-[#1890ff] hover:bg-[rgba(59,130,246,0.12)] transition-all group"
                >
                  <Icon className="w-6 h-6 text-[var(--color-text-muted)] group-hover:text-[#1890ff] transition-colors" />
                  <span className="text-sm font-medium text-[var(--color-text-secondary)] group-hover:text-[#1890ff] transition-colors">
                    {preset.name}
                  </span>
                </button>
              )
            })}
          </div>
        </div>
      </Card>

      {/* 概要信息 */}
      <Card className="browser-create-summary-card">
        <div className="p-6">
          <h3 className="text-base font-semibold text-[var(--color-text-primary)] mb-4">概要</h3>

          {/* 视觉展示区 */}
          <div className="mb-4 p-4 bg-[var(--color-bg-elevated)] rounded-lg border border-[rgba(59,130,246,0.28)]">
            <div className="flex items-center gap-3 mb-3">
              <div className="w-12 h-12 flex items-center justify-center bg-[rgba(59,130,246,0.12)] border border-[rgba(59,130,246,0.24)] rounded-lg shadow-sm">
                <PlatformIcon className="w-6 h-6 text-[#1890ff]" />
              </div>
              <div>
                <div className="text-sm font-semibold text-[var(--color-text-primary)]">{platformInfo.name}</div>
                <div className="text-xs text-[var(--color-text-muted)]">
                  {matchedDeviceProfile?.name || browserSummary}
                </div>
              </div>
            </div>

            {/* 壁纸缩略图 */}
            <div className="browser-create-profile-visual relative aspect-video bg-[var(--color-bg-surface)] rounded overflow-hidden shadow-inner border border-[var(--color-border-default)]">
              <div className="absolute inset-0 flex items-center justify-center">
                <Monitor className="w-12 h-12 text-white opacity-20" />
              </div>
              <div className="absolute bottom-2 left-2 text-xs text-white font-medium opacity-75">
                {resolutionSummary}
              </div>
            </div>
          </div>

          {/* 属性列表区（可滚动） */}
          <div className="browser-create-summary-list max-h-[500px] overflow-y-auto pr-2 space-y-2 scrollbar-thin scrollbar-thumb-[var(--color-border-strong)] scrollbar-track-[var(--color-bg-muted)]">
            {summaryItems.map((item, index) => (
              <div
                key={index}
                className="flex justify-between items-start text-xs py-2 border-b border-[var(--color-border-muted)] last:border-b-0"
              >
                <span className="text-[#1890ff] font-medium mr-2 flex-shrink-0">
                  {item.label}
                </span>
                <span className="text-[var(--color-text-primary)] text-right break-all">
                  {item.value}
                </span>
              </div>
            ))}
          </div>

          {/* 一键随机按钮 */}
          <div className="mt-6 pt-6 border-t border-[var(--color-border-muted)]">
            <button
              onClick={handleRandomFingerprint}
              className="browser-create-primary-action w-full flex items-center justify-center gap-2 py-2.5 text-sm font-medium text-white bg-[#1890ff] hover:bg-[#1070d0] rounded-lg transition-colors shadow-sm"
            >
              <Sparkles className="w-4 h-4" />
              一键随机指纹
            </button>
          </div>
        </div>
      </Card>
    </div>
  )
}
