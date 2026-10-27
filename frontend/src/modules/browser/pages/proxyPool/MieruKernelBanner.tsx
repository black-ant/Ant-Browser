import { useEffect, useState } from 'react'
import { Download, X } from 'lucide-react'
import { Button } from '../../../../shared/components'
import { browserProxyCoreStatus } from '../../api'
import type { BrowserProxy } from '../../types'
import { defaultProxyCoreTarget } from './useProxyCoreDownload'

const MIERU_DISMISS_KEY = 'browser:proxyPool:mieruBannerDismissed'

interface MieruKernelBannerProps {
  proxies: BrowserProxy[]
  onDownload: () => void
}

// 检测到 mieru 协议节点但 mieru 内核未安装时，顶部提示下载（默认不加载内核）。
export function MieruKernelBanner({ proxies, onDownload }: MieruKernelBannerProps) {
  const [visible, setVisible] = useState(false)
  const [checking, setChecking] = useState(true)
  const [dismissed, setDismissed] = useState(false)

  useEffect(() => {
    try {
      setDismissed(sessionStorage.getItem(MIERU_DISMISS_KEY) === '1')
    } catch {
      setDismissed(false)
    }
  }, [])

  useEffect(() => {
    let cancelled = false
    const hasMieruNode = proxies.some(proxy => {
      const config = proxy.proxyConfig || ''
      return config.toLowerCase().includes('type:') && /\btype:\s*mieru\b/i.test(config)
    })
    if (!hasMieruNode) {
      setChecking(false)
      setVisible(false)
      return
    }
    const { goos, goarch } = defaultProxyCoreTarget()
    browserProxyCoreStatus('mieru', goos, goarch)
      .then(status => {
        if (cancelled) return
        setVisible(!status.installed && !status.configured)
      })
      .catch(() => {
        if (!cancelled) setVisible(false)
      })
      .finally(() => {
        if (!cancelled) setChecking(false)
      })
    return () => {
      cancelled = true
    }
  }, [proxies])

  if (checking || dismissed || !visible) {
    return null
  }

  return (
    <div className="mb-3 flex items-center justify-between gap-3 rounded-lg border border-amber-300 bg-amber-50 px-4 py-2.5 text-sm text-amber-900">
      <div className="min-w-0 flex-1">
        <span className="font-medium">检测到 mieru 协议节点</span>
        <span className="ml-2 text-amber-700">
          测速与实例代理需要 mieru 内核（约 5 MB，默认不随应用加载），是否现在下载？
        </span>
      </div>
      <div className="flex shrink-0 items-center gap-2">
        <Button size="sm" onClick={onDownload}>
          <Download className="h-4 w-4" />
          下载 mieru 内核
        </Button>
        <button
          type="button"
          className="rounded p-1 text-amber-600 hover:bg-amber-100"
          aria-label="忽略提示"
          title="本次会话内不再提示"
          onClick={() => {
            try {
              sessionStorage.setItem(MIERU_DISMISS_KEY, '1')
            } catch {
              // ignore
            }
            setDismissed(true)
          }}
        >
          <X className="h-4 w-4" />
        </button>
      </div>
    </div>
  )
}