import { useEffect, useMemo, useRef, useState } from 'react'
import { Badge, Button, Card, Input, Modal, Select, Textarea, toast } from '../../../shared/components'
import type { BrowserProfile } from '../types'
import {
  fetchBrowserProfiles,
  CDPSessionCreate,
  CDPSessionClose,
  CDPGetNetworkRequests,
  CDPGetConsoleLogs,
  CDPGetWebSocketMessages,
  CDPGetCookies,
  CDPSetCookie,
  CDPDeleteCookie,
  CDPClearAllCookies,
  CDPClearNetworkRequests,
  CDPClearConsoleLogs,
  CDPClearWebSocketMessages,
  CDPReloadPage,
  CDPEnableConsoleCapture,
  CDPExportHAR,
  CDPExecuteJavaScript,
  CDPCaptureScreenshot,
  CDPGetStatistics,
  CDPGetStorage,
  type CDPNetworkRequest,
  type CDPConsoleLog,
  type CDPWebSocketMessage,
  type CDPCookie,
  type CDPInterceptRule,
  CDPEnableIntercept,
  CDPDisableIntercept,
  CDPAddInterceptRule,
  CDPRemoveInterceptRule,
  CDPGetInterceptRules,
  CDPUpdateInterceptRule,
} from '../api'
import { ResponseViewer } from '../components/ResponseViewer'

type ToolType = 'network' | 'console' | 'websocket' | 'cookies' | 'storage' | 'javascript' | 'screenshot' | 'performance' | 'intercept'

const TOOLS: { key: ToolType; label: string }[] = [
  { key: 'network', label: '网络抓包' },
  { key: 'console', label: '控制台' },
  { key: 'websocket', label: 'WebSocket' },
  { key: 'cookies', label: 'Cookie' },
  { key: 'intercept', label: '拦截' },
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
  const [connecting, setConnecting] = useState(false) // 连接中状态
  const [activeTool, setActiveTool] = useState<ToolType>('network')

  const [requests, setRequests] = useState<CDPNetworkRequest[]>([])
  const [consoleLogs, setConsoleLogs] = useState<CDPConsoleLog[]>([])
  const [wsMessages, setWsMessages] = useState<CDPWebSocketMessage[]>([])
  const [stats, setStats] = useState<Record<string, any>>({})
  const pollingRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const sessionIdRef = useRef<string | null>(null) // 用于 cleanup 时获取最新 sessionId

  // 网络抓包：筛选 / 排序 / 详情
  const [netSearch, setNetSearch] = useState('')
  const [netMethod, setNetMethod] = useState('all')
  const [netStatus, setNetStatus] = useState('all')
  const [netType, setNetType] = useState('all') // 资源类型筛选
  const [netDomain, setNetDomain] = useState('') // 域名筛选
  const [netMinSize, setNetMinSize] = useState<number | undefined>(undefined) // 最小大小（字节）
  const [netMaxSize, setNetMaxSize] = useState<number | undefined>(undefined) // 最大大小（字节）
  const [netMinDuration, setNetMinDuration] = useState<number | undefined>(undefined) // 最小耗时（ms）
  const [netMaxDuration, setNetMaxDuration] = useState<number | undefined>(undefined) // 最大耗时（ms）
  const [netSort, setNetSort] = useState<'time' | 'size' | 'duration' | 'status'>('time')
  const [netSortDesc, setNetSortDesc] = useState(true)
  const [selectedReq, setSelectedReq] = useState<CDPNetworkRequest | null>(null)
  const [reqTab, setReqTab] = useState<'general' | 'reqHeaders' | 'respHeaders' | 'response' | 'timing'>('general')

  // 控制台：筛选
  const [consoleFilter, setConsoleFilter] = useState<'all' | 'log' | 'warn' | 'error' | 'info'>('all')
  const [consoleSearch, setConsoleSearch] = useState('')

  // 高级筛选展开状态
  const [showAdvancedFilter, setShowAdvancedFilter] = useState(false)

  // WebSocket：筛选
  const [wsSearch, setWsSearch] = useState('')
  const [wsDirection, setWsDirection] = useState<'all' | 'send' | 'receive'>('all')
  const [wsConnection, setWsConnection] = useState('all')

  // Cookie：列表和筛选
  const [cookies, setCookies] = useState<CDPCookie[]>([])
  const [cookieSearch, setCookieSearch] = useState('')
  const [cookieDomain, setCookieDomain] = useState('')
  const [selectedCookie, setSelectedCookie] = useState<CDPCookie | null>(null)
  const [editingCookie, setEditingCookie] = useState<CDPCookie | null>(null)

  // HAR 导入
  const [importedRequests, setImportedRequests] = useState<CDPNetworkRequest[]>([])
  const [compareMode, setCompareMode] = useState(false)

  // 拦截规则
  const [interceptEnabled, setInterceptEnabled] = useState(false)
  const [interceptRules, setInterceptRules] = useState<CDPInterceptRule[]>([])
  const [editingRule, setEditingRule] = useState<CDPInterceptRule | null>(null)
  const [showRuleEditor, setShowRuleEditor] = useState(false)

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

  // 同步 sessionId 到 ref，确保 cleanup 能访问最新值
  useEffect(() => {
    sessionIdRef.current = sessionId
  }, [sessionId])

  useEffect(() => {
    loadProfiles()
    return () => {
      if (pollingRef.current) clearInterval(pollingRef.current)
      // 使用 ref 获取最新 sessionId，避免闭包捕获初始 null
      if (sessionIdRef.current) CDPSessionClose(sessionIdRef.current).catch(console.error)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // 仅当用户实际查看「控制台」时才按需启用 Runtime 域捕获页面 console 日志。
  // Runtime.enable 是最易被检测站点经侧信道探测的 CDP 痕迹，默认不开启以降低指纹暴露面。
  // 后端 EnableConsoleCapture 幂等，重复触发安全。
  useEffect(() => {
    if (activeTool === 'console' && sessionId) {
      CDPEnableConsoleCapture(sessionId).catch(console.error)
    }
  }, [activeTool, sessionId])

  const loadProfiles = async () => {
    try {
      const list = await fetchBrowserProfiles()
      // 只显示运行中且调试端口已就绪的窗口（debugPort > 0 表示端口已初始化）
      setProfiles(list.filter(p => p.running && p.debugPort > 0))
    } catch {
      toast.error('加载浏览器窗口失败')
    }
  }

  const startCapture = async () => {
    if (!selectedProfileId) {
      toast.error('请选择一个运行中的浏览器窗口')
      return
    }

    setConnecting(true) // 显示连接中状态
    try {
      const newSessionId = await CDPSessionCreate(selectedProfileId, 'page')
      setSessionId(newSessionId)
      setCapturing(true)
      setRequests([])
      setConsoleLogs([])
      setWsMessages([])
      setStats({})
      toast.success('开发工具已连接')

      const interval = setInterval(async () => {
        try {
          const [networkData, consoleData, wsData, statData] = await Promise.all([
            CDPGetNetworkRequests(newSessionId),
            CDPGetConsoleLogs(newSessionId),
            CDPGetWebSocketMessages(newSessionId),
            CDPGetStatistics(newSessionId),
          ])
          setRequests(networkData || [])
          setConsoleLogs(consoleData || [])
          setWsMessages(wsData || [])
          if (statData) setStats(statData)
        } catch (error) {
          console.error('轮询数据失败:', error)
        }
      }, 1000)
      pollingRef.current = interval
    } catch (error: any) {
      toast.error(error?.message || '连接失败')
      setCapturing(false)
    } finally {
      setConnecting(false) // 隐藏连接中状态
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
      toast.error('请先点击「开始」连接窗口')
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

  const handleClearWsMessages = async () => {
    if (sessionId) { try { await CDPClearWebSocketMessages(sessionId) } catch { /* ignore */ } }
    setWsMessages([])
  }

  // Cookie 加载和管理
  const loadCookies = async () => {
    const sid = requireSession(); if (!sid) return
    try {
      const data = await CDPGetCookies(sid)
      setCookies(data)
    } catch (error: any) {
      toast.error(`加载 Cookie 失败: ${error?.message || error}`)
    }
  }

  const handleDeleteCookie = async (name: string, domain: string, path: string) => {
    const sid = requireSession(); if (!sid) return
    try {
      await CDPDeleteCookie(sid, name, domain, path)
      toast.success('Cookie 已删除')
      await loadCookies()
    } catch (error: any) {
      toast.error(`删除失败: ${error?.message || error}`)
    }
  }

  const handleClearAllCookies = async () => {
    const sid = requireSession(); if (!sid) return
    try {
      await CDPClearAllCookies(sid)
      toast.success('所有 Cookie 已清空')
      await loadCookies()
    } catch (error: any) {
      toast.error(`清空失败: ${error?.message || error}`)
    }
  }

  const handleSaveCookie = async () => {
    if (!editingCookie) return
    const sid = requireSession(); if (!sid) return

    try {
      await CDPSetCookie(sid, editingCookie)
      toast.success('Cookie 已保存')
      setEditingCookie(null)
      await loadCookies()
    } catch (error: any) {
      toast.error(`保存失败: ${error?.message || error}`)
    }
  }

  // HAR 导入功能
  const handleImportHAR = () => {
    const input = document.createElement('input')
    input.type = 'file'
    input.accept = '.har'
    input.onchange = async (e) => {
      const file = (e.target as HTMLInputElement).files?.[0]
      if (!file) return

      try {
        const text = await file.text()
        const harData = JSON.parse(text)

        // 解析 HAR 格式
        const entries = harData.log?.entries || []
        const parsed: CDPNetworkRequest[] = entries.map((entry: any, index: number) => {
          const req = entry.request
          const resp = entry.response

          // 构造请求头和响应头对象
          const requestHeaders: Record<string, string> = {}
          req.headers?.forEach((h: any) => {
            requestHeaders[h.name] = h.value
          })

          const responseHeaders: Record<string, string> = {}
          resp.headers?.forEach((h: any) => {
            responseHeaders[h.name] = h.value
          })

          return {
            requestId: `har-${index}`,
            url: req.url,
            method: req.method,
            type: resp.content?.mimeType?.split('/')[0] || 'other',
            statusCode: resp.status,
            statusText: resp.statusText,
            size: resp.bodySize > 0 ? resp.bodySize : resp.content?.size || 0,
            duration: entry.time || 0,
            timestamp: new Date(entry.startedDateTime).getTime(),
            requestHeaders,
            responseHeaders,
            requestBody: req.postData?.text || '',
            responseBody: resp.content?.text || '',
            truncated: false
          }
        })

        setImportedRequests(parsed)
        setCompareMode(true)
        toast.success(`已导入 ${parsed.length} 个请求`)
      } catch (error: any) {
        toast.error(`导入失败: ${error?.message || '无效的 HAR 文件'}`)
      }
    }
    input.click()
  }

  const handleClearImportedHAR = () => {
    setImportedRequests([])
    setCompareMode(false)
    toast.success('已清除导入的 HAR 数据')
  }

  // 拦截规则管理
  const loadInterceptRules = async () => {
    const sid = requireSession(); if (!sid) return
    try {
      const rules = await CDPGetInterceptRules(sid)
      setInterceptRules(rules)
    } catch (error: any) {
      toast.error(`加载拦截规则失败: ${error?.message || error}`)
    }
  }

  const handleToggleIntercept = async () => {
    const sid = requireSession(); if (!sid) return
    try {
      if (interceptEnabled) {
        await CDPDisableIntercept(sid)
        setInterceptEnabled(false)
        toast.success('请求拦截已禁用')
      } else {
        await CDPEnableIntercept(sid)
        setInterceptEnabled(true)
        toast.success('请求拦截已启用')
      }
    } catch (error: any) {
      toast.error(`操作失败: ${error?.message || error}`)
    }
  }

  const handleAddRule = () => {
    const newRule: CDPInterceptRule = {
      id: `rule-${Date.now()}`,
      name: '新规则',
      enabled: true,
      urlPattern: '*',
      method: '',
      actions: {
        block: false,
        modifyRequest: false,
        modifyResponse: false,
        delay: 0,
      },
    }
    setEditingRule(newRule)
    setShowRuleEditor(true)
  }

  const handleEditRule = (rule: CDPInterceptRule) => {
    setEditingRule({ ...rule })
    setShowRuleEditor(true)
  }

  const handleSaveRule = async () => {
    if (!editingRule) return
    const sid = requireSession(); if (!sid) return

    try {
      const existing = interceptRules.find(r => r.id === editingRule.id)
      if (existing) {
        await CDPUpdateInterceptRule(sid, editingRule)
        toast.success('规则已更新')
      } else {
        await CDPAddInterceptRule(sid, editingRule)
        toast.success('规则已添加')
      }
      setShowRuleEditor(false)
      setEditingRule(null)
      await loadInterceptRules()
    } catch (error: any) {
      toast.error(`保存失败: ${error?.message || error}`)
    }
  }

  const handleDeleteRule = async (ruleId: string) => {
    const sid = requireSession(); if (!sid) return
    try {
      await CDPRemoveInterceptRule(sid, ruleId)
      toast.success('规则已删除')
      await loadInterceptRules()
    } catch (error: any) {
      toast.error(`删除失败: ${error?.message || error}`)
    }
  }

  const handleToggleRule = async (rule: CDPInterceptRule) => {
    const sid = requireSession(); if (!sid) return
    try {
      const updated = { ...rule, enabled: !rule.enabled }
      await CDPUpdateInterceptRule(sid, updated)
      await loadInterceptRules()
    } catch (error: any) {
      toast.error(`操作失败: ${error?.message || error}`)
    }
  }

  const handleReloadCapture = async () => {
    const sid = requireSession(); if (!sid) return
    try {
      await CDPReloadPage(sid)
      toast.success('已触发页面重新加载，正在抓取本次加载的请求')
    } catch (error: any) {
      toast.error(error?.message || '重新加载失败')
    }
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

  // ── 网络抓包筛选 / 排序 / 统计 ──
  const statusBucket = (code: number) => (!code ? 'pending' : code < 300 ? '2xx' : code < 400 ? '3xx' : code < 500 ? '4xx' : '5xx')

  const methodOptions = useMemo(() => {
    const set = new Set<string>()
    requests.forEach(r => r.method && set.add(r.method.toUpperCase()))
    return ['all', ...Array.from(set).sort()]
  }, [requests])

  const typeOptions = useMemo(() => {
    const set = new Set<string>()
    requests.forEach(r => {
      // 优先使用解析后的数据类型
      if (r.parsedData?.type) {
        set.add(r.parsedData.type)
      } else if (r.type) {
        set.add(r.type)
      }
    })
    return ['all', ...Array.from(set).sort()]
  }, [requests])

  const domainOptions = useMemo(() => {
    const set = new Set<string>()
    requests.forEach(r => {
      try {
        const url = new URL(r.url)
        set.add(url.hostname)
      } catch { /* ignore */ }
    })
    return ['', ...Array.from(set).sort()]
  }, [requests])

  const filteredRequests = useMemo(() => {
    const q = netSearch.trim().toLowerCase()
    const domain = netDomain.trim().toLowerCase()

    let list = requests.filter(r => {
      // URL 关键词筛选
      if (q && !(r.url || '').toLowerCase().includes(q)) return false

      // HTTP 方法筛选
      if (netMethod !== 'all' && (r.method || '').toUpperCase() !== netMethod) return false

      // 状态码筛选
      if (netStatus !== 'all' && statusBucket(r.statusCode) !== netStatus) return false

      // 资源类型筛选（优先使用解析后的数据类型）
      if (netType !== 'all') {
        const requestType = r.parsedData?.type || r.type || ''
        if (requestType !== netType) return false
      }

      // 域名筛选
      if (domain) {
        try {
          const url = new URL(r.url)
          if (!url.hostname.toLowerCase().includes(domain)) return false
        } catch {
          return false
        }
      }

      // 大小范围筛选
      if (netMinSize !== undefined && (r.size || 0) < netMinSize) return false
      if (netMaxSize !== undefined && (r.size || 0) > netMaxSize) return false

      // 耗时范围筛选
      if (netMinDuration !== undefined && (r.duration || 0) < netMinDuration) return false
      if (netMaxDuration !== undefined && (r.duration || 0) > netMaxDuration) return false

      return true
    })

    list = [...list].sort((a, b) => {
      let cmp = 0
      if (netSort === 'time') cmp = (a.startTime || a.timestamp || 0) - (b.startTime || b.timestamp || 0)
      else if (netSort === 'size') cmp = (a.size || 0) - (b.size || 0)
      else if (netSort === 'duration') cmp = (a.duration || 0) - (b.duration || 0)
      else cmp = (a.statusCode || 0) - (b.statusCode || 0)
      return netSortDesc ? -cmp : cmp
    })
    return list
  }, [requests, netSearch, netMethod, netStatus, netType, netDomain, netMinSize, netMaxSize, netMinDuration, netMaxDuration, netSort, netSortDesc])

  const netSummary = useMemo(() => {
    let success = 0, failed = 0, pending = 0, size = 0
    requests.forEach(r => {
      size += r.size || 0
      if (!r.statusCode) pending++
      else if (r.statusCode >= 400) failed++
      else success++
    })
    return { total: requests.length, success, failed, pending, size }
  }, [requests])

  const filteredConsole = useMemo(() => {
    const q = consoleSearch.trim().toLowerCase()
    return consoleLogs.filter(l => {
      if (consoleFilter !== 'all' && l.type !== consoleFilter) return false
      if (q && !(l.message || '').toLowerCase().includes(q)) return false
      return true
    })
  }, [consoleLogs, consoleFilter, consoleSearch])

  // WebSocket 消息筛选
  const wsConnectionOptions = useMemo(() => {
    const connections = new Set<string>()
    wsMessages.forEach(m => m.url && connections.add(m.url))
    return ['all', ...Array.from(connections)]
  }, [wsMessages])

  const filteredWsMessages = useMemo(() => {
    const q = wsSearch.trim().toLowerCase()
    return wsMessages.filter(m => {
      if (wsDirection !== 'all' && m.direction !== wsDirection) return false
      if (wsConnection !== 'all' && m.url !== wsConnection) return false
      if (q && !(m.data || '').toLowerCase().includes(q)) return false
      return true
    })
  }, [wsMessages, wsSearch, wsDirection, wsConnection])

  // Cookie 筛选
  const cookieDomainOptions = useMemo(() => {
    const domains = new Set<string>()
    cookies.forEach(c => c.domain && domains.add(c.domain))
    return ['', ...Array.from(domains).sort()]
  }, [cookies])

  const filteredCookies = useMemo(() => {
    const q = cookieSearch.trim().toLowerCase()
    const domain = cookieDomain.trim().toLowerCase()
    return cookies.filter(c => {
      if (q && !(c.name || '').toLowerCase().includes(q) && !(c.value || '').toLowerCase().includes(q)) return false
      if (domain && !(c.domain || '').toLowerCase().includes(domain)) return false
      return true
    })
  }, [cookies, cookieSearch, cookieDomain])

  const fmtSize = (n: number) => (n ? (n < 1024 ? `${n} B` : `${(n / 1024).toFixed(2)} KB`) : '-')

  // 根据耗时返回颜色类名
  const getDurationColor = (duration: number | undefined): string => {
    if (!duration) return 'text-[var(--color-text-muted)]'
    if (duration < 100) return 'text-green-600 dark:text-green-400'
    if (duration < 1000) return 'text-yellow-600 dark:text-yellow-400'
    return 'text-red-600 dark:text-red-400'
  }

  // 格式化耗时显示
  const fmtDuration = (duration: number | undefined): string => {
    if (!duration) return '-'
    if (duration < 1000) return `${duration}ms`
    return `${(duration / 1000).toFixed(2)}s`
  }

  // 生成 cURL 命令
  const generateCurl = (req: CDPNetworkRequest): string => {
    let curl = `curl '${req.url}'`

    // 添加方法
    if (req.method && req.method !== 'GET') {
      curl += ` -X ${req.method}`
    }

    // 添加请求头
    if (req.requestHeaders) {
      Object.entries(req.requestHeaders).forEach(([key, value]) => {
        // 跳过某些自动生成的头
        if (!['Host', 'Connection', 'Content-Length'].includes(key)) {
          curl += ` \\\n  -H '${key}: ${value}'`
        }
      })
    }

    // 添加请求体
    if (req.requestBody) {
      curl += ` \\\n  --data '${req.requestBody.replace(/'/g, "'\\''")}'`
    }

    return curl
  }

  // 复制 cURL 到剪贴板
  const copyCurl = (req: CDPNetworkRequest) => {
    const curl = generateCurl(req)
    navigator.clipboard.writeText(curl).then(() => {
      toast.success('cURL 命令已复制到剪贴板')
    }).catch(() => {
      toast.error('复制失败')
    })
  }

  // 复制文本到剪贴板
  const copyText = (text: string, label: string) => {
    navigator.clipboard.writeText(text).then(() => {
      toast.success(`${label}已复制到剪贴板`)
    }).catch(() => {
      toast.error('复制失败')
    })
  }

  // 重放请求
  const [replayLoading, setReplayLoading] = useState(false)
  const [replayResult, setReplayResult] = useState<{ status: number; statusText: string; body: string; time: number } | null>(null)

  const replayRequest = async (req: CDPNetworkRequest) => {
    const sid = requireSession(); if (!sid) return

    setReplayLoading(true)
    setReplayResult(null)

    try {
      const replayInit: RequestInit = {
        method: req.method,
        headers: req.requestHeaders || {},
      }
      if (req.requestBody) {
        replayInit.body = req.requestBody
      }

      // 构造 Fetch 请求代码。所有抓包数据都以 JSON 字面量注入，避免字符串逃逸成可执行 JS。
      const fetchCode = `
        (async () => {
          const replayUrl = ${JSON.stringify(req.url)}
          const replayInit = ${JSON.stringify(replayInit)}
          const startTime = Date.now()
          const response = await fetch(replayUrl, replayInit)
          const text = await response.text()
          const endTime = Date.now()
          return JSON.stringify({
            status: response.status,
            statusText: response.statusText,
            body: text.substring(0, 10000), // 限制 10KB
            time: endTime - startTime
          })
        })()
      `

      const result = await CDPExecuteJavaScript(sid, fetchCode)
      const parsed = JSON.parse(result)
      setReplayResult(parsed)
      toast.success(`请求已重放，耗时 ${parsed.time}ms`)
    } catch (error: any) {
      toast.error(`重放失败: ${error?.message || error}`)
    } finally {
      setReplayLoading(false)
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
          <Select
            value={selectedProfileId}
            onChange={(e) => setSelectedProfileId(e.target.value)}
            disabled={capturing}
            className="flex-1"
            options={[
              { value: '', label: profiles.length ? '选择运行中的浏览器窗口' : '暂无运行中的窗口（先去窗口列表启动）' },
              ...profiles.map(p => ({ value: p.profileId, label: `${p.profileName}（端口 ${p.debugPort}）` })),
            ]}
          />
          <Button
            onClick={capturing ? stopCapture : startCapture}
            variant={capturing ? 'secondary' : undefined}
            disabled={connecting}
          >
            {connecting ? '连接中...' : capturing ? '停止' : '连接'}
          </Button>
          <Button variant="ghost" onClick={loadProfiles} disabled={capturing || connecting}>刷新窗口</Button>
        </div>
        {profiles.length === 0 && (
          <p className="text-xs text-[var(--color-text-muted)] px-1 pt-2">
            没有运行中且调试就绪的窗口——请先在窗口列表启动一个窗口，并等待调试端口初始化（通常需要 2-3 秒）
          </p>
        )}
      </Card>

      {/* 工具切换 */}
      <div className="flex gap-2 flex-wrap">
        {TOOLS.map(t => (
          <Button key={t.key} size="sm" variant={activeTool === t.key ? undefined : 'secondary'} onClick={() => setActiveTool(t.key)}>
            {t.label}
            {t.key === 'network' && requests.length > 0 ? ` (${requests.length})` : ''}
            {t.key === 'console' && consoleLogs.length > 0 ? ` (${consoleLogs.length})` : ''}
            {t.key === 'websocket' && wsMessages.length > 0 ? ` (${wsMessages.length})` : ''}
          </Button>
        ))}
      </div>

      {/* 网络抓包 */}
      {activeTool === 'network' && (
        <Card title="网络抓包">
          <div className="flex gap-2 mb-3 items-center flex-wrap">
            <Button size="sm" onClick={handleReloadCapture} disabled={!capturing}>重新加载抓包</Button>
            <Button size="sm" variant="secondary" onClick={handleClearRequests}>清空</Button>
            <Button size="sm" variant="secondary" onClick={handleExportHAR}>导出 HAR</Button>
            <Button size="sm" variant="secondary" onClick={handleImportHAR}>导入 HAR</Button>
            {compareMode && (
              <Button size="sm" variant="danger" onClick={handleClearImportedHAR}>清除对比</Button>
            )}
            {compareMode && (
              <Badge variant="success" className="ml-2">对比模式：导入 {importedRequests.length} 个</Badge>
            )}
          </div>

          {/* 快捷类型筛选 */}
          <div className="flex gap-2 mb-3 items-center flex-wrap">
            <span className="text-xs text-[var(--color-text-muted)]">快速筛选:</span>
            {['all', 'json', 'xml', 'html', 'image', 'javascript', 'css', 'graphql'].map(type => {
              const count = requests.filter(r => {
                if (type === 'all') return true
                return (r.parsedData?.type || r.type) === type
              }).length
              return (
                <Button
                  key={type}
                  size="sm"
                  variant={netType === type ? 'primary' : 'ghost'}
                  onClick={() => setNetType(type)}
                  disabled={count === 0 && type !== 'all'}
                >
                  {getTypeIcon(type)} {getTypeLabel(type)} {count > 0 && `(${count})`}
                </Button>
              )
            })}
          </div>
          {/* 统计摘要 */}
          <div className="flex gap-2 mb-3 flex-wrap text-xs">
            <Badge variant="default">总数 {netSummary.total}</Badge>
            <Badge variant="success">成功 {netSummary.success}</Badge>
            <Badge variant="error">失败 {netSummary.failed}</Badge>
            <Badge variant="warning">进行中 {netSummary.pending}</Badge>
            <Badge variant="default">大小 {fmtSize(netSummary.size)}</Badge>
          </div>
          {/* 筛选 / 排序 */}
          <div className="flex gap-2 mb-3 flex-wrap items-center">
            <Input value={netSearch} onChange={e => setNetSearch(e.target.value)} placeholder="按 URL 过滤…" className="flex-1 min-w-[160px]" />
            <Select value={netMethod} onChange={e => setNetMethod(e.target.value)} className="w-28"
              options={methodOptions.map(m => ({ value: m, label: m === 'all' ? '全部方法' : m }))} />
            <Select value={netStatus} onChange={e => setNetStatus(e.target.value)} className="w-28"
              options={[
                { value: 'all', label: '全部状态' }, { value: '2xx', label: '2xx' }, { value: '3xx', label: '3xx' },
                { value: '4xx', label: '4xx' }, { value: '5xx', label: '5xx' }, { value: 'pending', label: '进行中' },
              ]} />
            <Select value={netSort} onChange={e => setNetSort(e.target.value as any)} className="w-28"
              options={[
                { value: 'time', label: '时间' }, { value: 'size', label: '大小' },
                { value: 'duration', label: '耗时' }, { value: 'status', label: '状态码' },
              ]} />
            <Button size="sm" variant="ghost" onClick={() => setNetSortDesc(v => !v)} title="切换升降序">{netSortDesc ? '↓' : '↑'}</Button>
            <Button size="sm" variant="ghost" onClick={() => setShowAdvancedFilter(v => !v)}>
              {showAdvancedFilter ? '▲ 收起筛选' : '▼ 高级筛选'}
            </Button>
          </div>
          {/* 高级筛选面板 */}
          {showAdvancedFilter && (
            <div className="mb-3 p-3 border border-[var(--color-border-default)] rounded-lg space-y-2">
              <div className="flex gap-2 flex-wrap items-center">
                <label className="text-xs text-[var(--color-text-muted)] w-16">资源类型</label>
                <Select value={netType} onChange={e => setNetType(e.target.value)} className="flex-1 min-w-[120px]"
                  options={typeOptions.map(t => ({ value: t, label: t === 'all' ? '全部类型' : t }))} />
              </div>
              <div className="flex gap-2 flex-wrap items-center">
                <label className="text-xs text-[var(--color-text-muted)] w-16">域名</label>
                <Input
                  value={netDomain}
                  onChange={e => setNetDomain(e.target.value)}
                  placeholder="输入域名筛选…"
                  className="flex-1 min-w-[200px]"
                  list="domain-suggestions"
                />
                <datalist id="domain-suggestions">
                  {domainOptions.map(d => d && <option key={d} value={d} />)}
                </datalist>
              </div>
              <div className="flex gap-2 flex-wrap items-center">
                <label className="text-xs text-[var(--color-text-muted)] w-16">大小范围</label>
                <Input
                  type="number"
                  value={netMinSize ?? ''}
                  onChange={e => setNetMinSize(e.target.value ? Number(e.target.value) : undefined)}
                  placeholder="最小（字节）"
                  className="w-32"
                />
                <span className="text-xs text-[var(--color-text-muted)]">-</span>
                <Input
                  type="number"
                  value={netMaxSize ?? ''}
                  onChange={e => setNetMaxSize(e.target.value ? Number(e.target.value) : undefined)}
                  placeholder="最大（字节）"
                  className="w-32"
                />
                <Button size="sm" variant="ghost" onClick={() => { setNetMinSize(1024 * 1024); setNetMaxSize(undefined) }}>
                  &gt; 1MB
                </Button>
              </div>
              <div className="flex gap-2 flex-wrap items-center">
                <label className="text-xs text-[var(--color-text-muted)] w-16">耗时范围</label>
                <Input
                  type="number"
                  value={netMinDuration ?? ''}
                  onChange={e => setNetMinDuration(e.target.value ? Number(e.target.value) : undefined)}
                  placeholder="最小（ms）"
                  className="w-32"
                />
                <span className="text-xs text-[var(--color-text-muted)]">-</span>
                <Input
                  type="number"
                  value={netMaxDuration ?? ''}
                  onChange={e => setNetMaxDuration(e.target.value ? Number(e.target.value) : undefined)}
                  placeholder="最大（ms）"
                  className="w-32"
                />
                <Button size="sm" variant="ghost" onClick={() => { setNetMinDuration(1000); setNetMaxDuration(undefined) }}>
                  &gt; 1s
                </Button>
              </div>
              <div className="flex gap-2">
                <Button size="sm" variant="ghost" onClick={() => {
                  setNetType('all')
                  setNetDomain('')
                  setNetMinSize(undefined)
                  setNetMaxSize(undefined)
                  setNetMinDuration(undefined)
                  setNetMaxDuration(undefined)
                }}>
                  清空高级筛选
                </Button>
              </div>
            </div>
          )}
          <div className="space-y-1 max-h-[460px] overflow-y-auto">
            {filteredRequests.length === 0 ? (
              <p className="text-sm text-[var(--color-text-muted)] text-center py-8">{capturing ? (requests.length ? '无匹配请求' : '等待网络请求…（可点「重新加载抓包」）') : '未连接'}</p>
            ) : filteredRequests.map(req => {
              // 对比模式：查找导入的 HAR 中是否有相同 URL 的请求
              let matchedImported: CDPNetworkRequest | undefined
              let statusChanged = false
              let sizeChanged = false
              let durationChanged = false

              if (compareMode && importedRequests.length > 0) {
                matchedImported = importedRequests.find(imp => imp.url === req.url && imp.method === req.method)
                if (matchedImported) {
                  statusChanged = matchedImported.statusCode !== req.statusCode
                  sizeChanged = Math.abs(matchedImported.size - req.size) > req.size * 0.1 // 变化超过 10%
                  durationChanged = !!(matchedImported.duration && req.duration && Math.abs(matchedImported.duration - req.duration) > matchedImported.duration * 0.2) // 变化超过 20%
                }
              }

              return (
                <button key={req.requestId} onClick={() => { setSelectedReq(req); setReqTab('general') }}
                  className={`w-full text-left p-2.5 border rounded-lg hover:border-[var(--color-accent)] transition-colors ${
                    compareMode && matchedImported && (statusChanged || sizeChanged || durationChanged)
                      ? 'border-yellow-400 bg-yellow-50 dark:bg-yellow-900/10'
                      : 'border-[var(--color-border-default)]'
                  }`}>
                  <div className="flex items-center gap-2">
                    <Badge variant={req.statusCode >= 200 && req.statusCode < 400 ? 'success' : req.statusCode >= 400 ? 'error' : 'default'}>
                      {req.statusCode || 'Pending'}
                    </Badge>
                    <span className="font-mono text-xs shrink-0">{req.method}</span>
                    <span className="truncate text-xs text-[var(--color-text-secondary)] flex-1">{req.url}</span>
                    {compareMode && matchedImported && (statusChanged || sizeChanged || durationChanged) && (
                      <Badge variant="warning" className="text-[10px]">变化</Badge>
                    )}
                    {compareMode && !matchedImported && (
                      <Badge variant="success" className="text-[10px]">新增</Badge>
                    )}
                  </div>
                  <div className="text-[11px] text-[var(--color-text-muted)] mt-1 flex items-center gap-1.5">
                    <span>{req.type || '-'}</span>
                    <span>•</span>
                    <span>{req.mimeType || '-'}</span>
                    <span>•</span>
                    <span className={sizeChanged ? 'font-bold text-yellow-600' : ''}>
                      {fmtSize(req.size)}
                      {matchedImported && sizeChanged && ` (原: ${fmtSize(matchedImported.size)})`}
                    </span>
                    <span>•</span>
                    <span className={`font-medium ${getDurationColor(req.duration)} ${durationChanged ? 'font-bold' : ''}`}>
                      {fmtDuration(req.duration)}
                      {matchedImported && durationChanged && ` (原: ${fmtDuration(matchedImported.duration)})`}
                    </span>
                  </div>
                </button>
              )
            })}
          </div>

          {/* 显示导入 HAR 中未在当前抓包出现的请求 */}
          {compareMode && importedRequests.length > 0 && (
            <div className="mt-4">
              <h4 className="text-xs font-semibold mb-2 text-[var(--color-text-muted)]">仅在导入的 HAR 中存在（已移除或未触发）</h4>
              <div className="space-y-1 max-h-[200px] overflow-y-auto">
                {importedRequests.filter(imp => !requests.some(req => req.url === imp.url && req.method === imp.method)).map(req => (
                  <div key={req.requestId} className="p-2.5 border border-red-300 dark:border-red-800 bg-red-50 dark:bg-red-900/10 rounded-lg">
                    <div className="flex items-center gap-2">
                      <Badge variant="error">{req.statusCode || 'N/A'}</Badge>
                      <span className="font-mono text-xs shrink-0">{req.method}</span>
                      <span className="truncate text-xs text-[var(--color-text-secondary)] flex-1">{req.url}</span>
                      <Badge variant="error" className="text-[10px]">已移除</Badge>
                    </div>
                    <div className="text-[11px] text-[var(--color-text-muted)] mt-1 flex items-center gap-1.5">
                      <span>{req.type || '-'}</span>
                      <span>•</span>
                      <span>{fmtSize(req.size)}</span>
                      <span>•</span>
                      <span className={getDurationColor(req.duration)}>{fmtDuration(req.duration)}</span>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}
        </Card>
      )}

      {/* 控制台 */}
      {activeTool === 'console' && (
        <Card title="控制台">
          <div className="flex gap-2 mb-3 items-center flex-wrap">
            <Button size="sm" variant="secondary" onClick={handleClearConsole}>清空</Button>
            <Select value={consoleFilter} onChange={e => setConsoleFilter(e.target.value as any)} className="w-28"
              options={[
                { value: 'all', label: '全部级别' }, { value: 'log', label: 'log' }, { value: 'info', label: 'info' },
                { value: 'warn', label: 'warn' }, { value: 'error', label: 'error' },
              ]} />
            <Input value={consoleSearch} onChange={e => setConsoleSearch(e.target.value)} placeholder="搜索日志…" className="flex-1 min-w-[140px]" />
          </div>
          <div className="space-y-1.5 font-mono text-xs max-h-[480px] overflow-y-auto">
            {filteredConsole.length === 0 ? (
              <p className="text-center py-8 text-[var(--color-text-muted)]">{capturing ? (consoleLogs.length ? '无匹配日志' : '等待日志…') : '未连接'}</p>
            ) : filteredConsole.map(log => (
              <div key={log.id} className={`p-2 rounded ${log.type === 'error' ? 'bg-red-50 text-red-700 dark:bg-red-900/20 dark:text-red-400' : log.type === 'warn' ? 'bg-yellow-50 text-yellow-700 dark:bg-yellow-900/20 dark:text-yellow-400' : 'bg-[var(--color-bg-subtle)]'}`}>
                <span className="font-bold">[{log.type}]</span> {log.message}
                {log.stackTrace && <pre className="mt-1 text-[11px] opacity-70 whitespace-pre-wrap">{log.stackTrace}</pre>}
              </div>
            ))}
          </div>
        </Card>
      )}

      {/* WebSocket */}
      {activeTool === 'websocket' && (
        <Card title="WebSocket 消息">
          <div className="flex gap-2 mb-3 items-center flex-wrap">
            <Button size="sm" variant="secondary" onClick={handleClearWsMessages}>清空</Button>
            <Select value={wsDirection} onChange={e => setWsDirection(e.target.value as any)} className="w-28"
              options={[
                { value: 'all', label: '全部方向' }, { value: 'send', label: '发送' }, { value: 'receive', label: '接收' },
              ]} />
            <Select value={wsConnection} onChange={e => setWsConnection(e.target.value)} className="flex-1 min-w-[200px]"
              options={wsConnectionOptions.map(c => ({ value: c, label: c === 'all' ? '全部连接' : c }))} />
            <Input value={wsSearch} onChange={e => setWsSearch(e.target.value)} placeholder="搜索消息内容…" className="flex-1 min-w-[140px]" />
          </div>
          <div className="space-y-1.5 text-xs max-h-[480px] overflow-y-auto">
            {filteredWsMessages.length === 0 ? (
              <p className="text-center py-8 text-[var(--color-text-muted)]">{capturing ? (wsMessages.length ? '无匹配消息' : '等待 WebSocket 消息…') : '未连接'}</p>
            ) : filteredWsMessages.map(msg => (
              <div key={msg.id} className="p-2.5 border border-[var(--color-border-default)] rounded-lg">
                <div className="flex items-center gap-2 mb-1">
                  <Badge variant={msg.direction === 'send' ? 'default' : 'success'}>
                    {msg.direction === 'send' ? '↑ 发送' : '↓ 接收'}
                  </Badge>
                  <span className="text-[var(--color-text-muted)] text-[11px]">
                    {new Date(msg.timestamp).toLocaleTimeString()}
                  </span>
                  <span className="text-[var(--color-text-muted)] text-[11px]">
                    {msg.payloadSize} 字节
                  </span>
                  <span className="text-[var(--color-text-muted)] text-[11px]">
                    {msg.opcode === 1 ? 'Text' : msg.opcode === 2 ? 'Binary' : `Opcode ${msg.opcode}`}
                  </span>
                </div>
                <div className="text-[11px] text-[var(--color-text-muted)] truncate mb-1">
                  {msg.url}
                </div>
                <pre className="text-[11px] bg-[var(--color-bg-subtle)] p-2 rounded whitespace-pre-wrap break-all max-h-32 overflow-y-auto font-mono">
                  {msg.data || '(空消息)'}
                </pre>
              </div>
            ))}
          </div>
        </Card>
      )}

      {/* Cookie 管理 */}
      {activeTool === 'cookies' && (
        <Card title="Cookie 管理">
          <div className="flex gap-2 mb-3 items-center flex-wrap">
            <Button size="sm" variant="secondary" onClick={loadCookies}>刷新</Button>
            <Button size="sm" variant="danger" onClick={handleClearAllCookies}>清空全部</Button>
            <Select
              value={cookieDomain}
              onChange={e => setCookieDomain(e.target.value)}
              className="flex-1 min-w-[200px]"
              options={cookieDomainOptions.map(d => ({ value: d, label: d === 'all' ? '全部域名' : d }))}
            />
            <Input
              value={cookieSearch}
              onChange={e => setCookieSearch(e.target.value)}
              placeholder="搜索 Cookie 名称或值…"
              className="flex-1 min-w-[140px]"
            />
          </div>
          <div className="space-y-1.5 text-xs max-h-[480px] overflow-y-auto">
            {filteredCookies.length === 0 ? (
              <p className="text-center py-8 text-[var(--color-text-muted)]">
                {cookies.length ? '无匹配 Cookie' : '无 Cookie'}
              </p>
            ) : filteredCookies.map(cookie => (
              <div key={`${cookie.name}-${cookie.domain}-${cookie.path}`} className="p-2.5 border border-[var(--color-border-default)] rounded-lg">
                <div className="flex items-center gap-2 mb-1.5">
                  <span className="font-semibold text-[var(--color-text-default)]">{cookie.name}</span>
                  <span className="text-[var(--color-text-muted)] text-[11px]">{cookie.domain}</span>
                  {cookie.secure && <Badge variant="default" className="text-[10px]">Secure</Badge>}
                  {cookie.httpOnly && <Badge variant="default" className="text-[10px]">HttpOnly</Badge>}
                  {cookie.session && <Badge variant="success" className="text-[10px]">Session</Badge>}
                  {cookie.sameSite && <Badge variant="default" className="text-[10px]">{cookie.sameSite}</Badge>}
                  <div className="ml-auto flex gap-1">
                    <Button
                      size="sm"
                      variant="secondary"
                      onClick={() => setSelectedCookie(cookie)}
                      className="text-[10px] px-2 py-0.5"
                    >
                      查看
                    </Button>
                    <Button
                      size="sm"
                      variant="danger"
                      onClick={() => handleDeleteCookie(cookie.name, cookie.domain, cookie.path)}
                      className="text-[10px] px-2 py-0.5"
                    >
                      删除
                    </Button>
                  </div>
                </div>
                <div className="text-[11px] text-[var(--color-text-muted)]">
                  路径: {cookie.path || '/'}
                </div>
                <div className="text-[11px] text-[var(--color-text-muted)] truncate">
                  值: {cookie.value || '(空)'}
                </div>
                {!cookie.session && cookie.expires > 0 && (
                  <div className="text-[11px] text-[var(--color-text-muted)]">
                    过期: {new Date(cookie.expires * 1000).toLocaleString()}
                  </div>
                )}
                <div className="text-[11px] text-[var(--color-text-muted)]">
                  大小: {cookie.size} 字节
                </div>
              </div>
            ))}
          </div>
        </Card>
      )}

      {/* Cookie 详情/编辑模态框 */}
      {selectedCookie && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={() => setSelectedCookie(null)}>
          <div className="bg-[var(--color-bg-default)] rounded-lg shadow-xl max-w-2xl w-full max-h-[80vh] overflow-y-auto m-4" onClick={e => e.stopPropagation()}>
            <div className="p-4 border-b border-[var(--color-border-default)] flex items-center justify-between">
              <h3 className="text-sm font-semibold">Cookie 详情</h3>
              <div className="flex gap-2">
                <Button
                  size="sm"
                  variant="primary"
                  onClick={() => {
                    setEditingCookie({ ...selectedCookie })
                  }}
                >
                  编辑
                </Button>
                <Button size="sm" variant="secondary" onClick={() => setSelectedCookie(null)}>关闭</Button>
              </div>
            </div>
            <div className="p-4 space-y-3 text-xs">
              <div>
                <div className="font-semibold mb-1">名称</div>
                <div className="p-2 bg-[var(--color-bg-subtle)] rounded font-mono">{selectedCookie.name}</div>
              </div>
              <div>
                <div className="font-semibold mb-1">值</div>
                <div className="p-2 bg-[var(--color-bg-subtle)] rounded font-mono break-all">{selectedCookie.value}</div>
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <div className="font-semibold mb-1">域名</div>
                  <div className="p-2 bg-[var(--color-bg-subtle)] rounded">{selectedCookie.domain}</div>
                </div>
                <div>
                  <div className="font-semibold mb-1">路径</div>
                  <div className="p-2 bg-[var(--color-bg-subtle)] rounded">{selectedCookie.path || '/'}</div>
                </div>
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <div className="font-semibold mb-1">过期时间</div>
                  <div className="p-2 bg-[var(--color-bg-subtle)] rounded">
                    {selectedCookie.session ? 'Session' : selectedCookie.expires > 0 ? new Date(selectedCookie.expires * 1000).toLocaleString() : '永久'}
                  </div>
                </div>
                <div>
                  <div className="font-semibold mb-1">大小</div>
                  <div className="p-2 bg-[var(--color-bg-subtle)] rounded">{selectedCookie.size} 字节</div>
                </div>
              </div>
              <div className="grid grid-cols-3 gap-3">
                <div>
                  <div className="font-semibold mb-1">Secure</div>
                  <div className="p-2 bg-[var(--color-bg-subtle)] rounded">{selectedCookie.secure ? '是' : '否'}</div>
                </div>
                <div>
                  <div className="font-semibold mb-1">HttpOnly</div>
                  <div className="p-2 bg-[var(--color-bg-subtle)] rounded">{selectedCookie.httpOnly ? '是' : '否'}</div>
                </div>
                <div>
                  <div className="font-semibold mb-1">SameSite</div>
                  <div className="p-2 bg-[var(--color-bg-subtle)] rounded">{selectedCookie.sameSite || 'None'}</div>
                </div>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Cookie 编辑模态框 */}
      {editingCookie && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={() => setEditingCookie(null)}>
          <div className="bg-[var(--color-bg-default)] rounded-lg shadow-xl max-w-2xl w-full max-h-[80vh] overflow-y-auto m-4" onClick={e => e.stopPropagation()}>
            <div className="p-4 border-b border-[var(--color-border-default)] flex items-center justify-between">
              <h3 className="text-sm font-semibold">编辑 Cookie</h3>
              <div className="flex gap-2">
                <Button
                  size="sm"
                  variant="primary"
                  onClick={handleSaveCookie}
                >
                  保存
                </Button>
                <Button size="sm" variant="secondary" onClick={() => setEditingCookie(null)}>取消</Button>
              </div>
            </div>
            <div className="p-4 space-y-3 text-xs">
              <div>
                <label className="font-semibold mb-1 block">名称</label>
                <Input
                  value={editingCookie.name}
                  onChange={e => setEditingCookie({ ...editingCookie, name: e.target.value })}
                  placeholder="Cookie 名称"
                />
              </div>
              <div>
                <label className="font-semibold mb-1 block">值</label>
                <Input
                  value={editingCookie.value}
                  onChange={e => setEditingCookie({ ...editingCookie, value: e.target.value })}
                  placeholder="Cookie 值"
                />
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="font-semibold mb-1 block">域名</label>
                  <Input
                    value={editingCookie.domain}
                    onChange={e => setEditingCookie({ ...editingCookie, domain: e.target.value })}
                    placeholder=".example.com"
                  />
                </div>
                <div>
                  <label className="font-semibold mb-1 block">路径</label>
                  <Input
                    value={editingCookie.path}
                    onChange={e => setEditingCookie({ ...editingCookie, path: e.target.value })}
                    placeholder="/"
                  />
                </div>
              </div>
              <div>
                <label className="font-semibold mb-1 block">过期时间 (Unix 时间戳秒)</label>
                <Input
                  type="number"
                  value={editingCookie.expires}
                  onChange={e => setEditingCookie({ ...editingCookie, expires: parseFloat(e.target.value) || 0 })}
                  placeholder="留空表示 Session Cookie"
                />
              </div>
              <div className="grid grid-cols-3 gap-3">
                <div className="flex items-center gap-2">
                  <input
                    type="checkbox"
                    id="edit-secure"
                    checked={editingCookie.secure}
                    onChange={e => setEditingCookie({ ...editingCookie, secure: e.target.checked })}
                  />
                  <label htmlFor="edit-secure" className="font-semibold">Secure</label>
                </div>
                <div className="flex items-center gap-2">
                  <input
                    type="checkbox"
                    id="edit-httponly"
                    checked={editingCookie.httpOnly}
                    onChange={e => setEditingCookie({ ...editingCookie, httpOnly: e.target.checked })}
                  />
                  <label htmlFor="edit-httponly" className="font-semibold">HttpOnly</label>
                </div>
                <div>
                  <label className="font-semibold mb-1 block">SameSite</label>
                  <Select
                    value={editingCookie.sameSite || 'None'}
                    onChange={e => setEditingCookie({ ...editingCookie, sameSite: e.target.value })}
                    options={[
                      { value: 'None', label: 'None' },
                      { value: 'Lax', label: 'Lax' },
                      { value: 'Strict', label: 'Strict' }
                    ]}
                  />
                </div>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* 拦截规则 */}
      {activeTool === 'intercept' && (
        <Card title="请求拦截">
          <div className="flex gap-2 mb-3 items-center flex-wrap">
            <Button
              size="sm"
              variant={interceptEnabled ? 'danger' : 'primary'}
              onClick={handleToggleIntercept}
            >
              {interceptEnabled ? '禁用拦截' : '启用拦截'}
            </Button>
            <Button size="sm" variant="secondary" onClick={handleAddRule}>添加规则</Button>
            <Button size="sm" variant="secondary" onClick={loadInterceptRules}>刷新</Button>
            {interceptEnabled && (
              <Badge variant="success" className="ml-2">拦截已启用</Badge>
            )}
          </div>
          <div className="space-y-2 text-xs max-h-[480px] overflow-y-auto">
            {interceptRules.length === 0 ? (
              <p className="text-center py-8 text-[var(--color-text-muted)]">无拦截规则</p>
            ) : interceptRules.map(rule => (
              <div key={rule.id} className="p-3 border border-[var(--color-border-default)] rounded-lg">
                <div className="flex items-center gap-2 mb-2">
                  <input
                    type="checkbox"
                    checked={rule.enabled}
                    onChange={() => handleToggleRule(rule)}
                    className="cursor-pointer"
                  />
                  <span className="font-semibold text-[var(--color-text-default)]">{rule.name}</span>
                  {rule.actions.block && <Badge variant="error" className="text-[10px]">阻止</Badge>}
                  {rule.actions.modifyRequest && <Badge variant="warning" className="text-[10px]">修改请求</Badge>}
                  {rule.actions.modifyResponse && <Badge variant="warning" className="text-[10px]">修改响应</Badge>}
                  {rule.actions.delay > 0 && <Badge variant="default" className="text-[10px]">延迟 {rule.actions.delay}ms</Badge>}
                  <div className="ml-auto flex gap-1">
                    <Button
                      size="sm"
                      variant="secondary"
                      onClick={() => handleEditRule(rule)}
                      className="text-[10px] px-2 py-0.5"
                    >
                      编辑
                    </Button>
                    <Button
                      size="sm"
                      variant="danger"
                      onClick={() => handleDeleteRule(rule.id)}
                      className="text-[10px] px-2 py-0.5"
                    >
                      删除
                    </Button>
                  </div>
                </div>
                <div className="text-[11px] text-[var(--color-text-muted)] space-y-1">
                  <div>URL 匹配: {rule.urlPattern}</div>
                  {rule.method && <div>方法: {rule.method}</div>}
                </div>
              </div>
            ))}
          </div>
        </Card>
      )}

      {/* 规则编辑器模态框 */}
      {showRuleEditor && editingRule && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={() => setShowRuleEditor(false)}>
          <div className="bg-[var(--color-bg-default)] rounded-lg shadow-xl max-w-2xl w-full max-h-[80vh] overflow-y-auto m-4" onClick={e => e.stopPropagation()}>
            <div className="p-4 border-b border-[var(--color-border-default)] flex items-center justify-between">
              <h3 className="text-sm font-semibold">编辑拦截规则</h3>
              <div className="flex gap-2">
                <Button size="sm" variant="primary" onClick={handleSaveRule}>保存</Button>
                <Button size="sm" variant="secondary" onClick={() => setShowRuleEditor(false)}>取消</Button>
              </div>
            </div>
            <div className="p-4 space-y-3 text-xs">
              <div>
                <label className="font-semibold mb-1 block">规则名称</label>
                <Input
                  value={editingRule.name}
                  onChange={e => setEditingRule({ ...editingRule, name: e.target.value })}
                  placeholder="规则名称"
                />
              </div>
              <div>
                <label className="font-semibold mb-1 block">URL 匹配模式（支持通配符 *）</label>
                <Input
                  value={editingRule.urlPattern}
                  onChange={e => setEditingRule({ ...editingRule, urlPattern: e.target.value })}
                  placeholder="例如: *api.example.com* 或 https://example.com/*"
                />
              </div>
              <div>
                <label className="font-semibold mb-1 block">HTTP 方法（留空表示全部）</label>
                <Input
                  value={editingRule.method}
                  onChange={e => setEditingRule({ ...editingRule, method: e.target.value })}
                  placeholder="GET, POST, PUT, DELETE 等"
                />
              </div>
              <div className="border-t border-[var(--color-border-default)] pt-3 mt-3">
                <div className="font-semibold mb-2">拦截动作</div>
                <div className="space-y-2">
                  <div className="flex items-center gap-2">
                    <input
                      type="checkbox"
                      id="rule-block"
                      checked={editingRule.actions.block}
                      onChange={e => setEditingRule({
                        ...editingRule,
                        actions: { ...editingRule.actions, block: e.target.checked }
                      })}
                    />
                    <label htmlFor="rule-block" className="font-semibold">阻止请求</label>
                  </div>
                  <div className="flex items-center gap-2">
                    <input
                      type="checkbox"
                      id="rule-modify-req"
                      checked={editingRule.actions.modifyRequest}
                      onChange={e => setEditingRule({
                        ...editingRule,
                        actions: { ...editingRule.actions, modifyRequest: e.target.checked }
                      })}
                    />
                    <label htmlFor="rule-modify-req" className="font-semibold">修改请求（暂不支持）</label>
                  </div>
                  <div className="flex items-center gap-2">
                    <input
                      type="checkbox"
                      id="rule-modify-resp"
                      checked={editingRule.actions.modifyResponse}
                      onChange={e => setEditingRule({
                        ...editingRule,
                        actions: { ...editingRule.actions, modifyResponse: e.target.checked }
                      })}
                      disabled
                    />
                    <label htmlFor="rule-modify-resp" className="font-semibold text-[var(--color-text-muted)]">修改响应（暂不支持）</label>
                  </div>
                  <div>
                    <label className="font-semibold mb-1 block">延迟（毫秒，0 表示无延迟）</label>
                    <Input
                      type="number"
                      value={editingRule.actions.delay}
                      onChange={e => setEditingRule({
                        ...editingRule,
                        actions: { ...editingRule.actions, delay: parseInt(e.target.value) || 0 }
                      })}
                      placeholder="0"
                      disabled
                    />
                    <p className="text-[10px] text-[var(--color-text-muted)] mt-1">延迟功能暂不支持</p>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
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

      {/* 请求详情 */}
      <Modal open={!!selectedReq} onClose={() => setSelectedReq(null)} title="请求详情" width="760px">
        {selectedReq && (
          <div className="space-y-3">
            <div className="flex gap-2 flex-wrap border-b border-[var(--color-border-muted)] pb-2">
              {([
                { k: 'general', label: '概要' },
                { k: 'reqHeaders', label: '请求头' },
                { k: 'respHeaders', label: '响应头' },
                { k: 'response', label: '响应体' },
                { k: 'timing', label: '计时' },
              ] as const).map(t => (
                <Button key={t.k} size="sm" variant={reqTab === t.k ? undefined : 'secondary'} onClick={() => setReqTab(t.k)}>{t.label}</Button>
              ))}
              <div className="ml-auto flex gap-2">
                <Button size="sm" variant="ghost" onClick={() => replayRequest(selectedReq)} loading={replayLoading}>🔁 重放</Button>
                <Button size="sm" variant="ghost" onClick={() => copyCurl(selectedReq)}>复制为 cURL</Button>
                <Button size="sm" variant="ghost" onClick={() => copyText(selectedReq.url, 'URL')}>复制 URL</Button>
              </div>
            </div>
            {replayResult && (
              <div className="mb-3 p-3 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded">
                <div className="text-sm font-medium text-blue-900 dark:text-blue-100 mb-2">重放结果</div>
                <div className="text-xs space-y-1">
                  <div className="flex gap-2">
                    <span className="text-[var(--color-text-muted)]">状态:</span>
                    <span className={replayResult.status >= 200 && replayResult.status < 400 ? 'text-green-600' : 'text-red-600'}>
                      {replayResult.status} {replayResult.statusText}
                    </span>
                  </div>
                  <div className="flex gap-2">
                    <span className="text-[var(--color-text-muted)]">耗时:</span>
                    <span className={getDurationColor(replayResult.time)}>{fmtDuration(replayResult.time)}</span>
                  </div>
                  {replayResult.body && (
                    <details className="mt-2">
                      <summary className="cursor-pointer text-[var(--color-accent)] hover:underline">查看响应体</summary>
                      <pre className="mt-2 p-2 bg-[var(--color-bg-subtle)] rounded text-[11px] whitespace-pre-wrap break-all max-h-40 overflow-y-auto">
                        {replayResult.body}
                      </pre>
                    </details>
                  )}
                </div>
              </div>
            )}
            <div className="text-xs max-h-[420px] overflow-y-auto">
              {reqTab === 'general' && (
                <div className="space-y-1.5">
                  {[
                    ['URL', selectedReq.url],
                    ['方法', selectedReq.method],
                    ['状态', `${selectedReq.statusCode || '-'} ${selectedReq.statusText || ''}`],
                    ['类型', selectedReq.type || '-'],
                    ['MIME', selectedReq.mimeType || '-'],
                    ['大小', fmtSize(selectedReq.size)],
                  ].map(([k, val]) => (
                    <div key={k} className="flex gap-2"><span className="text-[var(--color-text-muted)] w-16 shrink-0">{k}</span><span className="break-all font-mono">{val}</span></div>
                  ))}
                  <div className="flex gap-2">
                    <span className="text-[var(--color-text-muted)] w-16 shrink-0">耗时</span>
                    <span className={`break-all font-mono font-medium ${getDurationColor(selectedReq.duration)}`}>
                      {fmtDuration(selectedReq.duration)}
                    </span>
                  </div>
                </div>
              )}
              {reqTab === 'reqHeaders' && <HeaderList headers={selectedReq.requestHeaders} body={selectedReq.requestBody} />}
              {reqTab === 'respHeaders' && <HeaderList headers={selectedReq.responseHeaders} />}
              {reqTab === 'response' && (
                <div>
                  {selectedReq.truncated && (
                    <div className="mb-2 p-2 bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded text-xs text-yellow-800 dark:text-yellow-200">
                      ⚠️ 响应体超过 10MB，已自动截断显示
                    </div>
                  )}
                  <ResponseViewer
                    data={selectedReq.parsedData || null}
                    responseBody={selectedReq.responseBody}
                    mimeType={selectedReq.mimeType}
                  />
                </div>
              )}
              {reqTab === 'timing' && (
                <div className="space-y-3">
                  {!selectedReq.timing ? (
                    <p className="text-[var(--color-text-muted)]">无详细 timing 信息（可能是缓存响应或旧版数据）</p>
                  ) : (
                    <>
                      <div className="space-y-1.5">
                        <div className="font-semibold mb-2">基础信息</div>
                        <div className="flex gap-2"><span className="text-[var(--color-text-muted)] w-28 shrink-0">请求开始时间</span><span className="font-mono">{selectedReq.timing.requestTime.toFixed(3)}s</span></div>
                        <div className="flex gap-2">
                          <span className="text-[var(--color-text-muted)] w-28 shrink-0">总耗时</span>
                          <span className={`font-mono font-medium ${getDurationColor(selectedReq.duration)}`}>
                            {fmtDuration(selectedReq.duration)}
                          </span>
                        </div>
                      </div>

                      <div className="space-y-1.5">
                        <div className="font-semibold mb-2">详细阶段（毫秒）</div>
                        {[
                          { label: '代理协商', start: selectedReq.timing.proxyStart, end: selectedReq.timing.proxyEnd },
                          { label: 'DNS 查询', start: selectedReq.timing.dnsStart, end: selectedReq.timing.dnsEnd },
                          { label: 'TCP 连接', start: selectedReq.timing.connectStart, end: selectedReq.timing.connectEnd },
                          { label: 'SSL 握手', start: selectedReq.timing.sslStart, end: selectedReq.timing.sslEnd },
                          { label: '发送请求', start: selectedReq.timing.sendStart, end: selectedReq.timing.sendEnd },
                          { label: 'HTTP/2 Push', start: selectedReq.timing.pushStart, end: selectedReq.timing.pushEnd },
                        ].map(phase => {
                          const duration = phase.start >= 0 && phase.end >= 0 ? phase.end - phase.start : -1
                          const exists = phase.start >= 0 || phase.end >= 0
                          return (
                            <div key={phase.label} className={`flex gap-2 ${!exists ? 'opacity-50' : ''}`}>
                              <span className="text-[var(--color-text-muted)] w-28 shrink-0">{phase.label}</span>
                              <span className="font-mono">
                                {!exists ? '- (未发生)' : duration >= 0 ? `${duration.toFixed(2)}ms` : `开始: ${phase.start.toFixed(2)}ms`}
                              </span>
                            </div>
                          )
                        })}
                        <div className="flex gap-2">
                          <span className="text-[var(--color-text-muted)] w-28 shrink-0">接收响应头</span>
                          <span className="font-mono">
                            {selectedReq.timing.receiveHeadersEnd >= 0 ? `${selectedReq.timing.receiveHeadersEnd.toFixed(2)}ms` : '-'}
                          </span>
                        </div>
                      </div>

                      {/* 可视化时间线 */}
                      <div className="mt-4">
                        <div className="font-semibold mb-2">时间线可视化</div>
                        <div className="space-y-1">
                          {(() => {
                            const timing = selectedReq.timing
                            const maxTime = Math.max(
                              timing.receiveHeadersEnd,
                              timing.connectEnd,
                              timing.dnsEnd,
                              timing.sslEnd,
                              timing.sendEnd
                            )
                            const scale = maxTime > 0 ? 100 / maxTime : 0

                            const phases = [
                              { label: 'DNS', start: timing.dnsStart, end: timing.dnsEnd, color: 'bg-blue-400' },
                              { label: 'TCP', start: timing.connectStart, end: timing.connectEnd, color: 'bg-green-400' },
                              { label: 'SSL', start: timing.sslStart, end: timing.sslEnd, color: 'bg-purple-400' },
                              { label: '发送', start: timing.sendStart, end: timing.sendEnd, color: 'bg-yellow-400' },
                              { label: '响应头', start: timing.sendEnd >= 0 ? timing.sendEnd : 0, end: timing.receiveHeadersEnd, color: 'bg-orange-400' },
                            ].filter(p => p.start >= 0 && p.end >= 0 && p.end > p.start)

                            return phases.map(phase => {
                              const left = phase.start * scale
                              const width = (phase.end - phase.start) * scale
                              return (
                                <div key={phase.label} className="flex items-center gap-2 text-[11px]">
                                  <span className="w-12 text-[var(--color-text-muted)] shrink-0">{phase.label}</span>
                                  <div className="flex-1 relative h-4 bg-[var(--color-bg-elevated)] dark:bg-gray-800 rounded">
                                    <div
                                      className={`absolute h-full ${phase.color} rounded`}
                                      style={{ left: `${left}%`, width: `${width}%` }}
                                      title={`${phase.label}: ${(phase.end - phase.start).toFixed(2)}ms`}
                                    />
                                  </div>
                                  <span className="w-16 font-mono text-right">{(phase.end - phase.start).toFixed(2)}ms</span>
                                </div>
                              )
                            })
                          })()}
                        </div>
                      </div>
                    </>
                  )}
                </div>
              )}
            </div>
          </div>
        )}
      </Modal>
    </div>
  )
}

function HeaderList({ headers, body }: { headers: Record<string, string>; body?: string }) {
  const entries = Object.entries(headers || {})
  return (
    <div className="space-y-1">
      {entries.length === 0 ? (
        <p className="text-[var(--color-text-muted)]">（无）</p>
      ) : entries.map(([k, v]) => (
        <div key={k} className="flex gap-2 break-all"><span className="font-medium text-[var(--color-accent)] shrink-0">{k}:</span><span className="font-mono">{v}</span></div>
      ))}
      {body && (
        <div className="mt-2 pt-2 border-t border-[var(--color-border-muted)]">
          <div className="text-[var(--color-text-muted)] mb-1">请求体</div>
          <pre className="whitespace-pre-wrap break-all font-mono bg-[var(--color-bg-subtle)] p-2 rounded">{body}</pre>
        </div>
      )}
    </div>
  )
}

// 获取数据类型图标
function getTypeIcon(type: string): string {
  const icons: Record<string, string> = {
    all: '🌐',
    json: '📋',
    xml: '📄',
    html: '🌐',
    image: '🖼️',
    video: '🎬',
    audio: '🎵',
    javascript: '📜',
    css: '🎨',
    graphql: '🔷',
    form: '📝',
    binary: '🔢',
    text: '📝',
  }
  return icons[type] || '📦'
}

// 获取数据类型标签
function getTypeLabel(type: string): string {
  const labels: Record<string, string> = {
    all: '全部',
    json: 'JSON',
    xml: 'XML',
    html: 'HTML',
    image: '图片',
    video: '视频',
    audio: '音频',
    javascript: 'JS',
    css: 'CSS',
    graphql: 'GraphQL',
    form: '表单',
    binary: '二进制',
    text: '文本',
  }
  return labels[type] || type.toUpperCase()
}
