import { useEffect, useMemo, useState } from 'react'
import { Download, RefreshCw, Trash2 } from 'lucide-react'
import { Badge, Button, Card, Input, Table, toast } from '../../../shared/components'
import type { TableColumn } from '../../../shared/components/Table'
import type { CookieInfo } from '../types'
import { clearBrowserCookies, exportBrowserCookies, fetchBrowserCookies } from '../api'

interface Props {
  profileId: string
  profileName: string
  running: boolean
  ready: boolean
}

const formatExpires = (expires: number) => {
  if (expires <= 0) return 'Session'
  return new Date(expires * 1000).toLocaleString('zh-CN')
}

export function CookieManagerCard({ profileId, profileName, running, ready }: Props) {
  const [cookies, setCookies] = useState<CookieInfo[]>([])
  const [filterDomain, setFilterDomain] = useState('')
  const [loading, setLoading] = useState(false)
  const [clearing, setClearing] = useState(false)
  const [showConfirm, setShowConfirm] = useState(false)

  const loadCookies = async () => {
    if (!ready) return
    setLoading(true)
    try {
      const list = await fetchBrowserCookies(profileId)
      setCookies(list)
    } catch {
      toast.error('获取 Cookie 失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (ready) loadCookies()
    else setCookies([])
  }, [profileId, ready])

  const filteredCookies = useMemo(() => {
    if (!filterDomain.trim()) return cookies
    const kw = filterDomain.toLowerCase()
    return cookies.filter(c => c.domain.toLowerCase().includes(kw))
  }, [cookies, filterDomain])

  const handleClear = async () => {
    setClearing(true)
    try {
      await clearBrowserCookies(profileId)
      setCookies([])
      toast.success('Cookie 已清除')
    } catch {
      toast.error('清除 Cookie 失败')
    } finally {
      setClearing(false)
      setShowConfirm(false)
    }
  }

  const handleExport = async () => {
    try {
      const content = await exportBrowserCookies(profileId)
      const date = new Date().toISOString().slice(0, 10)
      const filename = `cookies_${profileName}_${date}.txt`
      const blob = new Blob([content], { type: 'text/plain' })
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = filename
      a.click()
      URL.revokeObjectURL(url)
      toast.success('Cookie 已导出')
    } catch {
      toast.error('导出 Cookie 失败')
    }
  }

  const columns: TableColumn<CookieInfo>[] = [
    { key: 'domain', title: '域名', render: v => <span className="font-mono text-xs">{v}</span> },
    { key: 'name', title: '名称', render: v => <span className="font-mono text-xs">{v}</span> },
    {
      key: 'value',
      title: '值',
      render: v => (
        <span className="font-mono text-xs max-w-[120px] truncate block" title={v as string}>{v}</span>
      ),
    },
    { key: 'expires', title: '过期时间', render: v => formatExpires(v as number) },
    {
      key: 'httpOnly',
      title: 'HttpOnly',
      render: v => <Badge variant={v ? 'success' : 'default'}>{v ? '是' : '否'}</Badge>,
    },
    {
      key: 'secure',
      title: 'Secure',
      render: v => <Badge variant={v ? 'success' : 'default'}>{v ? '是' : '否'}</Badge>,
    },
  ]

  const subtitle = !running
    ? '窗口未运行，无法管理 Cookie'
    : !ready
      ? '窗口运行中，等待调试接口就绪后可管理 Cookie'
      : `共 ${cookies.length} 条${filterDomain ? `，已过滤 ${filteredCookies.length} 条` : ''}`

  return (
    <Card title="Cookie 管理" subtitle={subtitle}>
      {!running ? (
        <p className="text-sm text-[var(--color-text-muted)] py-4 text-center">
          请先启动窗口以查看 Cookie
        </p>
      ) : !ready ? (
        <p className="text-sm text-[var(--color-text-muted)] py-4 text-center">
          浏览器已启动，正在等待调试接口就绪
        </p>
      ) : (
        <div className="space-y-3">
          <div className="flex flex-col sm:flex-row gap-2 items-start sm:items-center justify-between">
            <Input
              placeholder="按域名过滤..."
              value={filterDomain}
              onChange={e => setFilterDomain(e.target.value)}
              className="w-full sm:w-64"
            />
            <div className="flex gap-2 flex-shrink-0">
              <Button size="sm" variant="ghost" onClick={loadCookies} disabled={loading}>
                <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
                刷新
              </Button>
              <Button size="sm" variant="ghost" onClick={handleExport}>
                <Download className="w-4 h-4" />
                导出 Netscape
              </Button>
              <Button size="sm" variant="secondary" onClick={() => setShowConfirm(true)} disabled={clearing}>
                <Trash2 className="w-4 h-4" />
                清除全部
              </Button>
            </div>
          </div>

          {showConfirm && (
            <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-surface-elevated)] p-4 flex items-center justify-between gap-4">
              <span className="text-sm text-[var(--color-text-secondary)]">
                确认清除该窗口的所有 Cookie？此操作不可撤销。
              </span>
              <div className="flex gap-2 flex-shrink-0">
                <Button size="sm" variant="ghost" onClick={() => setShowConfirm(false)}>取消</Button>
                <Button size="sm" onClick={handleClear} disabled={clearing}>确认清除</Button>
              </div>
            </div>
          )}

          <Table columns={columns} data={filteredCookies} rowKey="name" />
        </div>
      )}
    </Card>
  )
}
