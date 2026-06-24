import { useMemo } from 'react'
import type { BrowserProfileInput, BrowserProxy } from '../../types'

interface ConfigSummaryProps {
  formData: BrowserProfileInput
  proxy?: BrowserProxy
  extensions: any[]
  selectedExtensionIds: string[]
  accountCount: number
}

export function ConfigSummary({
  formData,
  proxy,
  extensions,
  selectedExtensionIds,
  accountCount
}: ConfigSummaryProps) {
  const summary = useMemo(() => {
    const items: Array<{ label: string; value: string }> = []

    // 基础信息
    if (formData.profileName) {
      items.push({ label: '配置名称', value: formData.profileName })
    }

    // 代理
    if (proxy) {
      items.push({ label: '代理', value: proxy.proxyName || proxy.proxyConfig })
    } else {
      items.push({ label: '代理', value: '直连' })
    }

    // 扩展
    const selectedExtensions = extensions.filter(e => selectedExtensionIds.includes(e.extensionId))
    if (selectedExtensions.length > 0) {
      items.push({ label: '扩展', value: `${selectedExtensions.length}个扩展` })
    }

    // 账号
    if (accountCount > 0) {
      items.push({ label: '关联账号', value: `${accountCount}个账号` })
    }

    // 启动参数
    if (formData.launchArgs && formData.launchArgs.length > 0) {
      items.push({ label: '启动参数', value: `${formData.launchArgs.length}个参数` })
    }

    return items
  }, [formData, proxy, extensions, selectedExtensionIds, accountCount])

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
