import { useEffect, useRef, useState } from 'react'
import { Badge, Button, Card, Select, Textarea, toast } from '../../../shared/components'
import type { BrowserProfile } from '../types'
import {
  fetchBrowserProfiles,
  CDPSessionCreate,
  CDPSessionClose,
  CDPGetNetworkRequests,
  CDPGetConsoleLogs,
  CDPClearNetworkRequests,
  CDPClearConsoleLogs,
  CDPExportHAR,
  CDPExecuteJavaScript,
  CDPCaptureScreenshot,
  CDPGetStatistics,
  CDPGetStorage,
  type CDPNetworkRequest,
  type CDPConsoleLog,
} from '../api'

type ToolType = 'network' | 'console' | 'storage' | 'javascript' | 'screenshot' | 'performance'

const TOOLS: { key: ToolType; label: string }[] = [
  { key: 'network', label: '网络抓包' },
  { key: 'console', label: '控制台' },
  { key: 'storage', label: '存储' },
  { key: 'javascript', label: '执行JS' },
  { key: 'screenshot', label: '截图' },
  { key: 'performance', label: '性能' },
]

const STAT_LABELS: Record<string, string> = {
  total: '请求总数',
  success: '成功',
  failed: '失败',
  totalSize: '总大小(字节)',
  avgDuration: '平均耗时(ms)',
  consoleLogs: 'Console 条数',
}

// 通过后端 CDP 会话（Go dialer，绕开 Chrome 对带 Origin 的 WebSocket 的拒绝）驱动开发工具。
export function BrowserDevToolsPageNew() {
  const [profiles, setProfiles] = useState<BrowserProfile[]>([])
  const [selectedProfileId, setSelectedProfileId] = useState('')
  const [sessionId, setSessionId] = useState<string | null>(null)
  const [capturing, setCapturing] = useState(false)
  const [activeTool, setActiveTool] = useState<ToolType>('network')

  const [requests, setRequests] = useState<CDPNetworkRequest[]>([])
  const [consoleLogs, setConsoleLogs] = useState<CDPConsoleLog[]>([])
  const [stats, setStats] = useState<Record<string, any>>({})
  const pollingRef = useRef<ReturnType<typeof setInterval> | null>(null)

  // 存储
  const [storageType, setStorageType] = useState<'localStorage' | 'sessionStorage'>('localStorage')
  const [storageData, setStorageData] = useState<Record<string, string>>({})
  const [storageLoading, setStorageLoading] = useState(false)

  // 执行 JS
  const [jsCode, setJsCode] = useState('')
  const [jsResult, setJsResult] = useState('')
  const [jsRunning, setJsRunning] = useState(false)

  // 截图
  const [screenshot, setScreenshot] = useState('')
  const [shotLoading, setShotLoading] = useState(false)

  useEffect(() => {
    loadProfiles()
    return () => {
      if (pollingRef.current) clearInterval(pollingRef.current)
      if (sessionId) CDPSessionClose(sessionId).catch(console.error)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const loadProfiles = async () => {
    try {
      const list = await fetchBrowserProfiles()
      // 只要在运行中即可选择（CDP 需要调试端口；debugReady 只是附加附着标志，不作为硬条件）。
      setProfiles(list.filter(p => p.running))
    } catch {
      toast.error('加载浏览器实例失败')
    }
  }

  const startCapture = async () => {
    if (!selectedProfileId) {
      toast.error('请选择一个运行中的浏览器实例')
      return
    }
    try {
      const newSessionId = await CDPSessionCreate(selectedProfileId, 'page')
      setSessionId(newSessionId)
      setCapturing(true)
      setRequests([])
      setConsoleLogs([])
      setStats({})
      toast.success('开发工具已连接')

      const interval = setInterval(async () => {
        try {
          const [networkData, consoleData, statData] = await Promise.all([
            CDPGetNetworkRequests(newSessionId),
            CDPGetConsoleLogs(newSessionId),
            CDPGetStatistics(newSessionId),
          ])
          setRequests(networkData || [])
          setConsoleLogs(consoleData || [])
          if (statData) setStats(statData)
        } catch (error) {
          console.error('轮询数据失败:', error)
        }
      }, 1000)
      pollingRef.current = interval
    } catch (error: any) {
      toast.error(error?.message || '连接失败')
      setCapturing(false)
    }
  }

  const stopCapture = async () => {
    if (pollingRef.current) {
      clearInterval(pollingRef.current)
      pollingRef.current = null
    }
    if (sessionId) {
      try { await CDPSessionClose(sessionId) } catch (error) { console.error(error) }
      setSessionId(null)
    }
    setCapturing(false)
    toast.success('已停止')
  }

  const requireSession = (): string | null => {
    if (!sessionId) {
      toast.error('请先点击「开始」连接实例')
      return null
    }
    return sessionId
  }

  const handleClearRequests = async () => {
    if (sessionId) { try { await CDPClearNetworkRequests(sessionId) } catch { /* ignore */ } }
    setRequests([])
  }

  const handleClearConsole = async () => {
    if (sessionId) { try { await CDPClearConsoleLogs(sessionId) } catch { /* ignore */ } }
    setConsoleLogs([])
  }

  const handleExportHAR = async () => {
    const sid = requireSession(); if (!sid) return
    try {
      const harData = await CDPExportHAR(sid)
      const blob = new Blob([harData], { type: 'application/json' })
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = `network-${Date.now()}.har`
      link.click()
      URL.revokeObjectURL(url)
      toast.success('HAR 已导出')
    } catch (error: any) {
      toast.error(error?.message || '导出失败')
    }
  }

  const loadStorage = async (type = storageType) => {
    const sid = requireSession(); if (!sid) return
    setStorageLoading(true)
    try {
      const data = await CDPGetStorage(sid, type)
      setStorageData(data || {})
    } catch (error: any) {
      toast.error(error?.message || '读取存储失败')
    } finally {
      setStorageLoading(false)
    }
  }

  const runJs = async () => {
    const sid = requireSession(); if (!sid) return
    if (!jsCode.trim()) { toast.error('请输入要执行的 JavaScript'); return }
    setJsRunning(true)
    try {
      setJsResult(await CDPExecuteJavaScript(sid, jsCode))
    } catch (error: any) {
      setJsResult(`错误: ${error?.message || error}`)
    } finally {
      setJsRunning(false)
    }
  }

  const captureShot = async () => {
    const sid = requireSession(); if (!sid) return
    setShotLoading(true)
    try {
      const b64 = await CDPCaptureScreenshot(sid)
      setScreenshot(b64 || '')
      if (!b64) toast.error('截图为空')
    } catch (error: any) {
      toast.error(error?.message || '截图失败')
    } finally {
      setShotLoading(false)
    }
  }

  return (
    <div className="p-6 space-y-5 animate-fade-in">
      <div>
        <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">浏览器开发工具</h1>
        <p className="text-sm text-[var(--color-text-muted)] mt-1">基于 Chrome DevTools Protocol（经后端会话连接，避免 WebSocket Origin 限制）</p>
      </div>

      {/* 控制栏 */}
      <Card>
        <div className="flex items-center gap-3 p-1">
          <Select value={selectedProfileId} onChange={(e) => setSelectedProfileId(e.target.value)} disabled={capturing} className="flex-1" options={[]}>
            <option value="">选择运行中的浏览器实例</option>
            {profiles.map(p => (
              <option key={p.profileId} value={p.profileId}>{p.profileName}（端口 {p.debugPort}）</option>
            ))}
          </Select>
          <Button onClick={capturing ? stopCapture : startCapture} variant={capturing ? 'secondary' : undefined}>
            {capturing ? '停止' : '连接'}
          </Button>
          <Button variant="ghost" onClick={loadProfiles} disabled={capturing}>刷新实例</Button>
        </div>
        {profiles.length === 0 && (
          <p className="text-xs text-[var(--color-text-muted)] px-1 pt-2">没有"运行中且调试就绪"的实例——请先在实例列表启动一个实例。</p>
        )}
      </Card>

      {/* 工具切换 */}
      <div className="flex gap-2 flex-wrap">
        {TOOLS.map(t => (
          <Button key={t.key} size="sm" variant={activeTool === t.key ? undefined : 'secondary'} onClick={() => setActiveTool(t.key)}>
            {t.label}
            {t.key === 'network' && requests.length > 0 ? ` (${requests.length})` : ''}
            {t.key === 'console' && consoleLogs.length > 0 ? ` (${consoleLogs.length})` : ''}
          </Button>
        ))}
      </div>

      {/* 网络抓包 */}
      {activeTool === 'network' && (
        <Card title="网络抓包">
          <div className="flex gap-2 mb-3">
            <Button size="sm" variant="secondary" onClick={handleClearRequests}>清空</Button>
            <Button size="sm" variant="secondary" onClick={handleExportHAR}>导出 HAR</Button>
          </div>
          <div className="space-y-2 max-h-[480px] overflow-y-auto">
            {requests.length === 0 ? (
              <p className="text-sm text-[var(--color-text-muted)] text-center py-8">{capturing ? '等待网络请求…' : '未连接'}</p>
            ) : requests.map(req => (
              <div key={req.requestId} className="p-2.5 border border-[var(--color-border-default)] rounded-lg">
                <div className="flex items-center gap-2">
                  <Badge variant={req.statusCode >= 200 && req.statusCode < 400 ? 'success' : req.statusCode >= 400 ? 'error' : 'default'}>
                    {req.statusCode || 'Pending'}
                  </Badge>
                  <span className="font-mono text-xs shrink-0">{req.method}</span>
                  <span className="truncate text-xs text-[var(--color-text-secondary)]">{req.url}</span>
                </div>
                <div className="text-[11px] text-[var(--color-text-muted)] mt-1">
                  {req.type} • {req.size ? `${(req.size / 1024).toFixed(2)} KB` : '-'} • {req.duration ? `${req.duration}ms` : '-'}
                </div>
              </div>
            ))}
          </div>
        </Card>
      )}

      {/* 控制台 */}
      {activeTool === 'console' && (
        <Card title="控制台">
          <div className="flex gap-2 mb-3">
            <Button size="sm" variant="secondary" onClick={handleClearConsole}>清空</Button>
          </div>
          <div className="space-y-1.5 font-mono text-xs max-h-[480px] overflow-y-auto">
            {consoleLogs.length === 0 ? (
              <p className="text-center py-8 text-[var(--color-text-muted)]">{capturing ? '等待日志…' : '未连接'}</p>
            ) : consoleLogs.map(log => (
              <div key={log.id} className={`p-2 rounded ${log.type === 'error' ? 'bg-red-50 text-red-700 dark:bg-red-900/20 dark:text-red-400' : log.type === 'warn' ? 'bg-yellow-50 text-yellow-700 dark:bg-yellow-900/20 dark:text-yellow-400' : 'bg-[var(--color-bg-subtle)]'}`}>
                <span className="font-bold">[{log.type}]</span> {log.message}
                {log.stackTrace && <pre className="mt-1 text-[11px] opacity-70 whitespace-pre-wrap">{log.stackTrace}</pre>}
              </div>
            ))}
          </div>
        </Card>
      )}

      {/* 存储 */}
      {activeTool === 'storage' && (
        <Card title="存储">
          <div className="flex items-center gap-2 mb-3">
            <Select
              value={storageType}
              onChange={(e) => { const t = e.target.value as 'localStorage' | 'sessionStorage'; setStorageType(t); if (sessionId) void loadStorage(t) }}
              className="w-48"
              options={[{ value: 'localStorage', label: 'localStorage' }, { value: 'sessionStorage', label: 'sessionStorage' }]}
            />
            <Button size="sm" onClick={() => loadStorage()} loading={storageLoading}>读取</Button>
          </div>
          <div className="space-y-1.5 max-h-[460px] overflow-y-auto">
            {Object.keys(storageData).length === 0 ? (
              <p className="text-sm text-[var(--color-text-muted)] text-center py-8">点击「读取」获取当前页 {storageType} 数据</p>
            ) : Object.entries(storageData).map(([k, v]) => (
              <div key={k} className="p-2 border border-[var(--color-border-default)] rounded text-xs break-all">
                <span className="font-medium text-[var(--color-accent)]">{k}</span>
                <div className="text-[var(--color-text-secondary)] mt-0.5 font-mono">{v}</div>
              </div>
            ))}
          </div>
        </Card>
      )}

      {/* 执行 JS */}
      {activeTool === 'javascript' && (
        <Card title="执行 JavaScript">
          <Textarea value={jsCode} onChange={(e) => setJsCode(e.target.value)} rows={6} placeholder={`在页面上下文中执行，例如：\ndocument.title\nlocation.href`} />
          <div className="flex gap-2 mt-3">
            <Button size="sm" onClick={runJs} loading={jsRunning}>执行</Button>
            <Button size="sm" variant="ghost" onClick={() => { setJsCode(''); setJsResult('') }}>清空</Button>
          </div>
          {jsResult !== '' && (
            <pre className="mt-3 p-3 bg-[var(--color-bg-subtle)] rounded text-xs whitespace-pre-wrap break-all max-h-[360px] overflow-y-auto">{jsResult}</pre>
          )}
        </Card>
      )}

      {/* 截图 */}
      {activeTool === 'screenshot' && (
        <Card title="截图">
          <Button size="sm" onClick={captureShot} loading={shotLoading}>截取当前页面</Button>
          {screenshot && (
            <div className="mt-3 border border-[var(--color-border-default)] rounded overflow-hidden">
              <img src={`data:image/png;base64,${screenshot}`} alt="screenshot" className="w-full" />
            </div>
          )}
        </Card>
      )}

      {/* 性能 / 统计 */}
      {activeTool === 'performance' && (
        <Card title="性能与统计">
          {Object.keys(stats).length === 0 ? (
            <p className="text-sm text-[var(--color-text-muted)] text-center py-8">{capturing ? '统计采集中…' : '未连接'}</p>
          ) : (
            <div className="grid grid-cols-2 sm:grid-cols-3 gap-3">
              {Object.entries(stats).map(([k, v]) => (
                <div key={k} className="p-3 rounded-lg bg-[var(--color-bg-subtle)]">
                  <div className="text-xs text-[var(--color-text-muted)]">{STAT_LABELS[k] || k}</div>
                  <div className="text-lg font-semibold text-[var(--color-text-primary)]">{String(v)}</div>
                </div>
              ))}
            </div>
          )}
        </Card>
      )}
    </div>
  )
}
