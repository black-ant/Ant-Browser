import { useMemo } from 'react'
import type { FingerprintProfileConfig } from '../browser-create-v2/types'

interface ConfigSummaryProps {
  config: FingerprintProfileConfig
}

export function ConfigSummary({ config }: ConfigSummaryProps) {
  const summary = useMemo(() => {
    const items: Array<{ label: string; value: string }> = []

    // 基础信息
    if (config.name) {
      items.push({ label: '配置名称', value: config.name })
    }

    // 设备类型
    if (config.deviceType) {
      items.push({ label: '设备类型', value: config.deviceType })
    }

    // 操作系统
    if (config.os) {
      items.push({ label: '操作系统', value: config.os })
    }

    // 浏览器
    if (config.browser) {
      items.push({ label: '浏览器', value: config.browser })
    }

    // User Agent
    if (config.userAgent) {
      items.push({ label: 'User Agent', value: config.userAgent })
    }

    // 屏幕分辨率
    if (config.screen?.width && config.screen?.height) {
      items.push({
        label: '屏幕分辨率',
        value: `${config.screen.width} × ${config.screen.height}`,
      })
    }

    // 时区
    if (config.timezone) {
      items.push({ label: '时区', value: config.timezone })
    }

    // 语言
    if (config.languages?.length) {
      items.push({ label: '语言', value: config.languages.join(', ') })
    }

    // WebRTC
    if (config.webrtc?.mode) {
      items.push({ label: 'WebRTC', value: config.webrtc.mode })
    }

    // Canvas
    if (config.canvas?.mode) {
      items.push({ label: 'Canvas指纹', value: config.canvas.mode })
    }

    // WebGL
    if (config.webgl?.mode) {
      items.push({ label: 'WebGL指纹', value: config.webgl.mode })
    }

    return items
  }, [config])

  if (summary.length === 0) {
    return (
      <div className="text-sm text-[var(--color-text-muted)] text-center py-8">
        暂无配置信息
      </div>
    )
  }

  return (
    <div className="space-y-2">
      {summary.map((item, idx) => (
        <div key={idx} className="flex items-start gap-3 text-sm">
          <span className="text-[var(--color-text-muted)] min-w-[100px] shrink-0">
            {item.label}:
          </span>
          <span className="text-[var(--color-text-primary)] break-all">
            {item.value}
          </span>
        </div>
      ))}
    </div>
  )
}
