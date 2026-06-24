// 重构说明：BrowserDevToolsPage.tsx 需要的关键修改

// ========== 1. 导入部分修改 ==========
// 添加CDP API导入
import {
  fetchBrowserProfiles,
  CDPSessionCreate,
  CDPSessionClose,
  CDPGetNetworkRequests,
  CDPGetConsoleLogs,
  CDPClearNetworkRequests,
  CDPClearConsoleLogs,
  CDPExecuteJavaScript,
  CDPCaptureScreenshot,
  CDPExportHAR,
  CDPGetStatistics,
  type CDPNetworkRequest,
  type CDPConsoleLog,
} from '../api'

// ========== 2. State修改 ==========
// 删除：
// const [ws, setWs] = useState<WebSocket | null>(null)

// 添加：
const [sessionId, setSessionId] = useState<string | null>(null)
const [pollingInterval, setPollingInterval] = useState<NodeJS.Timeout | null>(null)

// 修改NetworkRequest类型使用CDPNetworkRequest：
const [requests, setRequests] = useState<CDPNetworkRequest[]>([])
const [selectedRequest, setSelectedRequest] = useState<CDPNetworkRequest | null>(null)

// 修改ConsoleLog类型使用CDPConsoleLog：
const [consoleLogs, setConsoleLogs] = useState<CDPConsoleLog[]>([])

// ========== 3. 删除整个WebSocket相关函数 ==========
// 删除以下所有函数：
// - startCapture (WebSocket版本)
// - handleNetworkEvent
// - handleConsoleEvent  
// - handlePerformanceEvent
// - stopCapture (WebSocket版本)

// ========== 4. 添加新的CDP函数 ==========

const startCapture = async () => {
  if (!selectedProfileId) {
    toast.error('请选择一个运行中的浏览器实例')
    return
  }

  try {
    // 创建CDP会话
    const newSessionId = await CDPSessionCreate(selectedProfileId, 'page')
    setSessionId(newSessionId)
    setCapturing(true)
    setRequests([])
    setConsoleLogs([])

    toast.success('开发工具已连接')

    // 开始轮询数据
    const interval = setInterval(async () => {
      try {
        if (activeTool === 'network') {
          const networkRequests = await CDPGetNetworkRequests(newSessionId)
          setRequests(networkRequests)
        }
        
        if (activeTool === 'console') {
          const logs = await CDPGetConsoleLogs(newSessionId)
          setConsoleLogs(logs)
        }

        // 获取统计信息
        const stats = await CDPGetStatistics(newSessionId)
        if (stats) {
          // 更新统计信息
        }
      } catch (error) {
        console.error('轮询数据失败:', error)
      }
    }, 1000) // 每秒轮询一次

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
  if (!sessionId) {
    setRequests([])
    return
  }

  try {
    await CDPClearNetworkRequests(sessionId)
    setRequests([])
    toast.success('已清空网络请求')
  } catch (error: any) {
    toast.error('清空失败')
  }
}

const handleClearConsoleLogs = async () => {
  if (!sessionId) {
    setConsoleLogs([])
    return
  }

  try {
    await CDPClearConsoleLogs(sessionId)
    setConsoleLogs([])
    toast.success('已清空Console日志')
  } catch (error: any) {
    toast.error('清空失败')
  }
}

const handleExecuteJS = async () => {
  if (!sessionId) {
    toast.error('请先启动抓包')
    return
  }

  try {
    const result = await CDPExecuteJavaScript(sessionId, jsCode)
    setJsResult(result)
    toast.success('执行成功')
  } catch (error: any) {
    toast.error(error?.message || '执行失败')
  }
}

const handleCaptureScreenshot = async () => {
  if (!sessionId) {
    toast.error('请先启动抓包')
    return
  }

  try {
    const base64Image = await CDPCaptureScreenshot(sessionId)
    // 下载图片
    const link = document.createElement('a')
    link.href = `data:image/png;base64,${base64Image}`
    link.download = `screenshot-${Date.now()}.png`
    link.click()
    toast.success('截图已保存')
  } catch (error: any) {
    toast.error(error?.message || '截图失败')
  }
}

const handleExportHAR = async () => {
  if (!sessionId) {
    toast.error('请先启动抓包')
    return
  }

  try {
    const harData = await CDPExportHAR(sessionId)
    // 下载HAR文件
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

// ========== 5. 修改useEffect清理逻辑 ==========
useEffect(() => {
  loadProfiles()
  return () => {
    // 清理轮询
    if (pollingInterval) {
      clearInterval(pollingInterval)
    }
    // 关闭会话
    if (sessionId) {
      CDPSessionClose(sessionId).catch(console.error)
    }
  }
}, [])

// ========== 6. 界面按钮调用修改 ==========
// "清空请求"按钮 → onClick={handleClearRequests}
// "清空日志"按钮 → onClick={handleClearConsoleLogs}
// "执行"按钮 → onClick={handleExecuteJS}
// "截图"按钮 → onClick={handleCaptureScreenshot}
// "导出HAR"按钮 → onClick={handleExportHAR}

// ========== 完成！==========
// 这样重构后，DevTools页面将：
// ✅ 不再直接管理WebSocket连接
// ✅ 使用后端CDP服务
// ✅ 数据持久化，刷新不丢
// ✅ 自动重连由后端处理
// ✅ HAR导出后端统一生成
