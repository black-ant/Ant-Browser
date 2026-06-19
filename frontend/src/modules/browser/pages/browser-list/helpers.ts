// BrowserListPage 的纯展示辅助函数（从页面提取，无副作用、便于复用与测试）。

export type StatusVariant = 'info' | 'default' | 'success' | 'warning' | 'error'

// resolveProfileStatus 根据运行/调试/本地待定状态与后端状态机推导展示用的状态标签。
export function resolveProfileStatus(
  running: boolean,
  debugReady: boolean,
  starting: boolean,
  stopping: boolean,
  status?: string,
): { variant: StatusVariant; label: string } {
  // 本地操作中的即时反馈优先（点击启动/停止后立即生效，无需等待后端事件）
  if (starting) return { variant: 'info', label: '启动中' }
  if (stopping) return { variant: 'default', label: '停止中' }
  // 其次使用后端权威状态机
  switch (status) {
    case 'starting':
      return { variant: 'info', label: '启动中' }
    case 'debug_pending':
      return { variant: 'info', label: '运行中（待就绪）' }
    case 'running':
      return { variant: 'success', label: '运行中' }
    case 'stopping':
      return { variant: 'default', label: '停止中' }
    case 'crashed':
      return { variant: 'error', label: '已崩溃' }
    case 'stopped':
      return { variant: 'warning', label: '已停止' }
  }
  // 回落：基于布尔字段推导（兼容 mock 数据与旧后端）
  if (running && !debugReady) return { variant: 'info', label: '运行中（待就绪）' }
  if (running) return { variant: 'success', label: '运行中' }
  return { variant: 'warning', label: '已停止' }
}

export function formatTime(value?: string): string {
  if (!value) return '-'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '-' : date.toLocaleString('zh-CN')
}
