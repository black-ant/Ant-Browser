export interface DashboardStats {
  totalInstances: number
  runningInstances: number
  proxyCount: number
  proxyAvailable: number
  coreCount: number
  memUsedMB: number
  maxProfileLimit: number
  appVersion: string
}

export interface ActivityLog {
  id: string
  timestamp: number
  type: 'start' | 'stop' | 'create' | 'delete' | 'update' | 'config'
  message: string
  profileName?: string
  icon?: string
}

export interface ResourceStats {
  cpuUsage: number
  memoryUsage: number
  memoryTotal: number
  activeConnections: number
  diskUsage?: number
}

// 真实资源采样（后端 gopsutil + runtime）
export interface MetricSample {
  timestamp: number
  cpuPercent: number
  memUsedMB: number
  memTotalMB: number
  appMemMB: number
  runningInstances: number
}

export interface DashboardMetrics {
  live: MetricSample
  history: MetricSample[]
}

// 真实活动事件
export interface ActivityEntry {
  timestamp: number // unix 秒
  type: string      // start/stop/crash/import/speedtest/config
  level: string     // info/warn/error
  message: string
  profileName?: string
}
