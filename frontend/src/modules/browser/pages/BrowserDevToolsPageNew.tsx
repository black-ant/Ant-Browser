import { useEffect, useState } from 'react'
import { Badge, Button, Card, Select, toast } from '../../../shared/components'
import type { BrowserProfile } from '../types'
import {
  fetchBrowserProfiles,
  CDPSessionCreate,
  CDPSessionClose,
  CDPGetNetworkRequests,
  CDPGetConsoleLogs,
  CDPClearNetworkRequests,
  CDPExportHAR,
  type CDPNetworkRequest,
  type CDPConsoleLog,
} from '../api'

export function BrowserDevToolsPageNew() {
  const [profiles, setProfiles] = useState<BrowserProfile[]>([])
  const [selectedProfileId, setSelectedProfileId] = useState('')
  const [sessionId, setSessionId] = useState<string | null>(null)
  const [capturing, setCapturing] = useState(false)
  const [activeTool, setActiveTool] = useState<'network' | 'console'>('network')

  // 数据
  const [requests, setRequests] = useState<CDPNetworkRequest[]>([])
  const [consoleLogs, setConsoleLogs] = useState<CDPConsoleLog[]>([])
  const [pollingInterval, setPollingInterval] = useState<ReturnType<typeof setInterval> | null>(null)

  useEffect(() => {
    loadProfiles()
    return () => {
      if (pollingInterval) clearInterval(pollingInterval)
      if (sessionId) CDPSessionClose(sessionId).catch(console.error)
    }
  }, [])

  const loadProfiles = async () => {
    try {
      const list = await fetchBrowserProfiles()
      setProfiles(list.filter(p => p.running && p.debugReady))
    } catch (error) {
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
      toast.success('开发工具已连接')

      // 开始轮询
      const interval = setInterval(async () => {
        try {
          const [networkData, consoleData] = await Promise.all([
            CDPGetNetworkRequests(newSessionId),
            CDPGetConsoleLogs(newSessionId)
          ])
          setRequests(networkData)
          setConsoleLogs(consoleData)
        } catch (error) {
          console.error('轮询数据失败:', error)
        }
      }, 1000)

      setPollingInterval(interval)
    } catch (error: any) {
      toast.error(error?.message || '连接失败')
      setCapturing(false)
    }
  }

  const stopCapture = async () => {
    if (pollingInterval) {
      clearInterval(pollingInterval)
      setPollingInterval(null)
    }

    if (sessionId) {
      try {
        await CDPSessionClose(sessionId)
      } catch (error) {
        console.error('关闭会话失败:', error)
      }
      setSessionId(null)
    }

    setCapturing(false)
    toast.success('已停止抓包')
  }

  const handleClearRequests = async () => {
    if (sessionId) {
      try {
        await CDPClearNetworkRequests(sessionId)
        setRequests([])
        toast.success('已清空')
      } catch (error) {
        toast.error('清空失败')
      }
    } else {
      setRequests([])
    }
  }

  const handleExportHAR = async () => {
    if (!sessionId) {
      toast.error('请先启动抓包')
      return
    }

    try {
      const harData = await CDPExportHAR(sessionId)
      const blob = new Blob([harData], { type: 'application/json' })
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = `network-${Date.now()}.har`
      link.click()
      URL.revokeObjectURL(url)
      toast.success('HAR已导出')
    } catch (error: any) {
      toast.error(error?.message || '导出失败')
    }
  }

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">开发工具</h1>
      </div>

      {/* 控制栏 */}
      <Card className="p-4">
        <div className="flex items-center gap-4">
          <Select
            value={selectedProfileId}
            onChange={(e) => setSelectedProfileId(e.target.value)}
            disabled={capturing}
            className="flex-1"
            options={[]}
          >
            <option value="">选择浏览器实例</option>
            {profiles.map(p => (
              <option key={p.profileId} value={p.profileId}>
                {p.profileName} (端口: {p.debugPort})
              </option>
            ))}
          </Select>

          <Button
            onClick={capturing ? stopCapture : startCapture}
            variant={capturing ? 'danger' : 'primary'}
          >
            {capturing ? '停止' : '开始'}
          </Button>

          <Button onClick={loadProfiles}>
            刷新
          </Button>
        </div>
      </Card>

      {/* 工具栏 */}
      <div className="flex gap-2">
        <Button
          variant={activeTool === 'network' ? 'primary' : 'secondary'}
          onClick={() => setActiveTool('network')}
        >
          网络 ({requests.length})
        </Button>
        <Button
          variant={activeTool === 'console' ? 'primary' : 'secondary'}
          onClick={() => setActiveTool('console')}
        >
          Console ({consoleLogs.length})
        </Button>
      </div>

      {/* 操作按钮 */}
      {activeTool === 'network' && (
        <div className="flex gap-2">
          <Button onClick={handleClearRequests}>
            清空
          </Button>
          <Button onClick={handleExportHAR}>
            导出HAR
          </Button>
        </div>
      )}

      {/* 网络面板 */}
      {activeTool === 'network' && (
        <Card className="p-4">
          <div className="space-y-2">
            {requests.length === 0 ? (
              <p className="text-gray-500 text-center py-8">暂无网络请求</p>
            ) : (
              requests.map((req) => (
                <div key={req.requestId} className="p-3 border rounded hover:bg-gray-50">
                  <div className="flex items-center justify-between">
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2">
                        <Badge variant={req.statusCode >= 200 && req.statusCode < 300 ? 'success' : 'error'}>
                          {req.statusCode || 'Pending'}
                        </Badge>
                        <span className="font-mono text-sm">{req.method}</span>
                        <span className="truncate text-sm">{req.url}</span>
                      </div>
                      <div className="text-xs text-gray-500 mt-1">
                        {req.type} • {req.size ? `${(req.size / 1024).toFixed(2)} KB` : '-'} • {req.duration ? `${req.duration}ms` : '-'}
                      </div>
                    </div>
                  </div>
                </div>
              ))
            )}
          </div>
        </Card>
      )}

      {/* Console面板 */}
      {activeTool === 'console' && (
        <Card className="p-4">
          <div className="space-y-2 font-mono text-sm">
            {consoleLogs.length === 0 ? (
              <p className="text-gray-500 text-center py-8">暂无日志</p>
            ) : (
              consoleLogs.map((log) => (
                <div
                  key={log.id}
                  className={`p-2 rounded ${
                    log.type === 'error' ? 'bg-red-50 text-red-700' :
                    log.type === 'warn' ? 'bg-yellow-50 text-yellow-700' :
                    'bg-gray-50'
                  }`}
                >
                  <span className="font-bold">[{log.type}]</span> {log.message}
                  {log.stackTrace && (
                    <pre className="mt-1 text-xs opacity-70">{log.stackTrace}</pre>
                  )}
                </div>
              ))
            )}
          </div>
        </Card>
      )}
    </div>
  )
}
