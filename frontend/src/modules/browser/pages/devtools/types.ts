// BrowserDevToolsPage 的类型定义（从页面提取，纯类型，零运行时风险）。

export type ToolType = 'network' | 'console' | 'storage' | 'screenshot' | 'javascript' | 'performance'

export interface NetworkRequest {
  requestId: string
  url: string
  method: string
  statusCode?: number
  statusText?: string
  type: string
  timestamp: number
  requestHeaders?: Record<string, string>
  responseHeaders?: Record<string, string>
  requestBody?: string
  responseBody?: string
  responseBodyLoading?: boolean
  responseBodyError?: boolean
  duration?: number
  size?: number
  mimeType?: string
  isApi?: boolean
}

export interface ConsoleLog {
  id: string
  type: 'log' | 'warn' | 'error' | 'info'
  message: string
  timestamp: number
  stackTrace?: string
}

export interface StorageItem {
  key: string
  value: string
  type: 'localStorage' | 'sessionStorage'
}

export interface Statistics {
  total: number
  success: number
  failed: number
  pending: number
  totalSize: number
  avgDuration: number
}
