import { useEffect, useState } from 'react'
import { Play, Square, Download, Trash2, Search, Filter, RefreshCw, Copy, Terminal, Code, Image as ImageIcon, Database, Cpu, Eye } from 'lucide-react'
import { Badge, Button, Card, Input, Modal, Select, toast, Textarea } from '../../../shared/components'
import type { BrowserProfile } from '../types'
import { fetchBrowserProfiles, resolveDevtoolsWsUrl } from '../api'
import type { ToolType, NetworkRequest, ConsoleLog, StorageItem, Statistics } from './devtools/types'
import { useCdpSession } from '../hooks/useCdpSession'

export function BrowserDevToolsPage() {
  const { capturing, reconnecting, start, stop, getSocket } = useCdpSession()
  const [profiles, setProfiles] = useState<BrowserProfile[]>([])
  const [selectedProfileId, setSelectedProfileId] = useState('')
  const [activeTool, setActiveTool] = useState<ToolType>('network')

  // Network相关
  const [requests, setRequests] = useState<NetworkRequest[]>([])
  const [filteredRequests, setFilteredRequests] = useState<NetworkRequest[]>([])
  const [searchQuery, setSearchQuery] = useState('')
  const [filterMethod, setFilterMethod] = useState('all')
  const [filterType, setFilterType] = useState('all')
  const [filterStatus, setFilterStatus] = useState('all')
  const [sortBy, setSortBy] = useState<'time' | 'size' | 'status'>('time')
  const [sortOrder, setSortOrder] = useState<'asc' | 'desc'>('desc')
  const [bodyRequestMap] = useState(new Map<number, string>()) // CDP ID -> Request ID 映射

  // 请求数量限制
  const MAX_REQUESTS = 500
  const [selectedRequest, setSelectedRequest] = useState<NetworkRequest | null>(null)
  const [detailModalOpen, setDetailModalOpen] = useState(false)
  const [showStats] = useState(true)
  const [activeTab, setActiveTab] = useState<'headers' | 'response' | 'cookies' | 'timing'>('headers')

  // Console相关
  const [consoleLogs, setConsoleLogs] = useState<ConsoleLog[]>([])
  const [consoleFilter, setConsoleFilter] = useState<'all' | 'log' | 'warn' | 'error' | 'info'>('all')
  const [consoleSearch, setConsoleSearch] = useState('')

  // Storage相关
  const [storageItems, setStorageItems] = useState<StorageItem[]>([])
  const [storageType, setStorageType] = useState<'localStorage' | 'sessionStorage'>('localStorage')

  // JavaScript执行相关
  const [jsCode, setJsCode] = useState('')
  const [jsResult, setJsResult] = useState('')

  // Performance相关
  const [perfMetrics, setPerfMetrics] = useState({
    jsHeapSize: 0,
    jsHeapSizeLimit: 0,
    domNodes: 0,
    frames: 0,
  })

  useEffect(() => {
    loadProfiles()
    // WebSocket 生命周期由 useCdpSession 在卸载时清理
  }, [])

  useEffect(() => {
    // 过滤网络请求
    let filtered = requests

    if (searchQuery) {
      filtered = filtered.filter(req =>
        req.url.toLowerCase().includes(searchQuery.toLowerCase())
      )
    }

    if (filterMethod !== 'all') {
      filtered = filtered.filter(req => req.method === filterMethod)
    }

    if (filterType !== 'all') {
      filtered = filtered.filter(req => req.type.toLowerCase() === filterType.toLowerCase())
    }

    if (filterStatus !== 'all') {
      if (filterStatus === 'success') {
        filtered = filtered.filter(req => req.statusCode && req.statusCode >= 200 && req.statusCode < 300)
      } else if (filterStatus === 'failed') {
        filtered = filtered.filter(req => req.statusCode && req.statusCode >= 400)
      } else if (filterStatus === 'pending') {
        filtered = filtered.filter(req => !req.statusCode)
      } else if (filterStatus === 'api') {
        filtered = filtered.filter(req => req.isApi)
      }
    }

    // 排序
    filtered.sort((a, b) => {
      let comparison = 0

      if (sortBy === 'time') {
        comparison = a.timestamp - b.timestamp
      } else if (sortBy === 'size') {
        comparison = (a.size || 0) - (b.size || 0)
      } else if (sortBy === 'status') {
        comparison = (a.statusCode || 0) - (b.statusCode || 0)
      }

      return sortOrder === 'asc' ? comparison : -comparison
    })

    setFilteredRequests(filtered)
  }, [requests, searchQuery, filterMethod, filterType, filterStatus, sortBy, sortOrder])

  // 计算统计信息
  const statistics: Statistics = {
    total: requests.length,
    success: requests.filter(r => r.statusCode && r.statusCode >= 200 && r.statusCode < 300).length,
    failed: requests.filter(r => r.statusCode && r.statusCode >= 400).length,
    pending: requests.filter(r => !r.statusCode).length,
    totalSize: requests.reduce((sum, r) => sum + (r.size || 0), 0),
    avgDuration: requests.length > 0
      ? requests.reduce((sum, r) => sum + (r.duration || 0), 0) / requests.length
      : 0,
  }

  const loadProfiles = async () => {
    try {
      const list = await fetchBrowserProfiles()
      setProfiles(list.filter(p => p.running && p.debugReady))
    } catch (error: any) {
      toast.error('加载浏览器实例失败')
    }
  }

  const startCapture = async () => {
    if (!selectedProfileId) {
      toast.error('请选择一个运行中的浏览器实例')
      return
    }

    const profile = profiles.find(p => p.profileId === selectedProfileId)
    if (!profile || !profile.debugPort) {
      toast.error('浏览器实例未就绪或无调试端口')
      return
    }

    // 解析真实的页面级调试地址（不能硬编码 /devtools/browser：新版 Chromium 拒绝，
    // 且浏览器级 target 不支持 Network/Console/Performance）。
    const wsUrl = await resolveDevtoolsWsUrl(selectedProfileId)
    if (!wsUrl) {
      toast.error('无法获取调试地址：请确认实例已启动并至少打开一个标签页')
      return
    }
    start({
      wsUrl,
      onOpen: (sendMsg, isReconnect) => {
        // 启用 Network / Console(Runtime) / Performance 域
        sendMsg({ id: 1, method: 'Network.enable' })
        sendMsg({ id: 2, method: 'Runtime.enable' })
        sendMsg({ id: 3, method: 'Performance.enable' })
        if (!isReconnect) {
          setRequests([])
          setConsoleLogs([])
        }
      },
      onMessage: (data, sendMsg) => {
        handleNetworkEvent(data, sendMsg)
        handleConsoleEvent(data)
        handlePerformanceEvent(data)
      },
    })
  }

  const handleNetworkEvent = (data: any, sendMsg: (payload: any) => void) => {
    if (data.method === 'Network.requestWillBeSent') {
      const params = data.params
      const isApi = params.request.url.includes('/api/') ||
                   params.type === 'XHR' ||
                   params.type === 'Fetch'

      const newRequest: NetworkRequest = {
        requestId: params.requestId,
        url: params.request.url,
        method: params.request.method,
        type: params.type || 'Other',
        timestamp: params.timestamp,
        requestHeaders: params.request.headers,
        requestBody: params.request.postData,
        isApi,
      }

      setRequests(prev => {
        const updated = [...prev, newRequest]
        // 限制请求数量，超过MAX_REQUESTS时删除最旧的
        if (updated.length > MAX_REQUESTS) {
          const toRemove = updated.length - MAX_REQUESTS
          return updated.slice(toRemove)
        }
        return updated
      })
    }

    if (data.method === 'Network.responseReceived') {
      const params = data.params
      const mimeType = params.response.mimeType || ''
      const isApi = mimeType.includes('json') || mimeType.includes('xml')

      setRequests(prev => prev.map(req =>
        req.requestId === params.requestId
          ? {
              ...req,
              statusCode: params.response.status,
              statusText: params.response.statusText,
              responseHeaders: params.response.headers,
              mimeType,
              isApi: req.isApi || isApi,
              responseBodyLoading: true,
            }
          : req
      ))

      // 请求响应体，使用精确的ID映射
      const cdpId = Date.now() + Math.random() // 确保唯一性
      bodyRequestMap.set(cdpId, params.requestId)

      sendMsg({
        id: cdpId,
        method: 'Network.getResponseBody',
        params: { requestId: params.requestId },
      })

      // 3秒后如果还没收到，标记为失败并清理映射
      setTimeout(() => {
        if (bodyRequestMap.has(cdpId)) {
          bodyRequestMap.delete(cdpId)
          setRequests(prev => prev.map(req =>
            req.requestId === params.requestId && req.responseBodyLoading
              ? { ...req, responseBodyLoading: false, responseBodyError: true }
              : req
          ))
        }
      }, 3000)
    }

    if (data.method === 'Network.loadingFinished') {
      const params = data.params
      setRequests(prev => prev.map(req =>
        req.requestId === params.requestId
          ? {
              ...req,
              duration: params.encodedDataLength ? Math.round(params.encodedDataLength / 1000) : undefined,
              size: params.encodedDataLength,
            }
          : req
      ))
    }

    // 响应体返回 - 使用精确的ID映射
    if (data.result && data.result.body !== undefined) {
      const cdpId = data.id
      const requestId = bodyRequestMap.get(cdpId)

      if (requestId) {
        setRequests(prev => prev.map(req =>
          req.requestId === requestId
            ? {
                ...req,
                responseBody: data.result.body,
                responseBodyLoading: false,
                responseBodyError: false,
              }
            : req
        ))
        bodyRequestMap.delete(cdpId)
      }
    }

    // 响应体获取失败 - 使用精确的ID映射
    if (data.error && data.error.message) {
      const cdpId = data.id
      const requestId = bodyRequestMap.get(cdpId)

      if (requestId) {
        setRequests(prev => prev.map(req =>
          req.requestId === requestId
            ? { ...req, responseBodyLoading: false, responseBodyError: true }
            : req
        ))
        bodyRequestMap.delete(cdpId)
      }
    }
  }

  const handleConsoleEvent = (data: any) => {
    if (data.method === 'Runtime.consoleAPICalled') {
      const params = data.params
      const log: ConsoleLog = {
        id: `log_${Date.now()}_${Math.random()}`,
        type: params.type,
        message: params.args.map((arg: any) => {
          if (arg.value !== undefined) return String(arg.value)
          if (arg.description) return arg.description
          return JSON.stringify(arg)
        }).join(' '),
        timestamp: params.timestamp,
      }
      setConsoleLogs(prev => [...prev, log])
    }

    if (data.method === 'Runtime.exceptionThrown') {
      const exception = data.params.exceptionDetails
      const log: ConsoleLog = {
        id: `error_${Date.now()}_${Math.random()}`,
        type: 'error',
        message: exception.exception?.description || exception.text,
        timestamp: exception.timestamp,
        stackTrace: exception.stackTrace?.callFrames.map((f: any) =>
          `    at ${f.functionName || 'anonymous'} (${f.url}:${f.lineNumber}:${f.columnNumber})`
        ).join('\n'),
      }
      setConsoleLogs(prev => [...prev, log])
    }
  }

  const handlePerformanceEvent = (data: any) => {
    // 性能指标处理
    if (data.result && data.result.metrics) {
      const metrics = data.result.metrics
      setPerfMetrics({
        jsHeapSize: metrics.JSHeapUsedSize || 0,
        jsHeapSizeLimit: metrics.JSHeapTotalSize || 0,
        domNodes: metrics.Nodes || 0,
        frames: metrics.Frames || 0,
      })
    }
  }

  const stopCapture = () => {
    stop()
    bodyRequestMap.clear() // 清理映射表
  }

  const clearData = () => {
    const confirmMessage = activeTool === 'network'
      ? `确定要清空所有网络请求吗？共 ${requests.length} 个请求将被删除。`
      : activeTool === 'console'
      ? `确定要清空所有控制台日志吗？共 ${consoleLogs.length} 条日志将被删除。`
      : '确定要清空数据吗？'

    if (!window.confirm(confirmMessage)) {
      return
    }

    if (activeTool === 'network') {
      setRequests([])
      bodyRequestMap.clear()
      toast.success('已清空网络请求')
    } else if (activeTool === 'console') {
      setConsoleLogs([])
      toast.success('已清空控制台日志')
    }
  }

  const loadStorage = async () => {
    const sock = getSocket()
    if (!sock) {
      toast.error('请先连接浏览器实例')
      return
    }

    // 执行JS获取Storage
    const code = `
      JSON.stringify({
        localStorage: Object.keys(localStorage).map(key => ({
          key,
          value: localStorage.getItem(key)
        })),
        sessionStorage: Object.keys(sessionStorage).map(key => ({
          key,
          value: sessionStorage.getItem(key)
        }))
      })
    `

    sock.send(JSON.stringify({
      id: Date.now(),
      method: 'Runtime.evaluate',
      params: {
        expression: code,
        returnByValue: true,
      },
    }))

    const handleStorageResponse = (event: MessageEvent) => {
      try {
        const data = JSON.parse(event.data)
        if (data.result && data.result.result && data.result.result.value) {
          const storage = JSON.parse(data.result.result.value)
          const items: StorageItem[] = [
            ...storage.localStorage.map((item: any) => ({ ...item, type: 'localStorage' as const })),
            ...storage.sessionStorage.map((item: any) => ({ ...item, type: 'sessionStorage' as const })),
          ]
          setStorageItems(items)
          toast.success('存储数据加载成功')
          sock.removeEventListener('message', handleStorageResponse)
        }
      } catch (error) {
        console.error('解析存储数据失败:', error)
      }
    }

    sock.addEventListener('message', handleStorageResponse)
  }

  const executeJavaScript = async () => {
    const sock = getSocket()
    if (!sock) {
      toast.error('请先连接浏览器实例')
      return
    }

    if (!jsCode.trim()) {
      toast.error('请输入JavaScript代码')
      return
    }

    sock.send(JSON.stringify({
      id: Date.now(),
      method: 'Runtime.evaluate',
      params: {
        expression: jsCode,
        returnByValue: true,
      },
    }))

    const handleJsResponse = (event: MessageEvent) => {
      try {
        const data = JSON.parse(event.data)
        if (data.result) {
          if (data.result.result) {
            const result = data.result.result.value !== undefined
              ? JSON.stringify(data.result.result.value, null, 2)
              : data.result.result.description || '执行成功'
            setJsResult(result)
            toast.success('代码执行成功')
          } else if (data.result.exceptionDetails) {
            setJsResult(`错误: ${data.result.exceptionDetails.exception.description}`)
            toast.error('代码执行出错')
          }
          sock.removeEventListener('message', handleJsResponse)
        }
      } catch (error) {
        console.error('解析执行结果失败:', error)
      }
    }

    sock.addEventListener('message', handleJsResponse)
  }

  const takeScreenshot = async () => {
    const sock = getSocket()
    if (!sock) {
      toast.error('请先连接浏览器实例')
      return
    }

    sock.send(JSON.stringify({
      id: Date.now(),
      method: 'Page.captureScreenshot',
      params: {
        format: 'png',
      },
    }))

    const handleScreenshotResponse = (event: MessageEvent) => {
      try {
        const data = JSON.parse(event.data)
        if (data.result && data.result.data) {
          const link = document.createElement('a')
          link.href = `data:image/png;base64,${data.result.data}`
          link.download = `screenshot-${Date.now()}.png`
          link.click()
          toast.success('截图已保存')
          sock.removeEventListener('message', handleScreenshotResponse)
        }
      } catch (error) {
        console.error('截图失败:', error)
      }
    }

    sock.addEventListener('message', handleScreenshotResponse)
  }

  // 导出和工具方法
  const exportHAR = () => {
    const har = {
      log: {
        version: '1.2',
        creator: { name: 'Ant Browser DevTools', version: '1.0' },
        entries: filteredRequests.map(req => ({
          startedDateTime: new Date(req.timestamp * 1000).toISOString(),
          time: req.duration || 0,
          request: {
            method: req.method,
            url: req.url,
            httpVersion: 'HTTP/1.1',
            headers: Object.entries(req.requestHeaders || {}).map(([name, value]) => ({ name, value })),
          },
          response: {
            status: req.statusCode || 0,
            statusText: req.statusText || '',
            httpVersion: 'HTTP/1.1',
            headers: Object.entries(req.responseHeaders || {}).map(([name, value]) => ({ name, value })),
          },
        })),
      },
    }

    const blob = new Blob([JSON.stringify(har, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `network-${Date.now()}.har`
    a.click()
    URL.revokeObjectURL(url)
    toast.success('HAR文件已导出')
  }

  const copyAsJSON = () => {
    const json = JSON.stringify(filteredRequests, null, 2)
    navigator.clipboard.writeText(json)
    toast.success('已复制为JSON')
  }

  const copyCookies = () => {
    const cookies: string[] = []
    filteredRequests.forEach(req => {
      const cookieHeader = req.requestHeaders?.['Cookie'] || req.requestHeaders?.['cookie']
      if (cookieHeader && !cookies.includes(cookieHeader)) {
        cookies.push(cookieHeader)
      }
    })
    if (cookies.length > 0) {
      navigator.clipboard.writeText(cookies.join('\n'))
      toast.success(`已复制 ${cookies.length} 个Cookie`)
    } else {
      toast.error('未找到Cookie')
    }
  }

  const extractTokens = () => {
    const tokens: Record<string, string> = {}

    filteredRequests.forEach(req => {
      const auth = req.requestHeaders?.['Authorization'] || req.requestHeaders?.['authorization']
      if (auth) tokens['Authorization'] = auth

      if (auth && auth.startsWith('Bearer ')) {
        tokens['Bearer Token'] = auth.substring(7)
      }

      const apiKey = req.requestHeaders?.['X-API-Key'] || req.requestHeaders?.['x-api-key']
      if (apiKey) tokens['API Key'] = apiKey

      const cookie = req.requestHeaders?.['Cookie'] || req.requestHeaders?.['cookie']
      if (cookie) {
        const sessionMatch = cookie.match(/(?:session|sessionid|SESSIONID)=([^;]+)/)
        if (sessionMatch) tokens['Session ID'] = sessionMatch[1]
      }
    })

    if (Object.keys(tokens).length > 0) {
      const text = Object.entries(tokens).map(([k, v]) => `${k}: ${v}`).join('\n')
      navigator.clipboard.writeText(text)
      toast.success(`已提取 ${Object.keys(tokens).length} 个Token`)
    } else {
      toast.error('未找到认证Token')
    }
  }

  const copyAsCurl = (request: NetworkRequest) => {
    let curl = `curl '${request.url}'`

    if (request.method !== 'GET') {
      curl += ` -X ${request.method}`
    }

    if (request.requestHeaders) {
      Object.entries(request.requestHeaders).forEach(([key, value]) => {
        curl += ` \\\n  -H '${key}: ${value}'`
      })
    }

    if (request.requestBody) {
      curl += ` \\\n  --data '${request.requestBody}'`
    }

    navigator.clipboard.writeText(curl)
    toast.success('已复制为cURL命令')
  }

  const handleViewDetails = (request: NetworkRequest) => {
    setSelectedRequest(request)
    setDetailModalOpen(true)
  }

  const formatBytes = (bytes: number) => {
    if (bytes === 0) return '0 B'
    const k = 1024
    const sizes = ['B', 'KB', 'MB', 'GB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return Math.round(bytes / Math.pow(k, i) * 100) / 100 + ' ' + sizes[i]
  }

  const getStatusColor = (statusCode?: number) => {
    if (!statusCode) return 'default'
    if (statusCode >= 200 && statusCode < 300) return 'success'
    return 'default'
  }

  const getConsoleIcon = (type: ConsoleLog['type']) => {
    switch (type) {
      case 'error': return '❌'
      case 'warn': return '⚠️'
      case 'info': return 'ℹ️'
      default: return '📝'
    }
  }

  const runningProfiles = profiles.filter(p => p.running && p.debugReady)

  const filteredConsoleLogs = consoleLogs
    .filter(log => consoleFilter === 'all' || log.type === consoleFilter)
    .filter(log => !consoleSearch || log.message.toLowerCase().includes(consoleSearch.toLowerCase()))

  const filteredStorageItems = storageItems.filter(item => item.type === storageType)

  return (
    <div className="overflow-auto p-5 space-y-5 animate-fade-in h-full">
      {/* 页头 */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">浏览器开发工具</h1>
          <p className="text-sm text-[var(--color-text-muted)] mt-1">
            基于Chrome DevTools Protocol的完整调试工具集
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="secondary" size="sm" onClick={loadProfiles}>
            <RefreshCw className="w-4 h-4" />刷新实例
          </Button>
        </div>
      </div>

      {/* 控制栏 */}
      <Card padding="md">
        <div className="flex flex-col md:flex-row gap-4">
          <div className="flex-1">
            <Select
              value={selectedProfileId}
              onChange={e => setSelectedProfileId(e.target.value)}
              options={[
                { value: '', label: '选择浏览器实例' },
                ...runningProfiles.map(p => ({
                  value: p.profileId,
                  label: `${p.profileName} (运行中)`,
                })),
              ]}
              disabled={capturing}
            />
          </div>
          <div className="flex gap-2">
            {!capturing && !reconnecting ? (
              <Button onClick={() => startCapture()} disabled={!selectedProfileId}>
                <Play className="w-4 h-4" />连接
              </Button>
            ) : reconnecting ? (
              <Button variant="secondary" disabled>
                <RefreshCw className="w-4 h-4 animate-spin" />重连中...
              </Button>
            ) : (
              <Button variant="secondary" onClick={stopCapture}>
                <Square className="w-4 h-4" />断开
              </Button>
            )}
            <Button variant="secondary" onClick={clearData} disabled={!capturing && !reconnecting}>
              <Trash2 className="w-4 h-4" />清空
            </Button>
          </div>
        </div>
      </Card>

      {/* 工具选择标签页 */}
      <Card padding="none">
        <div className="flex border-b border-[var(--color-border-default)]">
          <button
            className={`flex-1 flex items-center justify-center gap-2 px-4 py-3 text-sm font-medium transition-colors ${
              activeTool === 'network'
                ? 'text-[var(--color-accent)] bg-[var(--color-bg-muted)] border-b-2 border-[var(--color-accent)]'
                : 'text-[var(--color-text-muted)] hover:text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-muted)]'
            }`}
            onClick={() => setActiveTool('network')}
          >
            <Filter className="w-4 h-4" />网络抓包
          </button>
          <button
            className={`flex-1 flex items-center justify-center gap-2 px-4 py-3 text-sm font-medium transition-colors ${
              activeTool === 'console'
                ? 'text-[var(--color-accent)] bg-[var(--color-bg-muted)] border-b-2 border-[var(--color-accent)]'
                : 'text-[var(--color-text-muted)] hover:text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-muted)]'
            }`}
            onClick={() => setActiveTool('console')}
          >
            <Terminal className="w-4 h-4" />控制台
          </button>
          <button
            className={`flex-1 flex items-center justify-center gap-2 px-4 py-3 text-sm font-medium transition-colors ${
              activeTool === 'storage'
                ? 'text-[var(--color-accent)] bg-[var(--color-bg-muted)] border-b-2 border-[var(--color-accent)]'
                : 'text-[var(--color-text-muted)] hover:text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-muted)]'
            }`}
            onClick={() => setActiveTool('storage')}
          >
            <Database className="w-4 h-4" />存储
          </button>
          <button
            className={`flex-1 flex items-center justify-center gap-2 px-4 py-3 text-sm font-medium transition-colors ${
              activeTool === 'javascript'
                ? 'text-[var(--color-accent)] bg-[var(--color-bg-muted)] border-b-2 border-[var(--color-accent)]'
                : 'text-[var(--color-text-muted)] hover:text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-muted)]'
            }`}
            onClick={() => setActiveTool('javascript')}
          >
            <Code className="w-4 h-4" />执行JS
          </button>
          <button
            className={`flex-1 flex items-center justify-center gap-2 px-4 py-3 text-sm font-medium transition-colors ${
              activeTool === 'screenshot'
                ? 'text-[var(--color-accent)] bg-[var(--color-bg-muted)] border-b-2 border-[var(--color-accent)]'
                : 'text-[var(--color-text-muted)] hover:text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-muted)]'
            }`}
            onClick={() => setActiveTool('screenshot')}
          >
            <ImageIcon className="w-4 h-4" />截图
          </button>
          <button
            className={`flex-1 flex items-center justify-center gap-2 px-4 py-3 text-sm font-medium transition-colors ${
              activeTool === 'performance'
                ? 'text-[var(--color-accent)] bg-[var(--color-bg-muted)] border-b-2 border-[var(--color-accent)]'
                : 'text-[var(--color-text-muted)] hover:text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-muted)]'
            }`}
            onClick={() => setActiveTool('performance')}
          >
            <Cpu className="w-4 h-4" />性能
          </button>
        </div>
      </Card>

      {/* 工具内容区域 */}
      {!capturing ? (
        <Card padding="lg" className="text-center">
          <div className="w-16 h-16 mx-auto mb-4 rounded-2xl bg-gradient-to-br from-blue-50 to-blue-100 dark:from-blue-900/30 dark:to-blue-800/20 flex items-center justify-center">
            <Play className="w-8 h-8 text-blue-600" />
          </div>
          <h3 className="text-lg font-semibold text-[var(--color-text-primary)] mb-2">
            未连接
          </h3>
          <p className="text-sm text-[var(--color-text-muted)]">
            选择一个运行中的浏览器实例并点击"连接"开始使用开发工具
          </p>
        </Card>
      ) : (
        <>
          {/* 网络抓包工具 */}
          {activeTool === 'network' && (
            <div className="space-y-5">
              {/* 统计面板 */}
              {showStats && (
                <div className="grid grid-cols-2 md:grid-cols-6 gap-4">
                  <Card padding="md">
                    <div className="text-xs text-[var(--color-text-muted)] mb-1">总请求</div>
                    <div className="text-2xl font-semibold text-[var(--color-text-primary)]">{statistics.total}</div>
                  </Card>
                  <Card padding="md">
                    <div className="text-xs text-[var(--color-text-muted)] mb-1">成功</div>
                    <div className="text-2xl font-semibold text-green-600">{statistics.success}</div>
                  </Card>
                  <Card padding="md">
                    <div className="text-xs text-[var(--color-text-muted)] mb-1">失败</div>
                    <div className="text-2xl font-semibold text-red-600">{statistics.failed}</div>
                  </Card>
                  <Card padding="md">
                    <div className="text-xs text-[var(--color-text-muted)] mb-1">等待中</div>
                    <div className="text-2xl font-semibold text-[var(--color-text-secondary)]">{statistics.pending}</div>
                  </Card>
                  <Card padding="md">
                    <div className="text-xs text-[var(--color-text-muted)] mb-1">总大小</div>
                    <div className="text-2xl font-semibold text-[var(--color-text-primary)]">{formatBytes(statistics.totalSize)}</div>
                  </Card>
                  <Card padding="md">
                    <div className="text-xs text-[var(--color-text-muted)] mb-1">平均耗时</div>
                    <div className="text-2xl font-semibold text-[var(--color-text-primary)]">{Math.round(statistics.avgDuration)}ms</div>
                  </Card>
                </div>
              )}

              {/* 过滤栏 */}
              <Card padding="md">
                <div className="space-y-3">
                  <div className="flex flex-col md:flex-row gap-4">
                    <div className="flex-1 relative">
                      <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 w-4 h-4 text-[var(--color-text-muted)]" />
                      <Input
                        value={searchQuery}
                        onChange={e => setSearchQuery(e.target.value)}
                        placeholder="搜索URL..."
                        className="pl-10"
                      />
                    </div>
                    <Select
                      value={filterMethod}
                      onChange={e => setFilterMethod(e.target.value)}
                      options={[
                        { value: 'all', label: '所有方法' },
                        { value: 'GET', label: 'GET' },
                        { value: 'POST', label: 'POST' },
                        { value: 'PUT', label: 'PUT' },
                        { value: 'DELETE', label: 'DELETE' },
                      ]}
                    />
                    <Select
                      value={filterType}
                      onChange={e => setFilterType(e.target.value)}
                      options={[
                        { value: 'all', label: '所有类型' },
                        { value: 'document', label: 'Document' },
                        { value: 'xhr', label: 'XHR' },
                        { value: 'fetch', label: 'Fetch' },
                        { value: 'script', label: 'Script' },
                        { value: 'stylesheet', label: 'Stylesheet' },
                        { value: 'image', label: 'Image' },
                      ]}
                    />
                    <Select
                      value={filterStatus}
                      onChange={e => setFilterStatus(e.target.value)}
                      options={[
                        { value: 'all', label: '所有状态' },
                        { value: 'success', label: '成功 (2xx)' },
                        { value: 'failed', label: '失败 (4xx/5xx)' },
                        { value: 'pending', label: '等待中' },
                        { value: 'api', label: '仅API' },
                      ]}
                    />
                  </div>
                  <div className="flex flex-wrap gap-2 items-center">
                    <span className="text-sm text-[var(--color-text-muted)]">排序:</span>
                    <Select
                      value={sortBy}
                      onChange={e => setSortBy(e.target.value as any)}
                      options={[
                        { value: 'time', label: '时间' },
                        { value: 'size', label: '大小' },
                        { value: 'status', label: '状态' },
                      ]}
                    />
                    <Button
                      variant="secondary"
                      size="sm"
                      onClick={() => setSortOrder(sortOrder === 'asc' ? 'desc' : 'asc')}
                    >
                      {sortOrder === 'asc' ? '↑ 升序' : '↓ 降序'}
                    </Button>
                    <div className="flex-1"></div>
                    <Button variant="secondary" size="sm" onClick={exportHAR} disabled={filteredRequests.length === 0}>
                      <Download className="w-4 h-4" />导出HAR
                    </Button>
                    <Button variant="secondary" size="sm" onClick={copyAsJSON} disabled={filteredRequests.length === 0}>
                      <Copy className="w-4 h-4" />复制JSON
                    </Button>
                    <Button variant="secondary" size="sm" onClick={copyCookies} disabled={filteredRequests.length === 0}>
                      <Copy className="w-4 h-4" />提取Cookie
                    </Button>
                    <Button variant="secondary" size="sm" onClick={extractTokens} disabled={filteredRequests.length === 0}>
                      <Copy className="w-4 h-4" />提取Token
                    </Button>
                  </div>
                </div>
              </Card>

              {/* 请求列表 - 简化版，详细内容在弹窗 */}
              <Card padding="none">
                <div className="max-h-96 overflow-auto">
                  {filteredRequests.length === 0 ? (
                    <div className="py-10 text-center text-sm text-[var(--color-text-muted)]">
                      {capturing ? '等待网络请求...' : '暂无请求'}
                    </div>
                  ) : (
                    <div className="divide-y divide-[var(--color-border-default)]">
                      {filteredRequests.slice(0, 50).map(request => {
                        // 判断请求状态用于高亮
                        const isFailed = request.statusCode && request.statusCode >= 400

                        return (
                        <div
                          key={request.requestId}
                          className={`flex items-center gap-3 p-3 transition-colors cursor-pointer text-sm ${
                            isFailed
                              ? 'bg-red-50 dark:bg-red-900/20 hover:bg-red-100 dark:hover:bg-red-900/30'
                              : request.isApi
                              ? 'bg-green-50 dark:bg-green-900/20 hover:bg-green-100 dark:hover:bg-green-900/30'
                              : 'hover:bg-[var(--color-bg-muted)]'
                          }`}
                          onClick={() => handleViewDetails(request)}
                        >
                          <Badge variant="default">{request.method}</Badge>
                          <div className="flex-1 min-w-0 flex items-center gap-2">
                            <div className="truncate" title={request.url}>
                              {request.url}
                            </div>
                            {request.isApi && (
                              <Badge variant="success" className="flex-shrink-0">API</Badge>
                            )}
                          </div>
                          {request.statusCode && (
                            <Badge variant={getStatusColor(request.statusCode)}>
                              {request.statusCode}
                            </Badge>
                          )}
                          <Button size="sm" variant="ghost" onClick={(e) => {
                            e.stopPropagation()
                            handleViewDetails(request)
                          }}>
                            <Eye className="w-3.5 h-3.5" />
                          </Button>
                        </div>
                        )
                      })}
                    </div>
                  )}
                </div>
              </Card>

              <div className="text-sm text-[var(--color-text-muted)]">
                共 {filteredRequests.length} 个请求（显示前50个）
                {requests.length >= MAX_REQUESTS && (
                  <span className="ml-2 text-yellow-600">
                    • 已达到{MAX_REQUESTS}个请求上限，最旧的请求已自动清理
                  </span>
                )}
              </div>
            </div>
          )}

          {/* Console控制台 */}
          {activeTool === 'console' && (
            <div className="space-y-4">
              <div className="flex gap-2">
                <div className="flex-1 relative">
                  <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 w-4 h-4 text-[var(--color-text-muted)]" />
                  <Input
                    value={consoleSearch}
                    onChange={e => setConsoleSearch(e.target.value)}
                    placeholder="搜索日志内容..."
                    className="pl-10"
                  />
                </div>
                <Select
                  value={consoleFilter}
                  onChange={e => setConsoleFilter(e.target.value as any)}
                  options={[
                    { value: 'all', label: '全部' },
                    { value: 'log', label: 'Log' },
                    { value: 'warn', label: 'Warn' },
                    { value: 'error', label: 'Error' },
                    { value: 'info', label: 'Info' },
                  ]}
                />
              </div>

              <Card padding="none">
                <div className="max-h-96 overflow-auto bg-black text-green-400 font-mono text-sm">
                  {filteredConsoleLogs.length === 0 ? (
                    <div className="p-4 text-center text-gray-500">
                      暂无控制台输出
                    </div>
                  ) : (
                    <div className="p-4 space-y-2">
                      {filteredConsoleLogs.map(log => (
                        <div key={log.id} className="border-b border-gray-800 pb-2">
                          <div className="flex items-start gap-2">
                            <span>{getConsoleIcon(log.type)}</span>
                            <div className="flex-1">
                              <div className={log.type === 'error' ? 'text-red-400' : log.type === 'warn' ? 'text-yellow-400' : ''}>{log.message}</div>
                              {log.stackTrace && (
                                <pre className="text-xs text-gray-500 mt-1">{log.stackTrace}</pre>
                              )}
                            </div>
                            <span className="text-xs text-gray-600">
                              {new Date(log.timestamp).toLocaleTimeString()}
                            </span>
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              </Card>

              <div className="text-sm text-[var(--color-text-muted)]">
                共 {filteredConsoleLogs.length} 条日志
              </div>
            </div>
          )}

          {/* Storage存储 */}
          {activeTool === 'storage' && (
            <div className="space-y-4">
              <div className="flex gap-2">
                <Select
                  value={storageType}
                  onChange={e => setStorageType(e.target.value as any)}
                  options={[
                    { value: 'localStorage', label: 'LocalStorage' },
                    { value: 'sessionStorage', label: 'SessionStorage' },
                  ]}
                />
                <Button onClick={loadStorage}>
                  <RefreshCw className="w-4 h-4" />加载存储
                </Button>
              </div>

              <Card padding="md">
                {filteredStorageItems.length === 0 ? (
                  <div className="py-10 text-center text-sm text-[var(--color-text-muted)]">
                    暂无存储数据，点击"加载存储"
                  </div>
                ) : (
                  <div className="space-y-2 max-h-96 overflow-auto">
                    {filteredStorageItems.map((item, idx) => (
                      <div key={idx} className="p-3 bg-[var(--color-bg-muted)] rounded-lg">
                        <div className="font-semibold text-sm text-[var(--color-text-primary)] mb-1">{item.key}</div>
                        <div className="text-xs text-[var(--color-text-secondary)] break-all">{item.value}</div>
                      </div>
                    ))}
                  </div>
                )}
              </Card>

              <div className="text-sm text-[var(--color-text-muted)]">
                共 {filteredStorageItems.length} 个存储项
              </div>
            </div>
          )}

          {/* JavaScript执行器 */}
          {activeTool === 'javascript' && (
            <div className="space-y-4">
              <Card padding="md">
                <div className="space-y-3">
                  <div>
                    <label className="text-sm font-medium text-[var(--color-text-primary)] mb-2 block">
                      JavaScript代码
                    </label>
                    <Textarea
                      rows={8}
                      value={jsCode}
                      onChange={e => setJsCode(e.target.value)}
                      placeholder="输入要执行的JavaScript代码...&#10;例如: document.title"
                      className="font-mono text-sm"
                    />
                  </div>
                  <Button onClick={executeJavaScript}>
                    <Play className="w-4 h-4" />执行代码
                  </Button>
                </div>
              </Card>

              {jsResult && (
                <Card padding="md">
                  <label className="text-sm font-medium text-[var(--color-text-primary)] mb-2 block">
                    执行结果
                  </label>
                  <div className="bg-[var(--color-bg-muted)] p-3 rounded-lg max-h-60 overflow-auto">
                    <pre className="text-xs text-[var(--color-text-secondary)] whitespace-pre-wrap">
                      {jsResult}
                    </pre>
                  </div>
                </Card>
              )}
            </div>
          )}

          {/* 截图工具 */}
          {activeTool === 'screenshot' && (
            <Card padding="lg" className="text-center">
              <div className="w-16 h-16 mx-auto mb-4 rounded-2xl bg-gradient-to-br from-purple-50 to-purple-100 dark:from-purple-900/30 dark:to-purple-800/20 flex items-center justify-center">
                <ImageIcon className="w-8 h-8 text-purple-600" />
              </div>
              <h3 className="text-lg font-semibold text-[var(--color-text-primary)] mb-2">
                页面截图
              </h3>
              <p className="text-sm text-[var(--color-text-muted)] mb-4">
                捕获当前浏览器页面的可视区域截图
              </p>
              <Button onClick={takeScreenshot}>
                <ImageIcon className="w-4 h-4" />立即截图
              </Button>
            </Card>
          )}

          {/* 性能监控 */}
          {activeTool === 'performance' && (
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
              <Card padding="md">
                <div className="text-xs text-[var(--color-text-muted)] mb-1">JS堆内存</div>
                <div className="text-lg font-semibold text-[var(--color-text-primary)]">
                  {formatBytes(perfMetrics.jsHeapSize)}
                </div>
              </Card>
              <Card padding="md">
                <div className="text-xs text-[var(--color-text-muted)] mb-1">JS堆限制</div>
                <div className="text-lg font-semibold text-[var(--color-text-primary)]">
                  {formatBytes(perfMetrics.jsHeapSizeLimit)}
                </div>
              </Card>
              <Card padding="md">
                <div className="text-xs text-[var(--color-text-muted)] mb-1">DOM节点</div>
                <div className="text-lg font-semibold text-[var(--color-text-primary)]">
                  {perfMetrics.domNodes}
                </div>
              </Card>
              <Card padding="md">
                <div className="text-xs text-[var(--color-text-muted)] mb-1">帧数</div>
                <div className="text-lg font-semibold text-[var(--color-text-primary)]">
                  {perfMetrics.frames}
                </div>
              </Card>
            </div>
          )}
        </>
      )}

      {/* 请求详情弹窗 */}
      <Modal
        open={detailModalOpen}
        onClose={() => setDetailModalOpen(false)}
        title="请求详情"
        width="900px"
      >
        {selectedRequest && (
          <div className="py-4">
            {/* 基本信息栏 */}
            <div className="mb-4 p-3 bg-[var(--color-bg-muted)] rounded-lg">
              <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
                <div>
                  <span className="text-[var(--color-text-muted)]">方法: </span>
                  <Badge variant="default">{selectedRequest.method}</Badge>
                </div>
                <div>
                  <span className="text-[var(--color-text-muted)]">状态: </span>
                  {selectedRequest.statusCode ? (
                    <Badge variant={getStatusColor(selectedRequest.statusCode)}>
                      {selectedRequest.statusCode}
                    </Badge>
                  ) : (
                    <span className="text-[var(--color-text-muted)]">Pending</span>
                  )}
                </div>
                <div>
                  <span className="text-[var(--color-text-muted)]">类型: </span>
                  <span className="text-[var(--color-text-primary)]">{selectedRequest.type}</span>
                </div>
                <div>
                  <span className="text-[var(--color-text-muted)]">大小: </span>
                  <span className="text-[var(--color-text-primary)]">
                    {selectedRequest.size ? formatBytes(selectedRequest.size) : '--'}
                  </span>
                </div>
              </div>
            </div>

            {/* 标签页 */}
            <div className="border-b border-[var(--color-border-default)] mb-4">
              <div className="flex gap-4">
                <button
                  className={`px-4 py-2 text-sm font-medium transition-colors ${
                    activeTab === 'headers'
                      ? 'text-[var(--color-accent)] border-b-2 border-[var(--color-accent)]'
                      : 'text-[var(--color-text-muted)] hover:text-[var(--color-text-secondary)]'
                  }`}
                  onClick={() => setActiveTab('headers')}
                >
                  请求/响应头
                </button>
                <button
                  className={`px-4 py-2 text-sm font-medium transition-colors ${
                    activeTab === 'response'
                      ? 'text-[var(--color-accent)] border-b-2 border-[var(--color-accent)]'
                      : 'text-[var(--color-text-muted)] hover:text-[var(--color-text-secondary)]'
                  }`}
                  onClick={() => setActiveTab('response')}
                >
                  响应内容
                </button>
                <button
                  className={`px-4 py-2 text-sm font-medium transition-colors ${
                    activeTab === 'cookies'
                      ? 'text-[var(--color-accent)] border-b-2 border-[var(--color-accent)]'
                      : 'text-[var(--color-text-muted)] hover:text-[var(--color-text-secondary)]'
                  }`}
                  onClick={() => setActiveTab('cookies')}
                >
                  Cookies
                </button>
                <button
                  className={`px-4 py-2 text-sm font-medium transition-colors ${
                    activeTab === 'timing'
                      ? 'text-[var(--color-accent)] border-b-2 border-[var(--color-accent)]'
                      : 'text-[var(--color-text-muted)] hover:text-[var(--color-text-secondary)]'
                  }`}
                  onClick={() => setActiveTab('timing')}
                >
                  时间信息
                </button>
              </div>
            </div>

            {/* 标签页内容 */}
            <div className="max-h-96 overflow-auto">
              {activeTab === 'headers' && (
                <div className="space-y-4">
                  <div>
                    <h3 className="text-sm font-semibold text-[var(--color-text-primary)] mb-2">URL</h3>
                    <div className="bg-[var(--color-bg-muted)] p-3 rounded-lg text-sm break-all">
                      {selectedRequest.url}
                    </div>
                  </div>
                  {selectedRequest.requestHeaders && (
                    <div>
                      <h3 className="text-sm font-semibold text-[var(--color-text-primary)] mb-2">请求头</h3>
                      <div className="bg-[var(--color-bg-muted)] p-3 rounded-lg">
                        <pre className="text-xs text-[var(--color-text-secondary)]">
                          {JSON.stringify(selectedRequest.requestHeaders, null, 2)}
                        </pre>
                      </div>
                    </div>
                  )}
                  {selectedRequest.responseHeaders && (
                    <div>
                      <h3 className="text-sm font-semibold text-[var(--color-text-primary)] mb-2">响应头</h3>
                      <div className="bg-[var(--color-bg-muted)] p-3 rounded-lg">
                        <pre className="text-xs text-[var(--color-text-secondary)]">
                          {JSON.stringify(selectedRequest.responseHeaders, null, 2)}
                        </pre>
                      </div>
                    </div>
                  )}
                </div>
              )}

              {activeTab === 'response' && (
                <div>
                  {selectedRequest.responseBodyLoading ? (
                    <div className="text-center py-10">
                      <RefreshCw className="w-8 h-8 mx-auto mb-2 text-[var(--color-text-muted)] animate-spin" />
                      <div className="text-sm text-[var(--color-text-muted)]">正在加载响应内容...</div>
                    </div>
                  ) : selectedRequest.responseBodyError ? (
                    <div className="text-center py-10">
                      <div className="text-sm text-red-600 mb-2">⚠️ 响应内容获取失败</div>
                      <div className="text-xs text-[var(--color-text-muted)]">
                        可能原因：请求已完成、跨域限制或内容过大
                      </div>
                    </div>
                  ) : selectedRequest.responseBody ? (
                    <div className="bg-[var(--color-bg-muted)] p-3 rounded-lg">
                      <pre className="text-xs text-[var(--color-text-secondary)] whitespace-pre-wrap">
                        {selectedRequest.responseBody}
                      </pre>
                    </div>
                  ) : (
                    <div className="text-center py-10 text-sm text-[var(--color-text-muted)]">
                      暂无响应内容
                    </div>
                  )}
                </div>
              )}

              {activeTab === 'cookies' && (
                <div className="space-y-3">
                  {(() => {
                    const cookieHeader = selectedRequest.requestHeaders?.['Cookie'] || selectedRequest.requestHeaders?.['cookie']
                    const setCookieHeader = selectedRequest.responseHeaders?.['Set-Cookie'] || selectedRequest.responseHeaders?.['set-cookie']

                    return (
                      <>
                        {cookieHeader && (
                          <div>
                            <h3 className="text-sm font-semibold text-[var(--color-text-primary)] mb-2">请求Cookie</h3>
                            <div className="bg-[var(--color-bg-muted)] p-3 rounded-lg">
                              <pre className="text-xs text-[var(--color-text-secondary)] whitespace-pre-wrap">
                                {cookieHeader}
                              </pre>
                            </div>
                          </div>
                        )}
                        {setCookieHeader && (
                          <div>
                            <h3 className="text-sm font-semibold text-[var(--color-text-primary)] mb-2">响应Set-Cookie</h3>
                            <div className="bg-[var(--color-bg-muted)] p-3 rounded-lg">
                              <pre className="text-xs text-[var(--color-text-secondary)] whitespace-pre-wrap">
                                {setCookieHeader}
                              </pre>
                            </div>
                          </div>
                        )}
                        {!cookieHeader && !setCookieHeader && (
                          <div className="text-center py-10 text-sm text-[var(--color-text-muted)]">
                            此请求没有Cookie
                          </div>
                        )}
                      </>
                    )
                  })()}
                </div>
              )}

              {activeTab === 'timing' && (
                <div className="space-y-3">
                  <div className="grid grid-cols-2 gap-4">
                    <div className="bg-[var(--color-bg-muted)] p-3 rounded-lg">
                      <div className="text-xs text-[var(--color-text-muted)] mb-1">请求时间</div>
                      <div className="text-sm font-medium text-[var(--color-text-primary)]">
                        {new Date(selectedRequest.timestamp * 1000).toLocaleString()}
                      </div>
                    </div>
                    <div className="bg-[var(--color-bg-muted)] p-3 rounded-lg">
                      <div className="text-xs text-[var(--color-text-muted)] mb-1">耗时</div>
                      <div className="text-sm font-medium text-[var(--color-text-primary)]">
                        {selectedRequest.duration ? `${selectedRequest.duration}ms` : '计算中...'}
                      </div>
                    </div>
                  </div>
                </div>
              )}
            </div>
          </div>
        )}

        <div className="flex justify-end gap-2 pt-4 border-t border-[var(--color-border-default)]">
          <Button variant="secondary" onClick={() => {
            if (selectedRequest) {
              copyAsCurl(selectedRequest)
            }
          }}>
            <Copy className="w-4 h-4" />复制cURL
          </Button>
          <Button variant="secondary" onClick={() => {
            if (selectedRequest) {
              navigator.clipboard.writeText(JSON.stringify(selectedRequest, null, 2))
              toast.success('已复制请求详情')
            }
          }}>
            <Copy className="w-4 h-4" />复制JSON
          </Button>
          <Button onClick={() => setDetailModalOpen(false)}>
            关闭
          </Button>
        </div>
      </Modal>
    </div>
  )
}

